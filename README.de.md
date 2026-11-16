<div align="center">

<h1>Claude Eden</h1>

**Ein eigener Platz für Claude, damit deine Arbeit bei dir bleibt.**

<img src="assets/claude-eden-character.png" alt="Claude Eden" width="320" />

[![Release](https://img.shields.io/github/v/release/CarterLeeAlt/Claude-Eden?include_prereleases)](https://github.com/CarterLeeAlt/Claude-Eden/releases) [![CI](https://github.com/CarterLeeAlt/Claude-Eden/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/CarterLeeAlt/Claude-Eden/actions/workflows/ci.yml) [![MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

**[Projekt](https://github.com/CarterLeeAlt/Claude-Eden) · [Veröffentlichungen](https://github.com/CarterLeeAlt/Claude-Eden/releases) · [Benutzerhandbuch · 中文](docs/guide/index.md) · [Fehlersuche · 中文](docs/guide/troubleshooting.md)**

[简体中文](README.md) · [繁體中文](README.zh-TW.md) · [English](README.en.md) · [日本語](README.ja.md) · [한국어](README.ko.md) · [Deutsch](README.de.md) · [Français](README.fr.md)

</div>

## Genug vom Aufwand rund um Anthropic

Wer einfach mit Claude sprechen und arbeiten möchte, kann viel Zeit mit Servern, Browsern und Umgebungsprüfungen verlieren. **Claude Eden** bündelt diese Aufgaben: **eine Desktop-Arbeitsumgebung für einen isolierten Gast, deinen eigenen Proxy, Zeitzone, Sprache und Umgebungsprüfungen.**

Claude Eden verwaltet einen lokalen Debian-13-Gast auf einem Windows-10-Host. Terminal, Browser, Editor und der offizielle Claude Desktop laufen in einer gemeinsamen Arbeitsoberfläche. Du wählst die freigegebenen Projekte und verwaltest das Gastnetz getrennt. Fähigkeiten, Ressourcen und Speicherpfade werden auf dem jeweiligen Computer geprüft.

- Die Netzwerkeinstellungen des Gasts sind vom Host getrennt; öffentlicher Gastverkehr läuft über den gewählten Proxy.
- Zeitzone und Sprache können der Proxy-Ausgangsregion folgen oder manuell festgelegt werden.
- Browseranmeldung, Autorisierung und Claude-Code-Prozesse werden innerhalb der Gastumgebung verwendet.
- Fehlgeschlagene kritische Prüfungen oder unvollständige Nachweise halten den Internetzugang geschlossen. Prüfungen Dritter liefern ergänzende Informationen.

## Was du damit tun kannst

- **In einem Fenster arbeiten.** Starte Claude Code im Terminal, prüfe Änderungen im Editor und nutze den Workbench-Browser für Vorschauen und Anmeldungen.
- **Eine eigene Linux-Umgebung nutzen.** Arbeite mit Linux-Werkzeugen im lokalen Debian 13 und teile nur ausdrücklich freigegebene Projektordner. Werkzeuge, Dateien und Anmeldungen bleiben im Gast. [Debian-13-Gast](docs/guide/guest-systems.md)
- **Freigaben kontrollieren.** Nur der Standardarbeitsordner und ausdrücklich geöffnete Projekte werden geteilt. Claude-Anmeldungen, Browserprofile und SSH-Schlüssel des Hosts werden nicht automatisch übernommen.
- **Den Netzwerkzustand sehen.** Trage HTTP-CONNECT-, SOCKS5- oder SSH-Knoten manuell ein und verwende jeweils einen. Öffentlicher Zugang wird erst nach erfolgreichen kritischen Prüfungen freigegeben; ein Proxy-Ausfall führt niemals zu einer direkten Gastverbindung.
- **Später weiterarbeiten.** Werkzeuge, Dateien und Anmeldungen bleiben auf der virtuellen Festplatte erhalten. Hintergrundaufgaben können nach dem Schließen des Fensters weiterlaufen.
- **Claude-Code-Konten verwalten.** Prüfe Nutzungslimits, ordne neuen Terminals Konten zu und bestätige einen Kontowechsel ausdrücklich, bevor ein Gespräch fortgesetzt wird. [Konten](docs/guide/accounts.md)
- **Ein privates Netzwerk nutzen.** Aktiviere Tailscale in Claude Eden, um berechtigte Geräte und TCP-Dienste zu erreichen. [Tailscale](docs/guide/tailscale.md)

Lesezeichen, private Sitzungen, Cookies, Erweiterungen, Passworttresor, Darstellung und Wartung beschreibt das [Benutzerhandbuch](docs/guide/index.md). Die Gastsprachen sind vereinfachtes und traditionelles Chinesisch, Englisch, Japanisch, Koreanisch, Deutsch und Französisch. Die Workbench-Oberfläche und das ausführliche Handbuch sind chinesisch.

## Schnellstart

Du kannst deinem Coding-Agenten diese Aufgabe geben:

```text
Hole den aktuellen Quellcode von https://github.com/CarterLeeAlt/Claude-Eden, lies die Windows-Bauanleitung und baue und installiere die Anwendung. Richte ihren Proxy über den Knoten xx meines lokalen Clash-Clients mit der gewünschten Ausgangs-IP xx ein. Prüfe Installationsumfang und Netzwerkeinstellungen vor der Ausführung und verifiziere anschließend das Ergebnis.
```

Den Proxy-Dienst und das Claude-Konto stellst du selbst bereit. Das Projekt verkauft oder empfiehlt keine Proxy-Dienste.

## Herunterladen und installieren

Der Desktop-Host ist Windows 10 x64, der Gast ein lokales Debian 13. System, Virtualisierung, Ressourcen und Speicherorte werden auf dem tatsächlichen Computer geprüft. [Windows-Voraussetzungen](docs/windows.md) · [Gastsystem](docs/guide/guest-systems.md)

1. Suche in [GitHub Releases](https://github.com/CarterLeeAlt/Claude-Eden/releases) nach einem tatsächlich veröffentlichten Windows-Installationspaket. Ist noch keines verfügbar, verwende die [Bauanleitung](docs/windows.md).
2. Führe das Installationsprogramm aus und öffne **Claude Eden**.
3. Konfiguriere deinen Proxy, bereite die Umgebung vor und öffne ein Projekt.

Das Paket enthält die Workbench und Mihomo. Go, Node.js, Docker und Clash müssen nicht vorinstalliert sein. Bei der ersten Einrichtung wählst du den Speicherort für Debian 13; das Programm lädt und prüft das offizielle Abbild und installiert die Entwicklungswerkzeuge.

Der Quellcode ist öffentlich. Veröffentlichte Pakete, Versionshinweise und Prüfsummen stehen unter [Releases](https://github.com/CarterLeeAlt/Claude-Eden/releases). Funktionen und Prüfungsumfang richten sich nach dem verwendeten Build und dessen Versionshinweisen.

Signierte Windows-Versionen unterstützen Updates über den ausgewählten Proxy. Unsignierte Entwicklungsstände werden manuell mit dem Installer aktualisiert. Speichere und beende Aufgaben vor dem Update; persönliche Daten bleiben erhalten. [Updates und Deinstallation](docs/guide/maintenance.md)

## In drei Schritten starten

### 1. Einen Proxy auswählen

Klicke in den Netzwerkeinstellungen auf „添加节点“ (Knoten hinzufügen), wähle HTTP CONNECT, SOCKS5 oder SSH und gib Server, Port und Zugangsdaten ein. Speichere und prüfe die Angaben, wähle einen Knoten und klicke auf „应用代理“ (Proxy anwenden). Mehrere gespeicherte Knoten lassen sich manuell wechseln. [Netzwerk](docs/guide/network.md)

### 2. Die Umgebung starten

Wähle beim ersten Mal den Speicherort für den Debian-13-Gast und starte die Installation. Fortschritt, aktuelle Schritte und Fehler erscheinen in den Vorgangsdetails. Spätere Starts verwenden dieselbe Umgebung.

### 3. Ein Projekt öffnen und Claude starten

Wähle mit „打开项目“ (Projekt öffnen) einen lokalen Ordner, öffne ein Terminal und gib `claude` ein. Autorisiere dein eigenes Konto im Workbench-Browser. Du kannst auch zunächst den Standardarbeitsordner verwenden. [Erste Schritte](docs/getting-started.md)

## Diese Grenzen solltest du kennen

- Änderungen und Löschungen in freigegebenen Projekten betreffen die Originaldateien auf dem Host, einschließlich Git-Operationen. Nutze Versionsverwaltung oder Sicherungen und prüfe Commits.
- Projekte desselben Claude-Eden-Profils teilen den Debian-13-Gast, dessen Werkzeuge, Konten und Dateisystem. Die Projekte sind nicht voneinander isoliert.
- Öffentlicher ausgehender Verkehr unterstützt IPv4 TCP und DNS über denselben Proxy. Öffentliches UDP, IPv6, ICMP und direkte Host- oder LAN-Zugriffe sind eingeschränkt. Ausdrücklich aktiviertes Tailscale folgt seinen privaten Netzwerkberechtigungen.
- Prüfungen zeigen die aktuelle Konfiguration und erfasste Nachweise. Sie garantieren keine dauerhafte Kontoverfügbarkeit und ersetzen keine vollständige Sicherheitsprüfung der Virtualisierung.

Weitere Informationen: [Netzwerk- und Datengrenzen](docs/getting-started.md#网络与数据边界).

## Dokumentation und Beiträge

Das ausführliche [Benutzerhandbuch](docs/guide/index.md) ist nach Aufgaben gegliedert und derzeit auf Chinesisch verfügbar.

| Aufgabe | Einstieg |
|---|---|
| Per Agent installieren, konfigurieren und Fehler untersuchen | [Agent-Handbuch](docs/guide/agent.md) |
| Installieren, Proxy einrichten und ein Projekt öffnen | [Erste Einrichtung](docs/guide/installation.md) |
| Debian 13 und Linux-Werkzeuge verwenden | [Debian-13-Gast](docs/guide/guest-systems.md) |
| Konten, Nutzungslimits und Terminals verwalten | [Konten](docs/guide/accounts.md) |
| Einstellungen und ihren Wirkungszeitpunkt verstehen | [Einstellungen](docs/guide/settings.md) |
| Speicher, Ressourcen und Updates verwalten | [Wartung](docs/guide/maintenance.md) |
| Download-, Start- und Netzwerkprobleme untersuchen | [Fehlersuche](docs/guide/troubleshooting.md) |
| Projekte, Terminals und Dateien bedienen | [Workbench](docs/guide/workbench.md) |
| Ein reproduzierbares Problem melden | [GitHub Issues](https://github.com/CarterLeeAlt/Claude-Eden/issues) |
| Code oder Dokumentation beitragen | [Beitragsleitfaden](.github/CONTRIBUTING.md) |

Das Projekt wird aktiv weiterentwickelt. Rückmeldungen und Beiträge sind über dieses Repository willkommen; beachte den [Beitragsablauf](.github/CONTRIBUTING.md).

## Quellen und Lizenzen

Upstream-Projekte, ursprüngliche Autoren und Urheberrechtshinweise stehen in [NOTICE](NOTICE), die MIT-Projektlizenz in [LICENSE](LICENSE). Mitgelieferte Komponenten behalten ihre eigenen Lizenzen. Mihomo wird als separates Programm unter GPL-3.0 verteilt. Siehe auch [Drittanbieterlizenzen](THIRD_PARTY_LICENSES) und [noVNC](ui/vendor/novnc/LICENSE.txt).

Claude Eden ist ein unabhängiges Community-Projekt ohne Zugehörigkeit zu Anthropic.

## Referenzen und Dank

- [agent-vm](https://github.com/sylvinus/agent-vm): Grundlage der Weiterentwicklung.
- [Lima](https://github.com/lima-vm/lima): virtuelle Maschinen und Linux-Gäste.
- [gvisor-tap-vsock](https://github.com/containers/gvisor-tap-vsock): Netzwerk für virtuelle Maschinen.
- [sshocker](https://github.com/lima-vm/sshocker) und [SFTP](https://github.com/pkg/sftp): Dateifreigaben zwischen Host und Gast.
- [noVNC](https://github.com/novnc/noVNC): entfernte Anzeige und Eingabe.
- [Mihomo](https://github.com/MetaCubeX/mihomo): Proxy-Kern.
- [code-server](https://github.com/coder/code-server): Editor im Browser.
- [AIO Sandbox](https://github.com/agent-infra/sandbox): Referenz für Arbeitsumgebung und Bedienung.
- [claude-switcher](https://github.com/YOURSJEYO/claude-switcher) und [Claude-Guardian](https://github.com/DUNHKpcc/Claude-Guardian): Referenzen für Netzwerkprüfungen und Diagnose.
- [claude-post-ban-cleanup](https://github.com/l2461090/claude-post-ban-cleanup): Referenz für Bestandsaufnahme, Datenschutz und wiederherstellbare Cache-Bereinigung; dieses Projekt implementiert die Abläufe eigenständig.

<a href="https://www.star-history.com/?repos=CarterLeeAlt%2FClaude-Eden&amp;type=date"><picture><source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date&amp;theme=dark" /><source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date" /><img alt="GitHub-Sterne von Claude Eden" src="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date" width="800" /></picture></a>
