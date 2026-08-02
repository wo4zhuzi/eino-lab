package main

import "errors"

const (
	nodeInputAdapter        = "input_adapter"
	nodeNormalize           = "normalize"
	nodeAppendChannelNotice = "append_channel_notice"
	nodeInspectRefundNotice = "inspect_refund_notice"
	nodeApprove             = "approve"
	nodeArchiveApproved     = "archive_approved_review"
	nodeManualReview        = "manual_review"
	nodeStandardManualQueue = "standard_manual_queue"
	nodePriorityManualQueue = "priority_manual_queue"
	nodeRecordReviewResult  = "record_review_result"
	nodeSendApprovedNotice  = "send_approved_notice"
	nodeSendManualNotice    = "send_manual_review_notice"
)

var (
	// ErrNilContext 表示创建流水线时没有可用的 context。
	ErrNilContext = errors.New("context 不能为空")
	// ErrEmptyContent 表示规范化后的审核内容为空。
	ErrEmptyContent = errors.New("审核内容不能为空")
)

// ReviewRequest 是审核流水线的对外输入。
type ReviewRequest struct {
	Content string
}

// ReviewResult 是审核流水线的对外输出。
type ReviewResult struct {
	Approved bool
	Route    string
	Content  string
	Score    int
	Reasons  []string
	Steps    []string
}

// reviewContext 只在流水线前半段的审核节点之间传递。
type reviewContext struct {
	content string
	score   int
	reasons []string
	steps   []string
}
