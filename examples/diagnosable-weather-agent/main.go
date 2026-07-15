package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
)

const defaultTimeout = 15 * time.Second

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

func run(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	query := strings.TrimSpace(strings.Join(args, " "))
	if query == "" {
		fmt.Fprintln(stderr, "用法: go run ./examples/diagnosable-weather-agent \"北京天气怎么样？\"")
		return 2
	}

	apiKey := strings.TrimSpace(getenv("OPENAI_API_KEY"))
	modelName := strings.TrimSpace(getenv("OPENAI_MODEL"))
	if apiKey == "" || modelName == "" {
		fmt.Fprintln(stderr, "配置错误: OPENAI_API_KEY 和 OPENAI_MODEL 必填")
		return 2
	}

	timeout, err := configuredTimeout(getenv("WEATHER_AGENT_TIMEOUT"))
	if err != nil {
		fmt.Fprintln(stderr, "配置错误: WEATHER_AGENT_TIMEOUT 必须是大于 0 的 Go duration，例如 15s")
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  apiKey,
		Model:   modelName,
		BaseURL: strings.TrimSpace(getenv("OPENAI_BASE_URL")),
		Timeout: timeout,
	})
	if err != nil {
		fmt.Fprintf(stderr, "模型初始化失败（%s）\n", ErrorKind(err))
		return 1
	}

	agent, err := NewWeatherAgent(ctx, chatModel, NewStaticWeatherProvider())
	if err != nil {
		fmt.Fprintf(stderr, "Agent 初始化失败（%s）\n", ErrorKind(err))
		return 1
	}

	logger := slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	observer := NewObserver(logger)
	message, err := agent.Query(ctx, query, observer.Handler())
	if err != nil {
		fmt.Fprintf(stderr, "运行失败（%s）\n", ErrorKind(err))
		return 1
	}

	fmt.Fprintln(stdout, message.Content)
	return 0
}

func configuredTimeout(value string) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return defaultTimeout, nil
	}
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid timeout")
	}
	return duration, nil
}
