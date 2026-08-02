package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/compose"
)

func TestConfigurationDoesNotCreateStateOrCallPostHandler(t *testing.T) {
	graph, err := newDefaultTraceGraph()
	if err != nil {
		t.Fatalf("newDefaultTraceGraph() error = %v", err)
	}

	stateCreations, postCalls := graph.Counts()
	if stateCreations != 0 || postCalls != 0 {
		t.Fatalf("构建后 counts = (%d, %d), want (0, 0)", stateCreations, postCalls)
	}

	result, err := graph.Invoke(context.Background(), ReviewRequest{Question: "  配置保存在哪里？  "})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.StateID != 1 {
		t.Fatalf("StateID = %d, want 1", result.StateID)
	}
	if result.Answer != "回答：配置保存在哪里？ [已记录]" {
		t.Fatalf("Answer = %q", result.Answer)
	}
	wantEvents := []string{"post:回答：配置保存在哪里？"}
	if !reflect.DeepEqual(result.Events, wantEvents) {
		t.Fatalf("Events = %#v, want %#v", result.Events, wantEvents)
	}
	stateCreations, postCalls = graph.Counts()
	if stateCreations != 1 || postCalls != 1 {
		t.Fatalf("运行后 counts = (%d, %d), want (1, 1)", stateCreations, postCalls)
	}
}

func TestEachInvokeGetsIndependentState(t *testing.T) {
	graph, err := newDefaultTraceGraph()
	if err != nil {
		t.Fatalf("newDefaultTraceGraph() error = %v", err)
	}

	const runs = 32
	results := make(chan ReviewResult, runs)
	errorsCh := make(chan error, runs)
	var wait sync.WaitGroup
	for index := 0; index < runs; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			result, err := graph.Invoke(context.Background(), ReviewRequest{
				Question: fmt.Sprintf("question-%d", index),
			})
			if err != nil {
				errorsCh <- err
				return
			}
			results <- result
		}(index)
	}
	wait.Wait()
	close(results)
	close(errorsCh)

	for err := range errorsCh {
		t.Errorf("Invoke() error = %v", err)
	}
	stateIDs := make(map[uint64]struct{}, runs)
	for result := range results {
		if len(result.Events) != 1 {
			t.Errorf("StateID %d Events = %#v, want one event", result.StateID, result.Events)
		}
		stateIDs[result.StateID] = struct{}{}
	}
	if len(stateIDs) != runs {
		t.Fatalf("unique StateIDs = %d, want %d", len(stateIDs), runs)
	}
	stateCreations, postCalls := graph.Counts()
	if stateCreations != runs || postCalls != runs {
		t.Fatalf("counts = (%d, %d), want (%d, %d)", stateCreations, postCalls, runs, runs)
	}
}

func TestNodeErrorSkipsPostHandler(t *testing.T) {
	errGenerate := errors.New("生成依赖不可用")
	graph, err := newTraceGraph(
		func(context.Context, ReviewRequest) (string, error) {
			return "", errGenerate
		},
		func(_ context.Context, output string, _ *traceState) (string, error) {
			return output, nil
		},
	)
	if err != nil {
		t.Fatalf("newTraceGraph() error = %v", err)
	}

	_, err = graph.Invoke(context.Background(), ReviewRequest{Question: "question"})
	if !errors.Is(err, errGenerate) {
		t.Fatalf("Invoke() error = %v, want errors.Is(errGenerate)", err)
	}
	stateCreations, postCalls := graph.Counts()
	if stateCreations != 1 || postCalls != 0 {
		t.Fatalf("counts = (%d, %d), want (1, 0)", stateCreations, postCalls)
	}
}

func TestPostHandlerErrorPreservesRootCause(t *testing.T) {
	errPost := errors.New("状态写入失败")
	graph, err := newTraceGraph(
		func(_ context.Context, request ReviewRequest) (string, error) {
			return request.Question, nil
		},
		func(_ context.Context, _ string, _ *traceState) (string, error) {
			return "", errPost
		},
	)
	if err != nil {
		t.Fatalf("newTraceGraph() error = %v", err)
	}

	_, err = graph.Invoke(context.Background(), ReviewRequest{Question: "question"})
	if !errors.Is(err, errPost) {
		t.Fatalf("Invoke() error = %v, want errors.Is(errPost)", err)
	}
	if !strings.Contains(err.Error(), "post processor") || !strings.Contains(err.Error(), nodeGenerate) {
		t.Fatalf("Invoke() error = %q, want post processor and node path", err)
	}
	_, postCalls := graph.Counts()
	if postCalls != 1 {
		t.Fatalf("post calls = %d, want 1", postCalls)
	}
}

func TestAddNodeRejectsPostHandlerWithoutLocalState(t *testing.T) {
	graph := compose.NewGraph[ReviewRequest, string]()
	err := graph.AddLambdaNode(
		"node",
		compose.InvokableLambda(func(_ context.Context, request ReviewRequest) (string, error) {
			return request.Question, nil
		}),
		compose.WithStatePostHandler(func(_ context.Context, output string, _ *traceState) (string, error) {
			return output, nil
		}),
	)
	if err == nil || !strings.Contains(err.Error(), "needs state") {
		t.Fatalf("AddLambdaNode() error = %v, want missing state error", err)
	}
}

func TestAddNodeRejectsPostHandlerOutputTypeMismatch(t *testing.T) {
	graph := compose.NewGraph[ReviewRequest, int](
		compose.WithGenLocalState(func(context.Context) *traceState { return &traceState{} }),
	)
	err := graph.AddLambdaNode(
		"node",
		compose.InvokableLambda(func(context.Context, ReviewRequest) (int, error) {
			return 1, nil
		}),
		compose.WithStatePostHandler(func(_ context.Context, output string, _ *traceState) (string, error) {
			return output, nil
		}),
	)
	if err == nil || !strings.Contains(err.Error(), "different from its output type") {
		t.Fatalf("AddLambdaNode() error = %v, want output type mismatch", err)
	}
}

func TestNewTraceGraphRejectsNilHandlers(t *testing.T) {
	validGenerator := func(context.Context, ReviewRequest) (string, error) { return "", nil }
	validPost := func(_ context.Context, output string, _ *traceState) (string, error) { return output, nil }

	if _, err := newTraceGraph(nil, validPost); !errors.Is(err, ErrNilGenerator) {
		t.Fatalf("newTraceGraph(nil, validPost) error = %v, want ErrNilGenerator", err)
	}
	if _, err := newTraceGraph(validGenerator, nil); !errors.Is(err, ErrNilPostHandle) {
		t.Fatalf("newTraceGraph(validGenerator, nil) error = %v, want ErrNilPostHandle", err)
	}
}
