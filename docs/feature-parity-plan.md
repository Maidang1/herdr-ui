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
- Teleport/云设备管理及原生多机器侧栏（herdr-gpui `src/cloud*/`）；本地保存的 SSH 机器与交互式连接入口见下方新增管理能力。
- Worktree checkpoint/Review diff 审查、Orchestrator、Smart Dispatch（herdr-gpui）
- Agent 技能安装、程序状态上报 OSC 7501（kero `internal/cli` skills/report）
- VS Code/px0 内嵌编辑器（herdr-gpui `src/code_server/`）
- 崩溃报告面板（发布工程）；应用自更新已由现有原生更新器提供。

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

## 第五批（已实现）

| 功能 | 来源 | 说明 |
| --- | --- | --- |
| 鼠标拖动选中文字 | Ghostty/Terminal 常规行为 | `herdr terminal attach` 总是开启鼠标上报（即使 pane 里是普通 shell），拖动原先被 Herdr 丢弃。补丁后拖出按下的格子即本地选区；原地单击仍完整交给程序 |
| Mac 直接快捷键 | Terminal/iTerm2/Ghostty；kero `actions.go` | ⌘T 新标签、⌘W 关 pane（agent 在跑时先确认；设置窗在前时关设置窗）、⌘D/⇧⌘D 分屏、⌘1–8/⌘9 选标签（⌘9 为最后一个）、⇧⌘[ ⇧⌘] 切标签、⌘[ ⌘] 切 pane、⌥⌘方向键按方向切 pane |
| 字体族、光标、Option 作 Meta | kero `settings_view.go` | 设置 → Terminal：已安装等宽字体列表、光标形状（块/竖线/下划线）与闪烁、Option 作 Meta（默认关）。全部热更到已附加终端并落盘 |
| 复制提示 | Ghostty `app-notifications = clipboard-copy` | ⌘C、右键复制、选中即复制后底部浮出「Copied」 |

Option 作 Meta 同时修正了 mygo 的缺陷：macOS 输入法在 Meta 键之后仍会插入 Option 字符（⌥B 得到 `ESC b∫`），⌥E 等死键还会开启组字。补丁让终端在 Option 按下且输入法未在组字时绕过输入法。

终端响铃不做：attach 流会吞掉 BEL、OSC 52 与 OSC 9（实测 `printf '\a'` 后只剩重绘），客户端收不到。

## Herdr 管理能力（2026-10）

- Pane 菜单增加可关闭的 Agent/process Details，展示 `agent.get`、`pane.process_info` 与 `agent.explain`；agent 名称走 `agent.rename`，不更改 pane label。诊断原文可复制，session reference 不代表恢复保证。
- Settings → Plugins：列表、enable/disable、actions、支持的 tab/split 入口与按需展开日志；GitHub install/update 在独立命令终端交互确认，popup/overlay 不当普通 pane 打开。
- Settings → Session restore：写 Herdr 配置的 `session.resume_agents_on_restore` 和 `experimental.pane_history`，保留未知配置/注释，原子替换并保留符号链接目标，不主动重启。
- 输出提醒：独立可取消连接执行 `pane.wait_for_output`，recent 80 行，substring/Rust regex，默认 30 分钟，最大 24 小时。匹配后通知一次，不自动跳转；终端/epoch 变化和退出取消。
- Settings → Machines：读取本地 typed 机器和命名会话列表；状态检查显式执行。保存机器管理、SSH setup/auth/open 和本地 session attach 使用独立交互终端，命令退出结果保留至关闭。不承诺原生远程侧栏；远程连接与 Windows 命令终端未实机验证。

明确不提供 agent cost/history。agent start 与 pane move 已在第六批按显式用户操作提供。

## 第六批：协议 22 剩余能力与纯净模式（已实现）

| 功能 | Herdr API | 说明 |
| --- | --- | --- |
| 工作区拖拽 / 上移下移 | `workspace.move` | 仅顶层行可拖（worktree 随父工作区）；筛选时不拖；insert_index 按移动前顺序计数，与 tab.move 一致 |
| 标签拖拽重排 | `tab.move` | pills 互相拖放 |
| Pane 移动 | `pane.move` | 新标签 / 新工作区 / 其他标签（右侧分屏） |
| Agent 视图 | `agent.view.set/clear` | All / Needs attention / Working / This workspace；本地筛选侧栏并以 source=hergo 共享给 Herdr，启动时重新下发 |
| 发送 prompt | `agent.prompt` | 仅在选中 pane 有 agent 时可用，不重试 |
| Agent 停止提醒 | `agent.wait` | 独立长连接，until=done/blocked/idle，1–1440 分钟；终端/epoch 变化即取消 |
| 启动 agent | `agent.start` | 仅无 agent 的 pane；kind 由用户输入 |
| 布局模板 | `layout.export` + `layout.apply` | 保存时去掉 pane_id 与 env（可能含密钥），存于 prefs.json 的 layout_templates |
| 提醒同步到 Herdr | `notification.show` | 设置开关，默认关 |
| Agent manifests | `server.agent_manifests` / `reload_agent_manifests` | Settings → Herdr |
| 本地插件 | `plugin.link` / `plugin.unlink` | Settings → Plugins |
| 热交接 | `server.live_handoff` | 确认后交给检测到的 herdr 可执行文件，完成后重连 |
| 窗口标题 | — | 原生窗口标题跟随工作区与标签 |
| Herdr 检测 | — | PATH + 常见安装目录 + `--version`；缺失时主窗口给安装引导 |
| 纯净终端 | — | `internal/plain`：本地 shell 标签窗口，无侧栏；Automatic 启动模式在未安装 Herdr 时使用 |

未采用：`client.window_title.*` 作用于 Herdr 终端客户端所在的终端模拟器，与原生窗口无关；`pane.input.set` 配置 Herdr TUI 的右键目标，Hergo 右键由本地菜单处理；`pane.link.resolve/activate` 需要视口行列坐标，终端插件已按 pane 目录解析路径；`release_notes.dismiss` / `product_announcement.dismiss` 缺少读取内容的接口。

## 后续候选（暂缓，含原因）

1. ⌘+悬停图片 Flyout 预览（kero `peek.go`）：需 Flyout 窗口与图片解码管线，收益场景较窄。
2. 快捷键重绑定界面：prefix 层键位本就由 Herdr 自己的 config.toml 管理（`input.LoadBindings`）；直键（⌘B/⌘F 等）固定，与 macOS 惯例一致。
3. 终端配色主题（Ghostty 主题名）：锁定的 mygo v0.6.3-dev 未导出 `terminal.GhosttyTheme`（kero 用的 v0.6.4 才有）；升级框架属独立决策。

