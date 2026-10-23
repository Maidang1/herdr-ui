# 独立分屏卡片验证

日期：2026-10-24。按用户提供的Corra分屏参考调整多pane外观；上一条要求隐藏的全局工作区/标签/路径信息与pane ID回退标题继续隐藏。

多pane先按原权威矩形投影，在共享边缘两侧各收缩4px，外边界不收缩，形成8px间隙。各卡片独立背景、12px圆角、轻阴影和边界；间隙显示原生外壳。服务端树、比例和divider中心线不变，不按卡片缩水后的尺寸反算服务端路径。

32px轻量标识栏只使用明确的pane.label；普通shell无label时显示Terminal，不用title/cwd/pane ID回退。代理徽标来自实际agent字段。现有AgentSessionInfo没有model/version，不能从截图填入或假装已探测；没有这类数据时只显示代理名。标题栏使用KeepFocus，点击仍走本地pane选择；焦点恢复与差量host保留原行为。单pane和zoom也保留32px标识栏及操作按钮，确保可以首次分屏或恢复。

卡片实际内容空间需扣除2px边界、32px标识栏及插件32×24padding。创建需求与绘制均使用同一正尺寸判据，包括状态stale时的旧binding，防止小卡片把零尺寸View交给插件。各pane仍仅调用一个host.View，没有新的PTY/终端绘制或竞争resize。

Divider保留8px命中区和原布尔路径/结构指纹验证，视觉线只在hover/drag显示并提供方向resize光标。之前帧内layout来源匹配的panic修复继续保留。

测试覆盖左右＋嵌套上下边缘8px间距、外边界保持、真实agent徽标、opaque ID/terminal title不显示、卡片内容高度与小尺寸过滤、已有divider/focus/关闭回归。Headless卡片外观图输出到dist/split-card-renders（测试空白body，仅用于布局/表面检查，不是真实终端或OS截图）。实际终端行为通过/tmp隔离服务的五组窗口集成验证，配合竞态、vet、打包与严格签名校验。

卡片按钮补充：右分屏、下分屏、放大/恢复、关闭，窄卡片收为同样四项的操作菜单。按钮直接使用所在卡片身份；不以当前聚焦pane替代。测试覆盖身份替换/旧epoch拒绝、单pane放大后的恢复入口、关闭去重、远端拒绝、不确定响应不重放、权威快照确认，以及隔离服务中逐次关闭分屏和最后一个pane。关闭会结束远端终端，应用退出仍只detach。

按钮改动最终验证（2026-10-24）：`./scripts/go.sh test -race ./...`、`./scripts/go.sh vet ./...`、`make integration`（五组隔离服务测试）、`make package`、`codesign --verify --deep --strict 'dist/darwin-arm64/Herdr UI.app'`及`git diff --check`均通过。重新生成并检查浅色卡片布局图，按钮与代理徽标无重叠。只读复核通过；真实系统鼠标、VoiceOver和IME仍需桌面交互验收，headless渲染不能替代这些验证。
