<div align="center">

<h1>Claude Eden</h1>

**Give Claude a place of her own, and keep your work close.**

<img src="assets/claude-eden-character.png" alt="Claude Eden" width="320" />

[![Release](https://img.shields.io/github/v/release/CarterLeeAlt/Claude-Eden?include_prereleases)](https://github.com/CarterLeeAlt/Claude-Eden/releases) [![CI](https://github.com/CarterLeeAlt/Claude-Eden/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/CarterLeeAlt/Claude-Eden/actions/workflows/ci.yml) [![MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

**[Project](https://github.com/CarterLeeAlt/Claude-Eden) · [Releases](https://github.com/CarterLeeAlt/Claude-Eden/releases) · [User guide · 中文](docs/guide/index.md) · [Troubleshooting · 中文](docs/guide/troubleshooting.md)**

[简体中文](README.md) · [繁體中文](README.zh-TW.md) · [English](README.en.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md)

</div>

## Tired of the friction around Anthropic

For people who simply want to talk and work with Claude, managing servers, browsers and environment checks can become a project of its own. **Claude Eden** brings that work together: **a desktop workbench for managing an isolated environment, your own proxy, time zone, language and environment checks.**

Claude Eden runs a local Debian 13 guest on a Windows 10 host. A terminal, browser, editor and the official Claude Desktop share one workbench. You choose which projects to share and manage the guest network separately. Host capabilities, resources and paths are detected on each computer.

- Guest network settings are separate from the host; Claude Eden routes guest public traffic through the selected proxy.
- Time zone and language can follow the proxy exit region, with explicit manual settings available.
- The browser, login flows and Claude Code processes work within the guest environment.
- Network and isolation checks keep public access closed when critical checks fail or evidence is incomplete; third-party checks provide supplementary information.

## What you can do

- **Work in one window.** Run Claude Code in the terminal, review changes in the editor, and use the Workbench browser for previews and sign-in.
- **Use an independent Linux environment.** Run Linux tools in local Debian 13 and share only explicitly approved project folders. Tools, files and sign-in state remain in the guest. [Debian 13 guest](docs/guide/guest-systems.md)
- **Control file sharing.** Only the default workspace and explicitly opened projects are shared. Host Claude logins, browser profiles and SSH keys are not imported automatically.
- **See your network state.** Add HTTP CONNECT, SOCKS5 or SSH nodes manually and apply one at a time. Critical checks must pass before public access is allowed; proxy failure never gives the guest a direct connection.
- **Pick up where you left off.** Tools, files and sign-in state persist on the virtual disk. Closing the window can leave background tasks running.
- **Manage Claude Code accounts.** View usage limits, assign accounts to new terminals, and explicitly approve switching accounts to continue a conversation. [Accounts](docs/guide/accounts.md)
- **Connect over a private network.** Enable Tailscale inside Claude Eden to access authorized devices and TCP services. [Tailscale](docs/guide/tailscale.md)

Bookmarks, private browsing, cookies, extensions, the password vault, appearance and maintenance are covered in the [user guide](docs/guide/index.md). Guest language options are Simplified Chinese, Traditional Chinese, English, Japanese, Korean, German and French. The workbench interface and detailed guide are in Chinese.

## Quick start

Give your coding agent this request:

```text
Fetch the latest source from https://github.com/CarterLeeAlt/Claude-Eden, read the Windows build instructions, then build and install it. Configure Claude Eden to use node xx in my local Clash client and my intended exit IP xx. Check the installation scope and network settings before making changes, then verify the result.
```

You provide your own proxy service and Claude account. This project does not sell or recommend proxy services.

## Download and install

The desktop host is Windows 10 x64 and the guest is local Debian 13. System capabilities, virtualization, resources and storage are checked on the actual computer. [Windows requirements](docs/windows.md) · [Guest systems](docs/guide/guest-systems.md)

1. Find the actual published Windows installer in [GitHub Releases](https://github.com/CarterLeeAlt/Claude-Eden/releases). If none is published, follow the [build instructions](docs/windows.md).
2. Run the installer and open **Claude Eden**.
3. Configure your proxy, prepare the environment and open a project.

The installer includes the workbench and Mihomo core. Go, Node.js, Docker and Clash are not prerequisites. On first creation, choose storage for Debian 13; the application downloads and verifies the official image and installs the development tools.

Source code is available in this repository. Published installers, release notes and checksums are listed in [Releases](https://github.com/CarterLeeAlt/Claude-Eden/releases). Features and verification scope depend on the build and its release notes.

Signed Windows releases support updates through the selected proxy. Unsigned development builds use the installer manually. Save and stop tasks before updating; personal data is preserved. [Updates and uninstalling](docs/guide/maintenance.md)

## Start in three steps

### 1. Select a proxy

Click “Add node” in network settings, choose HTTP CONNECT, SOCKS5 or SSH, and enter the server, port and authentication details. Save and validate, select one node, then apply it. Multiple saved nodes can be switched manually. [Network guide](docs/guide/network.md)

### 2. Start the environment

Choose a storage location for the Debian 13 guest on first use. Start installation and follow its progress and checks. Operation details show the current stage and errors. Later launches reuse the same environment.

### 3. Open a project and run Claude

Choose a local folder with “Open project”, open a terminal and enter `claude`. Complete your account authorization in the Workbench browser. You can also start in the default workspace. [Getting started](docs/getting-started.md)

## Understand these boundaries

- Changes and deletions in shared projects affect the original host files, including Git operations. Use version control or backups and review commits.
- Projects in the same Claude Eden profile share one Debian 13 guest, its tools, accounts and filesystem; projects are not isolated from one another.
- Public outbound traffic supports IPv4 TCP, with DNS through the same proxy. Public UDP, IPv6, ICMP and direct host or LAN access are restricted. Explicitly enabled Tailscale access follows its private-network permissions.
- Checks report the current configuration and sampled evidence. They cannot guarantee account availability or replace a complete virtualization security audit.

See [network and data boundaries](docs/getting-started.md#网络与数据边界).

## Documentation and contributions

The detailed [user guide](docs/guide/index.md) is organized by task and currently written in Chinese.

| Task | Start here |
|---|---|
| Have an agent install, configure and diagnose Claude Eden | [Agent guide](docs/guide/agent.md) |
| Install, configure a proxy and open a project | [First launch](docs/guide/installation.md) |
| Prepare Debian 13 and use Linux tools | [Debian 13 guest](docs/guide/guest-systems.md) |
| Manage accounts, usage limits and terminals | [Accounts](docs/guide/accounts.md) |
| Understand settings and when they take effect | [Settings](docs/guide/settings.md) |
| Maintain storage, resources and updates | [Maintenance](docs/guide/maintenance.md) |
| Diagnose download, startup and network problems | [Troubleshooting](docs/guide/troubleshooting.md) |
| Work with projects, terminals and files | [Workbench](docs/guide/workbench.md) |
| Report a reproducible problem | [GitHub Issues](https://github.com/CarterLeeAlt/Claude-Eden/issues) |
| Contribute code or documentation | [Contribution guide](.github/CONTRIBUTING.md) |

The project is under active development. Feedback and contributions are welcome through this repository; follow the [contribution process](.github/CONTRIBUTING.md).

## Sources and licensing

Original authors, attribution and upstream sources are listed in [NOTICE](NOTICE); the project license is [MIT](LICENSE). Bundled components retain their own licenses. Mihomo is distributed as a separate GPL-3.0 executable. See [third-party licenses](THIRD_PARTY_LICENSES) and [noVNC](ui/vendor/novnc/LICENSE.txt).

Claude Eden is an independent community project and is not affiliated with Anthropic.

## References and acknowledgments

- [agent-vm](https://github.com/sylvinus/agent-vm): the upstream foundation.
- [Lima](https://github.com/lima-vm/lima): virtual machines and Linux guests.
- [gvisor-tap-vsock](https://github.com/containers/gvisor-tap-vsock): virtual machine networking.
- [sshocker](https://github.com/lima-vm/sshocker) and [SFTP](https://github.com/pkg/sftp): host/guest file sharing.
- [noVNC](https://github.com/novnc/noVNC): remote display and interaction.
- [Mihomo](https://github.com/MetaCubeX/mihomo): proxy core.
- [code-server](https://github.com/coder/code-server): browser-based editor.
- [AIO Sandbox](https://github.com/agent-infra/sandbox): workspace and interaction references.
- [claude-switcher](https://github.com/YOURSJEYO/claude-switcher) and [Claude-Guardian](https://github.com/DUNHKpcc/Claude-Guardian): network-check and diagnostic references.
- [claude-post-ban-cleanup](https://github.com/l2461090/claude-post-ban-cleanup): inventory, data protection and recoverable cache-cleanup references; this project implements its own workflow.

<a href="https://www.star-history.com/?repos=CarterLeeAlt%2FClaude-Eden&amp;type=date"><picture><source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date&amp;theme=dark" /><source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date" /><img alt="Claude Eden GitHub star history" src="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date" width="800" /></picture></a>
