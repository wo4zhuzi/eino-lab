package main

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"
)

// reviewLocalState 是单次 Invoke 内共享的运行审计状态。
//
// 它不保存 content、score 等主业务数据。主业务数据仍然通过节点输入输出显式传递；
// Local State 只保存不适合加入每个函数签名的旁路审计信息。
type reviewLocalState struct {
	audit []string
}

// newReviewLocalState 会在每次 Invoke 开始时由 Eino 调用。
// 必须每次返回新对象，不能返回包级共享变量，否则并发请求会互相污染。
func newReviewLocalState(context.Context) *reviewLocalState {
	return &reviewLocalState{}
}

// appendLocalAudit 使用 ProcessState 访问当前 Invoke 的 Local State。
// ProcessState 会查找父 Chain 的状态，并在 handler 执行期间自动加锁。
func appendLocalAudit(ctx context.Context, event string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return compose.ProcessState[*reviewLocalState](ctx, func(_ context.Context, state *reviewLocalState) error {
		state.audit = append(state.audit, event)
		return nil
	})
}

// attachLocalAudit 是最终汇聚节点。
// 两条通知分支都输出 ReviewResult，因此 Chain 会把它们自动连接到这里。
func attachLocalAudit(ctx context.Context, result ReviewResult) (ReviewResult, error) {
	if err := ctx.Err(); err != nil {
		return ReviewResult{}, fmt.Errorf("附加本地审计: %w", err)
	}
	if err := compose.ProcessState[*reviewLocalState](ctx, func(_ context.Context, state *reviewLocalState) error {
		state.audit = append(state.audit, "pipeline_completed")
		// 复制切片，避免把 Local State 内部切片直接暴露给调用方。
		result.Audit = append([]string(nil), state.audit...)
		return nil
	}); err != nil {
		return ReviewResult{}, fmt.Errorf("读取本地审计: %w", err)
	}
	result.Steps = append(result.Steps, nodeAttachLocalAudit)
	return result, nil
}
