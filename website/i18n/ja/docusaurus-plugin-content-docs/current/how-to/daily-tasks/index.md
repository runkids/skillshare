---
sidebar_position: 1
---

# ワークフロー

skillshare の一般的な使用パターンです。

## ワークフローを選ぶ

| やりたいこと | ワークフロー |
|-------------|----------|
| 日常的に Skill を使う | [日々のワークフロー](./daily-workflow.md) |
| AI ツール間でメモリを共有する | [メモリ共有ガイド](./sharing-memory) |
| 新しい Skill を見つけてインストールする | [Skill の発見](./skill-discovery.md) |
| Skill を保護する | [バックアップと復元](./backup-restore.md) |
| プロジェクトスコープの Skill を管理する | [プロジェクトワークフロー](./project-workflow.md) |
| 壊れたものを直す | [トラブルシューティング](/docs/troubleshooting) |

---

## クイックリファレンス

### 日々のサイクル
```bash
# Skill を編集する (Source またはどの Target でも)
$EDITOR ~/.config/skillshare/skills/my-skill/SKILL.md

# すべての Target に Sync する (必要な場合)
skillshare sync

# remote にプッシュする (クロスマシン Sync を使っている場合)
skillshare push -m "Update my-skill"
```

### 発見のサイクル
```bash
# Skill を検索する
skillshare search pdf

# リポジトリを閲覧する
skillshare install anthropics/skills

# インストールして Sync する
skillshare install anthropics/skills/skills/pdf
skillshare sync
```

### 安全のサイクル
```bash
# リスクのある変更の前に
skillshare backup

# 何かが壊れたら
skillshare restore claude

# 健全性を確認する
skillshare doctor
```
