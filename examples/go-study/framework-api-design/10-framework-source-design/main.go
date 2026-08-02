package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/cloudwego/eino/compose"
)

var (
	// ErrEmptyQuestion 是 generate 节点主动返回的业务校验错误。
	// 该错误出现时节点 action 执行失败，Eino 会跳过该节点的 StatePostHandler。
	ErrEmptyQuestion = errors.New("问题不能为空")
	// ErrNilGenerator 表示构图时没有提供节点的核心执行逻辑。
	ErrNilGenerator = errors.New("generator 不能为空")
	// ErrNilPostHandle 表示构图时没有提供需要追踪的节点后置处理器。
	ErrNilPostHandle = errors.New("post handler 不能为空")
)

const (
	// 节点 key 同时用于声明 Edge、定位运行错误以及生成节点路径。
	nodeGenerate = "generate"
	nodeResult   = "build_result"
)

// traceState 是一次 Runnable.Invoke 独享的 Local State。
//
// ID 用来证明状态生成器会为每次运行创建新对象；Events 用来保存同一次
// Graph 运行内、跨节点共享的信息。它不是数据库状态，Invoke 结束后不应再依赖它。
type traceState struct {
	ID     uint64
	Events []string
}

// ReviewRequest 是 Graph 对外接收的业务请求。
// 使用结构体而不是裸 string，可以直接看出 Question 是请求字段；以后增加用户 ID、
// 会话 ID 等信息时，也不需要修改 Graph 的整体输入类型。
type ReviewRequest struct {
	Question string
}

// ReviewResult 是 Graph 对外返回的业务结果，同时包含本次运行的状态快照，便于观察：
//  1. PostHandler 是否改写了节点输出；
//  2. build_result 节点是否读到了同一次运行的 Local State；
//  3. 多次 Invoke 是否使用了不同状态。
type ReviewResult struct {
	StateID uint64
	Answer  string
	Events  []string
}

// generator 描述 generate 节点的核心业务函数。
// 它把 Graph 的 ReviewRequest 输入转换成节点间传递的 string 回答。
// 它与 Eino InvokeWOOpt[ReviewRequest, string] 的底层签名一致，但这是一个独立的命名类型，
// 注册 Lambda 时需要显式转换，才能满足 InvokableLambda 的泛型参数要求。
type generator func(context.Context, ReviewRequest) (string, error)

// postHandler 描述 generate 节点成功后的状态处理函数。
// 返回的 string 会替换 generate 的原始输出并传给下游，因此它既可以只记录状态，
// 也可以同时改写数据流；返回 error 则会终止 Graph，并保留原始错误链。
type postHandler func(context.Context, string, *traceState) (string, error)

// traceGraph 保存编译后的 Runnable。
//
// stateCreations 和 postCalls 不是业务状态，只是本课的观测探针。Runnable 可以被
// 多个 goroutine 并发 Invoke，所以计数器使用 atomic，避免示例自身引入数据竞争。
type traceGraph struct {
	runnable       compose.Runnable[ReviewRequest, ReviewResult]
	stateCreations atomic.Uint64
	postCalls      atomic.Uint64
}

// newTraceGraph 完成以下构建链路：
//
//	注册 Local State 工厂
//	  -> 注册 generate 节点与 StatePostHandler
//	  -> 注册 build_result 节点
//	  -> 用 Edge 声明运行顺序
//	  -> Compile 得到可复用 Runnable
//
// 注意：该函数只完成配置、校验和编译。正常情况下，状态工厂、generator 和
// postHandler 都不会在 newTraceGraph 返回前执行。
func newTraceGraph(generate generator, post postHandler) (*traceGraph, error) {
	// 提前拒绝 nil，避免把无效函数注册到 Graph 后才在运行期 panic。
	if generate == nil {
		return nil, ErrNilGenerator
	}
	if post == nil {
		return nil, ErrNilPostHandle
	}

	// 先创建 result，再让下面两个闭包捕获同一指针。这样计数器属于当前编译产物，
	// 不需要使用跨 Graph 共享的全局变量。
	result := &traceGraph{}

	// WithGenLocalState 保存的是“状态生成函数”，此处不会立刻调用它。
	// Compile 会把它转换为 runner.runCtx；每次新的 Runnable.Invoke 开始时，
	// runner.runCtx 才调用该函数，并把返回的 *traceState 放入本次运行的 context。
	//
	// Graph 对外契约：Invoke(ReviewRequest) -> ReviewResult
	// 完整类型流：
	//
	//	START(ReviewRequest)
	//	  -> generate: ReviewRequest -> string
	//	  -> StatePostHandler: string -> string
	//	  -> build_result: string -> ReviewResult
	//	  -> END(ReviewResult)
	graph := compose.NewGraph[ReviewRequest, ReviewResult](
		compose.WithGenLocalState(func(context.Context) *traceState {
			return &traceState{ID: result.stateCreations.Add(1)}
		}),
	)

	if err := graph.AddLambdaNode(
		nodeGenerate,
		// generator 是自定义命名类型，因此显式转换为 Eino 的公开函数类型。
		// InvokableLambda 只负责把普通函数适配成 Compose 节点，此时不会执行函数。
		compose.InvokableLambda(compose.InvokeWOOpt[ReviewRequest, string](generate)),
		compose.WithNodeName(nodeGenerate),
		// WithStatePostHandler 返回 GraphAddNodeOpt。AddLambdaNode 应用该 Option 时，
		// Eino 会包装 Handler，并立即校验 Graph 是否启用 Local State、状态类型是否
		// 匹配以及 Handler 输入是否等于节点输出；真正的 post 调用仍发生在运行期。
		compose.WithStatePostHandler(func(
			ctx context.Context,
			output string,
			state *traceState,
		) (string, error) {
			// 只有 generate action 成功，Eino 的 taskManager.waitOne 才会执行到这里。
			// 节点 action 失败时，postCalls 不会增加。
			result.postCalls.Add(1)
			return post(ctx, output, state)
		}),
	); err != nil {
		return nil, fmt.Errorf("添加 %s 节点: %w", nodeGenerate, err)
	}

	// build_result 是 generate 的下游节点。它收到的 output 已经是 PostHandler
	// 返回的新值，因此可以验证 PostHandler 不仅能观察结果，也能改写后续数据流。
	buildResult := compose.InvokableLambda(func(ctx context.Context, output string) (ReviewResult, error) {
		var reviewed ReviewResult

		// ProcessState 从运行 context 中按类型查找 *traceState，并在回调执行期间持有
		// 对应 internalState 的互斥锁。业务代码不需要直接管理 Eino 内部的锁。
		if err := compose.ProcessState[*traceState](ctx, func(_ context.Context, state *traceState) error {
			reviewed = ReviewResult{
				StateID: state.ID,
				Answer:  output,
				// 在锁内复制切片，避免把 Local State 的底层数组暴露到回调和 Invoke 生命周期外。
				Events: append([]string(nil), state.Events...),
			}
			return nil
		}); err != nil {
			return ReviewResult{}, fmt.Errorf("读取运行状态: %w", err)
		}
		return reviewed, nil
	})
	if err := graph.AddLambdaNode(nodeResult, buildResult, compose.WithNodeName(nodeResult)); err != nil {
		return nil, fmt.Errorf("添加 %s 节点: %w", nodeResult, err)
	}

	// AddLambdaNode 的注册顺序不等于运行顺序；只有 Edge 才定义数据和控制流。
	// generate 的 PostHandler 完成后，改写后的输出才会沿 Edge 进入 build_result。
	for _, edge := range [][2]string{
		{compose.START, nodeGenerate},
		{nodeGenerate, nodeResult},
		{nodeResult, compose.END},
	} {
		if err := graph.AddEdge(edge[0], edge[1]); err != nil {
			return nil, fmt.Errorf("添加边 %s -> %s: %w", edge[0], edge[1], err)
		}
	}

	// Compile 把节点及其 postProcessor 编译进运行任务。编译后的 Runnable 应在服务
	// 启动阶段创建一次、请求阶段重复使用；Compile 本身不会创建本次运行的 Local State。
	runnable, err := graph.Compile(context.Background(), compose.WithGraphName("state_post_handler_trace"))
	if err != nil {
		return nil, fmt.Errorf("编译源码追踪 Graph: %w", err)
	}
	result.runnable = runnable
	return result, nil
}

func newDefaultTraceGraph() (*traceGraph, error) {
	return newTraceGraph(
		// generate 是节点 action：先规范化输入，再生成原始业务输出。
		// 若返回 ErrEmptyQuestion，Eino 会直接传播节点错误并跳过下面的 PostHandler。
		func(_ context.Context, request ReviewRequest) (string, error) {
			question := strings.TrimSpace(request.Question)
			if question == "" {
				return "", ErrEmptyQuestion
			}
			return "回答：" + question, nil
		},
		// post 在 generate 成功后执行。这里同时演示两种能力：
		//  1. 把原始输出记录到本次运行的 Local State；
		//  2. 返回新输出，交给下游 build_result。
		func(_ context.Context, output string, state *traceState) (string, error) {
			state.Events = append(state.Events, "post:"+output)
			return output + " [已记录]", nil
		},
	)
}

func (g *traceGraph) Invoke(ctx context.Context, request ReviewRequest) (ReviewResult, error) {
	// Invoke 会创建本次 Local State，并将派生后的 context 传给节点与 Handler。
	// 任一节点或 PostHandler 返回错误时，Eino 会附加节点路径，同时保留 Unwrap 错误链。
	return g.runnable.Invoke(ctx, request)
}

func (g *traceGraph) Counts() (stateCreations uint64, postCalls uint64) {
	// 原子读取允许测试在并发 Invoke 完成后安全检查总调用次数。
	return g.stateCreations.Load(), g.postCalls.Load()
}

func main() {
	graph, err := newDefaultTraceGraph()
	if err != nil {
		panic(err)
	}

	// 构建和 Compile 完成后两个计数仍应为 0，证明配置阶段没有执行状态工厂和 Handler。
	stateCreations, postCalls := graph.Counts()
	fmt.Printf("构建后：state=%d post=%d\n", stateCreations, postCalls)

	// 同一个 Runnable 连续运行两次。输出中的 StateID 应分别为 1 和 2，证明状态按
	// Invoke 创建；每次 Events 都只有当前问题的记录，证明运行之间没有状态串扰。
	for _, request := range []ReviewRequest{
		{Question: "配置保存在哪里？"},
		{Question: "Handler 在哪里调用？"},
	} {
		result, err := graph.Invoke(context.Background(), request)
		if err != nil {
			panic(err)
		}
		fmt.Printf("运行：state=%d answer=%q events=%q\n", result.StateID, result.Answer, result.Events)
	}
}
