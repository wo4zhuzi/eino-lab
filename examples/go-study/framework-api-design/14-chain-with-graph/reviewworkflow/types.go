package reviewworkflow

const (
	RouteApproved     = "approved"
	RouteManualReview = "manual_review"
)

// ReviewRequest 是工作流对外接收的稳定输入。
type ReviewRequest struct {
	Content string
}

// ReviewResult 是工作流对外返回的稳定输出。
type ReviewResult struct {
	Approved bool
	Route    string
	Content  string
	Score    int
	Attempts int
	Reasons  []string
	Steps    []string
	Summary  string
}

// reviewDraft 是外层 Chain 传给内层 Graph 的内部输入。
type reviewDraft struct {
	content  string
	score    int
	attempts int
	reasons  []string
	steps    []string
}

// reviewDecision 是内层 Graph 返回给外层 Chain 的内部输出。
type reviewDecision struct {
	approved bool
	route    string
	content  string
	score    int
	attempts int
	reasons  []string
	steps    []string
}
