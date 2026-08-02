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
		{Content: "  您好，退款将在 3 个工作日到账。  "},
		{Content: "  您好，请查看退款说明。  "},
		{Content: "  您好，请查看相关说明。  "},
	}
	for _, request := range requests {
		result, err := runnable.Invoke(context.Background(), request)
		if err != nil {
			panic(err)
		}
		fmt.Printf(
			"approved=%t route=%s score=%d steps=%v reasons=%v audit=%v\n",
			result.Approved,
			result.Route,
			result.Score,
			result.Steps,
			result.Reasons,
			result.Audit,
		)
	}
}
