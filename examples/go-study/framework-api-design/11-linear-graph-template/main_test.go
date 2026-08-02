package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudwego/eino/compose"
)

func TestReviewGraphRunsStepsInRegistryOrder(t *testing.T) {
	runnable, err := NewReviewGraph(context.Background())
	if err != nil {
		t.Fatalf("NewReviewGraph() error = %v", err)
	}

	result, err := runnable.Invoke(context.Background(), ReviewRequest{
		Content: "  您好，退款将在 3 个工作日到账。  ",
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if !result.Approved || result.Score != 9 {
		t.Fatalf("result = %#v, want approved with score 9", result)
	}
	if result.Content != "您好，退款将在 3 个工作日到账。" {
		t.Fatalf("Content = %q", result.Content)
	}
	wantSteps := []string{"normalize", "inspect_refund_notice"}
	if !reflect.DeepEqual(result.Steps, wantSteps) {
		t.Fatalf("Steps = %#v, want %#v", result.Steps, wantSteps)
	}
	wantReasons := []string{"包含退款到账说明"}
	if !reflect.DeepEqual(result.Reasons, wantReasons) {
		t.Fatalf("Reasons = %#v, want %#v", result.Reasons, wantReasons)
	}
}

func TestReviewGraphPreservesBusinessAndContextErrors(t *testing.T) {
	runnable, err := NewReviewGraph(context.Background())
	if err != nil {
		t.Fatalf("NewReviewGraph() error = %v", err)
	}

	_, err = runnable.Invoke(context.Background(), ReviewRequest{Content: " \n\t "})
	if !errors.Is(err, ErrEmptyContent) {
		t.Fatalf("empty Invoke() error = %v, want ErrEmptyContent", err)
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = runnable.Invoke(canceledCtx, ReviewRequest{Content: "退款到账说明"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Invoke() error = %v, want context.Canceled", err)
	}
}

func TestCompileLinearGraphUsesOnlyOrderedStepRegistry(t *testing.T) {
	type middle struct {
		value string
		steps []string
	}

	steps := []linearStep[middle]{
		{
			Key: "first",
			Run: func(_ context.Context, current middle) (middle, error) {
				current.value += "-first"
				current.steps = append(current.steps, "first")
				return current, nil
			},
		},
		{
			Key: "second",
			Run: func(_ context.Context, current middle) (middle, error) {
				current.value += "-second"
				current.steps = append(current.steps, "second")
				return current, nil
			},
		},
	}

	runnable, err := compileLinearGraph(
		context.Background(),
		"registry_test",
		func(_ context.Context, input string) (middle, error) {
			return middle{value: input}, nil
		},
		steps,
		func(_ context.Context, current middle) (string, error) {
			return current.value + ":" + strings.Join(current.steps, ","), nil
		},
	)
	if err != nil {
		t.Fatalf("compileLinearGraph() error = %v", err)
	}

	result, err := runnable.Invoke(context.Background(), "start")
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result != "start-first-second:first,second" {
		t.Fatalf("result = %q", result)
	}
}

func TestCompileLinearGraphSupportsNoMiddleSteps(t *testing.T) {
	runnable, err := compileLinearGraph(
		context.Background(),
		"no_middle_steps",
		func(_ context.Context, input int) (string, error) {
			return strings.Repeat("x", input), nil
		},
		nil,
		func(_ context.Context, middle string) (int, error) {
			return len(middle), nil
		},
	)
	if err != nil {
		t.Fatalf("compileLinearGraph() error = %v", err)
	}

	result, err := runnable.Invoke(context.Background(), 3)
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result != 3 {
		t.Fatalf("result = %d, want 3", result)
	}
}

func TestCompileLinearGraphRejectsInvalidDefinition(t *testing.T) {
	identity := compose.InvokeWOOpt[string, string](func(_ context.Context, input string) (string, error) {
		return input, nil
	})

	tests := []struct {
		name    string
		ctx     context.Context
		graph   string
		input   compose.InvokeWOOpt[string, string]
		steps   []linearStep[string]
		output  compose.InvokeWOOpt[string, string]
		options []compose.NewGraphOption
		want    string
	}{
		{name: "nil context", graph: "graph", input: identity, output: identity, want: "context 不能为空"},
		{name: "empty graph name", ctx: context.Background(), input: identity, output: identity, want: "Graph 名称不能为空"},
		{name: "nil input", ctx: context.Background(), graph: "graph", output: identity, want: "输入适配函数不能为空"},
		{name: "nil output", ctx: context.Background(), graph: "graph", input: identity, want: "输出适配函数不能为空"},
		{name: "empty step key", ctx: context.Background(), graph: "graph", input: identity, output: identity, steps: []linearStep[string]{{Run: identity}}, want: "key 不能为空"},
		{name: "reserved step key", ctx: context.Background(), graph: "graph", input: identity, output: identity, steps: []linearStep[string]{{Key: inputAdapterNode, Run: identity}}, want: "重复或为保留名称"},
		{name: "duplicate step key", ctx: context.Background(), graph: "graph", input: identity, output: identity, steps: []linearStep[string]{{Key: "same", Run: identity}, {Key: "same", Run: identity}}, want: "重复或为保留名称"},
		{name: "nil step handler", ctx: context.Background(), graph: "graph", input: identity, output: identity, steps: []linearStep[string]{{Key: "step"}}, want: "Handler 不能为空"},
		{name: "nil graph option", ctx: context.Background(), graph: "graph", input: identity, output: identity, options: []compose.NewGraphOption{nil}, want: "Graph Option 不能为空"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := compileLinearGraph(
				test.ctx,
				test.graph,
				test.input,
				test.steps,
				test.output,
				test.options...,
			)
			if !errors.Is(err, ErrInvalidLinearGraph) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("compileLinearGraph() error = %v, want ErrInvalidLinearGraph containing %q", err, test.want)
			}
		})
	}
}
