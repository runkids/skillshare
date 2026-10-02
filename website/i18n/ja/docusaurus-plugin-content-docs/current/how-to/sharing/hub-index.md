---
sidebar_position: 4
---

# Hub Index ガイド

組織のための集中管理された Skill カタログを構築します — GitHub API もトークンも不要です。

## なぜ Hub Index を使うのか？

Hub index は、Skill の名前、説明、Source を列挙した JSON ファイル（`skillshare-hub.json`）です。
社内でホストすれば、すべてのチームメンバーがそこから Skill を検索・インストールできます。

| ユースケース | GitHub 検索 | Hub Index |
|----------|--------------|-----------|
| 組織全体の Skill カタログ | いいえ | **はい** |
| プライベート/社内の Skill | いいえ | **はい** |
| エアギャップ / VPN 専用の環境 | いいえ | **はい** |
| 厳選・承認済みの Skill セット | いいえ | **はい** |
| GitHub トークン不要 | いいえ | **はい** |

実例については [Public Hub](#public-hub) セクションを参照してください。

## クイックスタート

### 1. インデックスを構築する

```bash
# Global の Skill から
skillshare hub index

# プロジェクトから
skillshare hub index -p

# 出力: <source>/skillshare-hub.json
```

### 2. インデックスを検索する

```bash
# ローカルファイル
skillshare search react --hub ./skillshare-hub.json

# リモート URL
skillshare search react --hub https://internal.corp/skills/skillshare-hub.json

# すべての Skill を閲覧する（クエリなし）
skillshare search --hub ./skillshare-hub.json --json
```

### 3. 結果からインストールする

インタラクティブな検索フローは GitHub 検索と同じように動作します — Skill を選択するとインストール
されます。

## 監査の充実

Skill の安全性を一目で確認できるよう、インデックスにセキュリティのリスクスコアを追加します。

```bash
# 監査スコア付きでインデックスを構築する
skillshare hub index --audit

# 完全なメタデータと組み合わせる
skillshare hub index --full --audit
```

`--audit` を使うと、各 Skill は `skillshare audit` のルールでスキャンされ、インデックスには
`riskScore`（0〜100）、`riskLabel`（clean/low/medium/high/critical）、`auditedAt` タイムスタンプが
含まれます。スキャンに失敗した Skill はリスクフィールドなしで含まれます。

監査済みインデックスからの検索結果にはリスクバッジが表示されます。

```
  1. safe-skill               owner/repo/safe-skill         [clean]
  2. risky-skill              owner/repo/risky-skill        [high]
```

## 共有戦略

### ファイル共有（最もシンプル）

インデックスファイルを共有場所にコピーします。

```bash
skillshare hub index -o /shared/team/skillshare-hub.json
```

チームメンバーは以下で検索します。
```bash
skillshare search --hub /shared/team/skillshare-hub.json
```

### HTTP サーバー

インデックスをローカルで生成してからホスティングにアップロードします。

```bash
# ステップ 1: 生成する
skillshare hub index -o ./skillshare-hub.json

# ステップ 2: アップロードする（お好みの方法で）
scp ./skillshare-hub.json server:/var/www/skills/
# または: aws s3 cp ./skillshare-hub.json s3://my-bucket/
# または: rsync、FTP など
```

チームメンバーは以下で検索します。
```bash
skillshare search --hub https://skills.company.com/skillshare-hub.json
```

### Git リポジトリ

チームメンバーが pull できるよう、インデックスを共有リポジトリにコミットします。

```bash
skillshare hub index -o ./skillshare-hub.json
git add skillshare-hub.json && git commit -m "Update skill index"
git push
```

チームメンバーは raw URL 経由、SSH 経由、またはローカルに clone して検索できます。
```bash
# raw URL 経由
skillshare search --hub https://raw.githubusercontent.com/team/skills/main/skillshare-hub.json

# SSH 経由 — リポジトリを clone してインデックスを読む（手動 clone 不要）
skillshare search --hub git@github.com:team/skills.git
skillshare search --hub git@ghe.corp.com:team/skills.git//hubs/team.json

# または clone してローカルで検索する
git pull
skillshare search --hub ./skillshare-hub.json
```

:::tip プライベートリポジトリと GitHub Enterprise
SSH の Hub Source は自分の SSH エージェント/キーで clone されるため、プライベートリポジトリや、
raw HTTPS URL がログインページにリダイレクトされる GitHub Enterprise（GHE）ホストでも動作します。
リポジトリ内のインデックスのパスは `//path` サフィックスから取得され、デフォルトはリポジトリルートの
`skillshare-hub.json` です。scp スタイル（`git@host:org/repo.git`）とスキームスタイル
（`ssh://git@host/org/repo.git`）の URL のどちらも動作します。ラベルで検索するには
[`hub add`](/docs/reference/commands/hub#hub-add) で一度保存しておいてください。

GitHub/GHE の Hub が SSH 経由で読み込まれた場合、同一ホストのドメインプレフィックス付き Skill
Source は、その Hub の SSH アイデンティティを継承します。例えば `acme@acme.ghe.com:Org/skills.git//hubs/team.json`
という Hub URL があれば、`acme.ghe.com/Org/skills/skills/reviewer` というエントリの Source は
SSH 経由でインストールできます。Hub が HTTP、ローカルファイル、または別のホスト経由で読み込まれた
場合、ドメインプレフィックス付き Source は HTTPS の Source のままです。
:::

## Web ダッシュボード

### JSON を書かずに Hub を作成する

ダッシュボード（`skillshare ui`）で **スキル → Hub** を開き、**Hub を追加・作成 → 新しい Hub を作成** を選びます。新しい Hub はそのまま編集状態で開きます。

1. Hub に **名前** と任意の **説明** を付けます。名前は共有する `skillshare hub add` コマンドの
   `--label` になります。どちらもエクスポートされたインデックスには含まれません。
2. **skill を追加** を選びます。**URL を貼り付け** タブでは **Git URL** を入力して **探す** を選び、
   **バージョン** を選んでから追加する Skill を選択します。**インストール済み** タブでは、このマシンに
   インストール済みの Skill を選択します。または **見つからない場合はソースを手入力** を選んで空の行を
   追加します。
3. 各 Skill の **名前**、**ソース**、**バージョン** を編集します。例えば、
   `runkids/demo-skills/skills/pdf` はリモートリポジトリ内の Skill を識別します。行を展開すると
   **スキルの説明**、**タグ（カンマ区切り）**、**スキルセレクター（任意）** を編集できます。
   スキルセレクターは、複数の Skill を含むリポジトリ内の Skill を選択します。
4. **保存** を選びます。ページはすべてのエントリをチェックします。他の人がインストールできない Skill が
   あれば、エディターは開いたままその行に印を付けます。
5. **共有 → skillshare-hub.json をダウンロード** を選びます。印の付いた Skill を **編集** で直すまで、
   ダウンロードはできません。
6. ダウンロードしたファイルを自分の Git リポジトリにコミットするか、HTTP サーバーにアップロード
   します。**共有** ダイアログにその URL を貼り付けると、受け取り側のための `skillshare hub add`
   コマンドをコピーできます。URL は Hub と一緒に保存されます。

ダウンロードは何も公開する**わけではありません**。カタログは Skill を参照するだけで、そのファイルを
バンドルするわけではありません。Source の検証は構文をチェックするだけで、リポジトリが存在するか、
受け取り側に権限があるかは確認しません。プライベートリポジトリには引き続きアクセス権が必要です。

:::tip ローカルの Skill も Hub に残せる
既知のリモート由来を持たないインストール済みの Skill は、そのローカル Source を保持します。
それを Hub に保存できます。リモートのインストール Source を指定するかそのエントリを削除するまで
ダウンロードはブロックされます。ビルダーがそれを黙って除外することはありません。
:::

### カタログを再開またはインポートする

自分の Hub は Hub 一覧で **自分の** と表示されます。これらはダッシュボードを実行しているマシン上の、
有効な設定ファイルの隣にある `hub-drafts/` に保存されます。Global と Project の設定は別々の Hub を
持ちます。再読み込みする前に **保存** を選んでください。編集中は Hub 一覧がロックされ、未保存の変更を
残したまま離れると破棄するか確認されます。古くなったウィンドウからの保存は拒否されるため、より新しい
リビジョンを上書きすることはできません。**キャンセル** は最後に保存された版を読み込み直します。

既存の v1 `skillshare-hub.json`（4 MB まで）には **Hub を追加・作成 → skillshare-hub.json を読み込む**
を使います。非対応のバージョンや無効なフィールド型はエラーになります。同じ表示名を持つエントリは
別々のまま残ります。追加の JSON フィールドや `skill` セレクターは保持されます。古いインデックスに
`sourcePath` が含まれる場合、相対 Source は既存のインデックスリーダーと同様にローカルパスとして
解決されます。エクスポート前にリモート Source に変更する必要があります。

ポータブルなエクスポートは、作者の `sourcePath` と既知のローカルメタデータ（`relPath`、`flatName`、
`installedAt`、`isInRepo`）を取り除きます。それにはインデックスが含まれ、Hub の名前、説明、
ID、リビジョン、公開先 URL は含まれません。エントリの Source や skill セレクターを変更すると、その前の
監査スコア、ラベル、タイムスタンプはクリアされます。URL の認証情報、クエリ文字列、フラグメントは
拒否されます。リポジトリの認証は別途設定してください。

**その他の操作 → Hub を削除** は確認を求め、その Hub のみを削除します。Skill をアンインストールしたり、
ホストされているインデックスを削除したり、サブスクライブ済みの Hub を削除したりすることはありません。

### 共有された Hub を検索する

1. **スキル → インストール** を開き、**検索** を選びます。
2. **検索先** セレクターで Hub を選びます。URL、SSH リポジトリ、またはローカルのインデックスパスを
   追加するには、セレクターの横の **ハブを管理** を選び、Hub ページで
   **Hub を追加・作成 → 既存の Hub を追加** を選びます。
3. Skill を検索、プレビュー、インストールします。

Hub ページで Hub を選び、その Skill を絞り込んでインストールすることもできます。サブスクライブ済みの
Hub Source は有効な skillshare 設定に保存され、CLI と共有されます。これらは自分の Hub とは別物です。

既存の `skillshare hub index` コマンドと `/api/hub/index` エンドポイントは、ローカル Source の
対応を含め、これまで通りインデックスを生成し続けます。上記のポータブルエクスポートのルールは
ダッシュボードのビルダーにも適用されます。

## インデックスのスキーマ

インデックスは Schema v1 に従います。

```json
{
  "schemaVersion": 1,
  "generatedAt": "2026-02-12T10:00:00Z",
  "sourcePath": "/home/user/.config/skillshare/skills",
  "skills": [
    {
      "name": "my-skill",
      "description": "Does something useful",
      "source": "owner/repo/.claude/skills/my-skill",
      "tags": ["workflow", "productivity"]
    }
  ]
}
```

### 必須フィールド（コンシューマー契約）

| フィールド | 必須 | 説明 |
|-------|----------|-------------|
| `name` | はい | Skill の表示名 |
| `source` | はい | インストール Source（GitHub の省略形、URL、またはローカルパス） |
| `description` | 推奨 | 検索マッチング用の短い説明 |
| `skill` | いいえ | 複数の Skill を含むリポジトリ内の特定の Skill 名（`install -s` と共に使用） |
| `tags` | いいえ | フィルタリングとグループ化のための分類タグ |

### ドキュメントレベルのフィールド

| フィールド | 説明 |
|-------|-------------|
| `schemaVersion` | 常に `1` |
| `generatedAt` | RFC 3339 タイムスタンプ |
| `sourcePath` | 相対 Source を解決するためのベースパス |

### Source パスの解決

`sourcePath` が設定されていて、Skill の `source` が相対パスの場合、検索側のコンシューマーはそれらを
結合します。

```
sourcePath: /home/user/.config/skillshare/skills
source:     _team/frontend-skill
→ resolved: /home/user/.config/skillshare/skills/_team/frontend-skill
```

これにより、相対パスが GitHub の省略形（`owner/repo`）と誤解釈されるのを防ぎます。

### Source をタグまたはコミットに固定する

エントリを特定のバージョンに固定するには、パスに ref を含む Web URL を使います。`tree/` または `blob/`（GitHub）、`-/tree/` または `-/blob/`（GitLab）、`src/`（Bitbucket）の後にあるブランチ、タグ、または commit SHA が、`install --branch` と同じようにインストール ref として使われます。

```json
{
  "name": "reviewer",
  "source": "github.com/owner/repo/tree/v1.2.0/skills/reviewer"
}
```

Hub からインストールするすべての人がそのリビジョンを取得し、`skillshare update` もそれを維持します。固定を移動するには、インデックス内の ref を編集します。リモートに存在しない ref はデフォルトブランチにフォールバックせず、インストールが失敗します。

絶対パス、URL、ドメインプレフィックス付きのパスは決して結合されません。

| Source パターン | 結合されるか？ |
|----------------|---------|
| `_team/my-skill` | はい |
| `subdir/skill` | はい |
| `/absolute/path` | いいえ |
| `github.com/owner/repo/skill` | いいえ |
| `https://...` | いいえ |

## 手書きのインデックス

`hub index` を使わずに手動でインデックスを作成することもできます。これは特に、GitHub 検索や公開
ツールが決して到達できない Source であるプライベートインフラでホストされている社内 Skill に有用です。

```json
{
  "schemaVersion": 1,
  "skills": [
    {
      "name": "company-style",
      "description": "Company coding standards and review checklist",
      "source": "ghe.internal.company.com/platform/ai-skills/company-style",
      "tags": ["quality", "workflow"]
    },
    {
      "name": "deploy-helper",
      "description": "Internal deployment automation",
      "source": "gitlab.internal.company.com/ops/skills/deploy-helper",
      "tags": ["devops"]
    },
    {
      "name": "onboarding",
      "description": "New hire onboarding skill for AI assistants",
      "source": "ghe.internal.company.com/hr/ai-skills/onboarding",
      "tags": ["workflow"]
    }
  ]
}
```

:::tip なぜ GitHub 検索だけではだめなのか？
`skillshare search` は github.com 上の公開リポジトリしか見つけられません。Hub index は
**あらゆる** Source を指すことができます — GitHub Enterprise、プライベートな GitLab、社内サーバーなど、
VPN の背後にいる自社の従業員だけがアクセスできるものです。これが Hub を組織全体の Skill 配布の
定番ソリューションにしている理由です。
:::

手書きインデックスのヒント:
- `sourcePath` は任意です — すべての Source が絶対パスなら省略してください
- `tags` は任意です — Web サイトや検索でのフィルタリングに便利です
- `name` が空の Skill はスキップされます
- 結果は名前のアルファベット順にソートされます
- SSH 専用の GitHub Enterprise インストールでは、明示的な SSH Source
  （`user@host:owner/repo.git//path`）を優先するか、Hub 自体を SSH 経由で読み込んで、同一ホストの
  GitHub/GHE ドメインプレフィックス付きエントリがその SSH アイデンティティを継承するようにして
  ください

## 組織へのデプロイ

プライベート Hub は、レビュー済みの Skill を検索できるカタログを提供します。カタログと Skill の Source は組織が管理するインフラに置いてください。認証とアクセス制御は Git ホストまたは HTTP サーバーが提供します。

### 1. Skill と Source を選定する

Skill とカタログの変更は PR でレビューします。SSH 専用の Git ホストでは、[インデックスのエントリー](#手書きのインデックス)に明示的な SSH Source を指定してください。例:

```json
{
  "schemaVersion": 1,
  "skills": [
    {
      "name": "code-review",
      "description": "Team code-review checklist",
      "source": "git@ghe.example.com:platform/ai-skills.git//skills/code-review"
    }
  ]
}
```

インストール済みのリモート Skill から `skillshare hub index --audit` でカタログを生成することもできます。公開前に、各 Source にチームメンバーがアクセスできることを確認してください。ローカルファイルから生成したインデックスには、そのマシン固有のパスが含まれる場合があるので、共有 Source に置き換えます。Audit バッジはある時点のスキャン結果であり、恒久的な承認ではありません。

### 2. レビュー済みの CLI バージョンで変更を Audit する

Skill リポジトリでは、固定した CLI バージョンと重大度のしきい値で PR を検査します:

```yaml
name: Validate shared skills
on:
  pull_request:
    paths: ['skills/**', 'skillshare-hub.json']

jobs:
  audit:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      # 読みやすさのためにタグを使用。各 Action はレビュー済みのコミット SHA に固定してください
      - uses: actions/checkout@v4
      - uses: runkids/setup-skillshare@v1
        with:
          version: '0.23.5' # 例: チームがレビューした CLI バージョンを選んでください
          source: ./skills
          audit: true
          audit-threshold: high
```

この例は、チェックアウトしたリポジトリの `skills/` に Skill があることを前提とします。社内 Git サーバーの場合は、CI runner の checkout とアクセス設定を使ってください。スキャンコマンドは同じです。他の CI システムについては [CI/CD Skill 検証](/docs/how-to/recipes/ci-cd-skill-validation)を参照してください。

Action の `version` input が固定するのは CLI release であり、Action 自体や Skill の内容ではありません。例では読みやすさのためにタグを使っています。組織のポリシーに従い、各 Action をレビュー済みの完全なコミット SHA に固定してください。[プロジェクトの lockfile](/docs/understand/project-skills#lockfile)でリモート Skill のコミットを記録し、それぞれの更新を別々にレビューします。`hub index --audit` はカタログにスキャン結果を追加するだけです。その重大度の検出結果を拒否するには、`skillshare audit --threshold high` または上のパイプラインゲートを使います。

### 3. カタログを非公開で配布する

レビュー後、`skillshare-hub.json` を社内 Skill リポジトリのルートにコミットします。Git ホストでメンバーに読み取り権限を与えてください。チームの環境からインデックスを取得できるなら、社内 HTTP ホスティングも使えます。公開 Hub を fork したり、公開 raw URL を用意したりする必要はありません。

### 4. 登録、検索、同期する

[skillshare の初期化](/docs/getting-started/first-sync)後、チームメンバーはプライベートカタログを一度だけ登録します:

```bash
skillshare hub add git@ghe.example.com:platform/ai-skills.git --label company -g
skillshare search code-review --hub company -g
# Select a skill to install, then distribute it to global targets
skillshare sync -g
```

SSH の Hub URL は、リポジトリのルートから `skillshare-hub.json` を読みます。カタログが別の場所にある場合は、`git@ghe.example.com:platform/ai-skills.git//catalog/skillshare-hub.json` のようにパスを付けてください。SSH アクセスにはメンバーの既存の SSH 設定を使います。Git ホストはカタログと各 Skill の Source の両方へのアクセスを許可する必要があります。Hub は Skill を見つける仕組みであり、他の Source からのインストールを禁止しません。

### 5. プロジェクトの依存を記録する

特定のプロジェクトに必要な Skill は Project mode でインストールし、生成された config と lockfile をコミットします。メンバーは clone または更新の pull 後に `skillshare install -p`、Audit、Sync を実行します。順序は[チームオンボーディング](/docs/how-to/recipes/team-onboarding-recipe)を参照してください。カタログの選定、Skill の更新、CLI のアップグレードは、それぞれ明示的にレビューする変更として扱います。

## Public Hub

[skillshare-hub](https://github.com/runkids/skillshare-hub) は、質の高い Skill の厳選されたカタログ
です。これは**デフォルトの Hub**です — Source を指定せずに `search --hub` を実行すると、ここが検索
されます。

```bash
skillshare search --hub              # public hub のすべての Skill を閲覧する
skillshare search react --hub        # "react" の Skill を検索する
```

また、自分の組織の Hub を構築するためのリファレンスとしても機能します。

- **インデックス構造** — 名前、説明、Source、タグを使って `skillshare-hub.json` をどう整理するか
- **CI 検証** — すべての PR での自動 JSON フォーマットチェックと `skillshare audit` セキュリティスキャン
- **コントリビューションワークフロー** — Fork → エントリ追加 → PR、CI ゲート付き

自分のチーム用の社内 Hub を構築したいですか？リポジトリを fork し、Skill を自分の組織のカタログに
置き換え、自分たちのセキュリティポリシーに合わせて CI パイプラインをカスタマイズしてください。

## ヒント

- **インデックス生成を自動化する** — Skill の変更後、CI パイプラインに `skillshare hub index` を追加する
- **監査には `--full` を使う** — Full モードにはバージョン、インストール日、種類の情報が含まれる
- **Project mode と組み合わせる** — `skillshare hub index -p` はプロジェクトレベルの Skill のみを
  インデックス化する

---

## 関連項目

- [search](/docs/reference/commands/search) — Hub から Skill を検索する
- [hub](/docs/reference/commands/hub) — Hub Source を管理する
- [install](/docs/reference/commands/install) — 発見された Skill をインストールする
