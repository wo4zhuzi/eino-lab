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
	target := nodeManualReview
	if current.score >= 8 {
		target = nodeApprove
	}
	if err := appendLocalAudit(ctx, "review_branch="+target); err != nil {
		return "", fmt.Errorf("记录审核分支审计: %w", err)
	}
	return target, nil
}

// routeManualQueue 为第二个 Branch 选择人工队列。
// 它接收 Branch 前一个节点 manual_review 输出的 ReviewResult。
func routeManualQueue(ctx context.Context, result ReviewResult) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("选择人工队列分支: %w", err)
	}
	target := nodePriorityManualQueue
	if result.Score >= 5 {
		target = nodeStandardManualQueue
	}
	if err := appendLocalAudit(ctx, "manual_queue_branch="+target); err != nil {
		return "", fmt.Errorf("记录人工队列分支审计: %w", err)
	}
	return target, nil
}

// routeNotification 为审核结果记录完成后的第三个 Branch 选择通知节点。
// 第一个 Branch 的不同路径已经汇聚，所以这里统一接收 ReviewResult。
func routeNotification(ctx context.Context, result ReviewResult) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("选择通知分支: %w", err)
	}
	target := nodeSendManualNotice
	if result.Approved {
		target = nodeSendApprovedNotice
	}
	if err := appendLocalAudit(ctx, "notification_branch="+target); err != nil {
		return "", fmt.Errorf("记录通知分支审计: %w", err)
	}
	return target, nil
}
