---
sidebar_position: 4
---

# 快速參考

skillshare 的指令速查表。

## 核心指令

| 指令 | 說明 |
|---------|-------------|
| `init` | 首次設定 |
| `install <source>` | 新增一個 skill |
| `uninstall <name>...` | 移除一個或多個 skills |
| `list` | 列出所有 skills |
| `search <query>` | 搜尋 skills |
| `sync` | 推送到所有 targets |
| `status` | 顯示 sync 狀態 |

## Skill 管理

| 指令 | 說明 |
|---------|-------------|
| `new <name>` | 建立新的 skill |
| `update <name>` | 更新一個 skill（git pull） |
| `update --all` | 更新所有 tracked repos |
| `check` | 檢查 skill 是否有更新 |
| `check --json` | 檢查更新（JSON 輸出） |
| `upgrade` | 升級 CLI 與內建 skill |
| `hub list` | 列出已設定的 skill hubs |
| `hub add <url>` | 新增一個 skill hub |

## Target 管理

| 指令 | 說明 |
|---------|-------------|
| `target list` | 列出所有 targets |
| `target <name>` | 顯示 target 詳細資訊 |
| `target <name> --mode <mode>` | 變更 sync 模式 |
| `target add <name> <path>` | 新增自訂 target |
| `target remove <name>` | 安全地移除 target |
| `diff [target]` | 顯示差異 |

## Extras 管理

| 指令 | 說明 |
|---------|-------------|
| `extras init <name> --target <path>` | 在設定中新增一筆 extras |
| `extras init <name> --file <file> --target <path>` | 新增只同步單一檔案的 extra（`--as` 可在 targets 改名） |
| `extras list` | 列出已設定的 extras 與其 sync 狀態 |
| `extras remove <name>` | 從設定中移除一筆 extras |
| `extras <name> --add-target <path>` | 為既有的 extras 項目新增一個 target |
| `extras <name> --remove-target <path>` | 移除一個 target（加上 `--prune` 可一併刪除已同步的檔案） |
| `extras collect <name>` | 把 extras target 中的本機檔案收集回 source |

## Agent 管理

| 指令 | 說明 |
|---------|-------------|
| `list agents` | 列出已安裝的 agents |
| `install <source> --kind agent` | 只安裝 repo 中的 agents |
| `install <source> -a <name>` | 依名稱安裝特定的 agent |
| `uninstall --kind agent <name>` | 移除一個 agent |
| `sync agents` | 只把 agents 同步到 targets |
| `check agents` | 檢查 agents 是否有更新 |
| `audit agents` | 對 agents 進行安全掃描 |
| `enable --kind agent <name>` | 重新啟用已停用的 agent |
| `disable --kind agent <name>` | 透過 `.agentignore` 停用 agent |

## Plugin 管理

| 指令 | 說明 |
|---------|-------------|
| `plugin` | 開啟互動式 plugin 管理介面 |
| `plugin list` | 顯示已管理的 plugins 與原生安裝狀態 |
| `plugin discover <source>` | 檢視一個目錄或 Git repository |
| `plugin add [source]` | 安裝完整的原生 plugin |
| `plugin import [plugin@market] --from claude` | 接管既有的原生安裝 |
| `plugin inspect <name>` | 檢視已管理的套件 |
| `plugin check [name]` | 檢查來源變更但不套用 |
| `plugin update [name] --target claude` | 從已檢視過的來源更新支援的 target |
| `plugin enable [name] --target codex` | 為下次 sync 選取一個 target |
| `plugin disable [name] --target codex` | 為下次 sync 取消選取一個 target |
| `plugin remove [name]` | 解除安裝已管理的綁定並移除定義 |
| `sync plugins [name]` | 套用 plugin 的 sync 選取結果；`plugin sync` 的別名 |

Targets：Claude Code、Codex、Cursor、Antigravity（`agy`）、Pi 與 OpenCode。
Project mode 支援 Claude、Antigravity、Pi 與 OpenCode。

Enable/disable 只會儲存選取結果。下一次 plugin sync 才會安裝已選取的綁定，
或解除安裝取消選取的綁定，同時保留其定義。
Plugins 不包含在 `sync --all` 之中。可用 `--dry-run --json` 預覽變更；
自動化情境請搭配明確輸入使用 `--no-tui`。原生用戶端需求與支援的 targets 請見 [plugin](/docs/reference/commands/plugin)。

## Sync 操作

| 指令 | 說明 |
|---------|-------------|
| `sync extras` | 同步非 skill 資源（rules、commands 等） |
| `sync mcp` | 同步 MCP 連線設定 |
| `sync --all` | 同步 skills + agents + extras + MCP + hooks（不含 plugins） |
| `collect <target>` | 把 target 的 skills 收集回 source |
| `collect --all` | 從所有 targets 收集 |
| `backup [target]` | 建立備份 |
| `backup --list` | 列出備份 |
| `restore <target>` | 從備份還原 |
| `commit [-m "msg"]` | 建立本機 git commit 但不 push |
| `push [-m "msg"] [--pull]` | Commit 並 push 到 git remote；`--pull` 會先合併 remote 的變更，然後 sync |
| `pull` | 從 git pull 並 sync |
| `trash list` | 列出軟刪除的 skills |
| `trash restore <name>` | 還原軟刪除的 skill |

## 工具指令

| 指令 | 說明 |
|---------|-------------|
| `analyze` | 分析 context window 使用量（互動式 TUI） |
| `analyze --filter <text>` | 依名稱／路徑子字串過濾 skills |
| `analyze --json` | 以 JSON 輸出 context 使用量 |
| `doctor` | 診斷問題 |
| `doctor --json` | 診斷問題（供 CI 使用的 JSON 輸出） |
| `log` | 檢視操作與稽核日誌 |
| `ui` | 在 `localhost:19420` 啟動網頁儀表板 |
| `ui -p` | 以 project mode 啟動網頁儀表板 |
| `completion <shell> --install` | 安裝 shell tab 補完（bash/zsh/fish/powershell/nushell） |
| `version` | 顯示 CLI 版本 |
| `make test-docker` | 執行離線 Docker sandbox 測試 |
| `make playground` | 啟動 playground 並進入 shell（一步完成） |
| `make playground-down` | 停止並移除 playground |
| `./scripts/sandbox.sh <cmd>` | 進階 sandbox 管理（up/down/shell/reset/status/logs/bare） |
| `make ui-build` | 建置前端 |
| `make build-all` | 建置含前端的完整執行檔 |

---

## 常見工作流程

### 安裝並同步一個 skill
```bash
skillshare install anthropics/skills/skills/pdf
skillshare sync
```

### 建立並部署一個 skill
```bash
skillshare new my-skill
# 編輯 ~/.config/skillshare/skills/my-skill/SKILL.md
skillshare sync
```

### 跨機器同步
```bash
# 設定（擇一）
# 互動式（逐步提示）
skillshare init --remote git@github.com:you/my-skills.git

# 非互動式（無提示，自動偵測已安裝的 targets）
skillshare init --remote git@github.com:you/my-skills.git --no-copy --all-targets --no-skill

# 選用：建立本機檢查點但不 push
skillshare commit -m "Save local skill edits"

# 機器 A：推送變更
skillshare push -m "Add new skill"

# 機器 B：拉取並同步
skillshare pull

# 在多台機器上編輯：一個指令完成雙向同步
skillshare push --pull -m "Update skills"
```

之後才需要的選用步驟（僅在設定完成後又安裝了其他 AI CLI 時）：

```bash
skillshare init --discover
```

在 discover 期間覆寫模式時，只會影響新加入的 targets：

```bash
skillshare init --discover --select cursor --mode copy
```

### 團隊 skill 共享
```bash
# 安裝團隊 repo
skillshare install github.com/team/skills --track

# 從特定 branch 安裝（有無 --track 皆可）
skillshare install github.com/team/skills --branch develop --all
skillshare install github.com/team/skills --track --branch develop

# 釘選到 tag 或 commit SHA（僅限一般安裝）
skillshare install github.com/team/skills --branch v1.2.0 --all

# 從團隊端更新
skillshare update --all
skillshare sync
```

### Sandbox playground 使用流程
```bash
make playground          # 啟動並進入 shell
skillshare --help
ss status
exit                     # 離開 shell
make playground-down     # 停止容器
```

---

## 重要路徑

| 路徑 | 說明 |
|------|-------------|
| `~/.config/skillshare/config.yaml` | 設定檔 |
| `~/.config/skillshare/skills/.metadata.json` | 已安裝 skill 的 metadata（自動管理） |
| `~/.config/skillshare/skills/` | Skill source 目錄 |
| `~/.config/skillshare/agents/` | Agent source 目錄 |
| `~/.config/skillshare/extras/<name>/` | Extras source 目錄 |
| `~/.local/state/skillshare/logs/` | 操作與稽核日誌 |
| `~/.local/share/skillshare/backups/` | 備份目錄 |

---

## 多數指令都支援的 Flags

| Flag | 說明 |
|------|-------------|
| `--dry-run`、`-n` | 預覽而不實際變更 |
| `--help`、`-h` | 顯示說明 |

---

## 延伸閱讀

- [指令參考](/docs/reference/commands) — 完整指令文件
- [核心概念](/docs/understand) — 核心概念說明
