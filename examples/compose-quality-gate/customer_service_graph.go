package main

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

const (
	nodeBuildCustomerReplyMessages = "build_customer_reply_messages"
	nodeGenerateCustomerReply      = "generate_customer_reply"
	nodeExtractCustomerReply       = "extract_customer_reply"
	customerReplyGraphName         = "customer_reply_generation"
)

type chatModelGraphCustomerReplyGenerator struct {
	runnable compose.Runnable[string, string]
	handlers []callbacks.Handler
}

// NewChatModelGraphCustomerReplyGenerator 将 ChatModel 注册为 Compose 节点。
func NewChatModelGraphCustomerReplyGenerator(
	ctx context.Context,
	chatModel model.BaseChatModel,
	handlers ...callbacks.Handler,
) (CustomerReplyGenerator, error) {
	if chatModel == nil {
		return nil, ErrChatModelRequired
	}

	graph := compose.NewGraph[string, string]()
	if err := graph.AddLambdaNode(
		nodeBuildCustomerReplyMessages,
		compose.InvokableLambda(customerReplyMessages),
		compose.WithNodeName(nodeBuildCustomerReplyMessages),
	); err != nil {
		return nil, fmt.Errorf("add customer reply message builder: %w", err)
	}
	if err := graph.AddChatModelNode(
		nodeGenerateCustomerReply,
		chatModel,
		compose.WithNodeName(nodeGenerateCustomerReply),
	); err != nil {
		return nil, fmt.Errorf("add customer reply chat model: %w", err)
	}
	if err := graph.AddLambdaNode(
		nodeExtractCustomerReply,
		compose.InvokableLambda(func(ctx context.Context, response *schema.Message) (string, error) {
			return customerReplyContent(ctx, response)
		}),
		compose.WithNodeName(nodeExtractCustomerReply),
	); err != nil {
		return nil, fmt.Errorf("add customer reply extractor: %w", err)
	}

	for _, edge := range [][2]string{
		{compose.START, nodeBuildCustomerReplyMessages},
		{nodeBuildCustomerReplyMessages, nodeGenerateCustomerReply},
		{nodeGenerateCustomerReply, nodeExtractCustomerReply},
		{nodeExtractCustomerReply, compose.END},
	} {
		if err := graph.AddEdge(edge[0], edge[1]); err != nil {
			return nil, fmt.Errorf("add customer reply edge %s -> %s: %w", edge[0], edge[1], err)
		}
	}

	runnable, err := graph.Compile(ctx, compose.WithGraphName(customerReplyGraphName))
	if err != nil {
		return nil, fmt.Errorf("compile customer reply graph: %w", err)
	}

	return &chatModelGraphCustomerReplyGenerator{
		runnable: runnable,
		handlers: append([]callbacks.Handler(nil), handlers...),
	}, nil
}

func (g *chatModelGraphCustomerReplyGenerator) Generate(ctx context.Context, question string) (string, error) {
	if len(g.handlers) == 0 {
		return g.runnable.Invoke(ctx, question)
	}
	return g.runnable.Invoke(ctx, question, compose.WithCallbacks(g.handlers...))
}
