# 生产级 RAG 与 Temporal 演进路线

## 目标

从 Demo 17 开始按最终生产架构建设真实 RAG：使用真实 ChatModel、真实 Embedding、PostgreSQL + pgvector、正式数据模型和可验证故障语义；在 RAG 主链稳定后，再用 Temporal 承载耗时、可恢复的知识库摄取、索引发布与人工审批流程。

## 当前阶段

Demo 17 进入设计讨论，未经用户确认不开始编码。

## 不可退让的工程约束

- 生产架构从 Demo 17 第一天建立，后续 Demo 只补充能力，不推翻核心边界。
- 每个 Demo 的主运行路径必须接入真实 ChatModel；涉及向量化的路径必须接入真实 Embedding。
- 每个 Demo 必须提供真实 PostgreSQL/pgvector 集成验证，不用内存 Store 代替生产路径。
- 单元测试可以使用桩以保证离线、稳定和可重复，但桩不能成为示例主运行实现。
- 密钥和生产地址只通过环境变量或 Secret 注入，不进入代码、测试、日志或提交文件。
- 数据库变更必须使用版本化迁移；索引、模型、Prompt、Embedding 维度和文档处理策略必须可追踪版本。
- 外部调用必须定义超时、取消、错误分类、重试边界和可观测字段。
- 每个 Demo 开工前必须先和用户讨论并确认设计，不能自行跳到实现。
- 当前规划不使用 `my-codebase-intel`。

## 目标架构

```text
在线查询路径
API / CLI
  -> Eino Query Graph
      -> Query Rewrite（按需）
      -> PostgreSQL + pgvector 混合检索
      -> Rerank / 证据门禁
      -> 真实 ChatModel 生成
      -> 引用校验与结构化结果

离线知识库路径
文档源
  -> 解析与切分
  -> 真实 Embedding
  -> PostgreSQL + pgvector 版本化写入
  -> 索引验证
  -> 激活知识库版本

后续 Temporal 路径
Temporal Workflow
  -> 摄取 Activity
  -> 切分与 Embedding Activity
  -> pgvector 写入 Activity
  -> 质量验证 Activity
  -> 等待审批 Signal
  -> 激活或回滚索引版本 Activity
```

## Demo 路线

| Demo | 生产能力 | 状态 | 交付结果 |
|---|---|---|---|
| 17 | 真实 RAG 生产纵向闭环 | 待讨论 | 真实文档摄取、真实 Embedding、pgvector 持久检索、真实 ChatModel、结构化引用全部贯通 |
| 18 | 可重复、可演进的索引写路径 | 待开始 | 文档版本、内容哈希、幂等 upsert、更新/删除、批处理和失败恢复具备正式语义 |
| 19 | 可评估的生产检索 | 待开始 | 向量与全文混合检索、元数据过滤、证据阈值、Rerank 和离线评测闭环 |
| 20 | 可靠答案生成与流式输出 | 待开始 | Prompt 版本、结构化输出、引用校验、无证据拒答、流式错误和取消语义完整 |
| 21 | 在线 RAG 服务治理 | 待开始 | HTTP API、并发控制、超时、限流、熔断、结构化日志、Trace、指标和敏感数据边界完整 |
| 22 | Temporal 持久化知识库摄取 | 待开始 | Temporal + 专用 PostgreSQL + UI 接入；长耗时索引流程可重试、可恢复、可查询 |
| 23 | Temporal 索引发布与人工审批 | 待开始 | Signal 审批、版本激活、定时任务、取消、回滚和重复请求幂等完成 |
| 24 | 端到端生产验收 | 待开始 | 版本升级、历史兼容、故障演练、容量验证、部署回滚、安全和运维清单完成 |

## 各 Demo 的讨论方向

### Demo 17：真实 RAG 生产纵向闭环

从最终结构中切出第一条可用纵向链路，不写假的 Retriever 或 Generator：

- 真实文档加载、清洗与切分。
- PostgreSQL schema migration，保存文档、Chunk、Embedding、来源和版本。
- 真实 Embedding 模型生成向量，维度与模型版本写入配置和数据。
- pgvector 相似度检索，返回稳定文档 ID、Chunk ID、来源和分数。
- Eino Query Graph 调用真实 ChatModel，根据证据生成带引用的结构化结果。
- 无证据门禁、context 取消、外部超时、错误链、RunID 和节点观测。
- 默认单元测试使用桩；显式集成测试连接真实 PostgreSQL 和真实模型，并记录验证结果。

Demo 17 讨论时必须先确认：模型供应方、ChatModel、Embedding 模型、向量维度、文档来源、切分策略、数据库 schema、配置变量、输出契约和验收问题集。

### Demo 18：可重复、可演进的索引写路径

在 Demo 17 的真实链路上加强写路径：

- 文档内容哈希、版本号和摄取批次。
- 重复导入幂等、文档更新、删除和失效 Chunk 清理。
- Embedding 批处理、并发上限、限速、部分失败和可恢复重试。
- 数据库事务边界、唯一约束和索引构建策略。
- 新旧 Embedding 模型或维度迁移时的双写/重建边界。

### Demo 19：可评估的生产检索

在真实 pgvector 和真实模型基础上提升召回质量：

- PostgreSQL 全文检索与向量检索融合。
- 租户、知识库、版本和文档元数据过滤。
- 候选召回、Rerank、TopK、相似度阈值和证据门禁。
- 固定评测问题集、期望证据、召回率和引用正确性指标。
- 使用真实 Embedding 与真实 Rerank/ChatModel 执行显式质量评测。

### Demo 20：可靠答案生成与流式输出

- Prompt 模板和版本治理。
- 真实 ChatModel 的结构化输出与 schema 校验。
- 引用必须来自本次检索证据，禁止模型编造引用。
- 无证据拒答、低置信度降级和模型空响应处理。
- 流式输出、客户端取消、Reader 关闭和流中错误语义。
- Token、延迟和模型调用成本记录。

### Demo 21：在线 RAG 服务治理

- 稳定 HTTP API、请求校验、认证边界和错误协议。
- 并发控制、限流、超时预算、熔断与优雅停机。
- 数据库连接池和模型客户端生命周期。
- Eino Callback + OpenTelemetry 的日志、Trace、指标关联。
- Prompt、用户问题、文档内容和凭据的脱敏策略。
- 压力测试、容量基线和慢查询分析。

### Demo 22：Temporal 持久化知识库摄取

Temporal 不默认进入低延迟在线问答路径，而是接管耗时且需要恢复的离线知识库生命周期：

- 独立 Temporal Server、Web UI 和 Temporal 专用 PostgreSQL。
- Workflow 只做确定性控制流；文档、模型和数据库 I/O 全部放进 Activity。
- 摄取、切分、Embedding、写入、验证拆成具备业务意义的 Activity 边界。
- Activity 超时、heartbeat、有限重试、不可重试错误和幂等键。
- Worker 重启、任务重复投递和执行历史恢复验证。
- 所有 Activity 继续调用真实模型和真实 PostgreSQL/pgvector。

### Demo 23：Temporal 索引发布与人工审批

- 新索引版本完成后等待审核 Signal。
- 批准后原子激活版本，拒绝或超时后保持旧版本。
- 支持 Query、Cancel、重复 Signal、迟到 Signal 和定时重建。
- 发布 Activity 使用幂等键，避免重复激活或重复通知。
- 在线查询始终读取已激活版本，不读取构建中的不完整数据。

### Demo 24：端到端生产验收

- Workflow、Activity、Eino Descriptor、Prompt、模型和数据库 schema 版本关系。
- Temporal 历史重放与旧 Workflow 执行兼容。
- Worker、RAG API 和数据库迁移的滚动升级与回滚。
- Worker 崩溃、模型超时、数据库短暂不可用、重复任务和索引发布中断演练。
- PostgreSQL 备份恢复、连接容量、索引膨胀、数据保留和清理策略。
- TLS、Secret、Temporal UI 权限、Payload 加密和审计边界。

## 每个 Demo 的强制讨论门禁

每个 Demo 严格执行以下流程：

1. 先讨论业务目标和该 Demo 在最终架构中的位置。
2. 核对官方 API、当前依赖版本和外部服务能力。
3. 给出架构图、调用链、目录结构和数据库变更。
4. 讨论事务、幂等、并发、失败、重试、取消、安全和版本边界。
5. 给出单元、集成、在线冒烟、故障演练和验收矩阵。
6. 用户明确认可设计后才修改代码和依赖。
7. 实现完成后共同审查验证结果，再讨论下一个 Demo。

## 测试与真实依赖策略

- 单元测试：使用接口桩，默认离线运行，覆盖正常、错误和边界条件。
- PostgreSQL 集成测试：连接真实 pgvector 容器，验证 migration、事务、约束和查询计划。
- 模型集成测试：使用真实 ChatModel 和 Embedding 凭据，显式运行，不输出密钥或敏感配置。
- 端到端测试：真实文档写入 pgvector，经真实模型生成答案并验证引用。
- 回归门槛：`gofmt`、相关包测试、全仓库测试、`go test -race ./...`、`go vet ./...`。
- 无法执行真实模型验证时，必须标记为未完成，不得用桩测试代替并声称 Demo 通过。

## 已做决策

| 决策 | 理由 |
|---|---|
| 废弃“先做 Temporal 玩具，再接 Eino”的路线 | 不符合用户从一开始按生产架构设计的要求 |
| Demo 17 直接做真实 RAG 纵向闭环 | 先建立最终系统的核心价值路径，后续只做增强 |
| 每个 Demo 主路径使用真实模型和 pgvector | 避免教学实现与生产实现形成两套不同架构 |
| 单元测试仍允许使用桩 | 测试隔离是生产工程要求，不代表主实现是假的 |
| Temporal 优先编排离线摄取与索引发布 | 这些流程耗时、需要恢复和人工审批；在线问答通常需要低延迟，不应默认经过 Temporal |
| 每个 Demo 设置用户讨论门禁 | 用户要求逐步共同决策，避免代理自行扩张或缩小范围 |

## 遇到的错误

| 错误 | 尝试次数 | 解决方案 |
|---|---:|---|
| 初版路线把生产能力推迟到后续 Demo，并安排假的 RAG 主路径 | 1 | 重写为“生产架构先行”的 Demo 17-24 路线；真实模型和 pgvector 从 Demo 17 起始终保留 |

