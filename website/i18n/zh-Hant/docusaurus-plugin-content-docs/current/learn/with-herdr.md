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

面板放在 skillshare repository 的 [`herdr/skillshare`](https://github.com/runkids/skillshare/tree/main/herdr/skillshare)。

1. 把 `mods/skillshare/` 複製到 `~/.config/herdr-app/mods/skillshare/`。
2. 把 `layouts/herdr-skillshare.json` 和 `layouts/herdr-skillshare.png` 複製到 `~/.config/herdr-app/layouts/`，再到 Herdr 設定切換到 `herdr-skillshare` layout。想保留自己的 layout，就照資料夾裡 `README.md` 列的兩個 block 加進去。
3. 執行 `herdr-app check ~/.config/herdr-app/mods/skillshare`。

tab bar 會出現 **skillshare** 按鈕，用來顯示或隱藏面板。切換 space 時，面板會維持原本的開關狀態。

## 面板功能

面板跟著 Herdr 顯示的 space 走，在該 space 的機器上、聚焦中 terminal 的資料夾裡執行 skillshare。

- **Resources** 列出 skills、agents、MCP servers、plugins 和 hooks，分成「Updates available」、「Needs sync」、「In sync」三組。在有 `.skillshare/config.yaml` 的專案裡，可以用 All · Project · Global 切換 scope。
- **列**會就地展開。skills 和 agents 提供 Update、Diff、Uninstall，以及「Ask &lt;agent&gt; to audit…」：確認後把該項目交給 Herdr 中聚焦的 agent。 頂層宣告的 hooks 提供 Enable、Disable 和 Remove，每項都先確認；Sync 會套用儲存的變更。
- hooks 計數包含具名項目和待清理項目；診斷通知使用相同篩選條件，零項目時也會分開顯示。檔案已同步時仍顯示無法執行的原因，巢狀列顯示完整專案路徑。僅原生檔案需要變更時也提供 Sync；hooks 衝突會阻止所有資源同步，需先在 CLI 或儀表板中解決。 預覽失敗時，項目會在分組外顯示同步狀態未知，每個範圍顯示一則錯誤通知。
- **Install** 接受 repository URL。含多個 skills 的 repository 會列出清單讓你挑選。面板會先 clone，對 clone 執行 [`skillshare audit`](/docs/reference/commands/audit)，通過才安裝。audit 擋下時會顯示 findings，並提供「Install anyway」（`--force`）和「Ask &lt;agent&gt; to review…」。
- **Source** 顯示 source repository 的狀態，對話框提供 Pull、Commit and push 和 Sync both ways。pull 遇到兩台機器都改過的檔案時，可以從左右對照預覽逐檔挑選要保留的版本。`.metadata.json` 會逐筆合併，與 [`skillshare pull`](/docs/reference/commands/pull) 的做法相同。
- **Machines** 顯示每台 Herdr 機器的 skillshare 版本、source 狀態和 doctor 警告，提供 Pull and sync；有新版時也提供 Update。

面板顯示時每 20 秒重新整理一次，每 10 分鐘執行 `skillshare check`，隱藏時不做任何事。

## 相關

- [Sync](/docs/reference/commands/sync)
- [Push](/docs/reference/commands/push) 和 [Pull](/docs/reference/commands/pull)
- [Audit](/docs/reference/commands/audit)
