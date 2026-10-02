---
sidebar_position: 2
---

# 安全優先的設計

> AI skills 是可執行的指令。skillshare 把它們視為不受信任的輸入。

## 威脅模型

當你從 GitHub 安裝一個 skill 時，你等於是在給 AI 工具一組會影響程式碼產生、檔案修改，甚至可能是命令執行的指令。惡意的 skill 可能會：

- **注入 prompt**，覆寫 AI 的安全準則
- **外洩資料**，指示 AI 把檔案內容傳送到外部 URL
- **透過 AI 的 shell 存取權執行破壞性命令**
- **竊取憑證**，存取環境變數或設定檔

這並非空想。Prompt injection 是 AI 工具鏈中頭號安全隱憂。

## Audit Engine

skillshare 內建一個安全掃描器（`skillshare audit`），會拿每個已安裝的 skill 比對 15 種以上、涵蓋 5 個嚴重程度等級的偵測模式：

| 嚴重程度 | 範例 |
|----------|----------|
| CRITICAL | Prompt injection、system prompt 覆寫 |
| HIGH | 資料外洩 URL、憑證存取模式 |
| MEDIUM | 破壞性命令（`rm -rf`、`DROP TABLE`）、檔案系統寫入 |
| LOW | 網路請求、外部工具呼叫 |
| INFO | 檔案過大、格式異常 |

### 運作原理

Audit engine 使用模式比對與啟發式方法掃描 SKILL.md 內容：

```bash
# Scan all installed skills
skillshare audit

# JSON output for CI integration
skillshare audit --json

# Scan project skills only
skillshare audit -p
```

### 自動封鎖

在 `skillshare install` 期間，audit 會自動執行。若偵測到 CRITICAL 發現，安裝就會被封鎖：

```
CRITICAL: Prompt injection detected in "malicious-skill"
  → Pattern: "ignore previous instructions"
  → Installation blocked. Use --force to override (not recommended).
```

## 縱深防禦

Audit engine 只是其中一層。skillshare 的安全模型還包含：

1. **安裝時稽核** — 在威脅抵達你的 AI 工具之前先攔截
2. **隨需稽核** — 隨著新增偵測模式，重新掃描既有 skills
3. **共用來源** — 連結的 targets 讀取同一份 skill 檔案；來源修改會傳到所有連結的 targets，因此需要審查變更並限制來源的寫入權限
4. **變更前備份** — `skillshare backup` 會為整個 skill 資料庫拍快照
5. **有 TTL 的垃圾桶機制** — 被刪除的 skills 會先進垃圾桶，而非直接永久刪除
6. **操作日誌** — 每個變更性操作都會被記錄到 `operations.log`（JSONL）

Symlink 提供共同來源，不是安全沙盒。同樣地，`.skillignore` 篩選 skill 探索結果，不會禁止 AI 工具存取檔案或指令。執行時權限與核准屬於 AI 工具的責任。完整職責區分請見[共用來源與執行邊界](/docs/how-to/advanced/security#shared-source-and-execution-boundaries)。

## 供應鏈考量

AI skill 生態系仍相當年輕。目前沒有具備審查流程的套件登錄中心，沒有程式碼簽章，也沒有相依性解析機制。Skills 就是 git repository 裡的 Markdown 檔案。

skillshare 的因應方式：
- **全面掃描** — 即使是來自受信任來源的 skills 也一樣
- **預設封鎖** — CRITICAL 發現會阻止安裝
- **全面記錄** — 稽核結果會被保存，供事後鑑識審查
- **持續更新模式** — 每次 skillshare 發布都會附上新的偵測模式

## 設定 Audit 行為

在 `config.yaml` 中設定封鎖門檻，控制哪個嚴重程度會封鎖安裝：

```yaml
# config.yaml
audit:
  block_threshold: HIGH   # Block on HIGH and CRITICAL (default: CRITICAL)
```

若要針對個別規則客製化，可使用獨立的 `audit-rules.yaml` 檔案（以 `skillshare audit --init-rules` 初始化）：

```yaml
# audit-rules.yaml
rules:
  - id: network-request-0
    enabled: false          # Disable this specific rule
  - id: my-custom-check
    severity: MEDIUM
    pattern: "TODO|FIXME"
    description: Policy violation — unresolved TODOs
```

## 相關內容

- [`audit` 指令參考](/docs/reference/commands/audit)
- [安全指南](/docs/how-to/advanced/security)
- [CI/CD 驗證範例](/docs/how-to/recipes/ci-cd-skill-validation)
