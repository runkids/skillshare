---
sidebar_position: 4
---

# hooks

名前付き hooks を管理し、各 Agent のネイティブ設定に同期します。Dashboard の **Hooks** で追加、編集、インポート、有効／無効、プレビュー、同期を行えます。**Targets**、**Projects**、**Sync**、**Settings → Backups** にも表示されます。

hook の行メニューから、各ターゲットのネイティブ設定やコード、スクリプトファイルと出力先を確認できます。この読み取り専用プレビューには、その hook の内容だけを表示します。共有ファイルの他の設定は保持されます。無効な hook も確認できますが、公開されません。

## コマンド

```bash
skillshare hooks
skillshare hooks list --json
skillshare hooks add check --file ./check.yaml
skillshare hooks edit check --file ./updated-check.yaml --sync
skillshare hooks import --from claude --json
skillshare hooks import imported --from claude --file ./settings.json --dry-run
skillshare hooks disable check --sync
skillshare hooks enable check --sync
skillshare hooks sync --dry-run --json
skillshare hooks sync
skillshare hooks sync check --replace --dry-run
skillshare sync hooks --dry-run --json
skillshare sync hooks
skillshare hooks remove check --sync
skillshare hooks remove check --keep-files
skillshare hooks restore BACKUP_ID --dry-run
skillshare hooks restore BACKUP_ID
```

サブコマンドなしでは entries を一覧表示します。`add`／`edit` は `--file` の Entry JSON/YAML を読み込みます。インポートで名前を省略すると候補（Agent の event またはファイルごとに 1 件）を表示し、名前を指定すると保存します。インポートを保存すると、読み取った登録をその場で引き継ぎます。次回の同期は `--replace` なしで adopt し、インポートしなかった event は管理外のままです。設定とコードを読むだけで実行しません。追加、編集、インポート、有効化、無効化、削除はソースのみを保存し、`--sync` で同期します。同期と復元は適用前に再プレビューします。

| Option | Meaning |
|---|---|
| `--file PATH` | 追加／編集の Entry JSON/YAML、またはインポートのネイティブ設定／コード |
| `--from AGENT` | `codex-2` などのアカウント target も可。  インポート元 Agent またはネイティブ形式 |
| `--sync` | 保存して同期 |
| `--keep-files` | `remove` と併用: hook の管理をやめ、ネイティブのエントリはそのまま残す。`--sync` とは併用できない。[下記](#stop-managing-a-hook)を参照 |
| `--replace` | 既存ソース entry またはその entry の競合出力を明示的に置換 |
| `--dry-run, -n` | 書き込まずプレビュー |
| `--json` | 構造化出力 |
| `--revision ID` | 指定プレビュー revision との一致を要求 |
| `--global, -g` | global 設定 |
| `--project, -p` | project 設定 |

`hooks list --json` でバックアップ ID と完全な保存先を確認します。設定が変わると古いプレビューは無効になるため、保存／同期前に更新してください。

## hook の管理をやめる {#stop-managing-a-hook}

```bash
skillshare hooks remove check --keep-files
```

ソースから `check` を削除し、Skillshare がそのために書き込んだネイティブの登録とファイルの記録を忘れます。Agent のファイルは変わりません。以後それらのエントリはあなたのものです。同期は削除も更新もせず、`hooks import` で再び候補に表示されます。`--keep-files` は `--sync` と併用できません。ダッシュボードの削除ダイアログでは、Hooks ページとプロジェクトの **Hooks** タブで **管理を停止** として選べます。**ソースからのみ削除** は異なり、次回の同期で Agent のファイルからその hook のエントリを削除します。

変わるのは削除したスコープだけです。global の hook の管理をやめても、同じ名前のプロジェクトの hook は管理されたままで、その逆も同様です。再び管理するには、インポートしてください。

## ソースのフィールド

選択した Skillshare 設定の `hooks.entries` に宣言します。名前で entry を識別し、`bindings` で受信 Agent とネイティブ定義を選びます。空の bindings はソースを保持し、公開しません。

```yaml
hooks:
  entries:
    check:
      description: Run the project's check after Claude finishes
      enabled: true
      bindings:
        claude:
          events:
            Stop:
              - hooks:
                  - type: command
                    command: "make check"
                    timeout: 120
```

`hooks add check --file check.yaml` のファイルは Entry の `description`、`enabled`、`bindings` のみを含み、外側の `hooks.entries` は不要です。

Skillshare は `hooks` セクションをインデントした block 形式で書き込み、保存のたびに以前 1 行に詰め込まれた entry も展開します。Hooks ページの **config.yaml** ボタンは **Settings → Files** を `hooks:` セクションの位置で開きます。`hooks` のキーをクリックすると右側のパネルに説明が表示され、**整形** はネストした 1 行のセクションを展開します。`targets: [claude, codex]` のような短いリストは 1 行のままです。

| Option | Meaning |
|---|---|
| `description` | 任意の説明 |
| `enabled` | 既定 true。false はソースを保持し、次の同期で未変更の所有出力を削除 |
| `bindings` | Agent ID ごとのネイティブ binding |
| `bindings.AGENT.events` | 設定型 Agent のネイティブ event map |
| `bindings.AGENT.code` | Pi、Amp、OpenCode のネイティブ extension/plugin ソース |
| `bindings.AGENT.files` | 相対ファイル名を key とした任意の UTF-8 スクリプト |

Agent ID は `claude`、`codex`、`gemini`、`copilot`、`cursor`、`droid`、`qwen`、`antigravity`、`pi`、`amp`、`opencode`。`factory` は `droid` の、`antigravity-cli` と `agy` は `antigravity` の別名です。event、matcher、handler type、command、timeout 単位、payload はネイティブ形式のまま保持し、自動変換しません。event 名は各 command Agent のドキュメントにある event と照合します。未知の名前（例：綴り違いの `Stopp`）はプレビューと plan の `warnings` に警告として表示されますが、Agent は event を追加していくため同期は止めません。Pi、Amp、OpenCode のコードは確認しません。Pi、Amp、OpenCode のコードと imports はインストール済みバージョンに合わせ、専用の `skillshare-NAME.ts` に出力します。共通実行エンジンは生成しません。command binding のスクリプトは Agent 設定ディレクトリの `hooks/skillshare/NAME/` に保存され、command 内の指定 macro／パスは変更しません。プレビューで完全なパスを確認してください。 global の binding は `targets` で宣言したアカウント target の名前も指定できます。

## ネイティブの保存先

| Agent | Global | Project | Format |
|---|---|---|---|
| [Claude Code](https://code.claude.com/docs/en/hooks) | `~/.claude/settings.json` | `.claude/settings.json` | `hooks` event map with matcher groups |
| [Codex](https://learn.chatgpt.com/docs/hooks) | `~/.codex/hooks.json` | `.codex/hooks.json` | Wrapped `hooks` event map |
| [Gemini CLI](https://geminicli.com/docs/hooks/reference/) | `~/.gemini/settings.json` | `.gemini/settings.json` | `hooks` event map |
| [Copilot CLI](https://docs.github.com/en/copilot/reference/hooks-reference) | `~/.copilot/hooks/skillshare-NAME.json` | `.github/hooks/skillshare-NAME.json` | Version 1, `hooks` event map |
| [Cursor](https://cursor.com/docs/hooks) | `~/.cursor/hooks.json` | `.cursor/hooks.json` | Version 1, native lowerCamelCase events |
| [Factory Droid](https://docs.factory.com/harness/hooks) | `~/.factory/hooks.json` | `.factory/hooks.json` | Unwrapped event map |
| [Qwen Code](https://qwenlm.github.io/qwen-code-docs/en/users/features/hooks/) | `~/.qwen/settings.json` | `.qwen/settings.json` | `hooks` event map |
| [Antigravity](https://antigravity.google/docs/hooks) | `~/.gemini/config/hooks.json` | `.agents/hooks.json` | hook ごとに 1 つの名前付きブロック |
| [Pi](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/extensions.md) | `~/.pi/agent/extensions/skillshare-NAME.ts` | `.pi/extensions/skillshare-NAME.ts` | Native extension code |
| [Amp](https://ampcode.com/docs/plugin-api) | `~/.config/amp/plugins/skillshare-NAME.ts` | `.amp/plugins/skillshare-NAME.ts` | Native plugin code |
| [OpenCode](https://opencode.ai/docs/plugins/) | `~/.config/opencode/plugins/skillshare-NAME.ts` | `.opencode/plugins/skillshare-NAME.ts` | Supplied v1/v2 plugin code |

アカウント target の `config_dir` を指す場合を除き、global scope はネイティブ設定ディレクトリの環境変数 override を使います。project scope はプロジェクト内にのみ書き込み、global にフォールバックしません。Codex inline TOML など他のソースは別のままです。Antigravity とその CLI（`agy`）は同じ `hooks.json` を読みます。各 hook は名前付きの 1 ブロックで、インポートしても名前は変わりません。CLI の `~/.gemini/antigravity-cli/settings.json` にある hooks は別のままです。Droid の独立 hooks ファイルは読み込むソースを変えるため、既存 inline hooks を先に確認してください。 Copilot は `.github/hooks` のプロジェクト hooks を信頼済みフォルダーでのみ読み込みます。


Droid の inline hooks が有効な場合、同期は独立ファイルの作成を拒否します。インポートして確認し、元の inline hooks を削除してから同期してください。

## Agent の別のアカウント {#accounts}

`agent` と `config_dir` を持つ target は、その名前で hooks を受け取れます。Claude、Codex、Pi のアカウントは Agent のネイティブ binding 形式を使います。

```yaml
targets:
  codex-2:
    agent: codex
    config_dir: ~/.codex-2
    skills: {enabled: false}
hooks:
  entries:
    check:
      bindings:
        codex-2:
          events:
            Stop:
              - hooks:
                  - type: command
                    command: "echo checked"
```

| アカウントの Agent | 保存先 |
|---|---|
| `claude` | `<config_dir>/settings.json` |
| `codex` | `<config_dir>/hooks.json` |
| `pi` | `<config_dir>/extensions/skillshare-NAME.ts` |

command のスクリプトは `<config_dir>/hooks/skillshare/NAME/` に入ります。アカウントは global のみです。project 設定と `hooks.projects` では、すべてのアカウントが同じプロジェクトファイルを読むため Agent 自体を指定します。`config_dir` がなければ警告してスキップし、アカウントのホームを作成しません。

`CODEX_HOME`、`CLAUDE_CONFIG_DIR`、`PI_CODING_AGENT_DIR` が宣言済みアカウントのディレクトリを指すと、通常の Agent binding は組み込みの既定ホームを使い、plan に警告が表示されます。各アカウントは自分の `config_dir` を使い続けます。それ以外の override は従来どおりです。symlink を含め、同じディレクトリに 2 つの binding key を同期することはできません。

target が削除された、ホームが移動した、またはアカウントのホームが存在しない場合、所有出力とその記録をそのまま残し、警告します。target やディレクトリを戻せば管理を再開できます。ファイルを変えずに特定エントリの保留中の所有記録を解放するには、`skillshare hooks sync NAME --replace -g` をプレビューして実行します。target の削除時に binding がまだその名前を使っていれば警告します。次の hooks 同期前に binding も外してください。

`skillshare hooks import --from codex-2 -g --json` でアカウントからインポートできます。ダッシュボードには **codex-2 (Codex)** と表示され、Codex の event とエディターを使います。Codex の各ホームで変更された hooks を `/hooks` から確認し、信頼する必要があります。同期は信頼を変更せず、hooks を実行しません。

## プロジェクト、競合と復元

global 設定の `hooks.projects` は絶対プロジェクトパスを同じ Entry 形式の `entries` に対応づけます。**Projects → Hooks** で管理できます。独自の `.skillshare/config.yaml` があるプロジェクトは project scope で管理します。プロジェクト同期はそのルートだけを適用します。

無関係な設定と所有していない hooks は保持します。内容の一致だけでは所有を判断しません。所有出力が外部編集されると、無効化、削除、復元も競合になります。明示的な置換は選択 entry のみが対象です。共有ファイルでは、plan の各行に entry が追加（`+`）、更新（`~`）、削除（`−`）する event を表示し、JSON plan の `events` にも含まれます。`update` は entry がそのファイルに登録を残すこと、`remove` はファイルから完全に外れることを意味します。編集はファイルの書式（コンパクトまたはインデント）を保ち、Skillshare が追加した `hooks` キーは最後の hook が外れると削除します。Skillshare が作成したファイルは、ほかに何も残っていなければ、作成して空になったフォルダーとともに削除し、プレビューではファイルの削除として表示します。以前からあったファイルやフォルダー、あなたが追加した内容は残ります。

バックアップは後から加わった無関係な変更を保持してネイティブ出力を復元し、ソース定義は変更しません。**Settings → Backups → Hooks** または `hooks restore` でプレビューして復元します。

**Synced** は Skillshare が設定を書いた状態です。Agent の手順に従って再起動／再読み込みしてください。信頼、hooks の有効化、コード互換性は Agent が制御します。管理時に hook command を実行したり、ネイティブ信頼を自動変更したりしません。
