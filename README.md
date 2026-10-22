# Herdr UI · 设计与实现文档

> Herdr 是一个终端多路复用守护进程：它拥有终端、工作区、标签页、面板，以及在它们里面干活的编码代理。
> **Herdr UI** 是这个守护进程的客户端窗口——把守护进程画出来的终端单元格呈现在屏幕上，
> 并回应"打开页面 / 聚焦标签 / 发送备注"这类命令。
>
> 设计稿不是新应用，而是 **Rust 客户端 `herdr-gpui` 的视觉与交互重设计**。

---

## 1. 仓库内容

本仓库是**设计稿 + 实现**。旧的 Go 实现已于 2026-10-22 移除，
现在的实现是**按设计稿从零写的**（Go + [mygo](https://github.com/egoist/mygo) v0.6.2）。

| 路径 | 内容 |
|---|---|
| `cmd/herdr-ui/` | 客户端窗口的可执行程序 |
| `cmd/herdr-preview/` | 把窗口离屏渲染成 PNG，用来和设计稿逐屏对照 |
| `cmd/herdr-probe/` | 连上真守护进程，打印握手、会话快照与终端表面 |
| `internal/protocol/` | **线上协议**：bincode 2 varint 子集、帧、报文、终端表面、会话快照 JSON |
| `internal/client/` | **守护进程连接**：socket 发现、握手、事件循环、40 个 RPC、按键编码 |
| `internal/design/` | **设计令牌**：颜色、度量、8 级表面与材质、mygo 主题桥接、交互状态、终端调色板 |
| `internal/ui/` | 窗口的外壳：标题栏、活动栏、侧边栏、主卡片（终端 + 标签条 + 仪表行） |
| `internal/ui/widgets/` | 通用控件：Card / Row / Chip / Badge / Meter / Section / Table / Icon |
| `internal/ui/icons/` | 设计稿用的图标（Lucide + GitHub 标记），见其中的 `NOTICE.md` |
| `docs.pen/01.pen` | **设计稿源文件**（Pen 格式，71 张根画板：35 亮 + 35 暗 + 1 张令牌板） |
| `docs/01-implementation-plan.md` | **实现方案**：数据来源分类、令牌系统、35 张画板 × 能力矩阵、分阶段排期、风险与验证 |
| `docs/02-ui-design-spec.md` | **主窗口精确规格**：令牌表、度量标尺、画板 1 的完整结构树与终端语义分色 |
| `reference/herdr-gpui-features.md` | Rust 客户端 `herdr-gpui` 的完整功能点梳理（948 行） |
| `reference/herdr-api-schema.json` | 守护进程 API 字段 schema |
| `reference/interface-review-2026-10-22.md` | 设计稿缺陷清单与修复记录（含暗色模式补充审查） |
| `design/*.png` | 设计稿的渲染导出（亮/暗各 35 张 + 令牌板）。**不入版本库**，由 `.pen` 重新导出 |

阅读顺序建议：

1. `docs/02-ui-design-spec.md` —— 先知道"要做成什么样"（精确到像素与十六进制）
2. `docs/01-implementation-plan.md` —— 再知道"数据从哪来、要建哪些地基、排什么期"
3. `reference/herdr-gpui-features.md` —— 需要对齐 Rust 行为语义时查

### 跑起来

```bash
go run ./cmd/herdr-ui                 # 打开窗口（自动连本机守护进程）
go run ./cmd/herdr-preview -out /tmp  # 不开窗，渲染成 PNG 供对照
go run ./cmd/herdr-probe              # 打印守护进程给的状态与终端表面
go test ./...                         # 协议、令牌与窗口的测试
```

窗口默认跟随系统的明暗；`Cmd-Shift-D` 在「亮 / 暗 / 跟随系统」之间切换。

### 线上协议

窗口是守护进程的客户端，两者说的是 herdr 的 endpoint 协议：

- **帧**：四字节长度（小端）+ 消息体。
- **消息体**：bincode 2 的 standard profile（小端 + **varint**：≤250 单字节，
  `0xFB`+u16、`0xFC`+u32、`0xFD`+u64）。`char` 直接写 UTF-8 字节、不带长度；
  `Option` 是一个字节的标记；枚举的序号是 u32 varint。
- **握手**：客户端发 `EndpointControl{kind:"endpoint.hello.v1", data:JSON}`，
  守护进程回 `endpoint.welcome.v1`（方法清单与编解码器协商），
  然后推 `shell.snapshot.v1`（会话快照，JSON）。
- **终端表面**：`PaneSurface` 是 bincode 的单元格网格（字符、前景色、背景色、
  修饰位、链接、光标），以及每个窗格在表面上的位置。
- **请求**：`ClientShellEndpointRequest` 里装 `{"id","method","params"}` 的 JSON，
  守护进程按块回 `ClientShellEndpointResponseChunk`。
- **输入**：`ClientShellPaneInput` 带按键 / 文本 / 粘贴事件，发给聚焦窗格。

`internal/protocol/testdata/golden.txt` 是 Rust 客户端写出的字节，
Go 端逐字节对齐（见该目录的 `NOTICE.md`）。



---

## 2. 窗口应该做什么

以下是对**功能行为**的约定，与具体实现语言无关。

### 2.1 主窗口

- **左侧活动栏**：工作区 / 概览 / PR / Issue / 安全 / 用量 六个视图入口，可带计数徽标；底部是设置。
- **侧边栏**：当前面板的内容。主窗口第一屏是**工作区树**——每个工作区一行，下面是它的分支、端口和在它里面干活的代理。
- **主卡片**：终端在中间，带 16px 圆角；下面是标签页条，再下面是仪表行（用量 / 端口 / CPU / 内存 / 时间）。

### 2.2 终端

- 守护进程把终端**画好**再发过来：一格一格的单元格，带颜色、修饰位、光标、宽字符。
  客户端负责把它们**原样画出来**。
- 键盘归焦点面板所有：用户敲的字逐个送进那个面板；命名键、和弦、功能键按 Rust 客户端的编码送。
- 选区：指针划过单元格选择，松开即复制；Cmd-C 再复制一次，Cmd-V 粘贴进面板。
- **复制模式**（Cmd-Shift-C）：光标在输出与回滚里走，vi 键位移动，`v` 选、`y` 复制，`/` 搜回滚。
- 链接：守护进程读出链接覆盖的单元格，悬停时加下划线；按住 Cmd 点击打开地址，
  除非会话里的插件先认领。本机路径在自己的页面里打开。
- **标注**：选中一段文字后松开会给一个胶囊，写下的备注送到该面板的代理手里——
  措辞要让被引用的文字读起来是**数据**而不是**指令**。代理不在场时，备注留在剪贴板。
- 面板右键菜单：右分、下分、放大、滚动、进入复制模式、聚焦、关闭。
- 状态栏：面板标题、目录、代理；在那里打字会送到该面板。

### 2.3 其它

- **浏览器页**：每个页面一个 web view。本地文件从它所在的目录提供，所引用的文件才加载得到。
- **命令面板**（Cmd-K）：列出守护进程提供的命令及其键位。
- **查找栏**（Cmd-F）：搜索当前面板的输出并标出命中项。窗口只搜自己画过的格子——
  守护进程若按自己的修订号作答，而内容已变，窗口会按自己画着的修订号重试。
- **横幅**：守护进程要说的新版本、发布说明或公告，用户可关掉。
- **主题**：只影响终端。Ghostty 主题文件，或内置调色板；可写 `light:Nord,dark:Dracula`。
- **侧边栏布局**：`normal` / `compact` / `comfortable`，可加 `-rounded` 后缀。
- **通知**：卡片浮在终端角落，任务完成与代理求助有系统提示音。
- **快捷键**：可改，存于配置文件的 `[keybindings]`。

---

## 3. 守护进程协议

守护进程的客户端 socket 讲 **bincode**，外面包一个 little-endian `u32` 长度。
第一帧是具名 hello；守护进程回一个 welcome，列出它服务的方法，然后推送会话快照与已绘制的表面。

快照以 JSON 形式放在具名控制消息里，**请求的应答也是 JSON**。
绘制的表面则是 bincode：终端网格的单元格，一面板一个矩形。

### 3.1 bincode 2 子集

bincode 的数字是**带标签**而非紧凑打包的：

- 装得下 250 的数字占 1 字节；
- 否则一个标记字节说明后面跟几个字节（251 → u16，252 → u32，253 → u64）；
- 有符号数按 zig-zag 写。

> 实现这条子集时，测试里要带上 **Rust 客户端真实发出的字节**——
> 否则破坏线格式的表现是"守护进程挂起"，而不是"测试失败"。

### 3.2 守护进程提供的方法

守护进程**只有 40 个方法**，全部围绕终端多路复用：

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
→ 设计稿中超过一半的功能面必然是**客户端本地**的，必须各自实现数据源。

---

## 4. 控制协议

窗口在自己的 socket 上**每个连接回答一个 JSON 请求**。
请求说方法与对象，应答说是否办成：

```json
{"method": "browser.open", "page": {"url": "https://example.com"}}
{"status": "opened", "page": {"id": "page-1", "url": "https://example.com"}}
```

方法清单：

```
status  app  window  tab  command  agent  notes.send
browser.open / list / close / reload / navigate / back / forward / text / zoom / feedback
find
```

---

## 5. 设计稿的画板

`docs.pen/01.pen` 共 71 张根画板：

- **35 张亮色 + 35 张暗色**（一一对应）+ 1 张暗色令牌板 + 主窗口暗色版
- 主窗口总稿 `Herdr · macOS`（1440 × 900）是所有画板的母版，
  它的精确结构见 `docs/02-ui-design-spec.md` §3
- 完整画板清单与阶段映射见 `docs/01-implementation-plan.md` §1.1

导出 PNG 在 `design/`，`.gitignore` 已排除；需要时从 `.pen` 重新渲染。

---

## 6. 当前状态

| 项 | 状态 |
|---|---|
| 设计稿 | ✅ 71 张画板，亮/暗齐全，5 项审阅缺陷已修复 |
| 实现方案 | ✅ 数据来源分类、令牌系统、能力矩阵、P0–P4 排期、5 个待确认决策点 |
| 主窗口精确规格 | ✅ 令牌表、度量标尺、结构树、终端语义分色 |
| Rust 功能规格 | ✅ `reference/herdr-gpui-features.md`（948 行） |
| P0 · 设计地基 | ✅ 令牌、度量、8 级表面、材质规则、mygo 桥接、交互状态、终端调色板、通用控件 |
| P1 · 线上协议 | ✅ bincode 2 varint 子集、帧、40 方法 RPC、快照 JSON、终端表面；**与 Rust 客户端逐字节对齐**，已连上真守护进程 |
| P1 · 主窗口骨架 | 🟡 画板 1 的外壳可渲染，**接的是真会话**：真工作区树、真标签、真终端网格；按键与点击会回到守护进程。面板路由、弹层栈、终端滚动/选择/查找未做 |
| P2–P4 | ⬜ 未开始（编排器、用量、设备、审阅、编辑器组等） |

已知的待办：

- **五个面板只有一个有内容**。概览 / PR / Issue / 安全 / 用量的内容规则见
  `docs/01-implementation-plan.md` §4.2–§4.6，尚未实现。
- **终端只有渲染与输入**。滚动、选择、复制模式、查找、链接跳转还没做
  （协议侧的方法都在，`pane.scroll` / `pane.copy_search` / `pane.link.activate`）。
- **窗口尺寸与主题跟随系统**的判断在 `internal/ui/mode.go`，
  还没有落到配置文件里（P1 的 `internal/config`）。
- **重连**还没有：守护进程断开后，窗口退回样例会话，而不是自己接回去。


