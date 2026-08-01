package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var errEmptyQuestion = errors.New("问题不能为空")

type handler func(context.Context, string) (string, error)
type beforeHook func(context.Context, string) (string, error)
type afterHook func(context.Context, string) (string, error)
type errorHook func(context.Context, error) error

type hooks struct {
	before  beforeHook
	after   afterHook
	onError errorHook
}

type taskRunner struct {
	run   handler
	hooks hooks
}

// Run 控制生命周期。业务代码只登记钩子，不直接决定它们的调用时机。
func (r taskRunner) Run(ctx context.Context, input string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", r.handleError(ctx, fmt.Errorf("任务开始前检查 context: %w", err))
	}

	current := input
	if r.hooks.before != nil {
		var err error
		current, err = r.hooks.before(ctx, current)
		if err != nil {
			return "", r.handleError(ctx, fmt.Errorf("BeforeHook 执行失败: %w", err))
		}
	}

	output, err := r.run(ctx, current)
	if err != nil {
		return "", r.handleError(ctx, fmt.Errorf("主逻辑执行失败: %w", err))
	}

	if r.hooks.after != nil {
		output, err = r.hooks.after(ctx, output)
		if err != nil {
			return "", r.handleError(ctx, fmt.Errorf("AfterHook 执行失败: %w", err))
		}
	}
	return output, nil
}

func (r taskRunner) handleError(ctx context.Context, runErr error) error {
	if r.hooks.onError == nil {
		return runErr
	}
	if hookErr := r.hooks.onError(ctx, runErr); hookErr != nil {
		return errors.Join(runErr, fmt.Errorf("ErrorHook 执行失败: %w", hookErr))
	}
	return runErr
}

func normalizeQuestion(_ context.Context, input string) (string, error) {
	question := strings.TrimSpace(input)
	if question == "" {
		return "", errEmptyQuestion
	}
	return question, nil
}

func answer(_ context.Context, question string) (string, error) {
	return "回答：" + question, nil
}

func addRequestID(_ context.Context, output string) (string, error) {
	return output + " [request_id=req-001]", nil
}

func reportError(_ context.Context, err error) error {
	fmt.Printf("ErrorHook 观察到：%v\n", err)
	return nil
}

func main() {
	runner := taskRunner{
		run: answer,
		hooks: hooks{
			before:  normalizeQuestion,
			after:   addRequestID,
			onError: reportError,
		},
	}

	output, err := runner.Run(context.Background(), "  生命周期钩子何时执行？  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(output)

	_, err = runner.Run(context.Background(), "   ")
	fmt.Printf("失败结果：%v\n", err)
}
