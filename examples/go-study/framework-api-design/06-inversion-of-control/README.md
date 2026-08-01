# 06. 控制反转 IoC

前置知识见 [04. 回调函数](../04-callback-function/README.md)和 [05. Functional Options](../05-functional-options/README.md)。

## 学习目标

本示例解释下面这类代码，并模拟它返回节点 Option 的行为：

```go
compose.WithStatePostHandler(saveQuestion)
```

需要理解两个问题：

1. 为什么这里只写 `saveQuestion`，没有写 `saveQuestion(...)`。
2. `saveQuestion` 的 `state` 参数最终从哪里传入。
3. 同一次 Graph 运行中的两个节点如何共享一个 Local State。
4. 两次 Graph 运行的 Local State 为什么不会互相污染。

## 第一步：定义普通函数

```go
func saveQuestion(
    _ context.Context,
    question string,
    state *queryState,
) (string, error) {
    state.question = question
    return question, nil
}
```

这只是普通函数。它需要三个参数，本身不会自动获得 `state`。

## 第二步：生成并应用节点 Option

```go
configuredNode := newNode(
	"validate_question",
	validateQuestion,
    withStatePostHandler(saveQuestion),
)
```

`withStatePostHandler(saveQuestion)` 接收 Handler，但不执行 Handler。它返回一个 `nodeOption`：

```go
func withStatePostHandler(handler statePostHandler) nodeOption {
    return func(config *nodeConfig) {
        config.postHandler = handler
    }
}
```

随后 `newNode` 执行这个 Option，把 Handler 保存进节点配置：

```go
func newNode(name string, run nodeRun, options ...nodeOption) *node {
	configuredNode := &node{name: name, run: run}
    for _, option := range options {
        option(&configuredNode.config)
    }
    return configuredNode
}
```

这里发生的是“配置节点”，仍然没有调用 `saveQuestion`。

对比立即调用：

```go
output, err := saveQuestion(ctx, question, state)
```

有括号才表示现在执行函数。

## 第三步：框架创建并传入状态

节点在构建阶段注册到 Graph，而不是在每次运行时作为参数传入：

```go
graph := newSequentialGraph(validateNode, answerNode)
output, state, err := graph.Run(ctx, input)
```

`Run` 每次只创建一个状态对象，然后依次执行已经注册的节点：

```go
state := &queryState{}
for _, current := range g.nodes {
    output, err = current.run(ctx, output)
    output, err = current.config.postHandler(ctx, output, state)
}
```

最后一行是关键：

- 第一个节点的 `postHandler` 实际就是之前登记的 `saveQuestion`。
- `question` 是节点输出。
- `state` 是框架为本次运行创建的状态。
- 因此调用 `validateNode.config.postHandler(...)` 等同于调用 `saveQuestion(...)`。

## 第四步：后续节点读取同一个状态

第二个节点登记了另一个 PostHandler：

```go
func answerFromState(
    _ context.Context,
    _ string,
    state *queryState,
) (string, error) {
    return "回答基于：" + state.question, nil
}
```

Graph 循环把同一个 `state` 指针继续传给节点 2：

```go
output, err = current.config.postHandler(ctx, output, state)
```

完整链路如下：

```text
一次 graph.Run
  -> 创建 Local State A
  -> 节点 1 的 saveQuestion 写入 A.question
  -> 节点 2 的 answerFromState 读取 A.question
  -> 运行结束

下一次 graph.Run
  -> 创建新的 Local State B
  -> 节点 1、节点 2 共享 B，不会读取 A
```

## 与 Eino 对应

| 本示例 | Eino Compose |
|---|---|
| `nodeOption` | `compose.GraphAddNodeOpt` |
| `withStatePostHandler(saveQuestion)` 返回 `nodeOption` | `compose.WithStatePostHandler(saveQuestion)` 返回 `GraphAddNodeOpt` |
| `newNode(...)` 应用并保存 Option | `graph.AddLambdaNode(...)` 应用并保存节点配置 |
| `newSequentialGraph(...)` | Graph 构建及编译阶段保存拓扑 |
| `graph.Run(ctx, input)` | 编译后 Runnable 的一次运行 |
| `state := &queryState{}` | `WithGenLocalState` 为一次运行创建状态 |
| 两次 `postHandler(..., state)` | Eino 向同次运行的多个 Handler 注入同一个 Local State |

Eino 内部还会将状态放入运行 `context`、按类型读取并使用互斥锁保护。本示例省略这些机制，只保留调用关系。

## 运行

在仓库根目录执行：

```bash
go run ./examples/go-study/framework-api-design/06-inversion-of-control
```

预期输出：

```text
节点 1 保存的状态："已校验：state 从哪里来？"
节点 2 读取状态后的输出："回答基于：已校验：state 从哪里来？"
```

## 验证

```bash
go test ./examples/go-study/framework-api-design/06-inversion-of-control -count=1
go test -race ./examples/go-study/framework-api-design/06-inversion-of-control -count=1
```

测试分别验证两条边界：同一次运行的两个节点共享状态，而且每次运行使用独立状态。

## 阅读顺序

1. 阅读 `saveQuestion`，确认它只是普通函数。
2. 阅读 `withStatePostHandler`，确认它返回的是修改节点配置的 Option。
3. 阅读 `newNode`，确认 Option 把 `saveQuestion` 保存到节点配置。
4. 阅读 `sequentialGraph.Run`，确认节点来自已构建的 Graph，而不是运行参数。
5. 确认同一个 `state` 依次传给节点 1 和节点 2。
6. 阅读两个测试，区分“同次运行共享”和“不同运行隔离”。
