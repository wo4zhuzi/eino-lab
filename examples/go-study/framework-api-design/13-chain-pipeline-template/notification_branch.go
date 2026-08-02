package main

import "github.com/cloudwego/eino/compose"

// newNotificationBranch 声明审核结果记录完成后的通知分支。
//
// 主 Chain 只保留 AppendBranch(newNotificationBranch()) 这一行业务语义；
// 通知分支允许选择的目标节点集中定义在这里，避免主流程出现嵌套实现。
func newNotificationBranch() *compose.ChainBranch {
	return compose.NewChainBranch(routeNotification).
		AddLambda(nodeSendApprovedNotice, compose.InvokableLambda(sendApprovedNotice)).
		AddLambda(nodeSendManualNotice, compose.InvokableLambda(sendManualReviewNotice))
}
