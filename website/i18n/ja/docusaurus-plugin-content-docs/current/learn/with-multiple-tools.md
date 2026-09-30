---
sidebar_position: 4
---

# 複数の AI ツールで skillshare を使う

> 単一の Source を、使っているすべての AI CLI に Sync する。

## 問題

あなたは職場で Claude Code、サイドプロジェクトで Cursor、実験には Codex を使っています。それぞれに
独自の Skill ディレクトリがあります。手動でそれらを同期し続けるのは面倒でミスが起きやすいです。

## 解決策

skillshare は単一の Source ディレクトリを維持し、1つのコマンドですべての Target に Sync します。

## ステップ 1: インストールして初期化する

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

インストーラーが PATH 設定の案内を表示した場合のみ、その案内に従ってから以下のコマンドを実行してください。PATH の警告がなければ追加の設定は不要です。

```bash
skillshare init
```

`init` はインストールされているすべての AI ツールを自動検出し、Target として追加します。

## ステップ 2: Target を確認する

```bash
skillshare target list
```

出力例:

```
  claude       ~/.claude/skills (merge)
  cursor       ~/.cursor/skills (merge)
  opencode     ~/.config/opencode/skills (merge)
  universal    ~/.agents/skills (merge)
```

Codex には専用の行はありません。Codex は共有の `~/.agents/skills` ディレクトリを読み込むため、`universal` Target がそれをカバーします。

## ステップ 3: Skill をインストールする

```bash
skillshare install runkids/my-skills
skillshare install anthropics/courses/prompt-eng
```

## ステップ 4: すべてを Sync する

```bash
skillshare sync
```

1つのコマンドで、すべての Skill がすべての Target にプッシュされます。各 Target は、単一の Source を
指し示すシンボリックリンクを得ます。

## ステップ 5: 確認する

```bash
skillshare status
```

すべての Target にわたる Sync ステータスを表示します — どの Skill が Sync 済みか、欠けているか、
古くなっているか。

## Target ごとのモード制御

ツールによってニーズは異なります。Target ごとに Sync モードを設定できます。

```bash
# Cursor はシンボリックリンクを問題なく辿れる（デフォルト）
skillshare target cursor --mode merge

# 一部のツールは実ファイルを必要とする
skillshare target opencode --mode copy
```

## 次のステップ

- [Sync モードを理解する →](/docs/understand/sync-modes)
- [クロスマシン Sync →](/docs/how-to/sharing/cross-machine-sync)
- [チーム共有 →](/docs/how-to/sharing/organization-sharing)
