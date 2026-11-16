---
title: 从这里开始
description: 在 Windows 10 与本地 Debian 13 的统一工作台使用 Claude，找到安装、项目、网络与维护指南。
group: 开始使用
order: 0
keywords: 概览 功能 导航 入门 版本 客机 Windows Debian 13 Linux
---
Claude Eden 在 Windows 10 桌面管理本地 Debian 13 客机，将终端、浏览器、编辑器和 Claude Desktop 放进同一个工作台。项目按需共享，网络独立管理，工具与登录资料随环境保留。

你可以打开本机项目，在客机中运行 Claude Code、查看改动、预览网页并完成授权；需要私网服务时可在客机登录 Tailscale。

本手册描述当前开发构建的功能。公开安装包的能力和验收范围以对应发行说明为准。首次创建使用 Debian 13 官方镜像建立独立客机；详细条件与验证范围见[Debian 13 客机](guest-systems.md)。

## 第一次使用

1. 阅读[安装与首次启动](installation.md)，在 Windows 10 安装 Claude Eden；按[Debian 13 客机](guest-systems.md)准备本地 Linux 环境。
2. 在[网络与代理](network.md)中导入自己的节点，应用代理后启动环境，等待关键检查通过。
3. 按[项目、终端与文件](workbench.md)打开项目，在 Workbench 浏览器内完成 Claude Code 授权，发出第一条消息。
4. 有多个 Claude 订阅时，继续配置[多账号与额度](accounts.md)。

## 按你想做的事查找

| 你想做什么 | 阅读哪篇 |
| --- | --- |
| 把安装、配置与排障交给自己的 Agent | [让 Agent 配置 Claude Eden](agent.md) |
| 安装应用、换盘、准备本机虚拟化 | [安装与首次启动](installation.md) |
| 准备 Debian 13 Linux 开发环境 | [Debian 13 客机](guest-systems.md) |
| 添加代理节点、切换单个出口、设置出口规则 | [网络与代理](network.md) |
| 打开项目、启动 Claude 终端、传文件与截图 | [项目、终端与文件](workbench.md) |
| 添加多个订阅、查看额度、分配或接续会话 | [多账号与额度](accounts.md) |
| 收藏网页、无痕浏览、清理 Cookie、安装扩展 | [浏览器与网站数据](browser.md) |
| 在 Claude Eden 内使用 Claude Desktop | [Claude Desktop](desktop.md) |
| 在 Claude Eden 登录私网，从本机与客机双向使用 SSH | [Tailscale 与 SSH 私网](tailscale.md) |
| 找到每一项设置，了解修改何时生效 | [设置参考](settings.md) |
| 换存储盘、调整 CPU 与内存、升级、备份或卸载 | [维护、资源与数据](maintenance.md) |
| 按错误现象恢复使用、导出诊断 | [故障排查](troubleshooting.md) |

## 主机与客机如何配合

Debian 13 虚拟机在 Windows 10 电脑运行。程序按实际电脑检查虚拟化能力；明确打开的项目与客机共享，改动直接影响原文件。Windows 负责原生文件选择、剪贴板和显示输入，客机负责开发工具与应用进程。关闭窗口可以保留后台任务；停止环境会结束其中的终端。

## 先了解数据与网络边界

- 打开的项目可以被环境中的程序读取与修改。重要文件先备份；项目内的 `.git` 可读写，可以在 Claude Eden 内审查、暂存与提交代码。
- 同一环境的所有项目共用客机系统、工具、账户和文件系统。切换项目不会停止原任务，也不会撤销已授权的共享。
- Claude Eden 需要你自己的代理和 Claude 使用资格。网络检查通过表示当前配置和采样证据通过，无法保证第三方账户不受限制。
- 默认公网出站支持 IPv4 TCP，DNS 通过同一代理；公网 UDP、IPv6、ICMP，以及环境到主机或局域网的直连受限。主动启用 Tailscale 后，已授权的私网设备可通过独立通道连接。
- 关闭窗口、锁定 Claude Eden、停止环境和退出账户的影响不同，操作前可查[关闭与退出](workbench.md#关闭锁定与退出)。
