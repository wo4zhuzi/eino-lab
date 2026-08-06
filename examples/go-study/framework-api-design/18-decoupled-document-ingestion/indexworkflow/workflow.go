package indexworkflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"
)

const (
	workflowName    = "rag_document_indexing"
	workflowVersion = "v2"

	nodeIngestDocument = "ingest_document"
	nodeChunkDocument  = "chunk_document"
	nodeEmbedChunks    = "embed_chunks"
	nodePersistIndex   = "persist_index"
	nodeValidateIndex  = "validate_index"
	nodePublishIndex   = "publish_index"
	nodeBuildResult    = "build_result"
)

// Workflow 保存编译一次、重复执行的 Eino 索引工作流。
type Workflow struct {
	runnable compose.Runnable[Request, Result]
}

// Descriptor 返回稳定工作流名称和版本。
func Descriptor() string {
	return workflowName + "@" + workflowVersion
}

// New 使用应用启动层提供的依赖编译完整索引拓扑。
func New(ctx context.Context, dependencies Dependencies) (*Workflow, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}
	if dependencies.Ingestor == nil {
		return nil, fmt.Errorf("%w: Ingestor 不能为空", ErrInvalidDependencies)
	}

	handlers := &workflowHandlers{ingestor: dependencies.Ingestor}
	chain := compose.NewChain[Request, Result]()
	chain.
		AppendLambda(
			compose.InvokableLambda(handlers.ingest),
			compose.WithNodeKey(nodeIngestDocument),
			compose.WithNodeName(nodeIngestDocument),
		).
		AppendLambda(
			compose.InvokableLambda(handlers.simulateChunk),
			compose.WithNodeKey(nodeChunkDocument),
			compose.WithNodeName(nodeChunkDocument),
		).
		AppendLambda(
			compose.InvokableLambda(handlers.simulateEmbedding),
			compose.WithNodeKey(nodeEmbedChunks),
			compose.WithNodeName(nodeEmbedChunks),
		).
		AppendLambda(
			compose.InvokableLambda(handlers.simulatePersist),
			compose.WithNodeKey(nodePersistIndex),
			compose.WithNodeName(nodePersistIndex),
		).
		AppendLambda(
			compose.InvokableLambda(handlers.simulateValidate),
			compose.WithNodeKey(nodeValidateIndex),
			compose.WithNodeName(nodeValidateIndex),
		).
		AppendLambda(
			compose.InvokableLambda(handlers.simulatePublish),
			compose.WithNodeKey(nodePublishIndex),
			compose.WithNodeName(nodePublishIndex),
		).
		AppendLambda(
			compose.InvokableLambda(handlers.buildResult),
			compose.WithNodeKey(nodeBuildResult),
			compose.WithNodeName(nodeBuildResult),
		)
	runnable, err := chain.Compile(ctx, compose.WithGraphName(Descriptor()))
	if err != nil {
		return nil, fmt.Errorf("编译索引工作流: %w", err)
	}
	return &Workflow{runnable: runnable}, nil
}

// Run 执行一次完整索引拓扑。
func (w *Workflow) Run(ctx context.Context, request Request) (Result, error) {
	if ctx == nil {
		return Result{}, ErrNilContext
	}
	if w == nil || w.runnable == nil {
		return Result{}, ErrWorkflowUnavailable
	}
	request.RunID = strings.TrimSpace(request.RunID)
	request.SourceURI = strings.TrimSpace(request.SourceURI)
	result, err := w.runnable.Invoke(ctx, request)
	if err != nil {
		return Result{}, fmt.Errorf("运行索引工作流 %s run_id=%q: %w", Descriptor(), request.RunID, err)
	}
	return result, nil
}
