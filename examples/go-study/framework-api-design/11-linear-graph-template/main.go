package main

import (
	"context"
	"fmt"
)

func main() {
	runnable, err := NewReviewGraph(context.Background())
	if err != nil {
		panic(err)
	}

	result, err := runnable.Invoke(context.Background(), ReviewRequest{
		Content: "  您好，退款将在 3 个工作日到账。  ",
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf(
		"approved=%t score=%d content=%q steps=%v reasons=%v\n",
		result.Approved,
		result.Score,
		result.Content,
		result.Steps,
		result.Reasons,
	)
}
