# 贡献指南

感谢你对 Kiro 注册机的关注！欢迎提交 Issue 和 Pull Request。

## 提交 Issue

- **Bug 报告**：请描述复现步骤、期望行为、实际行为，并附上日志截图
- **功能建议**：请说明使用场景和预期效果
- 提交前请先搜索是否已有相同 Issue

## 提交 Pull Request

### 环境准备

```bash
# 依赖
# - C++20 compiler, CMake, Ninja, Qt 6.8.3

git clone --branch future https://github.com/huey1in/kirox.git
cd kirox
cmake --preset dev -DCMAKE_PREFIX_PATH=/path/to/qt
cmake --build --preset dev
ctest --preset dev
```

### 开发规范

- **C++**：使用 C++20 和 `.clang-format`；提交前启用 `KIROX_WARNINGS_AS_ERRORS` 构建。
- **界面**：使用 Qt Quick / QML；业务逻辑通过控制器调用应用服务，保持接口与模块边界。
- **测试**：使用临时数据目录和本地协议夹具，避免真实账号、付费邮箱和线上注册请求。
- Windows 工具准备、跨平台构建和扩展接口见 [开发指南](docs/cpp-development.md)。
- **提交信息**：使用中文或英文均可，格式 `type: 简短描述`，例如：
  - `fix: 修复 MoeMail 域名加载失败`
  - `feat: 添加代理池支持`
  - `docs: 更新 README`

### 分支规范

- `main` — 稳定发布分支，不直接推送
- `future` — C++ 原生重构分支；对应功能请从该分支创建 `feat/xxx`
- Bug 修复请创建 `fix/xxx` 分支

### PR 要求

1. 确保对应 CMake 构建和 CTest 通过
2. 界面变化验证浅深色、中英日语言、最小窗口和减少动态/透明效果
3. 简要描述改动内容和原因

## 行为准则

请保持友善和尊重，共同维护良好的开源社区氛围。

---

如有疑问，欢迎加入 [AI 交流群](https://qm.qq.com/q/RXMTXUlc4w) 讨论。
