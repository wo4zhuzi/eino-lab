package reviewworkflow

import (
	"context"
	"errors"
	"testing"

	"github.com/wo4zhuzi/eino-lab/examples/go-study/framework-api-design/16-governable-workflow-runtime/workflowkit"
)

type inspectorFunc func(context.Context, string) (Inspection, error)

func (f inspectorFunc) Inspect(ctx context.Context, content string) (Inspection, error) {
	return f(ctx, content)
}

func TestWorkflowSelectsPathsAndReportsEvents(t *testing.T) {
	workflow, err := New(context.Background(), DefaultConfig(), Dependencies{Inspector: NewKeywordInspector()})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	tests := []struct {
		name     string
		content  string
		approved bool
		route    string
		node     string
	}{
		{name: "approved", content: "退款将在 3 个工作日到账。", approved: true, route: RouteApproved, node: nodeApprove},
		{name: "manual", content: "请处理这段内容。", route: RouteManualReview, node: nodeManual},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := workflowkit.NewRecorder()
			result, err := workflow.Run(
				context.Background(),
				Request{Content: test.content},
				workflowkit.WithRunID("review-"+test.name),
				workflowkit.WithObserver(recorder),
			)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if result.Approved != test.approved || result.Route != test.route {
				t.Fatalf("result = %#v", result)
			}
			if !containsSuccessfulNode(recorder.Events(), test.node) {
				t.Fatalf("events do not contain %q: %#v", test.node, recorder.Events())
			}
			for _, event := range recorder.Events() {
				if event.Execution.Descriptor != Descriptor() || event.Execution.RunID != "review-"+test.name {
					t.Fatalf("event execution = %#v", event.Execution)
				}
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
	_, err = workflow.Run(
		context.Background(),
		Request{Content: "待审核"},
		workflowkit.WithRunID("review-error"),
	)
	if !errors.Is(err, errInspector) {
		t.Fatalf("Run() error = %v, want errInspector", err)
	}
	var operationError *workflowkit.OperationError
	if !errors.As(err, &operationError) ||
		operationError.Execution.Descriptor != Descriptor() ||
		operationError.Execution.RunID != "review-error" {
		t.Fatalf("OperationError = %#v", operationError)
	}

	_, err = workflow.Run(context.Background(), Request{Content: " "})
	if !errors.Is(err, workflowkit.ErrInvalidRunID) {
		t.Fatalf("Run() without RunID error = %v", err)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = workflow.Run(canceled, Request{Content: "待审核"}, workflowkit.WithRunID("canceled"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run(canceled) error = %v", err)
	}
}

func containsSuccessfulNode(events []workflowkit.Event, node string) bool {
	for _, event := range events {
		if event.Name == node && event.Status == workflowkit.StatusSucceeded {
			return true
		}
	}
	return false
}
