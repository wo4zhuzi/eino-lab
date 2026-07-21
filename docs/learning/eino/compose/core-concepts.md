# Eino Compose 核心概念：Lambda、Edge、Branch 与 Local State

本文基于当前项目锁定的 Eino `v0.9.12`，使用
[`examples/compose-quality-gate`](../../../../examples/compose-quality-gate/) 解释 Compose Graph
的构建和运行机制。重点不是记住 API，而是区分以下几个动作：

```text
包装业务函数 -> 注册节点 -> 添加连接规则 -> 编译 Graph -> 执行 Runnable
```

## 先建立整体模型

Compose 把“节点做什么”“下一步去哪里”“一次运行需要共享什么状态”拆成三类对象：

| 概念 | 回答的问题 | 本项目对应内容 |
|---|---|---|
| Lambda 节点 | 当前步骤执行什么逻辑 | `validate`、`inspect`、`remediate`、`approve`、`manual` |
| Edge / Branch | 当前步骤完成后去哪里 | 固定后继、条件后继、补救循环 |
| Local State | 同一次运行的多个步骤共享什么 | 尝试次数 `Attempts`、审计记录 `Audit` |

完整生命周期分为两个阶段：

```text
构建期
业务函数 -> Lambda -> AddLambdaNode
路由函数 -> GraphBranch -> AddBranch
节点之间 -> AddEdge
                 |
                 v
              Compile
                 |
                 v
运行期
Review -> Runnable.Invoke -> 生成本次 Local State -> 按拓扑调度节点 -> 返回结果
```

构建 API 的调用顺序不是节点的运行顺序。运行顺序由最终注册到 Graph 的 Edge 和 Branch
共同决定，并在 `Compile` 后交给运行引擎调度。

## Lambda：把业务函数变成可编排节点

### 业务函数本身

[`nodes.go`](../../../../examples/compose-quality-gate/nodes.go) 中的 `inspect` 只是普通 Go 方法。
下面只保留与 Lambda 输入输出有关的主干，状态记录逻辑见后文：

```go
func (n *gateNodes) inspect(
	ctx context.Context,
	payload gatePayload,
) (gatePayload, error) {
	result, err := n.inspector.Inspect(ctx, payload.Content)
	if err != nil {
		return gatePayload{}, fmt.Errorf("inspect content: %w", err)
	}

	payload.Score = result.Score
	payload.Reason = result.Reason
	return payload, nil
}
```

它描述业务计算，但此时 Compose 还不知道它的存在。

### `InvokableLambda` 负责包装

[`topology.go`](../../../../examples/compose-quality-gate/topology.go) 先把方法包装成 Lambda：

```go
inspect := compose.InvokableLambda(nodes.inspect)
```

`InvokableLambda` 的职责是把符合 `func(context.Context, I) (O, error)` 形式的函数转换为
Compose 能统一处理的 `Lambda`。包装后，Compose 才能识别输入输出类型、接入 Runnable
调用范式、Callback 和错误路径。

这一步只创建 Lambda 对象，**没有把它放入 Graph**。

### `AddLambdaNode` 负责注册

```go
if err := graph.AddLambdaNode(
	nodeInspect,
	inspect,
	compose.WithNodeName(nodeInspect),
); err != nil {
	return fmt.Errorf("add %s node: %w", nodeInspect, err)
}
```

`AddLambdaNode` 才会建立节点键和节点实现的对应关系：

```text
"inspect" -> inspect Lambda
```

Eino `v0.9.12` 最终把节点保存到 `g.nodes[key]`。因此 Edge 或 Branch 返回的节点键
只有在节点已注册时才有意义。

### 为什么分成包装和注册

分成两步有三个直接收益：

1. 同一个 Graph 可以注册 Lambda、ChatModel、Tool、Retriever、子 Graph 等不同种类的节点。
2. Lambda 负责统一“如何执行”，Graph 负责管理“节点叫什么、如何连接”，职责不会混在一起。
3. Graph 可以在构建期检查节点键重复、类型兼容和拓扑合法性，而不必等到请求运行时才失败。

## Edge：声明固定后继

`AddEdge` 表示：上游节点成功完成后，固定调度某个下游节点。

```go
edges := [][2]string{
	{compose.START, nodeValidate},
	{nodeValidate, nodeInspect},
	{nodeRemediate, nodeInspect},
	{nodeApprove, compose.END},
	{nodeManual, compose.END},
}

for _, edge := range edges {
	if err := graph.AddEdge(edge[0], edge[1]); err != nil {
		return fmt.Errorf(
			"add edge %s -> %s: %w",
			edge[0],
			edge[1],
			err,
		)
	}
}
```

这些调用形成固定路径：

```text
START -> validate -> inspect
remediate -> inspect
approve -> END
manual -> END
```

`AddEdge` 不执行节点，也不负责注册节点。它只向 Graph 添加固定的控制流和数据流关系。
起点和终点除 `START`、`END` 外，都必须先通过 `Add...Node` 注册。

`remediate -> inspect` 是循环真正成立的原因。`Attempts` 只是循环过程中使用的状态；如果删除
这条 Edge，即使 `Attempts` 仍然递增，流程也无法返回 `inspect`。

### 为什么 Edge 必须显式声明

如果节点函数内部直接调用下一个节点，框架无法完整看到拓扑。显式 Edge 使 Compose 可以：

- 在构建和编译阶段检查端点、类型和拓扑。
- 统一调度节点，而不是让业务函数控制调度器。
- 记录节点路径、传播 Callback，并生成拓扑快照。
- 替换节点实现而不改变控制流。

## Branch：声明条件后继

### 路由函数仍然是普通业务函数

```go
func (r gateRouter) route(
	ctx context.Context,
	payload gatePayload,
) (string, error) {
	if payload.Score >= r.config.ApprovalThreshold {
		return nodeApprove, nil
	}

	var attempts int
	if err := compose.ProcessState[*gateState](ctx, func(
		_ context.Context,
		state *gateState,
	) error {
		attempts = state.Attempts
		return nil
	}); err != nil {
		return "", fmt.Errorf("read routing state: %w", err)
	}

	if attempts >= r.config.MaxAttempts {
		return nodeManual, nil
	}
	return nodeRemediate, nil
}
```

返回值是目标节点键，不是下一个节点的业务输入。下一个节点收到的业务输入仍然是
`inspect` 输出的 `gatePayload`。

### `NewGraphBranch` 负责包装路由规则

```go
branch := compose.NewGraphBranch(
	router.route,
	map[string]bool{
		nodeApprove:   true,
		nodeManual:    true,
		nodeRemediate: true,
	},
)
```

它把路由函数和允许返回的目标集合包装成 `GraphBranch`。候选集合不是重复配置，而是 Branch
的控制流契约。Eino 会拒绝未声明的返回目标，并检查候选节点是否已经注册。

### `AddBranch` 负责挂载

```go
if err := graph.AddBranch(nodeInspect, branch); err != nil {
	return fmt.Errorf("add review branch: %w", err)
}
```

这行的含义是：

```text
inspect 成功完成后 -> 执行 branch -> 从候选节点中选择后继
```

Branch 不是节点，不存在于 `g.nodes`；它是挂在某个节点之后的后继规则，保存在
`g.branches[startNode]`。

### Edge 与 Branch 如何配合

本项目的运行拓扑是：

```text
START --Edge--> validate --Edge--> inspect
                                      |
                                   Branch
                         +------------+------------+
                         |            |            |
                      approve       manual      remediate
                         |            |            |
                       Edge         Edge          Edge
                         |            |            |
                        END          END         inspect
```

不存在“从静态路径切换到动态路径”的特殊指令。运行引擎在节点完成后查看该节点注册了哪些后继：

- 普通 Edge 直接产生固定后继。
- Branch 先执行条件函数，再产生选中的后继。

如果同一个节点同时配置普通 Edge 和 Branch，两类后继都会参与调度；Branch 不会覆盖或取消
普通 Edge。生产代码通常让一个决策点只使用 Branch，避免意外并行执行固定后继。

## Local State：一次运行内共享、运行之间隔离

本项目有两类运行数据：

```go
type gatePayload struct {
	Content string
	Score   int
	Reason  string
}

type gateState struct {
	Attempts int
	Audit    []AuditEntry
}
```

它们的差异不是“哪个能保存数据”，而是传递方式和生命周期不同：

| 对象 | 传递方式 | 生命周期 | 用途 |
|---|---|---|---|
| `gatePayload` | 上游节点返回，下游节点参数接收 | 一次运行中的当前数据路径 | 当前正文、当前评分、当前原因 |
| `gateState` | 从运行 `ctx` 中通过 `ProcessState` 访问 | 一次 `Runnable.Invoke` | 累计尝试次数、完整审计记录 |
| `gateNodes` | 构图时创建并被节点方法引用 | 编译产物生命周期 | 保存可复用依赖，不保存请求状态 |

### `WithGenLocalState` 注册每次运行的状态生成器

```go
graph := compose.NewGraph[ReviewRequest, ReviewResult](
	compose.WithGenLocalState(func(context.Context) *gateState {
		return &gateState{}
	}),
)
```

这里不是立即为每个节点创建状态，而是告诉 Graph：每开始一次新的 Runnable 运行，就生成一个
新的 `*gateState`。

```text
Review A -> stateA
  inspect 第一次 -> stateA.Attempts = 1
  inspect 第二次 -> stateA.Attempts = 2

Review B -> stateB
  inspect 第一次 -> stateB.Attempts = 1
```

同一次 Review 中，循环再次进入 `inspect` 时读取的是同一个 `stateA`；不同 Review 之间使用
不同状态，因此可以并发执行而不串请求数据。

### `ProcessState` 负责受控访问

```go
if err := compose.ProcessState[*gateState](ctx, func(
	_ context.Context,
	state *gateState,
) error {
	state.Attempts++
	state.Audit = append(state.Audit, AuditEntry{
		Attempt: state.Attempts,
		Score:   result.Score,
		Reason:  result.Reason,
	})
	return nil
}); err != nil {
	return gatePayload{}, fmt.Errorf("record inspection state: %w", err)
}
```

Eino `v0.9.12` 从当前运行的 `ctx` 找到匹配类型的状态，并在调用处理函数时使用互斥锁。
应用不需要把状态放入全局变量或共享节点字段。

### 为什么状态不放在 `gateNodes`

编译后的 Runnable 会被多个请求复用，`gateNodes` 也会随之被复用。如果把 `Attempts` 放入
`gateNodes`，并发请求会修改同一个字段，既会串数据，也可能产生数据竞争。

Local State 把可复用对象和请求级状态分开：

```text
共享：编译后的 Runnable、节点实现、外部依赖客户端
隔离：每次运行的 payload、Local State、结果和错误
```

### Local State 不是什么

Local State 不是数据库，也不是 checkpoint：

- 调用结束后不承诺继续保留。
- 进程重启后不会恢复。
- 需要持久审计或断点恢复时，必须使用外部存储或单独学习 checkpoint。

## “包装、注册、添加、执行”速查

| 代码 | 阶段 | 精确职责 |
|---|---|---|
| `compose.InvokableLambda(fn)` | 构建期 | 包装节点业务函数，创建 Lambda |
| `graph.AddLambdaNode(key, lambda)` | 构建期 | 把 Lambda 以节点键注册到 Graph |
| `compose.NewGraphBranch(route, targets)` | 构建期 | 包装路由函数和允许目标，创建 Branch |
| `graph.AddBranch(start, branch)` | 构建期 | 把 Branch 挂到起始节点之后 |
| `graph.AddEdge(from, to)` | 构建期 | 添加固定后继关系 |
| `compose.WithGenLocalState(gen)` | 构建期配置 | 注册每次运行的状态生成器 |
| `graph.Compile(ctx, opts...)` | 编译期 | 校验并生成可复用 Runnable |
| `runnable.Invoke(ctx, input)` | 运行期 | 开始一次独立运行 |
| `compose.ProcessState(ctx, handler)` | 运行期 | 访问本次运行的 Local State |

## 为什么 Compose 采用这种设计

### 1. 计算、控制流和状态分离

Lambda 只关心当前步骤的输入输出，Edge/Branch 只关心下一步，Local State 只关心运行内共享。
这让业务逻辑、拓扑和状态生命周期可以分别测试和替换。

### 2. 构建一次，运行多次

生产服务通常在启动时注册节点、添加边并 Compile 一次，然后并发调用同一个 Runnable。
拓扑构建成本和校验成本不会在每个请求中重复发生。

### 3. 尽早发现错误

显式节点键、候选目标和输入输出类型让 Compose 可以在 Add/Compile 阶段发现未注册节点、
重复节点、非法目标和类型不匹配，而不是在流量进入后才暴露。

### 4. 框架拥有完整拓扑

因为节点不会私自调用下一节点，Compose 可以统一实现调度、循环步数保护、Callback、节点路径
错误、流式适配和拓扑观测。

### 5. 请求状态天然隔离

Local State 每次运行生成，避免把请求状态放到长生命周期节点对象中。这是同一 Runnable 能够
被多个请求安全复用的基础之一。

## 生产项目如何组织

生产级 Eino 项目仍然直接使用 `Add...Node`、`AddEdge`、`AddBranch` 和 `Compile`。Eino 自己的
ReAct Agent 也采用这种构图方式。区别主要在代码组织，而不是换一套 API：

```text
NewService / NewAgent
├── 创建 Graph 和依赖
├── registerNodes：只注册节点
├── addTopology：只声明 Edge 和 Branch
├── Compile：启动期失败则拒绝启动
└── 保存 Runnable：请求期重复 Invoke
```

当前项目已经按 `addGateNodes` 和 `addGateTopology` 分离。节点数量继续增长时，应按业务子流程
拆分构建函数或使用子 Graph；不要为了少写几行 `AddEdge` 过早发明自定义 DSL。以线性流程为主
时可以选 Chain；需要字段汇聚的无环流程可以选 Workflow；本项目需要显式回边，因此选择 Graph。

## 常见误区

1. Lambda 不负责注册自己；`AddLambdaNode` 才负责注册。
2. Branch 不是节点；`AddBranch` 把后继规则挂到节点之后。
3. Branch 返回节点键，不返回下一节点的业务数据。
4. 注册顺序不是运行顺序；Edge 和 Branch 定义运行拓扑。
5. `Attempts` 不会制造循环；`remediate -> inspect` Edge 才制造循环。
6. 同一节点上的 Branch 不会覆盖普通 Edge。
7. Local State 是每次运行创建一个，不是每个节点创建一个。
8. 同一次运行中的多个节点共享 Local State，不同运行之间隔离。
9. `gateNodes` 会被多个运行复用，不应保存请求级可变状态。

## 源码证据

- [`compose/graph.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/graph.go)：节点、Edge、Branch 的注册和校验。
- [`compose/branch.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/branch.go)：Branch 包装和目标约束。
- [`compose/generic_graph.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/generic_graph.go)：Graph 创建、Local State 生成器和 Compile 入口。
- [`compose/state.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/state.go)：`ProcessState` 的状态查找和互斥访问。
- [`flow/agent/react/react.go`](https://github.com/cloudwego/eino/blob/v0.9.12/flow/agent/react/react.go)：Eino 自身使用 Node、Edge 和 Branch 构建 ReAct Agent。
