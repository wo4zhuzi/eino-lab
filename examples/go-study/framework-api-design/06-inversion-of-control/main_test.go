package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

func TestNodesShareStateWithinOneRun(t *testing.T) {
	graph := newTestGraph()

	output, state, err := graph.Run(context.Background(), "问题")
	if err != nil {
		t.Fatalf("graph.Run() error = %v", err)
	}
	if state.question != "已校验：问题" {
		t.Fatalf("state.question = %q, want %q", state.question, "已校验：问题")
	}
	if output != "回答草稿；回答基于：已校验：问题" {
		t.Fatalf("output = %q, want %q", output, "回答草稿；回答基于：已校验：问题")
	}
}

func TestEachRunCreatesIndependentState(t *testing.T) {
	graph := newTestGraph()

	_, first, err := graph.Run(context.Background(), "第一个问题")
	if err != nil {
		t.Fatalf("first graph.Run() error = %v", err)
	}
	_, second, err := graph.Run(context.Background(), "第二个问题")
	if err != nil {
		t.Fatalf("second graph.Run() error = %v", err)
	}

	if first == second {
		t.Fatal("两次运行不应该共享同一个状态对象")
	}
	if first.question != "已校验：第一个问题" || second.question != "已校验：第二个问题" {
		t.Fatalf("状态发生串扰：first=%q, second=%q", first.question, second.question)
	}
}

func TestConcurrentRunsUseIndependentState(t *testing.T) {
	graph := newTestGraph()
	const runCount = 20

	var waitGroup sync.WaitGroup
	errors := make(chan error, runCount)
	for index := range runCount {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()

			input := fmt.Sprintf("问题-%d", index)
			output, state, err := graph.Run(context.Background(), input)
			if err != nil {
				errors <- fmt.Errorf("graph.Run(%q): %w", input, err)
				return
			}
			wantState := "已校验：" + input
			wantOutput := "回答草稿；回答基于：" + wantState
			if state.question != wantState || output != wantOutput {
				errors <- fmt.Errorf("运行 %q 状态串扰: state=%q output=%q", input, state.question, output)
			}
		}()
	}
	waitGroup.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

func newTestGraph() *sequentialGraph {
	return newSequentialGraph(
		newNode("validate_question", validateQuestion, withStatePostHandler(saveQuestion)),
		newNode("answer_question", draftAnswer, withStatePostHandler(answerFromState)),
	)
}
