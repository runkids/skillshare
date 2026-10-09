---
sidebar_position: 7
---

# status

skillshare の現在の状態（source、tracked repositories、targets、バージョン）を表示します。

```bash
skillshare status
```

`follow_source_links: true` の場合、利用できない最上位のソースリンクはリンク名付きの警告を表示し、正常なスキルは引き続き一覧に含まれます。`--json` では警告を stderr に出力し、stdout は有効な JSON のままです。

## 使うタイミング

- 変更を加えた後、すべての targets が sync 済みか確認する
- どの targets に `sync` の実行が必要か確認する
- tracked repos が最新かどうか確認する
- 有効な audit ポリシー（profile、threshold、dedupe mode）を確認する
- CLI や skill の更新を確認する

## 出力例

```
Source
  skills    ~/.config/skillshare/skills  43 skills
  agents    ~/.config/skillshare/agents  2 agents

Tracked repositories
✓ _superpowers  15 skills

Targets                        skills                 agents
  claude     ~/.claude/skills  ✓ 43 linked            ✓ 2
  cursor     ~/.cursor/skills  ✓ 43 linked · 1 local  ✓ 2
  gemini     ~/.gemini/skills  ✓ 43 linked            —
  universal  ~/.agents/skills  ✓ 43 linked            —
  all use merge

Extras
  rules     ~/.claude/rules     2 files · merge
  rules     ~/.cursor/rules     2 files · merge
  commands  ~/.claude/commands  1 file · merge
  team      ~/.codex            1 file · symlink
  team      ~/.claude           1 file · import

Audit    default · blocks critical
Version  CLI 0.24.0 · skill 0.21.12
! Skill 0.21.13 is available — run skillshare upgrade --skill && skillshare sync
```

## セクション

### Source

skills フォルダとその中の skill 数を表示します。agents フォルダがある場合は、2 行目にフォルダと agent 数を表示します。`.skillignore` が有効な場合は、パターン数と除外された skill 数の行が加わります。

### Tracked Repositories

`--track` でインストールした git リポジトリと、それぞれの skill 数を一覧表示します。`✓` はリポジトリに未コミットの変更がないことを示します。`!` には `uncommitted changes`、または git status を読み取れない場合はそのエラーが付きます。

### Targets

各 target を 1 行で表示します：名前、skills フォルダ、skills と agents の状態です。表の下の行には使用中の sync モードが表示されます。

```
Targets                     skills                agents
  claude  ~/.claude/skills  ✓ 8 linked · 2 local  ✓ 8
  cursor  ~/.cursor/skills  ! 6/8 copied          ! 7/8
  copy: cursor · merge: claude
! 2 skills not synced — run skillshare sync
```

**skills 列:**

| 表示 | 意味 |
|------|------|
| `✓ 8 linked` / `✓ 8 copied` | 想定されるすべての skill が配置済みです。merge と copy では `include`/`exclude` で絞り込んだ後の skill を数えます |
| `· 2 local` | そのフォルダにある自分の skill です。sync はこれらに触れません |
| `! 6/8 linked` | 一部の skill がまだ sync されていません。status の最後に数と `sync` コマンドが表示されます。`sync` が意図的にスキップする skill（`standard` / `prefixed` naming での無効な名前、または名前の衝突）は数えません。`sync` を再実行しても追加できないためで、`sync` がそれらを表示します |
| `✓ symlinked` | symlink モード：フォルダ全体が source にリンクしています |
| `! needs sync` | モードが変更されました。`sync` を実行して反映します |
| `! has files` / `! not synced yet` | この target はまだ一度も sync されていません |
| `✗ links to …` / `✗ broken link` | フォルダが別の場所、または存在しない場所へのリンクです |
| `skills off` | この target の skills はオフです |

**agents 列:** `✓ 8` はリンク済みの agent 数です（最新のコピーもリンク済みとして数えます）。`! 7/8` は一部が欠けていることを示します。`skillshare sync agents` を実行してください。数えるのはこの target が sync する agent だけで、`.agentignore`、target の agents の include/exclude、各 agent の `targets` frontmatter で残ったものです。copy fallback では、skillshare が所有していない同一内容のローカルファイルは保持され、`· 1 local` として数えられます。`—` はその target に agents フォルダがないことを示します。agents source がない場合、この列は省略されます。

### Extras

extras を設定している場合、extra の target ごとに 1 行表示します：

```
Extras
  rules     .cursor/rules     4 files · merge
  commands  .claude/commands  3 files · merge
```

各行には extra 名、target フォルダ、ファイル数、そしてファイルが実際に sync されるモードが表示されます：開発者モードが無効な Windows では、ファイルをリンクする target は `copy` と表示されます。

### Audit

有効な audit ポリシー（CLI フラグ、project 設定、global 設定から解決）を 1 行で表示します：profile（`default`、`strict`、`permissive`）と、インストールをブロックする最低の重大度（デフォルトは `critical`）です。dedupe モードと analyzer は、デフォルト（`global` とすべての analyzer）と異なる場合のみ表示されます。

### Version

CLI と skill のバージョンを表示します。新しい skill がリリースされている場合は、更新方法を示す行が加わります。（global mode のみ）

## オプション

| フラグ | 説明 |
|------|-------------|
| `--json` | JSON として出力（スクリプト/CI 用） |
| `--project, -p` | project mode を使用 |
| `--global, -g` | global mode を使用 |
| `--help, -h` | ヘルプを表示 |

## JSON 出力

```bash
skillshare status --json
```

```json
{
  "source": {
    "path": "~/.config/skillshare/skills",
    "exists": true,
    "skillignore": {
      "active": true,
      "files": [".skillignore", "_team-skills/.skillignore"],
      "patterns": ["test-*", "vendor/"],
      "ignored_count": 2,
      "ignored_skills": ["test-draft", "vendor/lib"]
    }
  },
  "skill_count": 12,
  "tracked_repos": [
    {"name": "_team-skills", "skill_count": 5, "dirty": false},
    {"name": "_personal-repo", "skill_count": 3, "dirty": true}
  ],
  "targets": [
    {
      "name": "claude",
      "path": "~/.claude/skills",
      "mode": "merge",
      "status": "merged",
      "synced_count": 8,
      "include": [],
      "exclude": []
    }
  ],
  "agents": {
    "source": "~/.config/skillshare/agents",
    "exists": true,
    "count": 8,
    "targets": [
      {"name": "claude", "path": "~/.claude/agents", "expected": 8, "linked": 8, "drift": false}
    ]
  },
  "audit": {
    "profile": "DEFAULT",
    "threshold": "CRITICAL",
    "dedupe": "GLOBAL",
    "analyzers": []
  },
  "version": "0.17.0"
}
```

git status を読み取れない tracked repo は `"status": "unknown"` となり、`message` にエラーが入ります。この場合 `dirty` は false で、意味を持ちません。

`source.skillignore` フィールドは、少なくとも 1 つの `.skillignore` または `.skillignore.local` ファイルが存在する場合にのみ存在します。存在しない場合: `"skillignore": { "active": false }`。`files` 配列には、存在する場合 `.skillignore.local` のパスも含まれます。テキストモードでは、`.skillignore.local` が有効な場合、`.skillignore` の行に `(.local active)` と表示されます。

JSON 出力は global mode と project mode の両方でサポートされています。

## Project Mode

project ディレクトリでは、status は project の source、targets、extras を、project ルートからの相対パスで表示します。

```bash
skillshare status        # .skillshare/ が存在する場合は自動検出
skillshare status -p     # 明示的な project mode
```

### 出力例

```
Source
  skills    .skillshare/skills  3 skills
  agents    .skillshare/agents  4 agents
  .skillignore: 3 patterns, 0 skills ignored

Targets                   skills      agents
  claude  .claude/skills  ✓ 3 linked  ✓ 4
  cursor  .cursor/skills  ✓ 3 linked  ✓ 4
  all use merge

Extras
  rules     .cursor/rules     4 files · merge
  commands  .claude/commands  3 files · merge

Audit    default · blocks critical
```

Project status では Tracked Repositories と Version のセクションは表示されません（これらは global 専用の機能です）。

## 関連項目

- [sync](/docs/reference/commands/sync) — Skill を targets に sync
- [diff](/docs/reference/commands/diff) — 詳細な差分を表示
- [doctor](/docs/reference/commands/doctor) — 問題を診断
- [Project Skills](/docs/understand/project-skills) — Project mode の概念
