# RAG 核心概念：从文档到可追溯回答

## 先纠正一个关键概念

下面这句话不准确：

```text
Embedding 模型把文档拆分成多个 Chunk。
```

正确关系是：

```text
Splitter 把文档拆分成多个 Chunk。
Embedding 把每个 Chunk 转换成一个向量。
```

进一步展开：

```text
一篇文档
  ↓ Splitter
多个 Chunk
  ↓ Embedding
每个 Chunk 对应一个向量
```

一个向量内部可以有几百或几千个数字，但整体仍然是一个向量。标准 RAG 通常是一个 Chunk 对应一个向量，一个查询也对应一个向量。

## 一、RAG 到底解决什么问题

RAG 是 Retrieval-Augmented Generation，即“检索增强生成”。它不要求大模型提前把企业资料训练进模型参数，而是在回答前先查找外部资料：

```text
用户问题
  ↓
检索相关资料
  ↓
把问题和资料一起交给 ChatModel
  ↓
生成有证据的回答
```

RAG 的核心不是“使用了向量数据库”，而是：

> 先取得外部证据，再约束模型依据证据回答。

向量检索是常见实现，也可以和关键词检索、元数据过滤、Reranker 一起使用。

## 二、两条独立主链路

### 索引写路径

索引写路径负责把原始资料变成可查询的索引，一般在文件上传、修改或后台同步时执行：

```text
原始资料
  ↓ Loader / Parser
Document
  ↓ Transformer / Splitter
Chunk Documents
  ↓ Embedding
Chunk Vectors
  ↓ Indexer
持久化存储
```

它的产物不是答案，而是以后可以反复使用的 Chunk 和检索索引。

### 查询读路径

查询读路径负责处理每一次用户问题：

```text
用户问题
  ↓ 可选 Query Analyzer / Query Rewrite
可独立检索的问题
  ↓ Retriever
TopK Chunks + Score
  ↓ 可选 Reranker
排序后的证据
  ↓ Prompt + ChatModel
答案 + 引用
```

写路径不应默认放进每次问答，否则每次提问都要重新解析、切分和生成资料向量。

## 三、组件分别扮演什么角色

| 组件 | 输入 | 输出 | 核心责任 | 是否必须使用生成式大模型 |
|---|---|---|---|---|
| Loader | 文件路径、URL、对象存储地址 | 原始 `Document` | 读取资料 | 否 |
| Parser | 文件字节 | 结构化文本、段落、表格等 | 解析 Markdown、PDF、Word、HTML | 否；复杂资料可辅助使用模型 |
| Transformer / Splitter | `Document` | 多个 Chunk `Document` | 清洗、切分、补充结构 | 否 |
| Embedding Embedder | 一批文本 | 一批固定维度向量 | 把文本映射到可比较的向量空间 | 否；通常使用专用 Embedding 模型 |
| Indexer | Chunk 及其向量 | 文档或 Chunk ID | 写入可检索后端 | 否 |
| Vector Store | 向量、Chunk ID、检索元数据 | 相似向量记录 | 保存向量并建立近邻索引 | 否 |
| Retriever | 查询字符串 | 排序后的 Chunk + Score | 生成查询向量、检索并返回资料 | 否 |
| Query Analyzer | 问题和对话历史 | 独立查询、过滤条件 | 消解指代、识别查询意图 | 可选使用 ChatModel |
| Reranker | 问题和候选 Chunk | 重新排序后的 Chunk | 对初次召回结果做精排 | 通常使用专用模型 |
| ChatModel | 问题、证据、约束 Prompt | 自然语言答案 | 基于证据组织回答 | 是 |

在 Eino 中，`document.Transformer` 只是文档转换组件，与大模型训练架构中的 Transformer 不是一个概念。

## 四、Document、Chunk、向量之间的关系

### Document

Eino 的 `schema.Document` 主要包含：

```text
ID
Content
MetaData
```

它既可以表示 Loader 刚读出的完整文档，也可以表示 Splitter 产生的一个 Chunk。是否是完整文档要看它位于哪一步。

### Chunk

Chunk 是为了检索而切出的文本证据单元。例如：

```markdown
# Redis

## 密码配置

Redis 密码可以通过 requirepass 配置。
```

切分后可以得到：

```text
heading_path = Redis > 密码配置
content      = Redis 密码可以通过 requirepass 配置。
```

Chunk 太大容易混入多个主题，太小又可能丢失回答问题所需的上下文。

### 向量

每个 Chunk 通过 Embedder 得到一个向量：

```text
Chunk 1 → Vector V1
Chunk 2 → Vector V2
Chunk 3 → Vector V3
```

查询也会得到一个向量：

```text
Question → Vector Q
```

Retriever 比较 `Q` 与 `V1/V2/V3` 的距离，返回最接近的 Chunk。向量不能还原成原始文字，所以 Chunk 正文仍然必须保存。

## 五、Chunk 是怎么拆出来的

### 确定性切分

第一版优先使用普通函数切分：

```text
Markdown → 按 #、##、### 标题
HTML     → 按标题和块级标签
代码     → 按类型、函数或方法
普通文本 → 按段落、句子和 Token 上限
```

Markdown 越规范，标题切分效果越稳定。可用的第一版仍需要长度兜底：

```text
先按标题切分
  ↓
章节过长：继续按段落或 Token 切分
章节过短：按同一父标题合并
  ↓
保留 heading_path、来源和原文位置
```

### 语义切分

语义切分可以使用 Embedding 比较相邻句子的语义变化，在主题发生明显变化的位置切开。它使用模型判断相似度，但不一定使用生成式大模型。

### 大模型辅助标准化

任意文档进入知识库时，可以增加标准化步骤：

```text
原始文档
  ↓ 专用 Parser / OCR
统一文档结构
  ↓ 质量判断
结构清晰 → 直接切分
结构混乱 → ChatModel 辅助识别章节
```

`建议` 大模型主要负责恢复结构、标题和段落归属，不应自由重写事实正文。必须保留原始段落 ID、页码或偏移量，确保每个 Chunk 能追溯回原文。

## 六、Embedding 的本质

从数学角度，Embedding 是一个映射函数：

```text
f(text) → vector
```

语义 Embedding 可以写成：

```text
fθ(text) → vector
```

其中 `θ` 是模型训练得到的参数。

### 训练阶段

训练数据通常包含查询、相关文本和不相关文本：

```text
Query：退款什么时候到账？
Positive：退款审核通过后通常在 3～5 个工作日到账。
Negative：收货地址可以在订单发货前修改。
```

训练目标是：

```text
Query 与 Positive 的向量更接近
Query 与 Negative 的向量更远
```

模型作者或云厂商完成训练，企业知识库一般直接调用训练好的模型，不需要自己训练。

### 使用阶段

RAG 运行时执行的是推理：

```text
已经训练好的 fθ
  +
当前 Chunk 或问题
  ↓
向量
```

资料没有被重新训练进模型参数，只是被转换成向量并保存在知识库索引中。

### 是否依赖“大模型”

- Hashing、TF-IDF 等方案不依赖神经网络模型。
- 真正的语义 Embedding 通常依赖训练好的专用 Embedding 模型。
- 专用 Embedding 模型通常不是负责对话生成的 ChatModel。
- RAG 也可以只使用关键词检索，不强制要求向量检索。

### 同一句话是否得到同一个向量

在以下条件相同时，多次推理通常会得到相同或数值上极其接近的向量：

```text
文本
模型及版本
Tokenizer 和预处理
query / passage 指令
输出维度
归一化方式
```

修改模型、模型版本、前缀、预处理、维度或归一化方式，向量会改变。索引资料和查询必须使用同一个或明确兼容的向量空间。

有些检索模型会分别使用：

```text
query: 用户问题
passage: 资料正文
```

两者的向量不要求完全相同，但模型训练保证它们仍然可以比较。

## 七、HashingEmbedder 与语义 Embedding

`已验证` 当前项目的 `HashingEmbedder` 没有训练模型。它执行：

```text
文本规范化
  ↓
提取 1～3 字符 n-gram
  ↓
哈希到固定维度
  ↓
累加并归一化
```

它擅长发现字面重合：

```text
退款什么时候到账
退款到账时间
```

但不擅长没有共同字面的近义表达：

```text
退款什么时候到账
钱多久能退回来
```

因此它可以验证 Embedder 接口、向量维度、相似度排序和错误路径，不能代表生产语义检索质量。

## 八、Retriever 到底做什么

Retriever 不是 ChatModel，也不是向量本身。一个向量 Retriever 通常执行：

```text
1. 接收自然语言问题
2. 使用 Embedder 生成问题向量
3. 查询向量索引
4. 应用租户、权限、版本等过滤条件
5. 按相似度排序
6. 应用 TopK 和 ScoreThreshold
7. 返回 Chunk Document + Score
```

所以分工是：

```text
Embedding：生成可比较的坐标
Vector Store：保存坐标并执行近邻查询
Retriever：组织查询并返回业务可用的 Chunk
```

自然语言问题可以直接进行 Embedding 和检索，不需要每次先调用 ChatModel。只有下面这种依赖上下文的问题才可能需要改写：

```text
上一轮：RAG 为什么分写路径和读路径？
这一轮：那这个有什么好处？
```

可以把第二轮改写为：

```text
RAG 区分索引写路径和查询读路径有什么好处？
```

第一版推荐：单轮问题直接检索；多轮问题存在明确指代时，才调用 ChatModel 生成独立查询。改写失败应回退到原问题，并记录原问题与改写结果。

## 九、检索后实际交给谁什么内容

标准流程是：

```text
向量数据库返回 chunk_id + score
  ↓
Retriever 取得 Chunk 正文和元数据
  ↓
可选 Reranker 重新排序
  ↓
Prompt 组合问题与 Chunk
  ↓
ChatModel 生成答案
  ↓
用户看到答案 + 引用
```

用户通常不会只看到向量或原始 Chunk。Chunk 是 ChatModel 的证据，最终界面可以额外展示证据片段和原始文档链接。

更高级的父子检索会用小 Chunk 精确命中，再取得它所属的更大父章节交给 ChatModel；当前项目没有使用这种方案。

## 十、原始文件、Chunk 和向量如何关联

它们通过明确的 ID 关联，不是通过内容临时猜测：

```text
Document
  ↓ 1:N
DocumentVersion
  ↓ 1:N
Chunk
  ↓ 1:N
ChunkEmbedding
```

推荐字段：

```text
documents
  document_id
  title
  current_version_id

document_versions
  version_id
  document_id
  source_path / object_key
  content_hash
  status

chunks
  chunk_id
  version_id
  sequence
  heading_path
  content
  source_start / source_end

chunk_embeddings
  chunk_id
  embedding_model
  embedding_version
  dimension
  vector
```

查询链路是：

```text
问题向量
  ↓
查到 chunk_id
  ↓
取得 Chunk 正文和 version_id
  ↓
根据 version_id 找到 document_id 和 source_path
  ↓
生成答案与原文引用
```

### 为什么都需要保存

| 数据 | 是否保存 | 原因 |
|---|---|---|
| 原始文件 | 是 | 事实来源、重新解析、下载和审计 |
| 标准化全文 | 有转换时建议保存 | 重新切分并保留解析产物 |
| Chunk | 是 | Retriever 和 ChatModel 实际使用的证据正文 |
| 向量 | 是 | 相似度检索索引；可由 Chunk 重新生成 |

原始文件是事实来源，Chunk 和向量是派生数据。向量不能反向恢复 Chunk，只有 Chunk 也无法完全恢复原始排版、表格和文件版本。

## 十一、Chunk 存在哪里

常见方案有三种。

### 向量数据库同时保存 Chunk

```text
vector
chunk_id
content
document_id
source_path
filter metadata
```

查询一次即可得到 Chunk，适合小型系统和第一版。

### 关系数据库保存权威 Chunk，向量库保存索引

```text
关系数据库：Document、Version、Chunk、权限、状态
向量数据库：vector、chunk_id、过滤字段、可选 Chunk 副本
文件存储：原始文件
```

适合已经有独立向量基础设施的大型系统，但必须处理两个数据库的同步和失败重试。

### PostgreSQL + pgvector

```text
PostgreSQL
  ├─ Document、Version、Chunk
  └─ pgvector 向量字段和近邻索引
```

`建议` 当前个人知识库优先采用这一方案，因为关系、Chunk 和向量可以在同一个数据库中管理，减少双写一致性问题。

pgvector 只负责保存和查询向量，不负责把文本生成向量。向量仍然由 Embedder 生成。

## 十二、当前个人知识库的存储设计

### 推荐拓扑

`建议` 第一版使用本地文件夹保存原始 Markdown，使用 PostgreSQL + pgvector 保存文档关系、Chunk 和向量：

```text
本地知识库目录
  └─ 原始 Markdown
       ↓ Loader
Go 应用
       ↓
PostgreSQL + pgvector
  ├─ KnowledgeBase
  ├─ Document
  ├─ DocumentVersion
  ├─ Chunk
  ├─ ChunkEmbedding
  └─ IndexingJob
```

这样保留了清晰的事实层次：

```text
原始 Markdown       = 原始事实来源
PostgreSQL 关系数据 = 文档与索引生命周期的事实来源
Chunk               = 检索和生成使用的证据
pgvector             = 可重建的检索索引
```

### 表及职责

| 表 | 关键字段 | 责任 |
|---|---|---|
| `knowledge_bases` | `id`、`name`、`source_root`、`chunking_profile`、`embedding_space_id` | 区分不同知识库及其策略 |
| `documents` | `id`、`knowledge_base_id`、`source_type`、`source_key`、`current_version_id`、`status` | 表示一份稳定的逻辑文档 |
| `document_versions` | `id`、`document_id`、`content_hash`、`source_uri`、`status`、`created_at` | 保存每次内容版本和处理状态 |
| `chunks` | `id`、`version_id`、`sequence`、`heading_path`、`content`、原文位置 | 保存可检索证据及其血缘 |
| `embedding_spaces` | `id`、模型、版本、维度、查询/文档指令、归一化方式 | 定义一个兼容的向量坐标空间 |
| `chunk_embeddings` | `chunk_id`、`embedding_space_id`、`embedding`、`status` | 保存 Chunk 在指定空间中的向量 |
| `indexing_jobs` | `id`、`document_id`、目标版本、阶段、重试次数、错误 | 管理可恢复的索引任务 |

`source_key` 应保存相对于知识库根目录的稳定路径，例如：

```text
knowledge_bases.source_root = ${RAG_KNOWLEDGE_ROOT}
documents.source_key        = notes/redis.md
```

不要把完整本机绝对路径复制进每个 Chunk。应用使用知识库根目录和相对路径定位原文件，迁移目录时只需要修改知识库配置。

### 为什么单独设计 Embedding Space

下面这些配置共同决定向量空间：

```text
模型名称和版本
向量维度
Tokenizer / 预处理
query / passage 指令
归一化方式
```

不同空间的向量不能直接混合比较。因此数据库中不能只记录一列来源不明的数字，必须知道它由哪套配置生成。

第一版固定一个 Embedding Space。以后更换模型时：

```text
旧 Chunk
  ├─ 旧模型向量：继续服务
  └─ 新模型向量：后台重建
       ↓ 全部完成并评测通过
切换活动 Embedding Space
       ↓
再下线旧向量索引
```

不要在重建一半时让同一次查询同时比较两个不兼容的空间。

pgvector 的近邻索引也必须建立在维度兼容的向量集合上。第一版使用一个固定维度的向量列；模型迁移到不同维度时，应建立新的列、表或分区及对应索引，不能把不同维度直接塞进同一个活动索引。

### 文档首次入库

```text
1. 扫描 Markdown，相对路径作为 source_key
2. 计算原文件 content_hash
3. 创建 Document 和 PENDING DocumentVersion
4. Loader / Splitter 生成 Chunk
5. 为每个 Chunk 生成稳定 chunk_id 和向量
6. 写入 Chunk 与 ChunkEmbedding
7. 在事务中把新版本设为 ACTIVE
8. 更新 documents.current_version_id
```

Embedding API 是外部调用，不应该把数据库事务一直保持到模型请求结束。更稳妥的方式是先创建 `PENDING` 任务，在外部计算完成后使用短事务发布新版本。

### 文档更新

```text
扫描文件
  ↓ content_hash 没变化
跳过索引

扫描文件
  ↓ content_hash 已变化
创建新版本和新 Chunk
  ↓ 全部成功
原子切换 current_version_id
  ↓
旧版本变为 INACTIVE
```

新版本处理失败时继续使用旧版本，不能让半完成的新索引替换仍然可用的旧索引。

### 文档删除

第一步先做逻辑删除：

```text
Document.status = DELETED
DocumentVersion / Chunk 不再参与检索
```

后台任务再清理 ChunkEmbedding 和历史派生数据。涉及隐私或合规删除时，还必须清理原始文件、备份和检索日志，不能只从向量索引删除。

### 查询时的数据流

```text
用户问题
  ↓ Embedder
问题向量
  ↓ PostgreSQL + pgvector
限定 knowledge_base_id、ACTIVE 版本和权限
  ↓ 向量距离排序
TopK chunk_id + score
  ↓ JOIN chunks / document_versions / documents
Chunk 正文 + 标题路径 + source_key
  ↓ ChatModel
答案 + 原始 Markdown 引用
```

在 PostgreSQL + pgvector 中，向量查询和 Chunk 关联可以在一次 SQL 查询中完成，不需要先查一个独立向量数据库，再跨库查询 Chunk。

### 必须观测的状态

可用版本至少需要记录：

- 每份文档当前活动版本及内容哈希。
- 每次索引任务当前阶段、耗时和失败原因。
- Chunk 数量、空 Chunk、超长 Chunk 和向量维度异常。
- 使用的 Embedding Space 和索引版本。
- 查询召回的 `chunk_id`、分数、过滤条件和耗时。
- 最终引用是否来自真实召回 Chunk。

这些记录用于区分“资料没有入库”“Chunk 切错”“向量生成失败”“Retriever 没召回”和“ChatModel 没按证据回答”。

## 十三、不同知识库会遇到什么差异

RAG 的 Loader、Splitter、Embedder、Retriever 等组件具有通用性，但知识库的业务规则并不通用。

### 常见知识库类型

| 知识库类型 | 主要问题 | 推荐处理重点 |
|---|---|---|
| 个人 Markdown 笔记 | 标题不规范、文件改名、重复笔记、隐私 | 标题切分加长度兜底、内容哈希、相对路径、本地部署 |
| 企业制度和 SOP | 新旧版本冲突、生效时间、部门权限、审计 | 版本有效期、ACL 过滤、只召回活动版本、精确引用 |
| 客服和产品文档 | 产品型号、地区、语言、更新频繁 | 产品/地区/语言元数据、增量同步、混合检索、时效规则 |
| 技术文档和代码 | 标识符要求精确、代码结构特殊、版本并存 | AST/章节切分、关键词加向量混合检索、代码版本过滤 |
| 合同、法律和财务资料 | PDF 版面复杂、表格和数字不能错、风险高 | OCR/版面解析、页码引用、数字校验、低置信度人工复核 |
| 工单、聊天和会议记录 | 噪声、重复、说话人、时间顺序、个人信息 | 去重、时间窗口、参与人元数据、PII 脱敏、时效衰减 |
| 订单、库存、账户等实时数据 | 需要精确值和实时一致性 | 不应只做向量 RAG；优先查询 SQL、API 或业务 Tool |

最后一类尤其重要：不是所有“知识”都应该切 Chunk 后向量化。

```text
制度说明、使用手册、笔记
  → 适合文档 RAG

订单状态、余额、库存、实时价格
  → 适合 SQL / API / Tool 查询
```

把实时结构化数据复制进向量库，会产生过期、精度和一致性问题。

### 设计知识库前必须回答的问题

| 维度 | 必须明确的问题 | 影响的组件 |
|---|---|---|
| 数据源 | 来自文件、网页、数据库还是第三方系统 | Connector、Loader、Parser |
| 结构 | 标题、表格、代码、图片是否重要 | Parser、Splitter、标准化 |
| 更新 | 实时、小时、每日还是人工发布 | 同步任务、版本、缓存 |
| 权限 | 用户能看到哪些文档和字段 | 元数据、Retriever 过滤、审计 |
| 有效性 | 多个版本冲突时谁是事实来源 | Version、状态、生效时间 |
| 查询类型 | 偏语义、关键词、精确编号还是计算 | Embedding、BM25、SQL、Tool |
| 引用 | 需要文件、章节、页码还是原始记录 | Chunk 元数据、Citation |
| 风险 | 错误回答的业务代价是什么 | 阈值、拒答、人工复核 |
| 语言与领域 | 是否有内部术语、代码或多语言 | Embedding、词典、Reranker |
| 规模与延迟 | 数据量、QPS、允许的响应时间 | Store、索引类型、缓存 |
| 删除与合规 | 数据如何彻底删除、保留多久 | 数据血缘、清理任务、日志策略 |
| 评测 | 什么结果才算召回和回答正确 | 评测集、指标、反馈闭环 |

没有这些答案就直接选择模型和向量数据库，容易得到一个能演示但无法上线的系统。

### 哪些部分保持统一

不同知识库仍应共享稳定的数据契约：

```text
KnowledgeBase
Document
DocumentVersion
Chunk
EmbeddingSpace
ChunkEmbedding
Citation
```

并共享基本生命周期：

```text
发现资料 → 解析 → 切分 → 向量化 → 发布 → 检索 → 引用 → 删除
```

### 哪些部分做成可配置策略

```text
Source Connector     不同来源如何同步
Parser Profile       不同格式如何解析
Chunking Profile     如何切分、合并和重叠
Embedding Space      使用哪个模型和向量配置
Retrieval Policy     向量、关键词、过滤、TopK、阈值
Access Policy        哪些用户可以检索哪些 Chunk
Freshness Policy     哪个版本有效、多久同步一次
Citation Policy      返回文件、页码、章节还是记录链接
Answer Policy        何时回答、何时拒答、何时人工复核
```

`建议` 不要为每类知识库复制一套完整应用，也不要一开始做能处理所有资料的万能流水线。保留统一的数据模型和接口，通过 Profile 选择少量策略实现。

### 当前项目如何面对差异性

当前个人知识库先固定：

```text
Source Connector  = 本地文件夹
Parser Profile    = Markdown 文本解析
Chunking Profile  = 标题切分 + 后续长度兜底
Embedding Space   = 第一版固定一个模型版本
Store             = PostgreSQL + pgvector
Retrieval Policy  = 向量 TopK + ScoreThreshold
Citation Policy   = 相对文件路径 + 标题路径
Access Policy     = 单用户，不做多租户 ACL
```

刻意暂不支持 PDF、Word、网页、多租户和复杂权限。等当前场景完成持久化、更新、删除和真实语义检索评测后，再通过增加新的 Parser、Chunking Profile 或 Retrieval Policy 扩展，而不是重写主链路。

## 十四、本项目当前真正实现了什么

### 索引 Graph

`已验证` 当前实现是：

```text
FileLoader
  → Markdown Header Splitter
  → prepareChunks 补充 source、heading、chunk_id
  → MemoryVectorStore.Store
      → HashingEmbedder
      → 保存 Chunk Document + Vector
```

### 查询 Graph

`已验证` 当前实现是：

```text
Question
  → 校验非空
  → MemoryVectorStore.Retrieve
      → HashingEmbedder 生成问题向量
      → 逐条计算余弦相似度
      → ScoreThreshold + TopK
  → Branch
      ├─ 无证据：固定拒答
      └─ 有证据：Prompt → ExtractiveChatModel
  → Answer + Retrieved Chunks
```

### 当前 Store 的数据结构

```text
memoryEntry
  ├─ document：Chunk 正文和元数据
  └─ vector：Chunk 向量
```

它实现了向量生成、余弦排序、TopK、阈值和引用链路，但只存在 Go 进程内存中：

```text
程序启动 → 重新读取和索引
程序退出 → Chunk 和向量丢失
```

当前示例没有接入 Redis，也没有接入 PostgreSQL。PostgreSQL + pgvector 目前只完成了安装文档，仍属于待验证迁移目标。

## 十五、从当前示例到可用最小闭环

`建议` 按单变量顺序推进：

```text
第一步：MemoryVectorStore → PostgreSQL + pgvector
目标：持久化、重启复用、增量更新、删除、原文追溯

第二步：HashingEmbedder → 正式语义 Embedding 模型
目标：验证近义表达和真实自然语言召回

第三步：建立检索评测集
目标：分别判断切分、召回、排序和生成是否正确
```

可用最小闭环至少应覆盖：

- 原始 Markdown、DocumentVersion、Chunk 和向量持久化。
- 未变化文件不重复索引。
- 文件更新生成新版本，旧版本不再召回。
- 文件删除后相关 Chunk 不再召回。
- 查询结果能追溯到原始文件和标题路径。
- 使用正式语义 Embedding 模型。
- 无可靠证据时拒绝推测。
- 应用和数据库重启后仍能查询。

## 十六、常见误区速查

| 说法 | 是否正确 | 正确理解 |
|---|---|---|
| Embedding 负责拆 Chunk | 否 | Splitter 拆 Chunk，Embedding 把 Chunk 转向量 |
| 一段 Chunk 会生成很多向量 | 标准 RAG 中通常否 | 一个 Chunk 通常生成一个多维向量 |
| Embedding 负责从数据库拿资料 | 否 | Embedder 生成向量，Retriever 组织查询并返回资料 |
| 企业知识库必须自己训练模型 | 否 | 通常使用训练好的 Embedding 模型和 ChatModel |
| Embedding 模型就是通用对话大模型 | 否 | 通常是专门用于向量表示和检索的模型 |
| 向量库只保存一串向量 | 不一定 | 还可保存 chunk_id、Chunk 正文和过滤元数据 |
| 原始文件查询时完全没有用 | 否 | 用于引用、权限、原文查看和上下文扩展 |
| PostgreSQL 会自动把文本变成向量 | 否 | pgvector 只存储和检索向量，Embedder 负责生成向量 |
| 当前项目已经持久化 | 否 | 当前使用 MemoryVectorStore，进程退出即丢失 |

## 十七、阅读顺序

1. 先记住两个主链路和组件职责。
2. 再理解“一篇文档、多段 Chunk、每段一个向量”。
3. 然后理解 Retriever 如何从问题向量找到 Chunk。
4. 最后理解 Document、Version、Chunk、Embedding 和原始文件的持久化关系。

相关文档：

- [RAG 架构全景](architecture.md)
- [RAG 学习协议](learning-protocol.md)
- [PostgreSQL + pgvector 本地安装](postgresql-pgvector-setup.md)
- [当前最小示例说明](../../../../examples/rag-minimal/README.md)
