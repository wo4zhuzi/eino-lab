package workflowkit

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/cloudwego/eino/compose"
)

type stringWorkflowDefinition struct {
	compileCalls atomic.Int64
	handlerCalls atomic.Int64
	compileErr   error
}

func (d *stringWorkflowDefinition) Compile(
	ctx context.Context,
	opts ...compose.GraphCompileOption,
) (compose.Runnable[string, string], error) {
	d.compileCalls.Add(1)
	if d.compileErr != nil {
		return nil, d.compileErr
	}
	chain := compose.NewChain[string, string]()
	chain.AppendLambda(compose.InvokableLambda(func(_ context.Context, input string) (string, error) {
		d.handlerCalls.Add(1)
		return strings.ToUpper(input), nil
	}))
	return chain.Compile(ctx, opts...)
}

func TestCompileOnceAndRunMany(t *testing.T) {
	definition := &stringWorkflowDefinition{}
	runner, err := Compile(context.Background(), "uppercase", definition)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	for _, input := range []string{"first", "second"} {
		output, err := runner.Run(context.Background(), input)
		if err != nil {
			t.Fatalf("Run(%q) error = %v", input, err)
		}
		if output != strings.ToUpper(input) {
			t.Fatalf("Run(%q) = %q", input, output)
		}
	}
	if definition.compileCalls.Load() != 1 || definition.handlerCalls.Load() != 2 {
		t.Fatalf(
			"compileCalls=%d handlerCalls=%d, want 1 and 2",
			definition.compileCalls.Load(),
			definition.handlerCalls.Load(),
		)
	}
	if runner.Name() != "uppercase" {
		t.Fatalf("Name() = %q", runner.Name())
	}
}

func TestCompileAndRunPreserveErrors(t *testing.T) {
	errCompile := errors.New("compile failed")
	_, err := Compile(context.Background(), "broken", &stringWorkflowDefinition{compileErr: errCompile})
	if !errors.Is(err, errCompile) {
		t.Fatalf("Compile() error = %v, want errCompile", err)
	}

	tests := []struct {
		name       string
		ctx        context.Context
		workflow   string
		definition Compilable[string, string]
		want       error
	}{
		{name: "nil context", workflow: "workflow", definition: &stringWorkflowDefinition{}, want: ErrNilContext},
		{name: "empty name", ctx: context.Background(), definition: &stringWorkflowDefinition{}, want: ErrInvalidName},
		{name: "nil definition", ctx: context.Background(), workflow: "workflow", want: ErrNilDefinition},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Compile(test.ctx, test.workflow, test.definition)
			if !errors.Is(err, test.want) {
				t.Fatalf("Compile() error = %v, want %v", err, test.want)
			}
		})
	}
	if _, err := Compile(
		context.Background(),
		"nil_option",
		&stringWorkflowDefinition{},
		compose.GraphCompileOption(nil),
	); !errors.Is(err, ErrNilCompileOption) {
		t.Fatalf("Compile(nil option) error = %v", err)
	}

	var nilRunner *Runner[string, string]
	if _, err := nilRunner.Run(context.Background(), "input"); !errors.Is(err, ErrRunnerNotInitialized) {
		t.Fatalf("nil Runner.Run() error = %v", err)
	}
}

func TestRunnerCanRunConcurrently(t *testing.T) {
	definition := &stringWorkflowDefinition{}
	runner, err := Compile(context.Background(), "concurrent", definition)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	const runs = 24
	errCh := make(chan error, runs)
	var wg sync.WaitGroup
	for index := 0; index < runs; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			output, err := runner.Run(context.Background(), "input")
			if err != nil {
				errCh <- err
				return
			}
			if output != "INPUT" {
				errCh <- errors.New("unexpected output")
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
	if definition.compileCalls.Load() != 1 || definition.handlerCalls.Load() != runs {
		t.Fatalf(
			"compileCalls=%d handlerCalls=%d, want 1 and %d",
			definition.compileCalls.Load(),
			definition.handlerCalls.Load(),
			runs,
		)
	}
}

func TestRequireDependencyRejectsTypedNil(t *testing.T) {
	var definition *stringWorkflowDefinition
	err := RequireDependency("definition", definition)
	if !errors.Is(err, ErrNilDependency) || !strings.Contains(err.Error(), "definition") {
		t.Fatalf("RequireDependency() error = %v", err)
	}
	if err := RequireDependency("definition", &stringWorkflowDefinition{}); err != nil {
		t.Fatalf("RequireDependency() error = %v", err)
	}
}
