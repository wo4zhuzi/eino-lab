# Eino RAG L2 故障矩阵

## 验证范围

本矩阵对应 Eino `v0.9.12` 与 `examples/rag-minimal/`。所有测试默认离线，不访问 Redis、模型服务或其他网络依赖。

## 实际结果

| 场景 | 注入位置 | 预期行为 | 实际行为 | 可观测证据 | 结论 |
|---|---|---|---|---|---|
| 正常路径 | 本地 Markdown 到两个 Graph | 索引后召回有序 Chunk，进入 ChatModel 并返回来源 | 索引 5 个 Chunk；召回 3 个，首条分数 `0.4777`；模型调用 1 次 | CLI 输出、`TestAppIndexesAndAnswersWithEvidence` | 已验证 |
| 重复索引 | 同一 Markdown 连续索引两次 | Chunk ID 稳定，内存 Store 不重复增长 | 两次 ID 相同，Store 数量不变 | `TestIndexingTheSameFileKeepsStableChunkIDs` | 已验证 |
| 业务错误 | 查询 Graph 的校验节点输入空白问题 | 返回 `ErrEmptyQuestion`，Retriever 和 ChatModel 均不执行 | `errors.Is` 成功，两者调用计数均为 0 | `TestAskRejectsEmptyQuestionBeforeRetriever` | 已验证 |
| 无证据 | Retriever 返回空集合 | Branch 走固定回答，ChatModel 不执行 | `NoEvidence=true`，模型调用计数为 0 | `TestNoEvidenceBranchSkipsChatModel` | 已验证 |
| 超时 | 查询 Embedding 在第二次调用等待 `ctx.Done()` | Graph 返回包含 `context.DeadlineExceeded` 的错误链 | `errors.Is` 成功，没有生成伪答案 | `TestEmbeddingTimeoutPropagatesThroughQueryGraph` | 已验证 |
| Indexer 不可用 | 索引 Graph 的 Indexer 返回 sentinel error | FileLoader 和 Splitter 完成后快速失败，保留依赖错误 | `errors.Is(ErrDependencyUnavailable)` 成功 | `TestIndexerDependencyErrorIsPreserved` | 已验证 |
| Retriever 不可用 | 查询 Graph 的 Retriever 返回 sentinel error | 快速失败，Branch 和 ChatModel 不执行 | 错误链保留，模型调用计数为 0 | `TestRetrieverDependencyErrorIsPreserved` | 已验证 |
| Embedding 数量错误 | Embedder 返回向量数少于输入文本数 | Indexer 拒绝写入 | 返回 `ErrEmbeddingCountMismatch` | Store 契约子测试 `count` | 已验证 |
| Embedding 维度错误 | Embedder 返回错误维度 | Indexer 拒绝写入 | 返回 `ErrEmbeddingDimension` | Store 契约子测试 `dimension` | 已验证 |
| Embedding 零向量 | Embedder 返回全零向量 | Indexer 拒绝写入，避免无意义相似度 | 返回 `ErrZeroEmbedding` | Store 契约子测试 `zero` | 已验证 |
| 并发查询 | 16 次并发调用同一 Query Runnable | Local State 不串问题，Store 无数据竞争 | 每个结果保留自己的问题；`-race` 无报告 | `TestConcurrentQueriesKeepLocalStateIsolated`、竞态测试 | 已验证 |

## Callback 边界

`Observer` 只记录组件类型、节点名、开始/成功/失败状态和错误分类。它不保存文档正文、问题、Prompt、向量或模型回答。正常路径测试已确认索引侧能观察 Embedding，查询侧能观察 Retriever 和 ChatModel。

## 验证命令

```bash
go test -v ./examples/rag-minimal -count=1
go test -race ./examples/rag-minimal -count=1
go test ./... -count=1
go vet ./...
go run ./examples/rag-minimal
```

## 剩余风险

- Hashing Embedder 不代表真实语义召回质量，尚未使用评测集验证准确率。
- MemoryVectorStore 不持久化，尚未验证进程重启、增量更新、删除和多实例一致性。
- 当前本机 Redis 缺少 RediSearch，不能直接作为后续向量 Store 迁移目标。
- ExtractiveChatModel 是确定性测试组件，尚未验证真实模型的上下文遵循、幻觉和引用一致性。
