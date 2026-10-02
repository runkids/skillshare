---
sidebar_position: 2
---

# 第一次 Sync

依序完成第一次設定，從安裝到可用的 sync 大約五分鐘。另外兩種變化型 — 在另一台機器上還原、以及在沒有終端互動的機器上無人值守執行 — 記錄在本頁最後。

## 先決條件

- macOS、Linux 或 Windows
- 至少安裝一套 AI CLI（Claude Code、Pi、Codex 等）

## 1. 安裝 CLI

**Homebrew（macOS / Linux）：**
```bash
brew install skillshare
```

:::note
Homebrew 的版本可能會落後幾天。想要最新版請改用安裝腳本。
:::

**安裝腳本（macOS / Linux）：**
```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

腳本預設安裝到 `~/.local/bin`，正常安裝與更新不需要 `sudo`。只有安裝器顯示 PATH 設定提示時，才需要依提示設定後再執行 `skillshare`。可把提示的指令加入 shell 設定檔（例如 `~/.zshrc` 或 `~/.bashrc`），讓之後開啟的終端機也能使用。可用 `INSTALL_DIR` 指定其他安裝位置。

既有安裝不會被搬移或刪除。如果 PATH 優先選到舊執行檔，安裝器會顯示它的位置；請把新安裝目錄放在 PATH 前面。自訂的受保護目錄仍可能需要 `sudo`。

如需整台電腦共用的安裝：

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | INSTALL_DIR=/usr/local/bin sh
```

**Windows（PowerShell）：**
```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

:::tip 之後如何更新
`skillshare upgrade` 會偵測你當初的安裝方式（Homebrew、腳本、手動），並就地更新 CLI。
:::

## 2. 初始化

```bash
skillshare init
```

<p>
  <img src="/img/init-with-mode.png" alt="Interactive init flow" width="720" />
</p>

`init` 會帶你完成四個選擇：

1. **Source 目錄** — 預設為 `~/.config/skillshare/skills/`。按 Enter 即可接受。
2. **Git remote** — 貼上你個人 skills repo 的 URL（例如 `git@github.com:you/skills.git`）。如果還沒有，先在 GitHub 上建一個空 repo；你也可以先略過，之後再加上 remote。
3. **Targets** — skillshare 會偵測已安裝的 AI CLI 並列出來。確認即可，或取消勾選你不想要的項目。
4. **內建 skill** — 選用。會加入 `/skillshare` 指令，讓你的 AI CLI 可以直接呼叫 skillshare。

### 選擇 sync 模式

`init` 接受 `--mode <merge|copy|symlink>`，用來設定新加入 targets 的預設值：

- `merge`（預設）— 逐一 skill 建立 symlink；target 上原有的本機 skills 會被保留
- `symlink` — 整個 target 目錄變成一個 symlink（最快，會取代整個目錄）
- `copy` — 實體檔案；變更會在下次 `sync` 時套用

之後可透過 `skillshare target <name> --mode <mode>` 針對個別 target 覆寫。

## 3. 安裝一個 skill

```bash
skillshare install anthropics/skills/skills/pdf
```

每次安裝都會執行安全稽核。Critical 等級的發現會擋下安裝；只有在你已檢視過並願意承擔風險時，才加上 `--force`。

## 4. Sync

```bash
skillshare sync
```

現在每個已設定的 target 都指向你的 source 了。

## 5. 驗證

```bash
skillshare status
```

```text
$ skillshare status

Source
─────────────────────────────────────────
✓ ~/.config/skillshare/skills (43 skills, 2026-09-28 12:39)
✓ ~/.config/skillshare/agents (2 agents, 2026-09-28 12:39)

Targets
─────────────────────────────────────────
claude
  skills   merged       [merge] ~/.claude/skills (43 shared, 0 local)
  agents   merged       [merge] 2/2 linked
cursor
  skills   merged       [merge] ~/.cursor/skills (43 shared, 0 local)
  agents   merged       [merge] 2/2 linked
gemini
  skills   merged       [merge] ~/.gemini/skills (43 shared, 0 local)
…
```

輸出會顯示 source 路徑與每個 target。`merge` 模式下已同步的 target 會標示為 `merged`，shared 數量也包含你剛安裝的 skill。

dashboard（`skillshare ui`）也能一眼看到同樣的狀態：

![首次同步後的 Dashboard：一個 source 連到每個 target，全部已同步](/img/web-dashboard-demo.png)

---

## 剛才發生了什麼事

1. **`init`** 建立了 `~/.config/skillshare/config.yaml` 與 `~/.config/skillshare/skills/`，自動偵測你的 AI CLI，並且 — 如果你提供了 remote — 從中 clone 既有的 skills。
2. **`install`** 把該 skill clone 進 source 目錄並執行安全稽核。`.metadata.json` 會記錄上游 URL 與 commit，`skillshare update` 之後才能拉取後續變更。
3. **`sync`** 套用各 target 所設定的模式。例如在 `merge` 模式下：
   ```
   ~/.claude/skills/pdf → ~/.config/skillshare/skills/pdf  (symlink)
   ```

在 `merge` 與 `symlink` 模式下，對 source 的編輯會立刻反映在每個 target；`copy` 模式則會在下次 `sync` 時套用。`merge` 與 `copy` 會保留 target 上原有的本機 skills；`skillshare backup` 會在破壞性操作前建立快照，`skillshare restore <target>` 可以還原。

只想讓某一個 target 使用不同模式？針對該 target 覆寫即可：

```bash
skillshare target <name> --mode copy
skillshare sync
```

完整的決策對照表請見 [Sync 模式](/docs/understand/sync-modes)。

---

## 變化型：在另一台機器上還原

你已經在別處使用 skillshare，而且 GitHub 上有個人的 skills repo。在新筆電、devcontainer 或 VM 上，四道指令就能還原一切 — 不需提示、不需選擇，重複執行也不會出問題：

```bash
# 1. 安裝 CLI（Homebrew 或 curl|sh — 與上面的步驟 1 相同）
brew install skillshare

# 2. Clone 你的 skills repo，並加入偵測到的 targets
skillshare init \
  --remote git@github.com:<you>/skills.git \
  --all-targets \
  --no-skill

# 3. 重新安裝 tracked 相依項目
#    （_ 前綴的目錄已被 gitignore，所以不會在 clone 下來的 repo 裡）
skillshare install https://github.com/<your-company>/skills --track --force

# 4. Sync
skillshare sync
```

`--no-skill` 會略過內建 skill 的提示；如果你想在這台機器上啟用，之後用 `skillshare upgrade --skill` 加上即可。

---

## 變化型：無 TTY 的無頭設定

對於 CI job、devcontainer 的 post-create hook，或雲端 VM 的佈建腳本，每個提示都有對應的非互動式 flag：

```bash
skillshare init \
  --source ~/.config/skillshare/skills \
  --remote https://github.com/<you>/skills \
  --targets codex \
  --mode merge \
  --no-copy \
  --no-skill

skillshare install https://github.com/<your-company>/skills --track --force
skillshare sync
```

| Flag | 作用 |
|---|---|
| `--source <path>` | 略過 source 路徑提示 |
| `--remote <url>` | 略過 remote 提示；若 remote 有內容則進行 clone |
| `--targets <name>` | 只加入列出的 targets（用 `--all-targets` 加入所有偵測到的） |
| `--mode merge` | 新 targets 的預設 sync 模式 |
| `--no-copy` | 略過「是否複製 target 既有 skills？」的提示，從空的開始 |
| `--no-skill` | 略過內建 skill 的提示 |

`--targets`、`--all-targets` 與 `--no-targets` 互斥 — 三選一。

---

## 接下來

- [建立你自己的 skill](/docs/how-to/daily-tasks/creating-skills)
- [跨機器同步](/docs/how-to/sharing/cross-machine-sync)
- [組織層級 skills](/docs/how-to/sharing/organization-sharing)
- [Agents](/docs/understand/agents) — 和 skills 一起管理單檔 `.md` agents
- [Sync 模式](/docs/understand/sync-modes) — 決策對照表與取捨
