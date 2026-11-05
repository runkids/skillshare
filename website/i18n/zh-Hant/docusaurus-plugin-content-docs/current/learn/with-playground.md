---
sidebar_position: 6
---

# 在 Playground 中試用 skillshare

> 一個預先設定好的 Docker sandbox，內含 demo skills、audit rules 與一個 project — 幾秒鐘內就能開始探索。

## 先決條件

- 已安裝 Docker 與 Docker Compose
- Clone skillshare repo：`git clone https://github.com/runkids/skillshare.git`

## 啟動 Playground

```bash
cd skillshare
make playground
```

這一個指令會：

1. 建置 sandbox 的 Docker image（包含 Go toolchain）
2. 在容器內編譯 `skillshare` 執行檔
3. 以 global mode 初始化，並自動偵測所有 targets
4. 建立跨類別的 demo skills（clean、warning、critical）
5. 建立一個 demo project，內含 project 層級的 skills 與自訂 audit rules
6. 帶你進入互動式 shell — 準備好開始探索

## 內含內容

### Demo Skills（Global）

| Skill | Category | Audit Findings |
|-------|----------|----------------|
| `audit-demo-clean` | root | 無（乾淨的基準） |
| `deploy-checklist` | `devops/` | 無 |
| `audit-demo-ci-release` | `security/` | HIGH + MEDIUM（sudo、外部 URL） |
| `audit-demo-debug-exfil` | `security/` | CRITICAL（憑證外洩） |
| `audit-demo-external-link` | `security/` | LOW（外部 URL） |
| `audit-demo-dangling-link` | `security/` | LOW（缺少連結）+ HIGH（超出根目錄） |

### Demo Project (`~/demo-project`)

一個預先設定好的 `.skillshare/` project，內含：
- `hello-world` — 乾淨的 project skill
- `demos/audit-demo-release` — 附帶 audit 警告的 release 輔助工具
- `guides/code-review` — 巢狀的 code review 指南
- 自訂的 `audit-rules.yaml`，內含 TODO/FIXME 規則政策

### 自訂 Audit Rules

Global 與 project 層級的 `audit-rules.yaml` 都已預先設定好，讓你能了解規則自訂如何運作 — 啟用/停用規則、新增自訂模式、設定 allowlist。

## 可以嘗試的操作

```bash
# 檢查已安裝的內容
skillshare status
skillshare list

# 執行安全性 audit — 查看各嚴重程度的結果
skillshare audit

# 試試 project mode
cd ~/demo-project
skillshare status          # 自動偵測 project mode
skillshare audit           # 使用自訂規則進行 project 層級掃描

# 啟動 web dashboard（port 19420）
skillshare-ui              # global mode
skillshare-ui-p            # project mode

# 探索巢狀 skills
ls ~/.config/skillshare/skills/security/
ls ~/.config/skillshare/skills/devops/
```

## Bare Mode

從一片空白開始 — 沒有自動 init，沒有 demo 內容：

```bash
./scripts/sandbox_playground_up.sh --bare
./scripts/sandbox_playground_shell.sh
```

適合用來從零開始測試 `skillshare init`。

## 停止 Playground

```bash
make playground-down
```

資料會保存在 Docker volume（`playground-home`）中。下次執行 `make playground` 時會從你離開的地方繼續。

## 架構

Playground 在一個經過安全性強化的 **唯讀（read-only）** Docker 容器中執行：

- `read_only: true` — 檔案系統不可變更，除了指定的 volumes
- `cap_drop: ALL` — 沒有任何 Linux capabilities
- `no-new-privileges` — 防止權限提升
- 可寫入的 volumes：`/sandbox-home`（持久化）、`/tmp`（tmpfs，256 MB）
- Port 19420 轉發給 web dashboard 使用

Workspace 是從主機 repo 以唯讀方式掛載 — 你可以在自己的機器上編輯程式碼，並在容器內重新建置。

## 接下來呢？

- [快速上手 →](/docs/getting-started)
- [安全性 audit 指南 →](/docs/how-to/advanced/security)
- [Docker sandbox 指南 →](/docs/how-to/advanced/docker-sandbox)
