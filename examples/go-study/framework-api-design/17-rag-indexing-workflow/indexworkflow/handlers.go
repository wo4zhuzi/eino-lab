package indexworkflow

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	docxparser "github.com/cloudwego/eino-ext/components/document/parser/docx"
	"github.com/cloudwego/eino/schema"
)

type workflowHandlers struct {
	config  Config
	parsers *parserService
}

func (h *workflowHandlers) inspect(ctx context.Context, request Request) (workflowState, error) {
	source, err := inspectSource(ctx, request, h.config.MaxFileBytes)
	if err != nil {
		return workflowState{}, fmt.Errorf("%s: %w", nodeInspectSource, err)
	}
	return workflowState{
		request: request,
		source:  source,
		stages: []StageResult{{
			Name:   nodeInspectSource,
			Status: StageStatusCompleted,
			Detail: "文件签名、大小、MIME 和 SHA-256 已验证",
		}},
	}, nil
}

func (h *workflowHandlers) parse(ctx context.Context, state workflowState) (workflowState, error) {
	parsed, err := h.parsers.parse(ctx, state.source)
	if err != nil {
		return workflowState{}, fmt.Errorf("%s: %w", nodeParseDocument, err)
	}
	state.parserName = parsed.parserName
	state.parserVersion = parsed.parserVersion
	state.documents = parsed.documents
	state.parsedUnits = summarizeDocuments(parsed.documents)
	state.stages = append(state.stages, StageResult{
		Name:   nodeParseDocument,
		Status: StageStatusCompleted,
		Detail: fmt.Sprintf("真实解析完成，共 %d 个标准化单元", len(parsed.documents)),
	})
	return state, nil
}

func (*workflowHandlers) simulateChunk(ctx context.Context, state workflowState) (workflowState, error) {
	if err := contextError(ctx, nodeChunkDocument); err != nil {
		return workflowState{}, err
	}
	state.placeholder.Simulated = true
	state.placeholder.PlannedChunkCount = len(state.documents)
	state.stages = append(state.stages, StageResult{
		Name:   nodeChunkDocument,
		Status: StageStatusSimulated,
		Detail: "暂按一个解析单元对应一个计划 Chunk，未生成真实 Chunk",
	})
	return state, nil
}

func (*workflowHandlers) simulateEmbedding(ctx context.Context, state workflowState) (workflowState, error) {
	if err := contextError(ctx, nodeEmbedChunks); err != nil {
		return workflowState{}, err
	}
	state.placeholder.EmbeddingModel = "not-configured"
	state.placeholder.VectorDimension = 0
	state.stages = append(state.stages, StageResult{
		Name:   nodeEmbedChunks,
		Status: StageStatusSimulated,
		Detail: "未调用 Embedding 服务，模型和向量维度尚未配置",
	})
	return state, nil
}

func (*workflowHandlers) simulatePersist(ctx context.Context, state workflowState) (workflowState, error) {
	if err := contextError(ctx, nodePersistIndex); err != nil {
		return workflowState{}, err
	}
	state.placeholder.PersistedRecordCount = 0
	state.stages = append(state.stages, StageResult{
		Name:   nodePersistIndex,
		Status: StageStatusSimulated,
		Detail: "未连接 PostgreSQL，持久化记录数固定为 0",
	})
	return state, nil
}

func (*workflowHandlers) simulateValidate(ctx context.Context, state workflowState) (workflowState, error) {
	if err := contextError(ctx, nodeValidateIndex); err != nil {
		return workflowState{}, err
	}
	state.placeholder.ValidationPassed = false
	state.stages = append(state.stages, StageResult{
		Name:   nodeValidateIndex,
		Status: StageStatusSimulated,
		Detail: "没有真实 Chunk、向量和存储记录，因此不执行索引校验",
	})
	return state, nil
}

func (*workflowHandlers) simulatePublish(ctx context.Context, state workflowState) (workflowState, error) {
	if err := contextError(ctx, nodePublishIndex); err != nil {
		return workflowState{}, err
	}
	state.placeholder.PublishedIndexVersion = ""
	state.stages = append(state.stages, StageResult{
		Name:   nodePublishIndex,
		Status: StageStatusSimulated,
		Detail: "未发布索引版本，线上活动索引不会发生变化",
	})
	return state, nil
}

func (*workflowHandlers) buildResult(ctx context.Context, state workflowState) (Result, error) {
	if err := contextError(ctx, nodeBuildResult); err != nil {
		return Result{}, err
	}
	state.stages = append(state.stages, StageResult{
		Name:   nodeBuildResult,
		Status: StageStatusCompleted,
		Detail: "已生成不包含完整原文的工作流结果",
	})
	return Result{
		Workflow:      Descriptor(),
		RunID:         state.request.RunID,
		Status:        "parsed_with_simulated_downstream",
		Source:        state.source,
		ParserName:    state.parserName,
		ParserVersion: state.parserVersion,
		ParsedUnits:   append([]ParsedUnit(nil), state.parsedUnits...),
		Stages:        append([]StageResult(nil), state.stages...),
		Placeholder:   state.placeholder,
	}, nil
}

func summarizeDocuments(documents []*schema.Document) []ParsedUnit {
	units := make([]ParsedUnit, 0, len(documents))
	for index, document := range documents {
		if document == nil {
			continue
		}
		unit := ParsedUnit{
			ID:         document.ID,
			Type:       metadataString(document.MetaData, "unit_type"),
			Index:      index + 1,
			Characters: utf8.RuneCountInString(document.Content),
			Preview:    preview(document.Content, 120),
			PageNumber: metadataInt(document.MetaData, "page_number"),
			SheetName:  metadataString(document.MetaData, "sheet_name"),
			RowNumber:  metadataInt(document.MetaData, "row_number"),
			Section:    metadataString(document.MetaData, docxparser.SectionTypeKey),
		}
		units = append(units, unit)
	}
	return units
}

func preview(content string, maxRunes int) string {
	content = strings.Join(strings.Fields(content), " ")
	runes := []rune(content)
	if len(runes) <= maxRunes {
		return content
	}
	return string(runes[:maxRunes]) + "..."
}

func metadataInt(metadata map[string]any, key string) int {
	switch value := metadata[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case string:
		parsed, _ := strconv.Atoi(value)
		return parsed
	default:
		return 0
	}
}

func contextError(ctx context.Context, operation string) error {
	if ctx == nil {
		return ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return nil
}
