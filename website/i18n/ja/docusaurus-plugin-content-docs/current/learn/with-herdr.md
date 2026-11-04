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

パネルは skillshare リポジトリの [`herdr/skillshare`](https://github.com/runkids/skillshare/tree/main/herdr/skillshare) にあります。

1. `mods/skillshare/` を `~/.config/herdr-app/mods/skillshare/` にコピーします。
2. `layouts/herdr-skillshare.json` と `layouts/herdr-skillshare.png` を `~/.config/herdr-app/layouts/` にコピーし、Herdr の設定で `herdr-skillshare` レイアウトに切り替えます。自分のレイアウトを使い続ける場合は、フォルダの `README.md` に載っている 2 つのブロックを追加します。
3. `herdr-app check ~/.config/herdr-app/mods/skillshare` を実行します。

タブバーに **skillshare** ボタンが表示され、パネルの表示と非表示を切り替えます。space を切り替えても、パネルの開閉状態はそのまま残ります。

## パネルでできること

パネルは Herdr が表示している space に追従し、その space のマシン上で、フォーカス中のターミナルのフォルダで skillshare を実行します。

- **Resources** は skills、agents、MCP サーバー、plugins、hooks を「Updates available」「Needs sync」「In sync」に分けて表示します。`.skillshare/config.yaml` のあるプロジェクトでは、All · Project · Global でスコープを選べます。
- **行**はその場で開きます。skills と agents には Update、Diff、Uninstall と「Ask &lt;agent&gt; to audit…」があり、確認後にその項目を Herdr でフォーカス中のエージェントに渡します。
- **Install** はリポジトリ URL を受け取ります。複数の skills を含むリポジトリでは一覧から選べます。パネルはクローンに対して [`skillshare audit`](/docs/reference/commands/audit) を実行し、通過した場合だけインストールします。audit でブロックされると findings を表示し、「Install anyway」（`--force`）と「Ask &lt;agent&gt; to review…」を提示します。
- **Source** は source リポジトリの状態を表示します。ダイアログには Pull、Commit and push、Sync both ways があります。両方のマシンで変更されたファイルで pull が止まった場合は、左右の比較プレビューからファイルごとに残すバージョンを選べます。`.metadata.json` は [`skillshare pull`](/docs/reference/commands/pull) と同じくエントリごとにマージされます。
- **Machines** は Herdr の各マシンの skillshare バージョン、source の状態、doctor の警告を表示し、Pull and sync と、新しいリリースがあれば Update を提供します。

パネルは表示中 20 秒ごとに更新し、10 分ごとに `skillshare check` を実行します。非表示の間は何もしません。

## 関連

- [Sync](/docs/reference/commands/sync)
- [Push](/docs/reference/commands/push) と [Pull](/docs/reference/commands/pull)
- [Audit](/docs/reference/commands/audit)
