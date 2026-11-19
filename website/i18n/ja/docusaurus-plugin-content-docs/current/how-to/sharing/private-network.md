---
sidebar_position: 5
---

# プライベートネットワーク上のマシン

自分のマシン同士がすでに SSH で届く環境（Tailscale、WireGuard、家庭内ネットワークなど）なら、それらのマシン間で skillshare を使えます。GitHub やホスティングサービスは不要です。

このページでは次の 3 つを扱います。

- ホスト型の Git サービスの代わりに、[自分のマシンの 1 台にあるリポジトリ経由で Sync する](#ssh-remote)
- 手元のマシンから[別のマシンのダッシュボードを開く](#remote-dashboard)
- MCP を使って[別のマシン上の Agent に Skill を提供する](#remote-skills)

例では、マシンを `mac-mini` と `laptop` と呼びます。実際には、ネットワーク上の名前（Tailscale の MagicDNS 名など）を使ってください。

## 始める前に {#before-you-start}

パスワードプロンプトなしで SSH でき、相手のマシンで skillshare が見つかることを確認します。

```bash
ssh you@laptop skillshare version
```

SSH で実行したコマンドは対話シェルのプロファイルを読み込まないため、skillshare のインストール先ディレクトリが見つからないことがあります。コマンドが見つからない場合は、そのマシンで `which skillshare` が表示するフルパスを使うか（たとえば Apple silicon の Homebrew なら `/opt/homebrew/bin/skillshare`）、非対話シェルが読み込むファイル（`~/.zshenv` など）でそのディレクトリを `PATH` に追加してください。

## 自分のマシンの 1 台にあるリポジトリ経由で Sync する {#ssh-remote}

`push` と `pull` は任意の Git remote で動作するため、自分のマシンの 1 台にある bare repository を GitHub の代わりに使えます。

リポジトリを置くマシン（Sync 対象のマシンのうちの 1 台でも構いません）で:

```bash
git init --bare ~/skillshare.git
```

最初のマシンで、SSH 経由でそのリポジトリに skillshare を接続して push します。

```bash
skillshare init --remote you@mac-mini:skillshare.git
skillshare push
```

`you@mac-mini:skillshare.git` はそのユーザーのホームディレクトリからの相対パスです。フルパスを使った `ssh://you@mac-mini/home/you/skillshare.git` でも動作します。リポジトリを置いているマシンでは、代わりにローカルパス（`~/skillshare.git`）を指定できます。skillshare がすでに GitHub など別の remote で設定されている場合、`init` はそれを変更しません。切り替えるには、`init` が表示するコマンド（例: `git -C ~/.config/skillshare/skills remote set-url origin you@mac-mini:skillshare.git`）を実行してから `skillshare push` します。

新しい bare repository のデフォルトブランチは `master`（または `init.defaultBranch`）になりますが、最初のマシンが push したブランチとは限りません。他のマシンはデフォルトブランチから始めるため、push したブランチを指すように設定します。

```bash
git --git-dir ~/skillshare.git branch                            # push されたブランチ（main など）
git --git-dir ~/skillshare.git symbolic-ref HEAD refs/heads/main
```

この手順を行わないと、次のマシンで `init` を実行しても pull するものが見つかりません。

その後、他の各マシンでも同じ `init --remote` を実行します。以降は通常のコマンドを使います。

```bash
skillshare push    # Skill を変更したマシンで
skillshare pull    # 別のマシンで。pull は sync も実行する
```

ログインせずに別のマシンを更新するには、そのマシンの `pull` を SSH で実行します。

```bash
skillshare push && ssh you@laptop skillshare pull
```

それ以外は [クロスマシン Sync](/docs/how-to/sharing/cross-machine-sync) の内容がそのまま当てはまります。初回 pull の動作、競合、`push` と `pull` が運ぶ内容も同じです。

:::tip
編集するマシン上にない Skill のコピーは、この bare repository だけになります。他のデータと同じようにバックアップするか、プライベートのホスト型リポジトリなど 2 つ目の remote を追加してください。
:::

## 別のマシンのダッシュボードを開く {#remote-dashboard}

各マシンに独自のダッシュボードがあり、そのマシンだけを管理します。別の場所から開く方法は 2 つあります。

### SSH トンネル経由

相手のマシンでは、自分自身以外では何も待ち受けません。相手のダッシュボードをバックグラウンドで起動し、ローカルのポートをそこに転送します。

```bash
ssh you@laptop skillshare ui start
ssh -N -L 19421:127.0.0.1:19420 you@laptop
```

そのダッシュボードがすでにプライベートアドレス（後述）で動作している場合、`ui start` はそこで実行中であると報告します。その場合は、`-L 19421:100.101.102.103:19420` のように、そのアドレスに転送してください。

`http://localhost:19421` を開きます。ローカルポートに `19421` を使うのは、このマシンですでに `19420` で動作しているダッシュボードとの衝突を避けるためです。

### プライベートネットワークのアドレスで

プライベートネットワークがそのマシンに割り当てるアドレス（Tailscale の IP など。`tailscale ip -4` で確認できます）でダッシュボードを起動します。

```bash
skillshare ui start --host 100.101.102.103
```

ネットワーク上の任意のマシンから `http://100.101.102.103:19420` を開きます。ダッシュボードは IP アドレスであればそのまま応答します。名前で開きたい場合は、その名前を指定して起動するか、入力するアドレスをそのまま trusted として指定します。

```bash
skillshare ui start --host laptop.example.ts.net
skillshare ui start --host 100.101.102.103 --trusted-origin http://laptop:19420
```

それ以外の名前は 403 になります。これは DNS rebinding からダッシュボードを守るためです。詳しいルールは [リバースプロキシ](/docs/reference/commands/ui#reverse-proxy) を参照してください。

:::warning
ダッシュボードにはログインがありません。そのアドレスに到達できる人は、Agent が実行するコマンドを書き込む MCP、Hooks、Plugins ページを含め、すべてを使えます。自分で管理しているネットワーク上のアドレスにのみバインドし、到達できる相手を制限し（たとえば Tailscale のアクセスルールで）、信頼できないネットワークにも参加するマシンでは `--host 0.0.0.0` を使わないでください。
:::

ダッシュボードから開いたエディターは、閲覧している側のマシンではなく、ダッシュボードが動作しているマシンで起動します。

## 別のマシン上の Agent に Skill を提供する {#remote-skills}

skillshare の設定を持たないマシンで Agent が動作している場合、Sync したフォルダーの代わりに、ネットワーク越しに `skillshare mcp serve` から Skill を読み取れます。

```bash
SKILLSHARE_MCP_TOKEN=change-me skillshare mcp serve --http 100.101.102.103:8765 --http-allow-insecure
```

`--http-allow-insecure` は、Tailscale や WireGuard のようにネットワーク自体が通信を暗号化する環境でのみ使ってください。それ以外では HTTPS で提供します。Agent の MCP クライアントを `http://100.101.102.103:8765/` に向け、トークンを bearer token として指定します。トランスポートのルール、TLS オプション、提供される内容は [MCP で Skill を提供する](/docs/reference/commands/mcp#serve-skills-over-mcp) を参照してください。

## 制限事項 {#limits}

- `push` と `pull` が運ぶのは Skill、Agent、extras であり、各マシンの `config.yaml` は含まれません。Plugins、MCP サーバー、hooks は各マシンで個別にセットアップする必要があります。[Plugins、MCP、Hooks](/docs/how-to/sharing/cross-machine-sync#plugins-mcp-hooks) を参照してください。
- ユーザー名が異なるマシンでは、保存されるホームパスも異なります。`config.yaml` を別の方法で共有する場合は、`preserve_tilde_on_save` を有効にしてください（[Configuration](/docs/reference/targets/configuration) を参照）。
- macOS では、SSH 経由で起動したコマンドがログインキーチェーンにアクセスできないことがあります。HTTPS の remote で `ssh you@laptop skillshare pull` が認証情報のエラーで失敗する場合は、そのマシンでは SSH の remote かトークンの環境変数を使ってください。

## 関連項目

- [クロスマシン Sync](/docs/how-to/sharing/cross-machine-sync) — ホスト型の Git サービス経由の Sync
- [ui](/docs/reference/commands/ui) — ダッシュボードのフラグとバックグラウンドサーバー
- [mcp](/docs/reference/commands/mcp#serve-skills-over-mcp) — MCP で Skill を提供する
- [init](/docs/reference/commands/init) — `--remote` での Init
