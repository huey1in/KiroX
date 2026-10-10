# Native migration checklist

Reference baseline: `2bd6bd6` on `main`. This file records native capabilities,
verification evidence and the remaining release checks on `future`.

- [x] C++20 domain/application/infrastructure/desktop build targets.
- [x] CMake Debug/Release configure, build and test presets.
- [x] Native Qt Quick navigation and light/dark Liquid Glass interpretation.
- [x] Reduced motion/transparency and keyboard focus controls.
- [x] Runtime settings patching with legacy JSON field compatibility.
- [x] Outlook/iCloud import, provider isolation, status updates and removal.
- [x] Atomic JSON repository, output export and directory transactions.
- [x] Proxy CRUD, normalization, weighted selection and isolated probe service.
- [x] Cancelable HTTP sessions and per-task cookies.
- [x] Runtime and visual verification of all native pages.
- [x] Legacy portable/Roaming `storage.conf` migration.
- [x] MoeMail client, domain/configuration management and OTP polling.
- [x] Cloud-Mail client, domain/configuration management and OTP polling.
- [x] MailNest client, balance, mailbox allocation and OTP polling.
- [x] Outlook OAuth, Graph/IMAP baselines, junk folders and OTP polling.
- [x] iCloud mailbox baselines, message detail decoding and OTP polling.
- [x] Authenticated HTTPS CONNECT and verified native browser TLS profiles.
- [x] XXTEA, JWE and remote application configuration cache.
- [x] Browser identity/fingerprint generation and cache compatibility.
- [x] OIDC/device/signup/profile/password/SSO/Kiro exchange workflow.
- [x] Registration service error and alive/usage result parsing.
- [x] Telemetry producers and dictionary-reference regressions.
- [x] Batch orchestration, admission, concurrency, cancellation and stop-on-risk.
- [x] Persistent logs, retention and native notifications/completion audio.
- [x] Release update checks with semantic version comparison.
- [x] Desktop text translated into Chinese, English and Japanese.
- [x] Native single-instance reactivation and desktop platform behavior.
- [ ] Windows/macOS/Linux build and installation packaging.
- [x] Remove superseded Go/Wails/web frontend source and build dependencies.
- [ ] Full functional and visual completion audit; push completed rewrite to `future`.

Automated evidence must be recorded after tests run successfully. Implemented
interfaces and placeholder screens do not count as migrated features.

## Verified evidence

Windows Debug and Release have passed twelve isolated CTest suites: core,
mailbox, native_transport, crypto, browser, registration, batch, platform,
desktop, https_interop, imap_interop and crypto_interop. The HTTPS fixture
verifies nested CONNECT, Basic proxy authentication, all three browser profiles
and rejection of an incorrect trust root at each TLS boundary. The IMAPS
fixture performs real TLS/XOAUTH2 and UID commands against a local trusted
server. Cryptographic interoperability uses independent Python RSA/AES/ECDSA
verification.

The final Windows package passed 66 offscreen rendering scenarios: eight pages
in Chinese/English/Japanese and light/dark themes, two reduced-effects variants
and sixteen Japanese scenarios at the minimum 820x580 window size. Tests fail
on QML warnings and missing screenshots. Installed application startup was
also verified with Qt/MinGW SDK search paths removed.

TLS fixtures verify cipher and extension order for the three browser profiles.
Chrome 133 uses curl's 133a implementation: its ALPN/ALPS protocols differ from
the Go baseline. This is explicitly not a claim of byte-for-byte TLS parity.

Cross-platform packaging is exercised by `.github/workflows/native.yml`;
passing results must be recorded before completing the remaining release checks.

## Verification limits

No live AWS signup or paid mailbox service was used. Browser/provider protocols
can change independently of the local fixtures. Completion audio and system
tray notifications are implemented and deployed, but audible playback and OS
notification delivery need manual verification on each desktop platform.
Offscreen rendering checks layout and QML behavior; it does not establish GPU
driver compatibility or prove material appearance on every graphics backend.
