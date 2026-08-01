# Go 原生学习示例

本目录收纳用于理解 Eino 示例中 Go 语言写法的最小实验。每个主题使用独立子目录，默认只依赖 Go 标准库，避免框架 API 干扰语言概念理解。

建议先阅读 [Go 框架 API 设计学习路线](framework-api-design/README.md)，再按顺序完成对应示例。后续相关练习统一放入 `framework-api-design/`。

## 当前主题

- [框架 API 设计 01：Go 函数类型](framework-api-design/01-function-type/README.md)：理解如何定义函数类型，以及如何把具体函数保存到变量和结构体字段中。
- [框架 API 设计 02：函数作为参数](framework-api-design/02-function-as-parameter/README.md)：把变化的风控规则交给固定订单检查流程，避免重复公共校验和错误处理。
- [框架 API 设计 03：高阶函数](framework-api-design/03-higher-order-function/README.md)：根据不同额度生成风控函数，理解返回函数与闭包保存配置的作用。
- [框架 API 设计 04：回调函数](framework-api-design/04-callback-function/README.md)：分开注册与执行位置，理解事件循环如何在事件发生后调用业务函数。
- [框架 API 设计 05：Functional Options](framework-api-design/05-functional-options/README.md)：用默认模型客户端配置与按需覆盖，理解 `With...` 选项解决的位置参数和 API 演进问题。
- [状态工厂函数](local-state-factory/README.md)：对比共享状态对象与按次创建状态，理解函数作为参数和运行状态隔离。
- [框架 API 设计 06：控制反转 IoC](framework-api-design/06-inversion-of-control/README.md)：用迷你 Graph 展开框架调用 PostHandler 并传入 Local State 的过程。
- [`With...` 配置命名示例](with-option-naming/main.go)：通过可运行的 Go 代码和注释理解 `With`、`State`、`Pre/Post`、`Handler` 的含义。
