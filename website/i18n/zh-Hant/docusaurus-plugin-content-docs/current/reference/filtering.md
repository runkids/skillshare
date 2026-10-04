---
sidebar_position: 3
---

# 篩選機制參考

控制哪些 skills 能到達哪些 targets 的三層篩選機制完整規格說明。

:::tip 想找快速指南？
請參閱 [篩選 Skills](/docs/how-to/daily-tasks/filtering-skills)，取得情境導向的指南。
:::

## 總覽

| Layer | Scope | Where to set | Syntax | Evaluated at |
|-------|-------|-------------|--------|-------------|
| `.skillignore` | 對所有 targets 隱藏 | Source 目錄或 tracked repo 根目錄 | [gitignore](https://git-scm.com/docs/gitignore) | Discovery（探索階段） |
| SKILL.md `metadata.targets` | 將 skills 限制在列出的 targets | 每個 skill 的 frontmatter | YAML list | Sync（於 discovery 階段解析） |
| Agent `targets` | 將 agent 限制在列出的 targets | 每個 agent 的 frontmatter | YAML list | Sync（於 discovery 階段解析） |
| Target include/exclude | 依 target、依資源 | `config.yaml` 或 CLI flags | Go [`filepath.Match`](https://pkg.go.dev/path/filepath#Match) glob | Sync |

:::note Sync mode 注意事項
這三層篩選機制只適用於 **merge** 與 **copy** sync modes。
在 **symlink** mode 下，整個 source 目錄會被當作單一單位連結 — 逐個 skill 的篩選不會生效。
:::

## 評估順序與優先權

一個 skill 必須通過**所有**層級才能到達某個 target：

1. **`.skillignore`** — 在 discovery 階段評估。符合的 skills 永遠不會進入 sync pipeline。
2. **Target include/exclude** — 在 sync 階段評估（`FilterSkills`）。Skills 會被探索到，但對不符合的 targets 會被略過。
3. **SKILL.md `metadata.targets`** — 在 sync 階段評估（`FilterSkillsByTarget`）。Skills 會被限制在它們宣告的 targets 之內。

## .skillfollow

[`.skillfollow`](./skillfollow.md) 是探索 opt-in，不是另一層 filter。檔案置於 skills source 根目錄，宣告第一層外部群組/tracked repo 連結；local 檔為名稱聯集，不接受 gitignore patterns。加入連結的根目錄錨定無尾斜線 ignore 與 `/.skillfollow.local`；root/repo `.skillignore` 仍篩選 followed skills。見[完整設定與狀態](./skillfollow.md#states)。

## .skillignore

**位置：**
- Source 根目錄：`~/.config/skillshare/skills/.skillignore` — 套用於所有 skills
- Tracked repo 根目錄：`_team-repo/.skillignore` — 只套用於該 repo 之內，不限 source 中的深度（例如以 `--into group/sub` 安裝的 repo，或 followed 群組內的 repo）。Repo 巢狀時以最內層 repo 的規則為準

**語法：** 完整的 [gitignore](https://git-scm.com/docs/gitignore) 語法 — `*`（單一段落）、`**`（任意深度）、`?`、`[abc]`、`!pattern`（否定）、`/pattern`（錨定）、`pattern/`（僅限目錄）。

**`.skillignore.local`：** 放在 `.skillignore` 旁邊。其中的規則會附加在基礎檔案之後 — 以最後一個符合的規則為準。使用 `!pattern` 可以取消忽略。請勿將此檔案 commit 進版本控制。不含 `/` 的規則會符合任意深度的同名路徑，所以 `!feature-radar` 也會重新啟用 `feature-radar/feature-radar`；只想符合頂層資料夾時寫 `!/feature-radar`。Dashboard 會在 **Settings → Files** 顯示此檔案的規則，被它覆蓋的開關會回報為失敗。

**CLI 可見性：**

| Command | Output |
|---------|--------|
| `skillshare sync` | 數量 + skill 名稱 |
| `skillshare status --json` | `source.skillignore` 物件，內含規則與被忽略清單 |
| `skillshare doctor` | 規則數量與被忽略數量 |

📖 [File structure reference](/docs/reference/appendix/file-structure#skillignore-optional)

## SKILL.md targets 欄位 {#skillmd-targets-field}

**格式：** 可放在頂層，或巢狀於 `metadata` 之下：

```yaml
# 建議寫法
metadata:
  targets: [claude, cursor]

# 舊版相容寫法
targets: [claude, cursor]
```

**行為：** 白名單機制 — 該 skill 只會 sync 到列出的 targets。省略此欄位表示 sync 到所有 targets。若 `metadata.targets` 與頂層的 `targets` 同時存在，以 `metadata.targets` 為準。

**Tracked repos：** 在 dashboard 為 tracked repo 內的 skill 設定的 targets 會存在 source 的 `.metadata.json`，而不是它的 SKILL.md，因此 clone 保持乾淨，`update` 也能照常運作。這個設定優先於 skill 的 `metadata.targets`。

**別名：** Target 名稱支援別名。`claude` 會符合設定為 `claude-code` 的 target。參見 [Supported Targets](/docs/reference/targets/supported-targets)。

📖 [Skill format — targets field](/docs/understand/skill-format#targets)

**Agents** 透過 agent frontmatter 中頂層的 `targets` 清單，支援相同的白名單機制。沒有此欄位的 agent 會 sync 到每一個支援 agent 的 target。參見 [Agents — Agent File Format](/docs/understand/agents#agent-file-format)。

## Target include/exclude 篩選器 {#target-includeexclude-filters}

**透過 CLI 設定：**

```bash
# Skills
skillshare target claude --add-include "team-*"
skillshare target cursor --add-exclude "legacy-*"
skillshare target claude --remove-include "team-*"

# Agents
skillshare target claude --add-agent-include "team-*"
skillshare target claude --add-agent-exclude "draft-*"
skillshare target claude --remove-agent-include "team-*"
```

**儲存位置：** `config.yaml` 中的 `targets.<name>.include` / `targets.<name>.exclude`（skills 用），以及 `targets.<name>.agents.include` / `targets.<name>.agents.exclude`（agents 用）。

**語法：** Go [`filepath.Match`](https://pkg.go.dev/path/filepath#Match) glob 模式，比對的是扁平化後的資源名稱。Skills 使用扁平化的 skill 名稱（例如 `_team__frontend__ui`）；agents 使用扁平化的 `.md` 檔名。

| Supported | Not supported |
|-----------|--------------|
| `*`（任意字元） | `**`（遞迴） |
| `?`（單一字元） | `{a,b}`（大括號展開） |
| `[abc]`（字元類別） | |

**優先權：** 當 `include` 與 `exclude` 同時設定時，會先套用 `include`，再套用 `exclude`。若某個資源同時符合兩者，最終會被排除。

**視覺化編輯器：** `skillshare ui` → Targets 頁面 → 「Customize filters」按鈕。

📖 [Target command](/docs/reference/commands/target#target-filters-includeexclude) · [Filter behavior examples](/docs/reference/commands/sync#filter-behavior-examples) · [Configuration](/docs/reference/targets/configuration#include--exclude-target-filters)

## 另請參閱

- [篩選 Skills](/docs/how-to/daily-tasks/filtering-skills) — 情境導向的操作指南
