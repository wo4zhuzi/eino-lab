# RAG 最小闭环

## 学习目标

本示例验证 Eino RAG 的两条独立主路径，以及如何通过召回文档、分数、来源和 Callback 区分索引、检索与生成问题。

```text
索引：Markdown -> FileLoader -> MarkdownSplitter -> Metadata -> Indexer
查询：Question -> Retriever -> Branch -> ChatModel -> Answer + Retrieved Chunks
```

涉及的 Eino 组件：`document.Loader`、`document.Transformer`、`embedding.Embedder`、`indexer.Indexer`、`retriever.Retriever`、`BaseChatModel`、Graph、Branch、Local State 和 Callback。

## 前置条件

- Go `1.26.3`，根模块 directive 为 Go `1.26.0`。
- CloudWeGo Eino `v0.9.12`。
- EinoExt File Loader 与 Markdown Header Splitter 使用官方同版本示例锁定的 commit 版本。
- 不需要 Redis、数据库、模型凭据或网络服务。

## 运行

在仓库根目录执行：

```bash
go run ./examples/rag-minimal
```

也可以指定自己的本地 Markdown 和问题：

```bash
go run ./examples/rag-minimal \
  -file ./notes/my-notes.md \
  "我的资料中如何约定 RAG 的索引更新？"
```

程序会输出索引文档数、Chunk 数、召回排名、分数、来源、标题、Chunk ID、模型调用次数和最终回答。正常情况下会看到类似结构：

```text
index_graph=rag_minimal_index documents=1 chunks=4 callback_records=...
query_graph=rag_minimal_query question="..." retrieved=3 no_evidence=false callback_records=...
rank=1 score=... source="..." heading="..." chunk_id=...
chat_model_calls=1
answer=根据检索到的资料：...
```

## 验证

```bash
go test ./examples/rag-minimal -count=1
go test -race ./examples/rag-minimal -count=1
go test ./... -count=1
go vet ./...
```

默认测试完全离线，覆盖正常召回、空问题、无证据、超时、依赖不可用、Embedding 数量/维度错误和并发调用。

## 已知限制

- Hashing Embedder 只验证向量接口、排序和错误边界，不代表生产语义检索质量。
- MemoryVectorStore 随进程退出丢失数据，不处理增量更新、删除或多实例一致性。
- ExtractiveChatModel 是确定性测试模型，只证明 ChatModel 节点和无证据分支行为，不代表真实生成质量。
- 示例只接收单个本地 Markdown 文件，不包含上传、权限、OCR、Reranker 或知识库管理界面。
- Callback 只记录组件、节点、状态和错误分类，不保存完整资料、问题或 Prompt。
