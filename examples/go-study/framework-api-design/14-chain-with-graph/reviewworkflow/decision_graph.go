package reviewworkflow

import (
	"fmt"

	"github.com/cloudwego/eino/compose"
)

// buildDecisionGraph 只负责局部非线性拓扑。
func buildDecisionGraph(handlers *workflowHandlers) (*compose.Graph[reviewDraft, reviewDecision], error) {
	graph := compose.NewGraph[reviewDraft, reviewDecision]()

	if err := addDecisionNode(graph, nodeInspect, handlers.inspectDraft); err != nil {
		return nil, err
	}
	if err := addDecisionNode(graph, nodeRevise, handlers.reviseDraft); err != nil {
		return nil, err
	}
	if err := addDecisionNode(graph, nodeApprove, handlers.approveDraft); err != nil {
		return nil, err
	}
	if err := addDecisionNode(graph, nodeManualReview, handlers.manualReviewDraft); err != nil {
		return nil, err
	}

	if err := graph.AddEdge(compose.START, nodeInspect); err != nil {
		return nil, fmt.Errorf("添加边 START -> %s: %w", nodeInspect, err)
	}
	branch := compose.NewGraphBranch(
		handlers.routeInspection,
		map[string]bool{
			nodeRevise:       true,
			nodeApprove:      true,
			nodeManualReview: true,
		},
	)
	if err := graph.AddBranch(nodeInspect, branch); err != nil {
		return nil, fmt.Errorf("在节点 %s 后添加分支: %w", nodeInspect, err)
	}
	if err := graph.AddEdge(nodeRevise, nodeInspect); err != nil {
		return nil, fmt.Errorf("添加回环边 %s -> %s: %w", nodeRevise, nodeInspect, err)
	}
	for _, node := range []string{nodeApprove, nodeManualReview} {
		if err := graph.AddEdge(node, compose.END); err != nil {
			return nil, fmt.Errorf("添加边 %s -> END: %w", node, err)
		}
	}

	return graph, nil
}

func addDecisionNode[I, O any](
	graph *compose.Graph[reviewDraft, reviewDecision],
	key string,
	handler compose.InvokeWOOpt[I, O],
) error {
	if err := graph.AddLambdaNode(
		key,
		compose.InvokableLambda(handler),
		compose.WithNodeName(key),
	); err != nil {
		return fmt.Errorf("添加节点 %q: %w", key, err)
	}
	return nil
}
