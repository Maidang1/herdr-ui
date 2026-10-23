# M4 原生收尾验证记录

日期：2026-10-23。环境沿用 macOS arm64、Go 1.27.1、Herdr 0.9.5 / protocol 22 与锁定 mygo `80afe0af6f9d`。M4 沿用主文档既定设计；不增加新建工作区、标签或工作树入口。

## 界面与交互

- 头部恢复为40px单行，包含工作区名、当前标签、可截断并复制的真实路径，以及28px `columns-2` 布局入口。M3 常驻第二行按钮已移除，prefix/resize 仅出现临时键盘提示；错误原因保留可见文字。
- 标签使用原生弹出菜单，列出真实名称、编号、pane数量与当前勾选；长名称截断但保留 tooltip，选择仍是窗口本地状态。原生菜单处理方向键、Enter及取消。
- 参考面板在右侧，固定344px、非模态，没有 scrim 或焦点陷阱。打开时聚焦关闭控件，关闭请求恢复终端焦点；Esc 只在面板自身焦点范围处理，终端 Esc 不被全局关闭逻辑截获。
- 侧栏默认248px，可用8px命中区在220–320px拖动。底部有28px键盘说明入口。参考面板打开且空间不足时临时隐藏侧栏，保留用户手动隐藏偏好；面板关闭后恢复。窄窗口中“Show sidebar”优先关闭参考面板以实际显示侧栏，不错误反转隐藏偏好。
- 820×560的布局按344px参考面板、至少400px终端计算，保留系统交通灯占位、工作区名、标签和布局入口，必要时隐藏路径。测试额外提供80px交通灯区域，不能以普通无标题栏的测试窗口代替。
- 帧内选择变化仍使用M3修复后的捕获布局来源，不再用新布局的数组下标索引旧 divider 投影。

## 外观与辅助功能

`internal/design` 集中提供普通/失焦选中表面、侧栏 tint/fallback、控制边界、焦点、文字、阴影和原生控件主题。系统 UI 字体保留空 family，字号13px，辅助文字12px；终端字体、16/256/RGB颜色和插件 VT 不变。

原生窗口使用 mygo.VibrancySidebar，只有真实 material 可用且未要求实底时，外壳三 stop 渐变使用0.96 alpha；内容与文字保持不透明。材质不可用或减少透明度时绘制不透明外壳和侧栏fallback。增加对比度时关闭材质、使用控制边界，选中项失焦只换背景，不降低文字透明度。

原生 View → Appearance 提供 System / Light / Dark。mygo.Theme.OnUpdated 与公开 Preferences 监听减少透明度/增加对比度并设置/移除原生材质；不轮询系统偏好，不新增插件补丁。Window focus/blur更新失焦配色并取消未完成prefix/resize；监听在窗口关闭时注销，所有排队UI操作仍检查closed。

控件提供名称、状态提示、Selected/Expanded辅助状态与键盘焦点指示；pane 的名称包含真实agent（若有）和状态。中性选中表面明确覆盖 mygo.Selected 的默认accent文字，避免暗色蓝黑文字或亮色白字不可读。Show/Hide Sidebar与Zoom/Restore原生菜单标签跟随当前视图，更新前先计算当前侧栏可见性。

## 验证证据

新增 headless 回归覆盖344px右侧面板、400px最小终端、临时与手动侧栏状态恢复、面板Esc范围、原生标签菜单真实记录与选择、侧栏拖动限值、交通灯区域、动态菜单状态、亮暗主题跟随、减少透明度/高对比度的opaque fallback及失焦取消输入。原有M3 divider切换布局panic、prefix/组字状态、迟到操作与生命周期测试保留。

`HERDR_UI_RENDER_DIR=/tmp/herdr-ui-m4-render ./scripts/go.sh test -race ./internal/ui` 导出8张headless UI渲染：820×560与1200×800，亮/暗、面板开/关。已目视检查紧凑参考布局和宽屏亮暗布局；图片副本在忽略的 `dist/m4-renders`。这些渲染使用测试快照和无插件的内容区，只证明应用UI布局，不是操作系统窗口或真实终端截图。

Host新增100次快速可见集合切换回归：始终可见的pane binding保持，所有创建实例在退出后恰好关闭一次，迟到安装不能复活。真实原生多窗格集成增加200行、每行间隔10ms的周期输出，验证窗口取得全部可见owner、驱动首帧尺寸、输出完成及退出后相同terminal可重新连接。该有界输出不是长时间CPU/内存或输入延迟基准。

完整命令：`./scripts/go.sh test -race ./...`、`make test`（含vet）、`make integration`、`make package` 与严格codesign校验。五组真实集成保留M0/M1/M2/M3覆盖，所有服务端修改位于/tmp隔离服务，不接管用户活动会话。

## 实机验收边界

系统中文输入法候选/预编辑、真实鼠标拖动与程序接管鼠标/alternate-scroll、窗口内输入不串pane、原生复制粘贴、VoiceOver朗读、系统外观/减少透明度偏好切换，以及长时间持续输出/大粘贴/resize交错下的响应性仍需实机验收。headless焦点/偏好测试、后端协议或短时周期输出不能替代这些证据。完整Herdr键位resolver仍有M0/M3记录的明确子集限制。

周期输出回归暴露了原有测试的Ready观察竞态：观察Ready后、入队前的scroll事件可使Session进入Resyncing，操作正确返回StaleTarget。集成测试改为仅对确定未发送的StaleTarget做5秒内有界等待并重新捕获身份；OutcomeUnknown/Rejected等结果立即失败且不重放。产品的修改操作队列与安全拒绝规则没有改变。
