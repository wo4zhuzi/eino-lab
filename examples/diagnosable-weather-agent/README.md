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
