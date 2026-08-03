package reviewworkflow

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"
)

// Workflow 是应用层使用的稳定门面，不向上层暴露 Eino Runnable。
// Workflow 应在进程启动时创建一次，并在请求之间复用。
type Workflow struct {
	runnable compose.Runnable[ReviewRequest, ReviewResult]
}

// New 校验配置和依赖，并在启动阶段编译工作流。
func New(ctx context.Context, config Config, dependencies Dependencies) (*Workflow, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("构建审核工作流: %w", err)
	}
	if err := config.validate(); err != nil {
		return nil, err
	}
	if err := dependencies.validate(); err != nil {
		return nil, err
	}

	runnable, err := buildPipeline(ctx, config, dependencies)
	if err != nil {
		return nil, fmt.Errorf("构建审核工作流: %w", err)
	}
	return &Workflow{runnable: runnable}, nil
}

// Run 执行一次审核。每次调用的输入、输出和 Graph 运行状态相互隔离。
func (w *Workflow) Run(ctx context.Context, request ReviewRequest) (ReviewResult, error) {
	if ctx == nil {
		return ReviewResult{}, ErrNilContext
	}
	if w == nil || w.runnable == nil {
		return ReviewResult{}, ErrWorkflowNotInitialized
	}

	result, err := w.runnable.Invoke(ctx, request)
	if err != nil {
		return ReviewResult{}, fmt.Errorf("运行审核工作流: %w", err)
	}
	return result, nil
}
