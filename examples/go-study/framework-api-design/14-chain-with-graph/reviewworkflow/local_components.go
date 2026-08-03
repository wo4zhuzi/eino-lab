package reviewworkflow

import (
	"context"
	"fmt"
	"strings"
)

var (
	_ Inspector = (*KeywordInspector)(nil)
	_ Reviser   = (*AppendReviser)(nil)
)

// KeywordInspector 是无需外部服务的示例 Inspector。
type KeywordInspector struct{}

// NewKeywordInspector 创建本地关键词审核器。
func NewKeywordInspector() *KeywordInspector {
	return &KeywordInspector{}
}

// Inspect 根据退款和到账关键词返回确定性分数。
func (*KeywordInspector) Inspect(ctx context.Context, content string) (Inspection, error) {
	if err := ctx.Err(); err != nil {
		return Inspection{}, err
	}
	if strings.Contains(content, "退款") && strings.Contains(content, "到账") {
		return Inspection{Score: 9, Reason: "包含退款到账说明"}, nil
	}
	return Inspection{Score: 5, Reason: "缺少退款到账说明"}, nil
}

// AppendReviser 是在原内容后追加固定说明的示例 Reviser。
type AppendReviser struct {
	suffix string
}

// NewAppendReviser 创建本地修订器。
func NewAppendReviser(suffix string) (*AppendReviser, error) {
	suffix = strings.TrimSpace(suffix)
	if suffix == "" {
		return nil, fmt.Errorf("%w: 修订后缀不能为空", ErrInvalidDependencies)
	}
	return &AppendReviser{suffix: suffix}, nil
}

// Revise 返回追加固定说明后的新内容。
func (r *AppendReviser) Revise(ctx context.Context, content string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return strings.TrimSpace(content) + " " + r.suffix, nil
}
