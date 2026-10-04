---
sidebar_position: 9
---

# 跨工具管理 plugin

一個 plugin 可以包含 Skills、MCP 連線、hooks、腳本，以及其他共同運作的檔案。
Skillshare 會保持這個套件完整，並讓你選擇哪些工具要接收它。你不需要撰寫 YAML 就能開始使用。

## 新增你的第一個 plugin

在 dashboard 中，開啟 **Plugins → Add plugin**：

1. 貼上一個 GitHub repository（`owner/repo`）、HTTPS Git URL，或本機目錄。
2. 若來源包含多個 plugin，選擇其中一個，然後選取相容的工具。
3. 檢視變更並套用。

![Add plugin 對話框：找到的 plugin，以及相容與不支援的 targets](/img/plugins-add-dialog.png)

大多數使用者只需要一個 repository 與 target 勾選框。**進階選項**可新增 Git ref，以選擇特定 release。探索結果會分別顯示每個 target 的元件與相容性。當 OpenCode 因為偵測不到其進入點而被列為不支援時，該列的 **Set entry path** 會使用已建置的檔案並重新搜尋來源，同時保留你先前的選擇。安全的相對 repository symlinks 會被保留。

同樣的引導式流程也能在終端機中使用：

```bash
skillshare plugin add
```

Claude Code、Codex、Copilot、Antigravity CLI、Grok、Pi 或 OpenCode CLI 必須安裝在 **Skillshare 後端執行的所在位置**。
Codex 的話，Codex 桌面 app 內附的 CLI 也算；Codex 裝在其他位置的機器，請設定 [`SKILLSHARE_CODEX_CLI`](/docs/reference/appendix/environment-variables#skillshare_codex_cli)。
Cursor 與 Antigravity desktop 則是直接在其本機 plugin 目錄中接收完整的檔案。
在容器中執行的 dashboard 無法管理僅安裝在主機上的 plugin。請改用本機 CLI，或將 Skillshare 與原生 client 並行執行。

安裝 target 包含 **Claude Code、Codex、Cursor、Antigravity Desktop、Antigravity CLI、
GitHub Copilot CLI、Pi 與 OpenCode**。Grok 需要先在原生環境安裝並信任後才能匯入。
Kimi、Hermes 與 Devin 格式可被探索到，但無法自動安裝，介面會說明原因。
請選擇為你的 target 發布的格式；Skillshare 不會在工具之間轉換 plugin 格式。
Project mode 支援 Claude、Antigravity Desktop、Pi 與 OpenCode。
Antigravity Desktop（`antigravity` 或 `agy`）與 CLI（`antigravity-cli`）使用各自獨立的儲存區。請選擇你實際使用的那一個。

## 已經安裝過了嗎？

選擇 **Import installed** 並選取一個原生安裝。匯入只會記錄它，
不會重新安裝、複製其驗證資訊，也不會改變它在該 Agent 中是否啟用。
匯入功能適用於 Claude、Codex、Copilot、Antigravity CLI、Grok、Pi 與 OpenCode；
Cursor 與 Antigravity 本機套件則請使用 **Add plugin**。

```bash
skillshare plugin import review@team --from claude --no-tui
```

如果同一個邏輯套件對不同工具使用不同的原生發佈形式，請對每種發佈形式使用相同的
`--name` 搭配對應的 target 分別新增／匯入。Skillshare 不會從顯示名稱推斷等價關係。

匯入綁定的是這台機器上的原生安裝。要在另一台機器上使用同一個 plugin，請改從它的
source 新增；請參考 [跨機器 Sync — Plugins](/docs/how-to/sharing/cross-machine-sync#plugins)。

## 選擇要同步到哪裡

每個受管理的綁定都有一個勾選框。這個勾選框代表**在同步時包含此 target**，
而不是「在該 Agent 內部啟用」。

- 勾選它，然後同步以安裝缺少的 plugin。
- 開啟一個 plugin 的列也會列出其來源有提供套件、但尚未勾選的其他 Agents。勾選其中一個會開啟安裝預覽。來源無法提供的 Agents 會在該列末尾以數字統計，點選該數字可查看原因。
- 新增 plugin 時可以一個 Agent 都不勾。它會留在 Skillshare 中並顯示 **尚未選擇 Agent**，直到你在該列勾選 Agent 之前都不會安裝任何東西。
- 取消勾選它，然後同步以移除該受管理的安裝。
- 套件定義仍會保留，因此之後可以再次選取該 target。
- 在 Claude 或 Codex 內部被停用的 plugin 仍會維持停用；請在該工具中管理原生設定。

在 dashboard 中，Plugins 頁面右上角的 **Sync** 方塊會列出下一次同步將為每個 Agent
安裝或移除的內容。點選其按鈕會開啟預覽；在你確認之前，任何 Agent 都不會有變動。
plugin 清單會立即顯示，而方塊下方的 **Agents** 欄位則會隨著各個 Agent 的 CLI 回應而逐步填入。
執行後，方塊會列出結果：失敗排在最前，接著是其他變更，結果相同的 Agent 合併成一列。沒有變化的項目收成一行 **Unchanged**，展開後每個 plugin 一列，Pi 套件分開列出。預覽也用同樣方式，把不會變動的項目收在要執行的變更下方。清單會顯示每個 Pi 套件的已安裝版本，讀取自 Pi 安裝的套件。

```bash
skillshare plugin disable review --target codex --no-tui
skillshare sync plugins --dry-run
skillshare sync plugins --no-tui
```

plugin 的選單中有 **View files**：即已審閱過的來源本機副本，唯讀，並以 Markdown 格式呈現。
匯入的 plugin 沒有本機副本，因此也沒有這個項目。

plugin 與一般的 Skill 及 MCP 同步是分開的。它們所包含的元件不會同時複製到獨立的
Skillshare Source 中。

## 更新與復原 {#updates-and-recovery}

使用 **Check updates**，然後檢視受支援 target 的更新內容。Claude 可透過其原生 CLI 更新。
Codex 會以已審閱的快照再次 add 該 plugin；但若它在 Codex 中被停用則不會更新，因為 add 會把它重新啟用；匯入的 Codex plugin 則以升級其 marketplace 的方式更新。
Cursor 與 Antigravity 會在檢查過本機是否有編輯後，取代受管理的本機副本。
Pi 與 OpenCode 會更新已審閱的快照。Copilot 可以在保留已知啟用狀態的同時重新整理已審閱的來源。
Antigravity CLI 與 Grok 的更新仍留在原生工具中進行；匯入套件的限制請參閱指令參考。
更新無法處理的 target 會被略過並說明原因，該 plugin 的其他 Agent 仍會照常更新。

Pi 的 npm 套件沒有可比對的 source，所以 **Check updates** 會拿已安裝版本和 npm 上的最新版本比較。找到新版本時，該列會顯示 `old → new` 和 **Update** 按鈕，檢查結果裡每個找到的 plugin 也各有一個；兩者都會先打開該 plugin 的更新預覽。

如果某個 target 失敗，結果會保留其餘成功的結果。請先解決原生 client 的驗證或設定問題，
再重新同步該 target：

```bash
skillshare sync plugins review --target claude --no-tui
```

快照由 Skillshare 擁有。若其內容已被外部編輯，Skillshare 會阻擋覆寫，讓你能先保留那些編輯。
來源摘要（digest）用於偵測變更；它並不保證每個原生安裝或匯入的 marketplace 在不同機器間都能重現。

如需自動化、範圍細節與所有旗標，請參閱 [plugin](/docs/reference/commands/plugin)。
