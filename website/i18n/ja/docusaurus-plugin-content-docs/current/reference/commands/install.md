---
sidebar_position: 3
---

# install

GitHub リポジトリ、git URL、またはローカルパスから Skill を追加します。

Tracked repo の Agent 別コピーが少数の論理 Skill を表す場合、`Found N skills, M variants` と表示します。すべての物理バリアントを Source に保持し、merge/copy Sync は一致する Agent の指示を選択します。

## 概要

```mermaid
flowchart TD
    INSTALL["install"] --> SOURCE["source"]
    SOURCE --> SYNC1["sync"] --> TARGETS["targets"]
    SOURCE --> UPDATE["update"]
    SOURCE --> UNINSTALL["uninstall"] --> SYNC2["sync"] --> REMOVED["removed from targets"]
```

## こんなときに使う

- GitHub、GitLab、Bitbucket、Azure DevOps、またはローカルパスから新しい Skill を追加する
- 組織で共有している Skill リポジトリをインストールする（`--track` を使用）
- 既存の Skill を再インストールまたは更新する（`--update` または `--force` を使用）

---

## クイックサンプル

```bash
# GitHub から（省略形）
skillshare install anthropics/skills/skills/pdf

# リポジトリ内の利用可能な Skill を閲覧
skillshare install anthropics/skills

# ローカルパスから
skillshare install ~/Downloads/my-skill

# トラック対象のリポジトリとして（チーム共有用）
skillshare install github.com/team/skills --track

# サブディレクトリへインストール（カテゴリ別に整理）
skillshare install ~/my-skill --into frontend

# config に記載された全 Skill をインストール（引数なし）
skillshare install
```

## ソースの形式

### GitHub 省略形

`owner/repo` 形式を使うと、自動的に `github.com/owner/repo` に展開されます。

```bash
skillshare install anthropics/skills                    # ブラウズモード
skillshare install anthropics/skills/skills/pdf         # 直接インストール
skillshare install ComposioHQ/awesome-claude-skills     # 別のリポジトリ
```

### GitLab / Bitbucket / その他のホスト

GitHub 以外のホストでは `domain/owner/repo` 形式を使用します。

```bash
skillshare install gitlab.com/user/repo                 # GitLab
skillshare install bitbucket.org/team/skills            # Bitbucket
skillshare install git.company.com/team/skills          # セルフホスト
```

フル URL や SSH も使用できます。

```bash
skillshare install https://gitlab.com/user/repo.git
skillshare install git@gitlab.com:user/repo.git
```

:::tip カスタムドメイン上のセルフマネージド GitLab
ホスト名に `gitlab` または `jihulab` を含む場合、ネストしたサブグループのサポートを含めて自動検出されます。その他のカスタムドメイン上のセルフマネージド GitLab インスタンス（例: `git.company.com`）の場合は、config の [`gitlab_hosts`](/docs/reference/targets/configuration#gitlab_hosts) にホスト名を追加すると、skillshare が URL パス全体をリポジトリとして扱うようになります。config を設定しない場合は、回避策として `.git` を末尾に付けることもできます: `git.company.com/team/frontend/ui.git`。
:::

### Azure DevOps

`ado:` の省略形、またはフルの Azure DevOps URL を使用します。

```bash
# 省略形（ado:org/project/repo）
skillshare install ado:myorg/myproject/myrepo
skillshare install ado:myorg/myproject/myrepo/skills/react    # サブディレクトリ付き

# フル HTTPS URL
skillshare install https://dev.azure.com/myorg/myproject/_git/myrepo

# レガシー形式（自動正規化）
skillshare install https://myorg.visualstudio.com/myproject/_git/myrepo

# SSH
skillshare install git@ssh.dev.azure.com:v3/myorg/myproject/myrepo
```

## ブラウズモード（Skill の閲覧）

パスを指定しない場合、skillshare はリポジトリをクローンし、Skill をスキャンして、インタラクティブなピッカーを表示します。

```bash
skillshare install anthropics/skills
```

```text
$ skillshare install anthropics/skills
✓ Source    github.com/anthropics/skills
  Found     20 skills

? Install which skills?
› ○ academy-guide        Complete terms in LICENSE.txt
  ○ algorithmic-art      Complete terms in LICENSE.txt
  ○ brand-guidelines     Complete terms in LICENSE.txt
  ○ canvas-design        Complete terms in LICENSE.txt
  ○ claude-api           Complete terms in LICENSE.txt
  ○ discernment-nudge
  ○ doc-coauthoring
  ○ docx                 Proprietary. LICENSE.txt has complete terms
  …

  ↑↓ move · space toggle · ctrl+a all · enter confirm · / filter · esc cancel
```

ディスカバリーは、`.git` と、通常リポジトリの Skill のコピーを置く `.claude/` や `.agents/` などのターゲットディレクトリを除き、すべてのディレクトリを `SKILL.md` ファイルについてスキャンします。`.curated/` や `.system/` のようなその他の隠しディレクトリはスキャンされるため、その中の Skill も自動的に検出されます。ほかに Skill がないリポジトリでは、代わりにルートの `.agents/skills/` がスキャンされます。複数の Skill が見つかった場合、選択プロンプトは同じフォルダの Skill をまとめて並べ、各 Skill の説明とフォルダを表示します。リストが長いときは `/` で絞り込めます。

`.agents/skills/` のフォールバックでは、`.agents` と `skills` の両方が実ディレクトリである必要があります。`follow_source_links: true` でもシンボリックリンクやジャンクションは拒否されます。意図的にたどるリポジトリルートは引き続き対応します。検出した Agent のインストールと更新は保存先の途中または末尾のリンクを拒否し、公開前に監査します。監査でブロックされた場合、既存の Agent の内容は変わりません。

リポジトリのルートに `.skillignore` ファイルがある場合、一致する Skill はディスカバリーから自動的に除外されます。詳細は下記の [.skillignore](#skillignore) を参照してください。

Skill の `SKILL.md` に `license:` フロントマターフィールドが含まれている場合、そのライセンスは選択プロンプト（例: `my-skill (MIT)`）と、単一 Skill インストール時の確認画面に表示されます。

**Tip**: インストールせずにプレビューするには `--dry-run` を使用します。
```bash
skillshare install anthropics/skills --dry-run
```

## 選択インストール（非インタラクティブ）

プロンプトなしで、複数 Skill を含むリポジトリから特定の Skill を選びます。`--skill` フラグは **fuzzy matching** と **glob パターン** をサポートしています。完全一致する名前が見つからない場合、glob マッチング（`*`、`?`、`[...]`）を試し、それでも見つからなければ最も近い部分文字列マッチにフォールバックします。

```bash
# 名前で特定の Skill をインストール（完全一致または fuzzy）
skillshare install anthropics/skills -s pdf,commit

# glob パターンに一致する Skill をインストール
skillshare install anthropics/skills -s "core-*"

# 検出されたすべての Skill をインストール
skillshare install anthropics/skills --all

# 自動承認（複数 Skill のリポジトリでは --all と同じ）
skillshare install anthropics/skills -y

# 他のフラグと組み合わせる
skillshare install anthropics/skills -s pdf --dry-run
skillshare install anthropics/skills --all -p
```

glob マッチングは大文字小文字を区別しません: `"Core-*"` は `core-auth`、`CORE-DB` などにマッチします。

:::tip シェルの glob 展開への対策
シェルがカレントディレクトリのファイル名に `*` を展開してしまわないよう、glob パターンは常にクォート（`"core-*"`）してください。
:::

CI/CD パイプラインやスクリプト化されたワークフローに便利です。

## 直接インストール（特定のパス）

フルパスを指定して即座にインストールします。

```bash
# サブディレクトリ付き GitHub
skillshare install anthropics/skills/skills/pdf
skillshare install google-gemini/gemini-cli/packages/core/src/skills/builtin/skill-creator

# ファジーなサブディレクトリ — 完全一致するパスが存在しない場合、Skill 名でマッチ
skillshare install runkids/my-skills/vue-best-practices

# フル URL
skillshare install github.com/user/repo/path/to/skill

# SSH URL
skillshare install git@github.com:user/repo.git

# サブディレクトリ付き SSH URL（// セパレータを使用）
skillshare install git@github.com:user/repo.git//path/to/skill

# ローカルパス
skillshare install ~/Downloads/my-skill
skillshare install /absolute/path/to/skill
```

ローカルパスはソースディレクトリにコピーされます。Global mode では `./my-skill` のような相対パスは絶対パスとして記録されるため、`update` と [`check`](./check.md#local-path-installs) はどのディレクトリからでもそのパスを見つけられます。Project mode では `.skillshare/config.yaml` が複数のマシンで共有されるため、入力どおりに記録されます。

:::tip ファジーなサブディレクトリ解決
`owner/repo/skill-name` のようなサブディレクトリパスを指定した際、そのパスがリポジトリ内に完全一致で存在しない場合、skillshare はすべての `SKILL.md` ファイルをスキャンし、ディレクトリのベース名でマッチさせます。同名の Skill が複数存在する場合は、フルパス付きの曖昧性エラーが表示されるので、正確なパスを指定できます。
:::

## Config からのインストール（引数なし） {#install-from-config-no-arguments}

ソースの引数を指定せずに実行すると、`skillshare install` は記録済みのリモート Skill メタデータ（グローバルモード）またはプロジェクトの `skills:` マニフェスト（プロジェクトモード）を読み込み、まだローカルに存在しないすべてのリモート Skill をインストールします。

```bash
# グローバル — ~/.config/skillshare/config.yaml を読む
skillshare install

# プロジェクト — .skillshare/config.yaml を読む
skillshare install -p
```

これにより、記録済みのメタデータ／マニフェストは **持ち運び可能な Skill セットアップ** になります。どのマシンでも同じ Skill 構成を再現するために共有できます。

```bash
# 新しいマシンのセットアップ
skillshare install       # メタデータからリモート Skill とトラック対象リポジトリを復元
skillshare sync          # ターゲットへ sync

# 新しいチームメンバーのオンボーディング
git clone github.com/team/project && cd project
skillshare install -p    # プロジェクト config からすべてのリモート Skill をインストール
skillshare sync
```

`tracked: true` の Skill は（`--track` と同様に）完全な git 履歴付きでクローンされるため、`skillshare update` が正しく動作します。すでにディスクに存在する Skill はスキップされます。これは、トラック対象リポジトリのディレクトリが gitignore されていてローカルに存在しない状態で、クローン直後に実行する復旧コマンドです。

:::tip push/pull と install from config の違い
`push`/`pull` は実際の Skill **ファイル** を git 経由で sync します。`install`（config から）は **ソース URL** から再ダウンロードします。両者は補完関係にあります — どちらを使うべきかは [Cross-Machine Sync](/docs/how-to/sharing/cross-machine-sync#alternative-install-from-config) を参照してください。
:::

引数なしの install では、`--name`、`--into`、`--track`、`--skill`、`--exclude`、`--all`、`--yes`、`--update` はサポートされません（これらはソース引数が必要です）。`--dry-run`、`--force`、`--skip-audit`、および閾値の上書き（`--audit-threshold` / `--threshold` / `-T`）は通常どおり動作します。

## プロジェクトモード

プロジェクトの `.skillshare/skills/` ディレクトリに Skill をインストールします。

```bash
# プロジェクトに Skill をインストール
skillshare install anthropics/skills/skills/pdf -p

# プロジェクト内のサブディレクトリへインストール
skillshare install anthropics/skills -s pdf --into tools -p
# → .skillshare/skills/tools/pdf/

# config からすべてのリモート Skill をインストール（新しいチームメンバー向け）
skillshare install -p
```

:::caution プロジェクトルートをそれ自体にインストールしない
プロジェクトモードでは、プロジェクトルートに解決されるローカルパス（例: `skillshare install ./ -p`）のインストールは拒否されます。ルートをその配下の `.skillshare/skills/` サブツリーにコピーすると、コピー先へ再帰してしまうためです。特定の Skill サブディレクトリを指定してください。

```bash
skillshare install ./my-skill -p
```

このガードは CLI と Web UI（[`skillshare ui`](./ui.md)）の両方に適用されます。
:::

### 何が違うか

| | Global | Project (`-p`) |
|---|---|---|
| 保存先 | `~/.config/skillshare/skills/` | `.skillshare/skills/` |
| `--track` | サポート | サポート |
| Config の更新 | `config.yaml` の `skills:` を自動調整 | `.skillshare/config.yaml` の `skills:` を自動調整 |
| 引数なし install | config に記載された全 Skill をインストール | config に記載された全 Skill をインストール |

別の source やブランチから同じ Skill を再インストールすると（アンインストール後、または `--force` で既存のコピーを上書き）、`.skillshare/config.yaml` のエントリと `skills.lock.json` の pin が置き換えられ、チームメンバーは新しい source からインストールします。それ以外のエントリが、このマシンのインストール記録から書き換えられることはありません。

**プロジェクトモードでのトラック対象リポジトリ** はグローバルと同様に動作します — リポジトリは `.git` を保持したままクローンされ、`.skillshare/.gitignore`（デフォルトで `.skillshare/logs/`、`.skillshare/trash/`、`.skillshare/backups/`、`.metadata.json` ファイルも無視）に追加されます。`tracked: true` フラグは `.skillshare/config.yaml` に自動的に記録されます。

```bash
skillshare install github.com/team/skills --track -p
skillshare sync
```

完全なガイドは [Project Setup](/docs/how-to/sharing/project-setup) を参照してください。

## オプション

| フラグ | 短縮形 | 説明 |
|------|-------|-------------|
| `--name <name>` | | インストールされる Skill がちょうど 1 つの場合、インストール名を上書き |
| `--into <dir>` | | サブディレクトリへインストール（例: `--into frontend` または `--into frontend/react`） |
| `--force` | `-f` | 既存の Skill を上書き。監査の検出結果によるブロック（スキャン失敗は除く）とクロスパス重複チェックを無視 |
| `--update` | `-u` | 存在する場合は更新（git pull または再インストール） |
| `--branch <ref>` | `-b` | インストール元の git ブランチ、タグ、または commit SHA（デフォルト: リモートのデフォルトブランチ） |
| `--track` | `-t` | トラック対象リポジトリとして `.git` を保持。`--all`、`--yes`、`--skill`、`--agent` との併用は拒否されます。 |
| `--kind <skill\|agent\|extra>` | | インストールを 1 種類のリソースに限定 |
| `--agent <names>` | `-a` | リポジトリから特定の agent を選択（カンマ区切り） |
| `--skill` | `-s` | 複数 Skill リポジトリから特定の Skill を選択（カンマ区切り。`core-*` のような glob パターンをサポート）。`--all`、`--yes` との併用は拒否されます。 |
| `--exclude` | | インストール時に特定の Skill をスキップ（カンマ区切り。`test-*` のような glob パターンをサポート） |
| `--all` | | プロンプトなしで検出されたすべての Skill をインストール |
| `--yes` | `-y` | すべてのプロンプトを自動承認（CI/CD 向け） |
| `--skip-audit` | | このインストールでセキュリティ監査をスキップ |
| `--audit-threshold <t>`, `--threshold <t>` | `-T` | このコマンドの監査ブロック閾値を上書き（`critical|high|medium|low|info`。省略形: `c|h|m|l|i`、加えて `crit`、`med`） |
| `--audit-verbose` | | Skill ごとの完全な監査結果を表示（デフォルト: コンパクトな要約） |
| `--project` | `-p` | プロジェクトの `.skillshare/skills/` へインストール |
| `--global` | `-g` | グローバルの `~/.config/skillshare/skills/` へインストール |
| `--dry-run` | `-n` | プレビューのみ |
| `--json` | | JSON として出力（`--force` と同様に既存の skill を上書きできるが、監査によるブロックは引き続き適用。`--skill`/`--agent` フィルタが指定されない場合、非インタラクティブ選択も暗黙的に有効化） |

## JSON 出力

```bash
skillshare install anthropics/skills --json
```

明示的に指定した非追跡の skill ソースでは、`--dry-run --json` を追加すると、インストールせずに `{name, path}` の `planned` 配列を取得できます。名前は CLI のインストール名です。Git のパスはリポジトリのルートからの相対パス（選択したサブディレクトリを含む）で、ローカル skill は `.` を使用します。検出、`.skillignore`、選択のルールが適用され、JSON モードではフィルターがなければ全選択になります。`skills` は引き続きインストール済みの名前のみを報告し、dry-run では空です。検出後に選択された skill がなければ `planned: []` を返します。追跡および設定ファイルからのインストールでは、このフィールドは提供されません。

```json
{
  "source": "anthropics/skills",
  "tracked": false,
  "dry_run": false,
  "skills": ["pdf", "commit", "review"],
  "failed": [],
  "duration": "2.345s"
}
```

`--into` を使用した場合、`into` フィールドが含まれます。

```bash
skillshare install anthropics/skills --json --into frontend
```

```json
{
  "source": "anthropics/skills",
  "tracked": false,
  "dry_run": false,
  "into": "frontend",
  "skills": ["pdf", "commit"],
  "failed": [],
  "duration": "1.890s"
}
```

agent のみのインストールでも、JSON 出力はインストールされた名前の報告に `skills` 配列を使用します。

```bash
skillshare install github.com/user/agents --kind agent --json
```

```json
{
  "source": "github.com/user/agents",
  "tracked": false,
  "dry_run": false,
  "skills": ["reviewer", "tutor"],
  "failed": [],
  "duration": "1.234s"
}
```

tracked リポジトリ、単一の skill、またはソースや config のすべての skill がセキュリティ監査にブロックされた場合など、インストール全体が失敗すると、コマンドは 0 以外で終了し、`error` フィールドに失敗した各 skill と理由が入ります。一部の skill だけが失敗した場合は `failed` に記録され、コマンドは 0 で終了します。監査の検出結果があってもインストールするには `--force` を付けます。

```json
{
  "source": "github.com/user/evil-skills",
  "tracked": true,
  "dry_run": false,
  "skills": [],
  "failed": [],
  "duration": "0.412s",
  "error": "blocked by security audit (use --force to override)"
}
```

## 重複検出

skillshare は、すでに存在するものをインストールしようとしていることを自動的に検出します。

### 同一リポジトリの再インストール

Skill がすでに存在し、**同一リポジトリ** からインストールされていた場合、skillshare は失敗させる代わりに警告付きでスキップします。

```bash
skillshare install anthropics/skills/skills/pdf
# ✓ Installed pdf

skillshare install anthropics/skills/skills/pdf
# ⊘ pdf — already installed from same repo
```

更新するには `--update`、上書きするには `--force` を使用します。

[たどられた source リンク](../targets/configuration.md#follow_source_links)の背後にある tracked repo への `--update` は、checkout に未コミットの変更がある場合 pull を拒否します。先に commit または stash してください。

### クロスパスの重複

リポジトリがすでに 1 箇所にインストールされていて、**別の** 場所にインストールしようとすると、skillshare は操作をブロックします。

```bash
# 最初のインストール（サブディレクトリへ）
skillshare install runkids/feature-radar --into feature-radar

# 後で、最初のインストールを忘れて…
skillshare install runkids/feature-radar
# ✗ this repo is already installed at skills/feature-radar/scan (and 2 more)
#   Use 'skillshare update' to refresh, or reinstall with --force to allow duplicates
```

これにより、異なるパスへの意図しない重複を防ぎます。意図的に許可する場合は `--force` を使用します。

### 別リポジトリとの競合

インストール先のディレクトリが存在するものの、**別の** リポジトリからインストールされていた場合、エラーメッセージには元のソースが含まれます。

```bash
skillshare install owner/repo-b --name my-skill
# ✗ my-skill already exists (installed from https://github.com/owner/repo-a.git).
#   To overwrite: skillshare install owner/repo-b --name my-skill --force
```

`--force` のヒントには、（該当する場合）`--into` を含む正しいフラグが常に表示されます。

## よくある使用例

**デフォルトのインストール名:** 通常は Skill のディレクトリ名を使います。ディレクトリ名が汎用的な `skill` または `skills` の場合は、`SKILL.md` frontmatter の有効な `name` を使い、小文字化して空白をハイフンに変換します（例: `MinerU Document Extractor` → `mineru-document-extractor`）。名前がない、無効、または他の Skill 名と競合する場合はディレクトリ名を使います。`--name` はこの既定値を上書きします。更新では既存のインストール名を保持し、`--track` のリポジトリ命名は変わりません。

**カスタム名でインストール:**
```bash
skillshare install google-gemini/gemini-cli/.../skill-creator --name my-creator
# Installed as: ~/.config/skillshare/skills/my-creator/
```

Skill install の `--name` は単一 Skill に解決される場合だけ有効です。remote extra では管理名を指定します。
`--track` モードでは、カスタム名はトラック対象リポジトリのディレクトリ名として保存され（自動的に `_` がプレフィックスされます）、パスセパレータや `..` を含んではいけません。
`--track` は、パスが git リポジトリのルートであればローカルパスも受け付けます（`file:///path` と同様に clone されます）。それ以外のローカルフォルダは `--track` なしでインストールしてください。`--track --force` では、新しい clone が成功した後にのみ既存のトラッキングリポジトリを置き換えるため、再インストールに失敗しても既存のリポジトリは残ります。そのトラッキングリポジトリ自体、またはその中にあるローカルソース（パスまたは `file://` URL）は拒否されます。

```bash
# ✅ 単一 Skill（動作する）
skillshare install comeonzhj/Auto-Redbook-Skills --name haha

# ❌ 複数の Skill が検出される（エラーになる）
skillshare install anthropics/skills --name my-skill
```

**既存のものを強制上書き:**
```bash
skillshare install ~/my-skill --force
```

**既存 Skill の更新:**
```bash
# Skill 名で（保存済みのソースを使用）
skillshare install pdf --update

# ソース URL で
skillshare install anthropics/skills/skills/pdf --update
```

**サブディレクトリへインストール:**
```bash
# カテゴリ別に整理
skillshare install ~/my-skill --into frontend
# → ~/.config/skillshare/skills/frontend/my-skill/

# 複数階層のネスト
skillshare install anthropics/skills -s pdf --into frontend/react
# → ~/.config/skillshare/skills/frontend/react/pdf/

# sync 後、ターゲットにはフラット名で表示される: frontend__my-skill, frontend__react__pdf
```

フォルダ運用の戦略については [Organizing Skills](/docs/how-to/daily-tasks/organizing-skills) を参照してください。

**特定のブランチからインストール:**
```bash
# ブランチから通常インストール
skillshare install github.com/team/skills --branch develop --all

# 特定のブランチをトラック
skillshare install github.com/team/skills --track --branch frontend

# 同一リポジトリ、異なるブランチ（衝突を避けるため --name を使用）
skillshare install github.com/team/skills --track --branch frontend --name team-frontend
skillshare install github.com/team/skills --track --branch backend --name team-backend
```

**タグまたは commit SHA に固定（再現可能なインストール）:**
```bash
# リリースタグに固定
skillshare install github.com/team/skills --branch v1.2.0 --all

# 特定のコミットに固定（完全または短縮 SHA）
skillshare install github.com/team/skills --branch 8f14e45fceea167a5a36dedd4bea2543ce848564 --all
```

Web URL でも同じように固定できます。`skillshare install github.com/team/skills/tree/v1.2.0/skills/foo` はタグ `v1.2.0` からインストールします。GitLab（`-/tree/<ref>/`）と Bitbucket（`src/<ref>/`）の URL も使えます。`tree/feature/x/skills/foo` のように `/` を含むブランチ名は、リモートのブランチとタグに照合されます。`main` へのリネーム後の `tree/master/` のように、リモートにもう存在しない ref はデフォルトブランチにフォールバックせず、インストールが失敗します。GitHub がデフォルトブランチに使う `tree/HEAD/` リンクは、リモートのデフォルトブランチからインストールします。明示的な `--branch` は URL 内の ref より優先されます。

プロジェクトでは通常これは不要です。`.skillshare/skills.lock.json` がすでにすべてのリモート skill をインストール時のコミットに固定しており、`skillshare update` がその固定を移動させるためです。[ロックファイル](/docs/understand/project-skills#lockfile)を参照してください。

固定した ref は Skill のメタデータに保存されるため、`skillshare update` は同じリビジョンを再インストールし、`skillshare check` は SHA 固定をリモートに接続せずに最新として報告します。`--track` にはブランチが必要です。タグや commit SHA ではクローンが detached 状態になり、`skillshare update` が pull するものがないため、インストールは拒否されます。

**チームリポジトリのインストール（トラック対象）:**
```bash
skillshare install addyosmani/web-quality-skills --track --name team-skills
```

```text
$ skillshare install addyosmani/web-quality-skills --track --name team-skills
✓ Source    github.com/addyosmani/web-quality-skills
  Found     6 skills
  Skills    accessibility, best-practices, core-web-vitals, performance, seo, web-quality-audit
! Audit     63 findings: HIGH=1, MEDIUM=1, LOW=60, INFO=1 — findings detected, but none at/above block threshold (CRITICAL) · risk CRITICAL (100/100)
✓ Install   _team-skills → ~/.config/skillshare/skills/_team-skills · 1.9s

Next
  skillshare sync                 link them into your targets
  skillshare update _team-skills  update this repo later
```

## プライベートリポジトリ {#private-repositories}

### SSH（推奨）

SSH は最もシンプルな方法です — SSH キーが設定済みであれば、そのまま動作します。

```bash
skillshare install git@github.com:org/private-skills.git --track
skillshare install git@gitlab.com:org/skills.git --track
skillshare install git@bitbucket.org:team/skills.git --track
skillshare install git@ssh.dev.azure.com:v3/org/project/skills --track

# サブディレクトリ付き
skillshare install git@github.com:org/skills.git//frontend-react
```

### トークン付き HTTPS

適切な環境変数を設定し、通常の HTTPS URL を使用します。skillshare はトークンを自動検出し、クローン時に注入します。

```bash
export GITHUB_TOKEN=ghp_your_token
skillshare install https://github.com/org/private-skills.git --track
```

| プラットフォーム | 環境変数 | トークンの種類 |
|----------|---------|------------|
| `github.com` | `GITHUB_TOKEN` | Personal access token（`repo` スコープ） |
| `gitlab.com` | `GITLAB_TOKEN` | Personal access token または CI job token |
| Bitbucket | `BITBUCKET_TOKEN` | Repository token、または app password（`BITBUCKET_USERNAME` と併用） |
| Azure DevOps | `AZURE_DEVOPS_TOKEN` | Personal Access Token（Code: Read スコープ） |
| 任意のホスト | `SKILLSHARE_GIT_TOKEN` | 汎用フォールバック |

プロバイダーのトークンは HTTPS の既定ポート（または 443）でのみ送信されます。対象は `github.com`、`gitlab.com`、`bitbucket.org`、`dev.azure.com`、`*.visualstudio.com`、`gitea.com`、`cnb.cool` です。セルフホストの Gitea/CNB は、名前にプロバイダー名を含む場合も `gitea_hosts`/`cnb_hosts` に正確なホスト名の登録が必要です。セルフホストの GitHub/GitLab などにはホスト別の Git credential helper を使うか、対象の実行時だけ `SKILLSHARE_GIT_TOKEN` を設定してください。この汎用フォールバックは**任意の HTTPS ソース**に送信されます。名前による判定だけではプロバイダートークンの送信は許可されません。

プラットフォーム固有の環境変数は `SKILLSHARE_GIT_TOKEN` より優先されます。

token はホスト単位の credential helper を通じて git に渡されるため、`ps` の出力などのプロセス引数には現れません。token がホストに適用されている間、git はそのホストについて自分の credential helper やキーチェーンに問い合わせず、何も保存しません。

公式のトークンドキュメント:
- GitHub: [Managing your personal access tokens](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)
- GitLab: [Token overview](https://docs.gitlab.com/security/tokens/)
- Bitbucket: [Access tokens](https://support.atlassian.com/bitbucket-cloud/docs/access-tokens/)
- Azure DevOps: [Use Personal Access Tokens](https://learn.microsoft.com/en-us/azure/devops/organizations/accounts/use-personal-access-tokens-to-authenticate?view=azure-devops)

Bitbucket の app password を使う場合は、ユーザー名も設定してください。

```bash
export BITBUCKET_USERNAME=your_bitbucket_username
export BITBUCKET_TOKEN=your_app_password
skillshare install https://bitbucket.org/team/skills.git --track
```

### CI/CD の例

**GitHub Actions:**

```yaml
- name: Install shared skills
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
  run: skillshare install https://github.com/org/skills.git --track
```

**GitLab CI:**

```yaml
install-skills:
  script:
    - skillshare install https://gitlab.com/org/skills.git --track
  variables:
    GITLAB_TOKEN: $CI_JOB_TOKEN
```

**Bitbucket Pipelines:**

```yaml
- step:
    name: Install shared skills
    script:
      - skillshare install https://bitbucket.org/team/skills.git --track
    env:
      BITBUCKET_USERNAME: $BITBUCKET_USERNAME   # for app passwords
      BITBUCKET_TOKEN: $BITBUCKET_TOKEN
```

**Azure Pipelines:**

```yaml
- script: skillshare install https://dev.azure.com/org/project/_git/skills --track
  env:
    AZURE_DEVOPS_TOKEN: $(System.AccessToken)
```

## セキュリティスキャン

すべての Skill は、インストール時に自動的にセキュリティ脅威をスキャンされます。

- `audit.block_threshold` 以上の検出結果は **インストールをブロック** します（デフォルト: `CRITICAL`）
- スキャンを実行できない場合（例えば `audit-rules.yaml` に無効な正規表現がある）、インストールはブロックされ削除されます。`--force` では上書きできません。原因を修正するか `--skip-audit` を使用してください
- それより低い検出結果は警告として表示され、リスクスコアのコンテキストも含まれます
- `audit.block_threshold` はブロックのレベルのみを制御し、スキャン自体を無効化するものでは **ありません**
- 常時 audit をスキップする config スイッチはありません。必要な場合はコマンドごとに `--skip-audit` を使用してください
- `--audit-threshold`、`--threshold`、`-T` でコマンドごとに閾値を上書きできます

閾値の config 例:

```yaml
audit:
  block_threshold: HIGH
```

```bash
# ブロックされる — 重大な脅威を検出
skillshare install evil-skill
# → Installation blocked at active threshold. Use --force to override.

# 警告を無視して強制インストール
skillshare install suspicious-skill --force

# スキャンを完全にスキップ（注意して使用）
skillshare install suspicious-skill --skip-audit

# コマンドごとの閾値上書き（同じ意味）
skillshare install suspicious-skill --audit-threshold high
skillshare install suspicious-skill --threshold high
skillshare install suspicious-skill -T h
```

検出結果によるブロック判定を上書きするには `--force`、スキャンを完全にバイパスするには `--skip-audit` を使用します。スキャンの詳細は [audit](/docs/reference/commands/audit) を参照してください。

インストールの判定は **検出結果の重大度 vs 閾値** で行われます。リスクスコア／ラベルは参考情報として報告されるだけで、それ単体でインストールをブロックすることはありません。デフォルトでは、監査結果は（重大度とメッセージでグルーピングされた）コンパクトな要約として表示されます。完全なリストを見るには `--audit-verbose` を使用してください。

### トラック対象リポジトリの監査ゲート（`--track`）

トラック対象リポジトリは同じ閾値モデルを使用しますが、スキャン範囲と失敗時の処理はより厳格です。

- 新規の `--track` インストールでは、（1 つの Skill フォルダだけでなく）**クローンされたリポジトリ全体** がスキャンされます
- 閾値以上の検出結果があると、`--force` を使わない限りインストールをブロックします
- スキャンを実行できない場合も、`--force` を使ってもインストールをブロックします
- 新規インストールがブロックされた場合、skillshare はクローンしたリポジトリをソースから自動的に削除します
- 自動クリーンアップが失敗した場合、install は明示的なエラーを返し、パスを手動で削除するよう案内します

install 経由の tracked repo 更新（`skillshare install <repo> --track --update`）も、同じ隔離候補の監査フローを使用します。

- 一時 checkout で pull と監査を実行し、既存の source ディレクトリを変更しません
- 拒否またはスキャンエラーの場合、既存の checkout と metadata は変更されません
- 承認された正確な commit を公開し、上流から再度 pull しません
- `--force` は検出結果を上書きできますが、スキャンエラーは上書きできません

### `--force` と `--skip-audit` の違い

どちらもインストールのブロックを解除できますが、動作は異なります。

| フラグ | 監査の実行 | 何が起きるか |
|------|------------------|-------------|
| `--force` | 監査は実行される | 検出結果は生成／記録され続ける。閾値に達してもインストールは継続 |
| `--skip-audit` | 監査はスキップされる | このインストールではスキャンが実行されない |

推奨される使い分け:

- 検出結果を確認したい場合は `--force` を優先してください
- スキャンを意図的にバイパスしたい場合のみ `--skip-audit` を使用してください
- 両方が指定された場合、実際には `--skip-audit` が優先されます（スキャンはスキップされます）

## Skill の除外 {#excluding-skills}

### `--exclude` フラグ

複数 Skill を含むリポジトリからインストールする際、特定の Skill をスキップします。完全一致の名前と **glob パターン** の両方をサポートしています。

```bash
# 特定の Skill 以外をすべてインストール
skillshare install anthropics/skills --all --exclude cli-sentry,delayed-command

# glob パターンで除外
skillshare install anthropics/skills --all --exclude "test-*"

# -y でも動作する
skillshare install org/skills -y --exclude internal-tool

# --skill と組み合わせてきめ細かく制御
skillshare install org/skills -s pdf,commit,docs --exclude docs
```

Skill が除外されると、何がスキップされたかを示すメッセージが表示されます: `Excluded 2 skills: cli-sentry, delayed-command`。

:::note 複数 Skill のディスカバリーが必要
`--exclude` は、複数の Skill を含む **git リポジトリ** からインストールする場合にのみ機能します。`--all`、`--yes`、`--skill`、およびインタラクティブ選択モードで動作します。直接インストール（ローカルパスや単一 Skill の git URL）には `--exclude` は適用されません — 指定した場合は警告が表示されます。
:::

### .skillignore {#skillignore}

リポジトリの管理者は、リポジトリルートに `.skillignore` ファイルを作成することで、ディスカバリーから Skill を隠せます。リポジトリからインストールするユーザーには、これらの Skill が選択プロンプトに表示されることはありません。

```text title=".skillignore"
# Internal tooling — not for public use
validation-scripts
scaffold-template

# Exclude all test/eval skills
prompt-eval-*

# Exclude an entire group directory
internal-tools
```

**実例** — [`runkids/my-skills`](https://github.com/runkids/my-skills) は `.skillignore` を使って、Skill ではないディレクトリや内部ツールを除外しています。

```text title=".skillignore"
skillshare
feature-radar
```

`--exclude` と組み合わせることで、ユーザーはさらに選択を絞り込めます。

```bash
skillshare install runkids/my-skills --exclude seo
```

**形式** — [gitignore の構文](https://git-scm.com/docs/gitignore) を使用します。

| パターン | 例 | 動作 |
|---------|---------|----------|
| 完全一致の名前 | `validation-scripts` | そのパスの Skill にマッチ |
| グループ一致 | `feature-radar` | `feature-radar/` 配下の **すべての** Skill にマッチ |
| 正確なパス | `feature-radar/feature-radar` | その特定の Skill のみ |
| `*` ワイルドカード | `prompt-eval-*` | 1 セグメントにマッチ（`/` は越えない） |
| `**` | `**/temp` | 任意のディレクトリ深さでマッチ |
| `?` | `?.md` | 1 文字にマッチ |
| `[abc]` | `[Tt]est` | 文字クラス |
| `!pattern` | `!important` | 否定 — 一度マッチした Skill を再度含める |
| `/pattern` | `/root-only` | `.skillignore` の位置にアンカー |
| `pattern/` | `build/` | ディレクトリのみのマッチ |
| `\#`, `\!` | `\#file` | エスケープされたリテラル文字 |

`#` で始まる行はコメントです。空行は無視されます。

**推奨される使用シーン:**
- 複数 Skill を含むリポジトリを公開する際に、内部ツールや作業中の Skill を隠す
- グループ化された Skill ディレクトリを持つモノレポで、グループ全体を除外する（例: `internal-tools`）
- インストールする全員が特定の Skill を決して発見できないよう、管理者レベルの可視性ルールを強制する

**適さないケース:**
- 直接のローカルパスインストール（ディスカバリーをスキップするため）
- 単一 Skill の直接インストール（`--exclude` と同様に、直接インストールパスでは無視されます）

`.skillignore` は git リポジトリのディスカバリー時に適用されるため、ディスカバリーベースのすべてのインストールパス（`--all`、`--skill`、`--yes`、インタラクティブ選択）に影響します。直接のローカルパスインストール（ディスカバリーを完全にスキップする）には **適用されません**。

:::tip .skillignore の適用範囲
**リポジトリレベル** の `.skillignore`（リポジトリルート内）は、あなたのリポジトリからインストールするユーザーに対して、どの Skill を発見可能にするかを制御します。インストール後、トラック対象リポジトリは `.skillignore` を保持し続け、`doctor`、`status`、`list`、`sync`、`audit`、`diff`、`check` からも尊重されます。

**ソースルート** の `.skillignore`（`~/.config/skillshare/skills/.skillignore`）は、トラック対象・非トラック対象を問わず、すべての Skill にグローバルに適用されます。アンインストールせずに Skill を一時的にミュートしたり、パターン（例: `draft-*`）を除外したりする際に使用します。
:::

### `.skillignore` と `--exclude` の違い

| | `.skillignore` | `--exclude` |
|---|---|---|
| **誰が制御するか** | リポジトリの管理者 | インストールするユーザー |
| **どこに置くか** | リポジトリルートの `.skillignore` | CLI フラグ |
| **いつ適用されるか** | ディスカバリー時（選択の前） | ディスカバリー後（プロンプトの前） |
| **適用範囲** | このリポジトリからインストールするすべてのユーザー | このインストールのみ |
| **要件** | 複数 Skill を含む git リポジトリ | 複数 Skill を含む git リポジトリ |

## Agent サポート

リポジトリをインストールする際、skillshare は Skill と並んで agent（単体の `.md` ファイル）を自動検出します。

- リポジトリに `agents/` ディレクトリがある場合、その中の `.md` ファイルは agent 候補として検出されます
- リポジトリに `skills/` と `agents/` の両方がある場合、両方がインストールされます
- リポジトリにルート直下の `.md` ファイルのみがあり（`SKILL.md` はない）場合、それらは agent として扱われます

### 明示的な agent フラグ

```bash
# リポジトリから agent のみをインストール
skillshare install github.com/user/repo --kind agent

# 名前で特定の agent をインストール（-a は短縮形）
skillshare install github.com/user/repo -a tutor,reviewer

# プロジェクトモードと組み合わせる
skillshare install github.com/user/repo --kind agent -p
```

`-a <name>` フラグは、Skill の `-s <name>` に相当する agent 用のフラグです。Agent は `~/.config/skillshare/agents/`（グローバル）または `.skillshare/agents/`（プロジェクト）にインストールされます。概念の全体像は [Agents](/docs/understand/agents) を参照してください。

### 混在リポジトリでの Skill と agent のスコープ指定

Skill と agent の両方を含むリポジトリでは、フィルタによってインストール対象が正確に制御されます。

| フラグ | インストールされるもの |
|-------|---------------------|
| _（なし）_ | すべての Skill とすべての agent |
| `--all` / `--yes` | すべての Skill とすべての agent |
| `-s <names>` | 指定した Skill のみ — **agent はインストールされない** |
| `-s <names> -a <names>` | 指定した Skill と指定した agent |
| `-a <names>` | 指定した agent のみ |

```bash
# 混在リポジトリから 1 つの Skill だけをインストール — agent は取り込まれない
skillshare install github.com/user/repo -s pdf

# Skill と agent を 1 つずつ一緒にインストール
skillshare install github.com/user/repo -s pdf -a tutor
```

未知の `-a` 名を指定すると、どの Skill もインストールされる前にコマンド全体が失敗します。そのため、自動化パイプラインが中途半端な状態のインストールを目にすることはありません。

## インストール後

必ずターゲットに配布するために sync してください。

```bash
skillshare install anthropics/skills/skills/pdf
skillshare sync  # ← 忘れずに！
```

## 関連項目

- [list](/docs/reference/commands/list) — インストール済み Skill を表示
- [update](/docs/reference/commands/update) — Skill やトラック対象リポジトリを更新
- [upgrade](/docs/reference/commands/upgrade) — CLI と組み込み Skill をアップグレード
- [uninstall](/docs/reference/commands/uninstall) — Skill を削除
- [sync](/docs/reference/commands/sync) — Skill をターゲットへ sync
- [Organization-Wide Skills](/docs/how-to/sharing/organization-sharing) — トラック対象リポジトリによる組織共有


## リモートソースから extra をインストールする {#remote-extras}

`SKILL.md` がなくても、リポジトリ内の正確なディレクトリを取得できます。

```bash
skillshare install example-org/ai-hub/rules --kind extra --name team-rules -p
skillshare extras team-rules --add-target .claude/rules -p
skillshare sync extras team-rules -p
```

Global では `-g` を使います。取得は Git のみで、ホスティング API への
フォールバックや曖昧なディレクトリ検索は行いません。リポジトリ全体も指定でき、
`.git` は除外し、submodule 内のパスは拒否します。ローカルフォルダーには
`skillshare extras init team-rules --source <path> --target <target>` を使います。
`--kind extra` は `--track`、`--into`、`--skill`、`--agent`、`--exclude`、
`--all`、`--yes` と併用できません。提案時の `--as` ではなく `--name` を使います。

取得先は `<extras root>/team-rules/` です。config には `remote`、任意の `branch`、
初期状態では `targets: []` を記録します。`--branch <ref>` はブランチ、タグ、commit
を選び、明示フラグは URL の ref より優先されます。Project の **version 1** lockfile
の `extras` map は 40 桁の commit を保持し、Global では同一 origin の metadata pin
を使います。pin は origin と ref が一致する場合だけ使い、`--branch` の省略は設定済み
ref を維持します。ref を変更するとその先頭を取得し、`--update` は同じ ref の pin を更新します。到達不能な pin は
エラーとなり、ブランチへ切り替えません。origin 不一致の metadata は pin ではなく、
既存ディレクトリの origin が異なる場合は削除してから再インストールします。

Project config と `skills.lock.json` を commit してください。この lockfile を書く
**全員**が `extras` map を保持できるバイナリへ更新する必要があります。Project 内の
取得ディレクトリ、staging、metadata は ignore されます。別マシンの復元には明示的な
コマンドを使い、同じ ref の pin を進める場合だけ `--update` を指定します。

```bash
skillshare install example-org/ai-hub/rules --kind extra --name team-rules -p
skillshare install example-org/ai-hub/rules --kind extra --name team-rules --update -p
```

引数なしの `skillshare install` はリモート extras を復元しません。`sync extras`
はローカルコピーを配布し、取得しません。コピーがない場合は明示的な復元コマンドを
表示します。内容の編集を破棄するには明示的な `--force` が必要で、`--json` は同意
になりません。モードだけの変更は内容編集に数えません。ローカル extra と同名、または
別 origin のエントリは `--force` でも上書きできません。

公開前に候補を staging し監査します。しきい値以上の検出はブロックし、`--force` は
検出を承認できますが scanner/rules エラーは回避できません。`--skip-audit` は明示的な
省略として報告します。既存 scanner は深さ 6、1,000,000 bytes 以下、拡張子 filter
の範囲を調べ、`.mdc` と `.cursorrules` は対象外です。監査の通過は安全の保証では
ありません。内部ファイルリンクは実体化し、絶対・範囲外・dangling・ディレクトリの
リンクや特殊ファイルは拒否します。Unix 実行 bit は保持し、Windows ではこの保証は
ありません。URL に認証情報を埋め込まず Git 認証や SSH を使ってください。

`.cursor/rules/policy.md` など隠しディレクトリ内のファイルはヘッダーだけを検査し、
通常のテキスト・整合性分析は省略します。`.git` と `.skillshare` ディレクトリは
対象外です。scanner がファイルを省略すると、成功した install に検査範囲の警告を
表示します。scanner の検査範囲自体は広げません。

ブランチが削除されても、Git が同じ commit を取得できる限り、一致する記録済み pin
で復元できます。新規宣言、明示的な ref 変更、`--update` には要求した ref の解決が必要です。

ロック取得時に、取得・監査より前に空の管理用 extras root と永続的な
`.skillshare-extras.lock` を作成する場合があります。監査拒否では管理項目、config、pin、
metadata、ignore 規則を新規作成しません。管理ロックは取得内容ではなく、extra 削除後も
残ります。削除や置き換えをしないでください。

設定済み項目の中断は list/status/doctor/`check extras` に表示します。宣言の保存前は、
`check extras`、status、doctor が管理対象 staging 直下の公開用ファイル名から名前だけを
報告します。marker 内の origin は信用しません。同じ `--name` の元の install を再実行してください。
live が欠落し previous が未変更なら、ネットワーク取得の前に既知の previous をローカルで
戻し、その後 journal の commit を取得・監査して前方修復します。
中断後の内容編集は明示 `--force` なしでは拒否します。取得不能や監査失敗では marker
を残すため staging パスを保持してください。`--dry-run` は移行、config、lock、metadata、
oplog を含め何も書きません。[ライフサイクルと同時実行の制限](extras.md#remote-extras)も参照してください。

最終 cleanup 前のプロセス内エラーでは、上記の同時実行制限の範囲で今回の変更を
ロールバックします。previous ディレクトリの削除に失敗した場合、新しいコピーと記録を
整合したまま journal を残し、前方修復を行います。一部削除済みの previous を修復時に
破棄するには明示的な `--force` が必要です。残存ファイルを救出する場合は保持してください。
