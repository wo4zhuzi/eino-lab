# Compose 内容质量门禁源码导航

本文按一次真实运行的调用链阅读 Eino `v0.9.12` 源码，不按目录顺序罗列 API。

## 入口与应用代码

| 文件 | 符号/位置 | 作用 |
|---|---|---|
| `examples/compose-quality-gate/customer_service.go` | `CustomerReplyGenerator`、`chatModelCustomerReplyGenerator.Generate` | 定义业务接口、共享消息转换，并提供 Graph 外模型调用基线 |
| `examples/compose-quality-gate/customer_service_graph.go` | `NewChatModelGraphCustomerReplyGenerator` | 使用两个 Lambda 和 `AddChatModelNode` 构建回复生成 Graph |
| `examples/compose-quality-gate/main.go` | `main` | 接收模拟用户问题，生成草稿并调用质量门禁 |
| `examples/compose-quality-gate/delivery.go` | `CustomerReplyDelivery`、`simulatedCustomerReplyDelivery.Deliver` | 将 approved 结果模拟发送给用户，将 manual_review 结果模拟放入人工队列 |
| `examples/compose-quality-gate/gate.go` | `NewQualityGate` | 创建类型化 Graph、注册 Local State、编译 Runnable |
| `examples/compose-quality-gate/topology.go` | `addGateNodes` | 将五个命名方法包装为 Lambda 并注册到 Graph |
| `examples/compose-quality-gate/topology.go` | `addGateTopology`、`gateRouter.route` | 声明 Branch、循环边、approve/manual 到 END 的边 |
| `examples/compose-quality-gate/nodes.go` | `gateNodes` 的五个节点方法 | 隔离 validate、inspect、remediate、approve、manual 的业务逻辑 |
| `examples/compose-quality-gate/snapshot.go` | `TopologySnapshotter.OnFinish` | 将 `GraphInfo` 复制为可比较快照 |
| `examples/compose-quality-gate/observer.go` | `Observer.Handler` | 注册运行级 Callback，不记录内容正文 |

## Eino 调度链路

```text
Graph[I,O].Compile
  -> compileAnyGraph
  -> graph.compile
  -> GraphCompileCallback.OnFinish(GraphInfo)
  -> toGenericRunnable
  -> Runnable.Invoke
  -> graphRun main execution loop
  -> node runnable / branch condition
  -> wrapGraphNodeError
  -> result or error
```

## 关键源码位置

| 阅读顺序 | 文件与符号 | 从本项目得到的结论 |
|---|---|---|
| 1 | `compose/generic_graph.go:72` `NewGraph` | Graph 的泛型输入输出和 Local State 生成器在构建期确定 |
| 2 | `compose/generic_graph.go:123` `Graph.Compile` | Compile 是从声明拓扑到可运行对象的边界 |
| 3 | `compose/generic_graph.go:127` `compileAnyGraph` | 编译选项、全局/本次 Compile Callback 和 Runnable 包装在此汇合 |
| 4 | `compose/graph.go:433` `AddLambdaNode`、`:466` `AddBranch` | 节点类型和 Branch 约束在构建阶段进入 Graph |
| 5 | `compose/graph.go:350` `AddChatModelNode`、`compose/component_to_graph_node.go:91` `toChatModelNode` | ChatModel 的 `Generate/Stream` 被适配为消息输入输出节点 |
| 6 | `compose/introspect.go:41` `GraphInfo` | 回调可读取节点、控制边、数据边、Branch 和嵌套 Graph 元数据 |
| 7 | `compose/runnable.go:28` `Runnable` | 四种数据流范式使用同一编译产物，缺失范式由适配器补齐 |
| 8 | `compose/graph_run.go:240` 主循环 | 每步先检查取消/最大步数，再调度任务并计算后继 |
| 9 | `compose/state.go:165` `ProcessState` | 状态按类型从当前或父作用域查找，并使用互斥锁保护访问 |
| 10 | `compose/error.go:79` `wrapGraphNodeError` | 节点路径前置到错误，并由 `internalError.Unwrap` 保留根因 |

## 扩展边界

`GraphCompileCallback` 的公开接口只有 `OnFinish(ctx, info)`，没有错误返回值。因此本示例的快照器只做观测和稳定化：排序节点、边和分支目标，复制嵌套元数据并通过锁隔离并发编译。若要阻止不符合策略的 Graph，必须在应用构建器或 Compile 前校验，而不能依赖回调否决。

## 单变量迁移

迁移变量是 `Inspector` 实现：

- 修改前预测：Graph 节点、边、Branch、Local State 和错误包装不变；只有评分结果、是否进入补救和最终尝试次数变化。
- 实际验证：`TestQualityGateInspectorMigrationKeepsGraphTopology` 用 `ruleInspector` 和始终返回 9 分的替代实现分别构建 Graph；拓扑快照完全相同，前者两次尝试通过，后者一次尝试通过。
- 结论：`Inspector` 接口确实隔离了业务检查实现，Compose 编排层没有隐式绑定具体检查算法。

第二次迁移只改变 ChatModel 的执行位置：

- 修改前预测：提示消息、模型实例、回复正文和 QualityGate 拓扑不变；Graph 内路径新增模型前后 Lambda、调用级 Callback 和节点路径。
- 实际验证：`TestChatModelGraphCustomerReplyGeneratorMatchesDirectPath` 证明两条路径的模型输入与业务输出一致；模型错误、超时和空响应测试证明根因保留且节点路径符合预测。
- 结论：`AddChatModelNode` 不改变模型协议，但把组件纳入 Compose 的类型检查、调用选项、Callback 和错误定位边界。
