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

## パネルでできること

パネルは Herdr が表示している space に追従し、その space のマシン上で、フォーカス中のターミナルのフォルダで skillshare を実行します。

- **Resources** は skills、agents、MCP サーバー、plugins、hooks を「Updates available」「Needs sync」「In sync」に分けて表示します。`.skillshare/config.yaml` のあるプロジェクトでは、All · Project · Global でスコープを選べます。
- **行**はその場で開きます。skills と agents には Update、Diff、Uninstall と「Ask &lt;agent&gt; to audit…」があり、確認後にその項目を Herdr でフォーカス中のエージェントに渡します。 トップレベルで宣言された hooks には Enable、Disable、Remove があり、それぞれ確認後に保存され、Sync で適用されます。
- hooks の件数には名前付きエントリと削除待ちが含まれ、診断通知はフィルターに従い、0 件でも別に表示されます。ファイルが同期済みでも無効な理由を表示し、入れ子の行にはプロジェクトのフルパスを表示します。ネイティブファイルのみの変更も Sync でき、hooks の競合は CLI またはダッシュボードで解決するまで全種類の同期をブロックします。 プレビューに失敗すると、エントリは同期状態が不明としてグループ外に表示され、スコープごとにエラー通知を 1 つ表示します。
- **Install** はリポジトリ URL を受け取ります。複数の skills を含むリポジトリでは一覧から選べます。パネルはクローンに対して [`skillshare audit`](/docs/reference/commands/audit) を実行し、通過した場合だけインストールします。audit でブロックされると findings を表示し、「Install anyway」（`--force`）と「Ask &lt;agent&gt; to review…」を提示します。
- **Source** は source リポジトリの状態を表示します。ダイアログには Pull、Commit and push、Sync both ways があります。両方のマシンで変更されたファイルで pull が止まった場合は、左右の比較プレビューからファイルごとに残すバージョンを選べます。`.metadata.json` は [`skillshare pull`](/docs/reference/commands/pull) と同じくエントリごとにマージされます。
- **Machines** は Herdr の各マシンの skillshare バージョン、source の状態、doctor の警告を表示し、Pull and sync と、新しいリリースがあれば Update を提供します。

パネルは表示中 20 秒ごとに更新し、10 分ごとに `skillshare check` を実行します。非表示の間は何もしません。

## 関連

- [Sync](/docs/reference/commands/sync)
- [Push](/docs/reference/commands/push) と [Pull](/docs/reference/commands/pull)
- [Audit](/docs/reference/commands/audit)
