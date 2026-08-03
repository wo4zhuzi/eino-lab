package ragworkflow

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/wo4zhuzi/eino-lab/examples/go-study/framework-api-design/15-multiple-workflows/workflowkit"
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
		steps      []string
	}{
		{
			name:     "direct evidence",
			question: "Eino 如何编排？",
			attempts: 1,
			steps: []string{
				nodeNormalize,
				nodeRetrieve,
				nodeEvidenceReady,
				nodeGenerate,
				nodeFormat,
			},
		},
		{
			name:     "rewrite then evidence",
			question: "如何编排？",
			attempts: 2,
			steps: []string{
				nodeNormalize,
				nodeRetrieve,
				nodeRewrite,
				nodeRetrieve,
				nodeEvidenceReady,
				nodeGenerate,
				nodeFormat,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := workflow.Run(context.Background(), Request{Question: test.question})
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if result.RetrievalAttempts != test.attempts || result.NoEvidence != test.noEvidence {
				t.Fatalf("result = %#v", result)
			}
			if !reflect.DeepEqual(result.Steps, test.steps) {
				t.Fatalf("Steps = %#v, want %#v", result.Steps, test.steps)
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
	result, err := noEvidenceWorkflow.Run(context.Background(), Request{Question: "今天天气如何？"})
	if err != nil {
		t.Fatalf("no-evidence Run() error = %v", err)
	}
	wantSteps := []string{
		nodeNormalize,
		nodeRetrieve,
		nodeRewrite,
		nodeRetrieve,
		nodeNoEvidence,
		nodeGenerate,
		nodeFormat,
	}
	if !result.NoEvidence || result.RetrievalAttempts != 2 {
		t.Fatalf("no-evidence result = %#v", result)
	}
	if !reflect.DeepEqual(result.Steps, wantSteps) {
		t.Fatalf("no-evidence Steps = %#v, want %#v", result.Steps, wantSteps)
	}
}

func TestWorkflowValidatesAndPreservesErrors(t *testing.T) {
	validDependencies := Dependencies{
		Retriever: NewMemoryRetriever(nil),
		Generator: NewCitationGenerator(),
	}
	if _, err := New(context.Background(), Config{}, validDependencies); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("New() error = %v, want ErrInvalidConfig", err)
	}
	if _, err := New(context.Background(), DefaultConfig(), Dependencies{}); !errors.Is(err, workflowkit.ErrNilDependency) {
		t.Fatalf("New() error = %v, want ErrNilDependency", err)
	}

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
	_, err = workflow.Run(context.Background(), Request{Question: "问题"})
	if !errors.Is(err, errRetriever) {
		t.Fatalf("Run() error = %v, want errRetriever", err)
	}

	_, err = workflow.Run(context.Background(), Request{Question: " \n\t "})
	if !errors.Is(err, ErrEmptyQuestion) {
		t.Fatalf("Run() error = %v, want ErrEmptyQuestion", err)
	}

	errGenerator := errors.New("generator unavailable")
	generatorWorkflow, err := New(context.Background(), DefaultConfig(), Dependencies{
		Retriever: retrieverFunc(func(context.Context, string) ([]string, error) {
			return []string{"证据"}, nil
		}),
		Generator: generatorFunc(func(context.Context, string, []string) (string, error) {
			return "", errGenerator
		}),
	})
	if err != nil {
		t.Fatalf("New() generator workflow error = %v", err)
	}
	_, err = generatorWorkflow.Run(context.Background(), Request{Question: "问题"})
	if !errors.Is(err, errGenerator) {
		t.Fatalf("Run() error = %v, want errGenerator", err)
	}

	emptyAnswerWorkflow, err := New(context.Background(), DefaultConfig(), Dependencies{
		Retriever: retrieverFunc(func(context.Context, string) ([]string, error) {
			return []string{"证据"}, nil
		}),
		Generator: generatorFunc(func(context.Context, string, []string) (string, error) {
			return " ", nil
		}),
	})
	if err != nil {
		t.Fatalf("New() empty-answer workflow error = %v", err)
	}
	_, err = emptyAnswerWorkflow.Run(context.Background(), Request{Question: "问题"})
	if !errors.Is(err, ErrEmptyAnswer) {
		t.Fatalf("Run() error = %v, want ErrEmptyAnswer", err)
	}
}
