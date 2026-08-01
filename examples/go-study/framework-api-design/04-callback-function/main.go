package main

import "fmt"

type order struct {
	id     string
	amount int
}

type OrderHandler func(order) error

type orderEventLoop struct {
	handler OrderHandler
}

// Register 保存业务回调，但不会在注册时执行它。
func (l *orderEventLoop) Register(handler OrderHandler) error {
	if handler == nil {
		return fmt.Errorf("订单回调不能为空")
	}
	l.handler = handler
	return nil
}

// Run 模拟框架收到订单事件后，在约定时机调用已注册的业务回调。
func (l *orderEventLoop) Run(current order) error {
	if l.handler == nil {
		return fmt.Errorf("订单回调尚未注册")
	}
	if err := l.handler(current); err != nil {
		return fmt.Errorf("处理订单 %s 失败: %w", current.id, err)
	}
	return nil
}

func checkAmount(current order) error {
	if current.amount > 1000 {
		return fmt.Errorf("金额 %d 元超过限额 1000 元", current.amount)
	}
	return nil
}

func main() {
	loop := &orderEventLoop{}

	// 注册位置：业务把 checkAmount 交给事件循环，此时不会检查订单。
	if err := loop.Register(checkAmount); err != nil {
		fmt.Printf("注册回调失败：%v\n", err)
		return
	}
	fmt.Println("订单回调已注册")

	// 调用位置：事件到达后，事件循环决定调用时机并传入订单。
	err := loop.Run(order{id: "order-001", amount: 3000})
	fmt.Printf("订单事件处理：%v\n", err)
}
