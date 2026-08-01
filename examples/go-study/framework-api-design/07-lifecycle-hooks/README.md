# 07. 生命周期钩子

前置知识见 [04. 回调函数](../04-callback-function/README.md)和 [06. 控制反转 IoC](../06-inversion-of-control/README.md)。

## 学习目标

生命周期钩子是框架在固定执行时点调用的回调。本示例用一个最小任务执行器回答三个问题：

1. `BeforeHook`、主逻辑、`AfterHook` 的正常执行顺序是什么。
2. 主逻辑失败后为什么跳过 `AfterHook`，改为调用 `ErrorHook`。
3. 钩子失败和 `context` 取消如何沿原错误链返回调用方。

## 核心结构

应用登记行为：

```go
runner := taskRunner{
    run: answer,
    hooks: hooks{
        before:  normalizeQuestion,
        after:   addRequestID,
        onError: reportError,
    },
}
```

框架统一控制调用时机：

```text
正常路径：BeforeHook -> 主逻辑 -> AfterHook -> 返回结果
失败路径：BeforeHook 或主逻辑或 AfterHook 失败 -> ErrorHook -> 返回错误
```

这体现了控制反转：`main` 没有逐个调用三个钩子，只负责注册；`taskRunner.Run` 决定执行顺序、传入参数和错误传播。

## 各钩子的职责

| 钩子 | 执行时机 | 适合做什么 | 不应做什么 |
|---|---|---|---|
| `BeforeHook` | 主逻辑之前 | 输入规范化、鉴权、前置校验 | 假装主逻辑已经成功 |
| `AfterHook` | 主逻辑成功之后 | 输出修饰、成功指标、结果校验 | 处理主逻辑失败 |
| `ErrorHook` | 任一执行阶段失败之后 | 记录错误、失败指标、告警 | 吞掉原错误并返回成功 |

本示例允许 Before 和 After 转换数据，所以它们既是观测点，也是处理链的一部分。生产 API 如果只允许观测，应把签名设计为不返回替换后的输入或输出，以减少钩子意外改变业务语义的风险。

## 错误传播

每个阶段使用 `%w` 补充位置上下文，因此调用方仍可使用 `errors.Is` 判断业务错误或 `context.Canceled`。如果 `ErrorHook` 自身也失败，示例使用 `errors.Join` 同时保留原始执行错误和钩子错误，不让观测故障覆盖根因。

## 与中间件的边界

生命周期钩子由执行器预先定义固定时点。中间件则接收并返回一个完整 Handler，可以包裹整个调用并自由决定是否、何时以及调用几次下一个 Handler。下一课会专门验证中间件的包裹顺序，本课不混合这两种机制。

## 运行与验证

本示例只使用 Go 标准库。仓库声明 Go `1.26.0`，当前验证运行时为 Go `1.26.3`，不需要外部服务。

在仓库根目录执行：

```bash
go run ./examples/go-study/framework-api-design/07-lifecycle-hooks
go test ./examples/go-study/framework-api-design/07-lifecycle-hooks -count=1
```

预期输出：

```text
回答：生命周期钩子何时执行？ [request_id=req-001]
ErrorHook 观察到：BeforeHook 执行失败: 问题不能为空
失败结果：BeforeHook 执行失败: 问题不能为空
```

测试覆盖正常顺序、主逻辑错误、取消传播，以及 ErrorHook 自身失败时保留两条错误链。

## 已知限制

- 钩子同步执行，不讨论异步上报、并发安全和超时隔离。
- 每个时点只支持一个钩子，不讨论多个钩子的排序和冲突。
- 这是用于理解 API 设计的标准库实验，不复制 Eino 的内部实现。
