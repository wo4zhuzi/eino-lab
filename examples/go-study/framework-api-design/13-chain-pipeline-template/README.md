# 13. 使用 Chain 编写流水式 Graph

前置示例见 [12. 带 Branch 的 Graph 构建模板](../12-branch-graph-template/README.md)。

## 结论

当业务流程以顺序执行为主、局部包含 Branch 时，推荐直接使用 Eino 官方 `compose.Chain`。它会根据 `Append...` 顺序注册节点并自动连接 Edge，业务代码不必分别维护“节点表”和“Edge 表”。

Demo 12 直接使用 Graph API，适合学习底层拓扑；Demo 13 表达相同业务，但代码按流水线组织，更适合作为此类生产业务的起点。

## 文件职责

| 文件 | 只负责什么 |
|---|---|
| [`pipeline.go`](pipeline.go) | 主流水线、分支和每条路径的结构 |
| [`nodes.go`](nodes.go) | 每个节点的业务逻辑 |
| [`routes.go`](routes.go) | 两个 Branch 的条件判断 |
| [`domain.go`](domain.go) | 输入、输出、内部数据和节点名称 |
| [`main.go`](main.go) | 构建、Invoke 和输出结果 |
| [`main_test.go`](main_test.go) | 路径互斥、错误和取消验证 |

阅读流程时先只看 `pipeline.go`，需要理解具体业务规则时再进入 `nodes.go` 或 `routes.go`。

## 流水线结构

主流程按照代码书写顺序运行：

```go
pipeline := compose.NewChain[ReviewRequest, ReviewResult]()
pipeline.
    AppendLambda(compose.InvokableLambda(requestToReviewContext), compose.WithNodeKey(nodeInputAdapter)).
    AppendLambda(compose.InvokableLambda(normalizeReview), compose.WithNodeKey(nodeNormalize)).
    AppendLambda(compose.InvokableLambda(appendChannelNotice), compose.WithNodeKey(nodeAppendChannelNotice)).
    AppendLambda(compose.InvokableLambda(inspectRefundNotice), compose.WithNodeKey(nodeInspectRefundNotice)).
    AppendBranch(newReviewBranch())
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
       |     -> END
       |
       └── manual_review path
             -> manual_review
             -> Branch 2
                  ├── standard_manual_queue -> END
                  └── priority_manual_queue -> END

所有 END 最终返回 ReviewResult
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
approved=true route=approve score=9 steps=[normalize append_channel_notice inspect_refund_notice approve archive_approved_review] reasons=[包含退款到账说明]
approved=false route=manual_review score=5 steps=[normalize append_channel_notice inspect_refund_notice manual_review standard_manual_queue] reasons=[缺少退款到账说明]
approved=false route=manual_review score=3 steps=[normalize append_channel_notice inspect_refund_notice manual_review priority_manual_queue] reasons=[未包含退款说明]
```

测试覆盖三条最终路径互斥、业务错误、nil context、context 取消和两个路由条件的取消传播。
