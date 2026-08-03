package ragworkflow

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type handlers struct {
	config       Config
	dependencies Dependencies
}

func (h *handlers) normalize(ctx context.Context, request Request) (queryState, error) {
	if err := ctx.Err(); err != nil {
		return queryState{}, fmt.Errorf("规范化问题: %w", err)
	}
	question := strings.Join(strings.Fields(request.Question), " ")
	if question == "" {
		return queryState{}, ErrEmptyQuestion
	}
	return queryState{
		originalQuestion: question,
		query:            question,
		steps:            []string{nodeNormalize},
	}, nil
}

func (h *handlers) retrieve(ctx context.Context, state queryState) (queryState, error) {
	if err := ctx.Err(); err != nil {
		return queryState{}, fmt.Errorf("检索证据: %w", err)
	}
	documents, err := h.dependencies.Retriever.Retrieve(ctx, state.query)
	if err != nil {
		return queryState{}, fmt.Errorf("调用 Retriever: %w", err)
	}
	state.evidence = append([]string(nil), documents...)
	state.attempts++
	state.steps = append(state.steps, nodeRetrieve)
	return state, nil
}

func (h *handlers) routeRetrieval(ctx context.Context, state queryState) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("选择检索路径: %w", err)
	}
	if len(state.evidence) > 0 {
		return nodeEvidenceReady, nil
	}
	if state.attempts >= h.config.MaxRetrievalAttempts {
		return nodeNoEvidence, nil
	}
	return nodeRewrite, nil
}

func (h *handlers) rewrite(ctx context.Context, state queryState) (queryState, error) {
	if err := ctx.Err(); err != nil {
		return queryState{}, fmt.Errorf("改写检索词: %w", err)
	}
	state.query = strings.TrimSpace(state.query + " " + h.config.RewriteSuffix)
	state.steps = append(state.steps, nodeRewrite)
	return state, nil
}

func (h *handlers) evidenceReady(ctx context.Context, state queryState) (retrievalOutcome, error) {
	return h.toOutcome(ctx, state, false, nodeEvidenceReady)
}

func (h *handlers) noEvidence(ctx context.Context, state queryState) (retrievalOutcome, error) {
	return h.toOutcome(ctx, state, true, nodeNoEvidence)
}

func (h *handlers) toOutcome(
	ctx context.Context,
	state queryState,
	noEvidence bool,
	node string,
) (retrievalOutcome, error) {
	if err := ctx.Err(); err != nil {
		return retrievalOutcome{}, fmt.Errorf("生成检索结果: %w", err)
	}
	steps := append([]string(nil), state.steps...)
	steps = append(steps, node)
	return retrievalOutcome{
		question:   state.originalQuestion,
		evidence:   append([]string(nil), state.evidence...),
		attempts:   state.attempts,
		noEvidence: noEvidence,
		steps:      steps,
	}, nil
}

func (h *handlers) generate(ctx context.Context, outcome retrievalOutcome) (answerDraft, error) {
	if err := ctx.Err(); err != nil {
		return answerDraft{}, fmt.Errorf("生成答案: %w", err)
	}
	answer := "未检索到足够证据，暂时无法回答。"
	if !outcome.noEvidence {
		generated, err := h.dependencies.Generator.Generate(ctx, outcome.question, outcome.evidence)
		if err != nil {
			return answerDraft{}, fmt.Errorf("调用 Generator: %w", err)
		}
		if strings.TrimSpace(generated) == "" {
			return answerDraft{}, ErrEmptyAnswer
		}
		answer = generated
	}
	outcome.steps = append(outcome.steps, nodeGenerate)
	return answerDraft{retrievalOutcome: outcome, answer: answer}, nil
}

func (h *handlers) format(ctx context.Context, draft answerDraft) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, fmt.Errorf("格式化 RAG 结果: %w", err)
	}
	steps := append([]string(nil), draft.steps...)
	steps = append(steps, nodeFormat)
	return Result{
		Question:          draft.question,
		Answer:            draft.answer,
		Evidence:          append([]string(nil), draft.evidence...),
		RetrievalAttempts: draft.attempts,
		NoEvidence:        draft.noEvidence,
		Steps:             steps,
	}, nil
}

// MemoryRetriever 是无需外部服务的示例 Retriever。
type MemoryRetriever struct {
	documents map[string][]string
}

// NewMemoryRetriever 创建内存检索器并复制输入文档。
func NewMemoryRetriever(documents map[string][]string) *MemoryRetriever {
	cloned := make(map[string][]string, len(documents))
	for keyword, values := range documents {
		cloned[keyword] = append([]string(nil), values...)
	}
	return &MemoryRetriever{documents: cloned}
}

// Retrieve 返回关键词命中的全部文档。
func (r *MemoryRetriever) Retrieve(ctx context.Context, query string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	keywords := make([]string, 0, len(r.documents))
	for keyword := range r.documents {
		keywords = append(keywords, keyword)
	}
	sort.Strings(keywords)
	var result []string
	for _, keyword := range keywords {
		if strings.Contains(query, keyword) {
			result = append(result, r.documents[keyword]...)
		}
	}
	return append([]string(nil), result...), nil
}

// CitationGenerator 是基于证据拼接答案的示例 Generator。
type CitationGenerator struct{}

// NewCitationGenerator 创建本地答案生成器。
func NewCitationGenerator() *CitationGenerator {
	return &CitationGenerator{}
}

// Generate 生成带证据摘要的确定性答案。
func (*CitationGenerator) Generate(
	ctx context.Context,
	question string,
	evidence []string,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return fmt.Sprintf("问题：%s；根据资料：%s", question, strings.Join(evidence, "；")), nil
}

var (
	_ Retriever = (*MemoryRetriever)(nil)
	_ Generator = (*CitationGenerator)(nil)
)
