package main

import (
	"context"
	"fmt"
)

func main() {
	runnable, err := NewReviewPipeline(context.Background())
	if err != nil {
		panic(err)
	}

	requests := []ReviewRequest{
		{Content: "退款将在 3 个工作日到账。"},
		{Content: "请尽快处理。"},
	}
	for _, request := range requests {
		result, err := runnable.Invoke(context.Background(), request)
		if err != nil {
			panic(err)
		}
		fmt.Printf(
			"approved=%t score=%d attempts=%d steps=%v content=%q\n",
			result.Approved,
			result.Score,
			result.Attempts,
			result.Steps,
			result.Content,
		)
	}
}
