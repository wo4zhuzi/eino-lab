# 02. 函数作为参数

## 这个案例解决什么问题

订单检查包含两部分：

1. 所有订单都要执行的公共检查，例如订单编号不能为空。
2. 根据订单类型变化的风控规则，例如普通订单和 VIP 订单采用不同金额上限。

普通写法可能为每种类型分别编写完整流程：

```go
func checkNormalOrder(current order) error {
    // 检查订单编号
    // 执行普通订单规则
    // 包装错误
}

func checkVIPOrder(current order) error {
    // 再次检查订单编号
    // 执行 VIP 订单规则
    // 再次包装错误
}
```

这种写法会重复公共检查和错误处理。以后修改公共流程时，容易漏改某一种订单类型。

## 函数参数写法

把变化的规则作为参数交给固定流程：

```go
type RiskRule func(order) error

func checkOrder(current order, rule RiskRule) error {
    if current.id == "" {
        return fmt.Errorf("订单编号不能为空")
    }
    if rule == nil {
        return fmt.Errorf("风控规则不能为空")
    }
    if err := rule(current); err != nil {
        return fmt.Errorf("订单 %s 风控检查失败: %w", current.id, err)
    }
    return nil
}
```

调用方决定本次使用哪条规则：

```go
checkOrder(current, normalRiskRule)
checkOrder(current, vipRiskRule)
```

这里传入的是函数值 `normalRiskRule`，不是调用结果 `normalRiskRule(current)`。真正调用规则的是 `checkOrder`。

## 在这个场景下好在哪里

1. 公共校验和错误包装只实现一次。
2. 变化的规则与固定流程分离，新增规则不需要复制完整流程。
3. 调用方可以为每次调用选择不同规则。
4. 测试可以传入一个受控函数，确认公共检查失败时不会执行风控规则。
5. 执行流程在调用前拒绝 nil 函数值，避免因无效扩展点配置而 panic。

推荐用于“处理流程固定，只有其中一个步骤需要替换”的场景，例如排序规则、重试条件、数据过滤、权限判断和框架回调。

如果函数始终只调用一个固定实现，直接调用更清晰；如果一组行为包含多个方法，应优先考虑接口。

## 与第 1 个示例的区别

两个示例的底层机制相同，都是把函数当作值使用。区别不在执行结果，而在于函数由谁持有、什么时候选择以及使用多久。

| 对比 | 第 1 个示例：函数保存到字段 | 第 2 个示例：函数作为参数 |
|---|---|---|
| 类似角色 | 配置完成的节点或组件实例 | 接收业务逻辑的通用执行器 |
| 函数持有者 | `riskNode` 结构体 | 当前调用栈 |
| 规则选择时机 | 创建节点时选择一次 | 每次调用时选择 |
| 使用周期 | 节点存活期间反复使用 | 只用于当前调用 |
| 适用条件 | 同一个组件长期执行同一种行为 | 不同调用可能临时采用不同规则 |

第 1 个示例类似一个已经配置好的框架节点：

```go
vipNode := riskNode{rule: vipRiskRule}
vipNode.Check(order1)
vipNode.Check(order2)
```

节点创建后一直使用 VIP 规则。它回答的是：“这个节点使用什么规则？”

第 2 个示例类似一个通用执行器：

```go
checkOrder(order1, normalRiskRule)
checkOrder(order2, vipRiskRule)
```

调用方每次执行时临时传入规则。它回答的是：“这次调用使用什么规则？”

工程上，同一个规则需要反复执行时，推荐保存到结构体字段；每次调用可能切换规则时，推荐使用函数参数；永远只有一个固定规则时，直接调用函数最简单。

## 运行

本示例只使用 Go 标准库。仓库使用 Go `1.26.0`。

在仓库根目录执行：

```bash
go run ./examples/go-study/framework-api-design/02-function-as-parameter
go test ./examples/go-study/framework-api-design/02-function-as-parameter
```

预期输出：

```text
普通订单检查：订单 order-001 风控检查失败: 金额不能超过 1000 元
VIP 订单检查：<nil>
```

已知限制：本示例只演示函数作为参数，不涉及返回函数；返回函数将在第 3 个示例中说明。
