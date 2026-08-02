# 05. Functional Options 函数式选项

## 这个案例解决什么问题

创建模型客户端时，服务地址是必填参数，超时和重试次数通常是可选参数。如果全部使用位置参数：

```go
NewModelClient("https://model.example.com", 30*time.Second, 5)
```

调用处看不出 `30*time.Second` 和 `5` 分别代表什么。新增可选配置还会迫使所有调用方修改函数参数。

Functional Options 保留少量必填参数，把可选配置写成有名称的选项：

```go
NewModelClient(
    "https://model.example.com",
    WithTimeout(30*time.Second),
    WithMaxRetries(5),
)
```

## 完整运行过程

构造函数先设置默认值：

```go
config := clientConfig{
    timeout:    10 * time.Second,
    maxRetries: 2,
}
```

`WithTimeout` 不会立即修改客户端。它返回一个持有 `timeout` 的函数：

```go
func WithTimeout(timeout time.Duration) Option {
    return func(config *clientConfig) error {
        config.timeout = timeout
        return nil
    }
}
```

`NewModelClient` 收到选项后，依次把它们应用到同一个配置对象：

```go
for _, option := range options {
    if err := option(&config); err != nil {
        return nil, err
    }
}
```

因此完整链路是：

```text
WithTimeout(30s)
  -> 返回一个 Option 函数，函数记住 30s
  -> NewModelClient 创建默认配置
  -> NewModelClient 调用 Option(&config)
  -> Option 把 timeout 改为 30s
  -> 使用最终配置创建客户端
```

这里使用了高阶函数和闭包，但它不是事件回调：Option 会在当前构造函数调用期间执行，不会等待未来事件。

## 适用场景

推荐用于以下情况：

- 构造对象有稳定的默认值。
- 只有少数参数必填，其余大多可选。
- 希望调用代码能通过 `WithTimeout` 等名称表达含义。
- 公共库需要在不破坏旧调用代码的情况下增加可选配置。
- 不希望调用方直接依赖内部配置结构。

不推荐用于以下情况：

- 参数很少且全部必填：直接使用普通参数更清楚。
- 配置需要从 YAML、JSON 或环境变量统一加载：公开配置结构体通常更合适。
- 业务需要频繁读取和修改完整配置：Option 会使配置内容不够直观。

## 与 Eino 的关系

Eino 的调用 API 广泛使用同类写法，例如：

```go
runner.Query(ctx, query, adk.WithCallbacks(handler))
```

`query` 是本次调用的必填输入，Callback 是可选能力。将来增加其他运行选项时，不需要改变已有调用方的位置参数。

## 运行

本示例只使用 Go 标准库。仓库使用 Go `1.26.0`。

在仓库根目录执行：

```bash
go run ./examples/go-study/framework-api-design/05-functional-options
go test ./examples/go-study/framework-api-design/05-functional-options
```

预期输出：

```text
default timeout=10s retries=2
batch timeout=30s retries=5
```

测试覆盖默认值、选项覆盖、非法超时与重试次数、空白服务地址、nil Option 和错误链。
