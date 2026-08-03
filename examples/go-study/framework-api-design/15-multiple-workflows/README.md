# 15. 多工作流与公共运行层

前置示例见 [14. 生产型 Chain 与 Graph 组合](../14-chain-with-graph/README.md)。

## 结论

同一个应用存在多个代码定义工作流时，推荐只抽取稳定的运行生命周期，不抽取业务拓扑 DSL：

- `workflowkit` 统一完成 `Compile once / Run many`、工作流命名、context 校验、错误包装和 typed nil 依赖校验。
- `reviewworkflow` 独立拥有审核配置、Inspector、输入输出和审核 Graph。
- `ragworkflow` 独立拥有检索配置、Retriever/Generator、输入输出和检索回环 Graph。
- `main` 在应用启动时分别构建两个 Workflow，请求阶段调用各自的 `Run`。

这样既减少真实重复，又保留 Eino 泛型类型检查和每个业务的独立演进能力。

## 学习目标

1. 在一个进程中启动并复用两个不同工作流。
2. 使用泛型 `workflowkit.Runner[I, O]` 统一编译与运行生命周期。
3. 理解哪些能力适合公共抽取，哪些业务概念必须留在各自模块。
4. 新增第三个工作流时复用公共运行层，而不是复制 Compile/Run 错误处理。

仓库声明 Go `1.26.0`，Eino 锁定为 `v0.9.12`。示例只使用本地确定性依赖，不访问网络，也不需要凭据。

## 目录结构

```text
15-multiple-workflows/
├── main.go                         应用装配入口
├── README.md                       设计与运行说明
├── workflowkit/
│   ├── runtime.go                  通用 Compilable、Compile 和 Runner
│   └── runtime_test.go             编译一次、运行多次和边界测试
├── reviewworkflow/
│   ├── workflow.go                 审核 Config、Dependencies、New/Run
│   ├── build.go                    审核 Chain 和决策 Graph
│   ├── handlers.go                 审核节点及本地 Inspector
│   ├── types.go                    审核输入输出和内部类型
│   └── workflow_test.go            通过、人工和异常测试
└── ragworkflow/
    ├── workflow.go                 RAG Config、Dependencies、New/Run
    ├── build.go                    RAG Chain 和检索 Graph
    ├── handlers.go                 RAG 节点及本地依赖
    ├── types.go                    RAG 输入输出和内部类型
    └── workflow_test.go            命中、改写、无证据和异常测试
```

## 公共层做什么

`workflowkit.Compilable[I, O]` 是 `compose.Chain[I, O]` 和 `compose.Graph[I, O]` 共有的最小能力：

```text
Compile(ctx, options...) -> Runnable[I, O]
```

`workflowkit.Compile` 在启动阶段执行：

```text
校验 context、名称和定义
-> 设置稳定 GraphName
-> 调用 Chain/Graph.Compile
-> 保存 Runnable 到 Runner
```

`Runner.Run` 在请求阶段执行：

```text
校验 context 和 Runner
-> Runnable.Invoke(ctx, input)
-> 统一添加工作流名称错误上下文
```

同一个 Runner 可以并发执行多个 `Run`；注入的 Inspector、Retriever、Generator 等具体依赖也必须支持并发调用，或在适配器内部完成同步保护。

公共层不知道审核分数、检索次数、文档或答案是什么。

## 两个工作流

### 内容审核

外层 Chain：

```text
normalize_review -> review_decision_graph -> format_review_result
```

内层 Graph：

```text
inspect_review
  -> approve_review -> END
  -> manual_review  -> END
```

审核工作流只依赖 `Inspector`，根据 `ApprovalScore` 选择通过或人工审核。

### 本地 RAG

外层 Chain：

```text
normalize_question
-> retrieval_graph
-> generate_answer
-> format_rag_result
```

内层 Graph：

```text
retrieve
  -> evidence_ready -> END
  -> rewrite_query -> retrieve
  -> no_evidence -> END
```

RAG 工作流依赖 `Retriever` 和 `Generator`。首次检索无结果时改写查询并回环；达到 `MaxRetrievalAttempts` 后正常返回无证据结果，`MaxGraphSteps` 只负责技术兜底。

## 抽取边界

| 内容 | 是否公共抽取 | 原因 |
|---|---|---|
| Compile once / Run many | 是 | 所有代码定义工作流生命周期一致 |
| Graph 名称和错误上下文 | 是 | 运行治理需要稳定标识 |
| typed nil 依赖校验 | 是 | Go 接口注入的公共边界问题 |
| Request/Result | 否 | 每个业务契约不同 |
| Config | 否 | 审核阈值和检索次数没有共同语义 |
| Dependencies | 否 | Inspector 与 Retriever/Generator 不是同一能力 |
| 节点、Edge、Branch | 否 | 拓扑必须保持显式和类型安全 |
| Local State | 否 | 应按每个工作流的旁路状态单独设计 |

不要把两个 Config 合并成超大 `WorkflowConfig`，也不要用 `map[string]any` 或 JSON 节点表替代当前泛型拓扑。

## 如何增加第三个工作流

1. 新建独立业务目录，例如 `orderworkflow/`。
2. 定义自己的 `Request`、`Result`、`Config` 和 `Dependencies`。
3. 使用 Eino Chain/Graph 显式构建业务拓扑。
4. 在 `New` 中调用 `workflowkit.Compile`。
5. 用业务 `Workflow.Run` 包装通用 Runner。
6. 在应用启动入口创建一次，并注入对应 HTTP/RPC Handler。

不需要修改 `workflowkit`，除非第三个工作流再次证明存在新的、与业务无关的真实重复。

## 运行与验证

在仓库根目录执行：

```bash
go run ./examples/go-study/framework-api-design/15-multiple-workflows
go test ./examples/go-study/framework-api-design/15-multiple-workflows/... -count=1
go test -race ./examples/go-study/framework-api-design/15-multiple-workflows/... -count=1
```

预期输出：

```text
review route=approved score=9 steps=[normalize_review inspect_review approve_review format_review_result]
rag no_evidence=false attempts=2 evidence=2 steps=[normalize_question retrieve rewrite_query retrieve evidence_ready generate_answer format_rag_result]
```

测试覆盖公共 Runner 只编译一次并运行多次、审核通过/人工路径、RAG 直接命中/改写命中/无证据退出、配置和依赖校验、外部依赖错误及 context 错误传播。

## 已知限制

- 两个本地依赖只用于验证编排，不代表真实模型、向量数据库或远程服务。
- 当前没有 checkpoint、Tracing、指标和副作用节点幂等控制。
- 工作流由 Go 代码定义，修改拓扑后需要重新构建和发布。
