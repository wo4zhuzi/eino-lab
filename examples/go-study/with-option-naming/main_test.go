package main

import (
	"context"
	"testing"
	"time"
)

func TestOptionsConfigureNodeAndHandlers(t *testing.T) {
	n := newNode(
		WithName("answer_question"),
		WithTimeout(3*time.Second),
		WithStatePreHandler(rememberOriginalQuestion),
		WithStatePostHandler(rememberFinalAnswer),
	)

	output, state, err := n.Run(context.Background(), "  question  ")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if n.config.name != "answer_question" || n.config.timeout != 3*time.Second {
		t.Fatalf("config = %#v", n.config)
	}
	if state.originalQuestion != "  question  " {
		t.Fatalf("originalQuestion = %q", state.originalQuestion)
	}
	if output != "回答：question" || state.finalAnswer != output {
		t.Fatalf("output=%q, finalAnswer=%q", output, state.finalAnswer)
	}
}
