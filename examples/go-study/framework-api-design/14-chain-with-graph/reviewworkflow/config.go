package reviewworkflow

import "fmt"

// Config 保存影响工作流拓扑或路由的稳定配置。
type Config struct {
	ApprovalScore int
	MaxAttempts   int
	MaxGraphSteps int
}

// DefaultConfig 返回可直接用于本地示例的默认配置。
func DefaultConfig() Config {
	return Config{
		ApprovalScore: 8,
		MaxAttempts:   2,
		MaxGraphSteps: 8,
	}
}

func (c Config) validate() error {
	if c.ApprovalScore < 1 || c.ApprovalScore > 10 {
		return fmt.Errorf("%w: ApprovalScore 必须在 [1, 10] 范围内", ErrInvalidConfig)
	}
	if c.MaxAttempts < 1 {
		return fmt.Errorf("%w: MaxAttempts 必须大于 0", ErrInvalidConfig)
	}
	// 最长正常路径包含 N 次 inspect、N-1 次 revise 和 1 个结束节点。
	// 使用除法比较，避免极端配置下 2*MaxAttempts 发生整数溢出。
	if c.MaxGraphSteps/2 < c.MaxAttempts {
		return fmt.Errorf(
			"%w: MaxGraphSteps 必须至少为 MaxAttempts 的 2 倍",
			ErrInvalidConfig,
		)
	}
	return nil
}
