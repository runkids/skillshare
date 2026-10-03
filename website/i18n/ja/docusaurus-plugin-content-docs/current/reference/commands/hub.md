---
sidebar_position: 8
---

# hub

Skill hub を管理します — 組織全体での Skill 発見のために保存された hub source です。

## 使うタイミング

- `search` が問い合わせる、組織の Skill カタログをセットアップする
- 複数の hub（例: 全社共通 vs チーム固有）を切り替える
- 保存済みの hub source を一覧表示または削除する

## hub add

hub source を config に保存し、[`search --hub`](./search.md) で再利用できるようにします。

```bash
skillshare hub add <url> [options]
```

| フラグ | 説明 |
|------|-------------|
| `--label`, `-l` | カスタムラベル（デフォルト: URL のホスト名から導出） |
| `--project`, `-p` | project config に保存 |
| `--global`, `-g` | global config に保存 |

最初に追加された hub は自動的にデフォルトに設定されます。ラベルは大文字小文字を区別しません。

hub URL には、HTTP(S) URL、ローカルファイルパス、または SSH URL（`git@host:org/repo.git` の scp 形式、または `ssh://git@host/org/repo.git` のスキーム形式）を指定できます。SSH source の場合、インデックスファイルは `//path` サフィックス経由でリポジトリから読み込まれ、デフォルトはリポジトリルートの `skillshare-hub.json` です。SSH source は自分の SSH agent/鍵を使ってクローンされるため、プライベートリポジトリや GitHub Enterprise ホストでも動作します。

SSH hub に同一ホストの GitHub または GitHub Enterprise ドメインプレフィックス付きエントリが含まれる場合、Skillshare はその hub の SSH identity 経由でインストールを行います。例えば、`acme@acme.ghe.com:Org/skills.git//hubs/team.json` として追加された hub には `acme.ghe.com/Org/skills/skills/reviewer` を含めることができ、検索結果ではそれを `acme@acme.ghe.com:Org/skills.git//skills/reviewer` としてインストールします。SSH hub の外では、ドメインプレフィックス付きの source は引き続き HTTPS を意味します。

```bash
skillshare hub add https://internal.corp/hub.json --label team
skillshare hub add ./local-hub.json                          # ラベルは導出される: "local-hub"
skillshare hub add git@ghe.corp.com:team/skills.git --label ghe
skillshare hub add git@ghe.corp.com:team/skills.git//hubs/team.json --label ghe-team
```

## hub list

保存済みの hub を一覧表示します。`*` はデフォルトの hub を示します。

```bash
skillshare hub list [options]
```

| フラグ | 説明 |
|------|-------------|
| `--project`, `-p` | project の hub を表示 |
| `--global`, `-g` | global の hub を表示 |

```
$ skillshare hub list
  team      https://internal.corp/hub.json · default
  local     ./local-hub.json

2 hubs
```

エイリアス: `hub ls`

## hub remove

ラベルを指定して保存済みの hub を削除します。

```bash
skillshare hub remove <label> [options]
```

| フラグ | 説明 |
|------|-------------|
| `--project`, `-p` | project config から削除 |
| `--global`, `-g` | global config から削除 |

削除された hub がデフォルトだった場合、デフォルトはクリアされます。

エイリアス: `hub rm`

## hub default

`search --hub`（フラグなし）が使用するデフォルトの hub を表示または設定します。

```bash
skillshare hub default [label] [options]
```

| フラグ | 説明 |
|------|-------------|
| `--reset` | デフォルトをクリア（community hub に戻す） |
| `--project`, `-p` | project config を使用 |
| `--global`, `-g` | global config を使用 |

```bash
skillshare hub default              # 現在のデフォルトを表示
skillshare hub default team         # デフォルトを "team" に設定
skillshare hub default --reset      # デフォルトをクリア → community hub
```

## hub index

インストール済みの Skill から `skillshare-hub.json` インデックスファイルを構築します。生成されたインデックスは、プライベートかつオフラインでの Skill 発見のために [`search --hub`](./search.md#private-index-search) で利用できます。

### 使い方

```bash
skillshare hub index [options]
```

### オプション

| フラグ | 説明 |
|------|-------------|
| `--source`, `-s` | スキャンする source ディレクトリ（デフォルト: 自動検出） |
| `--output`, `-o` | 出力ファイルパス（デフォルト: `<source>/skillshare-hub.json`） |
| `--full` | フルメタデータを含める（flatName、type、version など） |
| `--audit` | 各 Skill にセキュリティ監査を実行し、リスクスコアを含める |
| `--project`, `-p` | project mode を使用（`.skillshare/`） |
| `--global`, `-g` | global mode を使用（`~/.config/skillshare`） |
| `--help`, `-h` | ヘルプを表示 |

### 出力モード

**Minimal（デフォルト）** — 検索とインストールに必要な最小限のフィールドのみ:

```json
{
  "schemaVersion": 1,
  "generatedAt": "2026-02-12T10:00:00Z",
  "sourcePath": "/home/user/.config/skillshare/skills",
  "skills": [
    {
      "name": "my-skill",
      "description": "A useful skill",
      "source": "owner/repo/.claude/skills/my-skill",
      "tags": ["workflow"]
    }
  ]
}
```

**Full（`--full`）** — 監査と管理のためのメタデータを含む:

```json
{
  "name": "my-skill",
  "description": "A useful skill",
  "source": "github.com/owner/repo/.claude/skills/my-skill",
  "tags": ["workflow"],
  "flatName": "my-skill",
  "type": "github-subdir",
  "repoUrl": "https://github.com/owner/repo.git",
  "version": "abc1234",
  "installedAt": "2026-02-10T03:49:06Z",
  "isInRepo": false
}
```

**Audit（`--audit`）** — `skillshare audit` によるセキュリティリスクスコアを追加:

```json
{
  "name": "my-skill",
  "description": "A useful skill",
  "source": "owner/repo/.claude/skills/my-skill",
  "riskScore": 0,
  "riskLabel": "clean",
  "auditedAt": "2026-02-22T10:00:00Z"
}
```

`--audit` は `--full` と組み合わせて、メタデータとリスクスコアの両方を含めることができます。リスクラベル: `clean`（0）、`low`（1–25）、`medium`（26–50）、`high`（51–75）、`critical`（76–100）。

メタデータフィールドは `omitempty` を使用しており、冗長な値は省略されます。
- `flatName` は `name` と同じ場合は省略
- `relPath` は `source` と同じ場合は省略
- `isInRepo` は `false` の場合は省略

### 例

```bash
# minimal インデックスを構築（デフォルト）
skillshare hub index

# フルメタデータで構築
skillshare hub index --full

# セキュリティリスクスコア付きで構築
skillshare hub index --audit

# フルメタデータ + リスクスコア
skillshare hub index --full --audit

# カスタム出力パス
skillshare hub index -o /shared/team/skillshare-hub.json

# カスタム source ディレクトリ
skillshare hub index -s ~/my-skills

# project mode
skillshare hub index -p
```

### ワークフロー

典型的なプライベート hub のワークフロー:

```
1. Skill をインストール        → skillshare install ...
2. インデックスを構築          → skillshare hub index
3. インデックスファイルを共有  → Commit/host skillshare-hub.json (HTTP, file, or Git repo)
4. チームメンバーが検索        → skillshare search --hub [path-url-or-ssh]
```

インデックスがプライベートまたは GitHub Enterprise リポジトリに存在する場合、チームメンバーは各自でローカルにクローンすることなく、`--hub` を SSH 経由で直接指定できます（`git@host:org/repo.git`）。

詳細は [Hub Index Guide](/docs/how-to/sharing/hub-index) を参照してください。

## Config 形式

保存済みの hub は `config.yaml` の `hub:` キーの下に格納されます。

```yaml
hub:
  default: team
  hubs:
    - label: team
      url: https://internal.corp/hub.json
    - label: local
      url: ./local-hub.json
    - label: ghe
      url: git@ghe.corp.com:team/skills.git//hubs/team.json
```

[public hub](https://github.com/runkids/skillshare-hub) は組み込みのデフォルトであり、保存する必要はありません。カスタムのデフォルトが設定されていない場合、`search --hub` は自動的にこれにフォールバックします。このリポジトリを fork して、自分の組織の hub を立ち上げてください。
