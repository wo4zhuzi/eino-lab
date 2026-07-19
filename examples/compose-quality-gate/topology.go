package main

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"
)

type gateRouter struct {
	config GateConfig
}

func addGateNodes(graph *compose.Graph[ReviewRequest, ReviewResult], nodes *gateNodes) error {
	validate := compose.InvokableLambda(nodes.validate)
	inspect := compose.InvokableLambda(nodes.inspect)
	remediate := compose.InvokableLambda(nodes.remediate)
	approve := compose.InvokableLambda(nodes.approve)
	manual := compose.InvokableLambda(nodes.manual)

	if err := graph.AddLambdaNode(nodeValidate, validate, compose.WithNodeName(nodeValidate)); err != nil {
		return fmt.Errorf("add %s node: %w", nodeValidate, err)
	}
	if err := graph.AddLambdaNode(nodeInspect, inspect, compose.WithNodeName(nodeInspect)); err != nil {
		return fmt.Errorf("add %s node: %w", nodeInspect, err)
	}
	if err := graph.AddLambdaNode(nodeRemediate, remediate, compose.WithNodeName(nodeRemediate)); err != nil {
		return fmt.Errorf("add %s node: %w", nodeRemediate, err)
	}
	if err := graph.AddLambdaNode(nodeApprove, approve, compose.WithNodeName(nodeApprove)); err != nil {
		return fmt.Errorf("add %s node: %w", nodeApprove, err)
	}
	if err := graph.AddLambdaNode(nodeManual, manual, compose.WithNodeName(nodeManual)); err != nil {
		return fmt.Errorf("add %s node: %w", nodeManual, err)
	}
	return nil
}

func addGateTopology(graph *compose.Graph[ReviewRequest, ReviewResult], config GateConfig) error {
	router := gateRouter{config: config}
	branch := compose.NewGraphBranch(
		router.route,
		map[string]bool{
			nodeApprove:   true,
			nodeManual:    true,
			nodeRemediate: true,
		},
	)
	if err := graph.AddBranch(nodeInspect, branch); err != nil {
		return fmt.Errorf("add review branch: %w", err)
	}

	edges := [][2]string{
		{compose.START, nodeValidate},
		{nodeValidate, nodeInspect},
		{nodeRemediate, nodeInspect},
		{nodeApprove, compose.END},
		{nodeManual, compose.END},
	}
	for _, edge := range edges {
		if err := graph.AddEdge(edge[0], edge[1]); err != nil {
			return fmt.Errorf("add edge %s -> %s: %w", edge[0], edge[1], err)
		}
	}
	return nil
}

func (r gateRouter) route(ctx context.Context, payload gatePayload) (string, error) {
	if payload.Score >= r.config.ApprovalThreshold {
		return nodeApprove, nil
	}

	var attempts int
	if err := compose.ProcessState[*gateState](ctx, func(_ context.Context, state *gateState) error {
		attempts = state.Attempts
		return nil
	}); err != nil {
		return "", fmt.Errorf("read routing state: %w", err)
	}
	if attempts >= r.config.MaxAttempts {
		return nodeManual, nil
	}
	return nodeRemediate, nil
}
