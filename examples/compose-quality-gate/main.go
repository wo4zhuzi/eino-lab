package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/cloudwego/eino-ext/devops"
	"github.com/cloudwego/eino/compose"
)

const einoDevEnv = "EINO_DEV"

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
	einoDevEnabled := os.Getenv(einoDevEnv) == "true"
	if einoDevEnabled {
		if err := devops.Init(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "init eino devops: %v\n", err)
			os.Exit(1)
		}
	}

	customerID := "customer-001"
	question := "我的订单什么时候能退款？"
	replyObserver := NewObserver()
	generator, err := customerReplyGeneratorFromEnv(ctx, os.Getenv, replyObserver.Handler())
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

	gateObserver := NewObserver()
	result, err := gate.Review(
		ctx,
		ReviewRequest{Content: draft},
		compose.WithCallbacks(gateObserver.Handler()),
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
	fmt.Printf("reply_callback_records=%d\n", len(replyObserver.Records()))
	fmt.Printf("gate_callback_records=%d\n", len(gateObserver.Records()))

	var delivery CustomerReplyDelivery = simulatedCustomerReplyDelivery{output: os.Stdout}
	if err := delivery.Deliver(ctx, customerID, result); err != nil {
		fmt.Fprintf(os.Stderr, "deliver customer reply: %v\n", err)
		os.Exit(1)
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
