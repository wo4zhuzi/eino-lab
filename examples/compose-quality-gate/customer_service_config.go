package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/callbacks"
)

const defaultCustomerReplyTimeout = 15 * time.Second

func customerReplyGeneratorFromEnv(
	ctx context.Context,
	getenv func(string) string,
	handlers ...callbacks.Handler,
) (CustomerReplyGenerator, error) {
	mode := strings.ToLower(strings.TrimSpace(getenv("CUSTOMER_REPLY_MODE")))
	switch mode {
	case "", "simulated":
		return simulatedCustomerReplyGenerator{}, nil
	case "model", "model_graph":
		apiKey := strings.TrimSpace(getenv("OPENAI_API_KEY"))
		modelName := strings.TrimSpace(getenv("OPENAI_MODEL"))
		if apiKey == "" || modelName == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY and OPENAI_MODEL are required in model mode")
		}

		timeout, err := customerReplyTimeout(getenv("CUSTOMER_REPLY_TIMEOUT"))
		if err != nil {
			return nil, err
		}

		chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
			APIKey:  apiKey,
			Model:   modelName,
			BaseURL: strings.TrimSpace(getenv("OPENAI_BASE_URL")),
			Timeout: timeout,
		})
		if err != nil {
			return nil, fmt.Errorf("create OpenAI chat model: %w", err)
		}

		if mode == "model_graph" {
			return NewChatModelGraphCustomerReplyGenerator(ctx, chatModel, handlers...)
		}
		return NewChatModelCustomerReplyGenerator(chatModel)
	default:
		return nil, fmt.Errorf("unsupported CUSTOMER_REPLY_MODE %q", mode)
	}
}

func customerReplyTimeout(value string) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return defaultCustomerReplyTimeout, nil
	}

	timeout, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || timeout <= 0 {
		return 0, fmt.Errorf("CUSTOMER_REPLY_TIMEOUT must be a positive Go duration")
	}
	return timeout, nil
}
