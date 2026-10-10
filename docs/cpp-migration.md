# Native migration checklist

Reference baseline: `2bd6bd6` on `main`. The full migration remains in progress.
This file records evidence and remaining work; unchecked capabilities are
required before the branch can be considered a completed rewrite.

- [x] C++20 domain/application/infrastructure/desktop build targets.
- [x] CMake Debug/Release configure, build and test presets.
- [x] Native Qt Quick navigation and light/dark Liquid Glass interpretation.
- [x] Reduced motion/transparency and keyboard focus controls.
- [x] Runtime settings patching with legacy JSON field compatibility.
- [x] Outlook/iCloud import, provider isolation, status updates and removal.
- [x] Atomic JSON repository, output export and directory transactions.
- [x] Proxy CRUD, normalization, weighted selection and isolated probe service.
- [x] Cancelable HTTP sessions and per-task cookies.
- [ ] Runtime and visual verification of all native pages.
- [x] Legacy portable/Roaming `storage.conf` migration.
- [x] MoeMail client, domain/configuration management and OTP polling.
- [x] Cloud-Mail client, domain/configuration management and OTP polling.
- [x] MailNest client, balance, mailbox allocation and OTP polling.
- [x] Outlook OAuth, Graph/IMAP baselines, junk folders and OTP polling.
- [x] iCloud mailbox baselines, message detail decoding and OTP polling.
- [ ] HTTPS proxy tunnel and original browser TLS profile parity.
- [x] XXTEA, JWE and remote application configuration cache.
- [x] Browser identity/fingerprint generation and cache compatibility.
- [x] OIDC/device/signup/profile/password/SSO/Kiro exchange workflow.
- [x] Registration service error and alive/usage result parsing.
- [x] Telemetry producers and dictionary-reference regressions.
- [x] Batch orchestration, admission, concurrency, cancellation and stop-on-risk.
- [ ] Persistent logs, retention and native notifications/completion audio.
- [x] Release update checks with semantic version comparison.
- [ ] All user-facing text translated into Chinese, English and Japanese.
- [x] Native single-instance reactivation and desktop platform behavior.
- [ ] Windows/macOS/Linux build and installation packaging.
- [ ] Remove superseded Go/Wails/web frontend source and build dependencies.
- [ ] Full functional and visual completion audit; push completed rewrite to `future`.

Automated evidence must be recorded after tests run successfully. Implemented
interfaces and placeholder screens do not count as migrated features.

## Verified evidence

Windows Release has passed eleven isolated CTest suites: core, mailbox,
native_transport, crypto, browser, registration, batch, platform, desktop, imap_interop
and crypto_interop. The IMAPS fixture performs real TLS/XOAUTH2 and UID
commands against a local trusted server. Cryptographic interoperability uses
independent Python RSA/AES/ECDSA verification. No live signup or paid mailbox
service was used for verification.

An earlier Windows package passed 50 offscreen rendering scenarios (eight
pages, Chinese/English/Japanese, light/dark, plus reduced-effects variants).
The package must be redeployed after the latest multimedia/controller changes.

TLS fixtures verify cipher and extension order for the three browser profiles.
Chrome 133 uses curl's 133a implementation: its ALPN/ALPS protocols differ from
the Go baseline. This is explicitly not a claim of byte-for-byte TLS parity.

Cross-platform packaging is now exercised by `.github/workflows/native.yml`;
passing results must be recorded before completing this checklist.
