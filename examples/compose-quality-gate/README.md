# Compose 内容质量门禁

## 业务背景

本示例对应一个实际的 AI 客服场景：客服系统先生成回复草稿，再在发送前经过质量门禁。

```text
用户问题 -> 模拟客服生成草稿 -> 质量审核
                                |-> 通过并发送
                                |-> 修改后重新审核
                                `-> 多次失败转人工
```

当前学习阶段使用确定性模拟客服、离线审核规则和模拟交付组件，目的是先掌握 Compose 的节点、分支、循环和运行级状态，同时保留完整业务闭环。后续会把模拟客服替换为真实 ChatModel，再单独把审核器和交付组件替换为真实外部服务。

## 学习目标

本示例使用 Eino Compose 构建一个离线、可审计的内容质量门禁，重点验证：

- 类型化 `Graph`、Lambda 节点和 `GraphBranch`。
- `remediate -> inspect` 有界循环与 `WithMaxRunSteps` 保护。
- 每次调用独立生成的 Local State。
- 节点路径错误和调用级 Callback。
- 自定义 `GraphCompileCallback` 生成稳定拓扑快照。

## 运行链路

```text
START -> validate -> inspect -> Branch
                              |-> approve -> END
                              |-> remediate -> inspect
                              `-> manual -> END
```

模拟客服生成器、`Inspector` 和 `CustomerReplyDelivery` 是外部能力边界。默认实现都是确定性的本地能力，不调用真实模型、审核服务或网络。

## 阅读顺序

建议按职责从外到内阅读，不要从单个文件第一行一路读到底：

1. `main.go`：程序入口，只看问题、模拟客服草稿、配置、构建和一次 `Review` 调用。
2. `customer_service.go`：模拟客服的上游接口和确定性草稿生成。
3. `gate.go`：公开类型、Local State、`NewQualityGate`、`Compile` 和 `Review`。
4. `topology.go`：Lambda 节点注册、Branch 目标和 Edge 连接。
5. `nodes.go`：validate、inspect、remediate、approve、manual 的具体业务逻辑。
6. `delivery.go`：根据审核终态模拟发送回复或进入人工队列。
7. `observer.go` 与 `snapshot.go`：运行期和编译期观测扩展。
8. `*_test.go`：正常路径、故障、并发隔离和扩展边界证据。

这样可以先回答“程序如何运行”，再深入“每个节点做什么”。

## 前置条件

- Go `1.26.3`，以根 `go.mod` 的 `go 1.26.0` 为最低项目约束。
- Eino `v0.9.12`。
- 不需要 API Key 或其他外部服务。

## 运行

在仓库根目录执行：

```bash
go run ./examples/compose-quality-gate
```

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
- 本阶段不覆盖真实 ChatModel、流式运行、RAG、Agent Tool、HTTP 服务或生产部署；真实模型会在后续单变量迁移中接入。
