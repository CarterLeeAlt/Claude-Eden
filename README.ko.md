<div align="center">

<h1>Claude Eden</h1>

**클로드 양을 우리에 가둬서 우리가 함께하는 시간을 더 잘 보호할 수 있도록**

<img src="assets/claude-eden-character.png" alt="Claude Eden" width="320" />

[![Release](https://img.shields.io/github/v/release/CarterLeeAlt/Claude-Eden?include_prereleases)](https://github.com/CarterLeeAlt/Claude-Eden/releases) [![CI](https://github.com/CarterLeeAlt/Claude-Eden/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/CarterLeeAlt/Claude-Eden/actions/workflows/ci.yml) [![MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

**[프로젝트](https://github.com/CarterLeeAlt/Claude-Eden) · [릴리스](https://github.com/CarterLeeAlt/Claude-Eden/releases) · [시작하기](docs/getting-started.md) · [FAQ](docs/troubleshooting.md) · [사용 문서 (중국어)](docs/guide/index.md)**

[简体中文](README.md) · [繁體中文](README.zh-TW.md) · [English](README.en.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md)

</div>

## Anthropic에 지쳤습니다

클로드와 대화하고 협업하고 싶은 사용자로서 VPS, 브라우저 지문 프로필과 환경 검사를 설정하는 데 지쳤으며 계정 정지와 금전적 손실을 걱정합니다. **Claude Eden**을 소개합니다. **자신의 프록시를 설정하고 격리 환경, 시간대, 언어와 환경 검사를 한 곳에서 관리하는 데스크톱 작업 공간입니다.**

Claude Eden은 Windows 10에서 로컬 Debian 13 게스트를 관리하며 터미널, 브라우저, 편집기와 공식 Claude Desktop을 한 작업대에 모읍니다. 필요한 프로젝트만 공유하고 네트워크를 따로 관리하세요. 호스트 기능과 자원은 각 컴퓨터에서 실제로 확인합니다. 주요 기능:

- 호스트와 격리된 네트워크 설정: Claude Eden은 자체 프록시를 통해 환경 내부의 모든 네트워크 트래픽을 라우팅합니다.
- 네트워크 프록시의 IP 위치에 대한 자동 동기화를 통해 시간대 및 언어를 포함하여 호스트에서 격리된 브라우저 지문 설정
- Claude 워크플로우 전반에 걸친 통합: 브라우저 로그인, 계정 인증 및 Claude Code 프로세스
- 내장된 네트워크 및 격리 검사는 중요한 검사가 실패하거나 증거가 불완전한 경우 인터넷 액세스를 일시 중지합니다. 커뮤니티 및 제3자 확인을 통해 추가 참조 정보 제공

## Claude Eden으로 할 수 있는 일

- **단일 창에서 작업합니다.** 터미널에서 Claude Code를 실행하고, 편집기에서 변경 사항을 검토하고, Workbench 브라우저로 서비스를 미리 보고 로그인하세요.
- **독립된 Linux 환경에서 개발하세요.** 로컬 Debian 13에서 Linux 도구를 사용하며 명시적으로 허용한 프로젝트만 공유하세요. [Debian 13 게스트](docs/guide/guest-systems.md)
- **공유하는 내용을 제어하세요.** 기본 작업공간과 명시적으로 열린 프로젝트 폴더만 공유됩니다. 호스트의 Claude 로그인 상태, 브라우저 프로필 및 SSH 키는 자동으로 가져오지 않습니다.
- **네트워크 상태를 확인하세요.** HTTP CONNECT, SOCKS5 또는 SSH 노드를 수동으로 등록하고 하나를 선택해 출구 IP를 확인합니다. 필수 검사를 통과하기 전까지 환경은 오프라인으로 유지되며, 프록시에 장애가 발생해도 게스트는 절대 직접 연결하지 않습니다.
- **다음에 계속하세요.** 도구, 파일 및 로그인 상태는 가상 디스크에 유지됩니다. 창을 닫아도 백그라운드 작업이 계속 실행될 수 있습니다.
- **Claude Code 계정을 여러 개 관리하세요.** 사용 한도를 확인하고 새 세션에 계정을 배정하며, 명시적으로 허용한 뒤 계정을 바꾸어 대화 기록을 이어가세요. [Claude Code](docs/guide/accounts.md)
- **사설망으로 연결하세요.** Claude Eden 안에서 Tailscale에 로그인해 호스트와 환경 사이에 양방향 SSH 연결을 사용하세요. [Tailscale / SSH](docs/guide/tailscale.md)

게스트 언어는 중국어 간체·번체, 영어, 일본어, 한국어, 독일어, 프랑스어를 지원합니다. 출구 지역에 맞는 지원 언어를 추천하며, 추천이 없으면 선택한 언어를 유지합니다. 워크벤치 화면과 상세 사용 문서는 중국어로 제공됩니다.

## 빠른 시작

이 요청을 AI 에이전트(codex, dsh, workbuddy — Claude Code는 아닐 수도 있음 😁) 에 보냅니다.

```markdown
https://github.com/CarterLeeAlt/Claude-Eden의 최신 소스 코드를 받아 Windows용으로 빌드하고 설치해 줘. Claude Eden 프록시가 내 로컬 Clash의 xx 노드와 xx IP를 사용하도록 설정해 줘. 먼저 빌드 안내를 읽고 설치 범위와 네트워크 설정을 확인한 뒤 실행하고 검증해 줘.
```

필요한 것은 안정적인 네트워크 프록시 엔드포인트뿐입니다. 이 프로젝트는 프록시 공급자를 홍보하지 않습니다. 예를 들어 Black Error 웹사이트를 검색하여 제공자를 직접 찾을 수 있습니다.

## 다운로드 및 설치

Claude Eden의 데스크톱 호스트는 Windows 10 x64이며 로컬 게스트는 Debian 13입니다. 시스템과 가상화 기능은 각 컴퓨터의 실제 지원 능력을 기준으로 감지합니다. [Windows](docs/windows.md) · [Debian 13 게스트](docs/guide/guest-systems.md).

1. [GitHub Releases](https://github.com/CarterLeeAlt/Claude-Eden/releases)에서 실제 게시된 Windows 설치 파일을 확인하세요. 게시된 버전이 없으면 [빌드 안내](docs/windows.md)를 따르세요.
2. Windows 설치 프로그램을 실행하고 **Claude Eden**을 시작하세요.
3. 앱 내 지침에 따라 자신의 네트워크 프록시를 구성하고, 환경을 시작하고, 프로젝트를 엽니다.

설치 패키지에는 작업 공간과 프록시 엔진이 포함됩니다. Go, Node.js, Docker, Clash를 미리 설치할 필요가 없습니다. 환경을 만들 때 Debian 13 저장 위치를 선택하고 공식 이미지와 개발 도구를 준비하세요. 프록시 서비스와 Claude 계정은 사용자가 준비합니다.

소스 코드는 공개되어 있습니다. 게시된 버전의 안내와 체크섬은 [Releases](https://github.com/CarterLeeAlt/Claude-Eden/releases)에서 확인하세요.

기능은 사용 중인 빌드에 따라 다르며, 공개 설치 파일의 기능과 검증 범위는 릴리스 설명을 기준으로 합니다.

서명된 Windows 버전은 선택한 프록시를 통해 업데이트합니다. 서명되지 않은 개발 빌드는 설치 파일로 수동 업데이트합니다. 먼저 작업을 저장하고 중지하세요. 개인 데이터는 유지됩니다. [업데이트 및 제거](docs/guide/maintenance.md)

## 3단계로 시작하세요

### 1. 네트워크 프록시를 선택하세요

‘설정 → 네트워크 및 프록시’에서 ‘노드 추가’를 클릭합니다. HTTP CONNECT, SOCKS5 또는 SSH를 선택하고 서버, 포트, 인증 정보를 입력합니다. 저장 및 검증 후 노드 하나를 선택하고 ‘프록시 적용’을 클릭합니다. 여러 노드를 저장해 수동으로 전환할 수 있으며 한 번에 하나만 사용합니다. Claude Eden에는 Mihomo 코어가 포함되어 있습니다.

### 2. 환경 시작

처음 만들 때 Debian 13의 저장 위치를 지정하세요. 게스트를 설치하고 환경을 시작한 뒤 필요한 권한을 승인하세요. 작업 상세에서 진행 상태와 오류를 볼 수 있습니다. 이후에는 같은 환경을 다시 사용합니다.

### 3. 프로젝트를 열고 Claude를 실행하세요.

'프로젝트 열기'로 폴더를 선택하고 새 터미널에서 `claude`를 실행한 뒤 Workbench 브라우저에서 자신의 계정을 인증하세요. 기본 작업 공간으로 시작할 수도 있습니다. 전체 단계: [시작하기](docs/getting-started.md).

## 사용하기 전에 이러한 제한 사항을 이해하십시오.

- 공유 프로젝트의 편집과 삭제는 컴퓨터 파일에 직접 영향을 줍니다. 버전 관리 또는 백업을 사용하세요. `.git`은 읽기와 쓰기가 가능하며 Claude Eden에서 스테이징, 커밋과 브랜치 전환을 할 수 있습니다.
- 같은 Claude Eden 설정의 프로젝트는 게스트, 도구, 계정과 파일 시스템을 공유하며 서로 격리되지 않습니다. 시스템을 바꾸려면 별도로 설치해야 하며 기존 환경의 데이터는 보존됩니다.
- 기본 인터넷 아웃바운드는 IPv4 TCP를 지원하며 DNS는 같은 프록시를 사용합니다. 인터넷 UDP, IPv6, ICMP와 호스트·LAN 직접 연결은 제한됩니다. Tailscale을 직접 활성화하면 사설망 권한에 따라 기기에 연결할 수 있습니다.
- 네트워크 검사에는 현재 구성과 샘플링된 증거가 표시됩니다. 계정이 플랫폼 제한을 피할 것이라고 보장할 수 없으며 완전한 가상화 보안 감사를 대체하지 않습니다.

자세한 내용은 다음을 참조하세요. [네트워크 및 데이터 경계](docs/getting-started.md#网络与数据边界).

## 문서화 및 기여

전체 색인을 보려면 다음을 참조하세요. [문서 색인](docs/README.md).

| 귀하의 작업 | 여기서 시작하세요 |
|---|---|
| Debian 13 | [Debian 13 게스트](docs/guide/guest-systems.md) |
| 네트워크 프록시 설치, 구성 및 첫 번째 프로젝트 열기 | [시작하기](docs/getting-started.md) |
| 다운로드, 시작, 연결 및 로그인 문제 해결 | [FAQ 및 복구](docs/troubleshooting.md) |
| 키보드 컨트롤, 모양, 데이터 지속성 및 종료 동작 알아보기 | [매일 사용](docs/getting-started.md#日常使用) |
| 재현 가능한 문제 보고 | [GitHub Issues](https://github.com/CarterLeeAlt/Claude-Eden/issues) |
| 수정·기능·문서 기여: Issue 검토 → 협업자 초대 → PR | [기여 안내 (중국어)](.github/CONTRIBUTING.md) |

프로젝트는 계속 발전하고 있습니다. 이 저장소에서 문제와 개선 제안을 공유해 주세요.

## 원산지 및 라이선스

Claude Eden의 원래 출처, 기여자 및 저작권 고지는 [NOTICE](NOTICE)에, 라이선스 조건은 [LICENSE](LICENSE)에 있습니다.

프로젝트의 기본 라이센스는 다음과 같습니다. [MIT](LICENSE). 번들 구성 요소는 해당 라이센스를 유지합니다. Mihomo는 GPL-3.0에 따라 별도의 실행 파일로 배포됩니다. 또한 참조하십시오 [NOTICE](NOTICE) · [타사 라이선스](THIRD_PARTY_LICENSES) · [noVNC](ui/vendor/novnc/LICENSE.txt)。

Claude Eden은 독립적인 커뮤니티 프로젝트이며 Anthropic과 관련이 없습니다.

## 참고자료 및 감사의 말

기본 기능 및 설계 참조에 대한 다음 프로젝트에 감사드립니다.

- [agent-vm](https://github.com/sylvinus/agent-vm): 프로젝트의 기반.
- [Lima](https://github.com/lima-vm/lima): Linux 가상 머신 관리.
- [gvisor-tap-vsock](https://github.com/containers/gvisor-tap-vsock): 가상 머신 네트워킹.
- [sshocker](https://github.com/lima-vm/sshocker) 그리고 [SFTP](https://github.com/pkg/sftp): 가상 머신과 호스트 간의 파일 공유.
- [noVNC](https://github.com/novnc/noVNC): 브라우저에서의 원격 표시 및 상호 작용.
- [Mihomo](https://github.com/MetaCubeX/mihomo): 내장된 네트워크 프록시 코어입니다.
- [code-server](https://github.com/coder/code-server): 브라우저 기반 코드 편집기입니다.
- [AIO Sandbox](https://github.com/agent-infra/sandbox): 작업 환경 및 인터랙션 디자인에 대한 참고 자료입니다.
- [claude-switcher](https://github.com/YOURSJEYO/claude-switcher) 그리고 [Claude-Guardian](https://github.com/DUNHKpcc/Claude-Guardian): 네트워크 점검 및 보안 진단을 위한 참고 자료입니다.
- [claude-post-ban-cleanup](https://github.com/l2461090/claude-post-ban-cleanup): 정확한 목록 확인, 사용자 데이터 보호 및 복구 가능한 캐시 정리 절차 참고. Claude Eden에서 독립적으로 구현했습니다.


<a href="https://www.star-history.com/?repos=CarterLeeAlt%2FClaude-Eden&amp;type=date">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date&amp;theme=dark" />
    <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date" />
    <img alt="Claude Eden GitHub Star History" src="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date" width="800" />
  </picture>
</a>
