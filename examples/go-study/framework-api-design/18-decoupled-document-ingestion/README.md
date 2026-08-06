# 18. 解耦文档摄取：复用独立 Loader 与 Parser 组件

前置示例见 [17. RAG 文档索引工作流](../17-rag-indexing-workflow/README.md)。Demo 18 保留索引工作流入口和后续拓扑，将数据源加载、校验、格式路由和解析迁移到独立模块
[`eino-document-ingestion`](https://github.com/wo4zhuzi/eino-document-ingestion)。

## 结论

Demo 17 的 `inspect_source` 和 `parse_document` 同时包含了通用文档摄取能力与索引业务语义，导致其他项目无法复用 Loader、Parser 和格式校验。Demo 18 将边界调整为：

```text
eino-document-ingestion
  负责：本地文件或 HTTP 数据源、大小/MIME/签名校验、SHA-256、Loader、Parser
  输出：SourceInfo + ParserInfo + []*schema.Document

rag_document_indexing@v2
  负责：RunID、DocumentID、VersionID、解析单元 ID、后续索引阶段编排
  依赖：Ingestor 最小接口，由应用启动层显式注入
```

工作流不再包含格式注册表、PDF/DOCX/XLSX Parser 实现或文件签名检查，也不负责创建外部资源。应用启动层创建真实摄取组件并通过 `Dependencies.Ingestor` 注入；测试或其他应用可以注入满足同一最小接口的实现。

## 学习目标

1. 用最小接口隔离工作流与外部摄取实现。
2. 区分通用摄取元数据和 RAG 索引元数据的所有权。
3. 保持第三方错误链，使调用方仍可通过 `errors.Is` 判断摄取错误。
4. 复制摄取结果后再补充索引元数据，避免修改依赖方返回的 Document。
5. 在拆分实现后保留完整、可观察的 Eino Chain。

## 依赖组装

生产依赖由应用启动层显式创建，工作流构造函数只校验并保存依赖：

```go
ingestor, err := ingestion.New(ctx, ingestion.Config{
    MaxFileBytes: ingestion.DefaultMaxFileBytes,
})
if err != nil {
    return fmt.Errorf("创建文档摄取器: %w", err)
}

workflow, err := indexworkflow.New(ctx, indexworkflow.Dependencies{
    Ingestor: ingestor,
})
if err != nil {
    return fmt.Errorf("创建索引工作流: %w", err)
}
```

这种组装方式让应用层统一管理摄取器的配置和生命周期，工作流层只依赖 `Ingest` 方法。缺少依赖时会在启动阶段返回 `ErrInvalidDependencies`。

## 版本与前置条件

仓库使用 Go `1.26.x`、Eino `v0.9.12`，并锁定：

```text
github.com/wo4zhuzi/eino-document-ingestion @ f0ac8222e281
```

运行默认示例不需要模型、数据库、API Key 或 Temporal Server。摄取模块支持 `.md`、`.txt`、`.pdf`、`.docx`、`.xlsx` 以及受控的 HTTP/HTTPS URL；各格式 Parser 和远程访问安全限制以该模块当前版本文档为准。

## 工作流拓扑

```text
ingest_document   真实：调用独立摄取组件并补充索引元数据
chunk_document    模拟：不生成真实 Chunk
embed_chunks      模拟：不调用 Embedding 模型
persist_index     模拟：不连接 PostgreSQL
validate_index    模拟：不伪造校验通过
publish_index     模拟：不发布索引版本
build_result      真实：输出阶段状态和解析摘要
```

`DocumentID` 根据标准化后的源 URI 生成，同一 URI 更新内容时保持不变；`VersionID` 使用摄取组件计算的文件 SHA-256。每个解析单元的 ID 由版本、顺序和内容共同生成。

## 运行

在仓库根目录使用自带 Markdown：

```bash
go run ./examples/go-study/framework-api-design/18-decoupled-document-ingestion
```

解析指定本地文件或公网 URL：

```bash
go run ./examples/go-study/framework-api-design/18-decoupled-document-ingestion -- /absolute/path/to/document.pdf
go run ./examples/go-study/framework-api-design/18-decoupled-document-ingestion -- https://example.com/document.pdf
```

预期输出为 JSON，关键字段如下：

```text
workflow=rag_document_indexing@v2
status=ingested_with_simulated_downstream
ingest_document=completed
chunk_document=simulated
embed_chunks=simulated
persist_index=simulated
validate_index=simulated
publish_index=simulated
build_result=completed
```

远程 URL 默认拒绝私网、环回、链路本地和保留地址。需要鉴权、代理、私网或自定义 HTTP Client 时，应在启动层配置 `ingestion.Ingestor`，再通过 `Dependencies.Ingestor` 注入工作流；不要把令牌写入 URL、源码或日志。

## 使用 Eino Dev 查看工作流

```bash
EINO_DEV=true go run ./examples/go-study/framework-api-design/18-decoupled-document-ingestion
```

在 GoLand Eino Dev 中连接 `127.0.0.1:52538` 并选择 `rag_document_indexing@v2`。从 START 节点执行时可使用：

```json
{
  "run_id": "demo18-eino-dev-run",
  "source_uri": "examples/go-study/framework-api-design/18-decoupled-document-ingestion/testdata/knowledge.md"
}
```

Eino Dev 仅用于本地调试，不应暴露到公网或生产环境。

## 验证

```bash
gofmt -w examples/go-study/framework-api-design/18-decoupled-document-ingestion
go test ./examples/go-study/framework-api-design/18-decoupled-document-ingestion/... -count=1
go test -race ./examples/go-study/framework-api-design/18-decoupled-document-ingestion/... -count=1
go vet ./examples/go-study/framework-api-design/18-decoupled-document-ingestion/...
go test ./...
go vet ./...
```

单元测试通过 fake Ingestor 验证依赖边界、错误传播和元数据所有权，并通过真实摄取组件完成 Markdown 集成测试。PDF、DOCX、XLSX 和 HTTP 的格式细节由独立摄取模块自身测试覆盖，Demo 18 不重复测试其内部实现。

## 已知限制

- Chunk、Embedding、持久化、索引校验和版本发布仍为模拟阶段。
- 工作流内只保留解析摘要；完整正文仍会在进程内传给后续节点。
- 默认最大文件为 `32 MiB`。生产值应结合 Parser 内存模型和 Worker 资源设置。
- 远程源的原始 URL 会进入 SourceInfo；URL 可能包含敏感查询参数时，调用方必须在日志和观测链路中脱敏。
- `DocumentID` 当前以完整源 URI 为身份。同一文档使用不同 URL 或查询参数访问时会生成不同 ID；生产系统应按业务资源标识注入稳定 URI 或在后续版本增加身份规范化策略。

## 下一步

后续示例可以只替换 `chunk_document`，基于标准化解析单元实现格式感知 Chunk，而不再改动 Loader 和 Parser。
