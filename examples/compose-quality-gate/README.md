# Compose 内容质量门禁

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

`Inspector` 是唯一外部能力边界。默认 `ruleInspector` 是确定性本地实现，不调用真实模型、审核服务或网络。

## 阅读顺序

建议按职责从外到内阅读，不要从单个文件第一行一路读到底：

1. `main.go`：程序入口，只看配置、构建和一次 `Review` 调用。
2. `gate.go`：公开类型、Local State、`NewQualityGate`、`Compile` 和 `Review`。
3. `topology.go`：Lambda 节点注册、Branch 目标和 Edge 连接。
4. `nodes.go`：validate、inspect、remediate、approve、manual 的具体业务逻辑。
5. `observer.go` 与 `snapshot.go`：运行期和编译期观测扩展。
6. `*_test.go`：正常路径、故障、并发隔离和扩展边界证据。

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
graph=compose_quality_gate nodes=5 edges=5 branches=1
status=approved score=8 attempts=2
attempt=1 score=4 reason="required review marker is missing"
attempt=2 score=8 reason="required review marker is present"
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
- 确定性补救只用于验证循环控制流，不代表真实内容改写策略。
- `GraphCompileCallback.OnFinish` 没有错误返回值，拓扑快照器只能观测，不能否决编译。
- 本示例不覆盖真实 ChatModel、流式运行、RAG、Agent Tool、HTTP 服务或生产部署。
