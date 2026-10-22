---
sidebar_position: 3
---

# レシピ: プライベートエンタープライズ Skill

> トークン認証を使ってプライベートリポジトリから Skill をインストールする。

## シナリオ

あなたの組織は、内部の Skill をプライベートな GitHub/GitLab リポジトリでホストしています。設定ファイルに
認証情報を露出させずに、これらの Skill をインストール・更新する必要があります。

## 解決策

### ステップ 1: 認証をセットアップする

skillshare は環境変数からトークンを検出します。プラットフォーム固有の変数は、汎用のフォールバックより
優先されます。

| プラットフォーム | 環境変数 |
|----------|---------------------|
| GitHub / GitHub Enterprise | `GITHUB_TOKEN` |
| GitLab / セルフホストの GitLab | `GITLAB_TOKEN` |
| Bitbucket | `BITBUCKET_TOKEN`（+ 任意で `BITBUCKET_USERNAME`） |
| Azure DevOps | `AZURE_DEVOPS_TOKEN` |
| Gitea / セルフホストの Gitea | `GITEA_TOKEN` |
| CNB | `CNB_TOKEN` |
| 任意のプラットフォーム（フォールバック） | `SKILLSHARE_GIT_TOKEN` |

```bash
# オプション A: Git credential helper（GitHub に推奨）
gh auth login   # HTTPS 用の git credential helper をセットアップする

# オプション B: プラットフォーム固有の環境変数
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxx      # GitHub
export GITLAB_TOKEN=glpat-xxxxxxxxxxxxx    # GitLab
export AZURE_DEVOPS_TOKEN=your-pat-here    # Azure DevOps

# オプション C: 汎用フォールバック（任意の HTTPS ホストで動作）
export SKILLSHARE_GIT_TOKEN=your-token-here
```

### ステップ 2: プライベートリポジトリからインストールする

```bash
skillshare install your-org/internal-skills --track
```

skillshare は上記の環境変数からトークンを自動的に検出します。

### ステップ 3: トラッキングを確認する

```bash
skillshare list
```

インストールされたリポジトリは `_` プレフィックス付きで表示されます（Tracked repository）。

```
_your-org-internal-skills/
├── code-review/
├── testing-standards/
└── deployment-checklist/
```

### ステップ 4: 更新サイクル

```bash
skillshare check    # 上流の変更を検出する
skillshare update   # 最新を取得する
skillshare sync     # Target にプッシュする
```

## 確認

- `skillshare list` が Tracked repo を表示する
- `skillshare check` が remote に到達しハッシュを比較できる
- `skillshare sync` がすべての Target にシンボリックリンクを作成する

## バリエーション

- **選択的インストール**: `skillshare install your-org/internal-skills --skill code-review`
  は1つの Skill のみをインストールする
- **CI/CD トークン**: パイプラインでは、CI シークレットからプラットフォーム固有の環境変数
  （例: `GITHUB_TOKEN`）を設定する
- **セルフホストの GitLab**: `GITLAB_TOKEN` を設定し、HTTPS URL を使う:
  `skillshare install https://gitlab.internal.com/team/skills.git --track`
- **セルフホストの Gitea**: `GITEA_TOKEN` を設定する。ホスト名に `gitea` が含まれない場合は、
  [`gitea_hosts`](/docs/reference/targets/configuration#gitea_hosts) にも列挙する
- **Gitee / AtomGit**: `SKILLSHARE_GIT_TOKEN` を使った HTTPS URL 経由で対応

## 関連項目

- [`install` コマンドリファレンス](/docs/reference/commands/install)
- [`update` コマンドリファレンス](/docs/reference/commands/update)
- [組織共有ガイド](/docs/how-to/sharing/organization-sharing)
- [URL フォーマットリファレンス](/docs/reference/appendix/url-formats)
