package main

import (
	"context"
	"fmt"

	"github.com/wo4zhuzi/eino-lab/examples/go-study/framework-api-design/14-chain-with-graph/reviewworkflow"
)

func main() {
	reviser, err := reviewworkflow.NewAppendReviser("补充：退款将在 3 个工作日到账。")
	if err != nil {
		panic(err)
	}
	reviewFlow, err := reviewworkflow.New(
		context.Background(),
		reviewworkflow.DefaultConfig(),
		reviewworkflow.Dependencies{
			Inspector: reviewworkflow.NewKeywordInspector(),
			Reviser:   reviser,
		},
	)
	if err != nil {
		panic(err)
	}

	requests := []reviewworkflow.ReviewRequest{
		{Content: "退款将在 3 个工作日到账。"},
		{Content: "请尽快处理。"},
	}
	for _, request := range requests {
		result, err := reviewFlow.Run(context.Background(), request)
		if err != nil {
			panic(err)
		}
		fmt.Printf(
			"approved=%t route=%s score=%d attempts=%d steps=%v content=%q\n",
			result.Approved,
			result.Route,
			result.Score,
			result.Attempts,
			result.Steps,
			result.Content,
		)
	}
}
