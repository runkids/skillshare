---
sidebar_position: 2
---

# 桌面 App

用 **[Skillshare App](https://github.com/runkids/skillshare-app)**，在桌面視窗中管理技能、代理、MCP 與 hooks。它把 skillshare 儀表板帶到 macOS、Windows 與 Linux，並提供首次使用的設定引導。

**[下載 Skillshare App →](https://github.com/runkids/skillshare-app/releases/latest)**

## 在 macOS 安裝

Apple Silicon Mac 建議透過 Homebrew 安裝：

```bash
brew tap runkids/tap
brew install --cask skillshare-app
```

安裝後，從「應用程式」開啟 **skillshare**。也可以從[最新版本](https://github.com/runkids/skillshare-app/releases/latest)下載 `.dmg` 手動安裝。

## 在 Windows 或 Linux 安裝

從[最新 App 版本](https://github.com/runkids/skillshare-app/releases/latest)選擇安裝包：

| 平台 | 安裝包 |
|---|---|
| Windows（x64） | `.exe` 或 `.msi` |
| Linux（x64） | `.deb`、`.AppImage` 或 `.rpm` |

安裝適合系統的套件，再開啟 Skillshare App。

## 首次啟動

App 使用 skillshare CLI 運作。首次設定會引導你：

1. 安裝 CLI，或選取已安裝的執行檔。
2. 選擇要同步的 AI 工具。
3. 執行第一次同步，再開啟儀表板。

你可以在儀表板瀏覽與安裝技能、在同步前預覽變更，並管理代理、MCP 與 hooks。桌面 App 與 [`skillshare ui`](../reference/commands/ui.md) 使用相同的儀表板。

## 偏好終端機？

CLI 仍可用於終端機操作與自動化。請從 [CLI 首次同步指南](./first-sync.md)開始，或用[快速參考](./quick-reference.md)查詢指令。
