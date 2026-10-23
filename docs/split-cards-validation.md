# 独立分屏卡片验证

日期：2026-10-24。按用户提供的Corra分屏参考调整多pane外观；上一条要求隐藏的全局工作区/标签/路径信息与pane ID回退标题继续隐藏。

多pane先按原权威矩形投影，在共享边缘两侧各收缩4px，外边界不收缩，形成8px间隙。各卡片独立背景、12px圆角、轻阴影和边界；间隙显示原生外壳。服务端树、比例和divider中心线不变，不按卡片缩水后的尺寸反算服务端路径。

32px轻量标识栏只使用明确的pane.label；普通shell无label时显示Terminal，不用title/cwd/pane ID回退。代理徽标来自实际agent字段。现有AgentSessionInfo没有model/version，不能从截图填入或假装已探测；没有这类数据时只显示代理名。标题栏使用KeepFocus，点击仍走本地pane选择；焦点恢复与差量host保留原行为。单pane不增加此标识栏。

卡片实际内容空间需扣除2px边界、32px标识栏及插件32×24padding。创建需求与绘制均使用同一正尺寸判据，包括状态stale时的旧binding，防止小卡片把零尺寸View交给插件。各pane仍仅调用一个host.View，没有新的PTY/终端绘制或竞争resize。

Divider保留8px命中区和原布尔路径/结构指纹验证，视觉线只在hover/drag显示并提供方向resize光标。之前帧内layout来源匹配的panic修复继续保留。

测试覆盖左右＋嵌套上下边缘8px间距、外边界保持、真实agent徽标、opaque ID/terminal title不显示、卡片内容高度与小尺寸过滤、已有divider/focus/关闭回归。Headless卡片外观图输出到dist/split-card-renders（测试空白body，仅用于布局/表面检查，不是真实终端或OS截图）。实际终端行为通过/tmp隔离服务的五组窗口集成验证，配合竞态、vet、打包与严格签名校验。
