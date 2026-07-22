package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	filePath := flag.String(
		"file",
		"examples/rag-minimal/testdata/personal-notes.md",
		"需要建立索引的本地 Markdown 文件",
	)
	timeout := flag.Duration("timeout", 5*time.Second, "索引和查询的总超时时间")
	flag.Parse()

	question := strings.TrimSpace(strings.Join(flag.Args(), " "))
	if question == "" {
		question = "RAG 的索引写路径和查询读路径为什么要分开？"
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	app, err := NewApp(ctx, DefaultConfig())
	if err != nil {
		exitError("build RAG app", err)
	}
	indexObserver := NewObserver()
	indexReport, err := app.IndexFile(ctx, *filePath, indexObserver.Handler())
	if err != nil {
		exitError("index knowledge", err)
	}
	queryObserver := NewObserver()
	result, err := app.Ask(ctx, question, queryObserver.Handler())
	if err != nil {
		exitError("query knowledge", err)
	}

	fmt.Printf("index_graph=%s documents=%d chunks=%d callback_records=%d\n",
		indexGraphName, indexReport.Documents, indexReport.Chunks, len(indexObserver.Records()))
	fmt.Printf("query_graph=%s question=%q retrieved=%d no_evidence=%t callback_records=%d\n",
		queryGraphName, result.Question, len(result.Retrieved), result.NoEvidence, len(queryObserver.Records()))
	for _, chunk := range result.Retrieved {
		fmt.Printf("rank=%d score=%.4f source=%q heading=%q chunk_id=%s\n",
			chunk.Rank, chunk.Score, chunk.Source, chunk.Heading, chunk.ChunkID)
	}
	fmt.Printf("chat_model_calls=%d\n", app.ChatModelCallCount())
	fmt.Printf("answer=%s\n", result.Answer)
}

func exitError(operation string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", operation, err)
	os.Exit(1)
}
