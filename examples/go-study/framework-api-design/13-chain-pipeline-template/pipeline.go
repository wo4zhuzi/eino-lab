package main

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"
)

// NewReviewPipeline 创建并编译审核流水线。
//
// 这是调用方唯一需要使用的构建入口。ReviewRequest 和 ReviewResult 在这里明确写出，
// 整条流水线创建后，其对外输入和输出类型就固定了。
func NewReviewPipeline(ctx context.Context) (compose.Runnable[ReviewRequest, ReviewResult], error) {
	if ctx == nil {
		return nil, ErrNilContext
	}

	// 主流水线只描述公共步骤，阅读顺序就是运行顺序。
	// AppendBranch 到达时，再进入 newReviewBranch 声明的某一条子路径。
	// WithGenLocalState 不会在 Compile 时创建共享对象，而是在每次 Invoke 开始时
	// 调用 newReviewLocalState，为这一次运行创建独立状态。
	pipeline := compose.NewChain[ReviewRequest, ReviewResult](
		compose.WithGenLocalState(newReviewLocalState),
	)
	pipeline.
		AppendLambda(compose.InvokableLambda(requestToReviewContext), compose.WithNodeKey(nodeInputAdapter)).
		AppendLambda(compose.InvokableLambda(normalizeReview), compose.WithNodeKey(nodeNormalize)).
		AppendLambda(compose.InvokableLambda(appendChannelNotice), compose.WithNodeKey(nodeAppendChannelNotice)).
		AppendLambda(compose.InvokableLambda(inspectRefundNotice), compose.WithNodeKey(nodeInspectRefundNotice)).
		AppendBranch(newReviewBranch()).
		AppendLambda(compose.InvokableLambda(recordReviewResult), compose.WithNodeKey(nodeRecordReviewResult)).
		AppendBranch(newNotificationBranch()).
		AppendLambda(compose.InvokableLambda(attachLocalAudit), compose.WithNodeKey(nodeAttachLocalAudit))

	runnable, err := pipeline.Compile(ctx, compose.WithGraphName("review_pipeline"))
	if err != nil {
		return nil, fmt.Errorf("编译审核流水线: %w", err)
	}
	return runnable, nil
}
