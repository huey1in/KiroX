# Native development

KiroX uses C++20, CMake 3.25+ and Qt 6.8.3. Desktop modules are Core, Network,
Gui, Quick, QuickControls2, Svg, Widgets, Multimedia and ShaderTools. Test
targets additionally use Qt Test. A Ninja build is recommended.

## Windows

Use the matching Qt MinGW 13.1 compiler and Qt 6.8.3 SDK:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/bootstrap-cpp.ps1
powershell -ExecutionPolicy Bypass -File scripts/build-cpp.ps1 -Preset release -Deploy
out/package/bin/kirox.exe
```

Python provisions development tools. The packaged application has no Python,
Node.js, Go or browser runtime dependency. Do not mix a Qt MinGW distribution
with an MSVC compiler or another MinGW ABI.

## macOS and Linux

Install CMake, Ninja, a C++20 compiler, OpenSSL 3 development files and Qt
6.8.3 with the modules listed above. Set the prefix to your actual Qt SDK;
Qt's official archive uses `6.8.3/macos` on macOS and `6.8.3/gcc_64` on Linux.
With aqt, the Linux architecture name is `linux_gcc_64`, although its
installation directory is `gcc_64`.

```sh
cmake --preset release -DCMAKE_PREFIX_PATH=/absolute/path/to/qt \
  -DKIROX_WARNINGS_AS_ERRORS=ON
cmake --build --preset release --parallel 4
ctest --preset release
cmake --install out/release --prefix "$PWD/out/package"
cd out/release
cpack -G TGZ                 # Linux
# cpack -G DragNDrop         # macOS
```

Homebrew OpenSSL can be added to `CMAKE_PREFIX_PATH`, separated from Qt by a
semicolon. Linux needs system graphics/XCB and audio libraries; CI records the
packages used on Ubuntu 24.04. Qt's Linux SDK also requires its `icu` archive.
macOS CI uses Xcode 26.3's C++20 standard library. CMake removes Qt 6.8.3's
obsolete AGL fallback link flag when that framework is absent from the SDK.
The curl SDK is downloaded for the host architecture
and verified by a pinned SHA-256. Supply an existing SDK with
`-DKIROX_CURL_ROOT=/path/to/sdk`.

## Verification

`dev` and `release` presets use separate build trees. Tests isolate all data
and use synthetic credentials/local protocol fixtures. No test performs live
registration, contacts a paid mailbox provider or accesses the user's mailbox.

For independent JWE, ECDSA and IMAPS verification, install
`cryptography==46.0.3` in a development interpreter and configure
`-DKIROX_VERIFY_PYTHON=/absolute/path/to/python`. Both interoperability tests
are then included in CTest; that interpreter is not bundled with the app.

```powershell
.tools/python/Scripts/python.exe scripts/smoke-cpp.py `
  --executable out/package/bin/kirox.exe --output out/visual
```

Rendering covers all eight pages in three languages and two themes, reduced
effects and the minimum 820x580 window. It fails on QML warnings, missing images
or application errors. Copied system fonts stay in ignored `.tools` files.
`--data-home` and `KIROX_DATA_HOME` also isolate manual development sessions.

Source uses `.clang-format`. Format modified C++ files with clang-format and
changed QML with `qmlformat -i`. Build with warnings treated as errors and run
the tests relevant to the change.

## Extension points

Public contracts live in `include/kirox/ports`: repository transactions, HTTP
sessions, IMAP, mailbox sessions, cryptography, fingerprint configuration and
activity logs. Application services consume these contracts and return value
types; infrastructure supplies native implementations. Wire implementations
in `src/desktop/main.cpp` rather than constructing them in QML.

A mailbox adapter must establish a baseline before sending an OTP, poll only
new messages, honor cancellation/deadlines and confine each session to one
worker. A transport must preserve per-task cookies, honor the explicit proxy
mode and verify target/proxy TLS certificates.

The batch service snapshots settings, service configurations and proxy choices
at admission. Observers run on worker threads: post UI work asynchronously and
do not call `start()` or `wait()` from a callback. Repository writes use complete
read/mutate/atomic-write transactions.

TLS tests verify browser cipher and extension order. Chrome 133's native 133a
profile uses different ALPN/ALPS values from the former implementation and is
not byte-for-byte identical. Live AWS/provider behavior can change independently
of isolated regression fixtures.
