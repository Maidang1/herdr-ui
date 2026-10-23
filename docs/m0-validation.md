# M0 验证记录

日期：2026-10-23。M0 实现和自动验证已落地；下方人工验收与 resolver 差异仍是进入后续阶段前的门槛，**M0 尚未全部验收通过**。

后续 M2 补充实测：插件 Close 发起挂断后，原生验证入口还等待公开 Done 结束信号；它不能证明服务端已释放 owner。隔离测试对此使用有界重新连接探针，不把它作为应用自动 takeover/retry 的实现。

## 固定基线

| 项目 | 实际基线 |
|---|---|
| 平台 | macOS / arm64 |
| Go | 1.27.1，由 go.mod 要求；本机默认工具链原为 1.26.5 |
| mygo | `v0.6.3-0.20261021150203-80afe0af6f9d`，完整 commit `80afe0af6f9d0608b05caed22f4607205a60c3a5` |
| mygo CLI | 与应用使用同一模块版本；该 commit 的 CLI 自报 `v0.6.2` |
| Herdr CLI / 服务端 | `0.9.5` / `0.9.5`，实测 ping protocol `22`；未将 22 写死为兼容条件 |
| Herdr 研究源码 | `7c34cf20ce72dc83eabb017910e989696f973acc`；不声称它就是安装二进制的构建提交 |
| 实际 CLI schema SHA-256 | `9e2af207e9aa8183d4aeca5fde9cc48e7909bb40cdbd7cf21608a6d3ea78075b` |
| libghostty-vt | mygo 插件清单内的 Ghostty `befcdfd2c3a1cb24d9ec886e93c95b2b5daa7028` 构建 |
| arm64 动态库 SHA-256 | `6f385d3221f8c776ba40530db9baf8c71478907897959849896eeb0ddbf16b41` |

## 已实现

- `cmd/herdr-ui`：1000×680 的原生插件验证窗口，最小 820×560。显式 Command 模式连接 terminal ID；本地输入 fixture 明确运行 `/bin/cat`。后台 New、UI 安装、关闭检查与独立资源所有权处理初始化/关闭交错；Close 使用共享 5 秒预算。不创建 shell 回退或额外 PTY。
- `cmd/m0-probe`：读取实际 CLI version/schema、ping、session.snapshot 和每个真实 tab 的 layout.export；显示 pane/terminal 映射、树路径和客户端载入的绑定来源。不发送业务修改或订阅冒充完整 Session。
- `internal/herdr`：显式 socket > HERDR_SOCKET_PATH > HERDR_SESSION > 默认目录；遵守 XDG_CONFIG_HOME。HERDR_CONFIG_PATH 只改变键位配置文件。attach 传同一解析结果，移除继承的 session/caller pane 身份；参数数组启动、不拼接 shell、不 takeover。
- 有限 NDJSON 读取：16 MiB 上限、请求 ID 校验、5 秒超时、context 取消打断 socket，无请求重放。M0 为每次 Call 创建短连接，M1 的持久请求 lane 与订阅尚未实现。
- `internal/layout`：导出树解析、基本结构/重复 pane 校验和 false/true 路径。没有用 snapshot split ID 猜路径。
- `internal/input`：默认绑定、常见修饰键/标点别名、数组/清空覆盖、用户优先、自定义命令占用和冲突/不安全直接绑定诊断；prefix 消费与 literal Ctrl-B 双层转义的纯计算契约。自定义命令只识别绑定，不执行或输出命令正文。

## 自动与运行验证

运行 `make test`、`make integration`、`make package` 可复现。集成测试需要真实 Herdr 和 libghostty-vt，使用 build tag `integration` 隔离，不混入纯 Go 测试。

| 能力 | 证据与边界 |
|---|---|
| CGO_ENABLED=0 | 原生窗口编译成功；无绝对路径 replace |
| 原生窗口启动/关闭 | 实际 App.Run、terminal.New/View、自动关闭成功；本地 fixture 日志显示网格从初始 80×24 更新为 113×37，随后 Close 完成 |
| 实际协议 | 当前用户服务只读 ping/snapshot/14 个 tab 的 layout.export 成功；真实配置中的四个自定义命令绑定可识别 |
| 协议契约 | 22 个 Go 测试通过，覆盖 >64 KiB、16 MiB 超限、截断、错误响应/ID、取消、socket 优先级、布局、绑定与 prefix 转义 |
| direct attach | 隔离服务与真实插件执行命令后收到专有输出；断言使用转义构造输出，避免误匹配命令回显。等待远端 prompt 后才发输入 |
| 单 owner | 第二个无 takeover attach 以非零退出，第一连接仍存活 |
| 尺寸 | headless 插件公开 Resize 到 103×31，服务端权威 snapshot 的 viewport_rows 变为 31。该 Resize 仅用于测试；窗口交由 View 管理尺寸 |
| 真实窗口 direct attach | 指定 `HERDR_UI_M0_WINDOW_BINARY` 的集成测试启动真实窗口，权威快照确认 View 驱动远端 viewport_rows 从 27 增至 >31，并验证进程自动退出及 terminal closed 日志 |
| 退出存活 | Close 本地实例后，使用相同 terminal ID 再 attach 并真实执行命令成功；没有关闭远端 pane/workspace |
| 双层 prefix | 原始远端测试程序收到字节 `02 71`，证明双 Ctrl-B 后接 q 不会意外退出 attach。这不等于系统 IME/OnKey 已验收 |
| 布局路径 | 在隔离服务创建 right→second/down 嵌套树，对 `[true]` 写 0.63，返回树中 second 改变、root 仍 0.5 |
| 打包 | 固定版本 CLI 生成本地 ad-hoc 签名 .app，资源包含与清单 SHA 匹配的动态库；实际打包进程启动终端并自动关闭成功 |

隔离测试仅停止自己创建的临时服务，不连接用户继承的 socket，不按进程名清理 Herdr。测试失败时终端正文诊断仅来自隔离测试 fixture；正常应用不记录终端正文、粘贴或剪贴板。

## 必须补验的边界

窗口控制工具返回 `Computer Use was not approved to use Herdr UI M0`，因此无法自动操作该应用。原生窗口运行日志能证明插件被创建和布局，但不能证明视觉首屏、IME 或鼠标正确。

- 中文输入法候选、预编辑、提交、组字期间 Ctrl-B、不重复动作字符与 release；切换焦点时取消 prefix。当前插件 OnKey 没有公开 composition 状态，本窗口暂未连接业务 prefix Machine，避免把未验证的钩子作为完整输入方案。
- 普通 wheel 的 OnScroll 与程序接管鼠标/alternate-scroll 两条路径；复制粘贴、OnClear 可见反馈。验证窗口的 Herdr wheel 仅记录行数并消费，尚未接 pane.scroll；本地 fixture 保留插件滚动和清屏。最终服务端滚动按 M2 实现。
- 持续输出、大粘贴、resize 和键盘交错的真实窗口响应性/内存；当前测试不提供这些性能结论。
- resolver 与目标 Herdr 的逐样例对照尚未完成。当前显式拒绝自定义 navigate/indexed overlay、remote_image_paste、hyper 修饰键及非 ctrl+letter prefix；冲突直接报错，不复刻 Herdr 的“禁用并继续”诊断恢复。不能宣称已移植完整 resolver 或服务端内存中的生效键位。
- amd64、正式 Developer ID 签名/公证、实际断网冷启动尚未测试；已确认资源存在和插件加载，不把它等同于发布验收全部通过。

人工运行 `make window`，切到中文输入法后输入“终端输入测试”，观察候选窗与光标位置，提交后检查不丢字/重复；测试复制粘贴、窗口 resize、焦点切换。连接专用测试终端时用 `-socket ... -terminal ...`，不要自动接管已有 owner。记录系统版本、输入法、具体操作和结果后再更新本文。
