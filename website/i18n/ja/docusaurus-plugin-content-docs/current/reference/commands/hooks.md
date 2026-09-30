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
skillshare hooks restore BACKUP_ID --dry-run
skillshare hooks restore BACKUP_ID
```

サブコマンドなしでは entries を一覧表示します。`add`／`edit` は `--file` の Entry JSON/YAML を読み込みます。インポートで名前を省略すると候補（Agent の event またはファイルごとに 1 件）を表示し、名前を指定すると保存します。インポートを保存すると、読み取った登録をその場で引き継ぎます。次回の同期は `--replace` なしで adopt し、インポートしなかった event は管理外のままです。設定とコードを読むだけで実行しません。追加、編集、インポート、有効化、無効化、削除はソースのみを保存し、`--sync` で同期します。同期と復元は適用前に再プレビューします。

| Option | Meaning |
|---|---|
| `--file PATH` | 追加／編集の Entry JSON/YAML、またはインポートのネイティブ設定／コード |
| `--from AGENT` | インポート元 Agent またはネイティブ形式 |
| `--sync` | 保存して同期 |
| `--replace` | 既存ソース entry またはその entry の競合出力を明示的に置換 |
| `--dry-run, -n` | 書き込まずプレビュー |
| `--json` | 構造化出力 |
| `--revision ID` | 指定プレビュー revision との一致を要求 |
| `--global, -g` | global 設定 |
| `--project, -p` | project 設定 |

`hooks list --json` でバックアップ ID と完全な保存先を確認します。設定が変わると古いプレビューは無効になるため、保存／同期前に更新してください。

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

| Option | Meaning |
|---|---|
| `description` | 任意の説明 |
| `enabled` | 既定 true。false はソースを保持し、次の同期で未変更の所有出力を削除 |
| `bindings` | Agent ID ごとのネイティブ binding |
| `bindings.AGENT.events` | 設定型 Agent のネイティブ event map |
| `bindings.AGENT.code` | Pi、Amp、OpenCode のネイティブ extension/plugin ソース |
| `bindings.AGENT.files` | 相対ファイル名を key とした任意の UTF-8 スクリプト |

Agent ID は `claude`、`codex`、`gemini`、`copilot`、`cursor`、`droid`、`qwen`、`antigravity`、`pi`、`amp`、`opencode`。`factory` は `droid` の、`antigravity-cli` と `agy` は `antigravity` の別名です。event、matcher、handler type、command、timeout 単位、payload はネイティブ形式のまま保持し、自動変換しません。event 名は各 command Agent のドキュメントにある event と照合します。未知の名前（例：綴り違いの `Stopp`）はプレビューと plan の `warnings` に警告として表示されますが、Agent は event を追加していくため同期は止めません。Pi、Amp、OpenCode のコードは確認しません。Pi、Amp、OpenCode のコードと imports はインストール済みバージョンに合わせ、専用の `skillshare-NAME.ts` に出力します。共通実行エンジンは生成しません。command binding のスクリプトは Agent 設定ディレクトリの `hooks/skillshare/NAME/` に保存され、command 内の指定 macro／パスは変更しません。プレビューで完全なパスを確認してください。

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

global scope はネイティブ設定ディレクトリの環境変数 override を使います。project scope はプロジェクト内にのみ書き込み、global にフォールバックしません。Codex inline TOML など他のソースは別のままです。Antigravity とその CLI（`agy`）は同じ `hooks.json` を読みます。各 hook は名前付きの 1 ブロックで、インポートしても名前は変わりません。CLI の `~/.gemini/antigravity-cli/settings.json` にある hooks は別のままです。Droid の独立 hooks ファイルは読み込むソースを変えるため、既存 inline hooks を先に確認してください。


Droid の inline hooks が有効な場合、同期は独立ファイルの作成を拒否します。インポートして確認し、元の inline hooks を削除してから同期してください。

## プロジェクト、競合と復元

global 設定の `hooks.projects` は絶対プロジェクトパスを同じ Entry 形式の `entries` に対応づけます。**Projects → Hooks** で管理できます。独自の `.skillshare/config.yaml` があるプロジェクトは project scope で管理します。プロジェクト同期はそのルートだけを適用します。

無関係な設定と所有していない hooks は保持します。内容の一致だけでは所有を判断しません。所有出力が外部編集されると、無効化、削除、復元も競合になります。明示的な置換は選択 entry のみが対象です。共有ファイルでは、plan の各行に entry が追加（`+`）、更新（`~`）、削除（`−`）する event を表示し、JSON plan の `events` にも含まれます。`update` は entry がそのファイルに登録を残すこと、`remove` はファイルから完全に外れることを意味します。編集はファイルの書式（コンパクトまたはインデント）を保ち、Skillshare が追加した `hooks` キーは最後の hook が外れると削除します。

バックアップは後から加わった無関係な変更を保持してネイティブ出力を復元し、ソース定義は変更しません。**Settings → Backups → Hooks** または `hooks restore` でプレビューして復元します。

**Synced** は Skillshare が設定を書いた状態です。Agent の手順に従って再起動／再読み込みしてください。信頼、hooks の有効化、コード互換性は Agent が制御します。管理時に hook command を実行したり、ネイティブ信頼を自動変更したりしません。
