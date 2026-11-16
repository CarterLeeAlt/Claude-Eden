# 安全问题

Claude Eden 的隔离与网络检查范围见 [README](README.md) 和[网络与数据边界](docs/getting-started.md#网络与数据边界)。支持范围为 Windows 10 宿主与本地 Debian 13 客机。

**代理故障时客机绝不直连是必须满足的安全边界。** 客机公网业务及 DNS 在代理故障时只能失败或断网，不得自动回退主机直连、系统代理或主机 DNS；该要求覆盖安装、使用、更新和恢复过程。具体规则见[客机网络硬规则](docs/guide/network.md#客机网络硬规则)，相关改动必须通过[代理故障关闭验收](.github/CONTRIBUTING.md#代理故障关闭验收)。发现故障期间客机直连或通过管理通道绕过代理时，按隔离绕过问题处理并私下报告。

请勿在公开 Issue 中上传节点密码、SSH 私钥、会话入口、访问令牌、完整配置目录或未脱敏的诊断输出。

如果发现可绕过隔离、读取未共享文件或访问控制接口的问题，请使用[当前仓库](https://github.com/CarterLeeAlt/Claude-Eden)的 **Security → Report a vulnerability** 私下报告。若该入口尚未启用，请先通过[维护者的 GitHub 主页](https://github.com/CarterLeeAlt)联系维护者，仅说明需要私下报告安全问题，不公开漏洞细节。

报告请包含 Claude Eden 版本、操作系统版本与处理器架构、最小复现步骤、预期边界和实际结果。用测试文件和占位凭据复现，并说明是否仍在影响真实数据。
