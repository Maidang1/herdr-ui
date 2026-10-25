# Herdr UI

Herdr 的原生桌面客户端，第一版面向本机 macOS，使用 **Go + mygo 原生 UI与内置终端插件**。展示已有工作区、工作树、代理状态与终端，支持标签、分屏和 Herdr prefix 模式。

目前已实现 M4 原生收尾：当前标签的多窗格终端支持分屏、方向选择、zoom 与比例调整；精简的终端内容区、可调整侧栏和右侧快捷键参考面板适配最小 820×560 窗口。原生菜单提供 System / Light / Dark，外壳跟随减少透明度与增加对比度偏好。阶段证据见 [M0](docs/m0-validation.md)、[M1](docs/m1-validation.md)、[M2](docs/m2-validation.md)、[M3](docs/m3-validation.md)、[M4 验收记录](docs/m4-validation.md)。系统 IME、真实鼠标和 VoiceOver 等交互仍需实机验收。

对照 Herdr 与 Kero（同框架的 agent 终端）实机与源码调研后，补齐了一批终端相关能力（清单与取舍见 [功能对齐清单](docs/feature-parity-plan.md)）：外观与字号偏好持久化（⌘=/⌘-/⌘0，启动恢复）；agent 状态系统通知（需要你/完成）与 Dock 角标，点击跳转对应 pane；终端内查找条 ⌘F（实时计数与步进）；⌘+点击打开打印出的文件路径；⌘K 按 Herdr 语义清屏；Zen 模式 ⌃⌘Z；Go To 面板升级为会话目的地 + 命令的混合面板（frecency 排序）。⌘B 切换侧栏、⌘J 跳到等待中的 agent。常用 Mac 终端键位直接可用：⌘T、⌘W、⌘D/⇧⌘D、⌘1–9、⇧⌘[ ⇧⌘]、⌘[ ⌘]、⌥⌘方向键；鼠标拖动即可选中文字。设置窗口（⌘，）提供外观、终端（字体族/字号/字重/行高/光标形状与闪烁/Option 作 Meta/选中即复制/最小对比度，热更全部终端）、通知开关与快捷键参考四页，改动即时生效并落盘。

- [设计与实现方案](docs/implementation.md)：唯一主文档，包含第一版范围、界面、架构、接口、事件同步、终端、快捷键、颜色 token 与验收步骤。
- [第一版设计稿](docs.pen/new.pen)：Workspaces、Split Panes、Switch Tabs、Shortcuts Reference 四张画板，使用 Pencil 打开。
- [API schema](reference/herdr-api-schema.json)：机器可读参考，接入时以目标 Herdr 实际版本为准。
- [图标素材与授权](assets/icons/NOTICE.md)。

界面按 Apple 的同心圆角原则组织（浮动侧栏、内容面板、分屏卡片、菜单逐层派生半径，见主文档 §9.8）。Herdr 原本只能靠 prefix 键触达的命令都有了鼠标入口：工作区、标签、pane 标题与 agent 行都有右键菜单（重命名、worktree、拷贝路径/在 Finder 中显示、分屏、交换、复制模式、清屏、关闭等）；双击工作区或标签重命名，双击 pane 标题放大/恢复；有 agent 等待回复时 Agents 标题旁出现「N waiting」按钮一键跳转；菜单栏按 Workspace / Tab / Pane / Go / View 分组，标题旁以 ⌃B 等符号标出 Herdr 键位，⌘P 打开 Go To 调色板。

侧栏按 Herdr 的 spaces / agents 信息分区：工作区显示真实分支、ahead/behind 与监听端口，代理单独按工作区分组，不展开普通 shell 标题。API 未导出的本地 Git/进程信息由后台有界采集，细节与验证见 [侧栏修正记录](docs/sidebar-validation.md)。

终端直接使用 `github.com/egoist/mygo/plugins/terminal`，通过 `herdr terminal attach <terminal_id>` 连接已有会话；状态从第一版起使用 `events.subscribe`，由事件触发权威快照更新，不定时轮询。

通用图标采用 `lucide-static`，agent 品牌图标采用 `@lobehub/icons-static-svg`：开发时按需导出并嵌入 SVG，由 mygo 显示，运行时无需 Node/npm。单色使用 `ui.Icon`，彩色使用 `ui.Image`。

`docs.pen/01.pen` 保留为历史设计源素材。第一版范围、颜色和实现决策统一以主文档及 `docs.pen/new.pen` 为准。实现从主文档中的 M0 接入验证开始。

开发环境：macOS arm64、Go 1.27.1（支持 Go 自动工具链下载）、已安装并运行的 Herdr。mygo 固定到 `80afe0af6f9d0608b05caed22f4607205a60c3a5`，构建不使用本机路径 `replace`。

```sh
make test          # 纯 Go 契约测试与 vet，不加载终端动态库
make build         # CGO_ENABLED=0，输出到 bin/
make run           # 当前标签的多终端窗口
make probe         # 只读检查当前 Herdr、配置与布局，不占用终端
make window        # 本地 /bin/cat 输入测试窗口，不连接 Herdr
make integration   # 临时独立服务 + 真实插件 + 原生窗口，结束后清理
make package       # 固定版本 mygo CLI，输出含原生库的本地签名 .app
make dmg           # 通用二进制（arm64 + x86_64）.app 与 .dmg，与发布流程一致
```

发布：把 `mygo.json` 的 `version` 改为新版本并提交，再推送同名标签（如 `git tag v0.1.0 && git push origin v0.1.0`）。[Release 工作流](.github/workflows/release.yml)在 macOS runner 上校验标签与版本一致、运行 `make test`、执行 `make dmg`，然后创建 GitHub Release 并上传 `Herdr-UI-<version>-macos-universal.dmg` 及其 SHA-256。手动运行该工作流只构建，产物留在运行记录里，不发布。应用为 ad hoc 签名，未经公证，首次打开需移除隔离属性（发布说明中附命令）。

默认窗口按事件同步，并差量 attach 当前标签的全部可见终端。普通工作区/标签/pane 选择保持窗口本地，不修改 Herdr 全局焦点；滚动通过显式目标 `pane.scroll` 更新服务端视口。可使用 `./bin/herdr-ui -socket /path/to/herdr.sock` 指定目标。

快捷键覆盖工作区、标签、pane、worktree、导航、复制模式和自定义命令。默认先按 `Ctrl+B`，再按动作键；`?` 打开快捷键参考。设置与滚动历史通过内嵌 `$EDITOR` 打开，popup/插件交互通过内嵌 Herdr 客户端显示。支持范围、平台键位限制与验证方法见 [快捷键说明](docs/keyboard-parity.md)。缩放状态下切换隐藏 pane，以及图片粘贴和服务端自定义命令，会使用显式目标更新服务端焦点。

M0 验证窗口连接某个已有终端：先运行 `make probe` 取得真实 `terminal_id`，然后执行：

```sh
./bin/herdr-ui -terminal '<terminal_id>'
./bin/herdr-ui -socket /path/to/herdr.sock -terminal '<terminal_id>'
```

不会自动 takeover、创建工作区或启动守护进程。已有 owner 时 attach 拒绝并保留其诊断画面，提供 Retry；关闭 GUI 只释放本应用的 attach。放大窗格时释放隐藏 attach，恢复时重新连接；普通浏览不发送全局 focus，显式 split/zoom 会按 Herdr 语义改变服务端焦点。JSON 探针不会周期轮询；集成测试中的短时等待仅用于验收。

构建应使用 `make` 或 `./scripts/go.sh`：它们从固定依赖生成仅增加公开组字查询的 mygo 副本，不修改 Go module cache；普通 `go build` 缺少该能力时会显式禁用业务快捷键。配置解析不支持的有效形式会显示诊断，不静默采用默认键位。

开发期 `./scripts/go.sh run ./cmd/herdr-ui -local-fixture`/`make window` 首次可能下载插件的原生库；`make package` 把校验后的 `libghostty-vt` 放入应用资源，打包应用不在启动时下载。集成测试会在 `/tmp` 创建自己的隔离 Herdr 服务与终端，绝不关闭用户已有会话。

应用图标是已提交的生成文件，普通 Go 构建不需要 Node。更新图标时执行 `npm ci && npm run icons`；清单与确切来源位于 `scripts/icons.json` 和 `internal/icons/SOURCES.txt`，许可证随打包资源保留。

App 图标的源文件是 `assets/app/icon.svg`（同心圆角的分屏窗格，配 Herdr 的 working 弧与 waiting 光晕）。改动后执行 `resvg -w 1024 -h 1024 assets/app/icon.svg assets/app/icon.png` 重新导出；`make package` 读取 `mygo.json` 的 `icon` 生成 `.icns`。
