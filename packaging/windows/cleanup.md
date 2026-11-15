# Claude Eden 资源定位与彻底卸载清单

版本：`{{VERSION}}`。包类型：{{PACKAGE_KIND}}。

本清单适用于 Windows 10 宿主运行本地 Debian 13 客户机。它列出程序、运行数据及可选组件的实际资源位置，不扫描电脑，也不自动删除文件。正常卸载保留个人数据；彻底移除数据需要单独确认范围。

源码依据为 [Claude Eden 仓库](https://github.com/CarterLeeAlt/Claude-Eden) 的相对路径，应查阅与本包版本对应的源码。客户机路径在 Debian 内部解析，Windows 路径在宿主解析。

## 先建立实际路径与归属表

| 标记 | 如何确定 |
| --- | --- |
| `I` | 本文件所在的实际程序目录。核对 `Eden.exe` 路径与卸载登记 `InstallLocation`。首次默认固定 D 盘的 `D:\Eden`，否则为 `%LOCALAPPDATA%\Programs\Eden`；显式选择和更新位置以实际记录为准。 |
| `C`、`P` | `C` 为配置文件，`P` 为其父目录。读取启动参数与 `EDEN_CONFIG`；默认 `%USERPROFILE%\.eden\config.json`，自定义文件名和位置以实际启动记录为准。 |
| `S` | 环境存储物理根。读取 `P\environment-storage.json` 的 `directory`、`id`、`backups` 和未完成迁移记录。打包入口可默认使用 `I\EdenData`。 |
| `T` | 文件传输根。读取 `P\environment.json` 的 `directory`、`identity_directory`；默认是 `P` 的路径后加 `-downloads`。默认 `.eden` 内的嵌套 profile 映射到 `%USERPROFILE%\.eden-downloads\<相对profile路径>`。 |
| 用户目录、临时目录 | 按运行实例的 Windows 用户、进程环境和 Known Folder 实际值确定。桌面、开始菜单、`TEMP`、`TMP` 和 `.NET` 解压位置都可能重定向。 |

先保存工作，导出需要保留的恢复密钥、项目、下载和客机资料。读取配置、存储回执、实例身份及所属进程，形成准确路径表；凭据不贴入公开日志。

停止环境并正常退出，确认本实例 QEMU、SSH、Mihomo、后台与 WebView2 已结束。不要把隐藏窗口或保留后台当作停止，也不要仅按进程名结束其他软件的进程。保留路径表后再正常卸载，最后处理获准的个人数据和迁移备份。

## 程序、安装登记与快捷方式

| 位置／标识 | 内容与处理边界 |
| --- | --- |
| `I\Eden.exe`、`I\resources\`、附录文件 | 桌面、后台、随包 QEMU/OpenSSH/Mihomo、许可和 WebView2 安装器。按实际哈希清单识别；修改文件单独核对。 |
| `I\Eden.install-id` | 安装标识，内容为 `io.github.carterleealt.claude-eden`。 |
| `I\Eden.cleanup.md`、`I\Eden.install-files.json` | 本清单和包文件哈希。JSON 不哈希自身，Markdown 按原字节纳入 JSON。安装助手把实际生成的 `Uninstall.exe` 哈希加入安装后的 JSON。 |
| `I\Uninstall.exe` | 安装器生成的正常卸载入口。 |
| `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\ClaudeEden`（64 位视图） | 本用户卸载登记。核对 `InstallLocation`、`UninstallString`、版本、图标及快捷方式选项，确认属于当前安装。 |
| 当前用户桌面 `Claude Eden.lnk` | 通过 Known Folder 定位，核对目标 `I\Eden.exe`。 |
| 当前用户开始菜单 `Programs\Claude Eden\` | 主程序和卸载快捷方式；核对每个链接目标。 |
| `Global\Eden.Installation` | 安装期间的命名互斥对象，句柄释放后消失，不是磁盘文件。 |

哈希清单只证明程序文件归属，不授权删除整个安装目录。`I\EdenData`、用户放入的项目、链接和保留副本可在正常卸载后继续存在。

依据：`packaging/windows/Eden.nsi`、`installation-files.ps1`、`desktop/windows/Backend.cs`。

## 安装、更新与恢复事务

| 位置／来源 | 内容与核对方法 |
| --- | --- |
| `<I>.eden-installation.json` | 程序目录旁的事务回执，记录安装、stage、backup 和保留项。先完成恢复，不先删回执再覆盖安装。 |
| `I` 同父目录的 stage／backup | NSIS 动态分配，以回执为准；中断、空间不足、文件占用可能留下旧程序和个人文件。 |
| `I\Eden-preserved-files\<ID>\`、`late-<ID>\` | 与程序冲突的修改文件、无法确认归属的库及安装期间的额外改动。可能含用户原件。 |
| `/DIAGNOSTICS=<文件>`、`<文件>.helper.json` | 显式指定的诊断输出，可能在程序目录之外；从操作记录定位。 |
| `P\updates\` | 下载目录、更新助手 `helper-<UUID>.exe`、`launch-<UUID>.json`、`last-result.json` 和 `previous-<确认ID>` 回滚副本。先核对进程与事务状态。 |
| 系统临时目录的 NSIS 辅助目录 | `$PLUGINSDIR`、`ns*.tmp`、`~nsu*.tmp\Un.exe` 等由 NSIS 分配。按产品、签名、进程、时间和回执核对，不能按同名前缀批量清理。 |

依据：`desktop/windows/{AppUpdater.cs,UpdateInstaller.cs,UpdateRecovery.cs}`、`internal/appupdate/update.go`。

## 每个 profile 的配置与工作台数据

以下相对于实际 `P`。桌面连接本机后台使用的 `P\desktop-entry` 是独立入口 profile，也应核对；同一电脑可有多个显式配置文件。

| 路径 | 内容与边界 |
| --- | --- |
| 配置文件 `C` | 代理、环境和系统设置。 |
| `account.json`、`password-vault.json` | 本地账户、恢复／解锁资料和加密密码库。 |
| `workbench.json`、`appearance.json`、`favorites.json`、`projects.json` | 工作台、外观、收藏、分组与共享绑定；项目索引不是项目本体。 |
| `claude-accounts.json`、`cli-sessions\` | Claude 账号与本地 CLI 会话索引；客机内凭据和会话历史另在虚拟盘中。 |
| `controller-instance.json`、`desktop-entry\` | 本机后台实例身份和桌面入口资料。 |
| `control.json`、`control.lock`、`control-port.json` | 后台身份、锁和监听端口；先确认 PID、路径和句柄退役。 |
| `stopped-jobs\<jobID>` | 已停止作业标记。结合 `EDEN_JOB_ID`、实例 `eden-running.json.job_id` 核对，待任务及分离式 hostagent 退出后处理。 |
| `initialization.json`、`network-intent.json`、`last-safety-report.json`、`resource-limits.json`、`previous-profile.json` | 初始化、网络许可意图、检查结果、资源设置及配置恢复副本。 |
| `proxy\` | 自有 Mihomo 的来源、配置、进程记录及数据库，含用户订阅和节点凭据；不是其他代理软件的缓存。 |
| `webview2\`、`window-placement.json` | 本桌面 WebView2 用户数据和窗口位置，区别于 Microsoft 共享运行时。 |
| `logs\` | 当前实例诊断日志及轮换文件。 |
| `browser-downloads\` | 浏览器下载状态与索引；实际另存文件由记录定位。 |
| `environment.json`、`environment-storage.json`、`environment-storage-pending.json`、`environment-storage.lock` | 传输根、存储位置和迁移事务。 |
| `guest-install.json`、`guest-install-media-<InstallationID>.json` | Debian 安装绑定与已校验介质回执；ID、摘要、路径和磁盘须相互吻合。 |
| 原子写入临时文件 | `.storage-config-*`、`.eden-write-*`、`.eden-private-probe-*` 等，按所属写入程序和句柄核对。 |

只有确认目录专属、全部 profile 均在授权范围且没有混放个人文件时，才可整目录处理。外部资料不会因被软件读取而成为其自有缓存。

## 存储、镜像与下载

| 位置／记录 | 内容与边界 |
| --- | --- |
| `P\state` | 实际目录，或指向 `S\state` 的 junction／符号链接。分别核对链接与目标，不能沿链接误删其他数据。 |
| `S\.eden-storage-id`、`S\state\` | 存储身份、Lima/QEMU 实例和虚拟盘；ID 应与 profile 一致。 |
| `S\cache\lima` | Debian 镜像下载缓存。 |
| `state\lima\<实例名>\` | `lima.yaml`、磁盘、cidata、cloud-config、SSH 配置、PID、日志、socket、`eden-running.json`、`eden-gate.json`、`eden-resources.json`；磁盘含个人资料。 |
| `state\lima\_config\{user,user.pub}`、`state\file-mounts\<实例名>`、`.agent-vm-*` 标记 | 此状态根的管理 SSH 身份和文件挂载；不能由此清理用户独立安装的 Lima。 |
| `state\share\lima\` | 本后台提取的客机助手与 Debian 13 三文件模板闭包；以实际 `Share.Dir` 为准。 |
| `state\guest-media\` | 绑定的 `host-system.yaml`、`host-system-source.json`、镜像、`.partial`、锁和校验失败保留副本。 |
| `state-backup-<ID>`、`.storage-link-<ID>`、迁移记录的 `backups`／`Next` | 旧存储和事务恢复内容，按准确记录逐个核对。 |
| `T`、`T\Downloads` | 用户下载与传输根，可能在任意显式位置；删除需要独立确认。 |
| `AGENT_VM_STATE_DIR`、`AGENT_VM_LIMA_HOME`、`EDEN_LOGICAL_LIMA_HOME`、`LIMA_HOME`、`AGENT_VM_BIN_DIR` | 显式启动可改变状态和工具落点；按进程实际环境与真实路径核验归属。 |

共享项目中的 `.eden\workbench-mount-probe-*`、`.eden-share-probe-*`、`.eden-mount-probe-*`、`.workbench-mount-probe-*` 是探针候选，须与操作记录匹配。用户项目、Git 仓库、导出诊断和另存文件保持原样。

依据：`internal/eden/{storage.go,environment.go,job_stop.go}`、`internal/hostimage/`、`internal/state/state.go`、`third_party/lima/pkg/limatype/filenames/filenames.go`。

## Windows 用户目录外的缓存与临时文件

| 位置 | 核对规则 |
| --- | --- |
| `%LOCALAPPDATA%\workbench\clipboard-ownership\` | 文件剪贴板所有权记录及锁；确认相关 profile 停止后再核对孤立项。 |
| `%TEMP%\workbench-clipboard-*\` | 剪贴板副本及 `.workbench-clipboard-owner-*` 标记；按所有权记录处理，保留其他程序的剪贴板。 |
| `%LOCALAPPDATA%\Eden\trusted-local\<64位ID>.json` | 本机记住登录凭据。系统 DPAPI 主密钥及其他软件凭据不属于本程序。 |
| 实际临时根下 `eden-gate-*`、`eden-proxy-<profileHash>`、`eden-vm-assets-*` | 网络门禁、代理和嵌入资源临时项，按 profile、进程、时间和句柄确认。 |
| 目标旁 `.eden-download-*.part`、`.eden-agent-response-*`、`.eden-diagnostics-*.zip` | 下载、Agent 响应或诊断临时项；完成的用户导出单独保留。 |
| `%TEMP%\.net\<应用名>\<bundle标识>\` | `.NET` 单文件解压缓存。`DOTNET_BUNDLE_EXTRACT_BASE_DIR` 可覆盖；只核对此应用和更新助手子树，不能清空共享 `.net` 根。 |

源码工作区 `.tmp/`、`_output/` 和工具链缓存不是普通安装的运行数据。清理开发环境需另定范围，不能顺带删除用户全局 Go、npm、NuGet 或 Python 环境。

## Debian 13 客户机内部

删除已确认的完整私有虚拟盘可覆盖其内部文件，但不会删除宿主共享项目、另存下载和迁移备份。若保留 Debian 系统、仅移除内部组件，应按包登记、专属文件和修改记录处理。

### 账户、服务与系统配置

| 位置／对象 | 内容与边界 |
| --- | --- |
| `developer`、`/home/developer` | cloud-init 创建的开发和 SSH 管理账户，UID 1000；核对实际主组、sudo 配置与目录归属。 |
| `browser`、`/home/browser`、`workbench-audio` | 独立浏览器普通账户与音频组；实际 UID/GID 由客机分配，不能与开发账户合并。 |
| `workbench-{editor,display,window-manager,browser,display-governor,webvnc}.service` | 位于 `/etc/systemd/system/`，按服务依赖反向停止，核验进程退出。 |
| `workbench-audio.service`、`/etc/workbench/audio.pa`、`/etc/pulse/client.conf.d/99-workbench.conf` | 音频配置；运行目录 `/run/workbench-audio/{native,runtime,state}`，sink 为 `workbench_output`。 |
| `workbench-memory.service`、`/usr/local/bin/workbench-memory-start` | zram 配置；从 `/run/workbench-zram-device` 确定真实设备。停止 unit 不等于释放 swap，须按设备核验再处理。 |
| `workbench-claude-desktop.service` | 按需使用的 Claude Desktop，核对 X 会话、keyring、文件代理和输入依赖。 |
| transient `workbench-maintenance-operation.service`、`workbench-desktop-operation.service` | 以 developer 执行维护；取消后核验包管理进程和整个 cgroup 退出。 |
| `/etc/systemd/journald@workbench.conf.d/limits.conf`、`systemd-journald@workbench.service` | `LogNamespace=workbench` 日志，按真实 machine-id 定位，只处理该命名空间。 |
| `/etc/workbench/` | 环境、`components.json`、浏览器控制、显示比例、openbox、编辑器、下载和音频配置。 |
| `/etc/profile.d/workbench.sh`、`/etc/zsh/zshenv` 中 source 行 | 产品环境与 BROWSER，保留用户其余设置。 |
| `/etc/sudoers.d/workbench-{editor,input,download,claude-desktop,tailnet}` | 特定操作权限，逐个核对内容。 |
| `/etc/fonts/conf.d/71-english-cjk-fallback.conf` | 专属字体回退配置。 |
| `/etc/localtime`、`/etc/timezone`、`/etc/locale.gen`、默认 locale | 按出口或用户选择修改；保留系统时按原备份或用户目标恢复，不删除系统文件。 |
| `/etc/apt/mirrors/{debian,debian-security}.list` | 管理的 Debian 镜像源列表，用户自定义列表保持。 |

### 程序资产与依赖

| 位置／资源 | 内容与边界 |
| --- | --- |
| `/usr/local/bin/workbench-*`、`client_runtime.py` | 编辑器、浏览器启动、账户、显示、输入、文件与剪贴板入口；以安装脚本的实际文件集合确认，不能仅按通配前缀删除。 |
| `/usr/local/lib/workbench-claude-bin/claude`、`workbench-claude-{session,open,start}` | 账号 shim 和可选 Desktop 入口，区别于官方 Claude 二进制。 |
| `/usr/local/lib/` 内浏览器、下载、输入、显示与 `tailnet-client.mjs` | 按 `internal/eden` 的安装脚本核对具体模块及相邻 Python 字节码。 |
| `/usr/local/lib/workbench-split/` | 内置浏览器扩展 manifest、HTML、JS。 |
| `/usr/local/share/workbench-webvnc`、`/usr/share/novnc` | 前者指向 Debian noVNC；后者为包管理文件，包含产品 patch 和 `workbench-clipboard.js`，保留系统时经包版本恢复。 |
| `/etc/chromium/policies/managed/workbench-password-vault.json` | 禁用内建密码管理的指定策略文件。 |
| `/usr/local/bin/sshfs`、Chrome 兼容入口 | 核对实际包装内容和链接目标，保留既有其他 Chrome。 |
| `/etc/apt/sources.list.d/claude-desktop.list`、`/usr/share/keyrings/claude-desktop-archive-keyring.asc` | 可选 Desktop 软件源和经指纹校验的签名钥。 |
| NodeSource 配置 | `/etc/apt/keyrings/nodesource.asc`、`/etc/apt/sources.list.d/nodesource.list`、`/etc/apt/preferences.d/nodejs`。 |
| setup 系统修改 | `/etc/needrestart/conf.d/no-prompt.conf`；代理环境分支通过 `mktemp -t agent-vm-proxy.XXXXXX` 创建临时 sudoers，目标 `/etc/sudoers.d/10-agent-vm-proxy`。按实际 `TMPDIR` 和操作记录定位。 |

systemd、OpenSSH、FUSE/sshfs、Git、Python、zsh、Node.js、Chromium、TigerVNC、openbox、noVNC、字体、输入法和音频等依赖由包管理器管理。软件使用某包不等于独占该包；保留系统时按包记录判断。

### 用户资料、工具与运行时

| 位置／对象 | 内容与边界 |
| --- | --- |
| developer `~/.config/workbench/` | 编辑器／VNC 密码、工作目录、Desktop 安装锁和配置，不能只当缓存。 |
| browser `/home/browser/profile/`、`Downloads/` | Cookie、存储、会话、缓存和下载；同级 `workbench-state.json`、`homepage.json`、`download-state.json` 为用户资料。 |
| browser `extensions/{packages,staging,registry.json}`、`.workbench-uploads` | 用户扩展和上传文件。 |
| browser `~/.vnc/passwd`、`~/.cache/workbench-display/`、`~/.cache/workbench-input/` | VNC、显示、输入和 socket 状态。 |
| developer `~/.local/share/workbench/` | 附件、Desktop 下载、CLI 通道及会话交接资料，逐项按实际索引核对。 |
| `~/.claude/`、`~/.claude.json` | 默认账号凭据、技能、项目和历史，可能早于本软件。 |
| `~/.local/share/workbench/claude-accounts/<ID>/`、`claude-shared/` | 独立账号和共享内容／备份。先识别链接，不沿链接删除原件。 |
| developer `~/.config/Claude` | 可选 Claude Desktop 用户资料；gnome-keyring 属共享秘密服务，不能清空整个 keyring。 |
| code-server 用户目录 | `~/.local/share/code-server/`、`~/.config/code-server`，含扩展、设置和工作区资料。 |
| mise／Claude npm | `~/.local/bin/mise`、`.mise-new-*`、`~/.local/lib/node_modules/@anthropic-ai/claude-code` 及同 prefix 的 launcher/native 包；`~/.npmrc` 的 prefix 指向 `~/.local`，仅处理对应配置项。 |
| 工具缓存 | `~/.cache/workbench/downloads/<校验值>/` 的锁、deb/tgz、`.part`、`.mirror.part` 和分块状态；`~/.npm`、`~/.cache/mise`。 |
| `~/.cache-maintenance-recovery/<batch>/receipt.json` | 缓存清理恢复回执、锁及移入材料，按源／目标核对。 |
| shell 配置 | `~/.zshenv`、`~/.zshrc`、`~/.zsh_history`、`~/.bashrc` 中产品段及 `.agent-vm.env` source；默认 shell 为 zsh，保留用户其余内容。 |
| `/run/workbench-desktop/` | Xauthority、D-Bus、VNC、keyring-ready、缩放和文件 socket，先停对应 Desktop。 |
| X 显示 `:77`、`:78` 与 `/tmp/.X11-unix/X<display>` | 按实际显示和所属 X 服务核对，不清空其他会话。 |
| 共享项目／`T` 的 SSHFS mount | 路径来自授权。先停受限 SFTP/SSHFS，确认 mount 消失再处理空挂载点，不能递归删除挂载内容。 |
| 维护目录、`apt.conf` 与 SSH 反向转发 | 临时代理凭据由 `WORKBENCH_APT_CONFIG` 指定；SSH `-R` 接收于客机回环 `18961`，宿主随机端口。取消后核验维护进程与转发退出。 |

依据：`internal/eden/` 的当前安装／服务脚本、`internal/workbench/{maintenance.go,cli_channel.go}`、`agent-vm.setup.sh`。

## 可选客机 Tailscale

本组件使用客机 developer 的非特权 tsnet 节点；宿主独立安装的 Tailscale、VPN 适配器和其他节点不属于它。

| 资源 | 定位与清理 |
| --- | --- |
| `/usr/local/bin/workbench-tailnet`、`/etc/sudoers.d/workbench-tailnet` | 首次使用时安装；下载临时文件由 `mktemp -t workbench-tailnet.XXXXXX` 在实际 `TMPDIR` 下创建，发布暂存位于目标目录。 |
| `~/.config/workbench-tailnet/` | `settings.json`、`control.json`、`node.lock`、`ssh_host_ed25519_key`、`ssh_config`、`browser-return*` 和 tsnet `Server.Dir` 内部状态；包含节点身份。 |
| `~/.ssh/config` 的 Include | 仅核对精确的节点目录引用，保留其他密钥和 SSH 配置。 |
| `/usr/local/share/applications/tailnet-login.desktop` | browser 的 Tailscale URL 回调，按该用户 MIME 关联核对。 |
| 节点与转发 | 回环控制端口、tailnet SSH 和服务转发；按用户意愿 logout 后关闭进程。本地删状态不证明控制端登记已撤销。 |

节点设置 `CorpDNS=false`、`RouteAll=false`，不接收子网路由，不替换正常客机 DNS。依据：`tools/tailnet/`、`internal/workbench/tailnet_install.go`。

## 停止与彻底清理顺序

1. 保存项目、下载、恢复密钥、客机资料、存储与更新回执，确定实际 `I/P/S/T` 及全部相关 profile。
2. 从当前桌面正常停止环境并退出。确认维护任务、终端、编辑器、浏览器、Desktop、Tailscale 转发与本实例宿主进程退出。
3. 运行正常卸载，保留未完成事务回执和恢复副本；先完成事务再清理残留。
4. 按获准范围处理账户和用户数据、专属虚拟盘、缓存、存储迁移备份。共享项目和另存文件须独立批准，不能由删除磁盘推定它们已被清理。
5. 若保留 Debian 系统，仅停用和移除经确认专属的组件；区域设置、shell、MIME、noVNC patch 和 npm prefix 按原备份或用户指定目标恢复。并非所有修改都保存了安装前值，不能承诺自动恢复。
6. 核对锁、链接、挂载、监听及 Tailscale 控制端登记；记录明确保留项、未发现项和因权限／占用无法核验的位置。

Microsoft WebView2 Runtime、EdgeUpdate、Windows `.NET`／PowerShell、Windows 虚拟机监控程序平台、用户 SSH／Git／代理客户端及签名证书属于共享系统资源，不能随本程序批量卸载。

Windows 事件日志、安装与崩溃报告、WER／CrashDumps、Prefetch、最近使用、Defender、USN／审计、备份与回收站可能记录活动。它们不是本软件独占资源，本清单不授权清除这些历史，也不保证系统审计痕迹归零。
