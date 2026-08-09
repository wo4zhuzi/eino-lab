# 19. 全包化文档流水线：从 Loader、Parser 到 Chunk

前置示例见 [18. 解耦文档摄取](../18-decoupled-document-ingestion/README.md)。Demo 19 延续索引工作流，将模拟的 Chunk 节点替换为独立 Package 的真实实现。

## 结论

Demo 18 锁定的旧版摄取契约没有声明 Parser 输出是否具有可信结构，并且工作流会重写所有解析单元 ID。结构化 Parser 的父子引用和结构路径依赖原始 block ID，因此不能把该结果直接交给 Structure-aware Chunk。

Demo 19 使用三个独立模块的公共契约串联完整链路：

```text
eino-document-ingestion
  FileLoader、数据源校验、格式路由、标准 Result
        |
        v
eino-document-parser-structured/markdown
  Markdown AST、结构 block、稳定 ID 与 eino_ingestion.structure.*
        |
        v
eino-document-chunking
  IngestionAdapter、策略选择、Chunk、关系与统计
```

工作流只补充索引元数据。已有解析单元 ID 保持不变；仅为没有 ID 的非结构化单元生成稳定 ID。

## 学习目标

1. 使用 `ParserInfo.Output` 作为 Parser 与 Chunker 的能力契约。
2. 根据 `Structured` 自动选择策略，不在业务层根据扩展名猜测。
3. 保留结构化 Parser 的 block ID、父节点和结构路径。
4. 通过最小 `Ingestor` 与 `Chunker` 接口隔离工作流编排。
5. 保留第三方错误链，使调用方可以通过 `errors.Is` 判断失败原因。

## 版本与前置条件

仓库使用 Go `1.26.x`、Eino `v0.9.12`，并锁定：

```text
github.com/wo4zhuzi/eino-document-ingestion @ 7cc1616a8a0f
github.com/wo4zhuzi/eino-document-parser-structured @ 02602d613c64
github.com/wo4zhuzi/eino-document-chunking @ 70e40cb7a820
```

运行默认示例不需要模型、数据库、API Key 或 Temporal Server。支持本地文件以及受 ingestion 安全策略约束的 HTTP/HTTPS URL。

## Parser 与 Chunk 策略

应用启动时先创建 ingestion 默认 Registry，再使用独立结构化 Markdown Parser 替换默认 Markdown Parser，最后创建 Ingestor。Ingestor 会复制 Registry 快照，因此顺序不能颠倒。

| 输入 | Parser 输出 | Chunk 策略 |
|---|---|---|
| Markdown | `block + structured=true` | Structure-aware |
| TXT | `document + structured=false` | Parent-child |
| PDF | `page + structured=false` | Parent-child |
| DOCX | `section + structured=false` | Parent-child |
| XLSX | `row + structured=false` | Parent-child |

Structure-aware 使用中英混合知识库基线：`MaxRunes=1800`、`MinRunes=600`，结构路径只写入 Metadata。Parent-child 的父 Chunk 上限为 2000 字符，子 Chunk 上限为 500 字符。字符数按 Unicode rune 计算，不等同于模型 Token 数，生产配置应结合目标 Embedding 模型的 Tokenizer 和真实语料抽样调整。

## 工作流拓扑

```text
ingest_document   真实：Package Loader、格式校验和 Parser
chunk_document    真实：IngestionAdapter、自动策略和 Chunk Engine
embed_chunks      模拟：不调用 Embedding 模型
persist_index     模拟：不连接 PostgreSQL
validate_index    模拟：不伪造索引校验通过
publish_index     模拟：不发布索引版本
build_result      真实：输出 Parser、Chunk、关系、统计和阶段状态
```

稳定工作流标识为 `rag_document_indexing@v3`。成功状态为 `chunked_with_simulated_downstream`。

## 运行

在仓库根目录运行自带 Markdown：

```bash
go run ./examples/go-study/framework-api-design/19-packaged-document-pipeline
```

解析指定文件或公网 URL：

```bash
go run ./examples/go-study/framework-api-design/19-packaged-document-pipeline /absolute/path/to/document.txt
go run ./examples/go-study/framework-api-design/19-packaged-document-pipeline /absolute/path/to/document.pdf
go run ./examples/go-study/framework-api-design/19-packaged-document-pipeline https://example.com/document.md
```

输出 JSON 中的关键字段包括：

```text
workflow=rag_document_indexing@v3
status=chunked_with_simulated_downstream
parser.output.granularity=block
parser.output.structured=true
chunking.adapter_name=ingestion
chunking.strategy_name=structure_aware
chunking.chunks=[...]
chunking.relations=[...]
chunking.statistics={...}
```

`chunking.chunks` 包含完整正文和 Metadata，适合本地学习与后续索引节点消费；生产日志和观测链路不应直接记录完整结果。

## 使用 Eino Dev

```bash
EINO_DEV=true go run ./examples/go-study/framework-api-design/19-packaged-document-pipeline
```

在 GoLand Eino Dev 中连接 `127.0.0.1:52538` 并选择 `rag_document_indexing@v3`。从 START 节点执行时可使用：

```json
{
  "run_id": "demo19-eino-dev-run",
  "source_uri": "examples/go-study/framework-api-design/19-packaged-document-pipeline/testdata/knowledge.md"
}
```

Eino Dev 仅用于本地调试，不应暴露到公网或生产环境。

## 验证

```bash
gofmt -w examples/go-study/framework-api-design/19-packaged-document-pipeline
go test ./examples/go-study/framework-api-design/19-packaged-document-pipeline/... -count=1
go test -race ./examples/go-study/framework-api-design/19-packaged-document-pipeline/... -count=1
go vet ./examples/go-study/framework-api-design/19-packaged-document-pipeline/...
go test ./...
go vet ./...
```

测试通过真实 Package 链路覆盖 Markdown 的 Structure-aware 分支和 TXT 的 Parent-child 分支，并验证错误链、依赖边界、输入不可变性以及结构化原子块超限错误。

## 已知限制

- 独立结构化 Parser 当前只实现 Markdown；PDF、DOCX 和 XLSX 的分页、Section 或行粒度不等于语义结构。
- Structure-aware 将代码块和表格视为原子块。单个原子块超过 1800 字符时返回 `ErrOversizeBlock`，不会静默破坏结构；生产环境应按格式注入保留语义的 `OversizeSplitter`。
- Chunk 大小按 Unicode 字符配置，尚未接入模型 Tokenizer。
- Embedding、向量存储、索引校验和发布仍为模拟阶段。
- 远程 URL 的安全、大小、重定向和私网访问限制以 ingestion 当前版本为准。
