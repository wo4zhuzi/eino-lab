# 调研发现

## 已确认

- Eino ADK 主线已经完成 L2；Compose 主线已经通过 L3 验收。
- 当前 `chatModelCustomerReplyGenerator` 在 QualityGate Graph 外直接调用 `model.BaseChatModel.Generate`。
- 现有 scripted ChatModel 测试已经覆盖提示消息、模型错误、空响应、取消和环境配置。
- 当前尚未验证真实模型在线冒烟，也尚未比较 `BaseChatModel.Generate` 与 `AddChatModelNode` 的运行边界。
- Coze Coding 当前空间没有既有 `eino-lab` 项目；已从当前 Git `HEAD` 导入不含 `.env` 和本机文件的协作副本，项目 ID 为 `7664953204635041846`，初始化状态为 `processing`。
- Eino `v0.9.12` 的官方源码和测试均使用 `Graph.AddChatModelNode`；调用级 Callback 通过 `compose.WithCallbacks` 注入 `Runnable.Invoke`。
- `AddChatModelNode` 的原生节点类型是 `[]*schema.Message -> *schema.Message`；业务 `string` 输入输出需要由相邻 Lambda 显式转换。
- Compose 节点失败会由 `wrapGraphNodeError` 包装，错误文本包含 `node path`，`Unwrap` 保留底层错误供 `errors.Is` 使用。
- Eino Graph 官方测试验证了异构节点类型可以串联；因此可以使用 `Graph[string, string]`，中间节点依次传递消息切片和单条消息。
- `toComponentNode` 会依据组件是否自行实现 Callback 决定是否由 Compose 补充包装，避免支持 Callback 的 OpenAI 组件产生重复生命周期事件。
- EinoExt OpenAI `v0.1.13` 在 `Generate/Stream` 中调用 `EnsureRunInfo`，并通过 `IsCallbacksEnabled` 声明底层客户端已支持 Callback。
- 推荐保留 `model` 直接路径并新增 `model_graph`，这样可以用相同模型、提示和 QualityGate 做单变量对照。

## 已验证行为

- scripted ChatModel 在 Graph 内会产生构造消息 Lambda、ChatModel、提取正文 Lambda 的成功 Callback 记录。
- 模型错误与超时保留底层错误链，并包含 `generate_customer_reply` 节点路径。
- 空模型响应在 `extract_customer_reply` 节点转换为 `ErrEmptyCustomerReply`。
- Graph 外和 Graph 内路径向 ChatModel 传入完全相同的 System/User Message，并产生相同的去空白回复正文。
- 使用仓库已有且未回显的 OpenAI 兼容配置完成真实 `model_graph` 冒烟；模型草稿进入原 QualityGate，两次审核后 approved，回复阶段记录 8 条 Callback。
