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
	name   string
	run    func(context.Context, string) (string, error)
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

// answerFromState 模拟后续节点读取前一个节点保存的 Local State。
func answerFromState(
	_ context.Context,
	_ string,
	state *queryState,
) (string, error) {
	return "回答基于：" + state.question, nil
}

// withStatePostHandler 模拟 compose.WithStatePostHandler。
// 它接收 PostHandler，返回一个节点配置函数。
func withStatePostHandler(handler statePostHandler) nodeOption {
	return func(config *nodeConfig) {
		config.postHandler = handler
	}
}

// newNode 模拟 Graph.AddLambdaNode 的节点构建过程。
func newNode(
	name string,
	run func(context.Context, string) (string, error),
	options ...nodeOption,
) *node {
	configuredNode := &node{name: name, run: run}
	for _, option := range options {
		option(&configuredNode.config)
	}
	return configuredNode
}

// sequentialGraph 在构建阶段保存节点拓扑，运行时不再逐个传入节点。
type sequentialGraph struct {
	nodes []*node
}

func newSequentialGraph(nodes ...*node) *sequentialGraph {
	return &sequentialGraph{nodes: nodes}
}

// Run 模拟 Eino 执行一次编译后的 Graph。
func (g *sequentialGraph) Run(
	ctx context.Context,
	input string,
) (string, *queryState, error) {
	// Eino 的 WithGenLocalState 会为每次运行创建独立状态。
	state := &queryState{}
	output := input
	for _, current := range g.nodes {
		var err error
		output, err = current.run(ctx, output)
		if err != nil {
			return "", state, fmt.Errorf("节点 %s 执行失败: %w", current.name, err)
		}
		if current.config.postHandler != nil {
			output, err = current.config.postHandler(ctx, output, state)
			if err != nil {
				return "", state, fmt.Errorf("节点 %s PostHandler 执行失败: %w", current.name, err)
			}
		}
	}
	return output, state, nil
}

func validateQuestion(_ context.Context, input string) (string, error) {
	return "已校验：" + input, nil
}

func draftAnswer(_ context.Context, _ string) (string, error) {
	return "回答草稿", nil
}

func main() {
	validateNode := newNode(
		"validate_question",
		validateQuestion,
		withStatePostHandler(saveQuestion),
	)
	answerNode := newNode(
		"answer_question",
		draftAnswer,
		withStatePostHandler(answerFromState),
	)
	graph := newSequentialGraph(validateNode, answerNode)

	// 节点在构建阶段已经注册，运行阶段只传本次请求的 ctx 和 input。
	output, state, err := graph.Run(
		context.Background(),
		"state 从哪里来？",
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("节点 1 保存的状态：%q\n", state.question)
	fmt.Printf("节点 2 读取状态后的输出：%q\n", output)
}
