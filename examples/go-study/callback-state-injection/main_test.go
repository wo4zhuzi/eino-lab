package main

import (
	"context"
	"testing"
)

func TestRegisteredHandlerReceivesStateFromFramework(t *testing.T) {
	configuredNode := addLambdaNode(withStatePostHandler(saveQuestion))

	output, state, err := runGraph(context.Background(), "问题", configuredNode)
	if err != nil {
		t.Fatalf("runGraph() error = %v", err)
	}
	if output != "已校验：问题" {
		t.Fatalf("output = %q, want %q", output, "已校验：问题")
	}
	if state.question != output {
		t.Fatalf("state.question = %q, want %q", state.question, output)
	}
}

func TestEachRunCreatesIndependentState(t *testing.T) {
	configuredNode := addLambdaNode(withStatePostHandler(saveQuestion))

	_, first, err := runGraph(context.Background(), "第一个问题", configuredNode)
	if err != nil {
		t.Fatalf("first runGraph() error = %v", err)
	}
	_, second, err := runGraph(context.Background(), "第二个问题", configuredNode)
	if err != nil {
		t.Fatalf("second runGraph() error = %v", err)
	}

	if first == second {
		t.Fatal("两次运行不应该共享同一个状态对象")
	}
	if first.question != "已校验：第一个问题" || second.question != "已校验：第二个问题" {
		t.Fatalf("状态发生串扰：first=%q, second=%q", first.question, second.question)
	}
}
