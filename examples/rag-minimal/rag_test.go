package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/document/loader/file"
	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown"
)

func TestAppIndexesAndAnswersWithEvidence(t *testing.T) {
	ctx := context.Background()
	app, err := NewApp(ctx, DefaultConfig())
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	indexObserver := NewObserver()
	report, err := app.IndexFile(ctx, "testdata/personal-notes.md", indexObserver.Handler())
	if err != nil {
		t.Fatalf("IndexFile() error = %v", err)
	}
	if report.Documents != 1 || report.Chunks < 4 {
		t.Fatalf("IndexFile() report = %#v, want one document and at least four chunks", report)
	}
	if app.IndexedChunkCount() != report.Chunks {
		t.Fatalf("IndexedChunkCount() = %d, want %d", app.IndexedChunkCount(), report.Chunks)
	}

	queryObserver := NewObserver()
	result, err := app.Ask(ctx, "为什么 RAG 的索引写路径和查询读路径要分开？", queryObserver.Handler())
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if result.NoEvidence || len(result.Retrieved) == 0 {
		t.Fatalf("Ask() result = %#v, want retrieved evidence", result)
	}
	if app.ChatModelCallCount() != 1 {
		t.Fatalf("ChatModelCallCount() = %d, want 1", app.ChatModelCallCount())
	}
	if !strings.HasPrefix(result.Answer, "根据检索到的资料：") {
		t.Fatalf("Ask() answer = %q", result.Answer)
	}
	for i, chunk := range result.Retrieved {
		if chunk.Rank != i+1 || chunk.ChunkID == "" || chunk.Source == "" {
			t.Fatalf("Ask() retrieved[%d] = %#v", i, chunk)
		}
		if i > 0 && result.Retrieved[i-1].Score < chunk.Score {
			t.Fatalf("retrieval scores are not descending: %#v", result.Retrieved)
		}
	}
	if !hasSuccessfulComponent(indexObserver.Records(), "Embedding") {
		t.Fatalf("index callbacks = %#v, want successful Embedding callback", indexObserver.Records())
	}
	if !hasSuccessfulComponent(queryObserver.Records(), "Retriever") ||
		!hasSuccessfulComponent(queryObserver.Records(), "ChatModel") {
		t.Fatalf("query callbacks = %#v, want Retriever and ChatModel", queryObserver.Records())
	}
}

func TestIndexingTheSameFileKeepsStableChunkIDs(t *testing.T) {
	app, err := NewApp(context.Background(), DefaultConfig())
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	first, err := app.IndexFile(context.Background(), "testdata/personal-notes.md")
	if err != nil {
		t.Fatalf("first IndexFile() error = %v", err)
	}
	second, err := app.IndexFile(context.Background(), "testdata/personal-notes.md")
	if err != nil {
		t.Fatalf("second IndexFile() error = %v", err)
	}
	if strings.Join(first.ChunkIDs, ",") != strings.Join(second.ChunkIDs, ",") {
		t.Fatalf("chunk IDs changed: first=%v second=%v", first.ChunkIDs, second.ChunkIDs)
	}
	if app.IndexedChunkCount() != first.Chunks {
		t.Fatalf("duplicate indexing created extra chunks: got %d, want %d", app.IndexedChunkCount(), first.Chunks)
	}
}

func TestAskRejectsEmptyQuestionBeforeRetriever(t *testing.T) {
	rtr := &scriptedRetriever{}
	chatModel := &ExtractiveChatModel{}
	app := newQueryTestApp(t, rtr, chatModel, &scriptedIndexer{})

	_, err := app.Ask(context.Background(), " \n\t ")
	if !errors.Is(err, ErrEmptyQuestion) {
		t.Fatalf("Ask() error = %v, want ErrEmptyQuestion", err)
	}
	if rtr.calls.Load() != 0 {
		t.Fatalf("Retriever calls = %d, want 0", rtr.calls.Load())
	}
	if chatModel.CallCount() != 0 {
		t.Fatalf("ChatModel calls = %d, want 0", chatModel.CallCount())
	}
}

func TestNoEvidenceBranchSkipsChatModel(t *testing.T) {
	rtr := &scriptedRetriever{}
	chatModel := &ExtractiveChatModel{}
	app := newQueryTestApp(t, rtr, chatModel, &scriptedIndexer{})

	result, err := app.Ask(context.Background(), "资料中没有的问题")
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if !result.NoEvidence || result.Answer != noEvidenceReply {
		t.Fatalf("Ask() result = %#v, want no-evidence response", result)
	}
	if rtr.calls.Load() != 1 {
		t.Fatalf("Retriever calls = %d, want 1", rtr.calls.Load())
	}
	if chatModel.CallCount() != 0 {
		t.Fatalf("ChatModel calls = %d, want 0", chatModel.CallCount())
	}
}

func TestRetrieverDependencyErrorIsPreserved(t *testing.T) {
	dependencyErr := fmt.Errorf("%w: vector store offline", ErrDependencyUnavailable)
	rtr := &scriptedRetriever{err: dependencyErr}
	chatModel := &ExtractiveChatModel{}
	app := newQueryTestApp(t, rtr, chatModel, &scriptedIndexer{})

	_, err := app.Ask(context.Background(), "RAG 是什么？")
	if !errors.Is(err, ErrDependencyUnavailable) {
		t.Fatalf("Ask() error = %v, want ErrDependencyUnavailable", err)
	}
	if chatModel.CallCount() != 0 {
		t.Fatalf("ChatModel calls = %d, want 0", chatModel.CallCount())
	}
}

func TestIndexerDependencyErrorIsPreserved(t *testing.T) {
	dependencyErr := fmt.Errorf("%w: index store offline", ErrDependencyUnavailable)
	app := newQueryTestApp(t, &scriptedRetriever{}, &ExtractiveChatModel{}, &scriptedIndexer{err: dependencyErr})

	_, err := app.IndexFile(context.Background(), "testdata/personal-notes.md")
	if !errors.Is(err, ErrDependencyUnavailable) {
		t.Fatalf("IndexFile() error = %v, want ErrDependencyUnavailable", err)
	}
}

func TestEmbeddingTimeoutPropagatesThroughQueryGraph(t *testing.T) {
	base, err := NewHashingEmbedder(64)
	if err != nil {
		t.Fatalf("NewHashingEmbedder() error = %v", err)
	}
	embedder := &blockAfterFirstEmbedder{base: base}
	store, err := NewMemoryVectorStore(embedder, 64, 1, 0)
	if err != nil {
		t.Fatalf("NewMemoryVectorStore() error = %v", err)
	}
	doc := &schema.Document{
		ID:      "chunk-1",
		Content: "索引写路径和查询读路径应当分开。",
		MetaData: map[string]any{
			metaSource:  "test.md",
			metaHeading: "RAG",
			metaChunkID: "chunk-1",
		},
	}
	if _, err := store.Store(context.Background(), []*schema.Document{doc}); err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	app := newQueryTestApp(t, store, &ExtractiveChatModel{}, store)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = app.Ask(ctx, "为什么要分开？")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Ask() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestMemoryVectorStoreHonorsTopKAndThreshold(t *testing.T) {
	embedder := &mappedEmbedder{vectors: map[string][]float64{
		"apple":  {1, 0},
		"banana": {0, 1},
		"mix":    {0.7, 0.7},
	}}
	store, err := NewMemoryVectorStore(embedder, 2, 3, -1)
	if err != nil {
		t.Fatalf("NewMemoryVectorStore() error = %v", err)
	}
	docs := []*schema.Document{
		{ID: "apple", Content: "apple"},
		{ID: "banana", Content: "banana"},
		{ID: "mix", Content: "mix"},
	}
	if _, err := store.Store(context.Background(), docs); err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	result, err := store.Retrieve(
		context.Background(),
		"apple",
		retriever.WithTopK(2),
		retriever.WithScoreThreshold(0.5),
	)
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if len(result) != 2 || result[0].ID != "apple" || result[1].ID != "mix" {
		t.Fatalf("Retrieve() result = %#v, want apple then mix", result)
	}
}

func TestMemoryVectorStoreRejectsEmbeddingContractErrors(t *testing.T) {
	tests := []struct {
		name     string
		embedder embedding.Embedder
		want     error
	}{
		{name: "count", embedder: fixedEmbedder{vectors: nil}, want: ErrEmbeddingCountMismatch},
		{name: "dimension", embedder: fixedEmbedder{vectors: [][]float64{{1}}}, want: ErrEmbeddingDimension},
		{name: "zero", embedder: fixedEmbedder{vectors: [][]float64{{0, 0}}}, want: ErrZeroEmbedding},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, err := NewMemoryVectorStore(test.embedder, 2, 1, 0)
			if err != nil {
				t.Fatalf("NewMemoryVectorStore() error = %v", err)
			}
			_, err = store.Store(context.Background(), []*schema.Document{{ID: "doc", Content: "content"}})
			if !errors.Is(err, test.want) {
				t.Fatalf("Store() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestConcurrentQueriesKeepLocalStateIsolated(t *testing.T) {
	app, err := NewApp(context.Background(), DefaultConfig())
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	if _, err := app.IndexFile(context.Background(), "testdata/personal-notes.md"); err != nil {
		t.Fatalf("IndexFile() error = %v", err)
	}

	questions := []string{
		"当前使用哪个 Eino 版本？",
		"第一版 RAG 包含哪些能力？",
		"后续迁移会替换哪个组件？",
		"为什么写路径和读路径分开？",
	}
	var wait sync.WaitGroup
	errorsCh := make(chan error, len(questions)*4)
	for i := 0; i < 4; i++ {
		for _, question := range questions {
			question := question
			wait.Add(1)
			go func() {
				defer wait.Done()
				result, err := app.Ask(context.Background(), question)
				if err != nil {
					errorsCh <- err
					return
				}
				if result.Question != question {
					errorsCh <- fmt.Errorf("question leaked across state: got %q, want %q", result.Question, question)
				}
			}()
		}
	}
	wait.Wait()
	close(errorsCh)
	for err := range errorsCh {
		t.Error(err)
	}
}

func newQueryTestApp(
	t *testing.T,
	rtr retriever.Retriever,
	chatModel model.BaseChatModel,
	idx indexer.Indexer,
) *App {
	t.Helper()
	ctx := context.Background()
	loader, err := file.NewFileLoader(ctx, &file.FileLoaderConfig{
		UseNameAsID: true,
		Parser:      parser.TextParser{},
	})
	if err != nil {
		t.Fatalf("NewFileLoader() error = %v", err)
	}
	transformer, err := markdown.NewHeaderSplitter(ctx, &markdown.HeaderConfig{
		Headers: map[string]string{"#": "heading_1", "##": "heading_2", "###": "heading_3"},
	})
	if err != nil {
		t.Fatalf("NewHeaderSplitter() error = %v", err)
	}
	app, err := newAppWithComponents(ctx, appComponents{
		loader:      loader,
		transformer: transformer,
		indexer:     idx,
		retriever:   rtr,
		chatModel:   chatModel,
	})
	if err != nil {
		t.Fatalf("newAppWithComponents() error = %v", err)
	}
	return app
}

func hasSuccessfulComponent(records []CallbackRecord, component string) bool {
	for _, record := range records {
		if record.Component == component && record.Status == "succeeded" {
			return true
		}
	}
	return false
}

type scriptedRetriever struct {
	docs  []*schema.Document
	err   error
	calls atomic.Int64
}

func (r *scriptedRetriever) Retrieve(
	ctx context.Context,
	_ string,
	_ ...retriever.Option,
) ([]*schema.Document, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.calls.Add(1)
	return cloneDocuments(r.docs), r.err
}

type scriptedIndexer struct {
	err error
}

func (i *scriptedIndexer) Store(
	ctx context.Context,
	docs []*schema.Document,
	_ ...indexer.Option,
) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if i.err != nil {
		return nil, i.err
	}
	ids := make([]string, len(docs))
	for index, doc := range docs {
		ids[index] = doc.ID
	}
	return ids, nil
}

type blockAfterFirstEmbedder struct {
	base  embedding.Embedder
	calls atomic.Int64
}

func (e *blockAfterFirstEmbedder) EmbedStrings(
	ctx context.Context,
	texts []string,
	opts ...embedding.Option,
) ([][]float64, error) {
	if e.calls.Add(1) == 1 {
		return e.base.EmbedStrings(ctx, texts, opts...)
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

type mappedEmbedder struct {
	vectors map[string][]float64
}

func (e *mappedEmbedder) EmbedStrings(
	ctx context.Context,
	texts []string,
	_ ...embedding.Option,
) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i, text := range texts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result[i] = append([]float64(nil), e.vectors[text]...)
	}
	return result, nil
}

type fixedEmbedder struct {
	vectors [][]float64
}

func (e fixedEmbedder) EmbedStrings(
	context.Context,
	[]string,
	...embedding.Option,
) ([][]float64, error) {
	return e.vectors, nil
}

var _ document.Loader = (*file.FileLoader)(nil)
var _ embedding.Embedder = (*HashingEmbedder)(nil)
var _ indexer.Indexer = (*MemoryVectorStore)(nil)
var _ retriever.Retriever = (*MemoryVectorStore)(nil)
var _ model.BaseChatModel = (*ExtractiveChatModel)(nil)
