package reviewworkflow

import (
	"context"
	"fmt"
	"strings"
)

type workflowHandlers struct {
	config       Config
	dependencies Dependencies
}

func newHandlers(config Config, dependencies Dependencies) *workflowHandlers {
	return &workflowHandlers{config: config, dependencies: dependencies}
}

func (h *workflowHandlers) normalizeRequest(
	ctx context.Context,
	request ReviewRequest,
) (reviewDraft, error) {
	if err := ctx.Err(); err != nil {
		return reviewDraft{}, fmt.Errorf("规范化审核请求: %w", err)
	}
	content := strings.Join(strings.Fields(request.Content), " ")
	if content == "" {
		return reviewDraft{}, ErrEmptyContent
	}
	return reviewDraft{content: content, steps: []string{nodeNormalizeRequest}}, nil
}

func (h *workflowHandlers) inspectDraft(
	ctx context.Context,
	draft reviewDraft,
) (reviewDraft, error) {
	if err := ctx.Err(); err != nil {
		return reviewDraft{}, fmt.Errorf("检查审核内容: %w", err)
	}
	inspection, err := h.dependencies.Inspector.Inspect(ctx, draft.content)
	if err != nil {
		return reviewDraft{}, fmt.Errorf("调用 Inspector: %w", err)
	}
	if inspection.Score < 0 || inspection.Score > 10 {
		return reviewDraft{}, fmt.Errorf("%w: %d", ErrInvalidScore, inspection.Score)
	}

	draft.score = inspection.Score
	draft.attempts++
	draft.reasons = append(draft.reasons, inspection.Reason)
	draft.steps = append(draft.steps, nodeInspect)
	return draft, nil
}

func (h *workflowHandlers) routeInspection(
	ctx context.Context,
	draft reviewDraft,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("选择审核路径: %w", err)
	}
	if draft.score >= h.config.ApprovalScore {
		return nodeApprove, nil
	}
	if draft.attempts >= h.config.MaxAttempts {
		return nodeManualReview, nil
	}
	return nodeRevise, nil
}

func (h *workflowHandlers) reviseDraft(
	ctx context.Context,
	draft reviewDraft,
) (reviewDraft, error) {
	if err := ctx.Err(); err != nil {
		return reviewDraft{}, fmt.Errorf("修订审核内容: %w", err)
	}
	content, err := h.dependencies.Reviser.Revise(ctx, draft.content)
	if err != nil {
		return reviewDraft{}, fmt.Errorf("调用 Reviser: %w", err)
	}
	if strings.TrimSpace(content) == "" {
		return reviewDraft{}, ErrEmptyRevision
	}

	draft.content = content
	draft.steps = append(draft.steps, nodeRevise)
	return draft, nil
}

func (h *workflowHandlers) approveDraft(
	ctx context.Context,
	draft reviewDraft,
) (reviewDecision, error) {
	if err := ctx.Err(); err != nil {
		return reviewDecision{}, fmt.Errorf("生成通过决定: %w", err)
	}
	return decisionFromDraft(draft, true, RouteApproved, nodeApprove), nil
}

func (h *workflowHandlers) manualReviewDraft(
	ctx context.Context,
	draft reviewDraft,
) (reviewDecision, error) {
	if err := ctx.Err(); err != nil {
		return reviewDecision{}, fmt.Errorf("生成人工审核决定: %w", err)
	}
	return decisionFromDraft(draft, false, RouteManualReview, nodeManualReview), nil
}

func (h *workflowHandlers) formatResult(
	ctx context.Context,
	decision reviewDecision,
) (ReviewResult, error) {
	if err := ctx.Err(); err != nil {
		return ReviewResult{}, fmt.Errorf("格式化审核结果: %w", err)
	}

	steps := append([]string(nil), decision.steps...)
	steps = append(steps, nodeFormatResult)
	summary := fmt.Sprintf("转人工审核：score=%d attempts=%d", decision.score, decision.attempts)
	if decision.approved {
		summary = fmt.Sprintf("审核通过：score=%d attempts=%d", decision.score, decision.attempts)
	}
	return ReviewResult{
		Approved: decision.approved,
		Route:    decision.route,
		Content:  decision.content,
		Score:    decision.score,
		Attempts: decision.attempts,
		Reasons:  append([]string(nil), decision.reasons...),
		Steps:    steps,
		Summary:  summary,
	}, nil
}

func decisionFromDraft(
	draft reviewDraft,
	approved bool,
	route string,
	node string,
) reviewDecision {
	steps := append([]string(nil), draft.steps...)
	steps = append(steps, node)
	return reviewDecision{
		approved: approved,
		route:    route,
		content:  draft.content,
		score:    draft.score,
		attempts: draft.attempts,
		reasons:  append([]string(nil), draft.reasons...),
		steps:    steps,
	}
}
