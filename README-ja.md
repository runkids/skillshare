<p align="center" style="margin-bottom: 0;">
  <img src=".github/assets/skillshare-logo-card.png" alt="skillshare" width="280">
</p>

<h1 align="center" style="margin-top: 0.5rem; margin-bottom: 0.5rem;">skillshare</h1>

<p align="center">
  <a href="README.md">English</a> · <a href="README-ja.md">日本語</a> · <a href="README-ko.md">한국어</a> · <a href="README-zh-CN.md">简体中文</a> · <a href="README-zh-TW.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://skillshare.runkids.cc"><img src="https://img.shields.io/badge/Website-skillshare.runkids.cc-blue?logo=docusaurus" alt="Website"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  <a href="https://github.com/runkids/skillshare/releases"><img src="https://img.shields.io/github/v/release/runkids/skillshare" alt="Release"></a>
  <a href="https://github.com/runkids/skillshare/releases"><img src="https://img.shields.io/github/downloads/runkids/skillshare/total" alt="Downloads"></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-blue" alt="Platform">
  <a href="https://deepwiki.com/runkids/skillshare"><img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki"></a>
</p>

<p align="center">
  <a href="https://github.com/runkids/skillshare/stargazers"><img src="https://img.shields.io/github/stars/runkids/skillshare?style=social" alt="Star on GitHub"></a>
</p>

<p align="center">
  <a href="https://trendshift.io/repositories/21835" target="_blank"><img src="https://trendshift.io/api/badge/repositories/21835" alt="runkids%2Fskillshare | Trendshift" style="width: 250px; height: 55px;" width="250" height="55"/></a>
</p>

<p align="center">
  <strong>あなたの AI コーディング環境を、どこでも。</strong><br>
  skills、agents、rules、MCP 接続、hooks をひとつの場所で管理。<br>
  Claude Code、Codex、Pi、OpenCode などで使えます。
</p>

<p align="center">
  <a href="https://skillshare.runkids.cc">公式サイト</a> •
  <a href="#インストール">インストール</a> •
  <a href="#クイックスタート">クイックスタート</a> •
  <a href="#主な機能">主な機能</a> •
  <a href="#cli-と-ui-のプレビュー">スクリーンショット</a> •
  <a href="#desktop-app">デスクトップアプリ</a> •
  <a href="https://skillshare.runkids.cc/docs">ドキュメント</a>
</p>

<p align="center">
  <img src=".github/assets/demo.gif" alt="skillshare demo" width="960">
</p>

> [!NOTE]
> **最新バージョン**：v0.24.6 — `diff` とダッシュボードの Sync タブは、sync 済みの変換された agent（Codex、OpenCode）を未同期として表示しなくなり、各 target の include/exclude にも従います。Targets 一覧では target の各項目から対応するタブへ直接移動できます。Memory に **Refresh** ボタンが加わり、agent は短い事実を `INDEX.md` に書きます。skills フォルダーの外を指す skill 名はダッシュボードで拒否されます。新機能と修正の一覧は [Releases](https://github.com/runkids/skillshare/releases) と[変更履歴](https://skillshare.runkids.cc/changelog)をご覧ください。

## skillshare を使う理由

AI ツールを切り替えるたびに、環境を作り直す必要はありません。
skillshare は skills やほかの AI リソースを、自分で管理できる場所にまとめます。

- **ツールを変えても skills はそのまま** — 一度編集して、Claude Code、Codex、Pi など使っているツールに同期。
- **環境を別のマシンへ** — ソースを Git で管理し、別のマシンに pull。
- **チームで共有** — プロジェクトのリソースをコードと一緒に管理し、共通の skills を tracked repo で配布。

あるメンバーは Claude Code、別のメンバーは Codex を使っています。共通のコードレビューチェックリストを、プロジェクトの `.skillshare/` で管理します。新しいメンバーはチャットから指示をコピーする代わりに、定義済みの skills をインストールして設定されたツールへ同期できます。[チームのオンボーディング →](https://skillshare.runkids.cc/docs/how-to/recipes/team-onboarding-recipe)

デスクトップアプリまたは CLI でローカルに管理し、[使用前に skills を監査](https://skillshare.runkids.cc/docs/reference/commands/audit)して、[各ツールに届ける内容を選べます](https://skillshare.runkids.cc/docs/how-to/daily-tasks/filtering-skills)。

> ほかのツールから移行しますか？ [移行ガイド](https://skillshare.runkids.cc/docs/how-to/advanced/migration) · [比較](https://skillshare.runkids.cc/docs/understand/philosophy/comparison)

## CLI と UI のプレビュー

| Skill の詳細 | セキュリティ監査 |
|---|---|
| <img src=".github/assets/skill-detail-tui.png" alt="Skill の詳細" width="480" height="300"> | <img src=".github/assets/audit-tui.png" alt="セキュリティ監査" width="480" height="300"> |

| Web ダッシュボード | Web の Skills ページ |
|---|---|
| <img src=".github/assets/ui/web-dashboard-demo.png" alt="Web ダッシュボード" width="480"> | <img src=".github/assets/ui/web-skills-demo.png" alt="Web の Skills ページ" width="480"> |

## インストール

> [!TIP]
> **デスクトップで skillshare を管理。** macOS（Apple Silicon）、Windows、Linux 向けの [Skillshare App をダウンロード](https://github.com/runkids/skillshare-app/releases/latest)。初回起動では CLI のインストールまたは既存の CLI の選択、AI ツールの選択、初回の同期を案内します。[インストールガイド](https://skillshare.runkids.cc/ja/docs/getting-started/desktop-app)。

<a id="desktop-app"></a>

### デスクトップアプリ — 画面で設定・管理

[Skillshare App](https://github.com/runkids/skillshare-app) はスキル、エージェント、MCP、hooks をデスクトップのウィンドウで管理できます。アプリをインストールして開き、初回起動の案内に沿って設定してください。

macOS（Apple Silicon）では Homebrew を使用：

```bash
brew tap runkids/tap
brew install --cask skillshare-app
```

**Windows／Linux、または macOS に手動でインストールする場合：**[最新版のアプリインストーラーをダウンロード](https://github.com/runkids/skillshare-app/releases/latest)。各プラットフォームの詳細は[デスクトップアプリのガイド](https://skillshare.runkids.cc/ja/docs/getting-started/desktop-app)をご覧ください。

### CLI：macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

スクリプトはデフォルトで `~/.local/bin` にインストールするため、通常のインストールと更新に `sudo` は不要です。インストーラーが PATH 設定の案内を表示した場合のみ、その案内に従ってから `skillshare` を実行してください。今後のターミナルでも使えるよう、案内された行を shell 設定ファイル（`~/.zshrc` や `~/.bashrc` など）に追加できます。別の場所には `INSTALL_DIR` を指定できます。

### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

### CLI：Homebrew

```bash
brew install skillshare
```

> **ヒント：** `skillshare upgrade` で最新版に更新できます。インストール方法を自動で判別し、残りの作業も行います。

### GitHub Actions

```yaml
- uses: runkids/setup-skillshare@v1
  with:
    source: ./skills
- run: skillshare sync
```

すべてのオプション（audit、project モード、バージョン固定）は [`setup-skillshare`](https://github.com/marketplace/actions/setup-skillshare) を参照してください。

### 短縮コマンド（任意）

シェルの設定ファイル（`~/.zshrc` または `~/.bashrc`）に alias を追加します。

```bash
alias ss='skillshare'
```

## クイックスタート

```bash
skillshare init            # 設定、ソース、検出された target を作成
skillshare sync            # skills をすべての target に同期
```

## 仕組み

- macOS / Linux: `~/.config/skillshare/`
- Windows: `%AppData%\skillshare\`

```
┌─────────────────────────────────────────────────────────────┐
│                    Source Directory                         │
│   ~/.config/skillshare/skills/    ← skills (SKILL.md)       │
│   ~/.config/skillshare/agents/    ← agents                  │
│   ~/.config/skillshare/extras/    ← rules, commands, etc.   │
└─────────────────────────────────────────────────────────────┘
                              │ sync
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
       ┌───────────┐   ┌───────────┐   ┌───────────┐
       │  Claude   │   │  OpenCode │   │ OpenClaw  │   ...
       └───────────┘   └───────────┘   └───────────┘
```

| プラットフォーム | Skills のソース | Agents のソース | Extras のソース | リンク方式 |
|----------|---------------|---------------|---------------|-----------|
| macOS/Linux | `~/.config/skillshare/skills/` | `~/.config/skillshare/agents/` | `~/.config/skillshare/extras/` | Symlinks |
| Windows | `%AppData%\skillshare\skills\` | `%AppData%\skillshare\agents\` | `%AppData%\skillshare\extras\` | フォルダーは NTFS Junction（管理者権限は不要）。ファイルのシンボリックリンクには Developer Mode が必要で、ない場合はコピー |

| | 命令型（コマンドごとにインストール） | 宣言型（skillshare） |
|---|---|---|
| **単一のソース** | skills を個別にコピー | ひとつのソースから symlink（またはコピー）で配布 |
| **新しいマシンのセットアップ** | すべてのインストールを手作業でやり直す | 設定を `git clone` して `sync` |
| **セキュリティ監査** | なし | `audit` を内蔵。install と update の際に自動スキャン |
| **Web ダッシュボード** | なし | `skillshare ui` |
| **実行時の依存** | Node.js + npm | なし（単一の Go バイナリ） |

> [詳しい比較 →](https://skillshare.runkids.cc/docs/understand/philosophy/comparison)

## 主な機能

**skills のインストールと更新** — GitHub、GitLab、その他の Git ホストから

```bash
skillshare install github.com/reponame/skills
skillshare update --all
skillshare target claude --mode copy  # symlink が使えない場合
```

**symlink で問題が起きたら** — target ごとに copy モードへ切り替え

```bash
skillshare target <name> --mode copy
skillshare sync
```

**セキュリティ監査** — skills が agent に届く前にスキャン

```bash
skillshare audit
```

**プロジェクトの skills** — リポジトリごとに管理し、コードと一緒に commit

```bash
skillshare init -p && skillshare sync
```

**Agents** — カスタム agent を agent 対応の target に同期

```bash
skillshare sync agents            # agents だけを同期
skillshare sync --all             # skills、agents、extras、MCP、hooks をまとめて同期
```

**Extras** — rules、commands、prompts などを管理

```bash
skillshare extras init rules          # "rules" という extra を作成
skillshare sync --all                 # skills、agents、extras、MCP、hooks をまとめて同期
skillshare extras collect rules       # ローカルのファイルをソースに取り込む
```

**MCP 接続** — 一度の設定で Claude Code、Codex、Pi、VS Code、OpenCode などに反映

```bash
skillshare mcp add                    # ガイド付き設定。URL を入力するか JSON を貼り付け
skillshare sync mcp --dry-run         # 各ツールの設定ファイルへの変更をプレビュー
skillshare sync mcp                   # 接続設定を適用
```

定義は `config.yaml` に書くか、別の `mcp.yaml` を参照できます。
設定例、環境変数の参照、既存の接続のインポートは [MCP の設定](https://skillshare.runkids.cc/docs/how-to/daily-tasks/sharing-mcp)を参照してください。

hook を実行せずにネイティブ hooks を管理：

```bash
skillshare hooks add check --file ./check.yaml
skillshare hooks sync --dry-run
skillshare hooks sync
```

**Plugins** — plugin 一式をインストールし、インストール先のツールを選択

```bash
skillshare plugin add                 # ガイド付き：ソース、plugin、target、確認
skillshare plugin add owner/repo --target claude --target codex --no-tui
skillshare sync plugins --dry-run     # plugin の同期は sync --all とは別
```

ツールにインストール済みの plugin は `plugin import` で管理下に置けます。
[ツールをまたいだ plugin の管理](https://skillshare.runkids.cc/docs/how-to/daily-tasks/sharing-plugins)を参照してください。

**シェル補完** — コマンド、フラグ、サブコマンドを Tab で補完

```bash
skillshare completion bash --install   # zsh、fish、powershell、nushell にも対応
```

**ローカルチェックポイント** — ソースの変更を push せずに commit

```bash
skillshare commit -m "Update review skill"
skillshare commit --dry-run
```

**Web ダッシュボード** — 視覚的なコントロールパネル

```bash
skillshare ui
```

[すべてのコマンドとガイド →](https://skillshare.runkids.cc/docs/reference/commands)

## コントリビュート

コントリビュートを歓迎します。まず issue を立て、テスト付きの draft PR を送ってください。
セットアップの詳細は [CONTRIBUTING.md](CONTRIBUTING.md) を参照してください。

```bash
git clone https://github.com/runkids/skillshare.git && cd skillshare
make check  # format + lint + test
```

> [!TIP]
> どこから始めればよいか分からない場合は、[open issues](https://github.com/runkids/skillshare/issues) を見るか、セットアップ不要の開発環境 [Playground](https://skillshare.runkids.cc/docs/learn/with-playground) を試してください。

## コントリビューター

skillshare を支えてくださったすべての方に感謝します。一覧は[英語版 README](README.md#contributors) にあります。

---

skillshare が役に立ったら、⭐ をお願いします

## Star History

[![Star History Chart](https://star-history.dera.page/svg?repos=runkids/skillshare&type=date&legend=top-left)](https://star-history.dera.page/#runkids/skillshare&type=date&legend=top-left)

---

## ライセンス

MIT
