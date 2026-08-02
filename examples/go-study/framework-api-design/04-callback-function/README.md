# 04. 回调函数

## 这个案例解决什么问题

订单可能来自消息队列、网络连接或其他外部事件源，业务代码不知道下一笔订单何时到达。事件循环负责持续等待订单，但订单到达后执行什么业务检查，应由业务代码决定。如果事件循环直接调用固定的 `checkAmount`，以后替换规则就必须修改事件循环。

回调把“何时执行”和“执行什么”分开：

- 业务代码注册 `checkAmount`，决定执行什么。
- 事件循环收到订单后调用回调，决定何时执行并提供参数。

## 回调函数写法

先定义统一的回调类型，并把它保存在事件循环中：

```go
type OrderHandler func(order) error

type orderEventLoop struct {
    handler OrderHandler
}
```

`Register` 是注册位置，只保存函数值，不执行函数：

```go
func (l *orderEventLoop) Register(handler OrderHandler) error {
    if handler == nil {
        return fmt.Errorf("订单回调不能为空")
    }
    l.handler = handler
    return nil
}
```

`Run` 是真正的调用位置。它持续等待订单通道，在事件到达后传入参数并执行已登记的函数：

```go
func (l *orderEventLoop) Run(ctx context.Context, orders <-chan order) error {
    if l.handler == nil {
        return fmt.Errorf("订单回调尚未注册")
    }
    for {
        select {
        case <-ctx.Done():
            return fmt.Errorf("订单事件循环结束: %w", ctx.Err())
        case current, ok := <-orders:
            if !ok {
                return nil
            }
            if err := l.handler(current); err != nil {
                return fmt.Errorf("处理订单 %s 失败: %w", current.id, err)
            }
        }
    }
}
```

调用方只负责登记业务行为和启动事件处理：

```go
loop.Register(checkAmount)                 // 业务声明订单到达后做什么
loop.Run(context.Background(), orderEvents) // 框架等待事件并执行回调
```

## 为什么不直接调用函数

如果订单已经在当前业务代码手中，而且只需要处理一次，直接调用更简单：

```go
err := checkAmount(current)
```

回调适用于调用时机由另一个组件掌握的场景。本例中业务代码只注册 `checkAmount`，并不在发送订单时调用它：

```text
业务代码：注册 checkAmount，定义“订单到达后做什么”
外部事件源：在未知时刻产生订单
事件循环：等待订单，决定何时调用 checkAmount，并处理取消和错误
```

如果不用回调，业务代码就必须自己接管等待、循环、取消和错误处理。回调让事件循环保持通用，同时允许不同应用替换订单处理规则。

本示例约定取消优先：进入每轮等待以及收到通道结果后都会检查 `ctx.Err()`。因此 context 已取消且订单通道同时关闭时，`Run` 稳定返回取消错误，不会因 `select` 随机选择就绪分支而偶尔返回 `nil`。

典型应用包括消息队列消费者、HTTP 路由、定时任务、UI 事件和框架生命周期钩子。没有外部事件或框架控制流程时，不应为了使用回调而使用回调。

## 回调与普通函数参数的区别

回调在 Go 语法上仍然是函数参数，区别主要是控制权和执行时机。

| 对比 | 第 2 个示例：函数作为参数 | 第 4 个示例：回调函数 |
|---|---|---|
| 传入位置 | 调用执行函数时传入 | 提前注册并保存 |
| 调用时机 | 当前函数调用期间 | 后续事件发生时 |
| 谁控制时机 | 直接调用方 | 事件循环或框架 |
| 参数来源 | 直接调用方 | 事件循环或框架 |
| 典型用途 | 临时替换算法步骤 | 事件处理、生命周期钩子 |

“回调”描述的是使用关系，不是一种新的 Go 语法。`checkAmount` 既是普通函数，也是业务注册给事件循环的回调。

## 在框架 API 中的意义

框架通常掌握运行流程，应用只登记扩展行为。这个例子中的 `orderEventLoop` 类似框架，`checkAmount` 类似应用回调：

1. 应用知道业务规则，但不知道事件何时到达。
2. 框架知道调用时机和运行参数，但不应写死业务规则。
3. 回调让双方只依赖约定的函数签名。

注册后异步调用、并发调用或长期保存回调时，还需要明确生命周期和并发安全；本示例采用同步调用，只聚焦注册与执行时机。

## 运行

本示例只使用 Go 标准库。仓库使用 Go `1.26.0`。

在仓库根目录执行：

```bash
go run ./examples/go-study/framework-api-design/04-callback-function
go test ./examples/go-study/framework-api-design/04-callback-function
```

预期输出：

```text
订单回调已注册
订单事件循环：处理订单 order-002 失败: 金额 3000 元超过限额 1000 元
```

第一行先出现，说明注册本身没有执行订单检查。事件循环随后依次接收两笔订单，第一笔通过，第二笔失败并停止循环。

已知限制：本示例只注册一个同步回调，因此再次注册会覆盖之前的回调；它不讨论多个订阅者和并发执行。框架如何通过回调反转控制权，会在第 6 个示例中继续学习。
