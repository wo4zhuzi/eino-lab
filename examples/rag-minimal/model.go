package main

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// ExtractiveChatModel deterministically extracts the first evidence block.
type ExtractiveChatModel struct {
	calls atomic.Int64
}

func (m *ExtractiveChatModel) Generate(
	ctx context.Context,
	input []*schema.Message,
	_ ...model.Option,
) (*schema.Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.calls.Add(1)
	content := firstUserContent(input)
	evidence := extractFirstEvidence(content)
	if evidence == "" {
		return nil, ErrEmptyModelResponse
	}
	return schema.AssistantMessage("根据检索到的资料："+truncateRunes(evidence, 220), nil), nil
}

func (m *ExtractiveChatModel) Stream(
	ctx context.Context,
	input []*schema.Message,
	opts ...model.Option,
) (*schema.StreamReader[*schema.Message], error) {
	message, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{message}), nil
}

// CallCount reports how often the generation path ran.
func (m *ExtractiveChatModel) CallCount() int64 {
	return m.calls.Load()
}

func firstUserContent(messages []*schema.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i] != nil && messages[i].Role == schema.User {
			return messages[i].Content
		}
	}
	return ""
}

func extractFirstEvidence(content string) string {
	const marker = "正文:\n"
	start := strings.Index(content, marker)
	if start < 0 {
		return ""
	}
	remaining := content[start+len(marker):]
	if end := strings.Index(remaining, "\n\n[证据 "); end >= 0 {
		remaining = remaining[:end]
	}
	return strings.TrimSpace(remaining)
}

func truncateRunes(input string, limit int) string {
	if utf8.RuneCountInString(input) <= limit {
		return input
	}
	runes := []rune(input)
	return fmt.Sprintf("%s...", string(runes[:limit]))
}
