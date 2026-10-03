---
sidebar_position: 1
---

# 工作流程

skillshare 的常見使用模式。

## 選擇你的 Workflow

| 我想要... | Workflow |
|-------------|----------|
| 每天使用 Skills | [日常工作流程](./daily-workflow.md) |
| 跨 AI 工具共用記憶 | [記憶共用教學](./sharing-memory) |
| 尋找並安裝新 Skills | [Skill 探索](./skill-discovery.md) |
| 保護我的 Skills | [備份與還原](./backup-restore.md) |
| 管理專案範圍的 Skills | [Project Workflow](./project-workflow.md) |
| 修復壞掉的東西 | [Troubleshooting](/docs/troubleshooting) |

---

## 快速參考

### 每日循環
```bash
# 編輯 Skills（在 Source 或任何 Target 中）
$EDITOR ~/.config/skillshare/skills/my-skill/SKILL.md

# 同步到所有 Targets（如有需要）
skillshare sync

# 推送到遠端（若使用跨機器同步）
skillshare push -m "Update my-skill"
```

### 探索循環
```bash
# 搜尋 Skills
skillshare search pdf

# 瀏覽一個 repository
skillshare install anthropics/skills

# 安裝並同步
skillshare install anthropics/skills/skills/pdf
skillshare sync
```

### 安全循環
```bash
# 進行有風險的變更之前
skillshare backup

# 如果出了問題
skillshare restore claude

# 檢查健康狀態
skillshare doctor
```
