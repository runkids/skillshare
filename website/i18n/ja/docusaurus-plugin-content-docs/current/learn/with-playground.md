---
sidebar_position: 6
---

# Playground で skillshare を試す

> デモ Skill、監査ルール、プロジェクトが事前設定された Docker サンドボックス — 数秒で探索を始められます。

## 前提条件

- Docker と Docker Compose がインストールされていること
- skillshare リポジトリを clone する: `git clone https://github.com/runkids/skillshare.git`

## Playground を起動する

```bash
cd skillshare
make playground
```

このコマンド1つで:

1. サンドボックスの Docker イメージをビルドする（Go ツールチェーン込み）
2. コンテナ内で `skillshare` バイナリをコンパイルする
3. すべての Target が自動検出された状態で Global mode を初期化する
4. カテゴリ別にデモ Skill（clean、warning、critical）を作成する
5. プロジェクトレベルの Skill とカスタム監査ルールを持つデモプロジェクトをセットアップする
6. インタラクティブなシェルに入る — すぐに探索できる状態

## 中身

### デモ Skill（Global）

| Skill | カテゴリ | 監査の検出結果 |
|-------|----------|----------------|
| `audit-demo-clean` | root | なし（クリーンなベースライン） |
| `deploy-checklist` | `devops/` | なし |
| `audit-demo-ci-release` | `security/` | HIGH + MEDIUM（sudo、外部 URL） |
| `audit-demo-debug-exfil` | `security/` | CRITICAL（認証情報の流出） |
| `audit-demo-external-link` | `security/` | LOW（外部 URL） |
| `audit-demo-dangling-link` | `security/` | LOW（欠落したリンク）+ HIGH（ルート外への参照） |

### デモプロジェクト（`~/demo-project`）

事前設定された `.skillshare/` プロジェクトには以下が含まれます。
- `hello-world` — クリーンなプロジェクトの Skill
- `demos/audit-demo-release` — 監査の警告があるリリースヘルパー
- `guides/code-review` — ネストされたコードレビューガイド
- TODO/FIXME ポリシールール付きのカスタム `audit-rules.yaml`

### カスタム監査ルール

Global レベルと Project レベルの両方の `audit-rules.yaml` が事前設定されているため、ルールの
カスタマイズがどう機能するかを確認できます — ルールの有効化/無効化、カスタムパターンの追加、
許可リストの設定。

## 試してみること

```bash
# 何がインストールされているか確認する
skillshare status
skillshare list

# セキュリティ監査を実行する — さまざまな重大度レベルの検出結果を見る
skillshare audit

# Project mode を試す
cd ~/demo-project
skillshare status          # Project mode を自動検出
skillshare audit           # カスタムルールでのプロジェクトレベルのスキャン

# Web ダッシュボードを起動する（ポート 19420）
skillshare-ui              # Global mode
skillshare-ui-p            # Project mode

# ネストされた Skill を探索する
ls ~/.config/skillshare/skills/security/
ls ~/.config/skillshare/skills/devops/
```

## Bare モード

自動初期化やデモコンテンツなしの、まっさらな状態から始めます。

```bash
./scripts/sandbox_playground_up.sh --bare
./scripts/sandbox_playground_shell.sh
```

`skillshare init` をゼロからテストするのに便利です。

## Playground を停止する

```bash
make playground-down
```

データは Docker ボリューム（`playground-home`）に永続化されます。次に `make playground` を実行すると
続きから始められます。

## アーキテクチャ

Playground はセキュリティが強化された**読み取り専用**の Docker コンテナで実行されます。

- `read_only: true` — 指定されたボリュームを除きファイルシステムは不変
- `cap_drop: ALL` — Linux の capability なし
- `no-new-privileges` — 権限昇格を防ぐ
- 書き込み可能なボリューム: `/sandbox-home`（永続）、`/tmp`（tmpfs、256 MB）
- Web ダッシュボード用にポート `19420` が転送される

ワークスペースはホストのリポジトリから読み取り専用でマウントされます — 自分のマシンでコードを編集し、
コンテナ内で再ビルドできます。

## 次のステップ

- [はじめに →](/docs/getting-started)
- [セキュリティ監査ガイド →](/docs/how-to/advanced/security)
- [Docker サンドボックスガイド →](/docs/how-to/advanced/docker-sandbox)
