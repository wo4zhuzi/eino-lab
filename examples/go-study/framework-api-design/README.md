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
```

这不是十个互相独立的主题。前四项是 Go 语言能力，中间四项是框架常用机制，最后两项是综合应用。

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

对应仓库示例：[callback-state-injection/main.go](callback-state-injection/main.go)。

### 7. 生命周期钩子

依赖知识：回调函数、控制反转。

学习目标：理解框架在固定时间点开放扩展位置。

```text
Pre / Before：主要逻辑之前
Post / After：主要逻辑之后
Error：主要逻辑出错时
```

通过标准：能根据名称判断 `PreHandler`、`PostHandler`、`ErrorHandler` 的执行时机。

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

推荐搜索：

```text
Go 中间件设计
Go middleware 原理
Gin middleware 原理
gRPC interceptor 原理
```

## 第三阶段：综合设计

### 9. Go SDK API 设计

组合知识：配置结构体、Functional Options、接口、回调、错误处理、`context.Context`。

学习目标：设计稳定、可扩展且不泄漏内部实现的公开 API。

重点问题：

```text
哪些配置是必填参数？
哪些配置适合 With...？
哪些扩展使用接口？
哪些扩展使用函数回调？
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

不要同时学习多个主题。每一步都回答：函数在哪里定义、在哪里保存、在哪里调用、由谁传入参数。
