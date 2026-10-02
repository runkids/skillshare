---
sidebar_position: 5
---

# Skill フォーマット

skillshare Skill の構造とメタデータです。

:::tip これはいつ重要ですか？
SKILL.md のフォーマットは、AI CLI がどのように Skill を検出し読み込むかを決定します。`description` フィールドは特に重要で、AI がいつ Skill を有効化するかを判断する材料になります。
:::

## 概要

Skill とは、少なくとも `SKILL.md` ファイルを含むディレクトリです。

```
my-skill/
└── SKILL.md
```

`SKILL.md` ファイルは 2 つの部分から構成されます。
1. **YAML frontmatter** — メタデータ
2. **Markdown 本文** — AI への指示

---

## 基本構造

```markdown
---
name: my-skill
description: Brief description of what this skill does
---

# My Skill

Instructions for the agent when this skill is activated.

## When to Use

Describe when this skill should be used.

## Instructions

1. First step
2. Second step
3. Additional steps as needed
```

---

## 必須フィールド

### `name`

Skill の識別子です。次の用途に使われます。
- Skill の呼び出し（例: `/skill:my-skill`）
- 衝突の検出
- Skill 一覧での表示

```yaml
name: my-skill
```

**ルール:**
- 小文字英字、数字、ハイフン、アンダースコアのみ使用可能
- 英字または数字で始まる必要がある
- すべての Skill の中で一意であることが望ましい

**例:**
```yaml
name: code-review
name: pdf-tools
name: acme-frontend-ui  # Namespaced for teams
```

---

## 任意のフィールド

### `description`

Skill 一覧や検索結果に表示される簡単な説明です。

```yaml
description: Reviews code for bugs, style issues, and improvements
```

---

## 任意のフィールド

### `tags`

Hub インデックスでのフィルタリングやグルーピングに使う分類タグです。`skillshare hub index` を実行すると、SKILL.md の frontmatter にある tags が生成される `skillshare-hub.json` に含まれます。

```yaml
tags: git, workflow
```

タグは検索対象にもなります — `skillshare search workflow --hub ...` は "workflow" タグが付いた Skill にマッチします。

### `targets`

この Skill が Sync される Target を制限します。省略した場合、Skill は**すべての** Target に Sync されます。

配置方法は 2 通りサポートされています — `metadata:` の下（推奨）またはトップレベルです。

```yaml
# Recommended: under metadata
metadata:
  targets: [claude, cursor]

# Legacy: top-level (still fully supported)
targets: [claude, cursor]
```

:::info 優先順位のルール
両方が存在する場合、`metadata.targets` がトップレベルの `targets` より優先されます。これにより段階的な移行が可能です — `metadata:` を追加しても、残っているトップレベルのフィールドと競合しません。
:::

| 値 | 動作 |
|-------|----------|
| *(省略時)* | すべての Target に Sync されます（デフォルト） |
| `[claude]` | "claude" にマッチする Target にのみ Sync されます |
| `[claude, cursor]` | いずれかの名前にマッチする Target に Sync されます |

**モード横断のマッチング:** `targets: [claude]` を宣言した Skill は、project の Target `claude` ともマッチします。どちらも同じ AI CLI を指しているためです。マッチングには [target registry](/docs/reference/targets/supported-targets) が使われます。

**config フィルターとの関係:** Skill レベルの `targets` は、config レベルの `include`/`exclude` の**後に**適用されます。Skill が Sync されるには両方を満たす必要があります。詳しくは [Configuration](/docs/reference/targets/configuration#skill-level-targets) を参照してください。

**例 — Claude 専用の Skill:**

```markdown
---
name: claude-prompts
description: Prompt patterns for Claude Code
metadata:
  targets: [claude]
---

# Claude Prompts
...
```

Pi や Codex など他の Target を設定していても、この Skill は Claude Code の skills ディレクトリにのみ表示されます。

### `pattern`

この Skill で使われている構造的な設計パターンです。`skillshare new -P <pattern>` によって自動生成されます。

```yaml
pattern: reviewer
```

利用可能なパターン: `tool-wrapper`、`generator`、`reviewer`、`inversion`、`pipeline`。各パターンの詳細については [Skill Design Patterns](/docs/understand/philosophy/skill-design-patterns) を参照してください。

### `category`

この Skill のユースケースカテゴリです。`skillshare new` の対話中に設定するか、完全に省略できます。

```yaml
category: quality
```

利用可能なカテゴリ: `library`、`verification`、`data`、`automation`、`scaffold`、`quality`、`cicd`、`runbook`、`infra`。

### `license`

Skill のライセンス識別子です。コンプライアンス判断を助けるため、インストール時に表示されます。

```yaml
license: MIT
```

この値が存在する場合、`skillshare install` は Skill 選択プロンプトと確認画面にライセンスを表示します。

- **単一の Skill**: Skill 情報ボックスに `License: MIT` として表示されます
- **複数 Skill のリポジトリ**: 選択リストの Skill 名に追記されます（例: `my-skill (MIT)`）

これは純粋に情報提供のみで、インストールをブロックすることはありません。よく使われる値: `MIT`、`Apache-2.0`、`GPL-3.0`、`BSD-3-Clause`、`ISC`。

---

## `metadata` ブロック

`metadata:` ブロックは、デプロイや動作に関するフィールドをまとめた構造化された YAML オブジェクトです。これは、30 以上の AI CLI ツールで使われている [Agent Skills エコシステムの規約](https://developers.googleblog.com/en/5-agent-skill-design-patterns-every-adk-developer-should-know/) に沿ったものです。

```yaml
---
name: my-skill
description: My custom skill
metadata:
  targets: [claude]
  pattern: reviewer
  domain: python
---
```

現在、skillshare が処理する `metadata` フィールドは `targets` のみです。それ以外のフィールド（`pattern`、`domain`、`interaction` など）は frontmatter に保持されますが、skillshare では使用されません — エコシステム内の他のツールで利用される可能性があります。

後方互換性のため、skillshare はトップレベルの `targets` フィールドも読み取ります。両方が存在する場合は `metadata.targets` が優先されます。

## カスタムフィールド

任意のカスタムトップレベルフィールドを追加できます。

```yaml
---
name: my-skill
description: My custom skill
author: Your Name
version: 1.0.0
---
```

カスタムのトップレベルフィールドは frontmatter に保存されますが、skillshare 自体では使用されません。

---

## Markdown 本文

本文には AI への指示を記載します。人間のアシスタントに指示するつもりで書いてください。

**良い実践:**
- 明確で具体的な指示
- 入力と期待される出力の例
- エッジケースとエラー処理
- いつ使うか（そしていつ使わないか）

**例:**
```markdown
# Code Review

You are a code reviewer. Analyze code for:
- Bugs and potential issues
- Style and consistency
- Performance concerns
- Security vulnerabilities

## When to Use

Use this skill when the user asks you to review code, find bugs, or improve code quality.

## Instructions

1. Read the provided code carefully
2. Identify issues in order of severity
3. Suggest specific improvements with code examples
4. Be constructive and explain your reasoning

## Example

User: "Review this function"
```python
def add(a, b):
  return a + b
```

Response: "The function looks correct but could benefit from type hints..."
```

---

## 一元管理されたメタデータ

Skill をインストールすると、skillshare はそのメタデータを `.metadata.json`（すべての Skill を一元管理）に記録します。

```json
{
  "skills": [
    {
      "name": "pdf",
      "source": "anthropics/skills/skills/pdf",
      "type": "github",
      "installed_at": "2026-01-20T15:30:00Z",
      "repo_url": "https://github.com/anthropics/skills.git",
      "subdir": "skills/pdf",
      "version": "abc1234"
    }
  ]
}
```

各 Skill エントリには次が含まれます。

| フィールド | 説明 |
|-------|-------------|
| `name` | Skill ディレクトリ名 |
| `source` | インストール時に入力した元のソース |
| `type` | ソースの種類（`github`、`local` など） |
| `installed_at` | インストール日時 |
| `repo_url` | Git クローン URL（git ソースのみ） |
| `subdir` | サブディレクトリのパス（monorepo ソースのみ） |
| `version` | インストール時点の Git コミットハッシュ |

これは、`skillshare update` と `skillshare check` が更新の取得元を把握するために使われます。

**このファイルを手動で編集しないでください。**

---

## Skill の作成

```bash
skillshare new my-skill
```

これにより次が作成されます。
```
~/.config/skillshare/skills/my-skill/
└── SKILL.md  (with template)
```

生成された `SKILL.md` を編集し、`skillshare sync` を実行してデプロイします。

---

## Skill の検証

```bash
skillshare doctor
```

次の項目を確認します。
- SKILL.md フォーマットの妥当性
- 必須の `name` フィールド
- frontmatter YAML の妥当性
- 名前の衝突

---

## 関連項目

- [new](/docs/reference/commands/new) — 正しいテンプレートで Skill を作成
- [Creating Skills](/docs/how-to/daily-tasks/creating-skills) — Skill の書き方の完全ガイド
- [Best Practices](/docs/how-to/daily-tasks/best-practices) — 命名と整理のヒント
