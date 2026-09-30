---
sidebar_position: 9
---

# Skill 管理アプローチの比較

このページでは、AI CLI の Skill 管理における 2 つの主要なアーキテクチャアプローチ、**命令型**（コマンドごとのインストール）と**宣言型**（config + sync）を比較します。

ツールを評価中の方や乗り換えを検討している方は、この比較で根本的な設計の違いを理解できます。

## アーキテクチャの概観

### 命令型（コマンドごとのインストール）

命令型ツールはコマンドごとのインストールモデルを採用しており、各インストールは独立した操作です。

```
tool add owner/repo → select agents → choose method → done
tool add owner/repo → select agents → choose method → done
tool add owner/repo → select agents → choose method → done
```

すべての操作でユーザー入力が必要です。「どこに何をインストールすべきか」を記述する永続的な状態は存在しません。

### 宣言型（Config + Sync）

skillshare は宣言型モデルを採用しています。望ましい状態を一度定義すれば、あとは sync するだけです。

```yaml
# config.yaml — define once
source: ~/.config/skillshare/skills
targets:
  claude: ~/.claude
  cursor: ~/.cursor/skills
  codex: ~/.codex/skills
```

```bash
skillshare sync  # reconcile actual state to desired state
```

コマンド 1 つ、プロンプトなし、常に決定的な結果が得られます。

## 機能比較

| 機能 | 命令型（コマンドごとのインストール） | 宣言型（skillshare） |
|------------|------------------------|--------------------------|
| **設定** | 設定ファイルなし。実行のたびにプロンプトが表示される | `config.yaml` — 一度設定すれば以後も再利用可能 |
| **Agent 選択** | 毎回インタラクティブなプロンプト | config 内で定義済み。`sync` がすべて処理 |
| **インストール方法** | 操作ごとに copy/symlink を選択 | config 内の `sync_mode`（merge、copy、symlink） |
| **単一の信頼できる情報源** | Skill が各 agent に個別にコピーされる | Source ディレクトリ → 全 Target へのシンボリックリンク |
| **1 つの agent から Skill を削除する場合** | Source ファイルが削除され、他の agent が壊れることがある | その Target のシンボリックリンクにのみ影響 |
| **再現可能なセットアップ** | 新しいマシンで復元する組み込みの方法がない | `config.yaml` + Source ディレクトリで完全復元 |
| **プロジェクトスコープの Skill** | ロックファイルはグローバルのみ追跡 | `skillshare init -p` でリポジトリごとの Skill |
| **クロスマシン同期** | 手動（dotfiles 経由でロックファイルを同期） | git を使った組み込みの `push` / `pull` |
| **双方向のフロー** | 一方向（インストールのみ） | `collect` が Target 側の改善を取り込む |
| **自作 Skill とインストール済み Skill の分離** | 同じディレクトリに混在 | Tracked repo は `_` 接頭辞を使用 |
| **オフライン動作** | CLI 自体に npx とネットワークが必要 | 単一バイナリで、インストール後はオフラインで動作 |
| **Web ダッシュボード** | なし | `skillshare ui` — ビジュアル管理 |
| **バックアップ / 復元** | なし | `skillshare backup` / `skillshare restore` |
| **Git プラットフォーム対応** | update/check は GitHub のみ（GitHub Trees API にハードコード） | GitHub、GitLab、Bitbucket、Azure DevOps、Gitea、AtomGit、Gitee、セルフホストなど任意の Git remote |
| **ランタイム依存** | Node.js + npm | なし（単一の Go バイナリ） |

## よくある課題の解決

### 「インストールのたびに agent を選び直さないといけない」

skillshare では、Target を一度だけ設定します。

```yaml
targets:
  claude: ~/.claude
  cursor: ~/.cursor/skills
```

以降、`sync`、`install`、`collect` はどこに反映すべきか把握しています。プロンプトは不要です。

### 「1 つの agent から Skill を削除すると他が壊れる」

命令型ツールでは、1 つの agent から Skill を削除すると共有の Source ファイルが削除され、他の agent のシンボリックリンクが壊れることがあります。

skillshare のアーキテクチャはこれを完全に防ぎます。Source ディレクトリが唯一の真実であり、Target のシンボリックリンクは Source を**指す**だけです。Target を削除してもそのシンボリックリンクが消えるだけで、Source ファイルは無傷です。

```
Source: ~/.config/skillshare/skills/my-skill/SKILL.md  (always preserved)
  ├── ~/.claude/skills/my-skill → symlink to source  ✓
  ├── ~/.cursor/skills/my-skill  → symlink to source  ✓  (unaffected)
  └── ~/.codex/skills/my-skill  → symlink to source  ✓  (unaffected)
```

### 「新しいマシンでセットアップを復元できない」

skillshare なら、セットアップ全体を持ち運べます。

1. `~/.config/skillshare/`（Source + config）をバージョン管理する
2. 新しいマシンで、config リポジトリを `git clone` する
3. `skillshare sync` を実行する

すべての Target が即座に再現されます。

### 「update と check が GitLab / Bitbucket / Azure DevOps で動かない」

命令型ツールは更新チェックを GitHub Trees API に依存していることが多く、`update` と `check` が GitHub 以外のソースの Skill を黙ってスキップしてしまいます。

skillshare は**ローカルの git 操作**（`git fetch` + ツリーハッシュ比較）を使用するため、GitLab、Bitbucket、Azure DevOps、Gitea、AtomGit、Gitee、任意のセルフホストインスタンスを含む、あらゆる Git remote で動作します。プラットフォーム固有の API は不要です。

```bash
# All of these support install, update, and check:
skillshare install https://gitlab.com/team/skills
skillshare install git@bitbucket.org:company/private-skills.git
skillshare install https://git.mycompany.com/org/repo
skillshare update   # checks all sources, regardless of host
```

### 「大きなリポジトリの clone に時間がかかりすぎる」

skillshare は、tracked でないインストールに対してデフォルトで shallow clone（`--depth 1`）を使用し、ダウンロード時間を大幅に短縮します。完全な履歴が必要な tracked repo には `--track` を使用してください。

### 「Skill が各 agent のディレクトリに散らばっている」

skillshare はすべてを 1 か所にまとめます。

```
~/.config/skillshare/skills/
├── my-custom-skill/          # Your own skills
├── react-best-practices/     # Installed skills
├── _team-repo/               # Tracked repos (prefixed with _)
│   ├── frontend-guidelines/
│   └── code-review/
└── _another-org-repo/
```

`_` 接頭辞により、tracked（チーム/組織）の repo と個人の Skill が明確に区別されます。

## skillshare への移行

すでに別の Skill マネージャーを使用している場合は、以下の手順に従ってください。

### ステップ 1: skillshare をインストールする

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

インストーラーが PATH 設定の案内を表示した場合のみ、その案内に従ってから以下のコマンドを実行してください。PATH の警告がなければ追加の設定は不要です。

```bash

# Homebrew
brew install skillshare
```

### ステップ 2: 初期化して既存の Skill を取り込む

```bash
skillshare init              # Creates config and detects targets
skillshare collect --all     # Imports existing skills from all detected targets
```

### ステップ 3: Sync する

```bash
skillshare sync              # Symlinks source skills to all targets
```

これで既存の Skill が 1 か所から管理できるようになりました。詳しい手順は[移行ガイド](/docs/how-to/advanced/migration)を参照してください。

## 適切なツールの選び方

**次の場合は命令型ツールを選びましょう。**
- Skill をたまにしかインストールせず、インタラクティブなプロンプトが気にならない
- 使用している AI CLI が 1 つだけである
- クロスマシンやチームでのワークフローが不要である

**次の場合は skillshare を選びましょう。**
- 複数の AI CLI を使用しており、それらを同期させたい
- 一度設定したら放置できる構成を望んでいる
- 複数のマシンで作業している
- チームや組織で Skill を共有している
- Skill のバックアップ、復元、バージョン管理をしたい
- GitLab、Bitbucket、Azure DevOps、またはセルフホストの Git で Skill をホストしている
- ランタイム依存のない単一バイナリを好む
- ローカルワークフローの外でインストール/ダウンロードの活動が追跡されるのを望まない

---

## 関連項目

- [移行](/docs/how-to/advanced/migration) — 移行ガイド
- [コアコンセプト](/docs/understand) — skillshare の仕組み
