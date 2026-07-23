# Eino RAG v0.9.12 证据表

## 证据范围

本文只记录当前项目锁定的 Eino `v0.9.12`。优先使用本地模块源码、同版本 README、版本匹配的官方示例，以及当前仓库的实际测试和运行结果。

## 核心数据与组件证据

| ID | 结论 | 标签 | 精确证据 | 置信度 | 后续验证 |
|---|---|---|---|---|---|
| R-01 | Eino 官方定位包含 `Retriever` 等可复用组件，Compose 可把组件连接成 Graph/Workflow | 官方说明 | [`README.md`](https://github.com/cloudwego/eino/blob/v0.9.12/README.md#overview) | 高 | 自定义项目验证组件组合 |
| R-02 | `schema.Document` 是 Loader、Transformer、Indexer 与 Retriever 之间的共享数据结构 | 已验证 | [`schema/document.go`](https://github.com/cloudwego/eino/blob/v0.9.12/schema/document.go)、`components/document/doc.go` | 高 | 阶段 4 验证元数据贯穿链路 |
| R-03 | Transformer 应保留已有元数据，来源信息不能在切分时被替换丢失 | 已验证 | [`components/document/interface.go`](https://github.com/cloudwego/eino/blob/v0.9.12/components/document/interface.go) | 高 | 阶段 4 断言 source/chunk 元数据 |
| R-04 | Embedder 批量返回与输入顺序对应、维度一致的 `[][]float64` | 已验证 | [`components/embedding/interface.go`](https://github.com/cloudwego/eino/blob/v0.9.12/components/embedding/interface.go) | 高 | 阶段 4 验证数量与维度错误 |
| R-05 | Indexer 是 RAG 写路径，负责保存文档及可选向量 | 已验证 | [`components/indexer/doc.go`](https://github.com/cloudwego/eino/blob/v0.9.12/components/indexer/doc.go)、`interface.go` | 高 | 阶段 2/4 运行验证 |
| R-06 | Retriever 是 RAG 读路径，按查询返回相关 `Document` | 已验证 | [`components/retriever/doc.go`](https://github.com/cloudwego/eino/blob/v0.9.12/components/retriever/doc.go)、`interface.go` | 高 | 阶段 2/4 运行验证 |
| R-07 | Indexer 与 Retriever 必须使用同一个 Embedding 模型，否则向量空间不一致 | 已验证 | Embedding、Indexer、Retriever 三个包的 `doc.go` 与接口注释 | 高 | 阶段 4 注入维度不一致故障 |
| R-08 | Retriever 结果按相关度降序，分数通过 `Document.MetaData`/`Score()` 暴露 | 已验证 | `components/retriever/doc.go`、[`schema/document.go`](https://github.com/cloudwego/eino/blob/v0.9.12/schema/document.go) | 高 | 阶段 4 断言排序和引用 |
| R-09 | `TopK` 限制结果数量，`ScoreThreshold` 过滤低分结果 | 已验证 | [`components/retriever/option.go`](https://github.com/cloudwego/eino/blob/v0.9.12/components/retriever/option.go) | 高 | 阶段 4 对比实验 |

## Compose 与高级 Flow 证据

| ID | 结论 | 标签 | 精确证据 | 置信度 | 后续验证 |
|---|---|---|---|---|---|
| R-10 | Retriever、Embedding、Indexer 和 Document Transformer 都可注册为 Compose 节点 | 已验证 | [`compose/graph.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/graph.go)、`component_to_graph_node.go` | 高 | 在线查询路径只先使用 Retriever 节点 |
| R-11 | `compose.WithRetrieverOption` 可在调用时传递 Retriever 选项并指定节点 | 已验证 | [`compose/graph_call_options.go`](https://github.com/cloudwego/eino/blob/v0.9.12/compose/graph_call_options.go) | 高 | 阶段 4 传递 TopK 和阈值 |
| R-12 | Parent Indexer 负责切分、生成子 ID 并保存父文档关系 | 已验证 | [`flow/indexer/parent/parent.go`](https://github.com/cloudwego/eino/blob/v0.9.12/flow/indexer/parent/parent.go) | 高 | 第一项目省略，避免提前引入父子检索 |
| R-13 | Parent Retriever 先召回子文档，再按父 ID 获取原文档 | 已验证 | [`flow/retriever/parent/parent.go`](https://github.com/cloudwego/eino/blob/v0.9.12/flow/retriever/parent/parent.go) | 高 | 第一项目省略 |
| R-14 | MultiQuery Retriever 通过 Query Rewrite、并发检索和去重融合提高召回 | 已验证 | [`flow/retriever/multiquery/multi_query.go`](https://github.com/cloudwego/eino/blob/v0.9.12/flow/retriever/multiquery/multi_query.go) | 高 | 基础闭环后再学习 |
| R-15 | Router Retriever 可调用多个 Retriever 并默认使用 RRF 融合结果 | 已验证 | [`flow/retriever/router/router.go`](https://github.com/cloudwego/eino/blob/v0.9.12/flow/retriever/router/router.go) | 高 | 混合检索阶段再学习 |

## 边界结论

| ID | 结论 | 标签 | 依据 | 置信度 |
|---|---|---|---|---|
| R-16 | Eino 核心提供组件契约和编排，不是带资料管理 UI、权限和运维能力的知识库成品 | 已验证 | 核心公开包、README 责任范围；具体实现明确位于 EinoExt | 高 |
| R-17 | 索引写路径不应默认放进每次问答；Indexer 和 Retriever 是互补的独立组件 | 建议 | Indexer/ Retriever 包职责和常见运行生命周期 | 高 |
| R-18 | 第一条学习主路径不需要 Parent、MultiQuery、Router 或 Reranker | 建议 | 它们包装基础 Indexer/Retriever，删除后基础写读闭环仍成立 | 高 |

## 官方示例证据

| ID | 结论 | 标签 | 精确证据 | 置信度 | 后续验证 |
|---|---|---|---|---|---|
| R-19 | 锁定 commit 的 `quickstart/eino_assistant` 子模块精确依赖 Eino `v0.9.12` | 已验证 | [`quickstart/eino_assistant/go.mod`](https://github.com/cloudwego/eino-examples/blob/171220631fb7068ead50b7cd964b8c471647117d/quickstart/eino_assistant/go.mod) | 高 | 阶段 2 构建 |
| R-20 | 官方知识索引 Graph 是 `FileLoader -> MarkdownSplitter -> RedisIndexer` | 官方说明 | [`knowledgeindexing/orchestration.go`](https://github.com/cloudwego/eino-examples/blob/171220631fb7068ead50b7cd964b8c471647117d/quickstart/eino_assistant/eino/knowledgeindexing/orchestration.go) | 高 | 阶段 2 原样运行 |
| R-21 | 官方 Redis Retriever 使用 Ark Embedder、默认 TopK 8，并把距离转换为 `Document.Score()` | 官方说明 | [`einoagent/retriever.go`](https://github.com/cloudwego/eino-examples/blob/171220631fb7068ead50b7cd964b8c471647117d/quickstart/eino_assistant/eino/einoagent/retriever.go) | 高 | 阶段 2 原样运行 |
| R-22 | 官方完整示例要求 Redis 和 Ark Chat/Embedding 凭据，不是默认离线示例 | 官方说明 | [`quickstart/eino_assistant/README.md`](https://github.com/cloudwego/eino-examples/blob/171220631fb7068ead50b7cd964b8c471647117d/quickstart/eino_assistant/README.md) | 高 | 阶段 2 检查当前环境前置条件 |
| R-23 | 官方完整示例在 Go 1.26.3 下原样构建成功 | 已验证 | 临时目录执行 `go build ./...`，退出码 0；依赖缓存位于 `/private/tmp` | 高 | 无需重复构建 |
| R-24 | 官方索引入口在包初始化阶段先连接 Redis，Redis 未启动时在环境变量检查前退出 | 已验证 | 沙箱外首次运行返回 `failed to connect to Redis: dial tcp 127.0.0.1:6379: connect: connection refused`；`knowledgeindexing/indexer.go` 的 `init()` | 高 | 自定义项目避免在 `init()` 建立外部连接 |
| R-25 | 当前环境缺少官方示例所需三项 Ark 配置 | 已验证 | Redis 启动后再次只检查 `ARK_API_KEY`、`ARK_CHAT_MODEL`、`ARK_EMBEDDING_MODEL` 是否设置，结果仍均为 unset，未输出值 | 高 | 官方真实运行保留为未完成 |
| R-26 | 用户启动的 Redis 端口可达，但服务要求认证 | 已验证 | `nc -z 127.0.0.1 6379` 成功；沙箱外原样运行索引入口返回 `NOAUTH Authentication required` | 高 | 当前官方基线不能继续创建向量索引 |
| R-27 | 锁定版本的官方 Redis 初始化不支持从配置传入认证信息 | 已验证 | `pkg/redis/redis.go` 的 `Init()` 固定 `localhost:6379`，`redis.Options` 仅设置 `Addr` 与 `Protocol`；索引和检索组件初始化同样未设置密码 | 高 | 原样运行与临时认证适配分开记录；自定义项目把认证视为 Store 配置责任 |
| R-28 | 仓库根 `.env` 中的 Redis 地址和密码可以通过当前服务认证 | 已验证 | 仅检查变量名存在；临时源码副本读取认证配置后，运行不再返回 `NOAUTH`，未读取或输出密码值 | 高 | 认证不是当前剩余阻塞 |
| R-29 | 当前 Redis 不提供官方示例要求的 RediSearch 命令 | 已验证 | 认证成功后的 `FT.INFO eino:doc:vector_index` 返回 `ERR unknown command 'FT.INFO'` | 高 | 官方示例需要 Redis Stack/RediSearch，或纵向项目使用其他 Store |

## 自定义闭环证据

| ID | 结论 | 标签 | 精确证据 | 置信度 | 后续验证 |
|---|---|---|---|---|---|
| R-30 | 官方示例锁定的 File Loader 与 Markdown Splitter 能和当前 Eino `v0.9.12` 一起构建运行 | 已验证 | 根 `go.mod`；`go test ./... -count=1` 与 `go vet ./...` 通过 | 高 | 无 |
| R-31 | 自定义索引 Graph 能把一个 Markdown 文件加载、按标题切分、补齐来源/标题/Chunk ID 并写入 Indexer | 已验证 | `go run ./examples/rag-minimal` 输出 `documents=1 chunks=5`；`TestAppIndexesAndAnswersWithEvidence`、`TestIndexingTheSameFileKeepsStableChunkIDs` | 高 | 阶段 5 追踪节点与源码入口 |
| R-32 | 查询 Graph 能按分数召回 Document，通过 Branch 控制生成，并返回真实召回记录 | 已验证 | CLI 首条召回标题为“RAG 主路径”、分数 `0.4777`、ChatModel 调用 1 次；正常路径测试通过 | 高 | 阶段 5 追踪 Local State 与错误传播 |
| R-33 | 空问题在 Retriever 前失败；无证据跳过 ChatModel；Indexer/Retriever 不可用保留错误链；Embedding 超时保留 DeadlineExceeded | 已验证 | `go test -v ./examples/rag-minimal -count=1` 对应四类测试全部通过；详见 `failure-matrix.md` | 高 | 无 |
| R-34 | 内存 Store 遵守 TopK、ScoreThreshold、向量数量和维度契约，并支持并发查询 | 已验证 | Store 契约测试通过；`go test -race ./examples/rag-minimal -count=1` 通过 | 高 | 阶段 6 用 PostgreSQL + pgvector Store 执行单变量迁移 |
| R-35 | Hashing Embedder 的实际召回仅能证明确定性局部特征匹配，不能证明生产语义质量 | 建议 | 实现是字符 1-3 gram feature hashing，无训练语义模型 | 高 | 真实产品需要独立 Embedding 选型与评测集 |
