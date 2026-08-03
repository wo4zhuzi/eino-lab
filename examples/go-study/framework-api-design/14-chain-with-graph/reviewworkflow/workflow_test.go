package reviewworkflow

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type inspectorFunc func(context.Context, string) (Inspection, error)

func (f inspectorFunc) Inspect(ctx context.Context, content string) (Inspection, error) {
	return f(ctx, content)
}

type reviserFunc func(context.Context, string) (string, error)

func (f reviserFunc) Revise(ctx context.Context, content string) (string, error) {
	return f(ctx, content)
}

func TestWorkflowRunsDirectRevisionAndManualPaths(t *testing.T) {
	reviser, err := NewAppendReviser("补充：退款将在 3 个工作日到账。")
	if err != nil {
		t.Fatalf("NewAppendReviser() error = %v", err)
	}
	workflow, err := New(context.Background(), DefaultConfig(), Dependencies{
		Inspector: NewKeywordInspector(),
		Reviser:   reviser,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		name         string
		content      string
		wantAttempts int
		wantSteps    []string
	}{
		{
			name:         "direct approve",
			content:      "  退款将在 3 个工作日到账。  ",
			wantAttempts: 1,
			wantSteps:    []string{nodeNormalizeRequest, nodeInspect, nodeApprove, nodeFormatResult},
		},
		{
			name:         "revise then approve",
			content:      "  请尽快处理。  ",
			wantAttempts: 2,
			wantSteps: []string{
				nodeNormalizeRequest,
				nodeInspect,
				nodeRevise,
				nodeInspect,
				nodeApprove,
				nodeFormatResult,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := workflow.Run(context.Background(), ReviewRequest{Content: test.content})
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if !result.Approved || result.Route != RouteApproved || result.Attempts != test.wantAttempts {
				t.Fatalf("result = %#v, want approved attempts=%d", result, test.wantAttempts)
			}
			if !reflect.DeepEqual(result.Steps, test.wantSteps) {
				t.Fatalf("Steps = %#v, want %#v", result.Steps, test.wantSteps)
			}
		})
	}

	manualWorkflow, err := New(context.Background(), DefaultConfig(), Dependencies{
		Inspector: inspectorFunc(func(context.Context, string) (Inspection, error) {
			return Inspection{Score: 2, Reason: "仍需人工判断"}, nil
		}),
		Reviser: reviserFunc(func(_ context.Context, content string) (string, error) {
			return content + " 已自动修订。", nil
		}),
	})
	if err != nil {
		t.Fatalf("New() manual workflow error = %v", err)
	}
	result, err := manualWorkflow.Run(context.Background(), ReviewRequest{Content: "请处理"})
	if err != nil {
		t.Fatalf("manual Run() error = %v", err)
	}
	wantSteps := []string{
		nodeNormalizeRequest,
		nodeInspect,
		nodeRevise,
		nodeInspect,
		nodeManualReview,
		nodeFormatResult,
	}
	if result.Approved || result.Route != RouteManualReview || result.Attempts != 2 {
		t.Fatalf("manual result = %#v", result)
	}
	if !reflect.DeepEqual(result.Steps, wantSteps) {
		t.Fatalf("manual Steps = %#v, want %#v", result.Steps, wantSteps)
	}
}

func TestNewValidatesConfigAndDependencies(t *testing.T) {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	var typedNilInspector *KeywordInspector
	validDependencies := Dependencies{
		Inspector: NewKeywordInspector(),
		Reviser: reviserFunc(func(_ context.Context, content string) (string, error) {
			return content, nil
		}),
	}
	tests := []struct {
		name         string
		ctx          context.Context
		config       Config
		dependencies Dependencies
		want         error
	}{
		{name: "nil context", config: DefaultConfig(), dependencies: validDependencies, want: ErrNilContext},
		{name: "canceled context", ctx: canceledCtx, config: DefaultConfig(), dependencies: validDependencies, want: context.Canceled},
		{name: "invalid score", ctx: context.Background(), config: Config{ApprovalScore: 11, MaxAttempts: 2, MaxGraphSteps: 8}, dependencies: validDependencies, want: ErrInvalidConfig},
		{name: "invalid attempts", ctx: context.Background(), config: Config{ApprovalScore: 8, MaxGraphSteps: 8}, dependencies: validDependencies, want: ErrInvalidConfig},
		{name: "insufficient graph steps", ctx: context.Background(), config: Config{ApprovalScore: 8, MaxAttempts: 3, MaxGraphSteps: 5}, dependencies: validDependencies, want: ErrInvalidConfig},
		{name: "nil inspector", ctx: context.Background(), config: DefaultConfig(), dependencies: Dependencies{Reviser: validDependencies.Reviser}, want: ErrInvalidDependencies},
		{name: "typed nil inspector", ctx: context.Background(), config: DefaultConfig(), dependencies: Dependencies{Inspector: typedNilInspector, Reviser: validDependencies.Reviser}, want: ErrInvalidDependencies},
		{name: "nil reviser", ctx: context.Background(), config: DefaultConfig(), dependencies: Dependencies{Inspector: validDependencies.Inspector}, want: ErrInvalidDependencies},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(test.ctx, test.config, test.dependencies)
			if !errors.Is(err, test.want) {
				t.Fatalf("New() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestWorkflowPreservesNodeAndContextErrors(t *testing.T) {
	errInspector := errors.New("审核服务不可用")
	workflow, err := New(context.Background(), DefaultConfig(), Dependencies{
		Inspector: inspectorFunc(func(context.Context, string) (Inspection, error) {
			return Inspection{}, errInspector
		}),
		Reviser: reviserFunc(func(_ context.Context, content string) (string, error) {
			return content, nil
		}),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = workflow.Run(context.Background(), ReviewRequest{Content: "退款到账"})
	if !errors.Is(err, errInspector) {
		t.Fatalf("Run() error = %v, want errInspector", err)
	}
	if _, err := workflow.Run(nil, ReviewRequest{}); !errors.Is(err, ErrNilContext) {
		t.Fatalf("Run(nil) error = %v, want ErrNilContext", err)
	}

	_, err = workflow.Run(context.Background(), ReviewRequest{Content: " \n\t "})
	if !errors.Is(err, ErrEmptyContent) {
		t.Fatalf("empty Run() error = %v, want ErrEmptyContent", err)
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = workflow.Run(canceledCtx, ReviewRequest{Content: "退款到账"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Run() error = %v, want context.Canceled", err)
	}

	var nilWorkflow *Workflow
	if _, err := nilWorkflow.Run(context.Background(), ReviewRequest{}); !errors.Is(err, ErrWorkflowNotInitialized) {
		t.Fatalf("nil Workflow.Run() error = %v, want ErrWorkflowNotInitialized", err)
	}
}

func TestWorkflowCanBeReusedConcurrently(t *testing.T) {
	reviser, err := NewAppendReviser("补充：退款将在 3 个工作日到账。")
	if err != nil {
		t.Fatalf("NewAppendReviser() error = %v", err)
	}
	workflow, err := New(context.Background(), DefaultConfig(), Dependencies{
		Inspector: NewKeywordInspector(),
		Reviser:   reviser,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	const runs = 20
	errCh := make(chan error, runs)
	var wg sync.WaitGroup
	for index := 0; index < runs; index++ {
		index := index
		wg.Add(1)
		go func() {
			defer wg.Done()
			content := "退款将在 3 个工作日到账。"
			wantAttempts := 1
			if index%2 == 1 {
				content = "请尽快处理。"
				wantAttempts = 2
			}
			result, err := workflow.Run(context.Background(), ReviewRequest{Content: content})
			if err != nil {
				errCh <- err
				return
			}
			if result.Attempts != wantAttempts {
				errCh <- fmt.Errorf("attempts = %d, want %d", result.Attempts, wantAttempts)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

func TestWorkflowRejectsInvalidDependencyResults(t *testing.T) {
	errReviser := errors.New("修订服务不可用")
	tests := []struct {
		name      string
		inspector Inspector
		reviser   Reviser
		want      error
	}{
		{
			name: "invalid score",
			inspector: inspectorFunc(func(context.Context, string) (Inspection, error) {
				return Inspection{Score: 99}, nil
			}),
			reviser: reviserFunc(func(_ context.Context, content string) (string, error) {
				return content, nil
			}),
			want: ErrInvalidScore,
		},
		{
			name: "reviser error",
			inspector: inspectorFunc(func(context.Context, string) (Inspection, error) {
				return Inspection{Score: 1}, nil
			}),
			reviser: reviserFunc(func(context.Context, string) (string, error) {
				return "", errReviser
			}),
			want: errReviser,
		},
		{
			name: "empty revision",
			inspector: inspectorFunc(func(context.Context, string) (Inspection, error) {
				return Inspection{Score: 1}, nil
			}),
			reviser: reviserFunc(func(context.Context, string) (string, error) {
				return " ", nil
			}),
			want: ErrEmptyRevision,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workflow, err := New(context.Background(), DefaultConfig(), Dependencies{
				Inspector: test.inspector,
				Reviser:   test.reviser,
			})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			_, err = workflow.Run(context.Background(), ReviewRequest{Content: "待审核"})
			if !errors.Is(err, test.want) {
				t.Fatalf("Run() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestNewAppendReviserRejectsEmptySuffix(t *testing.T) {
	_, err := NewAppendReviser(" \t ")
	if !errors.Is(err, ErrInvalidDependencies) || !strings.Contains(err.Error(), "修订后缀") {
		t.Fatalf("NewAppendReviser() error = %v", err)
	}
}
