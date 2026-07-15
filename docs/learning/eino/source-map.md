# Eino v0.9.12 源码导航

## 版本与边界

本导航只引用当前项目锁定版本：

- Eino `v0.9.12`，commit `13e1a25c7238293a1e558391a65525a464acb324`。
- EinoExt OpenAI `v0.1.13`。
- EinoExt ACL OpenAI `v0.1.17`，由前一模块的依赖解析得到。

这些源码仅作为事实来源。应用代码只依赖公开包，不导入 Eino `internal` 包。

## 推荐阅读顺序

| 顺序 | 文件与关键符号 | 要回答的问题 |
|---|---|---|
| 1 | [agent.go](../../../examples/diagnosable-weather-agent/agent.go) `NewWeatherAgent`、`Query` | 应用把哪些对象交给 Eino，如何消费事件？ |
| 2 | [`adk/runner.go`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/runner.go#L38) `newUserMessage`、`Runner.Query`、`typedRunnerRunImpl` | 字符串怎样进入统一 Agent 生命周期？ |
| 3 | [`adk/flow.go`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/flow.go#L352) `flowAgent.Run`、`flowAgent.run` | Agent callback、run path 和内部事件怎样转发？ |
| 4 | [`adk/chatmodel.go`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/chatmodel.go#L855) `prepareExecContext`、`buildRunFunc`、`Run` | Agent 何时读取 ToolInfo、冻结配置并选择 ReAct？ |
| 5 | [`adk/react.go`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/react.go#L312) `genToolInfos`、`newReact` | ChatModel 和 ToolsNode 如何形成循环？ |
| 6 | [`components/model/option.go`](https://github.com/cloudwego/eino/blob/v0.9.12/components/model/option.go#L115) `WithTools`、`GetCommonOptions` | Tool schema 如何作为调用级配置传给模型？ |
| 7 | [EinoExt OpenAI `chatmodel.go`](https://github.com/cloudwego/eino-ext/blob/components/model/openai/v0.1.13/components/model/openai/chatmodel.go#L198) `NewChatModel`、`Generate` | 标准模型组件怎样进入 OpenAI ACL？ |
| 8 | [ACL OpenAI `chat_model.go`](https://github.com/cloudwego/eino-ext/blob/libs/acl/openai/v0.1.17/libs/acl/openai/chat_model.go#L557) `genRequest`、`Generate` | Eino message/ToolInfo 怎样变成 Chat Completions 请求？ |
| 9 | [`compose/tool_node.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/tool_node.go#L1044) `ToolsNode.Invoke` | ToolCall 如何按名称和 call ID 调度？ |
| 10 | [`components/tool/utils/invokable_func.go`](https://github.com/cloudwego/eino/blob/v0.9.12/components/tool/utils/invokable_func.go#L38) `InferTool`、`InvokableRun` | JSON schema、解码、领域函数和编码怎样衔接？ |
| 11 | [`adk/wrappers.go`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/wrappers.go#L261) model/tool event sender | Assistant 和 Tool 结果怎样变成 AgentEvent？ |
| 12 | [`compose/error.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/error.go#L62) `wrapGraphNodeError`、`internalError.Unwrap` | 节点路径为何不会破坏 `errors.Is`？ |
| 13 | [`adk/utils.go`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/utils.go#L278) `GetMessage` | 应用如何统一提取完整或流式消息？ |

## 关键调用关系

```text
WeatherAgent.Query
└── Runner.Query
    └── Runner.Run
        └── typedRunnerRunImpl
            └── flowAgent.Run
                └── ChatModelAgent.Run
                    ├── buildRunFunc [首次 sync.Once]
                    │   ├── prepareExecContext
                    │   │   └── genToolInfos -> BaseTool.Info
                    │   └── buildReActRunFunc
                    └── run closure [每次请求]
                        ├── newReact
                        │   ├── AddChatModelNode
                        │   ├── AddToolsNode
                        │   └── branches / loop edges
                        ├── Chain.Compile
                        └── Runnable.Invoke
                            ├── OpenAI.Generate
                            ├── ToolsNode.Invoke
                            │   └── InferTool.InvokableRun
                            │       └── WeatherProvider.Lookup
                            └── OpenAI.Generate
```

## 核心抽象

| 抽象 | 当前项目实现 | 运行责任 | 替换影响 |
|---|---|---|---|
| `model.BaseChatModel` / `ToolCallingChatModel` | EinoExt OpenAI；测试 scripted model | 接收 message history 和调用级 ToolInfo，返回 Assistant message | 可换模型组件；Agent、Tool 和 Provider 契约不变 |
| `tool.BaseTool` | `InferTool` 返回对象 | 提供模型可见的名称、描述与参数 schema | schema 变化会影响模型请求和 Tool 参数 |
| `tool.InvokableTool` | `weather_lookup` | 接收 JSON arguments，返回字符串结果或 error | 可换实现；ToolsNode 调度不变 |
| `adk.Agent` | `ChatModelAgent` | 选择无 Tool/有 Tool 路径并发出 AgentEvent | 替换 Agent 会改变主要控制流 |
| `adk.Runner` | 非流式 Runner | 创建输入、进入 flowAgent、管理运行生命周期 | 阶段 6 只改 streaming flag，不替换 Runner |
| `compose.Graph` | ADK 内部创建的 ReAct Graph | ChatModel/Tool 分支、循环、state 和节点错误路径 | 应用不直接操作这张内部图 |
| `callbacks.Handler` | `Observer.Handler()` | per-run 观测组件时点 | 可换日志/Trace 后端，不应改变业务结果 |
| `WeatherProvider` | 静态实现；测试故障实现 | 应用领域依赖边界，不属于 Eino | 可换真实天气服务，不影响 Eino Tool 契约 |

## 关键扩展点

| 扩展点 | 公开位置 | 当前选择 | 风险边界 |
|---|---|---|---|
| 模型替换 | `model.BaseModel`、`model.Option` | 在线 OpenAI，离线 scripted | 实现必须处理 `model.WithTools` 调用级 option |
| Tool 新增/替换 | `ToolsConfig.Tools`、`tool.BaseTool` | 单 `weather_lookup` | Tool name 必须唯一；参数 schema 与执行输入必须一致 |
| Tool 中间件 | `ToolsNodeConfig.ToolCallMiddlewares` | 仅框架默认 event sender/cancel wrapper | wrapper 顺序会影响事件看到的结果和错误 |
| Agent 中间件 | `ChatModelAgentConfig.Handlers` | 未配置 | 可改写模型 state/Tool；可能触发每次运行重建图 |
| per-run 观测 | `adk.WithCallbacks` | 每次 Query 注入 Observer | 不应保存未同步的跨请求可变状态 |
| 取消 | `context.Context` 与 ADK cancel option | 当前只用 deadline ctx | 额外 cancel mode 会引入中断安全点，不在本阶段范围 |
| 流式模式 | `RunnerConfig.EnableStreaming` | `false` | 阶段 6 的唯一迁移变量 |

## 生命周期与隐式行为

1. `NewChatModelAgent` 校验模型并加入默认 Tool event-sender middleware，但不调用模型。
2. 第一次 `Run` 通过 `sync.Once` 读取 ToolInfo、选择 ReAct，并把默认 run closure 冻结。
3. ReAct Graph 在 run closure 内创建并编译，所以每次 Query 都有独立 state 和 graph execution。
4. `ChatModelAgent.Run` 把 ToolInfo 包装成 `compose.WithChatModelOption(model.WithTools(...))`；这是调用级注入，不是调用 `ToolCallingChatModel.WithTools` 生成新实例。
5. 默认 model event sender 在模型成功返回后发 Assistant event；默认 Tool event sender 在 Tool 成功后发 Tool event。
6. flowAgent 为事件补齐 `AgentName` 与 `RunPath`，同时复制事件给 callback 和 session；应用收到的 iterator 在 generator 关闭后结束。

## 错误导航

| 故障位置 | 第一包装点 | 框架包装点 | 入口语义 |
|---|---|---|---|
| OpenAI 请求构造/HTTP | ACL `Generate` 使用 `%w` | Compose `NodeRunError` -> `AgentEvent.Err` | `internal` 或可识别的 context error |
| Tool JSON 解码 | `InferTool.InvokableRun` | ToolsNode `%w` -> Compose `NodeRunError` | `internal` |
| Provider 业务错误 | `weather_lookup` 与 InferTool `%w` | ToolsNode `%w` -> Compose `NodeRunError` | `unsupported_city` / `weather_unavailable` |
| Provider deadline | Provider ctx error | 同上 | `deadline_exceeded` |
| ReAct 最大轮次 | ChatModel state pre-handler | Compose `NodeRunError` -> `AgentEvent.Err` | `internal`，原始错误仍可 unwrap |

`compose.internalError.Unwrap()` 返回原始错误，因此增加 `[NodeRunError]` 和节点路径只改善定位，不会破坏 `errors.Is`。

## Callback 导航

- [`adk/call_option.go`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/call_option.go#L75)：`WithCallbacks` 保存 per-run handlers。
- [`adk/callback.go`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/callback.go#L116)：flowAgent 为 Agent 注入 `RunInfo`。
- [`compose/utils.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/utils.go#L100)：统一执行 `OnStart -> component -> OnEnd/OnError`。
- [`compose/tool_node.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/tool_node.go#L892)：Tool 调用前替换为 Tool 自己的 `RunInfo`。
- ACL OpenAI `Generate`：模型组件显式触发 `OnStart`、`OnEnd`、`OnError`。

## 已关闭的文档版本问题

官方官网是滚动文档，不能证明与 `v0.9.12` 完全一致。本阶段所有运行链路结论已经回到 tag 源码、精确模块版本和实际测试核对，因此官网版本差异不再阻塞当前 L2 主路径；未追踪的 API 仍不能据滚动文档直接外推到 `v0.9.12`。
