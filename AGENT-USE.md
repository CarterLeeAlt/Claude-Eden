# Use Claude Eden with an external Agent

Start with [the complete Agent guide](docs/guide/agent.md). It covers locating the installed CLI, release download and checksum verification, platform installation, accounts, proxy setup, guest installation, settings, job tracking and recovery.

Run `<installed Eden CLI> agent capabilities` to discover the protocol and supported input fields. The Windows executable is `resources/eden.exe`; the profile defaults to `~/.eden` and can be selected with `EDEN_CONFIG`. Every `agent` command returns JSON. The source guide may describe functionality absent from a published release; detect support before configuring it.

The Agent uses the same configuration, account checks, native operations and client preferences as the user interface. Use `agent routes` for the live HTTP route list, `agent request METHOD /api/PATH` for JSON or file operations, and `agent socket /api/PATH` for WebSocket NDJSON. Desktop configuration and selected workloads require an explicit scope and connection generation as described in the guide. Native credential operations stay inside the existing OS bridge.

Entry points: [Agent guide](docs/guide/agent.md), [llms.txt](www/public/llms.txt), [download helper](www/public/agent-bootstrap.mjs), [current repository](https://github.com/CarterLeeAlt/Claude-Eden). The application runs a local Debian 13 guest on a Windows 10 host. Guest Tailscale and proxy configuration are available. Detect the actual architecture, runtime and virtualization capabilities before installation.

`AGENTS.md` contains repository contributor instructions. Product users and their agents should follow the usage guide above.
