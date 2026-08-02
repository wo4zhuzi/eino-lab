package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type recordingLogger struct {
	entries []string
}

func (l *recordingLogger) Info(_ context.Context, message string) {
	l.entries = append(l.entries, "info:"+message)
}

func (l *recordingLogger) Error(_ context.Context, message string, err error) {
	l.entries = append(l.entries, "error:"+message+":"+err.Error())
}

func TestNodeRunUsesInjectedDependencies(t *testing.T) {
	logger := &recordingLogger{}
	var completion Completion
	node, err := NewNode(
		"answer",
		GeneratorFunc(func(_ context.Context, input string) (string, error) {
			return strings.ToUpper(input), nil
		}),
		WithLogger(logger),
		WithCompletionCallback(func(_ context.Context, result Completion) {
			completion = result
		}),
	)
	if err != nil {
		t.Fatalf("NewNode() error = %v", err)
	}
	if node.config.timeout != 3*time.Second {
		t.Fatalf("default timeout = %s, want %s", node.config.timeout, 3*time.Second)
	}

	output, err := node.Run(context.Background(), "  sdk  ")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if output != "SDK" {
		t.Fatalf("Run() output = %q, want %q", output, "SDK")
	}
	if completion.NodeName != "answer" || completion.Input != "sdk" || completion.Output != "SDK" || completion.Err != nil {
		t.Fatalf("completion = %#v, want successful normalized result", completion)
	}
	wantLogs := []string{
		"info:节点 \"answer\" 开始执行",
		"info:节点 \"answer\" 执行成功",
	}
	if !reflect.DeepEqual(logger.entries, wantLogs) {
		t.Fatalf("logger.entries = %#v, want %#v", logger.entries, wantLogs)
	}
}

func TestNodeRunPreservesGeneratorError(t *testing.T) {
	errDependency := errors.New("依赖不可用")
	var completion Completion
	node, err := NewNode(
		"answer",
		GeneratorFunc(func(context.Context, string) (string, error) {
			return "", errDependency
		}),
		WithCompletionCallback(func(_ context.Context, result Completion) {
			completion = result
		}),
	)
	if err != nil {
		t.Fatalf("NewNode() error = %v", err)
	}

	_, err = node.Run(context.Background(), "question")
	if !errors.Is(err, errDependency) {
		t.Fatalf("Run() error = %v, want errors.Is(errDependency)", err)
	}
	if !errors.Is(completion.Err, errDependency) {
		t.Fatalf("completion.Err = %v, want errors.Is(errDependency)", completion.Err)
	}
}

func TestNodeRunPropagatesTimeout(t *testing.T) {
	node, err := NewNode(
		"slow",
		GeneratorFunc(func(ctx context.Context, _ string) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		}),
		WithTimeout(10*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewNode() error = %v", err)
	}

	_, err = node.Run(context.Background(), "question")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestNodeRunRejectsEmptyInputBeforeGenerator(t *testing.T) {
	called := false
	node, err := NewNode("answer", GeneratorFunc(func(context.Context, string) (string, error) {
		called = true
		return "", nil
	}))
	if err != nil {
		t.Fatalf("NewNode() error = %v", err)
	}

	_, err = node.Run(context.Background(), "   ")
	if !errors.Is(err, ErrEmptyInput) {
		t.Fatalf("Run() error = %v, want ErrEmptyInput", err)
	}
	if called {
		t.Fatal("空输入不应调用 generator")
	}
}

func TestNodeRunRejectsNilContext(t *testing.T) {
	node, err := NewNode("answer", GeneratorFunc(func(context.Context, string) (string, error) {
		return "result", nil
	}))
	if err != nil {
		t.Fatalf("NewNode() error = %v", err)
	}

	_, err = node.Run(nil, "question")
	if !errors.Is(err, ErrNilContext) {
		t.Fatalf("Run() error = %v, want ErrNilContext", err)
	}
}

func TestNewNodeRejectsInvalidConstruction(t *testing.T) {
	generator := GeneratorFunc(func(context.Context, string) (string, error) {
		return "result", nil
	})
	tests := []struct {
		name    string
		node    string
		gen     Generator
		options []Option
		want    string
	}{
		{name: "empty name", node: " ", gen: generator, want: "节点名称不能为空"},
		{name: "nil generator", node: "node", want: "generator 不能为空"},
		{name: "nil option", node: "node", gen: generator, options: []Option{nil}, want: "配置项不能为空"},
		{name: "invalid timeout", node: "node", gen: generator, options: []Option{WithTimeout(0)}, want: "超时时间必须大于 0"},
		{name: "nil logger", node: "node", gen: generator, options: []Option{WithLogger(nil)}, want: "logger 不能为空"},
		{name: "nil callback", node: "node", gen: generator, options: []Option{WithCompletionCallback(nil)}, want: "完成回调不能为空"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewNode(test.node, test.gen, test.options...)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("NewNode() error = %v, want containing %q", err, test.want)
			}
		})
	}
}
