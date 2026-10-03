---
sidebar_position: 1
---

# 工作流程

Skillshare 的常见使用模式。

## 选择你的工作流程

| 我想要... | 工作流程 |
|-------------|----------|
| 日常使用 Skill | [Daily Workflow](./daily-workflow.md) |
| 跨 AI 工具共享记忆 | [记忆共享教程](./sharing-memory) |
| 发现并安装新 Skill | [Skill Discovery](./skill-discovery.md) |
| 保护我的 Skill | [Backup & Restore](./backup-restore.md) |
| 管理 project 范围的 Skill | [Project Workflow](./project-workflow.md) |
| 修复损坏的内容 | [Troubleshooting](/docs/troubleshooting) |

---

## 快速参考

### 日常循环
```bash
# Edit skills (in source or any target)
$EDITOR ~/.config/skillshare/skills/my-skill/SKILL.md

# Sync to all targets (if needed)
skillshare sync

# Push to remote (if using cross-machine sync)
skillshare push -m "Update my-skill"
```

### 发现循环
```bash
# Search for skills
skillshare search pdf

# Browse a repository
skillshare install anthropics/skills

# Install and sync
skillshare install anthropics/skills/skills/pdf
skillshare sync
```

### 安全循环
```bash
# Before risky changes
skillshare backup

# If something breaks
skillshare restore claude

# Check health
skillshare doctor
```
