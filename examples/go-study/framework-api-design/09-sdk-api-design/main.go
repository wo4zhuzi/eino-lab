package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrEmptyInput 表示调用方没有提供可处理的输入。
	ErrEmptyInput = errors.New("输入不能为空")
	// ErrNilContext 表示调用方传入了 nil context。
	ErrNilContext = errors.New("context 不能为空")
)

// Generator 是节点依赖的核心能力。调用方可以替换实现，而 Node 不依赖具体类型。
type Generator interface {
	Generate(context.Context, string) (string, error)
}

// GeneratorFunc 让简单函数也能作为 Generator 使用。
type GeneratorFunc func(context.Context, string) (string, error)

func (f GeneratorFunc) Generate(ctx context.Context, input string) (string, error) {
	return f(ctx, input)
}

// Logger 是可长期替换的外部能力，因此使用接口而不是单次事件回调。
type Logger interface {
	Info(context.Context, string)
	Error(context.Context, string, error)
}

type nopLogger struct{}

func (nopLogger) Info(context.Context, string)         {}
func (nopLogger) Error(context.Context, string, error) {}

// Completion 描述一次 Run 的最终结果。
type Completion struct {
	NodeName string
	Input    string
	Output   string
	Err      error
}

// CompletionCallback 是一次运行完成时的通知，不负责改变节点结果。
type CompletionCallback func(context.Context, Completion)

type nodeConfig struct {
	timeout      time.Duration
	logger       Logger
	onCompletion CompletionCallback
}

// Option 表示一项可选的节点配置。
type Option func(*nodeConfig) error

// WithTimeout 设置单次 Run 的最长执行时间。
func WithTimeout(timeout time.Duration) Option {
	return func(config *nodeConfig) error {
		if timeout <= 0 {
			return fmt.Errorf("超时时间必须大于 0")
		}
		config.timeout = timeout
		return nil
	}
}

// WithLogger 注入日志实现。
func WithLogger(logger Logger) Option {
	return func(config *nodeConfig) error {
		if logger == nil {
			return fmt.Errorf("logger 不能为空")
		}
		config.logger = logger
		return nil
	}
}

// WithCompletionCallback 注册单次运行完成后的通知函数。
func WithCompletionCallback(callback CompletionCallback) Option {
	return func(config *nodeConfig) error {
		if callback == nil {
			return fmt.Errorf("完成回调不能为空")
		}
		config.onCompletion = callback
		return nil
	}
}

// Node 组合稳定的公开 API 与可替换依赖。
type Node struct {
	name      string
	generator Generator
	config    nodeConfig
}

// NewNode 创建节点。名称和生成器决定对象是否成立，因此保留为必填参数。
func NewNode(name string, generator Generator, options ...Option) (*Node, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("节点名称不能为空")
	}
	if generator == nil {
		return nil, fmt.Errorf("generator 不能为空")
	}

	config := nodeConfig{
		timeout: 3 * time.Second,
		logger:  nopLogger{},
	}
	for index, option := range options {
		if option == nil {
			return nil, fmt.Errorf("第 %d 个配置项不能为空", index+1)
		}
		if err := option(&config); err != nil {
			return nil, fmt.Errorf("应用第 %d 个配置项失败: %w", index+1, err)
		}
	}

	return &Node{name: name, generator: generator, config: config}, nil
}

// Run 执行节点，并统一处理超时、日志、完成通知和错误传播。
func (n *Node) Run(ctx context.Context, input string) (output string, err error) {
	if ctx == nil {
		return "", ErrNilContext
	}

	runCtx, cancel := context.WithTimeout(ctx, n.config.timeout)
	defer cancel()

	input = strings.TrimSpace(input)
	n.config.logger.Info(runCtx, fmt.Sprintf("节点 %q 开始执行", n.name))
	defer func() {
		if err != nil {
			n.config.logger.Error(runCtx, fmt.Sprintf("节点 %q 执行失败", n.name), err)
		} else {
			n.config.logger.Info(runCtx, fmt.Sprintf("节点 %q 执行成功", n.name))
		}
		if n.config.onCompletion != nil {
			n.config.onCompletion(runCtx, Completion{
				NodeName: n.name,
				Input:    input,
				Output:   output,
				Err:      err,
			})
		}
	}()

	if input == "" {
		return "", ErrEmptyInput
	}
	if err := runCtx.Err(); err != nil {
		return "", fmt.Errorf("节点 %q 执行前检查 context: %w", n.name, err)
	}

	output, err = n.generator.Generate(runCtx, input)
	if err != nil {
		return "", fmt.Errorf("节点 %q 调用 generator 失败: %w", n.name, err)
	}
	return output, nil
}

type staticGenerator struct{}

func (staticGenerator) Generate(_ context.Context, input string) (string, error) {
	return "回答：" + input, nil
}

type consoleLogger struct{}

func (consoleLogger) Info(_ context.Context, message string) {
	fmt.Println("INFO:", message)
}

func (consoleLogger) Error(_ context.Context, message string, err error) {
	fmt.Printf("ERROR: %s: %v\n", message, err)
}

func main() {
	node, err := NewNode(
		"answer-node",
		staticGenerator{},
		WithTimeout(time.Second),
		WithLogger(consoleLogger{}),
		WithCompletionCallback(func(_ context.Context, completion Completion) {
			status := "success"
			if completion.Err != nil {
				status = "failed"
			}
			fmt.Printf("完成回调：node=%s status=%s\n", completion.NodeName, status)
		}),
	)
	if err != nil {
		panic(err)
	}

	output, err := node.Run(context.Background(), "SDK API 如何保持稳定？")
	if err != nil {
		panic(err)
	}
	fmt.Println(output)

	_, err = node.Run(context.Background(), "   ")
	fmt.Printf("失败结果：%v\n", err)
}
