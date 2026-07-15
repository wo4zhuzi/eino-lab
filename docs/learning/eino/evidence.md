# Eino v0.9.12 学习证据表

## 证据规则

- `已验证`：本轮已从精确 tag/commit 的源码、测试文件或命令输出核对。
- `官方说明`：来自版本匹配的官方 README 或示例，但尚未实际运行。
- `推断`：由多项证据得出的学习选择，必须在后续实验校正。
- 证据优先级：当前版本源码/测试/运行结果 > 当前版本官方文档与示例 > 维护者说明 > 社区资料。

## 结论证据

| ID | 结论 | 标签 | 精确证据 | 版本 | 置信度 | 后续验证 |
|---|---|---|---|---|---|---|
| E-01 | 当前最新非预发布 tag 候选是 `v0.9.12`；`v0.10.0` 只有 alpha tag | 已验证 | 官方仓库 `git ls-remote --refs --tags`；[`v0.9.12`](https://github.com/cloudwego/eino/tree/v0.9.12) 浅克隆 HEAD 为 `13e1a25c...` | 2026-07-15 refs | 高 | 决策门 1 确认是否采用 |
| E-02 | Eino 官方定位为遵循 Go 习惯的 LLM 应用开发框架，能力包括 Components、ADK、Composition | 官方说明 | [`README.md` Overview 与能力列表](https://github.com/cloudwego/eino/blob/v0.9.12/README.md) | `v0.9.12` | 高 | 无需额外实验；能力边界由源码继续校正 |
| E-03 | `v0.9.12` 的 Quick Start 首先推荐 `ChatModelAgent`，再说明精确控制流使用 Compose | 官方说明 | [`README.md` Quick Start](https://github.com/cloudwego/eino/blob/v0.9.12/README.md) | `v0.9.12` | 高 | 阶段 2 运行 ChatModelAgent 示例 |
| E-04 | `Runner` 是执行 Agent 的主要入口，并负责 flowAgent 管线、回调、命名、run path 与取消 | 已验证 | [`adk/runner.go` `TypedRunner`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/runner.go#L50)；`Query` 将字符串转成用户消息后调用 `Run` | `v0.9.12` | 高 | 阶段 5 从 `Runner.Query` 追踪真实请求 |
| E-05 | `ChatModelAgent` 在配置 Tool 时构建 ReAct 运行函数，否则走无 Tool 路径 | 已验证 | [`adk/chatmodel.go` `ToolsConfig`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/chatmodel.go#L136)；同文件 `getRunFunc` 在 `buildNoToolsRunFunc` 与 `buildReActRunFunc` 间选择 | `v0.9.12` | 高 | 阶段 4 用受控 ChatModel + Tool 运行验证 |
| E-06 | 模型与工具通过公开组件接口接入；Tool schema 与执行能力分离 | 已验证 | [`components/model/interface.go`](https://github.com/cloudwego/eino/blob/v0.9.12/components/model/interface.go#L31)；[`components/tool/interface.go`](https://github.com/cloudwego/eino/blob/v0.9.12/components/tool/interface.go#L25) | `v0.9.12` | 高 | 阶段 4 用测试实现替换外部依赖 |
| E-07 | Compose 编译产物 `Runnable` 统一支持 Invoke、Stream、Collect、Transform，并能做数据流范式适配 | 已验证 | [`compose/runnable.go` `Runnable`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/runnable.go#L28)；[`compose/generic_graph.go` `Graph.Compile`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/generic_graph.go#L110) | `v0.9.12` | 高 | 本轮仅建立边界；后续单独实践 Compose |
| E-08 | Callbacks 覆盖五个固定时点；流返回后的读取错误不会进入 `OnError`，流副本必须关闭 | 已验证 | [`callbacks/doc.go`](https://github.com/cloudwego/eino/blob/v0.9.12/callbacks/doc.go#L17)；[`callbacks/interface.go`](https://github.com/cloudwego/eino/blob/v0.9.12/callbacks/interface.go#L62) | `v0.9.12` | 高 | 阶段 4 注入流内错误并验证双通道观测 |
| E-09 | 官方示例仓库当前 commit 精确依赖 Eino `v0.9.12`，根模块要求 Go `1.24.7`，当前 Go 1.26.3 满足要求 | 已验证 | [`eino-examples/go.mod`](https://github.com/cloudwego/eino-examples/blob/171220631fb7068ead50b7cd964b8c471647117d/go.mod#L1) | commit `171220631...` | 高 | 阶段 2 实际运行示例 |
| E-10 | 官方 `adk/intro/chatmodel` 示例覆盖 Agent、Tool、Runner、事件、Interrupt/Resume 与 CheckpointStore | 官方说明 | [`chatmodel.go`](https://github.com/cloudwego/eino-examples/blob/171220631fb7068ead50b7cd964b8c471647117d/adk/intro/chatmodel/chatmodel.go#L34)；[`subagents/agent.go`](https://github.com/cloudwego/eino-examples/blob/171220631fb7068ead50b7cd964b8c471647117d/adk/intro/chatmodel/subagents/agent.go#L31)；[`booksearch.go`](https://github.com/cloudwego/eino-examples/blob/171220631fb7068ead50b7cd964b8c471647117d/adk/intro/chatmodel/subagents/booksearch.go#L27) | 示例 commit `171220631...` + Eino `v0.9.12` | 高 | 阶段 2 原样运行并记录输出 |
| E-11 | 官方示例默认从环境变量创建 OpenAI 兼容模型，也可用 `MODEL_TYPE=ark` 切换 Ark | 已验证 | [`adk/common/model/chat_model.go`](https://github.com/cloudwego/eino-examples/blob/171220631fb7068ead50b7cd964b8c471647117d/adk/common/model/chat_model.go#L35) | 示例 commit `171220631...` | 高 | 阶段 2 检查变量存在但不输出值 |
| E-12 | Eino 源码测试覆盖 Tool ReAct、per-run callbacks 与取消边界，但本轮没有执行这些测试 | 已验证 | [`adk/chatmodel_test.go`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/chatmodel_test.go)、[`adk/callback_integration_test.go`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/callback_integration_test.go)、[`adk/cancel_edge_test.go`](https://github.com/cloudwego/eino/blob/v0.9.12/adk/cancel_edge_test.go) | `v0.9.12` | 中 | 阶段 2/4 运行最小相关测试；当前仅证明上游存在覆盖 |
| E-13 | “ADK 为本轮主线，Compose 为次级路径”是学习范围选择，不是 Eino 只支持 ADK 的事实 | 建议 | E-03、E-04、E-05 与 E-07 交叉 | `v0.9.12` | 高 | 决策门 1 由用户确认 |

## 实际命令记录

| 命令 | 关键结果 | 状态 |
|---|---|---|
| `go version` | `go version go1.26.3 darwin/arm64` | 已执行；与用户确认的 Go 1.26 一致 |
| `git ls-remote --refs --tags https://github.com/cloudwego/eino.git` | 存在稳定 tag `v0.9.12`；更高版本仅见 `v0.10.0-alpha.*` | 已执行 |
| `git clone --depth 1 --branch v0.9.12 ...` | HEAD `13e1a25c7238293a1e558391a65525a464acb324`，tag `v0.9.12` | 已执行，源码位于系统临时目录且不纳入仓库 |
| 读取 Eino `go.mod` | 模块 `github.com/cloudwego/eino`，`go 1.18` | 已执行 |
| `git clone --depth 1 https://github.com/cloudwego/eino-examples.git ...` | HEAD `171220631fb7068ead50b7cd964b8c471647117d` | 已执行，源码位于系统临时目录且不纳入仓库 |
| 读取 Eino Examples `go.mod` | `go 1.24.7`，依赖 `github.com/cloudwego/eino v0.9.12` | 已执行 |
| 官方示例 `go run` | 未执行：协议模式要求停在决策门 1，且尚未确认 Go/凭据策略 | 待阶段 2 |
| `go test ./...` / `go vet ./...` | 未执行：当前学习仓库尚无 `go.mod`，本轮只新增文档 | 不适用，待实现阶段 |

## 低置信度与冲突项

| 项目 | 当前结论 | 风险 | 处理 |
|---|---|---|---|
| 官网文档版本 | 官网是滚动文档，未证明与 `v0.9.12` 一一对应 | 示例 API 可能先于或晚于 tag | 所有关键 API 回到 tag 源码核对 |
| 官方示例可运行性 | 依赖版本和 Go 工具链匹配，但仍要求真实模型服务 | 无凭据或模型端点不可达时不能原样运行 | 阻塞时如实记录，不用阅读代替运行 |
| 回调完整捕获错误 | 非流错误可进入 `OnError`，流读取期错误不会 | 只看回调会漏报 | AgentEvent/StreamReader 错误与 Callback 同时观测 |
