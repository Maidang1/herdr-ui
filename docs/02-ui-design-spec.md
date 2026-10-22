# Herdr UI · 设计规范（从 `docs.pen/01.pen` 提取）

> 本文是 `docs.pen/01.pen` 里那张 **主窗口画板**（`Herdr · macOS`，1440 × 900）的精确规格，
> 供实现时逐项对照。所有数值均直接取自 `.pen` 源文件，不含推测。
>
> - 设计稿源文件：`docs.pen/01.pen`（Pen 设计文件，71 张根画板）
> - 渲染导出：`design/*.png`（亮/暗各 35 张 + 令牌板，**不入库**，由 `.pen` 重新导出）
> - 实现方案：`docs/01-implementation-plan.md`
> - 功能规格：`reference/herdr-gpui-features.md`

---

## 1. 令牌系统

### 1.1 颜色令牌（亮色 → 暗色）

设计稿变量区共 **70 个变量**：30 组颜色有亮暗配对，另有 8 组仅暗色、2 个字体族。

| 令牌 | 亮色 | 暗色 |
|---|---|---|
| `win-bg` | `#FDF3EC` | `#221F2E` |
| `titlebar` | `#FBF1E8` | `#2A2636` |
| `panel` | `#FFFFFFB3` | `#2A2636CC` |
| `surface` | `#FFFFFF` | `#1B1922` |
| `sidebar-bg` | `#F3E9FB` | — |
| `rail-bg` | `#FFFFFF73` | `#FFFFFF0F` |
| `chip` | `#FFF4EA` | `#FFFFFF14` |
| `selection` | `#FFFFFFD9` | `#3E3A52` |
| `bar` | `#ECE5DB` | `#FFFFFF26` |
| `separator` | `#ECE5DB` | `#FFFFFF1A` |
| `terminal-bg` | `#FFFFFF` | `#16141C` |
| `text-primary` | `#1A1A1F` | `#F3F1F8` |
| `text-secondary` | `#4A4A51` | `#C3BED3` |
| `text-tertiary` | `#5E5E67` | `#ADA8C0` |
| `chiptext` | `#54545A` | — |
| `accent` | `#FF5F5F` | `#FF7A6E` |
| `accent-deep` | `#C8382F` | `#C93A31` |
| `accent-text` | `#A62B1E` | `#FF9E93` |
| `accent-wash` | `#FF5F5F29` | `#FF7A6E29` |
| `accent-track` | `#E84545` | `#E05B4E` |
| `green` | `#48BB78` | `#4ADE80` |
| `green-text` | `#1C6B3D` | `#7BE8A8` |
| `green-deep` | `#1C6B3D` | `#2F7D4F` |
| `yellow` | `#F6AD55` | `#F5C26B` |
| `yellow-text` | `#7A4E0C` | `#F7CE85` |
| `red` | `#F56565` | `#FF7B72` |
| `red-text` | `#B3261E` | `#FF9D95` |
| `mauve` | `#B794F4` | `#C9A0FF` |
| `track-off` | `#E4E0D9` | `#3A3648` |
| `knob-off` | `#6E6A63` | `#B6B1C4` |

**仅暗色**（亮色下用同族亮色令牌或直接留空）：

| 令牌 | 暗色 |
|---|---|
| `elevated` | `#2E2A3A` |
| `overlay` | `#1E1C28D9` |
| `on-bright` | `#2A1010` |
| `blue` | `#6FB3FF` |
| `cyan` | `#6FD6E8` |
| `yellow-wash` | `#F5C26B1A` |
| `green-wash` | `#4ADE801A` |
| `red-wash` | `#FF7B721A` |
| `blue-wash` | `#6FB3FF2E` |
| `red-stroke` | `#FF7B7280` |

**约定**：颜色一律写 8 位十六进制（含 alpha），如 `#FFFFFFB3` = 白 70%。
实现里**禁止裸 hex**——每个绘制用的颜色都必须能回溯到上面的某个令牌名。

### 1.2 字体族

| 令牌 | 值 | 用途 |
|---|---|---|
| `font-ui` | `Inter` | 窗口全部 UI 文字 |
| `font-mono` | `JetBrains Mono` | 终端字符、端口号、标签页编号 |

> 两套字族都未必装在用户机器上。实现应按 **列表回退**：
> `Inter, Helvetica Neue, Arial, sans-serif` 与 `JetBrains Mono, Menlo, SF Mono, monospace`。

---

## 2. 度量标尺

### 2.1 字号（画板实际取值）

| 用途 | 字号 | 字重 | 字距 |
|---|---|---|---|
| 终端字符 / 标签页编号 | 13 | normal / 600 | — |
| 侧边栏正文（空间名、代理名） | 13 | 600（选中时）/ normal | — |
| 侧边栏副行（分支、端口、状态） | 12 | normal | — |
| 侧边栏分区标题（Spaces / Agents） | 13 | 600 | — |
| 侧边栏头（Workspaces） | 12 | 600 | `0.9` |
| 标题栏标题（Herdr） | 13 | normal | — |
| 仪表行文字（用量 / CPU / MEM / 日期） | 10 | normal / 600（标签） | — |
| 徽标数字 | 10 | normal | — |

> 注意：主窗口画板里 **没有 11px 与 14px 正文**。11/14/16/20 只出现在其它画板
> （设置面板、弹层）。实现主窗口时不要引入。

### 2.2 间距

| 档位 | 值 | 画板中的用法 |
|---|---|---|
| `SpaceXS` | 2 | 树容器上下内边距 |
| `SpaceSM` | 4 | 树内行间距 |
| `SpaceMD` | 6 | 树容器左右内边距、行内左右内边距 |
| `SpaceLG` | 8 | 行内元素间距 |
| `SpaceXL` | 12 | 分区行左右内边距 |
| `Space2XL` | 16 | 主体三栏之间的间距 |
| `Space3XL` | 24 | — |
| `Space4XL` | 32 | — |

### 2.3 圆角

| 档位 | 值 | 用法 |
|---|---|---|
| `RadiusXS` | 2 | 仪表条 |
| `RadiusSM` | 4 | — |
| `RadiusMD` | 6 | 选中的空间行、标签页胶囊 |
| `RadiusLG` | 8 | 活动栏按钮、徽标 |
| `RadiusXL` | 10 | — |
| `Radius2XL` | 12 | — |
| `Radius3XL` | 16 | **主卡片（Main）** |

### 2.4 描边

| 用途 | 值 |
|---|---|
| 常规描边 | 1px |
| 强调描边 | 2px |
| 主卡片描边（亮 / 暗） | `#FFFFFF66` / `#FFFFFF1A` |

### 2.5 窗口部件尺寸

| 部件 | 尺寸 |
|---|---|
| 标题栏高 | 34 |
| 活动栏宽 | 44 |
| 侧边栏宽 | 248 |
| 标签页条高 | 36 |
| 仪表行高 | 26 |
| 活动栏按钮 | 32 × 32，左内边距 6，间距 4（按钮顶间距 36） |
| 活动栏图标 | 17 |
| 徽标 | 14 高，圆角 8（两位数时加宽） |
| 图标 | 14（树内）/ 16（分区头、标题栏） |
| 终端行高 | 19 |
| 终端字号 | 13 |
| 终端内边距 | 上下 16，左右 18 |

---

## 3. 主窗口解剖（`Herdr · macOS`，1440 × 900）

### 3.1 结构树

```
Frame  Herdr · macOS                         1440×900  圆角 12  渐变 135°
├─ Frame  Herdr Window Toolbar               34 高        ← macOS 原生标题栏区域
│   ├─ Ellipse  Title Bar Close              9,9    12×12  $accent
│   ├─ Ellipse  Title Bar Minimize           29,9   12×12  $yellow
│   ├─ Ellipse  Title Bar Zoom               49,9   12×12  $green
│   ├─ Frame    Title Bar Sidebar Toggle     80,3   28×28  内部图标 16×16 $text-secondary
│   ├─ Frame    Title Bar Nav                 114,5  48×24  圆角 6
│   │   ├─ Frame  Title Bar Back             24×24  箭头 16×16 $text-secondary
│   │   └─ Frame  Title Bar Forward          24×24  箭头 16×16 $text-tertiary
│   ├─ Text     Title Bar Title               x=703（居中）13 normal $text-secondary  "Herdr"
│   └─ Frame    Title Bar Account             1406,3 28×28
│       └─ Frame  Title Bar Avatar           24×24 圆角 10  图标 16×16 $text-tertiary
│
└─ Frame  Body                               撑满      内边距 [10,14,14,0]  间距 16
    ├─ Frame  Rail                           44 宽    填充 #00000000（透明）
    │   ├─ Frame  Rail workspaces            6,6    32×32 圆角 8  填充 $selection
    │   │   └─ Icon  Rail Icon workspaces    17×17  $text-primary
    │   ├─ Frame  Rail overview              6,42   32×32 圆角 8  填充透明
    │   │   └─ Icon                         17×17  $text-tertiary
    │   ├─ Frame  Rail pull-requests         6,78
    │   ├─ Frame  Rail issues                6,114
    │   ├─ Frame  Rail security              6,150
    │   ├─ Frame  Rail usage                 6,186
    │   ├─ Frame  Rail Badge overview        27.5,41  14×14 圆角 8  $accent-deep  "3"
    │   ├─ Frame  Rail Badge pull-requests   21.5,77  17×14 圆角 8  $accent-deep  "12"
    │   ├─ Frame  Rail Badge issues          27.5,113 14×14 圆角 8  $accent-deep  "4"
    │   ├─ Frame  Rail Badge security        27.5,149 14×14 圆角 8  $accent-deep  "2"
    │   └─ Frame  Rail Settings              6,800  32×32 圆角 8
    │       └─ Icon                         17×17  $text-tertiary
    │
    ├─ Frame  Sidebar                        248 宽  填充 #00000000（透明）  纵向
    │   ├─ Frame  Sidebar Header             34 高  间距 8  内边距 [0,12]
    │   │   ├─ Text   Sidebar Title          12 600 字距 0.9  $text-secondary  "Workspaces"
    │   │   └─ Icon   Toggle Sidebar Icon    16×16  $text-tertiary
    │   ├─ Frame  Spaces Section             28 高  间距 8  内边距 [0,12]
    │   │   ├─ Text   Spaces Label           13 600  $text-primary  "Spaces"
    │   │   └─ Icon   Search Icon            16×16  $text-tertiary
    │   ├─ Frame  Spaces Tree                纵向  间距 2  内边距 [2,6]
    │   │   ├─ Frame  Workspace …            24 高 圆角 6  间距 8  内边距 [0,6]
    │   │   │   ├─ Ellipse  Mark …           7×7    填充 $accent + 描边 $accent 1.5（选中）
    │   │   │   │                                    无填充 + 描边 $text-tertiary 1.5（未选中）
    │   │   │   ├─ Icon    Icon …             14×14  $text-tertiary
    │   │   │   ├─ Text    Name …             13 600（选中）/ normal  $text-primary
    │   │   │   └─ Frame   Row Spacer …       撑满
    │   │   ├─ Frame  Branch …               19 高  间距 8  内边距 [0,6]
    │   │   │   ├─ Frame   Branch Indent …    22 宽
    │   │   │   ├─ Icon    Branch Icon …      14×14  $yellow
    │   │   │   └─ Text    Branch Text …      12 normal  $yellow-text
    │   │   ├─ Frame  Ports …                18 高  间距 8  内边距 [0,6]
    │   │   │   ├─ Frame   Ports Indent …     22 宽
    │   │   │   ├─ Icon    Ports Icon …       14×14  $text-tertiary
    │   │   │   └─ Text    Ports Text …       12 normal  $text-tertiary
    │   │   └─ Frame  Agent …                20 高  间距 8  内边距 [0,6]
    │   │       ├─ Frame   Agent Indent …     22 宽
    │   │       ├─ Icon    Agent Icon …       14×14  $text-tertiary
    │   │       └─ Text    Agent Text …       13 normal  $text-primary
    │   ├─ Frame  Sidebar Footer             30 高  间距 8  内边距 [0,12]
    │   │   ├─ Text   New Label              13 normal  $accent-text   "New"
    │   │   └─ Text   Menu Label             13 normal  $text-tertiary "Menu"
    │   ├─ Rectangle  Sidebar Divider        1 高  $separator
    │   ├─ Frame  Agents Section             28 高  间距 8  内边距 [0,12]
    │   │   ├─ Text   Agents Label           13 600  $text-primary   "Agents"
    │   │   └─ Text   Grouped Label          12 normal  $text-tertiary "Grouped"
    │   └─ Frame  Agent Groups               纵向  间距 2  内边距 [2,6]
    │       ├─ Frame  Group …                22 高  间距 8  内边距 [0,6]
    │       │   ├─ Ellipse  Group Mark …      7×7  无填充 + 描边 $text-tertiary 1.5
    │       │   ├─ Text    Group Name …       13 600  $text-primary
    │       │   └─ Text    Group Count …      12 normal  $text-tertiary  "· N"
    │       ├─ Frame  Agent …                 20 高  内缩 22  图标 14  文字 13 $text-primary
    │       └─ Frame  State …                18 高  内缩 40  文字 12 $green-text（idle）
    │
    └─ Frame  Main                           撑满  填充 $panel  圆角 16  描边 #FFFFFF66
        ├─ Frame  Terminal                   撑满  填充 $terminal-bg  内边距 [16,18]
        │   ├─ Frame  Line Base               19 高
        │   │   └─ Text   Seg 1               13 mono  $text-secondary  "(base)"
        │   ├─ Frame  Line Prompt             19 高   ← 终端字符按语义分色，见下
        │   ├─ Frame  Line Cmd                19 高
        │   ├─ Frame  Line Pwd                19 高
        │   ├─ Frame  Line Base 2             19 高
        │   ├─ Frame  Line Prompt 2           19 高
        │   └─ Frame  Line Input              19 高
        │       └─ Rectangle  Cursor         7×15  #55555C
        │
        ├─ Frame  Tab Strip                  36 高  填充 $panel  上描边 $separator  间距 4  内边距 [0,8]
        │   ├─ Frame  Tab 1                   高 24  圆角 6  间距 8  内边距 [0,8]
        │   │   ├─ Ellipse  Tab Dot 1         7×7  $green
        │   │   ├─ Text    Tab Label 1        13 mono normal  $text-secondary  "1"
        │   │   └─ Icon    Tab Close 1        11×11 $text-tertiary
        │   ├─ Frame  Tab 2                   高 24  圆角 6  间距 8  内边距 [0,8]
        │   │                                     填充 #FFFFFFB3  描边 #FFFFFF8C 1
        │   │   ├─ Ellipse  Tab Dot 2         7×7  $yellow
        │   │   ├─ Text    Tab Label 2        13 mono 600  $text-primary  "2"
        │   │   └─ Icon    Tab Close 2        11×11 $text-secondary
        │   ├─ Frame  Add Tab                 24×24 圆角 6  图标 14×14 $text-secondary
        │   ├─ Frame  Tab Spacer              撑满
        │   ├─ Frame  Split Panes Button      26×24 圆角 6  图标 14×14 $text-secondary
        │   └─ Frame  Tab Menu                26×24 圆角 6  图标 14×14 $text-secondary
        │
        └─ Frame  Meter Row                  26 高  填充 $panel  上描边 $separator  间距 2  内边距 [0,12]
            ├─ Frame  Meter …                 高 16  间距 8
            │   ├─ Icon    Meter Icon         12×12 $text-tertiary
            │   ├─ Frame   Meter Bar          28×4  圆角 2  填充 #FFFFFF8C
            │   │   └─ Rectangle  Meter Fill  宽×4  圆角 2  $green / $yellow
            │   └─ Text    Meter Text         10 normal  $text-secondary
            ├─ Frame  Meter Spacer            撑满
            ├─ Frame  Ports Group             高 16  间距 6
            │   ├─ Icon    Ports Icon          14×14 $text-tertiary
            │   └─ Text    Ports Text          10 mono  $text-secondary
            ├─ Frame  CPU Group                高 14  间距 8
            │   ├─ Text    CPU Label           10 600  $text-secondary  "CPU"
            │   ├─ Frame   CPU Bar             44×4  圆角 2  填充 #FFFFFF8C
            │   │   └─ Rectangle  CPU Fill    宽×4  圆角 2  $green
            │   └─ Text    CPU Value           10 normal  $text-secondary  "48%"
            ├─ Frame  MEM Group                高 14  间距 8   （同 CPU 结构，$yellow）
            └─ Frame  Date Group               84×14  间距 6
                ├─ Icon    Clock Icon          12×12 $text-tertiary
                └─ Text    Date Text           10 normal  $text-tertiary  "10-20 11:24"
```

### 3.2 终端字符的语义分色

画板用 20 段文字描述一行终端输出。分色规则（可直接用于终端渲染器的着色优先级）：

| 段 | 文本 | 颜色 | 字重 |
|---|---|---|---|
| 1 | `(base)` | `$text-secondary` | normal |
| 2 | `deepseek-harness` | `$green-text` | **700** |
| 3 | ` on ` | `$text-secondary` | normal |
| 4 | `⑂ master` | `$yellow-text` | normal |
| 5 | ` via ` | `$text-secondary` | normal |
| 6 | `●` | `$green-text` | normal |
| 7 | ` v24.15.0` | `$green-text` | **700** |
| 8 | `❯ ` | `$yellow-text` | normal |
| 9 | `pwd` | `$green-text` | normal |
| 10 | `/Users/bytedance/codes/open-source/deepseek-harness` | `$text-primary` | normal |
| 16 | `[+?]` | `$red-text` | normal |

> 终端**字色由守护进程发来的单元格决定**（256 色调色板 + 修饰位）。
> 上表只描述画板示例的意图，不是终端着色的硬规则。
> 终端背景用 `$terminal-bg`；窗口其余部分用面板令牌。

### 3.3 关键视觉结论

1. **活动栏与侧边栏都是透明的** —— 填充 `#00000000`，直接坐在窗口渐变上。
   唯一有底色的是 **Main 卡片**（`$panel` + 圆角 16 + `#FFFFFF66` 描边）。
   （导出图采样验证：x=216…855 区域是连续渐变，无栏带分界。）
2. **只有选中态有面** —— 活动栏当前项填充 `$selection`；侧边栏当前空间行填充 `$selection`；
   标签页当前项填充 `#FFFFFFB3` + 描边 `#FFFFFF8C`。
3. **徽标浮在图标右上角**，越出按钮右边缘约 3.5px，两位数时自动加宽。
4. **主卡片里的终端有 16/18 的呼吸边**，终端字符不贴卡片边缘。
5. **窗口渐变**：亮色 `#DCCFF9 → #F7CDE3 → #FFE7CE`（135°），
   暗色 `#1E1638 → #2E1836 → #35201A`（135°），帧圆角 12，描边 `#FFFFFF59`（亮）/ `#FFFFFF21`（暗）。

---

## 4. 其它画板

`docs.pen/01.pen` 共 **71 张根画板**（35 亮 + 35 暗 + 1 张令牌板），
完整清单与阶段映射见 `docs/01-implementation-plan.md` §1.1。

渲染导出（亮/暗 PNG）位于 `design/`，**不入版本库**——`.gitignore` 已排除，
需要时从 `.pen` 重新导出。

---

## 5. 实现约束（mygo v0.6.2）

| 约束 | 结论 |
|---|---|
| `ui.Theme` 只有 20 个固定槽位 | 令牌不能 1:1 投影，**组件必须直接读 `design.Tokens`**，`ui.Theme` 只做兜底 |
| `ui.Color` 支持 alpha | `RGBA(r,g,b,alpha)`，材质规则（半透明填充 + 描边）可实现 |
| `Element.Font(family string)` | 接受**逗号分隔的字体列表**，可写回退链 |
| `Painter.Shape` / `Painter.Glyphs` | 终端必须**逐格定位**：按字符 shaping 后用 `Glyphs` 放到格子坐标上，否则宽字符会让整行错位 |
| `Painter.RichText` | 段落式排版，**不适合终端网格**（按字形宽度推进，不按格子） |
| `ui.Context.TitleBar()` + `Element.DragWindow()` | 自绘标题栏的标准做法：留出 `bar.Left/Right` 给系统控件，整行 `DragWindow()` |
