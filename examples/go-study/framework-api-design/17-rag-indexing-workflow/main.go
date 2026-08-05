package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wo4zhuzi/eino-lab/examples/go-study/framework-api-design/17-rag-indexing-workflow/indexworkflow"
)

func main() {
	sourceURI := filepath.Join(
		"examples",
		"go-study",
		"framework-api-design",
		"17-rag-indexing-workflow",
		"testdata",
		"knowledge.md",
	)
	if len(os.Args) > 1 {
		sourceURI = os.Args[1]
	}

	ctx := context.Background()
	workflow, err := indexworkflow.New(ctx, indexworkflow.DefaultConfig())
	if err != nil {
		panic(err)
	}
	result, err := workflow.Run(ctx, indexworkflow.Request{
		RunID:     "demo17-local-run",
		SourceURI: sourceURI,
	})
	if err != nil {
		panic(err)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		panic(fmt.Errorf("输出工作流结果: %w", err))
	}
}
