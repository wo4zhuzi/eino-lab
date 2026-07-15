# Eino v0.9.12 学习证据表

## 证据规则

- `已验证`：本轮已从精确 tag/commit 的源码、测试文件或命令输出核对。
- `官方说明`：来自版本匹配的官方 README 或示例，但尚未实际运行。
- `推断`：由多项证据得出的学习选择，必须在后续实验校正。
- 证据优先级：当前版本源码/测试/运行结果 > 当前版本官方文档与示例 > 维护者说明 > 社区资料。

## 结论证据

| ID | 结论 | 标签 | 精确证据 | 版本 | 置信度 | 后续验证 |
|---|---|---|---|---|---|---|
| E-01 | 当前最新非预发布 tag 是 `v0.9.12`；`v0.10.0` 只有 alpha tag，用户已确认采用稳定版 | 已验证 | 官方仓库 `git ls-remote --refs --tags`；[`v0.9.12`](https://github.com/cloudwego/eino/tree/v0.9.12) 浅克隆 HEAD 为 `13e1a25c...`；决策门 1 | 2026-07-15 refs | 高 | 已确认 |
| E-02 | Eino 官方定位为遵循 Go 习惯的 LLM 应用开发框架，能力包括 Components、ADK、Composition | 官方说明 | [`README.md` Overview 与能力列表](https://github.com/cloudwego/eino/blob/v0.9.12/README.md) | `v0.9.12` | 高 | 无需额外实验；能力边界由源码继续校正 |
| E-03 | `v0.9.12` 的 Quick Start 首先推荐 `ChatModelAgent`，再说明精确控制流使用 Compose | 官方说明 | [`README.md` Quick Start](https://github.com/cloudwego/eino/blob/v0.9.12/README.md) | `v0.9.12` | 高 | 阶段 2 已运行 ChatModelAgent 示例 |
| E-04 | `Runner` 是执行 Agent 的主要入口，并负责 flowAgent 管线、回调、命名、run path 与取消 | 已验证 | [`adk/runner.go` `TypedRunner`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/runner.go#L50)；`Query` 将字符串转成用户消息后调用 `Run` | `v0.9.12` | 高 | 阶段 5 从 `Runner.Query` 追踪真实请求 |
| E-05 | `ChatModelAgent` 在配置 Tool 时构建 ReAct 运行函数，否则走无 Tool 路径 | 已验证 | [`adk/chatmodel.go` `ToolsConfig`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/chatmodel.go#L136)；同文件 `getRunFunc` 在 `buildNoToolsRunFunc` 与 `buildReActRunFunc` 间选择 | `v0.9.12` | 高 | 阶段 4 用受控 ChatModel + Tool 运行验证 |
| E-06 | 模型与工具通过公开组件接口接入；Tool schema 与执行能力分离 | 已验证 | [`components/model/interface.go`](https://github.com/cloudwego/eino/blob/v0.9.12/components/model/interface.go#L31)；[`components/tool/interface.go`](https://github.com/cloudwego/eino/blob/v0.9.12/components/tool/interface.go#L25) | `v0.9.12` | 高 | 阶段 4 用测试实现替换外部依赖 |
| E-07 | Compose 编译产物 `Runnable` 统一支持 Invoke、Stream、Collect、Transform，并能做数据流范式适配 | 已验证 | [`compose/runnable.go` `Runnable`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/runnable.go#L28)；[`compose/generic_graph.go` `Graph.Compile`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/generic_graph.go#L110) | `v0.9.12` | 高 | 本轮仅建立边界；后续单独实践 Compose |
| E-08 | Callbacks 覆盖五个固定时点；流返回后的读取错误不会进入 `OnError`，流副本必须关闭 | 已验证 | [`callbacks/doc.go`](https://github.com/cloudwego/eino/blob/v0.9.12/callbacks/doc.go#L17)；[`callbacks/interface.go`](https://github.com/cloudwego/eino/blob/v0.9.12/callbacks/interface.go#L62) | `v0.9.12` | 高 | 阶段 4 注入流内错误并验证双通道观测 |
| E-09 | 官方示例仓库当前 commit 精确依赖 Eino `v0.9.12`，根模块要求 Go `1.24.7`，当前 Go 1.26.3 满足要求 | 已验证 | [`eino-examples/go.mod`](https://github.com/cloudwego/eino-examples/blob/171220631fb7068ead50b7cd964b8c471647117d/go.mod#L1) | commit `171220631...` | 高 | 阶段 2 实际运行已完成 |
| E-10 | 官方 `adk/intro/chatmodel` 示例实际覆盖 Agent、Tool、Runner、事件、Interrupt/Resume 与 CheckpointStore | 已验证 | [`chatmodel.go`](https://github.com/cloudwego/eino-examples/blob/171220631fb7068ead50b7cd964b8c471647117d/adk/intro/chatmodel/chatmodel.go#L34)；[`subagents/agent.go`](https://github.com/cloudwego/eino-examples/blob/171220631fb7068ead50b7cd964b8c471647117d/adk/intro/chatmodel/subagents/agent.go#L31)；阶段 2 运行输出 | 示例 commit `171220631...` + Eino `v0.9.12` | 高 | 已完成 |
| E-11 | 官方示例默认从环境变量创建 OpenAI 兼容模型，也可用 `MODEL_TYPE=ark` 切换 Ark | 已验证 | [`adk/common/model/chat_model.go`](https://github.com/cloudwego/eino-examples/blob/171220631fb7068ead50b7cd964b8c471647117d/adk/common/model/chat_model.go#L35) | 示例 commit `171220631...` | 高 | 变量检查与实际运行均已完成 |
| E-12 | Eino 源码测试覆盖 Tool ReAct、per-run callbacks 与取消边界，但本轮没有执行这些测试 | 已验证 | [`adk/chatmodel_test.go`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/chatmodel_test.go)、[`adk/callback_integration_test.go`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/callback_integration_test.go)、[`adk/cancel_edge_test.go`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/cancel_edge_test.go) | `v0.9.12` | 中 | 阶段 2/4 运行最小相关测试；当前仅证明上游存在覆盖 |
| E-13 | “ADK 为本轮主线，Compose 为次级路径”是已确认的学习范围选择，不是 Eino 只支持 ADK 的事实 | 建议 | E-03、E-04、E-05、E-07 与决策门 1 交叉 | `v0.9.12` | 高 | 已确认 |
| E-14 | 官方 `adk/intro/chatmodel` 示例可在 Go 1.26.3 下以锁定依赖构建 | 已验证 | 对示例 commit `171220631...` 执行 `go build -o <临时文件> ./adk/intro/chatmodel`，退出码 0，生成 arm64 Mach-O 可执行文件 | Eino `v0.9.12` | 高 | 构建兼容性已验证；不能替代在线运行 |
| E-15 | `.env` 键名与 `=` 之间的空格导致变量未导入，EinoExt 收到空 BaseURL 后回退到 OpenAI 默认端点 | 已验证 | 首次运行错误为 `NodeRunError`，目标为 `api.openai.com`；受控导入后 BaseURL 为空；规范化键格式后识别为有效非默认地址 | 2026-07-15 环境 | 高 | 已修复，并将 `.env` 权限收紧为 `0600` |
| E-16 | 自定义 OpenAI 兼容代理可驱动官方 ChatModelAgent 完成 Tool、Interrupt、Resume 和最终回答 | 已验证 | 运行锁定 commit 的官方二进制；实际输出依次出现 `ask_for_clarification`、交互输入、`search_book`、Tool response 与 answer；退出码 0 | Eino `v0.9.12` | 高 | 阶段 2 已完成 |
| E-17 | ToolsNode 对普通 Tool error 使用 `%w` 包装并终止调用，错误链可沿 AgentEvent 返回 | 已验证 | [`compose/tool_node.go` `ToolsNode.Invoke`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/tool_node.go#L1097) 在 task error 非 interrupt 时返回 `failed to invoke tool...: %w` | Eino `v0.9.12` | 高 | 阶段 4 用 `errors.Is` 验证三类错误 |
| E-18 | `WrapToolWithErrorHandler` 会把 Tool error 转换为字符串结果并返回 nil error | 已验证 | [`components/tool/utils/error_handler.go`](https://github.com/cloudwego/eino/blob/v0.9.12/components/tool/utils/error_handler.go#L27) 与 `errorHelper.InvokableRun` | Eino `v0.9.12` | 高 | 纵向项目不使用该 wrapper，避免掩盖 L2 错误通道 |
| E-19 | `adk.GetMessage` 能从完整或流式 AgentEvent 提取消息，并在流式场景复制后拼接 stream | 已验证 | [`adk/utils.go` `TypedGetMessage`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/utils.go#L278) | Eino `v0.9.12` | 高 | 非流式入口先采用；阶段 6 核对流式迁移预测 |
| E-20 | ChatModelAgent 在 `v0.9.12` 中把 Tool schema 作为调用级 `model.WithTools` option 传给模型，而不是依赖预先修改模型实例 | 已验证 | [`adk/chatmodel.go` ReAct 构建](https://github.com/cloudwego/eino/blob/v0.9.12/adk/chatmodel.go#L1468)；scripted model 通过 `model.GetCommonOptions` 实际接收 ToolInfo | Eino `v0.9.12` | 高 | 阶段 5 纳入运行链路图 |
| E-21 | 自定义天气 Agent 的非流式 ReAct、三类错误链和 per-run Callback 均可离线重复验证 | 已验证 | `examples/diagnosable-weather-agent/*_test.go`；`docs/learning/eino/failure-matrix.md`；仓库测试命令 | Eino `v0.9.12` + Go `1.26.3` | 高 | 阶段 6 流式迁移后回归 |
| E-22 | 自定义 OpenAI 兼容代理可驱动天气 Agent 完成两次模型调用和一次 `weather_lookup` Tool 调用 | 已验证 | 2026-07-15 在线冒烟 Callback 日志与最终回答；退出码 0 | Eino `v0.9.12` + EinoExt OpenAI `v0.1.13` | 高 | 在线结果仅作冒烟，离线测试仍是回归基线 |
| E-23 | `Runner.Query` 把字符串转为 UserMessage 后，所有 `*schema.Message` Agent 都进入 flowAgent 生命周期 | 已验证 | [`adk/runner.go`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/runner.go#L38) `newUserMessage`、`Query`、`typedRunnerRunImpl`；[`adk/flow.go`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/flow.go#L352) `flowAgent.Run` | Eino `v0.9.12` | 高 | 已纳入 `runtime-path.md` |
| E-24 | ChatModelAgent 首次运行冻结默认配置，但 ReAct Graph 在每次执行时创建并编译 | 已验证 | [`adk/chatmodel.go`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/chatmodel.go#L1366) `buildRunFunc` 的 `sync.Once`；同文件 `buildMessageReActRunFunc` run closure 内 `newReact` 与 `Chain.Compile` | Eino `v0.9.12` | 高 | 阶段 6 核对流式模式是否只改变执行分支 |
| E-25 | 默认 model/tool event-sender wrapper 把两类成功输出主动写入 Agent generator，flowAgent 再补 AgentName/RunPath 并转发 | 已验证 | [`adk/wrappers.go`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/wrappers.go#L261) model sender、同文件 Tool sender；[`adk/flow.go`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/flow.go#L478) `flowAgent.run` | Eino `v0.9.12` | 高 | 已纳入 `source-map.md` |
| E-26 | Tool 错误先由 InferTool 和 ToolsNode 以 `%w` 包装，再由可 `Unwrap` 的 Compose NodeRunError 增加节点路径，最终进入 `AgentEvent.Err` | 已验证 | [`invokable_func.go`](https://github.com/cloudwego/eino/blob/v0.9.12/components/tool/utils/invokable_func.go#L174)、[`tool_node.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/tool_node.go#L1097)、[`error.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/error.go#L62)、阶段 4 `errors.Is` 测试 | Eino `v0.9.12` | 高 | 已完成非流式错误链追踪 |
| E-27 | EinoExt OpenAI 从调用级 model options 读取 ToolInfo，转换为 Chat Completions tools，并用同一 ctx 发起 HTTP 请求 | 已验证 | EinoExt OpenAI `v0.1.13` `Generate`；ACL OpenAI `v0.1.17` `genRequest`、`Generate` 与 `CreateChatCompletion` | 精确模块版本 | 高 | 已纳入外部边界导航 |

## 实际命令记录

| 命令 | 关键结果 | 状态 |
|---|---|---|
| `go version` | `go version go1.26.3 darwin/arm64` | 已执行；与用户确认的 Go 1.26 一致 |
| `git ls-remote --refs --tags https://github.com/cloudwego/eino.git` | 存在稳定 tag `v0.9.12`；更高版本仅见 `v0.10.0-alpha.*` | 已执行 |
| `git clone --depth 1 --branch v0.9.12 ...` | HEAD `13e1a25c7238293a1e558391a65525a464acb324`，tag `v0.9.12` | 已执行，源码位于系统临时目录且不纳入仓库 |
| 读取 Eino `go.mod` | 模块 `github.com/cloudwego/eino`，`go 1.18` | 已执行 |
| `git clone --depth 1 https://github.com/cloudwego/eino-examples.git ...` | HEAD `171220631fb7068ead50b7cd964b8c471647117d` | 已执行，源码位于系统临时目录且不纳入仓库 |
| 读取 Eino Examples `go.mod` | `go 1.24.7`，依赖 `github.com/cloudwego/eino v0.9.12` | 已执行 |
| `GOMODCACHE=<临时目录> GOCACHE=<临时目录> go build -o <临时文件> ./adk/intro/chatmodel` | 首次受沙箱内本机代理限制；通过已授权代理重试后退出码 0，生成 arm64 Mach-O 可执行文件 | 已执行，构建通过 |
| 使用未规范化 `.env` 运行官方示例二进制 | ChatModel 节点请求 OpenAI 默认端点并返回 `EOF`；节点路径为 `[node_1, ChatModel]` | 已执行，失败根因已定位 |
| 规范化 `.env` 键格式并重新运行官方示例二进制 | 依次输出澄清 Tool、Interrupt、补充输入、搜索 Tool、Tool response 和最终回答；退出码 0 | 已执行，阶段 2 通过 |
| `go mod tidy` | 锁定依赖解析成功并生成 `go.sum`；首次沙箱内执行受本机代理网络边界阻止，授权后成功 | 已执行 |
| `go test ./examples/diagnosable-weather-agent/... -run 'TestWeather\|TestStatic\|TestNewWeather' -count=1` | 正常 ReAct、三类故障、Tool/Provider 与 Callback 测试通过 | 已执行 |
| `go test -race ./examples/diagnosable-weather-agent/... -count=1` | 通过，无竞态报告 | 已执行 |
| `go test ./... -count=1` | 通过 | 已执行 |
| `go vet ./...` | 通过 | 已执行 |
| 使用现有 `.env` 运行天气 Agent | 自定义代理完成两次 OpenAI ChatModel 调用和一次 `weather_lookup`；输出与静态数据一致；退出码 0 | 已执行，阶段 4 通过 |
| 沿在线天气请求追踪 Eino/EinoExt 精确版本源码 | 已定位 Runner、flowAgent、ReAct、OpenAI、ToolsNode、event sender、Callback 和 NodeRunError 的文件与符号 | 已执行，阶段 5 通过 |

## 低置信度与冲突项

| 项目 | 当前结论 | 风险 | 处理 |
|---|---|---|---|
| 官网文档版本 | 官网是滚动文档，未证明与 `v0.9.12` 一一对应 | 示例 API 可能先于或晚于 tag | 所有关键 API 回到 tag 源码核对 |
| 回调完整捕获错误 | 非流错误可进入 `OnError`，流读取期错误不会 | 只看回调会漏报 | AgentEvent/StreamReader 错误与 Callback 同时观测 |
