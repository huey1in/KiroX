<p align="center">
  <img src="frontend/assets/app.svg" width="100" height="100" alt="KiroX">
</p>

<h1 align="center">KiroX | Kiro Protocol Registration Tool</h1>

<p align="center">
  <a href="README.md">简体中文</a> ·
  <a href="README.en.md">English</a> ·
  <a href="README.ja.md">日本語</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-v1.0.4-6366f1?style=flat-square" alt="version">
  <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Linux-0078d4?style=flat-square" alt="platform">
  <img src="https://img.shields.io/badge/Go-1.24-00ADD8?style=flat-square&logo=go" alt="go">
  <img src="https://img.shields.io/badge/Wails-v2-red?style=flat-square" alt="wails">
  <img src="https://img.shields.io/badge/license-Apache%202.0-green?style=flat-square" alt="license">
</p>

---

## Overview

KiroX is a Kiro registration tool built on [Wails v2](https://wails.io) and implemented entirely over HTTP/TLS protocols. It completes account registration, email verification, authorization, and Kiro token exchange directly through protocol requests. It supports Outlook, MoeMail, Cloud-Mail, MailNest, and iCloud email sources, with batch scheduling, concurrency control, proxy management, and live logs.

---

## Special Thanks

<p align="center">
  <a href="https://www.ipwo.net/?ref=githubKiroX" target="_blank">
    <img src="docs/ipwo.webp" alt="IPWO" width="800">
  </a>
</p>

<p align="center">
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
- Create tasks from Overview, monitor progress in Logs, and stop the active batch

**Email sources**
- **Outlook mailbox pool** — import accounts in `email----password----clientID----RefreshToken[----imap/graph]` format; supports IMAP and Microsoft Graph, with IMAP as the default
- **MoeMail disposable mail** — multi-domain configurations with auto-rotation; random / all / specific domain modes
- **Cloud-Mail (self-hosted)** — integrates with [cloud-mail](https://github.com/jiangrungen/cloud-mail); domains can be pulled from the server automatically; random / round-robin / specific modes
- **MailNest temporary mail** — configure an API key and project code, with a connection and balance check before saving
- **iCloud mailbox pool** — import `email----messages URL` entries; verification codes are fetched from a compatible message-list page

**Pure protocol and networking**
- HTTP/TLS client and request parameter configuration via `tls-client`

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

KiroX is preparing for a substantial architecture refactor. Planned directions include:

- **Move from desktop GUI toward WebUI**: gradually separate the frontend and backend to support browser access, server deployment, and broader usage scenarios.
- **Built-in 2API support**: adapt AWS CodeWhisperer capabilities to standard OpenAI API and Anthropic API endpoints, making them easier to use with existing clients, workflows, and development tools.
- **Upgrade registration tasks and account management**: unify task orchestration, account management, credential lifecycle, and runtime monitoring.
- **Clearer service boundaries**: create room for more model adapters, proxy strategies, and automation capabilities.

These directions will be delivered incrementally as development progresses. The current release remains focused on the existing Wails desktop experience.

---

## Quick start

### Use a release

Download the archive matching your operating system and architecture from [Releases](https://github.com/huey1in/kirox/releases/latest), extract it, and run the application. Releases use `.zip` or `.tar.gz` archives; for example, Windows x64 uses `kiro-reg-windows-amd64.exe.zip`.

### Build from source

**Requirements**
- Go 1.24.1+ (`go.mod` specifies the Go 1.24.4 toolchain)
- Node.js 20+
- Wails CLI v2.11.0 and the platform dependencies reported by `wails doctor`

```bash
# Install the Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0

# Clone
git clone https://github.com/huey1in/kirox.git
cd kirox

# Check the local environment
wails doctor

# Prepare the application icon
node -e "require('fs').copyFileSync('frontend/assets/appicon.png', 'build/appicon.png')"

# Dev mode
wails dev

# Production build
wails build
```

The output binary is located under `build/bin/`.

On Linux, install the required GTK 3 and WebKitGTK development packages. If your system uses WebKitGTK 4.1, run `wails dev -tags webkit2_41` or `wails build -tags webkit2_41`.

The frontend uses vanilla HTML / CSS / JavaScript with no third-party npm dependencies. Wails runs its build command automatically; `node frontend/build.js` only copies static assets into `frontend/dist/` and does not start a web server. Application functions require the Wails bridge, so use `wails dev` to run the desktop app.

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

On the Emails page, add a MoeMail configuration with its API URL and API key, test the connection, and save. During registration you can pick random, all, or specific domains.

**Cloud-Mail (self-hosted)**

On the Emails page, add a Cloud-Mail configuration with its base URL, admin email, and password. Test the connection and save. KiroX fetches domains automatically from `/api/setting/websiteConfig`; the configuration form has no domain input. During registration, choose random, round-robin, or a specific domain.

**MailNest temporary mail**

On the Emails page, enter the MailNest API key and project code. Both are required; `aws001` is only an example placeholder, not a default project code. Saving first tests the connection and checks the balance.

**iCloud mailbox pool**

In the iCloud section of the Emails page, import one entry per line:

```text
email----messages URL
```

Paste entries or import a `.txt` / `.csv` file. Each URL must provide a compatible message-list page from which KiroX can read verification emails. A normal iCloud web login URL is not sufficient.

### 2. Start registration

On the Overview page, click "New task" to open the registration dialog:
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

Installed builds keep runtime files under `%LOCALAPPDATA%\KiroX` by default. `settings.json` stores task defaults, network policies, interface settings, and advanced overrides; `data` stores mailbox pools, mail service settings, and the proxy pool; `cache` stores rebuildable browser identity data; and `logs` stores optional redacted runtime logs. The business data directory can be changed in Settings, while settings and cache remain in local app data. On first launch, data from the old `%APPDATA%\kirox` layout is copied into the new layout without deleting sources or overwriting existing destination files.

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

```
kirox/
├── main.go                    # Entry; Wails initialization
├── app.go                     # App struct; methods bound to Wails
├── internal/
│   ├── core/                  # Kiro registration core
│   │   ├── registrar.go       # Registrar; HTTP client; steps 1–5
│   │   ├── run.go             # Step orchestration
│   │   ├── auth.go            # SSO workflow; token retrieval
│   │   ├── signup_flow.go     # Signup flow; email verification codes
│   │   ├── signup_password.go # Identity creation; password setup
│   │   ├── kiro_auth.go       # Kiro authorization
│   │   ├── kiro_exchange.go   # Step 15
│   │   └── verify.go          # Liveness check
│   ├── browser/               # Protocol request identity parameters
│   ├── email/                 # Outlook / MoeMail / Cloud-Mail / MailNest / iCloud
│   ├── crypto/                # JWE encryption; XXTEA
│   ├── storage/               # Account storage; config persistence
│   ├── task/                  # Batch scheduling; concurrency
│   ├── data/                  # Result I/O
│   ├── proxy/                 # Proxy pool; connectivity / egress IP / geo detection
│   ├── updater/               # Version checks
│   └── http/                  # TLS-client helpers
└── frontend/
    ├── index.html             # Single-page entry
    ├── js/                    # overview / accounts / moemail / cloudmail / mailnest / task / ip / app / ui / i18n / dropdown
    ├── css/                   # layout / components / style / dashboard
    └── build.js               # Copies static assets to dist/
```

---

## Tech stack

| Layer | Technology |
|----|------|
| Desktop framework | [Wails v2.11.0](https://wails.io) |
| Backend | Go 1.24.1+; Go 1.24.4 toolchain |
| HTTP client | [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) |
| Frontend | Vanilla HTML / CSS / JavaScript |
| Crypto | RSA-OAEP-256 + AES-256-GCM (JWE) |

The frontend calls Go methods through `window.go.main.App` and polls task status and logs. The current release is a Wails desktop application; standalone WebUI and 2API services are not implemented yet.

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
