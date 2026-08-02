# 09. Go SDK API 设计

前置知识见 [05. Functional Options](../05-functional-options/README.md)、[06. 控制反转 IoC](../06-inversion-of-control/README.md)和 [08. Go 中间件设计](../08-middleware/README.md)。

## 学习目标

本示例把前八课的基础能力组合成一个最小节点 SDK，重点回答：

1. 哪些参数应成为构造函数的必填参数，哪些配置适合 `With...`。
2. 可替换依赖何时使用接口，简单事件通知何时使用函数回调。
3. SDK 如何统一传递 `context.Context`、超时和错误链，又不泄漏内部配置结构。

这不是 Eino 源码的简化复制。它是只依赖 Go 标准库的 API 设计实验。

## 公开 API 的边界

调用方只需要使用下面这条主路径：

```go
node, err := NewNode(
    "answer-node",
    generator,
    WithTimeout(time.Second),
    WithLogger(logger),
    WithCompletionCallback(callback),
)
output, err := node.Run(ctx, input)
```

| 能力 | API 形式 | 选择依据 |
|---|---|---|
| 节点名称 | 构造函数必填参数 | 缺少名称时对象没有稳定身份 |
| `Generator` | 构造函数必填接口 | 缺少核心执行依赖时节点无法工作；接口允许替换实现和测试桩 |
| 超时 | `WithTimeout` | 有安全默认值，不是每个调用方都需要覆盖 |
| `Logger` | `WithLogger` 注入接口 | 是跨多次调用、具有多个行为的长期依赖 |
| 完成通知 | `WithCompletionCallback` 函数 | 只对应一个固定事件，不需要额外接口类型 |
| 输入与取消 | `Run(ctx, input)` | 属于单次调用，不应固化在 Node 配置中 |

`nodeConfig` 保持私有。这样将来增加可选配置时，不需要让调用方初始化完整配置结构，也不会暴露内部默认值。

## 完整运行链路

```text
NewNode
  -> 校验必填名称和 Generator
  -> 建立默认超时与空 Logger
  -> 依次应用 Option
  -> 返回不可变配置的 Node

Node.Run
  -> 从调用方 context 派生超时 context
  -> 规范化并校验输入
  -> 调用注入的 Generator
  -> 记录成功或失败日志
  -> 调用完成回调
  -> 向调用方返回结果或保留错误链的错误
```

`GeneratorFunc` 是接口适配器。简单函数可以直接转成 `GeneratorFunc`，已有结构体实现也能直接满足 `Generator` 接口，SDK 不需要提供两个构造函数。

## 错误与回调语义

`Run` 使用 `%w` 包装 Generator 错误，因此调用方仍能使用 `errors.Is` 判断依赖错误或 `context.DeadlineExceeded`。空输入在调用 Generator 前返回 `ErrEmptyInput`，`nil context` 返回 `ErrNilContext`；这两个导出哨兵错误让调用方无需匹配错误文本。

完成回调只观察最终结果，不返回新结果或错误。这是刻意的约束：观测代码不应把一次成功调用改成失败，也不应吞掉原始错误。回调在正常结果、业务校验失败和依赖失败时各执行一次；`nil context` 因为没有可用的调用上下文，不执行日志和回调。

## 运行与验证

本示例只使用 Go 标准库。仓库声明 Go `1.26.0`，当前验证运行时为 Go `1.26.3`，不需要凭据、网络或外部服务。

在仓库根目录执行：

```bash
go run ./examples/go-study/framework-api-design/09-sdk-api-design
go test ./examples/go-study/framework-api-design/09-sdk-api-design -count=1
```

预期输出：

```text
INFO: 节点 "answer-node" 开始执行
INFO: 节点 "answer-node" 执行成功
完成回调：node=answer-node status=success
回答：SDK API 如何保持稳定？
INFO: 节点 "answer-node" 开始执行
ERROR: 节点 "answer-node" 执行失败: 输入不能为空
完成回调：node=answer-node status=failed
失败结果：输入不能为空
```

测试覆盖依赖注入、默认配置、日志与完成回调、依赖错误链、超时传播、空输入、`nil context` 和非法构造参数。

## 已知限制

- SDK 只能把超时 context 传给 Generator；如果 Generator 忽略 context，SDK 无法强制中断它。
- Logger 和完成回调都同步执行，示例不处理慢日志、回调 panic 或异步投递。
- 示例只展示单节点 API，不包含图编排、重试、中间件注册和并发状态管理。
- 为保持单课边界，本示例使用 `package main`；真实公共 SDK 应放在可导入包中，并为导出 API 提供兼容性策略。
