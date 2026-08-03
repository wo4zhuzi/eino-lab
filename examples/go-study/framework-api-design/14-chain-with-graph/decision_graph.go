package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"
)

// newDecisionGraph 创建可独立组合的审核决策子图。
//
// inspect 不通过时进入 revise，再通过显式 Edge 回到 inspect；这种回环比线性 Chain
// 更适合用 Graph 表达。Graph 只暴露 reviewDraft -> reviewDecision 类型边界。
func newDecisionGraph() (*compose.Graph[reviewDraft, reviewDecision], error) {
	graph := compose.NewGraph[reviewDraft, reviewDecision]()

	if err := addDecisionNode(graph, nodeInspect, inspectDraft); err != nil {
		return nil, err
	}
	if err := addDecisionNode(graph, nodeRevise, reviseDraft); err != nil {
		return nil, err
	}
	if err := addDecisionNode(graph, nodeApprove, approveDraft); err != nil {
		return nil, err
	}

	if err := graph.AddEdge(compose.START, nodeInspect); err != nil {
		return nil, fmt.Errorf("添加边 START -> %s: %w", nodeInspect, err)
	}

	branch := compose.NewGraphBranch(
		routeInspection,
		map[string]bool{
			nodeRevise:  true,
			nodeApprove: true,
		},
	)
	if err := graph.AddBranch(nodeInspect, branch); err != nil {
		return nil, fmt.Errorf("在节点 %s 后添加分支: %w", nodeInspect, err)
	}

	if err := graph.AddEdge(nodeRevise, nodeInspect); err != nil {
		return nil, fmt.Errorf("添加回环边 %s -> %s: %w", nodeRevise, nodeInspect, err)
	}
	if err := graph.AddEdge(nodeApprove, compose.END); err != nil {
		return nil, fmt.Errorf("添加边 %s -> END: %w", nodeApprove, err)
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

func inspectDraft(ctx context.Context, draft reviewDraft) (reviewDraft, error) {
	if err := ctx.Err(); err != nil {
		return reviewDraft{}, fmt.Errorf("检查审核内容: %w", err)
	}

	draft.attempts++
	if strings.Contains(draft.content, "退款") && strings.Contains(draft.content, "到账") {
		draft.score = 9
	} else {
		draft.score = 5
	}
	draft.steps = append(draft.steps, nodeInspect)
	return draft, nil
}

func routeInspection(ctx context.Context, draft reviewDraft) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("选择审核路径: %w", err)
	}
	if draft.score >= 8 {
		return nodeApprove, nil
	}
	return nodeRevise, nil
}

func reviseDraft(ctx context.Context, draft reviewDraft) (reviewDraft, error) {
	if err := ctx.Err(); err != nil {
		return reviewDraft{}, fmt.Errorf("修订审核内容: %w", err)
	}

	draft.content += " 补充：退款将在 3 个工作日到账。"
	draft.steps = append(draft.steps, nodeRevise)
	return draft, nil
}

func approveDraft(ctx context.Context, draft reviewDraft) (reviewDecision, error) {
	if err := ctx.Err(); err != nil {
		return reviewDecision{}, fmt.Errorf("生成审核决定: %w", err)
	}

	steps := append([]string(nil), draft.steps...)
	steps = append(steps, nodeApprove)
	return reviewDecision{
		approved: true,
		content:  draft.content,
		score:    draft.score,
		attempts: draft.attempts,
		steps:    steps,
	}, nil
}
