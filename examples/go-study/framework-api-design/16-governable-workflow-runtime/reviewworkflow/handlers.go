package reviewworkflow

import (
	"context"
	"fmt"
	"strings"
)

type handlers struct {
	config       Config
	dependencies Dependencies
}

func (h *handlers) normalize(ctx context.Context, request Request) (reviewDraft, error) {
	if err := ctx.Err(); err != nil {
		return reviewDraft{}, fmt.Errorf("规范化审核内容: %w", err)
	}
	content := strings.Join(strings.Fields(request.Content), " ")
	if content == "" {
		return reviewDraft{}, ErrEmptyContent
	}
	return reviewDraft{content: content}, nil
}

func (h *handlers) inspect(ctx context.Context, draft reviewDraft) (reviewDraft, error) {
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
	draft.reason = inspection.Reason
	return draft, nil
}

func (h *handlers) route(ctx context.Context, draft reviewDraft) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("选择审核路径: %w", err)
	}
	if draft.score >= h.config.ApprovalScore {
		return nodeApprove, nil
	}
	return nodeManual, nil
}

func (h *handlers) approve(ctx context.Context, draft reviewDraft) (reviewDecision, error) {
	return h.decide(ctx, draft, true, RouteApproved)
}

func (h *handlers) manual(ctx context.Context, draft reviewDraft) (reviewDecision, error) {
	return h.decide(ctx, draft, false, RouteManualReview)
}

func (*handlers) decide(
	ctx context.Context,
	draft reviewDraft,
	approved bool,
	route string,
) (reviewDecision, error) {
	if err := ctx.Err(); err != nil {
		return reviewDecision{}, fmt.Errorf("生成审核决定: %w", err)
	}
	return reviewDecision{
		approved: approved,
		route:    route,
		content:  draft.content,
		score:    draft.score,
		reason:   draft.reason,
	}, nil
}

func (*handlers) format(ctx context.Context, decision reviewDecision) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, fmt.Errorf("格式化审核结果: %w", err)
	}
	return Result{
		Approved: decision.approved,
		Route:    decision.route,
		Content:  decision.content,
		Score:    decision.score,
		Reason:   decision.reason,
	}, nil
}

// KeywordInspector 是审核工作流的本地示例依赖。
type KeywordInspector struct{}

// NewKeywordInspector 创建本地审核器。
func NewKeywordInspector() *KeywordInspector {
	return &KeywordInspector{}
}

// Inspect 根据退款到账关键词返回确定性结果。
func (*KeywordInspector) Inspect(ctx context.Context, content string) (Inspection, error) {
	if err := ctx.Err(); err != nil {
		return Inspection{}, err
	}
	if strings.Contains(content, "退款") && strings.Contains(content, "到账") {
		return Inspection{Score: 9, Reason: "包含退款到账说明"}, nil
	}
	return Inspection{Score: 4, Reason: "缺少退款到账说明"}, nil
}

var _ Inspector = (*KeywordInspector)(nil)
