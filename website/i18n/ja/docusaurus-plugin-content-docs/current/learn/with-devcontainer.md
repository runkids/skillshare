---
sidebar_position: 5
---

# Dev Container で skillshare を使う

> VS Code で開けば、Skill はすぐに使える準備が整います — ローカルインストールは不要です。

## 前提条件

- [Dev Containers 拡張機能](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers) を入れた [VS Code](https://code.visualstudio.com/)

## 仕組み

VS Code の Dev Containers は Docker コンテナ内で開発できるようにします。`.devcontainer/` に環境を
定義しておけば、あとは VS Code が処理します — プロジェクトを開いて「Reopen in Container」をクリック
すれば、すべてが準備完了です。

skillshare はこのワークフローに自然に組み込めます。`postCreateCommand` に追加しておけば、コンテナの
起動時に Skill がインストールされ Sync されます。

## セットアップ

`.devcontainer/devcontainer.json` に2つのことを追加します。

```json
{
  "postCreateCommand": "curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh && export PATH=\"$HOME/.local/bin:$PATH\" && skillshare init --no-copy --all-targets --no-git --no-skill && skillshare sync"
}
```

この PATH 設定はセットアップコマンド内にのみ適用されます。今後のターミナルでも使えるよう、コンテナ内の shell 設定ファイル（`~/.bashrc` など）に `export PATH="$HOME/.local/bin:$PATH"` を追加してください。

これだけです。チームメンバーが VS Code でプロジェクトを開き「Reopen in Container」をクリックすると:

1. skillshare が自動的にインストールされる
2. `init` が質問なしで実行される — 検出されたすべての AI CLI Target を追加し、既存 Skill の取り込み、git、
   組み込み Skill をスキップする
3. `sync` がすべての Target に Skill を配布する

## プロジェクトの Skill を追加する

チームで共有する Skill には、`.skillshare/` の設定をリポジトリにコミットします。

```bash
# コンテナ内で
skillshare init -p
skillshare install your-org/team-skills -p
```

その後コミットし、プロジェクトの Skill も Sync するよう `postCreateCommand` を更新します。

```json
{
  "postCreateCommand": "curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh && export PATH=\"$HOME/.local/bin:$PATH\" && skillshare init --no-copy --all-targets --no-git --no-skill && skillshare sync && skillshare sync -p"
}
```

これで、コンテナを開いたすべてのチームメンバーが同じ Skill を得られます。

## GitHub Codespaces

同じ `.devcontainer/` の設定が変更なしで Codespaces でも動作します。Codespaces は VS Code と同じ
方法で `postCreateCommand` を実行します。

## ssenv による隔離テスト

Devcontainer 内では、`ssenv` を使って並列テスト用の隔離された skillshare 環境を作成できます。
各環境は、別々の config、Skill、Target を持つ独自の `HOME` ディレクトリを得ます。

| コマンド | 何をするか |
|---------|-------------|
| `ssnew <name>` | 新しい隔離環境を作成する |
| `ssuse <name>` | 環境を切り替える |
| `ssback` | 元の環境に戻る |
| `ssls` | すべての環境を一覧表示する |
| `ssrm <name>` | 環境を削除する |

```bash
ssnew demo && ssuse demo    # 作成して切り替える
ss init && ss sync          # コマンドは隔離された状態で実行される
ssback                      # 元に戻る
```

これはメインのセットアップに影響を与えずに、設定変更や Skill のインストールをテストするのに便利です。

## 次のステップ

- [プロジェクトの Skill セットアップ →](/docs/how-to/sharing/project-setup)
- [チーム共有 →](/docs/how-to/sharing/organization-sharing)
- [Sync モードの説明 →](/docs/understand/philosophy/sync-modes-explained)
