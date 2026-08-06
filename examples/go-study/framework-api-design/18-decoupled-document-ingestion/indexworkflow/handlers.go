package indexworkflow

import (
	"context"
	"fmt"

	ingestion "github.com/wo4zhuzi/eino-document-ingestion"
)

type workflowHandlers struct {
	ingestor DocumentIngestor
}

func (h *workflowHandlers) ingest(ctx context.Context, request Request) (workflowState, error) {
	if request.RunID == "" {
		return workflowState{}, fmt.Errorf("%s: %w", nodeIngestDocument, ErrInvalidRunID)
	}
	result, err := h.ingestor.Ingest(ctx, request.SourceURI)
	if err != nil {
		return workflowState{}, fmt.Errorf("%s: %w", nodeIngestDocument, err)
	}
	if result == nil {
		return workflowState{}, fmt.Errorf("%s: %w: 摄取器返回 nil 结果", nodeIngestDocument, ingestion.ErrNoParsedContent)
	}

	source, documents, err := prepareForIndexing(result)
	if err != nil {
		return workflowState{}, fmt.Errorf("%s: %w", nodeIngestDocument, err)
	}
	return workflowState{
		request:       request,
		source:        source,
		parserName:    result.Parser.Name,
		parserVersion: result.Parser.Version,
		documents:     documents,
		parsedUnits:   summarizeDocuments(documents),
		stages: []StageResult{{
			Name:   nodeIngestDocument,
			Status: StageStatusCompleted,
			Detail: fmt.Sprintf("独立摄取组件已完成加载、校验和解析，共 %d 个标准化单元", len(documents)),
		}},
	}, nil
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
		Status:        "ingested_with_simulated_downstream",
		Source:        state.source,
		ParserName:    state.parserName,
		ParserVersion: state.parserVersion,
		ParsedUnits:   append([]ParsedUnit(nil), state.parsedUnits...),
		Stages:        append([]StageResult(nil), state.stages...),
		Placeholder:   state.placeholder,
	}, nil
}
