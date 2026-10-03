---
sidebar_position: 4
---

# log

デバッグとコンプライアンスのために、永続的な operations ログと audit ログを閲覧します。

```bash
skillshare log                    # インタラクティブ TUI（TTY 上のデフォルト）
skillshare log --audit            # audit ログのみ表示
skillshare log --tail 50          # 直近 50 件のエントリを表示
skillshare log --cmd sync         # sync のエントリのみ表示
skillshare log --status error     # エラーのみ表示
skillshare log --since 2d         # 直近 2 日間のエントリ
skillshare log --stats            # サマリー統計を表示
skillshare log --json             # JSONL として出力
skillshare log --no-tui           # プレーンテキスト出力
skillshare log --clear            # operations ログをクリア
skillshare log -p                 # project の operations + audit ログ
```

## 使うタイミング

- ログエントリをインタラクティブに閲覧・フィルタする
- 失敗した操作で何が起きたかをデバッグする
- コンプライアンスやトラブルシューティングのために audit trail を確認する
- コマンド、ステータス、時間範囲でログをフィルタして調査する

## インタラクティブ TUI

TTY 上では、`skillshare log` は操作のインタラクティブなリストを開き、選択したエントリの詳細を横に表示します。フィルタと、コマンド別の統計画面もあります。操作ログと audit ログはマージされ、時刻順に並びます。キーは画面下部に表示されます。

`--no-tui` を使うと TUI をスキップし、代わりにプレーンテキストを出力します。

```bash
skillshare log --no-tui           # プレーンテキスト出力
skillshare log --no-tui | less    # 手動でページャーにパイプする
```

## 記録される内容

すべての変更を伴う CLI および Web UI の操作は、タイムスタンプ、コマンド、ステータス、所要時間、コンテキスト引数を含む JSONL エントリとして記録されます。

| コマンド | ログファイル |
|---------|------|
| `install`, `uninstall`, `sync`, `push`, `pull`, `collect`, `backup`, `restore`, `update`, `target`, `trash`, `config`, `check`, `diff`, `init`, `upgrade` | `operations.log` |
| `audit` | `audit.log` |

これらの API を呼び出す Web UI のアクションも、CLI の操作と同じ方法で記録されます。

## ログの種類

### デフォルトビュー
1 回の出力で**両方のセクション**を表示します。
- Operations ログ
- Audit ログ

```bash
skillshare log
```

### Audit 専用ビュー

セキュリティ audit のスキャンを通常の操作とは別に記録します。

```bash
skillshare log --audit
```

### フィルタリング

コマンド、ステータス、時間範囲で結果を絞り込みます。`--cmd` が特定のログのみを対象とする場合（例: `--cmd audit` は audit.log にのみ現れる）、関係のないセクションは自動的にスキップされます。

```bash
skillshare log --cmd install              # install のエントリのみ
skillshare log --status error             # エラーのみ
skillshare log --since 1h                 # 直近 1 時間（30m, 2d, 1w も可）
skillshare log --since 2026-01-15         # 特定の日付以降
skillshare log --cmd sync --status error  # フィルタの組み合わせ
```

### JSON 出力

スクリプトや自動化のために、生の JSONL を出力します。

```bash
skillshare log --json                     # 全エントリを JSONL で
skillshare log --json --cmd sync          # フィルタ済み JSONL
```

## 出力例（プレーンテキスト）

ターミナルで `--no-tui` を使う場合（パイプ時は各エントリの詳細が 1 行にまとめられます）:

```
Operations · last 2
✓ sync      2026-02-10 14:35 · 0.3s
            targets: 3
            scope: global

✗ sync      2026-02-10 14:31 · 0.8s · error
            targets: 3
            failed: 1
            scope: global
  ~/.local/state/skillshare/logs/operations.log

Audit · last 1
✗ audit     2026-02-10 14:36 · 1.1s · blocked
            scope: all-skills
            scanned: 12
            passed: 11
            failed: 1
            failed skills:
              - prompt-injection-skill
              - data-exfil-skill
  ~/.local/state/skillshare/logs/audit.log
```

## ログフォーマット

エントリは JSONL フォーマット（1 行につき 1 つの JSON オブジェクト）で保存されます。

```json
{"ts":"2026-02-10T14:30:00Z","cmd":"install","args":{"source":"anthropics/skills/pdf"},"status":"ok","ms":1200}
```

| フィールド | 説明 |
|-------|-------------|
| `ts` | ISO 8601 タイムスタンプ |
| `cmd` | コマンド名 |
| `args` | コマンド固有のコンテキスト（source、name、target など） |
| `status` | `ok`、`error`、`partial`、または `blocked` |
| `msg` | エラーメッセージ（ステータスが ok 以外の場合） |
| `ms` | 所要時間（ミリ秒） |

## ログの場所

```
~/.local/state/skillshare/logs/operations.log    # グローバルな operations
~/.local/state/skillshare/logs/audit.log         # グローバルな audit
<project>/.skillshare/logs/operations.log   # project の operations
<project>/.skillshare/logs/audit.log        # project の audit
```

## Git でログを追跡する（Project モード）

project モードでは、コミットが煩雑にならないよう、デフォルトで `.skillshare/logs/` は無視されます。

チームでログファイルをバージョン管理したい場合は、管理対象ブロックの後に `.skillshare/.gitignore` へ次の**ユーザーオーバーライド**ルールを追加してください。

```gitignore
# ユーザーオーバーライド: ログを追跡する
!logs/
!logs/*.log
```

リポジトリルートの `.gitignore` も `.skillshare/` を無視している場合は、そちらにも対応する unignore ルールを追加してください。

## オプション

| フラグ | 説明 |
|------|------------|
| `-a`, `--audit` | audit ログのみ表示 |
| `-t`, `--tail <N>` | 直近 N 件のエントリを表示（デフォルト: 20） |
| `--cmd <name>` | コマンド名でフィルタ（例: `sync`, `install`, `audit`） |
| `--status <status>` | ステータスでフィルタ（`ok`, `error`, `partial`, `blocked`） |
| `--since <dur\|date>` | 時間でフィルタ（`30m`, `2h`, `2d`, `1w`, または `2006-01-02`） |
| `--stats` | サマリー統計を表示（合計、成功率、コマンド別の内訳） |
| `--json` | 生の JSONL を出力（1 行につき 1 つの JSON オブジェクト） |
| `--no-tui` | インタラクティブ TUI を無効化し、プレーンテキスト出力を使用 |
| `-c`, `--clear` | 選択したログファイルをクリア（デフォルトは operations、`--audit` で audit） |
| `-p`, `--project` | project レベルのログを使用 |
| `-g`, `--global` | グローバルなログを使用 |
| `-h`, `--help` | ヘルプを表示 |

## Web UI

ログは Web ダッシュボードの `/log` でも利用できます。

```bash
skillshare ui
# Log ページに移動
```

Log ページでは以下が提供されます。
- `All`、`Operations`、`Audit` の **タブ**
- コマンド、ステータス、時間範囲（1h, 24h, 7d, 30d）の **フィルタ**
- 時間、コマンド、詳細、ステータス、所要時間を含む **テーブルビュー**
- 存在する場合に失敗／警告の Skill 名を示す **Audit の詳細行**
- **Clear** と **Refresh** のコントロール

## 統計ビュー

### CLI

```bash
skillshare log --stats                # 全操作のサマリー
skillshare log --stats --cmd sync     # sync のみの統計
skillshare log --stats --since 7d     # 直近 7 日間の統計
```

## ログの保持

ログは無制限に増え続けないよう、自動的に切り詰められます。デフォルトの上限はログファイルごとに**1000 エントリ**です（CLI または Web UI の各操作 = 1 エントリ）。`operations.log` と `audit.log` は別々に管理されます。

デフォルトを変更するには、`config.yaml` に以下を追加してください。

```yaml
log:
  max_entries: 500  # ファイルごとのエントリ数; 0 = 無制限（デフォルト: 1000）
```

## 関連項目

- [audit](/docs/reference/commands/audit) — セキュリティスキャン（audit.log に記録）
- [status](/docs/reference/commands/status) — 現在の同期状態を表示
- [doctor](/docs/reference/commands/doctor) — 問題を診断
