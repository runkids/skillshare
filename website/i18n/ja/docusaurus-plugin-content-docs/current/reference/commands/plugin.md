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
| `add [SOURCE]` | plugin 全体とその target アダプタを選択してインストール |
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
| Codex | `.codex-plugin/plugin.json` または Agent Plugins ルートマニフェスト | Yes | No | 検証済みのネイティブ再インストール |
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

Pi は `pi install` / `pi remove` を使用し、インベントリは拡張機能のコードをロードせずにドキュメント化された package 設定を読み取ります。`PI_CODING_AGENT_DIR` が尊重されます。Pi の project trust は Pi 側で確立する必要があり、Skillshare はあなたの代わりに `--approve` を渡しません。

OpenCode は、管理下エントリを `opencode.json` または既存の `opencode.jsonc` にファイル URL として登録し、コメントや無関係なエントリを保持します。version 1 は `plugin` を、version 2 は `plugins` を使用します。`XDG_CONFIG_HOME` と絶対パスの global `OPENCODE_CONFIG` は尊重されます。曖昧または非対応のオーバーライドは拒否されます。Skillshare が version のスキーマを選択できるよう、OpenCode は PATH 上にある必要があります。

ローカルの OpenCode source は、ビルド済みのエントリ（`main`、文字列のルート export、または `index.js`）と必要なランタイム依存関係をすでに含んでいる必要があります。Skillshare はビルドスクリプトを実行したり、source に依存関係をインストールしたりしません。登録はモジュールが正常にロードされたことの証明ではありません。再読み込み後に OpenCode を確認してください。

インポートは、通常の Pi package source と通常の OpenCode config エントリを受け付けます。resource フィルタ/オプション付きのエントリは、それらの設定を保持するために拒否されます。インポートされた Pi の package は、global モードでは `pi update SOURCE` で更新され、設定エントリは保持されます。project のものは Pi で更新してください。`pi update` は global の package にも及ぶためです。インポートされた OpenCode v1 の package は、そのネイティブツール上で更新されます。OpenCode v2 の global インポートはネイティブの update コマンドを使用できますが、v2 の update コマンドは global 向けであるため、project のインポートはネイティブに更新する必要があります。

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
- Codex の project インストールは未対応です。global 更新は検証済みのネイティブ再インストールを使用します。
- インポートは元の marketplace identity を保持します。Codex `check` は登録済み local/Git source を確認できます。他の確認可能な source を持たないインポートは release の有無を推測できません。
- update は対応できない Target を理由を示してスキップし、そのプラグインのほかの Agent は通常どおり更新されます。スキップされた update は保留のまま残り、後の sync で処理されます。
- 削除は共有されたマーケットプレイス登録と管理下スナップショットを保持します。関連のない plugin やネイティブキャッシュを直接削除することはありません。

ネイティブなライフサイクルは、Claude Code `2.1.276`、Codex CLI `0.154.0`、Pi `0.85.1`、Copilot CLI `1.0.86` で動作確認されています。Antigravity CLI `1.2.6` は分離されたネイティブの install/list/remove 操作で確認済みです。OpenCode `1.18.31` は version 対応登録の検証に使用され、v2 スキーマは fixture テストでカバーされています。Cursor と Antigravity のファイルシステム上のライフサイクルは分離されたディレクトリでテストされていますが、GUI 上でのランタイム有効化は保証しません。インストール済みコマンドの capability とインベントリスキーマは実行時に確認され、非対応の操作は理由とともにブロックされます。

## Codex 更新

global Codex 更新は管理下の source と、確認可能な local/Git marketplace source を持つインポートに対応します。`skillshare plugin update review --target codex --dry-run --json -g` で確認し、`--no-tui` で適用します。旧/新 version、ネイティブ操作、enabled 状態、影響する plugin を表示します。`plugin add` は確認済み catalog の非公開コピーを使います。Git インポートは対象を指定した `plugin marketplace upgrade NAME --json` を先に実行します。他の plugin が一時的に再インストールされる場合がありますが、成功前に cache の内容、権限、元の不在状態を復元します。その後の native refresh は Codex が管理します。

enabled 状態と設定の全 byte を保持し、インストール version を検証してから成功を記録します。失敗時は元の cache と Git marketplace を復元します。同時編集による競合時は非公開 recovery directory を保持して path を報告します。lock は Skillshare の操作のみを調整するため、同じ native directory への同時書き込みを避けてください。アカウントの `config_dir`、`CODEX_HOME`、`~/.codex` の順に使用します。インポートの Git ref は native config に従い、`--source-ref` は拒否します。状態不明、インストール済み cache の不足、version なし、未対応 source、安全でない recovery file は更新を阻止します。trust、認証、インストール policy は Codex が適用します。Codex CLI `0.159.3` で検証済みです。

Codex が inline TOML plugin table を書き換えるため、この形式は preview で拒否します。ordinary/dotted table を使用してください。

## 公式フォーマットの参考資料

- [Cursor local plugins](https://prod.cursor.com/docs/plugins)
- [Antigravity desktop plugins](https://www.antigravity.google/docs/plugins)
- [Antigravity standalone CLI plugins](https://www.antigravity.google/docs/cli/plugins)
- [Pi packages](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/packages.md)
- [OpenCode v1 plugins](https://opencode.ai/docs/plugins/)
- [OpenCode v2 plugins](https://opencode.ai/v2/docs/plugins)
