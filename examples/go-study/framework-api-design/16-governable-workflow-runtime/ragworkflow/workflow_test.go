package ragworkflow

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/compose"
	"github.com/wo4zhuzi/eino-lab/examples/go-study/framework-api-design/16-governable-workflow-runtime/workflowkit"
)

type retrieverFunc func(context.Context, string) ([]string, error)

func (f retrieverFunc) Retrieve(ctx context.Context, query string) ([]string, error) {
	return f(ctx, query)
}

type generatorFunc func(context.Context, string, []string) (string, error)

func (f generatorFunc) Generate(ctx context.Context, question string, evidence []string) (string, error) {
	return f(ctx, question, evidence)
}

func TestWorkflowRetrievesRewritesAndFallsBack(t *testing.T) {
	workflow, err := New(context.Background(), DefaultConfig(), Dependencies{
		Retriever: NewMemoryRetriever(map[string][]string{
			"Eino": {"Eino Compose 支持 Chain 和 Graph。"},
		}),
		Generator: NewCitationGenerator(),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	tests := []struct {
		name       string
		question   string
		attempts   int
		noEvidence bool
		wantNode   string
	}{
		{name: "direct evidence", question: "Eino 如何编排？", attempts: 1, wantNode: nodeEvidenceReady},
		{name: "rewrite then evidence", question: "如何编排？", attempts: 2, wantNode: nodeRewrite},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := workflowkit.NewRecorder()
			result, err := workflow.Run(
				context.Background(),
				Request{Question: test.question},
				workflowkit.WithRunID("rag-"+test.name),
				workflowkit.WithObserver(recorder),
			)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if result.RetrievalAttempts != test.attempts || result.NoEvidence != test.noEvidence {
				t.Fatalf("result = %#v", result)
			}
			if !containsSuccessfulNode(recorder.Events(), test.wantNode) {
				t.Fatalf("events do not contain %q: %#v", test.wantNode, recorder.Events())
			}
		})
	}

	noEvidenceWorkflow, err := New(context.Background(), DefaultConfig(), Dependencies{
		Retriever: NewMemoryRetriever(nil),
		Generator: NewCitationGenerator(),
	})
	if err != nil {
		t.Fatalf("New() no-evidence workflow error = %v", err)
	}
	result, err := noEvidenceWorkflow.Run(
		context.Background(),
		Request{Question: "今天天气如何？"},
		workflowkit.WithRunID("rag-no-evidence"),
	)
	if err != nil {
		t.Fatalf("no-evidence Run() error = %v", err)
	}
	if !result.NoEvidence || result.RetrievalAttempts != 2 {
		t.Fatalf("no-evidence result = %#v", result)
	}
}

func TestWorkflowRuntimeOptionsAndErrors(t *testing.T) {
	errRetriever := errors.New("retriever unavailable")
	workflow, err := New(context.Background(), DefaultConfig(), Dependencies{
		Retriever: retrieverFunc(func(context.Context, string) ([]string, error) {
			return nil, errRetriever
		}),
		Generator: NewCitationGenerator(),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = workflow.Run(
		context.Background(),
		Request{Question: "问题"},
		workflowkit.WithRunID("rag-error"),
	)
	if !errors.Is(err, errRetriever) {
		t.Fatalf("Run() error = %v, want errRetriever", err)
	}
	var operationError *workflowkit.OperationError
	if !errors.As(err, &operationError) ||
		operationError.Execution.Descriptor != Descriptor() ||
		operationError.Execution.RunID != "rag-error" {
		t.Fatalf("OperationError = %#v", operationError)
	}

	working, err := New(context.Background(), DefaultConfig(), Dependencies{
		Retriever: NewMemoryRetriever(nil),
		Generator: NewCitationGenerator(),
	})
	if err != nil {
		t.Fatalf("New() working workflow error = %v", err)
	}
	_, err = working.Run(
		context.Background(),
		Request{Question: "问题"},
		workflowkit.WithRunID("limited"),
		workflowkit.WithRuntimeMaxSteps(1),
	)
	if !errors.Is(err, compose.ErrExceedMaxSteps) {
		t.Fatalf("Run(max steps) error = %v, want ErrExceedMaxSteps", err)
	}

	_, err = working.Run(
		context.Background(),
		Request{Question: " "},
		workflowkit.WithRunID("empty-question"),
	)
	if !errors.Is(err, ErrEmptyQuestion) {
		t.Fatalf("Run(empty) error = %v", err)
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
