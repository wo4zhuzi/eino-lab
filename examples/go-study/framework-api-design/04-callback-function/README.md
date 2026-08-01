# 04. 回调函数

## 这个案例解决什么问题

订单事件循环负责等待事件，但订单到达后执行什么业务检查，应由业务代码决定。如果事件循环直接调用固定的 `checkAmount`，以后替换规则就必须修改事件循环。

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

`Run` 是真正的调用位置。事件循环在订单到达后传入参数并执行已登记的函数：

```go
func (l *orderEventLoop) Run(current order) error {
    if l.handler == nil {
        return fmt.Errorf("订单回调尚未注册")
    }
    if err := l.handler(current); err != nil {
        return fmt.Errorf("处理订单 %s 失败: %w", current.id, err)
    }
    return nil
}
```

调用方只负责登记业务行为和启动事件处理：

```go
loop.Register(checkAmount) // 注册函数值，此时不执行
loop.Run(current)          // 事件发生后，事件循环执行回调
```

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
订单事件处理：处理订单 order-001 失败: 金额 3000 元超过限额 1000 元
```

第一行先出现，说明注册本身没有执行订单检查；调用 `Run` 后才得到检查错误。

已知限制：本示例只注册一个同步回调，不讨论多个订阅者、并发执行和取消。框架如何通过回调反转控制权，会在第 6 个示例中继续学习。
