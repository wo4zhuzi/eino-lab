package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/cloudwego/eino-ext/devops"
	"github.com/wo4zhuzi/eino-lab/examples/go-study/framework-api-design/17-rag-indexing-workflow/indexworkflow"
)

const einoDevEnv = "EINO_DEV"

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
	einoDevEnabled := os.Getenv(einoDevEnv) == "true"
	if einoDevEnabled {
		if err := devops.Init(ctx); err != nil {
			panic(fmt.Errorf("初始化 Eino DevOps: %w", err))
		}
	}

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

	if einoDevEnabled {
		waitForEinoDev(ctx)
	}
}

func waitForEinoDev(parent context.Context) {
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Println("eino_dev=ready address=127.0.0.1:52538")
	fmt.Println("按 Ctrl+C 停止 Eino Dev 模式")
	<-ctx.Done()
}
