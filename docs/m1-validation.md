# M1 只读状态窗口验收

日期：2026-10-23。基线沿用 M0 的 Go 1.27.1、mygo `80afe0af6f9d0608b05caed22f4607205a60c3a5`、macOS arm64 和实际 Herdr 0.9.5 / protocol 22。协议版本只记录，不按常量 22 拒绝新服务。

默认运行 `make run` 打开 M1 原生窗口；`-terminal` 和 `-local-fixture` 保留 M0 验证行为。M1 不启动 attach、不占用 terminal owner、不改变 Herdr 全局焦点，不创建或关闭工作区。

## 实现与协议修正

- `internal/herdr/connection.go` 提供串行请求 lane、唯一请求 ID、5 秒超时和独立订阅连接。**真实服务的普通 RPC 是一连接一请求**：Herdr `src/api/server.rs` 的 `handle_connection_with_stop` 处理完普通请求后返回并关闭 socket。因此每次普通 RPC 建立新 socket；没有请求失败自动重放。主文档同步修正了此前对可复用普通连接的假设。
- 订阅 ACK 的请求 ID 和 `subscription_started` 必须正确；确认后才开始引导快照。接收器支持 16 MiB 行上限，ctx/Close 打断阻塞读取。
- `session` 单 coordinator 拥有 epoch 和活动订阅。流程为生命周期订阅确认 → 首次快照 → 所有 pane 的批量代理状态订阅确认 → 补取快照 → Ready。pane 集合变化时先确认新订阅，再关闭旧订阅；无 pane 时不创建代理订阅。
- 事件只标 dirty；一次快照在途，期间的新信号保留。没有定时状态刷新。订阅丢失单独记录，不被 dirty 覆盖；旧订阅主动关闭不引起重连。未知事件记录诊断并重新同步。
- 断线保留完整旧视图并标 stale，以带抖动、最高 5 秒退避重新订阅。缺失 pane 的订阅竞态先补快照；布局引用暂时不一致补同步一次，持续不一致或格式错误进入 Incompatible 并等待显式 Retry。
- 快照拥有私有索引、权威顺序和复制查询方法。缺少关键数组/ID、重复身份、跨范围引用、无效矩形或旧 agent→terminal 映射会拒绝；未知状态原值保留。M1 保留布局内 pane focus，完整几何投影留给 M3。
- App 只维护本窗口选择：真正查看过的 workspace/tab 才形成选择记忆；未查看范围仍使用当前服务端焦点。远端 focus 不抢走有效本地选择；关闭当前标签时选择相邻项；路径只来自 checkout_path 或选中 pane 的 cwd。
- worktree 层级只使用明确 repo 元数据及唯一父归属证据；无法确认的保持独立根项。相同 agent 名称不会合并 pane。
- 原生界面使用 1200×800 默认窗口、820×560 最小尺寸、248px 侧栏、42px 双行项和 40px 内容头部。状态用不同形状、颜色、tooltip 和辅助名称表达。M1 主内容为真实 pane 摘要，终端画面从 M2 接入。
- 默认启动后台读取实际 CLI version/schema；初始化错误可见并支持 Retry。后台通知合并成一次待执行 Window.Update，清 pending 后复查最新 publication；窗口关闭后忽略晚到更新。CLI 检查与 Session 共享 5 秒退出预算。

## 验证证据

| 检查 | 结果 |
|---|---|
| `make test` | 全量测试和 go vet 通过 |
| `go test -race ./...` | 88 项测试通过，未报告数据竞争 |
| 传输回归 | 普通服务每请求主动关闭连接时，8 个并发调用仍通过单 lane 使用8条连接、8个唯一ID；错误 ACK 和取消阻塞读取按预期处理 |
| Session 回归 | 引导恰有两次快照，空闲不轮询；在途 dirty 不丢；EOF 保留快照后重连；代理订阅替换不误重连；pane 在握手时消失可恢复；坏快照不无限重试 |
| App 回归 | 记忆、相邻回退、未查看范围更新、非当前标签 scoped focus、相同名称代理、真实路径、显式 worktree 分组、复制容器及 stale 时间戳均通过 |
| 原生 UI 引擎 | 亮暗 1200×800 / 820×560 渲染、真实文本、顶部位置、Retry 分发与未知状态提示通过；已目视检查 headless 布局渲染，不能替代系统截图 |
| `make integration` | M0 原生窗口/direct attach 全套回归，以及 M1 隔离真实服务事件验证通过 |
| 真实事件 | 测试外部创建 workspace、改名、报告 agent working→blocked、关闭 workspace，Session 通过事件自动更新，全过程无手动 Refresh、无 epoch 误递增 |
| 实际用户服务 | 默认原生应用运行到 epoch=1、revision=2、Ready，随后自动关闭；没有获取 terminal owner 或执行修改 RPC |
| `make package` | CGO_ENABLED=0 构建本地签名 `dist/darwin-arm64/Herdr UI.app`，包含 libghostty-vt 与图标许可证 |

集成测试只修改自己在 `/tmp` 创建的独立服务。测试里的短时等待用于观察验收条件，不是产品轮询逻辑。

## 素材与边界

Lucide 1.60.0、Lobe 静态图标 1.98.0 均精确锁定。生成清单、源文件映射、生成 Go/SVG、包锁和许可证已提交；包锁使用公开 npm registry。普通 Go 构建无需 Node/npm。当前使用单色品牌变体，未知 agent 使用 bot。

M0 的中文 IME、鼠标/程序接管滚动、大粘贴和持续输出人工验证仍未全部通过，本阶段没有用状态窗口测试替代它们。原生材质、减少透明度、失焦细节、菜单、参考面板和完整辅助功能实机验收属于 M4；当前外壳使用不透明三 stop 渐变。本阶段没有实现终端 host、Execute、split/zoom/ratio 或业务 prefix，不应据此宣称 M2–M4 完成。
