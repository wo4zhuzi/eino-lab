package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestReviewPipelineCombinesChainAndGraph(t *testing.T) {
	runnable, err := NewReviewPipeline(context.Background())
	if err != nil {
		t.Fatalf("NewReviewPipeline() error = %v", err)
	}

	tests := []struct {
		name         string
		content      string
		wantAttempts int
		wantSteps    []string
		wantRevised  bool
	}{
		{
			name:         "graph approves directly",
			content:      "  退款将在 3 个工作日到账。  ",
			wantAttempts: 1,
			wantSteps: []string{
				nodeNormalizeRequest,
				nodeInspect,
				nodeApprove,
				nodeFormatResult,
			},
		},
		{
			name:         "graph revises and loops back",
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
			wantRevised: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := runnable.Invoke(context.Background(), ReviewRequest{Content: test.content})
			if err != nil {
				t.Fatalf("Invoke() error = %v", err)
			}
			if !result.Approved || result.Score != 9 || result.Attempts != test.wantAttempts {
				t.Fatalf("result = %#v, want approved score=9 attempts=%d", result, test.wantAttempts)
			}
			if !reflect.DeepEqual(result.Steps, test.wantSteps) {
				t.Fatalf("Steps = %#v, want %#v", result.Steps, test.wantSteps)
			}
			if got := strings.Contains(result.Content, "补充："); got != test.wantRevised {
				t.Fatalf("Content = %q, revised = %t, want %t", result.Content, got, test.wantRevised)
			}
		})
	}
}

func TestReviewPipelinePreservesErrors(t *testing.T) {
	if _, err := NewReviewPipeline(nil); !errors.Is(err, ErrNilContext) {
		t.Fatalf("NewReviewPipeline(nil) error = %v, want ErrNilContext", err)
	}

	runnable, err := NewReviewPipeline(context.Background())
	if err != nil {
		t.Fatalf("NewReviewPipeline() error = %v", err)
	}

	_, err = runnable.Invoke(context.Background(), ReviewRequest{Content: " \n\t "})
	if !errors.Is(err, ErrEmptyContent) {
		t.Fatalf("empty Invoke() error = %v, want ErrEmptyContent", err)
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = runnable.Invoke(canceledCtx, ReviewRequest{Content: "退款到账"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Invoke() error = %v, want context.Canceled", err)
	}
}
