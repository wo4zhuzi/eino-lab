# Eino Compose v0.9.12 学习证据表

## 证据规则

- `已验证`：已从当前锁定模块、源码、测试代码或本机命令核对。
- `官方说明`：来自版本匹配的官方 README 或示例，尚未实际运行。
- `推断`：由多项证据推导，需后续实验校正。
- 证据优先级：当前版本源码、测试和运行结果 > 当前版本官方文档与示例 > 其他材料。

## 版本证据

| ID | 结论 | 标签 | 精确证据 | 置信度 | 后续验证 |
|---|---|---|---|---|---|
| C-01 | 当前项目实际依赖 Eino `v0.9.12` | 已验证 | 根 `go.mod`、`go.sum`；`go list -m` 解析到本机模块缓存 | 高 | 无需变更 |
| C-02 | 官方示例 commit `171220631f...` 精确依赖 Eino `v0.9.12` | 已验证 | [`eino-examples/go.mod`](https://github.com/cloudwego/eino-examples/blob/171220631fb7068ead50b7cd964b8c471647117d/go.mod) 与本机临时克隆 HEAD | 高 | 阶段 2 原样运行示例 |
| C-03 | 当前工具链为 Go `1.26.3 darwin/arm64` | 已验证 | `go env GOVERSION GOOS GOARCH`；官方 state 示例实际运行 | 高 | 无需变更 |

## 主路径证据

| ID | 结论 | 标签 | 精确证据 | 置信度 | 后续验证 |
|---|---|---|---|---|---|
| C-04 | Compose 用于连接组件形成可独立运行或暴露为 Agent Tool 的 Graph/Workflow | 官方说明 | [`README.md` Composition 定位](https://github.com/cloudwego/eino/blob/v0.9.12/README.md#overview) | 高 | 本轮只验证独立 Graph |
| C-05 | 需要精确控制执行流时，官方 Quick Start 使用 `NewGraph -> AddNode/AddEdge -> Compile -> Invoke` | 官方说明 | [`README.md` Composition Quick Start](https://github.com/cloudwego/eino/blob/v0.9.12/README.md#composition) | 高 | 阶段 2 和阶段 4 运行校正 |
| C-06 | Graph 支持组件、Lambda、Chain、Parallel 等节点，并编译为 `Runnable` | 已验证 | [`compose/generic_graph.go` `NewGraph` 与 `Compile`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/generic_graph.go#L46) | 高 | 阶段 4 构建自定义 Graph |
| C-07 | `Runnable` 统一 Invoke、Stream、Collect、Transform，并为缺失范式提供适配 | 已验证 | [`compose/runnable.go` `Runnable`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/runnable.go#L28) 及适配函数 | 高 | 阶段 6 单变量迁移 |
| C-08 | Chain 是构建在 Graph 之上的链式 builder | 已验证 | [`compose/chain.go` `NewChain`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/chain.go#L36) | 高 | 仅作为边界对照 |
| C-09 | Workflow 使用 `AllPredecessor`，源码明确不支持循环 | 已验证 | [`compose/workflow.go` `Workflow`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/workflow.go#L43) | 高 | 不作为本轮循环主路径 |
| C-10 | Graph Branch 只能返回预先声明的目标节点 | 已验证 | [`compose/branch.go` `NewGraphBranch`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/branch.go#L145) | 高 | 阶段 4 验证非法目标错误 |
| C-11 | `WithMaxRunSteps` 为循环提供最大执行步数保护 | 已验证 | [`compose/graph_compile_options.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/graph_compile_options.go#L53)、[`compose/error.go` `ErrExceedMaxSteps`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/error.go#L26) | 高 | 阶段 4 故障注入 |

## 状态、错误与扩展证据

| ID | 结论 | 标签 | 精确证据 | 置信度 | 后续验证 |
|---|---|---|---|---|---|
| C-12 | `WithGenLocalState` 为每次 Graph 运行生成本地状态 | 已验证 | [`compose/generic_graph.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/generic_graph.go#L35) | 高 | 阶段 4 并发调用验证隔离 |
| C-13 | `ProcessState` 使用互斥锁访问当前或父 Graph 状态，并支持内层同类型状态遮蔽外层状态 | 已验证 | [`compose/state.go` `ProcessState`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/state.go#L114) 与 `state_test.go` 嵌套并发测试 | 高 | 阶段 4 验证自定义项目状态隔离 |
| C-14 | 非流式 State Pre/Post Handler 在流式调用中会读取并合并全部分块 | 已验证 | [`compose/state.go` handler 注释](https://github.com/cloudwego/eino/blob/v0.9.12/compose/state.go#L40) | 高 | 阶段 6 决定是否改用 Stream Handler |
| C-15 | 节点错误增加 Graph 和节点路径，同时通过 `Unwrap` 保留原始错误链 | 已验证 | [`compose/error.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/error.go#L54) | 高 | 阶段 4 使用 `errors.Is` 验证 |
| C-16 | `GraphCompileCallback` 在编译完成时得到节点、边、分支和嵌套 Graph 元数据 | 已验证 | [`compose/introspect.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/introspect.go#L26)、`graph_test.go:TestGraphCompileCallback` | 高 | 阶段 4 实现拓扑快照器 |
| C-17 | 编译回调没有错误返回值，不能否决编译 | 已验证 | [`GraphCompileCallback.OnFinish`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/introspect.go#L54) | 高 | 扩展只承担观测职责 |
| C-18 | 每次编译可注入回调；全局回调只添加到顶层 Graph | 已验证 | [`WithGraphCompileCallbacks` 与 `InitGraphCompileCallbacks`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/graph_compile_options.go#L103) | 高 | 自定义项目采用每次编译注入 |

## 官方示例证据

| ID | 结论 | 标签 | 精确证据 | 置信度 | 后续验证 |
|---|---|---|---|---|---|
| C-19 | 官方 state Graph 示例覆盖 Local State、前后处理器、Branch、循环和最大步数 | 已验证 | [`compose/graph/state/state_graph.go`](https://github.com/cloudwego/eino-examples/blob/171220631fb7068ead50b7cd964b8c471647117d/compose/graph/state/state_graph.go#L115)；阶段 2 原样运行退出码 0 | 高 | 阶段 4 在自定义项目复现 |
| C-20 | 官方示例第二轮质量达到 7 并进入 `END` | 已验证 | 实际输出依次为 `round=1, quality=5 -> translate` 和 `round=2, quality=7 -> END` | 高 | 无需变更 |

## 纵向项目证据

| ID | 结论 | 标签 | 精确证据 | 置信度 | 后续验证 |
|---|---|---|---|---|---|
| C-21 | 内容质量门禁 Graph 的正常路径覆盖通过、补救循环和人工复核 | 已验证 | `examples/compose-quality-gate/gate_test.go`：`TestQualityGateApprovesAfterRemediation`、`TestQualityGateRoutesToManualReview`；示例运行输出 | 高 | 无需变更 |
| C-22 | Local State 在同一 Runnable 的并发调用间隔离 | 已验证 | `TestQualityGateLocalStateIsIsolatedAcrossConcurrentRuns`；`go test -race ./examples/compose-quality-gate` | 高 | 无需变更 |
| C-23 | 业务错误、超时、依赖不可用和超步数错误保留根因并带有可观测分类 | 已验证 | `gate_test.go` 故障测试、`failure-matrix.md`、`Observer.ErrorKind` | 高 | 真实依赖重试策略留待后续 |
| C-24 | 自定义 `GraphCompileCallback` 可生成稳定、脱离实例引用的嵌套拓扑快照 | 已验证 | `snapshot.go`、`snapshot_test.go`：稳定性、嵌套 Graph、并发 Compile 测试 | 高 | 无需变更 |
| C-25 | 只替换 `Inspector` 实现不会改变 Graph 拓扑或 Compose 状态边界 | 已验证 | `TestQualityGateInspectorMigrationKeepsGraphTopology`、`source-map.md` 单变量迁移记录 | 高 | 向真实 ChatModel 替换时补充在线冒烟 |

## 已识别的证据校正

官方 state 示例注释称闭包变量无法在 Branch 中访问，这不是 Go 语言事实。该示例仍能证明 Local State 的公开使用方式，但本协议只把“每次运行生成、互斥访问、嵌套作用域”作为已验证结论。与业务持久化、checkpoint 或跨进程状态有关的结论必须另行验证。
