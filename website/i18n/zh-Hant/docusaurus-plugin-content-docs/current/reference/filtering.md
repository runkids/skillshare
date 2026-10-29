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

## .skillignore

**位置：**
- Source 根目錄：`~/.config/skillshare/skills/.skillignore` — 套用於所有 skills
- Tracked repo 根目錄：`_team-repo/.skillignore` — 只套用於該 repo 之內

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

### Tracked repo 中的 Agent 專用變體 {#agent-specific-variants-in-tracked-repositories}

```text
_team/internal/skills/claude/review/SKILL.md
_team/internal/skills/codex/review/SKILL.md
```

在 tracked repo 中，`skills/<已知-agent>/<完整技能路徑>/SKILL.md` 會自動識別為 Agent 專用變體，例如 `_team/internal/skills/{claude,codex}/review/SKILL.md`。邏輯 Skill 由倉庫、Agent 前的完整路徑與 Agent 後的完整路徑共同決定；即使只剩一個變體也會依 Agent 路由。`--into` 下的倉庫也適用。

**merge/copy：** 僅選擇符合 Agent 的檔案，支援 `claude-code` 等註冊別名以及帳戶 Target 的 `agent: codex`。共用目錄不代表同一 Agent，`universal` 不會自動接收 Codex 變體。找不到符合項目時不選其他變體；同一組中有多個符合同一 Agent 的別名變體時會略過並提示，須以篩選器或覆寫設定明確選擇。

路由優先順序為：Source 的 `.metadata.json` Target 覆寫 > 非空 frontmatter targets > 變體 Agent 推斷 > 原有主機路徑推斷。空的 Source 覆寫表示所有 Target；明確宣告的 Target 保留原有比對規則。`.skillignore` 與 include/exclude 仍依實體路徑生效。未知 Agent、`shared`/`default` 及 tracked repo 外的配置保留一般探索行為，不推斷備用變體。

`status` 與 dashboard 的 Source/倉庫總數依邏輯 Skill 計數：兩個 Skill 的四份檔案顯示為 `2 skills, 4 variants`。JSON 的 `skill_count`/`skillCount` 是邏輯數量；`variant_count`/`variantCount` 及 Target API 的 `sourceVariantCount` 是啟用的實體檔案總數，也包含一般 Skill。資源清單、操作 ID、Sync 結果與上下文成本仍依實體檔案計算。

**symlink：** 整個目錄會暴露給 Target，無法依變體路由；需要選擇 Agent 時請使用 merge/copy。共用目標目錄的 Target 也無法隔離不同選擇，原有路徑衝突提示仍適用。Source 檔案保持不變；既有 prefixed copy 命名仍可能改寫副本的 `name:`。此功能不執行 sparse checkout。

升級後，merge/copy 會移除已管理但 Agent 不符的目標變體，並透過 CLI/伺服器警告說明自動路由原因（dry-run 也會提示）。使用 Source 的 Target 覆寫可還原所需副本；空覆寫表示所有 Target。Source 檔案保持完整。Target 欄顯示有效路由（包含推斷），不代表提供者宣告了該欄位。

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
