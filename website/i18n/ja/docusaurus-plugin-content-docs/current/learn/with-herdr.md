---
sidebar_position: 8
---

# Herdr で skillshare を使う

> エージェントの横に skillshare パネルを置き、[Herdr](https://herdr.dev) を離れずに更新や sync が必要なものを確認して、その場で対応できます。

![Herdr の skillshare パネル](/img/herdr-panel-demo.png)

## 前提条件

- Herdr 0.1.12 以降
- Herdr が接続する各マシンに skillshare がインストール済みで、`~/.local/bin`、`~/go/bin`、`/opt/homebrew/bin`、`/usr/local/bin` のいずれかにあること

## セットアップ

### Herdr layouts から

このレイアウトは [Herdr layouts](https://herdr.dev/layouts/runkids/skillshare--herdr--skillshare/) に掲載されています。Herdr 内のエージェントに次のプロンプトを貼り付けます。

```text
Read ~/.config/herdr-app/AGENTS.md, then add this layout: herdr.dev/layouts/runkids/skillshare--herdr--skillshare
```

エージェントはすべてのファイルを読んでからレイアウトを追加し、自分のレイアウトはそのまま残ります。レイアウトは `ctrl-shift-l` で切り替えます。

### 手動で

パネルは skillshare リポジトリの [`herdr/skillshare`](https://github.com/runkids/skillshare/tree/main/herdr/skillshare) にあります。

1. `mods/skillshare/` を `~/.config/herdr-app/mods/skillshare/` にコピーします。
2. `layouts/herdr-skillshare.json` と `layouts/herdr-skillshare.png` を `~/.config/herdr-app/layouts/` にコピーし、Herdr の設定で `herdr-skillshare` レイアウトに切り替えます。自分のレイアウトを使い続ける場合は、フォルダの `README.md` に載っている 2 つのブロックを追加します。
3. `herdr-app check ~/.config/herdr-app/mods/skillshare` を実行します。

タブバーに **skillshare** ボタンが表示され、パネルの表示と非表示を切り替えます。space を切り替えても、パネルの開閉状態はそのまま残ります。

## パネルの更新

パネルは表示中、1 時間に 1 回 GitHub に最新の skillshare リリースを問い合わせます。新しいリリースが公開されると、パネルのヘッダーに **Update panel** が表示されます。確認のあと、`~/.config/herdr-app/mods/skillshare/` のファイルをそのリリースに含まれるものに置き換え、Herdr がパネルを再読み込みします。置き換えられたファイルはそのフォルダーの `.bak-update-<version>` に残ります。それらのファイルに加えた変更は引き継がれません。レイアウトは変更されません。

このボタンが追加される前にインストールしたパネルは更新を確認しません。一度だけ手動で最新リリースの `mods/skillshare/` を `~/.config/herdr-app/mods/skillshare/` に上書きコピーしてください。以降の更新はボタンから行えます。

## パネルでできること

パネルは Herdr が表示している space に追従し、その space のマシン上で、フォーカス中のターミナルのフォルダで skillshare を実行します。

- **Overview** は最初に答えを示します。「Everything is in sync」か、同期が必要なターゲットの数と、1 つの Sync ボタンです。その下には、source にある各種類の件数と各ターゲットへの 1 行が並び、どれが待機中かがわかります。タブの残りには、対応が必要なものだけを表示します。同期待ちのもの、更新があるもの、hooks の通知です。`.skillshare/config.yaml` のあるプロジェクトでは、タブの横にある All · Project · Global でスコープを選べます。
- **Library** は skills、agents、MCP サーバー、plugins、hooks を「Needs sync」「Update available」「In sync」に分け、無効な項目は「Off」にまとめて表示します。skills と agents では、「In sync」が同じリポジトリの項目を折りたためるグループにまとめます。List に切り替えると一覧表示になります。
- **行**を開くとプレビューが表示されます。skill では `SKILL.md` とファイル、MCP サーバーでは秘密の値を隠した設定が見られ、プレビューの Targets タブはターゲットごとの状態を示します。skills と agents には Update、Diff、Disable（無効なものには Enable。次の sync で反映されます）、Uninstall と「Ask &lt;agent&gt; to audit…」があり、確認後にその項目を Herdr でフォーカス中のエージェントに渡します。トップレベルで宣言された hooks には Enable、Disable、Remove があり、それぞれ確認後に保存され、Sync で適用されます。
- hooks の件数には名前付きエントリと削除待ちが含まれ、診断通知は専用の行で表示されます。hooks の競合は CLI またはダッシュボードで解決するまで全種類の同期をブロックします。プランを読み取れない場合、エントリは同期状態が不明としてグループ外に表示されます。
- **Install** はリポジトリ URL を受け取ります。複数の skills を含むリポジトリでは一覧から選べます。パネルはクローンに対して [`skillshare audit`](/docs/reference/commands/audit) を実行し、通過した場合だけインストールします。audit でブロックされると findings を表示し、「Install anyway」（`--force`）と「Ask &lt;agent&gt; to review…」を提示します。
- **Source** は source リポジトリの状態を表示します。ダイアログには Pull、Commit and push、Sync both ways があります。両方のマシンで変更されたファイルで pull が止まった場合は、左右の比較プレビューからファイルごとに残すバージョンを選べます。`.metadata.json` は [`skillshare pull`](/docs/reference/commands/pull) と同じくエントリごとにマージされます。
- **Machines** は Herdr の各マシンの skillshare バージョン、remote、source、ターゲットの状態と doctor の警告を表示し、Pull and sync と、新しいリリースがあれば Update を提供します。未コミットまたは未 push の変更があるマシンでは、通常の pull が拒否されるため、代わりに Sync both ways を提供します。

パネルは表示中 20 秒ごとに更新し、10 分ごとに `skillshare check` を実行します。非表示の間は何もしません。

## 関連

- [Sync](/docs/reference/commands/sync)
- [Push](/docs/reference/commands/push) と [Pull](/docs/reference/commands/pull)
- [Audit](/docs/reference/commands/audit)
