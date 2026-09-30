---
sidebar_position: 1
---

# Claude Code で skillshare を使う

> インストールから最初の Sync まで — 5分。

## 前提条件

- [Claude Code](https://docs.anthropic.com/en/docs/claude-code/overview) がインストール済みで動作していること
- macOS、Linux、または Windows（WSL）

## ステップ 1: skillshare をインストールする

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

インストーラーが PATH 設定の案内を表示した場合のみ、その案内に従ってから以下のコマンドを実行してください。PATH の警告がなければ追加の設定は不要です。

## ステップ 2: 初期化する

```bash
skillshare init
```

これは Claude Code の Skill ディレクトリ（`~/.claude/skills/`）を検出し、自動的に Target として
追加します。

## ステップ 3: 最初の Skill をインストールする

```bash
skillshare install anthropics/courses/prompt-eng
```

Skill がダウンロードされ、セキュリティ監査を受け、あなたの Source ディレクトリに追加されます。

## ステップ 4: Sync する

```bash
skillshare sync
```

これは Source から `~/.claude/skills/` へのシンボリックリンクを作成します。Claude Code はすぐに
Skill を認識します — 再起動は不要です。

## ステップ 5: 確認する

```bash
ls ~/.claude/skills/
```

インストールした Skill がシンボリックリンクされているのが見えるはずです。

## Claude Code 統合の詳細

- **Skill のパス**: `~/.claude/skills/`（Global）または `.claude/skills/`（Project）
- **CLAUDE.md**: skillshare の Skill は `SKILL.md` フォーマットを使い、Claude Code はこれを
  ネイティブに読み取ります
- **Project mode**: リポジトリ内で `skillshare init -p` を実行し、プロジェクトごとに
  `.claude/skills/` を管理します

## 次のステップ

- [複数の Skill を管理する →](/docs/how-to/daily-tasks/organizing-skills)
- [チームと共有する →](/docs/how-to/sharing/organization-sharing)
- [さらに Skill を探す →](/docs/reference/commands/search)
