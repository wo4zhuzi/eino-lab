package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestReviewPipelineRunsOnlySelectedPath(t *testing.T) {
	runnable, err := NewReviewPipeline(context.Background())
	if err != nil {
		t.Fatalf("NewReviewPipeline() error = %v", err)
	}

	tests := []struct {
		name      string
		content   string
		approved  bool
		score     int
		wantSteps []string
	}{
		{
			name:     "approve path",
			content:  "  您好，退款将在 3 个工作日到账。  ",
			approved: true,
			score:    9,
			wantSteps: []string{
				nodeNormalize,
				nodeAppendChannelNotice,
				nodeInspectRefundNotice,
				nodeApprove,
				nodeArchiveApproved,
				nodeRecordReviewResult,
				nodeSendApprovedNotice,
			},
		},
		{
			name:    "standard manual queue path",
			content: "  您好，请查看退款说明。  ",
			score:   5,
			wantSteps: []string{
				nodeNormalize,
				nodeAppendChannelNotice,
				nodeInspectRefundNotice,
				nodeManualReview,
				nodeStandardManualQueue,
				nodeRecordReviewResult,
				nodeSendManualNotice,
			},
		},
		{
			name:    "priority manual queue path",
			content: "  您好，请查看相关说明。  ",
			score:   3,
			wantSteps: []string{
				nodeNormalize,
				nodeAppendChannelNotice,
				nodeInspectRefundNotice,
				nodeManualReview,
				nodePriorityManualQueue,
				nodeRecordReviewResult,
				nodeSendManualNotice,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := runnable.Invoke(context.Background(), ReviewRequest{Content: test.content})
			if err != nil {
				t.Fatalf("Invoke() error = %v", err)
			}
			if result.Approved != test.approved || result.Score != test.score {
				t.Fatalf("result = %#v, want approved=%t score=%d", result, test.approved, test.score)
			}
			if !reflect.DeepEqual(result.Steps, test.wantSteps) {
				t.Fatalf("Steps = %#v, want %#v", result.Steps, test.wantSteps)
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
	_, err = runnable.Invoke(canceledCtx, ReviewRequest{Content: "退款到账说明"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Invoke() error = %v, want context.Canceled", err)
	}
}

func TestRoutesPreserveContextCancellation(t *testing.T) {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := routeReview(canceledCtx, reviewContext{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("routeReview() error = %v, want context.Canceled", err)
	}
	if _, err := routeManualQueue(canceledCtx, ReviewResult{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("routeManualQueue() error = %v, want context.Canceled", err)
	}
	if _, err := routeNotification(canceledCtx, ReviewResult{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("routeNotification() error = %v, want context.Canceled", err)
	}
}
