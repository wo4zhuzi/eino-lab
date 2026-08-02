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
		wantAudit []string
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
				nodeAttachLocalAudit,
			},
			wantAudit: []string{
				"request_received",
				"review_branch=" + nodeApprove,
				"review_result_recorded",
				"notification_branch=" + nodeSendApprovedNotice,
				"pipeline_completed",
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
				nodeAttachLocalAudit,
			},
			wantAudit: []string{
				"request_received",
				"review_branch=" + nodeManualReview,
				"manual_queue_branch=" + nodeStandardManualQueue,
				"review_result_recorded",
				"notification_branch=" + nodeSendManualNotice,
				"pipeline_completed",
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
				nodeAttachLocalAudit,
			},
			wantAudit: []string{
				"request_received",
				"review_branch=" + nodeManualReview,
				"manual_queue_branch=" + nodePriorityManualQueue,
				"review_result_recorded",
				"notification_branch=" + nodeSendManualNotice,
				"pipeline_completed",
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
			if !reflect.DeepEqual(result.Audit, test.wantAudit) {
				t.Fatalf("Audit = %#v, want %#v", result.Audit, test.wantAudit)
			}
		})
	}
}

func TestReviewPipelineLocalStateIsIsolatedAcrossConcurrentInvokes(t *testing.T) {
	runnable, err := NewReviewPipeline(context.Background())
	if err != nil {
		t.Fatalf("NewReviewPipeline() error = %v", err)
	}

	type invokeResult struct {
		name  string
		audit []string
		err   error
	}
	tests := []struct {
		name      string
		content   string
		wantAudit []string
	}{
		{
			name:    "approve",
			content: "退款将在 3 个工作日到账。",
			wantAudit: []string{
				"request_received",
				"review_branch=" + nodeApprove,
				"review_result_recorded",
				"notification_branch=" + nodeSendApprovedNotice,
				"pipeline_completed",
			},
		},
		{
			name:    "priority manual",
			content: "请查看相关说明。",
			wantAudit: []string{
				"request_received",
				"review_branch=" + nodeManualReview,
				"manual_queue_branch=" + nodePriorityManualQueue,
				"review_result_recorded",
				"notification_branch=" + nodeSendManualNotice,
				"pipeline_completed",
			},
		},
	}

	results := make(chan invokeResult, len(tests))
	for _, test := range tests {
		test := test
		go func() {
			result, err := runnable.Invoke(context.Background(), ReviewRequest{Content: test.content})
			results <- invokeResult{name: test.name, audit: result.Audit, err: err}
		}()
	}

	wantByName := make(map[string][]string, len(tests))
	for _, test := range tests {
		wantByName[test.name] = test.wantAudit
	}
	for range tests {
		result := <-results
		if result.err != nil {
			t.Fatalf("%s Invoke() error = %v", result.name, result.err)
		}
		if !reflect.DeepEqual(result.audit, wantByName[result.name]) {
			t.Fatalf("%s Audit = %#v, want %#v", result.name, result.audit, wantByName[result.name])
		}
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
