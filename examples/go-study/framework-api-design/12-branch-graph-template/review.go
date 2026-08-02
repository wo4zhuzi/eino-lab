package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"
)

const (
	nodeInputAdapter        = "input_adapter"
	nodeNormalize           = "normalize"
	nodeAppendChannelNotice = "append_channel_notice"
	nodeInspectRefundNotice = "inspect_refund_notice"
	nodeApprove             = "approve"
	nodeArchiveApproved     = "archive_approved_review"
	nodeManualReview        = "manual_review"
	nodeStandardManualQueue = "standard_manual_queue"
	nodePriorityManualQueue = "priority_manual_queue"
)

// ErrEmptyContent 表示规范化后的审核内容为空。
var ErrEmptyContent = errors.New("审核内容不能为空")

// ReviewRequest 是审核 Graph 对外接收的输入类型。
type ReviewRequest struct {
	Content string
}

// ReviewResult 是审核 Graph 对外返回的输出类型。
type ReviewResult struct {
	Approved bool
	Route    string
	Content  string
	Score    int
	Reasons  []string
	Steps    []string
}

// reviewContext 是 Graph 内部节点之间传递的数据。
//
// Graph 的公开边界是 ReviewRequest -> ReviewResult；中间节点统一传递
// reviewContext，避免把只在流程内部使用的 score、reasons 和 steps 暴露给调用方。
type reviewContext struct {
	content string
	score   int
	reasons []string
	steps   []string
}

// NewReviewGraph 是调用方使用的稳定入口。
//
// 这里明确写出 ReviewRequest 和 ReviewResult，所以阅读入口时就能看到整个
// Graph 的输入、输出，不需要先理解抽象泛型 I 和 O。
func NewReviewGraph(ctx context.Context) (compose.Runnable[ReviewRequest, ReviewResult], error) {
	return compileDefinedGraph[ReviewRequest, ReviewResult](
		ctx,
		"branch_review_graph",
		defineReviewGraph,
	)
}

// defineReviewGraph 集中声明审核 Graph 的全部业务拓扑。
//
// 以后新增普通节点、调整 Edge 或增加 Branch，只修改这个业务定义层；
// compileDefinedGraph 和 main 调用入口都保持不动。
func defineReviewGraph(graph *compose.Graph[ReviewRequest, ReviewResult]) error {
	// -------------------- 注册节点 --------------------
	// AddLambdaNode 只是把“节点 key + Handler”登记到 Graph，并不会立刻调用 Handler。
	if err := addReviewNode(graph, nodeInputAdapter, requestToReviewContext); err != nil {
		return err
	}
	if err := addReviewNode(graph, nodeNormalize, normalizeReview); err != nil {
		return err
	}
	if err := addReviewNode(graph, nodeAppendChannelNotice, appendChannelNotice); err != nil {
		return err
	}
	if err := addReviewNode(graph, nodeInspectRefundNotice, inspectRefundNotice); err != nil {
		return err
	}
	if err := addReviewNode(graph, nodeApprove, approveReview); err != nil {
		return err
	}
	// archive_approved_review 只属于通过分支。
	// 它接收 approve 节点生成的 ReviewResult，并继续返回 ReviewResult。
	if err := addReviewNode(graph, nodeArchiveApproved, archiveApprovedReview); err != nil {
		return err
	}
	if err := addReviewNode(graph, nodeManualReview, sendToManualReview); err != nil {
		return err
	}
	if err := addReviewNode(graph, nodeStandardManualQueue, enqueueStandardManualReview); err != nil {
		return err
	}
	if err := addReviewNode(graph, nodePriorityManualQueue, enqueuePriorityManualReview); err != nil {
		return err
	}

	// -------------------- 注册分支前的固定 Edge --------------------
	// 这些 Edge 表示无条件执行的线性部分。运行时会依次执行到 inspect_refund_notice。
	fixedEdges := [][2]string{
		{compose.START, nodeInputAdapter},
		{nodeInputAdapter, nodeNormalize},
		{nodeNormalize, nodeAppendChannelNotice},
		{nodeAppendChannelNotice, nodeInspectRefundNotice},
	}
	for _, edge := range fixedEdges {
		if err := graph.AddEdge(edge[0], edge[1]); err != nil {
			return fmt.Errorf("添加边 %s -> %s: %w", edge[0], edge[1], err)
		}
	}

	// -------------------- 注册 Branch --------------------
	// routeReview 在运行期接收 inspect_refund_notice 的输出，只返回下一个节点的 key。
	// endNodes 是白名单：条件函数只能选择 approve 或 manual_review。
	reviewBranch := compose.NewGraphBranch(
		routeReview,
		map[string]bool{
			nodeApprove:      true,
			nodeManualReview: true,
		},
	)
	if err := graph.AddBranch(nodeInspectRefundNotice, reviewBranch); err != nil {
		return fmt.Errorf("在节点 %s 后添加审核分支: %w", nodeInspectRefundNotice, err)
	}

	// -------------------- 注册第二个 Branch --------------------
	// 第一个 Branch 选择 approve 或 manual_review；只有选择 manual_review 后，
	// 运行流程才会到达这里，并根据 ReviewResult.Score 再选择一个人工队列。
	manualQueueBranch := compose.NewGraphBranch(
		routeManualQueue,
		map[string]bool{
			nodeStandardManualQueue: true,
			nodePriorityManualQueue: true,
		},
	)
	if err := graph.AddBranch(nodeManualReview, manualQueueBranch); err != nil {
		return fmt.Errorf("在节点 %s 后添加人工队列分支: %w", nodeManualReview, err)
	}

	// -------------------- 注册分支后的固定 Edge --------------------
	// 两条 Branch 选中的节点仍需通过固定 Edge 连接后续节点或 END：
	//
	//	approve -> archive_approved_review -> END
	//	manual_review -> standard_manual_queue -> END
	//	              -> priority_manual_queue -> END
	//
	// AddBranch 只负责选择第一个目标节点，不会自动推导目标节点后面的路径，
	// 所以这里仍然要显式连接每一条分支的后续 Edge。
	branchEdges := [][2]string{
		{nodeApprove, nodeArchiveApproved},
		{nodeArchiveApproved, compose.END},
		{nodeStandardManualQueue, compose.END},
		{nodePriorityManualQueue, compose.END},
	}
	for _, edge := range branchEdges {
		if err := graph.AddEdge(edge[0], edge[1]); err != nil {
			return fmt.Errorf("添加边 %s -> %s: %w", edge[0], edge[1], err)
		}
	}
	return nil
}

// addReviewNode 只减少重复的 AddLambdaNode 错误处理，不决定节点顺序或分支逻辑。
// handler 的具体输入输出类型由泛型 I 和 O 保留，Eino 编译时仍会校验相邻类型。
func addReviewNode[I, O any](
	graph *compose.Graph[ReviewRequest, ReviewResult],
	key string,
	handler compose.InvokeWOOpt[I, O],
) error {
	if err := graph.AddLambdaNode(
		key,
		compose.InvokableLambda(handler),
		compose.WithNodeName(key),
	); err != nil {
		return fmt.Errorf("添加节点 %q: %w", key, err)
	}
	return nil
}

func requestToReviewContext(ctx context.Context, request ReviewRequest) (reviewContext, error) {
	if err := ctx.Err(); err != nil {
		return reviewContext{}, fmt.Errorf("转换审核请求: %w", err)
	}
	return reviewContext{content: request.Content}, nil
}

func normalizeReview(ctx context.Context, current reviewContext) (reviewContext, error) {
	if err := ctx.Err(); err != nil {
		return reviewContext{}, fmt.Errorf("规范化审核内容: %w", err)
	}
	current.content = strings.Join(strings.Fields(current.content), " ")
	if current.content == "" {
		return reviewContext{}, ErrEmptyContent
	}
	current.steps = append(current.steps, nodeNormalize)
	return current, nil
}

func appendChannelNotice(ctx context.Context, current reviewContext) (reviewContext, error) {
	if err := ctx.Err(); err != nil {
		return reviewContext{}, fmt.Errorf("追加渠道说明: %w", err)
	}
	current.content += " 请关注原支付渠道。"
	current.steps = append(current.steps, nodeAppendChannelNotice)
	return current, nil
}

func inspectRefundNotice(ctx context.Context, current reviewContext) (reviewContext, error) {
	if err := ctx.Err(); err != nil {
		return reviewContext{}, fmt.Errorf("检查退款说明: %w", err)
	}
	if strings.Contains(current.content, "退款") && strings.Contains(current.content, "到账") {
		current.score = 9
		current.reasons = append(current.reasons, "包含退款到账说明")
	} else if strings.Contains(current.content, "退款") {
		current.score = 5
		current.reasons = append(current.reasons, "缺少退款到账说明")
	} else {
		current.score = 3
		current.reasons = append(current.reasons, "未包含退款说明")
	}
	current.steps = append(current.steps, nodeInspectRefundNotice)
	return current, nil
}

// routeReview 是 Branch 的条件函数，只选择路径，不执行目标节点。
//
// 运行期顺序是：inspectRefundNotice 返回 current -> Eino 调用 routeReview ->
// routeReview 返回节点 key -> Eino 再调度这个 key 对应的 Handler。
func routeReview(ctx context.Context, current reviewContext) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("选择审核分支: %w", err)
	}
	if current.score >= 8 {
		return nodeApprove, nil
	}
	return nodeManualReview, nil
}

func approveReview(ctx context.Context, current reviewContext) (ReviewResult, error) {
	if err := ctx.Err(); err != nil {
		return ReviewResult{}, fmt.Errorf("生成通过结果: %w", err)
	}
	current.steps = append(current.steps, nodeApprove)
	return newReviewResult(current, true, nodeApprove), nil
}

// archiveApprovedReview 是本次新增的普通节点，只连接在 approve 分支后面。
//
// Branch 条件仍然只需要返回 approve；Eino 执行完 approve 后，会根据固定 Edge
// 继续调用本节点。manual_review 没有指向本节点的 Edge，因此不会执行这里。
func archiveApprovedReview(ctx context.Context, result ReviewResult) (ReviewResult, error) {
	if err := ctx.Err(); err != nil {
		return ReviewResult{}, fmt.Errorf("归档通过结果: %w", err)
	}
	result.Steps = append(result.Steps, nodeArchiveApproved)
	return result, nil
}

func sendToManualReview(ctx context.Context, current reviewContext) (ReviewResult, error) {
	if err := ctx.Err(); err != nil {
		return ReviewResult{}, fmt.Errorf("生成人工审核结果: %w", err)
	}
	current.steps = append(current.steps, nodeManualReview)
	return newReviewResult(current, false, nodeManualReview), nil
}

// routeManualQueue 是第二个 Branch 的条件函数。
//
// 第一个 Branch 的起点 inspect_refund_notice 输出 reviewContext，所以 routeReview
// 接收 reviewContext；第二个 Branch 的起点 manual_review 输出 ReviewResult，
// 所以这里接收 ReviewResult。Branch 条件的输入类型由它的起点输出决定。
func routeManualQueue(ctx context.Context, result ReviewResult) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("选择人工队列分支: %w", err)
	}
	if result.Score >= 5 {
		return nodeStandardManualQueue, nil
	}
	return nodePriorityManualQueue, nil
}

func enqueueStandardManualReview(ctx context.Context, result ReviewResult) (ReviewResult, error) {
	if err := ctx.Err(); err != nil {
		return ReviewResult{}, fmt.Errorf("加入普通人工队列: %w", err)
	}
	result.Steps = append(result.Steps, nodeStandardManualQueue)
	return result, nil
}

func enqueuePriorityManualReview(ctx context.Context, result ReviewResult) (ReviewResult, error) {
	if err := ctx.Err(); err != nil {
		return ReviewResult{}, fmt.Errorf("加入优先人工队列: %w", err)
	}
	result.Steps = append(result.Steps, nodePriorityManualQueue)
	return result, nil
}

func newReviewResult(current reviewContext, approved bool, route string) ReviewResult {
	return ReviewResult{
		Approved: approved,
		Route:    route,
		Content:  current.content,
		Score:    current.score,
		Reasons:  append([]string(nil), current.reasons...),
		Steps:    append([]string(nil), current.steps...),
	}
}
