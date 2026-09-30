---
sidebar_position: 2
---

# 在 GitHub Copilot 中使用 skillshare

> 從安裝到第一次 sync — 只要 5 分鐘。

## 先決條件

- 已在 VS Code 或 JetBrains 中啟用 [GitHub Copilot](https://github.com/features/copilot) coding agent
- macOS、Linux 或 Windows

## 步驟 1：安裝 skillshare

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

只有安裝器顯示 PATH 設定提示時，才需要依提示設定後再執行下方指令。沒有 PATH 警告就不需要額外設定。

## 步驟 2：初始化

```bash
skillshare init
```

這會偵測 Copilot 的 skill 目錄（`~/.copilot/skills/`），並自動將它加入為 target。

## 步驟 3：切換到 Copy Mode（建議）

我們收到回報，Copilot 有時無法正確跟隨 symlinks。為了避免問題，請將 Copilot target 切換到 **copy mode**：

```bash
skillshare target copilot --mode copy
```

Copy mode 會將 skill 檔案實際複製到 `~/.copilot/skills/`，而不是建立 symlinks。缺點是對 source 的編輯不會立即反映 — 你需要執行 `skillshare sync` 才能套用變更。但這在不同平台上更為可靠。

:::tip 何時使用 merge（symlink）mode
如果你使用 macOS 或 Linux，且 Copilot 在你的機器上能正確讀取 symlinks，預設的 merge mode 就可以正常運作。你隨時都可以切換回去：

```bash
skillshare target copilot --mode merge
```
:::

## 步驟 4：安裝你的第一個 Skill

```bash
skillshare install runkids/my-skills
```

## 步驟 5：Sync

```bash
skillshare sync
```

Skills 會被複製到 `~/.copilot/skills/`。Copilot 會將它們視為自訂指令來讀取。

## 步驟 6：驗證

```bash
ls ~/.copilot/skills/
```

你應該會看到已安裝的 skills，以真實目錄（copy mode 下）或 symlinks（merge mode 下）的形式出現。

## Copilot 專屬注意事項

- **Skill 路徑**：`~/.copilot/skills/`（global）或 `.github/skills/`（project）
- **Agent 路徑**：`~/.copilot/agents/`（global）或 `.github/agents/`（project）— Copilot CLI 會以與 skillshare 管理的相同 `.agent.md` 格式讀取自訂 agent，因此 `skillshare sync agents` 可以直接分發，不需要轉換格式。參見 [Agents](/docs/understand/agents)。
- **Project mode**：執行 `skillshare init -p` 以管理 project 層級的 Copilot skills — 它們會被放進 `.github/skills/`，與你的程式碼庫放在一起
- **Symlink 問題**：如果 Copilot 沒有讀取到你的 skills，請檢查你的 target 是否處於 merge mode（`skillshare status`），並依上述方式切換到 copy mode
- **`.github/copilot-instructions.md`**：如果你已經有現成的指示檔案，skillshare 的 skills 會與它互補 — 而不是取代它

## 接下來呢？

- [管理多個 skills →](/docs/how-to/daily-tasks/organizing-skills)
- [與團隊分享 →](/docs/how-to/sharing/organization-sharing)
- [探索更多 skills →](/docs/reference/commands/search)
