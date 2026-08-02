package main

import (
	"context"
	"fmt"
	"strings"
)

func requestToReviewContext(ctx context.Context, request ReviewRequest) (reviewContext, error) {
	if err := ctx.Err(); err != nil {
		return reviewContext{}, fmt.Errorf("转换审核请求: %w", err)
	}
	if err := appendLocalAudit(ctx, "request_received"); err != nil {
		return reviewContext{}, fmt.Errorf("记录请求审计: %w", err)
	}
	return reviewContext{content: request.Content}, nil
}

func normalizeReview(ctx context.Context, current reviewContext) (reviewContext, error) {
	if err := ctx.Err(); err != nil {
		return reviewContext{}, fmt.Errorf("规范化审核内容: %w", err)
	}
	current.content = strings.Join(strings.Fields(current.content), " ")
	if current.content == "" {
		return reviewContext{}, ErrEmptyContent
	}
	current.steps = append(current.steps, nodeNormalize)
	return current, nil
}

func appendChannelNotice(ctx context.Context, current reviewContext) (reviewContext, error) {
	if err := ctx.Err(); err != nil {
		return reviewContext{}, fmt.Errorf("追加渠道说明: %w", err)
	}
	current.content += " 请关注原支付渠道。"
	current.steps = append(current.steps, nodeAppendChannelNotice)
	return current, nil
}

func inspectRefundNotice(ctx context.Context, current reviewContext) (reviewContext, error) {
	if err := ctx.Err(); err != nil {
		return reviewContext{}, fmt.Errorf("检查退款说明: %w", err)
	}
	switch {
	case strings.Contains(current.content, "退款") && strings.Contains(current.content, "到账"):
		current.score = 9
		current.reasons = append(current.reasons, "包含退款到账说明")
	case strings.Contains(current.content, "退款"):
		current.score = 5
		current.reasons = append(current.reasons, "缺少退款到账说明")
	default:
		current.score = 3
		current.reasons = append(current.reasons, "未包含退款说明")
	}
	current.steps = append(current.steps, nodeInspectRefundNotice)
	return current, nil
}

func approveReview(ctx context.Context, current reviewContext) (ReviewResult, error) {
	if err := ctx.Err(); err != nil {
		return ReviewResult{}, fmt.Errorf("生成通过结果: %w", err)
	}
	current.steps = append(current.steps, nodeApprove)
	return newReviewResult(current, true, nodeApprove), nil
}

func archiveApprovedReview(ctx context.Context, result ReviewResult) (ReviewResult, error) {
	if err := ctx.Err(); err != nil {
		return ReviewResult{}, fmt.Errorf("归档通过结果: %w", err)
	}
	result.Steps = append(result.Steps, nodeArchiveApproved)
	return result, nil
}

func sendToManualReview(ctx context.Context, current reviewContext) (ReviewResult, error) {
	if err := ctx.Err(); err != nil {
		return ReviewResult{}, fmt.Errorf("生成人工审核结果: %w", err)
	}
	current.steps = append(current.steps, nodeManualReview)
	return newReviewResult(current, false, nodeManualReview), nil
}

func enqueueStandardManualReview(ctx context.Context, result ReviewResult) (ReviewResult, error) {
	if err := ctx.Err(); err != nil {
		return ReviewResult{}, fmt.Errorf("加入普通人工队列: %w", err)
	}
	result.Steps = append(result.Steps, nodeStandardManualQueue)
	return result, nil
}

func enqueuePriorityManualReview(ctx context.Context, result ReviewResult) (ReviewResult, error) {
	if err := ctx.Err(); err != nil {
		return ReviewResult{}, fmt.Errorf("加入优先人工队列: %w", err)
	}
	result.Steps = append(result.Steps, nodePriorityManualQueue)
	return result, nil
}

// recordReviewResult 是第一个审核 Branch 汇聚后的公共节点。
// 无论前面走通过路径还是人工审核路径，都会先执行这里，再进入通知 Branch。
func recordReviewResult(ctx context.Context, result ReviewResult) (ReviewResult, error) {
	if err := ctx.Err(); err != nil {
		return ReviewResult{}, fmt.Errorf("记录审核结果: %w", err)
	}
	if err := appendLocalAudit(ctx, "review_result_recorded"); err != nil {
		return ReviewResult{}, fmt.Errorf("记录审核结果审计: %w", err)
	}
	result.Steps = append(result.Steps, nodeRecordReviewResult)
	return result, nil
}

func sendApprovedNotice(ctx context.Context, result ReviewResult) (ReviewResult, error) {
	if err := ctx.Err(); err != nil {
		return ReviewResult{}, fmt.Errorf("发送审核通过通知: %w", err)
	}
	result.Steps = append(result.Steps, nodeSendApprovedNotice)
	return result, nil
}

func sendManualReviewNotice(ctx context.Context, result ReviewResult) (ReviewResult, error) {
	if err := ctx.Err(); err != nil {
		return ReviewResult{}, fmt.Errorf("发送人工审核通知: %w", err)
	}
	result.Steps = append(result.Steps, nodeSendManualNotice)
	return result, nil
}

func newReviewResult(current reviewContext, approved bool, route string) ReviewResult {
	return ReviewResult{
		Approved: approved,
		Route:    route,
		Content:  current.content,
		Score:    current.score,
		Reasons:  append([]string(nil), current.reasons...),
		Steps:    append([]string(nil), current.steps...),
	}
}
