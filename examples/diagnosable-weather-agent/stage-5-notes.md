# 阶段 5：天气 Agent 运行与错误责任详解

## 记录目的

本文保存学习 Eino ADK 过程中最容易混淆、但对排障最重要的问题：谁选择 Tool、谁执行 Tool、ToolCall 和 Tool 结果是谁生成的、失败由谁判断、为什么失败后不再调用模型，以及 `AgentEvent` 和 Callback 分别负责什么。

本文记录阶段 5 完成时的非流式基线。阶段 6 已把 Runner 单变量迁移为流式；当前实现与验证结果以[示例 README](README.md)和[学习协议](../../docs/learning/eino/learning-protocol.md)为准。

验证范围：

- Go `1.26.3`，模块 directive 为 `go 1.26.0`。
- Eino `v0.9.12`。
- EinoExt OpenAI `v0.1.13`。
- 本文追踪的阶段 5 基线为单 Agent、单本地 Tool、非流式 Runner。
- 默认验证使用离线 scripted model，不依赖真实模型服务。

详细源码入口见[源码导航](../../docs/learning/eino/source-map.md)，完整故障实验见[故障矩阵](../../docs/learning/eino/failure-matrix.md)。

## 先建立准确的角色模型

| 组件 | 当前示例中的责任 | 不负责什么 |
|---|---|---|
| `Runner` | 接收一次 Query，创建用户消息，进入统一 Agent 生命周期，向应用暴露事件迭代器 | 不理解天气业务，不选择 Tool，不执行 Tool |
| `ChatModelAgent` | 管理模型与 Tool 之间的 ReAct 循环，决定根据运行结果继续循环还是结束 | 不亲自查询天气；语义上的 Tool 选择来自模型输出 |
| `ChatModel` | 读取用户消息、Instruction 和 ToolInfo，生成普通回答或带名称与参数的 ToolCall | 不执行本地 Go Tool，不读取 Tool 的 Go 源码 |
| `ToolsNode` | 根据 ToolCall 中的名称查找已注册 Tool，传入参数并执行 | 不决定用户意图，不生成最终自然语言回答 |
| `weather_lookup` | 校验参数并调用 `WeatherProvider.Lookup`，返回天气 JSON 或 Go error | 不管理 ReAct，不包装最终对话措辞 |
| `WeatherProvider` | 提供具体天气数据或依赖错误；当前实现读取静态 `map` | 不属于 Eino，不知道 Agent 或模型的存在 |
| `AgentEvent` | 把模型消息、Tool 消息或运行错误交给调用方 | 不选择 Tool，也不判断业务是否正确 |
| Callback / `Observer` | 旁路记录组件、状态、耗时和错误类别 | 不传播业务结果，不控制成功或失败，不负责重试 |

最短记忆方式：

```text
Runner 管一次运行
ChatModelAgent 管循环
ChatModel 产生命令
ToolsNode 找到并调用 Tool
Tool 产生领域结果或错误
AgentEvent 把结果或错误送出去
Callback 只观察
```

## ToolCall 不是 Tool 执行结果

模型第一次返回的 ToolCall 只是一个结构化调用意图，类似：

```json
{
  "id": "weather-call-1",
  "name": "weather_lookup",
  "arguments": {
    "city": "Beijing"
  }
}
```

其中：

- ToolCall 的名称、参数和 call ID 由模型生成。
- 天气结果不是模型在这一步生成的。
- `ToolsNode` 读取名称 `weather_lookup`，从已注册索引中定位本地 Tool。
- `weather_lookup` 执行后才产生天气结果。
- 模型第二次看到 Tool 结果，才生成面向用户的最终措辞。

因此下面三个对象不能混为一谈：

| 对象 | 产生者 | 示例内容 |
|---|---|---|
| `ToolCall` | ChatModel | 调用 `weather_lookup`，参数为 `Beijing` |
| Tool 结果 | `weather_lookup` / `WeatherProvider` | 北京、晴、28°C、湿度 35% |
| 最终 Assistant 回答 | ChatModel | 将 Tool 结果组织成自然语言 |

## Tool 如何被模型和执行器同时知道

应用在 [agent.go](agent.go) 的 `NewWeatherAgent` 中把 `weatherTool` 放入 `ToolsNodeConfig.Tools`：

```go
ToolsConfig: adk.ToolsConfig{
	ToolsNodeConfig: compose.ToolsNodeConfig{
		Tools: []tool.BaseTool{weatherTool},
	},
},
```

注册后，同一个 Tool 对象走两条用途不同的路径：

```text
weatherTool.Info(ctx)
-> 名称、描述、参数 JSON Schema
-> model.WithTools(...)
-> 模型知道允许调用哪些 Tool

weatherTool.InvokableRun(ctx, arguments)
-> ToolsNode 保存执行入口并按名称索引
-> 模型产生 ToolCall 后执行本地函数
```

模型只看到 `Info(ctx)` 返回的说明书，不会看到 `InvokableRun` 的 Go 代码。Tool 写好了但没有放进 `Tools` 切片时，会同时出现两个结果：模型看不到它，`ToolsNode` 也没有它的执行入口。

## 分支一：Tool 成功

`已验证` 正常路径如下：

```text
1. Runner.Query 接收用户问题
2. ChatModelAgent 把消息和 ToolInfo 交给 ChatModel
3. ChatModel 返回 AssistantMessage + ToolCall
4. ReAct 图发现 ToolCalls 非空，进入 ToolsNode
5. ToolsNode 按名称找到 weather_lookup
6. weather_lookup 调用 WeatherProvider.Lookup
7. Provider 返回 Weather
8. InferTool 把 Weather 编码为 Tool 结果
9. Tool 结果作为 ToolMessage 追加到消息历史
10. ChatModelAgent 再次调用 ChatModel
11. ChatModel 生成无 ToolCall 的最终 AssistantMessage
12. AgentEvent 把最终消息交给 WeatherAgent.Query
```

[agent_test.go](agent_test.go) 的 `TestWeatherAgentReAct` 验证了：

- 模型总共调用两次。
- 第二次模型调用看到了 call ID 匹配的 ToolMessage。
- Tool Callback 出现 `succeeded` 记录。
- Observer 没有记录城市参数等正文数据。

这里第二次调用模型的目的不是再次查询天气，而是让模型根据可信 Tool 数据组织最终回答。

## 分支二：模型没有产生 ToolCall

`已验证` ReAct 图检查模型消息中的 `ToolCalls`：

```text
ToolCalls 非空 -> ToolsNode
ToolCalls 为空 -> 本轮作为最终回答并结束
```

因此“没有 ToolCall”不属于 Tool 失败，因为 Tool 根本没有执行。排查顺序应该从模型侧开始：

1. 模型服务是否真正支持 Tool Calling。
2. `model.WithTools(...)` 是否把 ToolInfo 传给了模型实现。
3. Tool 名称、描述和参数 Schema 是否清晰正确。
4. Instruction 是否表达了调用要求。
5. 模型是否忽略了 Instruction，直接生成普通 Assistant 回答。

当前 Instruction 写了 `Always call weather_lookup before answering`，但这只是给模型的指令，不是代码级强制校验。如果模型仍然直接回答，[agent.go](agent.go) 的 `Query` 会把无 ToolCall 的 AssistantMessage 当作最终结果。生产环境如果必须保证“未经 Tool 查询不得回答”，需要增加程序级校验或确定性编排，不能只依赖提示词。

## 分支三：Tool 返回错误

### 谁觉得 Tool 错了

在当前实现中，不是模型判断 Tool 错了。错误责任应分层理解：

| 层次 | 责任 |
|---|---|
| 错误来源 | `WeatherProvider` 或 `weather_lookup` 返回非 nil Go error |
| Tool 调度层 | `ToolsNode` 收到并使用 `%w` 增加 Tool 名称、call ID 等上下文 |
| 编排层 | Compose 节点失败，`ChatModelAgent` 不能继续本次 ReAct 循环 |
| 对外传递 | `ChatModelAgent` 发送 `AgentEvent{Err: err}` |
| 应用入口 | `WeatherAgent.Query` 检查 `event.Err` 并继续使用 `%w` 返回 |
| 错误分类 | `main.run` 通过 `ErrorKind` 和 `errors.Is` 判断具体类别 |
| 观测层 | Callback 记录 Tool `failed`，但不决定或传递失败 |

完整传播路径：

```text
WeatherProvider error
-> weather_lookup 返回 %w 包装错误
-> InferTool 返回错误
-> ToolsNode 返回 %w 包装错误
-> Compose NodeRunError（仍可 Unwrap）
-> ChatModelAgent.handleRunFuncError
-> AgentEvent.Err
-> WeatherAgent.Query 返回 %w 包装错误
-> main.run / errors.Is / ErrorKind
```

### 为什么失败后不再调用模型

当前 `weather_lookup` 把失败作为 Go error 返回。对 ReAct 图而言，`ToolNode` 节点执行失败，整条图进入错误返回路径，没有可追加到消息历史的正常 ToolMessage，所以不会进入第二次 ChatModel 调用。

```text
第一次模型调用
-> ToolCall
-> weather_lookup 返回 error
-> AgentEvent.Err
-> 结束

不会发生：
error -> 第二次模型调用 -> 模型解释错误
```

这也是为什么最终错误由应用看到，而模型不会生成“查询失败，请稍后重试”的包装文案。要让模型理解失败并选择其他 Tool，必须把该失败转换成模型可读的正常 Tool 结果，或者配置专门的 Tool 错误处理策略；当前示例没有这样做。

### Tool 的业务结果和 Go error 不同

是否返回 Go error 是应用设计决策，不是所有“不成功”都必须终止 Agent：

| 情况 | 推荐表达 | 当前 ReAct 行为 |
|---|---|---|
| 查询成功 | 正常 Tool 结果 | 回到模型生成最终回答 |
| 没找到数据，但希望模型解释或换条件 | 正常结构化结果，例如 `found=false` | 回到模型继续推理 |
| 用户可修正的输入问题，且希望模型追问 | 正常结构化结果或受控错误结果 | 回到模型追问用户 |
| 依赖不可用、超时、内部异常 | Go error | 通过 `AgentEvent.Err` 结束 |

当前示例故意把“不支持城市”实现为 `ErrUnsupportedCity`，目的是验证业务错误能否穿过 Eino 错误链，而不是宣称生产环境必须这样建模。真实天气 Agent 如果希望模型回答“目前只支持北京、上海和深圳，请重新选择”，可以把它设计成正常结构化 Tool 结果。

## 分支四：ChatModel 返回错误

模型错误与 Tool 错误的入口形式最终相同，都会通过 `AgentEvent.Err` 到达应用，但故障位置不同：

```text
第一次模型调用失败
-> 没有 ToolCall
-> Tool 从未执行
-> AgentEvent.Err

Tool 成功后第二次模型调用失败
-> 已有 Tool 结果
-> 没有最终 Assistant 回答
-> AgentEvent.Err
```

排障时应结合 Callback 的失败组件判断起点：

- `ChatModel failed`：先查模型配置、代理地址、鉴权、超时、模型兼容性。
- `Tool failed`：先查 Tool 参数、Provider、下游 API 和业务错误。
- 只有 `AgentEvent.Err`：继续沿错误链和节点路径定位，不要仅凭入口错误猜测来源。

## AgentEvent 和 Callback 为什么不能混用

`AgentEvent` 属于运行结果通道：

- 成功时携带 AssistantMessage 或 ToolMessage。
- 失败时携带 `Err`。
- 应用必须消费并检查 `event.Err`。

Callback 属于观测旁路：

- 记录组件开始、成功、失败、耗时和错误类别。
- 可以接日志、指标或 Trace。
- 不负责把 Tool 结果送回模型。
- 不负责把错误送回应用。
- 不应该通过吞掉错误来把失败伪装成成功。

所以即使删除当前 `Observer`，Agent 的成功与错误语义也不应改变；只是失去组件级诊断记录。

## 当前示例的自测问答

### 1. 谁管理一次用户请求的运行生命周期？

`Runner`。它把 Query 转为 Agent 输入，并向调用方暴露事件迭代器。

### 2. 谁控制“模型 -> Tool -> 模型”的循环？

`ChatModelAgent` 内部的 ReAct 图。

### 3. 谁决定是否产生 ToolCall，以及选择哪个 Tool？

ChatModel。它根据用户消息、Instruction 和已注册 Tool 的 ToolInfo 生成 ToolCall。Agent 负责提供可选集合并执行循环。

### 4. 谁真正执行 Tool？

`ToolsNode` 按名称找到执行入口，再调用 `weather_lookup.InvokableRun`。

### 5. ToolCall 是谁生成的？

ChatModel 生成。它是调用意图，不是天气结果。

### 6. 天气查询结果是谁生成的？

`WeatherProvider` 提供领域数据，`weather_lookup` 和 `InferTool` 将它变成 Tool 结果。ChatModel 只负责根据这个结果生成最终自然语言回答。

### 7. Tool 成功后为什么再次调用模型？

因为 Tool 返回的是领域数据，模型需要结合用户问题和该数据生成最终 Assistant 回答，也可能决定继续调用其他 Tool。

### 8. Tool 返回 Go error 后谁决定结束？

ToolNode 节点以错误结束，ReAct 图无法继续；`ChatModelAgent` 将运行错误写入 `AgentEvent.Err`。不是模型读取错误后决定结束。

### 9. Tool 失败后还会再次调用模型吗？

当前配置不会。错误路径直接通过 `AgentEvent.Err` 返回。

### 10. 如果模型没有生成 ToolCall，应该先查 Tool 吗？

不应该。此时 Tool 没执行，应先查 ChatModel 的 Tool Calling 支持、ToolInfo 传递、描述与 Instruction。

### 11. `AgentEvent` 的作用是什么？

统一把运行中的消息或错误送给调用方。应用先检查 `event.Err`，再提取消息。

### 12. Callback 的作用是什么？

只负责观测。它能帮助判断失败发生在 ChatModel 还是 Tool，但不负责控制 ReAct 或传播业务错误。

### 13. Tool 必须提前写好吗？

Tool 能力在调用前必须存在，并在本次运行中对 Agent 可见。Tool 集合可以启动时静态注册，也可以按权限、配置或外部发现结果动态选择。

### 14. 当前天气数据是实时的吗？

不是。`StaticWeatherProvider` 使用写死的三城市 `map`。它只是为了隔离真实网络并稳定验证 Agent 链路。

### 15. 阶段 5 基线是流式还是非流式？

阶段 5 基线是非流式：Tool 是 `InvokableTool`，Runner 设置 `EnableStreaming=false`。阶段 6 已把 Runner 改为 `EnableStreaming=true`，Tool 仍是非流式；模型输出流式与 Tool 输出流式是两个不同维度。

## 用故障位置选择排查入口

| 现象 | 第一个排查对象 | 原因 |
|---|---|---|
| 模型直接回答，没有 ToolCall | ChatModel、ToolInfo、Instruction | ToolsNode 尚未执行 |
| 返回 `tool ... not found` | ToolCall 名称与注册表 | 模型给出的名称无法在 ToolsNode 索引中找到 |
| Tool Callback 为 `failed` | Tool 参数、Provider、下游依赖 | 已经进入 Tool 执行阶段 |
| Tool 成功但没有最终回答 | 第二次 ChatModel 调用 | Tool 数据已经产生，最终包装阶段失败 |
| `AgentEvent.Err` 且 `errors.Is` 命中领域错误 | 最初返回该错误的 Provider/Tool | 中间层 `%w` 保留了原始错误链 |
| 只有 Callback 缺失但结果正常 | Observer 注入和 Callback 配置 | 观测缺失不等于业务执行失败 |

## 验证命令

在仓库根目录执行：

```bash
go test ./examples/diagnosable-weather-agent/... -run 'TestWeatherAgentReAct|TestWeatherAgentFailures|TestWeatherTool' -count=1
go test ./examples/diagnosable-weather-agent/... -count=1
```

这些测试覆盖正常两次模型调用、Tool 成功、Tool 参数错误、Provider 业务错误、依赖不可用、deadline 和 Callback 记录。当前没有单独的“模型无 ToolCall”回归测试，该分支结论来自 Eino `v0.9.12` ReAct 源码和应用 `Query` 的筛选逻辑；如果后续把“必须调用 Tool”升级为程序级契约，应先补充对应测试。
