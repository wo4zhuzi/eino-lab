package ragworkflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/wo4zhuzi/eino-lab/examples/go-study/framework-api-design/16-governable-workflow-runtime/workflowkit"
)

var descriptor = workflowkit.Descriptor{Name: "local_rag", Version: "v1"}

// Descriptor 返回 RAG 工作流的稳定名称和定义版本。
func Descriptor() workflowkit.Descriptor {
	return descriptor
}

// Config 保存 RAG 工作流自己的检索策略。
type Config struct {
	MaxRetrievalAttempts int
	MaxGraphSteps        int
	RewriteSuffix        string
}

// DefaultConfig 返回本地示例配置。
func DefaultConfig() Config {
	return Config{
		MaxRetrievalAttempts: 2,
		MaxGraphSteps:        8,
		RewriteSuffix:        "Eino",
	}
}

// Retriever 是 RAG 工作流依赖的检索能力。
type Retriever interface {
	Retrieve(ctx context.Context, query string) ([]string, error)
}

// Generator 是 RAG 工作流依赖的答案生成能力。
type Generator interface {
	Generate(ctx context.Context, question string, evidence []string) (string, error)
}

// Dependencies 是 RAG 工作流的依赖集合。
type Dependencies struct {
	Retriever Retriever
	Generator Generator
}

// Workflow 是 RAG 业务门面。
type Workflow struct {
	runner *workflowkit.Runner[Request, Result]
}

// New 在启动阶段校验 RAG 配置和依赖，并编译工作流。
func New(ctx context.Context, config Config, dependencies Dependencies) (*Workflow, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	if err := workflowkit.RequireDependency("Retriever", dependencies.Retriever); err != nil {
		return nil, err
	}
	if err := workflowkit.RequireDependency("Generator", dependencies.Generator); err != nil {
		return nil, err
	}

	definition, err := build(config, dependencies)
	if err != nil {
		return nil, fmt.Errorf("构建 RAG 拓扑: %w", err)
	}
	runner, err := workflowkit.Compile[Request, Result](ctx, Descriptor(), definition)
	if err != nil {
		return nil, err
	}
	return &Workflow{runner: runner}, nil
}

// Run 执行一次带 RunID 和可选 Observer 的 RAG 请求。
func (w *Workflow) Run(
	ctx context.Context,
	request Request,
	opts ...workflowkit.RunOption,
) (Result, error) {
	if w == nil {
		return Result{}, workflowkit.ErrRunnerNotInitialized
	}
	return w.runner.Run(ctx, request, opts...)
}

func (c Config) validate() error {
	if c.MaxRetrievalAttempts < 1 {
		return fmt.Errorf("%w: MaxRetrievalAttempts 必须大于 0", ErrInvalidConfig)
	}
	if c.MaxGraphSteps/2 < c.MaxRetrievalAttempts {
		return fmt.Errorf(
			"%w: MaxGraphSteps 必须至少为 MaxRetrievalAttempts 的 2 倍",
			ErrInvalidConfig,
		)
	}
	if strings.TrimSpace(c.RewriteSuffix) == "" {
		return fmt.Errorf("%w: RewriteSuffix 不能为空", ErrInvalidConfig)
	}
	return nil
}
