# Compose 内容质量门禁故障矩阵

本文记录 `examples/compose-quality-gate` 在 Eino `v0.9.12` 下的故障注入和实际行为。所有默认测试均为离线测试，不访问真实模型或网络服务。

## 验证命令

```bash
GOCACHE="${TMPDIR:-/tmp}/eino-lab-gocache" go test ./examples/compose-quality-gate -run 'TestQualityGate(RejectsEmptyContentBeforeInspection|InspectorFailures|StopsAtGraphStepLimit|RejectsInvalidInspection)' -v
GOCACHE="${TMPDIR:-/tmp}/eino-lab-race-gocache" go test -race ./examples/compose-quality-gate
```

## 矩阵

| 场景 | 注入位置 | 预期行为 | 实际行为 | 可观测证据 | 结论 |
|---|---|---|---|---|---|
| 空白内容 | `validate` Lambda | 返回 `ErrEmptyContent`，不调用 `Inspector` | `TestQualityGateRejectsEmptyContentBeforeInspection` 通过；`errors.Is` 成功，错误包含 `node path: [validate]`，调用数为 0 | 测试断言、NodeRunError 文本 | 业务输入错误在入口拒绝 |
| Inspector 超时 | `Inspector.Inspect` 等待 `ctx.Done()` | 传播 `context.DeadlineExceeded`，定位到 `inspect` | `TestQualityGateInspectorFailures/deadline_exceeded` 通过；`errors.Is` 成功，Callback 记录 `deadline_exceeded` | 测试断言、CallbackRecord | 取消语义和根因保留 |
| Inspector 不可用 | `Inspector` 返回 `ErrInspectorUnavailable` | 快速失败，不重试、不伪装成功 | `TestQualityGateInspectorFailures/dependency_unavailable` 通过；`errors.Is` 成功，Callback 记录 `inspector_unavailable` | 测试断言、CallbackRecord | 依赖错误由应用决定重试或降级，本轮显式失败 |
| 非法评分 | `Inspector` 返回 11 | 拒绝超出协议范围的结果 | `TestQualityGateRejectsInvalidInspection` 通过；`errors.Is` 成功，错误包含 `inspect` 节点路径 | 测试断言、NodeRunError 文本 | 组件契约在节点边界校验 |
| 循环超过 Graph 步数 | Inspector 永远返回低分，业务尝试上限设为 100，Graph 上限设为 3 | 返回 `compose.ErrExceedMaxSteps` | `TestQualityGateStopsAtGraphStepLimit` 通过，`errors.Is` 成功 | 测试断言 | `WithMaxRunSteps` 是最后一道运行保护 |
| 并发调用状态串扰 | 同一 `Runnable` 并发调用 24 次 | 每次尝试从 1 开始，审计长度独立 | `TestQualityGateLocalStateIsIsolatedAcrossConcurrentRuns` 和 `go test -race` 通过 | 每次结果的 Attempts/Audit、race 输出 | Local State 不应放入全局变量 |

## 错误传播结论

节点返回的业务错误先由 Compose 包装为带节点路径的错误，再通过 `Unwrap` 保留原始错误。应用层可以同时使用：

```go
errors.Is(err, ErrInspectorUnavailable)
strings.Contains(err.Error(), "node path: [inspect]")
```

这两种判断分别承担机器可分类性和人类诊断定位，不能只依赖错误字符串。

## 未验证风险

- `Inspector` 的真实网络重试、熔断和 SLA 不在本示例范围内。
- `WithMaxRunSteps` 只保护当前运行，不提供失败恢复或持久化。
- Callback 记录保存在进程内，生产环境仍需接入统一日志、指标或 Trace 后端。
