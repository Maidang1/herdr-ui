# M3 标签、分屏与输入验证记录

日期：2026-10-23。环境：macOS arm64，Go 1.27.1，Herdr 0.9.5 / protocol 22，mygo 固定版本 `v0.6.3-0.20261021150203-80afe0af6f9d`。

## 实现

当前标签按服务端 pane 矩形投影到原生容器，每个可见 pane 一个独立 terminal binding，每帧统一 Reconcile。选择 pane 不关闭其它可见连接；切换标签或 zoom 差量释放隐藏连接，恢复时经过原有同 terminal 清理屏障再创建。只在内容尺寸有效时创建，不替换插件输入、VT、PTY 或鼠标处理器。所有可见 pane 共用一个带明确 pane ID 的 scroll_changed 订阅批次。

真实服务的 zoom 快照仍包含底层全部矩形。模型在完整校验后按 zoomed 与该标签 FocusedPaneID 生成唯一可见窗格，并使用整个 area；App 避免选择隐藏窗格。普通本地浏览和方向选择不发送 focus；显式 split/zoom 按服务端语义改变焦点。

闭合操作包括 Neighbor / Split / Zoom / ExportLayout / SetRatio，捕获 target、epoch、workspace/tab、pane/terminal。重复 split/zoom 拦截，普通操作 FIFO，比例保留一个最新 pending 槽；每次比例修改前重新 export 验证忽略比例的结构指纹与 bool 路径。准备性 export 失败确定为 NotSent；其它不能确定是否发送的网络失败保守报告 OutcomeUnknown，绝不自动重放。

Split/Neighbor 返回 pane 后等待完整快照确认。意图记录 ViewRevision，迟到结果不能抢回已切换的标签或窗格；快照先于响应到达也可正确完成。未知结果显示检查刷新后布局的提示。

Divider 通过 fresh export 树的 descendant pane 矩形并集匹配路径，不解释 opaque split ID。权威边界按 Herdr 的 float32 网格舍入后投影，命中区域 8 DIP，视觉线 1 DIP。拖动预览使用独立克隆树，发送间隔至少 50ms 并保留 release 最后值；暂时 resync 暂停发送，拓扑、目标或本地选择变化取消。ResizeMode 优先调整请求方向边界，缺少时回退另一侧。

工具栏、原生 View / Terminal / Help 菜单与 prefix 共用 Dispatch/Can；包含右分、下分、方向选择、标签循环、zoom、resize、侧栏和基础键位参考。所有 UI 读取共用按 App.Version 缓存的投影。参考面板视觉收尾留给 M4。

## 组字能力与配置边界

原始固定版插件没有终端组字查询。`scripts/prepare-mygo.py` 从精确版本复制到忽略的 `.build/mygo`，只增加 atomic composition 标志、公开 `Composing()` 和 compose/commit/focus-loss 更新；不修改 module cache、不提交绝对路径 replace，也不复制私有 painter 到业务代码。生成记录包含版本和脚本 SHA-256；本次脚本摘要为 `053ac1a6bf39bca3df3cb26e524efbe0eb76688145cb7a2fe6e76c83f772b959`。所有 Makefile Go 命令通过 `scripts/go.sh` 使用生成工作区，包含打包器子进程。

业务 prefix 仅在公开 composition capability 可用且未组字时启用；组字、失焦、目标或选择变化取消 prefix/resize。组字期间不运行 literal Ctrl-B fallback。缺少 capability 或配置有误时显示原因，按钮仍可用；未打补丁的普通 Go 构建可编译，但业务快捷键明确禁用。

配置来源顺序为显式路径、HERDR_CONFIG_PATH、XDG_CONFIG_HOME/herdr/config.toml、用户 ~/.config/herdr/config.toml；仅文件不存在时采用已核对默认值，语法/不支持形式不静默回退。目前 resolver 仍是明确子集：部分 navigate/indexed/hyper 配置、非 ctrl+letter prefix 等会诊断；command bindings 预留但不执行，不宣称完整 Herdr resolver 一致性。自定义 prefix 时，直接 Ctrl-B 保留给 attach 字面传输并拒绝作为业务绑定。ResizeMode 的 q 留在模式内，Esc/Enter/配置绑定退出，方向键按键码匹配修饰符。

## 自动验证与尚需验收

自动测试覆盖不可变几何、共享边、zoom、嵌套方向路径、比例合并与路径拷贝、失效指纹不修改、export 失败未发送、重复 split、响应与快照先后、迟到意图、本地选择、prefix/转义/组字状态、失焦与配置错误，以及全部可见滚动订阅替换。Host 原有生命周期、跳代屏障和退出预算回归保留。

真实服务隔离在 /tmp：验证右分/下分、neighbor 不改共享焦点、zoom/restore 可见集合、嵌套 [true] 比例路径与指纹稳定；原生窗口验证所有可见 pane 均有 attach owner（第二 owner 被拒绝），窗口关闭后相同 terminal 可重新连接，远端会话继续存活。M0/M1/M2 回归保留。

系统中文输入法候选/预编辑、真实鼠标拖动、窗口内输入不串 pane、原生复制粘贴、程序接管鼠标/alternate-scroll、持续输出/大粘贴与 resize 交错的系统交互仍需实机验收。自动测试和协议证据不能替代这些验收。M4 参考面板、外观/材质/辅助功能收尾尚未完成。

验证命令：`make test`（含 vet）、`./scripts/go.sh test -race ./...`、未打补丁依赖的纯 Go 核心包测试、`make integration` 五组真实集成、`make package`、`codesign --verify --deep --strict 'dist/darwin-arm64/Herdr UI.app'`。应用输出位于 `dist/darwin-arm64/Herdr UI.app`。没有 push 或修改用户活动服务。

独立规格与质量审查中修复了：保留 Ctrl-B 绑定误报、ResizeMode 规则与嵌套边界、准备阶段 NotSent 分类、zoom 快照归一化、缓存绕过、divider 网格命中偏差，以及只读 export 造成新快照再触发 export 的循环。结果丢失的 split 在重连后不会自动重放，测试确认服务端仅接收到一次请求。

比例完成回调还校验本地单调 resize generation：旧拖动/键盘操作的迟到错误不能覆盖新操作；最终 release 后预览已移除的错误仍可见。分隔线 KeepFocus 的 headless 点击回归证明应用内部焦点保留；按键入口在派发前校验最新 pane→terminal 映射，旧 binding 不能操作替换后的新 terminal。这些内部回归仍不等同于系统鼠标/输入法验收。
