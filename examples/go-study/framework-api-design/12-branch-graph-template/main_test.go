package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudwego/eino/compose"
)

func TestReviewGraphSelectsApproveBranch(t *testing.T) {
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
	if !result.Approved || result.Route != nodeApprove || result.Score != 9 {
		t.Fatalf("result = %#v, want approve branch with score 9", result)
	}
	wantSteps := []string{
		nodeNormalize,
		nodeAppendChannelNotice,
		nodeInspectRefundNotice,
		nodeApprove,
	}
	if !reflect.DeepEqual(result.Steps, wantSteps) {
		t.Fatalf("Steps = %#v, want %#v", result.Steps, wantSteps)
	}
	if contains(result.Steps, nodeManualReview) {
		t.Fatalf("Steps = %#v, unselected branch %q must not run", result.Steps, nodeManualReview)
	}
}

func TestReviewGraphSelectsManualReviewBranch(t *testing.T) {
	runnable, err := NewReviewGraph(context.Background())
	if err != nil {
		t.Fatalf("NewReviewGraph() error = %v", err)
	}

	result, err := runnable.Invoke(context.Background(), ReviewRequest{
		Content: "  您好，请查看退款说明。  ",
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Approved || result.Route != nodeManualReview || result.Score != 5 {
		t.Fatalf("result = %#v, want manual_review branch with score 5", result)
	}
	wantSteps := []string{
		nodeNormalize,
		nodeAppendChannelNotice,
		nodeInspectRefundNotice,
		nodeManualReview,
	}
	if !reflect.DeepEqual(result.Steps, wantSteps) {
		t.Fatalf("Steps = %#v, want %#v", result.Steps, wantSteps)
	}
	if contains(result.Steps, nodeApprove) {
		t.Fatalf("Steps = %#v, unselected branch %q must not run", result.Steps, nodeApprove)
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

func TestBranchPropagatesConditionError(t *testing.T) {
	errRoute := errors.New("路由服务不可用")
	runnable, err := compileDefinedGraph[string, string](
		context.Background(),
		"branch_condition_error",
		func(graph *compose.Graph[string, string]) error {
			if err := addStringNode(graph, "inspect", func(_ context.Context, input string) (string, error) {
				return input, nil
			}); err != nil {
				return err
			}
			if err := addStringNode(graph, "approve", func(_ context.Context, input string) (string, error) {
				return input, nil
			}); err != nil {
				return err
			}
			if err := graph.AddEdge(compose.START, "inspect"); err != nil {
				return err
			}
			if err := graph.AddBranch("inspect", compose.NewGraphBranch(
				func(_ context.Context, _ string) (string, error) {
					return "", errRoute
				},
				map[string]bool{"approve": true},
			)); err != nil {
				return err
			}
			return graph.AddEdge("approve", compose.END)
		},
	)
	if err != nil {
		t.Fatalf("compileDefinedGraph() error = %v", err)
	}

	_, err = runnable.Invoke(context.Background(), "input")
	if !errors.Is(err, errRoute) {
		t.Fatalf("Invoke() error = %v, want errRoute", err)
	}
}

func TestBranchRejectsUnexpectedTargetAtRuntime(t *testing.T) {
	runnable, err := compileDefinedGraph[string, string](
		context.Background(),
		"unexpected_branch_target",
		func(graph *compose.Graph[string, string]) error {
			if err := addStringNode(graph, "inspect", func(_ context.Context, input string) (string, error) {
				return input, nil
			}); err != nil {
				return err
			}
			if err := addStringNode(graph, "approve", func(_ context.Context, input string) (string, error) {
				return input, nil
			}); err != nil {
				return err
			}
			if err := graph.AddEdge(compose.START, "inspect"); err != nil {
				return err
			}
			if err := graph.AddBranch("inspect", compose.NewGraphBranch(
				func(_ context.Context, _ string) (string, error) {
					return "unknown", nil
				},
				map[string]bool{"approve": true},
			)); err != nil {
				return err
			}
			return graph.AddEdge("approve", compose.END)
		},
	)
	if err != nil {
		t.Fatalf("compileDefinedGraph() error = %v", err)
	}

	_, err = runnable.Invoke(context.Background(), "input")
	if err == nil || !strings.Contains(err.Error(), "unintended end node: unknown") {
		t.Fatalf("Invoke() error = %v, want unexpected target error", err)
	}
}

func TestCompileDefinedGraphRejectsInvalidDefinition(t *testing.T) {
	validDefinition := graphDefinition[string, string](func(graph *compose.Graph[string, string]) error {
		return graph.AddEdge(compose.START, compose.END)
	})

	tests := []struct {
		name    string
		ctx     context.Context
		graph   string
		define  graphDefinition[string, string]
		options []compose.NewGraphOption
		want    string
	}{
		{name: "nil context", graph: "graph", define: validDefinition, want: "context 不能为空"},
		{name: "empty graph name", ctx: context.Background(), define: validDefinition, want: "Graph 名称不能为空"},
		{name: "nil definition", ctx: context.Background(), graph: "graph", want: "定义函数不能为空"},
		{name: "nil graph option", ctx: context.Background(), graph: "graph", define: validDefinition, options: []compose.NewGraphOption{nil}, want: "Graph Option 不能为空"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := compileDefinedGraph(
				test.ctx,
				test.graph,
				test.define,
				test.options...,
			)
			if !errors.Is(err, ErrInvalidGraphDefinition) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("compileDefinedGraph() error = %v, want ErrInvalidGraphDefinition containing %q", err, test.want)
			}
		})
	}
}

func addStringNode(
	graph *compose.Graph[string, string],
	key string,
	handler compose.InvokeWOOpt[string, string],
) error {
	return graph.AddLambdaNode(key, compose.InvokableLambda(handler))
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
