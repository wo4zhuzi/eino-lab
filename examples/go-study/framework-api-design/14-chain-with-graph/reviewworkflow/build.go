package reviewworkflow

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"
)

const (
	nodeNormalizeRequest = "normalize_request"
	nodeDecisionGraph    = "decision_graph"
	nodeInspect          = "inspect"
	nodeRevise           = "revise"
	nodeApprove          = "approve"
	nodeManualReview     = "manual_review"
	nodeFormatResult     = "format_result"
)

// buildPipeline 只负责外层线性拓扑，不包含节点业务实现。
func buildPipeline(
	ctx context.Context,
	config Config,
	dependencies Dependencies,
) (compose.Runnable[ReviewRequest, ReviewResult], error) {
	handlers := newHandlers(config, dependencies)
	decisionGraph, err := buildDecisionGraph(handlers)
	if err != nil {
		return nil, fmt.Errorf("创建审核决策 Graph: %w", err)
	}

	pipeline := compose.NewChain[ReviewRequest, ReviewResult]()
	pipeline.
		AppendLambda(
			compose.InvokableLambda(handlers.normalizeRequest),
			compose.WithNodeKey(nodeNormalizeRequest),
		).
		AppendGraph(
			decisionGraph,
			compose.WithNodeKey(nodeDecisionGraph),
			compose.WithGraphCompileOptions(
				compose.WithGraphName("review_decision_graph"),
				compose.WithMaxRunSteps(config.MaxGraphSteps),
			),
		).
		AppendLambda(
			compose.InvokableLambda(handlers.formatResult),
			compose.WithNodeKey(nodeFormatResult),
		)

	runnable, err := pipeline.Compile(ctx, compose.WithGraphName("review_workflow"))
	if err != nil {
		return nil, fmt.Errorf("编译审核 Chain: %w", err)
	}
	return runnable, nil
}
