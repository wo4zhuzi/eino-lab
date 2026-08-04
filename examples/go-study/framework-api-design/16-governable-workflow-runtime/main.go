package main

import (
	"context"
	"fmt"

	"github.com/wo4zhuzi/eino-lab/examples/go-study/framework-api-design/16-governable-workflow-runtime/ragworkflow"
	"github.com/wo4zhuzi/eino-lab/examples/go-study/framework-api-design/16-governable-workflow-runtime/reviewworkflow"
	"github.com/wo4zhuzi/eino-lab/examples/go-study/framework-api-design/16-governable-workflow-runtime/workflowkit"
)

func main() {
	ctx := context.Background()
	recorder := workflowkit.NewRecorder()

	reviewFlow, err := reviewworkflow.New(
		ctx,
		reviewworkflow.DefaultConfig(),
		reviewworkflow.Dependencies{Inspector: reviewworkflow.NewKeywordInspector()},
	)
	if err != nil {
		panic(err)
	}

	ragFlow, err := ragworkflow.New(
		ctx,
		ragworkflow.DefaultConfig(),
		ragworkflow.Dependencies{
			Retriever: ragworkflow.NewMemoryRetriever(map[string][]string{
				"Eino": {
					"Eino Compose 可以组合 Chain 与 Graph。",
					"Graph 适合 Branch、回环和任意连边。",
				},
			}),
			Generator: ragworkflow.NewCitationGenerator(),
		},
	)
	if err != nil {
		panic(err)
	}

	reviewResult, err := reviewFlow.Run(
		ctx,
		reviewworkflow.Request{Content: "退款将在 3 个工作日到账。"},
		workflowkit.WithRunID("review-demo-001"),
		workflowkit.WithObserver(recorder),
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf(
		"workflow=%s run_id=review-demo-001 route=%s score=%d\n",
		reviewworkflow.Descriptor(),
		reviewResult.Route,
		reviewResult.Score,
	)

	ragResult, err := ragFlow.Run(
		ctx,
		ragworkflow.Request{Question: "如何组合 Chain 和 Graph？"},
		workflowkit.WithRunID("rag-demo-001"),
		workflowkit.WithObserver(recorder),
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf(
		"workflow=%s run_id=rag-demo-001 no_evidence=%t attempts=%d evidence=%d\n",
		ragworkflow.Descriptor(),
		ragResult.NoEvidence,
		ragResult.RetrievalAttempts,
		len(ragResult.Evidence),
	)
	fmt.Printf("observed_events=%d\n", len(recorder.Events()))
}
