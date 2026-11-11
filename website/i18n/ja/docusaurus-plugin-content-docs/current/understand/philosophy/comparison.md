---
sidebar_position: 9
---

# Skill 管理方法の比較

**Vercel の `npx skills`** と **Skillshare** は、どちらも複数の AI ツールで skills を管理できます。違いは「一方はインストールだけ、もう一方は同期できる」ではありません。skills を管理したいのか、より広い AI 作業環境を管理したいのかで選びましょう。

本ページは、公開済みの **skills v1.8.1** の[パッケージ情報](https://registry.npmjs.org/skills/1.8.1)と[公式 CLI リファレンス](https://github.com/vercel-labs/skills/blob/e781cb4d07433dfe1d19e924ed45cc61738c55ec/docs/commands.md)に基づきます。実験的なコマンドは変更・削除される可能性があります。すべてのインストーラーについての一般論ではなく、特定バージョンの比較です。

## アーキテクチャの概要

### npx skills：skills の発見と管理

[skills.sh](https://skills.sh) は skills を見つける入口です。CLI は複数 agent、プロジェクト／グローバルのインストール、共通コピーへの symlink または独立したコピー、更新確認と更新に対応します。Agent の選択と確認はフラグでも指定でき、対話操作は必須ではありません。

インストール済み skills は lock ファイルにも記録されます。実験的なコマンドはプロジェクトの復元と npm ベースの同期に対応するため、状態を持たない、またはインストール専用という説明は正確ではありません。

### Skillshare：ツールをまたぐ作業環境の管理

Skillshare は source リソースと target 設定を分け、各ツールへ同期します。Skills に加え、agent 定義、rules などの extras、MCP 接続設定、対応クライアントのネイティブ hooks を管理します。ローカル Web dashboard から視覚的に管理できます。

違いは、これらをツール・マシン・チーム間のワークフローでまとめて管理することです。Symlink や複数 agent へのインストールは独自機能ではありません。リソース形式とクライアントの機能には違いがあり、すべてのリソースがすべてのツールで動くわけではありません。

## 機能比較

| 機能 | npx skills v1.8.1 | Skillshare |
|------|------------------|------------|
| **発見** | skills.sh と `skills find` | CLI 検索とローカル Web dashboard |
| **複数 agent とスコープ** | 複数 agent、プロジェクト／グローバルのインストール | 設定済み targets、プロジェクト／グローバルモード |
| **Skill ファイルの共有** | 共通コピーへの symlink、copy オプション | Source ディレクトリと merge、copy、ディレクトリ全体の symlink モード |
| **更新** | `skills check`（更新またはローカル変更があれば終了コード 1）と `skills update` | `skillshare check` と `skillshare update` |
| **プロジェクト復元** | 実験的な `skills experimental_install`（ソースなしの `skills install` でも実行可能）が `skills-lock.json` の対応ソースを復元 | 宣言的な skill 設定と source リソース。[manifest](/docs/understand/declarative-manifest) を参照 |
| **npm プロジェクト同期** | 実験的な `skills experimental_sync` がインストール済み依存関係と `package.json` の skill 宣言を読む | Source から target への同期。同じ npm 依存関係ワークフローではない |
| **マシン間のワークフロー** | プロジェクト lock の復元と npm 同期でプロジェクト skills の再構築を支援 | Git の `push` / `pull`。範囲は `git_root` で決定 |
| **Skills 以外** | 引用した CLI 文書は skills が中心 | Agent 定義、rules などの extras、MCP 設定、対応クライアントの hooks |
| **ローカル管理と復旧** | 引用した CLI 文書にローカル dashboard や target バックアップのコマンドは記載されていない | `skillshare ui`、target のバックアップ／復元、管理対象ファイルの復旧 |
| **実行環境** | Node.js と npm/npx | Go バイナリ。Git 操作には Git が必要 |

### 実験的なコマンドの範囲

- **`experimental_install`**（ソースなしの `skills install` でも実行可能）は対応する lock エントリーを `.agents/skills/` に復元し、agent ディレクトリにリンクします（copy モードで記録された skills はコピー）。対象は `--agent` で指定した agent、指定がなければ前回選択した agent、それもなければ検出された agent です。npm 依存関係のインストールは行いません。インストール済みでファイルが lock の内容 hash と一致する skills は再取得されず、不足している agent リンクだけが追加されます。内容が異なる skills はソースから復元され、`--force` はすべての skills を再インストールします。Frozen-lock モードでは内容の hash を検証し、CI ではこれがデフォルトです。
- **`experimental_sync`** はインストール済み npm 依存関係に含まれる skills をリンクし、`package.json` の skill 宣言を解決します。`skills-lock.json` で追跡し、提供されなくなった skills を削除し、別途追加した skills は保持します。Copy の場合、依存関係の更新後に再同期が必要です。

これらは実際の同期・復元機能ですが、AI ツール設定全体のバックアップではありません。

## 適切なツールの選び方

**npx skills を使い続ける**：主な目的が公開・非公開 skills の発見、インストール、更新なら、複数 agent を使う場合も合理的です。Skills を npm プロジェクトの依存関係に追従させたく、実験的なワークフローを受け入れられる場合にも適しています。

**Skillshare を選ぶ**：たとえば「Claude Code と Codex を使っていて、管理対象の skills、rules、MCP 設定、対応する hooks を別のマシンに持っていきたい」場合です。Source／target 設定、Git ワークフロー、ローカル dashboard、復旧ツールが、この広い管理ニーズに対応します。

移行先でも準備は必要です。Git の範囲を選び、targets を設定し、認証情報と外部依存関係を別途用意してください。`push` のデフォルトは skills の source であり、全設定の公開やマシン全体のバックアップは行いません。[Git root の範囲](/docs/reference/commands/push)と [sync](/docs/reference/commands/sync) を参照してください。

## Skillshare への移行

正常に動いているインストーラーをすぐに置き換える必要はありません。Skillshare に管理させたいリソースだけを移し、同じ管理対象 skill を両ツールで更新・削除しないでください。

現在の移行は、情報をすべて保持する lock インポートではありません。収集した skills はローカルコピーであり、元リポジトリの追跡情報を保持しません。Skillshare で更新を続けたい場合は、記録されたソースから再インストールしてください。管理を移す前にバックアップとプレビューを行い、収集だけで完了とせず [npx skills 移行ガイド](/docs/how-to/advanced/migration)に従ってください。

## 関連項目

- [移行](/docs/how-to/advanced/migration)
- [コアコンセプト](/docs/understand)
- [バックアップ](/docs/reference/commands/backup)
