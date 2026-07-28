package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

func TestSharedStateIsOverwritten(t *testing.T) {
	// 该场景代表跨运行共享状态，是与 Local State 对照的反例。
	// 只创建一个对象，两次运行都接收 shared 指针的副本。
	// 指针值虽然被复制了，但两个指针仍然指向同一块数据。
	shared := &queryState{}
	first := runWithSharedState(shared, "first")
	second := runWithSharedState(shared, "second")

	// 先证明两个返回值确实指向同一个对象。
	if first != second {
		t.Fatal("两次运行应该返回同一个共享状态对象")
	}
	// 再证明第二次运行已经覆盖第一次运行写入的问题。
	if first.question != "second" {
		t.Fatalf("第一次结果未被覆盖：got %q, want %q", first.question, "second")
	}
}

func TestStateFactoryCreatesIndependentState(t *testing.T) {
	// 该场景代表 Local State：每次运行通过工厂获得自己的状态对象。
	// 两次都传入同一个函数值，但该函数每次调用都会执行 &queryState{}。
	first := runWithStateFactory(context.Background(), newQueryState, "first")
	second := runWithStateFactory(context.Background(), newQueryState, "second")

	// 两次运行必须获得不同的状态对象。
	if first == second {
		t.Fatal("状态工厂应该为每次运行创建不同对象")
	}
	// 每个对象仍然保存自己的问题，说明运行之间没有状态串扰。
	if first.question != "first" || second.question != "second" {
		t.Fatalf("状态发生串扰：first=%q, second=%q", first.question, second.question)
	}
}

func TestStateFactoryKeepsConcurrentRunsIsolated(t *testing.T) {
	const runs = 20
	// 使用带缓冲 channel 收集每个 goroutine 返回的独立状态。
	// 缓冲大小等于运行次数，因此发送结果不依赖接收方同时读取。
	results := make(chan *queryState, runs)

	var wait sync.WaitGroup
	for i := 0; i < runs; i++ {
		// 每次循环创建本轮的问题字符串，闭包捕获这一轮的 question。
		question := fmt.Sprintf("question-%d", i)
		wait.Add(1)
		go func() {
			defer wait.Done()
			results <- runWithStateFactory(context.Background(), newQueryState, question)
		}()
	}
	// 必须等所有发送方结束后才能关闭 channel，否则发送方可能向已关闭的
	// channel 写入并触发 panic。
	wait.Wait()
	close(results)

	// 每个问题都应该恰好出现一次。重复或缺失都表示状态发生了串扰。
	seen := make(map[string]bool, runs)
	for state := range results {
		if seen[state.question] {
			t.Fatalf("发现重复或串扰的状态：%q", state.question)
		}
		seen[state.question] = true
	}
	if len(seen) != runs {
		t.Fatalf("独立状态数量 = %d, want %d", len(seen), runs)
	}
}
