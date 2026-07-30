package main

import "fmt"

type order struct {
	id     string
	amount int
}

type RiskRule func(order) error

// maxAmountRule 是高阶函数：它返回一个 RiskRule。
// 返回的函数会通过闭包记住本次传入的 limit。
func maxAmountRule(limit int) RiskRule {
	return func(current order) error {
		if current.amount > limit {
			return fmt.Errorf("订单 %s 金额 %d 元超过限额 %d 元", current.id, current.amount, limit)
		}
		return nil
	}
}

func main() {
	current := order{id: "order-001", amount: 3000}

	// 这里执行 maxAmountRule，生成并返回两个记住不同限额的函数。
	normalRiskRule := maxAmountRule(1000)
	vipRiskRule := maxAmountRule(5000)

	// 这里才执行返回的具体风控函数。
	fmt.Printf("普通订单检查：%v\n", normalRiskRule(current))
	fmt.Printf("VIP 订单检查：%v\n", vipRiskRule(current))
}
