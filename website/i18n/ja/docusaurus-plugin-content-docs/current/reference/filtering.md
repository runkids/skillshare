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

## .skillignore

**場所:**
- Source のルート: `~/.config/skillshare/skills/.skillignore` — すべての Skill に適用される
- Tracked repo のルート: `_team-repo/.skillignore` — そのリポジトリ内にのみ適用される

**構文:** 完全な [gitignore](https://git-scm.com/docs/gitignore) — `*`（1セグメント）、
`**`（任意の深さ）、`?`、`[abc]`、`!pattern`（否定）、`/pattern`（アンカー付き）、
`pattern/`（ディレクトリのみ）。

**`.skillignore.local`:** `.skillignore` と同じ場所に置きます。パターンはベースファイルの後に
追加されます — 最後に一致したルールが優先されます。無視解除には `!pattern` を使います。このファイルは
コミットしないでください。`/` を含まないパターンはどの深さの同名にも一致するため、`!feature-radar` は `feature-radar/feature-radar` も再び有効にします。トップレベルのフォルダだけに一致させるには `!/feature-radar` と書きます。ダッシュボードはこのファイルのルールを **Settings → Files** に表示し、これに上書きされた切り替えを失敗として報告します。

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

### Tracked repo 内の Agent 別バリアント {#agent-specific-variants-in-tracked-repositories}

```text
_team/internal/skills/claude/review/SKILL.md
_team/internal/skills/codex/review/SKILL.md
```

Tracked repo 内の `skills/<既知の-agent>/<完全なスキルパス>/SKILL.md` は Agent 別バリアントとして自動認識されます。例：`_team/internal/skills/{claude,codex}/review/SKILL.md`。リポジトリ、Agent より前の完全なパス、Agent より後の完全なパスで1つの論理 Skill を識別します。バリアントが1つだけでも Agent による振り分けが適用され、`--into` 配下のリポジトリも対象です。

**merge/copy:** 一致する Agent のファイルだけを選択します。`claude-code` などの登録済みエイリアスと、アカウント Target の `agent: codex` にも対応します。ディレクトリの共有は Agent の同一性を意味せず、`universal` が Codex バリアントを自動で受け取ることはありません。一致がなければ他のバリアントで代用せず、同じグループに同一 Agent のエイリアスが重複する場合は警告してスキップします。フィルターや上書き設定で明示的に選択してください。

優先順位は Source の `.metadata.json` にある Target の上書き > 空でない frontmatter targets > バリアントの Agent 推論 > 従来のホストパス推論です。Source の空の上書きは全 Target を意味し、明示的な Target 宣言の既存の照合規則は維持されます。`.skillignore` と include/exclude は引き続き物理パスに適用されます。未知の Agent、`shared`/`default`、tracked repo 外の配置は通常の discovery のままで、代替バリアントは推論しません。

`status` と dashboard の Source/リポジトリ総数は論理 Skill 数です。2つの Skill の4ファイルは `2 skills, 4 variants` と表示されます。JSON の `skill_count`/`skillCount` は論理数、`variant_count`/`variantCount` と Target API の `sourceVariantCount` は通常の Skill を含む有効な物理ファイル総数です。リソース一覧、操作 ID、Sync 結果、コンテキストコストは物理ファイル単位のままです。

**symlink:** ディレクトリ全体が公開されるため、バリアントの振り分けは適用できません。Agent ごとの選択には merge/copy を使ってください。同じ出力ディレクトリを共有する Target 間でも異なる選択を分離できず、既存のパス重複診断が適用されます。Source ファイルは変更しませんが、従来の prefixed copy 命名ではコピーの `name:` が書き換わる場合があります。Sparse checkout は行いません。

アップグレード後の merge/copy は、Agent が一致しない既存の管理対象バリアントを Target から削除します。CLI/サーバーの警告で自動ルーティングによる削除と復元方法を説明し、dry-run でも通知します。Source の Target 上書きで必要なコピーを復元でき、空の上書きは全 Target を意味します。Source ファイルは残ります。Target 欄は推論を含む有効なルーティングを示し、提供元の宣言とは限りません。

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
