# 11. 可复用线性 Graph 构建模板

前置知识见 [09. Go SDK API 设计](../09-sdk-api-design/README.md)和 [10. Go 框架源码设计](../10-framework-source-design/README.md)。

## 学习目标

本示例解决一个具体的代码组织问题：多个线性 Graph 都会重复创建 Graph、注册节点、连接 Edge 和 Compile；业务只想维护“有哪些中间步骤”。

推荐结构是：

```text
固定公共构建器：compileLinearGraph[I, M, O]
                       |
                       v
稳定业务入口：NewReviewGraph
                       |
                       v
可变中间方法：reviewSteps()
```

新增线性节点时，只修改 `reviewSteps()`；`compileLinearGraph`、`NewReviewGraph` 和 `main` 保持不动。

## 三种类型

公共构建器使用三个泛型类型：

```go
compileLinearGraph[I, M, O](...)
```

| 类型 | 本示例 | 责任 |
|---|---|---|
| `I` | `ReviewRequest` | Graph 对外输入 |
| `M` | `reviewContext` | 所有中间节点共用的数据 |
| `O` | `ReviewResult` | Graph 对外输出 |

实际类型链：

```text
START(ReviewRequest)
  -> input_adapter: ReviewRequest -> reviewContext
  -> normalize: reviewContext -> reviewContext
  -> append_channel_notice: reviewContext -> reviewContext
  -> inspect_refund_notice: reviewContext -> reviewContext
  -> output_adapter: reviewContext -> ReviewResult
  -> END(ReviewResult)
```

`I -> M` 和 `M -> O` 由固定边界适配器完成。因为每个中间节点都是 `M -> M`，公共构建器才能遍历步骤切片并自动连边。

## 固定公共构建器

[`builder.go`](builder.go) 中的 `compileLinearGraph` 固定负责：

1. 校验 context、Graph 名称、边界函数、节点 key 和 Handler。
2. 调用 `compose.NewGraph[I, O]`。
3. 注册输入适配节点。
4. 按 `steps` 顺序注册所有中间节点。
5. 注册输出适配节点。
6. 自动连接 `START -> 全部节点 -> END`。
7. Compile 并返回 `compose.Runnable[I, O]`。

它不认识 `ReviewRequest`、退款规则或评分逻辑，因此增加普通业务步骤时无需修改。

## 唯一经常修改的中间方法

[`review.go`](review.go) 中的 `reviewSteps()` 同时声明节点行为和顺序。当前注册表依次包含 `normalize`、`append_channel_notice` 和 `inspect_refund_notice`，三个 Handler 的完整实现都直接写在该方法内。

本示例为了让“只改一个方法”可直接观察，把 Handler 函数体内联在 `reviewSteps()` 中。本次实际新增的线性节点只是在切片中插入下面一项：

```go
{
    Key: "append_channel_notice",
    Run: func(ctx context.Context, current reviewContext) (reviewContext, error) {
        if err := ctx.Err(); err != nil {
            return reviewContext{}, fmt.Errorf("追加渠道说明: %w", err)
        }
        current.content += " 请关注原支付渠道。"
        current.steps = append(current.steps, "append_channel_notice")
        return current, nil
    },
},
```

公共构建器自动得到新拓扑，`builder.go`、`NewReviewGraph` 和 `main.go` 均未修改：

```text
normalize -> append_channel_notice -> inspect_refund_notice
```

生产代码中 Handler 变长后，应把函数体提取成命名函数，但 `reviewSteps()` 仍是唯一需要修改的节点注册表。

## 运行与验证

仓库声明 Go `1.26.0`，当前验证运行时为 Go `1.26.3`，Eino 锁定为 `v0.9.12`。示例不访问网络，也不需要凭据。

在仓库根目录执行：

```bash
go run ./examples/go-study/framework-api-design/11-linear-graph-template
go test ./examples/go-study/framework-api-design/11-linear-graph-template -count=1
go test -race ./examples/go-study/framework-api-design/11-linear-graph-template -count=1
```

预期输出：

```text
approved=true score=9 content="您好，退款将在 3 个工作日到账。 请关注原支付渠道。" steps=[normalize append_channel_notice inspect_refund_notice] reasons=[包含退款到账说明]
```

测试覆盖正常路径、步骤顺序、零中间节点、业务错误、context 取消、步骤注册表扩展，以及空名称、重复 key、保留 key、nil Handler 和 nil Option。

## 适用边界

- `已验证`：只调整 `steps` 切片即可改变线性节点数量和顺序，公共构建器无需修改。
- `建议`：多个 Graph 都符合 `I -> M -> ... -> M -> O` 时，可以复用该模板。
- `不适用`：中间节点输入输出类型不同、存在 Branch、并行、循环或字段映射时，不应强行套用该模板，应直接使用 Eino Graph API 或设计更匹配的构图函数。
- `必须修改边界时`：如果 `ReviewRequest`、`ReviewResult`、Local State 或 Compile 策略发生变化，`NewReviewGraph` 仍可能需要调整。“公共方法不动”只针对新增普通线性中间节点，不是绝对承诺。
