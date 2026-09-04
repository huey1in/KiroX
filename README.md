<p align="center">
  <img src="frontend/assets/appicon.png" width="100" height="100" alt="KiroX">
</p>

<h1 align="center">KiroX | kiro协议注册机</h1>

<p align="center">
  <a href="README.md">简体中文</a> ·
  <a href="README.en.md">English</a> ·
  <a href="README.ja.md">日本語</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-v1.0.3-6366f1?style=flat-square" alt="version">
  <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Linux-0078d4?style=flat-square" alt="platform">
  <img src="https://img.shields.io/badge/Go-1.24-00ADD8?style=flat-square&logo=go" alt="go">
  <img src="https://img.shields.io/badge/Wails-v2-red?style=flat-square" alt="wails">
  <img src="https://img.shields.io/badge/license-Apache%202.0-green?style=flat-square" alt="license">
</p>

---

## 简介

KiroX 是一款基于 [Wails v2](https://wails.io) 构建的 Kiro 注册机，采用纯 HTTP/TLS 协议实现。通过协议完成账号注册、邮箱验证、授权和 Kiro Token 交换。项目支持 Outlook 邮箱池、MoeMail 临时邮箱、MailNest 临时邮箱以及自部署的 Cloud-Mail，并提供并发控制和代理支持。

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
- 完整的 15 步协议注册流程（OIDC 注册 → 设备授权 → 邮箱验证 → 密码设置 → SSO → Kiro Token 交换）
- 注册完成后自动验证账号存活状态
- 支持批量注册，可配置数量、并发数和任务间隔

**邮箱支持**
- **Outlook 邮箱池**：导入 `邮箱----密码----客户端ID----RefreshToken` 格式账号，自动通过 IMAP 获取验证码
- **MoeMail 临时邮箱**：支持多域名配置，自动轮换，支持随机/全部/指定域名模式
- **Cloud-Mail 自部署邮箱**：对接 [cloud-mail](https://github.com/jiangrungen/cloud-mail) 服务，域名可自动从服务器拉取，支持随机/轮询/指定模式
- **MailNest-迈巢**：对接 [MailNest-迈巢](https://mailnest.top/) 服务，使用 Outlook 临时邮箱

**纯协议与网络**
- 基于 `tls-client` 的 HTTP/TLS 客户端与请求参数配置

**数据管理**
- 注册成功的账号以明文 JSON 写入可配置的输出目录
- Outlook 账号信息以 JSON 形式本地存储
- 支持自定义数据目录和结果输出目录

**代理**
- 全局代理配置，支持 HTTP / HTTPS / SOCKS5
- 支持 `协议://用户:密码@host:port` 或简写 `host:port:user:pass` 格式

**版本更新**
- 检查 GitHub Releases 最新版本（语义化版本比较）
- 通过 Releases 页面手动下载并安装新版本

---


## 快速开始

### 直接使用

从 [Releases](https://github.com/huey1in/kirox/releases/latest) 下载最新的 `kirox.exe`，双击运行即可。

### 从源码构建

**环境要求**
- Go 1.24+
- Node.js 20+
- Wails CLI

```bash
# 安装 Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# 克隆仓库
git clone https://github.com/huey1in/kirox.git
cd kirox

# 开发模式（热重载）
wails dev

# 生产构建
wails build
```

构建产物位于 `build/bin/`。

---

## 使用说明

### 1. 配置邮箱

**Outlook 邮箱池**（推荐）

在「邮箱池」页面导入账号，每行一条，格式：
```
邮箱----密码----客户端ID----RefreshToken
```
支持从 `.txt` / `.csv` 文件批量导入，也可手动粘贴。

**MoeMail 临时邮箱**

在「邮箱池」页面添加 MoeMail 配置，填入 API 地址和 API Key，测试连接后保存。注册时可选择随机域名、全部域名或指定域名。

**MailNest-迈巢 Outlook 临时邮箱**

在「邮箱池」页面添加 MailNest 配置，填入 API Key 和项目代码，通过测试连接按钮可以获取当前账户的余额，测试通过后点击添加配置按钮完成配置，即可使用。

- `api-key`：获取页面为 https://mailnest.top/account
- 项目代码：迈巢根据项目提供对应的 Outlook 临时邮箱，KiroX 的项目代码默认为`aws001`，可直接使用。项目代码获取页面：https://mailnest.top/buy-email

### 2. 启动注册

切换到「注册」页面：
- 设置注册数量、并发数（建议 1–5）、任务间隔（秒）
- 选择邮箱来源
- 点击「开始注册」

### 3. 查看结果

注册成功的账号实时写入结果输出目录（默认为程序所在目录），文件名 `accounts.json`，格式：

```json
[
  {
    "email": "xxx@outlook.com",
    "password": "...",
    "access_token": "...",
    "refresh_token": "...",
    "registered_at": "2026-05-16T12:00:00Z"
  }
]
```

### 4. 代理配置

在「设置」页面填入代理地址，支持以下格式：
```
http://user:pass@host:port
socks5://host:port
host:port:user:pass
```
留空则直连。

---

## 项目结构

```
kirox/
├── main.go                    # 入口，Wails 初始化
├── app.go                     # App 结构体，Wails 绑定方法
├── internal/
│   ├── core/                  # Kiro 注册核心逻辑
│   │   ├── registrar.go       # Registrar 结构体，HTTP 客户端
│   │   ├── run.go             # 步骤编排
│   │   ├── auth.go            # 步骤 1–5
│   │   ├── signup_flow.go     # 步骤 6–9
│   │   ├── signup_password.go # 步骤 10–12
│   │   ├── kiro_auth.go       # 步骤 13–14
│   │   ├── kiro_exchange.go   # 步骤 15
│   │   └── verify.go          # 账号验证
│   ├── browser/               # 协议请求身份参数生成
│   ├── email/                 # 邮箱服务（Outlook / MoeMail / MailNest / Cloud-Mail）
│   ├── crypto/                # JWE 加密、XXTEA
│   ├── storage/               # 账号存储、配置持久化
│   ├── task/                  # 批量任务调度、并发控制
│   ├── data/                  # 注册结果读写
│   ├── proxy/                 # 代理出口 IP / 归属检测
│   ├── subscription/          # 订阅链接：刷 Token + listAvailableSubscriptions / CreateSubscriptionToken / setUserPreference
│   ├── updater/               # 版本检查
│   └── http/                  # TLS 客户端工具
└── frontend/
    ├── index.html             # 单页应用入口
    ├── js/                    # 页面逻辑（overview / accounts / moemail / task / subscription / app / ui）
    ├── css/                   # 样式（layout / components / style）
    └── build.js               # 前端构建脚本
```

---

## 技术栈

| 层 | 技术 |
|----|------|
| 桌面框架 | [Wails v2](https://wails.io) |
| 后端语言 | Go 1.24 |
| HTTP 客户端 | [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) |
| 前端 | 原生 HTML / CSS / JavaScript |
| 加密 | RSA-OAEP-256 + AES-256-GCM (JWE) |

当前版本仍以 Wails 桌面端为主，后续重构可能转向 WebUI 架构。

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
