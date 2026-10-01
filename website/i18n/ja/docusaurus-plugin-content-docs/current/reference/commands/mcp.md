---
sidebar_position: 3
---

# mcp

ポータブルな MCP 接続定義を管理し、ネイティブな Agent 設定を同期します。
まずは [Set up MCP once](/docs/how-to/daily-tasks/sharing-mcp) から始めてください。

## コマンド

```bash
skillshare mcp
skillshare mcp add
skillshare mcp edit
skillshare mcp edit docs --url https://updated.example/mcp --no-tui
skillshare mcp add docs --url https://example.com/mcp --target claude --sync
skillshare mcp add local --target codex -- company-mcp --workspace /path/to/workspace
skillshare mcp import docs --from claude --target claude --target cursor --sync
skillshare mcp import docs --file ./provider.json --target claude
skillshare mcp list --json
skillshare mcp check
skillshare mcp check docs --json --no-dns
skillshare mcp check --live --timeout 30s
skillshare mcp remove docs --sync
skillshare mcp remove docs --keep-files
skillshare mcp restore BACKUP_ID --dry-run
skillshare sync mcp --dry-run --json
skillshare sync mcp
skillshare sync --all
```

| オプション | 意味 |
|---|---|
| `--target CLIENT` | 受け取り側クライアント。複数のクライアントを選択するには繰り返し指定。`--target none` はサーバーをどのクライアントにも書き込まずに Skillshare 内に保持する。[下記](#keep-a-server-without-syncing-it)を参照 |
| `--url URL` | `add` 用の Streamable HTTP エンドポイント |
| `-- command args...` | `add` 用のローカル実行ファイルとリテラルな引数 |
| `--disabled` | project mode で `add` と併用: Agent の global config が定義するサーバーをオフにする。[下記](#turn-off-a-global-server-in-one-project)を参照 |
| `--tools-allow TOOLS` | これらのツールだけを残す。カンマ区切り。`*` は任意の文字に一致。`""` でクリア。[ツールポリシー](#tool-policy)を参照 |
| `--tools-deny TOOLS` | これらのツールを常に除外する。カンマ区切り。allow より優先。`""` でクリア。[ツールポリシー](#tool-policy)を参照 |
| `--pi-options JSON` | Pi の内蔵 MCP のその他のサーバー別フィールドを JSON オブジェクトで指定する。[Pi](#pi-options)を参照 |
| `--from CLIENT` | インポート元の既存クライアント、または `--file` のフォーマット |
| `--file PATH` | ネイティブの JSON/JSONC、TOML、または Goose の YAML。`.toml` はデフォルトで Codex とみなされ、他のフォーマットはその MCP セクションから検出される。明示的な方言を指定するには `--from` を使う |
| `--sync` | 保存して同期する。非インタラクティブな add/import/remove ではそうしない限り保存のみ |
| `--keep-files` | `remove` と併用: サーバーの管理をやめ、Agent のエントリはそのまま残す。`--sync` とは併用できない。[下記](#stop-managing-a-server)を参照 |
| `--replace` | add/import 中に既存の source 定義を明示的に置き換える。import では、インポート元クライアントのエントリが異なる場合にそれも書き換える |
| `--dry-run`, `-n` | 保存もネイティブ設定への書き込みも行わずにプレビュー |
| `--json` | 構造化出力。sync/preview のレポートには name、path、action が含まれ、サーバーの値は含まれない |
| `--no-dns` | `check` で使用。リモートサーバーのホスト名の名前解決をスキップする。[下記](#check-servers-before-an-agent-starts-them)を参照 |
| `--live` | `check` で使用。各ローカルサーバーの起動と各リモートサーバーの呼び出しも行う。[下記](#probe-servers-live)を参照 |
| `--timeout DURATION` | `check --live` で使用。各サーバーのプローブの制限時間(`30s` など)。デフォルトは `10s` |
| `--no-tui` | インタラクティブメニューを無効化。`tui: false`、`--json`、または非ターミナルの入出力でも無効になる |
| `--revision ID` | add/import/remove または `sync mcp` に一致するプレビューを要求する |
| `--global`, `-g` | global の Skillshare 設定を使う |
| `--project`, `-p` | project の Skillshare 設定を使う |

サブコマンドを指定しない場合、`mcp` はインタラクティブなターミナルで検索可能なマネージャーを開くか、非インタラクティブモードではステータスを表示します。名前を指定しない非インタラクティブなインポートは、解析済みの候補を選択のために一覧表示し、保存は行いません。候補にはポータブルな定義が含まれ、認識可能な secret は参照に変換されます。Agent 固有のフィールドは警告として一覧表示され除外されます。無効化されたサーバーと非対応のトランスポートは候補をブロックします。`restore` は適用前に必ず再度プレビューします。適用せずに確認するには `--dry-run` を使ってください。

`--pi-extension`、`--pi-options-prune`、`--direct-tools` は 0.23.0 で削除され、現在は代わりに何を使うべきかを示すメッセージとともに失敗します。[0.22 からの Pi のアップグレード](#pi-migration)を参照してください。

`sync mcp` は scope フラグ、`--dry-run`、`--json`、`--no-tui`、`--revision` を受け付けます。
`sync --all` には skills、agents、extras、MCP、hooks が含まれます。単なる `sync` は既存のリソースの動作を維持します。MCP の競合は、`--all` が他のリソースを変更する前にチェックされます。リソース種別とネイティブファイルは、単一のトランザクションではなく別々の操作です。

## インタラクティブ管理

`skillshare mcp` または `skillshare mcp list` を実行します。Skill 一覧と同様に、マネージャーは検索用の `/` と詳細表示用の `Enter` に対応しています。接続一覧では、引数・ヘッダー・環境変数の値は非表示になり、URL のクエリも省略されます。

| キー | 動作 |
|---|---|
| `a` | 接続を追加 |
| `i` | 1 つ以上の接続をインポート |
| `e` | 選択した接続を編集 |
| `x` | 選択した接続を削除 |
| `s` | 同期をプレビューして確認 |
| `b` | クライアント別にバックアップを閲覧し、新しい順に表示 |
| `r` | ステータスを更新 |
| `q` | 終了 |

`mcp edit`、`mcp remove`、`mcp restore` は、name またはバックアップ ID が省略された場合に選択メニューを提供します。エディタは command/URL、引数、環境変数、HTTP ヘッダー、bearer-token の環境変数参照、受け取り側の target、[ツールポリシー](#tool-policy)（**ツール**）をカバーします。引数は 1 行につき 1 つのリテラル引数、または JSON 配列で受け付けます。トランスポートを切り替えると、新しい接続タイプに適用されないフィールドはクリアされます。

Add、edit、remove、import では、**Save and sync** または **Save only** の前にプレビューが表示されます。Remove には **Stop managing** もあり、`--keep-files` と同じ動作です。Escape で保留中のドラフトをキャンセルできます。Restore は Agent のエントリへの変更をプレビューし確認しますが、source 定義自体は書き換えません。

サーバー名を指定しないインポートは複数選択に対応しています（`Space` でトグル、`a` ですべて選択）。無効な候補はスキップされます。既存の source 名は `--replace` を指定しない限りスキップされます。バッチに対しては、互換性のある受け取り側クライアントを 1 セット選択してください。バッチ全体が検証された後、source は一度だけ保存されます。その後のネイティブファイル I/O 失敗については、既存の復旧動作が維持されます。

スクリプトからは、name とフラグを指定します。`mcp edit NAME --url URL`、`mcp edit NAME --target CLIENT`、`mcp edit NAME -- command args...` は、他の該当する設定を保持したまま指定されたフィールドを更新します。`--sync` を追加しない限り保存のみを行います。`--no-tui` の場合、remove には name が、restore にはバックアップ ID が必要です。`--dry-run` は変更を保存も同期も行いません。

## Source フィールド

インラインの `mcp.servers`、または `sources.mcp` で指定した外部ファイルのいずれかを選べます。外部ファイルにはトップレベルの `servers` マッピングが必要です。`mcp.targets` と [`mcp.projects`](#manage-several-projects-from-the-global-config) は Skillshare の config に残ります。スキーマはリポジトリ内の `schemas/mcp.schema.json` です。

| サーバーフィールド | 意味 |
|---|---|
| `command` | ローカル実行ファイル。`url` とは併用不可 |
| `args` | ローカル実行ファイルのリテラルな引数のリスト |
| `env` | ローカルの環境変数値: 文字列または `{fromEnv: VARIABLE}` |
| `url` | HTTP(S) の MCP エンドポイント。埋め込みの認証情報やフラグメントは不可 |
| `headers` | HTTP ヘッダー: 文字列または `{fromEnv: VARIABLE}` |
| `bearerToken` | `{fromEnv: VARIABLE}`。Authorization ヘッダーと共存不可 |
| `transport` | 任意の `stdio` または `streamable-http`。省略時は推測される |
| `targets` | 任意の受け取り側クライアント。`mcp.targets` を上書きする。空のリストにすると、サーバーは Skillshare 内にのみ保持される。[下記](#keep-a-server-without-syncing-it)を参照 |
| `tools` | どのツールをモデルに渡すか: `allow`、`deny`。一度書けば Agent ごとに変換される。[ツールポリシー](#tool-policy)を参照 |
| `piOptions` | Pi の内蔵 MCP のその他のサーバー別フィールド。[Pi](#pi-options)を参照 |
| `disabled` | `true` のみ、他の接続フィールドを伴わない、かつ project がスコープ内にあること: project mode、または `mcp.projects` 配下の root。[下記](#turn-off-a-global-server-in-one-project)を参照 |

クライアント ID は `claude`、`codex`、`cursor`、`vscode`、`opencode`、`kilocode`、
`grok`、`antigravity`、`amp`、`claude-desktop`、`cline`、`copilot`、`factory`、`gemini`、
`goose`、`junie`、`kiro`、`lmstudio`、`warp`、`windsurf`、`pi` です。
`grok` は公式の xAI Grok CLI を意味します。サーバー名には文字、
数字、ドット、アンダースコア、ハイフンを使用します。サーバーは同期前に、直接または
`mcp.targets` 経由で少なくとも 1 つのクライアントを選択する必要があります。ただし、サーバー自身の
`targets` が空のリストである場合は除きます。

### 同期せずにサーバーを保持する {#keep-a-server-without-syncing-it}

`targets: []` を持つサーバーは Skillshare の source に残り、どのクライアントにも書き込まれません。
定義を後のために残したまま、サーバーをすべてのクライアントから外したいときに使います。
以前に同期されていた場合、次の sync でそれらのクライアントからエントリが削除されます。

```yaml
mcp:
  targets: [claude, codex]
  servers:
    docs:
      url: https://example.com/mcp
      targets: []
```

```bash
skillshare mcp add docs --url https://example.com/mcp --target none
skillshare mcp edit docs --target none
skillshare mcp edit docs --target claude   # 元に戻す
```

- **`targets` を省略するのとは異なります。** その場合、サーバーは `mcp.targets` を継承し、
  そのリストも空であれば拒否されます。
- `none` はクライアントと組み合わせられません。
- ターミナルの選択メニューでは、クライアントを何も選択せずに確定します。ダッシュボードでは、
  すべてのクライアントのチェックを外します。サーバーには **Agent 未選択** のタグが付きます。
- project のサーバーでも、`mcp.projects` 配下のサーバーでも同じように動作します。
- `disabled` エントリには引き続き少なくとも 1 つのクライアントが必要です。どこかでサーバーを
  オフにする必要があるためです。
- `mcp list` では、このようなサーバーは `kept no targets` と表示されます。

Grok の場合、名前は文字またはアンダースコアで始まり、文字、数字、ハイフン、単一のアンダースコアのみを含み、アンダースコアで終わってはいけません。
`company-docs` のような名前は、すべての対応クライアントで動作します。

## ネイティブな送信先 {#native-destinations}

| クライアント | Global | Project | セクション |
|---|---|---|---|
| Claude Code | `~/.claude.json` | `.mcp.json` | `mcpServers` |
| Codex | `~/.codex/config.toml` | `.codex/config.toml` | `mcp_servers` |
| Cursor | `~/.cursor/mcp.json` | `.cursor/mcp.json` | `mcpServers` |
| VS Code | User `mcp.json`（下記） | `.vscode/mcp.json` | `servers` |
| OpenCode | `~/.config/opencode/opencode.json` | `opencode.json` | `mcp` |
| Kilo Code | `~/.config/kilo/kilo.jsonc` | `kilo.jsonc` | `mcp` |
| Grok CLI | `~/.grok/config.toml` | `.grok/config.toml` | `mcp_servers` |
| Antigravity (AGY) | `~/.gemini/config/mcp_config.json` | `.agents/mcp_config.json` | `mcpServers` |
| [Amp](https://ampcode.com/docs/customize/mcp) | `~/.config/amp/settings.json` | `.amp/settings.json` | `amp.mcpServers`（リテラルキー） |
| [Claude Desktop](https://modelcontextprotocol.io/docs/develop/connect-local-servers) | Claude のアプリケーションデータディレクトリ、`claude_desktop_config.json` | Global のみ | `mcpServers` |
| [Cline](https://github.com/cline/cline/tree/main/apps/vscode/src/services/mcp) | `~/.cline/data/settings/cline_mcp_settings.json` | Global のみ | `mcpServers` |
| [Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-mcp-servers) | `~/.copilot/mcp-config.json` | `.github/mcp.json` | `mcpServers` |
| [Factory Droid](https://docs.factory.ai/harness/mcp) | `~/.factory/mcp.json` | `.factory/mcp.json` | `mcpServers` |
| [Gemini CLI](https://geminicli.com/docs/tools/mcp-server/) | `~/.gemini/settings.json` | `.gemini/settings.json` | `mcpServers` |
| [Goose](https://block.github.io/goose/docs/guides/config-files/) | `~/.config/goose/config.yaml` | Global のみ | `extensions`（YAML） |
| [Junie](https://junie.jetbrains.com/docs/junie-cli-mcp-configuration.html) | `~/.junie/mcp/mcp.json` | `.junie/mcp/mcp.json` | `mcpServers` |
| [Kiro](https://kiro.dev/docs/mcp/configuration/) | `~/.kiro/settings/mcp.json` | `.kiro/settings/mcp.json` | `mcpServers` |
| [LM Studio](https://lmstudio.ai/docs/app/mcp) | `~/.lmstudio/mcp.json` | Global のみ | `mcpServers` |
| [Warp](https://docs.warp.dev/agents/capabilities/mcp/) | `~/.warp/.mcp.json` | `.warp/.mcp.json` | `mcpServers` |
| [Windsurf (Cascade)](https://docs.devin.ai/desktop/cascade/mcp) | `~/.codeium/windsurf/mcp_config.json` | Global のみ | `mcpServers` |

ダッシュボードのサーバーフォームは、HTTP ヘッダーを環境変数と同じ方法で編集し、`fromEnv` 参照にも対応します。サーバーのメニューとフォーム内のファイル数の横にある **View what each Agent gets** は、選択したクライアントに対して Sync が書き込む予定のネイティブテキストを読み取り専用で表示します。フォーム内では、まだ保存されていない編集内容が反映されます。secret は参照のままです。

JSON エントリは、そのファイル自体のインデントに合わせて 1 行に 1 フィールドで書き込まれます。Skillshare が所有しているにもかかわらず 1 行にまとまっているエントリは `update` として報告され、改めてレイアウトされて書き込まれます。Skillshare が所有していないエントリや、手動でフォーマットされたエントリは、そのレイアウトを保持します。

ダッシュボードは、現在の scope とホストのプラットフォームで利用可能な送信先のみを提供します。各サーバーは 1 行として表示され、名前の下に送信先のクライアントがチップで並びます。右側のカウントボタンからそのサーバーの全クライアント一覧を開けます。Global 専用のクライアントは project mode では選択できません。
右側の **Sync** ボックスには、まだ書き込まれていない変更が一覧表示されます。クライアントにチェックを入れるだけでは source のみが編集されます。**Sync MCP** はそれらの変更を一覧表示し、確認後に MCP の設定ファイルだけを書き込み、それぞれのバックアップを保存します。同じボックスの区切り線の下では、サーバーがあれば **チェック** で[サーバーをチェック](#check-servers-before-an-agent-starts-them)でき、**バックアップと復元** でそれらのバックアップを参照できます。その下の **Agents** には、このマシン上で検出されたクライアントが一覧表示されます。クライアントの MCP ファイルが存在するか、そのクライアントが設定を保存するフォルダが存在すれば「検出済み」と見なされるため、MCP ファイルがまだない新規インストールでも表示されます。project mode では、project が MCP ファイルを持っているか、そのクライアントが global で検出されている場合に一覧表示されます。

追加のクライアント詳細:

- `codex` の送信先は、Codex CLI、Codex IDE 拡張機能、ChatGPT デスクトップアプリが共有する 1 つの `config.toml` です。そのため、`codex` に同期したサーバーはこの 3 つすべてに表示されます。ChatGPT デスクトップアプリでは **Settings → MCP servers** の下に表示されます。
  Codex は、trust している project でのみ `.codex/config.toml` を読み込みます。trust していない project では、同期されたサーバーはエラーなく読み込まれません。`cwd`、
  `http_headers_helper`、承認モード、タイムアウト、`oauth` テーブルにはポータブルな形式がありません。インポートではこれらを警告付きで除外し、sync は既存のエントリ内にそれらを維持します。`enabled_tools` と `disabled_tools` は[ツールポリシー](#tool-policy)から書き込まれ、インポート時もそこに取り込まれます。Codex の plugin が同梱する MCP サーバーは `plugins.<plugin>.mcp_servers` の下に設定され、ここでは管理されません。
- Claude Desktop のファイル sync は macOS と Windows で **stdio のみ**に対応します。
  ディレクトリは macOS では `~/Library/Application Support/Claude`、
  Windows では `%APPDATA%/Claude` です。リモートコネクタはアプリケーション内で設定してください。
- Cline はデフォルトの VS Code Stable プロファイルを対象とし、Cline CLI や他の IDE は対象外です。
- Copilot CLI のエントリには `tools` が書き込まれます。[ツールポリシー](#tool-policy)が許可する完全一致のツール名で、それがなければ `["*"]` です。インポートでは `tools` をポリシーに読み戻します。project の `.mcp.json` が存在する場合、Copilot はその ファイルを `.github/mcp.json` より先に読み込むため sync は停止します。先にファイルを統合してください。
  project mode で Claude Code と Copilot CLI を同時に選択することも、いずれかのファイルを書き込む前にブロックされます。これらのクライアントの一方には global mode を使ってください。
- Gemini は Streamable HTTP に `httpUrl` を使用します。その `url` フィールドはレガシーな SSE を意味し、インポート時に拒否されます。Cline は `type: streamableHttp` を、Goose は
  `type: streamable_http` と `uri` を使用します。Skillshare はこれらを自動的に変換します。
- Goose は Windows で `%APPDATA%/Block/goose/config/config.yaml` を使用します。YAML の編集は
  関連のない設定、コメント、組み込みの extension を保持しますが、フォーマットが変わる場合があります。Alias、merge、重複キー、複数ドキュメントは編集をブロックします。
  組み込みの extension とキーチェーンの `env_keys` は、ポータブルな MCP 接続としてインポートできません。
- Claude Code は、組み込みサーバー用に予約されている `workspace`、`claude-in-chrome`、`computer-use` という名前のサーバーをスキップします。また、リモートサーバーに自身の認証情報を送ることは決してありません。`ANTHROPIC_API_KEY`、`ANTHROPIC_AUTH_TOKEN`、`AWS_BEARER_TOKEN_BEDROCK`、
  `HTTPS_PROXY`、`NPM_TOKEN` は `url` と `headers` では空として読み込まれます。Skillshare は Claude に対してその両方を拒否します。認証情報は自分で名前を付けた変数にコピーしてください。
- Claude Code にはローカルスコープもあります。`--scope` を指定せずに `claude mcp add` で追加したサーバーは、project ごとに `~/.claude.json` に保存されます。ローカルサーバーは、
  `.mcp.json` や user scope にある同名のものより丸ごと優先されます。project mode では、Skillshare はそのようなサーバーを、隠しているエントリの隣に、sync をブロックすることなく報告します。project フォルダから `claude mcp remove NAME -s local` で削除してください。
- Cline の VS Code 拡張機能、CLI、SDK は `~/.cline/data/settings/` を共有します。拡張機能は
  古い VS Code の `globalStorage` ファイルを一度だけそこに移行し、以降はそれを読まなくなります。そのため Skillshare は `~/.cline/data` がまだ存在しない場合のみ古いファイルに書き込みます。`CLINE_MCP_SETTINGS_PATH`、`CLINE_DATA_DIR`、`CLINE_DIR` はこの順序で尊重されます。
- Windsurf のサポートは、ドキュメント化された Cascade 設定を対象としています。Windsurf の新しい
  Devin Local agent は独自の `~/.config/devin/mcp_config.json` を読み込みますが、Skillshare はこれを管理しません。Warp の project 接続は、セッションごとに Warp 内での承認が引き続き必要です。
- Amp は、`amp mcp approve <name>` を実行した後にのみ、project の `.amp/settings.json` からサーバーを実行します。Global のサーバーには承認は不要です。
- Kiro は、その「Mcp Approved Env Vars」設定に一覧されている名前についてのみ `${VARIABLE}` を展開し、localhost に限り `http://` URL を受け付けます。
- VS Code は、デフォルト以外の各プロファイルについて `User/profiles/` 以下に別々の `mcp.json` を保持します。Skillshare はデフォルトプロファイルのファイルを管理します。

環境変数参照は、Amp、Copilot CLI、Factory、Gemini CLI、Kiro については `${VARIABLE}` として、Cline と Windsurf については `${env:VARIABLE}` としてエクスポートされます。
Claude Desktop、Goose、Junie、LM Studio、Warp は、そのネイティブな変数展開が検証されていないため、現時点では `fromEnv` と
`bearerToken` のエクスポートを拒否します。カスタム認証情報を使わない接続を利用するか、対応している受け取り側クライアントで認証してください。Skillshare は参照を平文に解決することは決してありません。

Antigravity は現在の[公式 MCP 設定](https://antigravity.google/docs/mcp)を使用し、
リモート接続には `serverUrl` を含みます。Skillshare はポータブルな `url` を自動的に
変換します。古い `.gemini/antigravity/` と `.gemini/antigravity-cli/` の設定
場所は管理対象外です。Antigravity の `fromEnv` と `bearerToken` のエクスポートは、そのドキュメント化された設定が環境変数の展開を規定していないためブロックされます。カスタムの secret ヘッダーを必要としない接続を使用し、
Antigravity 内で対応している OAuth ログインを完了してください。Skillshare は参照を平文の認証情報に展開することは決してありません。

OpenCode は、global ディレクトリについて `XDG_CONFIG_HOME` を尊重します。既存の
`opencode.jsonc` は `opencode.json` を作成する代わりに使用されます。project では、OpenCode は `.opencode/` からも両方の名前を読み込みます。そのため、そこに置かれたファイルがあれば Skillshare はそのファイルに書き込み、新しいファイルは project ルートに作成されます。複数存在する場合は、sync する前に統合してください。カスタムの OpenCode config パス、ディレクトリオーバーライド、インラインの config、継承された祖先ファイルは管理対象外です。これらは OpenCode 内で選択した送信先を上書きすることがあります。

Kilo Code は OpenCode と同じフォーマットを使用します。project ルートと `.kilo/` から `kilo.jsonc` と `kilo.json` を読み込み、
それらをマージします。そのため Skillshare は既に存在する方に書き込み、どちらも存在しない場合にのみ `kilo.jsonc` を作成します。
両方が存在する場合は、sync する前に統合してください。`KILO_CONFIG`、
`KILO_CONFIG_DIR`、および古い VS Code 拡張機能の `mcp_settings.json` は管理対象外です。

Kilo Code は project の config を信頼されていないものとして扱います。そこでの `{env:VARIABLE}`
参照は許可されず、project ファイルが見つかった場合はそのファイル全体を無視します。そのため project
mode では、Skillshare は `fromEnv` または `bearerToken` を使う Kilo Code サーバーを拒否します。そのようなサーバーは、参照が許可される global mode で定義してください。

OpenCode と Kilo Code は `local`/`remote` タイプと `{env:VARIABLE}` 参照を使用し、Grok は
`${VARIABLE}` 参照を使用します。Skillshare はこれらを自動的に変換します。Claude の
`"type": "streamable-http"` は HTTP としてインポートされます。無効化された接続はインポートをブロックします。
Codex の `startup_timeout_sec` や `envFile` のような、ポータブルな対応形式がないその他のネイティブオプションは、
警告付きでインポートから除外されます。sync はそれらを Agent の既存エントリ内に維持します。Pi は内蔵 MCP を使います。
[下記](#pi)を参照してください。

VS Code Stable のデフォルトのユーザーファイルは以下のとおりです。

- macOS: `~/Library/Application Support/Code/User/mcp.json`
- Linux: `${XDG_CONFIG_HOME:-~/.config}/Code/User/mcp.json`
- Windows: `%APPDATA%/Code/User/mcp.json`

Global の Claude、Codex、Grok、Copilot のパスは、`CLAUDE_CONFIG_DIR`、`CODEX_HOME`、
`GROK_HOME`、`COPILOT_HOME` を尊重します。`OPENCODE_CONFIG` と `OPENCODE_CONFIG_DIR` は管理対象外です。Amp と Goose は、`.config` パスを使うプラットフォームでは `XDG_CONFIG_HOME` を尊重します。
project の送信先は、選択された project ルートからの相対パスです。project の trust、
サーバーの承認、認証は引き続き受け取り側 Agent の責任です。

### Agent の別のアカウント {#accounts}

[Agent の別のアカウント](/docs/reference/targets/configuration#agent-config-dir)として宣言された Target は、`claude`（`CLAUDE_CONFIG_DIR`）、`codex`（`CODEX_HOME`）、`pi`（`PI_CODING_AGENT_DIR`）については MCP の Target でもあります。そのサーバーは、その Agent のフォーマットで、アカウント自身のファイル（Claude は `<config_dir>/.claude.json`、Codex は `<config_dir>/config.toml`、Pi は `<config_dir>/mcp.json`）に書き込まれます。

```yaml
targets:
  claude-work:
    agent: claude
    config_dir: ~/.claude-work

mcp:
  targets: [claude, claude-work]      # 両方のアカウントがすべてのサーバーを受け取る
  servers:
    docs:
      url: https://example.com/mcp
    jira:
      command: jira-mcp
      targets: [claude-work]          # 仕事用アカウントのみ
```

この例では、`docs` は `~/.claude.json` と `~/.claude-work/.claude.json` に、`jira` は 2 つ目のファイルにのみ書き込まれます。`--target claude-work` は `mcp add` と `mcp edit` で使え、ダッシュボードではそのアカウントが Agent と並んで一覧表示されます。

どのアカウントも同じ project ファイルを読み込むため、`mcp.projects` 内と project mode では Agent 自身の名前を使ってください。Claude Code は project のオフリストを各アカウントのファイルに保持します。[project でサーバーをオフにする](#turn-off-a-global-server-in-one-project)と、そのサーバーを持つすべてのアカウントにスイッチが書き込まれます。`mcp import --from claude-work` とダッシュボードの Import from target は、そのアカウント自身のファイルを読み込みます。`mcp import --file <path> --from claude-work` は、自分でエクスポートしたファイルを、そのアカウントの Agent のフォーマットとして読み込みます。

## 1 つの project だけで global サーバーをオフにする {#turn-off-a-global-server-in-one-project}

Agent は自身の global MCP ファイルと project のファイルを合わせて読み込みます。そのため、global
ファイルで定義されたサーバーはすべての project で読み込まれます。1 つの project だけでそれを読み込まれないようにするには、**Agent の global ファイルが使っているのと同じ名前**のエントリを追加し、
`disabled` を指定します。

これは以下の 4 つのクライアントでのみ機能します。

| クライアント | 対応 | Skillshare が書き込む内容 |
|---|---|---|
| Claude Code | Yes | `~/.claude.json`: この project の `disabledMcpServers` リストにその名前を追加 |
| OpenCode | Yes | `opencode.json`: `"NAME": {"enabled": false}` |
| Kilo Code | Yes | `kilo.jsonc`: `"NAME": {"enabled": false}` |
| Pi | Yes（`mcp.projects` から） | `.pi/mcp.json`: `"NAME": {"command": "...", "enabled": false}`、下記参照 |
| Codex | No | 下記参照 |
| その他すべてのクライアント | No | 選択するとエラー。何も書き込まれない |

書き込まれるのはスイッチのみです。Agent は global エントリのコマンドまたは URL をそのまま保持します。他のクライアントが拒否されるのは、global エントリ全体を project 側のものに置き換えてしまうか、project ファイルを持たないため、スイッチだけを書き込むとオフにするどころかサーバーを壊してしまうからです。

Codex が拒否されるのは別の理由によります。Codex は `.codex/config.toml` を global ファイルの上にフィールド単位でマージするため、
global config がそのサーバーを定義しているマシンでは `enabled = false` 単体でも機能します。しかしそれを定義していないマシンでは、マージされたエントリに
`command` も `url` もなくなり、Codex は `invalid transport` で設定全体の読み込みに失敗します。`.codex/config.toml` は通常コミットされるため、あるチームメンバーのスイッチが
別のメンバーの Codex の起動を止めてしまう可能性があります。代わりに、マシンごとに `~/.codex/config.toml` で
`enabled = false` を指定してサーバーをオフにしてください。

Pi は project の同名エントリで global エントリを丸ごと置き換え、`command` も `url` もないエントリは読み飛ばします。
そのため Pi には、global サーバーの `command`、またはクエリを除いた `url` を `enabled: false` と一緒に書き込みます。
オフにしたサーバーは起動しないので、args、env、headers は project ファイルに書き込まれず、他の project では
そのサーバーがそのまま使われます。sync のたびにエントリは global サーバーから書き直されます。global サーバーが
必要なので、これは global config の `mcp.projects` 配下の project でのみ機能します。project 自身の config からは
global サーバーが見えないため、そこで `disabled` エントリに `pi` を指定するとエラーになります。

### OpenCode と Kilo Code

```bash
cd my-project
skillshare mcp add company-docs --disabled --target opencode --target kilocode
skillshare sync mcp
```

```yaml
# .skillshare/config.yaml
mcp:
  servers:
    company-docs:
      disabled: true
      targets: [opencode, kilocode]
```

### Claude Code

Claude Code は 1 つのスコープからサーバーエントリ全体を取得し、フィールドをマージすることは決してないため、`.mcp.json` 内のスイッチはサーバーをオフにするのではなく置き換えてしまいます。Claude Code は
`/mcp` パネルが編集するのと同じ、project ごとの独自のオフリストを `~/.claude.json` に保持しています。
Skillshare はこの project の絶対パスの下にその名前を追加し、`.mcp.json` には何も書き込みません。

```bash
skillshare mcp add company-docs --disabled --target claude
skillshare sync mcp
```

- このリストはリポジトリではなく自分のマシン上に存在します。各チームメンバーは自分のチェックアウトで一度
  `skillshare sync mcp` を実行してください。
- `/mcp` 内で自分でオフにした名前は、決して奪われたり削除されたりしません。
- `/mcp` でサーバーを再度オンにすると、次の sync は競合を報告します。
  `.skillshare/config.yaml` からそのエントリを削除するか、再度オフにするために replace してください。
- このリストは project のパスをキーにしているため、project を移動すると新しい sync が必要になります。

### ルール

- **project がスコープ内にある必要があります。** `.skillshare/config.yaml` を持つ project 内で実行するか
  （`skillshare init -p` で作成）、`-p` を渡すか、
  [`mcp.projects`](#manage-several-projects-from-the-global-config) 内の project root の下にエントリを
  置いてください。project がスコープ内にない global の `mcp.servers` では拒否されます。
- **`disabled` は単独で指定します。** このエントリが取れるのは `targets` のみです。`command`、`url`、
  `env`、`headers`、`piOptions`、`tools` を追加するとエラーになります。
- **`targets` は省略できます。** その場合、エントリは project の target に従います。sync のたびに、
  project が使うクライアントのうち、project ごとのスイッチを持つものに書き込まれます。Skillshare が同名の
  global サーバーも把握している `mcp.projects` 配下では、そのサーバーの書き込み先クライアントにさらに
  絞り込まれます。後から project の target を変更しても、
  エントリの編集は不要です。自分で決めたい場合は `targets` を列挙してください。そのリスト内の
  非対応クライアントはエラーになります。
- **名前は一致している必要があります。** Skillshare は Agent の global ファイルを読み込まないため、
  この名前のサーバーがそこに存在するかを確認できません。何にも一致しない名前は無害です。Agent はそれを無視します。
- **再びオンにするには**、エントリを削除し（`skillshare mcp remove company-docs`）
  sync してください。スイッチは、それが書き込まれたファイル（project 自身のファイル、または
  Claude Code の場合は `~/.claude.json`）から削除されます。
- **Skillshare 自身が定義したサーバーにはこれは不要です。** 代わりにそのサーバーで Agent の選択を外せば、
  次の sync でそのエントリが削除されます。

ダッシュボードでは、**グローバルサーバーをオフにする** ボタンがこれに当たります。project mode では **サーバー** 見出しの横に、project の MCP タブでは **サーバーを追加** の横にあります。

## global config から複数の project を管理する {#manage-several-projects-from-the-global-config}

project mode では、各 project の MCP 設定をその project の `.skillshare/config.yaml` に保持し、
そのフォルダー内から sync します。すべての project を 1 か所で管理したい場合は、**global** config の
`mcp.projects` の下に project フォルダーを列挙してください。そうすれば、どこからでも `skillshare sync mcp` を
1 回実行するだけで、global のファイルとすべての project のファイルが 1 つのプランでまとめて書き込まれます。

```yaml
# ~/.config/skillshare/config.yaml
mcp:
  servers:
    context7:
      command: npx
      args: ["-y", "@upstash/context7-mcp"]
      targets: [claude, opencode]
  projects:
    ~/work/project01:
      targets: [claude, opencode]
      servers:
        context7:                  # この project だけでオフ
          disabled: true
    ~/work/project02:
      servers:
        internal-docs:             # この project にだけ存在する
          url: https://example.com/mcp
          targets: [opencode]
```

project には、global config と異なる部分だけを書きます。`context7` のような global サーバーは、ここに
エントリを書く必要がありません。Agent は自身の global ファイルと project のファイルを合わせて読み込むため、
すでにすべての project で読み込まれます。`disabled` エントリは、そこに列挙したすべての
クライアントについて、[そのフォルダーでそのサーバーをオフにします](#turn-off-a-global-server-in-one-project)。

各キーは project フォルダーで、絶対パスか `~` で始まるパスを指定します。その下には、その project 自身の
`config.yaml` が `mcp` の下に持つのと同じ `targets` と `servers` を書き、それらは同じ
[project ファイル](#native-destinations)に書き込まれます。`targets` を持たない project は、global の
`mcp.targets` を継承します。

1 つのサーバーが複数の場所に現れる場合、プレビューにはファイル名が表示されます。

```text
context7     add          opencode (~/.config/opencode/opencode.json)
context7     add          opencode (~/work/project01/opencode.json)
```

リストから project を削除すると、サーバーを削除した場合と同様に、Skillshare がそこに書き込んだエントリが
次の sync で削除されます。

複数の project に同じサーバーを持たせるには、YAML アンカーで一度だけ定義して再利用します。

```yaml
mcp:
  projects:
    ~/work/project01:
      servers:
        internal-docs: &internal-docs
          url: https://example.com/mcp
          targets: [opencode]
    ~/work/project02:
      servers:
        internal-docs: *internal-docs
```

アンカーは `mcp.projects` の中に置いてください。`mcp.servers` 上のアンカーへのエイリアスも機能しますが、
`skillshare mcp add` とダッシュボードは `mcp.servers` を書き換えます。保存時には、ファイルが有効なままになるよう
そうしたエイリアスを完全な形に展開して書き出すため、それ以降は global サーバーへの編集に追従しなくなります。

### ダッシュボードでの project {#projects-in-the-dashboard}

global mode では、ダッシュボードに **プロジェクト** ページがあります。[`projects`](/docs/reference/targets/configuration#projects) と
`mcp.projects` の下にあるすべてのフォルダーが一覧され、各 project には **MCP** タブがあります。

![プロジェクトの MCP タブ: プロジェクトごとに切り替えるグローバル server と、プロジェクト専用の server](/img/projects-mcp-tab.png)

- **プロジェクトを追加** では、フォルダーとその target を指定します。**MCP** にチェックを入れると、そのフォルダーは
  `mcp.projects` にも一覧されます。
- **MCP** タブには、すべての global サーバーがスイッチ付きで一覧表示されます。オフにすると、`targets` を
  持たない `disabled` エントリが保存されるため、[上記](#turn-off-a-global-server-in-one-project)のとおり
  project の target に従います。オンに戻すとそのエントリは削除されます。
  その下には、その project にのみ存在するサーバーが並びます。
- オフになっているサーバーには、オフになっている Agent のロゴが表示されます。project の Agent の中に
  project ごとのスイッチを持たないものがある場合、その行にはサーバーがそこでは引き続き読み込まれることが
  表示されます。project のものとは異なる独自の `targets` を持つエントリには **プロジェクトに合わせる** が
  表示され、`targets` なしでそのエントリを保存し直します。
- タブの Sync ボックスにある **Sync MCP** は MCP の計画全体を書き込み、そのうち何件の変更がこの project の外に
  あるかを表示します。project ページの上部にある **Sync project** は、この project の skills、agents、MCP だけを
  書き込みます。
- MCP ページの一番下にある **デフォルト** では、`mcp.targets` を編集します。
- project 自身の Agent ファイルに Skillshare が管理していないサーバーがある場合、タブの一覧の上にそのことが
  **Import** 付きで表示されます。[下記](#unmanaged-servers)を参照してください。

保存時に書き換えられるのは、変更した project だけです。他の project は、アンカーやエイリアスも含めて
YAML が書かれたまま保持され、`~/work/app` と書かれたフォルダーは `~` のまま残ります。このページの他の箇所と同様に、
保存で変更されるのは `config.yaml` だけで、ファイルを書き込むのは Sync です。

制限事項:

- `mcp.projects` は global config からのみ読み込まれます。これを含む project config は拒否されます。
- これを編集するコマンドはありません。`skillshare mcp add` は `mcp.servers` を管理し、
  `mcp.projects` は書かれたままにします。編集は `config.yaml`、または
  [ダッシュボード](#projects-in-the-dashboard)で行ってください。
- Claude Code 向けの `disabled` エントリは、global サーバーが書き込まれるのと同じファイルである
  `~/.claude.json` に書き込まれます。Claude Code は project ごとのオフリストをそこに保持しているためです。
  サーバー自体はそのままにされます。
- フォルダーが同じエントリを管理する独自の `.skillshare/config.yaml` も持っている場合、プランは上書きせずに
  競合を報告します。

## Agent が起動する前にサーバーをチェックする {#check-servers-before-an-agent-starts-them}

```bash
skillshare mcp check
skillshare mcp check docs github --json
skillshare mcp check --no-dns
```

`mcp check` は、source 内のすべてのサーバー、または指定したサーバーについて「sync したとおりに
動作するか」を確認します。global config では、
[`mcp.projects`](#manage-several-projects-from-the-global-config) の下にある各ルートのサーバーもチェックし、
各 Agent のルールと sync 状態はそのルートについて読み込みます。読み取り専用で、サーバーの起動、
HTTP リクエストの送信、コマンドの実行、ファイルの書き込みは一切行いません。ただし
[`--live`](#probe-servers-live) を付けた場合は除きます。

| チェック内容 | レベル |
|---|---|
| `env`、`headers`、`bearerToken` 内の `fromEnv` 変数が未設定または空 | error |
| ローカルサーバーの `command` が `PATH` 上に見つからない(先頭の `~/` は展開される) | error |
| リモートサーバーのホストが DNS で解決できない(制限は 3 秒。`--no-dns` でスキップ) | warning |
| Agent のルールがサーバーを拒否する(Claude Code が予約している名前など) | error |
| Agent のエントリが source と競合する(`sync mcp --dry-run` と同様) | error |
| Agent のエントリがまだ書き込まれていない、または更新されていない | warning |
| サーバーが `targets: []` を持ち、Skillshare 内にのみ保持されている | info |
| 選択された Agent がサーバーの[ツールポリシー](#tool-policy)の一部を保持できない | warning |

変数の値は表示されません。error が 1 件でもあれば終了コードは 1、それ以外は 0 です。warning で
失敗することはありません。不明なサーバー名は error となり、既知の名前が一覧表示されます。
名前を指定すると、global と各 project の両方で、その名前を持つすべてのサーバーが選択されます。
既知の名前には project のサーバーも含まれます。

ターミナルでは、project のサーバーの見出しにその project が表示されます:

```text
✓ docs
  · claude: in sync
✗ docs  (project ~/work/app)
  ✗ command no-such-mcp-binary was not found on PATH
  ! claude: not synced yet; run skillshare sync mcp
```

`--json` を指定すると、レポートは次の形式になります:

```json
{
  "servers": [
    {
      "name": "docs",
      "ok": false,
      "findings": [
        { "level": "error", "check": "env", "target": "", "message": "bearerToken reads DOCS_TOKEN, which is not set", "subject": "DOCS_TOKEN" },
        { "level": "warning", "check": "sync", "target": "claude", "message": "not synced yet; run skillshare sync mcp" }
      ]
    },
    {
      "name": "docs",
      "project": "/home/me/work/app",
      "ok": true,
      "findings": [
        { "level": "info", "check": "sync", "target": "claude", "message": "in sync" }
      ]
    }
  ],
  "summary": { "errors": 1, "warnings": 1 }
}
```

`check` は `env`、`command`、`url`、`dns`、`client-rule`、`sync`、`targets`、`tools`、`live` のいずれかです。
`target` は Agent またはアカウントを示し、指摘がサーバー自体に関するものである場合は空になります。
`subject` は `env`、`command`、`dns` の指摘では変数、コマンド、ホストを、成功した `live` プローブでは
サーバーが報告した名前を、`live` のサインイン warning ではリソースメタデータ URL を示し、それ以外では省略されます。
`project` はサーバーの `mcp.projects` ルートを絶対パスで示し(先頭の `~` は展開される)、global サーバーでは
省略されます。`summary` は、project のサーバーも含め、レポート内のすべてのサーバーを集計します。

ダッシュボードでは、MCP ページの Sync ボックスにある **チェック** ボタンで同じチェックを実行します。チェックする
サーバーがあるときに表示され、クリックしたときにだけ実行され、サーバー一覧の上に概要を、各サーバーの下にそれぞれの
error や warning を表示し、ページを再読み込みすると何も残りません。MCP ページには global サーバーしか一覧表示
されないため、その概要と各行には project のサーバーは含まれません。global サーバーと同じ名前のものも同様です。
project の MCP タブには Sync ボックスに専用の **チェック** があり、その project 自身のサーバーを報告します。
変数は `skillshare ui` を起動したターミナルから読み込まれます。

### サーバーをライブでプローブする {#probe-servers-live}

```bash
skillshare mcp check --live
skillshare mcp check docs --live --timeout 30s --json
```

`--live` はまず静的チェックを実行し、その後 error のない選択された各サーバーに接続します。error のある
サーバーや無効化されたエントリには接続せず、その理由を `info` の指摘で示します。

- **ローカル (stdio) サーバー。** Skillshare は現在の環境で `command` を `args` 付きで起動し、サーバーの
  `env` を加えます。各 `fromEnv` の値はシェルから読み込まれます。project のサーバーはその project フォルダーで、
  global サーバーはカレントディレクトリで起動します。これは Agent と同じようにサーバーのコードをあなたの
  マシン上で実行するため、`--live` は信頼できるサーバーにだけ使ってください。Skillshare は `server/discover` を
  送信します。MCP プロトコルエラーではない error を返すサーバーや、タイムアウトの 3 分の 1 以内に応答しない
  サーバーは MCP 2026-07-28 より古いものとみなされ、代わりに `initialize` ハンドシェイクが使われます。
  その後 Skillshare は `tools/list` を呼び出してツール数を数え、サーバーを停止します。stdin を閉じ、次に
  サーバーのプロセスグループへ SIGTERM、さらに SIGKILL を送ります。Windows ではプロセスを終了させます。
- **リモート (Streamable HTTP) サーバー。** Skillshare はサーバーの `headers` と `bearerToken` を付けて
  `server/discover` を POST し、JSON または SSE のレスポンスを読みます。MCP エラーを伴わない `400`、`404`、
  `405` の場合は `initialize` にフォールバックします。`401` は warning「sign-in required」となり、
  `WWW-Authenticate` ヘッダーから得たリソースメタデータ URL が示されます。Skillshare がサインインしたり
  OAuth を開始したりすることはありません。

各サーバーには、プローブ全体に対して 1 つの制限時間があります。10 秒、または `--timeout` で指定した値
(`30s` や `1m` など) です。同時にプローブするサーバーは最大 4 つです。`--live` なしの `--timeout` は
error になります。

| 結果 | レベル |
|---|---|
| サーバーが応答した: その名前とバージョン、プロトコルバージョン、ツール数 | info |
| リモートサーバーがサインインを必要とする (HTTP 401) | warning |
| コマンドを起動できなかった、早期に終了した、または時間内に応答しなかった | error |
| プロトコルエラー、未対応のプロトコルバージョン、またはその他の HTTP ステータス | error |

ローカルサーバーが失敗した場合、メッセージの末尾にその stderr が最大 5 行付きます。`env`、`headers`、
`bearerToken` の値はすべてのメッセージから取り除かれます。4 文字未満の値はそのまま残ります。値は書かれた
とおりに渡されます。Skillshare は Pi の `!command` の値を実行せず、`piOptions` も読みません。終了コードは
同じ規則に従い、error が 1 件でもあれば 1 です。`--live` はファイルも操作ログのエントリも書き込みません。

`--json` を指定すると、応答したサーバーには `live` オブジェクトも付きます:

```json
{
  "name": "docs",
  "ok": true,
  "findings": [
    { "level": "info", "check": "live", "target": "", "message": "responds: docs-server 1.4.0, protocol 2026-07-28, 12 tool(s)", "subject": "docs-server" }
  ],
  "live": { "protocolVersion": "2026-07-28", "serverInfo": { "name": "docs-server", "version": "1.4.0" }, "tools": 12 }
}
```

`serverInfo` はサーバーが自己申告した内容で、何も検証されていません。サーバーをプローブしなかった場合や
プローブが失敗した場合、`live` は省略されます。

ダッシュボードの **チェック** ボタンは静的チェックだけを実行します。ダッシュボードがサーバーをプローブするのは
1 か所だけで、サーバーダイアログの[ツールセクション](#tool-policy-dashboard)にある **ツールを読み込む** です。
これはダイアログの現在の入力内容でサーバーを一度起動してツールを一覧にします。`--json` では、`live` に `toolNames`
（`tools/list` が返した名前）も含まれます。

## サーバーの管理をやめる {#stop-managing-a-server}

```bash
skillshare mcp remove docs --keep-files
```

これは source から `docs` を削除し、Skillshare がそのために書き込んだ Agent のエントリの記録を消去します。
Agent ファイルは変更されません。以降、それらのエントリはあなたのものになり、sync は削除も更新もしません。
`--keep-files` は `--sync` と併用できません。ターミナルの削除ウィザードでは **Stop managing** として表示され、
ダッシュボードの削除ダイアログでも、MCP ページと project の **MCP** タブの両方で同じ選択肢が表示されます。

変更されるのは削除したスコープだけです。global サーバーの管理をやめても、同じ名前の project のサーバーは
管理されたままで、その逆も同様です。エントリを再び管理するには、それをインポートしてください。

## Skillshare が管理していないサーバー {#unmanaged-servers}

ダッシュボードは、現在のスコープと `mcp.projects` の下にあるすべてのフォルダーの Agent 設定ファイルを読み、
この source が定義しておらず、どの Skillshare 設定も管理していないサーバーを探します。見つかった場合は、
サーバー一覧の上に、その件数とどの Agent にあるかが表示されます。**Import** は、それらの Agent のうち
最初のものを選択した状態でインポートを開きます。project の **MCP** タブでは、その project 自身のファイルに
ついて同じ表示が出ます。そのインポートは project のファイルを読み、サーバーをその project に保存します。
Goose の組み込み拡張機能のように接続先を持たないエントリは数に含まれません。

### Agent がすでに持っているエントリを引き継ぐ

Agent ファイルがすでに使っている名前でサーバーを追加しても、sync はそのエントリを上書きしません。
プランは競合 `existing entry is not managed` を報告し、そのエントリについて次のどちらかを選ぶまで
ファイルを書き込みません。

- その Agent からインポートする: `skillshare mcp import NAME --from CLIENT`、またはダッシュボードの
  競合にある **Import from** ボタン（**Import from Cursor** など）。source と一致するエントリはそのまま
  採用されます。`mcp.projects` 配下のフォルダーでの競合では、ボタンはそのフォルダーのファイルを読み込み、
  その project にインポートします。
- source の定義で置き換える: ダッシュボードの **Replace with source**、またはインポート時の `--replace`。

## 安全性と制限事項

- JSONC のコメントと無関係な設定は保持されます。変更された所有エントリは
  ひとまとまりとして置き換えられるため、それらのエントリ内のコメントは変わる可能性があります。比較・置換の対象となるのは
  Skillshare が書き込むフィールドのみで、タイムアウトなどの Agent 固有のフィールドは保持されます。
  `"type": "stdio"`、空の `env`、ヘッダー名の大文字小文字などの Agent が補完するデフォルト値は
  変更とはみなされません。管理下のサーバーを `enabled: false` や `disabled: true` でオフにすることは
  競合として報告されます。
  Pi は例外です。`enabled` だけの変更は所有権の競合になりません。同期時には source の `piOptions.enabled` が優先されます。
- Claude Code が `~/.claude.json` で行うように、Agent が同じファイル内の無関係な設定を書き換えている間も、プレビューは有効なままです。その
  ファイルの MCP エントリへの変更のみが新しいプレビューを必要とします。
- Codex と Grok の編集は、通常の `[mcp_servers.NAME]` テーブルとそのサブテーブルに対応しています。
  更新されたエントリはその場に留まり、CRLF の改行も保持されます。
  インライン/ドット記法の MCP 定義はテーブルに変換してから書き込む必要があり、
  変換されていない場合はファイルを変更せずに拒否されます。
- ネイティブファイルの symlink、不正な形式のファイル、重複した JSON プロパティは
  書き込みをブロックします。Symlink された Skillshare の `config.yaml` は、そのターゲットへ書き込まれます。ファイルパーミッションは保持されます。新規のネイティブ
  ファイル、所有権の記録、バックアップにはプライベートなパーミッションが使われます。
- すでに source と一致しているエントリは、例えばチームメンバーの変更を pull した後などに、
  書き込みなしで unchanged として報告されます。この config がそれ以前にそのエントリを管理していなかった場合、
  例えばある Agent からインポートした後でその Agent にチェックを入れた場合などでは、プランに `adopt` が表示されます。
  sync はファイルを変更せずにそのエントリを管理対象として記録し、以後サーバーを削除するか
  その Agent のチェックを外すとエントリも削除されます。Agent 自身でオフにしたサーバーは
  引き続きユーザーのものです。別の
  管理外エントリには、インポートまたは明示的なエントリ単位の replace が必要です。別の
  Skillshare config の所有権は、その config ファイルがまだ存在する限り上書きできません。そのファイルがなくなっている場合、それは決して
  そのエントリを解放しないため、競合はそのエントリが取り残されていることを伝え、該当するファイルの名前を示します。そして
  ターミナルでも、ダッシュボードのその競合からでも、明示的なインポートまたは replace によってそれを引き継げます。
  マウントされていないドライブ上のファイルなど、読み取れないだけのファイルは、所有者がまだ存在しているものとみなされます。
- ダッシュボードの MCP 設定は、ブラウザがダッシュボードを `localhost` または IP アドレスで開いている場合にのみ機能します。ドメイン名経由（reverse proxy を含む）では、MCP リクエストは 403 を返します。
  なぜなら DNS rebinding 攻撃は常にドメイン名を使うからです。
- 認証情報は環境変数参照を使用します。secret ストア、OAuth セッション同期、
  継続的なヘルスモニタリング、package のインストール、gateway、レジストリ、plugin の同期はありません。
  サーバーを起動したり呼び出したりするコマンドは `mcp check --live` だけです。
- VS Code Insiders、カスタムプロファイル、リモート workspace、レガシー SSE は、このバージョンでは
  対応していません。
- VS Code は現在、`headers` 内で `${env:VARIABLE}` を置換しません
  （[microsoft/vscode#336232](https://github.com/microsoft/vscode/issues/336232)）。
  そのため、VS Code に同期されたヘッダーと `bearerToken` の参照は、それが修正されるまで解決されないままサーバーに届きます。
- ローカルの操作記録は Skillshare の state ディレクトリの `mcp/` 配下にあります。
  `state.json`、書き込み中の `pending.json`、そして `backups/`（Agent ファイルごとに最新 20 件）です。この
  ディレクトリをポータブルなマニフェストとして共有しないでください。


## ツールポリシー {#tool-policy}

`tools` は、サーバーのどのツールをモデルに渡すかを指定します。サーバーに一度書くだけで、
Skillshare が sync 時に各 Agent 独自のフィールドへ変換します。

```yaml
mcp:
  servers:
    github:
      command: github-mcp
      targets: [pi, codex, copilot, opencode]
      tools:
        allow: [get_*, search_code, list_issues]
        deny: [get_secret]
```

```bash
skillshare mcp add github --target pi --target codex --tools-allow 'get_*,search_code' --tools-deny get_secret -- github-mcp
skillshare mcp edit github --tools-allow ''          # clear the allow list
skillshare mcp import github --from claude --target pi --tools-deny get_secret
```

| フィールド | 意味 |
|---|---|
| `allow` | 設定すると、一致するツールだけが残る |
| `deny` | 一致するツールを除外する。`allow` に一致していても除外される |

`allow` と `deny` のエントリはツール名で、`*` は任意の文字に一致します。その他のワイルドカード
（`? [ ] { }`）、スペース、カンマは拒否され、同じ名前を 2 回書いた場合も拒否されます。
`allow` で残したツールを `deny` がすべて除外する場合はエラーです。`disabled` エントリには `tools` を
設定できません。2 つのフラグは `mcp add`、`mcp edit`、`mcp import` で使えます。リストはカンマ区切りで、
空の値を渡すとその部分がクリアされます。Pi がツールをどう提供するかはポリシーに含まれません。それは
Pi の `exposure` で、[`piOptions`](#pi-options) で設定します。

### 各 Agent が受け取る内容 {#tool-policy-agents}

すべての Agent がポリシーのすべての部分を保持できるわけではありません。Skillshare は Agent の
ドキュメント化された形式が対応する部分だけを書き込み、残りを明示します。黙って部分を落とすことはありません。

| Agent | 書き込まれる内容 | 適用されない部分 |
|---|---|---|
| [Pi](https://github.com/earendil-works/pi/blob/v0.99.0/packages/coding-agent/docs/mcp.md) | `toolExposure` には拒否したツールを `hidden` で、続いて許可したツールを、`allow` が設定されていれば最後に `"*": "hidden"` を書く | なし |
| [Codex](https://developers.openai.com/codex/config-reference) | `enabled_tools` と `disabled_tools`（完全一致の名前のみ）。Codex は `enabled_tools` の後に `disabled_tools` を適用する | `allow` 内の `*` パターン、完全一致の `allow` リストに畳み込めない `deny` 内の `*` パターン |
| [Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-mcp-servers) | `tools`: 許可した完全一致の名前から拒否した名前を除いたもの。それがなければ `["*"]` | `allow` 内の `*` パターン、`allow` が完全一致の名前を列挙していない場合の `deny`（Copilot には拒否リストがないため） |
| [OpenCode](https://opencode.ai/docs/permissions/)、[Kilo Code](https://kilo.ai/docs/code-with-ai/platforms/cli#permissions) | なし | すべて。どちらもツールを絞り込めるのは `<server>_<tool>` をキーとするトップレベルの `permission` マップだけで、これはサーバーのエントリの外にある |
| その他すべての Agent | なし | すべて |

Pi では完全一致のツール名がどのパターンよりも優先されるため、拒否したパターンに一致する許可済みの
完全一致名は `toolExposure` から除外されます。許可したツールにはサーバーの `piOptions.exposure` が、それが
未設定または `hidden` の場合は Pi の既定の `codemode` が設定されます。そのため `allow` と併せた
`hidden` は、許可したツールだけが見えることを意味します。

適用されない部分は 3 か所に表示されます。

- sync のプランに、Agent ごとに該当サーバーを列挙した warning 行として:

  ```text
  ! tool policy not applied for opencode: allow, deny (github)
  ```

  `--json` では、同じテキストがプランの `notices` に入ります。
- [`mcp check`](#check-servers-before-an-agent-starts-them) で、Agent ごとの `tools` warning として。
- ダッシュボードで、サーバーダイアログのツールセクションと **各 Agent に書き込まれる設定を表示** に。
  ダッシュボードは、これらについても下記の廃止された Pi 設定についても、ページ全体の通知は表示しません。

Codex の `enabled_tools` と `disabled_tools` は管理対象のフィールドです。ポリシーをクリアすると削除され、
Skillshare が所有するエントリでこれらを手で編集すると競合として表示されます。インポートでは Codex の
`enabled_tools`/`disabled_tools` と Copilot の `tools` を `tools` に読み戻します。Pi の
`toolExposure` が `tools` になるのは、サーバーの `exposure` と併せてそのポリシーを書き込んだときにまったく同じ
`toolExposure` になる場合だけです。そうでなければ warning 付きで `piOptions` に残ります。`exposure` は常に
`piOptions` に残ります。

### ダッシュボードのツール {#tool-policy-dashboard}

サーバーダイアログには、target の後に **ツール** セクションがあります（`disabled` エントリを除くすべての
サーバー）。このセクションは常に表示されます。見出しの横の情報アイコンがセクションを説明し、
概要には `すべてのツール`、ポリシーの内容（`1 個のみ許可、2 個を除外` など）、またはツールを読み込んだ後の
`9 / 14 選択` が表示されます。

- 見出しの下の枠にツール一覧が入ります。読み込む前は **ツールを読み込む** があり、保存済みかどうかに
  関係なくダイアログの現在の設定でサーバーを一度起動します（[`mcp check --live`](#probe-servers-live) と
  同じプローブ）。クリックしたときだけ実行され、何も保存しないので、新しいサーバーでも使えます。失敗したときは
  理由をわかりやすく表示し、元のエラーは横の情報アイコンのツールチップにあり、ボタンは **再試行** に変わります。
  その後コマンド、URL、関連する設定を変えると、読み込んだ一覧は消えます。
- 読み込むと各ツールにチェックボックスが付き、チェックしたツールだけがモデルに渡ります。チェックを外すと
  そのツールの完全な名前が `deny` に追加されます。チェックし直すとその名前が `deny` から外れ、空でない
  `allow` がまだ除外している場合は `allow` に名前が追加されます。`deny` のパターンで除外されたツールは
  チェックできず、ツールチップがそのルールを示します。検索ボックスで一覧を絞り込め、**すべて選択** と
  **すべて解除** は表示中の行だけに作用し、更新ボタンで一覧を読み込み直します。
- 枠の下部にある **除外ルール** の行には、`*` パターンやサーバーが一覧に出さない名前を入力します。入力して
  Enter を押します。`allow` に項目があるときは、その上に同じ使い方の **許可のみ** の行があります。ツールを
  読み込む前は、保存済みの項目がすべてここに表示されます。不正な名前や、許可したツールをすべて除外する
  拒否リストはダイアログに表示され、**保存** をブロックします。
- その下には、選択中の各 Agent が実際に何を受け取るかが表示されます。この一覧どおりに提供する Agent、
  一部だけ適用する Agent の動作（たとえば Copilot CLI は拒否リストがないため、チェックを外したツールも提供します）、
  そして絞り込みに対応していない Agent です。

サーバーの行にはポリシーを言葉で示すタグ（`ツール: 2 個のツールを除外` など）が表示され、**各 Agent に書き込まれる設定を表示** は、適用されない部分に
ついて Agent ごとに警告します。

## Pi {#pi}

Pi ≥ 0.99.0 は [MCP を内蔵](https://github.com/earendil-works/pi/blob/v0.99.0/packages/coding-agent/docs/mcp.md)しており、
Skillshare が Pi に MCP サーバーを書き込む方法はこれだけです。サードパーティの `pi-mcp-adapter` と
`pi-mcp-extension` は、sync の送信先としてはサポートされなくなりました。

| スコープ | ファイル |
|---|---|
| Global | `~/.pi/agent/mcp.json`（`PI_CODING_AGENT_DIR` に従う） |
| Project | `.pi/mcp.json` |

個人用や認証情報を持つサーバーは `~/.pi/agent/mcp.json` に配置してください。`.pi/mcp.json` は信頼済みプロジェクトが必要とするサーバーだけに使用します。同名の project entry は global entry 全体を置き換えます。Skillshare はプレビューとバックアップ付きでファイルを編集します。信頼の承認、サーバー起動、拡張のインストール、OAuth 認可は行いません。

```bash
skillshare mcp add docs --url https://example.com/mcp --target pi --tools-deny 'delete_*' --pi-options '{"exposure":"deferred","timeout":120}' --no-tui
skillshare sync mcp --dry-run
skillshare sync mcp
```

```yaml
mcp:
  servers:
    docs:
      url: https://example.com/mcp
      targets: [pi]
      tools:
        deny: [delete_*]
      piOptions:
        exposure: deferred
        timeout: 120
```

ネイティブ出力は `command`/`args` または `url` を使い、環境変数は `${NAME}` 参照になります。
sync 後は `/reload` を実行するか新しい Pi セッションを開始し、`/mcp` で接続を確認して OAuth を認可します。
Pi だけの簡単な設定なら、`pi mcp add` でグローバルファイルを編集できます。`-l` を付けると project ファイルに
書き込みます。`pi mcp list` はすべての有効なサーバーを起動して接続を確認し、`pi mcp login NAME` は
ユーザーの承認が必要です。

Pi のサーバー名には英数字、`_`、`-` のみを使えます。`-` と `_` だけが異なる名前は Pi では同じサーバーとして扱われるため、sync は 2 つ目を拒否します。Pi では project のエントリが同名の global
エントリを丸ごと置き換えます。1 つの project で global サーバーをオフにするには、
[1 つの project だけで global サーバーをオフにする](#turn-off-a-global-server-in-one-project)を参照してください。

### その他の Pi 設定 {#pi-options}

`piOptions` は、Pi の内蔵 MCP のその他のサーバー別フィールドを保持します。受け取るのは Pi だけです。

- `exposure` は `codemode`（Pi の既定）、`codemode-deferred`（`codemode` の旧名）、`deferred`、`direct`、`hidden` を
  受け付けます。`toolExposure` はツール名またはワイルドカードパターンをこれらの値のいずれかに対応付けます。
  完全一致の名前が優先され、次に最初に一致したパターンが採用されます。Skillshare はインポートと
  JSON／YAML 変換を通じてパターンの順序を保持します。`exposure` は、[`tools`](#tool-policy) の許可リストが
  残したツールの提供方法も決めます。`toolExposure` より `tools` を使うほうがよいでしょう。他の Agent にも
  届くためです。1 つのサーバーで `tools` と `toolExposure` を両方設定することはできません。
- `timeout`（正の秒数）、`cwd`、`enabled`、`oauth`、`auth` は検証されます。`description` などの未知の
  フィールドはそのまま渡されます。
- `auth: {provider: NAME}` は、そのプロバイダーの `/login` トークンを bearer トークンとして送ります。https の
  `url`（localhost なら http も可）が必要で、Pi は global ファイルからしか読まないため global モードでのみ使えます。
- `oauth.authServerMetadataUrl`（Pi 1.0 以降）は https（localhost なら http も可）が必要です。Pi は検出の代わりに
  このドキュメントを信頼するためです。Pi 1.0 は OAuth のサインインを server 名と URL ごとに保存するので、
  server の名前や `url` を変えた後は Pi で再度サインインしてください。
- 接続フィールドはメインフォームに入力します。`directTools`、`includeTools`、`excludeTools` など
  `pi-mcp-adapter` の設定は、Pi の内蔵 MCP が読まないため拒否されます。代わりに `tools` を使ってください。
- トップレベルの `settings` と `autoEnableCodemode` はサーバーのオプションではありません。Pi で直接
  編集してください。sync はそれらを保持します。
- 認証情報は環境変数参照に置いてください。ポータブルな env/headers 内の `!command` リテラルは拒否され、
  `piOptions` 内のコマンド値も、`oauth.clientId` のような秘密でないフィールドを含めてすべて拒否されます。

JSON をクリアしたりフィールドを削除したりすると、Skillshare が書き込み、その後変更されていないフィールドは
次の sync で Pi のファイルから削除されます。Pi で自分で追加したフィールドは残ります。Skillshare が
書き込んだ後に Pi で変更されたフィールドは、インポートするまで sync をブロックします。

```bash
skillshare mcp edit docs --pi-options '{"timeout":60}' --no-tui
skillshare mcp edit docs --pi-options '{}' --no-tui
```

ダッシュボードでは、サーバーダイアログの Pi ブロックに **ツール公開モード** と **その他の Pi 設定** があります。
**Pi の設定** と **ツール公開モード** の横にある情報アイコンがそれぞれを説明し、**Pi の設定** の横の
リンクから Pi の MCP ドキュメントを開けます。ダイアログは保存前に **その他の Pi 設定** 内の
`pi-mcp-adapter` のフィールドを指摘します。ツールセクションに設定がある間も **ツール公開モード** は
編集できます。そのとき拒否されるのは **その他の Pi 設定** 内の `toolExposure` だけで、`tools` が書き込むため
です。サーバー行の Pi チップは公開モードを短い言葉で表示します（`codemode` なら `コード経由`）。

### 0.22 からの Pi のアップグレード {#pi-migration}

アップグレード後の最初の sync の前に、Pi で次の 2 点を確認してください。

- **Pi 0.99.0 以降であること。** Skillshare は Pi のサーバーを `mcp.json` にだけ書き込み、
  Pi は 0.99.0 で追加された内蔵 MCP でこれを読みます。古い Pi はこのファイルを読まないため、
  Pi を更新するまでこれらのサーバーは読み込まれません。Skillshare は Pi のバージョンを確認しません。
- **`pi-mcp-adapter` または `pi-mcp-extension` がまだ入っていれば Pi から削除すること。** Pi の
  [MCP ドキュメント](https://github.com/earendil-works/pi/blob/v0.99.0/packages/coding-agent/docs/mcp.md)
  によると、`/mcp` を登録する extension がインストールされていると内蔵 MCP が置き換えられます。
  `pi-mcp-extension` は自身でも `mcp.json` を読みます。`pi-mcp-adapter` は 3.0.0 以降これを読まない
  ため、Skillshare が移したサーバーは adapter 経由では読み込まれません。

sync がサーバーをこれらの extension から移すとき、`sync mcp --dry-run`、`sync mcp`、`--json` は
一度だけ次のように表示します。

```text
! Pi's built-in MCP needs Pi 0.99.0 or later; on older Pi these servers stop loading until Pi is updated. If pi-mcp-adapter or pi-mcp-extension is still installed in Pi, remove it, because it can take the place of Pi's built-in MCP
```

この表示が出るのは、sync が Skillshare の書いた `mcp-adapter.json` のエントリを削除するとき、
`pi-mcp-extension` 向けに書いたエントリを書き換えるとき、またはこれらの extension だけが読む設定
（`piExtension: pi-mcp-adapter` または `pi-mcp-extension`、`directTools`、下記の `piOptions`
フィールド）が見つかったときです。その sync の後は表示されません。

0.23.0 では、Pi のモード選択（`piExtension`: `builtin`、`pi-mcp-adapter`、`pi-mcp-extension`）、
`piOptionsPrune` スイッチ、`directTools` が削除されました。古い config も引き続き読み込めます。
`sync mcp --dry-run` と `sync mcp` は、見つかった廃止設定の種類ごとに、該当サーバーを列挙した warning を
表示します。例:

```text
! Pi now uses its built-in MCP; the next sync updates the config: context7, local (shop)
```

`mcp.projects` 配下の project にだけあるサーバーは、括弧内に project フォルダーが表示されます。

次の sync で行われること:

| 0.23.0 より前 | sync 後 |
|---|---|
| `piExtension: builtin` | キーが削除される。それ以外は変わらない |
| `piExtension: pi-mcp-extension` | キーが削除される。エントリはもともと `mcp.json` にあったため、そこで内蔵形式に書き直される |
| `piExtension: pi-mcp-adapter` | キーが削除される。サーバーは `mcp.json` に書き込まれ、Skillshare が `mcp-adapter.json` に書き込んだエントリは削除される。自分で `mcp-adapter.json` に追加したエントリはそのまま残る |
| `piOptionsPrune` | キーが削除される。sync は常に、Skillshare が書き込み、変更されていないクリア済みフィールドを削除する（[上記](#pi-options)） |
| サーバーの `directTools` | `true` → `piOptions.exposure: direct`、`"search"` → `deferred`、名前のリスト → それらのツールを `direct` にした `piOptions.toolExposure` |
| `mcp.directTools`、または `mcp.projects` 配下の project の `directTools` | 既定値が、Pi に届き自身の値を持たない各サーバーに上記のとおり書き込まれる。project の `false` はグローバルの値より優先される |
| `piOptions.includeTools` / `excludeTools` | `tools.allow` / `tools.deny`。一緒に設定した `directTools` は引き続き `piOptions.exposure` になる |
| `piOptions` 内のその他の `pi-mcp-adapter` フィールド: `approveTools`、文字列の `auth`（Pi 自身の `auth` オブジェクトは保持）、`bearerToken`、`bearerTokenEnv`、`bearerTokenStore`、`caFile`、`debug`、`exposeResources`、`idleTimeout`、`inheritEnv`、`lifecycle`、`protocolVersion`、`requestHeadersCommand`、`requestTimeoutMs`、`searchKeywords`、`socket`、`tasks`、`toolPrefix`、`trace` | Pi の内蔵 MCP が読まないため削除される |

サーバーがすでに設定している exposure を上書きしてしまう `directTools`、`includeTools`、`excludeTools`、
またはツール名のリストではないものは、それぞれ独自の warning とともに破棄されます。

これらの変更を適用する最初の sync は、廃止された設定を除いて Skillshare の config（`config.yaml`、
または `sources.mcp` が指すファイル）も保存します。書き込む前に、古いファイルを理由 `migrate` で
[ファイル履歴](/docs/reference/commands/backup#file-history)に保存し、ファイルごとに 1 行を表示します。

```text
→ Updated config.yaml for 0.23.0 (backup: <path of the saved version>)
```

これは `skillshare sync mcp`、`skillshare sync --all`（Agent のファイルが変わらない場合も）、ダッシュボードの
sync で行われます。`--dry-run` とプレビューは何も書き込みません。config の保存に失敗した場合、Agent の
ファイルはすでに書き込まれており、config は元のままです。エラーがそのことを伝え、次の sync で再試行されます。
保存に成功すると warning は表示されなくなります。

削除されたフラグは、現在はメッセージとともに失敗します。

| フラグ | 代わりに使うもの |
|---|---|
| `--pi-extension` | 削除してください。Pi は常に内蔵 MCP を使います |
| `--pi-options-prune` | 削除してください。sync は、Skillshare が以前に書き込んで変更されていないフィールドを常に削除します |
| `--direct-tools` | すべてのツールには `--pi-options '{"exposure":"direct"}'`、Pi で個別のツールには `--pi-options '{"toolExposure":{"TOOL":"direct"}}'` |

`skillshare mcp import --from pi` は、Pi の `mcp.json` の隣にある `pi-mcp-adapter` の `mcp-adapter.json` も
引き続き読み込むため、サーバーを移行できます。両方のファイルが同じサーバーを定義している場合は `mcp.json` が
優先されます。sync はサーバーを Pi の `mcp.json` に書き込みます。`mcp-adapter.json` では、
0.23.0 より前に自分が書き込んだエントリを削除するだけです。その `directTools`、`includeTools`、`excludeTools` は上記の
とおり変換され、その他の adapter 専用フィールドは warning 付きで除外されます。ダッシュボードでは、
**target からインポート** が 2 つの Pi ファイルを別々のインポート元として一覧表示します。
