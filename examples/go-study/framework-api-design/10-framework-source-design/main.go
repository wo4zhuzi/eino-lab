package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/cloudwego/eino/compose"
)

var (
	ErrEmptyQuestion = errors.New("问题不能为空")
	ErrNilGenerator  = errors.New("generator 不能为空")
	ErrNilPostHandle = errors.New("post handler 不能为空")
)

const (
	nodeGenerate = "generate"
	nodeResult   = "build_result"
)

// traceState 是一次 Runnable.Invoke 独享的 Local State。
type traceState struct {
	ID     uint64
	Events []string
}

// TraceResult 同时返回业务输出和本次运行的状态快照。
type TraceResult struct {
	StateID uint64
	Output  string
	Events  []string
}

type generator func(context.Context, string) (string, error)

type postHandler func(context.Context, string, *traceState) (string, error)

// traceGraph 保存编译后的 Runnable；计数器仅用于观察配置与运行阶段的边界。
type traceGraph struct {
	runnable       compose.Runnable[string, TraceResult]
	stateCreations atomic.Uint64
	postCalls      atomic.Uint64
}

func newTraceGraph(generate generator, post postHandler) (*traceGraph, error) {
	if generate == nil {
		return nil, ErrNilGenerator
	}
	if post == nil {
		return nil, ErrNilPostHandle
	}

	result := &traceGraph{}
	graph := compose.NewGraph[string, TraceResult](
		compose.WithGenLocalState(func(context.Context) *traceState {
			return &traceState{ID: result.stateCreations.Add(1)}
		}),
	)

	if err := graph.AddLambdaNode(
		nodeGenerate,
		compose.InvokableLambda(compose.InvokeWOOpt[string, string](generate)),
		compose.WithNodeName(nodeGenerate),
		compose.WithStatePostHandler(func(
			ctx context.Context,
			output string,
			state *traceState,
		) (string, error) {
			result.postCalls.Add(1)
			return post(ctx, output, state)
		}),
	); err != nil {
		return nil, fmt.Errorf("添加 %s 节点: %w", nodeGenerate, err)
	}

	buildResult := compose.InvokableLambda(func(ctx context.Context, output string) (TraceResult, error) {
		var traced TraceResult
		if err := compose.ProcessState[*traceState](ctx, func(_ context.Context, state *traceState) error {
			traced = TraceResult{
				StateID: state.ID,
				Output:  output,
				Events:  append([]string(nil), state.Events...),
			}
			return nil
		}); err != nil {
			return TraceResult{}, fmt.Errorf("读取运行状态: %w", err)
		}
		return traced, nil
	})
	if err := graph.AddLambdaNode(nodeResult, buildResult, compose.WithNodeName(nodeResult)); err != nil {
		return nil, fmt.Errorf("添加 %s 节点: %w", nodeResult, err)
	}

	for _, edge := range [][2]string{
		{compose.START, nodeGenerate},
		{nodeGenerate, nodeResult},
		{nodeResult, compose.END},
	} {
		if err := graph.AddEdge(edge[0], edge[1]); err != nil {
			return nil, fmt.Errorf("添加边 %s -> %s: %w", edge[0], edge[1], err)
		}
	}

	runnable, err := graph.Compile(context.Background(), compose.WithGraphName("state_post_handler_trace"))
	if err != nil {
		return nil, fmt.Errorf("编译源码追踪 Graph: %w", err)
	}
	result.runnable = runnable
	return result, nil
}

func newDefaultTraceGraph() (*traceGraph, error) {
	return newTraceGraph(
		func(_ context.Context, question string) (string, error) {
			question = strings.TrimSpace(question)
			if question == "" {
				return "", ErrEmptyQuestion
			}
			return "回答：" + question, nil
		},
		func(_ context.Context, output string, state *traceState) (string, error) {
			state.Events = append(state.Events, "post:"+output)
			return output + " [已记录]", nil
		},
	)
}

func (g *traceGraph) Invoke(ctx context.Context, question string) (TraceResult, error) {
	return g.runnable.Invoke(ctx, question)
}

func (g *traceGraph) Counts() (stateCreations uint64, postCalls uint64) {
	return g.stateCreations.Load(), g.postCalls.Load()
}

func main() {
	graph, err := newDefaultTraceGraph()
	if err != nil {
		panic(err)
	}

	stateCreations, postCalls := graph.Counts()
	fmt.Printf("构建后：state=%d post=%d\n", stateCreations, postCalls)

	for _, question := range []string{"配置保存在哪里？", "Handler 在哪里调用？"} {
		result, err := graph.Invoke(context.Background(), question)
		if err != nil {
			panic(err)
		}
		fmt.Printf("运行：state=%d output=%q events=%q\n", result.StateID, result.Output, result.Events)
	}
}
