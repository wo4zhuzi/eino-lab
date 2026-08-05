# 发现与决策

## 用户需求

- 先建设生产级 RAG，再接入 Temporal。
- 生产级目标如果太大可以拆步骤，但必须从第一步就按最终架构设计。
- 每个 Demo 都需要先和用户讨论，确认后再实现。
- 每个 Demo 的可运行主路径都需要接入真实模型。
- 不接受用内存 Retriever、假 Generator 或 hello-world Temporal 作为主交付。
- 不使用 `my-codebase-intel`。

## 对初版计划的根因分析

- 初版计划把“逐步完成”误解为“每个 Demo 只验证一个孤立技术点”。
- 为了降低测试和环境复杂度，错误地把真实模型与 pgvector 推迟到路线末尾。
- 这种安排会先形成一套教学架构，再迁移到生产架构，造成重复实现，也违背用户已经明确提出的生产级目标。
- 正确方法是先确定最终生产架构，再按纵向能力和风险分阶段交付；每一步都运行在相同的核心边界上。

## 仓库事实

- Demo 16 已有强类型 RAG Graph、检索回环、无证据分支、RunID、Observer、结构化错误和最大步数保护。
- Demo 16 的 `MemoryRetriever` 与 `CitationGenerator` 仅适合验证运行层，不能作为 Demo 17 的生产主路径。
- 仓库已锁定 Eino `v0.9.12`，已有 OpenAI 兼容 ChatModel 依赖和真实模型冒烟经验。
- 仓库已有 PostgreSQL 16 + pgvector 的安装文档，但 Go RAG 尚未接入。
- Demo 17 开始前需要确认真实 Embedding 模型、向量维度和当前模型服务是否支持 Embedding API。

## 架构判断

- 在线 RAG 查询是低延迟请求路径，默认由 API 直接调用已编译 Eino Graph。
- 文档摄取、批量 Embedding、索引验证、定时重建和人工发布适合由 Temporal 编排。
- Temporal Workflow 不能直接调用模型、数据库或 Eino；这些 I/O 必须放在 Activity。
- Temporal 内部 PostgreSQL 与 RAG/pgvector 应使用独立数据库、账号和迁移生命周期。
- 真实生产代码与可重复单元测试并不冲突：主适配器连接真实依赖，测试通过接口桩隔离外部服务。

## 当前决策

| 决策 | 理由 |
|---|---|
| Demo 17-21 先完成生产 RAG 主链 | 在引入工作流引擎前先稳定核心业务和数据语义 |
| Demo 22 才引入 Temporal | Temporal 必须编排真实生产活动，而不是 hello-world Activity |
| Temporal 优先承载离线知识库生命周期 | 比把每次在线问答包成 Workflow 更符合延迟和恢复需求 |
| 所有在线模型验证显式运行 | 保持默认测试稳定，同时不降低真实依赖验收门槛 |
| 每个 Demo 先讨论后实施 | 将关键技术决策留给用户共同确认 |

## Demo 17 讨论前必须确认

- ChatModel 供应方、模型名、BaseURL 和当前可用性。
- Embedding 供应方、模型名、维度、批量限制和当前可用性。
- 第一批真实文档来源、格式、规模和更新方式。
- PostgreSQL/pgvector 实例复用或重建策略。
- RAG 输出结构、引用格式和第一组验收问题。
- 是否要求 Demo 17 第一版就提供 HTTP API，还是先以正式 CLI 完成纵向链路、Demo 21 再暴露服务。

## 外部资料

- 尚未开始 Demo 17 API/版本调研；应在设计讨论确认模型与环境后，只使用官方资料核对。

