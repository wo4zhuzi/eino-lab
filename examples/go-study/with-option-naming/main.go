package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type queryState struct {
	originalQuestion string
	finalAnswer      string
}

type stateHandler func(context.Context, string, *queryState) (string, error)

type nodeConfig struct {
	name        string
	timeout     time.Duration
	preHandler  stateHandler
	postHandler stateHandler
}

// Option 表示“一项节点配置”。
// 每个 With... 函数都会返回一个 Option，用于修改 nodeConfig。
type Option func(*nodeConfig)

// WithName：把节点名称配置为 name。
//
// 命名可以拆成：
//
//	With：配置一项内容
//	Name：配置的是名称
func WithName(name string) Option {
	return func(config *nodeConfig) {
		config.name = name
	}
}

// WithTimeout：把节点超时时间配置为 timeout。
// Timeout 是普通数据，因此参数类型是 time.Duration，不是函数。
func WithTimeout(timeout time.Duration) Option {
	return func(config *nodeConfig) {
		config.timeout = timeout
	}
}

// WithStatePreHandler：配置一个“节点执行前”的状态处理函数。
//
// 命名可以拆成：
//
//	With：配置一项内容
//	State：处理函数可以访问本次运行状态
//	Pre：在节点执行前调用
//	Handler：配置的是一个函数
func WithStatePreHandler(handler stateHandler) Option {
	return func(config *nodeConfig) {
		config.preHandler = handler
	}
}

// WithStatePostHandler：配置一个“节点执行后”的状态处理函数。
// Post 与 Pre 的区别只是执行时机不同。
func WithStatePostHandler(handler stateHandler) Option {
	return func(config *nodeConfig) {
		config.postHandler = handler
	}
}

type node struct {
	config nodeConfig
}

func newNode(options ...Option) *node {
	config := nodeConfig{
		name:    "unnamed",
		timeout: time.Second,
	}
	for _, option := range options {
		option(&config)
	}
	return &node{config: config}
}

// rememberOriginalQuestion 是一个 Handler 的具体实现。
// 函数名使用“动作 + 对象”：remember + OriginalQuestion。
func rememberOriginalQuestion(
	_ context.Context,
	question string,
	state *queryState,
) (string, error) {
	state.originalQuestion = question
	return strings.TrimSpace(question), nil
}

// rememberFinalAnswer 也是 Handler 的具体实现。
func rememberFinalAnswer(
	_ context.Context,
	answer string,
	state *queryState,
) (string, error) {
	state.finalAnswer = answer
	return answer, nil
}

func (n *node) Run(ctx context.Context, input string) (string, *queryState, error) {
	state := &queryState{}

	ctx, cancel := context.WithTimeout(ctx, n.config.timeout)
	defer cancel()

	if n.config.preHandler != nil {
		var err error
		input, err = n.config.preHandler(ctx, input, state)
		if err != nil {
			return "", state, err
		}
	}

	// 模拟节点的主要工作。
	output := "回答：" + input

	if n.config.postHandler != nil {
		var err error
		output, err = n.config.postHandler(ctx, output, state)
		if err != nil {
			return "", state, err
		}
	}

	return output, state, nil
}

func main() {
	queryNode := newNode(
		// 普通值配置：看到 WithName，先读成“配置名称”。
		WithName("answer_question"),

		// 普通值配置：看到 WithTimeout，先读成“配置超时”。
		WithTimeout(3*time.Second),

		// 函数配置：登记节点执行前调用的函数，此处不会立即执行。
		WithStatePreHandler(rememberOriginalQuestion),

		// 函数配置：登记节点执行后调用的函数，此处不会立即执行。
		WithStatePostHandler(rememberFinalAnswer),
	)

	output, state, err := queryNode.Run(context.Background(), "  state 从哪里来？  ")
	if err != nil {
		panic(err)
	}

	fmt.Printf("node_name=%s timeout=%s\n", queryNode.config.name, queryNode.config.timeout)
	fmt.Printf("original_question=%q\n", state.originalQuestion)
	fmt.Printf("output=%q\n", output)
	fmt.Printf("final_answer=%q\n", state.finalAnswer)
}
