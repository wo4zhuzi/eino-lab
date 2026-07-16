# 可诊断天气 Agent

## 学习目标

本示例用一条最小纵向链路验证 Eino ADK 的主路径：`ChatModelAgent -> weather_lookup Tool -> Runner -> AgentEvent`。示例同时通过 per-run Callback 记录组件、状态、耗时和错误类别，并验证 Tool 错误可以通过 `%w` 错误链返回应用入口。

涉及的 Eino 组件：

- `model.ToolCallingChatModel`：在线使用 EinoExt OpenAI 实现，测试使用 scripted 实现。
- `utils.InferTool`：从 `WeatherRequest` 推导 Tool schema 并解码参数。
- `adk.ChatModelAgent`：执行模型、Tool、模型的 ReAct 循环。
- `adk.Runner` 和 `adk.AgentEvent`：管理运行并向入口返回消息或错误。
- `callbacks.Handler`：按请求注入观测，不记录 prompt、Tool 参数或模型输出正文。

## 版本与前置条件

- Go `1.26.3`；模块 directive 为 `go 1.26.0`。
- Eino `v0.9.12`。
- EinoExt OpenAI `v0.1.13`。
- 在线运行需要兼容 OpenAI Chat Completions 与 Tool Calling 的模型服务。

环境变量：

| 变量 | 必填 | 用途 |
|---|---|---|
| `OPENAI_API_KEY` | 是 | 模型服务凭据 |
| `OPENAI_MODEL` | 是 | 支持 Tool Calling 的模型名 |
| `OPENAI_BASE_URL` | 否 | 自定义 OpenAI 兼容代理地址；不填时使用组件默认地址 |
| `WEATHER_AGENT_TIMEOUT` | 否 | 整次请求超时，默认 `15s` |

仓库根目录的 `.env` 不会被程序自动读取。当前 shell 可受控导入：

```bash
set -a
source .env
set +a
```

`.env` 必须使用 `KEY=value`，不能在键名与 `=` 之间留空格。不要提交真实密钥或代理地址。

## 运行

默认测试完全离线，不访问模型服务：

```bash
go test ./examples/diagnosable-weather-agent/... -count=1
go test -race ./examples/diagnosable-weather-agent/... -count=1
```

在线运行：

```bash
go run ./examples/diagnosable-weather-agent "北京天气怎么样？"
```

关键输入是用户天气问题。正常情况下，模型先生成 `weather_lookup` ToolCall，Tool 返回静态天气数据，模型再生成最终回答。标准错误输出会出现组件级结构化日志，标准输出是最终回答。回答措辞由模型决定，不应依赖完全一致的文本。

## 阶段 5 讨论结论

以下结论基于当前示例代码、离线测试和 Eino `v0.9.12` 源码验证，用于解释这条链路为什么存在，以及 Agent、模型和 Tool 分别负责什么。

角色分工、四条运行分支、错误责任和阶段 5 自测问答的完整记录见[阶段 5 运行与错误责任笔记](stage-5-notes.md)。本节只保留便于快速回顾的摘要。

### 这个天气 Agent 的作用

`已验证` 本示例不是为了替代一个确定的天气 API 调用，而是用最小业务验证 Eino ADK 的完整 ReAct 链路：

```text
用户自然语言
-> ChatModelAgent 调用模型
-> 模型产生 weather_lookup ToolCall
-> ToolsNode 执行本地 Tool
-> Tool 结果返回 ChatModelAgent
-> 模型基于结果生成最终回答
-> Runner 通过 AgentEvent 返回结果或错误
```

如果应用已经明确知道必须查询天气，并且只需要结构化天气数据，直接调用天气 API 更简单、便宜且确定。Agent 的价值出现在用户只表达目标、系统提供多个能力，需要模型判断调用哪个 Tool、如何填写参数，或者是否连续调用多个 Tool 的场景。

当前示例只有一个 Tool，并且 `Instruction` 要求模型回答前必须调用 `weather_lookup`。这是为了稳定触发并观察完整链路，不代表真实项目只能注册一个 Tool，也不代表所有 Agent 都必须调用 Tool。

### 当前 Tool 是什么

`已验证` 当前 `weather_lookup` 是项目自己实现的本地 Go Tool，不是 MCP Tool，也不调用真实天气 API：

- `NewWeatherTool` 使用 `utils.InferTool` 将类型化 Go 函数包装为 `tool.InvokableTool`。
- `WeatherRequest` 的字段和 tag 被推导为 Tool 参数 JSON Schema。
- `StaticWeatherProvider` 从进程内 `map` 返回北京、上海和深圳的固定数据。
- 当前模块没有接入 MCP Client 或 MCP Tool 适配器。

Tool 是 Agent 可调用能力的统一抽象，具体实现可以是本地函数、HTTP API、数据库操作，也可以是经过适配的 MCP Server 能力。MCP 是 Tool 的一种跨进程提供和发现方式，不是与 Tool 并列的另一套 Agent 调用机制。

```text
Eino Tool
|- 本地 Go 函数                  <- 当前示例
|- 本地 Tool -> HTTP 天气 API
|- 本地 Tool -> 数据库
`- MCP Tool -> MCP Server
```

对于单个应用自己维护的天气能力，优先使用本地 Tool 包装 HTTP Client。只有能力需要跨项目、跨语言或被多个 Agent 复用和发现时，MCP 才能提供更明显的工程收益。

### Tool 如何注册，模型如何知道

`已验证` Tool 不是注册到全局容器，而是显式传给当前 `ChatModelAgent`：

```go
ToolsConfig: adk.ToolsConfig{
	ToolsNodeConfig: compose.ToolsNodeConfig{
		Tools: []tool.BaseTool{weatherTool},
	},
},
```

同一个 Tool 对象承担两项职责：

| 接口能力 | 使用方 | 作用 |
|---|---|---|
| `Info(ctx)` | ChatModelAgent、模型组件 | 提供名称、描述和参数 JSON Schema |
| `InvokableRun(ctx, arguments)` | ToolsNode | 执行 Tool 并返回结果 |

注册后，Eino 的实际处理过程是：

1. `ChatModelAgent` 调用每个 `BaseTool.Info(ctx)`，得到 `[]*schema.ToolInfo`。
2. Eino 通过 `model.WithTools(...)` 把 Tool 的名称、描述和参数 Schema 传给模型服务。
3. 模型只能看到 Tool 的说明书，看不到 Go 函数实现。
4. 模型返回包含 Tool 名称和 JSON 参数的 `ToolCall`。
5. `ToolsNode` 使用名称索引找到对应执行入口并调用 `InvokableRun`。
6. 模型产生未注册的 Tool 名称时，当前配置会返回 `tool ... not found in toolsNode indexes` 错误。

因此模型的选择质量依赖清晰、准确且互相可区分的 Tool 名称、描述和参数 Schema。Tool 已实现但没有加入 `Tools` 切片时，模型看不到它，`ToolsNode` 也无法执行它。

### 一个 Agent 可以有多少 Tool

`已验证` `ToolsNodeConfig.Tools` 是 `[]tool.BaseTool`，一个 Agent 可以注册多个 Tool。模型可以在当前请求可见的 Tool 中选择一个、多个，或者不调用 Tool。模型不能调用未注册的任意能力。

生产环境通常不会把项目内所有 Tool 都暴露给同一个 Agent，而是只提供该 Agent 职责内、当前用户有权限使用的最小集合。例如旅行 Agent 可以使用天气、航班和酒店 Tool，订单 Agent 可以使用查询订单和退款 Tool。这样可以减少错误选择、无权限调用、提示上下文和模型成本。

Tool 实现必须在调用前已经存在，但可见集合不一定永久写死：可以在启动时静态注册，也可以在每次运行前根据用户权限、租户、配置或外部 Tool 发现结果动态选择。

### 当前哪些内容是写死的

`已验证` 当前固定的是 Tool 名称和描述、只支持三个城市的静态天气数据、单 Tool 配置、强制调用 Tool 的 Instruction，以及非流式 Runner。用户输入和模型生成的 `city` 参数不是写死的。

`WeatherProvider` 接口已经隔离数据来源。后续把 `StaticWeatherProvider` 单变量替换为调用真实天气 API 的实现时，可以保持 `NewWeatherTool`、Agent 注册和 ReAct 链路不变：

```text
当前：weather_lookup -> StaticWeatherProvider -> map
以后：weather_lookup -> HTTPWeatherProvider   -> 天气 API
```

这只是扩展边界说明，不代表阶段 6 迁移测试已经执行。

### 流式与非流式

`已验证` 当前示例是非流式：`NewWeatherTool` 返回 `tool.InvokableTool`，Runner 设置 `EnableStreaming=false`。模型和 Tool 都完成整个结果后再一次性向下游返回。

流式表示结果产生一部分就传递一部分，需要区分两个层面：

| 层面 | 非流式 | 流式 | 典型场景 |
|---|---|---|---|
| 模型输出 | 等完整 Assistant 消息后返回 | 按消息块逐步返回 | 长回答、实时对话展示 |
| Tool 输出 | 等完整 Tool 结果后返回 | 按结果块逐步返回 | 大规模搜索、日志读取、长报告生成 |

天气结果只有少量字段，使用非流式更简单合理。流式会增加流读取、关闭、拼接和中途错误处理成本，应在首包延迟或渐进展示确实有价值时采用。

## 故障语义

| 类别 | 入口分类 | 行为 |
|---|---|---|
| 请求超时 | `deadline_exceeded` | Tool 停止等待，错误经 `AgentEvent.Err` 返回，不生成最终回答 |
| 城市不支持 | `unsupported_city` | 保留 `ErrUnsupportedCity` 错误链，不让模型编造结果 |
| 天气依赖不可用 | `weather_unavailable` | 保留 `ErrWeatherUnavailable` 错误链并快速失败 |

## 已知限制

- 第一版固定 `EnableStreaming=false`；流式消费在学习阶段 6 单独迁移。
- 天气数据是进程内静态数据，只支持北京、上海和深圳，不代表实时天气。
- 在线模型是否正确选择 Tool 具有非确定性，因此在线运行只作为冒烟；回归依据是 scripted ChatModel 离线测试。
- 当前示例不包含重试、failover、checkpoint、多 Agent、RAG 或生产级日志后端。
