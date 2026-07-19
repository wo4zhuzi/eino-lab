package main

import (
	"context"
	"errors"
	"fmt"

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
	nodes := &gateNodes{inspector: inspector}

	if err := addGateNodes(graph, nodes); err != nil {
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
