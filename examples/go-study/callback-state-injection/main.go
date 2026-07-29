package main

import (
	"context"
	"fmt"
)

type queryState struct {
	question string
}

// statePostHandler 是框架要求的函数格式。
// 任何参数和返回值相同的函数，都可以注册成 PostHandler。
type statePostHandler func(
	context.Context,
	string,
	*queryState,
) (string, error)

// nodeConfig 保存一个节点最终使用的配置。
type nodeConfig struct {
	postHandler statePostHandler
}

// nodeOption 模拟 Eino 的 GraphAddNodeOpt。
// 它是一个函数：接收配置对象，并修改这个配置对象。
type nodeOption func(*nodeConfig)

// node 模拟完成配置后的 Graph 节点。
type node struct {
	config nodeConfig
}

// saveQuestion 是一个普通的命名函数。
// question 是节点输出，state 是框架为本次运行创建的状态。
func saveQuestion(
	_ context.Context,
	question string,
	state *queryState,
) (string, error) {
	state.question = question
	return question, nil
}

// withStatePostHandler 模拟 compose.WithStatePostHandler。
// 它接收 PostHandler，返回一个节点配置函数。
func withStatePostHandler(handler statePostHandler) nodeOption {
	return func(config *nodeConfig) {
		config.postHandler = handler
	}
}

// addLambdaNode 模拟 Graph.AddLambdaNode。
// 它执行所有 Option，把配置保存到节点中。
func addLambdaNode(options ...nodeOption) *node {
	configuredNode := &node{}
	for _, option := range options {
		option(&configuredNode.config)
	}
	return configuredNode
}

// runGraph 模拟 Eino 执行一次 Graph 中的节点。
func runGraph(ctx context.Context, input string, configuredNode *node) (string, *queryState, error) {
	// Eino 的 WithGenLocalState 会为每次运行创建独立状态。
	state := &queryState{}

	// 假设这是 validateQuestion 节点产生的输出。
	question := "已校验：" + input

	// 这一行模拟 Eino 从节点配置中取出并调用 PostHandler。
	// state 就是在这里由框架传给 saveQuestion 的。
	output, err := configuredNode.config.postHandler(ctx, question, state)
	if err != nil {
		return "", state, err
	}

	return output, state, nil
}

func main() {
	// withStatePostHandler 返回一个 Option；addLambdaNode 执行 Option，
	// 把 saveQuestion 保存到节点配置中。此时仍未调用 saveQuestion。
	configuredNode := addLambdaNode(
		withStatePostHandler(saveQuestion),
	)

	// runGraph 执行时，框架才从节点配置中取出 saveQuestion 并传入 state。
	output, state, err := runGraph(
		context.Background(),
		"state 从哪里来？",
		configuredNode,
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("节点最终输出：%q\n", output)
	fmt.Printf("PostHandler 保存的状态：%q\n", state.question)
}
