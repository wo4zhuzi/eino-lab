package main

import "github.com/cloudwego/eino/compose"

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
// manual_review 执行完成后直接追加人工队列 Branch，不需要维护
// manual_review -> queue 的两条底层 Edge。
func newManualReviewPath() *compose.Chain[reviewContext, ReviewResult] {
	path := compose.NewChain[reviewContext, ReviewResult]()
	return path.
		AppendLambda(compose.InvokableLambda(sendToManualReview), compose.WithNodeKey(nodeManualReview)).
		AppendBranch(newManualQueueBranch())
}

// newManualQueueBranch 声明审核模块内部的 Branch：普通人工队列或优先人工队列。
func newManualQueueBranch() *compose.ChainBranch {
	return compose.NewChainBranch(routeManualQueue).
		AddLambda(nodeStandardManualQueue, compose.InvokableLambda(enqueueStandardManualReview)).
		AddLambda(nodePriorityManualQueue, compose.InvokableLambda(enqueuePriorityManualReview))
}
