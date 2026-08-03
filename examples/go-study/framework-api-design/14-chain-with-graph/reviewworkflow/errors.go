package reviewworkflow

import "errors"

var (
	// ErrNilContext 表示构建或运行工作流时没有可用的 context。
	ErrNilContext = errors.New("context 不能为空")
	// ErrInvalidConfig 表示工作流配置不满足运行约束。
	ErrInvalidConfig = errors.New("工作流配置无效")
	// ErrInvalidDependencies 表示工作流缺少必需的外部依赖。
	ErrInvalidDependencies = errors.New("工作流依赖无效")
	// ErrWorkflowNotInitialized 表示调用了未初始化的 Workflow。
	ErrWorkflowNotInitialized = errors.New("工作流未初始化")
	// ErrEmptyContent 表示规范化后的审核内容为空。
	ErrEmptyContent = errors.New("审核内容不能为空")
	// ErrInvalidScore 表示 Inspector 返回了约定范围外的分数。
	ErrInvalidScore = errors.New("审核分数无效")
	// ErrEmptyRevision 表示 Reviser 没有返回可继续审核的内容。
	ErrEmptyRevision = errors.New("修订内容不能为空")
)
