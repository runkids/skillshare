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
skillshare plugin update --all --no-tui
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
| `unimport NAME` | 指定したターゲットのローカル import をアンインストールし、共有ソース宣言を保持 |
| `inspect NAME` | 管理下の 1 つの package を検査 |
| `sync [NAME]` | 選択済みの target を整合させ、未完了のネイティブ操作を再試行 |
| `check [NAME]` | Source の内容を記録済みのダイジェストと、または Pi の npm パッケージのバージョンを npm の最新と比較。更新は行わない |
| `update [NAME \| --all]` | Source の変更を確認し、対応するネイティブ更新操作を使用 |
| `enable / disable [NAME]` | 次の sync に target を含める / 除外する |
| `remove [NAME]` | 管理下のバインディングをアンインストールし、その定義を削除 |

引数なしのコマンドはターミナルで不足している入力をプロンプトします。非インタラクティブな変更コマンドには明示的な入力が必要です。`sync` と `check` はすべての package を対象に動作でき、`update` は `--all`（`-a`）を指定するとすべての管理下 package を更新します。`sync --all` は plugin を含み**ません** — `sync plugins` を明示的に使ってください。

## オプション

| オプション | 意味 |
|---|---|
| `--target TARGET` | 繰り返し指定可能な選択: `claude`、`codex`、`cursor`、`antigravity`（エイリアス `agy`）、`antigravity-cli`、`copilot`、`grok`、`pi`、`omp`、`opencode`、または [Agent の別のアカウント](#accounts)の名前。下の capability 表を参照 |
| `--plugin NAME` | source のマーケットプレイスから 1 つの plugin を選択 |
| `--name NAME` | 追加またはインポート時の論理的な package 名 |
| `--from TARGET` | Claude、Codex、Antigravity CLI、Copilot、Grok、Pi、OMP のマーケットプレイス、OpenCode、または [Agent の別のアカウント](#accounts)からインポート |
| `--all`, `-a` | すべての管理下 plugin を更新（`update` のみ。package 名や `--source-ref` とは併用不可） |
| `--dry-run`, `-n` | Skillshare または Agent の設定を変更せずにプレビュー |
| `--source-ref REF` | `discover`、`add`、`update` 用の git ブランチ、タグ、またはコミット。リモート source のみ |
| `--entry PATH` | package ルートからの相対パスで明示的にビルド済みの OpenCode JS/TS エントリを指定（`discover` と `add`） |
| `--revision ID` | プレビュー以降に source、設定、またはネイティブインベントリが変わっていた場合に適用を拒否 |
| `--json` | 機械可読な出力。TUI を無効化 |
| `--no-tui` | インタラクティブメニューを無効化。`tui: false` でも同様 |
| `--global`, `-g` | global Skillshare config とネイティブユーザースコープ |
| `--project`, `-p` | project config。Claude、Antigravity、Pi、OMP、または OpenCode（global へのフォールバックはなし） |

JSON 出力には source パスとネイティブ識別子が含まれます。source URL に認証情報を含めないでください。ある変更操作が失敗しても、target 単位では成功した結果が返る場合があります。いずれかの target が失敗すると CLI は非ゼロ終了コードを返します。再試行する前に結果を確認してください。

## Target 対応表

| Target | フォーマット | Global | Project | Update |
|---|---|:---:|:---:|---|
| Claude Code | `.claude-plugin/plugin.json` | Yes | Yes | ネイティブ update |
| Codex | `.codex-plugin/plugin.json` または Agent Plugins ルートマニフェスト | Yes | No | レビュー済みの Source を更新して再度 add。Codex で有効な場合のみ |
| Cursor | `.cursor-plugin/plugin.json` または Agent Plugins ルートマニフェスト | Yes | No | reviewed されたローカルコピーを置き換え |
| Antigravity Desktop | 明示的な name を持つルート `plugin.json` | Yes | Yes | reviewed されたローカルコピーを置き換え |
| Pi | `pi` resources を持つ `package.json`、または `pi-package` キーワードと慣例的な resource フォルダ | Yes | Yes（ネイティブな project trust が必要） | 管理下 source スナップショットを更新 |
| Oh My Pi | ネイティブ互換のメタデータを持つ、マーケットプレイス経由のローカル/Git plugin | 新しい cache へのインストール/再インストール、インポート、スコープを限定した削除 | OMP の project アンカーで | 更新はブロック。削除しても共有 cache は保持 |
| OpenCode | SDK 依存を持つ `package.json`、`.opencode/plugins/` エントリ、または明示的な `--entry` | Yes | Yes | 管理下 source スナップショットを更新 |

| Antigravity CLI | `agy` に受け入れられるネイティブルートマニフェストまたは Claude マニフェスト | Yes | No | reviewed された source を更新（ネイティブな enabled 状態がわかっており enabled の場合のみ） |
| GitHub Copilot CLI | `.plugin/plugin.json`、`.github/plugin/plugin.json`、Claude マニフェスト、または Agent Plugins ルートマニフェスト | Yes | No | reviewed された source を更新（ネイティブな enabled 状態がわかっており enabled の場合のみ） |
| Grok Build | `.grok-plugin/plugin.json` または Claude マニフェスト | インポート/削除のみ。インストールにはネイティブな trust が必要 | No | ネイティブに update |
| Kimi Code | `kimi.plugin.json` または `.kimi-plugin/plugin.json` | 検出のみ | No | 自動化されていない |
| Hermes | `.hermes-plugin/plugin.yaml` | 検出のみ | No | 自動化されていない |
| Devin | `.devin-plugin/plugin.json` | 検出のみ | No | 自動化されていない |

Kimi の非インタラクティブなライフサイクル、Hermes のプロファイルインベントリ/同意、Devin のローカルインベントリ/trust/クラウド区別は、これらのアダプタではまだ検証されていません。フォーマットは discovery 中に表示されますが、インストールは理由付きで無効化されています。source が target を宣言していても、Skillshare がそれを管理できることを意味しません。
`list --json` と `discover --json` には、許可された操作を含む `targetDefinitions` が含まれます。discovery ではさらに、各フォーマットの version、コンポーネント、entry、検証上の問題を示す `targetInfo` も公開されます。壊れたマニフェストはその target に限定され、不正なカタログは有効なフォーマットを隠すことなく警告として報告されます。

### Agent の別のアカウント {#accounts}

[Agent の別のアカウント](/docs/reference/targets/configuration#agent-config-dir)として宣言された Target は、`claude`、`codex`、`opencode`、`pi` については plugin の Target でもあります。Skillshare は `CLAUDE_CONFIG_DIR`、`CODEX_HOME`、`OPENCODE_CONFIG_DIR`、`PI_CODING_AGENT_DIR` を通じて、そのアカウントの config ディレクトリに対して Agent 自身の CLI を実行します。

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

### Oh My Pi (OMP) {#omp}

OMP は Pi の package コマンドではなく、独自のマーケットプレイスのライフサイクルを使います。Skillshare は
レビュー済みのローカル/Git の plugin source を管理し、`name@marketplace` で識別される
ネイティブのマーケットプレイスエントリをインポートします。

```bash
skillshare plugin add ./my-omp-plugin --target omp --dry-run --json -g
skillshare plugin add ./my-omp-plugin --target omp --no-tui -g
skillshare plugin import demo@my-market --from omp --dry-run --json -g
```

インストールには、OMP **18.6.1+**、レビュー済みのローカル/Git source、検証済みでまだ使われていない cache の配置先、
そしてまだ存在しないスコープ内のランタイム配置先が必要です。
ランタイムの package 名は、マーケットプレイス名だけでなく、レビュー済みの source から決まります。既存のモジュールディレクトリやリンクは
置き換えられずにインストールをブロックします。配置先はプレビューに紐付けられ、インストール前に再確認されます。
アダプタは plugin のツリーをまとめて保ち、
ネイティブのマーケットプレイスインストールを使い、結果の登録を検証します。カタログは `.omp-plugin/marketplace.json` を優先し、OMP カタログが存在しない場合に限り
`.claude-plugin/marketplace.json` にフォールバックします。OMP entry は `package.json` に
`omp.extensions` または `pi.extensions` を宣言した package を参照でき、npm 名が異なる場合や未指定でも marketplace の
plugin 名を使います。各カタログが同名 plugin を別のフォルダに置く場合、OMP は独自のパス、
バージョン、components を保持します。テキストの discovery は build ごとに行を分けます。
JSON は一つの論理 candidate を保持し、`targetInfo` に target ごとのパスと metadata を返します。
target のバージョンが異なるか不明な場合、共有 `version` は空になり、Agent を選ばずに追加しても特定 build のバージョンは記録されません。
OMP カタログが存在する限り、OMP entry が欠落または無効でも Claude build には戻りません。
プレビューでは、ファイルを書き込む可能性がある OMP ネイティブの `--dry-run` は呼び出しません。適用前に
source を確認してください。OMP は plugin の
extension とツールを、project の信頼確認なしに、次回起動時にプロセス内で実行できます。インストールは、それらのリソースが正常に読み込まれたことの証明ではありません。

**OMP 18.6.1+ の共有 cache の制限:** ネイティブの uninstall と upgrade は、
別の project がまだ参照している plugin の cache ファイルを削除または置き換える可能性があります。
ネイティブのインベントリはユーザーレジストリと現在の project のレジストリしか一覧表示しないため、
他の project をまたいだそれらの操作の安全性を確認できません。
Skillshare はそれらの破壊的なコマンドを呼び出しません。**Remove** と
disable のあとの sync は、代わりにスコープを限定したアダプタを使い、そのインストールの
レジストリエントリ、ランタイムの選択、検証済みの `node_modules` リンクを削除します。plugin は
単に無効化されるのではなく、そのスコープからアンインストールされます。共有 cache、マーケットプレイス、
plugin 設定は保持され、cache のガベージコレクションは行いません。
古い cache がまだ存在する場合、再インストールでは新しいマーケットプレイス/cache の identity が自動的に選ばれ、
別の project が使っている可能性のあるファイルは置き換えられません。読み取れない、
リンクである、またはディレクトリでない cache の配置先は引き続きブロックされます。

削除には、検証済みの 18.6.1+ メタデータ契約、一致するインストール済みバージョン、
曖昧さのない JSON、そして証明されたランタイムリンクの所有権が必要です。実際のモジュールフォルダー、
他者のリンク、npm 依存関係の衝突、曖昧なランタイム所有者は拒否されます。
cache のマニフェストがない場合、アダプタは、スコープ内のリンクがそのインストールの cache を指す一意のランタイムロックキーを要求します。
マーケットプレイス名を推測することはなく、実際のランタイムリンクがまだインストールされたまま成功を報告することもありません。
検証できない有効なインストールはブロックされたままです。ネイティブの uninstall がすでに
cache、登録、ランタイムのリンク/選択を削除している場合、Forget は古い Skillshare の binding だけを削除します。別の plugin の `node_modules` ディレクトリはこれをブロックしません。
ネイティブファイルの変更はプレビューを無効にします。途中まで完了した削除は、
完全なネイティブ設定のバックアップを保存しなくても、保持された cache と binding の identity から再試行できます。Windows での書き込みには、さらに検証可能なプライベート ACL が必要です。
Skillshare はそれを通すために権限を変更しません。extension のコードは実行されません。

アダプタは Skillshare の削除を直列化し、ネイティブファイルのリビジョンを確認しますが、
OMP の plugin コマンドはそのロックに参加しません。ネイティブの plugin 変更を並行して実行しないでください。これは複数ファイルにまたがるアトミックなネイティブトランザクションではありません。
非破壊的な upgrade 契約が利用可能になるまで、**更新はブロックされたまま**です。
インポートされたコードが、レビュー済みの source なしに upgrade されることはありません。

project の操作では、選択したルートが OMP ネイティブの project アンカーと一致する必要があります。
暗黙に global や祖先の project にインストールすることはありません。ネイティブの
プロファイル/config ルートの上書き（`PI_CONFIG_DIR`、`PI_CODING_AGENT_DIR`、
`OMP_PROFILE`、`PI_PROFILE`）があると plugin 管理はブロックされます。ネイティブ cache が移動された場合、
既存のプレビューは無効になります。任意のアカウントの `config_dir` は OMP の
plugin ストアをリダイレクトしないため、OMP アカウントの plugin target はサポートされません。

ネイティブの npm/Git/link package はインベントリに表示されますが、
マーケットプレイスの plugin としてはインポートされません。それらの package は OMP で管理してください。`plugin add npm:...` は
引き続き Pi 専用のワークフローです。plugin の extension、hooks、skills、
MCP エントリを、Skillshare の他のリソースに別途コピーしないでください。

#### スタンドアロンの OMP extension を選ぶ

OMP target の **Extensions** タブを開くと、ファイルベースの選択を確認できます。カードはファイルを
source とスコープごとにまとめ、plugin は検査した package のルートごとに、
それ以外のファイルはディレクトリごとにまとめます。ファイル一覧は表示されたままです。plugin のカードは、
extension のフォルダー名ではなくネイティブの package identity から、package アイコンと `Plugins · <name>` の見出しを使います。
既知のマニフェストのバージョンとスコープは表示されたままで、
繰り返しの由来タグの代わりに **Open Plugins** のナビゲーションアイコンが付きます。共有フォルダーは
plugin からの相対パスで表示され、完全なファイルパスはツールチップに残ります。**Details** は
Pi と同じラベル付きの 2 列レイアウトで、完全な source、ネイティブ名、選択の識別子、
注記を表示します。identity のメタデータがない場合は、検査した
フォルダーにフォールバックし、package の identity やバージョンを推測することはありません。読み取り専用の
理由と Hooks や Plugins へのリンクは、エントリとともに表示されたままです。
対応するスタンドアロンのモジュールにはスイッチがあります。**Preview** は変更を表示し、**Apply** は
選択したネイティブの YAML 設定ファイルの `disabledExtensions` だけを書き込みます。
global、project、明示的に設定したアカウントのディレクトリは別々のままです。
エディターは無関係な YAML とコメントを保持し、OMP ネイティブのロックの下でレビュー済みのリビジョンを
再確認し、古い書き込みや使用中の書き込みは拒否します。

選択の編集には、選択したランチャーの背後に検証可能な OMP **18.6.1+** の package が必要です。
識別できないバージョン、スタンドアロンのバイナリ、ラッパーのランチャーは
読み取り専用のままです。このチェックは OMP を実行せずに package のメタデータを読みます。同名の
モジュールグループ、無効化名のフィルターを回避する明示的なファイル、環境由来の
`hooks/pre|post` factory、不確かな選択、リンクされたパスは読み取り専用です。
Hooks が所有するエントリと plugin が所有するエントリは、それぞれのマネージャーで管理されます。

バックアップは、変更前後の選択と設定のハッシュを Skillshare の state ディレクトリの
`omp-extensions/backups` に記録し、無関係な YAML の値や認証情報は記録しません。これは選択を復旧するための記録であり、
設定全体を復元するものではありません。適用の成功はディスク上の設定を示すものであり、実行中の
extension を示すものではありません。再起動/再読み込みは OMP 側の操作です。

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

Pi はパッケージ名ごとに 1 つのエントリだけを保持します。Pi に同じソースがすでにある場合、`add` はそれをインポートします。同じパッケージの別バージョンはインストールされ、Pi がそのエントリのソースを置き換えます。`update` は `pi update` を実行しますが、正確なバージョンに固定されたパッケージは Pi がそのまま維持するため、新しいバージョンで追加し直してください。バージョンを指定せずに追加したパッケージでは、`check` はインストール済みパッケージの `package.json` のバージョンを、公開 registry で npm の `latest` タグが示すバージョンと比べます。`update` はすでにそのバージョンのパッケージには何もしません。バージョン範囲やタグを付けて追加したパッケージ、環境変数や `.npmrc` で別の registry に向けられたパッケージ、バージョンが単純な `X.Y.Z` でないパッケージは、Pi で確認するものとして表示されます。dashboard は Plugins ページと Pi Target の Extensions タブで、すべての Pi パッケージのインストール済みバージョンを表示します。パッケージの一部の extension をオフにしている場合、Pi はそのルールを新しいバージョンにも引き継ぎ、Skillshare も記録し直すため、後で再インストールしても元に戻ります。別の Skillshare パッケージがすでに管理している Pi パッケージは拒否されます。そちらを更新または削除してください。

ダッシュボードの追加ダイアログには、`pi install npm:<package>` コマンドやパッケージの pi.dev のアドレスをそのまま貼り付けられます。どちらも対応する `npm:` ソースに変換されます。

#### package の拡張機能を選ぶ

ダッシュボードでは、`pi` と Pi アカウントの target ページに **Extensions** タブがあります。その target の `settings.json` にある各パッケージエントリと、そのフィルターが選択する extension を一覧表示します。Skillshare が管理する package は、OMP の plugin カードと同じく package アイコンと `Plugins · <name>` の見出しを使います。インストール済みのバージョン、スコープ、関連する project の上書きの形は保持され、ナビゲーションアイコンから Plugins を開けます。管理外の Pi package はネイティブの identity と Pi アイコンのままです。この表示は所有権や extension の選択ルールを変えません。extension の行は表示されたままで、**Details** は source とフィルタールールを別途表示します。スイッチは、そのエントリの `extensions` リストに正確な `+path` または `-path` ルールを 1 つ書き込みます。ただし、そのファイル自身の正確なルールを削除するだけでスイッチが求める状態になる場合は、そのルールを削除します。そのため、ファイルを残りのルールが選ぶ状態に戻してもルールは残りません。**Remove rule** はスイッチがルールを削除しない場合に表示され、そのファイルの正確なルール（相対パスでも絶対パスでも）を削除し、以降そのファイルは残りのルールで決まります。結果はプレビューに表示されます。適用時は必ず先にプレビューを表示し、これらのリストだけを編集します。エントリの他のキー、`skills`・`prompts`・`themes` のフィルター、glob と `!` ルール、ファイルの残りの部分はそのまま保たれます。文字列エントリはルールを保持できるよう `{"source": ...}` に変わります。文字列エントリでは、Pi はパッケージの skills・prompts・themes を `pi` manifest からのみ読み込みますが、オブジェクトエントリでは manifest に書かれていないものをパッケージの `skills`・`prompts`・`themes` フォルダーからも読み込みます。そのようなフォルダーを持つパッケージの文字列エントリは、変換後もそれらのリソースが変わらないことを Skillshare が確認できないため読み取り専用です。1 つのファイルを指すソースも、Pi がそのまま読み込んでフィルターを無視するため読み取り専用です。プレビュー後にファイルが変更された場合や、Pi が設定ロックを保持している場合は何も書き込みません。書き込み中、Skillshare は Pi と同じ方法でそのロックを保持し、失った場合は書き込みません。適用の前に、変更する extension リストと変更前後のファイルハッシュの記録を保存します。成功した適用の記録は自動削除せずに保持し、適用が失敗した場合はその試行で新しく作成した記録だけを削除します。これは `settings.json` のコピーではなく、設定ファイル全体を復元することはできません。このタブには、Pi で直接インストールしたもの（[pi.dev](https://pi.dev/packages) の `npm:` パッケージなど）を含め、設定にあるすべてのパッケージが表示されます。Skillshare がパッケージをインストール・削除するのは `plugin` 経由だけです。`plugin add` はローカルディレクトリ、Git ソース、または [npm パッケージ](#pidev-の-npm-パッケージ)を受け付け、`plugin import --from pi` は Pi でインストールしたパッケージを管理下に取り込みます。

Skillshare はパッケージを実行せずに読み取るため、このタブが示すのは設定がどのファイルを選択しているか（スイッチと選択の注記）であり、Pi が実際に読み込んだかではありません。適用後は Pi を再読み込みしてください。設定で指定されているのにパッケージにないファイルは、ないものとして示されます。Skillshare が判断できない選択は**判断できません**と表示され、理由と変更方法が添えられます。オンかオフかを推測することはありません。編集には、その target 自身の Pi が 0.99.2 以降（Pi 本体で確認した最も古いバージョン）であり、設定が厳密な JSON であることが必要です。それより古いバージョンは読み取り専用で、タブには検出したバージョンが表示されます。別の実行ファイルを使う Pi アカウントは読み取り専用で、Skillshare はそれを実行しません。リストが `[]`（何も読み込まない）のエントリは読み取り専用で、Skillshare が評価できないパターン（emoji に対する `?` など）で決まる拡張機能も読み取り専用です。ソースが空のエントリや、ソースまたはルールに対になっていない UTF-16 サロゲートのエスケープや不正な UTF-8 を含むエントリは、Skillshare が Pi と同じように正確に読み取れないため、書かれたまま読み取り専用になります。Pi はパッケージの最初のグローバルエントリだけを使うため、Skillshare がそのエントリを読み取れない場合は、同じパッケージの後のエントリも読み取り専用になります。

Pi に同期するプロジェクトにも、プロジェクトページに同じタブがあります。プロジェクトの設定をグローバルの設定に重ねた結果、各パッケージで何が選択されるかを示し、`pi (global)` から継承したものかプロジェクトの上書きかを表示します。スイッチはルールをプロジェクトの `.pi/settings.json` だけに保存し、`pi config` と同じ方法で書きます。グローバルのパッケージには、指定したファイルだけを変更するプロジェクトエントリ `{"source": ..., "autoload": false, "extensions": [...]}` が追加され、グローバルのエントリはそのままです。ローカルのソースは `.pi` からの相対パスで、npm や git のソースはグローバル設定の記述どおりに書かれます。そのようなエントリの最後のプロジェクトルールを削除した場合、以前の登録の filters が有効にならないときだけエントリを削除します。それ以外は空の winning override を保持します。明示的な JSON `false` だけが delta を意味し、`autoload: null` は `false` ではありません。対応するグローバルエントリがなく `autoload: false` のプロジェクトエントリは、`+` で指定したファイルだけを読み込みます。ファイルとその `.pi` フォルダは適用したときにだけ作成されます。グローバル設定と Pi の `trust.json` は書き込まれず、Skillshare が代わりにプロジェクトを信頼することもありません。Pi はプロジェクトを信頼している場合にだけプロジェクトの設定を使います。認証情報やクエリを含むグローバルのソースはプロジェクトにコピーされないため、そのパッケージはプロジェクトでは読み取り専用です。プロジェクトの設定に Skillshare が読み取れないエントリがある場合は、すべてのパッケージが読み取り専用になります。適用時は Pi のプロジェクトファイルのロックを保持し、書き込む直前に両方の設定ファイルとパッケージを再確認します。Pi 自身の `extensions` フォルダにある拡張機能（[extras](./extras.md) がそこにリンクしたファイルを含む）は、変更する場所とともに読み取り専用で一覧表示されます。プロジェクトは自身のフォルダ（Pi はプロジェクトを信頼している場合にだけ読み込みます）とグローバルのフォルダを表示します。

OpenCode は、管理下エントリを `opencode.json` または既存の `opencode.jsonc` にファイル URL として登録し、コメントや無関係なエントリを保持します。version 1 は `plugin` を、version 2 は `plugins` を使用します。`XDG_CONFIG_HOME` と絶対パスの global `OPENCODE_CONFIG` は尊重されます。曖昧または非対応のオーバーライドは拒否されます。Skillshare が version のスキーマを選択できるよう、OpenCode は PATH 上にある必要があります。

ローカルの OpenCode source は、ビルド済みのエントリ（`main`、文字列のルート export、または `index.js`）と必要なランタイム依存関係をすでに含んでいる必要があります。Skillshare はビルドスクリプトを実行したり、source に依存関係をインストールしたりしません。登録はモジュールが正常にロードされたことの証明ではありません。再読み込み後に OpenCode を確認してください。

ユーザースコープの npm 登録に managed cache がない場合は Unknown／読み取り専用です。未インストールとは限りません。Pi は Skillshare が調べない legacy global npm/pnpm パスを使うことがあります。

インポートは通常の Pi source に加え、Pi 0.99.2 以降の対応する source とオプション形式を持つ filtered object を受け付けます。プレビューは保持するキー名だけを表示し、opaque 値を表示しません。ネイティブ設定とファイルは変更せず、元のエントリを Skillshare の非公開 state に保存し、ローカル receipt に digest だけを記録します。sync/update は現在のエントリを保持します。アンインストール前に最新のオプションを保存し、再インストール時はネイティブ install の前に object を復元して、他のリソースが一時的に既定で有効になるのを防ぎます。同じ Apply で複数を復元するときは、その操作自身が書いた正確な内容だけを受け入れます。他の設定変更がある場合は後続の復元を止めます。非公開 state を保持してください。記録の欠落・変更や別 target の記録は復元を拒否します。Windows の新しい登録ディレクトリは owner/SYSTEM 用の保護された ACL で作成します。既存ディレクトリと記録は、現在のユーザーと特権を持つ SYSTEM/Administrators 以外にアクセスを許可している場合、または ACL を確認できない場合、インポート・復元を拒否します。既存 ACL は変更しません。記録を保持し、所有者がアクセス保護を修復してから再試行してください。不明な source・優先順位、非対応のエンコーディング、Pi が正規化するローカル参照は読み取り専用です。通常の OpenCode エントリは取り込めますが、filtered OpenCode は引き続き拒否します。10 秒の stale 閾値を超えた空のロックディレクトリは inode と mtime が変わっていない場合だけ回収します。新しい・更新された・置換されたロック、空でないディレクトリ、ファイル、symlink は保持します。古さは所有者の終了を証明せず、最終確認と削除は atomic CAS ではありません。インポートされた Pi の package は、global モードでは `pi update SOURCE` で更新され、設定エントリは保持されます。project のものは Pi で更新してください。`pi update` は global の package にも及ぶためです。インポートされた OpenCode v1 の package は、そのネイティブツール上で更新されます。OpenCode v2 の global インポートはネイティブの update コマンドを使用できますが、v2 の update コマンドは global 向けであるため、project のインポートはネイティブに更新する必要があります。

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

共有バインディングには `source_ref` を記録し、解決済みの `commit` はローカルの記録に保存します。インストールには reviewed されたコミットが使われます。`check` と `update` は設定された ref を再度解決するため、コミットが固定されたままブランチが進むことがあります。`--revision` は Git ref ではなくプレビュートークンです。`--entry` は各候補の package ルートからの相対パスであり、既に存在している必要があります。ビルドや package manager のインストールをトリガーすることはありません。

Copilot と Antigravity CLI のインストールは、reviewed されたローカルスナップショットを使用します。インポートには再インストール用の reviewed source がありません。削除後は、ネイティブクライアントでインストールしてから再度 sync してください。Grok もインストール/再インストール前にネイティブな trust が必要です。Skillshare はネイティブな trust の承認フラグを提供しません。

## 互換性と制約

- Claude はネイティブな `.claude-plugin/plugin.json` package を必要とします。
- Codex は `.codex-plugin/plugin.json` と、認識可能なポータブルなルート `plugin.json` package を受け付けます。両方を含む package では Codex がポータブルなルート `plugin.json` からインストールするため、Skillshare はその manifest のバージョンを、ない場合は `1.0.0` を想定します。Claude 専用の package が黙って変換されることはありません。
- source にはローカル plugin エントリを含むマーケットプレイスが含まれる場合があります。ネイティブカタログは plugin 名でマージされ、target ごとの package パスを保持します。外部エントリはそのリポジトリを直接追加するか、ネイティブにインストールしてインポートするよう指示とともに報告されます。コマンドベースの source は自動承認されません。
- 完全な source スナップショットは、plugin のスクリプト、アセット、および安全な相対 symlink（`AGENTS.md → CLAUDE.md` を含む）を保持します。絶対パス、脱出、dangling、循環、`.git` を参照する symlink や特殊ファイルは拒否されます。source は 20,000 ファイルおよび 100 MiB に制限されます。
- ネイティブインストールは、ランタイムでの有効化を証明するものではありません。Agent を再起動/再読み込みし、その Agent 内で認証または hook trust を完了してください。
- Codex は `PATH` の `codex` を実行します。`PATH` にない場合、Skillshare は Homebrew の
  `/opt/homebrew/bin` と `/usr/local/bin`、次に Codex デスクトップ app に同梱の CLI
  （macOS の `ChatGPT.app` または `Codex.app`、Windows の `%LOCALAPPDATA%\OpenAI\Codex\bin`）を順に試します。Codex が別の場所にあるマシンでは
  [`SKILLSHARE_CODEX_CLI`](/docs/reference/appendix/environment-variables#skillshare_codex_cli) を設定してください。独自の `cli` を持つ account は検索されません。
- Codex のネイティブな project インストールは、このアダプタでは提供されません。global の Codex インストールに対する sync 選択は引き続き機能します。
- Codex には update コマンドがないため、update は更新後のスナップショットからプラグインを再度 add します。add は常にプラグインを有効にするため、Codex で無効化されたプラグインはスキップされます。Import した Codex プラグインは `codex plugin marketplace upgrade NAME` で更新されます。これは Codex がその marketplace からインストールしたすべてのプラグインを再インストールするもので、Codex も起動時に同じことを行います。
- update は対応できない Target を理由を示してスキップし、そのプラグインのほかの Agent は通常どおり更新されます。スキップされた update は保留のまま残り、後の sync で処理されます。
- インポートされた plugin は元のマーケットプレイス identity を保持します。`check` は、source のないインポート済み plugin についてリリースの有無を推測できません。ただし Pi の npm パッケージは npm で確認します。インポートした Claude または Codex の plugin のネイティブ marketplace がなくなった場合、sync と update はその Target をスキップして理由を示します。新しいマシンでは、同名の marketplace があっても明示的なローカル import が必要です。Agent で marketplace を復元するか、その Target を削除して source から plugin を追加し直してください。Skillshare がインポート済み plugin を勝手に別の source へ移すことはありません。
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

## 共有宣言とマシンごとのインストール {#host-state}

`plugins.version: 2` は global/project 設定に source、ref、plugin、entry、ターゲットごとのソースと `sync` 選択、または `imported: true` を保存します。ターゲットのソースは明示的に保存されるため、パッケージの既定値を変えても別の配布物には切り替わりません。

```yaml
plugins:
  version: 2
  packages:
    router:
      source: https://github.com/acme/router.git
      plugin: router
      bindings:
        pi: {source: https://github.com/acme/router.git, plugin: router}
        omp: {imported: true}
```

ネイティブ ID、確認済みの commit/digest/version、Pi 登録参照、保留中の操作は `<StateDir>/plugins/<config-path-hash>/state.json`（ローカル形式 version 1）に保存します。StateDir はマシンごとに保持してください。状態は設定ファイルの**絶対パス**で識別されます。プロジェクトや設定を移動・改名すると新しい状態領域になり、以前のインストールは追跡も削除もされません。移動前に旧設定から削除するか、移動先で明示的に管理・インポートしてください。

sync、通常の update、検証、失敗、再試行は共有宣言を書き換えません。`update --source-ref` は ref が変わったときだけ宣言を更新します。読み取りはローカルの操作ロックの下で旧形式の証拠を保存し、共有 YAML は変更しません。ロック使用中は保存を次の操作または読み取りに任せます。add、import、選択、remove は必要な宣言変更を保存し、外部変更を検出します。同時の Syncthing 編集はマージできません。

ソースの確認と commit の固定は**マシンごと**です。ブランチ ref はそのマシンでのインストール・更新時に解決されるため、非対話 sync が別のマシンで確認したものより新しい commit をインストールすることがあります。ref の変更も各マシンで解決します。全マシンで同じ版を使うには `source_ref` に commit SHA を指定してください。ローカルのソースパスも各マシンに必要です。

native-only ターゲットは「このマシンでインポートが必要」と表示され、sync はスキップします。ネイティブクライアントでインストール後、`plugin import LOCAL-ID --from TARGET --name NAME` を実行してください。同じインポートを繰り返しても、共有宣言が同じなら YAML は書き換えません。既存のソース宣言へのローカル import はローカル記録だけを作り、他のマシンのソースやインストールを変えません。確認済みソースの同じ ID を再 import してもソースは保持されます。Pi の明示的な `add npm:PACKAGE` はインストール手順を共有し、Pi がスクリプトを実行します。npm import は native-only 宣言を作ります。

`import` はリモートソースを取得しません。手書きのリモートソース設定に `plugin:` がない場合は、インポート前にネイティブパッケージ名を `plugin:` に指定してください。これによりオフラインで同一性を確認できます。`plugin add` で作成した設定には、この名前がすでに記録されています。ローカルディレクトリの設定はローカルで検出できます。

旧形式の読み取りと sync は YAML を変えず、次の宣言編集で version 2 にします。**最初の v2 編集前に全 writer を更新し、各マシンで共有 YAML が旧形式のうちに `skillshare plugin list` を実行してください。** ネイティブクライアントが一時的に使えなくても移行のヒントを保存できます。採用には元のネイティブ環境の inventory と、ソースの場合は所有する snapshot の証拠が必要です。他のホストの保留中操作は実行しません。未確認のヒントと削除の証拠は、所有権確認と cleanup が完了するまで保持します。旧 npm は add/import を区別できないため、共有手順が必要なら明示的に add し直してください。

アカウント変更時は以前のネイティブ環境を解決してから cleanup します。認証、ネイティブ設定、trust は各ツールに残ります。

同じインポートを繰り返しても共有の意図は書き換えません。クリーンアップが失敗すると再試行できるローカル記録が残り、次の sync で再試行します。古いバイナリはバージョン付きの宣言フィールドを認識しないため v2 を書き込めません。

共有ソースの設定が変わってもローカルのインポートは保持します。このマシンを共有ソースへ切り替えるには、次を実行してください。

```bash
skillshare plugin unimport NAME --target TARGET --no-tui
skillshare plugin sync NAME --no-tui
```

`unimport` には 1 つ以上の明示的な `--target` と、各ターゲットの既存のローカル import 記録および共有ソース設定が必要です。ターゲットの削除アダプターを使って、このマシンのインポート済みのインストールを解除してからローカル記録を削除します。共有宣言と他のマシンのインストールは変更しません。次の sync が共有ソースからインストールします。

アンインストールすると、そのインストールに関連するネイティブクライアントの設定やログイン情報も削除されます。
