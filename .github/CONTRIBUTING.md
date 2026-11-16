# 贡献指南

Claude Eden 采用“贡献 Issue 审核 → 邀请成为协作者 → 提交 PR → 审查合入”的流程。仓库仅允许协作者创建 PR；修复代码、改进功能和修改文档都从明确的贡献方案开始。

## 提交方案并等待审核

在[新建 Issue 页面](https://github.com/CarterLeeAlt/Claude-Eden/issues/new/choose)选择“方案贡献（Contribution）”。说明要解决的问题、基于的版本或提交、适用系统、解决思路、验证结果和影响范围。有代码时附上 Fork 分支、具体提交、diff 或补丁；尚未验证的部分请明确注明。

只有问题描述时使用“问题反馈”；尚无具体方案的功能需求使用“功能建议”。提交前先搜索已有 Issue，已有相关讨论时在贡献 Issue 中关联它。

通过 GitHub CLI 或 API 创建时，请保留标题开头的 `[Contribution]`。这类请求无法带上表单标签，**Label contribution issues** 工作流会按标题前缀补上 `contribution` 标签；修改标题后同样生效。

仓库管理员或具有 `maintain` 权限的维护者审核方案、确认愿意接纳本次贡献后，为仍处于打开状态的 `contribution` Issue 添加 `accepted` 标签。通过审核表示可以进入实现和 PR 审查阶段，最终合入仍取决于代码审查与验证结果。

## 系统标签

Issue 按涉及的系统带有两个标签：`Windows 主机`、`Linux 客机`。Windows 10 主机运行桌面应用，客机是 Claude Eden 内运行的本地 Debian 13 系统。一条 Issue 可以同时带两个标签，维护者按标签筛选并认领自己负责的 Issue。

新建 Issue 时，**Label issues by system** 工作流读取正文里标题含“主机”或“客机”的字段并添加对应标签。通过 GitHub CLI 或 API 创建时沿用表单的字段标题即可，例如 `### 主机系统` 下写 `Windows 10`，`### 客机系统` 下写 `Debian 13`。

工作流只添加标签，维护者手动调整的标签会保留。正文修改后需要重新识别时，在 **Actions → Label issues by system** 选择 **Run workflow** 并填写 Issue 编号；识别有误或正文没有系统信息时由维护者手动修正。

## 接受协作者邀请

添加 `accepted` 标签会触发 **Invite approved contributors** 工作流，向该 Issue 的作者发送协作者邀请，并在 Issue 中说明后续步骤。已有写权限的协作者可直接提交 PR；待接受的邀请会被复用。

贡献者需要通过 GitHub 通知或[仓库邀请页面](https://github.com/CarterLeeAlt/Claude-Eden/invitations)接受邀请。邀请过期或流程失败时，请在原 Issue 中联系维护者处理。

协作者拥有整个仓库的写权限，该权限不限于这一条 Issue。`main` 受分支保护，协作者通过独立分支和 PR 交付变更，由代码所有者 `@CarterLeeAlt` 审核后合入；新增提交后需要重新审核。关闭 Issue 或移除 `accepted` 标签不会自动取消邀请或撤销已有权限；权限调整由仓库管理员在仓库设置中处理。

## 提交 PR

1. 接受邀请后，将本地基线更新到仓库最新的 `main`。
2. 为本次贡献创建独立分支，命名为“开始日期-GitHub用户名-工作内容”，例如 `261112-alice-fix-startup`。保留无关的在途改动，提交范围与审核通过的方案保持一致；扩大范围时先在原 Issue 中讨论。
3. 完成实现并运行与改动相关的检查。涉及主客机协作时核对 Windows 10 桌面与本地 Debian 13 客机；说明实际验证的系统版本、能力、结果及未验证范围。产品要求依据实际依赖，验收机器的构建号与硬件配置只是测试记录。
4. 推送分支并创建指向 `main` 的 PR，在正文中关联贡献 Issue，例如 `Closes #123`，说明用户可见的变化、验证结果和已知限制。
5. 根据评审意见更新 PR。通过审查与所需检查后由维护者合入，随后删除已合入的分支并关闭对应 Issue。

普通推送不触发构建。PR 按改动范围运行桌面／客机和网站检查；维护者可在 Actions 中手动运行 **Windows desktop and Linux guest checks** 或 **Website and public releases**。发布后的安装包核验与网站部署由发布流程触发。尚无公开 Release 时，网站显示“尚无公开安装包”；指定版本或下载验收仍要求完整且校验通过的公开产物。

源代码、配置、日志和附件中均不得包含密码、令牌、SSH 私钥等敏感信息。引用第三方代码时注明来源并保留所需许可。

## 代理故障关闭验收

**Claude Eden 必须满足“代理故障时客机绝不直连”。** 完整边界见[客机网络硬规则](../docs/guide/network.md#客机网络硬规则)。涉及客机出站、代理核心或协议、DNS、网络放行与恢复的实现或运行配置改动，须在隔离的 Windows 宿主与本地 Debian 13 测试环境中验收；可能影响真实配置、数据或系统状态的测试，先明确落点与收尾方式并取得授权。

1. **正常路径**：覆盖本次改动涉及的全部受支持协议和认证方式，从客机终端、浏览器及受影响的安装或维护入口发起请求，确认 TCP 和业务 DNS 经过所选代理，出口符合规则。
2. **真实故障**：分别停止上游代理、终止本机代理核心，制造连接超时、认证失败、DNS 失败、配置缺失或失效，并覆盖工作台控制服务退出与联网许可到期；同时检查新请求和已有长连接。已有连接不得转成直连，闸门撤销后必须关闭。
3. **状态转换**：核对启动、手动暂停、节点切换、主机网络切换、重试与恢复期间的约束；关闭后须满足现有检查才恢复联网，人工暂停状态不得被自动恢复覆盖。
4. **网络证据**：用宿主侧抓包或可归因的连接观测，配合受控目标与 DNS 服务记录，确认没有客机请求绕过所选代理访问目标网站或 DNS。区分代理端点连接、宿主自身下载、代理启动解析及必要内部传输；同时核对现有公网 UDP、IPv6、ICMP 与宿主／局域网访问限制未被削弱。
5. **判定与收尾**：运行相关自动化回归并记录故障、请求结果与网络证据。仅有代理校验通过、界面显示“离线”、模拟测试或一次出口 IP 读数不足以验收；存在直连、证据缺失或未测项时不得宣称满足硬规则或交付相关改动。测试结束后恢复本次测试状态并收尾进程。

## 维护者：配置自动邀请

自动邀请使用 `contribution-invites` Environment 中的 Secret `CONTRIBUTION_INVITE_TOKEN`。使用只授权 `CarterLeeAlt/Claude-Eden` 的 fine-grained personal access token，并授予仓库权限 **Administration: Read and write**；其余权限保持最小范围。令牌应设置有效期，并在到期前更新。

在仓库 **Settings → Environments → contribution-invites** 中，只允许 `main` 分支部署，并将该令牌保存为 Environment Secret。邀请令牌不得放进普通仓库 Secret，否则具有写权限的协作者可以在其他分支的工作流中使用它。GitHub Actions 自带的 `GITHUB_TOKEN` 用于检查审核者权限和写入提醒，无法单独完成协作者邀请。

`main` 的分支保护要求 PR、至少一次代码所有者审核及新提交后的重新审核，禁止强制推送和删除；`.github/CODEOWNERS` 将所有路径的代码所有者设为 `@CarterLeeAlt`。管理员保留处理自有改动的权限。这些保护与仅限 `main` 的 Environment 共同阻止普通协作者在未审核的分支中取得邀请令牌。

日常审核步骤：

1. 确认 Issue 有 `contribution` 标签，并完成方案与贡献者的审核。标题没有 `[Contribution]` 前缀的贡献不会自动获得标签，确认属于贡献后手动添加。
2. 由管理员或具有 `maintain` 权限的维护者添加 `accepted` 标签。普通 `write`、`triage` 和 `read` 权限不会触发授权成功。
3. 查看 **Actions → Invite approved contributors** 的结果。只有邀请成功、已有待接受邀请或作者已有写权限时，工作流才会在 Issue 中发布后续步骤。
4. 如凭据缺失、过期、权限不足或邀请发送失败，先修复原因，再在该工作流页面选择 **Run workflow**，使用 `main` 分支并填写原 Issue 编号重试。重试同样检查当前审核者权限、Issue 状态及两个标签，并复用已有提醒。

自动化读取默认分支中的脚本，不执行申请者的代码。它只负责审核后的邀请和提醒；方案审核、接受邀请、实现代码和 PR 审查由对应参与者完成。

GitHub 参考：[邀请协作者 API](https://docs.github.com/en/rest/collaborators/collaborators#add-a-repository-collaborator)、[工作流令牌的权限边界](https://docs.github.com/en/actions/tutorials/authenticate-with-github_token#granting-additional-permissions)。
