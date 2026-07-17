# Eino 天气 Agent 故障矩阵

## 验证范围

- 版本：Go `1.26.3`、Eino `v0.9.12`、EinoExt OpenAI `v0.1.13`。
- 示例：`examples/diagnosable-weather-agent/`，阶段 6 已迁移为 `RunnerConfig.EnableStreaming=true`。
- 默认测试使用 scripted `ToolCallingChatModel`，不访问网络。
- 所有 Tool 失败均作为 error 返回，不使用 `WrapToolWithErrorHandler` 转成普通字符串。

## 实际结果

| 场景 | 注入位置 | 实际传播路径 | 入口断言 | Callback 证据 | 结果 |
|---|---|---|---|---|---|
| 正常流式 ReAct | scripted model 第一轮生成 `weather_lookup` ToolCall，第二轮返回两个回答分块 | Model stream -> ToolsNode -> Tool -> Tool message -> Model stream -> final `AgentEvent` | 两次 `Stream` 调用；第二轮看到匹配 `ToolCallID` 的 Tool message；分块拼接正确 | `Tool/weather_lookup` 有 `started`、`succeeded` 记录 | 已通过 |
| 请求超时 | Provider 等待 `ctx.Done()`；测试 deadline 为 30ms | Provider -> Tool `%w` -> ToolsNode `%w` -> `AgentEvent.Err` -> `WeatherAgent.Query` | `errors.Is(err, context.DeadlineExceeded)` 为真；最终消息为 nil | `Tool` 的失败记录为 `deadline_exceeded` | 已通过 |
| 不支持城市 | Provider 返回 `ErrUnsupportedCity` | Provider -> Tool `%w` -> ToolsNode `%w` -> `AgentEvent.Err` -> `WeatherAgent.Query` | `errors.Is(err, ErrUnsupportedCity)` 为真；最终消息为 nil | `Tool` 的失败记录为 `unsupported_city` | 已通过 |
| 天气依赖不可用 | Provider 返回 `ErrWeatherUnavailable` | Provider -> Tool `%w` -> ToolsNode `%w` -> `AgentEvent.Err` -> `WeatherAgent.Query` | `errors.Is(err, ErrWeatherUnavailable)` 为真；最终消息为 nil | `Tool` 的失败记录为 `weather_unavailable` | 已通过 |
| 模型流中途错误 | scripted model 的第二次 Stream 先发送部分内容，再发送 `errModelStreamInterrupted` | Model 返回 StreamReader -> `AgentEvent.MessageStream` -> `adk.GetMessage` | 最终消息为 nil；`errors.Is` 保留原错误 | ChatModel 有 `started`，没有 `failed`；证明流错误不能只靠 `OnError` | 已通过 |
| 流副本关闭 | `adk.GetMessage` 复制并拼接 Pipe stream | 入口关闭返回事件中的保留副本 | 再次 `Recv` 返回 `schema.ErrRecvAfterClosed` | 不适用 | 已通过 |

Callback 记录只包含组件、名称、类型、状态、耗时和错误类别。测试额外断言记录中不包含城市参数；实现也不记录 prompt、Tool 输出、模型正文或完整原始错误。

## 在线冒烟

2026-07-17 使用仓库现有 `.env` 中的自定义 OpenAI 兼容代理运行流式模式成功，未输出密钥或代理地址：

```bash
go run ./examples/diagnosable-weather-agent "北京天气怎么样？"
```

实际事件与 Callback 顺序包含：第一次 OpenAI ChatModel 流调用、`weather_lookup` Tool 调用、第二次 OpenAI ChatModel 流调用和最终回答。Tool 返回北京晴天、28°C、湿度 35%，最终回答与该受控数据一致，进程退出码为 0。

沙箱内首次运行在约 2ms 的请求建立阶段失败；在获准使用宿主网络后原样重跑成功，说明失败来自本机代理的沙箱网络边界，不是流式代码或服务端能力不兼容。

在线冒烟只证明当前模型服务可以完成一次真实 Tool Calling，不替代离线回归测试。

## 验证命令

| 命令 | 结果 |
|---|---|
| `go mod tidy` | 通过；生成 `go.sum` |
| `gofmt -w examples/diagnosable-weather-agent/*.go` | 通过 |
| `go test ./examples/diagnosable-weather-agent/... -run 'TestWeatherAgentReAct\|TestWeatherAgentStreamError\|TestConsumeAgentEventMessageClosesRetainedStream' -count=1 -v` | 通过；分块、流错误和显式关闭均符合预测 |
| `go test -race ./examples/diagnosable-weather-agent/... -count=1` | 通过 |
| `go test ./... -count=1` | 通过 |
| `go vet ./...` | 通过 |

首次在沙箱内执行 `go mod tidy` 时，本机代理 `127.0.0.1:6478` 被沙箱网络策略阻止。经授权在沙箱外执行同一命令后成功，说明该失败属于执行环境网络边界，不是依赖版本冲突。

## 阶段结论

阶段 4 的三类故障在阶段 6 流式迁移后全部回归通过；新增的分块拼接、流副本关闭和流内错误观测也已验证。迁移只改变 Runner 执行范式与入口消费方式，没有改变 Agent、Tool 或 Provider 契约。
