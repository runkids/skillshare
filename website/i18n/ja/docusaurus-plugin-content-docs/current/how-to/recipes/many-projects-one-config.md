---
sidebar_position: 8
---

# レシピ: 多数の Project を 1 つの Config で

> global config から、1 回の sync で複数の project フォルダーに Skill と MCP サーバーを配布する。

## シナリオ {#scenario}

`~/.claude/skills` のような global Target はすべての project で読み込まれるため、どの project からも同じ Skill が見えます。それで構わないなら、このレシピは必要ありません。

このレシピは、project ごとに**異なる**ものを持たせたい場合のためのものです:

- 多くの Skill をインストールしていて、frontend の project では `frontend-*` だけが必要な場合。セッション内の Skill が少ないほど、説明文に使われるコンテキストが減り、誤った選択も少なくなります。
- クライアントのリポジトリのように、他では問題なくても、ある project では読み込んではいけない MCP サーバーがある場合。
- Skillshare の config はコミットせず、Skill の実体だけを project 内に置いてコミットできるようにしたい場合。
- 使っているツールが project 内の特定のフォルダーしか読まない場合。

[Project mode](/docs/how-to/recipes/skill-per-project-workflow) も、project ごとに独自のセットを持たせます。各 project に `.skillshare/config.yaml` を置き、それぞれのフォルダー内から sync します。global config の `projects` を使えば、手元の 1 つのファイルから同じ結果が得られます:

| | Global Target | Global `projects` | Project mode |
|---|---|---|---|
| **Skill を受け取るのは** | すべての project、同じセット | 列挙したフォルダー、それぞれ別のセット | その project だけ |
| **セットアップの置き場所** | 自分のマシン | 自分のマシン | project のリポジトリ |
| **チームメイトに渡るか** | いいえ | いいえ | はい、クローンすれば |
| **project に追加されるファイル** | なし | sync された Skill と Agent のみ | `.skillshare/` と sync されたファイル |
| **Sync** | どこからでも `sync` を 1 回 | どこからでも `sync` を 1 回 | 各 project 内で `sync` |

セットアップをリポジトリと一緒に持ち運びたいなら project mode を選んでください。自分だけの project や、`.skillshare/` を追加できないクライアント・OSS のリポジトリで、1 回の `sync` ですべてを更新したい場合は `projects` を選んでください。

## 解決策

### Skill と Agent: `projects`

```yaml
# ~/.config/skillshare/config.yaml
projects:
  ~/work/project01:
    targets: [claude, codex]
    skills:
      mode: copy
      include:
        - myskill-*
    agents: {}
```

```bash
skillshare sync --dry-run   # プレビュー
skillshare sync
```

- `targets` は、その project で使うツールを指定します。Skillshare は各ツールの project パス（ここでは `.claude/skills` と `.agents/skills`）に書き込むため、パスを入力する必要はありません。
- `skills` と `agents` がその部分を有効にします。空のままなら全部を sync し、`include` と `exclude` で絞り込めます。[Skill のフィルタリング](/docs/how-to/daily-tasks/filtering-skills)を参照してください。
- `copy` は実ファイルを書き込むため、project 側でコミットできます。シンボリックリンクで問題なければ、デフォルトの `merge` のままにしてください。

ダッシュボードでは、**プロジェクト** ページが同じことを行います: **プロジェクトを追加**、Target を選び、sync する内容を選択します。フィールドの詳細は [`projects`](/docs/reference/targets/configuration#projects) を参照してください。

### MCP サーバー: `mcp.projects`

MCP サーバーは各 Agent 自身の config ファイルに書き込まれるため、パスではなく project フォルダーごとに列挙します。

```yaml
# ~/.config/skillshare/config.yaml
mcp:
  servers:
    context7:
      command: npx
      args: ["-y", "@upstash/context7-mcp"]
      targets: [opencode]
  projects:
    ~/work/project01:
      servers:
        context7:            # 他の場所では読み込まれ、ここではオフ
          disabled: true
          targets: [opencode]
```

```bash
skillshare sync mcp --dry-run   # すべてのファイルをプレビューする
skillshare sync mcp
```

フィールドと制限事項については、[`mcp`: 複数の project を管理する](/docs/reference/commands/mcp#manage-several-projects-from-the-global-config)を参照してください。

## 確認

- `skillshare sync` が project の Target を報告する（例: `project01@claude  1 copied`）
- `~/work/project01/.claude/skills/` に `include` に一致した Skill だけが含まれている
- `skillshare sync mcp --dry-run` が project のファイルごとに 1 行ずつ表示する
- 2 回目の `skillshare sync mcp` がすべてのエントリを `unchanged` と報告する

## バリエーション

- **ツールのパス以外のフォルダー**: Target は名前とパスの組にすぎないため、どのツールの project パスにも当てはまらないフォルダーには `skillshare target add project01 ~/work/project01/some/folder` がそのまま使えます。そうした Target がツールの project パスを指している場合、ダッシュボードの **プロジェクト** ページから変換を提案されます。
- **コミットするか無視するか**: `copy` mode では、Skillshare はコピーした内容を追跡するために、Target フォルダーに `.skillshare-manifest.json` も書き込みます。Skill と一緒にコミットするか、`.gitignore` に追加してください。
- **パス重複の警告**: project のフォルダーが、別の Target がすでに使っているものと同じ場合、`sync` はパス重複の警告を表示します。どの Target が共有しているかは `skillshare doctor` で確認できます。
- **複数の project で同じサーバーを使う**: 1 つの project の下で YAML アンカー（`docs: &docs`）を使って一度だけ定義し、他の project で再利用します（`docs: *docs`）。[`mcp` リファレンス](/docs/reference/commands/mcp#manage-several-projects-from-the-global-config)を参照してください。
- **共有される project**: project をクローンしたチームメイトには、あなたの global config は渡りません。セットアップをリポジトリと一緒に持ち運ぶ必要がある場合は、[project mode](/docs/how-to/recipes/skill-per-project-workflow) を使ってください。

## 関連項目

- [`target` コマンドリファレンス](/docs/reference/commands/target)
- [`mcp` コマンドリファレンス](/docs/reference/commands/mcp)
- [MCP サーバーの共有](/docs/how-to/daily-tasks/sharing-mcp)
