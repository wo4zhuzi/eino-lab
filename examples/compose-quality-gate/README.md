# Compose 内容质量门禁

## 业务背景

本示例对应一个实际的 AI 客服场景：客服系统先生成回复草稿，再在发送前经过质量门禁。

```text
用户问题 -> 模拟生成器或 ChatModel -> 回复草稿 -> 质量审核 Graph
                                              |-> 通过并发送
                                              |-> 修改后重新审核
                                              `-> 多次失败转人工
```

默认模式仍使用确定性模拟客服、离线审核规则和模拟交付组件，不需要网络或凭据。设置 `CUSTOMER_REPLY_MODE=model` 后，只把模拟客服替换为真实 ChatModel；审核 Graph、Inspector 和交付组件保持不变，以验证组件接口支持单变量迁移。

当前 ChatModel 是审核 Graph 的上游组件：`ChatModel -> CustomerReplyGenerator -> ReviewRequest -> QualityGate`。它不是 Branch，也没有注册为 Graph 节点。这样设计是为了先单独掌握 Eino 模型组件；后续再把模型迁入 Compose 节点，比较直接组件调用与 `AddChatModelNode` 的差异。

## 学习目标

本示例使用 Eino Compose 构建一个离线、可审计的内容质量门禁，重点验证：

- 类型化 `Graph`、Lambda 节点和 `GraphBranch`。
- `remediate -> inspect` 有界循环与 `WithMaxRunSteps` 保护。
- 每次调用独立生成的 Local State。
- 节点路径错误和调用级 Callback。
- 自定义 `GraphCompileCallback` 生成稳定拓扑快照。
- `model.BaseChatModel` 与业务接口 `CustomerReplyGenerator` 之间的适配边界。

## 运行链路

```text
START -> validate -> inspect -> Branch
                              |-> approve -> END
                              |-> remediate -> inspect
                              `-> manual -> END
```

客服回复生成器、`Inspector` 和 `CustomerReplyDelivery` 是外部能力边界。默认实现都是确定性的本地能力；只有显式启用模型模式时，回复生成器才会调用 OpenAI 兼容服务。

## 阅读顺序

如果对“包装、注册、添加和执行”的区别还不熟悉，先阅读
[Compose 核心概念](../../docs/learning/eino/compose/core-concepts.md)。

建议按职责从外到内阅读，不要从单个文件第一行一路读到底：

1. `main.go`：程序入口，只看问题、生成器选择、草稿、Graph 构建和一次 `Review` 调用。
2. `customer_service.go`：业务接口、模拟实现和 ChatModel 适配器。
3. `customer_service_config.go`：根据环境变量显式选择模拟模式或模型模式。
4. `gate.go`：公开类型、Local State、`NewQualityGate`、`Compile` 和 `Review`。
5. `topology.go`：Lambda 节点注册、Branch 目标和 Edge 连接。
6. `nodes.go`：validate、inspect、remediate、approve、manual 的具体业务逻辑。
7. `delivery.go`：根据审核终态模拟发送回复或进入人工队列。
8. `observer.go` 与 `snapshot.go`：运行期和编译期观测扩展。
9. `*_test.go`：正常路径、故障、并发隔离和扩展边界证据。

这样可以先回答“程序如何运行”，再深入“每个节点做什么”。

## 前置条件

- Go `1.26.3`，以根 `go.mod` 的 `go 1.26.0` 为最低项目约束。
- Eino `v0.9.12`。
- EinoExt OpenAI `v0.1.13`。
- 默认模拟模式不需要 API Key 或其他外部服务。

模型模式需要兼容 OpenAI Chat Completions 的服务：

| 环境变量 | 必填 | 用途 |
|---|---|---|
| `CUSTOMER_REPLY_MODE` | 是 | 设置为 `model` 后启用真实模型；默认是 `simulated` |
| `OPENAI_API_KEY` | 模型模式必填 | 模型服务凭据 |
| `OPENAI_MODEL` | 模型模式必填 | 模型名称 |
| `OPENAI_BASE_URL` | 否 | OpenAI 兼容服务地址；不填时使用组件默认地址 |
| `CUSTOMER_REPLY_TIMEOUT` | 否 | 单次模型 HTTP 请求超时，默认 `15s` |

## 运行

在仓库根目录执行：

```bash
go run ./examples/compose-quality-gate
```

使用真实模型生成回复草稿：

```bash
export CUSTOMER_REPLY_MODE=model
export OPENAI_API_KEY=your-api-key
export OPENAI_MODEL=your-model
export OPENAI_BASE_URL=https://your-openai-compatible-endpoint/v1
export CUSTOMER_REPLY_TIMEOUT=15s
go run ./examples/compose-quality-gate
```

模型输出具有非确定性。在线运行只用于冒烟验证；默认回归测试使用 scripted ChatModel，不访问网络。

预期输出的关键部分：

```text
question=我的订单什么时候能退款？
draft=您好，关于“我的订单什么时候能退款？”，我们已收到您的问题，正在为您核实处理。
graph=compose_quality_gate nodes=5 edges=5 branches=1
status=approved score=8 attempts=2
attempt=1 score=4 reason="refund timing notice is missing"
attempt=2 score=8 reason="refund timing notice is present"
delivery=sent customer_id=customer-001 reply="您好，关于“我的订单什么时候能退款？”，我们已收到您的问题，正在为您核实处理。\n退款到账时间以支付平台实际处理结果为准。"
```

## 验证

```bash
gofmt -w examples/compose-quality-gate/*.go
go test ./examples/compose-quality-gate
go test -race ./examples/compose-quality-gate
go test ./...
go vet ./...
```

## 已知限制

- Local State 只在一次 `Runnable` 调用内有效，不是持久化审计或 checkpoint。
- 确定性补救只追加退款到账时间说明，用于验证循环控制流，不代表通用内容改写策略。
- `GraphCompileCallback.OnFinish` 没有错误返回值，拓扑快照器只能观测，不能否决编译。
- 真实 ChatModel 目前只负责生成初始回复，离线 Inspector 和补救节点仍是确定性实现。
- ChatModel 尚未注册为 Compose 节点；本阶段也不覆盖流式模型输出、RAG、Agent Tool、HTTP 服务或生产部署。
