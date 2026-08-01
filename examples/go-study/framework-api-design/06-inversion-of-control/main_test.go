package main

import (
	"context"
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
	if output != "回答基于：已校验：问题" {
		t.Fatalf("output = %q, want %q", output, "回答基于：已校验：问题")
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

func newTestGraph() *sequentialGraph {
	return newSequentialGraph(
		newNode("validate_question", validateQuestion, withStatePostHandler(saveQuestion)),
		newNode("answer_question", draftAnswer, withStatePostHandler(answerFromState)),
	)
}
