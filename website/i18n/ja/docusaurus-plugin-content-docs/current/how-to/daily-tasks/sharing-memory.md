---
sidebar_position: 12
---

# AI ツール間でメモリを共有する

決定事項、学んだこと、プロジェクトの背景を Markdown ノートのフォルダーにまとめます。ツールに読み取りガイダンスを接続し、新しいセッションで関連ノートの読み取りを確認します。

このガイドは global mode を使い、Claude と Codex を Target として設定済みです。

```bash
skillshare ui -g
```

スクリーンショットの UI、ノート、ダイアログはすべて英語です。`/tmp/skillshare-memory-docs` の隔離されたデモ home を使っているため、実際のパスとは異なります。

## 1. メモリを作成する

**Extras → Memory** を開き、**Create memory** をクリックします。

![英語の Memory タブの初期状態](/img/memory-empty-demo.png)

Target のない `memory` extra を登録し、不足する初期ノートを作成します。`INDEX.md` は短い入口、`LEARNED.md` は日付、背景、結論、証拠を記録するテンプレートです。既存のノートと設定は保持します。

![INDEX.md と LEARNED.md を表示する Memory](/img/memory-starters-demo.png)

既定の global フォルダーは `~/.config/skillshare/extras/memory/` です。**Extras → Folders & files** にも表示され、初期状態では Target がありません。このマシンのエージェントは source を直接読み取るため、ノートのコピーを sync する必要はありません。

## 2. ツールを接続する

**Use with agents** で **Connect to agents** をクリックします。ツールを選び、それぞれの更新モードを選んでから **Review changes** をクリックします。

- `passive`（既定）：エージェントはノートを読み、依頼されたときだけ更新します。
- `active`：明示された好み、理由付きの決定、確認済みの落とし穴など、後のセッションでも役立つ事実を自分で保存します。一度きりの詳細や推測は残さず、迷うときはノートを提案して了承を待ち、重複を作らず既存のノートを更新し、保存した内容を伝えます。

同じファイルを読むツールは 1 つのブロックを共有するため、1 つを切り替えると全部切り替わります。設定済みツールのモードを変えるには、もう一度 **Connect to agents** を開いて切り替えます。変更は同じレビューを通ります。

![英語の接続ダイアログで指示ファイルの変更を確認](/img/memory-connect-demo.png)

各ファイルの差分（削除行は `−`、追加行は `+`）を確認し、**Apply changes** をクリックします。Skillshare は、ツールの既存の指示ファイル、またはツールがすでに読む共有 source に管理対象の読み取りガイダンスを追加します。ファイルがなければ作成できます。ブロックは scope と内容 hash のマーカーを持ち、それ以外の内容、既存の割り当て、接続モードは保持されます。既存ファイルは変更前にバックアップします。他のツールも同じファイルを読む場合や既知の文字数制限がある場合、レビューに警告が表示されます。

![設定済みのツールを表示する英語の Memory タブ](/img/memory-connected-demo.png)

**Configured** はツールの読み取り経路に現在のガイダンスがある状態です。エージェントが読んだことは示しません。**Not configured**、**Outdated**、**Needs attention** は指示ファイルの状態です。変更されていない古いブロックは再レビュー後に更新できます。手動変更済みやマーカーが不正なブロックは保持され、手動修正が必要です。別々のファイルから両方のモードのブロックを読むツールも要対応になります。それらのファイルを読むツールを同じモードにしてください。複数のファイルからブロックを読むツールは、余分なブロックを削除するまでモードを切り替えられません。未同期の共有指示は先に sync してください。読めない指示ファイルはスキップします。レビュー後にファイルが変わった場合は再レビューが必要です。

**Open AGENTS.md** で指示を確認・修正できます。接続レビューは dashboard の機能で、新しい CLI 接続コマンドはありません。

## 3. ノートとインデックスリンクを追加する

**New note** をクリックし、**File name** に `wiki/architecture.md` を入力します。**Link from INDEX.md** をオンのまま **Create** をクリックします。このチェックボックスは `INDEX.md` が読み取り可能な場合に表示され、既定でオンです。

![ネストしたパスとインデックス設定を示す英語の New note ダイアログ](/img/memory-folder-demo.png)

不足するサブフォルダーを作成し、`INDEX.md` の末尾に相対 Markdown リンクを追加します。インデックス更新では version を確認し、変更をバックアップします。リンク追加が失敗してもノートは残り、部分完了の警告を表示します。未登録のノートを選び **Add to INDEX** で再試行できます。インデックスは手動でも編集できます。短く保ってください。リンク切れには警告が表示され、自動削除はされません。

ノートは UTF-8 の `.md` ファイルで、最大 1 MiB です。未対応のノートは **Unsupported file** として一覧に残り、他の有効なノートは引き続き使えます。source 内の隠しファイル、隠しフォルダー、シンボリックリンクは除外します。

ノートを選び **Edit** をクリックして、次の英語のデモ内容を入力し **Save** します。

```markdown
# Architecture decisions

## Shared memory

Claude and Codex read the same Markdown notes from Skillshare.
Keep durable decisions here and verify facts that may have changed.

## Retrieval

Read INDEX.md first, then only the notes relevant to the current task.
Update notes when the user asks you to remember a decision.
```

![英語の保存済みノートの Markdown プレビュー](/img/memory-note-demo.png)

左のツリーはネストしたフォルダー、上部に検索ボックスがあり、その横の **Refresh** ボタンは、エージェントがダッシュボードの外からノートを書いた後に、一覧と開いているノートを再読み込みします。右のペインは **Preview** / **Source** を切り替えます。長いノートは折りたたまれて開き、**Show all** で全体を表示します。ノート名の横に **Edit** と **More actions** メニューがあり、**Copy file path**、**History**、**Move or rename**、**Delete note** を含みます。**Use with agents** はノートの下にあります。既存ノートへの相対リンクは同じビューアーで開きます。

![wiki を展開した英語の Memory ビューアー](/img/memory-tree-demo.png)

## 4. 検索・編集・復元する

**Search names and content** に `Retrieval` と入力します。検索は大文字小文字を区別せず、サブフォルダーのパスと内容を含みます。検索をクリアすると全ノートが表示されます。

![ネストしたノートを表示する英語の検索](/img/memory-search-demo.png)

テキストエディターも使えます。外部変更は dashboard の再読み込みで確認できます。編集中にノートが変更された場合、古い version の保存は拒否され、下書きは保持されます。**Latest saved version** と比較し、**Copy draft** でコピーできます。内容を手動で比較・統合してから **Save my draft** を選び、置換を確認します。更新された version を使い、保存済み内容をバックアップします。さらに外部変更があれば再び競合になります。

![最新の保存内容と下書きを並べて表示する英語のエディター](/img/memory-conflict-demo.png)

**History** はノートの絶対パスで絞り込んだ **Backup Files** を開きます。保存バージョンをプレビュー・復元し、Memory を再読み込みしてください。削除したノートも同じページで復元できます。

![wiki/workflow-check.md の英語の Backup Files 復元プレビュー](/img/memory-restore-demo.png)

## 5. 新しいエージェントセッションで確認する

関連ノートに `memory-check: demo-7429` などの一時的な値を追加して保存します。接続したツールで新しいセッションを開始してください。**Copy verification prompt** で次のプロンプトをコピーできます。

**Copy verification prompt** にカーソルを合わせると、コピー前に内容を確認できます。

![English verification prompt tooltip](/img/memory-verification-demo.png)

> 指示に記載された共有メモリの INDEX.md と、このタスクに関連するノートを読んでください。ノートの完全なパスと、私が追加した一時的な検証値を報告してください。読み取りイベントを確認できるよう、ファイル読み取りツールを使ってください。

実際の読み取りツールのイベントで完全なパスと一時的な値を確認します。他の接続ツールでも繰り返し、最後に値を削除します。これは手動検証で、Skillshare は読み取り telemetry を保証しません。読み取ったという回答や **Configured** だけでは証拠になりません。

教訓を残すときは、背景、結論、証拠を `LEARNED.md` に記録するよう依頼します。どちらのモードも各タスクの開始時に `INDEX.md` を読みます。ノートはユーザー所有です。`passive` のガイダンスは残す価値のある事実を指摘させ、更新は依頼時だけ行わせます。`active` のガイダンスは上記のとおり、そうした事実をツール自身のメモリではなくここに保存させます。1〜2 文で済む事実は `INDEX.md` の `## Notes` に箇条書き 1 件として直接書き、長いノートだけ別ファイルにします。Native automatic memory、自動学習、Obsidian 連携は有効になりません。

## Project mode

先にプロジェクトの Skillshare 設定を初期化してから実行します。

```bash
skillshare extras memory init -p
skillshare ui -p
```

既定の source は `.skillshare/extras/memory/`、可視設定ディレクトリなら `skillshare/extras/memory/` です。既存の extras source override も適用します。同じ接続、インデックス、編集、復元の機能を使えます。リポジトリ内の source は **project root** からの相対パスで示します。指示ファイルがサブフォルダーにある場合も同じです。Project のガイダンスは、このプロジェクトに関するメモはここに、あなた自身やツール、他のプロジェクトに関するメモはここには属さないとエージェントに伝えるので、shared memory のガイダンスも読むエージェントは各事実の置き場所が分かります。Project 外の override は絶対パスです。絶対 source の移動や配置変更後はガイダンスを再生成し、レビューしてください。

## 高度な代替手段：ガイダンスを手動でコピーする

**Copy guidance** を開いて `passive` か `active` を選び、コピーしたブロックをエージェントが読む指示ファイルに貼り付けます。**Open AGENTS.md** で既存のエディターを開けます。

![英語の Copy guidance プレビュー](/img/memory-guidance-demo.png)

**Extras → AGENTS.md** で共有指示を作成し、そのページの既存フローで接続することもできます。[1 つの AGENTS.md をツール間で共有する](./sharing-instructions.md)の接続モードと置換警告を確認してください。

![接続されたツールを示す英語の共有指示ページ](/img/memory-agents-demo.png)

Scope と hash のマーカーは保持してください。生成された本文を手動で変更すると変更済みとなり、後の接続レビューでは保持されます。

## CLI の代替手段

```bash
skillshare extras memory init -g
printf '# Architecture decisions\n\nRead relevant notes on demand.\n' |
  skillshare extras memory write wiki/architecture.md --from - -g
skillshare extras memory list --search architecture -g
skillshare extras memory show wiki/architecture.md -g
skillshare extras memory instructions -g
skillshare extras memory instructions --update-mode active -g
```

CLI は読み取りガイダンス（`--update-mode active` を指定しなければ `passive`）を出力するだけなので手動で貼り付けます。ツールの接続やインデックスリンクの追加は行いません。ノート更新には現在の `--version` が必要です。[`extras memory` リファレンス](../../reference/commands/extras.md#extras-memory)を参照してください。

## ノートの名前変更・移動

ノートを選び、**More actions** を開いて **Move or rename** をクリックし、新しい相対 `.md` パスを入力します。`wiki/architecture.md` → `wiki/design.md` は名前変更、`projects/design.md` への変更は別フォルダーへの移動です。存在しないフォルダーは自動作成されます。**Move** で適用します。

![英語の Move or rename ダイアログで新しいフォルダーパスを指定](/img/memory-move-demo.png)

内容と権限は保持されます。移動先が存在する場合や version が古い場合は拒否します。元のパスにバックアップを保存します。**Restore in Backup Files** でその履歴を確認できます。復元すると元のノートを再作成し、移動後のノートも残ります。

Markdown リンクはノート内の相対リンクも含めて自動更新されません。`INDEX.md` と他のノートを手動で修正してください。壊れたインデックスリンクはビューアーの上に表示されます。Agent のガイダンスは source 直下の `INDEX.md` を参照するため、その場所を維持してください。

## ノートを削除する

**More actions** またはエディターで **Delete note** を選び、ファイル名を確認します。未保存の編集は破棄されます。保存済み version を確認し、バックアップしてから選択ノートだけを削除します。フォルダーと他のノートは保持します。`INDEX.md` の古いリンクは手動で更新してください。削除後の **Restore in Backup Files** で絞り込み済み履歴を開けます。

![英語のノート削除確認](/img/memory-delete-demo.png)

CLI でも直前に読んだ version が必要です。

```bash
version=$(skillshare extras memory show wiki/architecture.md --json -g | jq -r '.version')
skillshare extras memory delete wiki/architecture.md --version "$version" -g
```

復元は [`backup files`](../../reference/commands/backup.md) で行います。`skillshare backup files show <absolute-note-path>`、続いて `skillshare backup files restore <absolute-note-path> <id>` を実行してください。
