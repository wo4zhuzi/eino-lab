package reviewworkflow

import (
	"context"
	"fmt"

	"github.com/wo4zhuzi/eino-lab/examples/go-study/framework-api-design/16-governable-workflow-runtime/workflowkit"
)

var descriptor = workflowkit.Descriptor{Name: "content_review", Version: "v1"}

// Descriptor 返回审核工作流的稳定名称和定义版本。
func Descriptor() workflowkit.Descriptor {
	return descriptor
}

// Config 保存审核工作流自己的业务配置。
type Config struct {
	ApprovalScore int
}

// DefaultConfig 返回本地示例配置。
func DefaultConfig() Config {
	return Config{ApprovalScore: 8}
}

// Inspection 是 Inspector 的审核结果。
type Inspection struct {
	Score  int
	Reason string
}

// Inspector 是审核工作流依赖的业务能力。
type Inspector interface {
	Inspect(ctx context.Context, content string) (Inspection, error)
}

// Dependencies 是审核工作流的依赖集合。
type Dependencies struct {
	Inspector Inspector
}

// Workflow 是审核业务门面。
type Workflow struct {
	runner *workflowkit.Runner[Request, Result]
}

// New 在启动阶段校验审核配置和依赖，并编译工作流。
func New(ctx context.Context, config Config, dependencies Dependencies) (*Workflow, error) {
	if config.ApprovalScore < 1 || config.ApprovalScore > 10 {
		return nil, fmt.Errorf("%w: ApprovalScore 必须在 [1, 10] 范围内", ErrInvalidConfig)
	}
	if err := workflowkit.RequireDependency("Inspector", dependencies.Inspector); err != nil {
		return nil, err
	}

	definition, err := build(config, dependencies)
	if err != nil {
		return nil, fmt.Errorf("构建审核拓扑: %w", err)
	}
	runner, err := workflowkit.Compile[Request, Result](ctx, Descriptor(), definition)
	if err != nil {
		return nil, err
	}
	return &Workflow{runner: runner}, nil
}

// Run 执行一次带 RunID 和可选 Observer 的审核请求。
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
