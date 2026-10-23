# M2 单终端验收记录

日期：2026-10-23。沿用锁定的 Go 1.27.1 / mygo `80afe0af6f9d0608b05caed22f4607205a60c3a5`。真实验证为 macOS arm64、Herdr 0.9.5；M0 尚未通过的系统交互验收不因 M2 自动测试而变为通过。

## 已实现

- 默认窗口将当前本地选择的 pane 映射到真实 terminal ID，每次显示一个终端。已有工作区、标签和 pane 选择继续使用 M1 的本地记忆；没有新建工作区或分屏入口。
- `internal/terminalhost` 封装公开插件 New/View/Close，绑定使用 SessionTargetID＋TerminalID＋TerminalGeneration。Reconcile 只改需求，后台创建/关闭，不在 UI 等待 IO。
- 新实例先登记资源所有权，再排队 UI 安装；即使窗口从不执行该安装，也可由 Close 清理。移除后晚到的 New、旧 OnTitle/OnExit、相同 terminal 重试后的回调都不能影响新实例。
- 同一目标/terminal 的新代次在后台等待旧代次初始化及清理结束；快速移出/再加入中的中间代次不会越过这条依赖链或启动工厂，UI 不阻塞。插件 Close 只发起挂断，清理任务还等待公开 Done 结束信号；该信号不能证明服务端已经释放 owner。
- 只有内容区得到有效尺寸且 JSON 状态 Ready、pane→terminal 映射仍有效时才创建。状态 stale 时保留当前匹配实例，暂停新的 attach/Retry/滚动请求；JSON 重连不作为终端身份，不因 epoch 变化重启已有终端。
- 切换选择后释放不再显示的本地 attach。新快照确认 terminal 替换后才重建。退出先从 UI 解绑，再以共享 5 秒预算关闭 Session、host 与 CLI 检查任务。
- attach 命令为显式参数数组，使用同一个解析后的 socket，不加 takeover，不留空 Command，不回退启动 shell。CWD 可用性检查在后台进行。特别处理插件 Env 会叠加父环境的行为：显式清空继承的 HERDR_SESSION / PANE_ID / TAB_ID / WORKSPACE_ID，防止过滤后重新带入旧身份。
- 初始化失败、attach 退出都有原位错误与显式 Retry。失败实例保留终端最后画面，以展示 CLI 的真实占用/版本诊断，不提取或记录终端正文；Retry 分配新代次。
- 普通输入、IME、复制粘贴、键盘协议、鼠标编码与尺寸由插件 View 负责。M2 的 OnKey 只处理字面 Ctrl-B 的双重 attach 转义；没有提前接入 M3 业务 prefix。程序读取剪贴板默认拒绝；右键使用 CopyOrPaste，避免提供不可用的 Clear 菜单项。OnClear 消费动作并显示可见说明。
- 复制插件亮暗 ANSI 调色板，仅覆盖主文档的默认背景/前景/光标/选区；字号 13，插件 padding 为 16×12，无第二套终端绘制、PTY 或 resize 队列。
- App 的投影版本随快照/本地选择变化；Window 缓存投影，终端输出的重绘不重新构造工作区树。

## 服务端滚动

Snapshot 保留可选的 offset/max/rows 指标。Session 为当前可见 pane 增加 `pane.scroll_changed` 确认订阅，替换先建后拆；事件只触发权威快照，没有轮询。

Execute 当前只接受 Scroll，捕获目标 ID、epoch、pane/terminal 映射，在入队与发送前验证。所有请求与快照共用串行 lane；每个目标只有一个在途、一个可替换 pending，替换结果为 Superseded，队列上限 32。结果通道容量为 1、完成后关闭，不重放超时请求。

Wheel 按最新期望位置累积并限制到服务端范围，不反复从慢快照的旧 offset 计算。失败回到权威值，换 binding/代次丢弃推测位置。普通 wheel 由 OnScroll 消费；程序接管鼠标或 alternate-scroll 时由插件原生编码处理，不拦截该路径。本阶段未添加本地历史滚动条。

父 context 取消会先关闭新操作准入，再完成 pending 通道，避免在最后一次 drain 之后仍入队而永久等待。

## 验证与限制

- `make test` 与 `go test -race ./...` 通过 114 项测试，vet 无问题；`make integration` 的 M0/M1/M2 三组真实集成全部通过，CGO_ENABLED=0 构建和打包签名通过。普通测试使用 fake factory，不加载终端动态库。
- Host 的 16 项测试覆盖保留/重复需求、初始化晚到、排队安装不执行、移除再加入、早期退出、重试、关闭预算、旧回调、输入转义、主题、Env 策略，以及同目标工厂/清理屏障和连续跳代关闭。
- Session/App/UI 回归覆盖 scroll 指标、慢请求只发第一/最后值、中间结果 Superseded、失效目标拒绝、scroll_changed 订阅与退休、失败回滚、换代次重置、空会话文案、投影缓存失效，以及父取消并发操作全部完成。
- M2 真实窗口测试先把测试终端置为 80×24，再验证默认窗口改变服务端尺寸；第二个 attach 被拒绝，证明当前窗口取得真实 owner。窗口自动退出后，用相同 terminal ID 再 attach，实际执行专有输出成功，证明未关闭远端终端。
- 测试终端生成真实历史后，Session 发送绝对 scroll=5，响应和事件更新后的权威指标均为 5。所有修改只发生在 `/tmp` 的独立测试服务。
- 保留 M0/M1 全部集成回归。没有通过实验接管用户当前终端。
- Herdr 服务端可能在本地进程退出后的下一轮事件循环才释放 owner；测试的重新连接探针只在明确本地关闭后，对这种占用做有界重试。产品仍是显式 Retry，不自动接管或重放业务请求。

系统中文输入法候选/预编辑、原生复制粘贴、程序接管鼠标/alternate-scroll、连续输出/大粘贴与 resize 交错的真实窗口响应性仍须实机验收；目前不能用 fake、headless 引擎或后端协议测试声称它们通过。M3 分屏、布局修改、完整键位 resolver/业务 prefix 与 M4 原生收尾尚未实现。
