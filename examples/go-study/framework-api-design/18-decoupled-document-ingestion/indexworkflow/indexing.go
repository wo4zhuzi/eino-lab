package indexworkflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	docxparser "github.com/cloudwego/eino-ext/components/document/parser/docx"
	"github.com/cloudwego/eino/schema"
	ingestion "github.com/wo4zhuzi/eino-document-ingestion"
)

func prepareForIndexing(result *ingestion.Result) (SourceInfo, []*schema.Document, error) {
	documentHash := sha256.Sum256([]byte("source:" + result.Source.URI))
	source := SourceInfo{
		URI:         result.Source.URI,
		ResolvedURI: result.Source.ResolvedURI,
		FileName:    result.Source.FileName,
		Extension:   result.Source.Extension,
		MIMEType:    result.Source.MIMEType,
		SizeBytes:   result.Source.SizeBytes,
		SHA256:      result.Source.SHA256,
		DocumentID:  hex.EncodeToString(documentHash[:]),
		VersionID:   result.Source.SHA256,
	}

	documents := make([]*schema.Document, 0, len(result.Documents))
	for _, document := range result.Documents {
		if document == nil || strings.TrimSpace(document.Content) == "" {
			continue
		}
		cloned := &schema.Document{
			ID:       document.ID,
			Content:  document.Content,
			MetaData: cloneMetadata(document.MetaData),
		}
		documents = append(documents, cloned)
	}
	if len(documents) == 0 {
		return SourceInfo{}, nil, ingestion.ErrNoParsedContent
	}

	for index, document := range documents {
		unitIndex := index + 1
		document.MetaData["source_uri"] = source.URI
		document.MetaData["mime_type"] = source.MIMEType
		document.MetaData["document_id"] = source.DocumentID
		document.MetaData["document_version_id"] = source.VersionID
		document.MetaData["unit_type"] = parsedUnitType(source.Extension)
		document.MetaData["unit_index"] = unitIndex
		unitHash := sha256.Sum256([]byte(
			source.VersionID + ":" + strconv.Itoa(unitIndex) + ":" + document.Content,
		))
		document.ID = hex.EncodeToString(unitHash[:])
	}
	return source, documents, nil
}

func parsedUnitType(extension string) string {
	switch extension {
	case ingestion.ExtensionPDF:
		return "page"
	case ingestion.ExtensionDOCX:
		return "section"
	case ingestion.ExtensionXLSX:
		return "row"
	default:
		return "document"
	}
}

func summarizeDocuments(documents []*schema.Document) []ParsedUnit {
	units := make([]ParsedUnit, 0, len(documents))
	for index, document := range documents {
		if document == nil {
			continue
		}
		units = append(units, ParsedUnit{
			ID:         document.ID,
			Type:       metadataString(document.MetaData, "unit_type"),
			Index:      index + 1,
			Characters: utf8.RuneCountInString(document.Content),
			Preview:    preview(document.Content, 120),
			PageNumber: metadataInt(document.MetaData, "page_number"),
			SheetName:  metadataString(document.MetaData, "sheet_name"),
			RowNumber:  metadataInt(document.MetaData, "row_number"),
			Section:    metadataString(document.MetaData, docxparser.SectionTypeKey),
		})
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

func cloneMetadata(metadata map[string]any) map[string]any {
	cloned := make(map[string]any, len(metadata)+6)
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

func metadataString(metadata map[string]any, key string) string {
	value, _ := metadata[key].(string)
	return value
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
