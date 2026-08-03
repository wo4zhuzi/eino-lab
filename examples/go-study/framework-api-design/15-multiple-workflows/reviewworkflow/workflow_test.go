package reviewworkflow

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/wo4zhuzi/eino-lab/examples/go-study/framework-api-design/15-multiple-workflows/workflowkit"
)

type inspectorFunc func(context.Context, string) (Inspection, error)

func (f inspectorFunc) Inspect(ctx context.Context, content string) (Inspection, error) {
	return f(ctx, content)
}

func TestWorkflowSelectsApprovedAndManualPaths(t *testing.T) {
	workflow, err := New(context.Background(), DefaultConfig(), Dependencies{Inspector: NewKeywordInspector()})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	tests := []struct {
		name     string
		content  string
		approved bool
		route    string
		steps    []string
	}{
		{
			name:     "approved",
			content:  "退款将在 3 个工作日到账。",
			approved: true,
			route:    RouteApproved,
			steps:    []string{nodeNormalize, nodeInspect, nodeApprove, nodeFormat},
		},
		{
			name:    "manual",
			content: "请处理这段内容。",
			route:   RouteManualReview,
			steps:   []string{nodeNormalize, nodeInspect, nodeManual, nodeFormat},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := workflow.Run(context.Background(), Request{Content: test.content})
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if result.Approved != test.approved || result.Route != test.route {
				t.Fatalf("result = %#v", result)
			}
			if !reflect.DeepEqual(result.Steps, test.steps) {
				t.Fatalf("Steps = %#v, want %#v", result.Steps, test.steps)
			}
		})
	}
}

func TestWorkflowValidatesAndPreservesErrors(t *testing.T) {
	if _, err := New(context.Background(), Config{}, Dependencies{Inspector: NewKeywordInspector()}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("New() error = %v, want ErrInvalidConfig", err)
	}
	if _, err := New(context.Background(), DefaultConfig(), Dependencies{}); !errors.Is(err, workflowkit.ErrNilDependency) {
		t.Fatalf("New() error = %v, want ErrNilDependency", err)
	}

	errInspector := errors.New("inspector unavailable")
	workflow, err := New(context.Background(), DefaultConfig(), Dependencies{
		Inspector: inspectorFunc(func(context.Context, string) (Inspection, error) {
			return Inspection{}, errInspector
		}),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = workflow.Run(context.Background(), Request{Content: "待审核"})
	if !errors.Is(err, errInspector) {
		t.Fatalf("Run() error = %v, want errInspector", err)
	}

	invalidScoreWorkflow, err := New(context.Background(), DefaultConfig(), Dependencies{
		Inspector: inspectorFunc(func(context.Context, string) (Inspection, error) {
			return Inspection{Score: 99}, nil
		}),
	})
	if err != nil {
		t.Fatalf("New() invalid-score workflow error = %v", err)
	}
	_, err = invalidScoreWorkflow.Run(context.Background(), Request{Content: "待审核"})
	if !errors.Is(err, ErrInvalidScore) {
		t.Fatalf("Run() error = %v, want ErrInvalidScore", err)
	}
}
