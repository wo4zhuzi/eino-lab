# 03. 高阶函数

## 这个案例解决什么问题

普通订单、VIP 订单和企业订单都需要检查最大金额。它们的检查逻辑完全相同，只有金额上限不同。

普通写法可能为每个额度分别定义函数：

```go
func normalRiskRule(current order) error {
    if current.amount > 1000 {
        return fmt.Errorf("超过限额")
    }
    return nil
}

func vipRiskRule(current order) error {
    if current.amount > 5000 {
        return fmt.Errorf("超过限额")
    }
    return nil
}
```

继续增加企业订单时，还要复制相同判断并修改数字。这会让同一逻辑散落在多个函数中。

## 高阶函数写法

高阶函数是接收函数或返回函数的函数。本示例使用“返回函数”这一种形式：

```go
func maxAmountRule(limit int) RiskRule {
    return func(current order) error {
        if current.amount > limit {
            return fmt.Errorf("超过限额 %d 元", limit)
        }
        return nil
    }
}
```

`maxAmountRule(1000)` 的结果不是订单检查结果，而是一个新的 `RiskRule`：

```go
normalRiskRule := maxAmountRule(1000)
vipRiskRule := maxAmountRule(5000)
```

两个返回函数共享同一套判断逻辑，但分别记住 `1000` 和 `5000`。函数记住其外层变量的能力称为闭包。

真正检查订单发生在调用返回函数时：

```go
normalRiskRule(current)
vipRiskRule(current)
```

## 在这个场景下好在哪里

1. 相同的金额判断逻辑只实现一次。
2. 调用方可以用配置生成不同规则，不需要为每个额度新增函数。
3. 返回的函数记住自己的配置，后续调用时不必重复传入额度。
4. 修改错误格式或边界判断时只需改一处。

推荐用于“行为逻辑相同，只有配置不同”的场景，例如：

- 根据超时时间生成超时处理器。
- 根据重试次数生成重试策略。
- 根据前缀生成文本转换器。
- 根据金额上限生成风控规则。
- Functional Options 中生成配置修改函数。

如果配置只使用一次，直接把配置作为普通参数传给执行函数更简单；如果闭包捕获大量可变状态，则要额外考虑并发安全和生命周期。

## 与前两个示例的关系

| 示例 | 核心动作 | 回答的问题 |
|---|---|---|
| 01. 函数类型 | 定义并保存函数 | 这个节点长期使用什么行为？ |
| 02. 函数作为参数 | 把已有函数交给执行器 | 这次调用临时使用什么行为？ |
| 03. 高阶函数 | 根据配置生成新函数 | 如何批量创建逻辑相同、配置不同的行为？ |

## 运行

本示例只使用 Go 标准库。仓库使用 Go `1.26.0`。

在仓库根目录执行：

```bash
go run ./examples/go-study/framework-api-design/03-higher-order-function
go test ./examples/go-study/framework-api-design/03-higher-order-function
```

预期输出：

```text
普通订单检查：订单 order-001 金额 3000 元超过限额 1000 元
VIP 订单检查：<nil>
```

已知限制：本示例只捕获创建后不再变化的整数配置，不讨论闭包共享可变状态时的并发问题。
