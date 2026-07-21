package main

import (
	"context"
	"errors"
	"sync"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/compose"
)

type CallbackRecord struct {
	Component string
	Name      string
	Status    string
	ErrorKind string
}

type Observer struct {
	mu      sync.Mutex
	records []CallbackRecord
}

func NewObserver() *Observer {
	return &Observer{}
}

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
			o.add(info, "failed", ErrorKind(err))
			return ctx
		}).
		Build()
}

func (o *Observer) Records() []CallbackRecord {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]CallbackRecord(nil), o.records...)
}

func (o *Observer) add(info *callbacks.RunInfo, status, errorKind string) {
	record := CallbackRecord{Status: status, ErrorKind: errorKind}
	if info != nil {
		record.Component = string(info.Component)
		record.Name = info.Name
	}

	o.mu.Lock()
	o.records = append(o.records, record)
	o.mu.Unlock()
}

func ErrorKind(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, ErrEmptyContent):
		return "empty_content"
	case errors.Is(err, ErrEmptyCustomerQuestion):
		return "empty_customer_question"
	case errors.Is(err, ErrEmptyCustomerReply):
		return "empty_customer_reply"
	case errors.Is(err, ErrInspectorUnavailable):
		return "inspector_unavailable"
	case errors.Is(err, ErrInvalidInspection):
		return "invalid_inspection"
	case errors.Is(err, compose.ErrExceedMaxSteps):
		return "exceeds_max_steps"
	default:
		return "internal"
	}
}
