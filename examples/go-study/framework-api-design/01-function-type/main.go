package main

import "fmt"

type order struct {
	amount int
}

// RiskRule 表示一条订单风控规则。
// 使用命名函数类型后，字段含义比裸写 func(order) error 更明确。
type RiskRule func(order) error

func normalRiskRule(current order) error {
	if current.amount > 1000 {
		return fmt.Errorf("普通订单金额不能超过 1000 元")
	}
	return nil
}

func vipRiskRule(current order) error {
	if current.amount > 5000 {
		return fmt.Errorf("VIP 订单金额不能超过 5000 元")
	}
	return nil
}

// riskNode 只负责执行配置好的规则，不关心规则的具体内容。
type riskNode struct {
	rule RiskRule
}

func (n riskNode) Check(current order) error {
	if n.rule == nil {
		return fmt.Errorf("风控规则不能为空")
	}
	return n.rule(current)
}

func main() {
	current := order{amount: 3000}

	// 两个节点的执行代码完全相同，差别只在保存的规则函数。
	normalNode := riskNode{rule: normalRiskRule}
	vipNode := riskNode{rule: vipRiskRule}

	fmt.Printf("普通订单检查：%v\n", normalNode.Check(current))
	fmt.Printf("VIP 订单检查：%v\n", vipNode.Check(current))
}
