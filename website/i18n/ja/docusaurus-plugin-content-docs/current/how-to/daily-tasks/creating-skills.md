---
sidebar_position: 2
---

# Skill の作成

アイデアから公開済みの Skill まで。

:::tip
自分の Skill をどの Target に配布するか制御したいですか？[Skill のフィルタリング](/docs/how-to/daily-tasks/filtering-skills) を参照してください。
:::

## 概要

```mermaid
flowchart LR
    IDEA["IDEA"] --> CREATE["CREATE"] --> WRITE["WRITE"] --> TEST["TEST"] --> PUBLISH["PUBLISH"]
```

---

## ステップ 1: Skill を作成する

```bash
skillshare new my-skill
```

これにより以下が作成されます。
```
~/.config/skillshare/skills/my-skill/
└── SKILL.md  (テンプレート付き)
```

---

## ステップ 2: Skill を書く

生成された `SKILL.md` を編集します。

```bash
$EDITOR ~/.config/skillshare/skills/my-skill/SKILL.md
```

### 基本構造

```markdown
---
name: my-skill
description: Brief description (shown in skill lists)
---

# My Skill

What this skill does and when to use it.

## Instructions

1. Step one
2. Step two
3. Step three
```

### 良い Skill を書くコツ

**具体的にする:**
```markdown
# Good
When the user asks to review code, analyze for:
- Bugs and potential issues
- Style consistency
- Performance concerns

# Bad
Review the code and make it better.
```

**例を含める:**
````markdown
## Example

User: "Review this function"
```python
def add(a, b):
    return a + b
```

Response: Suggest adding type hints...
````

**使うべきでない場面を明記する:**
```markdown
## When NOT to Use

- Don't use for simple syntax questions
- Don't use for explaining code (use explain-code skill instead)
```

---

## ステップ 3: デプロイしてテストする

### すべての Target にデプロイする

```bash
skillshare sync
```

### AI CLI でテストする

Skill を実際に使ってみます。
- 明示的に呼び出す: `/skill:my-skill`
- またはタスクを説明して AI が拾うか確認する

### 反復する

編集 → sync → テストを、うまく動くまで繰り返します。

---

## ステップ 4: 公開する（任意）

### チームと共有する

git remote にプッシュします。
```bash
skillshare push -m "Add my-skill"
```

チームメンバーは pull できます。
```bash
skillshare pull
```

### 公開で共有する

1. Skill 用の GitHub リポジトリを作成する
2. Skill のディレクトリをプッシュする
3. 他の人はインストールできます:
   ```bash
   skillshare install github.com/you/my-skills/my-skill
   ```

---

## Skill テンプレート

### シンプルな Skill

```markdown
---
name: simple-skill
description: Does one thing well
---

# Simple Skill

When the user asks to do X, follow these steps:

1. First, do Y
2. Then, do Z
3. Finally, confirm completion
```

### タスク指向の Skill

```markdown
---
name: code-review
description: Reviews code for quality and issues
---

# Code Review

You are a code reviewer. Analyze code for quality issues.

## What to Check

- Bugs and edge cases
- Performance issues
- Security vulnerabilities
- Code style and readability

## Output Format

For each issue found:
1. **Location**: File and line
2. **Severity**: High/Medium/Low
3. **Issue**: What's wrong
4. **Fix**: Suggested solution

## Example

[Include an example input and expected output]
```

### Target 固有の Skill

```markdown
---
name: claude-prompts
description: Prompt patterns specific to Claude Code
targets: [claude]
---

# Claude Prompts

Patterns that work best with Claude Code's capabilities.

## When to Use

Use when crafting prompts for Claude Code specifically.
```

`targets` が設定されている場合、その Skill は一致する Target にのみ Sync されます — 他の Target には配布されません。すべての Target に Sync するには `targets` を省略してください。

### プロセス Skill

```markdown
---
name: git-workflow
description: Guides through git commit workflow
---

# Git Workflow

Guide the user through proper git commit practices.

## Steps

1. **Check status**: Run `git status`
2. **Review changes**: Run `git diff`
3. **Stage files**: Add specific files, not `git add .`
4. **Write message**: Follow conventional commits
5. **Commit**: Create the commit
6. **Verify**: Run `git log -1`

## Commit Message Format

```text
type(scope): description

[optional body]
```

Types: feat, fix, docs, style, refactor, test, chore

---

## 高度なトピック

### Skill 内の複数ファイル

Skill には追加のファイルを含めることができます。

```
my-skill/
├── SKILL.md
├── examples/
│   └── sample.py
└── templates/
    └── component.tsx
```

SKILL.md 内でそれらを参照します。
```markdown
See the example in `examples/sample.py` for reference.
```

### チーム向けの名前空間化

名前空間付きの名前で衝突を避けます。

```yaml
name: acme-code-review
```

### バージョン管理

バージョンメタデータを追加します。

```yaml
---
name: my-skill
description: My skill
metadata:
  version: 1.0.0
  author: Your Name
---
```

### ライセンスメタデータ

インストール前にユーザーがライセンス情報を確認できるよう `license` フィールドを追加します。

```yaml
---
name: my-skill
description: My reusable skill
license: MIT
---
```

これが存在する場合、`skillshare install` は選択プロンプトと確認画面にライセンスを表示します。これは企業ユーザーのコンプライアンス判断に役立ちます。詳細は [Skill フォーマット](/docs/understand/skill-format#license) を参照してください。

### .skillignore による発見の制御

複数の Skill を含むリポジトリを公開する際、ユーザーに発見されたくない社内ツールや作業中の Skill があるかもしれません。リポジトリのルートに `.skillignore` ファイルを作成します。

```text title=".skillignore"
# Internal tooling
validation-scripts
scaffold-template

# Exclude an entire group directory
internal-tools/

# Work in progress
prompt-eval-*

# Ignore temp at any depth
**/temp

# Exclude tests but keep test-critical
test-*
!test-critical
```

`.skillignore` は [gitignore の構文](https://git-scm.com/docs/gitignore) を使用します — `*`、`**`、`?`、`[abc]`、`!negation`、`/anchored`、`pattern/`（ディレクトリのみ）、`\#`/`\!` のエスケープに対応しています。`internal-tools` のようなグループ名は、そのディレクトリ配下の**すべての** Skill を除外します。グループ内の特定の Skill のみを除外するには `internal-tools/helper` のような正確なパスを使用してください。

これらのパターンに一致する Skill は `skillshare install <repo>` の発見結果に表示されません。これはサーバー側（リポジトリ内）で適用されるため、すべてのユーザーが自動的にその恩恵を受けます。実際の例は [`runkids/my-skills`](https://github.com/runkids/my-skills) を、ユーザー側での除外については [install --exclude](/docs/reference/commands/install#excluding-skills) を参照してください。

### Source ルートの .skillignore（ローカル）

**Source ルート**（`~/.config/skillshare/skills/.skillignore`）にも `.skillignore` を置くことができ、`doctor`、`status`、`list`、`sync`、`audit`、`diff`、`check` のすべてのコマンドから Skill をグローバルに隠せます。

```text title="~/.config/skillshare/skills/.skillignore"
# Temporarily mute a skill without uninstalling
my-experimental-skill

# Exclude all draft skills
[Dd]raft*

# Hide an entire tracked repo
_archived-team-skills

# Ignore vendored deps at any depth
**/node_modules
*.venv
```

両方のレイヤーが適用されます。Source ルートのパターンはすべての Skill（Tracked と非 Tracked の両方）に影響し、リポジトリレベルのパターンはそのリポジトリの Skill にのみ影響します。どちらかのレイヤーが一致すれば、その Skill は除外されます。

### .skillignore.local（個人用オーバーライド）

共有リポジトリの `.skillignore` がローカルで必要な Skill をブロックしている場合、同じディレクトリに `.skillignore.local` を作成してください。そのパターンは `.skillignore` の後に追加されるため、`!pattern` の否定パターンがベースファイルをオーバーライドします。

```text title="_team-skills/.skillignore.local"
# The repo ignores private-*, but I need my own
!private-mine
```

このファイルは**コミットしないでください** — `.gitignore` に追加します。Source ルートとリポジトリレベルの両方で機能します。

---

## チェックリスト

公開前:

- [ ] 明確で具体的な名前
- [ ] 目的を説明する description
- [ ] 実行可能な指示
- [ ] 例を含む
- [ ] AI CLI でテスト済み
- [ ] 既存の Skill との衝突なし

---

## 関連項目

- [new](/docs/reference/commands/new) — テンプレートで Skill を作成する
- [Skill フォーマット](/docs/understand/skill-format) — SKILL.md の構造
- [Skill 設計](/docs/understand/philosophy/skill-design) — 複雑さのレベル、決定論、CLI ラッパーパターン
- [ベストプラクティス](./best-practices.md) — 命名と整理
- [Skill の整理](./organizing-skills.md) — フォルダ構造
