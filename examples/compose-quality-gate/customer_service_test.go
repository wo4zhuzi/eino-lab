package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type scriptedCustomerChatModel struct {
	response *schema.Message
	err      error
	inputs   [][]*schema.Message
}

func (m *scriptedCustomerChatModel) Generate(
	_ context.Context,
	input []*schema.Message,
	_ ...model.Option,
) (*schema.Message, error) {
	m.inputs = append(m.inputs, input)
	return m.response, m.err
}

func (m *scriptedCustomerChatModel) Stream(
	_ context.Context,
	_ []*schema.Message,
	_ ...model.Option,
) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("stream is not used by this generator")
}

func TestSimulatedCustomerReplyGenerator(t *testing.T) {
	generator := simulatedCustomerReplyGenerator{}

	draft, err := generator.Generate(context.Background(), "我的订单什么时候能退款？")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if draft == "" {
		t.Fatal("Generate() returned an empty draft")
	}
	if !strings.Contains(draft, "我的订单什么时候能退款？") {
		t.Fatalf("Generate() draft = %q, want question included", draft)
	}
}

func TestSimulatedCustomerReplyGeneratorRejectsEmptyQuestion(t *testing.T) {
	_, err := (simulatedCustomerReplyGenerator{}).Generate(context.Background(), " \n\t ")
	if !errors.Is(err, ErrEmptyCustomerQuestion) {
		t.Fatalf("Generate() error = %v, want ErrEmptyCustomerQuestion", err)
	}
}

func TestSimulatedCustomerReplyGeneratorHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := (simulatedCustomerReplyGenerator{}).Generate(ctx, "退款什么时候到账？")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Generate() error = %v, want context.Canceled", err)
	}
}

func TestChatModelCustomerReplyGenerator(t *testing.T) {
	chatModel := &scriptedCustomerChatModel{
		response: schema.AssistantMessage("  您好，我们正在核实退款进度。  ", nil),
	}
	generator, err := NewChatModelCustomerReplyGenerator(chatModel)
	if err != nil {
		t.Fatalf("NewChatModelCustomerReplyGenerator() error = %v", err)
	}

	draft, err := generator.Generate(context.Background(), "我的退款什么时候到账？")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if draft != "您好，我们正在核实退款进度。" {
		t.Fatalf("Generate() draft = %q", draft)
	}
	if len(chatModel.inputs) != 1 || len(chatModel.inputs[0]) != 2 {
		t.Fatalf("Generate() model inputs = %#v, want one system/user request", chatModel.inputs)
	}
	if chatModel.inputs[0][0].Role != schema.System {
		t.Fatalf("Generate() first role = %q, want system", chatModel.inputs[0][0].Role)
	}
	if chatModel.inputs[0][1].Role != schema.User ||
		!strings.Contains(chatModel.inputs[0][1].Content, "我的退款什么时候到账？") {
		t.Fatalf("Generate() user message = %#v", chatModel.inputs[0][1])
	}
}

func TestChatModelCustomerReplyGeneratorRejectsInvalidInputs(t *testing.T) {
	if _, err := NewChatModelCustomerReplyGenerator(nil); !errors.Is(err, ErrChatModelRequired) {
		t.Fatalf("NewChatModelCustomerReplyGenerator(nil) error = %v, want ErrChatModelRequired", err)
	}

	chatModel := &scriptedCustomerChatModel{
		response: schema.AssistantMessage("unused", nil),
	}
	generator, err := NewChatModelCustomerReplyGenerator(chatModel)
	if err != nil {
		t.Fatalf("NewChatModelCustomerReplyGenerator() error = %v", err)
	}
	if _, err := generator.Generate(context.Background(), " \n\t "); !errors.Is(err, ErrEmptyCustomerQuestion) {
		t.Fatalf("Generate() error = %v, want ErrEmptyCustomerQuestion", err)
	}
	if len(chatModel.inputs) != 0 {
		t.Fatalf("Generate() model calls = %d, want 0", len(chatModel.inputs))
	}
}

func TestChatModelCustomerReplyGeneratorPreservesModelError(t *testing.T) {
	modelErr := errors.New("model unavailable")
	generator, err := NewChatModelCustomerReplyGenerator(&scriptedCustomerChatModel{err: modelErr})
	if err != nil {
		t.Fatalf("NewChatModelCustomerReplyGenerator() error = %v", err)
	}

	_, err = generator.Generate(context.Background(), "退款什么时候到账？")
	if !errors.Is(err, modelErr) {
		t.Fatalf("Generate() error = %v, want model error in chain", err)
	}
}

func TestChatModelCustomerReplyGeneratorRejectsEmptyResponse(t *testing.T) {
	for _, response := range []*schema.Message{nil, schema.AssistantMessage(" \n ", nil)} {
		generator, err := NewChatModelCustomerReplyGenerator(&scriptedCustomerChatModel{response: response})
		if err != nil {
			t.Fatalf("NewChatModelCustomerReplyGenerator() error = %v", err)
		}

		_, err = generator.Generate(context.Background(), "退款什么时候到账？")
		if !errors.Is(err, ErrEmptyCustomerReply) {
			t.Fatalf("Generate() error = %v, want ErrEmptyCustomerReply", err)
		}
	}
}

func TestChatModelCustomerReplyGeneratorHonorsCancellation(t *testing.T) {
	chatModel := &scriptedCustomerChatModel{
		response: schema.AssistantMessage("unused", nil),
	}
	generator, err := NewChatModelCustomerReplyGenerator(chatModel)
	if err != nil {
		t.Fatalf("NewChatModelCustomerReplyGenerator() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = generator.Generate(ctx, "退款什么时候到账？")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Generate() error = %v, want context.Canceled", err)
	}
	if len(chatModel.inputs) != 0 {
		t.Fatalf("Generate() model calls = %d, want 0", len(chatModel.inputs))
	}
}
