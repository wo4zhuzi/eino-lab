package reviewworkflow

import "errors"

const (
	RouteApproved     = "approved"
	RouteManualReview = "manual_review"
)

var (
	ErrInvalidConfig = errors.New("审核工作流配置无效")
	ErrEmptyContent  = errors.New("审核内容不能为空")
	ErrInvalidScore  = errors.New("审核分数无效")
)

// Request 是审核工作流输入。
type Request struct {
	Content string
}

// Result 是审核工作流输出。
type Result struct {
	Approved bool
	Route    string
	Content  string
	Score    int
	Reason   string
	Steps    []string
}

type reviewDraft struct {
	content string
	score   int
	reason  string
	steps   []string
}

type reviewDecision struct {
	approved bool
	route    string
	content  string
	score    int
	reason   string
	steps    []string
}
