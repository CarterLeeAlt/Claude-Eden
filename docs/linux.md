# Debian 13 Linux 客机

Claude Eden 在 Windows 10 桌面管理本地 Debian 13 客机。日常入口为 Windows 应用，Linux 工具、浏览器、编辑器与 Claude Desktop 运行在独立客机中。

## Windows 上的 Linux 客机

Windows 10 x64 使用 QEMU 与 Windows 虚拟机监控程序平台运行 Debian 13。程序按实际电脑核对虚拟化和资源条件。首次安装下载官方镜像与开发工具，之后使用持久虚拟磁盘；具体版本、校验与空间要求见[客机系统指南](guide/guest-systems.md)。

Linux 客机普通账户为 `developer`，客机辅助工具采用 `Workbench` / `workbench` 标识。宿主程序和配置使用 `Claude Eden` / `eden`；两者职责和目录不同。

## 排查与反馈

- 环境不能联网：在工作台确认有效代理和网络检查；宿主浏览器联网不代表客机外网已放行。
- 本机 Linux 客机不能启动：按 [Windows 指南](windows.md)核对虚拟化及 WHPX 条件。

反馈时提供 Claude Eden 版本、Windows 版本、CPU 架构、失败步骤和错误原文，避免附带代理密码、订阅链接、账户文件和带会话信息的地址。
