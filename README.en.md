<p align="center">
  <img src="assets/kirox-light.svg" width="100" height="100" alt="KiroX">
</p>

<h1 align="center">KiroX | Kiro Protocol Registration Tool</h1>

<p align="center">
  <a href="README.md">简体中文</a> ·
  <a href="README.en.md">English</a> ·
  <a href="README.ja.md">日本語</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-v2.0.0-6366f1?style=flat-square" alt="version">
  <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Linux-0078d4?style=flat-square" alt="platform">
  <img src="https://img.shields.io/badge/C%2B%2B-20-00599C?style=flat-square" alt="C++20">
  <img src="https://img.shields.io/badge/Qt-6.8.3-41CD52?style=flat-square" alt="Qt">
  <img src="https://img.shields.io/badge/license-Apache%202.0-green?style=flat-square" alt="license">
</p>

---

## Overview

KiroX is a native C++20 and Qt Quick desktop registration tool. It uses HTTP/TLS protocols for AWS Builder ID signup, email verification, authorization and Kiro token exchange. Outlook, iCloud, MoeMail, Cloud-Mail and MailNest are supported, with concurrent batches and proxy pools. The Liquid Glass interface offers Chinese, English and Japanese, light/dark themes and reduced effects.

---

## Sponsors & Acknowledgments

<p align="center">
  <a href="https://proxylane.dev/?utm_source=kirox&amp;utm_medium=partnership&amp;utm_campaign=kirox_sponsor_202610&amp;utm_content=github_readme" target="_blank">
    <img src="docs/proxylane_kirox_1600x320.png" alt="ProxyLane residential proxies — 30% off your first purchase with KIROX30" width="800">
  </a>
</p>

<p align="left">
<b><a href="https://proxylane.dev/?utm_source=kirox&amp;utm_medium=partnership&amp;utm_campaign=kirox_sponsor_202610&amp;utm_content=github_readme" target="_blank">ProxyLane</a></b> residential proxies cover 195 countries, with 95%+ of IPs rated low risk by Scamalytics. Target by city, ISP, or ASN, with sticky sessions lasting up to 72 hours.<br>
For KiroX batch imports, enter one <code>protocol://user:password@host:port</code> per line, using a separate session name and a fixed residential IP for each account.<br>
The website offers a Chinese interface and accepts USDT payments. Use <code>KIROX30</code> for <b>30% off your first purchase</b>.
</p>

<p align="center">
  <a href="https://www.ipwo.net/?ref=githubKiroX" target="_blank">
    <img src="docs/ipwo.webp" alt="IPWO" width="800">
  </a>
</p>

<p align="left">
<b><a href="https://www.ipwo.net/?ref=githubKiroX" target="_blank">IPWO</a></b> provides residential proxy IPs covering 195+ countries and regions, supporting HTTP, HTTPS, and SOCKS5 protocols.<br>
It is suitable for Kiro, AI Coding, browser automation, and overseas network access, letting you choose the network environment that suits your region and business needs.<br>
Free trial supported. Exclusive discount code: <code>0205</code>
</p>

---

## Features

**Kiro registration flow**
- Protocol-based registration flow (OIDC signup → device authorization → email verification → password setup → SSO → Kiro token exchange)
- Liveness check on each account after registration
- Batch mode with configurable count and concurrency; the delay applies between registrations in serial mode
- Create batches on Tasks, monitor progress in Logs, and stop the active batch

**Email sources**
- **Outlook mailbox pool** — import accounts in `email----password----clientID----RefreshToken[----imap/graph]` format; supports IMAP and Microsoft Graph, with IMAP as the default
- **MoeMail disposable mail** — multi-domain configurations with auto-rotation; random / all / specific domain modes
- **Cloud-Mail (self-hosted)** — integrates with [cloud-mail](https://github.com/jiangrungen/cloud-mail); domains can be pulled from the server automatically; random / round-robin / specific modes
- **MailNest temporary mail** — configure an API key and project code, with a connection and balance check before saving
- **iCloud mailbox pool** — import `email----messages URL` entries; verification codes are fetched from a compatible message-list page

**Pure protocol and networking**
- HTTP/TLS client and request parameter configuration via `libcurl-impersonate`

**Data management**
- Successful accounts written as plain JSON to a configurable output directory
- Mailbox pool entries stored locally as JSON
- Custom data directory and result directory supported

**Proxy**
- Manage a proxy pool on the IPs page, including batch import, testing, enabling / disabling, and deletion
- Select an enabled HTTP / HTTPS / SOCKS5 proxy for a registration batch, or use a direct connection
- Use standard proxy URLs such as `scheme://user:pass@host:port`

**Desktop interface**
- Chinese, English, and Japanese interfaces with light and dark themes
- Overview statistics and live registration logs

**Version updates**
- Checks the latest GitHub Release (semantic-version comparison)
- Opens the Releases page for manual download and installation

---

## Roadmap

The native C++ modules provide the foundation for these future directions:

- **Move from desktop GUI toward WebUI**: gradually separate the frontend and backend to support browser access, server deployment, and broader usage scenarios.
- **Built-in 2API support**: adapt AWS CodeWhisperer capabilities to standard OpenAI API and Anthropic API endpoints, making them easier to use with existing clients, workflows, and development tools.
- **Upgrade registration tasks and account management**: unify task orchestration, account management, credential lifecycle, and runtime monitoring.
- **Clearer service boundaries**: create room for more model adapters, proxy strategies, and automation capabilities.

These directions will be delivered incrementally as development progresses. The future branch now provides a native Qt Quick desktop experience.

---

## Quick start

The `future` branch uses the native C++ architecture. Download branch builds from [Native C++ Actions](https://github.com/huey1in/KiroX/actions/workflows/native.yml), or published versions from [Releases](https://github.com/huey1in/KiroX/releases). Extract the Windows ZIP and run `bin/kirox.exe`; extract the Linux TGZ and run `bin/kirox`; on macOS open the DMG and use `KiroX.app`.

Windows source builds use Python 3.11+ to provision development tools:

```powershell
git clone --branch future https://github.com/huey1in/KiroX.git
cd KiroX
powershell -ExecutionPolicy Bypass -File scripts/bootstrap-cpp.ps1
powershell -ExecutionPolicy Bypass -File scripts/build-cpp.ps1 -Preset release -Deploy
out/package/bin/kirox.exe
```

The packaged app needs no Go, Node.js, Python or WebView2. See the [development guide](docs/cpp-development.md) for macOS/Linux dependencies, CMake builds and verification.

---

## Usage

### 1. Configure email

**Outlook mailbox pool** (recommended)

On the Emails page, import accounts, one per line:
```
email----password----clientID----RefreshToken----imap
email----password----clientID----RefreshToken----graph
```
The fifth field is optional and defaults to `imap`. Batch import from `.txt` / `.csv` files is supported; you can also paste manually.

**MoeMail disposable mail**

On the Mail services page, add a MoeMail configuration with its API URL and API key, test the connection, and save. During registration you can pick random, all, or specific domains.

**Cloud-Mail (self-hosted)**

On the Mail services page, add a Cloud-Mail configuration with its base URL, admin email, and password. Use Check to retrieve domains, or enter an optional domain list. Empty domain selections use the service domain list. During registration, choose random, round-robin, or a specific domain.

**MailNest temporary mail**

On the Mail services page, enter the MailNest API key and project code. Both are required; `aws001` is only an example placeholder, not a default project code. Use Check after saving to inspect the connection and balance.

**iCloud mailbox pool**

In the iCloud section of the Emails page, import one entry per line:

```text
email----messages URL
```

Paste entries or import a `.txt` / `.csv` file. Each URL must provide a compatible message-list page from which KiroX can read verification emails. A normal iCloud web login URL is not sufficient.

### 2. Start registration

Create a batch on the Tasks page:
- Set the count, concurrency (1–5 recommended), and delay in seconds; the delay only applies between registrations when concurrency is 1
- Choose the email source and its available domain options
- Select an enabled proxy or a direct connection for this batch
- Click "Start" and follow progress on the Logs page; use "Stop" to stop the active batch

Only one batch can run at a time.

### 3. View results

Successful accounts are streamed to the output directory (default `~/Documents/KiroX`) as `accounts.json`:

```json
[
  {
    "refreshToken": "...",
    "provider": "BuilderId",
    "clientId": "...",
    "clientSecret": "...",
    "region": "us-east-1",
    "email": "xxx@outlook.com",
    "time": "2026-09-05 12:00:00",
    "creditUsed": 0,
    "creditLimit": 0
  }
]
```

The credit values above are examples; `creditUsed` and `creditLimit` depend on the account check and may be absent or `null`. A new successful record replaces the previous record for the same email. Passwords and access tokens are not written to this file; failed or banned accounts remain in the logs.

Installed builds keep runtime files under `%LOCALAPPDATA%\KiroX` by default. `settings.json` stores task defaults, network policies, interface settings; `data` stores mailbox pools, mail service settings, and the proxy pool; `cache` stores rebuildable browser identity data; and `logs` stores optional redacted runtime logs. The business data directory can be changed in Settings, while settings and cache remain in local app data. On first launch, data from the old `%APPDATA%\kirox` layout is copied into the new layout without deleting sources or overwriting existing destination files.

Both the business data directory and result output directory contain a file named `accounts.json`, but they have different formats and purposes, so keep the directories separate. Changing the result directory does not move existing result files.

### 4. Proxy

On the IPs page, add proxies individually or in batches, test them, and enable those you want to use. Supported formats include:
```
http://user:pass@host:port
socks5://host:port
http://host:8080
```
In the New task dialog, choose an enabled proxy or a direct connection. Registration requests use the selected proxy for that batch.

---

## Project layout

```text
include/kirox/
  domain/          # Value types and validation
  application/     # Use cases and orchestration
  ports/           # Adapter contracts
  infrastructure/  # Adapter public interfaces
src/
  domain/          # Identity, fingerprints, MIME, settings
  application/     # Registration, batch, mailbox/proxy/account services
  infrastructure/  # Atomic JSON, native curl/IMAP, CNG/OpenSSL, logs
  desktop/         # Composition root and queued UI controller
ui/                # Native Qt Quick pages and glass controls
assets/            # SVG logo, platform icons and completion sound
cmake/             # Pinned native dependency setup
scripts/           # Bootstrap, builds and isolated UI verification
tests/             # Unit, local protocol and interoperability fixtures
packaging/         # Platform integration
```

[Architecture](docs/cpp-architecture.md) · [Development](docs/cpp-development.md) · [Migration evidence](docs/cpp-migration.md)

---

## Tech stack

| Layer | Technology |
|-------|------------|
| Language | C++20 |
| Desktop | Qt 6.8.3 / Qt Quick / QML |
| Build | CMake / Ninja / CPack |
| HTTP / IMAP | Native libcurl-impersonate 2.2.3 |
| Cryptography | Windows CNG / OpenSSL 3; RSA-OAEP-256 + AES-256-GCM |
| Verification | Qt Test / local protocol fixtures / independent interoperability |

Modules depend on explicit contracts. QML owns layout and interaction; C++ workers handle registration and scheduling. WebUI and 2API remain future extensions.

---

## Notes

- This tool is intended for learning and research. Comply with the AWS Terms of Service.
- A proxy is strongly recommended to avoid IP rate limits.
- Outlook accounts require a valid RefreshToken prepared in advance.
- High concurrency may trip AWS risk control — start low and ramp up.

---

## FAQ

### IP cleanliness

If you hit either of the errors below, the egress IP is likely flagged by AWS / Microsoft.

**Case 1: OTP 400 on email verification send**

![Case 1](docs/images/1.png)
![Case 1](docs/images/3.png)

Switch to a cleaner residential proxy.

> When using a self-hosted or disposable mailbox (MoeMail, etc.), OTP 400 can also mean the email *domain* is blacklisted by Microsoft / AWS — try a different domain.

**Case 2: Registration stalls or the mailbox is unreachable**

![Case 2](docs/images/2.png)

Try opening [outlook.live.com](https://outlook.live.com) in a real browser using the same proxy:

- Browser also fails / shows CAPTCHA → the IP is blocked by Microsoft; change the proxy
- Browser works → verify the Outlook account's RefreshToken is still valid

### macOS: "App is damaged and can't be opened"

Unsigned apps are blocked by Gatekeeper on first launch. Remove the quarantine attribute in a terminal:

```bash
xattr -cr /path/to/KiroX.app
```

Replace `/path/to/KiroX.app` with the real path (you can drag `KiroX.app` into the terminal to fill it in).

---

## Community

- QQ group: [join](https://qm.qq.com/q/RXMTXUlc4w)

---

## Author

**1in** · [@huey1in](https://github.com/huey1in)

Copyright © 2026

---

## License

Released under the [Apache License 2.0](LICENSE).

```
Copyright 2026 1in

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```

---

## Star History

[![Star History Chart](https://star-history.dera.page/svg?repos=huey1in/kirox&type=Date)](https://star-history.dera.page/#huey1in/kirox&Date)
