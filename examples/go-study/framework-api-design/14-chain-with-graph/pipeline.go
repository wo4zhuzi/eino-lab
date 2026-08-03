package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"
)

// NewReviewPipeline 创建“外层 Chain + 内层 Graph”的审核流水线。
func NewReviewPipeline(ctx context.Context) (compose.Runnable[ReviewRequest, ReviewResult], error) {
	if ctx == nil {
		return nil, ErrNilContext
	}

	decisionGraph, err := newDecisionGraph()
	if err != nil {
		return nil, fmt.Errorf("创建审核决策 Graph: %w", err)
	}

	// Chain 只描述稳定的线性阶段；需要回环的决策逻辑封装在 decisionGraph 中。
	pipeline := compose.NewChain[ReviewRequest, ReviewResult]()
	pipeline.
		AppendLambda(
			compose.InvokableLambda(normalizeRequest),
			compose.WithNodeKey(nodeNormalizeRequest),
		).
		AppendGraph(
			decisionGraph,
			compose.WithNodeKey(nodeDecisionGraph),
			compose.WithGraphCompileOptions(
				compose.WithGraphName("review_decision_graph"),
				compose.WithMaxRunSteps(8),
			),
		).
		AppendLambda(
			compose.InvokableLambda(formatResult),
			compose.WithNodeKey(nodeFormatResult),
		)

	runnable, err := pipeline.Compile(ctx, compose.WithGraphName("review_chain_with_graph"))
	if err != nil {
		return nil, fmt.Errorf("编译审核组合流水线: %w", err)
	}
	return runnable, nil
}

func normalizeRequest(ctx context.Context, request ReviewRequest) (reviewDraft, error) {
	if err := ctx.Err(); err != nil {
		return reviewDraft{}, fmt.Errorf("规范化审核请求: %w", err)
	}

	content := strings.Join(strings.Fields(request.Content), " ")
	if content == "" {
		return reviewDraft{}, ErrEmptyContent
	}

	return reviewDraft{
		content: content,
		steps:   []string{nodeNormalizeRequest},
	}, nil
}

func formatResult(ctx context.Context, decision reviewDecision) (ReviewResult, error) {
	if err := ctx.Err(); err != nil {
		return ReviewResult{}, fmt.Errorf("格式化审核结果: %w", err)
	}

	steps := append([]string(nil), decision.steps...)
	steps = append(steps, nodeFormatResult)
	return ReviewResult{
		Approved: decision.approved,
		Content:  decision.content,
		Score:    decision.score,
		Attempts: decision.attempts,
		Steps:    steps,
		Summary: fmt.Sprintf(
			"审核通过：score=%d attempts=%d",
			decision.score,
			decision.attempts,
		),
	}, nil
}
