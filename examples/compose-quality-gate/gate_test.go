package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/compose"
)

type inspectorFunc func(context.Context, string) (Inspection, error)

func (f inspectorFunc) Inspect(ctx context.Context, content string) (Inspection, error) {
	return f(ctx, content)
}

type sequenceInspector struct {
	mu      sync.Mutex
	results []Inspection
	calls   int
}

func (s *sequenceInspector) Inspect(context.Context, string) (Inspection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.results) == 0 {
		return Inspection{}, errors.New("script has no inspection result")
	}
	index := s.calls
	if index >= len(s.results) {
		index = len(s.results) - 1
	}
	s.calls++
	return s.results[index], nil
}

func (s *sequenceInspector) Calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func TestQualityGateApprovesAfterRemediation(t *testing.T) {
	inspector := &sequenceInspector{results: []Inspection{
		{Score: 4, Reason: "missing refund timing notice"},
		{Score: 8, Reason: "refund timing notice present"},
	}}
	config := DefaultGateConfig()
	config.MaxAttempts = 2
	gate := newTestGate(t, inspector, config)
	observer := NewObserver()

	result, err := gate.Review(
		context.Background(),
		ReviewRequest{Content: "draft"},
		compose.WithCallbacks(observer.Handler()),
	)
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if result.Status != ReviewApproved {
		t.Fatalf("Review() status = %q, want %q", result.Status, ReviewApproved)
	}
	if result.Score != 8 || result.Attempts != 2 {
		t.Fatalf("Review() score = %d, attempts = %d", result.Score, result.Attempts)
	}
	if !strings.Contains(result.Content, requiredRefundNotice) {
		t.Fatalf("Review() content = %q, want refund timing notice", result.Content)
	}
	wantAudit := []AuditEntry{
		{Attempt: 1, Score: 4, Reason: "missing refund timing notice"},
		{Attempt: 2, Score: 8, Reason: "refund timing notice present"},
	}
	if !reflect.DeepEqual(result.Audit, wantAudit) {
		t.Fatalf("Review() audit = %#v, want %#v", result.Audit, wantAudit)
	}
	if !hasCallbackRecord(observer.Records(), nodeInspect, "succeeded", "") {
		t.Fatalf("callback records = %#v, want successful inspect node", observer.Records())
	}
}

func TestQualityGateRoutesToManualReview(t *testing.T) {
	inspector := &sequenceInspector{results: []Inspection{
		{Score: 2, Reason: "major issues"},
		{Score: 5, Reason: "still below threshold"},
	}}
	config := DefaultGateConfig()
	config.MaxAttempts = 2
	gate := newTestGate(t, inspector, config)

	result, err := gate.Review(context.Background(), ReviewRequest{Content: "draft"})
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if result.Status != ReviewManualReview {
		t.Fatalf("Review() status = %q, want %q", result.Status, ReviewManualReview)
	}
	if result.Score != 5 || result.Attempts != 2 || len(result.Audit) != 2 {
		t.Fatalf("Review() result = %#v", result)
	}
}

func TestQualityGateRejectsEmptyContentBeforeInspection(t *testing.T) {
	var calls atomic.Int32
	inspector := inspectorFunc(func(context.Context, string) (Inspection, error) {
		calls.Add(1)
		return Inspection{Score: 10}, nil
	})
	gate := newTestGate(t, inspector, DefaultGateConfig())

	_, err := gate.Review(context.Background(), ReviewRequest{Content: " \n\t "})
	if !errors.Is(err, ErrEmptyContent) {
		t.Fatalf("Review() error = %v, want ErrEmptyContent", err)
	}
	if !strings.Contains(err.Error(), "node path: [validate]") {
		t.Fatalf("Review() error = %v, want validate node path", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("inspector calls = %d, want 0", calls.Load())
	}
}

func TestQualityGateInspectorFailures(t *testing.T) {
	tests := []struct {
		name      string
		inspector Inspector
		context   func(t *testing.T) (context.Context, context.CancelFunc)
		wantError error
		wantKind  string
	}{
		{
			name: "deadline exceeded",
			inspector: inspectorFunc(func(ctx context.Context, _ string) (Inspection, error) {
				<-ctx.Done()
				return Inspection{}, ctx.Err()
			}),
			context: func(t *testing.T) (context.Context, context.CancelFunc) {
				t.Helper()
				return context.WithTimeout(context.Background(), 30*time.Millisecond)
			},
			wantError: context.DeadlineExceeded,
			wantKind:  "deadline_exceeded",
		},
		{
			name: "dependency unavailable",
			inspector: inspectorFunc(func(context.Context, string) (Inspection, error) {
				return Inspection{}, ErrInspectorUnavailable
			}),
			context: func(t *testing.T) (context.Context, context.CancelFunc) {
				t.Helper()
				return context.WithCancel(context.Background())
			},
			wantError: ErrInspectorUnavailable,
			wantKind:  "inspector_unavailable",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gate := newTestGate(t, test.inspector, DefaultGateConfig())
			observer := NewObserver()
			ctx, cancel := test.context(t)
			defer cancel()

			_, err := gate.Review(
				ctx,
				ReviewRequest{Content: "draft"},
				compose.WithCallbacks(observer.Handler()),
			)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("Review() error = %v, want %v", err, test.wantError)
			}
			if !strings.Contains(err.Error(), "node path: [inspect]") {
				t.Fatalf("Review() error = %v, want inspect node path", err)
			}
			if !hasCallbackRecord(observer.Records(), nodeInspect, "failed", test.wantKind) {
				t.Fatalf("callback records = %#v, want inspect failure %q", observer.Records(), test.wantKind)
			}
		})
	}
}

func TestQualityGateRejectsInvalidInspection(t *testing.T) {
	inspector := inspectorFunc(func(context.Context, string) (Inspection, error) {
		return Inspection{Score: 11}, nil
	})
	gate := newTestGate(t, inspector, DefaultGateConfig())

	_, err := gate.Review(context.Background(), ReviewRequest{Content: "draft"})
	if !errors.Is(err, ErrInvalidInspection) {
		t.Fatalf("Review() error = %v, want ErrInvalidInspection", err)
	}
	if !strings.Contains(err.Error(), "node path: [inspect]") {
		t.Fatalf("Review() error = %v, want inspect node path", err)
	}
}

func TestQualityGateStopsAtGraphStepLimit(t *testing.T) {
	inspector := inspectorFunc(func(context.Context, string) (Inspection, error) {
		return Inspection{Score: 0, Reason: "always low"}, nil
	})
	config := DefaultGateConfig()
	config.MaxAttempts = 100
	config.MaxRunSteps = 3
	gate := newTestGate(t, inspector, config)

	_, err := gate.Review(context.Background(), ReviewRequest{Content: "draft"})
	if !errors.Is(err, compose.ErrExceedMaxSteps) {
		t.Fatalf("Review() error = %v, want ErrExceedMaxSteps", err)
	}
}

func TestQualityGateLocalStateIsIsolatedAcrossConcurrentRuns(t *testing.T) {
	gate := newTestGate(t, ruleInspector{}, DefaultGateConfig())

	const runs = 24
	errorsCh := make(chan error, runs)
	var wait sync.WaitGroup
	for i := 0; i < runs; i++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			result, err := gate.Review(context.Background(), ReviewRequest{Content: fmt.Sprintf("draft-%d", index)})
			if err != nil {
				errorsCh <- fmt.Errorf("run %d: %w", index, err)
				return
			}
			if result.Status != ReviewApproved || result.Attempts != 2 || len(result.Audit) != 2 {
				errorsCh <- fmt.Errorf("run %d: unexpected result %#v", index, result)
				return
			}
			if result.Audit[0].Attempt != 1 || result.Audit[1].Attempt != 2 {
				errorsCh <- fmt.Errorf("run %d: unexpected audit %#v", index, result.Audit)
			}
		}(i)
	}
	wait.Wait()
	close(errorsCh)
	for err := range errorsCh {
		t.Error(err)
	}
}

func TestQualityGateInspectorMigrationKeepsGraphTopology(t *testing.T) {
	config := DefaultGateConfig()
	config.MaxAttempts = 2

	before := newTestGate(t, ruleInspector{}, config)
	after := newTestGate(t, inspectorFunc(func(context.Context, string) (Inspection, error) {
		return Inspection{Score: 9, Reason: "replacement inspector approved"}, nil
	}), config)

	if !reflect.DeepEqual(before.Topology(), after.Topology()) {
		t.Fatalf("topology changed after Inspector replacement: before=%#v after=%#v", before.Topology(), after.Topology())
	}
	beforeResult, err := before.Review(context.Background(), ReviewRequest{Content: "draft"})
	if err != nil {
		t.Fatalf("before Review() error = %v", err)
	}
	afterResult, err := after.Review(context.Background(), ReviewRequest{Content: "draft"})
	if err != nil {
		t.Fatalf("after Review() error = %v", err)
	}
	if beforeResult.Attempts != 2 || afterResult.Attempts != 1 {
		t.Fatalf("attempts after Inspector migration = before:%d after:%d, want 2 and 1", beforeResult.Attempts, afterResult.Attempts)
	}
}

func TestNewQualityGateValidatesConfiguration(t *testing.T) {
	inspector := ruleInspector{}
	tests := []struct {
		name      string
		inspector Inspector
		config    GateConfig
	}{
		{name: "nil inspector", inspector: nil, config: DefaultGateConfig()},
		{name: "threshold below range", inspector: inspector, config: GateConfig{ApprovalThreshold: -1, MaxAttempts: 1, MaxRunSteps: 1}},
		{name: "threshold above range", inspector: inspector, config: GateConfig{ApprovalThreshold: 11, MaxAttempts: 1, MaxRunSteps: 1}},
		{name: "zero attempts", inspector: inspector, config: GateConfig{ApprovalThreshold: 7, MaxAttempts: 0, MaxRunSteps: 1}},
		{name: "zero run steps", inspector: inspector, config: GateConfig{ApprovalThreshold: 7, MaxAttempts: 1, MaxRunSteps: 0}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewQualityGate(context.Background(), test.inspector, test.config); err == nil {
				t.Fatal("NewQualityGate() error = nil, want configuration error")
			}
		})
	}
}

func newTestGate(t *testing.T, inspector Inspector, config GateConfig) *QualityGate {
	t.Helper()
	gate, err := NewQualityGate(context.Background(), inspector, config)
	if err != nil {
		t.Fatalf("NewQualityGate() error = %v", err)
	}
	return gate
}

func hasCallbackRecord(records []CallbackRecord, name, status, errorKind string) bool {
	for _, record := range records {
		if record.Name == name && record.Status == status && record.ErrorKind == errorKind {
			return true
		}
	}
	return false
}
