package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSimulatedCustomerReplyGenerator(t *testing.T) {
	generator := simulatedCustomerReplyGenerator{}

	draft, err := generator.Generate(context.Background(), "我的订单什么时候能退款？")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if draft == "" {
		t.Fatal("Generate() returned an empty draft")
	}
	if !strings.Contains(draft, "我的订单什么时候能退款？") {
		t.Fatalf("Generate() draft = %q, want question included", draft)
	}
}

func TestSimulatedCustomerReplyGeneratorRejectsEmptyQuestion(t *testing.T) {
	_, err := (simulatedCustomerReplyGenerator{}).Generate(context.Background(), " \n\t ")
	if !errors.Is(err, ErrEmptyCustomerQuestion) {
		t.Fatalf("Generate() error = %v, want ErrEmptyCustomerQuestion", err)
	}
}

func TestSimulatedCustomerReplyGeneratorHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := (simulatedCustomerReplyGenerator{}).Generate(ctx, "退款什么时候到账？")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Generate() error = %v, want context.Canceled", err)
	}
}
