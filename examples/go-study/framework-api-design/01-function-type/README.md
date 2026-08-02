# 01. Go 函数类型

## 这个案例解决什么问题

系统需要检查订单风险，但普通订单和 VIP 订单采用不同规则：

- 普通订单金额不能超过 1000 元。
- VIP 订单金额不能超过 5000 元。

如果把规则判断直接写进执行代码，通常会得到下面的普通写法：

```go
func checkOrder(level string, current order) error {
    switch level {
    case "normal":
        return normalRiskRule(current)
    case "vip":
        return vipRiskRule(current)
    default:
        return fmt.Errorf("未知订单类型")
    }
}
```

这种写法在规则固定且只有一两个分支时完全够用。问题是每增加一种订单类型，都必须修改 `checkOrder` 的分支，规则选择和规则执行耦合在一起。

## 函数类型写法

示例定义统一的规则类型，并把具体规则保存到节点字段中：

```go
type RiskRule func(order) error

type riskNode struct {
    rule RiskRule
}
```

创建节点时决定它保存哪条规则：

```go
normalNode := riskNode{rule: normalRiskRule}
vipNode := riskNode{rule: vipRiskRule}
```

运行节点时只执行已经保存的规则，不再判断订单类型：

```go
func (n riskNode) Check(current order) error {
    return n.rule(current)
}
```

## 在这个场景下好在哪里

1. `riskNode.Check` 只负责执行规则，新增规则时不需要修改它。
2. 所有规则必须满足 `func(order) error`，签名不匹配会在编译期报错。
3. `RiskRule` 给函数签名增加了业务名称，比结构体字段直接使用裸类型 `func(order) error` 更容易理解。
4. 测试可以给节点配置受控规则，不必进入 `switch` 的不同分支。

推荐在框架节点、校验器、事件处理器等“执行流程固定，但具体行为需要替换”的场景使用。规则很少、长期固定且不需要独立测试时，直接调用或 `switch` 更简单，不必为了模式而使用函数类型。

函数类型的零值是 `nil`，直接调用 nil 函数会导致 panic。因此 `riskNode.Check` 会先确认规则已经配置；真实组件也可以通过构造函数保证对象创建后始终有效。

## 运行

本示例只使用 Go 标准库。仓库使用 Go `1.26.0`。

在仓库根目录执行：

```bash
go run ./examples/go-study/framework-api-design/01-function-type
go test ./examples/go-study/framework-api-design/01-function-type
```

预期输出：

```text
普通订单检查：普通订单金额不能超过 1000 元
VIP 订单检查：<nil>
```

同一笔 3000 元订单使用两个节点执行：普通规则拒绝，VIP 规则通过。这说明节点保存的函数决定了它的行为。

已知限制：本示例只演示函数类型的定义、保存和零值保护。如何通过参数把规则传给执行流程，会在第 2 个示例中说明。
