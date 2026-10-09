---
sidebar_position: 3
---

# Sync Modes

skillshare 如何把 source 連結到 targets。

:::tip 這在什麼時候重要？
想要每個 skill 個別 symlink，並保留 target 中的本機 skills 時，選 merge mode（預設）。需要真實檔案而非 symlink（可攜性、CI，或個人偏好）時，選 copy mode。想要整個目錄都被連結、且不需要 target 專屬的本機 skills 時，選 symlink mode。
:::

## 總覽

| 模式 | 行為 | 使用情境 |
|------|----------|----------|
| `merge` | 每個 skill 個別建立 symlink | **預設。** 保留本機 skills。 |
| `copy` | 每個 skill 以真實檔案複製 | 可攜性、CI／沙盒環境，或你偏好真實檔案而非 symlink。 |
| `symlink` | 整個目錄是單一個 symlink | 各處都是完全相同的副本。 |

## 決策矩陣（中立版）

用這張表依你的限制條件來選擇，而非依 target 品牌名稱：

| 決策面向 | `merge` | `copy` | `symlink` |
|---|---|---|---|
| 跨不同 AI CLI 的相容性 | 中 | 高 | 低–中 |
| 編輯一次立即反映 | 高 | 低（需要 `sync`） | 高 |
| 磁碟使用量 | 低 | 高 | 低 |
| 防止從 target 誤刪的安全性 | 高 | 高 | 低 |
| 操作簡易度 | 中 | 中 | 高 |
| 依 target 過濾（`include`/`exclude`） | 支援 | 支援 | 不支援 |

如果不確定，先從 `merge` 開始，再視需要把特定 targets 改成 `copy`。

---

## Merge Mode（預設）

每個 skill 個別建立 symlink。Target 中的本機 skills 會被保留。

```
Source                          Target (claude)
─────────────────────────────────────────────────────────────
skills/                         ~/.claude/skills/
├── my-skill/        ────────►  ├── my-skill/ → (symlink)
├── another/         ────────►  ├── another/  → (symlink)
└── ...                         ├── local-only/  (preserved)
                                └── .skillshare-manifest.json
```

**優點：**
- 保留 target 專屬的 skills（不會被同步）
- 混用已安裝與本機的 skills
- 精細的控制
- 依 target 個別設定 include/exclude 過濾
- 以 manifest 為基礎的孤兒清理（在 uninstall 之後安全移除非 symlink 殘留）

:::info Project mode 中的相對 symlink
在 project mode（`-p`）中，symlink 會以**相對路徑**建立（例如 `../../.skillshare/skills/my-skill`）而非絕對路徑。這讓專案具有可攜性 — 移動或重新命名目錄後，symlink 仍然有效。在 global mode 中則使用絕對路徑，因為 source 與 targets 位於不同位置。
:::

**適用情境：**
- 你想要某些 skills 只出現在特定的 AI CLI 中
- 你想在同步前先試用本機 skills
- 你想要一個 source，但每個 target 有不同的 skill 子集

### Merge mode 中的過濾策略

`include` 與 `exclude` 會依以下順序，逐一 target 進行評估：
1. `include` 保留名稱相符的項目
2. `exclude` 從已保留的集合中移除

快速選擇：
- Target 只需要一小部分子集時，用 `include`
- Target 幾乎需要全部項目時，用 `exclude`
- 需要一個廣泛子集、但要明確排除特定項目時，用 `include + exclude`

規則變更時的行為：
- 先前已同步、但因規則變更而被過濾掉的 source-linked 項目，會在下次 `sync` 時被移除
- Target 中既有的本機非 symlink 資料夾會被保留

完整範例請見 [Target Configuration](/docs/reference/targets/configuration#include--exclude-target-filters)。

---

## Copy Mode

每個 skill 會以真實檔案複製到 target 目錄。`.skillshare-manifest.json` 檔案會追蹤哪些 skills 是受管理的以及它們的 checksum，因此本機 skills 會被保留。

```
Source                          Target (cursor)
─────────────────────────────────────────────────────────────
skills/                         ~/.cursor/skills/
├── my-skill/        ────copy►  ├── my-skill/    (real files)
├── another/         ────copy►  ├── another/     (real files)
└── ...                         ├── local-only/  (preserved)
                                └── .skillshare-manifest.json
```

### 為什麼要用 copy mode？

即使你的 AI CLI 能正確處理 symlink，copy mode 仍有其價值：

- **防禦性設計** — 並非每個 AI CLI 都保證支援 symlink，尤其是 Windows 上 symlink 行為會依平台與權限層級而異
- **沙盒環境** — 嚴格的 CI 流程、容器與氣隙（air-gapped）環境，可能不會跨檔案系統邊界跟隨 symlink
- **使用者偏好** — 有些使用者與團隊單純偏好真實檔案而非 symlink，考量透明度與可攜性

**優點：**
- 到處都能運作 — 不需要 AI CLI 或作業系統支援 symlink
- 保留本機 skills（與 merge mode 相同）
- 依 target 個別設定 include/exclude 過濾
- 以 checksum 為基礎跳過未變更的 skills

**適用情境：**
- 你的 AI CLI 回報「skill not found」，或無法讀取 symlink 的 skills
- 你想把 skills vendored 進專案 repo — project mode 下的 copy mode 讓團隊能把真實的 skill 檔案 commit 進 git，隊友不需要安裝 skillshare
- 你需要不依賴中央 source 也能運作的獨立 skill 目錄（可攜式設定、CI 流程、氣隙環境）
- 你想要跟 merge mode 一樣的過濾行為，但要真實檔案
- 常見適合改用 `copy` 的對象：`cursor`、`antigravity`、`copilot`、`opencode`

### 更新如何運作

每次 `skillshare sync` 時，會比對每個 source skill 的 checksum 與 manifest 中儲存的值：

- **Checksum 相同** → 略過該 skill（速度快）
- **Checksum 不同** → 用新版本覆寫
- **`--force`** → 無視 checksum，覆寫所有受管理的 skills

### Manifest 生命週期

Merge 與 copy 兩種模式都會寫入 `.skillshare-manifest.json` 以追蹤受管理的 skills：

- **Merge mode**：以 `"symlink"` 值記錄 skill 名稱 — 用於在 uninstall 後安全清理孤兒的真實目錄（例如 copy mode 留下的殘留）
- **Copy mode**：以 SHA-256 checksum 記錄 skill 名稱 — 用於增量同步與孤兒偵測
- 切換到 symlink mode 時會自動移除
- 若被手動刪除，下次 `sync` 會重建它

---

## Symlink Mode

整個 target 目錄是指向 source 的單一個 symlink。

```
Source                          Target (claude)
─────────────────────────────────────────────────────────────
skills/              ────────►  ~/.claude/skills → (symlink to source)
├── my-skill/
├── another/
└── ...
```

**優點：**
- 所有 targets 完全一致
- 管理更簡單
- 沒有孤兒 symlink

**適用情境：**
- 你想讓所有 AI CLI 擁有完全相同的 skills
- 你不需要 target 專屬的 skills

**警告：** 在 symlink mode 下，從 target 刪除會連 source 一起刪掉！
```bash
rm -rf ~/.claude/skills/my-skill  # ❌ Deletes from SOURCE
skillshare target remove claude   # ✅ Safe way to unlink
```

---

## 變更 Mode

### 依 target 個別設定

```bash
# Switch to copy mode (for AI CLIs that can't read symlinks)
skillshare target cursor --mode copy
skillshare sync

# Switch to symlink mode
skillshare target claude --mode symlink
skillshare sync

# Switch back to merge mode
skillshare target claude --mode merge
skillshare sync
```

### 依 target 覆寫（建議做法）

你不需要為每個 target 使用同一個全域 mode。常見的做法是：

```yaml
mode: merge
targets:
  claude:
    path: ~/.claude/skills
    # inherits merge
  cursor:
    path: ~/.cursor/skills
    mode: copy
  codex:
    path: ~/.codex/skills
    mode: symlink
```

當某個 target 需要優先考量相容性的行為（`copy`），而其他 target 想保持立即反映（`merge`/`symlink`）時，就用依 target 覆寫。

### 預設 Mode

在設定檔中為新的 targets 設定：

```yaml
# ~/.config/skillshare/config.yaml
mode: merge  # or symlink or copy

targets:
  claude:
    path: ~/.claude/skills
    # inherits default mode

  cursor:
    path: ~/.cursor/skills
    mode: copy  # real files for Cursor

  codex:
    path: ~/.codex/skills
    mode: symlink  # override default
```

---

## Target Naming

控制在 merge 或 copy mode 下，skill 目錄在 targets 中的命名方式。

| Naming | 行為 |
|--------|----------|
| `flat`（預設） | 巢狀 skills 以 `__` 分隔符扁平化：`frontend/dev` → `frontend__dev` |
| `standard` | 使用 SKILL.md 的 `name` 欄位：`frontend/dev` → `dev` |
| `prefixed` | 僅限 copy mode。在 `standard` 的基礎上，前面加上 tracked repo 名稱：`_mattpocock-skills/skills/prototype` → `mattpocock-skills-prototype` |

可全域設定或依 target 個別設定：

```yaml
target_naming: standard    # global default
targets:
  claude:
    skills:
      target_naming: flat  # per-target override
```

或透過 CLI：

```bash
skillshare target claude --target-naming standard
skillshare sync
```

**Standard mode** 遵循 [Agent Skills specification](https://agentskills.io/specification)，該規範要求 SKILL.md 的 `name` 欄位須與父層目錄名稱相符。合法的名稱最多 64 個字元，只能包含小寫字母（任何文字系統皆可）、數字與單一連字號，且不能以 `-` 開頭或結尾；不允許底線。名稱不合法或有名稱衝突的 skills 會顯示警告並被略過。

**Prefixed mode** 適用於含有同名 skill 的 tracked repo。在 `standard` 下，來自不同 repo 的兩個 `prototype` skill 會衝突而都被略過；在 `prefixed` 下，tracked repo 內的每個 skill 都會變成 `<repo>-<name>`，因此兩者都能送達工具。`<repo>` 是 tracked repo 資料夾名稱去掉開頭的 `_`、轉為小寫，並把字母與數字（任何文字系統）以外的字元替換為 `-` 的結果。加上前綴的名稱既用作資料夾名稱，也會寫入複製出的 `SKILL.md` 的 `name:`；source 不會被修改。

- 名稱已經以 repo 名稱開頭時，不會重複加前綴：`_bmad/skills/bmad-ux` 仍是 `bmad-ux`。
- 不在 tracked repo 內的 skill 維持原名。
- source skill 必須先通過 `standard` 檢查。加上前綴後的名稱超過 64 個字元時會顯示警告並被略過，仍然衝突的名稱與 `standard` 一樣被略過。tracked skill 無法在 `SKILL.md` 中改名，因此 sync 會建議重新命名另一個 skill，或以 `--name` 重新 track 該 repo。
- 想要更短的前綴，可以用較短的名稱 track repo：`skillshare install <repo> --track --name mp` 會得到 `mp-prototype`。
- 指向同層 skill 的相對連結（`../other-skill/`）不會被改寫，與 copy mode 下的 `flat` 相同。
- 該名稱就是工具中顯示的名稱；在 Claude Code 中它是 slash command，例如 `/mattpocock-skills-prototype`。

`prefixed` 需要 copy mode，因為 merge 連結指向 source，而 source 中的 `name:` 不能被修改。在 merge 或 symlink mode 下解析為 `prefixed` 的 target 會驗證失敗，並被 sync 略過。新 target 若會在 copy 以外的 mode 下繼承 `prefixed`（專案 target 預設為 merge），`target add` 會讓它使用 copy mode。`target add` 這麼做時會提示；`status` 和 `doctor` 會在 sync 之前標出在 copy 以外的 mode 下解析為 `prefixed` 的 target，`target list` 對 `targets:` 中的 target 也會如此，修正方式是在該 target 上設定 `mode: copy`；若該 target 由 `projects:` 項目展開而來，則設定 `projects.<root>.skills.mode: copy`。

```yaml
targets:
  universal:
    skills:
      mode: copy
      target_naming: prefixed
```

**Migration（遷移）**：在 `flat`、`standard` 與 `prefixed` 之間切換時，會就地重新命名既有的受管理項目。在 copy mode 下，manifest 會記錄每個複本是由哪種命名方式產生的，因此即使 source 沒有變動，被重新命名的複本也會重新複製，使其 `name:` 與新的命名方式一致。如果某個本機 skill 已佔用新名稱，原本的受管理項目會被保留。

**Symlink mode**：`flat` 和 `standard` 會被忽略 — 整個目錄會原封不動地被連結。`prefixed` 則如上所述會驗證失敗。

---

## Mode 比較

| 面向 | Merge | Copy | Symlink |
|--------|-------|------|---------|
| 保留本機 skills | ✅ 是 | ✅ 是 | ❌ 否 |
| Symlink 相容 | ✅ 是 | ❌ 真實檔案 | ✅ 是 |
| 所有 targets 一致 | ❌ 可能不同 | ❌ 可能不同 | ✅ 是 |
| 依 target include/exclude | ✅ 支援 | ✅ 支援 | ❌ 忽略 |
| 需要孤兒清理 | ✅ 是 | ✅ 是 | ❌ 否 |
| 刪除安全性 | ✅ 安全 | ✅ 安全 | ⚠️ 需謹慎 |
| 磁碟使用量 | 低（symlinks） | 較高（複製檔案） | 低（symlinks） |

---

## 孤兒清理

在 merge 與 copy 兩種模式下，`sync` 都會自動清理孤兒：

- **指向已刪除 source skills 的 symlink** 一律會被移除
- **真實目錄** 若出現在 `.skillshare-manifest.json` 中（先前由 skillshare 管理）會被移除
- **未知目錄**（不在 manifest 中）會被保留並顯示警告（假設為使用者自建）

這代表在 `uninstall` + `sync` 之後，即使是非 symlink 的殘留（例如先前 `copy` mode 留下的目錄）也會被安全清理。

```
$ skillshare sync
✓ claude    5 linked · 2 local · 1 pruned
✓ cursor    3 copied · 2 up to date · 1 pruned
```

:::info Agents 遵循相同的模式
所有三種模式（merge、copy、symlink）同樣適用於 agent 同步。Agent 的孤兒清理、依 target 的 include/exclude 過濾，以及 mode 轉換的行為都與 skills 相同 — 唯一的差別是 agents 是單一 `.md` 檔案，而非目錄。支援 agent 的 targets（Claude、Cursor、Augment、OpenCode）在各自的 `agents:` 子欄位中遵循相同的 `mode` 設定。詳情請見 [Agents](./agents.md)。
:::

---

## Extras Sync Modes

Extras（非 skill 的資源，如 rules、commands、prompts）同樣使用 merge 與 copy 模式。每個 extras target 都能指定自己的 mode：

```yaml
extras:
  - name: rules
    targets:
      - path: ~/.claude/rules          # merge (default): per-file symlinks
      - path: ~/.cursor/rules
        mode: copy                     # copy: real file copies
```

行為與 skill sync modes 相同 — merge 建立逐檔 symlink，copy 建立真實檔案複製。

:::note 沒有開啟開發人員模式的 Windows
Merge 模式會連結單一檔案，而 Windows 只有在開啟開發人員模式時才允許這麼做。沒有開啟時，merge 模式的 agents 與 extras 會改為複製，這些副本會像連結一樣被更新與清理。Skills 是資料夾，所以無論如何都會（以 junction）連結。請參閱 [Windows 疑難排解](/docs/troubleshooting/windows#file-links-need-windows-developer-mode-copying-instead)。

skillshare 不擁有、但內容相同的本機檔案會被保留。在 copy fallback 中，agent 計數會以 `local preserved` 分開顯示，例如 `0/1 linked, 1 local preserved`。
:::

---

## 另請參閱

- [sync](/docs/reference/commands/sync) — 執行 sync 以套用 mode 變更
- [target](/docs/reference/commands/target) — 變更某個 target 的 sync mode
- [Source & Targets](./source-and-targets.md) — 核心架構
- [Configuration](/docs/reference/targets/configuration) — 依 target 個別設定
