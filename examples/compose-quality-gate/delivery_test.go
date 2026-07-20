package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

type failingWriter struct {
	err error
}

func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestSimulatedCustomerReplyDeliverySendsApprovedReply(t *testing.T) {
	var output bytes.Buffer
	delivery := simulatedCustomerReplyDelivery{output: &output}

	err := delivery.Deliver(context.Background(), "customer-001", ReviewResult{
		Status:  ReviewApproved,
		Content: "审核通过的客服回复",
	})
	if err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	if got := output.String(); !strings.Contains(got, "delivery=sent") || !strings.Contains(got, "审核通过的客服回复") {
		t.Fatalf("Deliver() output = %q, want sent reply", got)
	}
}

func TestSimulatedCustomerReplyDeliveryQueuesManualReview(t *testing.T) {
	var output bytes.Buffer
	delivery := simulatedCustomerReplyDelivery{output: &output}

	err := delivery.Deliver(context.Background(), "customer-001", ReviewResult{
		Status:  ReviewManualReview,
		Content: "需要人工处理的客服回复",
		Reason:  "多次审核未通过",
	})
	if err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	if got := output.String(); !strings.Contains(got, "delivery=manual_review_queued") || !strings.Contains(got, "需要人工处理的客服回复") {
		t.Fatalf("Deliver() output = %q, want manual review task", got)
	}
}

func TestSimulatedCustomerReplyDeliveryValidatesInput(t *testing.T) {
	delivery := simulatedCustomerReplyDelivery{output: &bytes.Buffer{}}

	if err := delivery.Deliver(context.Background(), " ", ReviewResult{Status: ReviewApproved}); !errors.Is(err, ErrEmptyCustomerID) {
		t.Fatalf("Deliver() error = %v, want ErrEmptyCustomerID", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := delivery.Deliver(ctx, "customer-001", ReviewResult{Status: ReviewApproved}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Deliver() canceled error = %v, want context.Canceled", err)
	}
}

func TestSimulatedCustomerReplyDeliveryPropagatesWriteFailure(t *testing.T) {
	wantErr := errors.New("output unavailable")
	delivery := simulatedCustomerReplyDelivery{output: failingWriter{err: wantErr}}

	err := delivery.Deliver(context.Background(), "customer-001", ReviewResult{
		Status:  ReviewApproved,
		Content: "审核通过的客服回复",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Deliver() error = %v, want output failure", err)
	}
}
