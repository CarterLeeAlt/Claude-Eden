<div align="center">

<h1>Claude Eden</h1>

**Claude 専用の場所を用意して、大切な作業を守る。**

<img src="assets/claude-eden-character.png" alt="Claude Eden" width="320" />

[![Release](https://img.shields.io/github/v/release/CarterLeeAlt/Claude-Eden?include_prereleases)](https://github.com/CarterLeeAlt/Claude-Eden/releases) [![CI](https://github.com/CarterLeeAlt/Claude-Eden/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/CarterLeeAlt/Claude-Eden/actions/workflows/ci.yml) [![MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

**[プロジェクト](https://github.com/CarterLeeAlt/Claude-Eden) · [リリース](https://github.com/CarterLeeAlt/Claude-Eden/releases) · [使用ガイド・中国語](docs/guide/index.md) · [トラブル対処・中国語](docs/guide/troubleshooting.md)**

[简体中文](README.md) · [繁體中文](README.zh-TW.md) · [English](README.en.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md)

</div>

## Anthropic を使うための手間に疲れたら

Claude と会話し、一緒に作業したいだけなのに、サーバー、ブラウザー、環境確認の管理に時間がかかる。その負担をまとめるためのプロジェクトが **Claude Eden** です。**分離された作業環境、自分のプロキシ、タイムゾーン、言語、環境検査を一つのデスクトップで管理します。**

Claude Eden は Windows 10 上でローカルの Debian 13 ゲストを管理します。ターミナル、ブラウザー、エディター、公式 Claude Desktop を同じワークベンチに集約し、必要なプロジェクトだけを共有します。ホストの機能、リソース、保存先は実際のコンピューターで確認します。

- ゲストのネットワーク設定をホストから分離し、ゲストの公開ネットワーク通信を選択したプロキシへ転送します。
- タイムゾーンと言語をプロキシの出口地域に合わせられ、手動設定も可能です。
- ブラウザーログイン、認証、Claude Code のプロセスをゲスト環境内で扱います。
- 重要な検査が失敗した場合や証拠が不足する場合は外部通信を閉じます。第三者の検査は補助情報として利用します。

## できること

- **一つのウィンドウで作業する。** ターミナルで Claude Code を実行し、エディターで変更を確認し、Workbench ブラウザーでプレビューやログインを行います。
- **独立した Linux 環境を使う。** ローカルの Debian 13 で開発ツールを実行し、明示的に許可したプロジェクトだけを共有します。[Debian 13 ゲスト](docs/guide/guest-systems.md)
- **共有範囲を選ぶ。** 既定の作業フォルダーと明示的に開いたプロジェクトだけが共有されます。ホストの Claude ログイン、ブラウザー設定、SSH 鍵は自動で取り込みません。
- **ネットワーク状態を確認する。** HTTP CONNECT、SOCKS5、SSH のノードを手動登録し、一度に一つだけ適用します。重要な検査が通るまで外部通信は閉じたままです。プロキシ障害時にゲストが直接接続することはありません。
- **次回も同じ環境を使う。** ツール、ファイル、ログイン状態は仮想ディスクに保存されます。ウィンドウを閉じてもバックグラウンドの作業を継続できます。
- **Claude Code のアカウントを管理する。** 利用枠を確認し、新しいターミナルにアカウントを割り当て、明示的な許可のもとでアカウントを切り替えて会話を続けます。[アカウント管理](docs/guide/accounts.md)
- **プライベートネットワークに接続する。** Claude Eden 内の Tailscale で、許可した機器や TCP サービスに接続します。[Tailscale](docs/guide/tailscale.md)

ブックマーク、プライベートブラウジング、Cookie、拡張機能、パスワード保管庫、外観、保守については[使用ガイド](docs/guide/index.md)を参照してください。ゲストの言語は簡体字中国語、繁体字中国語、英語、日本語、韓国語、ドイツ語、フランス語に対応します。ワークベンチの画面と詳細ガイドは中国語です。

## クイックスタート

コーディング用エージェントに、次のように依頼できます。

```text
https://github.com/CarterLeeAlt/Claude-Eden の最新ソースを取得し、Windows 向けビルド手順を読んでから、ビルドとインストールを行ってください。Claude Eden のプロキシは、ローカルの Clash の xx ノードと指定した出口 IP xx を使うよう設定してください。実行前にインストール範囲とネットワーク設定を確認し、実行後に結果を検証してください。
```

プロキシサービスと Claude アカウントは自分で用意します。本プロジェクトはプロキシサービスの販売や推奨を行いません。

## ダウンロードとインストール

ホストは Windows 10 x64、ゲストはローカルの Debian 13 です。システム、仮想化機能、リソース、保存先は実機で確認します。[Windows の要件](docs/windows.md) · [ゲスト環境](docs/guide/guest-systems.md)

1. [GitHub Releases](https://github.com/CarterLeeAlt/Claude-Eden/releases) で公開済みの Windows インストーラーを確認します。未公開の場合は[ビルド手順](docs/windows.md)を使用してください。
2. インストーラーを実行して **Claude Eden** を開きます。
3. 自分のプロキシを設定し、環境を準備してプロジェクトを開きます。

インストーラーにはワークベンチと Mihomo が含まれます。Go、Node.js、Docker、Clash の事前インストールは不要です。初回は Debian 13 の保存先を選び、公式イメージのダウンロードと検証、開発ツールのインストールを行います。

ソースコードは本リポジトリで公開しています。公開インストーラー、リリースノート、チェックサムは [Releases](https://github.com/CarterLeeAlt/Claude-Eden/releases) を参照してください。利用可能な機能と検証範囲は、使用するビルドのリリースノートに従います。

署名済みの Windows 版は選択したプロキシ経由で更新できます。未署名の開発ビルドはインストーラーで手動更新します。更新前に作業を保存して停止してください。個人データは保持されます。[更新とアンインストール](docs/guide/maintenance.md)

## 三つの手順で開始

### 1. プロキシを選ぶ

ネットワーク設定の「添加节点」で HTTP CONNECT、SOCKS5、SSH のいずれかを選び、サーバー、ポート、認証情報を入力します。「保存并验证」で保存・検証し、ノードを一つ選んで「应用代理」を押します。複数のノードを保存して手動で切り替えられます。[ネットワークの説明](docs/guide/network.md)

### 2. 環境を起動する

初回に Debian 13 ゲストの保存先を選び、インストールを開始します。進行状況とエラーは操作詳細で確認できます。以後は同じ環境を再利用します。

### 3. プロジェクトを開いて Claude を実行する

「打开项目」でホストのフォルダーを選び、新しいターミナルで `claude` を実行します。Workbench ブラウザーで自分のアカウントを認証してください。既定の作業フォルダーから始めることもできます。[開始手順](docs/getting-started.md)

## 使用前に確認する境界

- 共有プロジェクト内の変更や削除は、Git 操作を含めてホストの元ファイルに反映されます。バージョン管理やバックアップを利用し、コミット内容を確認してください。
- 同じ Claude Eden 設定内のプロジェクトは、Debian 13 ゲスト、ツール、アカウント、ファイルシステムを共有します。プロジェクト同士は個別に分離されません。
- 公開ネットワークへの送信は IPv4 TCP と、同じプロキシを通る DNS に対応します。公開 UDP、IPv6、ICMP、ホストや LAN への直接接続は制限されます。明示的に有効化した Tailscale は私設ネットワークの権限に従います。
- 検査結果は現在の設定と採取した証拠を示します。アカウントの利用継続を保証するものではなく、仮想化全体のセキュリティ監査を代替しません。

詳細は[ネットワークとデータの境界](docs/getting-started.md#网络与数据边界)を参照してください。

## ドキュメントと貢献

詳細な[使用ガイド](docs/guide/index.md)は機能別に整理されており、中国語で提供しています。

| 目的 | 参照先 |
|---|---|
| エージェントで導入・設定・診断する | [Agent ガイド](docs/guide/agent.md) |
| 初回導入、プロキシ設定、プロジェクト開始 | [インストールと初回起動](docs/guide/installation.md) |
| Debian 13 と Linux ツールを使う | [Debian 13 ゲスト](docs/guide/guest-systems.md) |
| アカウント、利用枠、ターミナルを管理する | [アカウント管理](docs/guide/accounts.md) |
| 設定の意味と反映時点を確認する | [設定一覧](docs/guide/settings.md) |
| 保存先、リソース、更新を保守する | [保守](docs/guide/maintenance.md) |
| ダウンロード、起動、通信の問題を調べる | [トラブル対処](docs/guide/troubleshooting.md) |
| プロジェクト、ターミナル、ファイルを操作する | [ワークベンチ](docs/guide/workbench.md) |
| 再現可能な問題を報告する | [GitHub Issues](https://github.com/CarterLeeAlt/Claude-Eden/issues) |
| コードや文書を改善する | [貢献ガイド](.github/CONTRIBUTING.md) |

本プロジェクトは開発中です。問題の報告や改善提案を本リポジトリで受け付けています。[貢献手順](.github/CONTRIBUTING.md)をご確認ください。

## 出典とライセンス

上流プロジェクト、原作者、著作権表示は [NOTICE](NOTICE)、本体の MIT ライセンスは [LICENSE](LICENSE) に記載しています。同梱コンポーネントには各自のライセンスが適用され、Mihomo は独立した実行ファイルとして GPL-3.0 で配布します。[第三者ライセンス](THIRD_PARTY_LICENSES)と [noVNC](ui/vendor/novnc/LICENSE.txt)も参照してください。

Claude Eden は独立したコミュニティプロジェクトであり、Anthropic との提携関係はありません。

## 参考と謝辞

- [agent-vm](https://github.com/sylvinus/agent-vm)：開発の基盤。
- [Lima](https://github.com/lima-vm/lima)：仮想マシンと Linux ゲストの管理。
- [gvisor-tap-vsock](https://github.com/containers/gvisor-tap-vsock)：仮想マシンのネットワーク。
- [sshocker](https://github.com/lima-vm/sshocker) と [SFTP](https://github.com/pkg/sftp)：ホストとゲストのファイル共有。
- [noVNC](https://github.com/novnc/noVNC)：リモート画面と操作。
- [Mihomo](https://github.com/MetaCubeX/mihomo)：プロキシコア。
- [code-server](https://github.com/coder/code-server)：ブラウザー内のエディター。
- [AIO Sandbox](https://github.com/agent-infra/sandbox)：作業環境と操作設計の参考。
- [claude-switcher](https://github.com/YOURSJEYO/claude-switcher) と [Claude-Guardian](https://github.com/DUNHKpcc/Claude-Guardian)：ネットワーク検査と診断の参考。
- [claude-post-ban-cleanup](https://github.com/l2461090/claude-post-ban-cleanup)：データの棚卸し、保護、復元可能なキャッシュ整理の参考。本プロジェクトでは独自に実装しています。

<a href="https://www.star-history.com/?repos=CarterLeeAlt%2FClaude-Eden&amp;type=date"><picture><source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date&amp;theme=dark" /><source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date" /><img alt="Claude Eden の GitHub Star 推移" src="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date" width="800" /></picture></a>
