package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

var ErrNoFinalResponse = errors.New("agent returned no final response")

type WeatherAgent struct {
	runner *adk.Runner
}

func NewWeatherAgent(
	ctx context.Context,
	chatModel model.ToolCallingChatModel,
	provider WeatherProvider,
) (*WeatherAgent, error) {
	if chatModel == nil {
		return nil, errors.New("chat model is required")
	}

	weatherTool, err := NewWeatherTool(provider)
	if err != nil {
		return nil, fmt.Errorf("create weather tool: %w", err)
	}

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "diagnosable_weather_agent",
		Description: "Answers weather questions using a controlled weather lookup tool.",
		Instruction: "You answer weather questions. Always call weather_lookup before answering. Only claim facts returned by the tool. If the tool fails, do not invent an answer.",
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{weatherTool},
			},
		},
		MaxIterations: 4,
	})
	if err != nil {
		return nil, fmt.Errorf("create chat model agent: %w", err)
	}

	return &WeatherAgent{
		runner: adk.NewRunner(ctx, adk.RunnerConfig{
			Agent:           agent,
			EnableStreaming: true,
		}),
	}, nil
}

func (a *WeatherAgent) Query(
	ctx context.Context,
	query string,
	handlers ...callbacks.Handler,
) (*schema.Message, error) {
	if a == nil || a.runner == nil {
		return nil, errors.New("weather agent is not initialized")
	}
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("query is required")
	}

	iterator := a.runner.Query(ctx, query, adk.WithCallbacks(handlers...))
	var final *schema.Message
	for {
		event, ok := iterator.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}
		if event.Err != nil {
			return nil, fmt.Errorf("weather agent event: %w", event.Err)
		}

		message, err := consumeAgentEventMessage(event)
		if err != nil {
			return nil, fmt.Errorf("read agent message: %w", err)
		}
		if message != nil && message.Role == schema.Assistant && len(message.ToolCalls) == 0 {
			final = message
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("weather agent context: %w", err)
	}
	if final == nil {
		return nil, ErrNoFinalResponse
	}
	return final, nil
}

func consumeAgentEventMessage(event *adk.AgentEvent) (*schema.Message, error) {
	message, retainedEvent, err := adk.GetMessage(event)
	closeAgentEventMessageStream(retainedEvent)
	return message, err
}

func closeAgentEventMessageStream(event *adk.AgentEvent) {
	if event == nil || event.Output == nil || event.Output.MessageOutput == nil {
		return
	}

	output := event.Output.MessageOutput
	if output.IsStreaming && output.MessageStream != nil {
		output.MessageStream.Close()
	}
}
