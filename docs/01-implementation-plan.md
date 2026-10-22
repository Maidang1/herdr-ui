# Herdr UI · 实现方案（基于 `docs.pen/01.pen`）

> 本文把 `docs.pen/01.pen` 的设计稿翻译成一份可执行的实现方案。
> 功能语义一律以 Rust 客户端 `herdr-gpui`（`/Users/bytedance/codes/open-source/herdr-gpui`）为准，
> 通过 XERJ 索引 `herdr-gpui-docs`（28,623 条，覆盖 `crates/herdr-gpui/src/**`）检索得到。
>
> **仓库现状（2026-10-23 更新）**：全部 Go 实现、测试、模块依赖、mygo 配置及构建产物已移除，
> 当前仅保留设计文档、UI 设计稿与素材。
> 本文是一份**后续实现的规格书**：
> - §4.1 / §4.7 保留了旧 Go 基线的结构快照，**仅作功能清单与反例参考**，不再是可复用资产；
> - 其余章节中的技术选型、模块路径、阶段安排与验证要求均为实现参考，不代表当前已有代码；
> - 主窗口画板的精确规格已单独抽出到 `docs/02-ui-design-spec.md`，实现时以那份为准。


---

## 0. 一句话结论

设计稿不是新应用，而是 **Rust 客户端 `herdr-gpui` 的视觉与交互重设计**。
旧 `herdr-ui` 曾是该客户端的 Go 重写（16,782 行 Go，`internal/ui` 9,255 行 / 40 个文件，其中 12 个是测试），
**现已移除**。

因此本方案的本质是：**把 35 个功能面按新设计语言重做**，而不是从零发明功能。
旧 Go 侧约 **1/3 的功能面曾经存在**（终端、查找、复制模式、标注、标签页、侧边栏树、设置、命令面板、Toast），
**2/3 是全新的**（活动栏面板体系、编排器、设备、用量、派发、传送、编辑器组、审阅、日志等）。

工作量的大头不在"画界面"，而在 **两件地基**：
① 设计令牌系统；② 活动栏 + 面板路由 + 对话框系统（旧 Go 侧是单视图切换）。

---

## 1. 输入与依据

| 输入 | 位置 | 事实 |
|---|---|---|
| 设计稿 | `docs.pen/01.pen` | **71 张根画板**、**10,372 个节点**、**70 个变量** |
| 功能来源 | `herdr-gpui` (Rust) | `crates/herdr-gpui/src/**` + `crates/herdr-client/src/**` |
| 检索工具 | XERJ `herdr-gpui-docs` | 28,623 条，含 `body` / `code` / `defs` / `headings` / `impl` 字段 |
| 现有基线 | ~~`herdr-ui` (Go)~~ | **已移除**（2026-10-22）；历史快照见 §4.1 / §4.7 |
| 已有文档 | `reference/herdr-gpui-features.md` | 948 行功能规格 + `reference/herdr-api-schema.json` |
| 已有审阅 | `reference/interface-review-2026-10-22.md` | 设计稿缺陷清单，5 项已在 01.pen 修复 |

### 1.1 设计稿画板清单（35 亮 + 35 暗 + 1 令牌板）

| # | 亮色画板 | 对应暗色 | 性质 |
|---|---|---|---|
| 1 | Herdr · macOS | 37 | 主窗口总稿 |
| 2 | Command Palette | 38 | 全局命令面板 |
| 3 | Settings | 39 | 设置主面板（16 类目） |
| 4 | Overview | 36 | 活动栏 · 概览 |
| 5 | Pull Requests | 40 | 活动栏 · PR |
| 6 | Issues | 41 | 活动栏 · Issue |
| 7 | Usage | 42 | 活动栏 · 用量 |
| 8 | Zen Mode | 43 | 全屏专注 |
| 9 | Split Panes | 44 | 分屏 |
| 10 | Notifications | 45 | 通知/Toast |
| 11 | Transient States | 46 | 瞬态（重连/工作树/复制闪烁） |
| 12 | Agent Queue | 47 | 代理队列 |
| 13 | Find Bar | 48 | 查找栏 |
| 14 | Annotate | 49 | 标注 |
| 15 | Devices Overview | 50 | 设备总览 |
| 16 | Ports and Processes | 41→51 | 端口与进程 |
| 17 | First Run | 52 | 首次运行 |
| 18 | Review Changes | 53 | 变更审阅 |
| 19 | Browser & App Tabs | 54 | 标签页体系 |
| 20 | Editor Groups | 55 | 编辑器组 |
| 21 | Dispatch · Fan-out | 56 | 派发 |
| 22 | Teleport Wizard | 57 | 传送 |
| 23 | Security | 58 | 安全公告 |
| 24 | Logs Window | 59 | 日志窗口 |
| 25 | About | 63 | 关于 |
| 26 | Update Panel | 60 | 更新 |
| 27 | New Worktree | 61 | 新建工作树 |
| 28 | Add Device | 62 | 添加设备 |
| 29 | Settings · Plugins | 64 | 插件 |
| 30 | Settings · Notifications | 65 | 通知设置 |
| 31 | Settings · Integrations | 66 | 集成 |
| 32 | Sidebar Layouts | 67 | 侧边栏布局（10 种） |
| 33 | Settings · Integrations · Empty | 68 | 集成空态 |
| 34 | Zen Mode · Fullscreen | 69 | 专注全屏 |
| 35 | Interaction States | 70 | 交互状态规范 |
| — | *Dark Mode · Palette & Materials* | 71 | **令牌板（唯一非成对板）** |

---

## 2. 第一根支柱：数据来源的三分类

这是全案最重要的判定，直接决定架构与排期。
35 张画板里的所有数据，**只来自三个地方**：

| 类别 | 含义 | 覆盖的功能面 |
|---|---|---|
| **A · 守护进程** | session snapshot + 事件流 + RPC | 工作区 / 标签 / 面板 / 终端 / 工作树 / 链接 / 复制模式 / 查找 / 集成列表 |
| **B · 客户端本地** | 本地 git、GitHub API、本地 HTTP、配置文件、日志文件 | PR / Issue / 安全 / 用量 / 设备 / 端口 / 云 / 派发 / 传送 / 代码检索 / 日志 / 更新 |
| **C · 纯设计** | 令牌、布局、交互 | 全部视觉与布局 |

### 2.1 证据：守护进程只有 40 个 RPC 方法

`crates/herdr-client/src/method.rs:36-77` —— 全部围绕终端多路复用：

```
client_shell.surface.set    command.invoke          integration.install
integration.list            layout.set_split_ratio  pane.clear / close
pane.copy_motion / copy_search   pane.edit_scrollback   pane.focus
pane.focus_direction        pane.input.set          pane.link.activate
pane.link.resolve           pane.rename             pane.resize
pane.scroll                 pane.selection.read     pane.split
pane.swap                   pane.zoom               product_announcement.dismiss
release_notes.dismiss       server.reload_config    tab.close / create / focus
tab.move / rename           workspace.close/create/focus/move/move_block/rename
worktree.create / list / open / remove
```

**没有** PR、Issue、安全、用量、设备、端口、云、派发、传送、日志、更新的任何方法。
守护进程的数据模型同样印证 —— 快照只有 6 个实体：
`Workspace / Tab / Pane / Agent / Command / Worktree`。

> **结论**：设计稿中 **超过一半的面必然是客户端本地的**。这是排期与架构的分水岭，
> 也意味着 B 类功能不能"等守护进程支持"，必须各自实现数据源。

### 2.2 证据：Rust 客户端自己的活动栏与页面枚举

XERJ 检索 `ActivityPanel enum`（`crates/herdr-gpui/src/sidebar/activity.rs`）：

```rust
pub enum ActivityPanel {
    #[default] Workspaces,
    Overview,
    PullRequests,
    Issues,
    Security,
    Usage,
}
```

XERJ 检索 `Page enum`（`crates/herdr-gpui/src/menu/page.rs`）—— **约 50 个弹层**，
Go 侧需要一个等价的页面路由表：

```
About, AddCloud, AddCoder, AddDevice, AddWsl, AgentSkill, Application,
AppUpdate, CarryPane, Checkpoints, CodeSearch, ConfirmClose, CreateCloud,
DestroyCloud, Devices, DeviceTheme, Dialog(WorkspaceAction), FanOut,
ForwardPort, Git, GitCommit, GitHub, Group, Host, Install, Keybinds,
KillProcesses, Menu, MovePane, MoveTab, OpenCheckout, OpenLink, Palette,
Pane, PaneProcesses, PrComment, PrMerge, PrReview, RemoveDevice,
RemoveWsl, RenameDevice, RenamePane, RenameTab, Sessions, Tab, Teleport,
Themes, Update, Usage(Provider), VersionMismatch, Workspace, WorktreeScript
```

设置分类（`crates/herdr-gpui/src/settings_window/section.rs`）：
`Theme / Sidebar / Indicators / Fonts / StatusBar / General / Tabs / Code /
Notifications / Integrations / Plugins / CloudDevices / Configuration`（13 类）。

---

## 3. 第二根支柱：设计令牌系统

> 精确令牌表见 `docs/02-ui-design-spec.md` §1。本节讲**为什么要做**与**怎么落到 mygo**。

### 3.1 旧基线的问题（已随代码移除，留作反例）

旧 `internal/ui/chrome.go` 的 `chrome` 结构**只有 13 个颜色槽**（background / panel /
sidebar / text / muted / dim / accent / green / yellow / red / mauve），
且全部从 **Catppuccin 风格守护进程调色板**派生：

```go
found := palette.Catppuccin()
if name, err := theme.HerdrThemeName(!dark); err == nil && name != "" {
    if picked, ok := palette.ByName(name); ok { found = picked }
}
```

**问题**：
1. 守护进程调色板只有一份（暗色系），**无法表达设计稿的亮色模式** —— 亮色下 `found.Text` 是浅色，文字会看不见；
2. 设计稿的 28 组令牌远多于 13 个槽位；
3. 材质规则（透明、模糊、描边）完全缺失。

> 教训：**守护进程的调色板只能影响终端单元格**，窗口 chrome 全部走令牌。

### 3.2 设计稿令牌板（第 71 张）规定的内容

**28 组 light → dark 映射**，覆盖：
- 表面：窗口 / 面板 / 卡片 / 悬浮 / 选中 / 激活 / 轨道 / 凹陷
- 文本：主要 / 次要 / 禁用 / 反色
- 强调：accent / accent 弱化 / 成功 / 警告 / 危险 / 信息
- 状态底：wash 变体 / track / knob

**8 级表面**（每级含前景/背景/边框/悬停/按下 5 个派生值）——
`surface-0 (窗口)` → `surface-7 (代码块底)`。

**6 条材质规则**（token 板逐条给出）：
- 面板材质：`window` 底 + 1px 描边 + 圆角
- 卡片材质：半透明填充 + 1px 描边
- 悬浮材质：更亮的填充 + 描边 + 阴影
- 轨道/滑块材质：填充 + 描边
- 强调材质：accent 填充 + 反色文字
- 代码块材质：凹陷底 + 等宽字体

**度量标尺**（贯穿所有画板）：
- 字号 7 档：11 / 12 / 13 / 15 / 17 / 20 / 28（正文 13，标题 20/28）
- 间距 8 档：2 / 4 / 6 / 8 / 12 / 16 / 24 / 32
- 圆角 8 档：0 / 2 / 4 / 6 / 8 / 10 / 12 / 16
- 描边：1px 常规 / 2px 强调

### 3.3 mygo 的能力约束

`mygo v0.6.2` 的 `ui.Theme`（`ui/theme.go:6-8`）是 **20 个固定槽位** 的结构，
并**自带暗色判定**：`ui/runtime.go:453` `rt.dark, rt.darkKnown = rt.host.isDark(), true`，
`ui/runtime.go:462` 在暗色下切换到 `DarkTheme()`。

颜色支持 alpha：`ui/color.go:29` `RGBA(r, g, b uint8, alpha float32) Color` —— 材质规则可实现。

**因此桥接方案**：
1. 新建 `internal/design` 包持有 28 组令牌（Light / Dark 两套字面量）；
2. 写一个 `func (t Tokens) MygoTheme() *ui.Theme` 把令牌**投影**到 mygo 的 20 个槽位；
3. 组件一律读 `design.Tokens`，**不直接读 mygo.Theme** —— 这样 mygo 升级不会打乱设计；
4. 暗色来源三选一：`system`（mygo `isDark`）/ `light` / `dark`，落到 `config.Settings`。

### 3.4 建议的令牌结构

```go
// internal/design/tokens.go
package design

type Tokens struct {
    // 表面 8 级
    Surface    [8]Surface
    // 文本 4 档
    TextPrimary, TextSecondary, TextTertiary, TextInverse ui.Color
    // 强调与语义
    Accent, AccentMuted ui.Color
    Success, Warning, Danger, Info ui.Color
    // 状态底
    WashSuccess, WashWarning, WashDanger, WashInfo ui.Color
    Track, Knob ui.Color
    // 描边
    Border, BorderStrong ui.Color
}

type Surface struct {
    Base, Raised, Hover, Pressed, Border ui.Color
}
```

---

## 4. 差距矩阵：35 张画板 × 能力 × 现状

图例：**✅ 已有** / **🟡 部分** / **❌ 全新**

> 「旧 Go 基线」列记录的是**已移除**的 Go 实现曾经做到哪一步，仅供判断哪些功能
> 需要从零写、哪些只是要改样式。**该列不表示仓库里存在可复用代码。**

| # | 画板 | Rust 能力来源（XERJ 检索所得） | 数据源 | 旧 Go 基线 | 阶段 |
|---|---|---|---|---|---|
| 1 | Herdr · macOS 主窗口 | `app.rs`、`sidebar.rs`、`bottombar`、`terminal.rs` | A+C | 🟡 骨架在，缺令牌/活动栏 | P0/P1 |
| 2 | Command Palette | `palette/`、`controls.rs`（`Command`/`CommandInfo` 分类表） | A+C | 🟡 仅命令+本地项 | P1 |
| 3 | Settings（16 类目） | `settings_window/section.rs`（13 类目） | C | 🟡 扁平列表 | P1 |
| 4 | Overview | `overview/`、`sidebar/activity.rs` | A | ❌ | P1 |
| 5 | Pull Requests | `pull_request/`、`repo_items.rs`、`pr_actions/` | B | ❌ | P2 |
| 6 | Issues | `repo_items.rs`、`github/` | B | ❌ | P2 |
| 7 | Usage | `usage.rs`（Provider × 窗口种类） | B | ❌ | P3 |
| 8 | Zen Mode | `zen.rs`（`Zen`、`ZenWidth`） | C | ❌ | P1 |
| 9 | Split Panes | `pane.split` RPC | A+C | 🟡 有分屏 | P1 |
| 10 | Notifications | `notifications/`、`bell.rs` | A+C | 🟡 有 Toast，规则不全 | P1 |
| 11 | Transient States | `reconnect`、`worktree_banner` | A+C | 🟡 有重连 | P1 |
| 12 | Agent Queue | 编排器 `Run`/`RunState` 视图（PR 语义：仓库/分支/PR/评审态） | B | ❌ | P2 |
| 13 | Find Bar | `pane.copy_search` RPC | A | ✅ | P1（补样式） |
| 14 | Annotate | `annotate`/`terminal_notes`/`agent_notes` | A+B | ✅ | P1（补样式） |
| 15 | Devices Overview | `devices_overview/`、`connection/` | B | ❌ | P3 |
| 16 | Ports and Processes | `listening_ports/`、`processes.rs` | B | 🟡 有 meter，无面板 | P3 |
| 17 | First Run | `cloud_setup`、设备发现 | B | ❌ | P4 |
| 18 | Review Changes | `review/`（`Panel::Files/Notes`、`NARROW=1000`） | B | ❌ | P4 |
| 19 | Browser & App Tabs | `browser/`、`page.rs` | A | 🟡 有 page.go | P1 |
| 20 | Editor Groups | `editor/`、`group_menu` | C+B | ❌ | P4 |
| 21 | Dispatch · Fan-out | `dispatch/`、`fan_out/`（`Page::FanOut`） | B | ❌ | P3 |
| 22 | Teleport Wizard | `teleport/`（`Page::Teleport`、`CarryPane`） | A+B | ❌ | P3 |
| 23 | Security | `security.rs`、Dependabot advisory | B | ❌ | P2 |
| 24 | Logs Window | `log_window/` | B | ❌ | P4 |
| 25 | About | `about.rs`、`release_notes.rs` | C | ❌ | P4 |
| 26 | Update Panel | `updater/`、`update_panel.rs` | B | ✅ | P4 |
| 27 | New Worktree | `worktree.create` RPC | A | 🟡 | P4 |
| 28 | Add Device | `connection/`、`ssh`、`cloud/`（Azure/Coder/Daytona/Railway） | B | ❌ | P3 |
| 29 | Settings · Plugins | `plugin_values.rs` | B | ❌ | P4 |
| 30 | Settings · Notifications | `config/notifications.rs` | C | 🟡 | P4 |
| 31 | Settings · Integrations | `integrations.rs`、`integration.list` RPC | A+B | 🟡 | P4 |
| 32 | Sidebar Layouts（10 种） | `config/layout.rs`（`Density`/`Layout`/`LayoutMode`/`Style`） | C | 🟡 8 种 | P1 |
| 33 | Settings · Integrations · Empty | `integrations.rs` | C | ❌ | P4 |
| 34 | Zen Mode · Fullscreen | `zen.rs` | C | ❌ | P1 |
| 35 | Interaction States | `motion.rs`、`contrast.rs` | C | ❌ | P0 |

**统计**：✅ 2 · 🟡 13 · ❌ 20 —— **新增工作量约占 60%**。

### 4.1 旧 Go 基线的渲染骨架（已移除，仅作结构参考）

> 以下是移除前 `internal/ui/app.go` 中 `App.view` 的结构快照。
> 保留它是因为它清楚暴露了**外壳设计的三个坑**，新的实现必须避开：

```go
root := ui.Column(c).Fill().Background(colors.background)   // 背景取自守护进程调色板
root.Children(func() {
    ui.Row(c).Grow(1).Children(func() {
        a.drawRail(c, colors, "")                 // ← 第 2 参数恒为空串：无面板选中态
        a.drawSidebarPanel(c, colors, snapshot, …) // ← 恒为工作区树，无面板路由
        ui.Column(c).Grow(1).Children(func() {
            a.drawTopBar(c, colors, snapshot)
            ui.Column(c).Grow(1).Children(func() {
                a.drawFindBar(c)
                a.drawAnnotationBar(c)
                ui.Row(c).Grow(1).Children(func() {
                    if activePage != "" { a.drawPage(c, pages, activePage); return }
                    a.drawTerminal(c, snapshot)
                })
            })
            a.drawBottomBar(c, colors, snapshot, tab, pages, activePage)
        })
    })
})
if palette { a.drawPalette(c, snapshot, query) }   // 覆盖层
a.drawUpdatePanel(c, snapshot)                     // 覆盖层
a.drawSettings(c)                                  // 覆盖层
```

**三点关键结论**：

1. **底部栏是三段式** —— `drawTabStrip` + `drawMeterRow` 分开，
   与设计稿画板 1 的「meter row / tab strip / status row」结构一致。
2. **完全没有弹层（modal）系统** —— 覆盖层只有 3 个各自为政的绘制函数
   （palette / update panel / settings），彼此不互斥、无栈、无焦点管理。
   Rust 侧 `Page` 枚举约 50 页 → 必须**先建通用弹层框架**，否则 P2/P3/P4 无处安放。
3. **活动栏是空的** —— `drawRail(c, colors, "")` 的第二参数恒为空串，
   说明 rail 只是图标占位，**没有面板选中态、没有面板路由、没有徽标**。

> **P1 的本质**：把视图从「一棵固定树 + 3 个覆盖层」改成
> 「**活动栏驱动的状态机 + 可压栈的弹层栈**」。这是本方案里唯一一处需要重构既有代码的地方。

### 4.2 Overview 面板的精确规则（`crates/herdr-gpui/src/overview/`）

#### 分类规则（`overview/model.rs` `Sections::of`）—— **优先级有严格顺序**

```rust
for row in rows {
    let mark = marks.and_then(|marks| marks.get(&row.key));
    match mark.and_then(|mark| mark.hold(row.state, now)) {
        Some(Hold::Snoozed { until }) => sections.snoozed.push((row, until)),
        Some(Hold::Settled)         => sections.settled.push(row),
        None if mark.is_some_and(|mark| mark.pinned) => sections.pinned.push(row),
        None => match row.state {
            State::Blocked | State::Failed => sections.needs.push(row),
            State::Working => sections.working.push(row),
            State::Done    => sections.ready.push(row),
            State::Idle    => sections.idle.push(row),
        },
    }
}
sections.snoozed.sort_by_key(|(_, until)| *until);
```

**判定优先级**：**搁置（snoozed / settled）> 置顶（pinned）> 状态**。
即：被搁置的行**即使置顶也不进 Pinned 区**，而是留在搁置区。
状态映射：`Blocked | Failed → needs`（Needs you）、`Working → working`、
`Done → ready`（Ready for review）、`Idle → idle`。

**Snoozed 区按醒来时间升序排序**（最早醒的排最前）。

#### 分区与排序（`overview/model.rs:269+`）

```rust
pub(crate) struct Sections {
    pub pinned: Vec<Row>,
    pub needs:  Vec<Row>,
    pub working: Vec<Row>,
    pub ready:  Vec<Row>,
    pub idle:   Vec<Row>,
    pub snoozed: Vec<(Row, SystemTime)>,   // 附带各自的醒来时间
    pub settled: Vec<Row>,
}
```

`overview/view.rs` 的 `overview_list` 决定**绘制顺序**：

> **Pinned → Needs you → Ready for review → Working（按主机分组）→ Idle → 分隔线 → Snoozed → Settled → Closed**

细则（全部已验证）：
- **"Needs you" 是一张卡片**，不是普通列表：描边 `mix(background, blocked, 35)`，底色 `mix(background, blocked, 6)`，圆角 `corners::PANEL`，标题带 blocked 状态点与计数；
- **空分区不渲染** —— 没有待处理就不出卡片，分区为空就不出标题；
- **Working 区按主机分组**：行按侧栏顺序到达，因此同一主机的行自然聚在一起，主机前插一个 `icons/computer.svg` + 主机名的分组头；
- **工作架（shelf）只在有内容时出现**，空架是可点击但什么都不显示的死元素；
- 货架区之前有一条 `h(1px)` 分隔线。

#### 行的稳定标识（`overview/model.rs`）

```rust
pub(crate) enum Key {
    Agent { endpoint: String, checkout: String, agent: String },
    Run   { id: String },
}
```

> **关键语义**：面板 ID 与工作区 ID **在守护进程重启后不存活**，
> 所以一个代理的身份是「**主机 + 它工作的检出 + 它的名字**」。
> Go 侧的 marks 存储必须用这个键，不能用 pane id。

`Key::selector()` 生成 CSS 选择器串：`{endpoint}/{checkout}/{agent}` 或 `run/{id}`。

#### 标记与搁置（`overview/marks.rs`）

```rust
pub(crate) enum Hold {
    Snoozed { until: SystemTime },
    Settled,
}
pub(crate) struct Mark {
    pub key: Key,
    pub pinned: bool,
    held: Option<(Hold, State)>,
    pub woke: bool,          // 搁置到期且此后没被看过
}
```

常量：`MAX_MARKS = 512`、`MAX_FIELD_BYTES = 1024`、`FILE = "overview-marks.json"`。
**超限淘汰最久未改动的标记**（"the least recently changed go first beyond this"）。

**搁置何时解除**：`Mark::hold(state, now)` —— 状态从搁置时的状态**发生变化**，
且新状态是 `needs_you()` 或 `Working` 时，搁置失效，该行重新出现并标记为 `woke`。
即：**搁置只挡"没变化"的情况；代理一旦又需要人看，搁置自动解除**。

#### 状态优先级（`overview/model.rs`）

```rust
pub(crate) enum State { Blocked, Failed, Working, Done, ... }   // 派生 Ord，最紧急在前
```

`TITLE_LIMIT = 200` —— 不可信标题在此截断（一行只放得下一个标题）。

### 4.3 通知（Toast）的精确规则（`crates/herdr-gpui/src/notifications.rs`）

常量：`PENDING_LIMIT = 8`（`notifications.rs:10`），另有 `VISIBLE_LIMIT` / `GRACE` / `RECHECK`。

入队规则（`Toasts::receive`）：

1. **抑制启用前的通知** —— 非 `client_local` 且 `notice.arrived <= enabled_since` 的直接丢弃；
2. **同面板替换** —— `entries.retain(|(_, old)| old.pane_id != Some(pane))`，
   新通知**替换**该面板的旧通知，而不是叠加；
3. **无面板的通知不合并** —— 各自带独立 tag，互不替换（Rust 测试名即
   `ingress_is_bounded_and_same_pane_replaces_without_coalescing_targetless`）；
4. **容量上限** —— 队列长到 `PENDING_LIMIT * 2 + 1` 时，
   优先移除**最早一个不可见（invisible）的**条目，再入队新条目。

调度规则（`tick`）：
- 这是**窗口级**调度器，独立于主机顺序，统一施加顺序与边界；
- 选 `Delivery::System` 时，通过系统投递的通知**不走应用内队列**，
  而是在 `take_system` 里按序发出；
- **活动标签页只在本窗口 `focused` 时才抑制**通知（失焦时不抑制）。

徽标规则（`window/toasts/badge/tests.rs`）：
- **已知代理先显示代理标记，再显示种类图标**；
- 未知代理回落为 `icons/bell.svg`。

> **协议侧对齐点**：`notificationKinds = ["needs attention", "finished", "update installed", "custom"]`
> 与 Rust 的 `SemanticNotificationKind` 对齐，协议层需保留这四个取值。
> 但 Toast 的上述 4 条入队规则**在旧实现里也从未实现**，
> **这是 P1 中最容易被漏掉的行为差异**，新实现必须从一开始就按规则写。

### 4.4 安全面板的精确规则（`crates/herdr-gpui/src/security/`）

数据形状（`security/model.rs`）：

```rust
pub(crate) struct Repo {
    pub name: String,
    alerts: Result<Alerts, _>,   // Dependabot 告警
    reports: Result<Reports, _>, // 私有漏洞报告
}
pub(crate) enum Alerts { Open { alerts: Vec<Alert>, .. }, .. }
pub(crate) enum Reports { Open { reports: Vec<Report>, more: bool }, .. }
```

- `Repo::open()` —— 读不到或关闭时返回**空切片**（不报错，UI 显示空态）；
- `Repo::reported()` —— 本账号在 triage 中可见的私有漏洞报告；
- `Repo::more_reports()` —— **GitHub 里 triage 的报告比读到的更多**（部分列表标记）。

**排序规则（已由 Rust 测试固定）**：私有报告 **先按评级，再按最新**；
且**维护者自己的 draft 不算报告**（被过滤掉）。

严重级别枚举用的是 `Severity::Moderate`（不是 `Medium`）—— Go 侧要照抄这个拼写。

每条告警详情页展示的行（`security/pane/alert.rs`）：

| 行 | 取值 |
|---|---|
| Package | `{package} ({ecosystem})`，等宽字体 |
| Relationship | Direct（清单直接声明）/ Transitive（被别的依赖带入）/ Unknown（GitHub 说不出） |
| Scope | Runtime（随项目发布）/ Development（只用于构建测试）/ Unknown |
| Manifest | 等宽 |
| Requirement | 等宽，可选 |
| Affected versions | 等宽 |

另外还有 **"Dependabot's fix"** 区块（链到 `https://github.com/{repo}/pull/{number}`），
Transitive 时提示"GitHub 没说清是哪个依赖带入的，锁文件知道"并给"打开锁文件"按钮。

URL 规则：`{repo}/security/dependabot/{alert.number}`、`{repo}/security/dependabot`。

> **安全注意**：Rust 侧在解析 GraphQL 响应时**过滤了 `javascript:` 这类 URL**
> （见测试数据里的 `{"url": "javascript:alert(1)"}`）。Go 侧必须同样过滤，
> 否则外部漏洞报告里的链接会成为 XSS 载体。**这是 P2 的硬性验收项。**

### 4.5 审阅变更的精确规则（`crates/herdr-gpui/src/review/`）

`review.rs` 的定位：**审阅的是"焦点检出"的未提交 diff**，
在行上做笔记，再像页面标注那样送回代理。

范围（`review/diff.rs`）：

```rust
pub(crate) enum Scope {
    #[default] Uncommitted,   // 对 HEAD
    Branch,                    // 对与基线分支的 merge base，含未提交的工作
}
```

> **两种范围下 diff 都终止于工作树**，因此新增行与未变行携带的是文件**当前**的行号。

行标识与锚点：

```rust
pub(crate) enum RowId { Header(usize), Line { file: usize, line: usize } }
pub(crate) enum Anchor {
    File  { path: String },
    Line  { path: String, side: Side, number: u32, code: String, before: Option<String> },
}
pub(crate) enum Side { Added, Removed, Unchanged }
```

`Diff { files: Vec<FileDiff>, before: String }` —— `before` 是**被删除行编号所依据的修订**：

| 范围 | `diff.before` | 基线 |
|---|---|---|
| `Uncommitted` | `"HEAD"` | 无 |
| `Branch` | `"main at <commit>"` | 基线分支与 merge base |

**基线回退规则**：给定一个缺失或不安全的 hint（如 `release`、`--output=/tmp/x`、`a..b`）时，
回退到 `master`；若连 `master` 都没有 → `Error::ReviewNoBase`。

侧板布局（`review/view/panels.rs`、`review/view.rs`）：

```rust
pub(super) const NARROW: f32 = 1000.;     // 窄于此宽度，两个侧板默认都收起
pub(super) const PANEL_SHARE: f32 = 0.3;   // 文件列表与笔记各占审阅宽度的上限比例
pub(super) enum Panel { Files, Notes }
pub(crate) enum Layout { #[default] Unified, Split }   // 单列（删除行在上） / 左右对照
```

- 宽度 < 1000px 时默认收起两个侧板（**未知宽度按宽处理**）；
- **正在写笔记时，笔记板永远显示**（composer 不被藏起来）；
- 用户的显式开关会覆盖宽度默认值并保持；
- 审阅的 git 操作跑在**后台执行器**上，文件先列出、行再分批读取，
  所以**任意大小的变更都能立刻打开且保持响应**。

### 4.6 Rust 侧已验证的关键数据模型与常量（XERJ 检索所得）

这些是 Go 侧建模型时必须对齐的形状。

#### 编排器（`crates/herdr-gpui/src/orchestrator.rs`）

Rust 侧把编排器做成了一个**独立子系统**，模块划分如下：

```
orchestrator/
  actions/ beads/ error/ git/ github/ gitlab/ host/ location/
  model/ naming/ prompt/ repo/ review/ section/ service/ store/ view/
```

核心导出（`orchestrator.rs:44-59`）：

| 类别 | 类型 |
|---|---|
| 实体 | `Item` `ItemKey` `Provider` `PullRequest` `Run` `RunState` `Owner` `Activity` `Checkpoint` `HerdrSession` `SourceKey` |
| 后端 | `Backend`（GitHub / GitLab / Beads） |
| 审阅 | `CheckState` `PrStatus` `Review` |
| 服务 | `Notice` `Request` `Service` `Snapshot` `SyncState` `Timing` |
| 存储 | `Store` `Access` `SCHEMA_VERSION` |
| 仓库定位 | `Repository` `database_path` |

`Run` 的字段（`orchestrator/model.rs:257+`）：

```rust
pub(crate) struct Run {
    pub(crate) id: String,
    pub(crate) item_key: String,      // 所处理条目的规范键
    pub(crate) workspace: Option<Workspace>,
    pub(crate) agent: String,
    pub(crate) model: Option<String>,
    pub(crate) state: RunState,
    pub(crate) message: Option<String>,
    pub(crate) session_id: Option<String>,
    pub(crate) started_at: DateTime<Utc>,
    pub(crate) updated_at: DateTime<Utc>,
    pub(crate) owner: Owner,
}
```

**存储形态（重要）**：编排器**不是纯内存**，每个仓库有**自己的 SQLite 库**
（`location::database_path`），schema 始于 agent-launcher，首次打开仓库时复制过来；
重启时先显示数据库与 `crate::cache` 里的内容，再去读远端（`orchestrator.rs:6-8`）。

> **Go 侧含义**：PR/Issue 面板需要**自己的持久层**，不能只做内存列表。
> Go 侧没有 SQLite 依赖，需引入 `modernc.org/sqlite`（纯 Go）或改用 JSON 状态文件；
> 建议按 Rust 的 `Store`/`SCHEMA_VERSION` 设计 Go 版 `internal/orchestrator/store.go`。

**仓库发现上限**：`MAX_REPOS: usize = 32`（`orchestrator/host.rs:33`）。
编排器会把仓库按主机分组（`view.rs` 的 `targets()`：按 `ConnectTarget` 去重，每个主机只出现一次）。

**CLI 已存在**：`herdr orchestrator open --repo PATH` / `items --issues` / `items --prs`
（`cli/control/tabs.rs:107+`）—— Go 侧的 control socket 应能对齐这几个子命令。

#### 用量（`crates/herdr-gpui/src/usage/model.rs`）

窗口种类（`model.rs:114+`）：

```rust
pub(crate) enum Kind {
    Session,
    Daily,
    Weekly,
    Monthly,
    Named(String),     // 服务自命名的窗口，如"仅限某模型的周额度"
}
```

窗口长度常量（`model.rs`）：

```rust
pub(crate) const SESSION: Duration = Duration::from_secs(5 * 3600);      // 5 小时
pub(crate) const DAY:     Duration = Duration::from_secs(86_400);         // 1 天
pub(crate) const WEEK:    Duration = Duration::from_secs(7 * 86_400);     // 7 天
pub(crate) const MONTH:   Duration = Duration::from_secs(30 * 86_400);    // 30 天（近似日历月）
```

窗口与配速（`model.rs`）：

```rust
pub(crate) struct Window {
    pub kind: Kind,
    pub used: f32,          // 已用百分比，钳制在 0..=100
    pub resets_at: Option<SystemTime>,
    pub length: Option<Duration>,   // 已知时长，用于判断配速
}
pub(crate) struct Pace {
    pub expected: f32,      // 按均匀消耗此刻应已用的百分比
    pub runs_out: Option<Duration>,  // 按当前速率是否会在重置前耗尽
}
```

用量读取的**主机来源**（`usage/model.rs` `Host`）：

```rust
pub(crate) enum Host {
    Local,
    Ssh(String),
    Wsl(String),
    Azure { account: String, id: String },   // 仅采样 CPU/内存
}
```

> **关键语义**：远程主机的用量是**通过 SSH / `wsl.exe` 在那台机器上读**的，
> 凭据**不离开该主机**。Go 侧必须保持这一语义 —— 不能把远程用量拉到本地再解析。

用量 provider 是一组插件式解析器（`usage/providers/*.rs`：
`antigravity` `synthetic` `gitkraken` `sub2api` `zai` `grok` …），
每个 provider 声明自己的 `Meta`（名称、dashboard 链接、所需 `Setting`）。
`Setting` 形如 `Setting::new("token", &["GITKRAKEN_API_TOKEN"], "说明")`，
即 **设置键 → 环境变量候选列表 → 说明文案**。

> **Go 侧含义**：`internal/usage` 应做成 provider 接口，
> 每个 provider 实现 `Meta()` + `Fetch()`，凭据从配置或环境变量读，
> 保持"一个 provider 一个凭据槽"的形状。

### 4.7 旧 Go 基线覆盖过的功能面（代码已移除 → 现为功能清单）

> 下表记录**哪些功能面旧实现曾经做到过**。代码已删除，所以它不再是"可复用资产"，
> 而是一份**功能范围清单**：这些行现在都是 ❌，但**行为语义已经想清楚了**，
> 实现时按同一套语义重写即可，不必重新调研。

| 功能面 | 旧实现位置（已删除） | 旧行数 | 重写时的注意点 |
|---|---|---|---|
| 终端渲染 + 输入 | `internal/ui/terminal.go` | 486 | 旧版用段落排版画终端，宽字符会整行错位；新版必须**逐格定位**（见 §5 与 `02-ui-design-spec.md` §5） |
| 选区 / 复制模式 | `selection.go` / `copymode.go` | 398 / 900 | vi 键位与滚动语义照搬 Rust |
| 查找 | `find.go` | 455 | 需按画板 13 的查找栏样式重做 |
| 标注 | `annotate.go` | 536 | 需按画板 14 的样式重做 |
| 链接 | `links.go` | 344 | 链接区域解析与插件优先规则照搬 Rust |
| 浏览器页 | `page.go` | 421 | 需纳入画板 19 的页签体系 |
| 面板 / 分屏 | `panes.go` | 173 | 需按画板 9 的分屏样式重做 |
| 设置读写（保留注释） | `internal/config/config.go` | — | managed 文件 + local override 双层，与 Rust 同构；逐键写入以保留注释 |
| Toast | `toast.go` | 167 | 入队规则见 §4.3，需补全 |
| 仪表（CPU/MEM/端口） | `meters.go` | 253 | 抽成 P3 面板的数据源，样式按画板 1 的 meter row |
| 侧边栏树 | `sidebar_panel.go` | 390 | 重排为活动栏第 1 面板，度量按 `02-ui-design-spec.md` §3.1 |
| 命令面板 | `palette.go` | 166 | 扩源（6 类） |
| 守护进程协议 | `internal/herdr/` | 3,316 | bincode 2 子集 + 命名帧；新增方法沿用同一模式 |

---

## 5. 建议的包结构

> 旧 Go 实现是**平铺**结构（34 个文件同属 `package ui`），且已整体移除。
> 下表是**新实现的目标结构**，供从头搭建时参考；语言栈沿用 Go + mygo
> （mygo 的能力约束见 `docs/02-ui-design-spec.md` §5）。

```
internal/
  design/              【新】设计令牌与度量
    tokens.go            28 组 light/dark 令牌
    metrics.go           字号 7 / 间距 8 / 圆角 8 / 描边
    surface.go           8 级表面 + 6 条材质规则
    bridge.go            Tokens → mygo ui.Theme 投影
    tokens_test.go
  ui/                  【改造】外壳 + 共享控件
    rail.go              【新】活动栏（6 面板 + 徽标）
    activity.go          【新】面板路由
    zen.go               【新】专注模式
    widgets/             【新】通用控件
      card.go row.go chip.go badge.go meter.go section.go table.go split.go
  ui/panels/           【新】活动栏 6 面板
    workspaces.go overview.go pull_requests.go issues.go security.go usage.go
  ui/dialogs/          【新】对应 Rust Page 枚举（约 50 页）
    dialog.go            路由与栈
    *.go                 各具体弹层
  git/                 【新】本地 git 封装（worktree/diff/status/ahead-behind）
  gh/                  【新】GitHub API 客户端（PR/Issue/Dependabot）
  orchestrator/        【新】编排器实体与持久层
    model.go             Item/ItemKey/Provider/PullRequest/Run/RunState/Owner
    store.go             per-repo 持久化 + SCHEMA_VERSION（对齐 Rust Store）
    service.go           GitHub/GitLab/Beads 后端抽象 + SyncState
    host.go              仓库发现（MAX_REPOS=32）+ 按主机分组
  usage/               【新】用量采集
    model.go             Kind/Window/Pace/Report/Balance
    provider.go          provider 接口 + Meta + Setting
    providers/           各 provider 解析器
  devices/             【新】设备发现（SSH config/Tailscale/Bonjour/WSL）
  cloud/               【新】云 provider（Azure/Coder/Daytona/Railway）
  ports/               【新】监听端口 + 进程
  notes/               【新】代理备注队列（面板 + 终端 + 工作树三态）
```

**关键约定**：面板与弹层 **不直接触碰 `internal/herdr`**，只消费 `App` 暴露的只读快照 + 回调。
这样 `ui/panels`、`ui/dialogs` 可以脱离守护进程单测。

---

## 6. 分阶段实施方案

### P0 · 设计地基（前置阻塞项，必须最先做）

> 没有 P0，P1 之后每一屏都要返工。

| 任务 | 文件 | 验收标准 |
|---|---|---|
| 令牌字面量（28 组 × 亮/暗） | `internal/design/tokens.go` | 与画板 71 的 28 组映射逐条对齐 |
| 度量标尺（字号 7 / 间距 8 / 圆角 8） | `internal/design/metrics.go` | 数值与画板一致 |
| 8 级表面 + 6 条材质规则 | `internal/design/surface.go` | 逐条实现，含 alpha 与描边 |
| 令牌 → mygo 桥接 | `internal/design/bridge.go` | `ui.Theme` 20 槽位全部有映射，无槽位回退到硬编码 |
| 亮/暗模式切换 + 跟随系统 | `config` + `ui` | `system/light/dark` 三态；系统切换实时生效（mygo `isDark`） |
| 通用控件基元 | `internal/ui/widgets/` | Card / Row / Chip / Badge / Meter / SectionHeader / Table |
| 交互状态规范（画板 35） | `internal/design/state.go` | hover / press / focus / disabled / selected 五态有统一取值函数 |

**出口条件**：主窗口在亮/暗两种模式下按画板 1 渲染，所有颜色可回溯到令牌名（禁止裸 hex）。

### P1 · 外壳骨架（把单视图变成多面板）

| 任务 | 说明 | Rust 对应 |
|---|---|---|
| 活动栏 | 6 个面板图标 + 徽标计数；当前面板高亮 | `sidebar/activity.rs` `ActivityPanel` |
| 面板路由 | 活动栏点击 → 右侧面板替换；工作区树降级为第 1 面板 | `ActivityPanel::parse` |
| Overview 面板 | 7 个分区（Pinned/Needs you/Ready for review/Working/Idle/Snoozed/Settled）+ Closed 架；Working 按主机分组；Needs you 是带描边的卡片；空分区不出标题 | `overview/model.rs`、`overview/view.rs`、`overview/marks.rs`（详见 §4.2） |
| Overview 标记存储 | `overview-marks.json`；`Mark{key,pinned,held,woke}`；`MAX_MARKS=512`、`MAX_FIELD_BYTES=1024`；超限淘汰最久未改动者；**键 = 主机/检出/代理名**（不能 pane id） | `overview/marks.rs` |
| 状态栏三段拆分 | 画板 1 把底部拆成 meter row / tab strip / status row | `bottombar`、Rust `status_bar.rs` |
| 状态栏用量段 | 状态栏右侧显示每个 provider 的用量段（如 `5h 27% used 0m` / `wk 78% used 5d 2h`），超限标 `!`；**必须能容纳两个以上的 provider，且在两个方向的缩放下都自适应** | Rust `usage/tests/footer_fit.rs`（grok/claude/codex/copilot/zed/cursor 六段） |
| 标签页体系 | 8 种页签（Terminal/Web/File/Code/Review/Orchestrator/Devices/Editor）+ 右侧状态段 + 页签菜单 | `tab_menu.rs`、`browser/kit` |
| Zen Mode | 两步隐藏（侧栏→状态栏）、居中列宽、全屏开关、字号覆盖 | `zen.rs`（`Zen`、`ZenWidth`） |
| 侧边栏 10 种布局 | 补齐 `superset`、设备布局；行高/缩进规则 | `config/layout.rs`、`config/sidebar.rs`（`SidebarLayout`/`SpaceToken`/`AgentToken`） |
| 命令面板扩源 | 加工作区/代理/工程/备注/文件符号源 + 分类标签 + 状态过滤 + 预览 | `palette/`、`controls.rs` `CommandInfo` |
| 对话框框架 | 约 50 个 `Page` 的路由 + 栈 + 焦点陷阱 | `menu/page.rs` |
| Toast 规则补全 | 同面板替换旧通知、**无面板通知不合并**、容量上限 `PENDING_LIMIT*2+1` 时淘汰最旧不可见项、启用前的通知抑制、in-app/system/terminal 三通道、活动页仅在窗口 focused 时抑制 | `notifications.rs`（详见 §4.3） |
| Toast 徽标 | 已知代理**先显代理标记再显种类图标**，未知回落 `icons/bell.svg` | `window/toasts/badge/` |

**出口条件**：主窗口可切 6 面板，Zen / 10 种侧边栏布局可用，命令面板可搜到 6 类源，
Overview 的分区顺序与空分区规则与 Rust 逐条对齐。

### P2 · 编排器（PR / Issues / Security）—— 全新领域

> 三个面板共用一套"仓库发现"底座，**必须先做底座再做任何面板**。

| 任务 | 说明 |
|---|---|
| 编排器实体层 | `internal/orchestrator/model.go`：`Item` `ItemKey` `Provider` `PullRequest` `Run` `RunState` `Owner` `SourceKey` `Backend`（对齐 Rust `orchestrator/model.rs`） |
| 编排器持久层 | `internal/orchestrator/store.go`：per-repo 存储 + `SCHEMA_VERSION`；重启先显示本地缓存再拉远端 |
| 后端抽象 | `internal/orchestrator/service.go`：`Backend` = GitHub / GitLab / Beads；`SyncState` / `Timing` / `Notice` |
| 仓库发现底座 | 遍历本地 + 已连 SSH 主机的工作区，按仓库去重（同仓多工作树只留一条），**上限 `MAX_REPOS = 32`**；按 `ConnectTarget` 分主机去重 |
| `internal/gh` | PR 列表 / PR 详情 / review / merge / Issue / Dependabot advisory |
| PR 面板 | 过滤（Open / Mine / Has agent / Recently updated / 文本）、每行字段、检查与 review 状态（`CheckState` / `PrStatus`）、空态 |
| Agent Queue | 编排器 `Run` 的队列视图；优先级 `P0·Urgent` / `P1·High` / `P2·Medium` / `P3·Low` / 无优先级；状态 Needs review / Running / Ready to merge / Waiting / Queued / Closed and merged；过滤 Next / Priority / All / Review requested；每行含 仓库·代理 / 分支 / 时长 |
| Issues 面板 | GitHub + GitLab + Beads 三源，"无未关闭 issue" 空态 |
| Security 面板 | Dependabot 告警 + 私有漏洞报告；报告**先按评级再按最新**排序，**维护者自己的 draft 不计入**；`Severity::Moderate` 拼写照抄；`more_reports()` 部分列表标记；详情行含 Relationship/Scope/Manifest/Requirement/Affected versions + Dependabot fix 链接；**必须过滤 `javascript:` 等危险 URL** |
| PR review 弹层 | `Page::PrReview` / `PrComment` / `PrMerge` |
| 标题栏 Git 操作 | `Page::Git` / `GitCommit` / `GitHub` |
| Control 对齐 | `herdr orchestrator open --repo PATH` / `items --issues` / `items --prs` |

### P3 · 设备、用量与编排动作

| 任务 | 说明 | 数据源 |
|---|---|---|
| 设备发现 | SSH config / Tailscale / Bonjour / WSL；附近设备搜索 | B |
| Add Device | 已存 SSH 主机列表 + 4 家云 provider + "未检测到 Herdr" 引导 + 手动添加 | B |
| 云 provider 抽象 | Azure / Coder / Daytona / Railway 各自需要的凭据 | B |
| Devices Overview | 每设备代理计数、状态、搜索、轮询间隔、设备底栏 | B |
| Usage 面板 | Provider × 窗口种类（`Kind` = Session/Daily/Weekly/Monthly/Named）；已用%/重置时刻/配速 `Pace`；sign-in-failed 态；本周按 surface 分解；额度与余额 `Balance` | B |
| 用量主机语义 | 远程用量**在远端读**（SSH / `wsl.exe`），凭据不离开该主机；`Host` = Local/Ssh/Wsl/Azure | B |
| Ports & Processes | 监听端口发现、端口属主进程、侧栏端口 chip 开关、进程 CPU/内存 | B |
| Dispatch · Fan-out | 提示词 → 多主机泳道、核心数核算、运行/取消、进度态 | B |
| Teleport Wizard | 主机选择（已连/未连/已存）、审阅（分支/未推送提交/变更文件/未跟踪/代理会话）、面板映射、留在原主机的东西、进度步骤 | A+B |

### P4 · 高级面

| 任务 | 说明 |
|---|---|
| Review Changes | 本地 git diff；范围 `Scope::Uncommitted`（对 HEAD）/ `Scope::Branch`（对 merge base，含未提交）；两种范围 diff 都终止于工作树；`Diff.before` 分别为 `"HEAD"` 与 `"<base> at <commit>"`；基线 hint 缺失/不安全时回退 `master`，否则 `ReviewNoBase`；行锚点 `Anchor::Line{path,side,number,code,before}`；侧板 `Panel::Files/Notes`，`NARROW=1000` 以下默认收起，但**写笔记时笔记板必显示** |
| Editor Groups | 多编辑组、组内页签（Terminal/Web/File/Code/Review）、拆分、跨组移动、关闭组/其他、持久化 |
| Code 检索 | 索引与符号跳转 |
| Logs Window | tracing JSONL、级别、target 过滤、搜索、复制导出、环形缓冲、LIVE 指示 |
| About / Release Notes | 版本、许可、更新通道 |
| Update Panel | GitHub releases 比对、下载/安装两步确认、发布说明 |
| New Worktree | 名称/分支、自动命名、派生路径、worktree.create RPC |
| First Run | 添加工作区 / 登录 GitHub / 主机检测三步 |
| Plugins / Integrations 面板 | 插件值暴露（注意：Rust 侧当前**不能**列举/启用/禁用插件，需对齐此限制）、集成状态与安装 |
| Notifications / Integrations 设置 | 送达通道、声音、位置、延迟 |

---

## 7. 需要在实现前确认的五个决策点

1. **亮色模式与守护进程调色板的关系。**
   旧实现从 Catppuccin 调色板取色，无法表达亮色（已在移除的代码里犯过这个错）。
   建议：**令牌自洽，守护进程调色板只影响终端单元格配色**；窗口 chrome 全部走 `internal/design`。
   需确认是否接受"窗口不再跟随终端主题变色"。

2. **GitHub 凭据来源。**
   Rust 侧用 `GH`/`GITHUB_TOKEN` 与 `config/github.rs`（含 `MAX_ACCOUNTS`、`valid_account_id`、`valid_owner`）。
   Go 侧需实现同等校验，不能只读环境变量。

3. **云 provider 是否进 Go 首版。**
   Rust 侧 Azure/Coder/Daytona/Railway 均带 `#[cfg(feature = "...")]` 特性门。
   建议 Go 首版只做 **Azure + Coder**，其余留 stub，但**保留 `Page::AddCloud` 的形状**。

4. **远程用量的凭据边界（安全决策）。**
   Rust 侧的语义是：远程主机的用量**在那台机器上读**（SSH / `wsl.exe`），
   凭据**不离开该主机**；云机器则不读签入，只采样 CPU/内存。
   Go 侧建议**照搬**：`internal/usage` 对 `Host::Ssh` / `Host::Wsl` 走远程执行，
   只把解析后的百分比与重置时刻带回本地。
   若改成"拉回本地解析"，等于把远程主机上的 token 暴露到本机进程，需明确批准。

5. **编排器的持久层选型。**
   Rust 用 per-repo SQLite（`SCHEMA_VERSION` 迁移）。
   Go 侧可选：(a) 引入 `modernc.org/sqlite`（纯 Go，无 cgo）保持同构；
   (b) 用 JSON 状态文件 + 版本号，牺牲查询能力换取零依赖。
   建议 (a)，否则 P2 之后每次渲染都要全量反序列化。

---

## 8. 风险与验证

| 风险 | 表现 | 对策 |
|---|---|---|
| mygo 槽位不足 | 令牌无法 1:1 落到 `ui.Theme` | 组件读 `design.Tokens`，mygo 槽位只做兜底投影 |
| 面板分包后状态散落 | 多面板共享选中态难以维护 | 状态集中在 `App`（单一真源），面板只读 + 回调 |
| 本地 git / GitHub 阻塞 UI | 面板卡顿 | 全部走异步 + 缓存 + 空态；快照语义与 Rust 一致 |
| 令牌漂移 | 后续加面时颜色不一致 | 禁止裸 hex；加 lint 或单测断言所有绘制色来自令牌 |
| 暗色下守护进程调色板冲突 | 亮色模式文字不可读 | 见 §7.1 |
| **外部内容注入** | GitHub advisory 里的 `javascript:` / 含空格 URL 被渲染成可点链接 | Go 侧必须照 Rust 的 URL 白名单过滤（§4.4），这是**安全项，不是样式项** |
| **通知行为细节漂移** | Go 侧漏掉"同面板替换""无面板不合并""启用前抑制"，用户会看到重复/陈旧通知 | 把 §4.3 的 4 条入队规则写成表驱动单测 |
| **代理标识用 pane id** | 守护进程重启后 Overview 的搁置/置顶全部丢失 | 用 Rust 的 `Key{endpoint,checkout,agent}`，见 §4.2 |
| **用量凭据回流本地** | 远程主机的用量被拉到本地解析，凭据外泄 | 保持 Rust 语义：在远端读，只回传结果，见 §7 决策 4 |

**验证手段**：
- 令牌单测：`design.Tokens` 与画板 71 的 28 组映射做表驱动断言；
- 面板单测：面板纯函数（排序、过滤、空态）不依赖守护进程；
- 协议单测：守护进程协议层沿用旧实现的表驱动写法（`wire_test.go` / `endpoint_test.go` 模式），新增方法照此扩展；
- 行为对齐单测：把 Rust 侧已固定的规则逐条翻译成测试 ——
  Overview 分区顺序与空分区、marks 的搁置解除条件、Toast 的 4 条入队规则、
  报告的"评级优先 + 新次之 + 排除 draft"、审阅的基线回退与 `NARROW=1000` 行为；
- 视觉回归：按 35 张亮色画板 + 35 张暗色画板逐屏对照；
- 度量回归：主窗口部件尺寸与 `docs/02-ui-design-spec.md` §2.5 逐项断言。

---

## 附录 A · 检索来源

| 用途 | XERJ 查询 / 源码位置 |
|---|---|
| 活动栏枚举 | `ActivityPanel enum` → `crates/herdr-gpui/src/sidebar/activity.rs` |
| 弹层枚举 | `Page enum` → `crates/herdr-gpui/src/menu/page.rs` |
| 设置分类 | `settings_window/section.rs` |
| 布局枚举 | `LayoutMode sidebar layout enum` → `crates/herdr-gpui/src/config/layout.rs` |
| RPC 方法 | `crates/herdr-client/src/method.rs:36-77` |
| Overview 分区/排序 | `overview agent card ordering needs you snooze settle` → `overview/view.rs`、`overview/model.rs` |
| Overview 标记与搁置 | 同上 → `overview/marks.rs` |
| 通知入队规则 | `notification replace existing notice same pane waiting limit sound` → `notifications.rs` |
| 通知常量 | `PENDING_LIMIT const notifications visible` → `notifications.rs:10` |
| 编排器模块与实体 | `orchestrator model Item Provider Backend Section` → `orchestrator.rs`、`orchestrator/model.rs` |
| 仓库发现上限 | `MAX_REPOS repo discovery pull request across hosts` → `orchestrator/host.rs:33` |
| 用量窗口模型 | `usage window kind five hour weekly session quota` → `usage/model.rs` |
| 安全面板 | `security panel advisory severity dependabot pagination` → `security/model.rs`、`security/pane/alert.rs` |
| 审阅变更 | `review changes diff scope uncommitted side by side` → `review/diff.rs`、`review/view/panels.rs` |
| 用量主机语义 | 同用量查询 → `usage/model.rs` `Host` |
| 功能规格全文 | `reference/herdr-gpui-features.md`（948 行） |
| API 字段 | `reference/herdr-api-schema.json` |
| 设计稿审阅 | `reference/interface-review-2026-10-22.md` |
