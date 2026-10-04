---
sidebar_position: 1
---

# ui

視覚的な Skill 管理のための Web ダッシュボードを起動します。

```bash
skillshare ui                  # フォアグラウンドで実行
skillshare ui start            # バックグラウンドサーバーを起動（既存があれば再利用）
skillshare ui stop             # バックグラウンドサーバーを停止
```

デフォルトブラウザで `http://127.0.0.1:19420` を開きます。

## モード

| モード | 動作 |
|------|------|
| `skillshare ui`（デフォルト） | UI サーバーをフォアグラウンドで実行。`Ctrl+C` で停止 |
| `skillshare ui start` | UI サーバーをバックグラウンドプロセスとして起動し、シェルの制御を返す。再度 `start` を実行した場合、既存のプロセスが正常であればそれを再利用 |
| `skillshare ui stop` | `skillshare ui start` で起動したバックグラウンド UI サーバーを停止 |

## 使うタイミング

- Skill、Target、sync を視覚的な Web インターフェースで管理したい
- CLI フラグを覚えずに Skill を閲覧・インストールしたい
- 視覚的な検出結果レポート付きでセキュリティ監査を実行したい
- CLI に不慣れなチームメンバーとダッシュボードビューを共有したい

## フラグ

| フラグ | デフォルト | 説明 |
|------|---------|-------------|
| `-p`, `--project` | | Project mode で実行（`.skillshare/` を使用） |
| `-g`, `--global` | | グローバルモードで実行（`~/.config/skillshare/` を使用） |
| `--port <port>` | `19420` | HTTP サーバーのポート |
| `--host <host>` | `127.0.0.1` | バインドアドレス（Docker の場合は `0.0.0.0` を使用） |
| `-b`, `--base-path <path>` | | リバースプロキシ用のサブパス（例: `/skillshare`） |
| `--no-open` | `false` | ブラウザを自動的に開かない |
| `--app` | `false` | 可能な場合、ダッシュボードをデスクトップ風の Chromium アプリウィンドウとして開く（`start` モードのみ） |
| `--clear-cache` | | フォアグラウンド形式の場合: キャッシュされた UI アセットをクリアして終了。`start` と組み合わせた場合: キャッシュをクリアしてからバックグラウンドで起動 |

:::tip 自動検出
カレントディレクトリに `.skillshare/config.yaml` が存在する場合、ダッシュボードは自動的に Project mode で起動します。グローバルモードを強制するには `-g` を使用してください。
:::

## 例

```bash
# デフォルト: localhost:19420 でブラウザを開く（フォアグラウンド）
skillshare ui

# Project mode（.skillshare/ の Skill を管理）
skillshare ui -p

# カスタムポート
skillshare ui --port 8080

# Docker / リモートアクセス
skillshare ui --host 0.0.0.0 --no-open

# バックグラウンドで起動し、シェルに制御を戻す
skillshare ui start

# デスクトップ風のクロームレスアプリウィンドウとして起動
skillshare ui start --app

# バックグラウンドサーバーを停止（記憶されている host/port を使用）
skillshare ui stop

# キャッシュされた UI アセットをクリアしてからバックグラウンドで新規起動
skillshare ui start --clear-cache
```

## ダッシュボードのページ

サイドバーはページをタスクごとにグループ化します: sync すること、管理しているもの、同期先、メンテナンス。名前の下の行にはモードとそのフォルダ（例: `Global · ~/.config/skillshare`）が表示されます。

一部のページは、対応が必要な場合にサイドバーに件数を表示します。この件数は、ダッシュボードのタブが開いている間、15 秒ごとに更新されます。

- **Sync**: sync によって適用される変更
- **Git Sync**: 未コミットのファイル、またはツリーがクリーンな場合はまだプッシュされていないコミット
- **Audit**: 直近のスキャンでブロックされた Skill と Agent（スキャンを実行した後に表示）

| ページ | 説明 |
|------|-------------|
| **Dashboard** | Skill、Agent、Extras、MCP サーバー、Hook、Plugin、Target の件数、および対応が必要な項目 |
| **Sync** | 書き込む前に、Target ごとにすべての変更をプレビュー。含める項目を選択（Skills、Agents、Extras、MCP）。Target 内で編集されたファイルは、**Force** がオンでない限り保持される。Target にのみ存在する項目は、ここから Source に collect し戻せる。各 sync は最初に Target フォルダをバックアップする。Target が失敗しても他の Target は sync される。失敗した Target は、その部分（Skills、Agents、Extras、Config）とエラーとともに他の警告より上に一覧表示され、よくある原因（別の場所を指す symlink、権限不足、読み取り専用のファイルシステム、存在しないファイルやフォルダー、無効な Target 設定）にはわかりやすい説明も付き、別の場所を指す Skills の symlink には **Force をオンにする** が表示され、変更一覧でもその Target に印が付く。すべての Target が失敗した場合も、同じように一覧表示される。**前回の同期** カードには、直近の sync で失敗した Target が表示される |
| **Git Sync** | Source リポジトリのコミットとプッシュ、remote にまだないコミットのプッシュ、プルを実行。ページを開くと remote から fetch するため、**Pull** には remote にあるコミット数が表示される。プルはリポジトリのスコープ（`skills`、`agents`、`extras`、または `root`）が保持するものを sync する。詳細は [`pull`](/docs/reference/commands/pull) を参照。**Sync both ways** はローカルの変更をコミットし、プルしてマージし、target を sync してからプッシュする（[`push --pull`](/docs/reference/commands/push#push-and-pull-together) と同じ）。コンフリクトが発生した場合はプッシュ前に停止する。remote に新しいコミットがあるためにプッシュが拒否された場合、エラーに **Pull** が表示される。最初のプルが remote とマージできない場合、remote ブランチでローカルファイルを置き換える force pull を提案する |
| **Hubs** | スキル ページから移動。内蔵の hub、保存済みの hub、自分の Hub（**自分の**）を一覧表示し、選ぶとその Skill を絞り込んでインストールできる。**Hub を追加・作成** は既存の hub の追加、新しい Hub の作成、`skillshare-hub.json` の読み込みを行う。自分の Hub は **編集** で変更し、**共有** でインデックスをダウンロードして `hub add` コマンドを作る。[`hub`](/docs/reference/commands/hub) を参照 |
| **Skills** / **Agents** | インストール済みの項目、**Updates** タブ、**Trash** タブ。Skills にはさらに、Target のコンテキストに Skill が追加するトークン数を見積もる **Analyze** タブがある。**Install** は GitHub を検索するか、URL やパスからインストールする。**+ New Skill** は作成ウィザードを開く。一覧とカードビューは **Folder**（tracked repo、`frontend/react` のようなフォルダ、または **Root**）でフィルタでき、**Folder** でグループ化もできる。Root が先頭で、残りのフォルダは名前の A→Z 順。**tree** ビューは左に source のフォルダ、右に詳細を表示する。フォルダや Skill をクリックして選択し、Cmd/Ctrl クリックで選択に追加、Shift クリックで範囲選択、Skill をダブルクリックで開く。右側では、選択したものすべてを 1 つのスイッチで有効・無効にし、targets を設定し（tracked repo とそのサブフォルダも含む）、各 Skill を個別のスイッチ付きで一覧する。tracked repo では **Update repo** と **Uninstall repo** も使える。`disable-model-invocation: true` を持つ Skill は、一覧・タイル・詳細ページで **manual only** タグが付く。これは [`list`](/docs/reference/commands/list) で `m` キーが切り替えるのと同じ状態。Skill エディタでは、**Add field** が各フロントマターフィールドの説明を表示する。**Sync skills** / **Sync agents** はプレビューしてから、その種類だけをすべての Target に sync する。update、アンインストール、collect の後に表示される **Sync Now** からも同じダイアログが開く |
| **Extras** | **フォルダー**: Skill と一緒に sync される rules、commands、その他のフォルダ。**AGENTS.md**: global モードでは共有 `AGENTS.md` と、それを使う Target。Project では Project の `./AGENTS.md` と、各 Target がそれを読み込めるか。[1 つの AGENTS.md をツール間で共有する](../../how-to/daily-tasks/sharing-instructions.md)を参照  **Memory**: [メモリ共有ガイド](../../how-to/daily-tasks/sharing-memory.md)（英語スクリーンショット）。ノートの検索、プレビュー、編集、削除、INDEX リンク、競合時の下書き保持、Backup Files の履歴と復元、**Connect to agents** → **Review changes** → **Apply changes**。設定状態は読み取り済みを示しません |
| **MCP** | サーバーごとに 1 行表示され、sync 先の Agent がチップとして並び、行のカウントボタンでそれらのトグルを開く。**サーバーを追加** は URL、コマンド、貼り付けたスニペット、またはファイルを受け付ける。**target からインポート** はインストール済みの Agent が既に持っているものを読み込む。各サーバーのメニューには **各 Agent に書き込まれる設定を表示** があり、source とすべての Agent のファイルを一覧表示し、選んだものを未保存の編集を含めて表示する。コンフリクトでは **その Agent からインポート** か **ソースで上書き** を選べる。サーバーのダイアログの **ツール** セクションでは [ツールポリシー](./mcp.md#tool-policy) を設定する。保存済みかどうかに関係なくダイアログの現在の設定でサーバーを一度起動し、ツールをチェックできる一覧にする **ツールを読み込む**、`*` パターン用の **除外ルール** の行がある。ポリシーの一部を適用しない選択中の Agent がそれぞれ表示され、ポリシーを設定すると行にタグが付く。Pi サーバーでは、ダイアログに [Pi の内蔵 MCP](./mcp.md#pi) の設定もあり、ツールの公開方法とその他の Pi 設定を扱い、後者は [`piOptions`](./mcp.md#pi-options) を JSON で受け付ける。説明は情報アイコンのツールチップに表示される。**デフォルト** は `mcp.targets` を編集する。Sync ボックスの **Sync MCP** は保留中の変更を一覧表示し、MCP の設定ファイルだけを書き込み、それぞれのバックアップを保存する。その下では [**チェック**](./mcp.md#check-servers-before-an-agent-starts-them) がサーバーをチェックし、**バックアップと復元** でそれらのバックアップをプレビューして復元できる。 |
| **Plugins** | Plugin ごとに 1 行表示され、その Agent がトグルとして並ぶ。行を展開すると、そのソースが対応する他の Agent も一覧され、いずれかにチェックを入れるとインストールのプレビューが表示される。行のメニューから sync、update、削除ができ、Skillshare がレビューしたローカルコピーを読み取り専用で閲覧する **View files** も開ける。[Manage plugins across tools](/docs/how-to/daily-tasks/sharing-plugins) を参照 |
| **Targets** | ステータス付きの Target 一覧。**ターゲットを追加** では **別のアカウント** も選べます。すでに使っている Agent の 2 つ目の config フォルダで、書き込み先のプレビューが付きます。各 Target のページで include/exclude フィルタを編集し、ローカルのみの Skill を Source に collect し戻せる。一覧には各 Agent が受け取る MCP サーバー数も表示される。MCP 設定ファイルを持つ Target には **MCP** タブがあり、サーバーごとに 1 行で選ぶ。クリックするとすぐ保存され、**Sync all targets** ですべての Target の MCP ファイルに書き込まれる。各 Target には、そのファイル名のタブ（**CLAUDE.md**、**GEMINI.md**、**AGENTS.md** など）もあり、読み込み順を表示し、ファイルを編集し、`AGENTS.md` に変換できる |
| **Projects** | global mode のみ。global config が sync する project フォルダーで、[`projects`](/docs/reference/targets/configuration#projects) と [`mcp.projects`](./mcp.md#projects-in-the-dashboard) から一覧される。**プロジェクトを追加** はフォルダー、その target、sync する内容を指定する。各 project には **Skills** と **Agents** タブがあり、フィルター、プレビュー、書き込まれるフォルダーを表示する。**MCP** タブでは、そのフォルダー内で global サーバーをオフにしたり、その project 独自のサーバーを追加したりでき、それらのサーバー用の **チェック** もある。**Sync project** はプレビューしてから、その project の skills、agents、MCP だけを sync する。すでに project フォルダーを指している Target は変換できる |
| **Audit** | Skill と Agent のセキュリティスキャン。重大度別の検出結果を表示。**Rules** タブでは、カテゴリごとにすべてのルールを閲覧できる: ルールをオフにする、重大度を変更する、カテゴリ全体に重大度を適用する、スキャンプロファイル（`default`、`strict`、`permissive`）を選ぶ、カスタム `audit-rules.yaml` のエディタを開く、のいずれかができる |
| **Settings** | タブ分け: **General**（Source パス、sync モード、外観）、**Backup**（Target フォルダのスナップショット、`AGENTS.md` などのファイルの以前のバージョン、MCP 設定のバックアップ。[`backup`](./backup.md#dashboard) を参照）、**Log**（操作履歴）、**Health**（[`doctor`](/docs/reference/commands/doctor) と同じチェック）、**Extensions**（sync 時のファイル変換）、**Files**（`config.yaml`、`.skillignore`、`.agentignore` の直接編集） |

変更一覧の横の **変更を破棄** は、確認後に選択した Git スコープ内の追跡中のファイルとステージング領域を最後のコミットに戻し、未追跡のファイルとフォルダーを削除します。Git に無視されたファイル、入れ子になった Git リポジトリ、`root` スコープの `config.yaml` は保持されます。コミット履歴の変更やリモートへのプッシュは行わず、この操作は元に戻せません。**ドライラン** はファイルを変更せずプレビューします。変更を破棄するには、リポジトリに最初のコミットが必要です。

**Updates** タブでは、プログレスバーに更新の進行状況が表示され、更新中の行が示されます。ブロックされた更新や失敗した更新は別のセクションに表示されます。

`/collect`、`/install`、`/search`、`/trash`、`/analyze`、`/backup`、`/log`、`/doctor` などの古いリンクは、新しい場所にリダイレクトされます。

**Files** タブでは、エディタの横にパネルが表示されます。`config.yaml` の場合、カーソル位置のフィールドが何をするか、ファイルの構造、未保存の変更が表示されます。ignore ファイルの場合、現在のパターンが何を隠しているかが一覧表示されます。`Cmd+S` / `Ctrl+S` で保存します。**Audit -> Rules -> Edit YAML** のルールエディタには同じパネルに加え、貼り付けた行に対してルールの正規表現を実行する **Test** タブがあります。

### テーマシステム

ダッシュボードは、サイドバーの **Theme** ボタンで切り替え可能な 2 つのビジュアルスタイルと 3 つのカラーモードに対応しています。

| 設定 | 選択肢 | デフォルト |
|---------|---------|---------|
| **Style** | `Clean`（プロフェッショナル）、`Playful`（太いアウトライン、ハードシャドウ、手書き風の見出し） | Playful |
| **Mode** | `Light`、`Dark`、`System`（OS の設定に従う） | Light |

テーマの設定はセッションをまたいで localStorage に保持されます。

### Project mode の違い

Project mode（`-p`）で実行すると、ダッシュボードは以下のように適応します。

- **サイドバー** に名前の下に `Project · <project path>` が表示される
- **Git Sync ページ** は非表示（Project の Skill は Project 自体の git を使用するため）
- **Sync** は `skillshare sync -p` と同様、Agent の Target フォルダのみバックアップする
- **Backup タブ** は Settings 内で非表示（代わりにバージョン管理を使用すること）
- **Tracked Repos セクション** は Dashboard から非表示（該当しないため）
- **Settings -> Files** は、グローバル版の代わりに `.skillshare/config.yaml` と Project レベルの `.skillignore` を表示する
- **Available targets** は Project レベルの Target（例: プロジェクトルートからの相対パス `.claude/skills/`）を一覧する
- **Targets** は Project の MCP サーバーを数えて切り替え、Project 自身のファイル（`.mcp.json` など）に書き込む。Claude Code、OpenCode、Kilo Code、Pi では **MCP** タブに、グローバルサーバーをこの Project でオフにするスイッチも表示される
- **Install** は Project 設定の `skills:` エントリを自動的に整合させる
- **Extras -> AGENTS.md** は共有ファイルではなく Project の `./AGENTS.md` を編集し、自身のファイルしか読まない Target 向けに小さな修正を提案する

## UI プレビュー

<div style={{display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: '1rem'}}>
  <img src="/img/web-install-demo.png" alt="インストールの流れ" />
  <img src="/img/web-dashboard-demo.png" alt="ダッシュボードの概要" />
  <img src="/img/web-skills-demo.png" alt="スキル一覧" />
  <img src="/img/web-skill-detail-demo.png" alt="スキルの詳細画面" />
  <img src="/img/web-sync-demo.png" alt="同期操作" />
  <img src="/img/web-search-skills-demo.png" alt="GitHub の検索画面" />
  <img src="/img/web-projects-demo.png" alt="プロジェクトフォルダーを一覧表示する Projects ページ" />
</div>

## REST API

Web ダッシュボードは `/api/` に REST API を公開しています。すべてのエンドポイントは JSON を返します。

| メソッド | パス | 説明 |
|--------|------|-------------|
| GET | `/api/overview` | Skill/Target の件数、モード、バージョン、設定フォルダ（`configDir`） |
| GET | `/api/skills` | メタデータ付きですべての Skill を一覧 |
| GET | `/api/skills/{name}` | Skill の詳細 + SKILL.md の内容 |
| GET | `/api/skills/templates` | Skill 作成に利用可能なパターンとカテゴリを取得 |
| POST | `/api/skills` | 新しい Skill を作成（name、pattern、category、scaffoldDirs） |
| DELETE | `/api/skills/{name}` | Skill をアンインストール |
| GET | `/api/targets` | ステータス、include/exclude フィルタ、Target ごとの想定件数付きで Target を一覧 |
| POST | `/api/targets` | Target を追加 |
| DELETE | `/api/targets/{name}` | Target を削除 |
| POST | `/api/sync` | sync を実行（`dryRun`、`force`、`kind`、`project` に対応。`project` は宣言済みの project ルートで、sync をその project の Target に限定する）。`dryRun` が指定されていない限り、まず Target をバックアップする |
| POST | `/api/git/commit` | Source リポジトリからプッシュせずにローカル git commit を作成 |
| POST | `/api/git/discard` | 設定された Git スコープ内の未コミットの変更を破棄（グローバルモードのみ。リポジトリに最初のコミットが必要）。`dryRun` に対応。Git に無視されたファイル、入れ子になった Git リポジトリ、`root` スコープの `config.yaml` を保持 |
| GET | `/api/git/status` | まだプッシュされていないコミット（`ahead`）と、最後の fetch 時点でまだプルしていない upstream のコミット（`behind`）を含む、Source リポジトリの状態。fetch は行わない |
| POST | `/api/push` | 変更をコミットしてからプッシュ。初回プッシュ時は upstream を設定する。remote にこのリポジトリにないコミットがある場合、`409` とエラーコード `push_rejected` で失敗する。プルしてから再度プッシュする |
| POST | `/api/pull` | プルしてから、リポジトリのスコープが保持するものを sync する。分岐した履歴はマージされる。`.metadata.json` の競合は自動で解決され、それ以外の競合ではマージを取り消して失敗する。最初のプルがマージできない場合、エラーコード `merge_failed` で失敗する。`force: true` で再試行すると、remote ブランチでローカルファイルを置き換える。`alwaysSync: true` を指定すると、新しく取得したものがなくても target を sync する。ブランチのない remote では `400 remote_empty` を返す |
| GET | `/api/diff` | Source と Target 間の差分 |
| GET | `/api/search?q=` | GitHub で Skill を検索 |
| POST | `/api/install` | ソースから Skill をインストール |
| GET | `/api/audit` | すべての Skill をセキュリティ脅威についてスキャン |
| GET | `/api/audit/rules` | カスタム監査ルールの YAML を取得 |
| PUT | `/api/audit/rules` | カスタム監査ルールを保存（正規表現を検証） |
| POST | `/api/audit/rules` | スターター用の audit-rules.yaml を作成 |
| GET | `/api/audit/rules/compiled` | 組み込みルールとカスタムルールをマージした後のすべてのルール、および有効なプロファイル |
| POST | `/api/audit/rules/toggle` | ルールまたはパターン全体を有効化・無効化・再評価 |
| POST | `/api/audit/rules/reset` | カスタムルールを削除し、組み込みのデフォルトに戻す |
| PATCH | `/api/audit/policy` | `blockThreshold`、`profile`、またはその両方を設定 |
| GET | `/api/log` | オプションのフィルタ付きでログエントリを一覧 |
| GET | `/api/config` | 設定を YAML として取得 |
| PUT | `/api/config` | 設定 YAML を更新 |
| GET | `/api/skillignore` | `.skillignore` の内容 + ignore の統計を取得 |
| PUT | `/api/skillignore` | `.skillignore` の内容を更新 |
| GET | `/api/doctor` | すべてのヘルスチェックを実行（JSON） |
| GET | `/api/health` | 死活監視プローブ。サーバーの準備ができると `200` を返す |
| GET | `/api/version` | 現在/最新バージョンとアップグレードの可否 |
| POST | `/api/upgrade` | `skillshare upgrade` をその場で実行（バイナリが開発ビルドの場合は `devMode: true` を返す） |
| POST | `/api/restart` | ローカル UI サーバーを再起動。任意の `{ "clearCache": true }` ボディでキャッシュされた UI アセットを先にクリア |

## インプレースアップグレード

ダッシュボードが新しい CLI リリースが利用可能であることを検出すると、**Update** ダイアログと **Doctor** ページの *Version* カードの両方に **Update now** ボタンが表示されます。

1. UI が `POST /api/upgrade` を呼び出し、ホスト上で `skillshare upgrade` を実行します。
2. 新しいバイナリが配置されると、UI が `POST /api/restart` を呼び出してローカルサーバーを再起動します。
3. ブラウザは `GET /api/health` をポーリングし、新しいサーバーの準備ができ次第、自動的にリロードします。

実行中のバイナリが開発ビルド（`version == "dev"`）である場合、upgrade エンドポイントは `devMode: true` を返し、UI はディスク上の何も変更せずに再起動をシミュレートします。

自動リロードが完了しない場合、ダイアログは `skillshare ui start` を実行してバックグラウンドサーバーを復旧するよう案内します。

## リバースプロキシ {#reverse-proxy}

共有サーバー（ホームラボ、社内ツールプラットフォームなど）でダッシュボードを実行し、リバースプロキシの背後に置く場合は、`--base-path` を使って他のサービスと並ぶサブパスの下で配信できます。

```bash
skillshare ui --base-path /skillshare --host 0.0.0.0 --no-open
```

または環境変数を使う場合:

```bash
SKILLSHARE_UI_BASE_PATH=/skillshare skillshare ui --host 0.0.0.0 --no-open
```

### Nginx

```nginx
location /skillshare/ {
    proxy_pass http://127.0.0.1:19420;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}
```

### Caddy

```
handle_path /skillshare/* {
    reverse_proxy 127.0.0.1:19420
}
```

:::tip
`--base-path` を使わない場合、ダッシュボードは従来どおり動作します — `localhost:19420` への直接アクセスに追加の設定は不要です。
:::

:::note MCP settings
MCP ページは、ブラウザが `localhost` または `http://192.168.1.20:19420` のような IP アドレスでダッシュボードを開いている場合にのみ動作します。リバースプロキシを含め、ドメイン名経由の場合、MCP のリクエストは 403 を返します: DNS リバインディング攻撃は常にドメイン名を使うためです。リモートマシンで MCP の設定を管理するには、`ssh -L 19420:127.0.0.1:19420 HOST` でポートをフォワードし、`http://localhost:19420` を開いてください。
:::

## Docker での使用

Docker 内で Web UI を使うには（初回の UI ダウンロードにネットワークアクセスが必要）:

```bash
make playground

# コンテナ内で:
skillshare ui --host 0.0.0.0 --no-open
```

その後、ホストマシンで `http://localhost:19420` を開きます（ポート 19420 は自動的にマッピングされます）。

## Project mode

Web ダッシュボードは Project レベルの Skill を完全にサポートしています。

```bash
cd my-project
skillshare ui -p
```

または `.skillshare/config.yaml` が存在する場合は、単に `skillshare ui`（自動検出）でも構いません。

ダッシュボードは `.skillshare/config.yaml` を読み書きし、Project ローカルの Target に sync し、インストール後にリモートの Skill エントリを整合させます — CLI と同様です。

## ランタイム UI ダウンロード

`skillshare ui` は、初回起動時に対応する GitHub Release からビルド済みの UI アセットを自動的にダウンロードします。アセットは `~/.cache/skillshare/ui/<version>/`（`XDG_CACHE_HOME` を尊重）にキャッシュされるため、以降の起動は即座かつオフラインで行われます。

- **初回実行** には UI アセット（約 2 MB）をダウンロードするためのインターネット接続が必要です
- **以降の実行** はキャッシュされたアセットを使用します — ネットワーク不要
- **アップグレード時**、古いキャッシュ済みバージョンは自動的にクリーンアップされます。新しい UI は `skillshare upgrade` の際に事前ダウンロードされます
- **キャッシュを手動でクリアする** には `skillshare ui --clear-cache` を実行してください

## Homebrew に関する補足

すべてのインストール方法（Homebrew、インストーラースクリプト、手動バイナリ）はランタイム UI ダウンロードを使用します。`skillshare ui` を実行すると、初回起動時に GitHub から UI アセットが自動的にダウンロードされます。以降はキャッシュされたアセットがオフラインで使用されます。

ダウンロード済みの UI キャッシュをクリアするには:

```bash
skillshare ui --clear-cache
```

## アーキテクチャ

Web UI は、対応する GitHub Release からランタイムにダウンロードされ、ディスクキャッシュ（`~/.cache/skillshare/ui/<version>/`）から配信される単一ページの React アプリケーションです。

```
skillshare ui
  ├── Go HTTP server (net/http)
  │   ├── /api/*    → REST API handlers
  │   └── /*        → Cached React SPA (runtime download)
  └── Browser opens http://127.0.0.1:19420
```

## 関連項目

- [status](/docs/reference/commands/status) — CLI のステータスチェック
- [sync](/docs/reference/commands/sync) — CLI の sync コマンド
- [Project Setup](/docs/how-to/sharing/project-setup) — Project mode のセットアップガイド
- [Docker Sandbox](/docs/how-to/advanced/docker-sandbox) — Docker で UI を実行
