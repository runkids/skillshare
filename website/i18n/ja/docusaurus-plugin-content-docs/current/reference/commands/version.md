---
sidebar_position: 6
---

# version

現在の skillshare のバージョンを表示します。

## 使うタイミング

- バグを報告する前に、実行しているバージョンを確認する
- アップグレードが成功したかを確認する
- 自分のバージョンを [最新リリース](https://github.com/runkids/skillshare/releases) と比較する

## 構文

```bash
skillshare version
skillshare -v
skillshare --version
```

## 出力例

```
skillshare version 0.16.6
```

## アップデート通知

新しいバージョンが利用可能な場合、`skillshare` は対応しているコマンドの実行後にアップデート通知を表示します。この通知は **Homebrew を認識**します。skillshare が Homebrew でインストールされていた場合は `brew info` に問い合わせて最新の formula バージョンを確認し、`brew upgrade skillshare` を提案します。それ以外の場合は GitHub のリリースを確認し、`skillshare upgrade` を提案します。

検出は自動です。skillshare は自身の実行ファイルのパスを解決し、Homebrew の Cellar プレフィックス配下にあるかどうかを確認します（例: `/opt/homebrew/Cellar/skillshare/`）。`/nix/store/` 配下のバイナリは GitHub のリリースを確認します。バイナリが `nix profile` にある場合は `nix profile upgrade skillshare` を提案し、それ以外の場合は Nix の設定で skillshare を更新してリビルドするよう案内します。

バージョンチェックの結果は `~/.cache/skillshare/version-check.json` に 24 時間キャッシュされます。

## 関連項目

- [upgrade](./upgrade.md) — 最新バージョンへアップグレード
- [doctor](./doctor.md) — 環境の完全な診断
- [status](./status.md) — バージョン情報を含む同期状態の表示
