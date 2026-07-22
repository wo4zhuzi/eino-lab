package main

import (
	"context"
	"errors"
	"sync"

	"github.com/cloudwego/eino/callbacks"
)

// CallbackRecord contains metadata only; it never stores source text or prompts.
type CallbackRecord struct {
	Component string
	Name      string
	Status    string
	ErrorKind string
}

// Observer collects thread-safe component and node callback records.
type Observer struct {
	mu      sync.Mutex
	records []CallbackRecord
}

// NewObserver creates an empty callback observer.
func NewObserver() *Observer {
	return &Observer{}
}

// Handler returns an Eino callback handler.
func (o *Observer) Handler() callbacks.Handler {
	return callbacks.NewHandlerBuilder().
		OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, _ callbacks.CallbackInput) context.Context {
			o.add(info, "started", "")
			return ctx
		}).
		OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, _ callbacks.CallbackOutput) context.Context {
			o.add(info, "succeeded", "")
			return ctx
		}).
		OnErrorFn(func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			o.add(info, "failed", errorKind(err))
			return ctx
		}).
		Build()
}

// Records returns a snapshot of collected records.
func (o *Observer) Records() []CallbackRecord {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]CallbackRecord(nil), o.records...)
}

func (o *Observer) add(info *callbacks.RunInfo, status, kind string) {
	record := CallbackRecord{Status: status, ErrorKind: kind}
	if info != nil {
		record.Component = string(info.Component)
		record.Name = info.Name
	}
	o.mu.Lock()
	o.records = append(o.records, record)
	o.mu.Unlock()
}

func errorKind(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, ErrEmptyQuestion):
		return "empty_question"
	case errors.Is(err, ErrNoChunks):
		return "no_chunks"
	case errors.Is(err, ErrEmbeddingCountMismatch), errors.Is(err, ErrEmbeddingDimension), errors.Is(err, ErrZeroEmbedding):
		return "embedding_contract"
	case errors.Is(err, ErrDependencyUnavailable):
		return "dependency_unavailable"
	default:
		return "internal"
	}
}
