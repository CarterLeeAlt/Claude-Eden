# Claude Eden 开发与验收交接

本文是换机后的接手入口。当前源码提交在 `dev` 分支，不推送远端；本机收尾时会另行生成 Git bundle、源码包和关键验收证据包。接手时先阅读根目录 `AGENTS.md`，再核对 `git status`、`git log -5` 和本文件。

## 当前目标与边界

- 产品：Windows 10 宿主运行本地 Debian 13 客机。
- 宿主品牌：Claude Eden；程序和配置使用 `eden`、`EDEN_*`、`~/.eden`。
- 客机使用中性命名 `workbench`、`WORKBENCH_*`、`WORKBENCH_*` 环境变量。
- 只面向新装，不增加旧品牌迁移兼容层。
- 支持语言：简体中文、繁体中文、英语、日语、韩语、德语、法语；工作台中文，安装器中英。
- 宿主输入与验收范围：中文、英文键盘和输入法，覆盖切换、组合输入、提交、取消和焦点恢复。
- 客机公网 TCP 和 DNS 必须经过已应用的代理；代理故障时客机不得直连。

## 验收环境决定

后续使用另一台**专用实体 Windows 10 电脑**作为测试宿主机，不要求独立 Windows 测试 VM。该测试机必须可重装或可恢复基线，不能使用日常电脑；测试账户、配置、测试盘和证据放在工作区内，测试前后核对系统状态。

“不用独立 Windows 测试 VM”不等于跳过虚拟化：产品自身的 Debian 13 客机仍必须实际启动，继续验证 QEMU、共享目录、显示输入、Tailscale、代理正常链路和代理故障关闭。

## 代码结构

| 范围 | 位置 |
|---|---|
| 桌面入口 | `cmd/eden-desktop` |
| Windows 原生层 | `internal/desktop` |
| 更新事务 | `internal/desktopupdate` |
| 安装事务 | `internal/installfiles`、`cmd/eden-install` |
| React／严格 TypeScript | `ui/src/app`、`ui/src/features`、`ui/src/components`、`ui/src/lib`、`ui/src/styles` |
| Wails 来源与补丁 | `patches/wails`、`third_party/wails` |
| 安装器 | `packaging/windows` |
| 隔离运行入口 | `scripts/run-in-workspace.mjs` |

旧 C# Windows 桌面实现已从源码树删除：`desktop/windows`、`desktop/windows.tests`、`desktop/windows.update-tests`。不要恢复旧入口或旧兼容层。

## 已完成的代码和构建工作

- Wails 3.1.0、React、严格 TypeScript 和 Go 迁移代码已进入当前工作树。
- 官方 Wails 绑定已生成并核对，当前绑定为 27 Methods / 17 Models。
- 安装和更新事务已迁移到 Go，旧 C# 调用入口和引用已清理。
- Windows 候选已构建：
  - 应用：`_output/wails-candidate-cdp/_output/windows-amd64/app`
  - 安装器：`_output/releases-wails-migration-cdp/Eden-0.1.8-rc-windows-amd64-setup-unsigned.exe`
  - 安装器 SHA-256：`c2001fece3579282fc7fbceb6f748714131f5e80b7ce8867e5a392190bd057d4`
- 新候选归档检查已完成，应用文件数为 3508。

## 已验证结果

### Windows 安装矩阵

`package-20261117-cdp-26` 使用上述确切安装器和真实交互式 `edentest` 会话完成 16/16：

- 证据目录：`.tmp/wails-migration/package-acceptance/package-20261117-cdp-26/`
- 结果：`vm-results.json`
- 最终审计：`vm-final-audit.json`
- 安装状态：`vm-install-status.json`
- 任务回执：`vm-install-launcher.json`
- 16 个场景全部 `passed=true`
- `registrationRemains=false`
- `desktopRemains=false`
- `startMenuRemains=false`
- `remainingOwnedProcesses=0`
- `jobHandleClosed=true`
- `taskExitCode=0`
- `certificateDifference=[]`
- `taskDifference=[]`
- `inputsUnchanged=true`

### 失败根因和测试 harness 修正

早期 `package-20261117-cdp-22` 失败时，Windows 事件日志记录了虚拟内存耗尽：一个 Windows PowerShell 进程约占用 12.5 GB，随后出现“无法创建新的堆栈防护页面”和 `Stack overflow.`。

根因是验收 launcher 每 200 ms 调用一次 `Get-ScheduledTask`；受控 100 次调用使 PowerShell 工作集增加约 40 MB。后续验收 harness 将任务状态检查与每分钟状态读取合并，`package-26` 以修正版 harness 完成 16/16。该 harness 位于本机 `.tmp/`，不属于产品源码；换机后若重建验收驱动，应保留这个根因修正。

## 尚未完成

- Windows update 21 项事务尚未取得通过证据。
- `update-20261117-cdp-24` 为旧快照；`update-20261117-cdp-25-upload-failed` 保留了上传前置失败证据；最新 update 尝试因远端 preflight 脚本未上传而未进入矩阵。
- 实体 Windows 10 测试机上的最终宿主验收尚未开始。
- Debian 13 五套环境、真实代理故障关闭、剪贴板完整保存／回读／恢复、双屏同缩放和最终包整链路需要在新测试机继续完成。
- 真实外部 Claude／GitHub 登录不属于本轮验收。

不要把 `package-26` 的通过结果扩展为 update 或完整产品交付通过；交接时必须保留上述未完成项。

## 新电脑接续顺序

1. 从交接包恢复 Git bundle 或源码包，确认当前分支为 `dev`。
2. 阅读 `AGENTS.md`、本文件和关键证据索引；运行 `git status`、`git log -5`。
3. 在专用实体 Windows 10 测试机建立可恢复基线、专用测试账户和工作区工具链。
4. 用工作区隔离入口重建 Go、Node、Python、PowerShell、Wails 和 NSIS 工具链，不向全局环境安装依赖。
5. 重新构建或核对同一候选，先完成 21 项 update 事务。
6. 再按专用实体宿主机范围完成安装、更新、卸载、重启、Debian 13 客机、代理故障关闭、输入、显示和资料保持验收。
7. 所有系统集成测试串行执行；结束后核对进程、任务、注册表、快捷方式、证书、测试盘和代理状态。

## 关键交接文件

- `AGENTS.md`
- `docs/windows.md`
- `docs/guide/network.md`
- `docs/handover.md`
- `packaging/windows/README.txt`
- `scripts/run-in-workspace.mjs`
- `.tmp/wails-migration/package-acceptance/package-20261117-cdp-26/vm-final-audit.json`
- `.tmp/wails-migration/package-acceptance/package-20261117-cdp-26/vm-results.json`

`.tmp/` 和 `_output/` 默认被 Git 忽略，单独 clone 不能还原本机工具链、测试盘和验收证据。交接包只携带选定的候选文件、关键证据和 Git bundle；不要把完整测试盘、用户配置替身、密码或完整缓存复制到新电脑。

## Git 收尾约定

- 本次换机提交保留在 `dev`，不推送。
- 新电脑继续开发前先核对当前提交和工作树。
- update 完成、实体 Windows 10 宿主验收和最终审计通过后，再决定是否另行将 `dev` 收束为 `main`；本次换机不自动完成该分支变更。
