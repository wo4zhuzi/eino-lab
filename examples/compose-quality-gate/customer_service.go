package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var (
	ErrEmptyCustomerQuestion = errors.New("customer question is empty")
	ErrChatModelRequired     = errors.New("chat model is required")
	ErrEmptyCustomerReply    = errors.New("chat model returned an empty customer reply")
)

const requiredRefundNotice = "退款到账时间以支付平台实际处理结果为准。"

const customerReplySystemPrompt = `你是电商平台的中文客服助手。请根据客户问题生成一段简洁、礼貌、可直接发送的回复草稿。
只输出回复正文，不要输出标题、分析过程、Markdown 代码块或虚构的订单状态。无法确定的信息应说明正在核实。`

// CustomerReplyGenerator 表示生成客服回复草稿的上游服务。
type CustomerReplyGenerator interface {
	Generate(ctx context.Context, question string) (string, error)
}

// simulatedCustomerReplyGenerator 使用确定性逻辑，保证示例无需外部服务即可运行。
type simulatedCustomerReplyGenerator struct{}

func (simulatedCustomerReplyGenerator) Generate(ctx context.Context, question string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	question = strings.TrimSpace(question)
	if question == "" {
		return "", ErrEmptyCustomerQuestion
	}

	return fmt.Sprintf("您好，关于“%s”，我们已收到您的问题，正在为您核实处理。", question), nil
}

// chatModelCustomerReplyGenerator 使用 Eino ChatModel 生成客服回复草稿。
type chatModelCustomerReplyGenerator struct {
	chatModel model.BaseChatModel
}

// NewChatModelCustomerReplyGenerator 将 Eino ChatModel 适配为客服回复生成器。
func NewChatModelCustomerReplyGenerator(chatModel model.BaseChatModel) (CustomerReplyGenerator, error) {
	if chatModel == nil {
		return nil, ErrChatModelRequired
	}
	return &chatModelCustomerReplyGenerator{chatModel: chatModel}, nil
}

func (g *chatModelCustomerReplyGenerator) Generate(ctx context.Context, question string) (string, error) {
	messages, err := customerReplyMessages(ctx, question)
	if err != nil {
		return "", err
	}

	response, err := g.chatModel.Generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("generate customer reply with chat model: %w", err)
	}
	return customerReplyContent(ctx, response)
}

func customerReplyMessages(ctx context.Context, question string) ([]*schema.Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	question = strings.TrimSpace(question)
	if question == "" {
		return nil, ErrEmptyCustomerQuestion
	}

	return []*schema.Message{
		schema.SystemMessage(customerReplySystemPrompt),
		schema.UserMessage("客户问题：\n" + question),
	}, nil
}

func customerReplyContent(ctx context.Context, response *schema.Message) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if response == nil || strings.TrimSpace(response.Content) == "" {
		return "", ErrEmptyCustomerReply
	}

	return strings.TrimSpace(response.Content), nil
}
