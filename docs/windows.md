# Windows 安装、更新与发布

Claude Eden 的使用环境为 **Windows 10 主机 + 本地 Debian 13 客机**。在 Windows 安装并打开应用，选择存储位置、导入自己的代理后进入工作台。

## Windows 版本与功能范围

宿主是安装 Claude Eden 的电脑，客机是 Claude Eden 管理的虚拟机。各项条件分别适用于下表中的功能：

| 功能 | 宿主与运行条件 |
| --- | --- |
| 应用的安装、启动、更新和卸载 | Windows 10，使用与 CPU 架构匹配的安装包及 WebView2 运行时。当前 Windows 构建为 x64。 |
| 创建和运行本地 Debian 13 客机 | 上述 Windows 桌面系统，以及 Windows 虚拟机监控程序平台（WHPX）。 |

应用携带本机 Linux 虚拟机运行时。终端、浏览器、编辑器和 Claude Desktop 都在 Debian 13 环境内运行。客机使用方式见[Debian 13 客机](guide/guest-systems.md)。

系统版本与能力按以下依据判断：

- 宿主 SSH 终端使用 ConPTY。Microsoft 将 [CreatePseudoConsole](https://learn.microsoft.com/en-us/windows/console/createpseudoconsole)、[ResizePseudoConsole](https://learn.microsoft.com/en-us/windows/console/resizepseudoconsole) 和 [ClosePseudoConsole](https://learn.microsoft.com/en-us/windows/console/closepseudoconsole) 的最低客户端标为 Windows 10 1809；安装器与桌面入口检查这三个接口是否实际可用。
- 本机 Linux 客机检查 `WinHvPlatform.dll` 的 [WHvGetCapability](https://learn.microsoft.com/en-us/virtualization/api/hypervisor-platform/funcs/whvgetcapability)，确认监控程序正在运行且平台 API 可用。Microsoft 标注 x64 API 从 Windows 10 1803 提供；功能已勾选而系统尚未完成重启时，仍应以实际检查结果为准。
- 随包 QEMU 的 `docs/system/whpx.rst` 说明，x64 WHPX 从 Windows 10 2004 起经过上游测试，更早版本可能工作但未经其测试。接口可用性与这一上游测试范围共同构成运行依据；具体电脑的验证结果见相应发行记录。

## 安装前准备

下表适用于本地 Debian 13 环境。

| 项目 | 要求 |
|---|---|
| 系统 | Windows 10；当前 x64 构建用于 Intel／AMD x64 电脑，系统能力按实际运行时与虚拟化检查结果判断 |
| 虚拟化 | BIOS/UEFI 已启用处理器虚拟化，Windows 已启用“Windows 虚拟机监控程序平台” |
| 存储 | 使用本地磁盘，保留安装包、Linux 镜像、虚拟磁盘和更新备份所需空间；初始虚拟磁盘上限为 24 GiB |
| 代理 | 自己的 Clash/Mihomo YAML 来源或 HTTP/SOCKS5 节点；Claude Eden 不提供代理服务 |

在“启用或关闭 Windows 功能”中启用“Windows 虚拟机监控程序平台”，按系统要求重启。也可在管理员终端执行：

```powershell
DISM /Online /Enable-Feature /FeatureName:HypervisorPlatform /All /NoRestart
```

这个系统功能需要一次管理员授权，命令完成后按返回结果安排重启；Claude Eden 日常运行和应用安装使用普通用户权限。企业电脑若禁止虚拟化，应由设备管理员启用。Linux 客机使用独立的 QEMU 虚拟机，通过 WHPX 加速，不依赖 WSL 或 Docker。

## 安装并开始工作

1. 查看 [GitHub Releases](https://github.com/CarterLeeAlt/Claude-Eden/releases) 中实际发布的 Windows 安装包，核对对应发行条目的 `SHA256SUMS`。签名安装包名为 `Eden-<版本>-windows-amd64-setup.exe`；未签名开发包名为 `Eden-<版本>-windows-amd64-setup-unsigned.exe`，需手动安装和更新。尚无安装包时按下文从源码构建。
2. 运行安装程序，选择安装位置。首次安装默认选择本地 D 盘的 `D:\Eden`；没有本地 D 盘时使用 `%LOCALAPPDATA%\Programs\Eden`。安装器随附 .NET 桌面运行组件、WebView2 安装程序、QEMU、OpenSSH 客户端和代理核心，无需另装 Go、Node.js 或 Clash。
3. 在“快捷方式”页选择“添加桌面快捷方式”和“添加到开始菜单”。首次安装默认勾选两项，后续 Claude Eden 更新沿用上次选择。卸载会清理 Claude Eden 创建的快捷方式。
4. 从选中的快捷方式启动 Claude Eden，确认 Debian 13。如果 C 盘空间有限，选择其他磁盘的专用空文件夹，再导入代理并启动环境。程序准备官方镜像、工具和虚拟磁盘；已有环境可从设置迁移。见[选盘与迁移步骤](getting-started.md#选择环境存储位置)和[客机系统指南](guide/guest-systems.md)。

安装器不设置系统代理，不安装 SSH 服务，不把随附工具加入系统 PATH。WebView2 是 Microsoft 提供的系统浏览器运行时，其安装与维护由 Microsoft 安装程序负责。

安装器逐文件解压到安装位置所在磁盘的暂存目录，成功后替换应用，避免在系统临时目录生成整包解压副本。Windows 临时目录仍可能保存少量安装器辅助文件；缺少 WebView2 时，Microsoft 安装程序会管理其共享运行组件的位置。

“打开项目”接受 Windows 文件夹，例如 `C:\Users\姓名\Projects\demo` 在 Linux 内对应 `/c/Users/姓名/Projects/demo`。项目文件仍保存在原来的主机文件夹中，编辑和删除直接影响原文件；项目内的 `.git` 可读写，可在 Claude Eden 中暂存、提交和切换分支。项目共享只限明确打开的目录，下载使用独立的暂存目录。

Windows 快捷键以 `Ctrl` 为主：`Ctrl+O` 打开项目、`Ctrl+T` 新建网页、`Ctrl+L` 定位地址栏、`Ctrl+W` 关闭标签，`Ctrl+Shift+Enter` 新建终端。终端使用 `Ctrl+Shift+C/V` 复制粘贴，`Ctrl+C` 中断命令。网页和编辑器采用相应客机系统的按键行为。

网页输入框短暂失焦时，工作台会尝试恢复同一标签和窗格的键盘输入。恢复期间的点击、按键、切换标签、打开设置或切换窗口会取消这次自动恢复，保留用户选择的焦点。如果网页仍不接收按键，可重新点击输入框；尚未确认的输入法组合文字会在失焦时取消。

窗口仅保留系统标题栏，工作台直接占据下方区域。文件、项目和设置操作使用工作台入口与快捷键；检查应用更新和版本信息可从托盘图标的右键菜单打开。窗口首次打开时按当前屏幕的可用区域居中，并适配 Windows 显示缩放。手动调整后的窗口尺寸和最大化状态会保存；换屏或降低分辨率时，窗口保持在当前屏幕内。后台启动与版本检测不显示命令行窗口。

从开始菜单打开 Claude Eden 时，后台服务静默启动。关闭工作台窗口会保留环境与后台任务，可从托盘图标重新打开。要退出桌面程序，在托盘菜单选择“退出 Claude Eden”。本机环境仍在运行时，可以选择保留任务或停止环境；停止环境会结束其中的终端。

浏览器上传通过本机文件选择窗口选取文件；下载完成可用 Windows“另存为”选择位置。侧栏“下载”提供打开文件、在资源管理器中显示与再次另存为，取消保存窗口时保留默认 Downloads 中的文件。操作与限制见[浏览器上传与下载](getting-started.md#浏览器上传与下载)。

## 更新与卸载

应用内更新从当前仓库的公开 GitHub Releases 来源选择 Windows x64 安装器。检查与下载经过已经应用并运行的 Claude Eden 代理，不回退主机直连。下载安装器后校验 SHA-256、版本、产品名称和 Authenticode 签名；当前应用与新安装器必须具有相同的发布者公钥，未签名开发包不能自动安装更新。

没有新版时隐藏更新按钮；有新版时显示金色圆形按钮。用户先保存文件并结束运行中的任务，再点击一次，按钮显示环形下载进度和百分比。校验通过后自动安装，不再二次确认。更新器先备份应用，再通过工作台统一入口停止环境、代理和控制服务，等待当前窗口退出后执行静默安装。新应用完成工作台启动确认后才认定成功；安装或启动失败时尝试恢复旧程序，保留失败原因和必要备份。磁盘空间不足、文件被其他软件占用等情况可能阻止自动恢复，应按错误信息处理。

手动更新前也应保存文件、停止环境并退出 Claude Eden，再运行新版安装器。原后台仍运行时安装器会拒绝覆盖。客机工具在“设置 → 环境与恢复”中单独更新，替换应用会保留原系统与数据。

卸载前停止环境并退出，在 Windows“设置 → 应用”中卸载 Claude Eden。默认配置目录为 `%USERPROFILE%\.eden`，`EDEN_CONFIG` 可显式选择配置；打包应用的默认环境存储目录为安装目录内的 `EdenData`，另选的存储位置继续保留。自行打开的项目仍在原目录。WebView2 可能由其他程序共用，不随 Claude Eden 自动卸载。需要另行删除数据时先备份，删除虚拟磁盘会丢失只保存在客机中的文件与登录状态。

## 常见启动问题

Windows Hypervisor Platform 已启用时，QEMU 仍可能因嵌套虚拟化初始化失败而退出，错误码为 `80370302`。当前开发构建在 Windows x64 的默认 WHPX 参数中使用 `kernel-irqchip=off`，由 QEMU 实现中断控制器，保留 WHPX 的 CPU 加速。正常桌面入口直接使用该参数，无需外部 PowerShell 兼容脚本；显式设置的 QEMU machine 参数仍优先。此设置改变中断控制器实现，具体性能取决于硬件及虚拟化能力。启动失败时，“操作详情”显示经过脱敏的 QEMU stderr 摘要，完整脱敏诊断包可从设置导出。

当前开发构建在更新或强制结束后会检查连接记录及控制锁。锁空闲时核对其所有者和私有权限，再由后台重建连接记录；真实后台持有的锁继续阻止接管。权限问题会显示具体原因。请保留个人配置目录，不要通过删除账户或虚拟磁盘解决连接问题。点右上角 × 隐藏窗口并保留托盘，结束应用请使用“退出 Claude Eden”。

Claude Eden 支持含中文的安装目录。启动时优先读取随附 QEMU 的 UEFI 固件，无需在用户目录另存副本。在默认虚拟 CPU 配置下，会向 Windows 虚拟机监控程序平台查询可用指令；Claude Code 需要客机提供 SSE4.2。自行指定了虚拟 CPU 型号的环境仍沿用该设置。

- 提示虚拟化不可用：检查 BIOS/UEFI 的虚拟化开关及 Windows 虚拟机监控程序平台，重启后再试。远程或嵌套虚拟机还需要外层宿主允许嵌套虚拟化。
- 提示缺少 WebView2：重新运行完整安装程序。不要从第三方站点下载同名 DLL。
- 工作台能打开，环境不能联网：检查 Claude Eden 内是否已选择节点及网络检查结果。Windows 浏览器能联网不表示工作台节点已配置。
- 提示权限或路径不可用：使用当前用户有权限访问的本地目录，不把 Claude Eden 安装目录、私有配置或磁盘根目录作为共享项目。更新器拒绝含符号链接或目录联接的安装路径。

反馈问题时提供 Claude Eden 版本、Windows 版本与内部版本号、CPU 架构、失败步骤和错误原文。不要附带代理密码、订阅链接、会话地址或未脱敏的配置文件。

## 静默安装选项

静默安装可传入 `/DESKTOPSHORTCUT=0|1` 和 `/STARTMENUSHORTCUT=0|1`，参数放在最后的 `/D=` 安装目录之前。未提供参数的静默更新沿用已保存的快捷方式选择。

## 从源码构建

构建入口为 [`scripts/build-windows.ps1`](../scripts/build-windows.ps1)，安装包入口为 [`scripts/package-windows.ps1`](../scripts/package-windows.ps1)。版本与依赖以 `VERSION`、`go.mod`、`packaging/windows/dependencies.json` 和锁文件为准。构建需要 PowerShell 7、Go、Node.js/npm、.NET SDK 和 7-Zip，生成安装器还需要 NSIS；签名构建需要相应签名工具及授权。完整构建会执行真实桌面验收，宜在独立测试账户中运行。构建产物与本机已安装应用使用不同目录。

编译 `cmd/eden` 前，Windows 构建会生成 Linux amd64／arm64 的 `workbench-tailnet` 客机资源，并准备 Linux guest-agent。Lima 模板由 `scripts/prepare-lima-templates.mjs` 复制 Debian 13 的入口、镜像定义和共享挂载定义，保留上游镜像版本与摘要。

构建与普通回归统一从仓库根目录运行，先将工作区工具链放入当前进程的 `PATH`：

```powershell
node scripts/run-in-workspace.mjs -- pwsh -NoProfile -File scripts/build-windows.ps1
node scripts/run-in-workspace.mjs -- pwsh -NoProfile -File scripts/package-windows.ps1
node scripts/run-in-workspace.mjs -- go test ./internal/clipboardfiles
node scripts/run-in-workspace.mjs --cwd tools/tailnet -- go test ./...
node scripts/run-in-workspace.mjs -- node --test scripts/run-in-workspace.test.mjs
```

统一入口将用户目录替身、Go telemetry、工具缓存和临时文件映射到仓库 `.tmp/`，复用 `.tmp/go/cache`、`.tmp/go/pkg/mod`、`.tmp/nuget/packages` 和 `.tmp/npm-cache`。`TEMP`、`GOTMPDIR` 直接使用短路径 `.tmp`，保留 Unix socket 长度限制。创建前检查已有祖先的真实位置，越界链接和越界 `--cwd` 会报错；Go 测试先执行 `scripts/check-test-paths` 静态门禁。PowerShell 构建和测试入口校验同一环境契约，直接绕过入口会失败。

显式继承的 `EDEN_CONFIG`、`AGENT_VM_STATE_DIR`、`AGENT_VM_LIMA_HOME`、`EDEN_LOGICAL_LIMA_HOME`、`LIMA_HOME`、`AGENT_VM_BIN_DIR` 也须位于工作区；越界时报告变量名并停止。未设置的 `EDEN_CONFIG` 保持未设置，继续使用隔离后的用户目录默认值，保留 CLI 的模式选择。`.NET` 单文件解压使用 `.tmp/dotnet-bundle`。

参数直接传给子进程，Windows 的 `.cmd`／`.bat` 不经 shell 重解析；npm 可使用 `node scripts/run-in-workspace.mjs -- node <工作区Node目录>/node_modules/npm/bin/npm-cli.js ...`。入口继承工具链 `PATH`，不安装全局依赖。它约束环境和已知落点，不是操作系统安全沙箱，固定路径、原生系统 API 和提权操作仍需单独审查。

`scripts/ci-windows-build.ps1 -SystemIntegration` 是一次性 Windows CI 的系统集成入口，会执行真实安装、卸载，发布模式还会操作签名证书；它与普通回归分开。跨卷验收只在明确提供 `EDEN_TEST_STORAGE_VOLUME` 时运行，入口核对绝对路径、卷、已有祖先和目录归属。Windows CI 显式使用 `C:\Eden-CI-Storage\<run_id>-<run_attempt>`，执行后清理本次创建的目录；普通运行不会自动在系统盘根选择目录。
