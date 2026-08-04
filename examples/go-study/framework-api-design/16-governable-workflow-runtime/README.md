# 16. 可治理的多工作流运行层

前置示例见 [15. 多工作流与公共运行层](../15-multiple-workflows/README.md)。

## 结论

当多个 Eino 工作流需要进入同一个长期运行的应用时，公共层除了 `Compile once / Run many`，还需要提供稳定的执行治理契约：

- `Descriptor{Name, Version}` 标识工作流定义，支持日志、指标、灰度和兼容判断。
- 每次 `Run` 强制提供 `RunID`，由 HTTP、RPC、消息或任务入口负责生成。
- `Observer` 把日志、Trace 和指标适配为 Eino Callback，不污染业务输入输出。
- `RunOption` 只暴露经过选择的运行能力，不把任意 Eino Option 泄漏到业务调用方。
- `OperationError` 携带工作流、版本、RunID 和生命周期阶段，同时通过 `Unwrap` 保留原始错误链。
- 节点同时设置稳定 `NodeKey` 与 `NodeName`：Key 用于拓扑寻址，Name 用于 Callback 观测。

本示例仍然不抽取业务拓扑 DSL，不使用 `map[string]any` 注册不同输入输出的工作流。

## 学习目标

1. 把工作流定义身份与单次执行身份分开管理。
2. 使用 Eino Callback 观测多个工作流和具体业务节点。
3. 理解 `WithNodeKey` 与 `WithNodeName` 的不同职责。
4. 为运行时增加结构化错误和请求级最大步数保护。
5. 将通用执行轨迹移出业务 `Result`。

仓库声明 Go `1.26.0`，当前运行环境为 Go `1.26.3`，Eino 锁定为 `v0.9.12`。示例只使用本地确定性依赖，不访问网络，也不需要凭据。

## 相比 Demo 15 的变化

| 能力 | Demo 15 | Demo 16 |
|---|---|---|
| 工作流身份 | 只有名称 | 名称与定义版本 |
| 执行身份 | 无 | 每次运行必须提供 RunID |
| 运行时 Option | 未透传 | 通过受控 `RunOption` 转换为 Eino Option |
| Callback | 未接入 | 通过 `Observer` 按次注入 |
| 节点身份 | 只有 `NodeKey` | 同时设置 `NodeKey` 和 `NodeName` |
| 错误 | 文本包装并保留错误链 | 结构化 `OperationError` 并保留错误链 |
| 执行轨迹 | 混在业务 Result 的 `Steps` | 独立 `Event`，业务 Result 只保留业务数据 |
| 运行保护 | 只有编译级配置 | 支持请求级最大步数覆盖 |

`已验证` 在 Eino `v0.9.12` 中，`WithNodeKey` 不会自动填充 Lambda Callback 的 `RunInfo.Name`；需要同时设置 `WithNodeName`。本示例的测试会断言通过、人工审核、检索改写等节点名称确实出现在 Observer 事件中。

## 目录结构

```text
16-governable-workflow-runtime/
├── main.go                         应用装配和两次可观测执行
├── README.md                       设计、运行和生产边界
├── workflowkit/
│   ├── runtime.go                  Descriptor、RunOption、Runner 和结构化错误
│   ├── recorder.go                 并发安全的示例 Observer
│   └── runtime_test.go             生命周期、Option、错误和并发测试
├── reviewworkflow/                 强类型内容审核工作流
└── ragworkflow/                    强类型本地 RAG 工作流
```

审核和 RAG 继续独立拥有自己的 Config、Dependencies、Request、Result、节点和拓扑。`workflowkit` 不知道审核分数、检索次数、证据和答案是什么。

## 运行链路

```text
应用启动
  -> 校验业务 Config 和 Dependencies
  -> 使用 Descriptor 编译一次 Chain/Graph
  -> 保存 Runnable 到 Runner

请求到达
  -> 入口生成 RunID
  -> 解析 RunOption
  -> Observer(execution) 转换为 Eino Callback
  -> Runnable.Invoke(ctx, input, compose options...)
  -> 成功：返回纯业务 Result
  -> 失败：返回可解包的 OperationError
```

一次执行由以下信息唯一描述：

```text
Execution
├── Descriptor.Name       例如 content_review
├── Descriptor.Version    例如 v1
└── RunID                 例如 review-demo-001
```

不要在 Runner 内自动生成 RunID。HTTP Request ID、RPC Trace ID、消息 ID 或任务 ID 应由真实调用入口负责，并保持与入口日志一致。

## Observer 与节点名称

`workflowkit.Recorder` 是并发安全的教学实现。每条事件包含：

- 工作流名称、定义版本和 RunID。
- Eino 组件类型、名称和生命周期状态。
- 执行耗时和通用错误分类。

生产环境应实现自己的 `workflowkit.Observer`，将事件写入日志、OpenTelemetry 或指标系统，不应长期保存到进程内切片。

节点构建必须同时表达两个身份：

```text
NodeKey   -> Edge、Branch、DesignateNode 和错误路径使用
NodeName  -> Callback 的 RunInfo.Name 使用
```

两者在当前示例中使用相同稳定字符串，但职责不同，不能因为值相同而省略其中一个。

## 错误契约

`OperationError` 只增加治理上下文，不改变基础错误语义：

```text
OperationError
├── Execution.Descriptor
├── Execution.RunID
├── Operation: compile/run
└── Cause: context、业务错误或 Eino 错误
```

调用方应使用 `errors.Is` 判断 `context.Canceled`、`context.DeadlineExceeded`、业务错误和 `compose.ErrExceedMaxSteps`，使用 `errors.As` 读取 `OperationError`。不要解析错误字符串或依赖 Eino 未导出的内部错误类型。

公共 Runner 不自动重试整个 Graph。重跑整个流程可能重复模型调用、数据库写入或消息发送；重试应放在具体依赖适配器，并由副作用节点提供幂等键。

## 如何增加第三个工作流

1. 新建独立业务目录，例如 `orderworkflow/`。
2. 定义稳定 `Descriptor{Name: "order_fulfillment", Version: "v1"}`。
3. 定义自己的 Request、Result、Config 和 Dependencies。
4. 使用 Eino Chain/Graph 显式构建拓扑，并同时设置 NodeKey 与 NodeName。
5. 在 `New` 中调用 `workflowkit.Compile`，在 `Run` 中透传 `workflowkit.RunOption`。
6. 为正常路径、业务错误、context 取消、Observer 和并发运行补充测试。

增加第三个工作流不需要修改 `workflowkit`。只有新的业务再次证明存在业务无关的重复时，才扩展公共层。

## 运行与验证

在仓库根目录执行：

```bash
go run ./examples/go-study/framework-api-design/16-governable-workflow-runtime
go test ./examples/go-study/framework-api-design/16-governable-workflow-runtime/... -count=1
go test -race ./examples/go-study/framework-api-design/16-governable-workflow-runtime/... -count=1
go vet ./examples/go-study/framework-api-design/16-governable-workflow-runtime/...
```

预期输出：

```text
workflow=content_review@v1 run_id=review-demo-001 route=approved score=9
workflow=local_rag@v1 run_id=rag-demo-001 no_evidence=false attempts=2 evidence=2
observed_events=30
```

Callback 事件数与当前 Eino 版本及拓扑节点数量相关，生产逻辑不得依赖固定事件总数。测试只断言关键工作流身份、RunID、节点名称、状态和错误语义。

## 已知限制与生产边界

- `Recorder` 只用于离线验证，不是日志或 Trace 后端。
- 当前只有单值 `Invoke`。需要流式输出时应增加独立的流式门面，并测试 Reader 关闭和流内错误。
- 当前没有 checkpoint、跨进程恢复、工作流版本迁移和人工暂停。
- 当前没有副作用节点、幂等存储、限流、熔断和真实外部依赖。
- 工作流由 Go 代码定义，修改拓扑后需要重新构建和发布。
- 需要小时级任务、进程崩溃恢复、人工任务和可靠副作用时，应使用持久工作流引擎承载生命周期，把 Eino Graph 作为其中的 AI 编排活动，而不是继续扩大 `workflowkit` 的职责。
