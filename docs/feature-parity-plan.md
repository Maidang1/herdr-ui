# 功能对齐清单：从 Herdr（herdr-gpui）与 Kero 借鉴

调研方式：computer-use 实机操作 Herdr（so.pen.herdr-gpui）与两份源码
（/Users/bytedance/codes/open-source/kero，Go/mygo，同框架同域；
/Users/bytedance/codes/open-source/herdr-gpui，Rust/GPUI，Herdr 官方 GUI）。
产品定位不变：**围绕 herdr 的终端工具**。与终端无关的能力一律不搬。

## 已实现

| 功能 | 来源 | 状态 |
| --- | --- | --- |
| 侧栏信息结构（过滤框、状态环、端口条、agents 分组/优先级切换） | Herdr 侧栏 + Kero 侧栏 | 已上线 |
| 顶部标签条（pills、状态环、Tab actions 菜单） | Herdr 标签栏 | 已上线 |
| Prefix 和弦键位标签（Ctrl + B 小标签） | Herdr prefix 提示 | 已上线 |
| 主题 System/Light/Dark（原生菜单） | 两者共有 | 已上线；本批次补持久化 |
| 终端字号 ⌘=/⌘-/⌘0（8–48pt 夹取、热更新、持久化） | Herdr `window/commands.rs:455`；Kero `app.go:786` | 本批次 |
| 外观偏好持久化（~/Library/Application Support/herdr-ui/prefs.json） | Kero 设置持久化 | 本批次 |
| ⌘J 跳到需要你的 agent | Kero `session.attention` | 本批次 |
| ⌘B 切换侧栏（直键） | 两者共有 | 本批次 |
| Agent 通知 + Dock 徽章（blocked/done，点击跳转，激活清空） | Kero `app.go:319-388`；herdr-gpui `notifications.rs`/`app_badge.rs` | 本批次 |
| 终端内查找条（⌘F，实时计数，Enter/⇧Enter 导航） | Kero `find.go`；herdr-gpui `find.rs` | 本批次 |
| 未聚焦终端暗化 | Kero `panes.go` veil | 本批次 |

## 明确排除（与终端工具定位无关）

- GitHub 登录/PR/Issues/评论（herdr-gpui `src/github/`、`src/pull_request/`）
- AI 额度/用量面板（herdr-gpui `src/usage/`；Herdr 状态栏 usage chips）
- 内置浏览器标签与页面标注（kero `browser.go`；herdr-gpui `src/browser/`）
- Teleport/云设备/远程主机管理（herdr-gpui `src/cloud*/`；kero `internal/remote`）
- Worktree checkpoint/Review diff 审查、Orchestrator、Smart Dispatch（herdr-gpui）
- Agent 技能安装、程序状态上报 OSC 7501（kero `internal/cli` skills/report）
- VS Code/px0 内嵌编辑器（herdr-gpui `src/code_server/`）
- 应用自更新、崩溃报告面板（发布工程，非终端功能）

## 后续候选（按性价比，未实现）

1. 命令面板升级：现有 navigator 扩展为 frecency + 命令混合搜索（herdr-gpui `palette.rs`、kero `palette.go`），⌘P/⌘⇧P 与双击 Shift。
2. 链接与路径处理：⌘+点击打开 URL/文件路径，⌘+悬停图片 Flyout 预览（kero `peek.go`；herdr-gpui `window/links.rs` 的三态验证）。
3. ⌘K 清远端屏（herdr 语义 pane.clear）。
4. Zen 模式（隐藏全部 chrome + 可选居中列宽，herdr-gpui `window/zen.rs`）。
5. 快捷键重绑定界面（kero Settings›Keyboard 的录制/冲突检测）。
6. 终端配色主题（Ghostty 主题名，light/dark 双套，kero `terminal.GhosttyTheme`）。
7. 设置窗口（分页 + 搜索 + 配置热加载，kero `settings_view.go` 整体范式）。
8. 标签拖拽重排/拖出成窗（herdr-gpui `tab_drag.rs`）。
