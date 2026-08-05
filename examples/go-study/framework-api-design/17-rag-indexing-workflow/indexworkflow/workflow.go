package indexworkflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"
)

const (
	workflowName    = "rag_document_indexing"
	workflowVersion = "v1"

	nodeInspectSource = "inspect_source"
	nodeParseDocument = "parse_document"
	nodeChunkDocument = "chunk_document"
	nodeEmbedChunks   = "embed_chunks"
	nodePersistIndex  = "persist_index"
	nodeValidateIndex = "validate_index"
	nodePublishIndex  = "publish_index"
	nodeBuildResult   = "build_result"
)

// Config 保存文档索引工作流的资源边界。
type Config struct {
	MaxFileBytes int64
}

// DefaultConfig 返回适合本地验证的默认值。
func DefaultConfig() Config {
	return Config{MaxFileBytes: 32 << 20}
}

// Workflow 保存编译一次、重复执行的 Eino 索引工作流。
type Workflow struct {
	runnable compose.Runnable[Request, Result]
}

// Descriptor 返回稳定工作流名称和版本。
func Descriptor() string {
	return workflowName + "@" + workflowVersion
}

// New 创建真实 Parser 依赖并编译完整索引拓扑。
func New(ctx context.Context, config Config) (*Workflow, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}
	if config.MaxFileBytes < 1 {
		return nil, fmt.Errorf("%w: MaxFileBytes 必须大于 0", ErrInvalidConfig)
	}
	parsers, err := newParserService(ctx)
	if err != nil {
		return nil, err
	}
	handlers := &workflowHandlers{config: config, parsers: parsers}
	chain := compose.NewChain[Request, Result]()
	chain.
		AppendLambda(
			compose.InvokableLambda(handlers.inspect),
			compose.WithNodeKey(nodeInspectSource),
			compose.WithNodeName(nodeInspectSource),
		).
		AppendLambda(
			compose.InvokableLambda(handlers.parse),
			compose.WithNodeKey(nodeParseDocument),
			compose.WithNodeName(nodeParseDocument),
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
