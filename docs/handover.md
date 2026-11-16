# Claude Eden 开发与验收交接

**主要源码改造已经落地，大量分项验证通过，整体交付验收尚未完成。** 本轮已收尾，收尾时相关应用、VM、浏览器与监控进程均已退出。源码以包含本文件的开发交接提交为准，前一分批提交基点为 `a9b2b2f`；交接提交包含产品改造、性能候选和测试修正，待验状态见下文。下一统一交付候选为 `upstream-terminal`，它的生产前端、源码快照、应用和安装包尚未统一生成。

本文件是接手入口，包含目标与决策、完成状态、代码与构建定位、遗留问题、运行条件和接续顺序。详细过程见 [测试过程与证据索引](../.tmp/guest-neutralization/README.md)。本文相对路径均以仓库根为基准；Markdown 链接按文件位置解析。本机工作区为 `D:\OpenCode\ClaudeEden`。

## 任务目标与已定边界

目标是完成本产品适用的上游同步、品牌与功能精简、可靠性修复和低资源性能优化，并用同一个确切候选完成构建、安装及产品验收。

- **产品范围：** Windows 10 宿主＋本地 Debian 13；面向用户使用完整名称 Claude Eden，仓库为 `https://github.com/CarterLeeAlt/Claude-Eden.git`。程序使用 `eden`／`EDEN_*`，客机使用中性命名 `workbench`／`WORKBENCH_*`；仅面向新装。
- **上游核对范围：** 同步基点 `e8d02e7c8932434887e3587b248ae0b11556a04c`，目标 `5939ec478bd571c925a5b180f1bd074a75d27b6c`；只纳入 Windows、本地 Debian 13 和保留功能实际依赖的变化。
- **网络：** Mihomo，仅手动 HTTP CONNECT／SOCKS5／SSH 单节点；保留客机 Tailscale。客机公网 TCP 与 DNS 必须走已选择并应用的代理，故障时关闭公网，强制约束在宿主层。完整规则见 [AGENTS.md](../AGENTS.md) 和 [网络说明](guide/network.md)。
- **语言：** 简中、繁中、英语、日语、韩语、德语、法语；工作台中文，安装器中英。按出口地区推荐支持语言，没有对应建议时保留已选语言。
- **外观与终端：** 软件仅亮色；界面内置 Noto Sans SC＋Inter、默认 14 px；终端内置 Noto Sans Mono CJK SC＋JetBrains Mono、默认 13 px并保留字号调整。终端明确采用 DOM，使用 xterm 默认色并保留 OSC 控制。系统标题栏和系统文件选择窗口可用系统字体。
- **启动与资料：** 等待真实就绪、明确失败或用户取消，固定总时长不能单独成为生产启动失败或杀进程的理由。新建默认／随机／手动配置由续装与维护复用；既有身份、账户、HOME、磁盘、文件和登录资料保持。
- **性能采用门槛：** 用户允许采用实际复验能支持的改进，收益与功能正确性须共同通过。画质、资源占用、架构和文件一致性的新增取舍仍交用户决定。比较固定 4 vCPU／4 GiB和实际硬件虚拟化；产品手动资源配置只选择上限，继续动态回收。
- **协作与授权：** 关键改动可按已授权独立范围整理提交；推送、发布另行授权。真实 VM、安装及网络故障测试串行。`EDEN-WINTEST` 安装矩阵、受控 SSH 验收和 Linux 回归 VM 的精确临时 AppArmor 规则已有授权，使用前核对原设施和落点。

## 已完成工作与代码状态

### 分批提交基点

| 提交 | 内容 |
|---|---|
| `924b3c0` | Windows SFTP 短目标名及 POSIX 请求路径修复 |
| `c165310` | 不可用进程身份分支的确定性测试 |
| `16b502c` | Windows 桌面启动等待真实就绪与显式取消 |
| `0de5372` | 终端中文标点列宽修正 |
| `6e73abe` | Windows 安装共享／锁冲突有界重试 |
| `67b68fe` | 自动内存上限、压力增长与空闲回收策略 |
| `a9b2b2f` | 内置字体、统一亮色与终端配色隔离 |

这些提交及开发交接提交均不代表最终统一产品已验收。开发交接提交包含下表所列的后续源码和测试；接手时先看 `git status`、提交记录及差异，核对有无交接后的改动。

### 已有验证结论

| 工作块 | 已验证结果 | 适用范围 |
|---|---|---|
| 字体与外观 | DOM 列宽 480 组最大偏差 0.141 px，四档 DPI 字体加载、复制／换行／启动、OSC 配色、五套 Chromium UI、原生字体／提示框和 NSIS 安装前页面通过 | 字体提交及对应分项输入；最终统一包待验 |
| Windows 回归 | services 根模块 1352 pass／0 fail／125 skip；related 429 pass／0 fail／115 skip；vet 通过 | `.tmp/upstream-sync/windows-go-services/` 对应源码 |
| Linux 回归 | services 运行时 37 套通过；根模块 1569 pass／0 fail／19 skip | related 仍失败，详见下文；AppArmor 与 VM 收尾通过 |
| 代理与官网 | 本机真实 Mihomo 协议／认证／故障恢复检查、官网 22 页及七语言链接、14 个桌面／窄窗组合通过 | 不能代替真实客机代理故障关闭矩阵 |
| 文件参数传递 | `ui/src/file-transfer.test.mjs` 在真实 Debian／Node 24.22.3 中 4 过、0 失败、0 跳过 | 补齐 Windows 上缺 `bash` 的那项平台验证 |
| 输入与显示 | 五项真实客机输入检查、四项显示检查、四轮性能对比、300 秒 RFB 暂停恢复通过 | 冻结性能候选不含最新隐藏输入取消修正 |
| 隐藏输入取消 | 原失败复现，修正后的同路径和完整 Chromium 输入回归通过 | 原生 Windows 输入与新版客机实机链路待验 |

### 交接源码定位

| 范围 | 关键位置与当前含义 |
|---|---|
| 品牌、创建配置、设备与语言 | `internal/guestprofile/`、`internal/workbench/guest_profile*`／`device_check*`、`shared/guestidentity/`、`ui/src/`、`www/`、README 与 `docs/` |
| 代理精简和失败关闭 | `internal/managedproxy/`、`internal/eden/egress.go`／`gate*`、`internal/workbench/proxy*`、代理 UI 和网络文档 |
| 启动、认证、生命周期 | `internal/eden/guest-*`／`browser-*`／`claude-desktop-*`、`internal/vm/`、`internal/workbench/runtime.go`／`lifecycle.go`／`projects.go`，及 Lima／sshocker 对应补丁 |
| 显示连接与输入优化 | `internal/workbench/vnc*`／`desktop_visibility.go`、`internal/eden/native-ime.py`、`ui/src/views.jsx`／`remote-input*`、noVNC 源码与生成补丁 |
| 最新待验测试修正 | 三个第三方测试文件及 `sshocker/0016`、`sftp/0007` 补丁，详见 Linux 章节 |

重要实现点：编辑器 `--editor` 就绪检查只在需要认证端点时读取浏览器凭据；IME 用 F24 的 keypress 确认探测，经 GLib idle 提前结束，保留 45 ms 无响应期限和 context／generation 校验，7 项单测通过。上一代 keyup 串代问题已有复现与修正。浏览器停止阶段拒绝新 CDP 工作，以 `Browser.close` 正常关闭；宿主先封闭入口、取消工作及撤销联网许可，VM 成功停止后释放项目 SFTP，客机日志最后停止。

## 构建与证据基线

| 对象 | 位置 | 当前状态 |
|---|---|---|
| services 源码快照 | `.tmp/upstream-sync/services-source/` | 已冻结，支撑对应回归和应用构建 |
| 最近完整应用 | `_output/windows-upstream-services/app/` | 构建和完整包本机初始化通过 |
| 最近已验安装包 | `_output/releases-upstream-runtime/` | 对应 Windows 安装矩阵通过，产品矩阵未全部通过 |
| 冻结性能候选 | `.tmp/upstream-sync/performance/builds/press-acknowledged-input/` | 私有固定内存后端＋冻结前端，复用 services 原生外壳／运行时；不含最新隐藏输入取消修正 |
| 下一统一候选 | `terminal-source`／`upstream-terminal` | 尚未冻结、统一构建和验收 |

关键摘要用于核对受测输入：

```text
services_snapshot  ce58f201eafda5906cbedbe1d1a63ae0092c2dbedc9ed233a82d2caeffb92e68
services_archive   5a52c88cfe0999c58516b978b4b718b7b15bc454dd4f0dc18edd08999603c7db
services_backend   407f4821f956d01550db69728fbc02e461019f046211e550b6450ba576a339d6
performance_backend 8f072904b47a6a35a0861ad53951f644b116662797a8f71bd67c9f785d722eff
```

各冻结标签保持原字节。源码、受测前端、后端和安装包应按各自清单关联；既有快照通过记录只适用于当时输入。原始失败、截图和诊断脚本在 `.tmp/`，该目录被 Git 忽略；跨机器交接时需另行携带需要续验的证据、工具链和测试盘，单独拉取仓库不能还原这些本机设施。

## 接手顺序

1. 先读根 `AGENTS.md`、本文件及对应问题证据；核对 HEAD、工作树差异和各环境 `case.json`／`active.json`，确认没有他人正在操作同一设施。
2. 核对工作区工具链、实际写入位置和宿主严格快照。当前处于收尾状态，接续执行安排须明确后再启动长任务。
3. 按下列顺序补齐验收；新失败保留原始输入与证据，修正后走原路径复验。

- [ ] 完成隐藏输入取消修正的实机验证，核查点击延迟回退，确定可采用的性能改进。
- [ ] 完成 Linux related 失败修正的运行验收，定位仍可能存在的 token 耗时问题。
- [ ] 对稳定源码运行相关完整回归，生成唯一的 `upstream-terminal` 候选。
- [ ] 用该确切候选完成 Windows 安装、五套产品流程、七语言及代理故障关闭矩阵。
- [ ] 汇总可交付结果，按独立范围整理提交；推送和发布需要单独授权。

### 工具链与执行入口

普通命令从仓库根经 `scripts/run-in-workspace.mjs` 运行，子目录用其 `--cwd`。现有环境入口 `.tmp/guest-neutralization/environment.ps1 -Toolchain tailnet` 使用工作区 Go 1.27.3、Node 24.22.3、Python、.NET 等，并重定向用户目录、APPDATA、LOCALAPPDATA、XDG、缓存和临时目录；Go 驱动另外明确设置 GOROOT。SSH 使用 Git 自带 OpenSSH。

| 用途 | 入口与前置条件 |
|---|---|
| 宿主审计 | `.tmp/guest-neutralization/windows-go/host-audit.mjs check`；严格基线见收尾章节 |
| 源码输入回归 | 先在 `ui` 中执行 `scripts/prepare-vnc.mjs`，再执行 `.tmp/upstream-sync/run-ui.mjs --source remote-input-smoke.mjs`；均经工作区入口，已实测 |
| 性能候选 | `.tmp/upstream-sync/performance/build-benchmark.ps1 -Label <新标签>`；运行时设置 `EDEN_PERFORMANCE_BUILD` 后执行同目录 `run.mjs`，可附 `--verify-input --verify-display --verify-files` |
| Windows 完整回归 | `.tmp/upstream-sync/run-go.ps1 -Scope upstream-terminal`；含 Go 路径门禁、监控、输入与宿主核对，terminal 批次尚未执行 |
| Linux 完整回归 | `.tmp/upstream-sync/run-linux.mjs --terminal-candidate`；须先有对应快照，复用 `.tmp/guest-neutralization/linux-regression/qemu-tcg/`，新增分支尚未实际运行 |
| 源码冻结与构建 | `.tmp/upstream-sync/capture-build-source.py --output terminal-source`；随后 `build.ps1 -Stage app`／`installer`，均指定 `-BuildName upstream-terminal`，冻结前先生成生产 web |
| 产品矩阵 | `.tmp/upstream-sync/run-product-matrix.mjs --runtime-recheck`；设置 `EDEN_ACCEPTANCE_BUILD_NAME=upstream-terminal` 并核对原盘状态，确切包完成后使用 |

上述入口用于定位接续工作，执行前仍须满足其前置条件。普通回归不修改真实宿主配置；系统集成测试按已授权设施单独安排。性能测试私有 Go overlay 将内存目标固定为上限，生产构建继续使用动态资源策略。

### 原盘与手动环境

环境记录在 `.tmp/guest-neutralization/product-linux/<环境名>/`。`debian-13-local`、`debian-13-stop`、`debian-default`、`debian-random` 均已有盘；默认和随机环境已有取消／续装及保存标记，须复用。性能基准使用 `debian-13-local`，实际配置目录为 `.tmp/product-debian13local/`。

`debian-manual` 尚未创建，计划配置目录为 `.tmp/product-debianmanual/`，记录目录为 `product-linux/debian-manual/`。已定手动参数：主机名 `M`，账户 `alex`／`web`，HOME `/home/projects`／`/home/browsing`，音频 `desktop_audio`／`Desktop (USB)`，扩展名称 `Tab Layout`，图形为 SwiftShader。准备入口已带这些选择，创建前核对仍无既有实例。账户口令和登录资料留在原受控配置中，不复制进交接文档。

### 会话协作

主会话为 `ses_e1fdc1abeffeLF2HwXHAUJcpcb`；代理／字体／第三方测试协作会话为 `ses_e1e114aa2ffe4lazyVFO6ismOP`，其文件和索引已交回，收尾时无在途委派。Sub2API 资源协调会话为 `ses_e1cb3f1e0ffeuEMkZ0iBzuaQln`，用户已允许发送证据及协调无并行编译窗口；独占窗口已释放，未来测量重新约定。真实客机网络矩阵须在主会话完成统一包和环境基础验收后，明确实例与 `active.json` 再串行交接。

## 性能候选与点击延迟

**现状：视图切换和输入状态查询有重复测量支持的收益，完整正确性门槛仍有缺口。**

四轮按基线→候选→候选→基线运行。固定 4 vCPU／4 GiB，实际 WHPX，相同八份字体字节、1043×733 画面、DPR 1；驱动摘要相同，客机无换页和 balloon 调整，采样未发现外部 Go 编译。四轮的功能检查、正常退出和宿主审计均通过。

| 指标 | 基线 P50 / P95 | 候选 P50 / P95 | 有效样本数（每方案） |
|---|---|---|---|
| 点击到新画面 | 35.6 / 50.7 ms | 38.9 / 57.3 ms | 44 |
| 输入状态查询 | 53.5 / 90.9 ms | 11.5 / 23.2 ms | 24 |
| 返回视图到隐藏期间改变的新画面 | 171.6 / 206.9 ms | 87.4 / 124.0 ms | 24 |

**问题：点击指标没有改善，合并 P50 增加约 3.3 ms、P95 增加约 6.6 ms。** 候选两轮点击 P95 分别为 57.5 和 39.7 ms，基线为 36.8 和 51.9 ms；尾延迟存在轮次波动，不能只挑较快轮次。尚未定位稳定差异与波动的来源。

- [ ] 在完成隐藏输入修正后，冻结新标签，保留现有构建原字节。
- [ ] 复核点击路径、前后台调度和帧到达时序，判断该差异是否由改动引入。
- [ ] 用一致夹具复验新候选；收益与正确性共同通过后，才确认采用。

终端已确定采用 DOM；WebGL 原型只作为用户选型证据。

证据目录位于 `.tmp/upstream-sync/performance/`：

- `fixed-baseline-2026-11-16T08-27-34-165Z/`
- `press-acknowledged-input-2026-11-16T08-29-43-142Z/`
- `press-acknowledged-input-2026-11-16T08-33-48-448Z/`
- `fixed-baseline-2026-11-16T08-35-53-320Z/`
- 汇总核对脚本：`compare-abba.mjs`。

300 秒 RFB 暂停恢复检查也通过：隐藏期间新增帧为 0，30 次资源观察中 Documents 恒为 2、Nodes 恒为 376，JS 堆约 7.1–10.2 MB。证据为 `visibility-soak-20261116-083804/`；它使用真实浏览器和 RFB 线协议夹具，不能替代长期真实客机稳定性验收。

## 隐藏后的迟到输入

**问题与根因：** 保留 RFB 连接后，隐藏视图没有使旧输入队列失效。隐藏前等待 Fence（服务器处理确认）的输入，在迟到回执到达后仍会继续执行；仅暂停画面请求不足以取消输入。

**当前修正：** 输入队列增加代次，隐藏时拒绝旧代次的未完成 Fence 和粘贴，异步剪贴板解码结果也核对代次；随后允许新代次的组合输入清理和按键释放。涉及：

- `internal/eden/novnc-patch.py`
- `ui/vendor/novnc/core/rfb.js`、`ui/vendor/novnc/NOTICE.eden`
- `ui/src/views.jsx`
- `ui/scripts/remote-input-smoke.mjs`

原失败已经复现；修正后的同路径检查及真实 Chromium 输入回归通过，覆盖隐藏后返回、迟到回执、已确认但尚未执行的输入、异步文本／图片／文件、粘贴数据／确认／按键阶段、清理与恢复。回归使用实际 noVNC 方法及报文编码，远端回执由夹具控制。

**剩余缺口：** 当前冻结的 `press-acknowledged-input` 不含此修正；新修正尚未进入生产包，也未完成实际 WebView2→Windows 原生输入→客机的完整复验。

- [ ] 验证真实浏览器与 Claude Desktop 的隐藏、返回、隐藏期间断线及重连。
- [ ] 在真实客机上核对隐藏时组合输入取消、延迟粘贴取消、按键释放、恢复后输入及已提交文字保持。
- [ ] 运行最新源码的相关完整前端回归，随后核对最终包原入口。

本机复现入口：`.tmp/upstream-sync/performance/check-hidden-input.mjs`；完整输入回归入口：`ui/scripts/remote-input-smoke.mjs`。源码回归需要项目的 Vite 模块解析，不能用普通静态服务器替代。最近通过使用 Vite 8.3.5／Chromium 159.0.8154.6；实际产品 WebView2 仍待验证。

## Linux related 回归失败

原批次 `linux-services-resume/` 的运行时 37 套及根模块已通过；related 为 490 pass／44 fail／71 skip。44 个失败键含两个父测试，独立叶级失败为 39 个 sshocker 权限、一个 SFTP 权限、一个 StatVFS 和一个 token 耗时。

### 权限和 StatVFS

**已定位：** 测试入口 `umask 077` 将创建权限过滤，公共夹具期待的 `0644` 实际为 `0600`，另外两个创建权限测试同样受影响。StatVFS 测试比较两次不同时间的系统采样，可用块数相差两块；原 ID 断言还存在类型不同导致的无效比较。

协作会话已交回三个测试文件的修正：

- `third_party/sshocker/pkg/reversesshfs/rooted_test.go`：公共文件及重命名前后两个 inode 设定确定的初始权限；创建权限测试在独立 Linux 子进程内使用 `umask 022`，保留原 `0755` 断言。
- `third_party/sftp/server_test.go`：独立子进程使用 `umask 022`，保留两次 OPEN 的 `0745`／`0755` 请求和精确 `0745` 结果；路径使用 `t.TempDir()`，第二次操作后重新 Stat，传递 allocator 参数。
- `third_party/sftp/request-server_test.go`：记录同次真实 StatVFS 查询的值副本，经原 socket 协议链路比较全部统计字段和独立预期 ID。

对应补丁为 `patches/sshocker/0016-deterministic-permission-tests.patch`、`patches/sftp/0007-deterministic-filesystem-tests.patch`。静态正反应用、字节核对和差异检查通过；**Go 格式、编译与运行验收尚未完成**。

- [ ] 经工作区隔离入口运行 Go 格式检查、路径静态门禁和 Windows 适用回归；格式变化须同步补丁。
- [ ] 在真正 Linux 的原 `umask 077` 入口复验全部权限叶级及原 Fatal 之后的保护断言。
- [ ] 验证 SFTP allocator=false／true、StatVFS 全字段及错误分支。
- [ ] 核对子进程正常、失败、短父 deadline、取消、含空格可执行路径、退出后无残留及父 umask 保持。
- [ ] 完成相关完整回归。生产权限实现没有从这些日志中得到需要修改的证据。

### token 耗时

`TestExpectRemoveMany` 的 200000 次操作耗时 **11.689925 秒**，超过原 5 秒门限，后续过期清空断言未执行。该路径主要是词法路径处理、时钟、锁、队列和 map；现有证据不足以归因于某个函数，也不能仅因回归设施使用 TCG 就销项。

- [ ] 保留 200000 次和 5 秒原断言，在同源码 Linux 批次复验。
- [ ] 若再次超时，采 CPU profile 后定位根因；同时核对受支持 Windows 宿主对应路径。

`.tmp/upstream-sync/run-linux.mjs --terminal-candidate`、`linux-regression.py` 的条件采样入口已准备，尚未运行验收；TCG 数值不能作为 WHPX 产品性能结论。

## 共享文件访问瓶颈

**实测：** 2048 文件、32 目录、6258688 字节，SSHFS 与客机 ext4 的计数和内容摘要完全一致。独占轮次的三次测量中位数：

| 操作 | SSHFS 共享目录 | 客机 ext4 |
|---|---|---|
| 元数据刷新后扫描 | 2831 ms | 18.1 ms |
| 暖扫描 | 186.9 ms | 15.3 ms |
| 元数据刷新后逐文件读取 | 9321 ms | 68.3 ms |
| 暖逐文件读取 | 9129 ms | 61.3 ms |

这里只刷新未使用的元数据，保留文件内容页缓存，不能称为磁盘冷读。当前设置和新装默认均为 `project_file_cache=true`，启用缓存不是本轮新增优化。

- [ ] 定位逐文件 OPEN／READ／CLOSE、协议往返、宿主路径检查及客机缓存各自的开销，形成根因证据。
- [ ] 核对实际 Git／编辑器工作负载及双向文件一致性，再决定改进。
- [ ] 若方案改变现有 20 秒元数据缓存时效、资源占用或共享架构，先给用户说明收益与取舍。

当前没有交付新的共享文件优化。两批宿主测试数据已逐字节核对并移入回收站；成功执行的客机脚本已清理私有复制目录，原始结果保留。

## 统一构建和完整产品验收

- [ ] 收束上述修改后重建生产 web，冻结 `terminal-source`，构建 `upstream-terminal` 应用和安装器。性能测试用的固定内存 Go overlay 只属于私有基准构建。
- [ ] 对确切输入完成 Windows 根模块／相关模块／vet、Linux 运行时／根模块／相关模块回归。已有 `services-source` 结果不能自动覆盖后续源码。
- [ ] 完成包清单、摘要、资源、品牌名称、七语言文档和实际宿主最低条件的一致性检查。
- [ ] 在 `EDEN-WINTEST` 完成确切安装包的新装、覆盖升级、短暂／持续占用、访问拒绝、快捷方式及卸载资料保留验收。
- [ ] 复用 `debian-13-local`、`debian-13-stop`、`debian-default`、`debian-random` 四套原盘，保持身份、账户、HOME、磁盘、文件及登录资料。
- [ ] 只创建一次尚未创建的 `debian-manual`，按已定手动配置验证创建、取消、续装、维护和身份保持。
- [ ] 完成五套环境的启动／取消／真实失败、编辑器认证和有效会话保持、共享文件、组件更新、Desktop、停止及重启全流程。

最新 `windows-upstream-services` 应用通过构建和本机初始化；既有 `releases-upstream-runtime` 安装包的 Windows 安装矩阵通过。两者均不代表当前最终候选已完成验收。

## 原产品问题的最终包复验

- [ ] **重复输入／侧栏消失：** 旧包真实操作中出现过，部分追踪轮次正常，尚未取得最终包沿原触发路径的完整消除证据。保留真实 Quick Open、输入保存、焦点和按键追踪，不以单次正常代替结论。
- [ ] **重启后打开项目返回 409：** 已定位健康采样占用生命周期操作锁，源码修正和定向回归通过；仍须在最终包、原随机环境的重启后项目打开路径复验。
- [ ] **服务启停：** editor 凭据读取、显示就绪、浏览器保活与停止顺序、关闸及 VM 退出前保持共享已有分项证据；须在同一最终包完成慢启动、取消、失败传播、正常退出和资料保持。

## 网络、语言与外部业务

- [ ] 按 [.github/CONTRIBUTING.md](../.github/CONTRIBUTING.md) 的代理故障关闭验收，完成真实客机 HTTP CONNECT／SOCKS5／SSH、TCP／DNS、认证和解析失败、核心退出、上游不可达、配置失效、暂停／切换及恢复矩阵，提供宿主侧无直连证据。本机 Mihomo 检查通过不能替代此项。
- [ ] 核对首次安装、更新及恢复过程也满足“代理故障时客机绝不直连”。网络故障注入与其他 VM／系统交互测试串行。
- [ ] 用最终包完成简中、繁中、英语、日语、韩语、德语、法语的客机和 Chromium 验收，并核对无地区建议时保留所选语言。
- [ ] 完成真实 Claude／GitHub 登录、多账号外部业务和 Tailscale 私网对端验证；这些项目需要相应用户登录授权和实际对端条件。

## 收尾与恢复条件

- 本轮应用、VM、浏览器测试及监控均已结束；性能作业和 300 秒检查的所属进程收尾记录通过。
- Sub2API 两次宿主 Go telemetry 写入已归因，对方确认补齐工作区隔离；独占窗口已释放。原宿主文件保持，50／55 文件历史基线和失败记录保留，当前严格审计快照为 `.tmp/upstream-sync/external-go-quiescent-baseline.json`，最近检查为 30 个目标／56 个文件无变化。
- 验收驱动的整体 `passed` 在功能、收尾和宿主审计全部通过后设置，成功与各类失败分支已检查。早期 `08-05-36` 结果中的 `passed=true` 只代表功能阶段，其命令因宿主审计失败退出，不能当作整轮通过。
- 恢复性能测量前重新协调无并行编译窗口并检查宿主快照；遇到新增变化先归因，不以覆盖基线消除失败。
- 交接源码包含多批改造及待验修正；后续提交按实际范围和受测输入整理，避免整文件还原或覆盖其他工作。
