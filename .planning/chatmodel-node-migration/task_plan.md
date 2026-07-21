# ChatModel 节点迁移计划

## 目标

在不改变现有 QualityGate 节点、Branch、Local State 和审核语义的前提下，增加基于 `compose.AddChatModelNode` 的客服回复生成路径，并用离线测试比较 Graph 外直接调用与 Graph 内节点调用的数据类型、回调范围和错误传播。

## 阶段

| 阶段 | 状态 | 验收条件 |
|---|---|---|
| 1. 恢复现状与核对 API | 已完成 | 明确现有调用链、Eino v0.9.12 公开 API 和测试边界 |
| 2. 设计单变量迁移 | 已完成 | 只迁移回复生成器，QualityGate 拓扑保持不变 |
| 3. 实现 Graph 内 ChatModel | 已完成 | 支持显式模式选择、错误包装和 Callback 观测 |
| 4. 补充测试与文档 | 已完成 | 覆盖正常、超时、模型错误、空响应和两种路径对比 |
| 5. 完整验证与协议更新 | 已完成 | gofmt、test、race、vet、离线示例通过；在线冒烟按凭据条件记录 |

## 约束

- 锁定 Eino `v0.9.12` 和 EinoExt OpenAI `v0.1.13`，不升级依赖。
- 默认测试和示例不得访问网络；真实模型只做显式在线冒烟。
- 不修改现有 QualityGate 的业务拓扑和状态语义。
- 不输出 `.env` 中的密钥或服务地址。

## 实现决策

- 保留 `CUSTOMER_REPLY_MODE=model` 作为 Graph 外直接调用基线。
- 新增 `CUSTOMER_REPLY_MODE=model_graph`，内部拓扑固定为：构造消息 Lambda -> ChatModel 节点 -> 提取正文 Lambda。
- 两条模型路径共享问题校验、消息构造和响应解析函数，确保只改变模型是否进入 Compose Graph。
- Graph 路径在构造时接收调用级 Callback；现有 QualityGate 继续使用独立 Observer，避免观测记录混淆。

## 遇到的错误

| 错误 | 尝试次数 | 处理 |
|---|---:|---|
| Coze CLI 无法解析 `api.coze.cn` / `code.coze.cn` | 1 | 判定为沙箱网络限制，按权限规则申请网络重试 |
| 误判 Eino 模型 Callback 文件为 `components/model/callback.go` | 1 | 文件不存在，改用公开符号检索定位实际实现 |
| `go test` 无法写默认 Go 构建缓存 | 1 | 将 `GOCACHE` 指向 `/tmp/eino-lab-gocache` 后重跑 |
| 在线冒烟的临时 `.env` 读取器保留了值两侧引号 | 1 | URL 在请求前解析失败；调整临时读取器只剥离成对引号，不修改 `.env` |

## 完成结论

- ChatModel Graph 内外单变量对照已实现并验证。
- QualityGate 拓扑、状态和审核语义保持不变。
- 下一阶段进入流式 ChatModel，不在本计划继续扩展范围。
