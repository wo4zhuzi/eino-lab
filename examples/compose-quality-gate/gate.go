package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"
)

var (
	ErrEmptyContent         = errors.New("content is empty")
	ErrInspectorUnavailable = errors.New("inspector unavailable")
	ErrInvalidInspection    = errors.New("invalid inspection")
)

const (
	nodeValidate  = "validate"
	nodeInspect   = "inspect"
	nodeRemediate = "remediate"
	nodeApprove   = "approve"
	nodeManual    = "manual"
	graphName     = "compose_quality_gate"
)

type ReviewStatus string

const (
	ReviewApproved     ReviewStatus = "approved"
	ReviewManualReview ReviewStatus = "manual_review"
)

type ReviewRequest struct {
	Content string
}

type Inspection struct {
	Score  int
	Reason string
}

type AuditEntry struct {
	Attempt int
	Score   int
	Reason  string
}

type ReviewResult struct {
	Status   ReviewStatus
	Content  string
	Score    int
	Reason   string
	Attempts int
	Audit    []AuditEntry
}

type Inspector interface {
	Inspect(ctx context.Context, content string) (Inspection, error)
}

type GateConfig struct {
	ApprovalThreshold int
	MaxAttempts       int
	MaxRunSteps       int
}

func DefaultGateConfig() GateConfig {
	return GateConfig{
		ApprovalThreshold: 7,
		MaxAttempts:       3,
		MaxRunSteps:       16,
	}
}

type QualityGate struct {
	runnable    compose.Runnable[ReviewRequest, ReviewResult]
	snapshotter *TopologySnapshotter
}

type gatePayload struct {
	Content string
	Score   int
	Reason  string
}

type gateState struct {
	Attempts int
	Audit    []AuditEntry
}

func NewQualityGate(ctx context.Context, inspector Inspector, config GateConfig) (*QualityGate, error) {
	if inspector == nil {
		return nil, errors.New("inspector is required")
	}
	if config.ApprovalThreshold < 0 || config.ApprovalThreshold > 10 {
		return nil, fmt.Errorf("approval threshold must be between 0 and 10: %d", config.ApprovalThreshold)
	}
	if config.MaxAttempts < 1 {
		return nil, fmt.Errorf("max attempts must be positive: %d", config.MaxAttempts)
	}
	if config.MaxRunSteps < 1 {
		return nil, fmt.Errorf("max run steps must be positive: %d", config.MaxRunSteps)
	}

	snapshotter := NewTopologySnapshotter()
	graph := compose.NewGraph[ReviewRequest, ReviewResult](
		compose.WithGenLocalState(func(context.Context) *gateState {
			return &gateState{}
		}),
	)

	if err := addGateNodes(graph, inspector, config); err != nil {
		return nil, err
	}
	if err := addGateTopology(graph, config); err != nil {
		return nil, err
	}

	runnable, err := graph.Compile(
		ctx,
		compose.WithGraphName(graphName),
		compose.WithMaxRunSteps(config.MaxRunSteps),
		compose.WithGraphCompileCallbacks(snapshotter),
	)
	if err != nil {
		return nil, fmt.Errorf("compile quality gate graph: %w", err)
	}

	return &QualityGate{runnable: runnable, snapshotter: snapshotter}, nil
}

func (g *QualityGate) Review(ctx context.Context, request ReviewRequest, opts ...compose.Option) (ReviewResult, error) {
	return g.runnable.Invoke(ctx, request, opts...)
}

func (g *QualityGate) Topology() TopologySnapshot {
	return g.snapshotter.Snapshot()
}

func addGateNodes(graph *compose.Graph[ReviewRequest, ReviewResult], inspector Inspector, config GateConfig) error {
	validate := compose.InvokableLambda(func(ctx context.Context, request ReviewRequest) (gatePayload, error) {
		if err := ctx.Err(); err != nil {
			return gatePayload{}, err
		}
		content := strings.TrimSpace(request.Content)
		if content == "" {
			return gatePayload{}, ErrEmptyContent
		}
		return gatePayload{Content: content}, nil
	})
	if err := graph.AddLambdaNode(nodeValidate, validate, compose.WithNodeName(nodeValidate)); err != nil {
		return fmt.Errorf("add %s node: %w", nodeValidate, err)
	}

	inspect := compose.InvokableLambda(func(ctx context.Context, payload gatePayload) (gatePayload, error) {
		if err := ctx.Err(); err != nil {
			return gatePayload{}, err
		}
		result, err := inspector.Inspect(ctx, payload.Content)
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
	})
	if err := graph.AddLambdaNode(nodeInspect, inspect, compose.WithNodeName(nodeInspect)); err != nil {
		return fmt.Errorf("add %s node: %w", nodeInspect, err)
	}

	remediate := compose.InvokableLambda(func(ctx context.Context, payload gatePayload) (gatePayload, error) {
		if err := ctx.Err(); err != nil {
			return gatePayload{}, err
		}
		payload.Content = strings.TrimSpace(payload.Content) + "\n[remediated]"
		return payload, nil
	})
	if err := graph.AddLambdaNode(nodeRemediate, remediate, compose.WithNodeName(nodeRemediate)); err != nil {
		return fmt.Errorf("add %s node: %w", nodeRemediate, err)
	}

	if err := graph.AddLambdaNode(nodeApprove, newFinalizeNode(ReviewApproved), compose.WithNodeName(nodeApprove)); err != nil {
		return fmt.Errorf("add %s node: %w", nodeApprove, err)
	}
	if err := graph.AddLambdaNode(nodeManual, newFinalizeNode(ReviewManualReview), compose.WithNodeName(nodeManual)); err != nil {
		return fmt.Errorf("add %s node: %w", nodeManual, err)
	}

	return nil
}

func addGateTopology(graph *compose.Graph[ReviewRequest, ReviewResult], config GateConfig) error {
	route := compose.NewGraphBranch(
		func(ctx context.Context, payload gatePayload) (string, error) {
			if payload.Score >= config.ApprovalThreshold {
				return nodeApprove, nil
			}

			var attempts int
			if err := compose.ProcessState[*gateState](ctx, func(_ context.Context, state *gateState) error {
				attempts = state.Attempts
				return nil
			}); err != nil {
				return "", fmt.Errorf("read routing state: %w", err)
			}
			if attempts >= config.MaxAttempts {
				return nodeManual, nil
			}
			return nodeRemediate, nil
		},
		map[string]bool{
			nodeApprove:   true,
			nodeManual:    true,
			nodeRemediate: true,
		},
	)

	if err := graph.AddBranch(nodeInspect, route); err != nil {
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

func newFinalizeNode(status ReviewStatus) *compose.Lambda {
	return compose.InvokableLambda(func(ctx context.Context, payload gatePayload) (ReviewResult, error) {
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
	})
}
