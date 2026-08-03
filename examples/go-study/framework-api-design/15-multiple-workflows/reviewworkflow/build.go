package reviewworkflow

import (
	"fmt"

	"github.com/cloudwego/eino/compose"
)

const (
	nodeNormalize = "normalize_review"
	nodeDecision  = "review_decision_graph"
	nodeInspect   = "inspect_review"
	nodeApprove   = "approve_review"
	nodeManual    = "manual_review"
	nodeFormat    = "format_review_result"
)

func build(config Config, dependencies Dependencies) (*compose.Chain[Request, Result], error) {
	handlers := &handlers{config: config, dependencies: dependencies}
	decisionGraph, err := buildDecisionGraph(handlers)
	if err != nil {
		return nil, err
	}

	chain := compose.NewChain[Request, Result]()
	chain.
		AppendLambda(compose.InvokableLambda(handlers.normalize), compose.WithNodeKey(nodeNormalize)).
		AppendGraph(
			decisionGraph,
			compose.WithNodeKey(nodeDecision),
			compose.WithGraphCompileOptions(compose.WithGraphName(nodeDecision)),
		).
		AppendLambda(compose.InvokableLambda(handlers.format), compose.WithNodeKey(nodeFormat))
	return chain, nil
}

func buildDecisionGraph(handlers *handlers) (*compose.Graph[reviewDraft, reviewDecision], error) {
	graph := compose.NewGraph[reviewDraft, reviewDecision]()
	if err := graph.AddLambdaNode(nodeInspect, compose.InvokableLambda(handlers.inspect)); err != nil {
		return nil, fmt.Errorf("添加节点 %s: %w", nodeInspect, err)
	}
	if err := graph.AddLambdaNode(nodeApprove, compose.InvokableLambda(handlers.approve)); err != nil {
		return nil, fmt.Errorf("添加节点 %s: %w", nodeApprove, err)
	}
	if err := graph.AddLambdaNode(nodeManual, compose.InvokableLambda(handlers.manual)); err != nil {
		return nil, fmt.Errorf("添加节点 %s: %w", nodeManual, err)
	}
	if err := graph.AddEdge(compose.START, nodeInspect); err != nil {
		return nil, fmt.Errorf("添加边 START -> %s: %w", nodeInspect, err)
	}
	branch := compose.NewGraphBranch(
		handlers.route,
		map[string]bool{nodeApprove: true, nodeManual: true},
	)
	if err := graph.AddBranch(nodeInspect, branch); err != nil {
		return nil, fmt.Errorf("在节点 %s 后添加分支: %w", nodeInspect, err)
	}
	for _, node := range []string{nodeApprove, nodeManual} {
		if err := graph.AddEdge(node, compose.END); err != nil {
			return nil, fmt.Errorf("添加边 %s -> END: %w", node, err)
		}
	}
	return graph, nil
}
