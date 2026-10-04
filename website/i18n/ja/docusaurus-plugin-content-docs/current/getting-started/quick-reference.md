---
sidebar_position: 4
---

# クイックリファレンス

skillshare のコマンド早見表です。

## 基本コマンド

| コマンド | 説明 |
|---------|-------------|
| `init` | 初回セットアップ |
| `install <source>` | Skill を追加 |
| `uninstall <name>...` | 1 つ以上の Skill を削除 |
| `list` | すべての Skill を一覧表示 |
| `search <query>` | Skill を検索 |
| `sync` | すべての Target へ反映 |
| `status` | Sync の状態を表示 |

## Skill の管理

| コマンド | 説明 |
|---------|-------------|
| `new <name>` | 新しい Skill を作成 |
| `update <name>` | Skill を更新（git pull） |
| `update --all` | すべての Tracked repo を更新 |
| `check` | Skill の更新を確認 |
| `check --json` | 更新を確認（JSON 出力） |
| `upgrade` | CLI と Built-in skill をアップグレード |
| `follow <name> --to <dir>` | 外部ディレクトリを source にリンクしてたどる |
| `unfollow <name>` | たどるのをやめてリンクを削除 |
| `hub list` | 設定済みの Skill Hub を一覧表示 |
| `hub add <url>` | Skill Hub を追加 |

## Target の管理

| コマンド | 説明 |
|---------|-------------|
| `target list` | すべての Target を一覧表示 |
| `target <name>` | Target の詳細を表示 |
| `target <name> --mode <mode>` | Sync モードを変更 |
| `target add <name> <path>` | カスタム Target を追加 |
| `target remove <name>` | Target を安全に削除 |
| `diff [target]` | 差分を表示 |

## Extras の管理

| コマンド | 説明 |
|---------|-------------|
| `extras init <name> --target <path>` | extras エントリを設定に追加 |
| `extras init <name> --file <file> --target <path>` | 1 つのファイルを sync する extra を追加（`--as` で Target でのファイル名を変更） |
| `extras list` | 設定済みの extras を Sync 状態つきで一覧表示 |
| `extras remove <name>` | extras エントリを設定から削除 |
| `extras <name> --add-target <path>` | 既存の extras エントリに Target を追加 |
| `extras <name> --remove-target <path>` | Target を削除（同期済みファイルも消すには `--prune`） |
| `extras collect <name>` | extras の Target からローカルファイルを Source へ collect |

## Agent の管理

| コマンド | 説明 |
|---------|-------------|
| `list agents` | インストール済みの Agent を一覧表示 |
| `install <source> --kind agent` | リポジトリから Agent のみをインストール |
| `install <source> -a <name>` | 指定した Agent のみをインストール |
| `uninstall --kind agent <name>` | Agent を削除 |
| `sync agents` | Agent のみを Target へ同期 |
| `check agents` | Agent の更新を確認 |
| `audit agents` | Agent をセキュリティスキャン |
| `enable --kind agent <name>` | 無効化した Agent を再度有効化 |
| `disable --kind agent <name>` | `.agentignore` で Agent を無効化 |

## Plugin の管理

| コマンド | 説明 |
|---------|-------------|
| `plugin` | 対話式の plugin マネージャーを開く |
| `plugin list` | 管理下の plugin とネイティブのインストール状態を表示 |
| `plugin discover <source>` | ディレクトリまたは Git リポジトリを調べる |
| `plugin add [source]` | 完全なネイティブ plugin をインストール |
| `plugin import [plugin@market] --from claude` | 既存のネイティブインストールを取り込む |
| `plugin inspect <name>` | 管理下のパッケージを調べる |
| `plugin check [name]` | 適用せずにソースの変更を確認 |
| `plugin update [name] --target claude` | 確認済みのソースから対応 Target を更新 |
| `plugin enable [name] --target codex` | 次回の Sync 対象として Target を選択 |
| `plugin disable [name] --target codex` | 次回の Sync 対象から Target を除外 |
| `plugin remove [name]` | 管理下のバインディングをアンインストールし、定義を削除 |
| `sync plugins [name]` | plugin の Sync 選択を適用。`plugin sync` のエイリアス |

対応 Target: Claude Code、Codex、Cursor、Antigravity（`agy`）、Pi、OpenCode。
Project mode が対応するのは Claude、Antigravity、Pi、OpenCode です。

enable / disable は選択を保存するだけです。次回の plugin sync で、選択されたバインディングが
インストールされ、選択を外されたバインディングは定義を残したままアンインストールされます。
plugin は `sync --all` の対象外です。変更内容を事前確認するには `--dry-run --json` を、
自動化には明示的な入力とあわせて `--no-tui` を使ってください。ネイティブクライアントの要件と
対応 Target については [plugin](/docs/reference/commands/plugin) を参照してください。

## Sync 操作

| コマンド | 説明 |
|---------|-------------|
| `sync extras` | Skill 以外のリソース（rules、commands など）を同期 |
| `sync mcp` | MCP の接続設定を同期 |
| `sync --all` | Skill + Agent + extras + MCP + hooks を同期（plugin は除く） |
| `collect <target>` | Target から Source へ Skill を collect |
| `collect --all` | すべての Target から collect |
| `backup [target]` | バックアップを作成 |
| `backup --list` | バックアップを一覧表示 |
| `restore <target>` | バックアップから復元 |
| `commit [-m "msg"]` | push せずにローカルの git コミットを作成 |
| `push [-m "msg"] [--pull]` | コミットして git リモートへ push。`--pull` を付けると先に remote の変更をマージし、その後 sync |
| `pull` | git から pull して同期 |
| `trash list` | ソフト削除された Skill を一覧表示 |
| `trash restore <name>` | ソフト削除された Skill を復元 |

## ユーティリティ

| コマンド | 説明 |
|---------|-------------|
| `analyze` | コンテキストウィンドウの使用量を分析（対話式 TUI） |
| `analyze --filter <text>` | 名前・パスの部分一致で Skill を絞り込み |
| `analyze --json` | コンテキスト使用量を JSON で出力 |
| `doctor` | 問題を診断 |
| `doctor --json` | 問題を診断（CI 向け JSON 出力） |
| `log` | 操作ログと監査ログを表示 |
| `ui` | `localhost:19420` で Web ダッシュボードを起動 |
| `ui -p` | Web ダッシュボードを Project mode で起動 |
| `completion <shell> --install` | シェルのタブ補完をインストール（bash/zsh/fish/powershell/nushell） |
| `version` | CLI のバージョンを表示 |
| `make test-docker` | オフラインの Docker sandbox テストを実行 |
| `make playground` | playground を起動してシェルに入る（1 ステップ） |
| `make playground-down` | playground を停止して削除 |
| `./scripts/sandbox.sh <cmd>` | sandbox の詳細管理（up/down/shell/reset/status/logs/bare） |
| `make ui-build` | フロントエンドをビルド |
| `make build-all` | フロントエンドを含む完全なバイナリ |

---

## よくあるワークフロー

### Skill をインストールして同期する
```bash
skillshare install anthropics/skills/skills/pdf
skillshare sync
```

### Skill を作成して配布する
```bash
skillshare new my-skill
# ~/.config/skillshare/skills/my-skill/SKILL.md を編集
skillshare sync
```

### マシン間の同期
```bash
# セットアップ（どちらか 1 つ）
# 対話式（プロンプトに従って進める）
skillshare init --remote git@github.com:you/my-skills.git

# 非対話式（プロンプトなし、インストール済み Target を自動検出）
skillshare init --remote git@github.com:you/my-skills.git --no-copy --all-targets --no-skill

# push せずにローカルのチェックポイントを作る（任意）
skillshare commit -m "Save local skill edits"

# マシン A: 変更を push
skillshare push -m "Add new skill"

# マシン B: pull して同期
skillshare pull

# 複数のマシンで編集する場合: 1 つのコマンドで双方向に同期
skillshare push --pull -m "Update skills"
```

任意（セットアップ後に AI CLI を追加でインストールした場合のみ）:

```bash
skillshare init --discover
```

discover 時にモードを上書きすると、新しく追加された Target だけが対象になります:

```bash
skillshare init --discover --select cursor --mode copy
```

### チームでの Skill 共有
```bash
# チームのリポジトリをインストール
skillshare install github.com/team/skills --track

# 特定のブランチからインストール（--track の有無を問わず利用可能）
skillshare install github.com/team/skills --branch develop --all
skillshare install github.com/team/skills --track --branch develop

# タグまたは commit SHA に固定（通常インストールのみ）
skillshare install github.com/team/skills --branch v1.2.0 --all

# チームの変更を取り込む
skillshare update --all
skillshare sync
```

### Sandbox playground のセッション
```bash
make playground          # 起動してシェルに入る
skillshare --help
ss status
exit                     # シェルを抜ける
make playground-down     # コンテナを停止
```

---

## 主なパス

| パス | 説明 |
|------|-------------|
| `~/.config/skillshare/config.yaml` | 設定ファイル |
| `~/.config/skillshare/skills/.metadata.json` | インストール済み Skill のメタデータ（自動管理） |
| `~/.config/skillshare/skills/` | Skill の Source ディレクトリ |
| `~/.config/skillshare/agents/` | Agent の Source ディレクトリ |
| `~/.config/skillshare/extras/<name>/` | extras の Source ディレクトリ |
| `~/.local/state/skillshare/logs/` | 操作ログと監査ログ |
| `~/.local/share/skillshare/backups/` | バックアップディレクトリ |

---

## ほとんどのコマンドで使えるフラグ

| フラグ | 説明 |
|------|-------------|
| `--dry-run`, `-n` | 変更せずにプレビュー |
| `--help`, `-h` | ヘルプを表示 |

---

## 関連項目

- [Commands Reference](/docs/reference/commands) — コマンドの完全なドキュメント
- [Concepts](/docs/understand) — コアコンセプトの解説
