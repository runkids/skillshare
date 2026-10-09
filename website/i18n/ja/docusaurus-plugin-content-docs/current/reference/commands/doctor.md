---
sidebar_position: 1
---

# doctor

環境をチェックし、skillshare のセットアップに関する問題を診断します。

```bash
skillshare doctor
skillshare doctor -p        # Project mode（.skillshare/config.yaml）
skillshare doctor -g        # グローバルモードを強制
skillshare doctor --json    # CI 向けの構造化された JSON 出力
```

```text
skillshare doctor

Environment
✓ Config       ~/.config/skillshare/config.yaml
  Config dir   ~/.config/skillshare
  Data         ~/.local/share/skillshare
  State        ~/.local/state/skillshare
✓ Source       ~/.config/skillshare/skills · 43 skills
✓ Agents       ~/.config/skillshare/agents · 2 agents
  Skillignore  not configured
✓ Links        supported
! Git          not initialized (recommended for backup)
✓ Integrity    27/27 skills verified

Targets
✓ claude    skills  merged · merge · 43 shared
✓           agents  synced · merge · 2/2 linked
✓ cursor    skills  merged · merge · 43 shared, 1 local
✓           agents  synced · merge · 2/2 linked
✓ gemini    skills  merged · merge · 43 shared
…
! gemini will see content from: universal
  ~/.agents/skills ← universal
  suggestion: …
…
✗ claude: 1 broken symlink: frontend__css-review
…

Extras
✓ rules     2 files · 2/2 targets OK
✓ commands  1 file · 1/1 targets OK
✓ team      1 file · 4/4 targets OK

Storage
  Backups      last 2026-09-28_12-41-50 · 10m ago
  Trash        1 item, 247 B · oldest under a day

✗ 6 errors, 4 warnings · 1.2s

Next
  skillshare sync  bring the targets up to date
```

## 使うタイミング

- 何かがうまく動かないが、原因がわからないとき
- skillshare や OS をアップグレードした後
- すべての Target、git、シンボリックリンクが健全かを確認したいとき
- バグ報告をする前の最初の診断ステップとして

## チェック内容

```text
skillshare doctor

Environment
✓ Config       ~/.config/skillshare/config.yaml
  Config dir   ~/.config/skillshare
  Data         ~/.local/share/skillshare
  State        ~/.local/state/skillshare
✓ Source       ~/.config/skillshare/skills · 12 skills
✓ Agents       ~/.config/skillshare/agents · 8 agents
✓ Skillignore  2 patterns, 1 skill ignored
✓ Links        supported
✓ Git          initialized with remote
✓ Integrity    12/12 skills verified

Targets
✓ claude    skills  merged · merge · 8 shared, 2 local
✓           agents  synced · merge · 8/8 linked
✓ codex     skills  merged · merge · 8 shared
✓ cursor    skills  copied · copy · 8 managed
✓           agents  synced · merge · 8/8 linked

Extras
✓ commands  3 files · 1/1 targets OK
✓ rules     4 files · 1/1 targets OK

MCP, hooks and plugins
✓ MCP          all 2 servers OK
✓ Hooks        all 1 hook in sync
  Plugins      none configured

Storage
  Backups      last 2026-01-18_09-00-00 · 3d ago
  Trash        empty

Version
✓ CLI          0.23.5
✓ Skill        0.23.5

✓ All checks passed · 0.4s
```

## 実行されるチェック

### 環境

| チェック項目 | 検証内容 |
|-------|-----------------|
| Config | 設定ファイルが存在し、有効であること |
| Source | Source ディレクトリが存在し、読み取り可能であること |
| Agents | Agents source ディレクトリが存在すること（設定されている場合） |
| Skillignore | `.skillignore`（および `.skillignore.local`）の有効なパターンと、無視されている Skill 数 |
| Source link | skills source 直下にあるシンボリックリンクまたは Windows ジャンクションごとに 1 行。[`follow_source_links`](../targets/configuration.md#follow_source_links) がオフ（デフォルト）の場合: info として `not followed by discovery; its contents are invisible to skillshare. Set follow_source_links: true to follow it`。オンの場合: info として `followed as a directory (follow_source_links)`、または警告として `not followed: <reason>`（リンク先がない、source のルートまたはその親、sync target との重なり） |
| Links | システムがシンボリックリンクを作成できること |
| Git | リポジトリの状態と remote の設定 |

Source link の確認はグローバルモードとプロジェクトモードの両方で行われます。source のルートは discovery と同じ方法で解決され、直下のエントリだけを確認します。該当するリンクがなければ出力は追加されません。各リンクは `doctor --json` では `undeclared_source_links` チェックとしても現れ、status は `info`、ポリシーによってスキップされたリンクでは `warning` になります。

### Targets

各 Target には **skills** と **agents**（Agent が設定されている場合）のサブ項目が表示されます。
- Skills: パス、sync モード、sync 状態、共有/ローカルの件数
  - 「N skills not synced」には `sync` が配置する skill だけを数えます。`sync` が意図的にスキップするもの（`standard` / `prefixed` naming での無効な名前、または名前の衝突）は含みません
- Agents: sync モード、リンク済み件数、drift の検出。Developer Mode がオフの Windows では `merge` が `copy` と表示され、最新の管理対象コピーはリンク済みとして数えられます。skillshare が所有していない、内容が同じローカルファイルは保持されます。copy fallback では agent の件数に `local preserved` として別に表示されます（例：`0/1 linked, 1 local preserved`）。
- 壊れたシンボリックリンクがないこと
- 意図しないローカルの衝突を検出する Skill 重複チェック:
  - `merge` モード: スキップ（ローカルの Skill は想定内のため）
  - `copy` モード: マニフェストで管理されているコピーは無視され、ローカルで衝突しているコピーのみ警告
- 有効な include/exclude glob パターン
- Naming と mode：copy 以外の mode で `prefixed` に解決される Target はエラーになります。sync がスキップするためです
- 該当する場合、Target ごとの情報レベルの互換性ヒント（Target の優先順位の例: `cursor` → `antigravity` → `copilot` → `opencode`。これらの Target が存在しない場合はヒントなし）

### パスの重複

Doctor は、ランタイムのピッカーに到達する前に、Skill 重複のリスクを 2 種類のクラスとしてフラグ付けします。

**`shared_target_paths`** — 2 つ以上の有効な Target が同じプライマリパスに解決される場合に発生します。よくある原因: `universal` と、`~/.agents/skills` に書き込むツール（例: `warp`、`witsy`）の両方を有効にしている場合。

```text
! Shared path ~/.agents/skills ← universal, warp
```

解決方法: 重複している Target のいずれかを無効化するか、`skillshare target <name> --path <dir>` で別のパスを設定してください。

パスを共有する Target 同士で `include`、`exclude` のフィルター、`mode`、`target_naming` のいずれかが異なる場合（両方の Target が `symlink` モードの場合はフォルダ全体をリンクするため、フィルターと命名は影響しません）、同期のたびに一方の Target の設定どおりにフォルダが書き直され、もう一方の結果が打ち消されます。そのためフォルダが落ち着かず、`sync` には同じ保留中の変更が表示され続けます。Doctor はこのケースを示し、Target を削除する代わりに 1 つ（`universal` が含まれていればそれ）を残して残りの Skills 同期をオフにするよう提案します:

```text
! Shared path ~/.agents/skills ← codex, universal (different settings, so they undo each other on every sync)
  suggestion: Keep universal syncing skills to ~/.agents/skills and stop the rest with `skillshare target codex --skills=false`.
```

`sync` も同じ Target と実行すべきコマンドを表示し、ダッシュボードの同期ページには外す Target の Skills 同期を停止するボタンがあります。設定が同一の Target がパスを共有している場合は、上記の解決方法がそのまま当てはまります。

**`cross_target_discovery`** — ある有効な Target のランタイムが、別の有効な Target が書き込むディレクトリもスキャンすると文書化されている場合に発生します。例えば、以前のセットアップから残った設定では `codex` がレガシーな `~/.codex/skills` を指したままになっている一方、`universal` は `~/.agents/skills` に書き込みます — このディレクトリは Codex も読み込みます。両方を有効にすると、Codex は自身のコンテンツに加えて universal のコンテンツも見ることになります。

```text
! codex will see content from: universal
  ~/.agents/skills ← universal
```

解決方法: まずスキャンする側の Target（上の例では `codex`）を削除してください。そのランタイムは共有ディレクトリをすでに読み込んでおり、他のツールには影響しません。`skillshare target remove codex --dry-run` でプレビューできます。代わりに書き込み元（`universal`）を削除すると、`~/.agents/skills` を読み込む他のツールからもそれらの skill が見えなくなります。スキャンする側の Target が、書き込み元でフィルタされている skill を持つ場合に限り両方を残し、ランタイムのピッカーでの重複表示を受け入れてください。

OpenCode は `~/.claude/skills`、`~/.agents/skills`、自分のフォルダの間で同じ名前の skill を 1 つだけ残すため、source から両方に同期された skill は 1 回だけ読み込まれます。`opencode` については、OpenCode 自身のフォルダにない skill を他の target のフォルダから読み込む場合にだけ警告し、その skill を表示します。フォルダにない理由は、sync が入れなかった（`targets:`、include/exclude、または `target_naming: standard` によるスキップ）か、他のフォルダに手で置いたかのどちらかです：

```text
! opencode loads 1 skill missing from its own folder, from: claude
  ~/.claude/skills ← claude: claude-only
```

`claude` などの書き込み側が symlink mode で `opencode` がそうでない場合、そのフォルダは source そのものなので、OpenCode がそこからどの skill を読み込むかを判断できず、通常の警告を表示します。

対処：その skill を `opencode` にも同期するか、OpenCode を実行する環境で `OPENCODE_DISABLE_CLAUDE_CODE_SKILLS=1`（`.agents/skills` も読まない場合は `OPENCODE_DISABLE_EXTERNAL_SKILLS=1`）を設定します。skillshare が見えるのは自分の環境だけです。skillshare を実行する環境でもこの変数を設定すると、`doctor`、`sync`、dashboard はそのフォルダを読まれないものとして扱い、`doctor` はその旨を表示します：

```text
opencode skips ~/.claude/skills: OPENCODE_DISABLE_CLAUDE_CODE_SKILLS is set in this environment
```

OpenCode を別の環境（デスクトップのランチャーなど）から起動する場合は、そちらでも変数を設定してください。

`shared_target_paths` は設定済みのパスだけを読みます。`cross_target_discovery` は組み込みの `also_scans` テーブルも読み、`opencode` については各 target が受け取る skill も確認します。

### バージョン

- CLI のバージョン
- skillshare skill のバージョン（新しい skill が公開されていれば警告。`skillshare upgrade --skill`）
- 利用可能な更新のチェック

### Skill の整合性

ファイルハッシュのメタデータを持つインストール済み Skill について、doctor はインストール以降にファイルが改ざんされていないかを検証します。

- 現在の SHA-256 ハッシュを保存されているハッシュと比較
- Skill ごとに変更・欠落・追加されたファイルを報告
- `.metadata.json` に含まれないローカルの Skill は静かにスキップされます — これは想定内の挙動です
- メタデータはあるが `file_hashes` が欠落しているインストール済み Skill は、その名前とともにフラグ付けされます

```text
! Integrity    5/6 skills verified
!              _team-repo__api-helper: 1 modified, 1 missing
!              1 skill missing file hashes: _old-repo__legacy-skill
```

### Extras

Extras が設定されている場合、以下を検証します。
- 各 Extras の設定が有効であること（mode、`flatten`、`as`）、および複数の Extras が同じファイルを奪い合っていないこと
- 各 Extras の Source ディレクトリが存在すること
- Target ディレクトリに到達可能であること
- ディレクトリ Target 内の壊れたシンボリックリンク（error）
- Source と一致しないファイル。判定は `skillshare diff` と同じです（warning）。`flatten` または `extension` を設定した Target は比較しません。

```text
✗ rules     → ~/.claude/rules: broken symlink gone.md
!           → ~/.claude/rules: 1 file out of sync (a.md missing in target)
```

### MCP

[`mcp check`](./mcp.md) の静的チェックを実行します。参照している環境変数が設定されていること、`command` が `PATH` で見つかること、クライアントのルールがサーバーを受け入れること、各エントリが同期済みであることを確認します。Doctor はホストを解決せず、サーバーも起動しません。必要な場合は `skillshare mcp check` または `skillshare mcp check --live` を実行してください。サーバーが設定されていない場合は `info` を表示します。

```text
✗ MCP          docs: command no-such-mcp-binary was not found on PATH
!              docs → claude: not synced yet; run skillshare sync mcp
```

### Hooks

`skillshare sync hooks` を書き込みなしでプレビューします。プレビューの失敗は error です。Sync でまだ追加・更新・削除されるエントリ、ネイティブ Hooks との競合、注意喚起の警告（Agent が記載していないイベント名など）は warning です。Hook が設定されていない場合は `info` を表示します。

```text
! Hooks        bash-log → claude: not synced (add)
```

### Plugins

ソースを取得せずに `skillshare sync plugins` をプレビューします。Doctor は、プラグインパッケージがある場合にのみ、バインドされた各 Agent のネイティブ CLI にインストール済みの内容を問い合わせます。ブロックされたバインド（Agent の CLI が未インストールなど）と、まだ Sync が必要なバインドは warning です。ソースの新しいリリースの確認は引き続き `skillshare plugin check` が担当します。パッケージが設定されていない場合は `info` を表示します。

### その他

- `SKILL.md` ファイルがない Skill
- Skill レベルの `targets:` フィールド検証（未知の Target 名について警告）
- 最後のバックアップのタイムスタンプ（グローバルモード）
- Trash の状態（アイテム数、合計サイズ、最も古いアイテムの経過日数）
- Target 内の壊れたシンボリックリンク。リンク先が利用できない source リンク（マウントされていないドライブ）の背後にある Target リンクは、警告 `N links behind an unavailable source link, kept until it is back` として別に報告され、prune の提案はありません。`sync` は意図的にそれらを保持し、`doctor --json` の `broken_symlinks` チェックは `error` ではなく `warning` になります。

:::note Project mode
プロジェクトに `.skillshare/config.yaml` がある場合、`skillshare doctor` は自動的に Project mode で実行されます。

Project mode では:
- Config/Source のチェックには `.skillshare/config.yaml` と `.skillshare/skills` が使用されます
- Trash の状態には `.skillshare/trash` が使用されます
- バックアップは `not used in project mode` と表示されます
:::

## よくある問題

### "Needs sync"

Target のモードは変更されたものの、まだ適用されていません。

```bash
skillshare sync
```

### "Not synced"

Target のリンク済み Skill が Source より少ない状態です（新しい Skill をインストールした後など）。

```bash
skillshare sync
```

### "Has uncommitted changes"

トラッキング対象のリポジトリにローカルの変更があります。

```bash
cd ~/.config/skillshare/skills/_team-repo
git status
# 変更をコミットするか破棄する
```

### "Broken symlink"

Skill が Source から削除されたのに、シンボリックリンクが残っています。

```bash
skillshare sync  # 孤立したシンボリックリンクを削除します
```

代わりに行に `behind an unavailable source link, kept until it is back` と表示される場合、その Skill はリンク先が利用できない[たどられた source リンク](../targets/configuration.md#follow_source_links)の背後にあります。prune するものはありません。ドライブをマウントするかチェックアウトを復元して、`skillshare sync` を実行してください。

### "Skills without SKILL.md"

必須ファイルがない Skill フォルダです。

```bash
# 各 Skill に SKILL.md を追加するか、フォルダを削除する
skillshare new my-skill  # 正しい構造を作成
```

### "Link not supported"

`doctor` はシステムの一時ディレクトリ（Windows では `%TEMP%`、それ以外では `$TMPDIR` または `/tmp`）にテスト用フォルダーのリンクを作成します。Windows ではこのリンクは NTFS ジャンクションで、管理者権限も Developer Mode も不要なため、Developer Mode を有効にしてもこのエラーは解決しません。メッセージ内の `junction error:` の行に、Windows が拒否した理由が表示されます。一時ディレクトリについて次を確認してください:

1. FAT32、exFAT、ネットワーク共有ではなく、ローカルの NTFS ドライブ上にあること（ジャンクションは NTFS でのみ動作します）
2. 自分のアカウントで書き込みができ、ウイルス対策ソフトやセキュリティソフトにブロックされていないこと

このチェックはファイルのリンクをテストしません。Developer Mode がない場合、単一ファイルをリンクする agents と extras は代わりにコピーされます。詳しくは [Windows のトラブルシューティング](../../troubleshooting/windows.md#file-links-need-windows-developer-mode-copying-instead) を参照してください。

## 問題がある場合の出力例

```
Environment
✓ Config       ~/.config/skillshare/config.yaml
✓ Source       ~/.config/skillshare/skills · 12 skills
✓ Agents       ~/.config/skillshare/agents · 8 agents
✓ Links        supported
! Git          3 uncommitted changes
! Integrity    5/6 skills verified
!              _team-repo__api-helper: 1 modified
! Skills without SKILL.md: test-dir, temp

Targets
✓ claude    skills  merged · merge · 8 shared, 2 local
✓           agents  synced · merge · 8/8 linked
! codex     skills  linked · merge · needs sync
✓ cursor    skills  merged · merge · 6 shared
! claude    1 skill not synced · 2/3 linked
✗ cursor: 2 broken symlinks: old-skill, removed-skill

Storage
  Backups      last 2026-01-18_09-00-00 · 3d ago
  Trash        2 items, 45.2 KB · oldest 3 days

Version
✓ CLI          1.2.0
✓ Skill        0.16.0
  Update       v1.2.0 → v1.3.0 available

✗ 1 error, 5 warnings · 0.6s

Next
  skillshare sync          bring the targets up to date
  brew upgrade skillshare  update to v1.3.0
```

## JSON 出力

CI パイプラインや自動化のために、機械可読な出力には `--json` を使用します。

```bash
skillshare doctor --json
```

```json
{
  "checks": [
    { "name": "source", "status": "pass", "message": "Source: ~/.config/skillshare/skills (12 skills)" },
    { "name": "skillignore", "status": "pass", "message": ".skillignore: 3 patterns, 2 skills ignored", "details": ["test-*", "vendor/", "!important", "---", "test-draft", "vendor/lib"] },
    { "name": "sync_drift", "status": "warning", "message": "claude: 1 skill(s) not synced (7/8 linked)", "details": ["new-skill"] },
    { "name": "shared_target_paths", "status": "warning", "message": "1 shared target path(s) — enabled targets writing to the same directory may produce duplicate skills in runtime pickers", "details": ["~/.agents/skills ← universal, warp"], "suggestions": ["Choose one authoritative target for ~/.agents/skills; preview removing duplicate targets with `skillshare target remove <name> --global --dry-run` (currently: universal, warp)."] },
    { "name": "broken_symlinks", "status": "error", "message": "cursor: 1 broken symlink(s)", "details": ["old-skill"] }
  ],
  "summary": { "total": 14, "pass": 12, "warnings": 1, "errors": 1, "info": 0 },
  "version": { "current": "0.17.4", "latest": "0.18.0", "update_available": true }
}
```

チェックのステータス: `pass`、`warning`、`error`、`info`。`info` ステータスは、合格でも失敗でもない情報提供のみのチェック（例: `.skillignore` が見つからない場合）に使われます。Info チェックは `total` にはカウントされますが、`pass`、`warnings`、`errors` にはカウントされません。

一部の warning チェック（例: `shared_target_paths`、`cross_target_discovery`）には、実行可能な改善手順を示す任意の `suggestions` 配列も含まれます。提案することがない場合、このフィールドは省略されます。

### 終了コード

| 状態 | 終了コード |
|-----------|-----------|
| すべてのチェックが合格（または警告のみ） | `0` |
| いずれかのチェックが `error` ステータス | `1` |

### CI での例

```bash
# doctor がエラーを検出した場合にパイプラインを失敗させる
skillshare doctor --json | jq -e '.summary.errors == 0'

# 通知用に警告を抽出する
skillshare doctor --json | jq '[.checks[] | select(.status == "warning")]'
```

:::tip Web Dashboard
Web ダッシュボードの **Health Check** ページ（`skillshare ui`）は、`doctor --json` のビジュアル版で、フィルタの切り替えと展開可能な詳細を提供します。
:::

## 関連項目

- [status](/docs/reference/commands/status) — クイックステータスチェック
- [sync](/docs/reference/commands/sync) — sync の問題を修正
- [upgrade](/docs/reference/commands/upgrade) — CLI と Skill を更新
