# 10. Go 框架源码设计

前置知识见 [06. 控制反转 IoC](../06-inversion-of-control/README.md)、[07. 生命周期钩子](../07-lifecycle-hooks/README.md)和 [09. Go SDK API 设计](../09-sdk-api-design/README.md)。

## 学习目标

本课不再实现一个迷你框架，而是沿 Eino `v0.9.12` 的真实运行链路追踪：

```go
compose.WithStatePostHandler(handler)
```

完成后应能回答：

1. 用户传入的 Handler 在哪里被包装、保存和校验。
2. Local State 在什么时候创建，为什么不同 `Invoke` 不共享状态。
3. 节点成功、节点失败和 PostHandler 失败时分别如何执行与返回错误。

本示例直接依赖仓库锁定的 Eino `v0.9.12`，不复制 Eino 内部实现。

## 先看结论

`WithStatePostHandler` 本身不执行用户 Handler。它返回 `GraphAddNodeOpt`，随后由 AddNode 应用配置；Compile 把 PostHandler 放进节点运行任务；每次 `Runnable.Invoke` 创建 Local State；节点成功后，任务管理器才调用 PostHandler。

```text
构建期
WithStatePostHandler
  -> GraphAddNodeOpt
  -> AddLambdaNode / getGraphAddNodeOpts
  -> convertPostHandler
  -> addNode 校验状态类型与节点输出类型

编译期
Graph.Compile
  -> nodeInfo.postProcessor
  -> chanCall.postProcessor
  -> runner.runCtx

运行期
Runnable.Invoke
  -> runner.runCtx 创建本次 internalState
  -> 节点 action
  -> 节点失败：跳过 PostHandler
  -> 节点成功：runPostHandler
       -> getState
       -> mutex.Lock
       -> 用户 Handler
       -> Handler 返回值替换节点输出
  -> 下游节点或错误返回入口
```

## 五段源码链路

### 1. 注册配置

[`graph_add_node_options.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/graph_add_node_options.go#L101-L112) 中，`WithStatePostHandler` 返回一个闭包。闭包被应用时会：

- 使用 `convertPostHandler` 把强类型 Handler 适配成统一的 `composableRunnable`。
- 保存状态类型 `S`，供 AddNode 校验。
- 把 `needState` 标记为 `true`。

因此“注册 Handler”和“调用 Handler”是两个不同时间点。

### 2. 构建期校验与保存

[`graph.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/graph.go#L162-L229) 的 `addNode` 在写入 `g.nodes` 前检查：

- Graph 是否通过 `WithGenLocalState` 启用了状态。
- PostHandler 的状态类型是否等于 Graph 状态类型。
- PostHandler 的输入类型是否等于节点输出类型。

[`graph_node.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/graph_node.go#L165-L178) 把处理器保存在 `nodeInfo.postProcessor`。这些错误因此会在 AddNode 阶段暴露，而不是等到线上请求运行时才出现。

### 3. 编译为运行任务

[`graph.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/graph.go#L729-L756) 的 Compile 链路把 `nodeInfo.postProcessor` 复制到运行期的 `chanCall.postProcessor`。此时 Handler 仍未执行。

同一文件的 [Local State 编译逻辑](https://github.com/cloudwego/eino/blob/v0.9.12/compose/graph.go#L842-L854) 把状态生成器转换成 `runner.runCtx`。

### 4. 每次运行创建状态

[`graph_run.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/graph_run.go#L198-L205) 在一次新运行开始时调用 `runner.runCtx`。`runCtx` 调用用户注册的状态生成器，并把新的 `internalState` 放入本次运行的 context。

所以正确的生命周期是：

```text
Compile 一次 -> 复用同一 Runnable -> 每次 Invoke 创建一个新状态
```

不要在状态生成器外预先创建 `state := &State{}` 再反复返回它，否则仍会人为制造跨请求共享。

### 5. 成功后调用与错误传播

[`graph_manager.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/graph_manager.go#L381-L412) 的 `waitOne` 先检查节点错误：节点失败会直接跳过 PostHandler；节点成功才进入 `runPostHandler`。

[`graph_manager.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/graph_manager.go#L543-L555) 使用 PostHandler 的返回值替换节点输出。若 Handler 返回错误，错误会带上 `post processor` 上下文；Graph 再附加节点路径。Eino 的内部错误实现支持 `Unwrap`，因此入口仍可使用 `errors.Is` 判断根因。

[`state.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/state.go#L52-L65) 的 `convertPostHandler` 在调用用户 Handler 前按状态类型查找 Local State，并持有该状态的互斥锁直到 Handler 返回。

## 实验如何对应源码

本目录构建两个节点：

```text
START -> generate + WithStatePostHandler -> build_result -> END
```

`generate` 生成回答；PostHandler 把原始输出写入状态，并给节点输出追加 ` [已记录]`；`build_result` 通过 `ProcessState` 读取状态快照。

| 实验断言 | 对应源码结论 |
|---|---|
| 构建后状态创建数和回调数都是 0 | Option 与 Compile 不执行用户 Handler |
| 每次 Invoke 的 `StateID` 不同 | `runner.runCtx` 逐次创建 Local State |
| 下游收到追加了 ` [已记录]` 的输出 | PostHandler 返回值替换节点输出 |
| 节点返回错误时回调数仍为 0 | `waitOne` 跳过 PostHandler |
| PostHandler 错误可被 `errors.Is` 识别 | 错误包装保留根因 |
| 缺少状态或类型不匹配时 AddNode 失败 | 构建期校验先于运行期 |

## 运行与验证

仓库声明 Go `1.26.0`，当前验证运行时为 Go `1.26.3`；Eino 锁定为 `v0.9.12`。示例不访问网络，也不需要凭据。

在仓库根目录执行：

```bash
go run ./examples/go-study/framework-api-design/10-framework-source-design
go test ./examples/go-study/framework-api-design/10-framework-source-design -count=1
go test -race ./examples/go-study/framework-api-design/10-framework-source-design -count=1
```

预期输出：

```text
构建后：state=0 post=0
运行：state=1 output="回答：配置保存在哪里？ [已记录]" events=["post:回答：配置保存在哪里？"]
运行：state=2 output="回答：Handler 在哪里调用？ [已记录]" events=["post:回答：Handler 在哪里调用？"]
```

测试覆盖配置延迟执行、输出改写、并发运行状态隔离、节点错误、PostHandler 错误、缺少 Local State、类型不匹配和 nil 依赖。

## 已验证结论与边界

- `已验证`：上述注册、编译、运行和错误链路已按 Eino `v0.9.12` 源码及本目录测试核对。
- `已验证`：普通 `StatePostHandler` 在节点成功后同步执行，并在调用期间独占对应 Local State 的互斥锁。
- `已验证`：节点 action 失败时不调用 PostHandler；PostHandler 不能作为统一的成功/失败完成通知。
- `官方说明`：如果节点输出是真实流，普通 `StatePostHandler` 会读取并合并整个流；需要保留真实流时应使用 `WithStreamStatePostHandler`。
- `已知限制`：Local State 只解决单次运行内共享与隔离，不提供持久化、跨进程恢复或业务事务语义。

## 阅读练习

按下面顺序在 Eino `v0.9.12` 源码中逐个定位符号，每读完一个只回答“谁保存、谁调用、何时发生”：

1. `WithStatePostHandler`
2. `getGraphAddNodeOpts`
3. `graph.addNode`
4. `graph.compile`
5. `runner.run`
6. `taskManager.waitOne`
7. `runPostHandler`
8. `convertPostHandler`

最后修改本示例的 PostHandler，让它返回新的输出但不写状态；先预测 `build_result` 会看到什么，再运行测试验证。
