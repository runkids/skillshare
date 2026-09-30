---
sidebar_position: 5
---

# レシピ: クロスマシン Sync

> git の push/pull を使って複数のマシン間で Skill を同期し続ける。

## シナリオ

デスクトップとノートパソコン（あるいは自宅とオフィスのマシン）で作業しています。各マシンでインストール
コマンドを再実行することなく、どこでも同じ Skill ライブラリを使えるようにしたいです。

## 解決策

### 初期セットアップ（マシン A）

```bash
# skillshare を初期化する
skillshare init

# Skill をインストールする
skillshare install your-org/team-skills
skillshare install another/repo --into tools

# Source を git remote にプッシュする
skillshare push
```

`skillshare push` は Source ディレクトリを git 追跡下のブランチにコミットし、設定された remote に
プッシュします。

### 新しいマシンでのセットアップ（マシン B）

```bash
# skillshare をインストールする
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

インストーラーが PATH 設定の案内を表示した場合のみ、その案内に従ってから以下のコマンドを実行してください。PATH の警告がなければ追加の設定は不要です。

```bash

# 初期化する
skillshare init

# remote から pull する
skillshare pull

# ローカルの Target に Sync する
skillshare sync
```

### 日々の Sync ワークフロー

どのマシンでも:

```bash
# 他のマシンからの最新の変更を pull する
skillshare pull

# ローカルの AI ツールに Sync する
skillshare sync

# ローカルで変更を加えた後
skillshare push
```

## 確認

- `skillshare push` が 0 で終了し、コミットされた変更を報告する
- 別のマシンでの `skillshare pull` が受け取った変更を表示する
- `skillshare list` が両方のマシンで同一の Skill を表示する
- `skillshare sync` が Target のマシン上にシンボリックリンクを作成する

## バリエーション

- **ログイン時の自動 Sync**: シェルのプロファイル（`.bashrc` / `.zshrc`）に
  `skillshare pull && skillshare sync` を追加する
- **競合の解決**: `pull` は両方のマシンのコミットをマージし、`.metadata.json` の競合は自動で解決します。
  両方のマシンが同じ Skill ファイルを編集した場合、`pull` は停止してマージを取り消し、該当ファイルを表示します —
  Source ディレクトリ内で git を使って解決してください
- **選択的な Sync**: `config.yaml` の Target ごとの `include` / `exclude` フィルターを使い、
  各マシンに Sync される Skill を制御する

## 関連項目

- [クロスマシン Sync ガイド](/docs/how-to/sharing/cross-machine-sync)
- [`push` コマンドリファレンス](/docs/reference/commands/push)
- [`pull` コマンドリファレンス](/docs/reference/commands/pull)
