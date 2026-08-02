package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"
)

// ErrEmptyContent 表示规范化后的审核内容为空。
var ErrEmptyContent = errors.New("审核内容不能为空")

// ReviewRequest 是审核 Graph 对外接收的请求。
type ReviewRequest struct {
	Content string
}

// ReviewResult 是审核 Graph 对外返回的结果。
type ReviewResult struct {
	Approved bool
	Content  string
	Score    int
	Reasons  []string
	Steps    []string
}

// reviewContext 只在 Graph 内部的中间节点之间传递。
// 新增线性节点时继续读写这个类型，Graph 的 Request/Result 边界无需改变。
type reviewContext struct {
	content string
	score   int
	reasons []string
	steps   []string
}

// NewReviewGraph 是业务侧稳定的构建入口。
// 仅增删线性中间节点时，该函数和 compileLinearGraph 都不需要修改。
func NewReviewGraph(ctx context.Context) (compose.Runnable[ReviewRequest, ReviewResult], error) {
	return compileLinearGraph(
		ctx,
		"linear_review_graph",
		requestToReviewContext,
		reviewSteps(),
		reviewContextToResult,
	)
}

func requestToReviewContext(ctx context.Context, request ReviewRequest) (reviewContext, error) {
	if err := ctx.Err(); err != nil {
		return reviewContext{}, fmt.Errorf("转换审核请求: %w", err)
	}
	return reviewContext{content: request.Content}, nil
}

// reviewSteps 是本示例唯一需要频繁修改的“中间方法”。
//
// 每个元素就是一个 Eino Lambda 节点，切片顺序就是运行顺序。新增线性节点时，
// 只需在这里增加一个 Key + Run；公共构建器会自动注册节点并重新连接全部 Edge。
func reviewSteps() []linearStep[reviewContext] {
	return []linearStep[reviewContext]{
		{
			Key: "normalize",
			Run: func(ctx context.Context, current reviewContext) (reviewContext, error) {
				if err := ctx.Err(); err != nil {
					return reviewContext{}, fmt.Errorf("规范化审核内容: %w", err)
				}
				current.content = strings.Join(strings.Fields(current.content), " ")
				if current.content == "" {
					return reviewContext{}, ErrEmptyContent
				}
				current.steps = append(current.steps, "normalize")
				return current, nil
			},
		},
		{
			Key: "inspect_refund_notice",
			Run: func(ctx context.Context, current reviewContext) (reviewContext, error) {
				if err := ctx.Err(); err != nil {
					return reviewContext{}, fmt.Errorf("检查退款说明: %w", err)
				}
				if strings.Contains(current.content, "退款") && strings.Contains(current.content, "到账") {
					current.score = 9
					current.reasons = append(current.reasons, "包含退款到账说明")
				} else {
					current.score = 5
					current.reasons = append(current.reasons, "缺少退款到账说明")
				}
				current.steps = append(current.steps, "inspect_refund_notice")
				return current, nil
			},
		},
	}
}

func reviewContextToResult(ctx context.Context, current reviewContext) (ReviewResult, error) {
	if err := ctx.Err(); err != nil {
		return ReviewResult{}, fmt.Errorf("生成审核结果: %w", err)
	}
	return ReviewResult{
		Approved: current.score >= 8,
		Content:  current.content,
		Score:    current.score,
		Reasons:  append([]string(nil), current.reasons...),
		Steps:    append([]string(nil), current.steps...),
	}, nil
}
