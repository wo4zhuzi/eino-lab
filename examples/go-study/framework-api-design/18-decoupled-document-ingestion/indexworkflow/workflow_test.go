package indexworkflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	ingestion "github.com/wo4zhuzi/eino-document-ingestion"
)

type stubIngestor struct {
	result   *ingestion.Result
	err      error
	received string
}

func (s *stubIngestor) Ingest(_ context.Context, uri string) (*ingestion.Result, error) {
	s.received = uri
	return s.result, s.err
}

func TestWorkflowUsesInjectedIngestorAndAddsIndexMetadata(t *testing.T) {
	document := &schema.Document{
		Content:  "第一页内容",
		MetaData: map[string]any{"page_number": 1, "origin": "ingestion"},
	}
	stub := &stubIngestor{result: &ingestion.Result{
		Source: ingestion.SourceInfo{
			URI:       "https://example.com/guide.pdf",
			FileName:  "guide.pdf",
			Extension: ingestion.ExtensionPDF,
			MIMEType:  "application/pdf",
			SizeBytes: 18,
			SHA256:    strings.Repeat("a", 64),
		},
		Parser:    ingestion.ParserInfo{Name: "eino_ext_pdf", Version: "parser-version"},
		Documents: []*schema.Document{document},
	}}
	workflow := newWorkflowWithIngestor(t, stub)

	result, err := workflow.Run(context.Background(), Request{
		RunID:     "  injected-run  ",
		SourceURI: "  https://example.com/guide.pdf  ",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stub.received != "https://example.com/guide.pdf" || result.RunID != "injected-run" {
		t.Fatalf("received=%q result.RunID=%q", stub.received, result.RunID)
	}
	if result.Workflow != "rag_document_indexing@v2" || result.ParserName != "eino_ext_pdf" {
		t.Fatalf("result = %#v", result)
	}
	if result.Source.DocumentID == "" || result.Source.VersionID != result.Source.SHA256 {
		t.Fatalf("Source = %#v", result.Source)
	}
	if len(result.ParsedUnits) != 1 || result.ParsedUnits[0].Type != "page" ||
		result.ParsedUnits[0].PageNumber != 1 || result.ParsedUnits[0].ID == "" {
		t.Fatalf("ParsedUnits = %#v", result.ParsedUnits)
	}
	if document.ID != "" || len(document.MetaData) != 2 {
		t.Fatalf("摄取组件返回的 Document 被工作流修改: %#v", document)
	}
	assertStageStatuses(t, result.Stages)
}

func TestWorkflowIntegratesRealIngestorForMarkdown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "knowledge.md")
	if err := os.WriteFile(path, []byte("# Eino\n\n独立摄取组件。\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	ingestor, err := ingestion.New(context.Background(), ingestion.Config{
		MaxFileBytes: ingestion.DefaultMaxFileBytes,
	})
	if err != nil {
		t.Fatalf("ingestion.New() error = %v", err)
	}
	workflow, err := New(context.Background(), Dependencies{Ingestor: ingestor})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	result, err := workflow.Run(context.Background(), Request{RunID: "integration", SourceURI: path})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ParserName != "eino_text_markdown" || len(result.ParsedUnits) != 1 ||
		!strings.Contains(result.ParsedUnits[0].Preview, "独立摄取组件") {
		t.Fatalf("result = %#v", result)
	}
}

func TestWorkflowPreservesIngestionAndContextErrors(t *testing.T) {
	stub := &stubIngestor{err: ingestion.ErrUnsupportedFormat}
	workflow := newWorkflowWithIngestor(t, stub)
	_, err := workflow.Run(context.Background(), Request{RunID: "run", SourceURI: "document.csv"})
	if !errors.Is(err, ingestion.ErrUnsupportedFormat) {
		t.Fatalf("Run() error = %v", err)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	stub.err = context.Canceled
	_, err = workflow.Run(canceled, Request{RunID: "run", SourceURI: "document.md"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run(canceled) error = %v", err)
	}
}

func TestWorkflowValidatesBoundary(t *testing.T) {
	stub := &stubIngestor{}
	workflow := newWorkflowWithIngestor(t, stub)
	if _, err := workflow.Run(context.Background(), Request{SourceURI: "document.md"}); !errors.Is(err, ErrInvalidRunID) {
		t.Fatalf("Run(empty run ID) error = %v", err)
	}
	stub.result = nil
	if _, err := workflow.Run(context.Background(), Request{RunID: "run", SourceURI: "document.md"}); !errors.Is(err, ingestion.ErrNoParsedContent) {
		t.Fatalf("Run(nil result) error = %v", err)
	}
	if _, err := New(nil, Dependencies{Ingestor: stub}); !errors.Is(err, ErrNilContext) {
		t.Fatalf("New(nil) error = %v", err)
	}
	if _, err := New(context.Background(), Dependencies{}); !errors.Is(err, ErrInvalidDependencies) {
		t.Fatalf("New(invalid dependencies) error = %v", err)
	}
	var unavailable *Workflow
	if _, err := unavailable.Run(context.Background(), Request{}); !errors.Is(err, ErrWorkflowUnavailable) {
		t.Fatalf("nil Workflow.Run() error = %v", err)
	}
}

func newWorkflowWithIngestor(t *testing.T, ingestor Ingestor) *Workflow {
	t.Helper()
	workflow, err := New(context.Background(), Dependencies{Ingestor: ingestor})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return workflow
}

func assertStageStatuses(t *testing.T, stages []StageResult) {
	t.Helper()
	wantNames := []string{
		nodeIngestDocument,
		nodeChunkDocument,
		nodeEmbedChunks,
		nodePersistIndex,
		nodeValidateIndex,
		nodePublishIndex,
		nodeBuildResult,
	}
	if len(stages) != len(wantNames) {
		t.Fatalf("stages = %#v", stages)
	}
	for index, name := range wantNames {
		status := StageStatusSimulated
		if name == nodeIngestDocument || name == nodeBuildResult {
			status = StageStatusCompleted
		}
		if stages[index].Name != name || stages[index].Status != status {
			t.Fatalf("stage[%d] = %#v, want name=%s status=%s", index, stages[index], name, status)
		}
	}
}
