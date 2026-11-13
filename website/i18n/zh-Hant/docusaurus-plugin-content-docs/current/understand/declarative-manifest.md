---
sidebar_position: 8
---

# Declarative Skill Manifest

把你的 skill 收藏定義為程式碼 — 從單一 manifest 檔案安裝、分享並重現整個設定。

:::tip 這什麼時候重要？
當你想要跨機器可重現的 skill 設定、以單一指令完成團隊導入，或是開源專案的啟動流程時，就該使用 declarative manifest。
:::

## 什麼是 Skill Manifest？

Skill manifest 是你 skill 收藏的一份**可攜式宣告**。你不需要逐一手動安裝 skills，而是把它們列在一個 manifest 檔案中，然後執行 `skillshare install` 把所有內容一次建立起來。

Manifest 的位置取決於模式：

| 模式 | Manifest 位置 | 可 commit？ |
|------|------------------|-------------|
| **Project** | `.skillshare/config.yaml`（`skills:` 區段） | 可以 — commit 以分享給團隊 |
| **Global** | `~/.config/skillshare/skills/.metadata.json` | 不可 — 屬於個人機器狀態 |

### Project Mode Manifest

在 project mode 中，`skills:` 位於 `config.yaml` 中，與 `targets:` 並列：

```yaml
# .skillshare/config.yaml
targets:
  - claude
  - cursor

skills:
  - name: react-best-practices
    source: anthropics/skills/skills/react-best-practices
    group: frontend
  - name: _team-skills
    source: my-org/shared-skills
    tracked: true
  - name: commit
    source: anthropics/skills/skills/commit
```

這個檔案會被 commit 進 git — 隊友 clone repo 後執行 `skillshare install -p`，就能安裝所有列出的 skills。

Manifest 記錄的是*要安裝什麼*。每個 skill 實際解析到的確切 commit 則記錄在旁邊的 `.skillshare/skills.lock.json`，這個檔案是自動寫入的，也應該一併 commit。有了這兩個檔案，即使 upstream 之後已經往前推進，`skillshare install -p` 仍會讓每位隊友得到相同的 commit。詳見 [Lockfile](./project-skills.md#lockfile)。

### Global Mode Manifest

在 global mode 中，skill 記錄儲存在 `.metadata.json`（集中化的 metadata 儲存區）。這個檔案也包含執行期追蹤資料（雜湊值、時間戳記），並且是自動管理的。

## 運作方式

### 從 Manifest 安裝

在**不帶任何參數**的情況下執行 `skillshare install`，會讀取 manifest 並安裝所有列出的 skills：

```bash
# Global mode — installs all skills from ~/.config/skillshare/skills/.metadata.json
skillshare install

# Project mode — installs all skills from .skillshare/config.yaml skills: section
skillshare install -p

# Preview without installing
skillshare install --dry-run
```

已存在的 skills 會自動被略過。在 project mode 中，若某個 skill 已安裝的 commit 與 lockfile 不同，則會改為把它移動到釘選的 commit。

### 自動同步協調

Manifest 會與你實際的 skill 收藏保持同步：

- **`skillshare install <source>`** — 自動把安裝好的 skill 加入 manifest
- **`skillshare uninstall <name>...`** — 自動從 manifest 中移除該項目

在 project mode 中，更新的是 `config.yaml` 與 `skills.lock.json`。在 global mode 中，更新的是 `.metadata.json`。你永遠不需要手動編輯 manifest（雖然你也可以這麼做）。

這台機器尚未安裝的 skill（例如剛 clone 下來時）其項目會保留，`skillshare install` 仍然可以安裝它；只有解除安裝該 skill 或刪除已安裝的資料夾時，項目才會從 manifest 移除。

## Skill 項目欄位

`skills:` 清單中每個項目都有以下欄位：

| 欄位 | 必要 | 說明 |
|-------|----------|-------------|
| `name` | 是 | Skill 名稱（source 中的目錄名稱） |
| `source` | 是 | 安裝來源（GitHub 簡寫、HTTPS URL、SSH URL） |
| `tracked` | 否 | `true` 表示 tracked repository（保留 `.git`） |
| `group` | 否 | 子目錄路徑（例如 `frontend` 或 `frontend/vue`）。對應到安裝時的 `--into`。 |

## 使用情境

### 個人設定

跨機器維護你的個人 skill 收藏：

```bash
# On machine A — skills are already installed and tracked in registry
skillshare push   # backup config + registry to git

# On machine B — fresh machine
skillshare pull   # restore config + registry from git
skillshare install  # install all skills from manifest
skillshare sync   # distribute to all targets
```

### 團隊導入

新團隊成員只需一個指令就能取得相同的 AI context：

```bash
# .skillshare/config.yaml skills: section is committed to the repo
git clone <project-repo>
cd <project-repo>
skillshare install -p   # installs all declared skills
skillshare sync -p      # links to project targets
```

### 開源專案啟動流程

專案維護者在 `config.yaml` 中宣告建議的 skills：

```yaml
# .skillshare/config.yaml
targets:
  - claude
  - cursor

skills:
  - name: react-best-practices
    source: anthropics/skills/skills/react-best-practices
    group: frontend
  - name: commit
    source: anthropics/skills/skills/commit
```

:::info Group 欄位與 `--into`
當你使用 `--into` 安裝時，group 會自動被記錄下來：

```bash
skillshare install anthropics/skills/skills/pdf --into frontend -p
# config.yaml will contain: name: pdf, group: frontend
```

執行 `skillshare install -p`（不帶參數）會從 manifest 重新建立相同的目錄結構。
:::

貢獻者 clone 後執行 `skillshare install -p`，就能立刻取得專案專屬的 AI context。

## 工作流程總覽

```
Project mode:
1. Install skills normally      →  config.yaml skills: auto-updates
2. Commit config.yaml and skills.lock.json via git  →  same skills, same commits for the team
3. Run `skillshare install -p`  →  reproduce on clone
4. Run `skillshare sync`        →  distribute to all targets

Global mode:
1. Install skills normally      →  .metadata.json auto-updates
2. Push/pull config via git     →  portable across machines
3. Run `skillshare install`     →  reproduce on new machine
4. Run `skillshare sync`        →  distribute to all targets
```

## Extras 設定

除了 skills 之外，`config.yaml` 也能宣告 **extras** — 非 skill 資源（rules、commands、prompts），會同步到獨立的目錄。Extras 設定在 `config.yaml` 的 `extras:` 區段中（global 與 project 皆適用）：

```yaml
extras:
  - name: rules
    targets:
      - path: ~/.claude/rules
      - path: ~/.cursor/rules
        mode: copy
```

詳見 [sync extras](/docs/reference/commands/sync#sync-extras)。

## 相關文件

- [Install command](/docs/reference/commands/install) — 有無參數的 `skillshare install`
- [Push/Pull](/docs/reference/commands/push) — 透過 git 備份與還原 config
- [Project Skills](./project-skills.md) — Project 層級的 manifest


### 復原遠端 extra

可宣告 `remote: example-org/ai-hub/rules`、選用的 `branch: main` 和 `targets: []`。
`remote` 與 `source` 互斥。明確執行 `skillshare install example-org/ai-hub/rules --kind extra
--name team-rules -p`（一行），新增 target 後 sync。Project version 1 lockfile 的 `extras`
map 需要每位貢獻者升級至能保留它的二進位檔。無參數 install、pull、sync 不擷取 extras。
明確 install 的 `--update` 前移 pin；Global 使用相同 origin 的 metadata pin。參閱
[稽核涵蓋範圍、中斷修復與平台限制](/docs/reference/commands/install#remote-extras) 和
[sync extras](/docs/reference/commands/sync#sync-extras)。
