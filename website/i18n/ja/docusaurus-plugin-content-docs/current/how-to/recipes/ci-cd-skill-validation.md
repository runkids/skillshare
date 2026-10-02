---
sidebar_position: 2
---

# レシピ: CI/CD Skill 検証

> CI パイプラインで Skill を自動的に監査・Sync する。

## シナリオ

チームの Skill リポジトリがあり、すべての PR が以下を満たすようにしたいとします。
- セキュリティ監査を通過する（プロンプトインジェクションや認証情報の窃取などがない）
- SKILL.md フォーマットを検証する
- エラーなく Sync できる

## 解決策

### GitHub Actions（setup-skillshare を使用）

[`setup-skillshare`](https://github.com/marketplace/actions/setup-skillshare) アクションは、
インストール、初期化、任意のセキュリティ監査を1ステップで処理します。

```yaml
name: Skill Validation
on:
  pull_request:
    paths:
      - 'skills/**'

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: runkids/setup-skillshare@v1
        with:
          source: ./skills
          audit: true
          audit-threshold: high
      - run: skillshare sync --dry-run
```

### SARIF アップロード付きの GitHub Actions

[GitHub Code Scanning](https://docs.github.com/en/code-security/code-scanning) でインラインの PR アノテーションを表示するには、SARIF 出力を使います。`audit-output` を指定すると、Action は失敗する代わりに結果を保存して終了コードを記録するため、SARIF ファイルをアップロードできます。最後に、検出結果があれば job を失敗させるステップを追加してください:

```yaml
name: Skill Security Scan
on:
  pull_request:
    paths: ['skills/**']
  push:
    branches: [main]

jobs:
  validate:
    runs-on: ubuntu-latest
    permissions:
      security-events: write
    steps:
      - uses: actions/checkout@v4
      - uses: runkids/setup-skillshare@v1
        id: skillshare
        with:
          source: ./skills
          audit: true
          audit-threshold: high
          audit-format: sarif
          audit-output: results.sarif

      - name: Upload SARIF to Code Scanning
        if: always()
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: results.sarif
          category: skillshare-audit

      - run: skillshare sync --dry-run

      - name: Fail on audit findings
        if: steps.skillshare.outputs.audit-exit-code != '0'
        run: exit 1
```

### アクションを使わない場合（手動セットアップ）

アクションを使いたくない場合は、skillshare を直接インストールできます。

```yaml
jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: |
          curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
          echo "$HOME/.local/bin" >> "$GITHUB_PATH"
      - run: skillshare init --no-copy --all-targets --no-git --no-skill --source ./skills
      - run: skillshare audit --threshold high --format json
      - run: skillshare sync --dry-run
```

### GitLab CI

`.gitlab-ci.yml` を作成します。

```yaml
skill-validation:
  image: ghcr.io/runkids/skillshare-ci:latest
  stage: test
  script:
    - skillshare init
    - skillshare install . --into ci-check
    - skillshare audit --threshold high --format json
    - skillshare sync --dry-run
  rules:
    - changes:
        - skills/**/*
```

### CI Docker イメージを使う

パイプラインの起動を速くするには、ビルド済みの CI イメージを使います。

```yaml
# GitHub Actions
jobs:
  validate:
    runs-on: ubuntu-latest
    container:
      image: ghcr.io/runkids/skillshare-ci:latest
    steps:
      - uses: actions/checkout@v4
      - run: skillshare init && skillshare audit --format json
```

## 出力フォーマット

`audit` コマンドは、さまざまな CI/CD 統合のニーズに応じて複数の出力フォーマットに対応しています。

### 終了コード

```bash
# しきい値以上の検出結果がある Skill があればデプロイをブロックする
skillshare audit --threshold high
echo $?  # 0 = クリーン、1 = 検出結果あり
```

### SARIF 出力

[SARIF（Static Analysis Results Interchange Format）](https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html)
は、GitHub Code Scanning、VS Code SARIF Viewer、Azure DevOps、SonarQube、その他の静的解析ツールで
利用される OASIS 標準です。

```bash
skillshare audit --format sarif              # 標準出力へ
skillshare audit --format sarif > results.sarif  # ファイルへ保存
```

SARIF 出力には以下が含まれます。
- **ツールメタデータ** — ツール名（`skillshare`）、バージョン、情報 URI
- **ルール** — `security-severity` スコア付きの重複排除されたルール記述子
- **結果** — ファイルの位置と重大度レベルを含む、各検出結果を SARIF result にマッピングしたもの

重大度から SARIF レベルへのマッピング:

| skillshare の重大度 | SARIF レベル | security-severity |
|---------------------|-------------|-------------------|
| CRITICAL | `error` | 9.0 |
| HIGH | `error` | 7.0 |
| MEDIUM | `warning` | 4.0 |
| LOW | `note` | 2.0 |
| INFO | `note` | 0.5 |

### Markdown レポート

GitHub Issues、Pull Request、ドキュメントへの貼り付けに適した、自己完結型の Markdown レポートを
生成します。

```bash
skillshare audit --format markdown               # 標準出力へ表示
skillshare audit --format markdown > report.md   # ファイルへ保存
skillshare audit -p --format markdown > report.md  # Project mode
```

レポートには以下が含まれます。
- **ヘッダー** — スキャン数、モード、しきい値
- **サマリーテーブル** — 合格/警告/失敗数、重大度の内訳、リスクスコア、分析可能性
- **検出結果** — 重大度、パターン、メッセージ、位置ごとの Skill 別テーブル。折りたたみ可能なスニペット
- **クリーンな Skill** — 検出結果のない Skill のカンマ区切りリスト

### jq を使った JSON 出力

```bash
# CRITICAL の検出結果があるすべての Skill を一覧表示する
skillshare audit --json | jq '[.skills[] | select(.findings[] | .severity == "CRITICAL")]'

# すべての Skill のリスクスコアを抽出する
skillshare audit --json | jq '.skills[] | {name: .skillName, score: .riskScore, label: .riskLabel}'

# 重大度ごとに検出結果をカウントする
skillshare audit --json | jq '[.skills[].findings[].severity] | group_by(.) | map({(.[0]): length}) | add'
```

## 確認

- PR チェックが通過する: audit が 0 で終了する（しきい値以上の検出結果なし）
- Audit の JSON 出力を下流のツールでパースできる
- SARIF アップロードが PR の diff にインラインアノテーションとして表示される
- Sync のドライランが期待されるシンボリックリンク操作を表示する

## バリエーション

- **HIGH の重大度でブロックする**: `audit` に `--threshold HIGH`（または `-T HIGH`）を追加する —
  HIGH 以上の検出結果があれば非ゼロで終了する
- **Code Scanning 用の SARIF**: インライン PR アノテーションには `github/codeql-action/upload-sarif@v3`
  とともに `--format sarif` を使う
- **並列検証**: audit と sync を別々の CI ジョブで実行し、フィードバックを速くする
- **定期監査**: 既存の Skill で新たに検出されたパターンを捕捉するため、毎晩実行する

## 関連項目

- [セキュリティ監査ガイド](/docs/how-to/advanced/security)
- [`audit` コマンドリファレンス](/docs/reference/commands/audit)
- [`audit rules` リファレンス](/docs/reference/commands/audit-rules)
- [監査エンジン](/docs/understand/audit-engine) — エンジンの仕組み
- [Docker サンドボックスガイド](/docs/how-to/advanced/docker-sandbox)
