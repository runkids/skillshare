---
sidebar_position: 2
---

# セキュリティファーストの設計

> AI Skill は実行可能な命令です。skillshare はこれを信頼できない入力として扱います。

## 脅威モデル

GitHub から Skill をインストールすると、コード生成やファイル変更、場合によってはコマンド実行にまで影響を与える命令を AI ツールに与えることになります。悪意ある Skill は次のようなことをする可能性があります。

- **プロンプトインジェクション**によって AI の安全ガイドラインを上書きする
- ファイルの内容を外部 URL に送信するよう AI に指示して**データを窃取する**
- AI のシェルアクセスを通じて**破壊的なコマンドを実行する**
- 環境変数や設定ファイルにアクセスして**認証情報を盗む**

これは理論上の話ではありません。プロンプトインジェクションは AI ツール群における最大級のセキュリティ懸念事項です。

## Audit エンジン

skillshare には組み込みのセキュリティスキャナー（`skillshare audit`）が搭載されており、インストール済みのすべての Skill を、5 段階の深刻度にわたる 15 以上の検出パターンと照合します。

| 深刻度 | 例 |
|----------|------|
| CRITICAL | プロンプトインジェクション、システムプロンプトの上書き |
| HIGH | データ窃取用の URL、認証情報へのアクセスパターン |
| MEDIUM | 破壊的なコマンド（`rm -rf`、`DROP TABLE`）、ファイルシステムへの書き込み |
| LOW | ネットワークリクエスト、外部ツールの呼び出し |
| INFO | 大きすぎるファイルサイズ、不自然なフォーマット |

### 仕組み

Audit エンジンは、パターンマッチングとヒューリスティクスを用いて SKILL.md の内容をスキャンします。

```bash
# Scan all installed skills
skillshare audit

# JSON output for CI integration
skillshare audit --json

# Scan project skills only
skillshare audit -p
```

### 自動ブロック

`skillshare install` の実行中には Audit が自動的に実行されます。CRITICAL の指摘が検出された場合、インストールはブロックされます。

```
CRITICAL: Prompt injection detected in "malicious-skill"
  → Pattern: "ignore previous instructions"
  → Installation blocked. Use --force to override (not recommended).
```

## 多層防御

Audit エンジンはその一層にすぎません。skillshare のセキュリティモデルには次が含まれます。

1. **インストール時の Audit** — 脅威が AI ツールに到達する前に検出する
2. **オンデマンドの Audit** — 新しいパターンが追加されるたびに既存の Skill を再スキャンする
3. **共有 Source** — リンクされた Target は同じ Skill ファイルを読み、Source の変更がすべてに届くため、変更をレビューし、Source に書き込める人を制限する
4. **変更前のバックアップ** — `skillshare backup` が Skill ライブラリ全体をスナップショットする
5. **TTL 付きの Trash** — 削除された Skill はまず Trash に入り、即座には完全削除されない
6. **操作ログ** — すべての変更操作は `operations.log`（JSONL）に記録される

Symlink は共通の Source を提供しますが、セキュリティサンドボックスではありません。同様に、`.skillignore` は Skill の検出を絞り込むもので、AI ツールによるファイルやコマンドへのアクセスを禁止しません。実行時の権限と承認は AI ツールが担当します。責任の分担については[共有 Source と実行の境界](/docs/how-to/advanced/security#shared-source-and-execution-boundaries)を参照してください。

## サプライチェーンに関する考慮事項

AI Skill エコシステムはまだ若く、レビュープロセスを備えたパッケージレジストリも、コード署名も、依存関係解決の仕組みもありません。Skill は git リポジトリ内の Markdown ファイルにすぎません。

skillshare のアプローチは次のとおりです。
- **すべてをスキャンする** — 信頼できるソースからの Skill であっても
- **デフォルトでブロックする** — CRITICAL の指摘はインストールを阻止する
- **すべてを記録する** — Audit の結果はフォレンジックレビューのために保存される
- **パターンを更新する** — skillshare の各リリースで新しい検出パターンが追加される

## Audit の挙動を設定する

`config.yaml` でブロックの閾値を設定し、どの深刻度でインストールをブロックするかを制御できます。

```yaml
# config.yaml
audit:
  block_threshold: HIGH   # Block on HIGH and CRITICAL (default: CRITICAL)
```

ルールごとのカスタマイズには、別途 `audit-rules.yaml` ファイルを使用します（`skillshare audit --init-rules` で初期化）。

```yaml
# audit-rules.yaml
rules:
  - id: network-request-0
    enabled: false          # Disable this specific rule
  - id: my-custom-check
    severity: MEDIUM
    pattern: "TODO|FIXME"
    description: Policy violation — unresolved TODOs
```

## 関連項目

- [`audit` コマンドリファレンス](/docs/reference/commands/audit)
- [セキュリティガイド](/docs/how-to/advanced/security)
- [CI/CD 検証レシピ](/docs/how-to/recipes/ci-cd-skill-validation)
