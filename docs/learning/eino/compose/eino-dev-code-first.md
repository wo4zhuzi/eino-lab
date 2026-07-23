# Eino Dev 查看 Code First Workflow

## 结论

本项目采用 `Go Code First + Eino Dev 可视化调试`：Graph 的节点、边、Branch、Local State 和业务逻辑继续以 Go 源码为唯一事实来源；Eino Dev 连接正在运行的开发进程，读取编译后的 Graph 元数据，并提供拓扑展示和 mock 调试。

Eino Dev 不扫描源码，也不会把画布修改反向写回现有 Go 文件。对于已有 Code First Graph，推荐把它定位为开发期的查看和调试工具。

## 版本与前置条件

本文已经针对以下版本验证：

- Go `1.26.3`，项目最低约束见根目录 `go.mod`。
- Eino `v0.9.12`。
- EinoExt DevOps `v0.1.9`。
- GoLand Eino Dev 插件 `1.3.1`。

默认模拟模式不访问模型服务，不需要 API Key。首次启动本地 HTTP 服务时，操作系统或 IDE 可能弹出网络访问提示，只允许本机开发所需的访问即可。

## 运行原理

```text
应用进程启动
  -> devops.Init(ctx)
       -> 注册全局 GraphCompileCallback
       -> 启动 127.0.0.1:52538 HTTP 调试服务
  -> 应用照常创建节点、边和 Branch
  -> Graph.Compile(...)
       -> Eino 完成拓扑和类型校验
       -> DevOps 编译回调接收 GraphInfo
       -> 按 Graph 名称保存节点、边、分支、类型和 Runnable 信息
  -> GoLand Eino Dev 连接 127.0.0.1:52538
       -> 查询 Graph 列表和画布信息
       -> 展示拓扑
       -> Test Run 时向调试接口提交 mock 输入
```

关键点不是“在 Go 文件加载前导入包”，而是必须在任何目标 Graph 调用 `Compile` 之前执行 `devops.Init`。否则编译事件已经结束，DevOps 无法捕获该 Graph。

本项目的质量门禁在 `NewQualityGate` 中编译，并通过 `compose.WithGraphName("compose_quality_gate")` 提供稳定名称。启用 `CUSTOMER_REPLY_MODE=model_graph` 时，还会编译 `customer_reply_generation`。

## 本项目的最小接入

入口使用 `EINO_DEV=true` 显式开启开发模式：

```go
einoDevEnabled := os.Getenv("EINO_DEV") == "true"
if einoDevEnabled {
	if err := devops.Init(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "init eino devops: %v\n", err)
		os.Exit(1)
	}
}
```

当前示例执行一次就会退出，而 Eino Dev 需要持续连接目标进程，所以开发模式在业务演示结束后等待 `SIGINT` 或 `SIGTERM`：

```go
ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
defer stop()
<-ctx.Done()
```

这段等待是一次性命令行 Demo 的生命周期补偿，不是 DevOps 的通用要求。HTTP、RPC、消息消费者等常驻服务已经有自己的阻塞和优雅退出流程，不应再增加一份等待逻辑。

EinoExt DevOps `v0.1.9` 没有 `Exit` API；调试 HTTP 服务随当前进程退出。

## 查看当前 Workflow

### 1. 启动开发模式

在仓库根目录执行：

```bash
EINO_DEV=true go run ./examples/compose-quality-gate
```

默认会先执行一次离线审核，随后输出：

```text
eino_dev=ready address=127.0.0.1:52538
按 Ctrl+C 停止 Eino Dev 模式
```

保持该进程运行。

### 2. 验证调试服务

新开终端执行：

```bash
curl http://127.0.0.1:52538/eino/devops/ping
```

预期响应：

```json
{"code":0,"msg":"success","data":"pong"}
```

也可以直接查看已注册 Graph：

```bash
curl http://127.0.0.1:52538/eino/devops/debug/v1/graphs
```

响应中应包含 `compose_quality_gate`。

### 3. 在 GoLand 中连接

1. 打开 GoLand 右侧的 `Eino Dev` 工具窗口。
2. 进入调试功能，点击配置调试地址。
3. 填写 `127.0.0.1:52538` 并确认。
4. 从 Graph 列表选择 `compose_quality_gate`。
5. 画布应展示 `START -> validate -> inspect`，以及 `approve`、`remediate`、`manual` 三个分支目标。

### 4. 执行 Test Run

从 START 节点开始运行，输入：

```json
{
  "Content": "您好，我们正在为您核实退款处理进度。"
}
```

确认后，在调试区域切换 `Input` 和 `Output`，查看各节点的输入、输出和执行路径。本输入缺少退款到账时间说明，预期先经过 `remediate`，再回到 `inspect`，最终进入 `approve`。

Eino Dev 也支持从可操作的中间节点开始 mock 调试。中间节点输入必须匹配该节点的真实 Go 类型，不能只按字段名称猜测。

### 5. 停止开发模式

回到运行进程的终端，按 `Ctrl+C`。普通运行不设置 `EINO_DEV`，行为保持不变：

```bash
go run ./examples/compose-quality-gate
```

## 两个 Graph 的显示条件

| 运行模式 | Eino Dev 中的 Graph | 外部条件 |
|---|---|---|
| 默认 `simulated` | `compose_quality_gate` | 无 |
| `model` | `compose_quality_gate` | 需要 OpenAI 兼容服务；模型调用不在 Graph 中 |
| `model_graph` | `customer_reply_generation`、`compose_quality_gate` | 需要 OpenAI 兼容服务 |

如果只是学习 Eino Dev，不要先启用模型模式。默认模拟模式已经可以完整观察 Branch、Local State 和 `remediate -> inspect` 回环。

## 常见问题

### 插件连接失败

依次检查：

```bash
lsof -nP -iTCP:52538 -sTCP:LISTEN
curl http://127.0.0.1:52538/eino/devops/ping
```

若端口未监听，确认使用了 `EINO_DEV=true`，并检查程序是否在 `devops.Init` 或 Graph 构建阶段报错退出。

### 服务可连接但没有 Graph

确认以下顺序：

```text
devops.Init
-> NewGraph / NewChain
-> AddNode / AddEdge / AddBranch
-> Compile
```

`devops.Init` 晚于 `Compile` 是最常见原因。另一个常见原因是程序只创建 Graph，却没有执行 `Compile`。

### 端口被占用

先停止旧的示例进程。确需换端口时，可调用：

```go
err := devops.Init(ctx, devops.WithDevServerPort("52539"))
```

随后把 GoLand 的调试地址改为 `127.0.0.1:52539`。团队内应固定端口约定，避免每个人使用不同配置。

### 为什么画布不能修改现有源码

运行时 `GraphInfo` 描述的是编译结果，不包含把动态配置、函数封装、闭包、自定义 Lambda 和跨文件调用无损还原为原始 Go 代码所需的全部信息。因此 Code First 模式是单向关系：

```text
Go 源码 -> 编译后的 GraphInfo -> Eino Dev 画布
```

插件中的可视化编排和代码生成属于另一条工作流，不应与手写源码建立双向同步假设。

## 生产级项目的推荐方式

### 推荐架构

```text
cmd/service                 生产入口，不初始化 DevOps
cmd/service-dev             本地调试入口，初始化 DevOps
internal/workflow           共享 Graph 构建函数
internal/nodes              业务节点和外部能力适配
internal/observability      生产 Callback、Trace、Metrics
```

生产项目推荐使用独立开发入口或 Go build tag 隔离 DevOps，而不是仅依赖环境变量保护同一个生产二进制。两种方案的取舍如下：

| 方案 | 适用场景 | 取舍 |
|---|---|---|
| 独立 `cmd/service-dev` | 推荐；中大型服务 | 边界最清晰，可复用相同 Graph builder，但多一个入口 |
| `//go:build einodev` | 单二进制源码树、严格构建流程 | 生产产物不包含 DevOps，需要维护构建标签 |
| `EINO_DEV=true` 运行时开关 | Demo、本地实验 | 改动最少，但 DevOps 代码仍进入二进制，不是生产首选 |

### 安全边界

- 默认只监听 `127.0.0.1`，不要在生产配置中使用 `0.0.0.0`。
- DevOps 调试接口可以枚举和执行 Graph，不应直接暴露到公网、集群入口或服务网格公共路由。
- 远程调试优先使用 SSH 端口转发或受控开发环境，不要通过开放安全组暴露 52538。
- 调试输入可能包含业务数据；只使用脱敏或构造数据，不把密钥、用户隐私和生产请求粘贴到 Test Run。

SSH 端口转发示例：

```bash
ssh -L 52538:127.0.0.1:52538 developer@dev-host
```

然后 Eino Dev 仍连接本机 `127.0.0.1:52538`。

### 工程治理

- 锁定 Eino 与 EinoExt DevOps 版本，升级后重新验证 Graph 列表、类型序列化和 Test Run。
- 所有 Graph 使用稳定且唯一的 `WithGraphName`，避免列表中出现调用位置生成的名称或名称冲突。
- Graph builder、节点逻辑和分支规则必须有单元测试；可视化调试不能替代自动化测试。
- CI 默认不启动 DevOps，不开放端口，也不依赖 IDE 插件完成验收。
- 生产可观测性使用 Callback、Trace、Metrics 和结构化日志；不要把 Eino Dev 当作线上监控系统。
- 常驻服务复用已有的启动和优雅退出生命周期，不为 Eino Dev 单独增加永久阻塞。

## 已验证事实与限制

已验证：

- DevOps `v0.1.9` 通过全局 Graph 编译回调收集 `GraphInfo`。
- 默认地址是 `127.0.0.1:52538`，ping 路径为 `/eino/devops/ping`。
- Graph 列表路径为 `/eino/devops/debug/v1/graphs`。
- 当前示例为 Graph 指定了稳定名称 `compose_quality_gate`。
- 本地画布接口已经返回 `ReviewRequest.Content` 输入结构、5 个业务节点、Branch 节点和 `remediate -> inspect` 回边。

限制：

- DevOps `Init` 使用进程级全局编译回调，初始化应集中在入口，不能在每个请求中重复调用。
- DevOps `v0.1.9` 没有独立关闭 HTTP 服务的公开 API。
- Eino Dev 展示的是运行时编译拓扑，不是源码编辑器，也不提供 Go 源码双向同步。
- HTTP 与画布协议已自动验证；GoLand 内的画布布局和交互需要开发者在插件窗口中完成最终人工确认。

## 参考资料

- [CloudWeGo：Eino Dev 可视化调试插件功能指南](https://www.cloudwego.io/zh/docs/eino/core_modules/devops/visual_debug_plugin_guide/)
- [CloudWeGo EinoExt DevOps](https://github.com/cloudwego/eino-ext/tree/main/devops)
- 本项目 `examples/compose-quality-gate/main.go`
- 本项目 `examples/compose-quality-gate/gate.go`
- 本项目 `examples/compose-quality-gate/topology.go`
