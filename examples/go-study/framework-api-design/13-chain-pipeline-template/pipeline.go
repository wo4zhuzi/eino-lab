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
	pipeline := compose.NewChain[ReviewRequest, ReviewResult]()
	pipeline.
		AppendLambda(compose.InvokableLambda(requestToReviewContext), compose.WithNodeKey(nodeInputAdapter)).
		AppendLambda(compose.InvokableLambda(normalizeReview), compose.WithNodeKey(nodeNormalize)).
		AppendLambda(compose.InvokableLambda(appendChannelNotice), compose.WithNodeKey(nodeAppendChannelNotice)).
		AppendLambda(compose.InvokableLambda(inspectRefundNotice), compose.WithNodeKey(nodeInspectRefundNotice)).
		AppendBranch(newReviewBranch())

	runnable, err := pipeline.Compile(ctx, compose.WithGraphName("review_pipeline"))
	if err != nil {
		return nil, fmt.Errorf("编译审核流水线: %w", err)
	}
	return runnable, nil
}

// newReviewBranch 声明第一个 Branch：通过或进入人工审核。
//
// 两个目标都使用子 Chain，因此每条路径可以继续 AppendLambda 或 AppendBranch，
// 不需要回到主流程中手工注册节点和连接 Edge。
func newReviewBranch() *compose.ChainBranch {
	return compose.NewChainBranch(routeReview).
		AddGraph(nodeApprove, newApprovePath()).
		AddGraph(nodeManualReview, newManualReviewPath())
}

// newApprovePath 是审核通过后的完整流水线。
//
// 以后只在通过路径增加普通节点时，就在这个方法中追加一行 AppendLambda。
func newApprovePath() *compose.Chain[reviewContext, ReviewResult] {
	path := compose.NewChain[reviewContext, ReviewResult]()
	return path.
		AppendLambda(compose.InvokableLambda(approveReview), compose.WithNodeKey(nodeApprove)).
		AppendLambda(compose.InvokableLambda(archiveApprovedReview), compose.WithNodeKey(nodeArchiveApproved))
}

// newManualReviewPath 是进入人工审核后的完整流水线。
//
// manual_review 执行完成后直接追加第二个 Branch，不需要维护
// manual_review -> queue 的两条底层 Edge。
func newManualReviewPath() *compose.Chain[reviewContext, ReviewResult] {
	path := compose.NewChain[reviewContext, ReviewResult]()
	return path.
		AppendLambda(compose.InvokableLambda(sendToManualReview), compose.WithNodeKey(nodeManualReview)).
		AppendBranch(newManualQueueBranch())
}

// newManualQueueBranch 声明第二个 Branch：普通人工队列或优先人工队列。
func newManualQueueBranch() *compose.ChainBranch {
	return compose.NewChainBranch(routeManualQueue).
		AddLambda(nodeStandardManualQueue, compose.InvokableLambda(enqueueStandardManualReview)).
		AddLambda(nodePriorityManualQueue, compose.InvokableLambda(enqueuePriorityManualReview))
}
