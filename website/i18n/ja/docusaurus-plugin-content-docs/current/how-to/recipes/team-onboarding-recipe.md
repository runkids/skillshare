---
sidebar_position: 6
---

# レシピ: チームオンボーディング

> 新しいメンバーに、チーム共通の Skill とプロジェクトの文脈を届ける。

## シナリオ

新しい開発者がチームに参加します。彼らに必要なものは:
- 組織全体の Skill（コーディング標準、レビューガイドライン）
- プロジェクト固有の Skill（ドメイン知識、アーキテクチャルール）
- 自分の AI ツール（Claude Code、Pi など）すべてで動作すること

あるメンバーは Claude Code、別のメンバーは Codex、新しいメンバーは Pi を使っています。3 人とも、避けるべき旧 API を示すレビューチェックリストが必要です。各ツールにコピーすると別々のバージョンが生まれ、プロジェクトの変更とともにずれていきます。

プロジェクトのローカル Skill、`.skillshare/config.yaml`、`.skillshare/skills.lock.json` をプロジェクトリポジトリに保存します。Config はリモート Skill と Target を宣言し、lockfile はリモート Skill のコミットを記録します。各メンバーがこれらのファイルをローカルに適用します。共有の指示は共通の文脈を提供しますが、各ツールの権限や動作はそれぞれ異なります。

## 解決策

### ステップ 1: オンボーディングスクリプトを作成する

チームの wiki やリポジトリに `scripts/setup-skills.sh` として保存します。

```bash
#!/bin/bash
set -e

echo "Installing skillshare..."
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"

echo "Initializing..."
skillshare init -g

echo "Installing organization skills..."
skillshare install github.com/your-org/org-skills --track -g

echo "Running security audit..."
skillshare audit -g --threshold high

echo "Syncing to all AI tools..."
skillshare sync -g

echo "Done! Run 'skillshare list -g' to see installed skills."
```

### ステップ 2: 新入社員がスクリプトを実行する

```bash
curl -fsSL https://your-org.github.io/setup-skills.sh | sh
```

またはスクリプトがチームのリポジトリにある場合:

```bash
git clone your-org/team-tools
./team-tools/scripts/setup-skills.sh
```

### ステップ 3: プロジェクト固有のセットアップ

管理者はまず[プロジェクトセットアップ](/docs/how-to/sharing/project-setup)に従い、プロジェクト設定、ローカル Skill、lockfile をコミットします。新しいメンバーがそのプロジェクトを clone したら:

```bash
cd your-project
skillshare install -p
skillshare audit -p --threshold high
skillshare sync -p
```

`install -p` は `.skillshare/config.yaml` で宣言されたリモート Skill をインストールし、コミットが固定されていればそのバージョンを使います。`audit -p` はコミット済みのローカル Skill を含むプロジェクトの Skill を検査します。`sync -p` は設定済みの Target に配布します。順番に実行し、失敗したらそこで止めてください。Audit によるブロックは、同期する前にレビューが必要です。

プロジェクトの更新を pull した後もこの手順を繰り返します。Git は設定と lockfile を転送しますが、不足しているリモート Skill のインストールや Target のコピーの更新は行いません。意図した Skill の更新は PR でレビューし、生成された lockfile の変更もコミットします。

### ステップ 4: すべてが動作することを確認する

```bash
# Global の Skill を確認する
skillshare list -g

# プロジェクトの Skill を確認する
skillshare list -p

# Sync のステータスを確認する
skillshare status -p
```

## 確認

- `skillshare list -g` に組織の Skill が表示される
- `skillshare list -p` にプロジェクトのローカル Skill とインストール済みリモート Skill が表示される
- `skillshare status -p` で設定済みのプロジェクト Target が同期済みになっている
- 設定した AI ツールを開いて期待する Skill を確認し、既知の旧 API の例でレビューチェックリストを試す

## バリエーション

- **Dev Container でのオンボーディング**: チームが Dev Container を使っている場合、
  `.devcontainer/Dockerfile` と `postCreateCommand` に skillshare を追加すれば、コンテナ起動時に
  Skill が準備される
- **Homebrew ベースのインストール**: macOS/Linux チームでは `curl | sh` の代わりに
  `brew install skillshare` を使う
- **Hub による発見**: 新入社員に自分たちの Hub を案内する:
  `skillshare search --hub https://your-org.github.io/skillshare-hub.json`

## 関連項目

- [はじめにガイド](/docs/getting-started)
- [組織共有](/docs/how-to/sharing/organization-sharing)
- [プロジェクトセットアップ](/docs/how-to/sharing/project-setup)
- [Dev Container ガイド](/docs/learn/with-devcontainer)
