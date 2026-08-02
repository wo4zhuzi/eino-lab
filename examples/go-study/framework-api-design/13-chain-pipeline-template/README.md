# 13. 使用 Chain 编写流水式 Graph

前置示例见 [12. 带 Branch 的 Graph 构建模板](../12-branch-graph-template/README.md)。

## 结论

当业务流程以顺序执行为主、局部包含 Branch 时，推荐直接使用 Eino 官方 `compose.Chain`。它会根据 `Append...` 顺序注册节点并自动连接 Edge，业务代码不必分别维护“节点表”和“Edge 表”。

Demo 12 直接使用 Graph API，适合学习底层拓扑；Demo 13 表达相同业务，但代码按流水线组织，更适合作为此类生产业务的起点。

## 文件职责

| 文件 | 只负责什么 |
|---|---|
| [`pipeline.go`](pipeline.go) | 顶层流水线的步骤顺序 |
| [`review_branch.go`](review_branch.go) | 审核分支、通过路径、人工路径和队列分支 |
| [`notification_branch.go`](notification_branch.go) | 审核结果记录后的通知分支 |
| [`local_state.go`](local_state.go) | 单次运行审计状态、并发安全访问和最终读取 |
| [`nodes.go`](nodes.go) | 每个节点的业务逻辑 |
| [`routes.go`](routes.go) | 三个 Branch 的条件判断 |
| [`domain.go`](domain.go) | 输入、输出、内部数据和节点名称 |
| [`main.go`](main.go) | 构建、Invoke 和输出结果 |
| [`main_test.go`](main_test.go) | 路径互斥、错误和取消验证 |

阅读流程时先只看 `pipeline.go`；需要展开审核或通知路径时，再进入对应的 Branch 文件；需要理解具体业务规则时，最后进入 `nodes.go` 或 `routes.go`。

## 流水线结构

主流程按照代码书写顺序运行：

```go
pipeline := compose.NewChain[ReviewRequest, ReviewResult](
    compose.WithGenLocalState(newReviewLocalState),
)
pipeline.
    AppendLambda(compose.InvokableLambda(requestToReviewContext), compose.WithNodeKey(nodeInputAdapter)).
    AppendLambda(compose.InvokableLambda(normalizeReview), compose.WithNodeKey(nodeNormalize)).
    AppendLambda(compose.InvokableLambda(appendChannelNotice), compose.WithNodeKey(nodeAppendChannelNotice)).
    AppendLambda(compose.InvokableLambda(inspectRefundNotice), compose.WithNodeKey(nodeInspectRefundNotice)).
    AppendBranch(newReviewBranch()).
    AppendLambda(compose.InvokableLambda(recordReviewResult), compose.WithNodeKey(nodeRecordReviewResult)).
    AppendBranch(newNotificationBranch()).
    AppendLambda(compose.InvokableLambda(attachLocalAudit), compose.WithNodeKey(nodeAttachLocalAudit))
```

完整业务拓扑：

```text
START(ReviewRequest)
  -> input_adapter
  -> normalize
  -> append_channel_notice
  -> inspect_refund_notice
       |
       | Branch 1
       ├── approve path
       |     -> approve
       |     -> archive_approved_review
       |     -> ReviewResult
       |
       └── manual_review path
             -> manual_review
             -> Branch 2
                  ├── standard_manual_queue -> ReviewResult
                  └── priority_manual_queue -> ReviewResult

审核路径汇聚为 ReviewResult
  -> record_review_result
  -> Branch 3
       ├── send_approved_notice ------\
       └── send_manual_review_notice --+-> attach_local_audit -> END
```

## Local State 如何传递

最外层 Chain 注册状态工厂：

```go
pipeline := compose.NewChain[ReviewRequest, ReviewResult](
    compose.WithGenLocalState(newReviewLocalState),
)
```

Eino 在每次 `Invoke` 开始时调用一次工厂：

```go
type reviewLocalState struct {
    audit []string
}

func newReviewLocalState(context.Context) *reviewLocalState {
    return &reviewLocalState{}
}
```

因此同一个 Runnable 并发处理两个请求时，会得到两个不同的 `reviewLocalState`，不是共享全局对象。

节点和 Branch 条件使用 `ProcessState` 访问当前运行状态：

```go
func appendLocalAudit(ctx context.Context, event string) error {
    return compose.ProcessState[*reviewLocalState](
        ctx,
        func(_ context.Context, state *reviewLocalState) error {
            state.audit = append(state.audit, event)
            return nil
        },
    )
}
```

`ProcessState` 的已验证行为：

- 当前子 Chain 没有同类型状态时，会继续查找父 Chain 的状态。
- 审核子 Chain、嵌套人工队列 Branch 和通知 Branch 因此能访问最外层创建的状态。
- Eino 在回调执行期间自动持有该状态的互斥锁。
- 内层如果定义同类型状态，会遮蔽父级同类型状态；本示例没有定义内层状态。

最终节点读取状态并复制到结果：

```go
func attachLocalAudit(
    ctx context.Context,
    result ReviewResult,
) (ReviewResult, error) {
    err := compose.ProcessState[*reviewLocalState](
        ctx,
        func(_ context.Context, state *reviewLocalState) error {
            state.audit = append(state.audit, "pipeline_completed")
            result.Audit = append([]string(nil), state.audit...)
            return nil
        },
    )
    return result, err
}
```

### 两种数据通道

本示例没有用 Local State 替代节点输入输出：

| 数据 | 传递方式 | 原因 |
|---|---|---|
| `content`、`score`、`ReviewResult` | Handler 参数和返回值 | 属于后续节点明确依赖的主业务数据 |
| Branch 决策和运行审计 | `reviewLocalState` | 属于跨节点旁路信息，不应污染每个 Handler 的输入输出类型 |

```text
显式数据流：ReviewRequest -> reviewContext -> ReviewResult
Local State：request_received -> Branch 决策 -> recorded -> completed
```

## Branch 后接 Node，再接 Branch

主 Chain 中现在直接包含用户请求的结构：

```go
pipeline.
    AppendBranch(newReviewBranch()).
    AppendLambda(
        compose.InvokableLambda(recordReviewResult),
        compose.WithNodeKey(nodeRecordReviewResult),
    ).
    AppendBranch(newNotificationBranch())
```

运行原理是：

```text
newReviewBranch 的某一条路径
  -> 两条路径都输出 ReviewResult
  -> Chain 自动把路径连接到 record_review_result
  -> record_review_result 仍输出 ReviewResult
  -> routeNotification 接收 ReviewResult
  -> 只选择一个通知节点
```

这里没有进入 `newReviewBranch` 的下沉层级增加公共节点，因为 `record_review_result` 对通过和人工审核两条路径都要执行。它应该放在父 Chain 的 `AppendBranch` 后面。

新的通知 Branch 单独定义在 `notification_branch.go`：

```go
func newNotificationBranch() *compose.ChainBranch {
    return compose.NewChainBranch(routeNotification).
        AddLambda(nodeSendApprovedNotice, compose.InvokableLambda(sendApprovedNotice)).
        AddLambda(nodeSendManualNotice, compose.InvokableLambda(sendManualReviewNotice))
}
```

## 分支就是子流水线

第一个 Branch 不直接堆放所有节点，而是指向两个命名子 Chain：

```go
func newReviewBranch() *compose.ChainBranch {
    return compose.NewChainBranch(routeReview).
        AddGraph(nodeApprove, newApprovePath()).
        AddGraph(nodeManualReview, newManualReviewPath())
}
```

通过路径只包含自己的节点：

```go
func newApprovePath() *compose.Chain[reviewContext, ReviewResult] {
    path := compose.NewChain[reviewContext, ReviewResult]()
    return path.
        AppendLambda(compose.InvokableLambda(approveReview), compose.WithNodeKey(nodeApprove)).
        AppendLambda(compose.InvokableLambda(archiveApprovedReview), compose.WithNodeKey(nodeArchiveApproved))
}
```

人工审核路径中可以继续嵌套第二个 Branch：

```go
func newManualReviewPath() *compose.Chain[reviewContext, ReviewResult] {
    path := compose.NewChain[reviewContext, ReviewResult]()
    return path.
        AppendLambda(compose.InvokableLambda(sendToManualReview), compose.WithNodeKey(nodeManualReview)).
        AppendBranch(newManualQueueBranch())
}
```

## 新增节点时改哪里

假设只在通过路径新增 `send_approved_notice`：

1. 在 `nodes.go` 增加 `sendApprovedNotice` Handler。
2. 在 `newApprovePath` 对应位置追加一行：

```go
AppendLambda(
    compose.InvokableLambda(sendApprovedNotice),
    compose.WithNodeKey("send_approved_notice"),
)
```

不需要修改节点注册区、Edge 数组、`START`、`END` 或公共编译方法。`Chain` 会按 `AppendLambda` 的位置自动连接前后节点。

## 新增 Branch 时改哪里

新增 Branch 仍然需要三个业务信息：

1. 条件函数：根据什么选择路径。
2. 目标路径：允许选择哪些 key。
3. 插入位置：在哪条 Chain 中调用 `AppendBranch`。

这三项无法由框架自动推断，但可以分别放在 `routes.go` 和对应的路径方法中，不需要维护底层 Edge。

## 为什么不再自定义公共 Builder

Eino 的 `Chain` 已经完成了流水式 Builder 应负责的工作：

- `AppendLambda` 按顺序自动连接节点。
- `AppendBranch` 把 Branch 接在当前步骤之后。
- `ChainBranch.AddGraph` 允许每个分支拥有自己的子流水线。
- `Compile` 自动连接各条终止路径到 `END`。

继续自定义一套 `linearStep`、Edge 生成器或 DSL，会增加学习成本，也容易丢失 Eino 自带的类型校验和错误处理。

## Graph 与 Chain 的取舍

| 场景 | 推荐 |
|---|---|
| 主要是顺序步骤，局部有 Branch 或 Parallel | `compose.Chain` |
| 需要任意连边、汇聚、循环或精确控制节点 key | `compose.Graph` |
| 当前这个审核流程 | `compose.Chain` |

Chain 不是替换 Graph 的另一套执行引擎。它在内部仍然构建 Graph，只是提供更适合流水线的声明方式。

## 运行与验证

仓库声明 Go `1.26.0`，Eino 锁定为 `v0.9.12`。示例不访问网络，也不需要凭据。

在仓库根目录执行：

```bash
go run ./examples/go-study/framework-api-design/13-chain-pipeline-template
go test ./examples/go-study/framework-api-design/13-chain-pipeline-template -count=1
go test -race ./examples/go-study/framework-api-design/13-chain-pipeline-template -count=1
```

预期输出：

```text
approved=true route=approve score=9 steps=[normalize append_channel_notice inspect_refund_notice approve archive_approved_review record_review_result send_approved_notice attach_local_audit] reasons=[包含退款到账说明] audit=[request_received review_branch=approve review_result_recorded notification_branch=send_approved_notice pipeline_completed]
approved=false route=manual_review score=5 steps=[normalize append_channel_notice inspect_refund_notice manual_review standard_manual_queue record_review_result send_manual_review_notice attach_local_audit] reasons=[缺少退款到账说明] audit=[request_received review_branch=manual_review manual_queue_branch=standard_manual_queue review_result_recorded notification_branch=send_manual_review_notice pipeline_completed]
approved=false route=manual_review score=3 steps=[normalize append_channel_notice inspect_refund_notice manual_review priority_manual_queue record_review_result send_manual_review_notice attach_local_audit] reasons=[未包含退款说明] audit=[request_received review_branch=manual_review manual_queue_branch=priority_manual_queue review_result_recorded notification_branch=send_manual_review_notice pipeline_completed]
```

测试覆盖审核、人工队列和通知三个 Branch 的路径互斥、Branch 后公共节点、Local State 审计顺序、同一 Runnable 并发 Invoke 的状态隔离、业务错误、nil context、context 取消和三个路由条件的取消传播。
