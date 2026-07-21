# 进度记录

## 2026-07-21

- 恢复 Eino 与 Compose 学习协议，确认下一阶段是 ChatModel 单变量迁移。
- 读取现有 ChatModel 适配器、环境配置和示例 README。
- 读取 `learn-framework`、`planning-with-files-zh` 和 `using-coze-cli` 执行规则。
- Coze CLI 认证与项目查询首次失败：沙箱无法解析 Coze API 域名，尚未创建或修改任何 Coze 项目。
- 经授权重试后确认 Coze CLI 已登录；当前空间不存在同名项目。
- 使用 `git archive` 从当前 `HEAD` 生成无敏感信息临时 ZIP，并导入 Coze Coding 项目 `7664953204635041846`；没有部署或推送。
- 开始核对 Eino `v0.9.12` 的 `AddChatModelNode`、Callback 和错误包装实现。
- 完成 API 核对与迁移设计：新增 `model_graph` 对照模式，三节点回复生成 Graph 与现有 QualityGate 分离。
- 完成本地首版实现：新增 ChatModel 回复生成子 Graph、`model_graph` 配置模式、独立回复 Callback 统计和五组离线行为测试。
- `gofmt` 已完成；首次示例包测试因默认 Go 构建缓存无写权限而在 setup 阶段失败，尚未执行测试代码。
- 设置临时 `GOCACHE` 后，`go test ./examples/compose-quality-gate` 通过，首版行为断言与 Eino 实际错误路径一致。
- 默认离线 `go run` 通过；首次在线 `model_graph` 冒烟因临时环境读取器保留 URL 双引号而失败，请求尚未发出，错误已正确包含 `generate_customer_reply` 节点路径。
- 临时读取器安全剥离成对双引号后，真实 `model_graph` 在线冒烟通过，未输出密钥或 BaseURL。
- 已更新根 README、示例 README、运行链路、故障矩阵、源码导航和 Compose 学习协议。
- 完整回归通过：示例包测试、全仓库测试、竞态检测、`go vet` 和默认离线运行退出码均为 0。
- 本地实现与学习协议已收口，下一学习主题为流式 ChatModel。
