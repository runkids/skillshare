---
sidebar_position: 2
---

# 桌面 App

通过 **[Skillshare App](https://github.com/runkids/skillshare-app)**，在桌面窗口中管理技能、代理、MCP 和 hooks。它将 skillshare 仪表板带到 macOS、Windows 和 Linux，并提供首次使用的设置引导。

**[下载 Skillshare App →](https://github.com/runkids/skillshare-app/releases/latest)**

## 在 macOS 安装

Apple Silicon Mac 建议通过 Homebrew 安装：

```bash
brew tap runkids/tap
brew install --cask skillshare-app
```

安装后，从“应用程序”打开 **skillshare**。也可以从[最新版本](https://github.com/runkids/skillshare-app/releases/latest)下载 `.dmg` 手动安装。

## 在 Windows 或 Linux 安装

从[最新 App 版本](https://github.com/runkids/skillshare-app/releases/latest)选择安装包：

| 平台 | 安装包 |
|---|---|
| Windows（x64） | `.exe` 或 `.msi` |
| Linux（x64） | `.deb`、`.AppImage` 或 `.rpm` |

安装适合系统的软件包，再打开 Skillshare App。

## 首次启动

App 使用 skillshare CLI 运行。首次设置会引导你：

1. 安装 CLI，或选择已安装的可执行文件。
2. 选择要同步的 AI 工具。
3. 执行第一次同步，再打开仪表板。

你可以在仪表板浏览和安装技能、在同步前预览更改，并管理代理、MCP 和 hooks。桌面 App 与 [`skillshare ui`](../reference/commands/ui.md) 使用相同的仪表板。

## 偏好终端？

CLI 仍可用于终端操作和自动化。请从 [CLI 首次同步指南](./first-sync.md)开始，或通过[快速参考](./quick-reference.md)查询命令。
