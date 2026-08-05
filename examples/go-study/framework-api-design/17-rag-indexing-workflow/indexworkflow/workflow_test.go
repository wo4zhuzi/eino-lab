package indexworkflow

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestWorkflowParsesMarkdownAndKeepsDownstreamExplicitlySimulated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "knowledge.md")
	writeFile(t, path, []byte("# Eino\n\n索引工作流先解析文档。\n"))

	workflow := newTestWorkflow(t)
	result, err := workflow.Run(context.Background(), Request{
		RunID:     "markdown-run",
		SourceURI: path,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != "parsed_with_simulated_downstream" || result.ParserName != "eino_text_markdown" {
		t.Fatalf("result = %#v", result)
	}
	if len(result.ParsedUnits) != 1 || !strings.Contains(result.ParsedUnits[0].Preview, "索引工作流") {
		t.Fatalf("ParsedUnits = %#v", result.ParsedUnits)
	}
	if result.Source.SHA256 == "" || result.Source.VersionID != result.Source.SHA256 {
		t.Fatalf("Source = %#v", result.Source)
	}
	assertStageStatuses(t, result.Stages)
	if !result.Placeholder.Simulated || result.Placeholder.PersistedRecordCount != 0 ||
		result.Placeholder.PublishedIndexVersion != "" {
		t.Fatalf("Placeholder = %#v", result.Placeholder)
	}
}

func TestWorkflowParsesSupportedFormats(t *testing.T) {
	tests := []struct {
		name       string
		extension  string
		write      func(*testing.T, string)
		parserName string
		unitType   string
		minUnits   int
	}{
		{
			name:      "text",
			extension: ".txt",
			write: func(t *testing.T, path string) {
				writeFile(t, path, []byte("纯文本资料"))
			},
			parserName: "eino_text",
			unitType:   "document",
			minUnits:   1,
		},
		{
			name:       "pdf",
			extension:  ".pdf",
			write:      writeMinimalPDF,
			parserName: "eino_ext_pdf",
			unitType:   "page",
			minUnits:   1,
		},
		{
			name:       "docx",
			extension:  ".docx",
			write:      writeMinimalDOCX,
			parserName: "eino_ext_docx",
			unitType:   "section",
			minUnits:   1,
		},
		{
			name:       "xlsx all sheets",
			extension:  ".xlsx",
			write:      writeXLSX,
			parserName: "excelize_xlsx_all_sheets",
			unitType:   "row",
			minUnits:   2,
		},
	}

	workflow := newTestWorkflow(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "document"+test.extension)
			test.write(t, path)
			result, err := workflow.Run(context.Background(), Request{
				RunID:     "run-" + test.name,
				SourceURI: path,
			})
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if result.ParserName != test.parserName || len(result.ParsedUnits) < test.minUnits {
				t.Fatalf("result = %#v", result)
			}
			for _, unit := range result.ParsedUnits {
				if unit.Type != test.unitType || unit.ID == "" || unit.Characters == 0 {
					t.Fatalf("unit = %#v", unit)
				}
			}
			if test.extension == ".xlsx" {
				sheets := map[string]bool{}
				for _, unit := range result.ParsedUnits {
					sheets[unit.SheetName] = true
				}
				if !sheets["产品"] || !sheets["库存"] {
					t.Fatalf("XLSX sheets = %#v", sheets)
				}
			}
		})
	}
}

func TestWorkflowRejectsInvalidInputsAndPreservesContextError(t *testing.T) {
	workflow := newTestWorkflow(t)
	tempDir := t.TempDir()
	unsupported := filepath.Join(tempDir, "data.csv")
	writeFile(t, unsupported, []byte("a,b\n1,2\n"))
	fakePDF := filepath.Join(tempDir, "fake.pdf")
	writeFile(t, fakePDF, []byte("not a pdf"))
	malformedDOCX := filepath.Join(tempDir, "malformed.docx")
	writeDOCX(t, malformedDOCX, false)
	oversized := filepath.Join(tempDir, "large.txt")
	writeFile(t, oversized, []byte("too large"))

	tests := []struct {
		name    string
		ctx     context.Context
		request Request
		want    error
	}{
		{name: "missing run id", ctx: context.Background(), request: Request{SourceURI: unsupported}, want: ErrInvalidRunID},
		{name: "missing source", ctx: context.Background(), request: Request{RunID: "run"}, want: ErrEmptySourceURI},
		{name: "unsupported", ctx: context.Background(), request: Request{RunID: "run", SourceURI: unsupported}, want: ErrUnsupportedFormat},
		{name: "invalid signature", ctx: context.Background(), request: Request{RunID: "run", SourceURI: fakePDF}, want: ErrInvalidFileFormat},
		{name: "parser panic isolated", ctx: context.Background(), request: Request{RunID: "run", SourceURI: malformedDOCX}, want: ErrInvalidFileFormat},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := workflow.Run(test.ctx, test.request)
			if !errors.Is(err, test.want) {
				t.Fatalf("Run() error = %v, want %v", err, test.want)
			}
		})
	}

	smallWorkflow, err := New(context.Background(), Config{MaxFileBytes: 2})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := smallWorkflow.Run(context.Background(), Request{RunID: "run", SourceURI: oversized}); !errors.Is(err, ErrSourceTooLarge) {
		t.Fatalf("Run(oversized) error = %v", err)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = workflow.Run(canceled, Request{RunID: "run", SourceURI: unsupported})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run(canceled) error = %v", err)
	}
}

func TestNewValidatesConfiguration(t *testing.T) {
	if _, err := New(nil, DefaultConfig()); !errors.Is(err, ErrNilContext) {
		t.Fatalf("New(nil) error = %v", err)
	}
	if _, err := New(context.Background(), Config{}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("New(invalid config) error = %v", err)
	}
	var workflow *Workflow
	if _, err := workflow.Run(context.Background(), Request{}); !errors.Is(err, ErrWorkflowUnavailable) {
		t.Fatalf("nil Workflow.Run() error = %v", err)
	}
}

func newTestWorkflow(t *testing.T) *Workflow {
	t.Helper()
	workflow, err := New(context.Background(), DefaultConfig())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return workflow
}

func assertStageStatuses(t *testing.T, stages []StageResult) {
	t.Helper()
	wantNames := []string{
		nodeInspectSource,
		nodeParseDocument,
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
	for index, wantName := range wantNames {
		wantStatus := StageStatusSimulated
		if wantName == nodeInspectSource || wantName == nodeParseDocument || wantName == nodeBuildResult {
			wantStatus = StageStatusCompleted
		}
		if stages[index].Name != wantName || stages[index].Status != wantStatus {
			t.Fatalf("stage[%d] = %#v, want name=%s status=%s", index, stages[index], wantName, wantStatus)
		}
	}
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
}

func writeMinimalPDF(t *testing.T, path string) {
	t.Helper()
	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.4\n")
	offsets := make([]int, 6)
	objects := map[int]string{
		1: "<< /Type /Catalog /Pages 2 0 R >>",
		2: "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		3: "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		4: "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	for id := 1; id <= 4; id++ {
		offsets[id] = pdf.Len()
		fmt.Fprintf(&pdf, "%d 0 obj\n%s\nendobj\n", id, objects[id])
	}
	stream := "BT /F1 12 Tf 72 720 Td (Demo PDF content) Tj ET"
	offsets[5] = pdf.Len()
	fmt.Fprintf(&pdf, "5 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", len(stream), stream)
	xrefOffset := pdf.Len()
	pdf.WriteString("xref\n0 6\n0000000000 65535 f \n")
	for id := 1; id <= 5; id++ {
		fmt.Fprintf(&pdf, "%010d 00000 n \n", offsets[id])
	}
	fmt.Fprintf(&pdf, "trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xrefOffset)
	writeFile(t, path, pdf.Bytes())
}

func writeMinimalDOCX(t *testing.T, path string) {
	writeDOCX(t, path, true)
}

func writeDOCX(t *testing.T, path string, includeStyles bool) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("os.Create(%q) error = %v", path, err)
	}
	archive := zip.NewWriter(file)
	entries := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body><w:p><w:r><w:t>Demo DOCX content</w:t></w:r></w:p><w:sectPr/></w:body>
</w:document>`,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
		"word/styles.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"></w:styles>`,
	}
	names := []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml", "word/_rels/document.xml.rels"}
	if includeStyles {
		names = append(names, "word/styles.xml")
	}
	for _, name := range names {
		writer, err := archive.Create(name)
		if err != nil {
			t.Fatalf("archive.Create(%q) error = %v", name, err)
		}
		if _, err := writer.Write([]byte(entries[name])); err != nil {
			t.Fatalf("write DOCX entry %q error = %v", name, err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close DOCX archive error = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close DOCX file error = %v", err)
	}
}

func writeXLSX(t *testing.T, path string) {
	t.Helper()
	workbook := excelize.NewFile()
	if err := workbook.SetSheetName("Sheet1", "产品"); err != nil {
		t.Fatalf("SetSheetName() error = %v", err)
	}
	for cell, value := range map[string]any{"A1": "名称", "B1": "价格", "A2": "键盘", "B2": 399} {
		if err := workbook.SetCellValue("产品", cell, value); err != nil {
			t.Fatalf("SetCellValue(产品, %s) error = %v", cell, err)
		}
	}
	if _, err := workbook.NewSheet("库存"); err != nil {
		t.Fatalf("NewSheet() error = %v", err)
	}
	for cell, value := range map[string]any{"A1": "仓库", "B1": "数量", "A2": "上海", "B2": 20} {
		if err := workbook.SetCellValue("库存", cell, value); err != nil {
			t.Fatalf("SetCellValue(库存, %s) error = %v", cell, err)
		}
	}
	if err := workbook.SaveAs(path); err != nil {
		t.Fatalf("SaveAs() error = %v", err)
	}
	if err := workbook.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
