<p align="center">
  <img src="frontend/assets/app.svg" width="100" height="100" alt="KiroX">
</p>

<h1 align="center">KiroX | kiro协议注册机</h1>

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

## 简介

KiroX 是一款基于 [Wails v2](https://wails.io) 构建的 Kiro 桌面注册工具，采用 HTTP/TLS 协议完成 AWS Builder ID 账号注册、邮箱验证、授权和 Kiro Token 交换。项目支持 Outlook 邮箱池、iCloud 消息列表取件、MoeMail 临时邮箱、MailNest 临时邮箱以及自部署的 Cloud-Mail，并提供批量任务、并发控制和代理管理。

---

## 特别鸣谢

<p align="center">
  <a href="https://www.ipwo.net/?ref=githubKiroX" target="_blank">
    <img src="docs/ipwo.webp" alt="IPWO" width="800">
  </a>
</p>

<p align="center">
  <b><a href="https://www.ipwo.net/?ref=githubKiroX" target="_blank">IPWO</a></b> 提供覆盖 195+ 国家和地区的住宅代理 IP，支持 HTTP、HTTPS 及 SOCKS5 协议。<br>
  适用于 Kiro、AI Coding、浏览器自动化及海外网络访问，可根据不同地区和业务需求选择相应的网络环境。<br>
  支持免费测试，专属折扣码：<code>0205</code>
</p>

---

## 功能特性

**Kiro 注册流程**

- 协议注册流程（OIDC 注册 → 设备授权 → 邮箱验证 → 密码设置 → SSO → Kiro Token 交换）
- 注册完成后自动验证账号存活状态
- 支持批量注册、停止任务，可配置数量、并发数和串行任务间隔；同一时间运行一批任务

**邮箱支持**

- **Outlook 邮箱池**：支持 IMAP / Microsoft Graph 取码，导入时可指定模式，默认 IMAP
- **iCloud 邮箱**：导入 `邮箱----消息列表URL`，通过兼容的消息列表页面获取验证码
- **MoeMail 临时邮箱**：支持多域名配置，自动轮换，支持随机/全部/指定域名模式
- **Cloud-Mail 自部署邮箱**：对接 [cloud-mail](https://github.com/jiangrungen/cloud-mail) 服务，域名可自动从服务器拉取，支持随机/轮询/指定模式
- **MailNest-迈巢**：对接 [MailNest-迈巢](https://mailnest.top/) 服务，使用 Outlook 临时邮箱

**纯协议与网络**

- 基于 `tls-client` 的 HTTP/TLS 客户端与请求参数配置

**数据管理**

- 注册成功的账号以明文 JSON 写入可配置的输出目录
- Outlook / iCloud 邮箱池以 JSON 形式本地存储
- 支持自定义数据目录和结果输出目录

**代理**

- 独立「IP 管理」页面，支持添加、批量导入、测试、启停、删除代理及查看出口 IP、位置和延迟
- 新建任务时选择已启用的代理或直连，注册请求使用本次任务所选代理
- 支持 HTTP / HTTPS / SOCKS5
- 支持 `协议://用户:密码@host:port` 格式，批量导入时每行一个代理 URL

**界面与监控**

- 概览、运行日志、邮箱池、IP 管理、关于、设置六个页面
- 中 / 英 / 日语言切换，浅色 / 深色主题，任务完成提示音
- 轮询更新任务统计和运行日志

**版本更新**

- 检查 GitHub Releases 最新版本（语义化版本比较）
- 通过 Releases 页面手动下载并安装新版本

---


## 快速开始

### 直接使用

从 [Releases](https://github.com/huey1in/kirox/releases/latest) 下载匹配系统和 CPU 架构的安装包：Windows 使用 `.exe`，macOS 使用 `.dmg`，Debian / Ubuntu Linux 使用 `.deb`。macOS 打开 DMG 后将 KiroX 拖入 Applications；Linux 可运行 `sudo apt install ./kirox-linux-amd64.deb` 安装。

### 从源码构建

**环境要求**

- Go 1.24.1+（`go.mod` 指定工具链 Go 1.24.4）
- Node.js 20+
- Wails CLI v2.11.0
- 对应系统的桌面依赖，可用 `wails doctor` 检查；Windows 需要 WebView2，Linux 需要 GTK3 和 WebKitGTK 开发库

```bash
# 安装 Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0

# 克隆仓库
git clone https://github.com/huey1in/kirox.git
cd kirox

# 检查桌面环境依赖
wails doctor

# 准备构建图标（与发布流程一致）
node -e "require('fs').copyFileSync('frontend/assets/appicon.png', 'build/appicon.png')"

# 开发模式
wails dev

# 生产构建
wails build

# Windows 安装包（需要 NSIS 3）
wails build -nsis
```

构建产物位于 `build/bin/`。Linux 使用 WebKitGTK 4.1 时，需安装 `libgtk-3-dev` 和 `libwebkit2gtk-4.1-dev`（Debian / Ubuntu 包名），并使用 `wails dev -tags webkit2_41` 或 `wails build -tags webkit2_41`。

前端使用 `d3@7.9.0` 绘制指纹偏移曲线。`node frontend/build.js` 会将静态资源和所需依赖整理到 `frontend/dist/`；前端的 `npm run dev` / `npm run build` 都执行这一构建脚本，不会启动独立 Web 服务。完整功能依赖 Wails 提供的 Go 方法绑定，应通过 `wails dev` 运行桌面应用。

---

## 使用说明

### 1. 配置邮箱

**Outlook 邮箱池**（推荐）

在「邮箱池」页面导入账号，每行一条，格式：

```text
邮箱----密码----客户端ID----RefreshToken
邮箱----密码----客户端ID----RefreshToken----graph
```

第五段可选，支持 `imap` / `graph`，省略时默认 `imap`。支持从 `.txt` / `.csv` 文件批量导入，也可手动粘贴；文件内容仍使用上述 `----` 分隔格式。

**iCloud 邮箱**

在「邮箱池」的 iCloud 区域添加账号，可粘贴或从 `.txt` / `.csv` 文件导入，每行格式为：

```text
邮箱----消息列表URL
```

需要预先取得兼容的消息列表 URL，程序通过该页面读取邮件和验证码，不使用普通 iCloud 网页登录。

**MoeMail 临时邮箱**

在「邮箱池」页面添加 MoeMail 配置，填入 API 地址和 API Key，测试连接后保存。注册时可选择随机域名、全部域名或指定域名。

**Cloud-Mail 自部署邮箱**

在「邮箱池」页面添加 Cloud-Mail 配置，填入服务地址、管理员邮箱和密码。测试连接或保存时会自动从服务器拉取可用域名，无需手动填写；创建任务时可选择随机、全部或指定域名。

**MailNest-迈巢 Outlook 临时邮箱**

在「邮箱池」页面添加 MailNest 配置，填入 API Key 和项目代码，通过测试连接按钮可以获取当前账户的余额，测试通过后点击添加配置按钮完成配置，即可使用。

- `API Key`：从 [账户页面](https://mailnest.top/account) 获取。
- 项目代码：根据 [购买邮箱页面](https://mailnest.top/buy-email) 的项目填写。输入框中的 `aws001` 是示例提示，项目代码需要手动填写。

### 2. 启动注册

在「概览」页面点击「新建任务」：

1. 设置注册数量、并发数和延迟（秒）。延迟仅用于串行模式下两项任务之间的等待。
2. 选择邮箱来源；MoeMail / Cloud-Mail 还需选择域名模式。
3. 选择本次任务的代理，默认直连；代理需先在「IP 管理」添加并启用。
4. 点击「开始注册」，在概览查看统计，在「运行日志」查看过程；需要结束时点击「停止」。

### 3. 查看结果

注册成功的账号写入结果输出目录，默认位置为 `~/Documents/KiroX/accounts.json`，可在「设置」中修改。保存格式如下（令牌和额度均为示例）：

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

同一邮箱再次保存时覆盖旧记录。`creditUsed` / `creditLimit` 来自注册后的验证结果，可能缺失或为 `null`。结果文件不包含注册密码和访问令牌，失败记录仅保留在运行日志中。

安装版的运行时文件默认位于 `%LOCALAPPDATA%\KiroX`：`settings.json` 保存任务默认值、网络策略、界面与高级配置，`data` 保存邮箱池、邮箱服务配置和代理池，`cache` 保存可重建的浏览器指纹缓存，`logs` 保存可选的脱敏运行日志。可在「设置」中更改业务数据目录；缓存和设置仍固定在本机应用数据目录。旧版 `%APPDATA%\kirox` 数据会在首次启动时复制到新结构，源文件保留且不会覆盖已有目标文件。

业务数据目录和结果输出目录中都有一个 `accounts.json`，两者用途与格式不同，应分开设置。修改结果目录不会迁移已有结果文件。

### 4. 代理配置

在「IP 管理」页面添加代理，可单个填写协议、主机、端口和认证信息，也可批量导入地址，例如：

```text
http://user:pass@host:port
socks5://host:port
http://host:8080
```

添加后可测试出口 IP 和延迟，并启用或停用代理。新建任务时从下拉列表选择一个已启用代理；选择「直连」则不为注册请求使用代理。

---

## 项目结构

```
kirox/
├── main.go                    # 入口，Wails 初始化
├── app.go                     # App 结构体，Wails 绑定方法
├── internal/
│   ├── core/                  # Kiro 注册核心逻辑
│   │   ├── registrar.go       # Registrar、HTTP 客户端、初始授权步骤
│   │   ├── run.go             # 步骤编排
│   │   ├── auth.go            # SSO 工作流、令牌获取
│   │   ├── signup_flow.go     # 注册流程、邮箱验证码
│   │   ├── signup_password.go # 身份创建、密码设置
│   │   ├── kiro_auth.go       # Kiro 授权
│   │   ├── kiro_exchange.go   # 步骤 15
│   │   └── verify.go          # 账号验证
│   ├── browser/               # 协议请求身份参数生成
│   ├── email/                 # Outlook / iCloud / MoeMail / Cloud-Mail / MailNest
│   ├── crypto/                # JWE 加密、XXTEA
│   ├── storage/               # 账号存储、配置持久化
│   ├── task/                  # 批量任务调度、并发控制
│   ├── data/                  # 注册结果读写
│   ├── proxy/                 # 代理池持久化、出口 IP / 归属检测
│   ├── updater/               # 版本检查
│   └── http/                  # TLS 客户端工具
└── frontend/
    ├── index.html             # 单页应用入口
    ├── js/                    # app / ui / task / overview / accounts / ip /
    │                          # moemail / cloudmail / mailnest / dropdown / i18n
    ├── css/                   # layout / components / style / dashboard
    └── build.js               # 静态资源复制到 dist/
```

---

## 技术栈

| 层 | 技术 |
|----|------|
| 桌面框架 | [Wails v2.11.0](https://wails.io) |
| 后端语言 | Go 1.24.1+，工具链 1.24.4 |
| HTTP 客户端 | [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) |
| 前端 | 原生 HTML / CSS / JavaScript |
| 加密 | RSA-OAEP-256 + AES-256-GCM (JWE) |

前端通过 `window.go.main.App` 调用 Go 方法，定时读取任务状态和日志。当前版本是 Wails 桌面应用，尚未提供独立 WebUI 或 2API 服务。

---

## 后续规划

KiroX 正准备进行一次较大规模的架构重构，后续方向包括：

- **从桌面 GUI 走向 WebUI**：逐步拆分前后端，支持浏览器访问、服务端部署和更多使用场景。
- **内置 2API 能力**：计划将 AWS CodeWhisperer 能力适配为标准 OpenAI API 与 Anthropic API 端点，方便接入现有客户端、工作流和开发工具。
- **注册任务与账号管理升级**：统一任务编排、账号管理、凭证生命周期和运行监控。
- **更清晰的服务化边界**：为后续扩展更多模型适配、代理策略和自动化能力做准备。

以上方向会根据实际开发进度逐步落地，当前版本仍以现有桌面端体验为主。

---

## 注意事项

- 本工具仅供学习和研究使用，请遵守 AWS 服务条款
- 建议配合代理使用，避免 IP 被限速
- Outlook 账号需提前准备好有效的 RefreshToken
- 并发数过高可能触发 AWS 风控，建议从低并发开始测试

---

## 常见问题

### IP 纯净度相关

如果运行中出现下面这两类报错，多半是当前出口 IP 不够纯净（代理 IP 已被 AWS / Microsoft 风控）。

**情况一：发送邮箱验证码响应 OTP 400**

![情况一](docs/images/1.png)
![情况一](docs/images/3.png)

建议更换更干净的住宅代理。

> 如果使用的是自建邮箱或一次性邮箱（MoeMail 等），OTP 400 也可能是邮箱域名已被 Microsoft / AWS 拉黑导致；可换一个域名再试。

**情况二：注册流程直接卡住或邮箱无法访问**

![情况二](docs/images/2.png)

此时先用本机浏览器（带相同代理）尝试打开 [outlook.live.com](https://outlook.live.com)：

- 如果浏览器都打不开 / 跳验证码 → 当前 IP 已被 Microsoft 风控，需要换代理
- 如果浏览器能正常访问 → 检查 Outlook 账号的 RefreshToken 是否仍然有效

### macOS 提示「应用已损坏，无法打开」

未签名的应用首次运行时会被 macOS Gatekeeper 拦截。在终端执行下面的命令移除下载隔离标记即可正常打开：

```bash
xattr -cr /path/to/KiroX.app
```

将 `/path/to/KiroX.app` 替换成实际路径（例如把 `KiroX.app` 拖入终端可自动填入）。

---

## 交流

- QQ 交流群：[点击加入](https://qm.qq.com/q/RXMTXUlc4w)

---

## 作者

**1in** · [@huey1in](https://github.com/huey1in)

Copyright © 2026

---

## 开源协议

本项目基于 [Apache License 2.0](LICENSE) 开源。

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
