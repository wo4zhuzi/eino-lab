package main

import (
	"context"
	"errors"
	"fmt"
)

var errUnauthorized = errors.New("未通过鉴权")

type principalKey struct{}

// Handler 表示可以独立执行的一段业务处理逻辑。
type Handler func(context.Context, string) (string, error)

// Middleware 接收旧 Handler，并返回增加了新行为的 Handler。
type Middleware func(Handler) Handler

// Chain 按注册顺序组装中间件：第一个中间件位于最外层，并最先收到请求。
func Chain(final Handler, middlewares ...Middleware) Handler {
	wrapped := final
	for i := len(middlewares) - 1; i >= 0; i-- {
		wrapped = middlewares[i](wrapped)
	}
	return wrapped
}

func loggingMiddleware(next Handler) Handler {
	return func(ctx context.Context, input string) (string, error) {
		fmt.Println("日志：进入")
		output, err := next(ctx, input)
		if err != nil {
			fmt.Println("日志：失败")
			return "", fmt.Errorf("日志中间件观察到下游失败: %w", err)
		}
		fmt.Println("日志：退出")
		return output, nil
	}
}

func authenticationMiddleware(next Handler) Handler {
	return func(ctx context.Context, input string) (string, error) {
		principal, _ := ctx.Value(principalKey{}).(string)
		if principal == "" {
			// 不调用 next，因此后面的中间件和核心 Handler 都不会执行。
			return "", errUnauthorized
		}
		return next(ctx, input)
	}
}

func contextMiddleware(next Handler) Handler {
	return func(ctx context.Context, input string) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("请求已取消: %w", err)
		}
		return next(ctx, input)
	}
}

func answer(_ context.Context, question string) (string, error) {
	return "回答：" + question, nil
}

func main() {
	handler := Chain(
		answer,
		loggingMiddleware,
		authenticationMiddleware,
		contextMiddleware,
	)

	ctx := context.WithValue(context.Background(), principalKey{}, "user-001")
	output, err := handler(ctx, "中间件为什么返回 Handler？")
	if err != nil {
		panic(err)
	}
	fmt.Println(output)

	_, err = handler(context.Background(), "未登录请求")
	fmt.Printf("失败结果：%v\n", err)
}
