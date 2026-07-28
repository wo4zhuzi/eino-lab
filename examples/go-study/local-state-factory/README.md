# 状态工厂函数最小示例

## 学习目标

本示例仅使用 Go 标准库，对比以下两种状态传递方式：

1. 传入同一个 `*queryState` 对象，多次运行会相互覆盖。
2. 传入 `stateFactory` 函数，每次运行创建独立的 `*queryState`。

它用于解释 Eino `WithGenLocalState` 为什么接收状态生成函数，不涉及任何 Eino API。

## 前置条件

- Go `1.26.3`，根模块 directive 为 Go `1.26.0`。
- 不需要第三方依赖、凭据或网络服务。

## 运行

在仓库根目录执行：

```bash
go run ./examples/go-study/local-state-factory
```

预期输出：

```text
传入同一个状态对象：
  是否同一个对象：true
  第一次运行现在保存的问题："什么是 Embedding？"
  第二次运行保存的问题："什么是 Embedding？"
传入状态工厂函数：
  是否同一个对象：false
  第一次运行保存的问题："什么是 RAG？"
  第二次运行保存的问题："什么是 Embedding？"
```

共享对象场景中，第二次运行修改了同一个对象，因此第一次保存的引用读到的也是第二个问题。工厂函数场景中，每次运行先调用 `newQueryState`，两个结果指向不同对象。

## 验证

```bash
go test ./examples/go-study/local-state-factory -count=1
go test -race ./examples/go-study/local-state-factory -count=1
```

测试覆盖共享状态被覆盖、工厂状态相互隔离以及并发运行状态隔离。

## 已知限制

- 示例只说明状态生命周期和工厂函数，不模拟完整 Graph 或节点调度。
- 共享状态测试使用顺序调用来稳定展示覆盖问题；不会故意制造数据竞争。
