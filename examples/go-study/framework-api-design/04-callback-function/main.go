package main

import (
	"context"
	"fmt"
)

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

// Run 持续等待订单事件，并由事件循环调用已注册的业务回调。
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

	// 订单由外部事件源产生，业务代码不直接调用 checkAmount。
	orders := make(chan order, 2)
	orders <- order{id: "order-001", amount: 800}
	orders <- order{id: "order-002", amount: 3000}
	close(orders)

	// 事件循环负责等待事件、传入订单，并在回调失败时停止运行。
	if err := loop.Run(context.Background(), orders); err != nil {
		fmt.Printf("订单事件循环：%v\n", err)
	}
}
