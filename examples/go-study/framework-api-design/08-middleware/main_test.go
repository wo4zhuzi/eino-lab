package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestChainUsesOnionOrder(t *testing.T) {
	var calls []string
	record := func(name string) Middleware {
		return func(next Handler) Handler {
			return func(ctx context.Context, input string) (string, error) {
				calls = append(calls, name+":before")
				output, err := next(ctx, input)
				calls = append(calls, name+":after")
				return output, err
			}
		}
	}
	final := func(_ context.Context, input string) (string, error) {
		calls = append(calls, "handler:"+input)
		return "result", nil
	}

	handler := Chain(final, record("first"), record("second"))
	output, err := handler(context.Background(), "input")
	if err != nil {
		t.Fatalf("handler() error = %v", err)
	}
	if output != "result" {
		t.Fatalf("handler() output = %q, want %q", output, "result")
	}
	want := []string{
		"first:before",
		"second:before",
		"handler:input",
		"second:after",
		"first:after",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestAuthenticationMiddlewareShortCircuitsChain(t *testing.T) {
	called := false
	final := func(_ context.Context, _ string) (string, error) {
		called = true
		return "不应返回", nil
	}
	handler := Chain(final, authenticationMiddleware)

	_, err := handler(context.Background(), "input")
	if !errors.Is(err, errUnauthorized) {
		t.Fatalf("handler() error = %v, want errUnauthorized", err)
	}
	if called {
		t.Fatal("鉴权失败后不应调用下游 Handler")
	}
}

func TestLoggingMiddlewarePreservesDownstreamError(t *testing.T) {
	errBusiness := errors.New("业务失败")
	final := func(_ context.Context, _ string) (string, error) {
		return "", errBusiness
	}
	handler := Chain(final, loggingMiddleware)

	_, err := handler(context.Background(), "input")
	if !errors.Is(err, errBusiness) {
		t.Fatalf("handler() error = %v, want errors.Is(errBusiness)", err)
	}
}

func TestContextMiddlewareStopsCanceledRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	final := func(_ context.Context, _ string) (string, error) {
		called = true
		return "", nil
	}
	handler := Chain(final, contextMiddleware)

	_, err := handler(ctx, "input")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("handler() error = %v, want context.Canceled", err)
	}
	if called {
		t.Fatal("context 取消后不应调用下游 Handler")
	}
}

func TestChainWithoutMiddlewareCallsFinalDirectly(t *testing.T) {
	final := func(_ context.Context, input string) (string, error) {
		return "handled:" + input, nil
	}
	handler := Chain(final)

	output, err := handler(context.Background(), "input")
	if err != nil {
		t.Fatalf("handler() error = %v", err)
	}
	if output != "handled:input" {
		t.Fatalf("handler() output = %q, want %q", output, "handled:input")
	}
}
