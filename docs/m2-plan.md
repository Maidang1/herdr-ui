# M2 单终端实施计划

目标：默认窗口按本地当前选择显示一个真实终端；保留 M1 状态订阅和 M0 验证入口。不在 M2 增加分屏布局修改或业务 prefix 动作。

- [x] terminalhost：公开插件适配、目标/terminal/generation 注册表、后台初始化、晚到结果丢弃、退出状态、显式重试和有界关闭。
- [x] Session：pane scroll 指标、当前可见 pane 的 scroll_changed 订阅、显式目标 Scroll 操作；每目标一在途与一最新 pending，旧 pending 返回 Superseded。
- [x] UI/App：按有效选择协调单终端；JSON stale 保留已有连接、不新建；新快照映射变化才替换；切换释放旧本地 attach、错误与 Retry 原位显示。
- [x] 输入与主题：普通输入/IME/复制粘贴交给插件，字面 Ctrl-B 使用 attach 转义；OnClear 可见反馈；默认主题 token、服务端 wheel 与程序接管路径分离。
- [x] fake 生命周期/竞态/滚动契约测试、真实插件与独立 Herdr 集成、原生窗口/构建/打包/签名回归。
- [x] 独立审查、验收记录、提交 M2 增量。

M0 的系统 IME 等人工验收状态继续如实保留。所有真实测试只操作独立临时服务，不接管用户正在使用的 terminal。
