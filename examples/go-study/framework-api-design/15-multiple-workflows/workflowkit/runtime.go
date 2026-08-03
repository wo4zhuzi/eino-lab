package workflowkit

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/cloudwego/eino/compose"
)

var (
	// ErrNilContext 表示构建或运行时没有可用的 context。
	ErrNilContext = errors.New("context 不能为空")
	// ErrInvalidName 表示工作流没有稳定名称。
	ErrInvalidName = errors.New("工作流名称无效")
	// ErrNilDefinition 表示没有传入可编译的 Chain 或 Graph。
	ErrNilDefinition = errors.New("工作流定义不能为空")
	// ErrRunnerNotInitialized 表示 Runner 尚未成功构建。
	ErrRunnerNotInitialized = errors.New("工作流 Runner 未初始化")
	// ErrNilDependency 表示业务工作流缺少必需依赖。
	ErrNilDependency = errors.New("工作流依赖不能为空")
	// ErrNilCompileOption 表示公共编译入口收到了 nil Option。
	ErrNilCompileOption = errors.New("工作流编译 Option 不能为空")
)

// Compilable 是 compose.Chain 和 compose.Graph 共有的最小编译能力。
// 它只抽取生命周期，不包装节点、Edge 或 Branch DSL。
type Compilable[I, O any] interface {
	Compile(
		ctx context.Context,
		opts ...compose.GraphCompileOption,
	) (compose.Runnable[I, O], error)
}

// Runner 保存一次编译得到的 Runnable，并提供统一运行入口。
type Runner[I, O any] struct {
	name     string
	runnable compose.Runnable[I, O]
}

// Compile 在启动阶段编译一次 Chain 或 Graph。
func Compile[I, O any](
	ctx context.Context,
	name string,
	definition Compilable[I, O],
	opts ...compose.GraphCompileOption,
) (*Runner[I, O], error) {
	if ctx == nil {
		return nil, ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("编译工作流: %w", err)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrInvalidName
	}
	if isNilDefinition(definition) {
		return nil, ErrNilDefinition
	}
	for index, option := range opts {
		if option == nil {
			return nil, fmt.Errorf("%w: 第 %d 个 Option", ErrNilCompileOption, index+1)
		}
	}

	compileOptions := append([]compose.GraphCompileOption(nil), opts...)
	compileOptions = append(compileOptions, compose.WithGraphName(name))
	runnable, err := definition.Compile(ctx, compileOptions...)
	if err != nil {
		return nil, fmt.Errorf("编译工作流 %q: %w", name, err)
	}
	if runnable == nil {
		return nil, fmt.Errorf("%w: %s", ErrRunnerNotInitialized, name)
	}
	return &Runner[I, O]{name: name, runnable: runnable}, nil
}

// Run 使用已编译的 Runnable 执行一次请求。
func (r *Runner[I, O]) Run(ctx context.Context, input I) (O, error) {
	var zero O
	if ctx == nil {
		return zero, ErrNilContext
	}
	if r == nil || r.runnable == nil {
		return zero, ErrRunnerNotInitialized
	}
	if err := ctx.Err(); err != nil {
		return zero, fmt.Errorf("运行工作流 %q: %w", r.name, err)
	}
	output, err := r.runnable.Invoke(ctx, input)
	if err != nil {
		return zero, fmt.Errorf("运行工作流 %q: %w", r.name, err)
	}
	return output, nil
}

// Name 返回用于日志和错误上下文的稳定工作流名称。
func (r *Runner[I, O]) Name() string {
	if r == nil {
		return ""
	}
	return r.name
}

// RequireDependency 校验依赖接口，包括接口中包含 typed nil 指针的情况。
func RequireDependency(name string, value any) error {
	if !isNilValue(value) {
		return nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "dependency"
	}
	return fmt.Errorf("%w: %s", ErrNilDependency, name)
}

func isNilDefinition(value any) bool {
	return isNilValue(value)
}

func isNilValue(value any) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
