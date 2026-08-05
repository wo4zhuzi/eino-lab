# Go 框架 API 设计学习路线

## 总体脉络

```text
Go 函数类型
  -> 函数作为参数
  -> 高阶函数
  -> 回调函数
  -> Functional Options
  -> 控制反转 IoC
  -> 生命周期钩子
  -> 中间件设计
  -> SDK API 设计
  -> 框架源码设计
  -> 可复用线性 Graph 构建模板
  -> 带 Branch 的 Graph 构建模板
  -> 使用 Chain 编写流水式 Graph
  -> 生产型 Chain 与 Graph 组合
  -> 多工作流与公共运行层
  -> 可治理的多工作流运行层
  -> RAG 文档索引工作流：真实解析与完整骨架
```

这些不是互相独立的主题。前四项是 Go 语言能力，中间四项是框架常用机制，最后九项是综合与工程应用。

## 第一阶段：函数可以被保存和传递

### 1. Go 函数类型

学习目标：理解函数也有类型，可以赋值给变量或结构体字段。

```go
type Handler func(string) (string, error)

func validate(input string) (string, error) {
    return input, nil
}

var handler Handler = validate
```

通过标准：能说明 `Handler` 是函数类型，`handler` 保存了函数 `validate`。

可运行示例：[01-function-type](01-function-type/README.md)。

推荐搜索：

```text
Go 函数类型
Go 自定义函数类型
```

### 2. Go 函数作为参数

学习目标：把一个函数交给另一个函数使用。

```go
func execute(input string, handler Handler) (string, error) {
    return handler(input)
}
```

通过标准：能区分 `execute("x", validate)` 与 `validate("x")`。

可运行示例：[02-function-as-parameter](02-function-as-parameter/README.md)。

推荐搜索：

```text
Go 函数作为参数
Go 函数传参
```

### 3. Go 高阶函数

学习目标：理解“接收函数”或“返回函数”的函数。

```go
func withPrefix(prefix string) Handler {
    return func(input string) (string, error) {
        return prefix + input, nil
    }
}
```

通过标准：能指出 `withPrefix` 返回的是函数，不是最终字符串。

可运行示例：[03-higher-order-function](03-higher-order-function/README.md)。

推荐搜索：

```text
Go 高阶函数
Go 返回函数
Go 闭包
```

### 4. Go 回调函数

学习目标：理解“现在登记函数，稍后由调用方执行”。

```go
func register(handler Handler) {
    // 保存 handler，等事件发生后调用。
}

register(validate)
```

通过标准：能够分别找到回调的注册位置和真正调用位置。

可运行示例：[04-callback-function](04-callback-function/README.md)。

推荐搜索：

```text
Go 回调函数
Go callback
```

## 第二阶段：把函数用于配置和流程控制

### 5. Functional Options 函数式选项

依赖知识：函数类型、高阶函数、闭包。

学习目标：理解 `With...` 如何返回一个修改配置的函数。

```go
type Config struct {
    timeout int
}

type Option func(*Config)

func WithTimeout(timeout int) Option {
    return func(config *Config) {
        config.timeout = timeout
    }
}
```

通过标准：能自己为已有 `Config` 增加一个简单的 `WithName`，并找到配置最终被读取的位置。

推荐搜索：

```text
Go Functional Options
Go 函数式选项模式
Go Option 模式
```

可运行示例：[05-functional-options](05-functional-options/README.md)。

### 6. 控制反转 IoC

依赖知识：回调函数。

学习目标：理解业务代码只登记行为，框架控制调用时机和参数。

```go
register(validate) // 业务登记
handler(input)     // 框架调用
```

通过标准：看到 `WithStatePostHandler(saveQuestion)` 时，能判断 Eino 是调用方，`saveQuestion` 是被登记的行为。

推荐搜索：

```text
Go 控制反转 IoC
Go 回调 控制反转
```

可运行示例：[06-inversion-of-control](06-inversion-of-control/README.md)。

### 7. 生命周期钩子

依赖知识：回调函数、控制反转。

学习目标：理解框架在固定时间点开放扩展位置。

```text
Pre / Before：主要逻辑之前
Post / After：主要逻辑之后
Error：主要逻辑出错时
```

通过标准：能根据名称判断 `PreHandler`、`PostHandler`、`ErrorHandler` 的执行时机。

可运行示例：[07-lifecycle-hooks](07-lifecycle-hooks/README.md)。

推荐搜索：

```text
Go 生命周期钩子
Go hooks
Go before after handler
```

### 8. Go 中间件设计

依赖知识：高阶函数、回调函数、控制反转。

学习目标：理解如何在不修改主要逻辑的情况下增加日志、鉴权、超时或恢复处理。

```go
type Middleware func(Handler) Handler
```

通过标准：能说明中间件为什么同时接收 Handler 并返回 Handler，以及多个中间件的包裹顺序。

可运行示例：[08-middleware](08-middleware/README.md)。

推荐搜索：

```text
Go 中间件设计
Go middleware 原理
Gin middleware 原理
gRPC interceptor 原理
```

## 第三阶段：综合设计

### 9. Go SDK API 设计

组合知识：配置结构体、Functional Options、依赖注入、接口、回调、错误处理、`context.Context`。

学习目标：设计稳定、可扩展且不泄漏内部实现的公开 API。

可运行示例：[09-sdk-api-design](09-sdk-api-design/README.md)。

重点问题：

```text
哪些配置是必填参数？
哪些配置适合 With...？
哪些扩展使用接口？
哪些扩展使用函数回调？
Logger 等外部能力如何通过构造函数注入？
错误和 context 如何向调用方传播？
```

推荐搜索：

```text
Go SDK API 设计
Go library API design
Go 公共库设计
```

### 10. Go 框架源码设计

组合知识：SDK API 设计、控制反转、生命周期、调度、并发和状态管理。

学习目标：沿一条真实运行链路找到配置注册、编译、运行、回调和错误传播位置。

可运行示例：[10-framework-source-design](10-framework-source-design/README.md)。

阅读框架源码时固定追踪：

```text
用户在哪里注册配置？
配置保存在哪里？
什么时候创建运行状态？
框架在哪里调用 Handler？
错误如何返回用户入口？
```

推荐搜索：

```text
Go 框架源码设计
Go 框架源码解析
gRPC Go 源码 Option
Gin middleware 源码
```

### 11. 可复用线性 Graph 构建模板

组合知识：泛型、函数作为参数、SDK API 设计、Graph 输入输出边界和 Compile 生命周期。

学习目标：把线性 Graph 的创建、节点注册、Edge 连接和 Compile 固定为公共构建器，只通过有序步骤清单扩展业务节点。

可运行示例：[11-linear-graph-template](11-linear-graph-template/README.md)。

通过标准：新增一个 `M -> M` 线性节点时，只修改业务步骤注册表，不修改公共构建器、业务构建入口和调用入口。

### 12. 带 Branch 的 Graph 构建模板

组合知识：Graph 输入输出边界、节点注册、固定 Edge、Branch 条件函数和 Compile 生命周期。

学习目标：在线性公共流程后增加二选一分支，区分注册 Branch 与运行期选择目标节点，并理解固定 Edge 不会被 Branch 覆盖。

可运行示例：[12-branch-graph-template](12-branch-graph-template/README.md)。

通过标准：能说明 `AddBranch` 的起点、条件函数和目标白名单分别负责什么，并能新增一个分支目标而不修改公共编译方法。

### 13. 使用 Chain 编写流水式 Graph

组合知识：Chain Builder、Lambda、子 Chain、嵌套 Branch 和 Graph/Chain 选型。

学习目标：把以顺序步骤为主的业务组织成按代码书写顺序执行的流水线，通过命名子 Chain 隔离每条分支，不再手工维护节点表和 Edge 表。

可运行示例：[13-chain-pipeline-template](13-chain-pipeline-template/README.md)。

通过标准：新增普通节点时，只在对应路径追加一个 `AppendLambda`；新增 Branch 时，只声明条件、目标路径和插入位置。

### 14. 生产型 Chain 与 Graph 组合

组合知识：应用门面、依赖注入、配置校验、Chain Builder、Graph 任意连边、Branch、循环保护和组合节点的类型边界。

学习目标：通过稳定 `Workflow` 门面隔离 Eino，在启动阶段注入配置和依赖并编译一次；用外层 Chain 表达固定阶段，把需要回环的局部决策封装为子 Graph。

可运行示例：[14-chain-with-graph](14-chain-with-graph/README.md)。

通过标准：新增普通节点时只修改 Handler 和外层拓扑，新增复杂流程时只增加独立子 Graph；应用调用入口不依赖 Eino API。

### 15. 多工作流与公共运行层

组合知识：泛型接口、Compile/Invoke 生命周期、多个业务 Workflow、依赖注入和最小公共抽取。

学习目标：在一个应用中启动审核与 RAG 两个独立工作流，只共享与业务无关的编译和运行生命周期，不抽象节点 DSL。

可运行示例：[15-multiple-workflows](15-multiple-workflows/README.md)。

通过标准：新增第三个工作流时复用 `workflowkit.Compile/Runner`，但保留自己的 Config、Dependencies、输入输出和拓扑。

### 16. 可治理的多工作流运行层

组合知识：多工作流公共运行层、Functional Options、Callback、稳定身份、结构化错误、并发观测和运行级保护。

学习目标：在不抽象业务拓扑 DSL 的前提下，为公共运行层增加工作流版本、RunID、Observer、节点名称、结构化错误和请求级最大步数。

可运行示例：[16-governable-workflow-runtime](16-governable-workflow-runtime/README.md)。

通过标准：每次执行都有稳定工作流身份和 RunID；Observer 能识别关键节点；运行错误可读取治理上下文并通过 `errors.Is` 找回原始错误。

## 主题之间的关系

| 后续主题 | 依赖的前置知识 |
|---|---|
| 函数作为参数 | 函数类型 |
| 高阶函数 | 函数作为参数、函数返回值 |
| 回调函数 | 函数作为参数 |
| Functional Options | 高阶函数、闭包、配置结构体 |
| 控制反转 | 回调函数 |
| 生命周期钩子 | 回调函数、控制反转 |
| 中间件 | 高阶函数、回调、控制反转 |
| SDK API 设计 | Functional Options、接口、错误与 context |
| 框架源码设计 | 上述全部内容 |
| 线性 Graph 构建模板 | 泛型、函数作为参数、SDK API 设计、Graph 源码设计 |
| Branch Graph 构建模板 | 线性 Graph、声明式拓扑、条件函数、运行期调度 |
| Chain 流水式 Graph | Branch Graph、Builder、子流程和 Graph/Chain 选型 |
| 生产型 Chain 与 Graph 组合 | Chain、Graph 任意连边、依赖注入、配置校验、循环保护、组合边界 |
| 多工作流与公共运行层 | 生产型工作流、泛型、Compile/Invoke 生命周期、最小公共抽取 |
| 可治理的多工作流运行层 | 多工作流公共层、Functional Options、Callback、错误链、并发安全 |

## 推荐练习顺序

每个主题只做一个小练习，通过后再进入下一项：

1. 定义 `Handler` 函数类型并为它赋值。
2. 写一个接收 `Handler` 的 `execute`。
3. 写一个返回 `Handler` 的 `withPrefix`。
4. 分开实现 `register` 和 `run`，观察回调何时执行。
5. 为 `Config` 实现 `WithName` 和 `WithTimeout`。
6. 让框架而不是 `main` 调用已登记的 Handler。
7. 增加 Pre、Post 和 Error 三个执行时机。
8. 实现一个 `Middleware func(Handler) Handler`。
9. 把上述能力组合成一个最小节点 SDK。
10. 回到 Eino，追踪 `WithStatePostHandler` 的注册和运行链路。
11. 把线性 Graph 的固定构建流程与可变业务步骤清单分离。
12. 把固定 Edge 与 Branch 分开注册，观察两次 Invoke 选择不同目标节点。
13. 使用 Chain 重写同一流程，对比手工 Edge 与流水式 Builder 的维护成本。
14. 用 Workflow 门面封装 Eino，把一个需要回环的局部流程作为 Graph 放入外层 Chain，并注入可替换依赖。
15. 同时实现审核和 RAG 工作流，仅抽取两者重复的编译与运行生命周期。
16. 为公共运行层增加版本、RunID、Observer、结构化错误和请求级运行保护。

不要同时学习多个主题。每一步都回答：函数在哪里定义、在哪里保存、在哪里调用、由谁传入参数。
