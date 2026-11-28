---
sidebar_position: 4
---

# analyze

各 Target の Skill について、コンテキストウィンドウの使用量と Skill の品質を分析します。

```bash
skillshare analyze                    # インタラクティブ TUI（デフォルト）
skillshare analyze claude             # 単一 Target の詳細
skillshare analyze --verbose          # 説明文が最も長い上位 10 件
skillshare analyze --json             # 機械可読な出力
skillshare analyze -p                 # Project mode
```

## 使うタイミング

### コンテキスト予算を最適化する

どの Skill がコンテキストウィンドウのトークンを最も消費しているかを特定する。

```bash
skillshare analyze           # すべての Target をインタラクティブに閲覧
```

### Target 間で比較する

Target 間（例: Claude と Cursor）でコンテキスト使用量がどう違うかを確認する。

```bash
skillshare analyze           # TUI 内で Tab キーを押して Target を切り替え
```

### Skill の品質を確認する

フィールドの欠落、説明文の短さ、トリガーフレーズの欠如がある Skill を見つける。

```bash
skillshare analyze           # TUI 内に Lint アイコン（✗/⚠）が表示される
```

### CI/スクリプト

機械可読なコンテキストメトリクスと Lint 結果を取得する。

```bash
skillshare analyze --json | jq '.targets[].always_loaded.estimated_tokens'
skillshare analyze --json | jq '.targets[].skills[] | select(.lint_issues | length > 0)'
```

## 実行内容

`analyze` は各 Skill について、2 層のコンテキストコストを計算します。

1. **常時ロード** — SKILL.md のフロントマターにある `name + description`（Skill マッチングのため、すべてのリクエストでコンテキストにロードされる）
2. **オンデマンド** — フロントマター以降の Skill 本文（Skill がトリガーされたときのみロードされる）

トークン数の見積もりには近似値として `chars / 4` を使用します。

各 target は、ツールが実際に見る内容で計測します。

- **merge / copy モード** — target の include/exclude と `targets:` の絞り込みを通る source の skills から `.skillignore` で無効にしたものを除き、source から来たものではなく最初から target フォルダにある skills を加えたもの（JSON では `local` が付きます）。Claude の target では、Claude Code が claude.ai から `synced/<id>/skill-creator` に同期した skill は、テキスト出力と dashboard で `skill-creator (claude.ai)` と表示されます。JSON は完全な `name` を保持します。
- **symlink モード** — `.skillignore` で除外した skills を含む source フォルダ全体（JSON では `disabled` が付きます）。symlink によってツールからはそれらも見えるためです。

### Skill 品質 Lint

トークン分析に加えて、`analyze` はすべての Skill に対して組み込みの Lint エンジンを実行します。Lint ルールは SKILL.md の構造と説明文の品質をチェックし、問題を TUI と JSON 出力に直接表示します。

| ルール | 重大度 | チェック内容 |
|------|----------|----------------|
| `missing-name` | error | `name` フィールドが空または欠落している |
| `missing-description` | error | `description` フィールドが空または欠落している |
| `empty-body` | error | Skill 本文（フロントマター以降）が空である |
| `description-too-short` | warning | 説明文が 50 文字未満 |
| `description-too-long` | warning | 説明文が目標上限の 1024 文字を超えている |
| `description-near-limit` | warning | 説明文が 900〜1024 文字の範囲にある |
| `body-too-long` | warning | Skill 本文（front matter の後）が 500 行を超えている。詳細は参照ファイルへ移す |
| `no-trigger-phrase` | warning | 説明文にトリガーフレーズ（例: "Use when…"）がない |

TUI では、Lint 上の問題がある Skill は名前の横に ✗（error）または ⚠（warning）アイコンが表示されます。詳細パネルには、すべての検出結果を一覧する **Quality** セクションが含まれます。

## インタラクティブ TUI

TTY 上では、`analyze` はターゲットごとにインタラクティブな画面を開きます。左側にトークン量順の Skill、右側に選択した Skill のトークン内訳、lint の問題、説明が表示されます。Skill を開くと、front matter を含むファイルの原文が表示され、lint の問題と照らし合わせられます。キーは画面下部に表示されます。プレーンテキストにするには `--no-tui` を使用します。TUI には MCP のツール定義は表示されません。表示するには `--no-tui` または `--json` を使用します。

### 色分け

トークン消費量のレベルは、Target ごとに動的なパーセンタイル閾値を使用します。

| 色 | 意味 |
|-------|-------|
| 🔴 赤 | P75 以上（消費量上位 25%） |
| 🟡 黄 | P25〜P75（中間 50%） |
| 🟢 緑 | P25 未満（下位 25%） |

## 出力例

### デフォルト（--no-tui）

```
claude · 7 skills
  Always loaded  ~362 tokens
  On-demand max  ~22 tokens

  Add -v for the 10 largest descriptions
```

### 詳細

```
skillshare analyze --verbose

claude · 7 skills
  Always loaded  ~362 tokens
  On-demand max  ~22 tokens

  Largest descriptions
    my-big-skill   ~180 tokens
    another-skill  ~120 tokens
    …
```

### MCP ツール定義 {#mcp-tool-definitions}

source 全体を対象にした実行（Target 引数も `--filter` もなし）では、MCP サーバーのツール定義も追加され、MCP Target ごとに数えられます（`claude`、`codex`、`cursor` などの `mcp.targets` の Agent で、Skill の Target とは別のリストです）:

```
MCP tool definitions (upper bound)  claude ~6.8K · codex ~5.7K

! claude: MCP tool definitions are ~6,750 tokens (budget: 5,000)
   Largest: github ~3,276, playwright ~1,932, sentry ~1,092
  1 MCP server not counted yet; run skillshare mcp check --live
```

数は [`mcp check --live`](/docs/reference/commands/mcp#probe-servers-live) から取得され、各サーバーのツール名、説明、入力スキーマを見積もったものです。`analyze` はサーバーを起動しません。ライブチェックがまだ数えていないサーバーは、0 ではなく「未カウント」として表示されます。どのサーバーも数えていない場合、`analyze` は `MCP tool definitions have not been counted yet; run skillshare mcp check --live` と表示します。場所ごとに別のグループになります: global の一覧と、各 `mcp.projects` ルートです。ルートのグループには、そのプロジェクトが無効にしていない global のサーバーも含まれます。Agent はそこでそれらも読み込むためです。ツールポリシーは適用されないため、合計は上限値です。警告には [`context_budget.warn_mcp_tool_tokens`](/docs/reference/targets/configuration#context_budget) を使います。

### 単一 Target

Target 名を渡すと、自動的に verbose 出力が有効になります。

```bash
skillshare analyze claude
```

### グループでフィルタする

```bash
# frontend 系のすべての Skill の合計トークンコストを見る
skillshare analyze claude --json --filter frontend

# TUI の検索ボックスを事前入力する
skillshare analyze --filter marketing
```

## オプション

| フラグ | 説明 |
|------|-------------|
| `[target]` | 単一 Target の詳細を表示（自動的に verbose を有効化） |
| `--verbose`, `-v` | Target ごとに説明文が最も長い上位 10 件を表示 |
| `--no-tui` | インタラクティブ TUI を無効化し、プレーンテキストで出力 |
| `--project`, `-p` | Project レベルの Skill（`.skillshare/`）を分析 |
| `--global`, `-g` | グローバルの Skill（`~/.config/skillshare`）を分析 |
| `--filter <text>` | 名前/パスの部分文字列で Skill をフィルタ |
| `--json` | JSON として出力（スクリプト/CI 向け） |
| `--help`, `-h` | ヘルプを表示 |

:::tip 自動検出
`--project` も `--global` も指定しない場合、skillshare は自動検出します。カレントディレクトリに `.skillshare/config.yaml` が存在すれば Project mode、それ以外はグローバルモードになります。
:::

## JSON 出力

```bash
skillshare analyze --json
```

```json
{
  "targets": [
    {
      "name": "claude",
      "skill_count": 7,
      "always_loaded": {
        "chars": 1448,
        "estimated_tokens": 362
      },
      "on_demand_max": {
        "chars": 88,
        "estimated_tokens": 22
      },
      "skills": [
        {
          "name": "my-skill",
          "description_chars": 180,
          "description_tokens": 45,
          "body_chars": 400,
          "body_tokens": 100,
          "lint_issues": [
            {
              "rule": "no-trigger-phrase",
              "severity": "warning",
              "category": "format",
              "message": "Description lacks trigger phrases (e.g. 'Use when...'); agents may not know when to invoke this skill"
            }
          ]
        }
      ]
    }
  ]
}
```

Lint の問題がない Skill では `lint_issues` フィールドは省略されます。

source 全体を対象にした実行では `mcp` オブジェクトも追加されます。どの MCP Target にもサーバーが渡されない場合は省略されます:

```json
"mcp": {
  "budget": 30000,
  "groups": [
    {
      "target": "claude", "agent": "claude", "tokens": 6750, "measured": 4,
      "servers": [
        { "name": "github", "tools": 26, "tokens": 3276, "checked": "2026-11-25T08:30:00Z" }
      ]
    }
  ]
}
```

`root`（`mcp.projects` のフォルダ）は project のグループに設定されます。サーバーに `project` があるのは project 由来の場合で、`checked` があるのはライブチェックが数えた場合だけです。`checked` がなければ `tokens` は実際の数ではありません。グループの `tokens` は数えたサーバーの合計で、`measured` はその数です。

## Project mode

```bash
skillshare analyze -p                  # Project の Skill のインタラクティブ TUI
skillshare analyze -p --verbose        # Verbose テキスト出力
skillshare analyze -p claude           # 単一 Target の詳細
skillshare analyze -p --json           # JSON 出力
```

## フィルタリング

`--filter` を使うと、結果を Skill のサブセットに絞り込めます。フィルタは Skill の相対パス（グループディレクトリを含む）に対して大文字小文字を区別しない部分一致で行われます。

例えば、`--into frontend` で Skill をインストールした場合:
- `--filter frontend` は `frontend/` グループ内のすべての Skill にマッチ
- `--filter react` はパスに "react" を含む任意の Skill にマッチ

TUI モードでは、`--filter` はフィルタ入力欄を事前入力します。インタラクティブにフィルタするには `/` キーも使用できます。

JSON モードでは、出力に集計済みのトークン数を含む `filtered_summary` が含まれます。

```json
{
  "filter": "frontend",
  "matched_count": 5,
  "total_count": 50,
  "filtered_summary": {
    "always_loaded": { "chars": 2400, "tokens": 600 },
    "on_demand": { "chars": 8000, "tokens": 2000 },
    "total": { "chars": 10400, "tokens": 2600 }
  },
  "skills": [...]
}
```

Web UI でも、検索やフィルタが有効なときは動的なトークンサマリーバーが表示されます。

## 予算の警告

`context_budget` の閾値が設定されている場合、`analyze` はいずれかの Target が予算を超えていると警告を表示します（MCP ツール定義には `warn_mcp_tool_tokens`）。設定の詳細については [sync — Context Cost](/docs/reference/commands/sync#context-cost) を参照してください。

## 関連項目

- [list](/docs/reference/commands/list) — インストール済みの Skill を表示
- [audit](/docs/reference/commands/audit) — Skill のセキュリティ脅威をスキャン
- [tui](/docs/reference/commands/tui) — インタラクティブ TUI の有効/無効を切り替え
