---
sidebar_position: 2
---

# 環境変数

skillshare が認識するすべての環境変数です。

## 設定

### SKILLSHARE_CONFIG

Config ファイルのパスを上書きします。

```bash
SKILLSHARE_CONFIG=~/custom-config.yaml skillshare status
```

**デフォルト:** `~/.config/skillshare/config.yaml`

---

### XDG_CONFIG_HOME

[XDG Base Directory Specification](https://specifications.freedesktop.org/basedir/latest/) に従い、
ベースとなる設定ディレクトリを上書きします。

```bash
export XDG_CONFIG_HOME=~/my-config
# skillshare は ~/my-config/skillshare/ を使うようになる
```

**デフォルトの挙動:**

| プラットフォーム | デフォルト |
|----------|---------|
| Linux | `~/.config/skillshare/` |
| macOS | `~/.config/skillshare/` |
| Windows | `%AppData%\skillshare\` |

**優先順位:** `SKILLSHARE_CONFIG` > `XDG_CONFIG_HOME` > プラットフォームのデフォルト。

:::note
初期セットアップの後で `XDG_CONFIG_HOME` を設定した場合、既存の `~/.config/skillshare/` ディレクトリを
手動で新しい場所に移動してください。
:::

---

### XDG_DATA_HOME

データディレクトリ（バックアップ、trash）を上書きします。

```bash
export XDG_DATA_HOME=~/my-data
# skillshare は ~/my-data/skillshare/backups/ と ~/my-data/skillshare/trash/ を使うようになる
```

**デフォルト:** `~/.local/share/skillshare/`

---

### XDG_STATE_HOME

状態ディレクトリ（操作ログ）を上書きします。

```bash
export XDG_STATE_HOME=~/my-state
# skillshare は ~/my-state/skillshare/logs/ を使うようになる
```

**デフォルト:** `~/.local/state/skillshare/`

---

### XDG_CACHE_HOME

キャッシュディレクトリ（バージョンチェックキャッシュ、UI dist キャッシュ）を上書きします。

```bash
export XDG_CACHE_HOME=~/my-cache
# skillshare は ~/my-cache/skillshare/ を使うようになる
```

**デフォルト:** `~/.cache/skillshare/`

:::tip 自動移行
v0.13.0 以降、skillshare はバックアップ、trash、ログについて XDG Base Directory Specification に
従います。古いバージョンからアップグレードする場合、これらのディレクトリは初回実行時に
`~/.config/skillshare/` から正しい XDG の場所へ自動的に移行されます。
:::

---

### SKILLSHARE_GITLAB_HOSTS

ネストされたサブグループに対応させるセルフマネージドの GitLab ホスト名のカンマ区切りリストです。
config ファイルがない場合がある CI/CD で便利です。

```bash
SKILLSHARE_GITLAB_HOSTS=git.company.com,code.internal.io skillshare install git.company.com/team/frontend/ui
```

環境変数と config ファイルの [`gitlab_hosts`](/docs/reference/targets/configuration#gitlab_hosts) の
両方が設定されている場合、それらの値は**マージ**されます（重複排除）。無効なエントリ（scheme、パス、
ポートを含む、または空のもの）は黙ってスキップされます。

**デフォルト:** なし（config ファイルの値のみが使われる）

---

### SKILLSHARE_AZURE_HOSTS

セルフホストの Azure DevOps Server ホスト名のカンマ区切りリストです。

```bash
SKILLSHARE_AZURE_HOSTS=azuredevops.mycompany.com,tfs.internal.io
```

環境変数と config ファイルの [`azure_hosts`](/docs/reference/targets/configuration#azure_hosts) の
両方が設定されている場合、それらの値は**マージ**されます（重複排除）。環境変数内の無効なエントリは
黙ってスキップされます。

**デフォルト:** _(なし)_

### SKILLSHARE_GITEA_HOSTS

セルフホストの Gitea ホスト名のカンマ区切りリストです。名前に `gitea` を含むホストはこれがなくても
検出されます。

```bash
SKILLSHARE_GITEA_HOSTS=git.company.com,code.internal.io
```

config ファイルの [`gitea_hosts`](/docs/reference/targets/configuration#gitea_hosts) とマージされます。

**デフォルト:** _(なし)_

### SKILLSHARE_CNB_HOSTS

セルフホストの CNB ホスト名のカンマ区切りリストです。`cnb.cool` はこれがなくても検出されます。

```bash
SKILLSHARE_CNB_HOSTS=cnb.company.com
```

config ファイルの [`cnb_hosts`](/docs/reference/targets/configuration#cnb_hosts) とマージされます。

**デフォルト:** _(なし)_

---

## Web UI

### SKILLSHARE_UI_BASE_PATH

リバースプロキシの背後で Web ダッシュボードを配信するための URL サブパスを設定します。

```bash
SKILLSHARE_UI_BASE_PATH=/skillshare skillshare ui --host 0.0.0.0 --no-open
```

`--base-path /skillshare` と同等です。両方が設定されている場合はフラグが優先されます。

**デフォルト:** なし（ダッシュボードはルート `/` で配信される）

Nginx と Caddy の例については [リバースプロキシ](/docs/reference/commands/ui#reverse-proxy) を
参照してください。

---

## GitHub API

### GITHUB_TOKEN

GitHub の personal access token です。

**用途:**
- GitHub API リクエスト（`skillshare search`、`skillshare upgrade`、バージョンチェック）
- **Git clone 認証** — HTTPS 経由でプライベートリポジトリをインストールする際に自動的に注入される

**トークンの作成方法:**
1. https://github.com/settings/tokens にアクセスする
2. 新しいトークン（classic）を生成する
3. スコープ: プライベートリポジトリには `repo`、公開リポジトリにはなし
4. トークンをコピーする

公式ドキュメント: [Managing your personal access tokens](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)

**使い方:**
```bash
export GITHUB_TOKEN=ghp_your_token_here
skillshare install https://github.com/org/private-skills.git --track
```

**Windows:**
```powershell
# 現在のセッション
$env:GITHUB_TOKEN = "ghp_your_token"

# 永続化
[Environment]::SetEnvironmentVariable("GITHUB_TOKEN", "ghp_your_token", "User")
```

---

### GH_TOKEN

代替の GitHub トークン変数です。[GitHub CLI (`gh`)](https://cli.github.com/) が使用し、skillshare は
フォールバックとして認識します。

**トークン解決の順序:** `GITHUB_TOKEN` → `GH_TOKEN` → `gh auth token`

すでに `gh` を使っていて `gh auth login` で認証済みなら、skillshare は自動的にトークンを取得します —
追加の環境変数は不要です。

```bash
export GH_TOKEN=ghp_your_token_here
skillshare search "react patterns"
```

---

## Git 認証 {#git-authentication}

これらの変数は、プライベートリポジトリの HTTPS 認証を有効にします。設定されている場合、skillshare は
`install` と `update` の際にトークンを自動的に注入します — URL の変更は不要です。

詳細と CI/CD の例については [プライベートリポジトリ](/docs/reference/commands/install#private-repositories)
を参照してください。


### GITLAB_TOKEN

GitLab の personal access token または CI job token です。GitLab がホストするプライベートリポジトリの
HTTPS clone に使われます。

**トークンの作成方法:**
1. https://gitlab.com/-/user_settings/personal_access_tokens にアクセスする
2. 新しいトークンを追加する
3. スコープ: `read_repository`（pull のみ）または `read_repository` + `write_repository`（push と pull）
4. トークン（`glpat-` プレフィックス）をコピーする

公式ドキュメント: [Token overview](https://docs.gitlab.com/security/tokens/)

:::warning トークンの種類
git 操作に使えるのは **Personal Access Token**（`glpat-`）と **Project/Group Access Token** のみです。
Feed Token（`glft-`）は git アクセスを**持ちません**。
:::

```bash
export GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx
skillshare install https://gitlab.com/org/skills.git --track
```

**Windows:**
```powershell
$env:GITLAB_TOKEN = "glpat-xxxxxxxxxxxxxxxxxxxx"
```

### BITBUCKET_TOKEN

Bitbucket のリポジトリアクセストークン、または app password です。Bitbucket がホストするプライベート
リポジトリの HTTPS clone に使われます。

**トークンの作成方法（リポジトリアクセストークン）:**
1. Repository → Settings → Access tokens に移動する
2. **Read** 権限（pull のみ）または **Read + Write**（push と pull）でトークンを作成する
3. トークンをコピーする — `x-token-auth` を自動的に使用（ユーザー名は不要）

**トークンの作成方法（app password）:**
1. https://bitbucket.org/account/settings/app-passwords/ にアクセスする
2. **Repositories: Read**（pull のみ）または **Repositories: Read + Write**（push と pull）で
   app password を作成する
3. `BITBUCKET_USERNAME` も設定する（または URL に `https://<username>@bitbucket.org/...` として含める）

公式ドキュメント: [Access tokens](https://support.atlassian.com/bitbucket-cloud/docs/access-tokens/)

```bash
export BITBUCKET_USERNAME=your_bitbucket_username
export BITBUCKET_TOKEN=your_app_password
skillshare install https://bitbucket.org/team/skills.git --track
```

**Windows:**
```powershell
$env:BITBUCKET_USERNAME = "your_bitbucket_username"
$env:BITBUCKET_TOKEN = "your_app_password"
```

### BITBUCKET_USERNAME

`BITBUCKET_TOKEN` が app password の場合に、それと一緒に使われる Bitbucket のユーザー名です。

```bash
export BITBUCKET_USERNAME=your_bitbucket_username
export BITBUCKET_TOKEN=your_app_password
skillshare install https://bitbucket.org/team/skills.git --track
```

### AZURE_DEVOPS_TOKEN

Azure DevOps の Personal Access Token（PAT）です。Azure DevOps がホストするプライベートリポジトリの
HTTPS clone に使われます。

**トークンの作成方法:**
1. `https://dev.azure.com/{org}/_usersSettings/tokens` に移動する
2. **+ New Token** を選択する
3. スコープ: **Code → Read**（pull のみ）または **Code → Read & Write**（push と pull）
4. トークン（`AZDO` シグネチャ付きの84文字の文字列）をコピーする

公式ドキュメント: [Use Personal Access Tokens](https://learn.microsoft.com/en-us/azure/devops/organizations/accounts/use-personal-access-tokens-to-authenticate?view=azure-devops)

```bash
export AZURE_DEVOPS_TOKEN=your_pat_here
skillshare install https://dev.azure.com/org/project/_git/repo --track
```

**Windows:**
```powershell
$env:AZURE_DEVOPS_TOKEN = "your_pat_here"
```

### GITEA_TOKEN

Gitea のアクセストークンです。`gitea.com` とセルフホストの Gitea 上のプライベートリポジトリの
HTTPS clone、およびフルクローンなしでサブディレクトリをインストールする際の Gitea Contents API に
使われます。

**トークンの作成方法:**
1. Gitea で **Settings → Applications** に移動する
2. 新しいトークンを生成する
3. 権限: `repository: Read`（pull のみ）または `repository: Read and Write`（push と pull）

```bash
export GITEA_TOKEN=your_token
skillshare install https://gitea.com/org/skills.git --track
```

**Windows:**
```powershell
$env:GITEA_TOKEN = "your_token"
```

トークンはインストール元のホストにのみ送信されます。名前に `gitea` を含まないドメイン上のセルフ
ホストインスタンスには [`gitea_hosts`](/docs/reference/targets/configuration#gitea_hosts) が必要で、
それがなければ代わりに `SKILLSHARE_GIT_TOKEN` が使われます。

### CNB_TOKEN

リポジトリへの読み取り権限を持つ [CNB](https://cnb.cool) のアクセストークンです。`cnb.cool` と
セルフホストの CNB 上のプライベートリポジトリの HTTPS clone に使われます。

```bash
export CNB_TOKEN=your_token
skillshare install https://cnb.cool/org/skills --track
```

**Windows:**
```powershell
$env:CNB_TOKEN = "your_token"
```

別のドメイン上のプライベートデプロイには [`cnb_hosts`](/docs/reference/targets/configuration#cnb_hosts)
が必要です。

### SKILLSHARE_GIT_TOKEN

任意の HTTPS git ホスト用の汎用フォールバックトークンです。プラットフォーム固有のトークンが
設定されていない場合に使われます。

```bash
export SKILLSHARE_GIT_TOKEN=your_token
skillshare install https://git.example.com/org/skills.git --track
```

**Windows:**
```powershell
$env:SKILLSHARE_GIT_TOKEN = "your_token"
```

**トークンの優先順位:** プラットフォーム固有（`GITHUB_TOKEN`、`GITLAB_TOKEN`、`BITBUCKET_TOKEN`、
`AZURE_DEVOPS_TOKEN`、`GITEA_TOKEN`、`CNB_TOKEN`） > `SKILLSHARE_GIT_TOKEN`。

---

## Plugin {#plugins}

### SKILLSHARE_CODEX_CLI

[plugin コマンド](/docs/reference/commands/plugin)が実行する Codex CLI を、このマシンだけで指定します。Skillshare が Codex を自分で見つけられないときに設定します。環境変数から読むため、macOS と Windows で同じ `config.yaml` を共有しても、もう一方で誤りになるパスを書く必要はありません。

設定しない場合、Skillshare はまず `PATH` の `codex` を実行し、次の場所を順に試します。

| OS | 場所 |
|---|---|
| macOS | `/opt/homebrew/bin/codex`、`/usr/local/bin/codex`、および Codex app に同梱の CLI（`/Applications` または `~/Applications` の `ChatGPT.app` または `Codex.app`） |
| Windows | 公式 PowerShell インストーラーの `%LOCALAPPDATA%\Programs\OpenAI\Codex\bin\codex.exe`、次に Codex app がインストールする CLI：`%LOCALAPPDATA%\OpenAI\Codex\bin\<version>\codex.exe`（最新のもの） |

```bash
export SKILLSHARE_CODEX_CLI=/Applications/ChatGPT.app/Contents/Resources/codex-cli/bin/codex
```

**Windows:**
```powershell
$env:SKILLSHARE_CODEX_CLI = "C:\Tools\codex.exe"
```

独自の `cli` を設定した [account target](/docs/reference/commands/plugin#accounts) は、その `cli` を使い続けます。

---

## Git SSL / TLS {#git-ssl--tls}

これらの標準的な Git 環境変数は、すべての git 操作にそのまま渡されます。自己署名証明書や内部 CA を
使うセルフホストの Git サーバー（GitLab、Gitea など）に便利です。

### GIT_SSL_CAINFO

カスタム CA 証明書バンドルへのパスです。セルフホストの Git サーバーが内部 CA によって署名された
証明書を使っている場合に使用します。

```bash
export GIT_SSL_CAINFO=/path/to/ca-bundle.crt
skillshare install https://gitlab.internal.company.com/team/skills.git --track
```

### GIT_SSL_NO_VERIFY

SSL 証明書の検証を完全に無効化します。CA 証明書のセットアップが現実的でない場合の最終手段として
使用してください。

:::warning セキュリティリスク
SSL 検証を無効化すると、中間者攻撃に対して脆弱になります。適切な CA バンドルを使った
`GIT_SSL_CAINFO` を優先するか、代わりに SSH を使ってください。
:::

```bash
GIT_SSL_NO_VERIFY=true skillshare install https://gitlab.internal.company.com/team/skills.git --track
```

**代替案: SSL を完全に回避するために SSH を使う:**
```bash
skillshare install git@gitlab.internal.company.com:team/skills.git --track
```

---

## テスト

### SKILLSHARE_TEST_BINARY

統合テスト用の CLI バイナリパスを上書きします。

```bash
SKILLSHARE_TEST_BINARY=/path/to/skillshare go test ./tests/integration
```

**デフォルト:** プロジェクトルートの `bin/skillshare`

---

## 使用例

### 一時的な上書き

```bash
# 単一のコマンド
SKILLSHARE_CONFIG=/tmp/test-config.yaml skillshare status

# 複数のコマンド
export SKILLSHARE_CONFIG=/tmp/test-config.yaml
skillshare status
skillshare list
unset SKILLSHARE_CONFIG
```

### 永続的な設定（macOS/Linux）

`~/.bashrc` または `~/.zshrc` に追加します。
```bash
export GITHUB_TOKEN="ghp_your_token_here"
```

### 永続的な設定（Windows）

```powershell
[Environment]::SetEnvironmentVariable("GITHUB_TOKEN", "ghp_your_token", "User")
```

---

## まとめ

| 変数 | 用途 | デフォルト |
|----------|---------|---------|
| `SKILLSHARE_CONFIG` | Config ファイルのパス | `~/.config/skillshare/config.yaml` |
| `SKILLSHARE_UI_BASE_PATH` | リバースプロキシ用の Web UI サブパス | なし |
| `SKILLSHARE_GITLAB_HOSTS` | カスタム GitLab ホスト名（カンマ区切り） | なし |
| `SKILLSHARE_AZURE_HOSTS` | カスタム Azure DevOps Server ホスト名（カンマ区切り） | なし |
| `SKILLSHARE_GITEA_HOSTS` | カスタム Gitea ホスト名（カンマ区切り） | なし |
| `SKILLSHARE_CNB_HOSTS` | カスタム CNB ホスト名（カンマ区切り） | なし |
| `XDG_CONFIG_HOME` | ベースとなる config ディレクトリ | `~/.config`（Linux/macOS）、`%AppData%`（Windows） |
| `XDG_DATA_HOME` | データディレクトリ（バックアップ、trash） | `~/.local/share` |
| `XDG_STATE_HOME` | 状態ディレクトリ（ログ） | `~/.local/state` |
| `XDG_CACHE_HOME` | キャッシュディレクトリ（バージョンチェック、UI） | `~/.cache` |
| `GITHUB_TOKEN` | GitHub API + git clone 認証 | なし |
| `GH_TOKEN` | GitHub API（`GITHUB_TOKEN` のフォールバック） | なし |
| `GITLAB_TOKEN` | GitLab git clone 認証 | なし |
| `BITBUCKET_TOKEN` | Bitbucket git clone 認証 | なし |
| `BITBUCKET_USERNAME` | app password 認証用の Bitbucket ユーザー名 | なし |
| `AZURE_DEVOPS_TOKEN` | Azure DevOps git clone 認証 | なし |
| `GITEA_TOKEN` | Gitea git clone + Contents API 認証 | なし |
| `CNB_TOKEN` | CNB git clone + contents API 認証 | なし |
| `SKILLSHARE_GIT_TOKEN` | 汎用 git clone 認証（フォールバック） | なし |
| `SKILLSHARE_CODEX_CLI` | このマシンで plugin コマンドが使う Codex CLI | `PATH` 上の `codex`、次に既知のインストール場所 |
| `GIT_SSL_CAINFO` | カスタム CA 証明書バンドルのパス | システムデフォルト |
| `GIT_SSL_NO_VERIFY` | SSL 証明書の検証を無効化する | `false` |
| `SKILLSHARE_TEST_BINARY` | テストバイナリのパス | `bin/skillshare` |

---

## 関連項目

- [Configuration](/docs/reference/targets/configuration) — Config ファイルリファレンス
- [Windows の問題](/docs/troubleshooting/windows) — Windows 環境のセットアップ
