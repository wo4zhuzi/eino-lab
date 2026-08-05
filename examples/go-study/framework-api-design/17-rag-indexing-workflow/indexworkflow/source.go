package indexworkflow

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var supportedMIMETypes = map[string]string{
	".md":   "text/markdown",
	".txt":  "text/plain",
	".pdf":  "application/pdf",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
}

func inspectSource(ctx context.Context, request Request, maxFileBytes int64) (SourceInfo, error) {
	if err := contextError(ctx, "检查数据源"); err != nil {
		return SourceInfo{}, err
	}
	runID := strings.TrimSpace(request.RunID)
	if runID == "" {
		return SourceInfo{}, ErrInvalidRunID
	}
	uri := strings.TrimSpace(request.SourceURI)
	if uri == "" {
		return SourceInfo{}, ErrEmptySourceURI
	}

	absPath, err := filepath.Abs(uri)
	if err != nil {
		return SourceInfo{}, fmt.Errorf("解析数据源绝对路径: %w", err)
	}
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		return SourceInfo{}, fmt.Errorf("检查数据源状态: %w", err)
	}
	if !fileInfo.Mode().IsRegular() {
		return SourceInfo{}, fmt.Errorf("%w: %s", ErrSourceNotRegularFile, absPath)
	}
	if fileInfo.Size() > maxFileBytes {
		return SourceInfo{}, fmt.Errorf(
			"%w: size=%d limit=%d",
			ErrSourceTooLarge,
			fileInfo.Size(),
			maxFileBytes,
		)
	}

	extension := strings.ToLower(filepath.Ext(fileInfo.Name()))
	mimeType, ok := supportedMIMETypes[extension]
	if !ok {
		return SourceInfo{}, fmt.Errorf("%w: %s", ErrUnsupportedFormat, extension)
	}
	if err := validateFileSignature(absPath, extension); err != nil {
		return SourceInfo{}, err
	}

	file, err := os.Open(absPath)
	if err != nil {
		return SourceInfo{}, fmt.Errorf("打开数据源计算指纹: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return SourceInfo{}, fmt.Errorf("计算数据源指纹: %w", err)
	}
	contentHash := hex.EncodeToString(hash.Sum(nil))
	documentHash := sha256.Sum256([]byte("file:" + absPath))

	return SourceInfo{
		URI:          absPath,
		FileName:     fileInfo.Name(),
		Extension:    extension,
		MIMEType:     mimeType,
		SizeBytes:    fileInfo.Size(),
		SHA256:       contentHash,
		DocumentID:   hex.EncodeToString(documentHash[:]),
		VersionID:    contentHash,
		DetectedFrom: "extension+signature",
	}, nil
}

func validateFileSignature(path, extension string) error {
	switch extension {
	case ".md", ".txt":
		return validateTextSignature(path)
	case ".pdf":
		return validatePDFSignature(path)
	case ".docx":
		return validateOfficePackage(path, "word/document.xml")
	case ".xlsx":
		return validateOfficePackage(path, "xl/workbook.xml")
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedFormat, extension)
	}
}

func validateTextSignature(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取文本文件: %w", err)
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return fmt.Errorf("%w: 文本文件包含 NUL 字节", ErrInvalidFileFormat)
	}
	return nil
}

func validatePDFSignature(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("打开 PDF 文件: %w", err)
	}
	defer file.Close()
	header := make([]byte, 5)
	if _, err := io.ReadFull(file, header); err != nil {
		return fmt.Errorf("%w: PDF 文件头不完整", ErrInvalidFileFormat)
	}
	if !bytes.Equal(header, []byte("%PDF-")) {
		return fmt.Errorf("%w: 缺少 PDF 文件头", ErrInvalidFileFormat)
	}
	return nil
}

func validateOfficePackage(path, requiredEntry string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("%w: 无法打开 Office ZIP 包: %v", ErrInvalidFileFormat, err)
	}
	defer reader.Close()
	var hasContentTypes, hasRequiredEntry bool
	for _, entry := range reader.File {
		switch entry.Name {
		case "[Content_Types].xml":
			hasContentTypes = true
		case requiredEntry:
			hasRequiredEntry = true
		}
	}
	if !hasContentTypes || !hasRequiredEntry {
		return fmt.Errorf(
			"%w: Office 包缺少 %s 或 [Content_Types].xml",
			ErrInvalidFileFormat,
			requiredEntry,
		)
	}
	return nil
}
