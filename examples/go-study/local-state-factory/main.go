package main

import (
	"context"
	"fmt"
)

type queryState struct {
	question string
}

// stateFactory 是一个“函数类型”，不是状态对象。
//
// 任何满足以下签名的函数都可以赋值给 stateFactory：
//
//	func(context.Context) *queryState
//
// 调用这个函数时，它会返回一个新的查询状态。把创建动作包装成函数后，
// 调用方就能决定何时创建状态，并保证每次运行都拿到独立对象。
type stateFactory func(context.Context) *queryState

// runWithSharedState 代表“跨运行共享状态”，不代表 Eino Local State。
// 它接收一个已经创建好的状态对象。
//
// 如果调用者多次传入同一个指针，每次运行修改的就是同一块数据。
// 后一次写入会覆盖前一次写入，这正是共享运行状态的问题。
func runWithSharedState(state *queryState, question string) *queryState {
	state.question = question
	return state
}

// runWithStateFactory 代表 Eino WithGenLocalState 的核心行为：
// 每次 Graph 运行时，通过工厂函数生成该次运行专属的 Local State。
//
// 它接收的是创建状态的函数，而不是状态对象。每次调用都先执行
// factory(ctx)，因此每次运行拥有独立的 queryState。
func runWithStateFactory(ctx context.Context, factory stateFactory, question string) *queryState {
	// 这里有括号，表示现在调用 factory，并取得它返回的新状态。
	state := factory(ctx)
	state.question = question
	return state
}

// newQueryState 满足 stateFactory 的函数签名。
// 当前示例不需要从 context 读取信息，所以省略了参数名。
func newQueryState(context.Context) *queryState {
	return &queryState{}
}

func main() {
	// 错误示范：状态只创建一次，随后把同一个指针传给两次运行。
	shared := &queryState{}
	sharedFirst := runWithSharedState(shared, "什么是 RAG？")
	sharedSecond := runWithSharedState(shared, "什么是 Embedding？")

	// 推荐示范：这里传入的是函数值 newQueryState，而不是函数调用结果。
	//
	// newQueryState    表示“把这个函数交给 runWithStateFactory”。
	// newQueryState(ctx) 表示“现在调用函数，把返回的 *queryState 交出去”。
	//
	// runWithStateFactory 会在内部调用一次 newQueryState，所以两次运行
	// 分别得到两个状态对象。
	factoryFirst := runWithStateFactory(context.Background(), newQueryState, "什么是 RAG？")
	factorySecond := runWithStateFactory(context.Background(), newQueryState, "什么是 Embedding？")

	fmt.Println("共享状态对象（不是 Local State）：")
	// 指针可以直接比较。true 说明两个变量指向同一个 queryState。
	fmt.Printf("  是否同一个对象：%t\n", sharedFirst == sharedSecond)
	// sharedFirst 和 sharedSecond 指向同一对象，所以它们都读到最后写入的问题。
	fmt.Printf("  第一次运行现在保存的问题：%q\n", sharedFirst.question)
	fmt.Printf("  第二次运行保存的问题：%q\n", sharedSecond.question)

	fmt.Println("状态工厂函数（对应 Local State）：")
	// false 说明状态工厂为两次运行创建了两个不同对象。
	fmt.Printf("  是否同一个对象：%t\n", factoryFirst == factorySecond)
	fmt.Printf("  第一次运行保存的问题：%q\n", factoryFirst.question)
	fmt.Printf("  第二次运行保存的问题：%q\n", factorySecond.question)
}
