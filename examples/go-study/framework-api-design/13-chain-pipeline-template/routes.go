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

// routeNotification 为审核结果记录完成后的第三个 Branch 选择通知节点。
// 第一个 Branch 的不同路径已经汇聚，所以这里统一接收 ReviewResult。
func routeNotification(ctx context.Context, result ReviewResult) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("选择通知分支: %w", err)
	}
	if result.Approved {
		return nodeSendApprovedNotice, nil
	}
	return nodeSendManualNotice, nil
}
