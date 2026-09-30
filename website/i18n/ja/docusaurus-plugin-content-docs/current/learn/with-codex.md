---
sidebar_position: 3
---

# Codex で skillshare を使う

> インストールから最初の Sync まで — 5分。

## 前提条件

- [OpenAI Codex CLI](https://github.com/openai/codex) がインストール済みで動作していること
- macOS、Linux、または Windows

## ステップ 1: skillshare をインストールする

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

インストーラーが PATH 設定の案内を表示した場合のみ、その案内に従ってから以下のコマンドを実行してください。PATH の警告がなければ追加の設定は不要です。

## ステップ 2: 初期化する

```bash
skillshare init
```

Codex は `~/.agents/skills/` を読み込みます。これは Codex がユーザーレベルの Skill パスとして文書化している共有ディレクトリです。`init` は Codex の設定ディレクトリ（`~/.codex/`）から Codex を検出し、自動的に共有の `universal` Target をセットアップします。

## ステップ 3: 最初の Skill をインストールする

```bash
skillshare install runkids/my-skills
```

## ステップ 4: Sync する

```bash
skillshare sync
```

Skill は `~/.agents/skills/` にシンボリックリンクされます。

## ステップ 5: 確認する

```bash
ls ~/.agents/skills/
```

インストールした Skill がシンボリックリンクされているのが見えるはずです。

## Codex 固有の注意事項

- **Skill のパス**: `~/.agents/skills/`（Global）または `.agents/skills/`（Project）
- **既存の設定**: 設定でまだ `codex` が `~/.codex/skills` を指している場合、Codex はそれを読み込み続けます。ただし `universal` も同時に有効になっていると、すべての Skill が二重に表示されます — `codex` Target を削除してください（`skillshare target remove codex --dry-run` で事前に確認できます）
- **説明文の文字数制限**: Codex は Skill の description に 1024 文字の制限があります。
  `SKILL.md` フロントマターの `description` フィールドは簡潔に保ってください
- **Project mode**: プロジェクトレベルの Codex Skill を管理するには `skillshare init -p` を実行します

## 次のステップ

- [複数の Skill を管理する →](/docs/how-to/daily-tasks/organizing-skills)
- [チームと共有する →](/docs/how-to/sharing/organization-sharing)
- [さらに Skill を探す →](/docs/reference/commands/search)
