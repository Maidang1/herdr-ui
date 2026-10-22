# herdr-gpui 功能点梳理

> 来源:`/Users/bytedance/codes/open-source/herdr-gpui`(commit `a64c6eb`,PR #755 合并后)
> 权威来源:`crates/herdr-gpui/README.md`(4562 行功能规格)+ `docs/cli.md` + 源码模块
> 整理时间:2026-10-22

---

## 0. 一句话定位

Herdr UI 是 Herdr 守护进程的**原生 GPUI 客户端**(Rust,macOS/Linux/Windows)。它不自带终端模拟器、不启动 PTY、不包装 TUI——只是把守护进程吐出的终端单元格原生画出来,并额外提供浏览器标签页、代码标签页、Review 视图、设备总览等"只有客户端才知道"的功能。

工作区:`crates/embed-resource` `herdr-client` `herdr-gpui` `herdr-gpui-launcher` `herdr-protocol`;主 crate 2133 个 `.rs`、约 43 万行。

---

## 1. 应用外壳(App Shell)

### 1.1 启动与连接
- 启动方式:`herdr-gpui` / `--session NAME` / `--socket PATH` / `--dev`(用 `herdr-dev` 配置目录)。
- 本地守护进程不存在时**自动拉起**已安装的 `herdr server`;不安装、不升级、不停守护进程。
- `--socket` 必须是**二进制 client socket**,不是 JSON API socket;指定后完全隔离:不加载已保存主机、不读写选择。
- Unix 登录 shell 环境探测在连接 worker 上做一次(5 秒超时),失败回落到 per-user / Homebrew bin 目录;终端启动跳过探测。

### 1.2 SSH 主机
- 要求:已装 POSIX Herdr、非交互认证、已信任 host key。
- MFA/密码主机需 `~/.ssh/config` 里配 `ControlMaster auto` + `ControlPath`,应用只复用主连接,从不自己创建。
- 失败重试退避上限 **30 秒**;需要用户介入的失败(未信任 host key / 认证失败 / 未装 Herdr)固定等满 30 秒。

### 1.3 连接状态与降级
| 状态 | 表现 |
|---|---|
| 本地守护进程启动中 | 状态点**琥珀色脉冲** |
| 已断开 | 状态点**红色** |
| 健康 | 状态栏保持安静(连接指示在设备选择器里) |
| 断连主机 | 保留最后画面并变暗,盖一张卡片说明原因 + `Reconnect now` 按钮 |

- 断连卡片会明确指出原因:**host key 未信任 / 认证失败 / 主机不可达 / 未安装 Herdr**。
- 掉线期间键入的按键不会发送,并有 flash 提示。
- 休眠唤醒后:每条存活 SSH 链路必须在 **3 秒**内响应 ping,否则重连;所有掉线端点立即同时拨号。
- `Terminal > Reconnect` 立即重试,**不回放输入**;命令面板 `Detach` 断开当前端点但不关窗口、不停守护进程,并暂停重试直到 Reconnect。

### 1.4 窗口
- 恢复上次退出时开着的主窗口(尺寸 + 屏幕位置),最多 **64** 个;关闭单个窗口会从恢复集里移除,关掉最后一个仍保留几何。
- 显示器已拔 → 在主屏打开并缩放适配;全屏窗口恢复为普通矩形;Logs 窗口不恢复。
- 状态文件:`window-state.json`(`$XDG_STATE_HOME/herdr/gpui`)。
- **Focus follows mouse**:分屏内悬停即聚焦(默认开)。按住鼠标键/修饰键/拖拽/IME 组字/菜单/终端弹窗会抑制它。

### 1.5 升级器
- 侧边栏 `app updates` 或 `Herdr > Check for Updates...` 打开 GPUI 面板,显示下载进度与不确定进度动画条。
- **Download** 和 **Install Update** 是两次独立确认;关面板不取消;显式 `Cancel` 才取消。
- 安装方式分流:Homebrew → `brew upgrade --cask`;Linux 发行版 → 签名的 deb/rpm/Arch 包,经 PolicyKit 调 APT/DNF/pacman(不可中途取消,完成后给 `Restart`);独立安装 → 安装即重启。

### 1.6 QA 菜单(`qa-menu` cargo feature,默认构建与发布版不含)
`Show NeedsAttention toast` / `Show Finished toast` / `Show UpdateInstalled toast` / `Show Custom toast` / `Show app update available`(合成版本 `9999.0.0`)/ `Show update download progress (50%)` / `Show Homebrew update progress` / `Enable badge` / `Send notification in 3 seconds` 等。

---

## 2. 菜单栏(native)

macOS 走系统菜单栏,Linux/Windows 在窗口内画一条。

| 菜单 | 项 |
|---|---|
| **Herdr** | About Herdr / Command Palette / Settings / Keyboard Shortcuts / Check for Updates... / Hide Herdr / Hide Others / Show All / Quit Herdr |
| **File** | New Workspace / New Worktree... / New Tab / Go To / Go to Symbol... / Go to File... / Close Pane... / Close Tab... / Reopen Closed Tab |
| **Edit** | Cut / Copy / Paste / Select All(系统动作,终端聚焦时按上下文启用) |
| **View** | Increase/Decrease/Reset Font Size / Devices Overview / Toggle Full Screen / Toggle Zen Mode / **Layout**(子菜单) |
| **Terminal** | Split Vertically (Right) / Split Horizontally (Down) / Next Tab / Previous Tab / Back / Forward / Toggle Pane Zoom / Clear Pane / Find / Find Next / Find Previous / Copy Mode / Open Scrollback in Editor / Open Notification Target / Toggle Sidebar / Toggle Status Bar / Open VS Code / Open px0 / Reconnect |
| **Window** | Minimize / New Window / Cycle Through Windows / Close Window / Logs |
| **QA** | 仅 opt-in 构建 |

**View > Layout** 共 10 种:`normal` `compact` `comfortable` + 各自 `-rounded`(共 6)+ `superset` `orca` `minimal` `devices`,当前项打勾。

---

## 3. 标题栏(Title Bar)

- `ui.tab_bar_position = "top"`(默认)时,**标签行本身就是标题栏**,不画独立 header;贴顶的 strip 会长到 header 的 **34px**。
- 侧边栏列顶部一行:交通灯让位 + 侧边栏开关 + Back / Forward。侧边栏收起或隐藏时,最左 strip 领衔。
- Back / Forward 像浏览器一样走"这个连接聚焦过的 pane"历史,跨标签跨工作区,**最多 100 个 pane**,守护进程重启即重置。`Cmd-[` / `Cmd-]` 同效,鼠标侧键任意位置可用。
  - macOS 把两个 chevron 合在一个分段 bezel 里(同 Finder / Xcode);Windows 用 Fluent 箭头按钮;Linux 用 GNOME 扁平圆角按钮。
  - header 窄于 **320px** 时直接省略这一对。
- **macOS 尺寸**:交通灯位于 **(9, 9)** 逻辑像素;让位区 **80px**;右上角 **40px** 槽位放 **16px 圆形头像**(内含 12px SVG,28px hover 目标),点击打开原生 GitHub 登录。header 与让位区在全屏下保留。
- **工作树横幅**:linked worktree 构建在 header 下方加一条通栏 **22px 琥珀色**横幅,显示分支或短 SHA + 可选 PR 号(可点开 GitHub);分支名可截断,PR 号始终可见。主检出构建没有横幅。

---

## 4. 侧边栏(Sidebar)

### 4.1 Spaces 列表
- 顺序:**Local 优先**,再按上游目录顺序排已保存主机;已启用主机后台连接,已禁用主机仍可见。
- 目录**每 2 秒**轮询一次;目标/会话/启用状态/顺序变更无需重启即生效;读目录出错时保留上一份有效列表。
- 主机分组:多主机时 Spaces 按主机分组,**置顶主机行**会随滚动吸顶,直到被下一台主机挤出。
- 行内容:分支详情、守护进程驱动的实心/空心活动点(实心=工作)、PR 号与 diff 计数、未提交工作(黄色铅笔片)、上游漂移 `↑`/`↓`(绿=待推,红=待拉)、仓库 owner 头像。
- `change_counts = "local"` 改显示本地未提交 `+新增/-删除` 行数(只覆盖本机检出,每次一个,至少隔 30 秒)。
- 状态点与删除图标跨行对齐;`Add session…` 行无状态点。
- 右键菜单:**Rename** / **Close**(有未提交内容或未推送提交时需键入 `close` 明确同意)/ **New worktree** / **Open worktree...** / `Note...` / `Teleport...` / `Move to…`。
- 悬停菜单开关:`Settings > General > Open space menus on hover`(默认关)。

### 4.2 Agents 面板
- 聚合所有已连接端点的 agent,带主机标签;排序 `grouped` / `priority`,点标题即切换,持久化。
- `priority` 排序:断线主机的排最后,然后是需要注意的,再按最近变更。

### 4.3 设备选择器(左下固定)
- 项:**All Devices** / **Local** / 已保存 SSH 设备 / **Add Device…**。
- 齿轮按钮开 Settings(`Cmd-,`);会话图标列本机与各设备的会话(`Cmd-Shift-S`),设备列表**最多 30 秒**问一次 SSH。
- `device_footer = "auto"` → 只有 ≥2 台设备时才画;默认 `always`。

### 4.4 侧边栏布局(10 种)
| 布局 | 特征 |
|---|---|
| `normal` / `compact` / `comfortable` | Herdr 原有密度,平/圆角两版 |
| `superset` | 单行 + 图标槽 + PR 计数 |
| `orca` | 圆角卡片 + 元信息行(主机/分支/PR) |
| `minimal` | 一行只有状态点和名字,无图标 |
| `devices` | 每台设备下挂其 agent,带搜索 |

---

## 5. 活动栏(Activity Bar)—— 窗口左缘 44px 图标列

分区顺序即源码 `ActivityPanel::ALL` 顺序:

| # | 分区 | 内容 |
|---|---|---|
| 1 | **Workspaces** | 终端旁边的侧边栏本体;再点一次折叠成 rail |
| 2 | **Overview** | 每个主机上"需要你处理"的总览 |
| 3 | **Pull requests** | 所有已连接主机工作区持有的仓库的 PR |
| 4 | **Issues** | 同上的 issue |
| 5 | **Security** | 各仓库的 Dependabot 告警 + 私密漏洞报告 |
| 6 | **Usage** | 每台主机的套餐额度 |

- 除 Workspaces 外,其余分区占据窗口主体,**保留标题栏和状态栏**;Zen 模式保留终端。
- `Settings > Sidebar > Show activity bar` 或 `[sidebar] activity_bar = false` 全局关闭。
- 徽标语义:有计数时用强调色,打开后转灰,直到计数内容再变化。图标计数 = "需要你处理"的数。
- 状态点/忙碌动画 1600ms 呼吸一次。

### 5.1 Overview 详细
- 顺序:**等你回复的 agent 和失败运行**各自成卡在前 → 已完成待审 → 按主机分组的工作中 → 空闲。
- 悬停行给 `Snooze` / `Settle` / `Pin` / `Open`;被 snooze/settle 的行折到底部,时间到或 agent 再次动作时回来。
- agent 的 pane 关闭但主机还活着 → 落到底部 **Closed** 架子上:`Reopen` 在其工作区新标签里按原目录重开,并尽量续接上次对话(`claude --continue`、`codex resume --last`、`opencode --continue`);`Forget` 丢弃。
- 右侧一列显示每台主机的 CPU / 内存 / agent 数 / 端口,可折叠;再折成一条负载点带。

---

## 6. Orchestrator(Pull requests / Issues 两个分区)

- **范围**:跨窗口,列出所有已连接本地/SSH 主机工作区持有的仓库的 GitHub issue 与 PR、GitLab issue、beads issue(主检出含 `.beads` 目录时),以及为它们派发的 agent 运行。
- 非 Git 目录若没有 `.beads` 会被静默跳过,`Refresh` 会重查被跳过的目录。同一仓库的多个 worktree 只算一次。**最多 32 个仓库**同时列出,每个仓库独立同步。
- **胶囊筛选行**:后端(GitHub / GitLab / Beads)与项目,各带计数;全不选 = 全列。
- **Issues 分组**:按优先级分组,bead 的子项挂在 bead 下。`Filters` 按钮展开一列(有运行的 / 被阻塞的 / 单一来源 / 单一标签),折叠后用 chip 表示。5 种排序。搜索跨标题/ID/作者/标签/状态/正文,大小写敏感规则同 Herdr 复制模式。行上 `Start agent` 开派发对话框。
- **PR 分组**:按"在等什么"分组——**Review requested / Fix(要改或 CI 挂) / Ready to merge / Waiting / Closed and merged**。行显示分支、作者、review 状态、CI 彩色胶囊、diff 大小、运行、年龄。
- **详情面板**:点击选中,Enter 或双击在列表旁开一个 pane;PR 详情页有 GitHub 风格的对话(头像、Markdown 评论、内联 review thread、已解决折叠、顶部结论汇总),可直接评论;显示 CI、review 决定与合并状态;合并时列出仓库允许的方式并指明 head commit。
- **派发对话框**:提示词 + 目标主机上已装的 agent + 可选模型 + 分支(`agent/<number>-<slug>-<hash>`)+ 额外指令。从主检出 `HEAD` 建新 worktree 并在其中启动 agent。
- `Review with agent` 在 `agent/review-…` 分支上做只读审查,不写 GitHub。
- **运行卡片**:发消息 / 停止 / 删除其 worktree,每次都要确认。
- **无数据库匹配**:分支名尾的 hash 就是条目自己的标识,所以不靠数据库也能找到工作(其他机器上的 Herdr 工作区在同名分支上会被识别为该条目的运行)。
- **来源**:GitHub 用该主机上 PR 所用的账号;GitLab 用 `GITLAB_TOKEN` / `GITLAB_ACCESS_TOKEN` / `glab auth login`,只走 HTTPS,**GitLab 的 MR 尚未支持**,**GitHub Enterprise 尚不支持**。

---

## 7. 终端与分屏

### 7.1 渲染
- 语义单元格画布:ANSI 命名色 / 256 / RGB / reverse / dim / hidden / bold / italic / underline / strikeout / 宽字符跳过。
- 实线制表符、块元素(含分数块与四象限)、四种实心提示符分隔线(U+E0B0 / E0B2 / E0B4 / E0B6)按单元格网格绘制,边缘对齐设备像素;边框、块艺术 logo、提示符帽顶达满单元格高度。
- 虚线/双线/圆角/斜线制表符与组合字符仍走字体渲染。
- **两遍绘制**,不是损坏行渲染器(大 surface 上 CPU 余量少)。

### 7.2 输入
- `EntityInputHandler` 提交文本、Unicode 组字、行内标记文本、dead key、F1–F24、AltGr(Windows = Ctrl+Alt)。
- 滚轮发**语义事件**(`ScrollUp`/`ScrollDown` + 面板相对位置 + 修饰键 + 行数),不是绝对偏移。
- 复制模式(`Cmd-Shift-C`)块状光标走整个 scrollback,`hjkl`/方向键移动,`w b e W B E $ ^ { }` 是守护进程提供的文本感知动作,`/` 向新搜、`?` 向旧搜,`v`/`V` 按格/按行标记,`y`/Enter 复制。
- 查找栏(`Cmd-F`)搜整个 scrollback,显示 `"3 of 17"`,Enter/↑ 找更旧、Shift-Enter/↓ 找更新,输入永不到达终端。

### 7.3 选区与复制
- 鼠标拖选、Shift 强制本地选择、释放即复制并 flash `copied to clipboard`。
- 按住 **Shift** 起选会在选区旁浮出 **Annotate** 药丸 → 打开紧凑的注释输入框 → Enter 发送 / Shift-Enter 排队 / Esc 取消。注释发给该 pane 的 agent,没有 agent 则落到剪贴板。
- OSC 52 的复制字节会写进本机剪贴板(有大小与 UTF-8 校验)。
- 选区不跨 pane;拖出 pane 边界只选到边缘;弹窗里的选区只取弹窗自己的单元。
- 选区高亮在释放复制后保留,直到下一次点击/按键/重连/切换端点。

### 7.4 标签页与分屏
- `Cmd-D` 垂直分屏(右侧新 pane),`Cmd-Shift-D` 水平分屏(下方新 pane)。
- `Cmd-Shift-Enter` 切换 pane 缩放;`Cmd-K` 清屏 + 清 scrollback(需守护进程广告 `pane.clear`)。
- `Cmd-1..9` 切到第 N 个标签;`Cmd-Alt-方向键`切 pane;`Cmd-Alt-]`/`Cmd-Alt-[` 在当前标签内循环 pane。
- `Cmd-W` 关 pane;`Cmd-Shift-W` 关标签(仅当标签内有 agent 在工作/阻塞时确认,且 **Cancel 默认选中**;pane 关闭对话框有 `Do not ask again` 空格键开关)。
- 关闭标签会终止运行中的进程,与退出 GUI 不同(退出只是 detach)。
- 面板菜单还提供 `Swap with Focused Pane`、`Zoom` / `Unzoom`、`Move to New Tab` / `Move to New Workspace` / `Move to…`。

### 7.5 编辑器组(Editor Groups)
- 标签条右端的分割按钮或 `Cmd-\` 把视图拆成左右并排组;拆分不新开标签也不移动标签。
- 每个组持有自己的守护进程连接,只有"在用组"持键盘;通知/声音/标题跟随在用组。
- 组 `…` 菜单:**New Browser Tab**(`Cmd-Shift-B`)/ **Review Changes**(Git 芯片跟踪到检出时)/ 该工作区的监听端口(各自开页面)/ **Split Right** / **Close Tab** / **Close Others** / **Close Group**。
- 标签右键:**New Tab** / **Rename** / **Close Tab**。
- 状态持久化在 `editor-groups.json`(每工作区的组、标签、宽度、在用组)。

---

## 8. 浏览器标签页(Browser Tabs)

- **应用自有**,守护进程和其他客户端都看不到;每个窗口对同一个标签有各自的页面实例;持久化在 `browser-tabs.json`。
- 打开方式:组 `…` 菜单的 `New Browser Tab`、命令面板、终端链接点击、**agent 打开**。
- 新标签默认空白并聚焦地址栏,同时列出该工作区的监听端口,点端口直接加载。
- 工具条:后退 / 前进 / 刷新 / 地址栏 / 在系统浏览器打开。地址栏接受裸主机名(`localhost:3000` → `http://localhost:3000/`)。
- **favicon**:来自 `<link rel="icon">` 或 `/favicon.ico`;跨源图标需 CORS 允许,否则保留地球图标。
- **缩放**:`Cmd-=` / `Cmd--` 50%–300%,`Cmd-0` 复位;每标签记住自己的缩放,工具条显示当前缩放,点击回 100%。
- 安全:只开 `http`/`https` 与 agent 展示的本地文件;禁止跨 scheme 跳转、拒绝下载、新窗口开成新标签;页面只能向 app 发两类消息(注释拾取、图标)。
- **原生 web view**(macOS WebKit / Windows WebView2)。**Linux 无内嵌页面**,浏览器标签请求改为打开系统浏览器。
- 标签顺序:Next/Previous Tab 和数字键把浏览器标签算在内,排在工作区 Herdr 标签之后。

---

## 9. 页面注释与 Review(两类"给 agent 留言"的流程)

### 9.1 页面注释(Annotate)
- 浏览器标签工具条的 `Annotate` 进入注释模式:悬停高亮 + 点击拾取元素,或选文本,或在注释面板里开 `Region` 拖拽区域(随时 Shift 拖拽);链接在注释时不跟随。`Note on page` 对整页写注释。Esc 放弃当前拾取,再 Esc 退出。
- 每次拾取在 **macOS** 上截一张该区域的图(WebKit 自带快照),随注释一起发送;发送后存为私有文件,一周后删除。Windows 无截图。
- 注释写在**页面右侧的注释面板**里(页面本身看不到),每条注释在页面上带编号;每个标签最多 **20** 条待发。
- `Send to agent` 组装一条 prompt:页面 → 每条注释(元素选择器路径、文本、简短 HTML 片段、截图、注释文字)。`Copy` 把同样的 prompt 放剪贴板。
- **投递是单向且有范围的**:只投给 `browser feedback --wait` 正在等的 agent,否则打字进 agent 的 pane 并在守护进程报空闲后提交(两分钟仍在工作也提交)。绝不打进"没有运行 agent"的 pane(Enter 会跑 shell),也不投给正在提问的 agent——这些注释等 `browser feedback`。
- 已发送的注释留在列表里(变暗 + 标记 sent);点文字可编辑,保存后变回未发送。`Send 2 new` / `Resend all` / `Clear sent`。
- 等待投递时 Send 位置换成 `Withdraw`;已打进 pane 的批次无法撤回。
- 本地页面可用 `herdr.send(summary, data)` 回传结果,显示为 `Page result` 注释,只在 Send 时随注释发给 agent;summary 限一行 300 字符,data 超 8 KB 拒收。

### 9.2 Review Changes(审查 agent 的改动)
- 组 `…` 菜单 → `Review Changes`,打开 focused 本地检出的未提交改动的 review 标签;未跟踪的文本文件算整体新增。
- 头部两个图标:切换改动文件列表、切换注释面板。宽度小于 **1000px** 时默认两个都隐藏,切开后记住选择。
- **diff 范围**:`Uncommitted`(`git diff HEAD`)或 `Branch`(工作区 vs 分支基点;基点 = GitHub 报的 PR base 分支,否则 `origin/HEAD`,再否则 `main`/`master`;不 fetch)。基点可点开选择其他分支,`Automatic` 回到自动。
- **布局**:`unified`(删除在上)或 `side by side`(左旧右新),拖分隔条可调(每侧至少 1/5),双击平分;选择记住。
- **文件树**:目录树,只含一个子目录的与子目录同行;每文件显示变更字母(`A`/`M`/`D`/`R`)、增删行数、排队注释数、已看过的勾。过滤框按路径筛,`Hide viewed` 隐藏已看。点文件把它顶到 diff 顶部。
- **diff 细节**:长行按 GitHub 方式换行;文件头可折叠并标记 `Viewed`;删除文件、锁文件、`.gitattributes` 标记 `linguist-generated` 的文件默认折叠;重命名显示 `old → new`;hunk 头显示省略了多少未改动行(可点开,每次最多 200 行);编辑对内的变化词高亮;空白图标忽略纯空白改动。
- **搜索**:搜索图标 / `/` / `Cmd-F`;Enter/Shift-Enter 或 `n`/`N` 跳匹配;折叠文件里的匹配会自动展开。
- **键盘**:`j`/`k`/方向键滚一行,Space/PageUp/Down 翻页,`]`/`[` 上下个 hunk,`.`/`,` 上下个文件,`x` 折叠顶部文件,`v` 标记已看。
- **注释**:点行号槽(增/删/未改)或文件名写注释,Enter 或 `Add note`;已注释行带编号;未发送的注释在标签打开期间一直留着。
- **发送**:发给 focused pane 的 agent,没有就发给 focused 工作区第一个 agent(头部显示是谁)。prompt = 检出 + 每条 `path:line` + 该行引文 + 注释。
- **规模限制**:单次 Git 调用最多 64 文件或 20,000 变更行;超 500,000 变更行的文件只在滚动到时读;单文件超 20,000 行或 4 MiB 标为 `Large change not shown` + `Load diff` 按钮;二进制文件列出但不读。
- 复制只复制代码,不带行号、`+`/`-` 符号和 hunk 头。

---

## 10. 工作树(Worktree)

### 10.1 脚本 `.herdr/worktree.toml`
- 仓库级文件,`[scripts]` 表,键:`setup` / `run` / `archive`;未知键报错。
- `setup` 在 worktree 创建后跑(打开已有检出**不**跑);`run` 从检出右键菜单起;`archive` 在确认删除检出时跑,失败则保留检出并把输出留在标签里。
- 每次在新标签里跑(标签名 `setup`/`run`/`archive`),`sh -e`,工作目录为检出,注入 `HERDR_WORKTREE_PATH` 和 `HERDR_ROOT_PATH`。
- **信任对话框**:首次运行展示脚本全部内容;`Trust and run`(记住该主机该仓库的该文件,文件一变就重新问)/ `Don't run` / `Remove without running`。授权存在 `trusted-worktree-scripts.json`。
- 远程主机的检出在**那台机器上**读和跑;**WSL 不支持脚本**。

### 10.2 备注(Worktree Notes)
- 检出右键 `Note...`;行上带便签图标,首行显示在行下方(过长截断),点那行即编辑;空则删除。
- 一行,最多 **500 字符**,存 `worktree-notes.json`,不上报守护进程,所有窗口共享;绑定"主机 + 仓库 + 分支"。

### 10.3 移动 pane
- `Move to New Tab` / `Move to New Workspace` / `Move to…`(列出所有工作区的其他标签、每个其他工作区的新标签、持有同仓库的设备上的新 worktree 与空间)。
- 落到已有标签会拆分该标签的 focused pane,把搬来的 pane 放右边;终端继续跑。
- 仅 Linux/macOS + 本地/命名会话/SSH 设备;自定义 socket、`--dev` 会话不可用。

### 10.4 搬到另一台设备
- 复制提交 + 已暂存/未暂存/未跟踪改动到目标设备的新 worktree(同分支或从同一提交新开分支),目标空间必须干净且不覆盖,否则拒绝。
- agent 会话可迁移并续跑;其他程序用相同参数重启;源 pane 关闭后窗口跟随过去。
- 不在 Git 仓库里 → 只能迁 agent 会话(`DEVICE › Agent Session Only`),cwd 相对家目录映射,目标目录必须已存在。

### 10.5 Teleport(跨主机搬家)
- linked worktree 右键 `Teleport...`,搬分支与提交、暂存/未暂存/未跟踪改动、**标签与分屏布局**、运行中的程序、agent 会话。
- 三步:① 选目标主机(已连或未连都列出,不探测)→ ② Review(分支、未推送提交、改动文件、每个 pane 变成什么)→ ③ 执行。
- **Git 传输**:提交走只含目标缺少部分的 bundle;未提交改动走临时索引构造的两个临时提交,源检出/index/分支不动;忽略文件(`.env`、构建产物)与 submodule 内容不搬。
- **GitHub token 出借**:目标机够不到 `origin` 时,把本机 `gh auth token` 通过脚本 stdin 送过去,存 `.git/herdr/github-token`(600),仓库级 credential helper 从那里回答 `https://github.com`,`.envrc` 加 `export GH_TOKEN`。
- **agent 会话表**:Claude Code → `claude --resume <id>`;Codex → `codex resume <id>`;opencode → `opencode export`/`import` → `opencode --session <id>`;pi/omp → `pi --session <file>` / `omp --resume=<file>`;GitHub Copilot CLI → `copilot --resume=<id>`;Letta Code → `letta --conversation <id>`(对话留在服务器)。
- 目标机缺该 agent 或没报会话 → 先让它写交接说明 `.herdr/teleport/handoff-N.md`,随改动一起走。
- 源工作区留下一个空闲的 `teleported` shell 标签,本机在 `teleported.json` 记一笔,行上打传送图标,菜单给 `Go to teleported copy` / `Clear teleported mark`;目标副本给 `Teleport back`。

---

## 11. 智能派发(Smart Dispatch)与 Fan-out

- 超过一台主机时,**新建 worktree / 工作区**对话框会按"剩余核心数"给主机排序,前三名做成磁贴,其余在 `Other` 字段下。
- 排名:核心数 − 5 分钟负载;每个工作中的 agent 扣半核;要克隆仓库扣一核;内存超 90% 腰斩;该仓库最近八个新检出各加半核(上限三)。
- 选另一台主机就在那边建检出:找到仓库→打开或克隆,基点提交用 bundle 送过去,那边的守护进程建 worktree。
- **Fan-out** 对话框有 `Spread agents across hosts`:每条 lane 去剩余空间最大的主机,lane 算一核;host chip 可展开所有主机并搬 lane。

---

## 12. 设备(Devices)

### 12.1 Add Device 对话框
- 一个对话框管所有设备类型。左侧 rail:SSH 主机(Windows 上再列 WSL 发行版)+ **Cloud** 下各云厂商(用 logo,绿点 = 已配置,否则 `Set up`)。`Ctrl-Tab` / `Ctrl-Shift-Tab` 在 rail 里走,窄窗只显示 logo。
- **Nearby devices** 自动搜索(约 10 秒滑动进度条,`Search again` 重来):**SSH config** 的具体 `Host` 别名 + **Tailscale** 在线对端(MagicDNS,排除手机/电视/Windows)+ **Bonjour** `_ssh._tcp` 广告。已被两源同时命名的机器只列一次。
- 主机去重按 `ssh -G` 解析出的 user/host/port 三元组,`ssh 别名`、`user@address`、`ssh://` 写法算同一台。
- 添加前用非交互 SSH 检查主机:装了 Herdr 就直接保存(停着的会被启动);没装则问 "Herdr was not detected on the host. Should we install it?";版本旧则问是否更新。
- **macOS 15+** 首次搜索会要「本地网络」权限。
- 终端设置:在设备上建工作区并在里面显示安装过程,`herdr machine add` 在 app 拥有的终端里跑。

### 12.2 设备级菜单
- SSH 设备头右键:**Rename** / **Remove device…**(只改本地目录,远端 Herdr 继续跑;若该设备有自己的 GitHub 登录,确认框里附带删除选项)。
- 云设备头右键:**Rename…** / **Cloud device settings…** / **Remove device…**(Azure/Boat/Daytona/Coder 默认连机器一起删,旁有 `Remove from Herdr only`;Railway VM 只从本地移除,自己到期)。
- **`Forward port…`**:每个转发一条独立 `ssh -N` master + `ssh -O forward -L 127.0.0.1:<local>:localhost:<remote>`;特权端口 +10000(80→10080);列表显示 connecting / listening(带 `Open`,打开 `http://127.0.0.1:<local>/`)/ ended。

### 12.3 云设备
支持的厂商:**Azure**、**Boat**、**Coder**、**Daytona**、**Railway**(后四个多为实验性)。设置在 `Settings > Cloud devices`。

---

## 13. Devices Overview(设备总览标签)

- 入口:设备选择器顶部的活动卡 / 侧边栏底部 `All Devices` / `View > Devices Overview` / 命令面板。
- **活动卡**:每台设备最近两小时的 agent 活动,以及现在有几个在工作/被阻塞。
- **活动泳道**:每台设备一条,横轴 **120 分钟**,格子越深表示当时越多 agent 在工作,红色格子表示当时有 agent 在等输入,空白表示当时没连上。`View all devices` 把所有设备画在同一时间轴上。
- **汇总**:working / blocked / idle 或 done 的总数,加上在线设备数。
- **设备表**:一行一台——每 agent 一个点、工作/阻塞数、CPU、内存、家目录所在卷的剩余空间、uptime。点行 → 展开该设备上的 agent(工作优先);点 agent → 定位到它的 pane。
- 搜索框按名字/地址/agent/工作区过滤。数据来自快照,**每次开窗从空开始**,不落盘;机器休眠的分钟算空;墙钟跳变 ≥5 秒视为休眠。

---

## 14. 代码相关

- **Go to Symbol… / Go to File…**(`Cmd-Shift-O` / `Cmd-O`):聚焦 pane 的检出里列函数/类型/模块/常量与文件;Tab 切换两栏;输入前先显示 agent 正在改的文件(未提交文件按新→旧,带 `+新增 -删除` 行数)或其中的定义。Enter 在旁边开编辑器,`Cmd-Enter` 开成 code 标签。
- **Code 标签**:只读文件视图,语法着色 + 定义大纲,可点大纲跳转。`j/k`/方向键滚动,`g/G` 首尾,`o` 显隐大纲,`e` 在编辑器打开。
- **编辑器选择**:`$VISUAL` → `$EDITOR` → `vi`;`editor_command` 可覆盖,支持 `{file}` `{line}` 占位。Neovim 以 `--listen` 启动并复用同一实例(`nvim --server … --remote-expr` `:drop`)。
- **索引限制**:只索引本地检出;Rust/Swift/Ruby/Python/Go/JS-TS/Kotlin/Java/C#/Elixir/Zig 启发式解析(C/C++ 函数不列);最多 100,000 文件、300,000 符号;单文件 >1 MiB 或非 UTF-8 跳过。
- **Open Scrollback in Editor**:把 pane 的历史丢给配置的编辑器打开(需守护进程广告 `pane.edit_scrollback`)。

---

## 15. 终端链接 / 图片 / 文件拖放 / SSH 文件拷贝

- **链接**:Cmd-click 打开。三种视觉信号——虚线下划线 = 可能是链接、实线下划线 = 已验证、手型光标 = 可点击对象。验证:`http://` `https://` `mailto:` `vscode:` 纯文本按地址读;文件路径存在才验证(后台查,相对路径按 pane 工作目录解析)。文件在终端编辑器里打开并右分屏,文件夹/图片/视频/PDF 用系统默认程序。Alt(Option)按住 = 换另一种目标。`open_links_in = "browser-tab"` 可改为开在浏览器标签。
- **图片**:拖图进 pane 走 Herdr 图片桥;`Cmd-V` 贴图(文本优先),`Ctrl-V` 是 Herdr TUI 的默认贴图键。单图 **16 MiB** 上限,超了先重压缩再缩放;透明与 EXIF 方向保留;TIFF 剪贴板转无损 PNG。GUI 永远不代按 Enter。
- **文件拖放**:本地拖放只贴路径(带 POSIX 转义,空格/引号/逗号/冒号前加反斜杠);SSH 拖放会**传输文件**。上限 **256 条路径**、**64 KiB** 转义文本。
- **SSH 文件拷贝**:卡片显示 `Copying...`、已传字节、百分比、进度条、`Cancel` 按钮,完成后才把转义后的远端路径贴进原目标。64 KiB 分块 + 64 位计数(4 GiB ISO 不需 4 GiB 内存)。远端落到私有 `herdr-upload.*` 临时目录,不覆盖已有文件。**30 秒无进展则放弃**。

---

## 16. VS Code / px0

- **VS Code 标签**:读代码、看 diff,作为编辑器组里的一个标签,可与终端并排。页面来自 VS Code 自己的 `code serve-web`。
  - `Start VS Code automatically`(找到 VS Code 后默认开):跑 `code-tunnel serve-web --host 127.0.0.1 --port <port> --connection-token-file <state>/vscode-token --accept-server-license-terms`。
  - **许可必须用户点** `Accept and start`,应用不代接受。
  - token 只按路径传,从不显示、从不写日志;应用拥有该进程并在退出时结束它。
  - `Use an address` 用自建服务器(粘贴带 `?tkn=` 的地址),有 `Test connection`。
- **px0 标签**:本地工作区检出上的 px0 workbench;一个工作区一个 px0 标签。自动发现 `workbench.lock` 或启动 `px0`;`[px0] url` 指定自建地址(只接受 loopback HTTP)。仅 macOS/Windows 支持,远程工作区与 Linux 不支持。

---

## 17. 通知体系

- **应用内 toast**:守护进程的语义通知(agent 完成 / 需要注意 / 自定义 / 更新);同 pane 的新通知顶掉旧的;未读上限 **16** 条;首条播提示音;点击跳到目标 pane/标签/工作区。
- **系统通知**(`[ui.toast] delivery = "system"`):走 OS 通知中心,同样有延迟与证据校验。macOS 需 app bundle 与 bundle id(`so.pen.herdr-gpui`),裸二进制不行;Linux 走 freedesktop D-Bus;Windows 走 WinRT toast + `herdr-gpui://notification?tag=`。
- **跳过规则**:当前选中主机的活动标签的通知,仅在窗口聚焦时跳过。
- **Dock 徽标**(macOS):显示 Done 或 Blocked 的 agent 数,归零清零;覆盖所有主机和窗口(含最小化窗口),不重复计数;不随侧边栏显隐、静音、已关 toast 或投递设置变化。Windows 是任务栏按钮红色覆盖层(超过 9 显示 `9+`),Linux 无。
- **通知音**:可按事件(done / request)配不同音频,可按 agent 覆盖(`claude = "off"`),音量百分比可调。
- **没有通知历史**;旧式通知命令和服务端剪贴板写入不执行。

---

## 18. 命令面板

- 双击 Shift 或 `Cmd-Shift-P` 打开统一面板:`Cmd-P` 打开在 **Navigation**。
- 过滤:**All / Navigation / Commands / Projects**;`Ctrl-Tab` / `Ctrl-Shift-Tab`(或 `Alt-]` / `Alt-[`)循环且不清搜索。
- 匹配是 fzf 风格:每个空格分隔词都要命中,支持 `'exact` `^prefix` `suffix$` `!negate`。
- 内容:各主机上的工作区、agent 与终端 pane、本地 GUI 动作、守护进程命令、本地项目文件夹(带 frecency)、agent 状态过滤、预览面板、Resume Palette。
- `run COMMAND` 可按 `[keybindings]` 名字跑任何 app 命令;`commands` 列出可跑的名字。

---

## 19. Settings 窗口

13 个页面,4 个分组(Appearance / Behaviour / Connect / Advanced),General 单独置顶:

**General / Theme / Sidebar / Indicators / Fonts / Status Bar / Tabs / Code / Notifications / Integrations / Plugins / Cloud Devices / Configuration**

要点:
- **字体**:可搜索的已安装字体选择器,四族(Terminal / Sidebar / Tabs / Interface)可一起设或各自设(含 Platform default),字号 **8–48**,`−/+` 与可编辑数字框,即时生效。
- **主题**:内置主题 + Herdr/Ghostty 主题目录,选中即应用并保存(保留其他设置与注释);悬停预览。
- **Indicators**:状态图标可选 Dots / Solid / Outline / Tiles / Wave / Braille / Orca;可加行首 edge rail。
- **Status Bar**:Usage、CPU 和内存(隐藏后侧边栏主机行也不再显示)、网络延迟与丢包(每 15 秒探一次 1.1.1.1:443)、Keep awake(咖啡杯)、Theme、Shortcuts、Report issue。
- **Plugins**:列出插件上报的所有自定义 `$name` token(带示例与上报主机),每个 token 一个开关,控制共享 Herdr 配置里的 `rows`;带预览。
- 改动**实时生效**到所有打开的窗口;Settings 打开时终端仍可用;`Cmd-W` 关 Settings 才会保存待定偏好。

---

## 20. 凭据与 GitHub 账号

- 存储:macOS **Keychain** / Linux Secret Service / Windows Credential Manager;`gh` CLI 登录标记;`GH_TOKEN` / `GITHUB_TOKEN` 覆盖;明文文件存储只在 Linux 与 macOS dev 构建可选(**Windows 没有**)。
- PR / issues 标签需要**登录到 Herdr 自身**的 GitHub 账号,没登录时标签变淡,悬停提示去账号按钮登录。`gh` 或浏览器登录不会自动被认领。
- 头像缓存:128 槽、单项 1 MB、24 小时 stale-while-refresh。

---

## 21. 插件(Herdr Plugins)

- 动作在 Herdr 配置里以 `[[keys.command]] type = "plugin_action"` 绑定,可由快捷键 / 命令面板触发,未绑定的无法从本客户端运行。
- 选中文本:在 focused pane 有高亮时运行插件动作,`HERDR_PLUGIN_CONTEXT_JSON` 里带 `selected_text`。
- 链接处理器:插件可声明 `[[link_handlers]]` 抢链接;抢到的链接本客户端不开。
- 插件弹窗、分屏、标签都是普通守护进程终端;元数据(标签栏状态段、自定义侧边栏 token、agent 视图)按上报显示。

---

## 22. CLI(`herdr-gpui`)

通过 `$XDG_STATE_HOME/herdr/gpui/control.sock`(Windows 为命名管道)与运行中的 app 通信,只对当前用户可开。`--json` 输出单个 JSON;`--window ID` 指定窗口;退出码 0–8 有明确语义(3=app 没运行,4=暂时没有,5=东西不存在,6=有对话框或 app 忙,7=超时,8=平台/版本不支持)。

| 分组 | 命令 |
|---|---|
| 应用与窗口 | `status` `app focus/hide/quit` `window list/new/focus/fullscreen/size/detach/reconnect` `ui sidebar/statusbar/font-size/panel` `run` `commands` `open DIR` |
| 自有标签 | `tab list/focus/close` `browser open/reload/feedback/list/navigate/back/forward/zoom/text/screenshot` `code open/edit` `devices` `orchestrator [items|issues|prs]` `review` `px0 [open]` |
| 工作树与 agent | `notes list/get/set/clear` `feedback` `checkpoint list/create/diff/restore` `scripts list/setup/run` `teleport --to HOST` `fan-out PROMPT` |
| 主机 | `host list/select` `pr status/checks/comments/open` `ports list/forward/unforward/open` `usage` `system` `security [open]` `cloud list/add/setup/create/exec/destroy/wait/batches` `processes` `search symbol/file` |
| 设置与服务 | `config path/get/set/unset/check/reload` `theme` `font` `keybinds` `caffeine` `sound` `update` `integrations` `logs` `diagnostics bundle` |
| 事件 | `events [--follow --after CURSOR --kind K]` `wait tab-closed N / window` |

**原则**:命令**从不绕过确认**——恢复检查点、传送、fan-out、跑没见过的仓库脚本、创建/销毁云机器,都会打开 app 自己的对话框让用户确认。`cloud exec` 是唯一不询问的(它不确认任何东西)。

**URL scheme**:`herdr-gpui://open?path=` `herdr-gpui://device?id=` `herdr-gpui://notification?tag=`。链接是不可信输入,只做"显示",绝不执行命令。

---

## 23. 平台差异(设计时必须决策)

| 能力 | macOS | Linux | Windows |
|---|---|---|---|
| 标题栏 | 应用自绘紧凑原生标题栏,交通灯 (9,9),80px 让位,40px 头像槽 | 窗口内菜单栏,窄窗收成 `Menu` | 保留 Windows 系统标题栏(按钮/双击/拖拽吸附/Snap Layouts),透明用 DWM Acrylic/Mica |
| 内嵌页面 | WebKit | **无**,改开系统浏览器 | WebView2 |
| 徽标 | Dock 徽标 | 无 | 任务栏红色覆盖层(>9 显示 `9+`) |
| 系统通知 | `UNUserNotificationCenter`(需 bundle) | freedesktop D-Bus | WinRT toast |
| 凭据 | Keychain(dev 构建用文件存储) | Secret Service | Credential Manager(**无明文文件**) |
| SSH 终端设置 | 支持 | 支持 | **不支持**(需自己跑 `herdr machine add`) |
| SSH 连接复用 | 支持 ControlMaster | 支持 | **不支持** |
| 端口转发 / 上传 | 支持 | 支持 | **不支持** |
| Teleport / fan-out / pane 移动 / 进程列表 | 支持 | 支持 | **不支持**(需要本机 `/bin/sh`) |
| WSL | — | — | 支持,**不提供**端口转发与 Teleport |
| 快捷键 | Cmd 档 | Ctrl+Shift 档 | 从 Linux 档起步;重置字号 `Ctrl+0` |
| 开机启动 | **无此设置** | 无 | `HKCU\...\Run` |
| 字体默认 | Menlo + 系统字体 | DejaVu / 首个常见族 | Cascadia Mono(回退 Consolas)+ 系统字体 |

---

## 24. 明确的取舍(Deliberate Limitations)

- 不捆绑 Nerd Font,默认字体各平台不同 → 私用区字形可能缺失。
- **没有可拖拽的 scrollback 滚动条、没有拖拽分屏、没有内嵌图片渲染、没有光标闪烁动画**。
- 没有服务端自定义快捷键、没有会话选择器、没有编辑/停止已保存主机的 GUI(只有每设备可选的 `Use server keybindings`)。
- IME 只是最小瞬时缓冲,不是可编辑文档;组合串只有一条下划线;**不报告物理按键元数据**。
- 终端弹窗是基础居中文字呈现,**没有原生标题/边框 chrome**。
- 只有语义通知产生应用内 toast;**没有通知历史**。
- 渲染是两遍单元格画布,不是损坏行渲染器。

---

## 25. 状态与文件位置速查

| 内容 | 路径 |
|---|---|
| 主配置 | `config-gpui.local.toml`(TOML,保注释) |
| 窗口几何 | `window-state.json` |
| 编辑器组 | `editor-groups.json` |
| 浏览器标签 | `browser-tabs.json` |
| 工作树备注 | `worktree-notes.json` |
| 脚本信任 | `trusted-worktree-scripts.json` |
| 传送标记 | `teleported.json` |
| 派发历史 | `dispatch-history.json` |
| 端点目录 / 选择 | `client/endpoint-selection.json` |
| 日志 | `logs/herdr-gpui.jsonl`(16 MiB 轮转) |
| 崩溃报告 | `crash-YYYYMMDD-HHMMSS.mmm-PID.txt`(保留最新 5 份) |
| 头像缓存 | 128 槽 / 1 MB / 24h |
| 状态根目录 | `$XDG_STATE_HOME/herdr/gpui`,默认 `~/.local/state/herdr/gpui`;Windows `%LOCALAPPDATA%\herdr\gpui` |

---

## 26. .pen 设计的对齐状态

`docs.pen/01.pen` 现有 **34 个画板**(2026-10-22 完成),全部按本文件第 1–25 节与 `crates/herdr-gpui` 源码(`a64c6eb`)对齐。

### 26.1 画板清单

| 行 | y | 画板 |
|---|---|---|
| 1 | -297 | Herdr · macOS / Command Palette / Settings |
| 2 | 743 | Overview / Pull Requests / Issues / Usage |
| 3 | 1783 | Zen Mode / Split Panes / Notifications / Transient States / Agent Queue / Zen Mode · Fullscreen |
| 4 | 2823 | Find Bar / Annotate / Devices Overview / Ports and Processes / First Run |
| 5 | 3863 | Review Changes / Browser & App Tabs / Editor Groups / Dispatch · Fan-out / Teleport Wizard |
| 6 | 4903 | Security / Logs Window / Update Panel / New Worktree / Add Device |
| 7 | 5943 | About / Settings · Plugins / Settings · Notifications / Settings · Integrations |
| 8 | 6983 | Sidebar Layouts / Settings · Integrations · Empty |

预览图:`design/` 下 34 张 PNG(与画板同名 kebab-case)。

### 26.2 已对齐

| 项 | 对齐结果 | 依据 |
|---|---|---|
| 活动栏宽度 | 44px(`BAR_WIDTH`) | `sidebar/activity.rs:31` |
| 活动栏分区 | 6 项:Workspaces(terminal)/ Overview(activity)/ Pull requests(git-pull-request)/ Issues(circle-dot)/ Security(shield)/ Usage(chart-column) | `ActivityPanel::ALL` + `panel.icon()` |
| 活动栏按钮 | 32×32、圆角 8(`corners::CONTROL`)、图标 17px、gap 4、padding 6 | `BUTTON` / `ICON` / `corners::CONTROL` |
| 活动栏选中态 | 白色半透明圆角方块 + 柔和投影,**不画左侧竖条** —— 有意偏离源码的 primary wash + 竖条,向 Codex 对齐 | `bar_button` 的 `BarState::Selected`(故意改样式) |
| 计数气泡 | 14×14 起(两位 17px)、圆角全圆、9px 字、贴在图标右上外侧 | `count_bubble` + `BUBBLE_END` |
| 底部设置按钮 | 活动栏底部(弹性占位之后) | `render_activity_bar` 末尾的 `activity-settings` |
| 标题栏高度 | 34px | `titlebar::HEIGHT` |
| 交通灯 | (9, 9),12px 三灯 20px 间距 | `TitlebarOptions::traffic_light_position` |
| 交通灯让位 | 80px(`LEADING`) | `titlebar::LEADING` |
| 侧边栏开关 | 28×28 命中区 + lucide `panel-left` 16px 单色图标 —— 有意偏离源码的 18×14 描边字形,向 Codex 对齐 | `sidebar_glyph`(故意改样式) |
| 后退/前进 | 两个 16px chevron 直接排开,**无边框、无分割线** —— 有意偏离源码的 macOS 分段控件,向 Codex 对齐 | `navigation::Style::Segmented`(故意改样式) |
| 账号槽 | 右上 40px,20px 圆形头像内含 12px GitHub 标记 | `titlebar-account-slot` / `AVATAR` |
| Settings 窗口 | 13 个页面 / 4 个分组(General 置顶,APPEARANCE·BEHAVIOUR·CONNECT·ADVANCED),独立窗口(无活动栏) | `settings_window::Section` / `Group` |
| Settings 行 | 标签在上、说明在下的两行结构;行内来源标签(Herdr / Herdr UI)、分组标题全大写 | `settings_window/ui.rs` 的 `section()` / `row()` |
| Zen 交通灯 | 窗口态保留在 (9, 9),顶部留 28px 让位带;全屏态取消 | `titlebar::ZEN_INSET = 28` |
| 命令面板过滤 | 5 个:All / Navigation / Commands / Projects / Notes,右侧 `{filtered} of {entries}` 计数 | `palette.rs:71` `enum Filter` |
| 命令面板行 | 两行(标题 + 详情)+ 右对齐徽标(Workspace / Tab / Terminal / GUI action / Herdr command / Project / Note) | `palette/go_to.rs` / `palette/tests.rs` |
| 命令面板页脚 | Navigation 用长提示(⌥B/W/I/D/A 状态、⌥P 预览),其余用短提示 | `palette/render.rs:11-13` |
| 命令面板空态 | `No matching results. Try a shorter search.` | `palette/render.rs:397` |
| 侧边栏布局 | 10 种画全:normal / compact / comfortable / normal-rounded / compact-rounded / comfortable-rounded / superset / orca / minimal / devices | `config/layout.rs` `enum LayoutMode` |
| Overview 分区 | Needs you 卡片(等待 + 失败运行)→ Ready for review → Working(按主机)→ Idle → 隔断 → Snoozed / Settled / Closed 架子 | `overview/view.rs:150-300` |
| Overview 行操作 | 活动行 Snooze / Settle / Pin / Open;Closed 行 Reopen / Forget;Snoozed 行 Wake now | `overview/view.rs:552-621` |
| Overview 卡片配色 | Needs you 卡片描边偏红,行状态点红/黄/灰 | `overview/view.rs` 的 `mix(background, blocked, …)` |
| Devices Overview | 活动泳道(每设备一行 + All devices 汇总行,近 2 小时)+ Totals(Working / Blocked / Idle or done / Devices online)+ 设备表(Agents / CPU / Memory / Disk free / Uptime)+ 搜索框 + Add device | `devices_overview/chart.rs` / `view.rs` / `table.rs:22` |
| Settings · Notifications | DELIVERY / IN-APP TOASTS / SOUND 三分区,行内来源标签,分段控件 Off·Herdr·Terminal·System,Delay 数值框,Position 下拉,Volume 滑块 | `settings_window/pages/notifications.rs` |
| Settings · Plugins | `Search plugin values...` + SIDEBAR VALUES 分区 + 左侧值开关列表 + 右侧 Preview 面板 + 底部限制说明 | `settings_window/controls/plugins.rs` |
| Settings · Integrations | `Search integrations by name or ID...` + `Daemon host: …` + 两条说明 + `Refresh integrations` + 行(标签 / `{status} | {command}` / `Agent not available on this host` / Install·Reinstall) | `integrations.rs:355-420` |
| Integrations 空态 | `No integrations reported by this daemon.` | `integrations.rs:380` |
| Security | 标题 + 副标题 + Updated {ago} + Refresh + 搜索占位 + 筛选 pill + 卡片式列表(72px 严重度列)+ 空态 | `security/pane.rs` |
| Logs 窗口 | 1100×650(最小 620×360),搜索占位 + 级别按钮 `Minimum: {LEVEL}` + Pause / Copy / Export… + 页脚 `{n} shown \| {n} dropped \| LIVE\|PAUSED` | `log_window/` |
| About | `Herdr` / `Version {APP_VERSION}` / 免责声明 / 3 个链接 / `© 2026 Fabien Penso · Apache-2.0` / OK | `about_window/` |
| Update 面板 | 480px 宽,`App Updates` / `Updates from GitHub` / Current·Latest version / `What's new` / 页脚 Manual Releases·Release Notes·Download | `update_panel/` |
| 预览图导出 | 33 张 PNG,带 210px 透明阴影留白(根 frame 阴影 blur 70 + offset y 22) | pen.dev Export |

### 26.3 仍未对齐(有意保留)

| 项 | 真实实现 | 设计取舍 |
|---|---|---|
| 自有标签类型 | Blank / Web / File / Code / Review / Orchestrator / Devices / Editor(VS Code)+ px0 | 设计把 Orchestrator / Devices 画成独立画板,未在标签条上区分 |
| 侧边栏密度命名 | README 写 "nine choices",实际 `LayoutMode::ALL` 有 10 个 | 以源码为准,画了 10 个 |
| 活动泳道分辨率 | 真实为 120 分钟 1 格 | 设计每格代表 2 分钟(60 格),以控制节点数 |
| 集成项名称 | 由 daemon `integration.list` 运行时下发 | 设计用 Claude Code / Codex / Gemini CLI 作示例 |
| 命令面板过滤 | 5 个过滤,切换时内容不同 | 画板只展示 Navigation 选中态 + 其二级 agent 状态 chip |
| Rename / Close 确认 | 需键入 `close` 的确认框、Open worktree 搜索框 | 未单独出画板,仅有概念 |

---

## 27. 视觉语言:Arc 色板 × 苹果美学

第 26 节对齐的是**信息架构与文案**;本节记录**视觉语言**的来源与落地规则,供后续改版时保持一致。

### 27.1 色板来源

主色板取自 [open-design.ai — Design System: Arc](https://open-design.ai/plugins/design-system-arc/)(抓取时间 2026-10-22),按 Arc token 原名映射为 .pen 变量:

| Arc token | .pen 变量 | 值 | 用途 |
|---|---|---|---|
| `--bg` | `$win-bg` | `#FDF3EC` | 窗口底色(暖奶油) |
| `--surface` | `$surface` | `#FFFFFF` | 白色面板底 |
| `--surface-warm` | `$chip` | `#FFF4EA` | 暖色 chip / 药丸标签 |
| `--fg` | `$text-primary` | `#1A1A1F` | 主文字 |
| `--fg-2` | `$text-secondary` | `#54545A` | 次级文字 |
| `--muted` | `$text-tertiary` | `#8C8C93` | 三级文字 / 未选中图标 |
| `--border` | `$separator` | `#ECE5DB` | 发丝分隔线 |
| `--accent` | `$accent` | `#FF5F5F` | 品牌珊瑚红 |
| `--success` | `$green` | `#48BB78` | 成功 / 交通灯 zoom |
| `--warn` | `$yellow` | `#F6AD55` | 警告 / 交通灯 minimize |
| `--danger` | `$red` | `#F56565` | 危险 |
| Arc Lavender | `$mauve` | `#B794F4` | 淡紫强调 |
| accent 16% | `$accent-wash` | `#FF5F5F29` | 选中行底色 |

### 27.2 材质与层次(苹果美学)

| 元素 | 做法 |
|---|---|
| 根画板 | 斜向线性渐变 `#E4DBFA → #F8D2E5 → #FFEDD9`,rotation 135,`cornerRadius` 12(macOS 窗口圆角),`clip` |
| 活动 rail(44px)/ 侧边栏(248px) | **透明**,直接透出根画板的斜向渐变 —— 整条左侧色带是一块连续色场,没有接缝 |
| 主面板 | **磨砂玻璃卡片**:`$panel` = `#FFFFFFB3`(Arc Glass Light 0.7)+ 玻璃描边 `#FFFFFF66` + `cornerRadius` 16 + 投影 `0 8px 32px rgba(0,0,0,0.12)`,半透明白叠在渐变上,色相能透出来 |
| 窗口工具条(34px) | 命名 `<前缀> Window Toolbar`,`fill: transparent`,无描边;交通灯 12px @ (9,9),红/黄/绿 = `$accent`/`$yellow`/`$green` |
| Body 布局 | `padding: [10,14,14,0]`、`gap: 14` —— 工具条与面板之间留 10px 呼吸位,面板右侧/底部各留 14px,栏间 14px |
| 玻璃层级 | Heavy `0.85` → `#FFFFFFD9`(浮层:命令面板 / 对话框 / toast);Light `0.7` → `#FFFFFFB3`(主内容面板 `$panel`);Medium `0.45` → `#FFFFFF73` |
| 圆角 | 8 / 12 / 16 / 9999(Arc 规范);窗口外框 12 |
| 窗口边缘 | 1px `#FFFFFF59` 高光描边 —— 苹果玻璃窗的镜面边缘 |

### 27.3 饱和渐变的使用护栏

遵循 Arc 的「渐变只用于 shell / 主 CTA / 一个焦点面板」:

- **Sunset 渐变** `#FF7E5F → #FEB47B` 仅用于 28 处主 CTA(Add device、Open、Reopen、Refresh 等),白字、`radius 12`、投影 `0 4px 16px rgba(255,127,95,0.3)`;
- **Twilight 色相**只以淡色版出现在 rail/侧栏渐变里,不用于按钮;
- 其余界面全部使用纯色 + 发丝描边,不做彩色铺底。

### 27.4 排印

- 字体:UI = Inter(`$font-ui`),等宽 = JetBrains Mono(`$font-mono`);
- 字号按苹果档位上调一档:10.5→11、11→11.5、11.5→12、12→12.5,正文 13 / 标题 18 / 页面大标题 22;
- 行高比 1.55,紧标题 1.05;文本不换行处一律用 `textGrowth: fixed-width` + `fill_container` 以免触发裁切。

### 27.5 本轮修复记录

| 问题 | 处理 |
|---|---|
| 底部 Meter Row 溢出主面板 24px | 三条用量条 38→28px,填充按比例缩放(10/30/3 → 8/22/2),行宽改回 `fill_container` |
| 25 个标题栏的交通灯 / 前后翻页图标 / 窗口标题全部透明(`#00000000`) | 按语义补色:交通灯 `$accent`/`$yellow`/`$green`,导航图标 `$text-tertiary`,标题文字 `$text-secondary`(共 182 个节点) |
| Zen 提示卡、通知提示块正文贴边、底边无内边距 | 补 `padding`,正文改 `fill_container`;Zen 提示卡上移到 y=818 避免超出画板 |
| 全局审计 | 4927 节点 / 0 裁切 / 0 未命名 / 0 placeholder / 0 重名 / 33 个根画板 |

---

## 28. 窗口外壳布局:Codex 式「渐变壳 + 白色圆角面板」

第 27 节解决"长什么样",本节解决"怎么排"。外壳先后经历两轮结构调整,最终形态是 **Codex(macOS)式:一块渐变/纯色窗口底 + 一张白色大圆角内容面板**。

### 28.1 最终结构

```
根画板(斜向渐变 #E4DBFA→#F8D2E5→#FFEDD9,radius 12,clip)
├── … Window Toolbar(34,透明)              ← 交通灯 / 开关 / 前进后退 / 标题 / 账户
└── Body(padding [10,14,14,0],gap 14)
    ├── Rail(44)    transparent ─┐  两者都透明,直接坐在根画板的渐变上,
    ├── Sidebar(248) transparent ┘  与工具条左端连成同一块连续色场
    └── Main / Content
         纯白 $surface · cornerRadius 16
         投影 0 8px 32px rgba(0,0,0,0.08)
         → 悬浮在渐变底上,右侧/底部各留 14px,顶部留 10px 呼吸位
```

### 28.2 关键决策

1. **左侧色场合一**:rail 与 sidebar 不再各自带一条渐变。渐变统一提到根画板,两者设为透明 —— 因此「工具条左端 / rail / sidebar」是同一块连续色场,不存在色差断层(这是任何留白方案都必须一起解决的问题)。
2. **工具条透明**:顶条不画自己的底色,透出根渐变,于是"浮在上面的条"消失了,和左栏连成一片。
3. **主内容 = 一张白色圆角卡片**:圆角 16,投影 `0 8px 32px rgba(0,0,0,0.08)`,与工具条之间留 10px。面板与窗口是两层,不是五层。
4. **侧栏不做卡片化**:侧栏内容直接坐在渐变上,保持通透;若再给侧栏加一张卡,就会退回"卡片套卡片"。
5. **栏内固定宽度回退**:主面板宽度因此回到 1106(有侧栏)/ 1368(无侧栏),与初版一致,固定宽度的表格与工具条无需再改。

### 28.3 覆盖范围

| 分组 | 画板 | 处理 |
|---|---|---|
| 主窗口型(Rail [+Sidebar] + Content) | 22 张 | 标准转换 |
| Settings 型(Categories 210 + Pane) | 5 张 | Pane 卡片化,Categories 留在渐变上 |
| 幽灵窗口 | Command Palette、Notifications | 幽灵体按真实窗口结构重排 |
| 单栏工具窗口 | Logs Window、Sidebar Layouts | 纯白窗口 / 卡片浮在渐变上 |
| 专注模式 | Zen Mode | 居中大圆角列(cornerRadius 20)+ 同款投影 |

### 28.4 工具条与活动栏:向 Codex 对齐

在把外壳改成"渐变壳 + 白色圆角面板"之后,窗口左上角的**工具条控件**和**活动栏**仍保留了旧版做法,与 Codex 的观感差距明显,已统一改写:

| 部位 | 旧做法 | Codex 做法(现) |
|---|---|---|
| 侧栏开关 | 手搓图形(18×14 描边圆角框 + 5px 实心竖条),小尺寸下糊成一个黑块 | 换成 lucide `panel-left` 图标 16×16,`$text-secondary`,单色描边 |
| 前进/后退 | 63px 宽的**带边框分段控件**(圆角 6 + `$separator` 描边 + 中间 1px 竖线) | 去掉容器与竖线,两个 16px chevron 直接排开;后退 `$text-secondary`,前进 `$text-tertiary`(置灰感) |
| 活动栏选中态 | `$accent-wash` 珊瑚色底 + 左侧 2px 珊瑚竖条(`Rail Selection`) | 选中项 = 白色半透明圆角方块 `$selection` + 柔和投影;**删除左侧竖条** |
| 活动栏图标 | 选中 `$text-primary` / 未选中 `$text-tertiary` | 保持不变(与 Codex 一致) |
| 计数徽标 | 珊瑚色圆点徽标 | 保留(功能数据,品牌强调),未做中性化处理 |

改动覆盖 25 个活动栏 + 25 套工具条(共 75 个导航部件、15 个侧栏开关、24 个选中态、55 个节点删除)。First Run 板原有的 `FR Sidebar Toggle Icon` 与新命名撞车,改名为 `FR Sidebar Header Toggle Icon`。

### 28.5 踩坑记录

- Body 加 10px 顶部内边距后,活动栏净高减少 10px,底部绝对定位的「设置」按钮溢出被裁 —— 25 个 rail 的按钮统一改为 `y = 栏高 − 42`,重新贴底。
- Annotate 板的底部条同样因容器变矮而溢出,上移到 `y = 798`。
- 工具条改名 `<前缀> Window Toolbar` 以避免与板内既有 `PR Toolbar` 等内部节点重名。

### 28.6 改造后的校验

4927 节点 / **0 裁切** / 0 未命名 / 0 placeholder / 0 重名 / 33 个根画板。

---

## 29. Zen 模式:真实行为与两张画板

「Zen 模式是不是全屏?」—— 源码给的答案是 **默认不是**。

### 29.1 源码依据

| 事实 | 出处 |
|---|---|
| Zen 的全部行为由 `[zen]` 配置决定 | `config/zen.rs:1` "what Toggle Zen Mode does beyond clearing the window's chrome" |
| **全屏是可选的**,默认 `false`;开启后进入原生全屏,退出时再退回来(除非本来就是全屏) | `config/zen.rs:11-14` `Zen::fullscreen` + `window/zen.rs:184-186` `window.toggle_fullscreen()` |
| Zen 清掉的东西:标题头、侧边栏、标签条、状态栏、worktree 横幅、通知条 | `window/zen.rs:1-3` |
| 退场顺序:状态栏先滑走(220ms),再是侧边栏(280ms);进场是倒放 | `window/zen/chrome.rs:9-15` |
| 窗口态下顶部保留 **28px 空带**(`ZEN_INSET`)给 macOS 交通灯让位,同时兼作拖拽区;全屏时不需要 | `titlebar.rs:485-503` `zen_inset` |
| 居中列宽度可配:`[zen] width` 支持列数或 10–100% 百分比;**不设即占满整宽** | `config/zen.rs:15-16` |
| 字体大小也可配:`[zen] font_size` | `config/zen.rs:17-19` |
| 离场提示文案:`Zen mode: {key} to leave`,macOS 快捷键 **⌃⌘Z** | `window/zen.rs:217` + `controls/shortcuts/macos.rs:46` |
| Zen 仍然保留 toast 与菜单 | `window/render.rs:728-730` |

### 29.2 两张画板

| 画板 | 对应 | 画面 |
|---|---|---|
| **Zen Mode** | 默认形态(`[zen] fullscreen` 未设) | 窗口保留圆角 12 与交通灯,顶部 28px 让位带,内容从 y=28 起;窗体填充 = 终端底色,居中列 700px;无侧栏、无活动栏、无标签条、无状态栏;顶部有 `Zen mode: ⌃⌘Z to leave` 的 flash,右下角有说明卡 |
| **Zen Mode · Fullscreen** | `[zen] fullscreen = true` | 全出血(无圆角),**没有交通灯、没有 28px 让位带**,内容从 y=0 起;列宽仍居中;说明卡改为全屏文案 |

原先这张画板画成"渐变壳 + 白色悬浮大圆角卡片",既不符合实现(真实 Zen 模式里整窗就是终端底色,没有卡片),也误解了快捷键(⌘⇧F → ⌃⌘Z),本轮已按源码重画。

---

## 30. 磨砂材质:苹果式 vibrancy

窗口不再用"实色卡片贴在渐变上",而是改成**半透明磨砂**:面板本身只有 70% 白,底下的渐变会透上来,于是同一张面板在左上偏冷、右下偏暖 —— 这就是苹果 `NSVisualEffectView` 的观感。

### 30.1 做法

`.pen` 没有真正的 backdrop-filter,磨砂靠三件套表达:

| 要素 | 值 | 作用 |
|---|---|---|
| 半透明底 | `$panel` = `#FFFFFFB3`(Arc **Glass Light**,0.7 白) | 让底下的渐变色透进面板 |
| 玻璃描边 | `stroke #FFFFFF66` + `strokeWidth 1` | 面板边缘的一道镜面高光 |
| 柔和投影 | `0 8px 32px rgba(0,0,0,0.12)` | 把面板从背景里"抬"起来,补足没有真实模糊的失真 |

底下的渐变本身也加深了一档(`#DCCFF9 → #F7CDE3 → #FFE7CE`),否则 70% 的白会把颜色洗没,磨砂就看不出层次。

### 30.2 分层

| 材质 | 透明度 | 用在哪 |
|---|---|---|
| 玻璃 Heavy | 0.85 → `#FFFFFFD9` | 浮在暗化遮罩上的层:命令面板、7 种对话框、4 个 toast |
| 玻璃 Light | 0.70 → `#FFFFFFB3`(`$panel`) | 窗口内的主内容面板(31 个) |
| 玻璃 Medium | 0.45 → `#FFFFFF73` | 活动栏、侧边栏底 |
| 终端底 | 不透明 `$terminal-bg` | Zen 模式整窗 —— 真实终端本就是不透明的 |

窗口外框统一加 1px `#FFFFFF59` 高光描边,呼应苹果玻璃窗的镜面边缘。

### 30.3 保留实心的地方

Zen 模式的两张画板整窗是**不透明**的终端底色:真实实现里 zen 模式就是把窗口全部让给终端(`window/zen.rs:1-3`),终端本身没有材质,加磨砂反而不对。

---

## 31. 可访问性与交互态规范(2026-10-22 审查后)

一次跨领域审查(`/better-interface`)发现 15 条问题,已在 35 张画板上全部修完。本节记录最终规范。

### 31.1 语义色重新定档

原 token 按色相挑选、没按「实际压在什么底色上」验过,导致 669 / 1528 个文本节点(44%)低于 WCAG AA。现在按**实测对比度**重新定档,并把「彩色本体」和「彩色文字版」拆成两组 token:

| 用途 | token | 值 | 用途 | token | 值 |
|---|---|---|---|---|---|
| 正文 | `$text-primary` | `#1A1A1F` | 彩色强调文字 | `$accent-text` | `#A62B1E` |
| 次级 | `$text-secondary` | `#4A4A51` | 绿色文字 | `$green-text` | `#1C6B3D` |
| 三级 | `$text-tertiary` | `#5E5E67` | 黄色文字 | `$yellow-text` | `#7A4E0C` |
| 填充底 | `$accent` | `#FF5F5F` | 红色文字 | `$red-text` | `#B3261E` |
| 承载白字 | `$accent-deep` | `#C8382F` | 深绿底 | `$green-deep` | `#1C6B3D` |
| 开关开 | `$accent-track` | `#E84545` | 开关关轨道 | `$track-off` | `#E4E0D9` |
| | | | 开关关滑块 | `$knob-off` | `#6E6A63` |

关键约束:

- 任何**承载白字**的填充都要 ≤ L 0.18,白字才能到 4.5:1 → 按钮与徽标用 `$accent-deep`(#C8382F,白字 5.17:1),不用 `$accent`。
- `$panel` / `$chip` / `$accent-wash` 这类**背景与淡色** token 一律不得进文字槽位(曾出现 1.06:1 的隐形文字)。
- 状态色各留一个「彩色本体」和一个「深色文字版」:圆点、描边用前者,正文用后者。

### 31.2 开关的三层对比度

关闭态原来是「浅色轨道 + 白滑块」,轨道对面板 1.03:1、滑块对轨道 1.08:1 —— 整颗开关等于不存在。改成:

- **关**:`$track-off` 浅轨道 + `$knob-off` **深滑块** → 滑块对轨道 4.14:1、对面板 5.00:1
- **开**:`$accent-track` 轨道 + 白滑块 → 滑块对轨道 3.91:1、轨道对面板 3.59:1
- 尺寸 30×18 → **30×24**,圆角 9 → 12,滑块 14 → 16(同时满足 24×24 命中区)

`.pen` 的 ellipse 不支持 `effect`,所以滑块没有投影,对比度必须自己扛住。

### 31.3 字号刻度

18 个字号(含 5 个半像素,737 个节点)收敛为 7 档**整数**刻度:

| 档 | 10 | 11 | 12 | 13 | 14 | 16 | 20 |
|---|---|---|---|---|---|---|---|
| 用途 | 密集数据 / 徽标 | UI 标签 | 正文 | 导语 / 空状态说明 | 小节标题 | 页面标题 | 数值 / 大标题 |

半像素字号(9.5 / 10.5 / 11.5 / 12.5 / 13.5)全部并入整数档 —— 它们在不同 DPR 下会落错半个像素。15px 的长段落降到 13,短标题升到 16,顺带修掉了「页面标题和正文一样大」的层级问题。5 个多行文本块另设 `lineHeight 1.45`。

### 31.4 命中区与图标

- 所有纯图标控件的命中框 ≥ **24×24**(WCAG 2.5.8):标题栏前后翻页 20×22 → 24×24,行尾 chevron 12×28 → 24 宽,行内图标 20×20 / 24×22 / 30×21 → 24 高,38×22 → 38×24。原来有 107 个不达标,现在 0 个。
- 图标按**角色**定尺寸:标题栏与工具条 16、侧栏树 14、活动栏 17、空状态插画 20/22/26、端口小点 10、品牌标 44。原来 14 种尺寸混用。

### 31.5 间距与圆角刻度

- `gap` 16 个取值 → 8 个(`2 / 4 / 8 / 12 / 16 / 24 / 32 / 48`);`padding` 的小值同刻度。
- `cornerRadius` 18 个取值 → 8 个(`2 / 4 / 6 / 8 / 10 / 12 / 16 / 20`)。

### 31.6 分段控件的段间距

三处分段控件的容器 `gap` 是 0,渲染出来是一串连在一起的字:

- `UncommittedBranch` → `Uncommitted · Branch`
- `UnifiedSide by side` → `Unified · Side by side`
- `Off|Terminal|System` → `Off · Herdr · Terminal · System`

修法是给 label 与 value 之间加 `gap: 6` 并插入 `·` 分隔符;选项之间加 `gap: 4`,选中药丸加左右内边距。

### 31.7 新增画板:Interaction States

第 35 张画板(`Interaction States`,网格第 9 行左起第 1 位),两张面板:

- **Interaction states** —— 图标按钮 / 开关 / 主按钮 / chip / 分段选项各自的 default、hover、focus、pressed、disabled 五态;焦点环是 2px `$accent` + 2px 外扩;禁用态保持几何尺寸不变(行不会回流)。底部附六条规则(焦点环、hover、pressed、disabled、动效、命中区)。
- **Icon controls → accessible name** —— 18 个纯图标控件(role → 朗读名)的对照表:活动栏 7 项、标题栏 6 项、行 chevron、新建 worktree、打开/关闭文件、toast 徽标、空状态标记(装饰性,不入无障碍树)。

`.pen` 格式本身承载不了 `aria`,所以这一页是**交付给实现的规格**:每个图标控件都要有可朗读名称,实现时对应 `aria-label`。

### 31.8 状态栏用量计

原来三条计只有百分比和时长,看不出各是什么额度。按源码 `usage/model.rs:200-209` 的窗口种类(会话 `5h` / 周 `wk`)给每条读数加前缀:

```
5h 27% used 0m · 5h 86% used 3h 21m    wk 78% used 5d 3h    5h 9% used 0m · wk 34% used 4d 0h
```

加上标签后状态栏变长,已把整条状态栏的字号收到 10px、日期缩为 `10-20 11:24`,内容宽 1090 < 可用 1102,不再被裁切。

### 31.9 文案

26 个全大写分区标签改句首大写(`Appearance` / `Behavior` / `Connect` / `Advanced` / `In-app toasts` / `SSH hosts` …),英式 `BEHAVIOUR` 统一为美式 `Behavior`。

### 31.10 修完后的审计

| 指标 | 修前 | 修后 |
|---|---|---|
| 低于 WCAG AA 的文本节点 | 669 / 1528(44%) | **0 / 1618** |
| 低于 24×24 的命中框 | 107 | **0** |
| 半像素字号节点 | 737 | **0** |
| 字号档数 | 18 | **7** |
| `fontWeight` 误用 token | 35 | **0** |
| 圆角档数 / 间距档数 | 18 / 16+60 | **8 / 8** |
| 重名 / 未命名 / placeholder 节点 | 0 / 0 / 0 | **0 / 0 / 0** |
| 根画板 | 34 | **35** |

`fontWeight` 误用的 35 个节点是**字体族 token 被写进了字重槽位**,已改为 `fontFamily: $font-mono` + `fontWeight: normal`,与同栏其它分支名一致。

---

## 32. 暗色模式(2026-10-22)

亮色稿定了 35 张画板之后,同一批画板又各做了一份暗色变体,加上 1 张暗色系统板,画布上共 **71 张根画板 / 10372 个节点 / 70 个变量**。

### 32.1 为什么不用 theme 轴

`.pen` 的 `themes?` 字段没有可用的切换 API(编辑器侧不暴露 theme 切换动作),所以暗色不能用同一张画板换主题实现。改走**独立 token + 独立画板**:

- 亮色 35 张留在原网格(x = -200 起,6 列 × 1580,行距 1040);
- 暗色 35 张复制到右侧独立网格(x = 9260 起,同样 6 列 × 1580 / 行距 1040),便于左右并排比对;
- 第 71 张 `Dark Mode · Palette & Materials` 系统板放在 x = 18740,列出色板、对比度表、token 对照和材质规则。

暗色板的后代节点统一命名为 `<板名> Dark <原名>`,保证全局零重名。

### 32.2 暗色 token(35 个,新增)

**表面层级**

| token | 值 | 用途 |
|---|---|---|
| `$dark-win-bg` | `#221F2E` | 窗口基底 |
| `$dark-titlebar` | `#2A2636` | 标题栏 |
| `$dark-panel` | `#2A2636CC` | 磨砂面板(80% alpha) |
| `$dark-surface` | `#1B1922` | 实心表面 |
| `$dark-elevated` | `#2E2A3A` | 浮层 / 提示卡 |
| `$dark-overlay` | `#1E1C28D9` | 模态遮罩、命令面板 |
| `$dark-terminal-bg` | `#16141C` | 终端底色 |
| `$dark-rail-bg` | `#FFFFFF0F` | 活动栏玻璃 |

**文字三级**

| token | 值 | 在面板上的对比度 |
|---|---|---|
| `$dark-text-primary` | `#F3F1F8` | **13.41:1** |
| `$dark-text-secondary` | `#C3BED3` | **8.32:1** |
| `$dark-text-tertiary` | `#ADA8C0` | **6.54:1** |

**强调与状态**

| token | 值 | 说明 |
|---|---|---|
| `$dark-accent` | `#FF7A6E` | 主强调:图标 / 激活态 / 焦点环(5.91:1) |
| `$dark-accent-deep` | `#C93A31` | 徽标底色,配白字 **5.08:1** |
| `$dark-accent-text` | `#FF9E93` | 强调文字、链接(7.56:1) |
| `$dark-accent-wash` | `#FF7A6E29` | 强调底纹(装饰,不承载文字) |
| `$dark-on-bright` | `#2A1010` | 亮色面上的深字,在 `$dark-accent` 上 **7.00:1** |
| `$dark-green` / `$dark-green-text` | `#4ADE80` / `#7BE8A8` | 成功状态 / 成功文字 |
| `$dark-green-deep` | `#2F7D4F` | 成功徽标底色,配白字 **5.04:1** |
| `$dark-yellow` / `$dark-yellow-text` | `#F5C26B` / `#F7CE85` | 警告状态 / 警告文字 |
| `$dark-red` / `$dark-red-text` | `#FF7B72` / `#FF9D95` | 错误状态 / 错误文字 |
| `$dark-blue` | `#6FB3FF` | 信息 / 链接 |
| `$dark-mauve` | `#C9A0FF` | 次要强调 |
| `$dark-cyan` | `#6FD6E8` | 终端前景 / 高亮(终端上 10.83:1) |

**底纹与控件轨道**

`$dark-yellow-wash` `#F5C26B1A` · `$dark-green-wash` `#4ADE801A` · `$dark-red-wash` `#FF7B721A` · `$dark-blue-wash` `#6FB3FF2E` · `$dark-red-stroke` `#FF7B7280` · `$dark-accent-track` `#E05B4E` · `$dark-track-off` `#3A3648` · `$dark-knob-off` `#B6B1C4`

开关两态:开 = 轨道 `$dark-accent-track` + 白滑块(**3.64:1**),关 = 轨道 `$dark-track-off` + 滑块 `$dark-knob-off`(**5.59:1**)。

### 32.3 材质规则

1. **窗口底** —— `$dark-win-bg` 之上叠 135° 渐变 `#1E1638 → #2E1836 → #35201A`,模拟壁纸透光。暖紫到暖褐的偏移提供温度感,避免纯灰的冷硬。
2. **磨砂面板** —— `$dark-panel` 以 80% alpha 叠在渐变上,合成后比基底亮 2–4%,抬起感来自「比背景更亮」而不是加投影。
3. **分隔与描边** —— 统一用白色叠加透明度:`#FFFFFF1A` 行分隔、`#FFFFFF21` 窗口外框、`#FFFFFF26` 浮层描边、`#FFFFFF1F` 卡片边框,禁止硬编码灰线。
4. **覆盖层** —— `$dark-overlay` 比面板更暗,形成前后景深,背景内容仍可辨认。
5. **终端** —— `$dark-terminal-bg` 是画布里唯一的纯黑区,比面板深一档,终端前景与 ANSI 色 ≥ 7:1。
6. **控件轨道** —— 开关的两态轨道/滑块颜色见上表,焦点环用 `$dark-accent` 2px + 2px 外扩。

### 32.4 暗色板是怎么生成的

1. 复制亮色板(后代自动获得新 id);
2. 按 token 映射表重指:`$win-bg→$dark-win-bg`、`$panel→$dark-panel`、`$text-*→$dark-text-*`、`$accent→$dark-accent` 等共 28 组;
3. 硬编码色按暖色阶重映射(如 `#FDE7DC→#2A2028`、`#F8B79E→#4A2A1C`),交通灯色 `#FF5F57` / `#FEBC2E` 保持不变;
4. 渐变与投影整体替换(亮色 `#DCCFF9,#F7CDE3,#FFE7CE` → 暗色 `#1E1638,#2E1836,#35201A`;阴影 `#00000059` → `#000000A6`);
5. **白字上下文重判**:承载白字的底色若合成后亮度 L > 0.25,文字改用 `$dark-on-bright`,否则保留白字;
6. 暗色板只允许引用 `$dark-*` token —— 生成后全量扫描确认暗色板里**没有**任何一个亮色 token 残留。

### 32.5 暗色阶段修掉的问题

**对比度 25 → 0**(逐节点按 `.pen` 实际 alpha 在根画板渐变上合成后计算 WCAG 相对亮度)

| 症状 | 处数 | 处理 |
|---|---|---|
| 浏览器内嵌页 / zen flash 的白底配深色文字(1.12–2.55:1) | 9 | 页面底改 `$dark-surface`,页头与遮罩卡改 `$dark-elevated` |
| Find Bar 黄色高亮上的深色文字(1.46:1) | 3 | 改 `$dark-on-bright` |
| 重连横幅说明 `#7A5A16`(1.88:1) | 1 | 改 `$dark-yellow-text` |
| 禁用态主按钮标签 `#6F6A78`(2.27:1) | 1 | 改 `$dark-text-tertiary` |
| 分段控件选中药丸底色误用 `$dark-text-tertiary`(2.28:1) | 1 | 改 `$dark-selection` |
| hover 主按钮 `#D65044` 配白字(4.13:1) | 1 | 改 `#CE4437`(5.09:1) |
| `$dark-text-tertiary` 在选中行上只有 4.26:1 | 8 | token 由 `#A49FB8` 提到 `#ADA8C0` |

**残留亮色值**:生成后扫描出 `$win-bg`/`#FDF3EC` ×4、`$green-deep` ×3、`#FFEFE2`、`#F6AD5514` ×2、`#9C97B0`、白色面板 ×4、亮色阴影 ×1 —— 全部换成对应暗色 token。

**引擎裁切 350 → 0**

- 16 处「完全裁切」是状态栏溢出窗口(时钟与日期节点在 8 张板上被裁没了)。根因是状态栏把 CPU / MEM 画成两个 150px 的框,总宽超可用宽度 163px。按源码 `window/status_bar.rs` + `system_load/render.rs`,系统负载合并成一条紧凑项 `CPU 12%  MEM 61%`,22 张板的状态栏内容宽度回到可用宽度内,余量 20px;
- 标题栏前进/后退是 24×24 的命中框,却放在 22px 高的容器里 → 容器改 24px 高、导航宽度 40/63 → 48;
- Settings 各页面 634px 宽塞进 604px 的内容区 → 页面宽改 604;
- 设备总览表格列宽合计超 2px(表头)/14px(数据行)→ 设备列 378 → 364;
- Split Panes、Settings·Notifications 的双行说明文本框 50px 放进 48px 的行 → 行高 48 → 52;
- Sidebar Layouts 的图标槽 16px 装 18px 的 "128" → 槽宽 20;
- Agents 列的 5 个圆点加文字超 3px → 圆点间距 8 → 6;
- 更新面板说明、Add Device 脚注、Sidebar Layouts 元信息等零散 1–14px 的溢出 → 分别收字宽、改文案、字号 11 → 10。

### 32.6 修完后的审计

| 指标 | 暗色阶段开始 | 现在 |
|---|---|---|
| 低于 WCAG AA 的文本节点 | 25 | **0** |
| 引擎 `problems` 裁切 | 350(16 完全裁切) | **0** |
| 暗色板里的亮色 token 残留 | 11 类 | **0** |
| 根画板 | 35 | **71**(35 亮 + 35 暗 + 1 系统板) |
| 变量 | 35 | **70** |
| 重名 / 未命名 / placeholder | — | **0 / 0 / 0** |

全量复跑:对比度失败 0、命中区 < 24px 0、半像素字号 0、`fontWeight` 误用 token 0、字号仍为 7 档(10 / 11 / 12 / 13 / 14 / 16 / 20)。
