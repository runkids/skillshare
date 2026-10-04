---
sidebar_position: 4
---

# plugin

対応ツール全体で完全な native plugin を管理します。Capability チェックは、インストール対応とフォーマット検出を区別します。まずは [Manage plugins across tools](/docs/how-to/daily-tasks/sharing-plugins) から始めてください。

```bash
skillshare plugin                         # インタラクティブマネージャー
skillshare plugin add                     # Source → plugin → targets → review
skillshare plugin discover ./my-plugin --json
skillshare plugin add ./my-plugin --target claude --target codex --no-tui
skillshare plugin add ./my-plugin --no-tui   # Skillshare で管理だけして、target は後で選ぶ
skillshare plugin import review@team --from claude --no-tui
skillshare plugin list --json
skillshare plugin inspect review --json
skillshare plugin disable review --target codex --no-tui
skillshare sync plugins --dry-run --json
skillshare sync plugins --no-tui
skillshare plugin enable review --target codex --no-tui
skillshare plugin check review --json
skillshare plugin update review --target claude --no-tui
skillshare plugin remove review --no-tui
```

`enable` と `disable` は **Skillshare の sync 選択**を変更するものであり、Agent 側のネイティブな enabled 状態を変更するものではありません。Target の選択を外すとその選択が保存されます。次の `sync plugins` で、定義自体は残したままその管理下インストールが削除されます。再び選択すると、次の sync で再インストールできるようになります。管理外の plugin には影響しません。

`--target` なしの `add`（またはインタラクティブな選択で何も選ばない場合）は、plugin をどこにもインストールせずに Skillshare で管理します。後から同じ source と `--name` で target を追加するか、dashboard でその plugin の行から選べます。その時点の source がインストールされます。このような plugin は最後の target を外しても管理下に残ります。`--target` なしの `remove NAME` で Skillshare から削除します。

## コマンド

| コマンド | 動作 |
|---|---|
| `list` | 設定済みのバインディングとネイティブインストール状態。ターミナルではインタラクティブマネージャー |
| `discover SOURCE` | ローカルディレクトリ、`owner/repo`、または HTTPS Git リポジトリを検査 |
| `add [SOURCE]` | plugin 全体とその target アダプタを選択してインストール。または Pi 経由で `npm:` パッケージをインストール |
| `import [NATIVE-ID]` | 再インストールや有効化を行わずに既存のインストールを取り込む |
| `inspect NAME` | 管理下の 1 つの package を検査 |
| `sync [NAME]` | 選択済みの target を整合させ、未完了のネイティブ操作を再試行 |
| `check [NAME]` | Source の内容を記録済みのダイジェストと比較。更新は行わない |
| `update [NAME]` | Source の変更を確認し、対応するネイティブ更新操作を使用 |
| `enable / disable [NAME]` | 次の sync に target を含める / 除外する |
| `remove [NAME]` | 管理下のバインディングをアンインストールし、その定義を削除 |

引数なしのコマンドはターミナルで不足している入力をプロンプトします。非インタラクティブな変更コマンドには明示的な入力が必要です。`sync` と `check` はすべての package を対象に動作できます。`sync --all` は plugin を含み**ません** — `sync plugins` を明示的に使ってください。

## オプション

| オプション | 意味 |
|---|---|
| `--target TARGET` | 繰り返し指定可能な選択: `claude`、`codex`、`cursor`、`antigravity`（エイリアス `agy`）、`antigravity-cli`、`copilot`、`grok`、`pi`、`opencode`、または [Agent の別のアカウント](#accounts)の名前。下の capability 表を参照 |
| `--plugin NAME` | source のマーケットプレイスから 1 つの plugin を選択 |
| `--name NAME` | 追加またはインポート時の論理的な package 名 |
| `--from TARGET` | Claude、Codex、Antigravity CLI、Copilot、Grok、Pi、OpenCode、または [Agent の別のアカウント](#accounts)からインポート |
| `--dry-run`, `-n` | Skillshare または Agent の設定を変更せずにプレビュー |
| `--source-ref REF` | `discover`、`add`、`update` 用の git ブランチ、タグ、またはコミット。リモート source のみ |
| `--entry PATH` | package ルートからの相対パスで明示的にビルド済みの OpenCode JS/TS エントリを指定（`discover` と `add`） |
| `--revision ID` | プレビュー以降に source、設定、またはネイティブインベントリが変わっていた場合に適用を拒否 |
| `--json` | 機械可読な出力。TUI を無効化 |
| `--no-tui` | インタラクティブメニューを無効化。`tui: false` でも同様 |
| `--global`, `-g` | global Skillshare config とネイティブユーザースコープ |
| `--project`, `-p` | project config。Claude、Antigravity、Pi、または OpenCode（global へのフォールバックはなし） |

JSON 出力には source パスとネイティブ識別子が含まれます。source URL に認証情報を含めないでください。ある変更操作が失敗しても、target 単位では成功した結果が返る場合があります。いずれかの target が失敗すると CLI は非ゼロ終了コードを返します。再試行する前に結果を確認してください。

## Target 対応表

| Target | フォーマット | Global | Project | Update |
|---|---|:---:|:---:|---|
| Claude Code | `.claude-plugin/plugin.json` | Yes | Yes | ネイティブ update |
| Codex | `.codex-plugin/plugin.json` または Agent Plugins ルートマニフェスト | Yes | No | レビュー済みの Source を更新して再度 add。Codex で有効な場合のみ |
| Cursor | `.cursor-plugin/plugin.json` または Agent Plugins ルートマニフェスト | Yes | No | reviewed されたローカルコピーを置き換え |
| Antigravity Desktop | 明示的な name を持つルート `plugin.json` | Yes | Yes | reviewed されたローカルコピーを置き換え |
| Pi | `pi` resources を持つ `package.json`、または `pi-package` キーワードと慣例的な resource フォルダ | Yes | Yes（ネイティブな project trust が必要） | 管理下 source スナップショットを更新 |
| OpenCode | SDK 依存を持つ `package.json`、`.opencode/plugins/` エントリ、または明示的な `--entry` | Yes | Yes | 管理下 source スナップショットを更新 |

| Antigravity CLI | `agy` に受け入れられるネイティブルートマニフェストまたは Claude マニフェスト | Yes | No | enable 状態を保持しつつネイティブに update |
| GitHub Copilot CLI | `.plugin/plugin.json`、`.github/plugin/plugin.json`、Claude マニフェスト、または Agent Plugins ルートマニフェスト | Yes | No | reviewed された source を更新（ネイティブな enabled 状態がわかっており enabled の場合のみ） |
| Grok Build | `.grok-plugin/plugin.json` または Claude マニフェスト | インポート/削除のみ。インストールにはネイティブな trust が必要 | No | ネイティブに update |
| Kimi Code | `kimi.plugin.json` または `.kimi-plugin/plugin.json` | 検出のみ | No | 自動化されていない |
| Hermes | `.hermes-plugin/plugin.yaml` | 検出のみ | No | 自動化されていない |
| Devin | `.devin-plugin/plugin.json` | 検出のみ | No | 自動化されていない |

Kimi の非インタラクティブなライフサイクル、Hermes のプロファイルインベントリ/同意、Devin のローカルインベントリ/trust/クラウド区別は、これらのアダプタではまだ検証されていません。フォーマットは discovery 中に表示されますが、インストールは理由付きで無効化されています。source が target を宣言していても、Skillshare がそれを管理できることを意味しません。
`list --json` と `discover --json` には、許可された操作を含む `targetDefinitions` が含まれます。discovery ではさらに、各フォーマットの version、コンポーネント、entry、検証上の問題を示す `targetInfo` も公開されます。壊れたマニフェストはその target に限定され、不正なカタログは有効なフォーマットを隠すことなく警告として報告されます。

### Agent の別のアカウント {#accounts}

[Agent の別のアカウント](/docs/reference/targets/configuration#agent-config-dir)として宣言された Target は、`claude`、`codex`、`pi` については plugin の Target でもあります。Skillshare は `CLAUDE_CONFIG_DIR`、`CODEX_HOME`、`PI_CODING_AGENT_DIR` を通じて、そのアカウントの config ディレクトリに対して Agent 自身の CLI を実行します。

```yaml
targets:
  claude-work:
    agent: claude
    config_dir: ~/.claude-work
```

```bash
skillshare plugin add owner/repo --target claude-work
skillshare plugin import demo@market --from claude-work
```

[`cli`](/docs/reference/targets/configuration#agent-config-dir) を設定したアカウントは、代わりにその互換 CLI（Pi アカウントなら `omo` など）を同じ Config ディレクトリに対して実行します。Pi のアカウントでは `SENPI_CODING_AGENT_DIR` と `OMO_CODING_AGENT_DIR` も設定します。Pi の fork は `PI_CODING_AGENT_DIR` より先にこの 2 つを読むためです。CLI が見つからない場合、操作は失敗します。Skillshare が Agent 本体の CLI に切り替えることはありません。

アカウントは、所属する Agent の操作をそのまま引き継ぎ、自分の名前で独自のバインディングを保持します。そのため、一方のアカウントにだけ plugin をインストールすることもできます。アカウントは global スコープにのみ存在します。project の plugin は、1 つのアカウントではなく project に属します。`--target` と `--from` はアカウント名を受け付け、ターミナルのピッカーとダッシュボードの Plugins ページでは Agent と並んで一覧表示されます。

### Cursor と Antigravity

これらのアダプタは、plugin 全体をドキュメント化されたローカル discovery ディレクトリにコピーします。CLI 実行ファイルを必要とせず、マーケットプレイスのレジストリも変更しません。

- Cursor: `~/.cursor/plugins/local/<name>`。ローカルインポートを許可しておく必要があります。Cursor を再読み込みし、Customize を確認してください。同名のインストール済みマーケットプレイス plugin はローカルコピーより優先されます。
- Antigravity desktop: global では `~/.gemini/config/plugins/<name>`。workspace では `.agents/plugins/<name>`（または既存の `_agents/plugins/` ディレクトリ）。両方の workspace ディレクトリが存在する場合は、先に統合してください。
- スタンドアロンの **agy CLI** は独自の plugin ストアを持ちます。`antigravity` target はデスクトップ/workspace の discovery パスを管理するものであり、その CLI ストアではありません。`agy` は単なる Skillshare target のエイリアスです。スタンドアロン CLI には `--target antigravity-cli` を使ってください。`--target agy` は既存のデスクトップの意味のままです。

これらのローカル package には `plugin add` を使ってください。既存のローカルフォルダやマーケットプレイスインストールのインポートには対応していません。Skillshare は、所有していないフォルダ、symlink、またはローカルで編集された管理下コンテンツを上書きすることを拒否します。明示的な Antigravity マニフェスト名を指定すると、Git のチェックアウトやスナップショットをまたいで identity を安定させられます。

### Pi と OpenCode

Pi は同じパッケージの最初のグローバル登録と最後のプロジェクト登録を優先します。先行するグローバルソースや後続のプロジェクトソースの identity を確認できない場合、Skillshare はどの登録が優先されるかを証明できないため、上書きされる可能性のあるエントリ（継承するプロジェクト delta を含む）を Unknown／読み取り専用に保ちます。URL の query を削除して identity を推測することはありません。この不確実性の影響を受けない、確認済みのエントリは引き続き編集できます。

Pi は `pi install` / `pi remove` を使用し、インベントリは拡張機能のコードをロードせずにドキュメント化された package 設定を読み取ります。`PI_CODING_AGENT_DIR` が尊重されます。Pi の project trust は Pi 側で確立する必要があり、Skillshare はあなたの代わりに `--approve` を渡しません。

#### pi.dev の npm パッケージ

`plugin add npm:<package>` は、[pi.dev](https://pi.dev/packages) に載っているような npm で公開されたパッケージを、Pi 自身を通じてインストールします。

```bash
skillshare plugin add npm:@scope/package --target pi --dry-run --json -g
skillshare plugin add npm:@scope/package@1.2.0 --target pi --no-tui -g
```

Pi がパッケージをダウンロードして install script を実行するため、Skillshare は事前に内容を確認できません。追加する前に pi.dev または npm でパッケージを確認してください。`discover` は npm ソースを受け付けず、npm ソースには `--source-ref`、`--entry`、`--plugin` を指定できません。npm ソースを受け付けるのは Pi の target だけで、`pi` を実行する Pi アカウントも含みます。別の実行ファイルを使うアカウントでは、その実行ファイルでインストールしてからインポートしてください。`--project` を付けると、Pi はパッケージをプロジェクトの設定にインストールします。プロジェクトに `.pi` フォルダがある場合、Pi でプロジェクトを信頼するまで Pi はそのパッケージを変更しません。

Pi はパッケージ名ごとに 1 つのエントリだけを保持します。Pi に同じソースがすでにある場合、`add` はそれをインポートします。同じパッケージの別バージョンはインストールされ、Pi がそのエントリのソースを置き換えます。`update` は `pi update` を実行しますが、正確なバージョンに固定されたパッケージは Pi がそのまま維持するため、新しいバージョンで追加し直してください。パッケージの一部の extension をオフにしている場合、Pi はそのルールを新しいバージョンにも引き継ぎ、Skillshare も記録し直すため、後で再インストールしても元に戻ります。別の Skillshare パッケージがすでに管理している Pi パッケージは拒否されます。そちらを更新または削除してください。

ダッシュボードの追加ダイアログには、`pi install npm:<package>` コマンドやパッケージの pi.dev のアドレスをそのまま貼り付けられます。どちらも対応する `npm:` ソースに変換されます。

#### package の拡張機能を選ぶ

ダッシュボードでは、`pi` と Pi アカウントの target ページに **Extensions** タブがあります。その target の `settings.json` にある各パッケージエントリと、そのフィルターが選択する extension を一覧表示します。スイッチは、そのエントリの `extensions` リストに正確な `+path` または `-path` ルールを 1 つ書き込みます。ただし、そのファイル自身の正確なルールを削除するだけでスイッチが求める状態になる場合は、そのルールを削除します。そのため、ファイルを残りのルールが選ぶ状態に戻してもルールは残りません。**Remove rule** はスイッチがルールを削除しない場合に表示され、そのファイルの正確なルール（相対パスでも絶対パスでも）を削除し、以降そのファイルは残りのルールで決まります。結果はプレビューに表示されます。適用時は必ず先にプレビューを表示し、これらのリストだけを編集します。エントリの他のキー、`skills`・`prompts`・`themes` のフィルター、glob と `!` ルール、ファイルの残りの部分はそのまま保たれます。文字列エントリはルールを保持できるよう `{"source": ...}` に変わります。文字列エントリでは、Pi はパッケージの skills・prompts・themes を `pi` manifest からのみ読み込みますが、オブジェクトエントリでは manifest に書かれていないものをパッケージの `skills`・`prompts`・`themes` フォルダーからも読み込みます。そのようなフォルダーを持つパッケージの文字列エントリは、変換後もそれらのリソースが変わらないことを Skillshare が確認できないため読み取り専用です。1 つのファイルを指すソースも、Pi がそのまま読み込んでフィルターを無視するため読み取り専用です。プレビュー後にファイルが変更された場合や、Pi が設定ロックを保持している場合は何も書き込みません。書き込み中、Skillshare は Pi と同じ方法でそのロックを保持し、失った場合は書き込みません。適用の前に、変更する extension リストと変更前後のファイルハッシュの記録を保存します。成功した適用の記録は自動削除せずに保持し、適用が失敗した場合はその試行で新しく作成した記録だけを削除します。これは `settings.json` のコピーではなく、設定ファイル全体を復元することはできません。このタブには、Pi で直接インストールしたもの（[pi.dev](https://pi.dev/packages) の `npm:` パッケージなど）を含め、設定にあるすべてのパッケージが表示されます。Skillshare がパッケージをインストール・削除するのは `plugin` 経由だけです。`plugin add` はローカルディレクトリ、Git ソース、または [npm パッケージ](#pidev-の-npm-パッケージ)を受け付け、`plugin import --from pi` は Pi でインストールしたパッケージを管理下に取り込みます。

Skillshare はパッケージを実行せずに読み取るため、このタブが示すのは設定がどのファイルを選択しているか（**設定**列）であり、Pi が実際に読み込んだかではありません。適用後は Pi を再読み込みしてください。設定で指定されているのにパッケージにないファイルは、ないものとして示されます。Skillshare が判断できない選択は**判断できません**と表示され、理由と変更方法が添えられます。オンかオフかを推測することはありません。編集には、その target 自身の Pi が 0.99.2 以降（Pi 本体で確認した最も古いバージョン）であり、設定が厳密な JSON であることが必要です。それより古いバージョンは読み取り専用で、タブには検出したバージョンが表示されます。別の実行ファイルを使う Pi アカウントは読み取り専用で、Skillshare はそれを実行しません。リストが `[]`（何も読み込まない）のエントリは読み取り専用で、Skillshare が評価できないパターン（emoji に対する `?` など）で決まる拡張機能も読み取り専用です。ソースが空のエントリや、ソースまたはルールに対になっていない UTF-16 サロゲートのエスケープや不正な UTF-8 を含むエントリは、Skillshare が Pi と同じように正確に読み取れないため、書かれたまま読み取り専用になります。Pi はパッケージの最初のグローバルエントリだけを使うため、Skillshare がそのエントリを読み取れない場合は、同じパッケージの後のエントリも読み取り専用になります。

Pi に同期するプロジェクトにも、プロジェクトページに同じタブがあります。プロジェクトの設定をグローバルの設定に重ねた結果、各パッケージで何が選択されるかを示し、`pi (global)` から継承したものかプロジェクトの上書きかを表示します。スイッチはルールをプロジェクトの `.pi/settings.json` だけに保存し、`pi config` と同じ方法で書きます。グローバルのパッケージには、指定したファイルだけを変更するプロジェクトエントリ `{"source": ..., "autoload": false, "extensions": [...]}` が追加され、グローバルのエントリはそのままです。ローカルのソースは `.pi` からの相対パスで、npm や git のソースはグローバル設定の記述どおりに書かれます。そのようなエントリの最後のプロジェクトルールを削除した場合、以前の登録の filters が有効にならないときだけエントリを削除します。それ以外は空の winning override を保持します。明示的な JSON `false` だけが delta を意味し、`autoload: null` は `false` ではありません。対応するグローバルエントリがなく `autoload: false` のプロジェクトエントリは、`+` で指定したファイルだけを読み込みます。ファイルとその `.pi` フォルダは適用したときにだけ作成されます。グローバル設定と Pi の `trust.json` は書き込まれず、Skillshare が代わりにプロジェクトを信頼することもありません。Pi はプロジェクトを信頼している場合にだけプロジェクトの設定を使います。認証情報やクエリを含むグローバルのソースはプロジェクトにコピーされないため、そのパッケージはプロジェクトでは読み取り専用です。プロジェクトの設定に Skillshare が読み取れないエントリがある場合は、すべてのパッケージが読み取り専用になります。適用時は Pi のプロジェクトファイルのロックを保持し、書き込む直前に両方の設定ファイルとパッケージを再確認します。Pi 自身の `extensions` フォルダにある拡張機能（[extras](./extras.md) がそこにリンクしたファイルを含む）は、変更する場所とともに読み取り専用で一覧表示されます。プロジェクトは自身のフォルダ（Pi はプロジェクトを信頼している場合にだけ読み込みます）とグローバルのフォルダを表示します。

OpenCode は、管理下エントリを `opencode.json` または既存の `opencode.jsonc` にファイル URL として登録し、コメントや無関係なエントリを保持します。version 1 は `plugin` を、version 2 は `plugins` を使用します。`XDG_CONFIG_HOME` と絶対パスの global `OPENCODE_CONFIG` は尊重されます。曖昧または非対応のオーバーライドは拒否されます。Skillshare が version のスキーマを選択できるよう、OpenCode は PATH 上にある必要があります。

ローカルの OpenCode source は、ビルド済みのエントリ（`main`、文字列のルート export、または `index.js`）と必要なランタイム依存関係をすでに含んでいる必要があります。Skillshare はビルドスクリプトを実行したり、source に依存関係をインストールしたりしません。登録はモジュールが正常にロードされたことの証明ではありません。再読み込み後に OpenCode を確認してください。

ユーザースコープの npm 登録に managed cache がない場合は Unknown／読み取り専用です。未インストールとは限りません。Pi は Skillshare が調べない legacy global npm/pnpm パスを使うことがあります。

インポートは通常の Pi source に加え、Pi 0.99.2 以降の対応する source とオプション形式を持つ filtered object を受け付けます。プレビューは保持するキー名だけを表示し、opaque 値を表示しません。ネイティブ設定とファイルは変更せず、元のエントリを Skillshare の非公開 state に保存し、共有 config には digest だけを記録します。sync/update は現在のエントリを保持します。アンインストール前に最新のオプションを保存し、再インストール時はネイティブ install の前に object を復元して、他のリソースが一時的に既定で有効になるのを防ぎます。同じ Apply で複数を復元するときは、その操作自身が書いた正確な内容だけを受け入れます。他の設定変更がある場合は後続の復元を止めます。非公開 state を保持してください。記録の欠落・変更や別 target の記録は復元を拒否します。Windows の新しい登録ディレクトリは owner/SYSTEM 用の保護された ACL で作成します。既存ディレクトリと記録は、現在のユーザーと特権を持つ SYSTEM/Administrators 以外にアクセスを許可している場合、または ACL を確認できない場合、インポート・復元を拒否します。既存 ACL は変更しません。記録を保持し、所有者がアクセス保護を修復してから再試行してください。不明な source・優先順位、非対応のエンコーディング、Pi が正規化するローカル参照は読み取り専用です。通常の OpenCode エントリは取り込めますが、filtered OpenCode は引き続き拒否します。10 秒の stale 閾値を超えた空のロックディレクトリは inode と mtime が変わっていない場合だけ回収します。新しい・更新された・置換されたロック、空でないディレクトリ、ファイル、symlink は保持します。古さは所有者の終了を証明せず、最終確認と削除は atomic CAS ではありません。インポートされた Pi の package は、global モードでは `pi update SOURCE` で更新され、設定エントリは保持されます。project のものは Pi で更新してください。`pi update` は global の package にも及ぶためです。インポートされた OpenCode v1 の package は、そのネイティブツール上で更新されます。OpenCode v2 の global インポートはネイティブの update コマンドを使用できますが、v2 の update コマンドは global 向けであるため、project のインポートはネイティブに更新する必要があります。

```bash
skillshare plugin add ./cursor-plugin --target cursor --no-tui
skillshare plugin add ./agy-plugin --target agy --no-tui -p
skillshare plugin add ./pi-package --target pi --no-tui
skillshare plugin add ./opencode-package --target opencode --no-tui
skillshare plugin import npm:my-pi-package --from pi --name my-package --no-tui
```

## Ref と明示的なエントリ

**Add plugin** の詳細オプションでは、任意の Git ref と OpenCode entry を指定できます。通常のガイド付きフローでは空のままにしてください。ターミナルウィザードも同じフラグを受け付け、自動化には以下を使用できます。

```bash
skillshare plugin discover obra/superpowers --source-ref v6.3.0 --json
skillshare plugin add owner/repo --source-ref v1.0.0 --target copilot --no-tui
skillshare plugin add ./package --entry dist/plugin.js --target opencode --no-tui
skillshare plugin update review --source-ref v1.1.0 --target claude --dry-run --json
```

バインディングには `source_ref` と解決済みの `commit` が記録されます。インストールには reviewed されたコミットが使われます。`check` と `update` は設定された ref を再度解決するため、コミットが固定されたままブランチが進むことがあります。`--revision` は Git ref ではなくプレビュートークンです。`--entry` は各候補の package ルートからの相対パスであり、既に存在している必要があります。ビルドや package manager のインストールをトリガーすることはありません。

Copilot と Antigravity CLI のインストールは、reviewed されたローカルスナップショットを使用します。インポートには再インストール用の reviewed source がありません。削除後は、ネイティブクライアントでインストールしてから再度 sync してください。Grok もインストール/再インストール前にネイティブな trust が必要です。Skillshare はネイティブな trust の承認フラグを提供しません。

## 互換性と制約

- Claude はネイティブな `.claude-plugin/plugin.json` package を必要とします。
- Codex は `.codex-plugin/plugin.json` と、認識可能なポータブルなルート `plugin.json` package を受け付けます。Claude 専用の package が黙って変換されることはありません。
- source にはローカル plugin エントリを含むマーケットプレイスが含まれる場合があります。外部カタログは plugin の name/path でマージされます。パスが衝突する場合は拒否され、外部エントリはそのリポジトリを直接追加するか、ネイティブにインストールしてインポートするよう指示とともに報告されます。コマンドベースの source は自動承認されません。
- 完全な source スナップショットは、plugin のスクリプト、アセット、および安全な相対 symlink（`AGENTS.md → CLAUDE.md` を含む）を保持します。絶対パス、脱出、dangling、循環、`.git` を参照する symlink や特殊ファイルは拒否されます。source は 20,000 ファイルおよび 100 MiB に制限されます。
- ネイティブインストールは、ランタイムでの有効化を証明するものではありません。Agent を再起動/再読み込みし、その Agent 内で認証または hook trust を完了してください。
- Codex のネイティブな project インストールは、このアダプタでは提供されません。global の Codex インストールに対する sync 選択は引き続き機能します。
- Codex には update コマンドがないため、update は更新後のスナップショットからプラグインを再度 add します。add は常にプラグインを有効にするため、Codex で無効化されたプラグインはスキップされます。Import した Codex プラグインは `codex plugin marketplace upgrade NAME` で更新されます。これは Codex がその marketplace からインストールしたすべてのプラグインを再インストールするもので、Codex も起動時に同じことを行います。
- update は対応できない Target を理由を示してスキップし、そのプラグインのほかの Agent は通常どおり更新されます。スキップされた update は保留のまま残り、後の sync で処理されます。
- インポートされた plugin は元のマーケットプレイス identity を保持します。`check` は、source のないインポート済み plugin についてリリースの有無を推測できません。インポートした Claude または Codex の plugin のネイティブ marketplace がなくなった場合、sync と update はその Target をスキップして理由を示します。その marketplace を一度も追加していない別のマシンでも同じことが起こります。Agent で marketplace を復元するか、その Target を削除して source から plugin を追加し直してください。Skillshare がインポート済み plugin を勝手に別の source へ移すことはありません。
- Skillshare は管理下の Claude/Codex plugin ごとに marketplace を 1 つ登録し、`skillshare-<plugin>-<hash>` と名付けます（以前のインストールは `skillshare-<hash>` のまま）。plugin を削除または除外すると、plugin がすでになくてもその marketplace も削除し、失敗したクリーンアップは次回の sync で再試行します。marketplace がなくなっていた場合、update で再登録します。別のパスにある同名の登録とインポート済み plugin の marketplace には触れません。スナップショットとネイティブキャッシュは保持されます。
- これらの登録は、user 設定でも project 設定でも、このマシンの Skillshare 状態ディレクトリを指します。Git や dotfile マネージャーで Agent の設定を共有すると、ほかのマシンには存在しないパスが持ち込まれます。各マシンで source から plugin を追加してください。
- Claude は、skills ディレクトリ内で plugin manifest を持つ skill フォルダも `<name>@skills-dir` という plugin として読み込み、同じ名前の plugin は 1 つしか読み込みません。同名の Claude plugin を追加するとプレビューでそのことを伝えます。名前を変えるかどちらかを削除するまで、Claude は plugin を読み込み、skill フォルダはスキップします。

ネイティブなライフサイクルは、Claude Code `2.1.276`、Codex CLI `0.154.0`、Pi `0.85.1`、Copilot CLI `1.0.86` で動作確認されています。Antigravity CLI `1.2.6` は分離されたネイティブの install/list/remove 操作で確認済みです。OpenCode `1.18.31` は version 対応登録の検証に使用され、v2 スキーマは fixture テストでカバーされています。Cursor と Antigravity のファイルシステム上のライフサイクルは分離されたディレクトリでテストされていますが、GUI 上でのランタイム有効化は保証しません。インストール済みコマンドの capability とインベントリスキーマは実行時に確認され、非対応の操作は理由とともにブロックされます。

## 公式フォーマットの参考資料

- [Cursor local plugins](https://prod.cursor.com/docs/plugins)
- [Antigravity desktop plugins](https://www.antigravity.google/docs/plugins)
- [Antigravity standalone CLI plugins](https://www.antigravity.google/docs/cli/plugins)
- [Pi packages](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/packages.md)
- [OpenCode v1 plugins](https://opencode.ai/docs/plugins/)
- [OpenCode v2 plugins](https://opencode.ai/v2/docs/plugins)
