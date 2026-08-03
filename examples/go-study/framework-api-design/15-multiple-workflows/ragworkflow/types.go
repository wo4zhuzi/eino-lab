package ragworkflow

import "errors"

var (
	ErrInvalidConfig = errors.New("RAG 工作流配置无效")
	ErrEmptyQuestion = errors.New("问题不能为空")
	ErrEmptyAnswer   = errors.New("生成答案不能为空")
)

// Request 是 RAG 工作流输入。
type Request struct {
	Question string
}

// Result 是 RAG 工作流输出。
type Result struct {
	Question          string
	Answer            string
	Evidence          []string
	RetrievalAttempts int
	NoEvidence        bool
	Steps             []string
}

type queryState struct {
	originalQuestion string
	query            string
	evidence         []string
	attempts         int
	steps            []string
}

type retrievalOutcome struct {
	question   string
	evidence   []string
	attempts   int
	noEvidence bool
	steps      []string
}

type answerDraft struct {
	retrievalOutcome
	answer string
}
