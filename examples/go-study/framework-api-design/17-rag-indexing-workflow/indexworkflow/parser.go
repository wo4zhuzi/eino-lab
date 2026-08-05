package indexworkflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	fileloader "github.com/cloudwego/eino-ext/components/document/loader/file"
	docxparser "github.com/cloudwego/eino-ext/components/document/parser/docx"
	pdfparser "github.com/cloudwego/eino-ext/components/document/parser/pdf"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
	"github.com/xuri/excelize/v2"
)

const (
	textParserVersion = "eino-text@v0.9.12"
	extParserVersion  = "eino-ext@90a15623ddb6"
	xlsxParserVersion = "excelize@v2.9.0"
)

type parserRegistration struct {
	name    string
	version string
	parser  parser.Parser
}

type parserService struct {
	loader        document.Loader
	registrations map[string]parserRegistration
}

type strictParserRouter struct {
	registrations map[string]parserRegistration
}

func newParserService(ctx context.Context) (*parserService, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}
	pdf, err := pdfparser.NewPDFParser(ctx, &pdfparser.Config{ToPages: true})
	if err != nil {
		return nil, fmt.Errorf("创建 PDF Parser: %w", err)
	}
	docx, err := docxparser.NewDocxParser(ctx, &docxparser.Config{
		ToSections:     true,
		IncludeHeaders: true,
		IncludeFooters: true,
		IncludeTables:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 DOCX Parser: %w", err)
	}

	registrations := map[string]parserRegistration{
		".md": {
			name:    "eino_text_markdown",
			version: textParserVersion,
			parser:  strictTextParser{},
		},
		".txt": {
			name:    "eino_text",
			version: textParserVersion,
			parser:  strictTextParser{},
		},
		".pdf": {
			name:    "eino_ext_pdf",
			version: extParserVersion,
			parser:  recoveringParser{name: "pdf", delegate: pdf},
		},
		".docx": {
			name:    "eino_ext_docx",
			version: extParserVersion,
			parser:  recoveringParser{name: "docx", delegate: docx},
		},
		".xlsx": {
			name:    "excelize_xlsx_all_sheets",
			version: xlsxParserVersion,
			parser: recoveringParser{
				name:     "xlsx",
				delegate: allSheetsXLSXParser{},
			},
		},
	}
	loader, err := fileloader.NewFileLoader(ctx, &fileloader.FileLoaderConfig{
		Parser: &strictParserRouter{registrations: registrations},
	})
	if err != nil {
		return nil, fmt.Errorf("创建 Eino FileLoader: %w", err)
	}
	return &parserService{loader: loader, registrations: registrations}, nil
}

func (s *parserService) parse(ctx context.Context, source SourceInfo) (parseResult, error) {
	if err := contextError(ctx, "解析文档"); err != nil {
		return parseResult{}, err
	}
	registration, ok := s.registrations[source.Extension]
	if !ok {
		return parseResult{}, fmt.Errorf("%w: %s", ErrUnsupportedFormat, source.Extension)
	}
	documents, err := s.loader.Load(ctx, document.Source{URI: source.URI})
	if err != nil {
		return parseResult{}, fmt.Errorf("调用 Eino FileLoader: %w", err)
	}
	documents, err = normalizeDocuments(source, documents)
	if err != nil {
		return parseResult{}, err
	}
	return parseResult{
		parserName:    registration.name,
		parserVersion: registration.version,
		documents:     documents,
	}, nil
}

func (r *strictParserRouter) Parse(
	ctx context.Context,
	reader io.Reader,
	opts ...parser.Option,
) ([]*schema.Document, error) {
	if err := contextError(ctx, "选择文档 Parser"); err != nil {
		return nil, err
	}
	options := parser.GetCommonOptions(&parser.Options{}, opts...)
	extension := strings.ToLower(filepath.Ext(options.URI))
	registration, ok := r.registrations[extension]
	if !ok || registration.parser == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, extension)
	}
	return registration.parser.Parse(ctx, reader, opts...)
}

type strictTextParser struct{}

func (strictTextParser) Parse(
	ctx context.Context,
	reader io.Reader,
	opts ...parser.Option,
) ([]*schema.Document, error) {
	if err := contextError(ctx, "读取文本内容"); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("读取文本内容: %w", err)
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("%w: 文本不是有效 UTF-8", ErrInvalidFileFormat)
	}
	options := parser.GetCommonOptions(&parser.Options{}, opts...)
	metadata := cloneMetadata(options.ExtraMeta)
	metadata[parser.MetaKeySource] = options.URI
	return []*schema.Document{{Content: string(data), MetaData: metadata}}, nil
}

type recoveringParser struct {
	name     string
	delegate parser.Parser
}

func (p recoveringParser) Parse(
	ctx context.Context,
	reader io.Reader,
	opts ...parser.Option,
) (documents []*schema.Document, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			documents = nil
			err = fmt.Errorf(
				"%w: %s Parser panic: %v",
				ErrInvalidFileFormat,
				p.name,
				recovered,
			)
		}
	}()
	return p.delegate.Parse(ctx, reader, opts...)
}

type allSheetsXLSXParser struct{}

func (allSheetsXLSXParser) Parse(
	ctx context.Context,
	reader io.Reader,
	opts ...parser.Option,
) ([]*schema.Document, error) {
	if err := contextError(ctx, "读取 XLSX 内容"); err != nil {
		return nil, err
	}
	workbook, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, fmt.Errorf("读取 XLSX 工作簿: %w", err)
	}
	defer workbook.Close()
	commonOptions := parser.GetCommonOptions(&parser.Options{}, opts...)

	var documents []*schema.Document
	for _, sheetName := range workbook.GetSheetList() {
		if err := contextError(ctx, "解析 XLSX 工作表"); err != nil {
			return nil, err
		}
		rows, err := workbook.Rows(sheetName)
		if err != nil {
			return nil, fmt.Errorf("读取 XLSX 工作表 %q: %w", sheetName, err)
		}
		rowNumber := 0
		var headers []string
		for rows.Next() {
			rowNumber++
			if err := contextError(ctx, "解析 XLSX 行"); err != nil {
				rows.Close()
				return nil, err
			}
			columns, err := rows.Columns()
			if err != nil {
				rows.Close()
				return nil, fmt.Errorf("读取 XLSX 工作表 %q 第 %d 行: %w", sheetName, rowNumber, err)
			}
			if rowNumber == 1 {
				headers = append([]string(nil), columns...)
				continue
			}
			content, rowMetadata := normalizeSpreadsheetRow(headers, columns)
			if content == "" {
				continue
			}
			metadata := cloneMetadata(commonOptions.ExtraMeta)
			metadata["sheet_name"] = sheetName
			metadata["row_number"] = rowNumber
			metadata["row"] = rowMetadata
			documents = append(documents, &schema.Document{Content: content, MetaData: metadata})
		}
		if err := rows.Error(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("遍历 XLSX 工作表 %q: %w", sheetName, err)
		}
		if err := rows.Close(); err != nil {
			return nil, fmt.Errorf("关闭 XLSX 工作表 %q: %w", sheetName, err)
		}
	}
	return documents, nil
}

func normalizeSpreadsheetRow(headers, columns []string) (string, map[string]string) {
	normalized := make([]string, len(columns))
	rowMetadata := make(map[string]string, len(columns))
	allEmpty := true
	for index, column := range columns {
		column = strings.TrimSpace(column)
		normalized[index] = column
		if column != "" {
			allEmpty = false
		}
		if index < len(headers) && strings.TrimSpace(headers[index]) != "" {
			rowMetadata[strings.TrimSpace(headers[index])] = column
		}
	}
	if allEmpty {
		return "", nil
	}
	return strings.Join(normalized, "\t"), rowMetadata
}

func normalizeDocuments(source SourceInfo, documents []*schema.Document) ([]*schema.Document, error) {
	normalized := make([]*schema.Document, 0, len(documents))
	for _, document := range documents {
		if document == nil || strings.TrimSpace(document.Content) == "" {
			continue
		}
		document.MetaData = cloneMetadata(document.MetaData)
		document.MetaData["source_uri"] = source.URI
		document.MetaData["mime_type"] = source.MIMEType
		document.MetaData["document_id"] = source.DocumentID
		document.MetaData["document_version_id"] = source.VersionID
		normalized = append(normalized, document)
	}
	if len(normalized) == 0 {
		return nil, ErrNoParsedContent
	}

	if source.Extension == ".docx" {
		sort.SliceStable(normalized, func(i, j int) bool {
			return metadataString(normalized[i].MetaData, docxparser.SectionTypeKey) <
				metadataString(normalized[j].MetaData, docxparser.SectionTypeKey)
		})
	}
	for index, document := range normalized {
		unitType := parsedUnitType(source.Extension)
		document.MetaData["unit_type"] = unitType
		document.MetaData["unit_index"] = index + 1
		if source.Extension == ".pdf" {
			document.MetaData["page_number"] = index + 1
		}
		unitHash := sha256.Sum256([]byte(
			source.VersionID + ":" + strconv.Itoa(index+1) + ":" + document.Content,
		))
		document.ID = hex.EncodeToString(unitHash[:])
	}
	return normalized, nil
}

func parsedUnitType(extension string) string {
	switch extension {
	case ".pdf":
		return "page"
	case ".docx":
		return "section"
	case ".xlsx":
		return "row"
	default:
		return "document"
	}
}

func cloneMetadata(metadata map[string]any) map[string]any {
	cloned := make(map[string]any, len(metadata)+4)
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

func metadataString(metadata map[string]any, key string) string {
	value, _ := metadata[key].(string)
	return value
}

var _ parser.Parser = (*strictParserRouter)(nil)
var _ parser.Parser = strictTextParser{}
var _ parser.Parser = recoveringParser{}
var _ parser.Parser = allSheetsXLSXParser{}
