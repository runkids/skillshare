---
sidebar_position: 6
---

# レシピ: チームオンボーディング

> 新しいチームメンバーの AI Skill 環境を5分以内でセットアップする。

## シナリオ

新しい開発者がチームに参加します。彼らに必要なものは:
- 組織全体の Skill（コーディング標準、レビューガイドライン）
- プロジェクト固有の Skill（ドメイン知識、アーキテクチャルール）
- 自分の AI ツール（Claude Code、Pi など）すべてで動作すること

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
skillshare init

echo "Installing organization skills..."
skillshare install your-org/org-skills

echo "Running security audit..."
skillshare audit

echo "Syncing to all AI tools..."
skillshare sync

echo "Done! Run 'skillshare list' to see installed skills."
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

新入社員がプロジェクトを clone するとき:

```bash
cd your-project
skillshare sync -p
```

これによりプロジェクトスコープの Skill が自動的に取得されます。

### ステップ 4: すべてが動作することを確認する

```bash
# Global の Skill を確認する
skillshare list

# プロジェクトの Skill を確認する
skillshare list -p

# Sync のステータスを確認する
skillshare status
```

## 確認

- `skillshare list` が組織の Skill を表示する
- `skillshare status` がすべての Target が Sync されていることを示す
- Claude Code / Pi を開くと Skill が読み込まれていることが分かる

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
