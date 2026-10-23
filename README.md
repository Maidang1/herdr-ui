# Herdr UI

Herdr 的原生桌面客户端，第一版面向本机 macOS，使用 **Go + mygo 原生 UI与内置终端插件**。展示已有工作区、工作树、代理状态与终端，支持标签、分屏和 Herdr prefix 模式。

目前仓库只有规格、设计稿和素材，尚无可运行的应用代码。

- [设计与实现方案](docs/implementation.md)：唯一主文档，包含第一版范围、界面、架构、接口、事件同步、终端、快捷键、颜色 token 与验收步骤。
- [第一版设计稿](docs.pen/new.pen)：Workspaces、Split Panes、Switch Tabs、Shortcuts Reference 四张画板，使用 Pencil 打开。
- [API schema](reference/herdr-api-schema.json)：机器可读参考，接入时以目标 Herdr 实际版本为准。
- [图标素材与授权](assets/icons/NOTICE.md)。

终端直接使用 `github.com/egoist/mygo/plugins/terminal`，通过 `herdr terminal attach <terminal_id>` 连接已有会话；状态从第一版起使用 `events.subscribe`，由事件触发权威快照更新，不定时轮询。

通用图标采用 `lucide-static`，agent 品牌图标采用 `@lobehub/icons-static-svg`：开发时按需导出并嵌入 SVG，由 mygo 显示，运行时无需 Node/npm。单色使用 `ui.Icon`，彩色使用 `ui.Image`。

`docs.pen/01.pen` 保留为历史设计源素材。第一版范围、颜色和实现决策统一以主文档及 `docs.pen/new.pen` 为准。实现从主文档中的 M0 接入验证开始。
