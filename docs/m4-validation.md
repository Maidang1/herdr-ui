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


## 2026-10-24 用户精简标注

根据用户截图，移除顶部工作区/标签/路径信息栏及所有pane标题栏（包括ID回退标题），将原40px/30px高度交还终端。原生菜单与prefix继续提供标签切换、split/zoom/resize；隐藏侧栏时仅保留交通灯避让和恢复侧栏控件。Prefix/resize提示仅在对应模式出现，错误与Retry保留。更新旧头部测试，验证信息栏缺席、最小控制区不覆盖交通灯及标签切换动作仍有效。UI/App竞态测试、vet、五组真实隔离集成、打包与严格签名校验通过。

后续用户提供新的分屏卡片参考，已在保持全局信息栏隐藏的前提下加入独立卡片和最小代理标识，最新规则与验证见 [分屏卡片验证](split-cards-validation.md)。

## 2026-10-24 对齐 Herdr 侧栏与标签条

用 computer-use 实机操作参考应用 Herdr（so.pen.herdr-gpui）核对侧栏与底部标签条的功能和样式后落地：侧栏新增 Workspaces 标题行、spaces 过滤框（点击展开为实时搜索，匹配名称/分支/路径/agent，放大镜与清除按钮切换，Escape 退出）、工作区行首的 agent 状态圆环/圆点、端口行内嵌底色、new（立即建工作区）与 menu（设置/快捷键/重载配置/分离）行，以及 agents 排序切换 grouped↔priority（priority 按注意优先级平铺并带工作区上下文）。分组头部补状态环与数量。选中行改为中性底色加粗标题、分支用强调色，↑ 计数用强调色。

底部标签条改为 Herdr 的浮式排布：选中 pill 中性底色、状态环图标，未选中 tab 之间细分隔线，右侧新增侧栏开关与 Tab actions（···）弹出菜单（New/Rename/Split right/Split down/Close，按 Can 状态禁用）；工作区上下文文本仅在侧栏隐藏时显示。TextInput 聚焦期间根视图不消费 prefix 键；runShortcut 在无 attached session 时给出消息而不是崩溃。

验证：过滤（激活/输入 dai 只剩 daimon/清除/Escape）、排序切换、菜单开关均为 headless tester 测试；并在运行中的实例上经无障碍树实机核对过滤、切换与 Tab actions 菜单。侧栏渲染工件与既有测试全部通过，vet 通过。参考截图见 /tmp 对比记录，不存在对用户会话焦点的修改（误建的对照工作区已即时关闭）。

同日修正实机核对发现的三处细节：侧栏头部预留 TitleBar.Left，Workspaces 标题不再压住红绿灯；隐藏侧栏后的顶部控制条从 max(36,TitleBar.Height)（实测 66pt）压缩到固定 36pt，Show sidebar 按钮中心与红绿灯中心逐点对齐（实测均为 y=26；红绿灯实际中心高于常用假设的 16pt），标签条的 Toggle sidebar 保留（无会话无标签条时顶部按钮仍是唯一恢复入口）；端口行移出选中高亮，改为行下方的全宽独立底色条，文本与分支精确对齐（x=68），与 Herdr 的选择高亮结构一致。实测：标题 x=95（红绿灯区 14–70 之外）、隐藏态按钮中心 y=22（原 41）、首个终端卡从 y=78 提前到 y=40；标签条开关可恢复侧栏。测试与 vet 通过。

同日再按用户参考图（红绿灯 + 侧栏开关 + 新建标签 + 标签行内联的标题栏样式）重排隐藏侧栏后的顶部：36pt 标题行内依次为红绿灯让位区、Show sidebar、标签 pills、新建标签、工作区上下文与 Tab actions，底部标签条在该状态不再渲染，消除空条与重复；红绿灯、开关与 pill 中心实测同在 y=26，首个终端卡从 y=78 提前到 y≈48。侧栏可见时保持 Herdr 式底部标签条不变。

随后按用户决定把标签条固定移到顶部：侧栏可见时，36pt 标签行位于内容区顶部、与侧栏头部同一水平带（pill 中心 y=26 与红绿灯线一致），右端为 Tab actions；底部标签条与冗余的 Toggle sidebar 控件移除（侧栏可见用头部 Hide sidebar，隐藏用标题行 Show sidebar）。侧栏隐藏时维持标题行内联标签不变。Tab bar 相关测试随位置更新，vet 与全量测试通过。

同日再按用户选择把顶部标签组右对齐：顶栏左端为工作区上下文（工作区名与聚焦 agent，跟随本窗口选择），右端为标签 pills、新建标签与 Tab actions；标题行（侧栏隐藏）同样式。实测可见态：上下文 x=268、Tab 1 x=967、Tab actions 右缘 x=1162–1188。测试与 vet 通过。

同日按用户标注再调顶栏：标签组放回左端（Tab 1 实测 x=268），右端保留 Tab actions；等待 prefix 和弦时不再在内容区显示 "Prefix" 文字面板，改为顶栏右侧的小键位标签（如 "Ctrl + B"，由实际 Bindings.Prefix 生成，warning 底色），Resize 提示保持不变。新增 tester 测试：按下 prefix 后顶栏出现 "Ctrl + B" 标签且无裸 "Prefix" 字样。测试与 vet 通过。

## 2026-10-24 对齐 Kero/Herdr 的功能批次

调研：两个只读子代理分别产出 kero（同框架 mygo 的 agent 终端）与 herdr-gpui（Herdr 官方 GUI）功能报告；computer-use 实机核对 Herdr 界面；Kero 进程窗口不可用（AX 仅残留），以源码与 docs/ 为准。定位边界：GitHub PR、usage 额度、浏览器标签、teleport/云设备等一律不搬。清单与决策见 docs/feature-parity-plan.md。

批次 1（feat(prefs)）：internal/prefs 设置持久化（~/Library/Application Support/herdr-ui/prefs.json，原子写、坏值回退默认、保留未知字段）；System/Light/Dark 启动恢复（SetSource before Run）+ 菜单切换写盘；字号 ⌘=/⌘-/⌘0（8–48pt，Host.SetFont 热更全部已附加终端并持久化）；⌘B 侧栏、⌘J 跳到下一个 blocked agent（jumpToAttention 循环）。

批次 2（feat(notifications,find)）：notifier 跟踪每 pane 的 agent 状态迁移（→blocked "needs you"、working→done "finished"），仅窗口不活跃时发系统通知（首见只基线，无启动风暴；每 agent 一槽覆盖旧横幅），点击经 App.Focus+JumpTo 跳转，Dock 角标=当前 blocked 数，窗口聚焦清横幅；终端查找条 ⌘F/⌘G/⌘⇧G（插件 Search API，实时 "1 of 3" 计数，Enter/⇧Enter 步进，Esc 还焦点，跟随 pane 退役）；未聚焦分屏卡片 veil 暗化（指针穿透，tester 验证点击可达）。本批实现由两个并行子代理完成、区域互斥；期间一次二分回退误删未提交产物，从 agent 转录（rollout toolCalls）逐调用回放恢复，42 调用 41 成功 1 条为原代理自身的失败尝试，恢复后全量 -count=1 测试与原状态一致通过。

批次 3（feat(links,zen,palette)）：OpenLink——插件检测的 URL 走系统、打印的文件路径按 pane cwd 解析后 Shell.OpenPath 打开；⌘K 清屏走 pane.clear Shortcut（服务端清屏）；Zen ⌃⌘Z——窗口级隐藏侧栏/标签条/参考面板/标题条（chromeLayout 与 windowControls 增加 zen 短路，修复 reference 参数回归后测试恢复）；goto 面板升级为会话目的地+全部命令混合、frecency（计数+会话衰减）排序、命令项显示动态标签与键位提示。

验证：make test（12 包）与 vet 全绿；新增 prefs 往返/通知迁移（10 例）/find bar 交互/暗化穿透/zen 往返/palette 命令分发与排序/clear_pane 映射测试。实机（AX）：批次 3 构建侧栏/标签条/4 终端渲染正常，sidebar menu popover 开合正常；种子 prefs（dark/15pt）启动路径正常。CUA 键盘直发因与用户 Herdr 应用名称包含冲突不可用，直键（⌘F/⌘K/⌃⌘Z 等）以菜单加速器接线 + headless dispatch 测试覆盖，留待实机人工验收。
