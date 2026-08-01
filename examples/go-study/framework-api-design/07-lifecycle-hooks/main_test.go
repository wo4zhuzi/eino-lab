package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestRunCallsHooksInLifecycleOrder(t *testing.T) {
	var calls []string
	runner := taskRunner{
		run: func(_ context.Context, input string) (string, error) {
			calls = append(calls, "run:"+input)
			return "result", nil
		},
		hooks: hooks{
			before: func(_ context.Context, input string) (string, error) {
				calls = append(calls, "before:"+input)
				return "normalized", nil
			},
			after: func(_ context.Context, output string) (string, error) {
				calls = append(calls, "after:"+output)
				return output + ":decorated", nil
			},
			onError: func(_ context.Context, _ error) error {
				calls = append(calls, "error")
				return nil
			},
		},
	}

	output, err := runner.Run(context.Background(), "raw")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if output != "result:decorated" {
		t.Fatalf("Run() output = %q, want %q", output, "result:decorated")
	}
	wantCalls := []string{"before:raw", "run:normalized", "after:result"}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestRunCallsErrorHookAndSkipsAfterHook(t *testing.T) {
	errBusiness := errors.New("业务规则拒绝")
	var calls []string
	runner := taskRunner{
		run: func(_ context.Context, _ string) (string, error) {
			calls = append(calls, "run")
			return "", errBusiness
		},
		hooks: hooks{
			before: func(_ context.Context, input string) (string, error) {
				calls = append(calls, "before")
				return input, nil
			},
			after: func(_ context.Context, output string) (string, error) {
				calls = append(calls, "after")
				return output, nil
			},
			onError: func(_ context.Context, err error) error {
				calls = append(calls, "error")
				if !errors.Is(err, errBusiness) {
					t.Fatalf("ErrorHook error = %v, want errors.Is(errBusiness)", err)
				}
				return nil
			},
		},
	}

	_, err := runner.Run(context.Background(), "input")
	if !errors.Is(err, errBusiness) {
		t.Fatalf("Run() error = %v, want errors.Is(errBusiness)", err)
	}
	wantCalls := []string{"before", "run", "error"}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestRunReportsCanceledContextBeforeOtherStages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var observed error
	runner := taskRunner{
		run: func(_ context.Context, _ string) (string, error) {
			t.Fatal("context 已取消时不应执行主逻辑")
			return "", nil
		},
		hooks: hooks{
			onError: func(_ context.Context, err error) error {
				observed = err
				return nil
			},
		},
	}

	_, err := runner.Run(ctx, "input")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
	if !errors.Is(observed, context.Canceled) {
		t.Fatalf("ErrorHook observed = %v, want context.Canceled", observed)
	}
}

func TestRunPreservesRunAndErrorHookFailures(t *testing.T) {
	errBusiness := errors.New("业务失败")
	errObserver := errors.New("上报失败")
	runner := taskRunner{
		run: func(_ context.Context, _ string) (string, error) {
			return "", errBusiness
		},
		hooks: hooks{
			onError: func(_ context.Context, _ error) error {
				return errObserver
			},
		},
	}

	_, err := runner.Run(context.Background(), "input")
	if !errors.Is(err, errBusiness) || !errors.Is(err, errObserver) {
		t.Fatalf("Run() error = %v, want both errors preserved", err)
	}
}
