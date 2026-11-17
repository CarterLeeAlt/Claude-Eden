<div align="center">

<h1>Claude Eden</h1>

**把 Claude 大小姐关起来，为了更好地保护我们的相遇**

<img src="assets/claude-eden-character.png" alt="Claude Eden" width="320" />

[![Release](https://img.shields.io/github/v/release/CarterLeeAlt/Claude-Eden?include_prereleases)](https://github.com/CarterLeeAlt/Claude-Eden/releases) [![CI](https://github.com/CarterLeeAlt/Claude-Eden/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/CarterLeeAlt/Claude-Eden/actions/workflows/ci.yml) [![MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

**[项目主页](https://github.com/CarterLeeAlt/Claude-Eden) · [发行版本](https://github.com/CarterLeeAlt/Claude-Eden/releases) · [使用文档](docs/guide/index.md) · [常见问题](docs/guide/troubleshooting.md)**

</div>

## 受够了 Anthropic

作为一个只是想和 Claude 对话与合作的正常用户，受够了折腾各种 VPS，指纹浏览器，环境检测，面对封号的担惊受怕和财产损失。我们提出 **Claude Eden** 项目：**一个桌面工作台，配置好自己的网络代理，集中管理隔离环境、时区语言和环境检查。**

Claude Eden 在 Windows 10 桌面上管理独立的本地 Debian 13 客机。终端、浏览器、编辑器和官方 Claude Desktop 汇集在同一个工作台，项目按需共享，网络独立管理。宿主条件、资源和存储路径按实际电脑检测。支持以下几个重点功能：

- 和主机隔离的网络设置，Claude Eden 自己接管和代理内部的所有网络流量
- 和主机隔离的指纹设置，包括时区、语言等，支持自动与代理 IP 同步
- 针对 Claude 使用过程中的全链路适配（浏览器登录，授权，claude code 进程）
- 内置网络与隔离检查，关键检查失败或证据不完整时暂停外网；社区和第三方检测提供补充参考

## 你可以用它做什么

- **在一个窗口里完成工作。** 用终端运行 Claude Code，用编辑器查看改动，用 Workbench 浏览器预览服务和完成登录。
- **使用独立的 Linux 开发环境。** 在本地 Debian 13 中使用 Linux 工具链，共享明确授权的项目目录；系统、工具和登录资料保存在独立环境中。 [Debian 13 客机](docs/guide/guest-systems.md)
- **控制共享范围。** 只共享默认工作区和明确打开的项目文件夹；主机的 Claude 登录、浏览器配置和 SSH 密钥不会自动导入。
- **看清网络状态。** 手动管理 HTTP CONNECT、SOCKS5、SSH 节点，选择单个出口并完成检查。关键检查未通过时，环境保持离线；代理故障时客机绝不直连。
- **下次继续使用。** 工具、文件和登录状态保存在虚拟磁盘里，关闭窗口可以保留后台任务。
- **管理多个 Claude Code 订阅。** 查看额度、为新会话分配账号，并在明确授权后切换账号接续历史。 [Claude Code](docs/guide/accounts.md)
- **通过私网连接设备。** 在 Claude Eden 内登录 Tailscale，查看网络中的设备，使用 SSH、Web 与其他 TCP 服务。 [Tailscale 私网](docs/guide/tailscale.md)

浏览器收藏、无痕会话、Cookie、扩展、密码保险箱，以及外观、资源、存储和工具更新，见[完整使用文档](docs/guide/index.md)。

客机语言支持简体中文、繁体中文、英语、日语、韩语、德语和法语；按出口地区推荐支持的语言，没有对应建议时保留已选语言。工作台界面与详细使用文档使用中文。

## 快速开始

把这句话发给你的 agent（codex，dsh，workbuddy，反正不会是 claude code😁）

```markdown
帮我拉取 https://github.com/CarterLeeAlt/Claude-Eden 的最新源码，在 Windows 上打包安装，并将 Claude Eden 的代理配置为通过我本机 Clash 的 xx 节点使用我的 xx IP 出口。先读取仓库构建说明，核对安装范围和网络配置，再执行并验证。
```

你唯一需要的，是一个靠谱的代理节点，本项目不做任何代理推广，可以自行搜索黑色错误网站解决。

## 下载与安装

Claude Eden 的桌面宿主为 Windows 10，当前安装包为 x64，客机固定为本地 Debian 13。系统与虚拟化能力、资源和路径按实际电脑检测。 [Windows](docs/windows.md) · [Debian 13 客机](docs/guide/guest-systems.md)。

1. 当前候选的实测范围见[当前版本状态](docs/current-status.md)；正式下载以 [GitHub Releases](https://github.com/CarterLeeAlt/Claude-Eden/releases) 的实际发布内容为准，没有发布包时按[构建说明](docs/windows.md)构建。
2. 便携版解压后直接运行 **`ClaudeEden.exe`**，保留同目录 `portable.flag` 和 `resources/`，不需要安装器或 CMD 启动器，也不要求新建 ClaudeEden 本地账户密码。
3. 按应用内引导配置自己的代理，启动环境后打开项目。

便携包随附工作台和代理核心，无需先安装 Go、Node.js、Docker 或 Clash。配置、缓存、日志、下载和客机磁盘保存在 `data/`；首次创建环境时准备 Debian 13 官方镜像和开发工具。换位置前停止环境并退出，再移动整个程序目录。代理服务和 Claude 账户由用户自行准备。[便携约定](docs/windows.md#便携模式约定)

源码在本仓库公开，已发布安装包的版本说明和校验文件见 [GitHub Releases](https://github.com/CarterLeeAlt/Claude-Eden/releases)。镜像来源与数据保留方式见[客机系统指南](docs/guide/guest-systems.md)。

功能以所用构建为准，公开安装包的能力与验证范围以发行说明为准。

便携候选尚未完成自动更新验收，不要用安装器覆盖便携目录；更换程序前保存、停止环境并退出，备份并保留完整 `data/`。安装器模式的签名更新另见[升级与卸载](docs/guide/maintenance.md)。

## 三步开始

### 1. 选择代理

点击“添加节点”，手动填写 HTTP CONNECT、SOCKS5 或 SSH 的服务器与认证信息，保存并验证后选择一个节点，点击“应用代理”。[网络与代理](docs/guide/network.md)

### 2. 启动环境

首次为 Debian 13 客机选择存储位置。点击“开始安装”或启动环境，按引导完成安装与检查；进度和错误可在“操作详情”查看。以后启动复用原环境。

### 3. 打开项目，运行 Claude

点击“打开项目”选择本机上的文件夹，再新建终端，输入 `claude`，在 Workbench 浏览器中完成自己的账户授权。也可以先使用默认工作区，无需提前准备项目。完整步骤见[开始使用](docs/getting-started.md)。

## 使用前了解这些边界

- 共享项目中的修改和删除直接影响本机原文件，包括 Git 操作。请使用版本控制或备份，并核对提交结果。
- 同一份 Claude Eden 配置中的项目共用 Debian 13 客机、工具、账户与文件系统，项目之间没有独立隔离。
- 默认公网出站支持 IPv4 TCP，DNS 通过同一代理；公网 UDP、IPv6、ICMP 及主机和局域网直连受限。主动启用 Tailscale 后，可按私网权限连接设备。
- 网络检查展示当前配置和采样证据，无法保证账户不会受到平台限制，也不能代替完整的虚拟化安全审计。

详细机制见[网络与数据边界](docs/getting-started.md#网络与数据边界)。

## 文档与贡献

面向用户的[使用文档](docs/guide/index.md)按功能组织，各项操作见[使用文档导航](docs/README.md)。

| 你要做什么 | 从这里开始 |
|---|---|
| 让自己的 Agent 下载、安装、配置和排查 Claude Eden | [Agent 使用指南](docs/guide/agent.md) |
| 第一次安装、配置代理、打开项目 | [安装与首次启动](docs/guide/installation.md) |
| 准备 Debian 13 客机，使用 Linux 开发工具 | [Debian 13 客机](docs/guide/guest-systems.md) |
| 添加多个订阅、查看额度和分配终端 | [多账号与额度](docs/guide/accounts.md) |
| 了解每一项设置及何时生效 | [设置参考](docs/guide/settings.md) |
| 维护存储、资源、更新与备份 | [维护、资源与数据](docs/guide/maintenance.md) |
| 处理下载、启动、联网和登录问题 | [故障排查](docs/guide/troubleshooting.md) |
| 了解文件、按键和关闭与退出行为 | [项目、终端与文件](docs/guide/workbench.md) |
| 报告可复现问题 | [GitHub Issues](https://github.com/CarterLeeAlt/Claude-Eden/issues) |
| 提交修复、功能实现或文档改进 | [贡献指南](.github/CONTRIBUTING.md) |

代码与文档贡献流程见[贡献指南](.github/CONTRIBUTING.md)，问题与方案可提交到[本仓库 Issues](https://github.com/CarterLeeAlt/Claude-Eden/issues)。

本项目仍在迭代中，欢迎通过本仓库反馈问题和参与维护。

## 来源与许可

Claude Eden 的上游来源、原作者与版权声明见 [NOTICE](NOTICE)，许可证见 [LICENSE](LICENSE)。

项目主许可为 [MIT](LICENSE)。随附组件保留各自许可，Mihomo 作为独立可执行文件按 GPL-3.0 分发；请同时参阅 [NOTICE](NOTICE) · [第三方许可](THIRD_PARTY_LICENSES) · [noVNC](ui/vendor/novnc/LICENSE.txt)。

Claude Eden 是独立社区项目，与 Anthropic 无隶属关系。

## 参考与致谢

感谢以下项目为 Claude Eden 提供基础能力或设计参考：

- [agent-vm](https://github.com/sylvinus/agent-vm)：二次开发底座。
- [Lima](https://github.com/lima-vm/lima)：虚拟机管理与 Linux 客机的运行基础。
- [gvisor-tap-vsock](https://github.com/containers/gvisor-tap-vsock)：虚拟机网络能力。
- [sshocker](https://github.com/lima-vm/sshocker) 和 [SFTP](https://github.com/pkg/sftp)：虚拟机与主机之间的文件共享能力。
- [noVNC](https://github.com/novnc/noVNC)：浏览器中的远程画面与交互。
- [Mihomo](https://github.com/MetaCubeX/mihomo)：内置代理核心。
- [code-server](https://github.com/coder/code-server)：浏览器中的代码编辑器。
- [AIO Sandbox](https://github.com/agent-infra/sandbox)：工作环境和交互体验参考。
- [claude-switcher](https://github.com/YOURSJEYO/claude-switcher) 和 [Claude-Guardian](https://github.com/DUNHKpcc/Claude-Guardian)：网络检查和安全诊断方法参考。
- [claude-post-ban-cleanup](https://github.com/l2461090/claude-post-ban-cleanup): 精确盘点、保护用户资料与可恢复缓存清理的流程参考；本项目独立实现。

<a href="https://www.star-history.com/?repos=CarterLeeAlt%2FClaude-Eden&amp;type=date">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date&amp;theme=dark" />
    <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date" />
    <img alt="Claude Eden 的 GitHub Star 增长曲线" src="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date" width="800" />
  </picture>
</a>
