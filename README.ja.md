<p align="center">
  <img src="frontend/assets/app.svg" width="100" height="100" alt="KiroX">
</p>

<h1 align="center">KiroX | Kiroプロトコル登録ツール</h1>

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

## 概要

KiroX は [Wails v2](https://wails.io) ベースの Kiro 登録ツールで、HTTP/TLS プロトコルだけで実装されています。プロトコルリクエストだけで AWS Builder ID のアカウント登録、メール認証、認可、Kiro トークン交換を行います。Outlook、MoeMail、Cloud-Mail、MailNest、iCloud の 5 種類のメールソースを利用でき、並列制御とプロキシにも対応します。

---

## 特別な感謝

<p align="center">
  <a href="https://www.ipwo.net/?ref=githubKiroX" target="_blank">
    <img src="docs/ipwo.webp" alt="IPWO" width="800">
  </a>
</p>

<p align="center">
  <b><a href="https://www.ipwo.net/?ref=githubKiroX" target="_blank">IPWO</a></b> は 195 以上の国・地域をカバーするレジデンシャルプロキシ IP を提供し、HTTP・HTTPS・SOCKS5 プロトコルに対応しています。<br>
  Kiro、AI コーディング、ブラウザ自動化、海外ネットワークアクセスに適しており、地域やビジネスニーズに応じてネットワーク環境を選択できます。<br>
  無料トライアル対応。専用割引コード：<code>0205</code>
</p>

---

## 機能

**Kiro 登録フロー**

- プロトコル登録処理を自動化（OIDC 登録 → デバイス認可 → メール認証 → パスワード設定 → SSO → Kiro トークン交換）
- 登録後にアカウントの生存確認を自動実行
- バッチ処理：登録件数 / 並行数 / タスク間隔を設定可能（間隔は直列実行時のみ適用）
- 実行ログ、進捗と統計の確認、実行中タスクの停止に対応。同時に実行できるバッチは 1 件

**メールソース**

- **Outlook メールボックスプール**：`メール----パスワード----クライアントID----RefreshToken` 形式でインポート。末尾に `----imap` または `----graph` を指定でき、省略時は IMAP で認証コードを取得
- **MoeMail 使い捨てメール**：複数ドメイン設定、自動ローテーション、ランダム / 全て / 指定ドメインモード
- **Cloud-Mail（セルフホスト）**：[cloud-mail](https://github.com/jiangrungen/cloud-mail) と連携、ドメインはサーバーから自動取得、ランダム / ラウンドロビン / 指定モード
- **MailNest 一時メール**：API キーとプロジェクトコードを設定し、残高確認と認証コード取得に対応
- **iCloud メールボックスプール**：`メール----メッセージ一覧URL` 形式でインポートし、互換性のあるメッセージ一覧ページから認証コードを取得

**純粋なプロトコルとネットワーク**

- `tls-client` による HTTP/TLS クライアントとリクエストパラメータ設定

**データ管理**

- 登録成功アカウントは設定可能な出力ディレクトリに平文 JSON で書き出し
- メールボックスプールとサービス設定はローカル JSON として保存
- データディレクトリと結果出力ディレクトリのカスタマイズに対応

**プロキシ**

- 「IP 管理」ページでプロキシの一括追加、接続テスト、有効化 / 無効化、削除に対応（HTTP / HTTPS / SOCKS5）
- 新規タスク作成時に登録リクエスト用の有効なプロキシを 1 件選択、または直接接続
- `scheme://user:pass@host:port` 形式のプロキシアドレスに対応

**バージョン更新**

- GitHub Releases の最新バージョンを確認（セマンティックバージョン比較）
- Releases ページを開き、手動でダウンロードとインストールを行う

**インターフェース**

- 中国語 / 英語 / 日本語とライト / ダークテーマに対応
- 概要ページでリアルタイムの統計を、ログページで実行状況を確認

---

## 今後の方向性

KiroX は現在、大規模なアーキテクチャリファクタリングを準備しています。今後は次の方向を予定しています：

- **デスクトップ GUI から WebUI へ**：フロントエンドとバックエンドを段階的に分離し、ブラウザアクセスやサーバー運用などに対応します。
- **2API を内蔵**：AWS CodeWhisperer の機能を標準的な OpenAI API および Anthropic API のエンドポイントへ適応し、既存のクライアントや開発ツールから利用しやすくします。
- **登録タスクとアカウント管理の強化**：タスク編成、アカウント管理、認証情報のライフサイクル、実行監視を統合します。
- **サービス境界の整理**：今後のモデルアダプター、プロキシ戦略、自動化機能の拡張に備えます。

これらは開発状況に応じて段階的に実装します。現行版は引き続き Wails デスクトップ体験を中心とします。

---

## クイックスタート

### リリース版を使う

[Releases](https://github.com/huey1in/kirox/releases/latest) から OS と CPU アーキテクチャに合った `.zip` / `.tar.gz` をダウンロードし、展開してアプリを実行します。Windows x64 の例は `kiro-reg-windows-amd64.exe.zip` です。

Windows では WebView2、Linux のリリース版では GTK 3 / WebKitGTK 4.1 などのシステムランタイムが必要です。

### ソースからビルド

**必要環境**

- Go 1.24.1 以上（`go.mod` の toolchain は Go 1.24.4）
- Node.js 20+
- Wails CLI v2.11.0 と OS ごとのビルド依存関係（`wails doctor` で確認）

```bash
# Wails CLI をインストール
go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0

# リポジトリをクローン
git clone https://github.com/huey1in/kirox.git
cd kirox

# 環境を確認
wails doctor

# アプリアイコンを準備
node -e "require('fs').copyFileSync('frontend/assets/appicon.png', 'build/appicon.png')"

# 開発モード（ホットリロード）
wails dev

# 本番ビルド
wails build
```

ビルド成果物は `build/bin/` に出力されます。

WebKitGTK 4.1 を使う Linux 環境では、`wails dev -tags webkit2_41` / `wails build -tags webkit2_41` を使用してください。

フロントエンドに npm 依存関係はありません。Wails が実行する `frontend/build.js` は静的ファイルを `frontend/dist/` にコピーするだけで、単独の Web サーバーは起動しません。Go の機能は Wails のバインディング経由で呼び出すため、アプリの開発には `wails dev` を使用します。

---

## 使い方

### 1. メールを設定

**Outlook メールボックスプール**（推奨）

「メール」ページでアカウントをインポート、1 行 1 件、形式：

```text
メール----パスワード----クライアントID----RefreshToken
メール----パスワード----クライアントID----RefreshToken----graph
```

5 番目の項目は `imap` または `graph` で、省略時は `imap` です。`.txt` / `.csv` ファイルからの一括インポートと手動貼り付けに対応します。

**MoeMail 使い捨てメール**

「メール」ページで MoeMail 設定を追加し、API アドレスと API キーを入力、接続テスト後に保存。登録時にランダム / 全て / 指定ドメインを選択可能。

**Cloud-Mail（セルフホスト）**

「メール」ページで Cloud-Mail 設定を追加し、サーバー URL、管理者メール、パスワードを入力します。接続テスト後に設定を保存します。ドメインは接続テストや保存時に `/api/setting/websiteConfig` から自動取得するため、ドメインの入力欄はありません。

**MailNest 一時メール**

「メール」ページで API キーとプロジェクトコードを入力します。両方とも必須で、`aws001` は入力例でありデフォルト値ではありません。保存時に接続と残高を確認し、テストが成功すると設定を保存します。

**iCloud メールボックスプール**

「メール」ページで iCloud アカウントを手動貼り付け、または `.txt` / `.csv` ファイルからインポートします。1 行 1 件、形式：

```text
メール----メッセージ一覧URL
```

URL は本プロジェクトの取得処理に対応するメッセージ一覧ページを指定します。通常の iCloud Web メールへのログイン URL や Apple ID のパスワードでは利用できません。

### 2. 登録を開始

「概要」ページの「新規タスク」を開きます：

- 登録件数、並行数、タスク間隔（秒）を設定。間隔は並行数 1 の直列実行時にタスク間で適用
- メールソースと、必要に応じて設定 / ドメインを選択
- このタスクで使う有効なプロキシを 1 件選択、または直接接続
- タスクを開始し、「ログ」ページで進捗とログを確認

同時に実行できるバッチは 1 件です。実行中のタスクは画面から停止できます。

### 3. 結果を確認

成功したアカウントは結果出力ディレクトリ（デフォルト `~/Documents/Kirox`）に `accounts.json` として書き込まれます。出力ディレクトリは「設定」で変更できます。保存形式の例：

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

同じメールアドレスの成功記録は最新の内容で置き換えます。`creditUsed` / `creditLimit` の値は例で、検証結果によって項目が省略されたり `null` になったりします。パスワードとアクセストークンはこのファイルに保存されません。

アプリデータのデフォルト保存先は `os.UserConfigDir()/kirox`（Windows では `%APPDATA%\kirox`）です。メールボックスプールやメールサービス設定は平文 JSON、保存先などの設定を持つ `storage.conf` はキーと値のテキスト形式です。

データディレクトリにもメールボックスプール用の `accounts.json` があり、結果出力ディレクトリの同名ファイルとは用途と形式が異なります。2 つのディレクトリは別々に設定してください。結果出力先を変更しても既存の結果ファイルは自動移行されません。

### 4. プロキシ設定

「IP 管理」ページでプロキシを追加します。1 行 1 件の一括追加に対応し、接続テスト、有効化 / 無効化、削除ができます。以下の形式に対応：

```text
http://user:pass@host:port
socks5://host:port
http://host:8080
```

新規タスク作成時に、有効なプロキシの中から登録リクエストに使用する 1 件を選択します。プロキシを選択しなければ直接接続します。

---

## プロジェクト構成

```text
kirox/
├── main.go                    # エントリポイント、Wails 初期化
├── app.go                     # App 構造体、Wails バインドメソッド
├── internal/
│   ├── core/                  # Kiro 登録コア
│   │   ├── registrar.go       # Registrar、HTTP クライアント、ステップ 1–5
│   │   ├── run.go             # ステップオーケストレーション
│   │   ├── auth.go            # SSO ワークフロー、トークン取得
│   │   ├── signup_flow.go     # 登録フロー、メール認証コード
│   │   ├── signup_password.go # ID 作成、パスワード設定
│   │   ├── kiro_auth.go       # Kiro 認可
│   │   ├── kiro_exchange.go   # ステップ 15
│   │   └── verify.go          # アカウント生存確認
│   ├── browser/               # プロトコルリクエストの識別情報生成
│   ├── email/                 # Outlook / MoeMail / Cloud-Mail / MailNest / iCloud
│   ├── crypto/                # JWE 暗号化、XXTEA
│   ├── storage/               # アカウント保存、設定永続化
│   ├── task/                  # バッチスケジューリング、並行制御
│   ├── data/                  # 結果 I/O
│   ├── proxy/                 # プロキシプール管理、出口 IP / 地域検出
│   ├── updater/               # バージョン確認
│   └── http/                  # TLS クライアントヘルパ
└── frontend/
    ├── index.html             # シングルページエントリ
    ├── js/                    # overview / accounts / moemail / cloudmail / mailnest / task / ip / app / ui / i18n / dropdown
    ├── css/                   # layout / components / style / dashboard
    └── build.js               # 静的ファイルを frontend/dist/ にコピー
```

---

## 技術スタック

| レイヤ | 技術 |
|----|------|
| デスクトップフレームワーク | [Wails v2.11.0](https://wails.io) |
| バックエンド | Go 1.24.1+（toolchain 1.24.4） |
| HTTP クライアント | [bogdanfinn/tls-client](https://github.com/bogdanfinn/tls-client) |
| フロントエンド | ネイティブ HTML / CSS / JavaScript |
| 暗号化 | RSA-OAEP-256 + AES-256-GCM (JWE) |

フロントエンドは `window.go.main.App` 経由で Go メソッドを呼び出し、タスク状態とログを定期的に取得します。現行版は Wails デスクトップアプリで、独立した WebUI や 2API サービスはまだ実装されていません。

---

## 注意事項

- 本ツールは学習・研究目的のみで使用し、AWS の利用規約を遵守してください
- IP レート制限を避けるため、プロキシの併用を強く推奨
- Outlook アカウントは有効な RefreshToken を事前に準備する必要があります
- 並行数が高すぎると AWS のリスク管理にかかる可能性があり、低い並行数から徐々に上げるのを推奨

---

## FAQ

### IP クリーン度関連

実行中に下記いずれかのエラーが発生する場合、出口 IP が AWS / Microsoft のリスク管理対象になっている可能性が高いです。

**ケース 1：メール認証コード送信が OTP 400 を返す**

![ケース 1](docs/images/1.png)
![ケース 1](docs/images/3.png)

よりクリーンな住宅プロキシへ切り替えてください。

> セルフホストメールや使い捨てメール（MoeMail など）使用時は、ドメイン自体が Microsoft / AWS にブラックリスト入りしている可能性もあるため、別ドメインを試してみてください。

**ケース 2：登録フローが停止するかメールにアクセスできない**

![ケース 2](docs/images/2.png)

同じプロキシ設定の実ブラウザで [outlook.live.com](https://outlook.live.com) を開いてみてください：

- ブラウザでも開けない / CAPTCHA → IP が Microsoft によりブロック、プロキシ変更が必要
- ブラウザでは正常 → Outlook アカウントの RefreshToken の有効性を確認

### macOS で「アプリが破損している」と表示される

未署名アプリは Gatekeeper により初回起動がブロックされます。ターミナルで以下のコマンドを実行して隔離属性を削除してください：

```bash
xattr -cr /path/to/KiroX.app
```

`/path/to/KiroX.app` を実際のパスに置き換えてください（`KiroX.app` をターミナルにドラッグすると自動入力されます）。

---

## コミュニティ

- QQ グループ：[参加する](https://qm.qq.com/q/RXMTXUlc4w)

---

## 作者

**1in** · [@huey1in](https://github.com/huey1in)

Copyright © 2026

---

## ライセンス

本プロジェクトは [Apache License 2.0](LICENSE) の下で公開されています。

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
