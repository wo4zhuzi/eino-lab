package main

import "fmt"

type order struct {
	id     string
	amount int
}

type RiskRule func(order) error

func normalRiskRule(current order) error {
	if current.amount > 1000 {
		return fmt.Errorf("金额不能超过 1000 元")
	}
	return nil
}

func vipRiskRule(current order) error {
	if current.amount > 5000 {
		return fmt.Errorf("金额不能超过 5000 元")
	}
	return nil
}

// checkOrder 固定检查流程，并通过参数接收本次需要使用的规则。
func checkOrder(current order, rule RiskRule) error {
	if current.id == "" {
		return fmt.Errorf("订单编号不能为空")
	}
	if err := rule(current); err != nil {
		return fmt.Errorf("订单 %s 风控检查失败: %w", current.id, err)
	}
	return nil
}

func main() {
	current := order{id: "order-001", amount: 3000}

	// 同一个检查流程，由调用方为本次调用选择不同规则。
	normalErr := checkOrder(current, normalRiskRule)
	vipErr := checkOrder(current, vipRiskRule)

	fmt.Printf("普通订单检查：%v\n", normalErr)
	fmt.Printf("VIP 订单检查：%v\n", vipErr)
}
