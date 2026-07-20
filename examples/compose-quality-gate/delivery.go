package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

var ErrEmptyCustomerID = errors.New("customer id is empty")

// CustomerReplyDelivery 处理审核完成后的发送或人工复核终态。
type CustomerReplyDelivery interface {
	Deliver(ctx context.Context, customerID string, result ReviewResult) error
}

// simulatedCustomerReplyDelivery 将最终动作输出到指定 Writer，模拟发送和人工队列。
type simulatedCustomerReplyDelivery struct {
	output io.Writer
}

func (d simulatedCustomerReplyDelivery) Deliver(ctx context.Context, customerID string, result ReviewResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if d.output == nil {
		return errors.New("delivery output is required")
	}

	customerID = strings.TrimSpace(customerID)
	if customerID == "" {
		return ErrEmptyCustomerID
	}

	switch result.Status {
	case ReviewApproved:
		if _, err := fmt.Fprintf(d.output, "delivery=sent customer_id=%s reply=%q\n", customerID, result.Content); err != nil {
			return fmt.Errorf("write sent reply: %w", err)
		}
	case ReviewManualReview:
		if _, err := fmt.Fprintf(d.output, "delivery=manual_review_queued customer_id=%s content=%q reason=%q\n", customerID, result.Content, result.Reason); err != nil {
			return fmt.Errorf("write manual review task: %w", err)
		}
	default:
		return fmt.Errorf("unsupported review status: %q", result.Status)
	}
	return nil
}
