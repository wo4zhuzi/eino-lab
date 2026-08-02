package main

import (
	"context"
	"errors"
	"testing"
)

func TestEventLoopCallsHandlerForIncomingOrders(t *testing.T) {
	var received []order
	loop := &orderEventLoop{}
	handler := OrderHandler(func(current order) error {
		received = append(received, current)
		return nil
	})

	if err := loop.Register(handler); err != nil {
		t.Fatalf("Register() error = %v, want nil", err)
	}
	if len(received) != 0 {
		t.Fatal("handler was called before an order arrived")
	}

	orders := make(chan order, 2)
	want := []order{
		{id: "order-001", amount: 800},
		{id: "order-002", amount: 900},
	}
	for _, current := range want {
		orders <- current
	}
	close(orders)

	if err := loop.Run(context.Background(), orders); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if len(received) != len(want) {
		t.Fatalf("handler call count = %d, want %d", len(received), len(want))
	}
	for i := range want {
		if received[i] != want[i] {
			t.Fatalf("handler received[%d] = %+v, want %+v", i, received[i], want[i])
		}
	}
}

func TestRunPreservesHandlerError(t *testing.T) {
	errRejected := errors.New("order rejected")
	loop := &orderEventLoop{}
	if err := loop.Register(func(order) error { return errRejected }); err != nil {
		t.Fatalf("Register() error = %v, want nil", err)
	}

	orders := make(chan order, 1)
	orders <- order{id: "order-001", amount: 3000}
	close(orders)
	err := loop.Run(context.Background(), orders)
	if !errors.Is(err, errRejected) {
		t.Fatalf("Run() error = %v, want wrapped %v", err, errRejected)
	}
}

func TestRunRequiresRegisteredHandler(t *testing.T) {
	loop := &orderEventLoop{}

	orders := make(chan order)
	close(orders)
	if err := loop.Run(context.Background(), orders); err == nil {
		t.Fatal("Run() error = nil, want unregistered handler error")
	}
}

func TestRegisterRejectsNilHandler(t *testing.T) {
	loop := &orderEventLoop{}

	if err := loop.Register(nil); err == nil {
		t.Fatal("Register(nil) error = nil, want validation error")
	}
}

func TestRunStopsWhenContextIsCanceled(t *testing.T) {
	loop := &orderEventLoop{}
	if err := loop.Register(func(order) error { return nil }); err != nil {
		t.Fatalf("Register() error = %v, want nil", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := loop.Run(ctx, make(chan order))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want wrapped %v", err, context.Canceled)
	}
}

func TestCanceledContextTakesPriorityOverClosedChannel(t *testing.T) {
	loop := &orderEventLoop{}
	if err := loop.Register(func(order) error { return nil }); err != nil {
		t.Fatalf("Register() error = %v, want nil", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	orders := make(chan order)
	close(orders)

	err := loop.Run(ctx, orders)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want wrapped %v", err, context.Canceled)
	}
}
