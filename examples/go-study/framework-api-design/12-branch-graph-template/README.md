# 12. 带 Branch 的 Graph 构建模板

前置示例见 [11. 可复用线性 Graph 构建模板](../11-linear-graph-template/README.md)。

## 结论

出现 Branch 后，不需要重写所有节点 Handler，但不能继续把整个 Graph 当成一个有序切片自动连边。推荐保留公共创建和编译方法，把业务节点、固定 Edge 和 Branch 集中声明在 `defineReviewGraph` 中。

本示例的拓扑是：

```text
START(ReviewRequest)
  -> input_adapter
  -> normalize
  -> append_channel_notice
  -> inspect_refund_notice
       |
       | Branch 1：审核结果
       ├── score >= 8 -> approve -> archive_approved_review -> END
       |
       └── score < 8  -> manual_review
                              |
                              | Branch 2：人工队列
                              ├── score >= 5 -> standard_manual_queue -> END
                              └── score < 5  -> priority_manual_queue -> END

所有 END 最终返回 ReviewResult
```

## 为什么不能只在 Demo 11 后面补 AddBranch

Demo 11 的 `compileLinearGraph` 会根据步骤切片自动生成每一条固定 Edge。假设它已经生成：

```text
inspect_refund_notice -> output_adapter
```

再给同一个起点添加 Branch，不会覆盖或删除这条固定 Edge。运行时固定后继和 Branch 选中的后继都可能被调度，这不是“二选一”。

所以本示例保留已有 Handler，只升级拓扑声明方式：

- 公共层 `compileDefinedGraph[I, O]`：固定创建 Graph、调用业务定义、Compile。
- 业务层 `defineReviewGraph`：显式登记节点、固定 Edge、Branch 和分支后的 Edge。
- 运行层 `routeReview` 和 `routeManualQueue`：每次 Invoke 到达对应分叉点时，根据当前结果返回一个目标节点 key。

## 输入输出仍在创建时固定

业务入口直接写明：

```go
compileDefinedGraph[ReviewRequest, ReviewResult](
    ctx,
    "branch_review_graph",
    defineReviewGraph,
)
```

因此 Graph 对外始终接收 `ReviewRequest`，最终必须产出 `ReviewResult`。Branch 只改变中途执行哪个节点，不能在运行时修改 Graph 的输入、输出类型。

三条最终路径连接到 `END` 时都是 `ReviewResult`：

```text
approve:                 reviewContext -> ReviewResult
archive_approved_review: ReviewResult  -> ReviewResult -> END
manual_review:           reviewContext -> ReviewResult
standard_manual_queue:   ReviewResult  -> ReviewResult -> END
priority_manual_queue:   ReviewResult  -> ReviewResult -> END
```

这保证无论选择哪条路径，连接到 `END` 的值都符合 Graph 的最终输出类型。

## AddBranch 的三个参数关系

核心注册代码位于 [`review.go`](review.go)：

```go
branch := compose.NewGraphBranch(
    routeReview,
    map[string]bool{
        nodeApprove:      true,
        nodeManualReview: true,
    },
)
err := graph.AddBranch(nodeInspectRefundNotice, branch)
```

含义分别是：

| 代码 | 含义 | 执行时机 |
|---|---|---|
| `nodeInspectRefundNotice` | 从哪个节点之后开始分叉 | 注册期 |
| `routeReview` | 根据该节点的输出选择目标 key | 每次 Invoke 的运行期 |
| `map[string]bool{...}` | 条件函数允许返回的目标节点白名单 | 注册期保存、运行期校验 |

`routeReview` 返回 `approve` 时，Eino 才调用已注册的 `approveReview`；它本身不会直接调用这个 Handler。执行完 `approve` 后，固定 Edge 再把结果交给 `archive_approved_review`。未选中的 `manual_review` 不会执行。

## 增加第二个 Branch

第二个 Branch 从 `manual_review` 后开始：

```go
manualQueueBranch := compose.NewGraphBranch(
    routeManualQueue,
    map[string]bool{
        nodeStandardManualQueue: true,
        nodePriorityManualQueue: true,
    },
)

err := graph.AddBranch(nodeManualReview, manualQueueBranch)
```

条件函数根据人工审核结果选择队列：

```go
func routeManualQueue(
    ctx context.Context,
    result ReviewResult,
) (string, error) {
    if err := ctx.Err(); err != nil {
        return "", fmt.Errorf("选择人工队列分支: %w", err)
    }
    if result.Score >= 5 {
        return nodeStandardManualQueue, nil
    }
    return nodePriorityManualQueue, nil
}
```

这里不能保留原来的 `manual_review -> END`。第二个 Branch 需要成为 `manual_review` 唯一的后继选择方式，再由两个队列节点分别连接 `END`：

```go
{nodeStandardManualQueue, compose.END},
{nodePriorityManualQueue, compose.END},
```

两个 Branch 的条件输入类型不同：

| Branch 起点 | 起点输出 | 条件函数 |
|---|---|---|
| `inspect_refund_notice` | `reviewContext` | `routeReview(context.Context, reviewContext)` |
| `manual_review` | `ReviewResult` | `routeManualQueue(context.Context, ReviewResult)` |

因此 `NewGraphBranch` 的泛型类型不是全局固定的，它由当前分叉起点传出的数据类型决定。

## 在一个分支后增加普通节点

本示例在 `approve` 后新增了 `archive_approved_review`。需要修改的只有 `defineReviewGraph` 和新增节点 Handler。

第一步，注册节点：

```go
if err := addReviewNode(graph, nodeArchiveApproved, archiveApprovedReview); err != nil {
    return err
}
```

第二步，把原来的 `approve -> END` 改成两条 Edge：

```go
{nodeApprove, nodeArchiveApproved},
{nodeArchiveApproved, compose.END},
```

新增节点是普通 Handler：

```go
func archiveApprovedReview(
    ctx context.Context,
    result ReviewResult,
) (ReviewResult, error) {
    if err := ctx.Err(); err != nil {
        return ReviewResult{}, fmt.Errorf("归档通过结果: %w", err)
    }
    result.Steps = append(result.Steps, nodeArchiveApproved)
    return result, nil
}
```

Branch 的白名单和 `routeReview` 都不需要修改，因为 Branch 仍然只选择第一个目标 `approve`。`archive_approved_review` 是 `approve` 后面的普通节点，由固定 Edge 调度。

## 以后增加节点时改哪里

在分支前增加普通节点，需要在 `defineReviewGraph` 中完成两项修改：

1. 调用 `addReviewNode` 注册节点。
2. 调整对应的固定 Edge，把新节点接入路径。

增加新的分支目标，需要完成三项修改：

1. 注册新的目标节点。
2. 把节点 key 加入 `NewGraphBranch` 的白名单，并让 `routeReview` 在对应条件下返回它。
3. 为目标节点连接后续节点或 `END`。

`compileDefinedGraph`、`NewReviewGraph` 和 `main` 不需要变化。与线性步骤切片相比，多改 Edge 和目标白名单是非线性拓扑必须表达的信息，无法只靠数组顺序准确推导。

## 注册期、编译期和运行期

```text
NewReviewGraph
  -> 注册期：defineReviewGraph 登记节点、Edge、Branch
  -> 编译期：Compile 校验并生成 Runnable

runnable.Invoke(request)
  -> 运行期：执行固定节点
  -> 运行期：Branch 1 的 routeReview 返回一个目标 key
  -> approve 路径：归档后结束
  -> manual_review 路径：继续执行 Branch 2
  -> 运行期：Branch 2 的 routeManualQueue 返回一个队列 key
  -> 返回 ReviewResult
```

同一个编译后的 `Runnable` 可以处理多次请求，每次请求都可以根据自己的分数走不同路径。

## 运行与验证

仓库声明 Go `1.26.0`，Eino 锁定为 `v0.9.12`。示例不访问网络，也不需要凭据。

在仓库根目录执行：

```bash
go run ./examples/go-study/framework-api-design/12-branch-graph-template
go test ./examples/go-study/framework-api-design/12-branch-graph-template -count=1
go test -race ./examples/go-study/framework-api-design/12-branch-graph-template -count=1
```

预期输出包含三条最终路径：

```text
approved=true route=approve score=9 steps=[normalize append_channel_notice inspect_refund_notice approve archive_approved_review] reasons=[包含退款到账说明]
approved=false route=manual_review score=5 steps=[normalize append_channel_notice inspect_refund_notice manual_review standard_manual_queue] reasons=[缺少退款到账说明]
approved=false route=manual_review score=3 steps=[normalize append_channel_notice inspect_refund_notice manual_review priority_manual_queue] reasons=[未包含退款说明]
```

测试覆盖两个 Branch 的三条最终路径、通过分支归档节点、普通与优先人工队列互斥、未选中分支不执行、业务错误、context 取消、条件函数错误传播、非法目标节点以及公共构建参数校验。

## 已验证边界

- `已验证`：`NewGraphBranch` 是单选分支，每次只返回并调度一个目标节点。
- `已验证`：条件函数返回不在白名单中的 key 时，Invoke 返回错误。
- `已验证`：条件函数返回的错误会沿 Invoke 返回。
- `限制`：本示例没有并行分支、循环、流式 Branch 或状态持久化；这些能力应使用对应 Eino API 单独建例学习。
