---
sidebar_position: 4
---

# FAQ

關於 skillshare 的常見問題。

## 一般問題

### 這不就是 `ln -s` 嗎？

從核心來說，是的。但 skillshare 還處理了：
- 多 Target 偵測
- 備份／還原
- Merge 模式（個別 Skill 的 symlink）
- 跨裝置同步
- 損壞 symlink 的復原

所以你不用自己動手。

### 如果我在 Target 目錄中修改一個 Skill 會發生什麼事？

由於 Target 是 symlink，變更會直接套用到 Source。所有 Target 會立刻看到這個變更。

### 我要如何保留一個 CLI 專屬的 Skill？

使用 `merge` 模式（預設）。Target 中的本地 Skill 不會被覆寫或同步走。

```bash
skillshare target claude --mode merge
skillshare sync
```

然後直接在 `~/.claude/skills/` 中建立 Skill — 它們不會被動到。

### 我使用 dotfiles 管理工具（stow/chezmoi/yadm） — skillshare 會弄壞我的 symlink 嗎？

不會。Skillshare 會偵測 Source 與 Target 目錄上的外部 symlink，並保留它們。所有指令 — sync、update、uninstall、list、diff、install — 都會解析 symlink 並對底層目錄操作，而不會移除 symlink 本身。詳情請參閱 [Dotfiles 管理工具相容性](/docs/reference/commands/sync#dotfiles-manager-compatibility)。

如果你透過 dotfiles 對 `config.yaml` 做版本控制，可以考慮啟用 `preserve_tilde_on_save: true`，讓路徑保持 `~/...` 的形式而非絕對路徑 — 詳見[設定](/docs/reference/targets/configuration#preserve_tilde_on_save)。

---

## 安裝

### 我可以把 Skill 同步到自訂或不常見的工具嗎？

可以。用該工具的 Skill 目錄搭配 `skillshare target add <name> <path>`。

```bash
mkdir -p ~/.myapp/skills
skillshare target add myapp ~/.myapp/skills
skillshare sync
```

### 我可以搭配私人的 git 儲存庫使用 skillshare 嗎？

可以。使用 SSH URL：

```bash
skillshare init --remote git@github.com:you/private-skills.git
```

---

## Sync

### 為什麼我在每次 install/update 之後都要執行 `sync`？

Sync 被刻意設計成一個獨立的步驟。像 `install`、`update` 和 `uninstall` 這樣的操作只會修改 **Source** 目錄 — `sync` 才會把這些變更套用到所有 Target。

這讓你可以：
- **批次處理變更** — 安裝 5 個 Skill 後只同步一次，而不是同步 5 次
- **先預覽** — 套用前先執行 `sync --dry-run`
- **保持掌控** — 由你決定何時更新 Target

**注意：** `pull` 是唯一會自動同步的指令，因為它的用意就是「讓一切保持最新」。

完整的設計理由請參閱[為什麼 Sync 是獨立的步驟](/docs/understand/source-and-targets#why-sync-is-a-separate-step)。

### 我要如何跨多台機器同步？

使用以 git 為基礎的跨機器同步：

```bash
# 機器 A：push 變更
skillshare push -m "Add new skill"

# 機器 B：pull 並同步
skillshare pull
```

完整設定請參閱[跨機器同步](/docs/how-to/sharing/cross-machine-sync)。

### 如果我不小心透過 symlink 刪除了一個 Skill 怎麼辦？

如果你已初始化 git（建議這麼做），可以用以下方式復原：

```bash
cd ~/.config/skillshare/skills
git checkout -- deleted-skill/
```

或從備份還原：
```bash
skillshare restore claude
```

### 如果我不小心解除安裝了一個 Skill 怎麼辦？

解除安裝的 Skill 會被移到垃圾桶並保留 7 天。用以下方式還原：

```bash
skillshare trash list                  # 查看垃圾桶內容
skillshare trash restore my-skill      # 還原到 Source
skillshare sync                        # 同步回 Target
```

如果該 Skill 是從遠端 Source 安裝的，你也可以重新安裝：

```bash
skillshare install github.com/user/repo/my-skill
skillshare sync
```

在 Project mode 下，垃圾桶位於專案目錄下的 `.skillshare/trash/`。使用垃圾桶相關指令時要加上 `-p` 旗標。

執行 `skillshare doctor` 可查看目前垃圾桶的狀態（項目數、大小、存放時間）。

### backup 和 trash 有什麼差別？

| | backup | trash |
|---|---|---|
| **保護對象** | Target 目錄（同步快照） | Source Skill（uninstall） |
| **位置** | `~/.local/share/skillshare/backups/` | `~/.local/share/skillshare/trash/` |
| **觸發時機** | `sync`、`target remove` | `uninstall` |
| **還原方式** | `skillshare restore <target>` | `skillshare trash restore <name>` |
| **自動清理** | 手動（`backup --cleanup`） | 7 天 |

兩者是互補的關係 — backup 保護 Target 不受同步變更影響，trash 保護 Source Skill 不被意外刪除。

### 我可以把特定 Skill 同步到特定 CLI 嗎？

可以。例如，Skill A 只同步到 Claude，Skill B 同步到 Antigravity 和 Codex，Skill C 同步到全部：

**選項 1：SKILL.md 中的 `targets` 欄位**（由 Skill 作者設定）

```yaml
# skills/skill-a/SKILL.md
---
name: skill-a
targets: [claude]
---
```

```yaml
# skills/skill-b/SKILL.md
---
name: skill-b
targets: [antigravity, codex]
---
```

```yaml
# skills/skill-c/SKILL.md — 沒有 targets 欄位 = 同步到全部
---
name: skill-c
---
```

**選項 2：設定中的 `include`/`exclude` 篩選條件**（由使用端設定）

```yaml
# ~/.config/skillshare/config.yaml
targets:
  claude:
    path: ~/.claude/skills
    include: [skill-a, skill-c]
  codex:
    path: ~/.codex/skills
    include: [skill-b, skill-c]
```

兩種做法可以合併使用 — 設定端的篩選條件會先套用，接著才是 Skill 層級的 `targets` 欄位。請參閱 [Skill 格式 — `targets`](/docs/understand/skill-format#targets) 與[設定 — 篩選條件](/docs/reference/targets/configuration#skill-level-targets)。

---

## Targets

### 搭配 npx skills 使用 universal {#using-universal-alongside-npx-skills}

`universal` Target 指向 `~/.agents/skills`，與 [npx skills CLI](https://github.com/vercel-labs/skills) 使用的是同一個目錄。兩個工具可以同時管理這個目錄，但有一些注意事項：

**可行的部分：**
- 在 merge 模式（預設）下，skillshare 會在 `~/.agents/skills/` 中建立 **symlink**；npx skills 則建立**真實目錄**。只要 Skill 名稱不衝突，兩者可以共存。
- skillshare 的 prune 邏輯只會移除自己管理的項目 — 不會刪除由 npx skills 安裝的檔案。
- Agent CLI（Claude Code、Pi 等）會直接讀取目錄，所以它們能看到來自兩個工具的 Skill。

**需要注意的地方：**
- **名稱衝突** — 如果兩個工具安裝了同名的 Skill，最後一次 sync/install 會勝出。避免用兩個工具安裝同一個 Skill。
- **Copy 模式較為激進** — 在 copy 模式下（`skillshare target universal --mode copy`），skillshare 會在每次同步時覆寫受管理的目錄。如果 npx skills 在兩次同步之間修改了同名的 Skill，skillshare 會將它取代掉。Merge 模式（預設）只會建立 symlink，對共存來說更安全。
- **`npx skills list` 不會顯示 skillshare 的 Skill** — npx skills CLI 是透過 lock 檔案（`~/.agents/.skill-lock.json`）追蹤安裝項目，而非掃描目錄。由 skillshare 同步的 Skill 不會出現在 `npx skills list -g` 中，但 Agent CLI **看得到**它們。
- **其他 Agent 專屬的 Target 仍然有用** — `universal` 與 `claude` 指向不同的路徑（`~/.agents/skills` 對比 `~/.claude/skills`）。同時選用兩者是安全且不重複的。

**建議的工作流程：**
```bash
# 把 skillshare 當成你的主要 Skill 管理工具
skillshare install github.com/user/skills --track
skillshare sync

# npx skills 只用於不需要同步的一次性社群安裝
npx skills add someone/skill -g
```

:::tip
為了與 npx skills 最安全地共存，請讓 universal Target 保持在 **merge 模式**（預設）。除非你完全不使用 npx skills，否則避免切換到 copy 模式。
:::

### 我曾把 `claude-code`（或 `gemini-cli` 等）當作 Project Target 使用 — 這樣還有效嗎？

有效。像 `claude-code`、`gemini-cli`、`github-copilot` 這種舊的 Project Target 名稱，仍然可以透過別名解析。例如，`gemini-cli` 會解析為 `gemini`。我們建議你更新 `.skillshare/config.yaml`，改用正式名稱：

```yaml
# 之前
targets:
  - claude-code

# 之後
targets:
  - claude
```

### `target remove` 是如何運作的？安全嗎？

是的，它是安全的：

1. **備份** — 建立該 Target 的備份
2. **偵測模式** — 檢查是 symlink 還是 merge 模式
3. **解除連結** — 移除所有由 skillshare 管理的 symlink，並把 Source 內容以真實檔案的形式複製回去。在 merge 模式下，只會移除指向 Source 目錄的 symlink；本地（非 symlink）的 Skill 會被保留。
4. **更新設定** — 從 config.yaml 中移除該 Target

這就是為什麼 `skillshare target remove` 是安全的，而 `rm -rf ~/.claude/skills` 會刪除你的 Source 檔案。

### 為什麼在 Target 上執行 `rm -rf` 很危險？

在 symlink 模式下，整個 Target 目錄就是指向 Source 的一個 symlink。刪除它就等於刪除 Source。

在 merge 模式下，每個 Skill 都是一個 symlink。透過 symlink 刪除一個 Skill，就會刪除 Source 檔案。

**請務必使用：**
```bash
skillshare target remove <name>   # 安全
skillshare uninstall <skill>      # 安全
```

---

## Tracked Repos

### Tracked repo 與一般 Skill 有什麼不同？

| 面向 | 一般 Skill | Tracked Repo |
|--------|---------------|--------------|
| Source | 複製到 Source | 連同 `.git` 一起 Clone |
| 更新 | `install --update` | `update <name>`（git pull） |
| 前綴 | 無 | `_` 前綴 |
| 巢狀 Skill | 攤平 | 用 `__` 攤平 |

### 為什麼要有底線前綴？

`_` 前綴用來識別 Tracked 儲存庫：
- 幫助你與一般 Skill 區分
- 避免名稱衝突
- 在清單中清楚顯示

---

## Skills

### SKILL.md 的格式是什麼？

```markdown
---
name: skill-name
description: Brief description
---

# Skill Name

Instructions for the AI...
```

完整細節請參閱 [Skill 格式](/docs/understand/skill-format)。

### "unknown target" 警告是什麼意思？

執行 `skillshare check` 或 `skillshare doctor` 時，你可能會看到：

```
! Skill targets: my-skill: unknown target "*"
```

這代表該 Skill 的 `SKILL.md` frontmatter 中的 `targets` 欄位，包含一個無法辨識的名稱 — 通常是 `"*"`（萬用字元）。skillshare 期望的是**精確的 Target 名稱**（例如 `claude`、`cursor`、`codex`），而不是 glob 樣式。

**如果你想讓某個 Skill 同步到所有 Target**，完全省略 `targets` 欄位即可：

```yaml
---
name: my-skill
description: Works everywhere
# 沒有 targets 欄位 = 同步到所有 Target
---
```

**如果這個警告來自第三方 Skill**，代表該 Skill 作者使用了不支援的語法。你可以：
1. **忽略警告** — 該 Skill 仍會安裝，只是不會自動篩選到特定 Target
2. **Fork 並修正** — 移除或修正該 Skill 的 `SKILL.md` 中的 `targets` 欄位

完整規範請參閱 [Skill 格式 — `targets`](/docs/understand/skill-format#targets)。

### 一個 Skill 可以有多個檔案嗎？

可以。一個 Skill 目錄可以包含：
- `SKILL.md`（必要）
- 任何額外的檔案（範例、範本等）

在你的 SKILL.md 指令中參照它們即可。

---

## 效能

### Sync 感覺很慢

檢查你的 Skill 目錄中是否有大型檔案。新增忽略樣式：

```yaml
# ~/.config/skillshare/config.yaml
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
  - "**/node_modules/**"
  - "**/*.log"
```

### 我可以有多少個 Skill？

沒有硬性限制。效能取決於：
- Skill 的數量
- Skill 檔案的大小
- Target 的數量

數千個小型 Skill 也能正常運作。

---

## 備份

### 備份儲存在哪裡？

```
~/.local/share/skillshare/backups/<timestamp>/
```

### 備份會保留多久？

預設情況下，會無限期保留。可用以下方式清理：
```bash
skillshare backup --cleanup
```

---

## Agents

### Agent 和 Skill 有什麼差別？

Skill 是包含 `SKILL.md` 檔案的**目錄**（可選擇性地附帶輔助檔案、範例、範本）。Agent 則是帶有 frontmatter 的**單一 `.md` 檔案** — 沒有巢狀結構。兩者都支援 install、sync、audit、check、backup 和 trash。

完整比較與 Agent 檔案格式請參閱 [Agents](/docs/understand/agents)。

### 哪些 Target 支援 Agent？

開箱即用支援的有：`claude`、`cursor`、`augment`、`opencode`（以及 `universal` 別名）。其他 Target 在 Agent 同步時會被靜默跳過，並顯示 `No agents folder: <targets>` 警告。你可以透過編輯 `config.yaml`，手動新增 Agent 路徑：

```yaml
targets:
  myapp:
    path: ~/myapp/skills
    agents:
      path: ~/myapp/agents
```

### 我要如何停用單一 Agent 而不刪除它？

使用 `disable` 指令（或直接編輯 `.agentignore`）：

```bash
skillshare disable my-agent --kind agent     # 在 .agentignore 中新增項目
skillshare enable my-agent --kind agent      # 移除該項目
```

`.agentignore` 位於 Agent Source 根目錄下（Global 時為 `~/.config/skillshare/agents/.agentignore`，Project mode 時為 `.skillshare/agents/.agentignore`），並使用 [gitignore 語法](https://git-scm.com/docs/gitignore)。也支援 `.agentignore.local` 覆蓋層，用於本地專屬的覆寫設定。

### 我可以在 Project mode 下備份 Agent 嗎？

可以 — 而且**只能**備份 Agent。`backup` 在 Project mode 下不允許用於 Skill，但 Agent 流程是明確的例外：

```bash
skillshare backup -p agents     # 備份 Project Agent Target
skillshare backup -p --all      # 效果相同；--all 在 Project mode 下會縮小範圍到 Agent
```

如果忘記加上 `agents` 篩選條件，你會看到 `backup is not supported in project mode (except for agents)`。`restore` 也套用相同規則。Agent 備份會存放在一般 Skill 備份旁邊的 `<target>-agents/` 底下。

---

## 安全性

### 我可以信任第三方 Skill 嗎？

Skill 是給你的 AI Agent 的指令 — 惡意的 Skill 可能會指示 AI 外洩機密資訊或執行破壞性指令。skillshare 內建了安全掃描器來降低這類風險：

- **安裝時自動掃描** — 每個 Skill 在 `skillshare install` 時都會被掃描
- **CRITICAL 發現會被阻擋** — Prompt injection、資料外洩、憑證存取預設會被阻擋
- **手動掃描** — 隨時可執行 `skillshare audit` 來掃描所有已安裝的 Skill

完整偵測樣式清單請參閱 [audit 指令](/docs/reference/commands/audit)。

### 如果 audit 阻擋了我的安裝怎麼辦？

如果某個 Skill 觸發了 CRITICAL 等級的發現，安裝就會被阻擋。你有兩個選項：

1. **檢視發現的問題** — 確認是否為誤判（例如文件範例）
2. **強制安裝** — 如果你信任此 Source，使用 `--force` 略過檢查

```bash
skillshare install suspicious-skill --force
```

### audit 能抓到所有問題嗎？

沒有任何掃描器是完美的。`skillshare audit` 能抓到常見的樣式，例如 Prompt injection、帶有機密資訊的 `curl`/`wget`、憑證檔案存取，以及被混淆的 payload。對於來自不受信任 Source 的 Skill，請務必手動檢視。

---

## 取得協助

### 我要在哪裡回報臭蟲？

[GitHub Issues](https://github.com/runkids/skillshare/issues)

### 我要在哪裡提問？

[GitHub Discussions](https://github.com/runkids/skillshare/discussions)

---

## 相關文件

- [常見錯誤](./common-errors.md) — 錯誤解決方法
- [Windows](./windows.md) — Windows 專屬 FAQ
- [疑難排解流程](./troubleshooting-workflow.md) — 一步步除錯
