package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type scriptedModelState struct {
	mu              sync.Mutex
	calls           int
	streamCalls     int
	city            string
	sawToolResponse bool
}

type scriptedWeatherModel struct {
	state       *scriptedModelState
	tools       []*schema.ToolInfo
	streamError error
}

var errModelStreamInterrupted = errors.New("model stream interrupted")

func newScriptedWeatherModel(city string) *scriptedWeatherModel {
	return &scriptedWeatherModel{state: &scriptedModelState{city: city}}
}

func (m *scriptedWeatherModel) Generate(
	_ context.Context,
	input []*schema.Message,
	opts ...model.Option,
) (*schema.Message, error) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	m.state.calls++
	switch m.state.calls {
	case 1:
		commonOptions := model.GetCommonOptions(&model.Options{Tools: m.tools}, opts...)
		if len(commonOptions.Tools) != 1 || commonOptions.Tools[0].Name != weatherToolName {
			return nil, fmt.Errorf("weather tool was not bound")
		}
		return schema.AssistantMessage("", []schema.ToolCall{{
			ID: "weather-call-1",
			Function: schema.FunctionCall{
				Name:      weatherToolName,
				Arguments: fmt.Sprintf(`{"city":%q}`, m.state.city),
			},
		}}), nil
	case 2:
		if len(input) == 0 {
			return nil, errors.New("missing model history")
		}
		last := input[len(input)-1]
		if last.Role != schema.Tool || last.ToolCallID != "weather-call-1" {
			return nil, fmt.Errorf("last message is not the expected tool response: %#v", last)
		}
		m.state.sawToolResponse = true
		return schema.AssistantMessage("Beijing is sunny, 28 C, with 35% humidity.", nil), nil
	default:
		return nil, fmt.Errorf("unexpected model call %d", m.state.calls)
	}
}

func (m *scriptedWeatherModel) Stream(
	ctx context.Context,
	input []*schema.Message,
	opts ...model.Option,
) (*schema.StreamReader[*schema.Message], error) {
	m.state.mu.Lock()
	m.state.streamCalls++
	m.state.mu.Unlock()

	message, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	if m.streamError != nil && len(message.ToolCalls) == 0 {
		reader, writer := schema.Pipe[*schema.Message](2)
		writer.Send(schema.AssistantMessage("Beijing is sunny, ", nil), nil)
		writer.Send(nil, m.streamError)
		writer.Close()
		return reader, nil
	}
	if len(message.ToolCalls) == 0 {
		return schema.StreamReaderFromArray([]*schema.Message{
			schema.AssistantMessage("Beijing is sunny, ", nil),
			schema.AssistantMessage("28 C, with 35% humidity.", nil),
		}), nil
	}
	return schema.StreamReaderFromArray([]*schema.Message{message}), nil
}

func (m *scriptedWeatherModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &scriptedWeatherModel{state: m.state, tools: tools, streamError: m.streamError}, nil
}

func (m *scriptedWeatherModel) snapshot() (calls, streamCalls int, sawToolResponse bool) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	return m.state.calls, m.state.streamCalls, m.state.sawToolResponse
}

func TestWeatherAgentReAct(t *testing.T) {
	ctx := context.Background()
	chatModel := newScriptedWeatherModel("Beijing")
	agent, err := NewWeatherAgent(ctx, chatModel, NewStaticWeatherProvider())
	if err != nil {
		t.Fatalf("NewWeatherAgent() error = %v", err)
	}
	observer := NewObserver(slog.New(slog.NewTextHandler(io.Discard, nil)))

	message, err := agent.Query(ctx, "What is the weather in Beijing?", observer.Handler())
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if message.Content != "Beijing is sunny, 28 C, with 35% humidity." {
		t.Fatalf("Query() content = %q", message.Content)
	}
	calls, streamCalls, sawToolResponse := chatModel.snapshot()
	if calls != 2 || streamCalls != 2 || !sawToolResponse {
		t.Fatalf("model calls = %d, stream calls = %d, saw tool response = %v", calls, streamCalls, sawToolResponse)
	}
	if !hasCallbackRecord(observer.Records(), "Tool", "succeeded", "") {
		t.Fatalf("callback records = %#v, want successful Tool record", observer.Records())
	}
	if strings.Contains(fmt.Sprint(observer.Records()), "Beijing") {
		t.Fatalf("callback records contain tool input: %#v", observer.Records())
	}
}

func TestWeatherAgentStreamError(t *testing.T) {
	ctx := context.Background()
	chatModel := newScriptedWeatherModel("Beijing")
	chatModel.streamError = errModelStreamInterrupted
	agent, err := NewWeatherAgent(ctx, chatModel, NewStaticWeatherProvider())
	if err != nil {
		t.Fatalf("NewWeatherAgent() error = %v", err)
	}
	observer := NewObserver(nil)

	message, err := agent.Query(ctx, "What is the weather in Beijing?", observer.Handler())
	if message != nil {
		t.Fatalf("Query() message = %#v, want nil", message)
	}
	if !errors.Is(err, errModelStreamInterrupted) {
		t.Fatalf("Query() error = %v, want stream error", err)
	}
	records := observer.Records()
	if !hasCallbackRecord(records, "ChatModel", "started", "") {
		t.Fatalf("callback records = %#v, want ChatModel start record", records)
	}
	if hasCallbackRecord(records, "ChatModel", "failed", "internal") {
		t.Fatalf("callback records = %#v, stream read error must not rely on ChatModel OnError", records)
	}
}

func TestConsumeAgentEventMessageClosesRetainedStream(t *testing.T) {
	reader, writer := schema.Pipe[*schema.Message](2)
	writer.Send(schema.AssistantMessage("hello ", nil), nil)
	writer.Send(schema.AssistantMessage("world", nil), nil)
	writer.Close()
	event := &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				IsStreaming:   true,
				MessageStream: reader,
				Role:          schema.Assistant,
			},
		},
	}

	message, err := consumeAgentEventMessage(event)
	if err != nil {
		t.Fatalf("consumeAgentEventMessage() error = %v", err)
	}
	if message.Content != "hello world" {
		t.Fatalf("consumeAgentEventMessage() content = %q", message.Content)
	}
	if _, err := event.Output.MessageOutput.MessageStream.Recv(); !errors.Is(err, schema.ErrRecvAfterClosed) {
		t.Fatalf("retained stream Recv() error = %v, want ErrRecvAfterClosed", err)
	}
}

func TestConsumeAgentEventMessageClosesRetainedStreamOnError(t *testing.T) {
	reader, writer := schema.Pipe[*schema.Message](2)
	writer.Send(schema.AssistantMessage("partial", nil), nil)
	writer.Send(nil, errModelStreamInterrupted)
	writer.Close()
	event := &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				IsStreaming:   true,
				MessageStream: reader,
				Role:          schema.Assistant,
			},
		},
	}

	message, err := consumeAgentEventMessage(event)
	if message != nil {
		t.Fatalf("consumeAgentEventMessage() message = %#v, want nil", message)
	}
	if !errors.Is(err, errModelStreamInterrupted) {
		t.Fatalf("consumeAgentEventMessage() error = %v, want stream error", err)
	}
	if _, err := event.Output.MessageOutput.MessageStream.Recv(); !errors.Is(err, schema.ErrRecvAfterClosed) {
		t.Fatalf("retained stream Recv() error = %v, want ErrRecvAfterClosed", err)
	}
}

func TestWeatherAgentFailures(t *testing.T) {
	tests := []struct {
		name      string
		provider  WeatherProvider
		context   func(t *testing.T) (context.Context, context.CancelFunc)
		wantError error
		wantKind  string
	}{
		{
			name: "deadline exceeded",
			provider: weatherProviderFunc(func(ctx context.Context, _ string) (Weather, error) {
				<-ctx.Done()
				return Weather{}, ctx.Err()
			}),
			context: func(t *testing.T) (context.Context, context.CancelFunc) {
				t.Helper()
				return context.WithTimeout(context.Background(), 30*time.Millisecond)
			},
			wantError: context.DeadlineExceeded,
			wantKind:  "deadline_exceeded",
		},
		{
			name: "unsupported city",
			provider: weatherProviderFunc(func(context.Context, string) (Weather, error) {
				return Weather{}, ErrUnsupportedCity
			}),
			context:   backgroundContext,
			wantError: ErrUnsupportedCity,
			wantKind:  "unsupported_city",
		},
		{
			name: "weather unavailable",
			provider: weatherProviderFunc(func(context.Context, string) (Weather, error) {
				return Weather{}, ErrWeatherUnavailable
			}),
			context:   backgroundContext,
			wantError: ErrWeatherUnavailable,
			wantKind:  "weather_unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := tt.context(t)
			defer cancel()
			chatModel := newScriptedWeatherModel("Beijing")
			agent, err := NewWeatherAgent(ctx, chatModel, tt.provider)
			if err != nil {
				t.Fatalf("NewWeatherAgent() error = %v", err)
			}
			observer := NewObserver(nil)

			message, err := agent.Query(ctx, "What is the weather in Beijing?", observer.Handler())
			if message != nil {
				t.Fatalf("Query() message = %#v, want nil", message)
			}
			if !errors.Is(err, tt.wantError) {
				t.Fatalf("Query() error = %v, want errors.Is(%v)", err, tt.wantError)
			}
			if !hasCallbackRecord(observer.Records(), "Tool", "failed", tt.wantKind) {
				t.Fatalf("callback records = %#v, want failed Tool/%s record", observer.Records(), tt.wantKind)
			}
		})
	}
}

func backgroundContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.Background(), func() {}
}

func hasCallbackRecord(records []CallbackRecord, component, status, errorKind string) bool {
	for _, record := range records {
		if record.Component == component && record.Status == status && record.ErrorKind == errorKind {
			return true
		}
	}
	return false
}
