# 进度日志

## 2026-08-05

### 总路线初版

- 创建了 Demo 17-24 的初版计划。
- 初版错误地优先安排 Temporal 最小样例，并推迟真实模型与 pgvector。
- 用户指出该路线仍然是“先做小玩具，最后再生产化”，不符合目标。

### 总路线修订

- **状态：** 已完成
- 根因确认：错误地把“分步骤”理解为“先建立非生产架构”。
- 将总目标改为先完成生产级 RAG，再接入 Temporal。
- Demo 17 改为真实 ChatModel + 真实 Embedding + PostgreSQL/pgvector 的生产纵向闭环。
- Demo 18-21 在相同架构上依次加强索引、检索、生成和在线服务治理。
- Demo 22-24 使用 Temporal 编排真实的知识库摄取、索引审批和发布，不再安排 hello-world Activity。
- 增加每个 Demo 的强制讨论门禁，未经用户确认不实现。
- 明确单元测试可使用桩，但真实依赖集成验证是 Demo 完成条件。

## 当前状态

- Demo 17：待设计讨论。
- 代码和依赖：尚未修改。
- 基础设施：尚未启动或变更。

## 本轮创建或修改

- `.planning/eino-temporal-production-roadmap/task_plan.md`
- `.planning/eino-temporal-production-roadmap/findings.md`
- `.planning/eino-temporal-production-roadmap/progress.md`

## 验证结果

| 验证项 | 结果 | 说明 |
|---|---|---|
| 计划方向 | 已修订 | 生产 RAG 优先，Temporal 后接 |
| Markdown 一致性检查 | 通过 | `git diff --check -- .planning` 无错误 |
| 代码测试 | 未执行 | 本轮只修订计划，没有修改代码或依赖 |

## 五问重启检查

| 问题 | 答案 |
|---|---|
| 我在哪里？ | Demo 17 设计讨论前 |
| 我要去哪里？ | 先确认真实模型、Embedding、文档、schema 和验收问题，再实现 Demo 17 |
| 目标是什么？ | 从第一步按最终架构建设生产 RAG，随后接入 Temporal |
| 我学到了什么？ | 生产级必须是全程约束，不能延迟到路线末尾 |
| 我做了什么？ | 废弃玩具优先路线，重写 Demo 17-24 计划 |
