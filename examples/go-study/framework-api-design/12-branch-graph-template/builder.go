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
	// ErrInvalidGraphDefinition 表示公共构建器收到的 Graph 定义无效。
	ErrInvalidGraphDefinition = errors.New("Graph 定义无效")
)

// graphDefinition 是“业务拓扑定义函数”的类型。
//
// I 和 O 仍然是整个 Graph 的对外输入、输出类型。业务代码通过 define 参数拿到
// *compose.Graph[I, O]，只负责向其中注册节点、固定 Edge 和 Branch。
type graphDefinition[I, O any] func(graph *compose.Graph[I, O]) error

// compileDefinedGraph 是 Demo 12 的固定公共构建器。
//
// 这个函数只处理所有 Graph 都相同的公共生命周期：
//
//  1. 注册期开始前：校验公共参数并创建 Graph。
//  2. 注册期：调用 define，让业务代码登记节点、Edge 和 Branch。
//  3. 编译期：调用 Compile，把声明的拓扑编译成 Runnable。
//
// 它不认识审核节点，也不判断应该走哪个分支，因此业务拓扑变化时无需修改这里。
func compileDefinedGraph[I, O any](
	ctx context.Context,
	graphName string,
	define graphDefinition[I, O],
	graphOptions ...compose.NewGraphOption,
) (compose.Runnable[I, O], error) {
	graphName = strings.TrimSpace(graphName)
	if err := validateGraphDefinition(ctx, graphName, define, graphOptions); err != nil {
		return nil, err
	}

	// NewGraph[I, O] 在这里固定整个 Graph 的外部边界。
	// 本示例实例化后，I 是 ReviewRequest，O 是 ReviewResult。
	graph := compose.NewGraph[I, O](graphOptions...)

	// 此时只是在内存中登记拓扑，还没有执行任何业务节点。
	if err := define(graph); err != nil {
		return nil, fmt.Errorf("定义 Graph %q: %w", graphName, err)
	}

	// Compile 之后才得到可反复 Invoke 的 Runnable。
	runnable, err := graph.Compile(ctx, compose.WithGraphName(graphName))
	if err != nil {
		return nil, fmt.Errorf("编译 Graph %q: %w", graphName, err)
	}
	return runnable, nil
}

func validateGraphDefinition[I, O any](
	ctx context.Context,
	graphName string,
	define graphDefinition[I, O],
	graphOptions []compose.NewGraphOption,
) error {
	if ctx == nil {
		return fmt.Errorf("%w: %w", ErrInvalidGraphDefinition, ErrNilContext)
	}
	if graphName == "" {
		return fmt.Errorf("%w: Graph 名称不能为空", ErrInvalidGraphDefinition)
	}
	if define == nil {
		return fmt.Errorf("%w: Graph 定义函数不能为空", ErrInvalidGraphDefinition)
	}
	for index, option := range graphOptions {
		if option == nil {
			return fmt.Errorf("%w: 第 %d 个 Graph Option 不能为空", ErrInvalidGraphDefinition, index+1)
		}
	}
	return nil
}
