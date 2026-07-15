package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/cloudwego/eino/callbacks"
)

type CallbackRecord struct {
	Component string
	Name      string
	Type      string
	Status    string
	Duration  time.Duration
	ErrorKind string
}

type Observer struct {
	logger *slog.Logger
	now    func() time.Time

	mu      sync.Mutex
	records []CallbackRecord
}

type callbackStartKey struct{}

func NewObserver(logger *slog.Logger) *Observer {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Observer{logger: logger, now: time.Now}
}

func (o *Observer) Handler() callbacks.Handler {
	return callbacks.NewHandlerBuilder().
		OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, _ callbacks.CallbackInput) context.Context {
			o.addRecord(info, "started", 0, "")
			return context.WithValue(ctx, callbackStartKey{}, o.now())
		}).
		OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, _ callbacks.CallbackOutput) context.Context {
			o.addRecord(info, "succeeded", o.duration(ctx), "")
			return ctx
		}).
		OnErrorFn(func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			o.addRecord(info, "failed", o.duration(ctx), ErrorKind(err))
			return ctx
		}).
		Build()
}

func (o *Observer) Records() []CallbackRecord {
	o.mu.Lock()
	defer o.mu.Unlock()
	records := make([]CallbackRecord, len(o.records))
	copy(records, o.records)
	return records
}

func (o *Observer) duration(ctx context.Context) time.Duration {
	startedAt, ok := ctx.Value(callbackStartKey{}).(time.Time)
	if !ok {
		return 0
	}
	return o.now().Sub(startedAt)
}

func (o *Observer) addRecord(info *callbacks.RunInfo, status string, duration time.Duration, errorKind string) {
	record := CallbackRecord{Status: status, Duration: duration, ErrorKind: errorKind}
	if info != nil {
		record.Component = string(info.Component)
		record.Name = info.Name
		record.Type = info.Type
	}

	o.mu.Lock()
	o.records = append(o.records, record)
	o.mu.Unlock()

	attrs := []any{
		"component", record.Component,
		"name", record.Name,
		"type", record.Type,
		"status", record.Status,
		"duration", record.Duration,
	}
	if record.ErrorKind != "" {
		attrs = append(attrs, "error_kind", record.ErrorKind)
	}
	o.logger.Info("eino component", attrs...)
}

func ErrorKind(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, ErrUnsupportedCity):
		return "unsupported_city"
	case errors.Is(err, ErrWeatherUnavailable):
		return "weather_unavailable"
	default:
		return "internal"
	}
}
