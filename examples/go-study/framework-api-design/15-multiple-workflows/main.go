package main

import (
	"context"
	"fmt"

	"github.com/wo4zhuzi/eino-lab/examples/go-study/framework-api-design/15-multiple-workflows/ragworkflow"
	"github.com/wo4zhuzi/eino-lab/examples/go-study/framework-api-design/15-multiple-workflows/reviewworkflow"
)

func main() {
	ctx := context.Background()

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

	reviewResult, err := reviewFlow.Run(ctx, reviewworkflow.Request{
		Content: "退款将在 3 个工作日到账。",
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf(
		"review route=%s score=%d steps=%v\n",
		reviewResult.Route,
		reviewResult.Score,
		reviewResult.Steps,
	)

	ragResult, err := ragFlow.Run(ctx, ragworkflow.Request{Question: "如何组合 Chain 和 Graph？"})
	if err != nil {
		panic(err)
	}
	fmt.Printf(
		"rag no_evidence=%t attempts=%d evidence=%d steps=%v\n",
		ragResult.NoEvidence,
		ragResult.RetrievalAttempts,
		len(ragResult.Evidence),
		ragResult.Steps,
	)
}
