---
sidebar_position: 3
---

# フィルタリングリファレンス

どの Skill がどの Target に届くかを制御する3つのフィルタリングレイヤーの完全な仕様です。

:::tip 手っ取り早いガイダンスをお探しですか？
シナリオ駆動のガイドは [Skill のフィルタリング](/docs/how-to/daily-tasks/filtering-skills) を
参照してください。
:::

## 概要

| レイヤー | スコープ | 設定場所 | 構文 | 評価されるタイミング |
|-------|-------|-------------|--------|-------------|
| `.skillignore` | すべての Target から隠す | Source ディレクトリまたは Tracked repo のルート | [gitignore](https://git-scm.com/docs/gitignore) | Discovery |
| SKILL.md `metadata.targets` | Skill をリストされた Target に制限する | Skill ごとのフロントマター | YAML リスト | Sync（discovery 時にパース） |
| Agent `targets` | Agent をリストされた Target に制限する | Agent ごとのフロントマター | YAML リスト | Sync（discovery 時にパース） |
| Target の include/exclude | Target ごと、リソースごと | `config.yaml` または CLI フラグ | Go [`filepath.Match`](https://pkg.go.dev/path/filepath#Match) の glob | Sync |

:::note Sync モードの注意点
これら3つのレイヤーはすべて **merge** と **copy** の Sync モードにのみ適用されます。
**symlink** モードでは Source ディレクトリ全体が1つの単位としてリンクされるため、Skill ごとの
フィルタリングは効果を持ちません。
:::

## 評価順序と優先順位

Skill が Target に届くには、**すべての**レイヤーを通過する必要があります。

1. **`.skillignore`** — Discovery 時に評価される。一致した Skill は Sync パイプラインに入らない。
2. **Target の include/exclude** — Sync 時に評価される（`FilterSkills`）。Skill は発見されるが、
   一致しない Target ではスキップされる。
3. **SKILL.md `metadata.targets`** — Sync 時に評価される（`FilterSkillsByTarget`）。Skill は
   宣言された Target に制限される。

## .skillfollow

[`.skillfollow`](./skillfollow.md) は filter でなく discovery の opt-in です。Skills source ルートで第一階層の外部グループ/tracked repo リンク名を宣言。Local は名前の和集合で gitignore pattern は不可。リンクのルート固定/末尾 `/` なし ignore と `/.skillfollow.local` を追加します。Root/repo `.skillignore` は適用されます。[設定と全状態](./skillfollow.md#states)を確認してください。

## .skillignore

**場所:**
- Source のルート: `~/.config/skillshare/skills/.skillignore` — すべての Skill に適用される
- Tracked repo のルート: `_team-repo/.skillignore` — そのリポジトリ内にのみ適用される

**構文:** 完全な [gitignore](https://git-scm.com/docs/gitignore) — `*`（1セグメント）、
`**`（任意の深さ）、`?`、`[abc]`、`!pattern`（否定）、`/pattern`（アンカー付き）、
`pattern/`（ディレクトリのみ）。

**`.skillignore.local`:** `.skillignore` と同じ場所に置きます。パターンはベースファイルの後に
追加されます — 最後に一致したルールが優先されます。無視解除には `!pattern` を使います。このファイルは
コミットしないでください。

**CLI での可視性:**

| コマンド | 出力 |
|---------|--------|
| `skillshare sync` | 件数 + Skill 名 |
| `skillshare status --json` | パターンと無視リストを含む `source.skillignore` オブジェクト |
| `skillshare doctor` | パターン数と無視数 |

📖 [ファイル構造リファレンス](/docs/reference/appendix/file-structure#skillignore-optional)

## SKILL.md targets フィールド {#skillmd-targets-field}

**フォーマット:** トップレベルまたは `metadata` の下にネスト:

```yaml
# 推奨
metadata:
  targets: [claude, cursor]

# レガシーなフォールバック
targets: [claude, cursor]
```

**動作:** ホワイトリスト — その Skill はリストされた Target にのみ Sync されます。このフィールドを
省略すると、すべての Target に Sync されます。`metadata.targets` とトップレベルの `targets` の両方が
存在する場合、`metadata.targets` が優先されます。

**Tracked repo:** dashboard で tracked repo 内の Skill に設定した targets は、その SKILL.md ではなく source の `.metadata.json` に保存される。そのため clone はクリーンなままで、`update` も通常どおり動作する。この設定は Skill の `metadata.targets` より優先される。

**エイリアス:** Target 名はエイリアスに対応しています。`claude` は `claude-code` として設定された
Target に一致します。[対応する Target](/docs/reference/targets/supported-targets) を参照してください。

📖 [Skill フォーマット — targets フィールド](/docs/understand/skill-format#targets)

**Agent** も、Agent のフロントマター内のトップレベルの `targets` リストを通じて同じホワイトリストに
対応しています。このフィールドがない Agent は、Agent 対応のすべての Target に Sync されます。
[Agents — Agent ファイルフォーマット](/docs/understand/agents#agent-file-format) を参照してください。

## Target の include/exclude フィルター {#target-includeexclude-filters}

**CLI で設定する:**

```bash
# Skill
skillshare target claude --add-include "team-*"
skillshare target cursor --add-exclude "legacy-*"
skillshare target claude --remove-include "team-*"

# Agent
skillshare target claude --add-agent-include "team-*"
skillshare target claude --add-agent-exclude "draft-*"
skillshare target claude --remove-agent-include "team-*"
```

**保存場所:** `config.yaml` の中の、Skill 用の `targets.<name>.include` / `targets.<name>.exclude`、
Agent 用の `targets.<name>.agents.include` / `targets.<name>.agents.exclude`。

**構文:** フラットなリソース名に対して照合される Go [`filepath.Match`](https://pkg.go.dev/path/filepath#Match)
の glob パターン。Skill はフラットな Skill 名（例: `_team__frontend__ui`）を使い、Agent はフラットな
`.md` ファイル名を使います。

| 対応 | 非対応 |
|-----------|--------------|
| `*`（任意の文字） | `**`（再帰的） |
| `?`（単一文字） | `{a,b}`（波括弧展開） |
| `[abc]`（文字クラス） | |

**優先順位:** `include` と `exclude` の両方が設定されている場合、`include` が先に適用され、その後
`exclude` が適用されます。両方に一致するリソースは除外されます。

**ビジュアルエディタ:** `skillshare ui` → Targets ページ → 「Customize filters」ボタン。

📖 [Target コマンド](/docs/reference/commands/target#target-filters-includeexclude) ・
[フィルター動作の例](/docs/reference/commands/sync#filter-behavior-examples) ・
[Configuration](/docs/reference/targets/configuration#include--exclude-target-filters)

## 関連項目

- [Skill のフィルタリング](/docs/how-to/daily-tasks/filtering-skills) — シナリオ駆動の How-to ガイド
