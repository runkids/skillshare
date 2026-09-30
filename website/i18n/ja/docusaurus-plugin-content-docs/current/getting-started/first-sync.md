---
sidebar_position: 2
---

# 初回の Sync

初回セットアップの全手順を順番に説明します。インストールから Sync が動くまでおよそ 5 分です。別マシンでの復元と、ヘッドレス環境での無人実行という 2 つのバリエーションは、このページの最後にまとめています。

## 前提条件

- macOS、Linux、または Windows
- AI CLI が 1 つ以上インストールされていること（Claude Code、Cursor、Codex など）

## 1. CLI をインストールする

**Homebrew（macOS / Linux）:**
```bash
brew install skillshare
```

:::note
Homebrew のリリースは数日遅れることがあります。最新版が必要な場合はインストールスクリプトを使ってください。
:::

**インストールスクリプト（macOS / Linux）:**
```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

スクリプトはデフォルトで `~/.local/bin` にインストールするため、通常のインストールと更新に `sudo` は不要です。インストーラーが PATH 設定の案内を表示した場合のみ、その案内に従ってから `skillshare` を実行してください。今後のターミナルでも使えるよう、案内された行を shell 設定ファイル（`~/.zshrc` や `~/.bashrc` など）に追加できます。別の場所には `INSTALL_DIR` を指定できます。

既存のインストールは移動・削除されません。PATH で古い実行ファイルが優先される場合、インストーラーがその場所を表示します。新しいインストール先を PATH の先頭に置いてください。保護されたカスタムディレクトリでは引き続き `sudo` が必要になる場合があります。

システム全体で共有する場合：

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | INSTALL_DIR=/usr/local/bin sh
```

**Windows（PowerShell）:**
```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

:::tip あとから更新する
`skillshare upgrade` はインストール方法（Homebrew、スクリプト、手動）を判別し、その場で CLI を更新します。
:::

## 2. 初期化する

```bash
skillshare init
```

<p>
  <img src="/img/init-with-mode.png" alt="Interactive init flow" width="720" />
</p>

`init` では 4 つの選択を順に行います:

1. **Source ディレクトリ** — 既定値は `~/.config/skillshare/skills/` です。Enter を押せばそのまま使えます。
2. **Git リモート** — 個人の Skill リポジトリの URL を貼り付けます（例: `git@github.com:you/skills.git`）。まだ無い場合は、先に GitHub で空のリポジトリを作ってください。スキップして後からリモートを追加することもできます。
3. **Target** — skillshare がインストール済みの AI CLI を検出して一覧表示します。確認するか、不要なものの選択を外してください。
4. **Built-in skill** — 任意です。`/skillshare` コマンドを追加し、AI CLI から skillshare を直接呼び出せるようになります。

### Sync モードを選ぶ

`init` は `--mode <merge|copy|symlink>` を受け付け、新しく追加される Target の既定モードを設定します:

- `merge`（既定）— Skill ごとの symlink。Target 側に元からあるローカルの Skill は保持されます
- `symlink` — Target ディレクトリ全体が 1 つの symlink になります（最速ですが、ディレクトリを置き換えます）
- `copy` — 実ファイル。変更は次回の `sync` で反映されます

Target ごとの上書き設定は、後から `skillshare target <name> --mode <mode>` で行えます。

## 3. Skill をインストールする

```bash
skillshare install anthropics/skills/skills/pdf
```

インストールのたびにセキュリティ監査が実行されます。Critical の検出があるとインストールはブロックされます。内容を確認しリスクを受け入れる場合にのみ `--force` を指定してください。

## 4. Sync する

```bash
skillshare sync
```

これで設定済みのすべての Target が Source を指すようになりました。

## 5. 確認する

```bash
skillshare status
```

```text
$ skillshare status

Source
─────────────────────────────────────────
✓ ~/.config/skillshare/skills (43 skills, 2026-09-28 12:39)
✓ ~/.config/skillshare/agents (2 agents, 2026-09-28 12:39)

Targets
─────────────────────────────────────────
claude
  skills   merged       [merge] ~/.claude/skills (43 shared, 0 local)
  agents   merged       [merge] 2/2 linked
cursor
  skills   merged       [merge] ~/.cursor/skills (43 shared, 0 local)
  agents   merged       [merge] 2/2 linked
gemini
  skills   merged       [merge] ~/.gemini/skills (43 shared, 0 local)
…
```

出力には Source のパスとすべての Target が並びます。`merge` モードで同期済みの Target は `merged` と表示され、shared の数にはいまインストールした Skill も含まれます。

ダッシュボード（`skillshare ui`）でも同じ状態をひと目で確認できます。

![初回同期後の Dashboard：1 つの Source がすべての Target につながり、すべて同期済み](/img/web-dashboard-demo.png)

---

## いま何が起きたのか

1. **`init`** が `~/.config/skillshare/config.yaml` と `~/.config/skillshare/skills/` を作成し、AI CLI を自動検出し、リモートを指定していた場合はそこにあった既存の Skill をクローンしました。
2. **`install`** が Skill を Source ディレクトリにクローンし、セキュリティ監査を実行しました。`.metadata.json` に upstream の URL とコミットが記録され、`skillshare update` で今後の変更を取得できます。
3. **`sync`** が各 Target に設定されたモードを適用しました。たとえば `merge` モードでは:
   ```
   ~/.claude/skills/pdf → ~/.config/skillshare/skills/pdf  (symlink)
   ```

`merge` と `symlink` モードでは、Source への編集がすべての Target に即座に現れます。`copy` モードでは次回の `sync` で反映されます。Target 側に元からあるローカルの Skill は `merge` と `copy` では保持されます。`skillshare backup` は破壊的な操作の前にスナップショットを取り、`skillshare restore <target>` で元に戻せます。

特定の Target だけ別のモードにしたい場合は、Target ごとに上書きします:

```bash
skillshare target <name> --mode copy
skillshare sync
```

判断の早見表は [Sync Modes](/docs/understand/sync-modes) を参照してください。

---

## バリエーション: 別マシンでの復元

すでに他の環境で skillshare を使っていて、GitHub に個人の Skill リポジトリがある場合です。新しいノート PC、devcontainer、VM では 4 つのコマンドですべてが復元できます。プロンプトも選択もなく、再実行しても冪等です:

```bash
# 1. CLI をインストール（Homebrew または curl|sh — 上のステップ 1 と同じ）
brew install skillshare

# 2. Skill リポジトリをクローンし、検出された Target を追加
skillshare init \
  --remote git@github.com:<you>/skills.git \
  --all-targets \
  --no-skill

# 3. tracked な依存を再インストール
#    （_ 始まりのディレクトリは gitignore されているため、クローンには含まれません）
skillshare install https://github.com/<your-company>/skills --track --force

# 4. Sync
skillshare sync
```

`--no-skill` は Built-in skill のプロンプトをスキップします。このマシンでも使いたくなったら、あとから `skillshare upgrade --skill` で追加できます。

---

## バリエーション: ヘッドレスセットアップ（TTY なし）

CI ジョブ、devcontainer の post-create フック、クラウド VM のプロビジョナー向けに、すべてのプロンプトには非対話用のフラグが用意されています:

```bash
skillshare init \
  --source ~/.config/skillshare/skills \
  --remote https://github.com/<you>/skills \
  --targets codex \
  --mode merge \
  --no-copy \
  --no-skill

skillshare install https://github.com/<your-company>/skills --track --force
skillshare sync
```

| フラグ | 効果 |
|---|---|
| `--source <path>` | Source パスのプロンプトをスキップ |
| `--remote <url>` | リモートのプロンプトをスキップ。リモートに内容があればクローン |
| `--targets <name>` | 指定した Target のみ追加（検出されたすべてを追加するには `--all-targets`） |
| `--mode merge` | 新しい Target の既定 Sync モード |
| `--no-copy` | 「既存の Target の Skill をコピーしますか？」のプロンプトをスキップし、空の状態で開始 |
| `--no-skill` | Built-in skill のプロンプトをスキップ |

`--targets`、`--all-targets`、`--no-targets` は排他的です。いずれか 1 つを選んでください。

---

## 次のステップ

- [自分の Skill を作る](/docs/how-to/daily-tasks/creating-skills)
- [マシン間で同期する](/docs/how-to/sharing/cross-machine-sync)
- [組織全体の Skill](/docs/how-to/sharing/organization-sharing)
- [Agents](/docs/understand/agents) — 単一ファイルの `.md` エージェントを Skill と並べて管理する
- [Sync modes](/docs/understand/sync-modes) — 判断の早見表とトレードオフ
