# Go 原生学习示例

本目录收纳用于理解 Eino 示例中 Go 语言写法的最小实验。每个主题使用独立子目录，默认只依赖 Go 标准库，避免框架 API 干扰语言概念理解。

## 当前主题

- [状态工厂函数](local-state-factory/README.md)：对比共享状态对象与按次创建状态，理解函数作为参数和运行状态隔离。
- [回调状态注入](callback-state-injection/README.md)：用迷你 Graph 展开框架调用 PostHandler 并传入 Local State 的过程。
- [`With...` 配置命名示例](with-option-naming/main.go)：通过可运行的 Go 代码和注释理解 `With`、`State`、`Pre/Post`、`Handler` 的含义。
