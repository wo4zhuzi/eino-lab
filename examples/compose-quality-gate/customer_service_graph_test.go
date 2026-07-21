package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type customerChatModelFunc func(context.Context, []*schema.Message) (*schema.Message, error)

func (f customerChatModelFunc) Generate(
	ctx context.Context,
	input []*schema.Message,
	_ ...model.Option,
) (*schema.Message, error) {
	return f(ctx, input)
}

func (f customerChatModelFunc) Stream(
	context.Context,
	[]*schema.Message,
	...model.Option,
) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("stream is not used by this generator")
}

func TestChatModelGraphCustomerReplyGeneratorMatchesDirectPath(t *testing.T) {
	directModel := &scriptedCustomerChatModel{
		response: schema.AssistantMessage("  您好，我们正在核实退款进度。  ", nil),
	}
	graphModel := &scriptedCustomerChatModel{
		response: schema.AssistantMessage("  您好，我们正在核实退款进度。  ", nil),
	}
	direct, err := NewChatModelCustomerReplyGenerator(directModel)
	if err != nil {
		t.Fatalf("NewChatModelCustomerReplyGenerator() error = %v", err)
	}
	observer := NewObserver()
	graph, err := NewChatModelGraphCustomerReplyGenerator(context.Background(), graphModel, observer.Handler())
	if err != nil {
		t.Fatalf("NewChatModelGraphCustomerReplyGenerator() error = %v", err)
	}

	directDraft, err := direct.Generate(context.Background(), "我的退款什么时候到账？")
	if err != nil {
		t.Fatalf("direct Generate() error = %v", err)
	}
	graphDraft, err := graph.Generate(context.Background(), "我的退款什么时候到账？")
	if err != nil {
		t.Fatalf("graph Generate() error = %v", err)
	}
	if graphDraft != directDraft {
		t.Fatalf("drafts differ: direct=%q graph=%q", directDraft, graphDraft)
	}
	if !reflect.DeepEqual(graphModel.inputs, directModel.inputs) {
		t.Fatalf("model inputs differ: direct=%#v graph=%#v", directModel.inputs, graphModel.inputs)
	}

	records := observer.Records()
	for _, name := range []string{
		nodeBuildCustomerReplyMessages,
		nodeGenerateCustomerReply,
		nodeExtractCustomerReply,
	} {
		if !hasCallbackRecord(records, name, "succeeded", "") {
			t.Fatalf("callback records = %#v, want successful %s node", records, name)
		}
	}
}

func TestChatModelGraphCustomerReplyGeneratorPreservesModelErrorAndNodePath(t *testing.T) {
	modelErr := errors.New("model unavailable")
	observer := NewObserver()
	generator, err := NewChatModelGraphCustomerReplyGenerator(
		context.Background(),
		&scriptedCustomerChatModel{err: modelErr},
		observer.Handler(),
	)
	if err != nil {
		t.Fatalf("NewChatModelGraphCustomerReplyGenerator() error = %v", err)
	}

	_, err = generator.Generate(context.Background(), "退款什么时候到账？")
	if !errors.Is(err, modelErr) {
		t.Fatalf("Generate() error = %v, want model error in chain", err)
	}
	if !strings.Contains(err.Error(), "node path: ["+nodeGenerateCustomerReply+"]") {
		t.Fatalf("Generate() error = %v, want model node path", err)
	}
	if !hasCallbackRecord(observer.Records(), nodeGenerateCustomerReply, "failed", "internal") {
		t.Fatalf("callback records = %#v, want failed model node", observer.Records())
	}
}

func TestChatModelGraphCustomerReplyGeneratorRejectsEmptyResponseAtExtractor(t *testing.T) {
	observer := NewObserver()
	generator, err := NewChatModelGraphCustomerReplyGenerator(
		context.Background(),
		&scriptedCustomerChatModel{response: schema.AssistantMessage(" \n ", nil)},
		observer.Handler(),
	)
	if err != nil {
		t.Fatalf("NewChatModelGraphCustomerReplyGenerator() error = %v", err)
	}

	_, err = generator.Generate(context.Background(), "退款什么时候到账？")
	if !errors.Is(err, ErrEmptyCustomerReply) {
		t.Fatalf("Generate() error = %v, want ErrEmptyCustomerReply", err)
	}
	if !strings.Contains(err.Error(), "node path: ["+nodeExtractCustomerReply+"]") {
		t.Fatalf("Generate() error = %v, want extractor node path", err)
	}
	if !hasCallbackRecord(observer.Records(), nodeExtractCustomerReply, "failed", "empty_customer_reply") {
		t.Fatalf("callback records = %#v, want failed extractor node", observer.Records())
	}
}

func TestChatModelGraphCustomerReplyGeneratorReportsModelTimeout(t *testing.T) {
	blockingModel := customerChatModelFunc(func(ctx context.Context, _ []*schema.Message) (*schema.Message, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	observer := NewObserver()
	generator, err := NewChatModelGraphCustomerReplyGenerator(
		context.Background(),
		blockingModel,
		observer.Handler(),
	)
	if err != nil {
		t.Fatalf("NewChatModelGraphCustomerReplyGenerator() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err = generator.Generate(ctx, "退款什么时候到账？")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Generate() error = %v, want context.DeadlineExceeded", err)
	}
	if !strings.Contains(err.Error(), "node path: ["+nodeGenerateCustomerReply+"]") {
		t.Fatalf("Generate() error = %v, want model node path", err)
	}
	if !hasCallbackRecord(observer.Records(), nodeGenerateCustomerReply, "failed", "deadline_exceeded") {
		t.Fatalf("callback records = %#v, want model timeout", observer.Records())
	}
}

func TestChatModelGraphCustomerReplyGeneratorRejectsInvalidInput(t *testing.T) {
	if _, err := NewChatModelGraphCustomerReplyGenerator(context.Background(), nil); !errors.Is(err, ErrChatModelRequired) {
		t.Fatalf("NewChatModelGraphCustomerReplyGenerator(nil) error = %v, want ErrChatModelRequired", err)
	}

	model := &scriptedCustomerChatModel{response: schema.AssistantMessage("unused", nil)}
	generator, err := NewChatModelGraphCustomerReplyGenerator(context.Background(), model)
	if err != nil {
		t.Fatalf("NewChatModelGraphCustomerReplyGenerator() error = %v", err)
	}
	_, err = generator.Generate(context.Background(), " \n\t ")
	if !errors.Is(err, ErrEmptyCustomerQuestion) {
		t.Fatalf("Generate() error = %v, want ErrEmptyCustomerQuestion", err)
	}
	if !strings.Contains(err.Error(), "node path: ["+nodeBuildCustomerReplyMessages+"]") {
		t.Fatalf("Generate() error = %v, want message builder node path", err)
	}
	if len(model.inputs) != 0 {
		t.Fatalf("Generate() model calls = %d, want 0", len(model.inputs))
	}
}
