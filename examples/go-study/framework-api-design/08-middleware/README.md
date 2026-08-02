# 08. Go 中间件设计

前置知识见 [03. 高阶函数](../03-higher-order-function/README.md)、[06. 控制反转 IoC](../06-inversion-of-control/README.md)和 [07. 生命周期钩子](../07-lifecycle-hooks/README.md)。

## 学习目标

本示例解释下面这个签名为什么既接收又返回 `Handler`：

```go
type Middleware func(Handler) Handler
```

需要理解三个问题：

1. 中间件如何在不修改核心 Handler 的情况下增加日志、鉴权和取消检查。
2. 多个中间件为什么形成“先进后出”的洋葱执行顺序。
3. 中间件如何通过不调用 `next` 来短路后续处理。

本课直接在日志中间件中调用 `fmt.Println`，不引入日志依赖注入。可替换 Logger、接口与构造函数注入将在第 9 课最小 SDK 中统一学习。

## 为什么接收 Handler

传入的 `Handler` 是链中的下一个处理器。中间件需要调用它，才能把请求继续交给下游：

```go
func loggingMiddleware(next Handler) Handler {
    // ...
}
```

如果不接收 `next`，中间件只能执行自己的逻辑，无法包裹核心业务，也无法在下游返回后执行退出逻辑。

## 为什么返回 Handler

中间件返回一个签名完全相同的新 `Handler`。新 Handler 可以在调用 `next` 前后增加行为：

```go
return func(ctx context.Context, input string) (string, error) {
    fmt.Println("进入")
    output, err := next(ctx, input)
    fmt.Println("退出")
    return output, err
}
```

因为返回值仍然是 `Handler`，它还能继续传给外层中间件。每个中间件只包装相邻的下一层，不需要知道整条链有多长。

## 构建顺序与运行顺序

调用方按阅读顺序注册：

```go
handler := Chain(final, first, second)
```

`Chain` 必须从后向前包装：

```text
构建：second(final) -> first(second(final))

运行：
first before
  -> second before
    -> final
  <- second after
<- first after
```

如果构建时正序包装，最后注册的中间件会变成最外层，与常见 Web 框架的注册直觉相反。

顺序也决定短路优先级。主示例把 context 检查放在鉴权外层，因此已取消且未认证的请求优先返回取消错误，避免继续执行鉴权。不同系统可以选择其他顺序，但必须通过组合测试固定语义。

## 短路与错误传播

鉴权中间件只有成功时才调用 `next`：

```go
if principal == "" {
    return "", errUnauthorized
}
return next(ctx, input)
```

因此鉴权失败后，内层中间件和核心 Handler 都不会执行。日志中间件使用 `%w` 包装下游错误，调用方仍可用 `errors.Is` 判断原始业务错误；它同时保留下游输出，避免纯观测中间件改变部分结果语义。

## 与生命周期钩子的区别

| 能力 | 生命周期钩子 | 中间件 |
|---|---|---|
| 调用时点 | 框架固定 Before、After、Error | 中间件自行决定 |
| 是否必须调用核心逻辑 | 由 Runner 固定调用 | 可以不调用 `next` |
| 包装范围 | 某一个预留时点 | 整个下游 Handler |
| 多层组合 | 通常由框架定义排序 | 天然形成嵌套调用链 |
| 典型用途 | 生命周期观测、固定扩展点 | 日志、鉴权、限流、超时、恢复 |

中间件能力更强，但也更容易因忘记调用 `next`、调用多次或错误处理不当而改变业务语义。只需要固定观测点时，生命周期钩子通常更容易约束。

## 运行与验证

本示例只使用 Go 标准库。仓库声明 Go `1.26.0`，不需要外部服务。

在仓库根目录执行：

```bash
go run ./examples/go-study/framework-api-design/08-middleware
go test ./examples/go-study/framework-api-design/08-middleware -count=1
```

预期输出：

```text
日志：进入
日志：退出
回答：中间件为什么返回 Handler？
日志：进入
日志：失败
失败结果：日志中间件观察到下游失败: 未通过鉴权
```

测试覆盖洋葱顺序、鉴权短路、下游输出与错误链、`context` 取消、取消与鉴权的组合优先级，以及无中间件边界。

## 已知限制

- 示例同步执行，不包含 panic 恢复、指标统计和超时派生。
- 身份信息仅用于演示，通过私有 context key 传递，不代表完整认证方案。
- `Chain` 假定 `final` 和每个 Middleware 都非 nil；生产 SDK 应在构造阶段校验。
