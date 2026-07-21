package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino/compose"
)

type ruleInspector struct{}

func (ruleInspector) Inspect(ctx context.Context, content string) (Inspection, error) {
	if err := ctx.Err(); err != nil {
		return Inspection{}, err
	}
	if strings.Contains(content, requiredRefundNotice) {
		return Inspection{Score: 8, Reason: "refund timing notice is present"}, nil
	}
	return Inspection{Score: 4, Reason: "refund timing notice is missing"}, nil
}

func main() {
	ctx := context.Background()
	customerID := "customer-001"
	question := "我的订单什么时候能退款？"
	generator, err := customerReplyGeneratorFromEnv(ctx, os.Getenv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "configure customer reply generator: %v\n", err)
		os.Exit(1)
	}
	draft, err := generator.Generate(ctx, question)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate customer reply: %v\n", err)
		os.Exit(1)
	}

	config := DefaultGateConfig()
	config.MaxAttempts = 2

	gate, err := NewQualityGate(ctx, ruleInspector{}, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build quality gate: %v\n", err)
		os.Exit(1)
	}

	observer := NewObserver()
	result, err := gate.Review(
		ctx,
		ReviewRequest{Content: draft},
		compose.WithCallbacks(observer.Handler()),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "review content: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("question=%s\n", question)
	fmt.Printf("draft=%s\n", draft)
	fmt.Printf("graph=%s nodes=%d edges=%d branches=%d\n",
		gate.Topology().Name,
		len(gate.Topology().Nodes),
		len(gate.Topology().Edges),
		len(gate.Topology().Branches),
	)
	fmt.Printf("status=%s score=%d attempts=%d\n", result.Status, result.Score, result.Attempts)
	for _, entry := range result.Audit {
		fmt.Printf("attempt=%d score=%d reason=%q\n", entry.Attempt, entry.Score, entry.Reason)
	}
	fmt.Printf("callback_records=%d\n", len(observer.Records()))

	var delivery CustomerReplyDelivery = simulatedCustomerReplyDelivery{output: os.Stdout}
	if err := delivery.Deliver(ctx, customerID, result); err != nil {
		fmt.Fprintf(os.Stderr, "deliver customer reply: %v\n", err)
		os.Exit(1)
	}
}
