package main

import (
	"context"
	"fmt"
)

// routeReview 为第一个 Branch 选择审核路径。
// 它接收 Branch 前一个节点 inspect_refund_notice 输出的 reviewContext。
func routeReview(ctx context.Context, current reviewContext) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("选择审核分支: %w", err)
	}
	if current.score >= 8 {
		return nodeApprove, nil
	}
	return nodeManualReview, nil
}

// routeManualQueue 为第二个 Branch 选择人工队列。
// 它接收 Branch 前一个节点 manual_review 输出的 ReviewResult。
func routeManualQueue(ctx context.Context, result ReviewResult) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("选择人工队列分支: %w", err)
	}
	if result.Score >= 5 {
		return nodeStandardManualQueue, nil
	}
	return nodePriorityManualQueue, nil
}
