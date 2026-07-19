package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"
)

type gateNodes struct {
	inspector Inspector
}

func (n *gateNodes) validate(ctx context.Context, request ReviewRequest) (gatePayload, error) {
	if err := ctx.Err(); err != nil {
		return gatePayload{}, err
	}
	content := strings.TrimSpace(request.Content)
	if content == "" {
		return gatePayload{}, ErrEmptyContent
	}
	return gatePayload{Content: content}, nil
}

func (n *gateNodes) inspect(ctx context.Context, payload gatePayload) (gatePayload, error) {
	if err := ctx.Err(); err != nil {
		return gatePayload{}, err
	}
	result, err := n.inspector.Inspect(ctx, payload.Content)
	if err != nil {
		return gatePayload{}, fmt.Errorf("inspect content: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return gatePayload{}, err
	}
	if result.Score < 0 || result.Score > 10 {
		return gatePayload{}, fmt.Errorf("%w: score %d is outside [0,10]", ErrInvalidInspection, result.Score)
	}

	payload.Score = result.Score
	payload.Reason = result.Reason
	if err := compose.ProcessState[*gateState](ctx, func(_ context.Context, state *gateState) error {
		state.Attempts++
		state.Audit = append(state.Audit, AuditEntry{
			Attempt: state.Attempts,
			Score:   result.Score,
			Reason:  result.Reason,
		})
		return nil
	}); err != nil {
		return gatePayload{}, fmt.Errorf("record inspection state: %w", err)
	}
	return payload, nil
}

func (n *gateNodes) remediate(ctx context.Context, payload gatePayload) (gatePayload, error) {
	if err := ctx.Err(); err != nil {
		return gatePayload{}, err
	}
	payload.Content = strings.TrimSpace(payload.Content) + "\n[remediated]"
	return payload, nil
}

func (n *gateNodes) approve(ctx context.Context, payload gatePayload) (ReviewResult, error) {
	return n.finalize(ctx, payload, ReviewApproved)
}

func (n *gateNodes) manual(ctx context.Context, payload gatePayload) (ReviewResult, error) {
	return n.finalize(ctx, payload, ReviewManualReview)
}

func (n *gateNodes) finalize(ctx context.Context, payload gatePayload, status ReviewStatus) (ReviewResult, error) {
	if err := ctx.Err(); err != nil {
		return ReviewResult{}, err
	}

	var attempts int
	var audit []AuditEntry
	if err := compose.ProcessState[*gateState](ctx, func(_ context.Context, state *gateState) error {
		attempts = state.Attempts
		audit = append([]AuditEntry(nil), state.Audit...)
		return nil
	}); err != nil {
		return ReviewResult{}, fmt.Errorf("read final state: %w", err)
	}

	return ReviewResult{
		Status:   status,
		Content:  payload.Content,
		Score:    payload.Score,
		Reason:   payload.Reason,
		Attempts: attempts,
		Audit:    audit,
	}, nil
}
