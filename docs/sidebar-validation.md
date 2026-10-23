# 侧栏信息修正验证记录

日期：2026-10-23。依据用户提供的GUI与Herdr TUI侧栏对比图，本次替代旧版“路径副行＋普通终端展开列表”，保留M4原生外观与M3终端行为。

## 最终展示

spaces依服务端工作区顺序展示名称、真实分支、非零ahead/behind计数、来源图标及监听端口。路径只在tooltip/内容头部展示，普通shell提示符不再展开到侧栏。GitHub使用真实origin对应品牌图标，其它Git来源使用通用图标，非Git目录使用文件夹；不伪造分支或来源。

agents独立按工作区分组，包含实际报告agent的pane，显示工作区名、agent及状态；不把普通shell加入，不按同名合并。点击按PaneID切换本窗口选择，保留workspace/tab/pane记忆和其它可见attach。工作区状态来自API的聚合状态，不能被所选普通shell的unknown覆盖。UI统一兼容状态字符串大小写。

## 数据来源与刷新

目标Herdr0.9.5的session.snapshot没有TUI使用的cached Git/ports字段，当前用户工作区也未通过tokens提供这些值。没有通过解析终端提示符取得分支，也没有修改、重启或接管用户服务。

新增internal/sidebarinfo辅助采集，后台读取明确checkout_path或权威tab/pane顺序中第一个有效cwd。本地Git使用参数数组、只读命令、清理继承的GIT_*定位/配置环境、不加索引锁；读取symbolic-ref、upstream差异计数、repo root和origin类别。origin URL或凭据不进入发布模型及日志。detached/nonGit/失败时不补造main或远端差异。

端口优先通过显式pane.process_info的shellPID与监听进程祖先链关联；独立IDE/其它终端启动的服务读取监听PID真实cwd，按唯一规范化工作区目录归属。Git checkout允许目录内后代，非Git仅精确cwd；处理路径边界、symlink逃逸、HOME/root过宽目录、最长唯一匹配与同目录歧义。监听进程必须在前后两次ps快照中都存在，且PID/parent/birth身份稳定；不能把两次都缺失的零值当稳定PID。端口去重排序，IPv4/IPv6重复只显示一次。部分失败只保留确认信息，并标记范围不完整，不将全机端口分配给某个工作区。

辅助本地信息每5秒最多一次串行批次，Git最多4个workspace并行；每命令2秒、批次8秒、stdout上限2MiB、stderr64KiB。shell根身份缓存30秒，缺失短缓存，目标/pane/terminal移除清理缓存；空工作区不做系统采集。此周期只针对未由API导出的本地信息，session.snapshot仍由已确认订阅的事件触发，没有全量快照轮询。

源变化立即取消旧代次。发布包含TargetID及workspace/path/pane/terminal指纹，App在当前快照身份仍匹配时才安装并复制slice，换目标即清空旧信息。metadata更新不提升本地ViewRevision，因此不会取消用户选择意图或抢焦点。渲染只读取缓存，没有IO。关闭加入原有共享5秒预算，排队UI回调继续检查closed。

## 验证

- 模型/App/UI：真实agent过滤、同名agent的PaneID隔离、workspace聚合状态、分支/计数/端口展示、普通shell标题和路径不进入侧栏文本、agent点击选择、metadata slice隔离、替换terminal/复用workspace ID换target的迟到结果拒绝。
- Git真实临时仓库：分支与upstream双向1/1差异、继承Git环境污染隔离、无远端/nonGit/失败处理；不需要网络fetch。
- 真实子进程TCP监听：只收归属进程、排除其它listener；parser回归覆盖IPv4/IPv6、PID复用、前后快照缺失、partial错误与cwd边界/symlink/歧义。
- Collector：相同源不因Session revision重复采集、最新源取消旧任务、输入及发布数据独立复制、关闭清理。
- 原有M0–M4的协议、焦点、分屏、比例、IME状态机与资源回归保留；完整race/vet、五组/tmp隔离真实服务及原生窗口集成、构建/打包/签名验证。

只读核对当前实际会话：七个工作区分支可读取，coze-loop-frontend为↑13/↓11；deepseek-harness端口8080/8081/9200、daimon3180、dsh4310可按实际cwd核实，herdr-ui的当前子进程监听也能按身份归属。旧截图中的52325/9900当前进程cwd分别为临时测试目录和.bifrost，4331/4340位于dsh-upstream-kernel兄弟目录；在没有明确归属证据时不挂到截图所示工作区。端口随当前进程启停变化，不是固定配置或截图常量。

界面渲染使用测试快照，只验证原生侧栏信息与交互，不冒充真实终端/操作系统截图。系统IME、实际鼠标和VoiceOver等M0/M4尚需实机验收的边界保持不变。

另用当前只读session快照和已采集Publication生成侧栏实际数据预览：`dist/sidebar-renders/live-sidebar-dark-{false,true}.png`。opt-in测试通过HERDR_UI_SIDEBAR_SNAPSHOT/HERDR_UI_SIDEBAR_METADATA/HERDR_UI_RENDER_DIR读取指定本地文件，不创建terminal host、不发服务调用、不取得owner。它是实际数据的headless侧栏渲染，仍不是操作系统截图。
