# M0 实施计划

目标：按主文档 §10 验证 Go + mygo 终端与 Herdr direct attach 的实际边界。
本阶段只提供能力探针，不实现 M1 的订阅状态窗口或 M3 的业务动作。

- [x] 固定 mygo 完整提交、Go 与 CLI 基线；CGO_ENABLED=0 构建。
- [x] 接入目标解析、有限 NDJSON 请求与只读探针：ping、snapshot、layout.export。
- [x] 最小 mygo 原生终端窗口：显式 attach command、统一 socket、异步初始化与退出清理、公开输入钩子。
- [x] 纯 Go 契约测试：socket 优先级、attach 转义、键位解析诊断、布局 false/true 路径。
- [x] 隔离真实 Herdr 的 stream、owner、尺寸、退出存活、Ctrl-B/q 和嵌套布局路径验证；真实窗口 View 尺寸同步与关闭验证。
- [ ] 视觉首屏、IME、鼠标/滚动、持续输出与大粘贴真实窗口验收；完整 resolver 对照与模式规则。
- [x] 更新主文档和运行说明，明确通过、阻塞和人工待验项。

验收证据保存在 docs/m0-validation.md；未知能力不以源码检查或 fake 冒充实测。
