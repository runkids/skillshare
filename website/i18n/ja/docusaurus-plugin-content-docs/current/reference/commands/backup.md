---
sidebar_position: 2
---

# backup

ターゲットディレクトリのバックアップを作成、一覧表示、管理します。

```bash
skillshare backup              # すべての Skill ターゲットをバックアップ
skillshare backup claude       # 特定のターゲットをバックアップ
skillshare backup agents       # すべての agent ターゲットをバックアップ
skillshare backup --all        # Skill と agent をバックアップ
skillshare backup --list       # すべてのバックアップを一覧表示
skillshare backup --cleanup    # 古いバックアップを削除
skillshare backup --delete 2026-01-19_10-00-00  # 1 つのバックアップを削除
skillshare backup files        # skillshare が書き換えた単一ファイルのバージョン
```

## こんなときに使う

- 危険な変更を行う前に手動でバックアップを作成する
- 既存のバックアップを一覧表示して復元の選択肢を確認する
- 古いバックアップをクリーンアップする、または不要になったバックアップを 1 つ削除する
- `AGENTS.md` や `CLAUDE.md` などのファイルの以前のバージョンを取り戻す

## 自動バックアップ

バックアップは、以下の前に **自動的に** 作成されます。
- `skillshare sync`（Skill ターゲットと agent ターゲット）
- `skillshare sync agents`（agent ターゲットのみ）
- `skillshare target remove`

保存場所: `~/.local/share/skillshare/backups/<timestamp>/`（グローバル）、`.skillshare/backups/`（プロジェクトモード、agent のみ）

保持ポリシーは、自動バックアップのたびに `--cleanup` と同じポリシーで自動的に適用されます。スナップショットを手動で整理する必要はありません。

## コマンド

### バックアップの作成

```bash
skillshare backup              # すべてのターゲット
skillshare backup claude       # 特定のターゲット
skillshare backup --dry-run    # プレビュー
```

### バックアップの一覧表示

```bash
skillshare backup --list
```

```
All backups (15.3 MB total)
  2026-01-20_15-30-00  claude, cursor     4.2 MB  ~/.local/share/.../2026-01-20_15-30-00
  2026-01-19_10-00-00  claude             2.1 MB  ~/.local/share/.../2026-01-19_10-00-00
  2026-01-18_09-00-00  claude, cursor     4.0 MB  ~/.local/share/.../2026-01-18_09-00-00
```

### 古いバックアップのクリーンアップ

```bash
skillshare backup --cleanup           # 古いバックアップを削除
skillshare backup --cleanup --dry-run # クリーンアップをプレビュー
```

デフォルトのクリーンアップポリシー:
- 直近 10 個のバックアップを保持
- 30 日以上前のバックアップを削除
- 合計サイズの上限を 500 MB に設定

最新のスナップショットは、それ単体でサイズ上限を超えていても常に保持されます — 復元ポイントがまったくない状態にはなりません。

このポリシーはすべての `sync` の後に自動的に実行されるため、`--cleanup` はオンデマンドで整理したいときにのみ必要です。

### バックアップの削除

```bash
skillshare backup --delete 2026-01-19_10-00-00            # 1 つのスナップショットを削除
skillshare backup --delete 2026-01-19_10-00-00 --dry-run  # 削除される内容を表示
skillshare backup --delete 2026-01-19_10-00-00 -p         # プロジェクトの .skillshare/backups/ から削除
```

タイムスタンプは `--list` が表示するフォルダ名です。スナップショット全体が、含まれるすべてのターゲットとともに削除されます。

### ファイル履歴 {#file-history}

skillshare は、単一ファイル（`AGENTS.md` や `CLAUDE.md` などの指示ファイル、または[共有ファイル](/docs/how-to/daily-tasks/sharing-instructions#backups)の配置先）を書き換えたり置き換えたりする前に、古い内容を保存します。`backup files` はそれらのバージョンを一覧表示し、復元します。

```bash
skillshare backup files                                   # 保存済みバージョンがあるファイル
skillshare backup files show ~/.claude/CLAUDE.md          # 1 つのファイルのバージョン（新しい順）
skillshare backup files restore ~/.claude/CLAUDE.md origin
skillshare backup files restore ./CLAUDE.md 1769000000000000000.shim --dry-run
```

```
Versions of /Users/me/.claude/CLAUDE.md
  1769000000000000000.edit          2026-01-21 12:53:20  history/edit          2.1 KB  # Team rules
  drift:1768900000000000000.mode    2026-01-20 09:06:40  drift/mode            1.9 KB  # Team rules
  origin                            2026-01-10 08:00:00  origin                1.2 KB  # My notes
```

各バージョンには ID があります。

| ID | 種類 | 意味 |
|----|------|---------|
| `<time>[.<reason>]` | `history` | skillshare がファイルに書き込む前に保存したもの |
| `drift:<time>[.<reason>]` | `drift` | skillshare が置き換えた、あなた自身の編集 |
| `origin` | `origin` | 共有ファイルを最初につないだ時点のファイル。その配置先を削除すると自動的に復元されます。ファイルがなかった場合、復元すると現在のファイルが削除されます |

理由（reason）は、skillshare が何をしようとしていたかを示します。

| 種類 | 理由 | 保存されるタイミング |
|------|--------|--------------|
| `history` | `convert` | ファイルを `AGENTS.md` に変換する、または `AGENTS.md` に名前変更する前 |
| `history` | `shim` | プロジェクトのファイルに `@AGENTS.md` を追加する前 |
| `history` | `edit` | ダッシュボードでの編集の前 |
| `history` | `collect` | ターゲットの変更を共有ファイルに取り込む前 |
| `history` | `attach` | 最初につないだときに共有ファイルで置き換える前 |
| `history` | `restore` | 古いバージョンを復元する前 |
| `history` | `migrate` | 0.23.0 で廃止された MCP 設定を除いて sync が config を保存する前。[0.22 からの Pi のアップグレード](/docs/reference/commands/mcp#pi-migration)を参照 |
| `drift` | `overwrite` | ファイルを直接編集し、**上書き** を選んだとき |
| `drift` | `mode` | 配置先のモードを切り替える前 |
| `drift` | `restore` | 配置先を復元する前 |

以前のリリースで保存されたバージョンには理由がありません。各ファイルについて、種類ごとに最新 10 件が保持されます。

`restore` は、まず現在の内容を理由 `restore` の新しいバージョンとして保存してから、選んだバージョンを書き込みます。パスがシンボリックリンクの場合は、`--unlink` を付けない限り拒否されます。`--unlink` はリンクを通常のファイルに置き換えます。

`backup files` はモードに従います。プロジェクト内（または `-p` 指定時）では、そのプロジェクト内のファイルのみを一覧表示し、`show` / `restore` はプロジェクト外のパスを拒否します。`-g` はすべてのファイルが対象です。`files` はサブコマンドなので、文字どおり `files` という名前のターゲットをバックアップするには `skillshare backup -t files` を使います。

## ダッシュボード {#dashboard}

[`skillshare ui`](/docs/reference/commands/ui) の **設定 › バックアップ** には 3 つのタブがあります。

- **ターゲットフォルダー** — 上記のスナップショットを日付ごとに表示します。ターゲットまたは **agents のみ** でフィルタできます。スナップショットを開くと各フォルダーのファイル数とサイズが表示され、どれか 1 つを **復元**（Skill と agent のエントリのどちらも）したり、**パスをコピー** や **このバックアップを削除** ができます。**今すぐバックアップ** と **古いバックアップを整理** は、`backup` と `--cleanup` に相当します。
- **ファイル** — 上記のファイル履歴。ファイルを選ぶと理由付きでバージョンが表示され、**プレビューして復元** で現在のファイルとの差分、またはバージョン全体を確認できます。リンクされた配置先は、**復元してリンクを切る** を確認した後にのみ通常のファイルに置き換えられます。
- **MCP** — MCP 設定の書き込み前に毎回取られるバックアップ。Agent の設定ごとにグループ化され、それぞれで追加・変更・削除されたサーバーが表示されます。**プレビューして復元** は **MCP** ページと同じ復元ダイアログを開きます（コマンドラインでは [`mcp restore`](/docs/reference/commands/mcp)）。

![設定 › バックアップ › ファイル: 復元前に以前の CLAUDE.md を確認する](/img/backup-files-preview.png)

プロジェクトモードでは、このページはプロジェクトのみを対象とします。`.skillshare/backups/` にある agent のスナップショット、プロジェクト内のファイル、プロジェクトの MCP 設定のバックアップです。削除した Skill と agent はここにはありません。**スキル** と **エージェント** の **ゴミ箱** タブに移動します。

## オプション

| フラグ | 説明 |
|------|-------------|
| `--all` | Skill と agent の両方をバックアップ |
| `--project, -p` | プロジェクトモードを使用（`.skillshare/backups/`）。**agent のみ** |
| `--global, -g` | グローバルモードを使用（Skill のデフォルト） |
| `--list, -l` | すべてのバックアップを一覧表示（`-p` ではプロジェクトのもの） |
| `--cleanup, -c` | 古いバックアップを削除（`-p` ではプロジェクトのもの） |
| `--delete <timestamp>` | 1 つのバックアップを削除。`-p` 指定時は `.skillshare/backups/` から削除 |
| `--target, -t <name>` | 特定のバックアップを対象にする（位置引数の代替） |
| `--dry-run, -n` | 変更を加えずにプレビュー |

`backup files` には独自のオプションがあります: `--project, -p`、`--global, -g`、`restore` 用の `--unlink` と `--dry-run, -n`。[ファイル履歴](#file-history) を参照してください。

`backup` は位置引数として種類も受け付けます。`skillshare backup agents` は、バックアップを agent ターゲットのみに限定します。

## バックアップの構造

```
~/.local/share/skillshare/backups/
├── 2026-01-20_15-30-00/
│   ├── claude/
│   │   ├── skill-a/
│   │   └── skill-b/
│   └── cursor/
│       ├── skill-a/
│       └── skill-b/
└── 2026-01-19_10-00-00/
    └── claude/
        └── ...
```

存在する Skill ディレクトリは、ターゲットのモードによって異なります — [何がバックアップされるか](#what-gets-backed-up) を参照してください。

## 何がバックアップされるか {#what-gets-backed-up}

バックアップが保護するのは、`sync` が破壊し得るものだけです。すなわち **ターゲットには存在するがソースには存在しないローカルコンテンツ** です。

- ターゲット内の通常のファイルやディレクトリはバックアップされます
- merge モードのターゲットにあるシンボリックリンクは **スキップ** されます — これらはソース（唯一の信頼できる情報源）を指しており、すでに安全だからです。`skillshare sync` がこれらを再作成します

つまり:
- merge モードでは: ローカル（シンボリックリンクされていない）Skill のみがバックアップされます。sync 済みの Skill はソースに存在します
- copy モードでは: 管理下のすべての Skill ディレクトリがバックアップされます（実体ファイルであるため）
- symlink モードでは: 何もバックアップされません（ディレクトリ全体が単一のシンボリックリンクであるため）

ターゲットにシンボリックリンクしか含まれていない場合、バックアップは作成されず、`backup` は「何もすることがない」と報告します — 中身のない復元ポイントは意味がないからです。

## バックアップとディスク容量 {#backups--disk-space}

バックアップはソースをコピーしないため、サイズは小さく保たれます。混同しやすい 3 つの独立した仕組みがあります。

| 仕組み | 範囲 | 制御する対象 |
|-----------|-------|------------------|
| ソース内の `.gitignore` | Git のみ | Git が追跡する対象。無視されたファイルもディスク上には存在する |
| `config.yaml` の `ignore:` | `sync` | `sync` がターゲットにコピーするファイル（主に copy モード）。[sync](/docs/reference/commands/sync) を参照 |
| Backup | スナップショット | ローカルのターゲットコンテンツのみ — シンボリックリンク、したがってソースの成果物は除外される |

シンボリックリンクされた Skill は辿られないため、ソース Skill 内にある重いアーティファクト（モデルの重み、`.venv`、ブラウザプロファイル、メディアなど）は、`.gitignore` や `ignore:` に記載されているかどうかにかかわらず、スナップショットに **決して** コピーされません。

保持処理はすべての `sync` の後に、以下のデフォルトポリシーで自動的に実行されます。使用状況を手動で調べるには:

```bash
du -sh ~/.local/share/skillshare/backups   # ディスク上の合計サイズ
skillshare backup --list                   # スナップショットごとのサイズ
skillshare backup --cleanup --dry-run      # 保持ポリシーが何を削除するかプレビュー
```

Copy モードのターゲットは、スナップショットが依然として大きくなり得る唯一のケースです。これらは実体ファイルであるため、Skill ディレクトリ配下のあらゆるものがコピーされます。ランタイムキャッシュや大きなアーティファクトは Skill ツリーの外に置くか、`ignore:` で除外して、そもそもターゲットに届かないようにしてください。

## Agent のバックアップ {#agent-backup}

Agent には、Skill のバックアップと並行して動作する独自のバックアップフローがあり、知っておくべき 2 つの違いがあります。

**エントリの命名。** Agent のバックアップは、各タイムスタンプディレクトリ内の `<target>-agents/` に、Skill のバックアップと並んで保存されます。例えば、`skillshare backup --all` の後のレイアウトは次のようになります。

```
~/.local/share/skillshare/backups/2026-01-20_15-30-00/
├── claude/          # Skills backup for claude
├── claude-agents/   # Agents backup for claude
└── cursor/
```

**プロジェクトモードは Skill と逆になります。** プロジェクトモード（`-p`）では、`backup` は Skill ターゲットのバックアップを拒否しますが、agent ターゲットのバックアップは **行います**。`agents` フィルタを忘れると、次のエラーが表示されます。

```
backup is not supported in project mode (except for agents)
```

そのため、プロジェクトモードでは `skillshare backup -p agents` または `skillshare backup -p --all` のいずれかを指定する必要があります。`--list -p` と `--cleanup -p` は指定不要で、`.skillshare/backups/` を対象にします。

```bash
skillshare backup agents                  # すべての agent ターゲット（グローバル）
skillshare backup agents claude           # claude の agent のみ
skillshare backup agents -p               # プロジェクトの agent ターゲット
skillshare backup --all                   # 一度に Skill と agent の両方
```

agent のリソースモデルについては [Agents](/docs/understand/agents) を、復旧については [restore](/docs/reference/commands/restore) を参照してください。

## 関連項目

- [restore](/docs/reference/commands/restore) — バックアップから復元
- [sync](/docs/reference/commands/sync) — 自動的にバックアップを作成
- [target remove](/docs/reference/commands/target) — 自動的にバックアップを作成
