---
sidebar_position: 2
---

# よくあるエラー

エラーメッセージとその解決方法です。

## Config エラー

### `config not found: run 'skillshare init' first`

**原因:** Config ファイルが存在しません。

**解決策:**
```bash
skillshare init
```

カスタムパスにしたい場合は `--source` を追加してください。
```bash
skillshare init --source ~/my-skills
```

---

### `failed to load project config: ...`

**原因:** `.skillshare/config.yaml` は存在するがパースできません（不正な YAML、型の間違いなど）。
変更を伴うコマンド（`uninstall`、`new`、`enable`/`disable`、`check`）は、カスタムの `sources`
設定がある場合にデフォルトの `.skillshare/skills/` ディレクトリを誤って操作しないよう、
この状態では処理を拒否します。

**解決策:** YAML を修正してコマンドを再実行してください。よくある問題:

```yaml
# 誤り — targets はリストでなければならない
targets: {}

# 正しい
targets: []
```

```yaml
# 誤り — skills はリストでなければならない
skills: my-skill

# 正しい
skills:
  - name: my-skill
    source: github.com/org/my-skill
```

任意の YAML リンターでファイルを検証するか、`.skillshare/backups/` があれば一時的にそこから
復元してください。

---

### `target "<name>": skills target path X overlaps skills source Y`

**原因:** `sources.skills` が、ある Target の Skill パスと同じディレクトリに解決されている
（または一方がもう一方を含んでいる）。例えば `sources.skills: .claude/skills` を `claude`
Target と一緒に設定すると、両方が `.claude/skills/` を指すことになります。このガードが
なければ、`sync --force` は Source を Target ディレクトリとして扱い、その内容を削除して
しまいます。

**解決策:** どの Target ともエイリアスにならない Source パスを選んでください。よくある安全な
選択肢:

```yaml
# プロジェクトのドキュメントと同じ場所に置く
sources:
  skills: ./docs/skills

# .skillshare/ 配下に維持する（デフォルト — sources キーを完全に削除する）
```

同じチェックが、Agent の Target パスに対する `sources.agents` にも適用されます。

---

## Target エラー

### `target add: path does not exist`

**原因:** Skill ディレクトリがまだ存在しません。

**解決策:**
```bash
mkdir -p ~/.myapp/skills
skillshare target add myapp ~/.myapp/skills
```

### `target path does not end with 'skills'`

**原因:** パスが規約に従っていないという警告。

**解決策:** これはエラーではなく警告です。意図的なパスであればそのまま進めるか、修正してください。
```bash
skillshare target add myapp ~/.myapp/skills  # 推奨
```

### `target directory already exists with files`

**原因:** Target に上書きされる可能性のある既存のファイルがある。

**解決策:**
```bash
skillshare backup
skillshare sync
```

---

## Sync エラー

### `deleting a symlinked target removed source files`

**原因:** symlink モードの Target に対して `rm -rf` を実行した。

**解決策:**
```bash
# git が初期化されている場合
cd ~/.config/skillshare/skills
git checkout -- .

# またはバックアップから復元する
skillshare restore <target>
```

**予防策:** 手動で削除する代わりに `skillshare target remove` を使用してください。

### `sync` が同じ変更を表示し続ける {#sync-keeps-showing-the-same-changes}

**原因:** 2 つの Target が、`include` / `exclude` フィルター、`mode`、`target_naming` のいずれかが異なる状態で（両方の Target が `symlink` モードの場合はフォルダ全体をリンクするため、フィルターと命名は影響しません）同じフォルダに Skill を Sync している。各 Sync が一方の Target の設定どおりにフォルダを書き直し、もう一方の結果を打ち消す（フィルター対象の Skill の追加や削除、名前の変更、リンクとコピーの切り替え）ため、フォルダの状態が安定しません。`sync` は該当する Target を表示します:

```
! codex and universal sync skills to ~/.agents/skills with different settings, so each sync undoes the other
  keep one: skillshare target codex --skills=false
```

**解決策:** 一方の Target にフォルダへの書き込みを任せ、もう一方の Skill をオフにしてください。その Target の agents、MCP サーバー、instructions は引き続き管理され、ツールは共有フォルダ内の Skill を引き続き読み込みます:

```bash
skillshare target codex --skills=false --dry-run
skillshare target codex --skills=false
```

ダッシュボードの **Sync** ページにも同じ警告が表示され、その Target の Skill の Sync を停止するボタンがあります。両方の Target に同じフィルター、mode、target naming を設定する方法でも解決できます。

### `sync seems stuck or slow`

**原因:** skills ディレクトリ内に大きなファイルがある。

**解決策:** ignore パターンを追加してください。
```yaml
# ~/.config/skillshare/config.yaml
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
  - "**/node_modules/**"
```

### Sync 中の `no space left on device` / `ENOSPC`

**原因:** 何かがボリュームを圧迫している。まずバックアップディレクトリを確認し、次に Source を
確認してください。

**解決策:**
```bash
df -h ~                                     # ボリュームが満杯であることを確認する
du -sh ~/.local/share/skillshare/backups    # バックアップの使用量
du -sh ~/.config/skillshare/skills          # Source の使用量
```

バックアップが大きい場合は整理してください — 保持ポリシーは各 `sync` の後に自動的に実行されますが、
それ以前に肥大化したディレクトリはオンデマンドでクリアできます。

```bash
skillshare backup --cleanup --dry-run   # プレビュー
skillshare backup --cleanup
```

ボリュームが100%に張り付いている場合、少し空き容量ができるまで `rm` が「Permission denied」で
失敗することがあります。まず大きなファイルを1つ解放してから、整理してください。

**Source** が大きい場合、そのアーティファクトは Skill の中にあります。バックアップはそれらを
コピーしません（シンボリックリンクされた Skill はスキップされる）が、copy モードのすべての Target
はコピーします。ランタイムキャッシュ、モデルの重み、ブラウザプロファイルは Skill ツリーの外に
移動するか、`ignore:` で除外してください。

バックアップの範囲が `.gitignore` や `ignore:` とどう違うかについては
[Backups & Disk Space](/docs/reference/commands/backup#backups--disk-space) を参照してください。

---

## Git エラー

### `Could not read from remote repository`

**原因:** SSH キーが設定されていない、またはリモート URL が間違っている。

**解決策:**
```bash
# SSH アクセスを確認する
ssh -T git@github.com

# SSH が設定されていない場合は代わりに HTTPS を使う
git -C ~/.config/skillshare/skills remote set-url origin https://github.com/you/my-skills.git

# または SSH キーを設定する
ssh-keygen -t ed25519 -C "you@example.com"
# その後、公開鍵を GitHub → Settings → SSH keys に追加する
```

### `push: remote has changes`

**原因:** リモートリポジトリがローカルより先行している。

**解決策:**
```bash
skillshare pull   # まずリモートの変更を取得する
skillshare push   # これで push できる
```

### `pull: local has uncommitted changes`

**原因:** push されていないローカルの変更がある。

**解決策:**
```bash
# オプション1: 先に変更を push する
skillshare push -m "Local changes"
skillshare pull

# オプション2: ローカルの変更を破棄する
cd ~/.config/skillshare/skills
git checkout -- .
skillshare pull
```

### `pull stopped: this machine and the remote both changed ...`

**原因:** 同じファイルが2台のマシンで編集された。`pull` はマージを取り消したため、リポジトリは変更されていない。`.metadata.json` だけの競合ではこのエラーは起きない（自動で解決される）。

**解決策:**
```bash
cd ~/.config/skillshare/skills
git pull --no-rebase          # マージをやり直し、競合を残す
# 競合したファイルを編集する
git add .
git commit --no-edit
skillshare push
skillshare sync
```

### `Git had no identity`

**原因:** `skillshare init` が source repo を作成したとき、git に `user.name` / `user.email` が設定されていなかった。skillshare は commit できるよう、その repo 自身の設定にフォールバック（`skillshare@local`）を書き込みます。repo の設定は `git config --global` より優先されるため、後から global の identity を設定しても置き換わりません。

自分で作成した repo は変更されません。skillshare がフォールバックを使うのは最初の 1 回の commit だけです。

**解決策:** その repo で identity を設定します（パスはメッセージに表示されたものを使ってください。以下はデフォルト）:
```bash
git -C ~/.config/skillshare/skills config user.name "Your Name"
git -C ~/.config/skillshare/skills config user.email "you@example.com"
```

または repo の設定を削除して global の identity を使います: `git -C ~/.config/skillshare/skills config --unset user.name`（`user.email` も同様）。

### `Git root mismatch`

**原因:** `config.yaml` の `git_root` が git リポジトリのないスコープディレクトリを指しているが、
別のスコープディレクトリにはリポジトリがある。これは、リポジトリを再配置せずに `git_root` を
変更した場合に発生します — スコープの切り替えは「別のディレクトリのバージョン管理を開始する」
ことであり、「既存の履歴を移動する」ことではありません。
[`git_root`](/docs/reference/targets/configuration#git-root) を参照してください。

**解決策:** エラーが表示する3つのオプションのいずれかを選んでください。
```bash
# 設定されているスコープに新しいリポジトリを開始する（履歴なし）
skillshare init --git-root <scope>

# 既存のリポジトリを移動し、履歴を維持する
mv <old-scope>/.git <new-scope>/.git

# または既存のリポジトリを使い続ける: config.yaml の git_root を戻す
#   git_root: <scope-that-has-the-repo>
```

### `tracked repository clone is missing`

**原因:** Tracked リポジトリが `.metadata.json` に宣言されているが、clone されたディレクトリ
（例: `skills/_team-skills/`）がローカルに存在しない。これは、Tracked リポジトリのディレクトリが
意図的に管理された `.gitignore` ブロックに列挙されているため、新しいマシンで skillshare の Source
リポジトリを clone した後によく発生します。

**解決策:** メタデータから欠けている Tracked リポジトリの clone を復元してください。
```bash
skillshare install
skillshare sync
```

プロジェクトモードの場合:
```bash
skillshare install -p
skillshare sync -p
```

`status`、`check`、`update --all`、`doctor` はこの状態を報告し、`skillshare install` を
提案します。

### `nested git repositories must be disabled first`

**原因:** `git_root: root` の場合、サブディレクトリ（例: `skills/_org/` 配下の Tracked された
Skill リポジトリ）が独自の `.git` を持っている。Git はこれを **空のサブモジュール** として
アップロードし、そのファイルを黙って失ってしまうため、それぞれのネストされたリポジトリが無効化
されるまで `commit`/`push` は中止されます。

**解決策:**
```bash
# 報告された各ネストされたリポジトリを無効化する（可逆的 — 名前を戻せば再度有効化できる）
mv ~/.config/skillshare/<dir>/.git ~/.config/skillshare/<dir>/.git.disabled
```
または Web UI の Git Sync ページでワンクリック無効化を使ってください。skillshare は、
（マシン固有のパスを保持しているため）`config.yaml` を root スコープのリポジトリから自動的に
除外します。

### `Invalid git_root`

**原因:** `config.yaml` の `git_root` が認識できない値に設定されている（例: タイプミス）。

**解決策:** `skills`、`agents`、`extras`、`root` のいずれかを使うか、空のままにしてください
（デフォルトは `skills`）。

---

## Install エラー

### `skill already exists`

**原因:** 同じ名前の Skill がすでにインストールされている。

**解決策:**
```bash
# 既存の Skill を更新する
skillshare install <source> --update

# または強制的に上書きする
skillshare install <source> --force
```

### `git failed (exit 128): repository not found or authentication required`

**原因:** リポジトリの URL が間違っている、リポジトリが存在しない、または認証情報が不足している。

skillshare は現在、一般的な git の失敗に対して生の終了コードの代わりに実用的なエラーメッセージを
提供します。エラーメッセージには提案が含まれます。

```
Error: git failed (exit 128): repository not found or authentication required
```

トークンが使用されたが拒否された場合:

```
Error: git failed (exit 128): authentication token was rejected — check permissions and expiry
```

**解決策:** 下記の認証オプションを参照してください。

### `Authentication failed` / `Access denied`

**原因:** HTTPS の認証情報が不足している、期限切れである、またはトークンの種類が間違っている。

**解決策 — オプション1: トークンの環境変数を設定する:**

```bash
# GitHub
export GITHUB_TOKEN=ghp_xxxx

# GitLab（Personal Access Token でなければならない。プレフィックスは glpat-）
export GITLAB_TOKEN=glpat-xxxx

# Bitbucket
export BITBUCKET_TOKEN=your_app_password
```

**Windows（PowerShell）:**
```powershell
$env:GITLAB_TOKEN = "glpat-xxxx"

# 永続化（再起動後も残る）
[Environment]::SetEnvironmentVariable("GITLAB_TOKEN", "glpat-xxxx", "User")
```

**解決策 — オプション2: SSH URL を使う:**
```bash
skillshare install git@github.com:team/private-skills.git
skillshare install git@gitlab.com:team/skills.git
skillshare install git@bitbucket.org:team/skills.git
```

**解決策 — オプション3: git credential helper を使う:**
```bash
gh auth login          # GitHub CLI
git credential approve # またはプラットフォーム固有の credential manager
```

**必要なトークンの権限:**

| プラットフォーム | トークンの種類 | スコープ / 権限 |
|----------|-----------|---------------------|
| GitHub | Personal Access Token（`ghp_`） | `repo`（プライベートリポジトリ）、なし（パブリック） |
| GitLab | Personal Access Token（`glpat-`） | `read_repository` + `write_repository` |
| Bitbucket | Repository Access Token | Read + Write |
| Bitbucket | App Password + `BITBUCKET_USERNAME` | Repositories: Read + Write |

:::warning GitLab のトークンの種類
git 操作に使えるのは **Personal Access Token**（`glpat-`）のみです。Feed Token（`glft-`）には
git アクセス権限が **ありません**。
:::

[環境変数](/docs/reference/appendix/environment-variables#git-authentication) と
[プライベートリポジトリ](/docs/reference/commands/install#private-repositories) を参照してください。

### `SSL certificate problem` / `certificate verification failed`

**原因:** Git サーバーが自己署名証明書、またはシステムが信頼していない内部 CA を使用している。
セルフホストの GitLab、Gitea、Gogs インスタンスでよく見られます。

**解決策 — オプション1: カスタム CA バンドル（推奨）:**
```bash
export GIT_SSL_CAINFO=/path/to/company-ca-bundle.crt
skillshare install https://gitlab.internal.company.com/team/skills.git --track
```

**解決策 — オプション2: 代わりに SSH を使う（SSL を完全に回避する）:**
```bash
skillshare install git@gitlab.internal.company.com:team/skills.git --track
```

**解決策 — オプション3: SSL 検証を無効化する（非推奨）:**
```bash
GIT_SSL_NO_VERIFY=true skillshare install https://gitlab.internal.company.com/team/skills.git --track
```

:::warning
SSL 検証を無効化するのはセキュリティリスクです。オプション1か2を優先してください。
:::

[環境変数 — Git SSL / TLS](/docs/reference/appendix/environment-variables#git-ssl--tls) を
参照してください。

### `invalid skill: SKILL.md not found`

**原因:** Source に有効な SKILL.md ファイルがない。

**解決策:** Source のパスが正しく、Skill ディレクトリを指していることを確認してください。

---

## Update エラー

### `pull stopped: this machine and the remote both changed ...`（Tracked リポジトリ）

**原因:** Tracked リポジトリに、リモートと競合するローカルコミットがある。`update` は分岐した履歴をマージするが、競合するファイルがあると停止し、マージは取り消される。

**解決策:**
```bash
# 強制的に更新する（ローカルをリモートで置き換える）
skillshare update --force

# または手動で解決する
cd ~/.config/skillshare/skills/_repo-name
git pull --no-rebase
# 競合したファイルを編集してから
git add . && git commit --no-edit
```

:::tip
`skillshare update` と `skillshare install` は現在、git の失敗（認証、SSL、分岐したブランチ）に
対して、生の終了コードの代わりに実用的なエラーメッセージを表示します。
:::

---

## Audit エラー

### `security audit failed — critical threats detected`

**原因:** Skill に重大なセキュリティ脅威（プロンプトインジェクション、データ持ち出し、認証情報への
アクセス）にマッチするパターンが含まれている。

**解決策:**
```bash
# 検出結果を確認する
skillshare audit <skill-name>

# Source を信頼する場合は強制インストールする
skillshare install <source> --force
```

### `audit HIGH: Hidden zero-width Unicode characters detected`

**原因:** Skill にコピー&ペーストの痕跡、または意図的な難読化である可能性のある不可視の
Unicode 文字が含まれている。

**解決策:** 隠し文字を表示できるエディタでファイルを開いて削除するか、Source を信頼する場合は
強制インストールしてください。

---

## Upgrade エラー

### `GitHub API rate limit exceeded`

**原因:** 未認証の API リクエストが多すぎる。

**解決策:**
```bash
# オプション1: GitHub トークンを設定する（推奨）
export GITHUB_TOKEN=ghp_your_token_here
skillshare upgrade

# オプション2: 強制的にアップグレードする
skillshare upgrade --cli --force
```

トークンの作成: https://github.com/settings/tokens （パブリックリポジトリにはスコープ不要）

---

## Skill エラー

### `skill not appearing in AI CLI`

**原因:**
1. Skill が Sync されていない
2. SKILL.md のフォーマットが無効
3. AI CLI がキャッシュしている

**解決策:**
```bash
# 1. Sync する
skillshare sync

# 2. フォーマットを確認する
skillshare doctor

# 3. AI CLI を再起動する
```

### Antigravity が Sync された Skill を読み込まない {#antigravity-does-not-load-synced-skills}

**原因:** Antigravity アプリの Skill スキャナーは **実際のディレクトリ** のみを検出します —
シンボリックリンクはスキップされます。skillshare のデフォルトの `merge` モードは Skill ごとに
1つのシンボリックリンク（Windows では NTFS ジャンクション）を作成するため、そのどれも検出されません。
Windows ではこれが `Incorrect function` エラーとして表面化し、macOS と Linux では Skill が
黙って表示されません。

これは Antigravity 側の制限であり、skillshare のバグではありません。対象は `antigravity` ターゲット（アプリ、`~/.gemini/config/skills`）のみで、スタンドアロンの `agy` CLI は `~/.gemini/antigravity-cli/skills` を読む別の `antigravity-cli` ターゲットです。回避策は2つあります。

**オプション1 — Target を `copy` モードに切り替える**

```bash
skillshare target antigravity --mode copy
skillshare sync --force
```

シンボリックリンクの代わりに実際のディレクトリが書き込まれます。トレードオフ: Source の Skill を
編集した後は `skillshare sync` を再実行する必要があります。

**オプション2 — Antigravity に Source ディレクトリを指定する**

Antigravity で: **Settings → Customizations → Skill Custom Paths → 「+ Add」** を開き、
skillshare の Source への**絶対**パス（例: `/Users/you/.config/skillshare/skills`）を入力して
ください。`~` の省略形は展開されないため、完全なパスが必要です。

どちらの方法でも、Skill を再読み込みするために Antigravity を再起動してください。

### `skill name 'X' is defined in multiple places`

**原因:** 複数の Skill が同じ `name` フィールドを持ち、同じ Target に配置されている。

**解決策:** SKILL.md で一方の名前を変更するか、`include`/`exclude` フィルターを使って異なる
Target にルーティングしてください。
```yaml
# オプション1: SKILL.md で名前空間を分ける
name: team-a-skill-name

# オプション2: フィルターでルーティングする（グローバル Config）
targets:
  codex:
    path: ~/.codex/skills
    include: [_team-a__*]
  claude:
    path: ~/.claude/skills
    include: [_team-b__*]

# オプション2: フィルターでルーティングする（プロジェクト Config）
targets:
  - name: claude
    exclude: [codex-*]
  - name: codex
    include: [codex-*]
```

:::tip
フィルターがすでに重複を分離している場合、sync はそれについて何も出力しません —
対応は不要です。
完全な構文は [Target フィルター](/docs/reference/targets/configuration#include--exclude-target-filters)
を参照してください。
:::

---

## Agent エラー

### 警告: `No agents folder: <targets>`

**原因:** `skillshare sync`（または `skillshare sync agents`）を実行したが、設定済みの
Target のうち1つ以上に Agent ディレクトリが定義されていない。組み込みの Agent パスを持つのは
Claude、Cursor、Augment、OpenCode のみで、他の Target は黙ってスキップされます。

**解決策:**

1. それらの Target に Agent が不要であれば、警告は無視してください。
2. `config.yaml` の Target に `agents:` サブキーを追加して、その Target の Agent Sync を
   有効にしてください。

```yaml
targets:
  myapp:
    path: ~/myapp/skills
    agents:
      path: ~/myapp/agents
```

その後 `skillshare sync agents` を再実行してください。

### `backup is not supported in project mode (except for agents)`

**原因:** `agents` フィルターなしで `skillshare backup -p`（または
`skillshare backup -p <target>`）を実行した。プロジェクトモードでは Agent のバックアップのみが
対応しており、Skill のバックアップはグローバルモード限定です。

**解決策:** `agents` の位置引数を追加するか、`--all` を使用してください。

```bash
skillshare backup -p agents          # プロジェクトの Agent Target
skillshare backup -p agents claude   # 特定の Target
skillshare backup -p --all           # 上と同じ（Agent に絞り込まれる）
```

同じルールが `restore` にも適用されます:
`restore is not supported in project mode (except for agents)`

### `agent name 'X' has invalid characters`

**原因:** Agent のファイル名または `name:` frontmatter フィールドに、許可された文字セット外の
文字が含まれている。

**解決策:** Agent 名には `a-z`、`0-9`、`_`、`-`、`.` のみを使用できます。ファイル名を変更し
（`name:` フィールドも一致するよう更新して）、同じ正規名を共有するようにしてください。

### `.agentignore` パターンが反映されない

**原因:**

1. ファイルが間違った場所にある。Agent Source のルート、つまり
   `~/.config/skillshare/agents/.agentignore`（グローバル）または
   `.skillshare/agents/.agentignore`（プロジェクト）に置かなければなりません。
2. パターンが想定と異なるセグメントにマッチしている — このファイルは
   [gitignore 構文](https://git-scm.com/docs/gitignore) を使用します。

**解決策:** `skillshare doctor` でファイルパスを確認し、パターンを再チェックしてください。
Agent はベース名（`.md` を除く）でマッチするため、`draft-*` は `draft-experiment.md` に
マッチします。CLI にエントリを書き込ませるには `skillshare disable <agent> --kind agent` を
使用してください。

---

## Plugin エラー {#plugin-errors}

### `<agent> CLI is not installed or not on PATH` {#plugin-cli-not-on-path}

**原因:** Plugin コマンドは Skillshare を実行するマシン上で Agent のネイティブ CLI（`claude`、`codex` など）を実行しますが、その CLI が見つかりませんでした。スケジュールされたジョブやサービスから起動した dashboard は、ターミナルより `PATH` が短いことがよくあります。

**解決策:**

1. そのマシンに Agent の CLI をインストールします。
2. インストール済みなら、Skillshare を起動するもの（スケジュールされたジョブの環境など）の `PATH` にそのディレクトリを追加します。
3. [account target](/docs/reference/commands/plugin#accounts) の場合は、代わりに `cli` に実行ファイルの絶対パスを設定できます。

### `Codex CLI not found on the machine running Skillshare` {#plugin-codex-not-found}

**原因:** Skillshare は `PATH`、Homebrew のフォルダー、Codex デスクトップ app の中で Codex CLI を探しましたが、見つかりませんでした。メッセージには探したすべての場所が表示されます。

**解決策:**

1. そのマシンに Codex app または Codex CLI をインストールします。
2. Codex が別の場所にある場合は、Skillshare を起動する環境（スケジュールされたジョブなど）で [`SKILLSHARE_CODEX_CLI`](/docs/reference/appendix/environment-variables#skillshare_codex_cli) にそのパスを設定します。この設定はそのマシンだけに残るため、他の OS と共有する config には影響しません。

### `The native marketplace X is gone` {#plugin-marketplace-gone}

**原因:** この plugin はインポートされたものなので、Skillshare は元のネイティブ marketplace から再インストールしますが、その marketplace が Agent に登録されていません。2 台目のマシンでよく起こります。インポートは最初のマシンでしか記録されていないためです。

**解決策:**

1. Agent で marketplace を追加し直してから、`skillshare sync plugins` を実行します。
2. または、その plugin からその Agent を外し、source から plugin を追加し直します。[クロスマシン Sync — Plugins](/docs/how-to/sharing/cross-machine-sync#plugins) を参照してください。

---

## Binary エラー

### `integration tests cannot find the binary`

**原因:** バイナリがビルドされていない、またはパスが間違っている。

**解決策:**
```bash
go build -o bin/skillshare ./cmd/skillshare
# または設定する
export SKILLSHARE_TEST_BINARY=/path/to/skillshare
```

---

## まだ問題がありますか？

体系的なデバッグ方法については
[トラブルシューティングワークフロー](./troubleshooting-workflow.md) を参照してください。
