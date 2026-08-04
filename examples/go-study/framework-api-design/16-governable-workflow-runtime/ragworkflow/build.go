package ragworkflow

import (
	"fmt"

	"github.com/cloudwego/eino/compose"
)

const (
	nodeNormalize      = "normalize_question"
	nodeRetrievalGraph = "retrieval_graph"
	nodeRetrieve       = "retrieve"
	nodeRewrite        = "rewrite_query"
	nodeEvidenceReady  = "evidence_ready"
	nodeNoEvidence     = "no_evidence"
	nodeGenerate       = "generate_answer"
	nodeFormat         = "format_rag_result"
)

func build(config Config, dependencies Dependencies) (*compose.Chain[Request, Result], error) {
	handlers := &handlers{config: config, dependencies: dependencies}
	retrievalGraph, err := buildRetrievalGraph(handlers)
	if err != nil {
		return nil, err
	}

	chain := compose.NewChain[Request, Result]()
	chain.
		AppendLambda(
			compose.InvokableLambda(handlers.normalize),
			compose.WithNodeKey(nodeNormalize),
			compose.WithNodeName(nodeNormalize),
		).
		AppendGraph(
			retrievalGraph,
			compose.WithNodeKey(nodeRetrievalGraph),
			compose.WithNodeName(nodeRetrievalGraph),
			compose.WithGraphCompileOptions(
				compose.WithGraphName(nodeRetrievalGraph),
				compose.WithMaxRunSteps(config.MaxGraphSteps),
			),
		).
		AppendLambda(
			compose.InvokableLambda(handlers.generate),
			compose.WithNodeKey(nodeGenerate),
			compose.WithNodeName(nodeGenerate),
		).
		AppendLambda(
			compose.InvokableLambda(handlers.format),
			compose.WithNodeKey(nodeFormat),
			compose.WithNodeName(nodeFormat),
		)
	return chain, nil
}

func buildRetrievalGraph(handlers *handlers) (*compose.Graph[queryState, retrievalOutcome], error) {
	graph := compose.NewGraph[queryState, retrievalOutcome]()
	if err := graph.AddLambdaNode(
		nodeRetrieve,
		compose.InvokableLambda(handlers.retrieve),
		compose.WithNodeName(nodeRetrieve),
	); err != nil {
		return nil, fmt.Errorf("添加节点 %s: %w", nodeRetrieve, err)
	}
	if err := graph.AddLambdaNode(
		nodeRewrite,
		compose.InvokableLambda(handlers.rewrite),
		compose.WithNodeName(nodeRewrite),
	); err != nil {
		return nil, fmt.Errorf("添加节点 %s: %w", nodeRewrite, err)
	}
	if err := graph.AddLambdaNode(
		nodeEvidenceReady,
		compose.InvokableLambda(handlers.evidenceReady),
		compose.WithNodeName(nodeEvidenceReady),
	); err != nil {
		return nil, fmt.Errorf("添加节点 %s: %w", nodeEvidenceReady, err)
	}
	if err := graph.AddLambdaNode(
		nodeNoEvidence,
		compose.InvokableLambda(handlers.noEvidence),
		compose.WithNodeName(nodeNoEvidence),
	); err != nil {
		return nil, fmt.Errorf("添加节点 %s: %w", nodeNoEvidence, err)
	}
	if err := graph.AddEdge(compose.START, nodeRetrieve); err != nil {
		return nil, fmt.Errorf("添加边 START -> %s: %w", nodeRetrieve, err)
	}
	branch := compose.NewGraphBranch(
		handlers.routeRetrieval,
		map[string]bool{
			nodeRewrite:       true,
			nodeEvidenceReady: true,
			nodeNoEvidence:    true,
		},
	)
	if err := graph.AddBranch(nodeRetrieve, branch); err != nil {
		return nil, fmt.Errorf("在节点 %s 后添加分支: %w", nodeRetrieve, err)
	}
	if err := graph.AddEdge(nodeRewrite, nodeRetrieve); err != nil {
		return nil, fmt.Errorf("添加回环边 %s -> %s: %w", nodeRewrite, nodeRetrieve, err)
	}
	for _, node := range []string{nodeEvidenceReady, nodeNoEvidence} {
		if err := graph.AddEdge(node, compose.END); err != nil {
			return nil, fmt.Errorf("添加边 %s -> END: %w", node, err)
		}
	}
	return graph, nil
}
