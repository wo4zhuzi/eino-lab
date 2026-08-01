package main

import (
	"errors"
	"testing"
)

func TestRegisterDoesNotCallHandlerUntilRun(t *testing.T) {
	called := false
	var received order
	loop := &orderEventLoop{}
	handler := OrderHandler(func(current order) error {
		called = true
		received = current
		return nil
	})

	if err := loop.Register(handler); err != nil {
		t.Fatalf("Register() error = %v, want nil", err)
	}
	if called {
		t.Fatal("handler was called during registration")
	}

	want := order{id: "order-001", amount: 800}
	if err := loop.Run(want); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if !called {
		t.Fatal("handler was not called when the event loop ran")
	}
	if received != want {
		t.Fatalf("handler received %+v, want %+v", received, want)
	}
}

func TestRunPreservesHandlerError(t *testing.T) {
	errRejected := errors.New("order rejected")
	loop := &orderEventLoop{}
	if err := loop.Register(func(order) error { return errRejected }); err != nil {
		t.Fatalf("Register() error = %v, want nil", err)
	}

	err := loop.Run(order{id: "order-001", amount: 3000})
	if !errors.Is(err, errRejected) {
		t.Fatalf("Run() error = %v, want wrapped %v", err, errRejected)
	}
}

func TestRunRequiresRegisteredHandler(t *testing.T) {
	loop := &orderEventLoop{}

	if err := loop.Run(order{id: "order-001", amount: 800}); err == nil {
		t.Fatal("Run() error = nil, want unregistered handler error")
	}
}

func TestRegisterRejectsNilHandler(t *testing.T) {
	loop := &orderEventLoop{}

	if err := loop.Register(nil); err == nil {
		t.Fatal("Register(nil) error = nil, want validation error")
	}
}
