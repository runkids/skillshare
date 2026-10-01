---
sidebar_position: 2
---

# デスクトップアプリ

**[Skillshare App](https://github.com/runkids/skillshare-app)** で、スキル、エージェント、MCP、hooks をデスクトップのウィンドウから管理できます。macOS、Windows、Linux で skillshare ダッシュボードを使え、初回の設定も案内します。

**[Skillshare App をダウンロード →](https://github.com/runkids/skillshare-app/releases/latest)**

## macOS にインストール

Apple Silicon Mac では Homebrew でのインストールをおすすめします：

```bash
brew tap runkids/tap
brew install --cask skillshare-app
```

インストール後、「アプリケーション」から **skillshare** を開いてください。[最新リリース](https://github.com/runkids/skillshare-app/releases/latest)の `.dmg` を使って手動でインストールすることもできます。

## Windows または Linux にインストール

[アプリの最新リリース](https://github.com/runkids/skillshare-app/releases/latest)からインストーラーを選んでください：

| プラットフォーム | インストーラー |
|---|---|
| Windows（x64） | `.exe` または `.msi` |
| Linux（x64） | `.deb`、`.AppImage`、`.rpm` |

システムに合ったパッケージをインストールし、Skillshare App を開きます。

## 初回起動

アプリは skillshare CLI を使って動作します。初回の設定で次の手順を案内します：

1. CLI をインストールするか、インストール済みの実行ファイルを選択します。
2. 同期する AI ツールを選びます。
3. 初回の同期を実行し、ダッシュボードを開きます。

ダッシュボードからスキルを探してインストールし、同期前に変更を確認し、エージェント、MCP、hooks を管理できます。デスクトップアプリと [`skillshare ui`](../reference/commands/ui.md) は同じダッシュボードを使用します。

## ターミナルを使いたい場合

CLI はターミナルでの操作や自動化にも引き続き使えます。[CLI の初回同期ガイド](./first-sync.md)から始めるか、[クイックリファレンス](./quick-reference.md)でコマンドを調べてください。
