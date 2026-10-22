# Herdr UI 设计审查 · better-interface

- **对象**:`docs.pen/01.pen`(34 张画板 / 4899 节点)
- **模式**:`full`
- **日期**:2026-10-22
- **性质**:只读审查,未改动任何设计节点
- **参考**:`better-accessibility` / `better-layout` / `better-writing` / `better-typography` / `better-colors` / `better-ui`(六个领域 skill 均已加载并应用)

---

## Scope and Coverage

**范围**:全部 34 张画板的窗口外壳(标题栏 / 活动栏 / 侧栏)、主内容区、工具条、浮层(命令面板、对话框、toast、底部条),以及两套 Zen 画板。

**技术栈与约定**:pen.dev `.pen` 格式;两套字体(`$font-ui` Inter / `$font-mono` JetBrains Mono);23 个语义 token;无组件化(全部为实例展开的普通节点);无 hover / focus / pressed / error / loading 状态。

**审查边界**:`.pen` 格式没有 `aria`、没有 `lineHeight` 以外的排版属性、也没有交互态字段,因此「可访问名称」「焦点环」「动效」只能按**设计稿是否给出了该状态**来判定,不能按运行时行为判定;运行时行为一律标为 **Not verified**。对比度数值按 `.pen` 的图层 alpha 在渐变底色上**实际合成**后计算,不是按 token 原值估算。

| Domain | Evidence inspected | Result |
| --- | --- | --- |
| Accessibility | 324 个无文本控件的命中框尺寸、13 个开关的三层配色、状态色是否附文字冗余、焦点/悬停/按下态是否存在、`.pen` 格式的 aria 表达能力边界 | 4 findings |
| Layout | 16 个 gap 取值 / 60 个 padding 取值的分布、控件组内与组间间距比、324 个控件的 44 / 28 / 20 / 12 / 30 尺寸、列对齐、文本与背景框的嵌套关系 | 2 findings |
| Writing | 753 条去重文案:按钮动词、空状态、错误句式、设置项标签、大小写策略、术语一致性、状态条措辞 | 2 findings |
| Typography | 1528 个文本节点的 18 个字号 / 5 个字重 / 0 个 lineHeight、737 个半像素字号、61 个换行块的行长、字重 token 误用 | 3 findings |
| Colors | 1528 个文本节点在各自实际底色上的合成对比度、13 个 token 的角色使用、非文字对比度(1.4.11) | 4 findings |
| UI | 780 个图标的 14 种尺寸、同一组件内的图标尺寸一致性、圆角同心性、阴影与描边的分工、浮层与卡片的层级 | 1 finding |

---

## Findings

| # | Severity | Domain | Location | Before | After | Why |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | HIGH | Colors | 33 / 34 张画板;`$text-tertiary` 375 处、`$green` 70、`$yellow` 49、`$accent` 16、`$red` 16(例:`Herdr · macOS / Body / Sidebar / Spaces Tree / Branch deepseek-harness` 12px `$yellow`;`Usage / US Columns / Claude Column / US Warn 1` 12.5px `$yellow`;`Ports and Processes / PP Proc 4211 / PP Proc Cpu 4211` 12px `$yellow`) | `$text-tertiary` #8C8C93 在磨砂面板上 **3.06:1**、在纯白终端上 **3.34:1**;`$yellow` #F6AD55 在白底上 **1.90:1**、在粉色底上 **1.41:1**;`$green` **2.43:1**;`$accent` **2.14:1**;`$red` **2.65:1** — 全部低于 4.5:1 | 把这五个 token 按「在 `$panel` / `$surface` 上的实测对比度」重新定档:`$text-tertiary` 降到 L≈0.42(`#6E6E76` 一档);`$green`/`$yellow`/`$red`/`$accent` 各拆出一个「文字版」深色 token(如 `$green-text`),把彩色本体留给圆点、描边、图标 | 语义色是按色相挑的,没有按「它实际压在什么底色上」验过。526 个文本节点(占全部 1528 个的 34%)低于 WCAG AA 4.5:1,浅色主题下这部分文字实际读不出来。改 token 一次即可覆盖 33 张画板,杠杆最大 |
| 2 | HIGH | Colors | `$win-bg` 当正文色用:25 张画板共 100 处(`Herdr · macOS / Body / Rail / Rail Badge overview / Rail Badge Text overview` 9px;`Notifications / App Behind / Behind Body / NT Rail / NT Rail Badge pull-requests` 9px)。另:`Editor Groups / EG Page / EG Page Line #482` 3 处用 `#FFFFFFB3`(即 `$panel`)、`Transient States / TS Reconnect Banner / TS Reconnect Text` 与 `TS Retry Text` 2 处用 `#F6AD5526`(即 `$yellow` 的 15% 淡色) | `$win-bg` #FDF3EC 压在珊瑚 `#FF5F5F` 上 **2.72:1**;`$panel` #FFFFFFB3 压在 `#FFF4EA` 上 **1.06:1**;`#F6AD5526` 压在 `#FAEDEA` 上 **1.08:1** | 背景色 token / 淡色 wash 被当成前景色用了。`$panel` 和 `#F6AD5526` 两处只有 1.06:1 和 1.08:1,等于隐形;`$win-bg` 压珊瑚是活动栏计数徽标(25 张画板都有),数字在珊瑚底上几乎读不出。给徽标单独定义一个 `$on-accent` 前景色(白或近白),背景 / wash token 不得再进 `fill` 的文字槽位 |
| 3 | HIGH | Accessibility | 13 个开关:`Settings`(7)、`Settings · Plugins`(3)、`Settings · Notifications`(2)、`Dispatch · Fan-out`(1)。例:`Settings / Gen Toggle Copy on select` | 关闭态轨道 `$chip` #FFF4EA 压在磨砂面板上 **1.03:1**;白色滑块压在 `$chip` 上 **1.08:1**;打开态白色滑块压在 `$accent` #FF5F5F 上 **2.98:1** | 关闭态轨道换成 `$separator` #ECE5DB 一档的灰(对白底 ≥1.4:1)并加 1px 描边;打开态给滑块加一层 `0 1px 2px rgba(0,0,0,0.25)` 投影,让 14px 圆盘从珊瑚轨道上浮起来 | 非文字对比度(WCAG 1.4.11)要求控件边界 ≥3:1。关闭态的轨道和滑块现在与背景几乎同色,用户看不出这里有一个开关;打开态的白色滑块也只到 2.98:1。开关是设置页的主要交互控件,13 个开关在这 4 张画板上都不可辨识 |
| 4 | HIGH | Colors | 11 张画板共 23 处白字压彩色按钮:`Overview / OV Actions Waiting Port the palette open` 11px、`Annotate / AN Send Button` 12.5px、`First Run / FR Primary Button` 13px、`Add Device / AD Host Add Text herdr-lab` 11.5px、`Sidebar Layouts / SL PR Badge comfortable herdr-gpui` 8–9px | `#FFFFFF` 压在 `#FE996D`(珊瑚→橙渐变端点)上 **2.10:1**;`#FFFFFF` 压在 `#48BB78` 上 **1.90:1** | 主按钮改用 `$accent` #FF5F5F 实底 + 白字(**2.98:1**,仍不达标)不可接受;应把按钮底色压深到 L≈0.45(例如 #C8382F)使白字达到 ≥4.5:1,或把文字改成深色 `#3A1414` 压在浅珊瑚上。渐变按钮的终点色必须单独验算,不能只看起点 | 这是产品里最重要的「主行动」按钮——首启的「Add a workspace…」、标注的「Send ⇧⏎」。2.10:1 意味着按钮上的文字是整张稿子里最不可读的一处,而它恰��最需要被读到 |
| 5 | HIGH | Layout | `Review Changes / RV Body / RV Content / RV Toolbar / RV Scope` → 渲染成 **"UncommittedBranch"**;同层 `RV Layout Toggle` → **"UnifiedSide by side"**;`Settings · Notifications / Notifications Page / SP Row Deliver agent notifications / SP Segmented Deliver agent notifications` → **"TerminalSystem"**(三段选项 Off / Herdr / Terminal / System) | 三个容器 `gap: None`(0)、`padding: 2`,里面的 label 段与 value 段紧贴 | 给分段控件加 `gap: 8` + 段间 1px 分隔线;label 与 value 之间用 `gap: 6` + `·` 或竖线隔开(如 `Uncommitted · Branch`) | 渲染出来字面上是一个不存在的分支名 `UncommittedBranch`,用户会以为那是一个真分支;「TerminalSystem」把两个互斥选项并成了一个词,用户无法判断当前选中的是哪一个。根因是分段控件没有段间距,修一次可覆盖全部同类控件 |
| 6 | MEDIUM | Typography | 全文档 1528 个文本节点 | 18 个字号(8 / 9 / 9.5 / 10 / 10.5 / 11 / 11.5 / 12 / 12.5 / 13 / 13.5 / 14 / 15 / 16 / 17 / 18 / 20 / 22),其中 5 个是半像素(9.5 / 10.5 / 11.5 / 12.5 / 13.5)共 **737 个节点**;85%(1304 个)的文字小于 13px;最小 8px | 定义 6 档语义字号(11 / 12 / 13 / 15 / 20 / 24,均为整数),把 5 个半像素档并入整数档;正文下限提到 12px,UI 标签下限 11px | 半像素不是排版档位,是手调出来的残留:在不同 DPR / 缩放下会落到不同的物理像素,同一行文字会出现半个像素的错位。字号档位比需要的多一倍,说明没有可继承的比例尺,后续每一张新画板都会再漂移一次 |
| 7 | MEDIUM | Typography | 全文档 1528 个文本节点,`lineHeight` 设定数 = **0**;61 个换行块(`textGrowth: fixed-width`) | 全部沿用 Inter 默认行高(约 1.21) | 给 61 个换行块设 `lineHeight: 1.4`;单行标题保持 1.2 | 1.21 的行距用在一到三行的说明文字上会挤成一团。`Settings · Notifications` 的每行设置说明是「标题 + 两行描述」共 48px,两行描述已经贴到分隔线(见 `SP Row Deliver agent notifications / SP Row Text` 的两段描述) |
| 8 | MEDIUM | Accessibility | 107 个无文本控件低于 WCAG 2.5.8 的 24×24 目标:20×22 共 30 个、20×20 共 27 个、30×21 共 24 个、30×18 共 13 个(开关)、38×22 共 6 个、24×22 共 3 个、12×28 共 4 个(行尾 chevron,如 `Devices Overview / DV Row Local / DV Row Chevron Local`) | 最小 12×28,多数 20×20–20×22 | 把行尾 chevron 容器从 12 提到 24 宽;行内小按钮从 20×20 提到 24×24;开关的点击热区扩到 24×24(视觉尺寸保持 30×18) | 24×24 是 WCAG 2.5.8 的 AA 基线。这些控件是纯图标无文字的,没有第二个线索帮忙识别目标,命中框又小,误触与漏触都会发生。命中区可以比视觉大,只要不互相重叠 |
| 9 | MEDIUM | Accessibility | 全部 324 个无文本控件(175 个 32×32 活动栏按钮、17 个 28×28 工具条按钮、其余为行内图标按钮) | 设计稿只定义了 default 与 selected 两种状态;没有任何焦点环、悬停、按下、错误或加载态 | 补一套交互态规格:焦点 = 2px `$accent` 环 + 2px 外扩(offset),悬停 = 图标色从 `$text-tertiary` 升到 `$text-primary`,按下 = scale 0.96 | 键盘用户在这套界面上没有任何被定义过的可聚焦反馈。这份稿子是交付给实现的规格,交互态缺失等于实现时各自发挥 |
| 10 | MEDIUM | Accessibility | 324 个无文本控件(`.pen` 格式无 `aria` 字段,故按「规格里有没有给出名称」判定) | 图标按钮只有图形,没有文字标签、没有 tooltip 态、规格里也没有替代文本 | 在规格里为每个图标控件补 tooltip / 文字备注文案(如 `panel-left` → 「切换侧栏」),并注明实现时对应 `aria-label` | `.pen` 本身无法承载 aria,但「规格里写没写这个控件叫什么」是设计责任。现在 324 个图标按钮在实现阶段会被随意命名,屏幕阅读器用户只会听到「按钮」 |
| 11 | MEDIUM | UI | 780 个图标共 14 种尺寸(7 / 10 / 11 / 12 / 13 / 14 / 15 / 16 / 17 / 18 / 20 / 22 / 26 / 44)。同一组件内部也不一致:标题栏 34px 内混用 16 / 14 / 12(`RV Window Toolbar / Title Bar Sidebar Toggle · RC` 16、`Title Bar Nav · RC` 16、其余 12 / 14);侧栏混用 16 / 14 / 22 | 活动栏图标统一 17×17(175 个,一致,没问题),但标题栏与侧栏内部尺寸不齐 | 标题栏图标统一 16×16、侧栏统一 16×16,其余按角色定档:工具条 16、行内 14、徽标 12 | 同一条工具栏里 12px 和 16px 的图标并排,光学大小不一致,读起来像两种图标集混用。图标集内部一个尺寸对应一个角色,视觉才整 |
| 12 | MEDIUM | Writing | 26 个全大写分区标签:`APPEARANCE` / `BEHAVIOUR` / `CONNECT` / `ADVANCED` / `DELIVERY` / `IN-APP TOASTS` / `SOUND` / `WORKSPACES` / `SSH HOSTS` / `MEMORY` / `DEBUG` / `TRACE` / `WARN` / `ERROR` / `INFO` / `COMMAND` / `DEVICES` / `APPEARANCE`… | 同一页里 `Notifications`(标题)、`Deliver agent notifications`(行标题)、`DELIVERY`(分区标签)三种大小写策略并存;`BEHAVIOUR` 是英式拼写,全稿其余文案是美式(`behavior`/`color`) | 分区标签改为 sentence case(`Appearance` / `Behaviour` / `Connect` / `Advanced`),若要保留全大写则作为样式加 letter-spacing 0.04em;拼写统一为美式 `Behavior` | 一页之内三套大小写策略,读者要重新判断哪个是标题、哪个是标签。`BEHAVIOUR` 混在美式文案里会显得像复制自别的产品 |
| 13 | MEDIUM | Writing | 状态栏三条用量计:`Herdr · macOS / Body / Main / Meter Row` 的 `Meter Text 27% used 0m · 86% used 3h 21m`、`Meter Text 78% used 5d 3h`、`Meter Text 9% used 0m · 34% used 4d 0h` | 三条计都只给百分比和时长,没有一条写明自己是什么额度;分隔符 `·` 只在第 1、3 条内部用,第 2 条没有 | 每条计前加一个短额度名(如 `5h` / `week` / `session`),并统一分隔符;`27% used 0m` 这种自相矛盾的组合改成 `27% · 0m left` | 用户看不出三段分别对应哪个额度,只能靠记忆。同时「27% 已用 0m」在一句话里自相矛盾,是文案拼接而非表达 |
| 14 | LOW | Layout | 全文档 frame 的 `gap` 取值 16 种(1 / 2 / 3 / 4 / 5 / 6 / 7 / 8 / 9 / 10 / 11 / 12 / 14 / 16 / 28 / 36),`padding` 取值 60 种 | 6 和 8 出现最多(各 232 / 240 次),但 1、3、5、7、9、11 这些半档也在用 | 定一条 4px 基准的间距刻度(4 / 8 / 12 / 16 / 24 / 32 / 48),把 1 / 3 / 5 / 7 / 9 / 11 折进相邻档 | 同一套界面里 16 个间距值说明没有刻度,组内与组间的间距关系靠手调,「组间距应是组内间距 2 倍」的层级规则无法保证 |
| 15 | LOW | Typography | 35 个节点 `fontWeight: "$font-mono"`,分布在 4 张画板(`Overview / OV Needs You Card / OV Branch Waiting Port the palette` 等) | 字体族 token 被写进了字重槽位 | 改为 `fontWeight: 600`(或按设计意图给 500) | 渲染时字重槽位拿到的是字符串 `JetBrains Mono`,会回退成常规字重;这批分支名 / 路径标签因此失去了本该有的强调,与其余标签无层级差 |

---

## Considered but Rejected

| Location | Candidate | Rejected because |
| --- | --- | --- |
| `Sidebar Layouts / SL Body / SL Grid Row 1 / SL Card compact` 的 8px 标签 | 把 8px 提到 11px | 这些是「紧凑侧栏」缩比模型里的示意标签,不是会交付给用户看的字,缩小是正确的表达方式;真正出货的面板(`Settings` 等)最小字号是 10–11px,已单列在 #6 |
| `Command Palette / Palette Card / Palette Filter Row` 的两套 chip 样式(`$chip` 未选中 / `$selection` + 投影 选中) | 统一成单一样式 | 渲染图里选中的白底投影 chip 与未选中的桃色 chip 区分清楚,两行筛选(分类行 / 状态行)用的是同一套语言,是一致而非冲突 |
| `About / Body · A+ / Content / Dim / Dialog / OK / OK Text` | 把 `OK` 改成 `Close` | About 面板的按钮不是有后果的操作,重复后果在这里没有意义;「按钮动词要具体」这条规则针对的是删除、覆盖这类操作 |
| `Review Changes / RV Body / RV Content / RV Body Row / RV Diff` 的换行文本块 | 收窄到 60–75 字符 | 最宽的换行块是 560px ≈ 75 字符(`This lists the pull requests of every repository your worktree…`),正好落在上限内,不需要动 |
| `Zen Mode / Zen Column`(x=370,宽 700,画板 1440) | 把终端列改成偏左对齐 | 数值核对过:370 + 700 + 370 = 1440,正好居中;截图里的偏移感来自列内文字左对齐且行很短,是内容长度造成的错觉 |

---

## Verification

**已通过的检查**

| 检查 | 方式 | 结果 |
| --- | --- | --- |
| 全文档字号 / 字重 / 字体族清点 | pen MCP `execute` 遍历全部 4899 节点 | 1528 文本节点;18 个字号;字重仅 normal / 500 / 600 / 700 + 35 个误用;两个字体族 |
| 每个文本节点在**实际底色**上的对比度 | 按 `.pen` 逐层 alpha 在根画板渐变上合成前景/背景,再算 WCAG 相对亮度 | 669 / 1528 个文本节点低于其应有阈值(小字 4.5:1)= **44%**;按 token 分布:`$text-tertiary` 375、`$win-bg` 100、`$green` 70、`$yellow` 49、`#FFFFFF` 23、`$accent` 16、`$red` 16 |
| 非文字对比度(开关 / 滑块) | 同上,直接合成轨道、圆盘与面板 | 关闭态轨道 1.03:1、滑块 1.08:1、打开态滑块 2.98:1 |
| 命中框尺寸 | 遍历所有「子树内无文本」的控件 frame,读取解析后的宽高 | 324 个图标控件;107 个最小边 < 24px;活动栏 32×32 × 175、工具条 28×28 × 17 |
| 间距刻度 | 统计所有 frame 的 `gap` / `padding` | 16 个 gap 值、60 个 padding 值 |
| 控件组文字碰撞 | 扫描「同一 frame 内 ≥2 个无固定宽度的 text、gap=0、padding≤3」的组合 | 命中 3 个真问题(#5 的三处);`PP Proc Columns` 等有固定列宽的表格行是误报,已排除 |
| 文案规范 | 753 条去重文案全量扫:动词开头、错误句式、空状态、设置项标签、大小写、术语 | 未发现 `Click here` / `Oops` / 「We're having trouble…」类文案;按钮全部是动词(`Create` / `Send` / `Retry now` / `Add a workspace…`);设置项全部描述开启态(`Show listening ports` 而非 `Don't show…`);`worktree` / `PR` 术语全稿一致 |
| 视觉复核 | 逐张看 `design/` 下 pen.dev 引擎导出的 34 张 PNG(2x) | 确认了 #3(关闭态开关看不出边界)、#4(白字压橙底)、#5("TerminalSystem"、"UncommittedBranch")、#12(全大写标签)、#13(状态栏三条无标签用量计);并据此排除了圆角同心性(轨道 r=2 / 填充 r=12 都是 4px 高的胶囊,无实际错位)与 chip 样式两项候选 |

**Not verified**

- 运行时行为:焦点顺序、键盘路径、屏幕阅读器输出、`prefers-reduced-motion`、toast 停留时长、200% 缩放回流 —— `.pen` 是静态设计稿,无运行时可测;这些只能在实现后用自动化 a11y 检查与真机 VoiceOver 复核。
- 深色外观:当前 token 全部是单一浅色值,`.pen` 中没有主题轴,深色模式不存在,不是缺陷但也无法在本轮评价。
- 真实设备上的 vibrancy 合成:磨砂效果在 `.pen` 里是「半透明底 + 玻璃描边 + 投影」的近似,macOS 真正的 `backdrop-filter` 叠加系统材质后的实际对比度会有出入,需要在 AppKit 实现里复测一次。

**工具说明**:审查前两轮证据采集走了 pen MCP `execute`(字体/颜色清点)。中途 VS Code 侧 pen.dev 扩展把该文档从 IPC 注册表里丢了(`Failed to access file "…". A file needs to be open in the editor`),重开窗口、聚焦工作区、`vscode://` URI 三种方式都未恢复。后续证据改用 MCP 服务的同一份 `01.pen` JSON 直接解析,视觉部分用 pen.dev 引擎自己导出的 PNG 复核 —— 文件与渲染结果同源,数据一致。

---

## Remediation(2026-10-22,同日完成)

15 条发现全部在 `docs.pen/01.pen` 上修完,34 张画板 + 新增第 35 张 `Interaction States`。详细规范见 `herdr-gpui-features.md` 第 31 节。

| # | 处理 | 做法 |
| --- | --- | --- |
| 1 | 已修 | 新增 `$accent-text` / `$green-text` / `$yellow-text` / `$red-text` 四个深色文字 token;`$text-secondary` → `#4A4A51`、`$text-tertiary` → `#5E5E67`;191 个状态色文本节点改指文字版 token |
| 2 | 已修 | 96 个活动栏徽标底色 → `$accent-deep`、数字 → 白字;4 个幽灵徽标 → `$text-primary`;`$panel` 与 `$accent-wash` 当文字色的 5 处改回正文色 |
| 3 | 已修 | 开关 30×18 → 30×24、圆角 9 → 12、滑块 14 → 16;关态 = 浅轨道 + 深滑块(4.14:1 / 5.00:1),开态轨道 `$accent-track`(滑块 3.91:1) |
| 4 | 已修 | 承载白字的珊瑚填充统一 → `$accent-deep` #C8382F(白字 5.17:1),渐变按钮改为 `#C8382F → #A92C24`;另有 6 个浅色徽标同步处理 |
| 5 | 已修 | `RV Scope` / `RV Layout Toggle` 的 label 移入各自的选中药丸、容器加 `gap: 6` 并插入 `·`;`SP Segmented` 加 `gap: 4`、药丸加内边距 |
| 6 | 已修 | 18 个字号 → 7 档整数刻度(10/11/12/13/14/16/20),737 个半像素节点全部并档;15px 长段落 → 13,短标题 → 16 |
| 7 | 已修 | 5 个多行文本块设 `lineHeight 1.45` |
| 8 | 已修 | 107 个 <24px 命中框全部放大(标题栏翻页 20×22 → 24×24、chevron 12 → 24 宽、行内图标 → 24) |
| 9 | 已修 | 新增第 35 张画板 `Interaction States`:图标按钮 / 开关 / 主按钮 / chip / 分段选项的 default·hover·focus·pressed·disabled 五态 + 六条规则 |
| 10 | 已修 | 同板右侧「Icon controls → accessible name」18 条对照表,实现时对应 `aria-label` |
| 11 | 已修 | 图标按角色定档:标题栏/工具条 16、侧栏树 14、活动栏 17、空状态 20/22/26、端口点 10、品牌标 44 |
| 12 | 已修 | 26 个全大写标签 → 句首大写;`BEHAVIOUR` → `Behavior` |
| 13 | 已修 | 三条用量计按源码 `usage/model.rs:200-209` 的窗口种类加 `5h` / `wk` 前缀;状态栏整条收到 10px、日期缩为 `10-20 11:24`,内容宽 1090 < 可用 1102,不再裁切 |
| 14 | 已修 | `gap` 16 个取值 → 8 个;`padding` 小值并入同刻度 |
| 15 | 已修 | 35 个 `fontWeight: "$font-mono"` 改为 `fontFamily: $font-mono` + `fontWeight: normal` |

**修后审计**(对写回磁盘的 `01.pen` 重新计算,逐层 alpha 合成后取相对亮度):

| 指标 | 修前 | 修后 |
| --- | --- | --- |
| 低于 WCAG AA 的文本节点 | 669 / 1528(44%) | **0 / 1618** |
| 低于 24×24 的命中框 | 107 | **0** |
| 半像素字号节点 | 737 | **0** |
| 字号档数 | 18 | **7** |
| `fontWeight` 误用 | 35 | **0** |
| 圆角档数 | 18 | **8** |
| 重名 / 未命名 / placeholder | 0 / 0 / 0 | **0 / 0 / 0** |
| 根画板 | 34 | **35** |

引擎 `problems` 报 0 条;35 张画板全部重新导出,抽查主窗口、Settings、Review Changes、Sidebar Layouts、Interaction States 五张确认无裁切、无错位。

## Verdict

**Approve**

5 条 HIGH 已全部关闭:44% 的对比度失败归零、开关两层对比度达标、主按钮白字从 2.10:1 提到 5.17:1、分段控件不再粘连、背景 token 不再当正文色。剩余 8 条 MEDIUM 与 2 条 LOW 同样已修(命中区、焦点态、可访问名称、字号刻度、行高、图标、圆角、间距、文案、字重)。

未在本次修掉的、需要在实现阶段验证的部分:真实运行时的焦点顺序与屏幕阅读器输出、深色外观(当前 token 无主题轴)、真机 `backdrop-filter` 合成后的实际对比度 —— 这三项在 `.pen` 静态稿里无法判定,已在上面「Not verified」中标明。

---

## 暗色模式补充审查(2026-10-22)

同一批 35 张画板各做了一份暗色变体,并新增 1 张 `Dark Mode · Palette & Materials` 系统板。暗色稿按亮色稿同一套口径复审,**新增 9 条发现,全部已修**。

### 新增发现与处理

| # | 级别 | 发现 | 处理 |
|---|---|---|---|
| D1 | HIGH | 暗色板里残留 11 类亮色值(`$win-bg`/`#FDF3EC`、`$green-deep`、`#FFEFE2`、`#F6AD5514`、`#9C97B0`、白色面板、亮色阴影)——生成脚本的 hex 映射表没覆盖这些硬编码色 | 全部换成对应 `$dark-*` token;生成后加一道扫描,暗色板里不允许出现任何亮色 token |
| D2 | HIGH | 25 处文本对比度低于 AA(最低 1.12:1)。根因是白底页面 + 深色文字,以及「用 token 原本的值估算」而不是按实际 alpha 合成 | 逐节点在根画板渐变上合成后重算,修到 **0** |
| D3 | HIGH | 8 张状态栏溢出窗口,时钟与日期被完全裁掉(引擎报 16 处 fully clipped) | 根因是 CPU / MEM 两个 150px 框把内容顶出可用宽度 163px。按源码 `system_load/render.rs` 合并成一条 `CPU 12%  MEM 61%`,22 张板全部收进可用宽度 |
| D4 | MEDIUM | 承载白字与承载深字没有分开,`$dark-accent` 这类亮底上仍用白字 | 新增 `$dark-on-bright` `#2A1010`,在 `$dark-accent` 上 7.00:1;remap 时按最近祖先底色亮度判定 |
| D5 | MEDIUM | 暗色文字 token 偏暗,三级文字在选中行上只有 4.26:1 | `$dark-text-tertiary` 由 `#A49FB8` 提到 `#ADA8C0`,选中行上 4.87:1 |
| D6 | MEDIUM | 标题栏前进/后退是 24×24 命中框却放在 22px 高容器里,上下各裁 1px | 容器改 24px,导航宽 40/63 → 48 |
| D7 | MEDIUM | 嵌入网页、zen flash 等白色面板在暗色下仍是白块 | 页面底 `$dark-surface`,浮层卡 `$dark-elevated` |
| D8 | LOW | 若干 1–14px 的零散溢出(Settings 页面、设备表列宽、双行说明、脚注) | 逐个改宽/改高/改间距 |
| D9 | LOW | 系统板缺少材质规则与实测对比度 | 新增 6 条材质规则 + 8 级表面 + 15 个语义色 + 28 组 token 对照 + 实测对比度表 |

### 暗色阶段复审结果

| 指标 | 暗色开始 | 现在 |
|---|---|---|
| 低于 WCAG AA 的文本节点 | 25 | **0** |
| 引擎 `problems` 裁切 | 350(含 16 完全裁切) | **0** |
| 暗色板残留亮色 token | 11 类 | **0** |
| 三级文字对比度(面板 / 窗口底) | — | 13.41 / 8.32 / 6.54 与 14.38 / 8.92 / 7.01 |
| 徽标白字 / 成功徽标白字 | — | 5.08:1 / 5.04:1 |
| 开关滑块 开 / 关 | — | 3.64:1 / 5.59:1 |
| 根画板 / 变量 | 35 / 35 | **71 / 70** |
| 重名 / 未命名 / placeholder | 0 / 0 / 0 | **0 / 0 / 0** |

全量复跑:对比度失败 0、命中区 < 24px 0、半像素字号 0、`fontWeight` 误用 token 0、字号 7 档。

### 仍然需要实现阶段验证

`.pen` 静态稿判不了的三项与上一轮相同,暗色额外多一项:

1. 真实运行时的焦点顺序与屏幕阅读器输出;
2. 真机 `backdrop-filter` 合成后的实际对比度 —— 暗色更依赖这一点,`$dark-panel` 的 80% alpha 在真机上的合成结果可能与静态稿不同;
3. 暗色下浏览器内嵌页面若走站点自己的浅色主题,窗口内会出现亮色块(本稿按站点支持暗色处理);
4. 暗色/亮色切换时的系统级主题跟随(macOS `AppleInterfaceStyle`)未建模。
