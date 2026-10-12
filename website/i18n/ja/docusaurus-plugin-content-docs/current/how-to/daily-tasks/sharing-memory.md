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

## メモリーの試用と整理

**Use with agents → メモリーツール** で **別の会話で試す** を選びます。

1. 最初のプロンプトを現在の会話に貼り付けます。このソースの `INDEX.md` に一時的な `memory-check` 行を追加し、他の内容は保持します。
2. 同じ、または別の接続済みツールで新しい会話を開き、2番目を貼り付けます。ソースのパスは含まれますが、検証値は含まれません。実際の読み取りイベント、完全なパス、返された値を手順1と照合します。
3. 削除用プロンプトでテスト行だけを削除します。このブラウザータブではダイアログの再表示やページの再読み込み後も値を保持します。ブラウザーの保存機能が使えない場合は、削除完了までページを開いたままにしてください。

ボタンはプロンプトをコピーするだけです。Skillshareはエージェントを実行せず、合格とも判定しません。明示的な読み取りは取得の検証であり、ネイティブな自動読み込みの証明ではありません。

**今回を振り返る** では重複、矛盾、古い情報、壊れた索引リンク、日付・理由・出典の不足を調べるプロンプトをコピーできます。エージェントはパス、証拠、変更前後の文を提案します。会話で具体的な変更を承認してから、書き込み・移動・削除を行います。不明な日付や入手できない証拠は明示します。

Dashboard の Memory セクションにも同じ **メモリーツール** ダイアログの入口があります。整理用プロンプトを振り返りたい会話に貼り付けてください。現在の会話、利用可能なツール結果、ユーザーが提供したセッション記録だけから、訂正や繰り返す失敗の再利用できる教訓を提案します。[retro の手順](https://github.com/mattpocock/skills/blob/main/skills/engineering/retro/SKILL.md) を参考に、既存の文書やチェックを確認し、機械的な誤りには自動チェックを優先して、影響順に提案します。承認済みのメモ変更は表示中のメモリーソース内に限定し、プロジェクトのチェックや指針の変更には別の承認が必要です。Skillshare は非公開の会話履歴を取得せず、このレビューを実行しません。

**Dream** は複数の会話で保存したノートや教訓を比較し、繰り返すパターンと重複・古い情報・矛盾・リンク切れの整理案を提示します。複数セッションの出典があるノートからも分析でき、元の transcript は必須ではありません。証拠不足による限界を明示します。[Agent Memory Repo](https://cognition.com/agent-memory-repo) を参考に、使用中のホストエージェントが実行します。

3つのタブは inline です。完全なプロンプトは既定で折りたたみ、**プロンプト全文を見る** で展開できます。**Agent にコピー** はソースパスとプロジェクト／共有の範囲を含めてコピーします。更新済みの組み込み Skillshare skill がホストにあれば、この会話の振り返りや指定ノートの Dreaming を直接依頼できます。Skill は解決済みのソースを確認し、具体的なパスと変更を承認用に提案してから、version 確認とバックアップ付きで保存します。新しい journal や transcript archive は不要です。ホスト側で予定したレビューは提案のみ返します。

メモリーツールはソースの範囲が確認できるまで無効です。Project モードではプロジェクトのパスも必要です。振り返りと Dream のプロンプトは両方とも version 確認とバックアップ付きの Skillshare CLI を使い、ソース不一致、CLI 使用不可、競合時は保存を止めるよう求めます。

更新された読み取り指示は、訂正の繰り返しや矛盾するノートを見つけたとき、会話中に一度だけ短いレビュー提案をします。自動では開始しません。Skillshare はエージェントを起動・スケジュールしません。

両モードの指示は、決定や教訓に日付・理由・入手可能な出典や証拠を添え、書き込み成功後に保存内容、プロジェクト／共有の範囲、パスを1行で報告するよう求めます。例：「保存：このプロジェクトのテストは devcontainer で実行。Project memory: testing.md」。通常の Markdown を使い、metadata schema は不要です。エディターにも同じ案内を表示します。

既存ツールは **Connect to agents → Review changes → Apply changes** で指示を更新します。完全な古いブロックは更新を確認でき、手動変更したブロックは保持します。ツールや既存メモを無断で更新しません。


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
