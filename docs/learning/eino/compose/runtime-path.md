# Compose 内容质量门禁运行链路

本文从一次“模拟客服生成草稿 -> `QualityGate.Review` -> 发送或转人工”调用出发，追踪 Eino Compose `v0.9.12` 的数据、状态、分支和错误路径。

## 请求链路

```text
用户问题
  -> CustomerReplyGenerator.Generate
  -> ReviewRequest{Content: 草稿}
  -> Runnable.Invoke
  -> Graph START
  -> validate(ReviewRequest -> gatePayload)
  -> inspect(gatePayload -> gatePayload)
  -> Branch(读取 gatePayload.Score + Local State.Attempts)
       -> approve(gatePayload -> ReviewResult)
       -> remediate(gatePayload -> gatePayload) -> inspect
       -> manual(gatePayload -> ReviewResult)
  -> END
  -> CustomerReplyDelivery
       -> approved: 模拟发送给用户
       -> manual_review: 模拟进入人工队列
```

## 逐步说明

1. `CustomerReplyGenerator` 根据用户问题生成草稿；当前是确定性的模拟实现，后续可单独替换为真实 ChatModel。
2. `NewQualityGate` 使用 `compose.NewGraph[ReviewRequest, ReviewResult]` 创建类型化 Graph，并通过 `WithGenLocalState` 注册 `gateState` 生成器。每次 `Runnable.Invoke` 都应得到一个新的状态实例。
3. `validate` 去除首尾空白；空内容直接返回 `ErrEmptyContent`。此时不调用 `Inspector`。
4. `inspect` 调用注入的 `Inspector`，校验评分范围，并通过 `compose.ProcessState` 增加尝试次数和审计记录。
5. `inspect` 的输出进入 `GraphBranch`。达标进入 `approve`；未达标且未达到 `MaxAttempts` 进入 `remediate`；达到上限进入 `manual`。
6. `remediate` 只做确定性内容补救，然后沿显式边回到 `inspect`。因此每次补救都必须重新检查，不能绕过质量门禁。
7. `approve` 或 `manual` 读取 Local State 的审计副本，构造 `ReviewResult` 并连接到 `END`。
8. `CustomerReplyDelivery` 根据 `ReviewResult.Status` 模拟发送回复或进入人工队列，形成业务终态。
9. `NewQualityGate` 在 Compile 时注入 `TopologySnapshotter`。它只复制名称、类型、节点、边和分支目标，不保留 Graph 节点实例。

## Compose 源码对应关系

| 运行步骤 | Eino 源码 | 关键行为 |
|---|---|---|
| Graph 创建 | `compose/generic_graph.go:72-88` | `NewGraph` 保存输入输出类型、状态生成器和新建选项 |
| 节点/边构建 | `compose/generic_graph.go:97-124`、`compose/graph.go:426-467` | `AddLambdaNode` 和 `AddEdge` 先记录拓扑，`Compile` 生成 Runnable |
| 编译回调 | `compose/generic_graph.go:127-151`、`compose/introspect.go:41-58` | 编译完成后将 `GraphInfo` 交给 `OnFinish` |
| Runnable 入口 | `compose/runnable.go:28-36` | `Invoke`、`Stream`、`Collect`、`Transform` 统一暴露 |
| 调度循环 | `compose/graph_run.go:240-260` | 每步检查取消、最大步数，提交并等待节点任务 |
| Local State | `compose/state.go:34-52`、`compose/state.go:165-190` | 状态带互斥锁；`ProcessState` 按类型查找并锁定状态 |
| 错误包装 | `compose/error.go:26-54`、`compose/error.go:79-105` | 超步数错误和节点路径包装，`Unwrap` 保留根因 |

## 取消和错误返回

Graph 运行循环在提交下一批任务前检查 `ctx.Done()`；节点错误由 Compose 追加节点路径后返回。示例中的 `Inspector` 仍必须主动响应 `ctx`，Compose 不会替外部依赖实现取消协议。

## 可观测点

- Compile：`TopologySnapshotter.OnFinish` 记录稳定拓扑摘要。
- 节点生命周期：`compose.WithCallbacks(observer.Handler())` 记录 Lambda 节点开始、成功和失败。
- 业务结果：`ReviewResult.Attempts` 和 `Audit` 提供本次运行的审计摘要。
- 错误分类：`errors.Is` 识别根因，Callback 的 `ErrorKind` 生成稳定分类。
