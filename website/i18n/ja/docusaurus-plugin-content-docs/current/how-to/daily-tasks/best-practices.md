---
sidebar_position: 6
---

# ベストプラクティス

Skill の命名規則、整理方法、バージョン管理です。

## 命名

### Skill 名

**推奨:**
- 小文字とハイフンを使う: `code-review`、`pdf-tools`
- 説明的にする: `rcg` ではなく `react-component-generator`
- チーム用に名前空間を付ける: `acme-code-review`

**非推奨:**
- スペースや特殊文字を使う
- 汎用的な名前を使う: `helper`、`utils`、`tools`
- よくある Skill 名と衝突させる

### リポジトリ名

**個人の場合:**
```
my-skills
ai-skills
```

**チームの場合:**
```
<team>-skills
<org>-skills
```

---

## 整理

### 個人の Skill

```
~/.config/skillshare/skills/
├── code-review/
├── pdf-tools/
├── git-workflow/
└── _team-skills/      # Tracked repo
```

### チームのリポジトリ

```
team-skills/
├── frontend/
│   ├── react/
│   ├── vue/
│   └── testing/
├── backend/
│   ├── api/
│   └── database/
├── devops/
│   ├── deploy/
│   └── monitoring/
└── README.md
```

### Skill ディレクトリ

```
my-skill/
├── SKILL.md           # 必須
├── README.md          # 任意: 人間向け
├── examples/          # 任意: サンプルファイル
└── templates/         # 任意: コードテンプレート
```

---

## バージョン管理

### コミットメッセージ

Conventional Commits に従います。
```
feat(code-review): add security check
fix(pdf-tools): handle empty files
docs(readme): update installation
```

### ブランチ

**個人の場合:**
- 単一の `main` ブランチで十分
- 実験用にはブランチを使う

**チームの場合:**
- 安定した Skill 用の `main`
- 開発用の feature ブランチ
- マージ前の PR レビュー

### タグ

安定版リリースにタグを付けます。
```bash
git tag v1.0.0
git push --tags
```

---

## Skill の書き方

### 構造

```markdown
---
name: skill-name
description: One-line description
---

# Skill Name

Brief overview.

## When to Use

Clear trigger conditions.

## Instructions

1. Step one
2. Step two

## Examples

Concrete input/output examples.

## When NOT to Use

Explicit exclusions.
```

### ライセンス

公開する Skill には `license` フィールドを追加してください — 特に企業環境では重要です。

```yaml
---
name: code-review
description: Reviews code for quality
license: MIT
---
```

これは `skillshare install` の実行時に表示され、ユーザーがコンプライアンス上の判断を的確に下せるようにします。

### 内容

**推奨:**
- 明確で実行可能な指示を書く
- 例を含める
- エッジケースを明記する
- 焦点を絞る（1 Skill = 1目的）

**非推奨:**
- 曖昧な指示を書く
- あまりに多くの責務を含める
- エラーハンドリングを忘れる
- テストを省略する

---

## チームでの協業

### リポジトリ固有の Skill には Project mode（`-p`）を使う

Skill が1つのコードベース（アーキテクチャ、ドメインルール、デプロイフローなど）に密結合している場合は、Project mode を優先してください。

```bash
skillshare init -p
skillshare install <source> -p
skillshare sync
```

**これがなぜ役立つか:**
- **再現可能なオンボーディング**: `.skillshare/config.yaml` は、リポジトリを clone する誰にとっても持ち運び可能な Skill マニフェストとして機能します。
- **明確なスコープ**: プロジェクトの Skill は `.skillshare/skills/` に留まり、Global の個人ワークフローに漏れ出しません。
- **より安全な協業**: 変更はプロジェクトコードと同じ通常の git PR フローでレビューされます。
- **コミットのノイズが少ない**: Project mode では `.skillshare/logs/` がデフォルトで無視されます。

個人のプロジェクト横断的な Skill には Global mode を、リポジトリ固有のチームコンテキストには `-p` を使ってください。

### 社内ツールには .skillignore を使う

チームのリポジトリに社内ツールや作業中の Skill が含まれる場合は、`.skillignore` を追加して誤って発見されるのを防いでください。

```text title=".skillignore"
# Hide from public discovery
_internal-scripts
test-*
wip-feature
```

これにより、外部のコントリビューターや `skillshare install <repo> --all` を実行する自動化が社内 Skill を取り込むことを防げます。

**`.skillignore.local` によるローカルオーバーライド**: 共有リポジトリの `.skillignore` がローカルで必要な Skill をブロックしている場合、同じディレクトリに `.skillignore.local` を作成すれば、共有ファイルを変更せずにオーバーライドできます。

```text title="_team-skills/.skillignore.local"
# Un-ignore my own private skill
!private-mine
```

`.skillignore.local` は `.gitignore` に追加してください — これはローカルに留めるためのものです。

### 所有権

- Skill カテゴリごとに担当者を割り当てる
- README に誰が何をメンテナンスしているかを記載する
- マージ前に PR をレビューする

### ドキュメント

```
team-skills/
├── README.md           # セットアップ手順
├── CONTRIBUTING.md     # Skill の追加方法
├── CHANGELOG.md        # 変更内容
└── skills/
    └── ...
```

### コミュニケーション

- チームチャットで新しい Skill を告知する
- 破壊的変更を文書化する
- ユーザーからフィードバックを集める

---

## メンテナンス

### 定期タスク

```bash
# 毎週
skillshare update --all     # Tracked repos を更新
skillshare doctor           # 問題をチェック
skillshare backup --cleanup # 古いスナップショットと放置された未完成ディレクトリを削除

# 毎月
skillshare list             # インストール済みの Skill を見直す
# 未使用のものを削除: skillshare uninstall <name>...
```

### 未使用の Skill のクリーンアップ

```bash
# すべての Skill を一覧表示
skillshare list

# 使っていないものを削除
skillshare uninstall unused-skill
skillshare sync
```

### 依存関係の更新

```bash
# CLI を更新
skillshare upgrade --cli

# 組み込み Skill を更新
skillshare upgrade --skill

# Tracked repos を更新
skillshare update --all
```

---

## セキュリティ

### 機密情報

**Skill に絶対に入れてはいけないもの:**
- API キー
- パスワード
- 個人情報
- 内部 URL

**代わりに:**
- 環境変数を使う
- 外部設定を参照する
- Skill を汎用的に保つ

### インストール前のレビュー

サードパーティの Skill をインストールする前に:
- Source を確認する
- SKILL.md を読む
- まず `--dry-run` を使う

包括的なセキュリティワークフローについては、[Skill をセキュアに保つ](/docs/how-to/advanced/security) ガイドを参照してください。

---

## チェックリスト

### 新しい Skill

- [ ] 説明的な名前
- [ ] 明確な説明
- [ ] 実行可能な指示
- [ ] 例を含む
- [ ] AI CLI でテスト済み
- [ ] 名前の衝突なし

### チームのリポジトリ

- [ ] 明確なフォルダ構造
- [ ] セットアップ手順を含む README
- [ ] 名前空間化された Skill 名
- [ ] 社内ツール用の `.skillignore`
- [ ] PR レビュープロセス
- [ ] CHANGELOG のメンテナンス

---

## 関連項目

- [Skill の作成](./creating-skills.md) — Skill 作成ガイド
- [Skill 設計](/docs/understand/philosophy/skill-design) — 複雑さのレベル、決定論、CLI ラッパーパターン
- [Skill フォーマット](/docs/understand/skill-format) — SKILL.md リファレンス
- [組織全体の Skill](/docs/how-to/sharing/organization-sharing) — チーム共有パターン
