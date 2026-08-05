# 17. RAG 文档索引工作流：真实解析与完整骨架

前置示例见 [16. 可治理的多工作流运行层](../16-governable-workflow-runtime/README.md)。Demo 17 不再装配内容审核工作流，只保留 RAG 的文档索引写入流程。

## 结论

本示例第一次建立完整的生产型索引拓扑，但只把前两个阶段实现为真实能力：

```text
inspect_source   真实：校验文件、格式签名、大小并计算 SHA-256
parse_document   真实：按格式调用 Eino Parser，输出标准化解析单元
chunk_document   模拟：不生成真实 Chunk
embed_chunks     模拟：不调用 Embedding 模型
persist_index    模拟：不连接 PostgreSQL
validate_index   模拟：不伪造校验通过
publish_index    模拟：不发布索引版本
build_result     真实：输出阶段状态和解析摘要
```

模拟阶段会在返回结果中明确标记为 `simulated`，不会把固定值描述成真实索引产物。后续 Demo 18、19、20 可以逐个替换节点，而不改变工作流入口和整体顺序。

## 学习目标

1. 将文件发现、解析、Chunk、Embedding、持久化、校验和发布放进一条完整索引工作流。
2. 使用可扩展 Parser 注册表隔离不同文件格式。
3. 让真实阶段和模拟阶段在类型、状态和输出中可区分。
4. 为文件内容建立稳定 Document ID 与基于内容的 Version ID。
5. 验证错误发生时工作流立即失败，不继续产生伪造的下游结果。

## 版本与前置条件

仓库使用 Go `1.26.x`、Eino `v0.9.12`，并锁定以下 EinoExt Parser 版本：

```text
github.com/cloudwego/eino-ext/components/document/parser/pdf  @ 90a15623ddb6
github.com/cloudwego/eino-ext/components/document/parser/docx @ 90a15623ddb6
github.com/xuri/excelize/v2                                  @ v2.9.0
```

运行 Demo 17 不需要模型、数据库、API Key 或 Temporal Server。

## 支持格式

| 格式 | Parser | 解析单元 | 当前行为 |
|---|---|---|---|
| `.md` | 严格 UTF-8 Text Parser | document | 保留原始 Markdown 结构 |
| `.txt` | 严格 UTF-8 Text Parser | document | 拒绝 NUL 字节和非法 UTF-8 |
| `.pdf` | EinoExt PDF Parser | page | 逐页提取文本并补充页码 |
| `.docx` | EinoExt DOCX Parser | section | 提取正文、页眉、页脚和表格并按 section 排序 |
| `.xlsx` | 实现 Eino Parser 接口的 Excelize 流式适配器 | row | 单次打开并遍历全部工作表，保留工作表名和行号 |

路由前会验证扩展名和内容签名。DOCX 必须包含 `word/document.xml`，XLSX 必须包含 `xl/workbook.xml`，PDF 必须以 `%PDF-` 开头。只修改扩展名不能绕过检查。

## 统一结果

Parser 的完整正文只在工作流内部传递。最终结果只返回解析单元摘要，避免把大文件正文写入日志：

```text
ParsedUnit
├── id                 内容版本和单元内容生成的稳定 ID
├── type               document / page / section / row
├── index              单元顺序
├── characters         字符数
├── preview            最多 120 个字符
├── page_number        PDF 页码
├── sheet_name         XLSX 工作表名
├── row_number         XLSX 行号
└── section            DOCX section 类型
```

`DocumentID` 根据文件绝对路径生成，同一路径更新内容时保持不变；`VersionID` 等于文件内容 SHA-256，文件内容变化后生成新版本。

## 运行

使用仓库自带 Markdown：

```bash
go run ./examples/go-study/framework-api-design/17-rag-indexing-workflow
```

解析指定文件：

```bash
go run ./examples/go-study/framework-api-design/17-rag-indexing-workflow -- /absolute/path/to/document.pdf
```

输出包含：

```text
workflow=rag_document_indexing@v1
status=parsed_with_simulated_downstream
inspect_source=completed
parse_document=completed
chunk_document=simulated
embed_chunks=simulated
persist_index=simulated
validate_index=simulated
publish_index=simulated
build_result=completed
```

实际程序输出为包含上述字段的 JSON。

## 验证

在仓库根目录执行：

```bash
gofmt -w examples/go-study/framework-api-design/17-rag-indexing-workflow
go test ./examples/go-study/framework-api-design/17-rag-indexing-workflow/... -count=1
go test -race ./examples/go-study/framework-api-design/17-rag-indexing-workflow/... -count=1
go vet ./examples/go-study/framework-api-design/17-rag-indexing-workflow/...
go test ./...
go vet ./...
```

测试在临时目录动态生成 PDF、DOCX 和多工作表 XLSX，不依赖外部服务或仓库中的二进制测试附件。

## 生产边界

- 当前 EinoExt PDF Parser 官方标记为 alpha，只适合文本型 PDF；复杂版面、双栏、表格和扫描件需要后续接入更强的解析/OCR 服务。
- DOCX Parser 不保留图片和完整富文本样式，复杂表格仍需针对业务资料验证。
- XLSX 当前按首行表头、每行一个解析单元；合并单元格、公式、隐藏工作表和大表格需要独立策略。
- 本示例限制单文件最大 `32 MiB`。生产值应按来源、解析器内存模型和 Worker 资源分别配置。
- Eino Graph 负责当前进程内的索引拓扑，不提供节点崩溃恢复。引入 Temporal 后，每个外部操作应放入 Activity，并只在 Workflow History 中传递产物 ID。
- 当前没有 Chunk、Embedding、PostgreSQL/pgvector、索引校验和版本发布；输出已经明确标记这些阶段为模拟。

## 下一步

Demo 18 只替换 `chunk_document`：基于当前标准化解析单元实现格式感知 Chunk，同时保持其余未实现阶段继续标记为模拟。
