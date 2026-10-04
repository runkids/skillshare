---
sidebar_position: 9
---

# ツール間でプラグインを管理する

プラグインには、skills、MCP 接続、hooks、スクリプト、その他連携して動作するファイルを含めることが
できます。skillshare はそのパッケージをそのまま維持し、どのツールがそれを受け取るかを選択させます。
始めるのに YAML を書く必要はありません。

## 最初のプラグインを追加する

ダッシュボードで **Plugins → Add plugin** を開きます。

1. GitHub リポジトリ（`owner/repo`）、HTTPS Git URL、またはローカルディレクトリを貼り付けます。
2. Source に複数含まれている場合はプラグインを選択し、対応するツールを選びます。
3. 変更内容をレビューして適用します。

![Add plugin ダイアログ: 見つかった plugin と、対応・非対応の Target](/img/plugins-add-dialog.png)

ほとんどのユーザーは、リポジトリと Target のチェックボックスだけで十分です。**Advanced options** では
リリースを選ぶための Git ref を追加できます。Discovery は各 Target のコンポーネントと互換性を別々に
表示します。OpenCode のエントリが検出できず非対応と表示された場合、その行の **Set entry path** を
使うとビルド済みファイルを指定し、すでに選択した内容を保持したまま Source を再検索します。安全な
相対リポジトリのシンボリックリンクは保持されます。

同じガイド付きフローはターミナルからも利用できます。

```bash
skillshare plugin add
```

Claude Code、Codex、Copilot、Antigravity CLI、Grok、Pi、または OpenCode CLI は、**skillshare の
バックエンドが動作している場所**にインストールされている必要があります。
Codex の場合、Codex デスクトップ app に同梱の CLI も対象です。Codex が別の場所にあるマシンでは [`SKILLSHARE_CODEX_CLI`](/docs/reference/appendix/environment-variables#skillshare_codex_cli) を設定してください。
Cursor と Antigravity
デスクトップは代わりに、それぞれのローカルプラグインディレクトリに完全なファイルを受け取ります。
コンテナ内で動作しているダッシュボードは、ホストマシンにのみインストールされているプラグインを
管理できません。ローカルの CLI を使うか、ネイティブクライアントと並べて skillshare を実行してください。

インストール Target は **Claude Code、Codex、Cursor、Antigravity Desktop、Antigravity CLI、
GitHub Copilot CLI、Pi、OpenCode** です。Grok は Import 前にネイティブなインストールと信頼設定が
必要です。Kimi、Hermes、Devin のフォーマットは検出可能ですが、自動インストールは利用できず、
インターフェースがその理由を説明します。Target 向けに公開されているフォーマットを選んでください。
skillshare はツール間でプラグインを変換しません。Project mode は Claude、Antigravity Desktop、
Pi、OpenCode に対応しています。Antigravity Desktop（`antigravity` または `agy`）と CLI
（`antigravity-cli`）は別々の store を使います。実行するほうを選んでください。

## すでに何かインストール済みの場合

**Import installed** を選び、ネイティブなインストールを選択します。Import はそれを記録するだけで、
再インストールしたり、その認証をコピーしたり、その Agent 内で有効かどうかを変更したりすることは
ありません。Import は Claude、Codex、Copilot、Antigravity CLI、Grok、Pi、OpenCode で利用できます。
Cursor と Antigravity のローカルパッケージには **Add plugin** を使ってください。

```bash
skillshare plugin import review@team --from claude --no-tui
```

論理的に1つのパッケージが、ツールごとに異なるネイティブな配布形式を使っている場合は、同じ `--name`
と適切な Target を使ってそれぞれの配布形式を add/import してください。skillshare は表示名から
同等性を推測することはありません。

インポートはこのマシンのネイティブインストールに結び付いています。同じ plugin を別のマシンで使うには、
代わりに source から追加してください。[クロスマシン Sync — Plugins](/docs/how-to/sharing/cross-machine-sync#plugins) を参照してください。

## Sync する場所を選ぶ

管理対象の各バインディングにはチェックボックスがあります。このチェックボックスは**この Target を
Sync に含める**ことを意味し、「その Agent 内で有効にする」ことを意味するものではありません。

- チェックしてから Sync すると、不足しているプラグインがインストールされます。
- プラグインの行を開くと、その Source がパッケージを持っている他の Agent がチェックなしで一覧表示
  されます。1つにチェックを入れると、インストールのプレビューが開きます。Source が提供できない
  Agent は行の末尾にまとめてカウントされ、その数をクリックすると理由が表示されます。
- Agent を 1 つもチェックせずにプラグインを追加できます。Skillshare に残り **Agent 未選択** と表示され、行で Agent をチェックするまで何もインストールされません。
- チェックを外してから Sync すると、その管理対象のインストールが削除されます。
- パッケージの定義は残るため、後で再び Target を選択できます。
- Claude または Codex 内で無効化されたプラグインは無効のままです。そのツール内のネイティブな設定で
  管理してください。

ダッシュボードでは、Plugins ページの右上にある **Sync** ボックスに、次の Sync で各 Agent に対して
インストールまたは削除される内容が一覧表示されます。そのボタンはプレビューを開きます。確認するまで
Agent 内は何も変わりません。プラグインの一覧はすぐに表示されますが、ボックス下の **Agents** 列は
各 Agent の CLI が応答するたびに埋まっていきます。
実行後、ボックスには結果が表示されます。失敗が先頭で、その後に他の変更が続き、同じ結果になった Agent は 1 行にまとめられます。変わらなかったものは 1 行の **Unchanged** にまとめられ、開くと plugin ごとに 1 行ずつ、Pi パッケージは別に表示されます。プレビューも同じように、行う変更の下に変わらないものをまとめます。一覧には、Pi がインストールしたパッケージから読み取った、すべての Pi パッケージのインストール済みバージョンが表示されます。

```bash
skillshare plugin disable review --target codex --no-tui
skillshare sync plugins --dry-run
skillshare sync plugins --no-tui
```

プラグインのメニューには **View files** があります。これは、その Source のレビュー済みローカルコピーを
Markdown をレンダリングした状態で読み取り専用表示するものです。インポートされたプラグインには
ローカルコピーがないため、この項目はありません。

プラグインは通常の skills や MCP の Sync とは別物です。バンドルされたコンポーネントが、独立した
skillshare の Source にコピーされることはありません。

## 更新と復旧 {#updates-and-recovery}

**Check updates** を使い、対応する Target について更新をレビューします。Claude はネイティブな CLI
経由で更新できます。Codex はレビュー済みのスナップショットからプラグインを再度 add します。
ただし Codex で無効化されている場合は、add で再び有効になってしまうため更新しません。Import した Codex プラグインは marketplace のアップグレードで更新されます。Cursor と Antigravity は、ローカルの編集が
ないか確認した後、管理対象のローカルコピーを置き換えます。Pi と OpenCode はレビュー済みのスナップ
ショットを更新します。Copilot は既知の有効状態を保持しながらレビュー済みの Source を更新できます。
Antigravity CLI と Grok の更新はネイティブツール側にとどまります。インポートされたパッケージの制限
についてはコマンドリファレンスを参照してください。update が対応できない Target は理由を示して
スキップされ、そのプラグインのほかの Agent は通常どおり更新されます。

Pi の npm パッケージには比較できる source がないため、**Check updates** はインストール済みバージョンを npm の最新バージョンと比べます。新しいバージョンが見つかると、行に `old → new` と **Update** ボタンが表示され、チェック結果にも見つかった plugin ごとにボタンが表示されます。どちらも先にその plugin の更新プレビューを開きます。

1つの Target が失敗した場合、結果は成功した部分を保持します。ネイティブクライアントの認証や設定の
問題を解決してから、その Target を再度 Sync してください。

```bash
skillshare sync plugins review --target claude --no-tui
```

スナップショットは skillshare が所有します。その内容が外部で編集されていた場合、skillshare はまず
その編集を保持できるよう置き換えをブロックします。Source のダイジェストは変更を検出しますが、
すべてのネイティブインストールやインポートされた marketplace がマシン間で再現可能であることを
保証するものではありません。

自動化、scope の詳細、すべてのフラグについては [plugin](/docs/reference/commands/plugin) を参照して
ください。
