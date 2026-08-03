package main

import "errors"

const (
	nodeNormalizeRequest = "normalize_request"
	nodeDecisionGraph    = "decision_graph"
	nodeInspect          = "inspect"
	nodeRevise           = "revise"
	nodeApprove          = "approve"
	nodeFormatResult     = "format_result"
)

var (
	// ErrNilContext 表示构建流水线时没有可用的 context。
	ErrNilContext = errors.New("context 不能为空")
	// ErrEmptyContent 表示规范化后的审核内容为空。
	ErrEmptyContent = errors.New("审核内容不能为空")
)

// ReviewRequest 是组合流水线的对外输入。
type ReviewRequest struct {
	Content string
}

// ReviewResult 是组合流水线的对外输出。
type ReviewResult struct {
	Approved bool
	Content  string
	Score    int
	Attempts int
	Steps    []string
	Summary  string
}

// reviewDraft 是外层 Chain 传给内层 Graph 的输入。
type reviewDraft struct {
	content  string
	score    int
	attempts int
	steps    []string
}

// reviewDecision 是内层 Graph 返回给外层 Chain 的输出。
type reviewDecision struct {
	approved bool
	content  string
	score    int
	attempts int
	steps    []string
}
