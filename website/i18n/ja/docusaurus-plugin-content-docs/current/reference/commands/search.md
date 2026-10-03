---
sidebar_position: 5
---

# search

GitHub リポジトリから Skill を発見し、インストールします。

## こんなときに使う

- ハブのインデックスからコミュニティの Skill を発見する
- 名前、タグ、説明で Skill を探す
- インストール前に利用可能な Skill を閲覧する

## クイックスタート

```bash
skillshare search vercel       # キーワードで検索
skillshare search              # 人気の Skill を閲覧
```

これは、クエリに一致する `SKILL.md` ファイルを含むリポジトリを GitHub 上で検索します。

## ブラウズモード

クエリを指定しない場合、`search` は GitHub 上の人気の Skill を閲覧します。

```bash
skillshare search              # 人気の Skill を閲覧
skillshare search --list       # 人気の Skill を一覧表示
```

これは `filename:SKILL.md` を GitHub のクエリとして使用し、スター数でソートして、最も人気のある Skill リポジトリを先頭に表示します。

## 仕組み

```
skillshare search [query]
        │
        ▼
GitHub Code Search API (filename:SKILL.md + query)
        │
        ▼
Fetch star counts for each repository
        │
        ▼
Sort by stars (most popular first)
        │
        ▼
Interactive selector → Install selected skill
```

## インストールする Skill を選ぶ

ターミナルでは、結果がセレクターで開き、カーソル位置の Skill の詳細が一覧の横に表示されます。複数を選択してまとめてインストールでき、何も選択していない場合は `Enter` でカーソル位置の Skill をインストールします。キーは画面下部に表示されます。

## オプション

| フラグ | 説明 |
|------|-------------|
| `--project`, `-p` | プロジェクトレベルの config（`.skillshare/`）にインストール |
| `--global`, `-g` | グローバル config（`~/.config/skillshare`）にインストール |
| `--hub [URL]` | ハブインデックスから検索（デフォルト: [skillshare-hub](https://github.com/runkids/skillshare-hub)。またはカスタムの URL/パス） |
| `--list`, `-l` | インストールプロンプトなしで結果のみ表示 |
| `--json` | JSON として出力（スクリプト向け） |
| `--limit N`, `-n N` | 最大結果数（デフォルト: 20、最大: 100） |
| `--help`, `-h` | ヘルプを表示 |

:::tip 自動検出
`--project` も `--global` も指定されない場合、skillshare は自動検出を行います。カレントディレクトリに `.skillshare/config.yaml` が存在すればプロジェクトモードに、存在しなければグローバルモードになります。
:::

## 使用例

### 人気の Skill を閲覧

```bash
skillshare search              # 人気の Skill を閲覧（クエリなし）
```

### 基本的な検索

```bash
skillshare search pdf           # インタラクティブに検索してインストール
skillshare search "code review" # 複数単語の検索
```

### リストモード

```bash
skillshare search commit --list
```

出力:
```
  fix            facebook/react/.claude/skills/fix         ★ 242.7k
                 Use when you have lint errors, formatting issues...
  verify         facebook/react/.claude/skills/verify      ★ 242.7k
                 Use when you want to validate changes before committing...
  commit-helper  ...ockroach/.claude/skills/commit-helper  ★ 31.8k
                 Help create git commits and PRs with properly formatted messages...

Found 3 skills

Next
  skillshare install <source>  install one
```

### JSON 出力

```bash
skillshare search react --json --limit 5
```

```json
[
  {
    "Name": "react-patterns",
    "Description": "React and Next.js performance optimization...",
    "Source": "facebook/react/.claude/skills/react-patterns",
    "Stars": 242700,
    "Owner": "facebook",
    "Repo": "react",
    "Path": ".claude/skills/react-patterns"
  }
]
```

### プロジェクトモード

```bash
skillshare search pdf -p           # 検索してプロジェクトにインストール
skillshare search react --project  # 同じ動作、ロングフラグ版
```

インストールされた Skill は `.skillshare/skills/` に配置され、プロジェクト config は自動的に更新されます。プロジェクトがまだ初期化されていない場合、skillshare は先に `init -p` を実行します。

### 結果数の制限

```bash
skillshare search frontend -n 5   # 上位 5 件のみ表示
```

## 認証 {#authentication}

GitHub Code Search API には認証が必要です。skillshare は自動的に認証情報を検出します。

1. **GitHub CLI**（推奨） — `gh` でログイン済みの場合:
   ```bash
   gh auth login
   ```

2. **環境変数** — `GITHUB_TOKEN` または `GH_TOKEN` を設定:
   ```bash
   export GITHUB_TOKEN=ghp_your_token_here
   ```

### トークンの作成

`gh` CLI を使用していない場合:

1. [GitHub Settings → Tokens](https://github.com/settings/tokens) にアクセス
2. 新しいトークンを生成（classic）
3. パブリックリポジトリにはスコープ不要
4. トークンを設定:
   ```bash
   export GITHUB_TOKEN=ghp_your_token_here
   ```

## 検索結果のランキング方法

1. **検索** — GitHub Code Search がクエリに一致する `SKILL.md` ファイルを探す
2. **フィルタ** — フォークされたリポジトリ（重複）を除去
3. **スター数の取得** — 各ユニークなリポジトリのスター数を取得
4. **ソート** — スター数（人気順）で並び替え
5. **上限** — 上位 N 件を返す

これにより、高品質で人気のある Skill が先頭に表示されます。

## コミュニティ Hub

[skillshare-hub](https://github.com/runkids/skillshare-hub) からコミュニティがキュレーションした Skill を閲覧・インストールできます。

```bash
skillshare search --hub                # skillshare-hub 内のすべての Skill を閲覧
skillshare search react --hub          # skillshare-hub 内で "react" を検索
```

`--hub` を URL なしで使用すると、コミュニティの [skillshare-hub](https://github.com/runkids/skillshare-hub) インデックスがデフォルトで使われます。

あなたの Skill をコミュニティと共有しませんか？[PR を開いて](https://github.com/runkids/skillshare-hub) Skill を追加してください — CI がすべての投稿に対して `skillshare audit` を実行します。

## 保存済みのハブラベル

[`hub add`](./hub.md#hub-add) でハブを保存し、フル URL を入力する代わりにラベルで検索できます。

```bash
# 一度ハブを保存
skillshare hub add https://internal.corp/hub.json --label team

# ラベルで検索
skillshare search react --hub team

# 引数なしの --hub のデフォルトに設定
skillshare hub default team
skillshare search --hub              # "team" ハブを使用
```

`--hub <value>` の解決順序:
1. URL またはパス（`http`、`/`、`.`、`~`、`file://`、または `git@…`/`ssh://…` のような SSH URL で始まる）→ そのまま使用
2. それ以外 → 保存済みハブからのラベル検索
3. 値なしの `--hub` → config のデフォルト → コミュニティハブへのフォールバック

保存済みハブの管理については [`hub`](./hub.md) を参照してください。

## プライベートインデックス検索 {#private-index-search}

GitHub の代わりにプライベートなハブインデックスから検索します。

```bash
# ローカルファイル
skillshare search react --hub ./skillshare-hub.json

# HTTP URL
skillshare search react --hub https://internal.corp/skills/skillshare-hub.json

# SSH URL — リポジトリをクローンしてインデックスを読み込む（プライベート/GHE ホストでも動作）
skillshare search react --hub git@github.com:org/skills.git
skillshare search react --hub git@ghe.corp.com:team/skills.git//hubs/team.json

# すべての Skill を閲覧（空のクエリ）
skillshare search --hub ./skillshare-hub.json --json

# イコール構文も使用可能
skillshare search react --hub=./skillshare-hub.json
```

:::note SSH ハブソース
SSH の `--hub` 値は、（あなたの SSH エージェント/キーを使って）リポジトリをシャロークローンし、その中のインデックスファイルを読み込むことで解決されます。リポジトリ内のファイルパスは `//path` サフィックスから決まります — `git@host:org/repo.git//hubs/team.json` — 省略した場合はリポジトリルートの `skillshare-hub.json` がデフォルトになります。scp スタイル（`git@host:org/repo.git`）とスキームスタイル（`ssh://git@host/org/repo.git`）の両方の URL がサポートされます。

[web ダッシュボード](./ui.md) では、SSH ハブソースは事前に [保存](./hub.md#hub-add) しておく必要があります。サーバーは保存済みのハブのみをクローンします。
:::

インデックスの構築には [`hub index`](./hub.md) を使用します。

```bash
skillshare hub index                           # skillshare-hub.json を生成
skillshare search --hub ./skillshare-hub.json  # それを検索
```

:::tip デフォルトハブ
`skillshare search --hub`（URL なし）は、デフォルトでコミュニティの [skillshare-hub](https://github.com/runkids/skillshare-hub) インデックスを使用するため、毎回フル URL を入力する必要はありません。または `skillshare hub default <label>` で独自のデフォルトを設定できます。
:::

詳細は [Hub Index Guide](/docs/how-to/sharing/hub-index) を参照してください。

## ヒント

### 公式の Skill を探す

有名な組織を検索します。
```bash
skillshare search anthropic    # Anthropic の Skill
skillshare search facebook     # Meta/Facebook の Skill
skillshare search vercel       # Vercel の Skill
```

### 特定の機能を探す

やりたいことで検索します。
```bash
skillshare search "pull request"
skillshare search deployment
skillshare search testing
skillshare search database
```

### 連続検索

結果が見つからなかったとき、またはセレクターで `Esc` を押したときは、新しいキーワードを尋ねられます。

```
? Search again (or press Enter to quit): react
```

## トラブルシューティング

### "GitHub Code Search API requires authentication"

`gh auth login` を実行するか、`GITHUB_TOKEN` を設定してください。[認証](#authentication) を参照してください。

### "GitHub API rate limit exceeded"

- 認証済みユーザー: Code Search で 1 分間に 30 リクエスト
- 1 分待って再試行してください
- API 呼び出しを減らすために `--limit` を使用してください

### 新しいリポジトリが見つからない

GitHub は新しいリポジトリのインデックス化に遅延があります（数時間〜数日）。リポジトリが見つからない場合:
- 直接インストールしてください: `skillshare install owner/repo/path/to/skill`
- GitHub がリポジトリをインデックスするのを待ってください

### 検索結果がクエリと一致しない

GitHub Code Search は `SKILL.md` ファイル内のコンテンツにマッチします。説明に "vercel" と書かれた Skill は、たとえ Vercel に特化していなくても、vercel の検索結果に表示されます。

インストール前に結果を確認するには `--list` を使用してください。
