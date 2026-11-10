---
sidebar_position: 8
---

# 在 Herdr 中使用 skillshare

> 在 agents 旁邊放一個 skillshare 面板：不必離開 [Herdr](https://herdr.dev)，就能看到哪些需要更新或 sync，並直接處理。

![Herdr 中的 skillshare 面板](/img/herdr-panel-demo.png)

## 先決條件

- Herdr 0.1.12 以上
- Herdr 連線的每台機器都已安裝 skillshare，位置在 `~/.local/bin`、`~/go/bin`、`/opt/homebrew/bin` 或 `/usr/local/bin`

## 設定

### 從 Herdr layouts 安裝

這個 layout 已上架到 [Herdr layouts](https://herdr.dev/layouts/runkids/skillshare--herdr--skillshare/)。把下面這段 prompt 貼給 Herdr 裡的 agent：

```text
Read ~/.config/herdr-app/AGENTS.md, then add this layout: herdr.dev/layouts/runkids/skillshare--herdr--skillshare
```

agent 會先讀過所有檔案再加入 layout，你自己的 layout 不會被改動。用 `ctrl-shift-l` 切換 layout。

### 手動安裝

面板放在 skillshare repository 的 [`herdr/skillshare`](https://github.com/runkids/skillshare/tree/main/herdr/skillshare)。

1. 把 `mods/skillshare/` 複製到 `~/.config/herdr-app/mods/skillshare/`。
2. 把 `layouts/herdr-skillshare.json` 和 `layouts/herdr-skillshare.png` 複製到 `~/.config/herdr-app/layouts/`，再到 Herdr 設定切換到 `herdr-skillshare` layout。想保留自己的 layout，就照資料夾裡 `README.md` 列的兩個 block 加進去。
3. 執行 `herdr-app check ~/.config/herdr-app/mods/skillshare`。

tab bar 會出現 **skillshare** 按鈕，用來顯示或隱藏面板。切換 space 時，面板會維持原本的開關狀態。

## 更新面板

面板顯示時，每小時向 GitHub 查詢一次最新的 skillshare release。有新的 release 發布時，面板標題列會出現 **Update panel**。它會先詢問，再用該 release 附帶的檔案取代 `~/.config/herdr-app/mods/skillshare/` 裡的檔案，接著 Herdr 會重新載入面板。被取代的檔案保留在該資料夾的 `.bak-update-<version>`；你對這些檔案做的修改不會保留。你的 layout 不會被更動。

在這個按鈕出現之前安裝的面板不會檢查更新。請手動把最新 release 的 `mods/skillshare/` 複製到 `~/.config/herdr-app/mods/skillshare/` 一次；之後的更新就能用按鈕完成。

## 面板功能

面板跟著 Herdr 顯示的 space 走，在該 space 的機器上、聚焦中 terminal 的資料夾裡執行 skillshare。

- **Overview** 先給出答案：「Everything is in sync」，或是有多少個 target 需要同步，並提供一個 Sync 按鈕。下面列出 source 中每種資源的數量，以及通往每個 target 的一列，可以看出哪些 target 在等待同步。分頁其餘部分只列出需要你處理的內容：等待同步的、有更新的，以及 hooks 通知。在有 `.skillshare/config.yaml` 的專案裡，分頁旁邊的 All · Project · Global 可以切換 scope。
- **Library** 列出 skills、agents、MCP servers、plugins 和 hooks，分成「Needs sync」、「Update available」、「In sync」三組，已停用的項目歸入「Off」。對於 skills 和 agents，「In sync」會把同一個 repo 的項目收進可收合的分組；切換到 List 則顯示為平鋪清單。
- **列**會開啟預覽。skill 顯示其 `SKILL.md` 和檔案，MCP server 顯示隱藏了機密值的設定，預覽的 Targets 分頁顯示各 target 的狀態。skills 和 agents 提供 Update、Diff、Uninstall，以及「Ask &lt;agent&gt; to audit…」：確認後把該項目交給 Herdr 中聚焦的 agent。頂層宣告的 hooks 提供 Enable、Disable 和 Remove，每項都先確認；Sync 會套用儲存的變更。
- hooks 計數包含具名項目和待清理項目；診斷通知有獨立的列。hooks 衝突會阻止所有資源同步，需先在 CLI 或儀表板中解決。無法讀取計畫時，項目會在分組外顯示同步狀態未知。
- **Install** 接受 repository URL。含多個 skills 的 repository 會列出清單讓你挑選。面板會先 clone，對 clone 執行 [`skillshare audit`](/docs/reference/commands/audit)，通過才安裝。audit 擋下時會顯示 findings，並提供「Install anyway」（`--force`）和「Ask &lt;agent&gt; to review…」。
- **Source** 顯示 source repository 的狀態，對話框提供 Pull、Commit and push 和 Sync both ways。pull 遇到兩台機器都改過的檔案時，可以從左右對照預覽逐檔挑選要保留的版本。`.metadata.json` 會逐筆合併，與 [`skillshare pull`](/docs/reference/commands/pull) 的做法相同。
- **Machines** 顯示每台 Herdr 機器的 skillshare 版本、remote、source 和 target 狀態，以及 doctor 警告，提供 Pull and sync；有新版時也提供 Update。有未提交或未推送變更的機器會改為提供 Sync both ways，因為一般的 pull 會被拒絕。

面板顯示時每 20 秒重新整理一次，每 10 分鐘執行 `skillshare check`，隱藏時不做任何事。

## 相關

- [Sync](/docs/reference/commands/sync)
- [Push](/docs/reference/commands/push) 和 [Pull](/docs/reference/commands/pull)
- [Audit](/docs/reference/commands/audit)
