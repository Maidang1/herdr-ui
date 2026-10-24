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

## 第三批（已实现）

| 功能 | 来源 | 说明 |
| --- | --- | --- |
| 命令面板升级 | herdr-gpui `palette.rs`；kero `palette.go` | goto 面板混合会话目的地与全部命令（含动态标签与键位提示），frecency 排序（使用计数 + 会话衰减），选择即执行 |
| 链接与路径 ⌘+点击 | kero `app.go openLink` | 终端插件自带 OSC 8/URL/路径检测与相对路径解析；URL 走系统，路径经 Shell.OpenPath 打开 |
| ⌘K 清屏 | herdr 语义 pane.clear | 走 Shortcut 操作清服务端屏幕，避免本地清被下一帧覆盖 |
| Zen 模式 ⌃⌘Z | herdr-gpui `window/zen.rs` | 窗口级视图状态：隐藏侧栏/标签条/参考面板/标题条，红绿灯浮于终端；不影响服务端焦点与侧栏偏好 |

## 第四批（已实现）

| 功能 | 来源 | 说明 |
| --- | --- | --- |
| 设置窗口 ⌘, | kero `settings_view.go`；herdr 设置窗口范式 | 独立可复用窗口（StateKey 记忆几何），左侧页面导航 + 右侧卡片：Appearance（System/Light/Dark 分段）、Terminal（字号/字重/行高滑杆与分段、等宽预览、选中即复制、最小对比度）、Notifications（通知与角标总开关）、Shortcuts（键位来源说明 + 编辑 Herdr 配置入口 + 动作清单）。改动即时生效并落盘 |
| 终端行为热更 | kero `app.go:812-842` | CopyOnSelect / MinimumContrast / 字体全量经 Host 热更到所有已附加终端，并为新 attach 记住 |

## 后续候选（暂缓，含原因）

1. ⌘+悬停图片 Flyout 预览（kero `peek.go`）：需 Flyout 窗口与图片解码管线，收益场景较窄。
2. 快捷键重绑定界面：prefix 层键位本就由 Herdr 自己的 config.toml 管理（`input.LoadBindings`）；直键（⌘B/⌘F 等）固定，与 macOS 惯例一致。
3. 终端配色主题（Ghostty 主题名）：锁定的 mygo v0.6.3-dev 未导出 `terminal.GhosttyTheme`（kero 用的 v0.6.4 才有）；升级框架属独立决策。

4. 标签拖拽重排（herdr 服务端有 tab.move）：pills 交互重排属打磨项。
