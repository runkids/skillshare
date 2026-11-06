---
sidebar_position: 1
---

# Appendix

skillshare の技術リファレンス Appendix です。

## 何をお探しですか？

| トピック | 読むもの |
|-------|------|
| skillshare に影響する環境変数 | [Environment Variables](/docs/reference/appendix/environment-variables) |
| skillshare が config、Skill、ログ、キャッシュを保存する場所 | [File Structure](/docs/reference/appendix/file-structure) |
| 対応する git URL フォーマット | [URL Formats](/docs/reference/appendix/url-formats) |
| Config ファイルのフォーマットとオプション | [Configuration](/docs/reference/targets/configuration) |
| すべての CLI コマンド | [Commands](/docs/reference/commands) |

## クイックリファレンス

### 主要なパス（Unix）

| パス | 用途 |
|------|---------|
| `~/.config/skillshare/config.yaml` | 設定ファイル |
| `~/.config/skillshare/skills/` | Source ディレクトリ（あなたの Skill） |
| `~/.config/skillshare/skills/.metadata.json` | インストール済み Skill のメタデータ（自動管理） |
| `~/.local/share/skillshare/backups/` | バックアップディレクトリ |
| `~/.local/share/skillshare/trash/` | ソフトデリートされた Skill |
| `~/.local/state/skillshare/logs/` | 操作ログと監査ログ |
| `~/.local/state/skillshare/projects/` | マシンローカルのプロジェクト登録とロック |
| `~/.cache/skillshare/ui/` | ダウンロードされた Web ダッシュボード |

### 環境変数

| 変数 | 用途 |
|----------|---------|
| `SKILLSHARE_CONFIG` | Config パスを上書きする |
| `GITHUB_TOKEN` | GitHub API 認証 |

## 関連項目

- [Configuration](/docs/reference/targets/configuration) — Config ファイルの詳細
- [Commands](/docs/reference/commands) — すべてのコマンド
