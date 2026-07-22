package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/document/loader/file"
	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown"
)

const (
	indexGraphName = "rag_minimal_index"
	queryGraphName = "rag_minimal_query"

	nodeLoad          = "load_markdown"
	nodeSplit         = "split_markdown"
	nodePrepareChunks = "prepare_chunks"
	nodeIndex         = "index_chunks"
	nodeValidateQuery = "validate_question"
	nodeRetrieve      = "retrieve_chunks"
	nodeNoEvidence    = "no_evidence"
	nodeBuildMessages = "build_grounded_messages"
	nodeGenerate      = "generate_answer"
	nodeFinalize      = "finalize_answer"
	noEvidenceReply   = "没有检索到足够相关的资料，因此不生成推测性回答。"
)

type queryState struct {
	question string
	docs     []*schema.Document
}

type appComponents struct {
	loader      document.Loader
	transformer document.Transformer
	indexer     indexer.Indexer
	retriever   retriever.Retriever
	chatModel   model.BaseChatModel
}

// App owns the independent index and query graphs.
type App struct {
	indexRunnable compose.Runnable[document.Source, []string]
	queryRunnable compose.Runnable[QueryRequest, QueryResult]
	store         *MemoryVectorStore
	chatModel     *ExtractiveChatModel
}

// NewApp builds the fully offline RAG application.
func NewApp(ctx context.Context, config Config) (*App, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}
	embedder, err := NewHashingEmbedder(config.EmbeddingDimension)
	if err != nil {
		return nil, fmt.Errorf("create hashing embedder: %w", err)
	}
	store, err := NewMemoryVectorStore(
		embedder,
		config.EmbeddingDimension,
		config.TopK,
		config.ScoreThreshold,
	)
	if err != nil {
		return nil, fmt.Errorf("create memory vector store: %w", err)
	}
	loader, err := file.NewFileLoader(ctx, &file.FileLoaderConfig{
		UseNameAsID: true,
		Parser:      parser.TextParser{},
	})
	if err != nil {
		return nil, fmt.Errorf("create file loader: %w", err)
	}
	transformer, err := markdown.NewHeaderSplitter(ctx, &markdown.HeaderConfig{
		Headers: map[string]string{
			"#":   "heading_1",
			"##":  "heading_2",
			"###": "heading_3",
		},
		TrimHeaders: false,
	})
	if err != nil {
		return nil, fmt.Errorf("create markdown splitter: %w", err)
	}
	chatModel := &ExtractiveChatModel{}
	app, err := newAppWithComponents(ctx, appComponents{
		loader:      loader,
		transformer: transformer,
		indexer:     store,
		retriever:   store,
		chatModel:   chatModel,
	})
	if err != nil {
		return nil, err
	}
	app.store = store
	app.chatModel = chatModel
	return app, nil
}

func newAppWithComponents(ctx context.Context, components appComponents) (*App, error) {
	if components.loader == nil || components.transformer == nil || components.indexer == nil ||
		components.retriever == nil || components.chatModel == nil {
		return nil, fmt.Errorf("%w: all RAG components are required", ErrDependencyUnavailable)
	}
	indexRunnable, err := buildIndexGraph(ctx, components)
	if err != nil {
		return nil, err
	}
	queryRunnable, err := buildQueryGraph(ctx, components)
	if err != nil {
		return nil, err
	}
	return &App{indexRunnable: indexRunnable, queryRunnable: queryRunnable}, nil
}

// IndexFile loads, splits and indexes one local Markdown file.
func (a *App) IndexFile(
	ctx context.Context,
	path string,
	handlers ...callbacks.Handler,
) (IndexReport, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return IndexReport{}, ErrEmptySource
	}
	ids, err := invokeWithCallbacks(a.indexRunnable, ctx, document.Source{URI: path}, handlers)
	if err != nil {
		return IndexReport{}, fmt.Errorf("index markdown %q: %w", path, err)
	}
	return IndexReport{
		Source:    path,
		Documents: 1,
		Chunks:    len(ids),
		ChunkIDs:  append([]string(nil), ids...),
	}, nil
}

// Ask runs retrieval, the no-evidence branch and grounded generation.
func (a *App) Ask(
	ctx context.Context,
	question string,
	handlers ...callbacks.Handler,
) (QueryResult, error) {
	result, err := invokeWithCallbacks(a.queryRunnable, ctx, QueryRequest{Question: question}, handlers)
	if err != nil {
		return QueryResult{}, fmt.Errorf("answer question: %w", err)
	}
	return result, nil
}

// IndexedChunkCount reports the number of chunks held by the default store.
func (a *App) IndexedChunkCount() int {
	if a.store == nil {
		return 0
	}
	return a.store.Count()
}

// ChatModelCallCount reports generation calls made by the default model.
func (a *App) ChatModelCallCount() int64 {
	if a.chatModel == nil {
		return 0
	}
	return a.chatModel.CallCount()
}

func buildIndexGraph(
	ctx context.Context,
	components appComponents,
) (compose.Runnable[document.Source, []string], error) {
	graph := compose.NewGraph[document.Source, []string]()
	if err := graph.AddLoaderNode(nodeLoad, components.loader, compose.WithNodeName(nodeLoad)); err != nil {
		return nil, fmt.Errorf("add loader node: %w", err)
	}
	if err := graph.AddDocumentTransformerNode(nodeSplit, components.transformer, compose.WithNodeName(nodeSplit)); err != nil {
		return nil, fmt.Errorf("add splitter node: %w", err)
	}
	if err := graph.AddLambdaNode(
		nodePrepareChunks,
		compose.InvokableLambda(prepareChunks),
		compose.WithNodeName(nodePrepareChunks),
	); err != nil {
		return nil, fmt.Errorf("add chunk metadata node: %w", err)
	}
	if err := graph.AddIndexerNode(nodeIndex, components.indexer, compose.WithNodeName(nodeIndex)); err != nil {
		return nil, fmt.Errorf("add indexer node: %w", err)
	}
	for _, edge := range [][2]string{
		{compose.START, nodeLoad},
		{nodeLoad, nodeSplit},
		{nodeSplit, nodePrepareChunks},
		{nodePrepareChunks, nodeIndex},
		{nodeIndex, compose.END},
	} {
		if err := graph.AddEdge(edge[0], edge[1]); err != nil {
			return nil, fmt.Errorf("add index edge %s -> %s: %w", edge[0], edge[1], err)
		}
	}
	runnable, err := graph.Compile(ctx, compose.WithGraphName(indexGraphName))
	if err != nil {
		return nil, fmt.Errorf("compile index graph: %w", err)
	}
	return runnable, nil
}

func buildQueryGraph(
	ctx context.Context,
	components appComponents,
) (compose.Runnable[QueryRequest, QueryResult], error) {
	graph := compose.NewGraph[QueryRequest, QueryResult](
		compose.WithGenLocalState(func(context.Context) *queryState { return &queryState{} }),
	)
	if err := graph.AddLambdaNode(
		nodeValidateQuery,
		compose.InvokableLambda(validateQuestion),
		compose.WithNodeName(nodeValidateQuery),
		compose.WithStatePostHandler(func(_ context.Context, question string, state *queryState) (string, error) {
			state.question = question
			return question, nil
		}),
	); err != nil {
		return nil, fmt.Errorf("add question validator: %w", err)
	}
	if err := graph.AddRetrieverNode(
		nodeRetrieve,
		components.retriever,
		compose.WithNodeName(nodeRetrieve),
		compose.WithStatePostHandler(func(_ context.Context, docs []*schema.Document, state *queryState) ([]*schema.Document, error) {
			state.docs = cloneDocuments(docs)
			return docs, nil
		}),
	); err != nil {
		return nil, fmt.Errorf("add retriever node: %w", err)
	}
	if err := graph.AddLambdaNode(
		nodeNoEvidence,
		compose.InvokableLambda(noEvidenceResult),
		compose.WithNodeName(nodeNoEvidence),
	); err != nil {
		return nil, fmt.Errorf("add no-evidence node: %w", err)
	}
	if err := graph.AddLambdaNode(
		nodeBuildMessages,
		compose.InvokableLambda(buildGroundedMessages),
		compose.WithNodeName(nodeBuildMessages),
	); err != nil {
		return nil, fmt.Errorf("add message builder: %w", err)
	}
	if err := graph.AddChatModelNode(nodeGenerate, components.chatModel, compose.WithNodeName(nodeGenerate)); err != nil {
		return nil, fmt.Errorf("add chat model node: %w", err)
	}
	if err := graph.AddLambdaNode(
		nodeFinalize,
		compose.InvokableLambda(finalizeAnswer),
		compose.WithNodeName(nodeFinalize),
	); err != nil {
		return nil, fmt.Errorf("add answer finalizer: %w", err)
	}
	for _, edge := range [][2]string{
		{compose.START, nodeValidateQuery},
		{nodeValidateQuery, nodeRetrieve},
		{nodeNoEvidence, compose.END},
		{nodeBuildMessages, nodeGenerate},
		{nodeGenerate, nodeFinalize},
		{nodeFinalize, compose.END},
	} {
		if err := graph.AddEdge(edge[0], edge[1]); err != nil {
			return nil, fmt.Errorf("add query edge %s -> %s: %w", edge[0], edge[1], err)
		}
	}
	branch := compose.NewGraphBranch(
		func(_ context.Context, docs []*schema.Document) (string, error) {
			if len(docs) == 0 {
				return nodeNoEvidence, nil
			}
			return nodeBuildMessages, nil
		},
		map[string]bool{nodeNoEvidence: true, nodeBuildMessages: true},
	)
	if err := graph.AddBranch(nodeRetrieve, branch); err != nil {
		return nil, fmt.Errorf("add evidence branch: %w", err)
	}
	runnable, err := graph.Compile(ctx, compose.WithGraphName(queryGraphName))
	if err != nil {
		return nil, fmt.Errorf("compile query graph: %w", err)
	}
	return runnable, nil
}

func prepareChunks(ctx context.Context, docs []*schema.Document) ([]*schema.Document, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	prepared := make([]*schema.Document, 0, len(docs))
	for _, doc := range docs {
		if doc == nil || strings.TrimSpace(doc.Content) == "" {
			continue
		}
		copyDoc := cloneDocument(doc)
		if copyDoc.MetaData == nil {
			copyDoc.MetaData = make(map[string]any)
		}
		source := metadataString(copyDoc, file.MetaKeySource)
		heading := joinHeadings(copyDoc)
		chunkID := chunkID(source, heading, copyDoc.Content)
		copyDoc.ID = chunkID
		copyDoc.MetaData[metaSource] = source
		copyDoc.MetaData[metaHeading] = heading
		copyDoc.MetaData[metaChunkID] = chunkID
		prepared = append(prepared, copyDoc)
	}
	if len(prepared) == 0 {
		return nil, ErrNoChunks
	}
	return prepared, nil
}

func validateQuestion(ctx context.Context, request QueryRequest) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	question := strings.TrimSpace(request.Question)
	if question == "" {
		return "", ErrEmptyQuestion
	}
	return question, nil
}

func noEvidenceResult(ctx context.Context, _ []*schema.Document) (QueryResult, error) {
	if err := ctx.Err(); err != nil {
		return QueryResult{}, err
	}
	var question string
	if err := compose.ProcessState[*queryState](ctx, func(_ context.Context, state *queryState) error {
		question = state.question
		return nil
	}); err != nil {
		return QueryResult{}, fmt.Errorf("read query state: %w", err)
	}
	return QueryResult{Question: question, Answer: noEvidenceReply, NoEvidence: true}, nil
}

func buildGroundedMessages(ctx context.Context, docs []*schema.Document) ([]*schema.Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var question string
	if err := compose.ProcessState[*queryState](ctx, func(_ context.Context, state *queryState) error {
		question = state.question
		return nil
	}); err != nil {
		return nil, fmt.Errorf("read query state: %w", err)
	}

	var builder strings.Builder
	builder.WriteString("问题:\n")
	builder.WriteString(question)
	builder.WriteString("\n\n检索证据:\n")
	for i, doc := range docs {
		fmt.Fprintf(&builder, "\n[证据 %d]\n来源: %s\n标题: %s\n正文:\n%s\n",
			i+1,
			metadataString(doc, metaSource),
			metadataString(doc, metaHeading),
			strings.TrimSpace(doc.Content),
		)
	}
	return []*schema.Message{
		schema.SystemMessage("只能依据检索证据回答；不得补充证据之外的事实。"),
		schema.UserMessage(builder.String()),
	}, nil
}

func finalizeAnswer(ctx context.Context, response *schema.Message) (QueryResult, error) {
	if err := ctx.Err(); err != nil {
		return QueryResult{}, err
	}
	if response == nil || strings.TrimSpace(response.Content) == "" {
		return QueryResult{}, ErrEmptyModelResponse
	}
	var stateCopy queryState
	if err := compose.ProcessState[*queryState](ctx, func(_ context.Context, state *queryState) error {
		stateCopy.question = state.question
		stateCopy.docs = cloneDocuments(state.docs)
		return nil
	}); err != nil {
		return QueryResult{}, fmt.Errorf("read query state: %w", err)
	}
	return QueryResult{
		Question:  stateCopy.question,
		Answer:    strings.TrimSpace(response.Content),
		Retrieved: toRetrievedChunks(stateCopy.docs),
	}, nil
}

func toRetrievedChunks(docs []*schema.Document) []RetrievedChunk {
	result := make([]RetrievedChunk, 0, len(docs))
	for i, doc := range docs {
		if doc == nil {
			continue
		}
		result = append(result, RetrievedChunk{
			Rank:    i + 1,
			Score:   doc.Score(),
			Source:  metadataString(doc, metaSource),
			Heading: metadataString(doc, metaHeading),
			ChunkID: metadataString(doc, metaChunkID),
			Content: strings.TrimSpace(doc.Content),
		})
	}
	return result
}

func metadataString(doc *schema.Document, key string) string {
	if doc == nil || doc.MetaData == nil {
		return ""
	}
	value, _ := doc.MetaData[key].(string)
	return value
}

func joinHeadings(doc *schema.Document) string {
	parts := make([]string, 0, 3)
	for _, key := range []string{"heading_1", "heading_2", "heading_3"} {
		if value := metadataString(doc, key); value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, " / ")
}

func chunkID(source, heading, content string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(source) + "\x00" + heading + "\x00" + strings.TrimSpace(content)))
	return hex.EncodeToString(sum[:8])
}

func cloneDocuments(docs []*schema.Document) []*schema.Document {
	result := make([]*schema.Document, len(docs))
	for i, doc := range docs {
		result[i] = cloneDocument(doc)
	}
	return result
}

func validateConfig(config Config) error {
	if config.EmbeddingDimension <= 0 {
		return ErrInvalidDimension
	}
	if config.TopK <= 0 {
		return ErrInvalidTopK
	}
	if config.ScoreThreshold < -1 || config.ScoreThreshold > 1 {
		return ErrInvalidScoreThreshold
	}
	return nil
}

func invokeWithCallbacks[I, O any](
	runnable compose.Runnable[I, O],
	ctx context.Context,
	input I,
	handlers []callbacks.Handler,
) (O, error) {
	if len(handlers) == 0 {
		return runnable.Invoke(ctx, input)
	}
	return runnable.Invoke(ctx, input, compose.WithCallbacks(handlers...))
}
