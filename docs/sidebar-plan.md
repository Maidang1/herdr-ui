# 侧栏信息对齐

依据：用户提供的当前GUI与预期Herdr侧栏截图。此要求替代主文档原先“路径副行、不增加代理分组”的侧栏决定；其余终端与本地选择边界不变。

- [x] spaces行展示工作区名、真实分支及ahead/behind、仓库来源图标；路径只作tooltip，不展开shell标题。
- [x] 端口行来自本机监听socket：优先pane shell进程后代，外部启动服务使用实际process cwd与唯一规范化checkout路径匹配；不按名称或模糊前缀猜关联。
- [x] agents独立分区，仅包含实际报告的agent，展示所属工作区/agent/状态，点击按pane ID选择，不按同名合并。
- [x] API缺少TUI缓存字段时，后台有界读取本地Git与进程/端口，按workspace/terminal身份缓存并隔离迟到结果；render不执行IO，状态快照仍事件驱动。
- [x] UI保留原生外观、紧凑字号、底部连接状态和键盘入口，不添加工作区创建/删除或端口启动动作。
- [x] 数据/竞态/真实仓库与监听进程、UI渲染与完整回归，文档和本地提交。

验证与当前数据边界见 [侧栏验证记录](sidebar-validation.md)。
