package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"
)

var (
	// ErrNilContext 表示构建 Graph 时没有可用的 context。
	ErrNilContext = errors.New("context 不能为空")
	// ErrInvalidLinearGraph 表示公共构建器收到的名称、边界函数或步骤定义无效。
	ErrInvalidLinearGraph = errors.New("线性 Graph 定义无效")
)

const (
	inputAdapterNode  = "input_adapter"
	outputAdapterNode = "output_adapter"
)

// linearStep 描述一个输入输出类型相同的中间节点。
//
// 所有中间节点统一使用 M -> M，公共构建器才能在不知道业务细节的情况下，
// 自动按切片顺序注册节点并连接 Edge。
type linearStep[M any] struct {
	Key string
	Run compose.InvokeWOOpt[M, M]
}

// compileLinearGraph 是 Demo 11 的固定公共构建器。
//
// I 是 Graph 对外输入，M 是所有中间步骤共用的数据类型，O 是 Graph 对外输出。
// 调用方只提供两个边界适配函数和一组有序步骤，公共构建器固定完成：
//
//	NewGraph[I, O]
//	  -> 注册 I -> M 输入适配节点
//	  -> 按顺序注册全部 M -> M 中间节点
//	  -> 注册 M -> O 输出适配节点
//	  -> 自动连接 START、全部节点和 END
//	  -> Compile 得到 Runnable[I, O]
//
// 只增加、删除或调整线性业务步骤时，不需要修改这个函数。
func compileLinearGraph[I, M, O any](
	ctx context.Context,
	graphName string,
	adaptInput compose.InvokeWOOpt[I, M],
	steps []linearStep[M],
	adaptOutput compose.InvokeWOOpt[M, O],
	graphOptions ...compose.NewGraphOption,
) (compose.Runnable[I, O], error) {
	graphName = strings.TrimSpace(graphName)
	if err := validateLinearGraph(ctx, graphName, adaptInput, steps, adaptOutput, graphOptions); err != nil {
		return nil, err
	}

	graph := compose.NewGraph[I, O](graphOptions...)
	orderedNodeKeys := make([]string, 0, len(steps)+2)

	if err := graph.AddLambdaNode(
		inputAdapterNode,
		compose.InvokableLambda(adaptInput),
		compose.WithNodeName(inputAdapterNode),
	); err != nil {
		return nil, fmt.Errorf("添加输入适配节点: %w", err)
	}
	orderedNodeKeys = append(orderedNodeKeys, inputAdapterNode)

	for _, step := range steps {
		if err := graph.AddLambdaNode(
			step.Key,
			compose.InvokableLambda(step.Run),
			compose.WithNodeName(step.Key),
		); err != nil {
			return nil, fmt.Errorf("添加中间节点 %q: %w", step.Key, err)
		}
		orderedNodeKeys = append(orderedNodeKeys, step.Key)
	}

	if err := graph.AddLambdaNode(
		outputAdapterNode,
		compose.InvokableLambda(adaptOutput),
		compose.WithNodeName(outputAdapterNode),
	); err != nil {
		return nil, fmt.Errorf("添加输出适配节点: %w", err)
	}
	orderedNodeKeys = append(orderedNodeKeys, outputAdapterNode)

	previous := compose.START
	for _, current := range orderedNodeKeys {
		if err := graph.AddEdge(previous, current); err != nil {
			return nil, fmt.Errorf("添加边 %s -> %s: %w", previous, current, err)
		}
		previous = current
	}
	if err := graph.AddEdge(previous, compose.END); err != nil {
		return nil, fmt.Errorf("添加边 %s -> %s: %w", previous, compose.END, err)
	}

	runnable, err := graph.Compile(ctx, compose.WithGraphName(graphName))
	if err != nil {
		return nil, fmt.Errorf("编译 Graph %q: %w", graphName, err)
	}
	return runnable, nil
}

func validateLinearGraph[I, M, O any](
	ctx context.Context,
	graphName string,
	adaptInput compose.InvokeWOOpt[I, M],
	steps []linearStep[M],
	adaptOutput compose.InvokeWOOpt[M, O],
	graphOptions []compose.NewGraphOption,
) error {
	if ctx == nil {
		return fmt.Errorf("%w: %w", ErrInvalidLinearGraph, ErrNilContext)
	}
	if graphName == "" {
		return fmt.Errorf("%w: Graph 名称不能为空", ErrInvalidLinearGraph)
	}
	if adaptInput == nil {
		return fmt.Errorf("%w: 输入适配函数不能为空", ErrInvalidLinearGraph)
	}
	if adaptOutput == nil {
		return fmt.Errorf("%w: 输出适配函数不能为空", ErrInvalidLinearGraph)
	}
	for index, option := range graphOptions {
		if option == nil {
			return fmt.Errorf("%w: 第 %d 个 Graph Option 不能为空", ErrInvalidLinearGraph, index+1)
		}
	}

	seen := map[string]struct{}{
		inputAdapterNode:  {},
		outputAdapterNode: {},
		compose.START:     {},
		compose.END:       {},
	}
	for index, step := range steps {
		key := strings.TrimSpace(step.Key)
		if key == "" {
			return fmt.Errorf("%w: 第 %d 个中间节点 key 不能为空", ErrInvalidLinearGraph, index+1)
		}
		if key != step.Key {
			return fmt.Errorf("%w: 中间节点 key %q 不能包含首尾空白", ErrInvalidLinearGraph, step.Key)
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("%w: 中间节点 key %q 重复或为保留名称", ErrInvalidLinearGraph, key)
		}
		if step.Run == nil {
			return fmt.Errorf("%w: 中间节点 %q 的 Handler 不能为空", ErrInvalidLinearGraph, key)
		}
		seen[key] = struct{}{}
	}
	return nil
}
