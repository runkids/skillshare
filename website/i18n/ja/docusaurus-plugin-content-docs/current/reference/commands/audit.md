---
sidebar_position: 3
---

# audit

インストール済みの Skill をスキャンし、セキュリティ上の脅威や悪意のあるパターンを検出します。

```bash
skillshare audit                        # インストール済みの全 Skill をスキャン
skillshare audit <name>                 # 特定のインストール済み Skill をスキャン
skillshare audit a b c                  # 複数の Skill をスキャン
skillshare audit --group frontend       # グループ内の全 Skill をスキャン
skillshare audit <path>                 # ファイル/ディレクトリのパスをスキャン
skillshare audit --threshold high       # HIGH 以上の findings でブロック
skillshare audit -T h                   # --threshold high と同じ
skillshare audit --format json           # JSON 出力
skillshare audit --format sarif         # SARIF 2.1.0 出力（GitHub Code Scanning）
skillshare audit --format markdown      # Markdown レポート（GitHub Issues/PR 向け）
skillshare audit --json                 # --format json と同じ（非推奨）
skillshare audit -p                     # project の Skill をスキャン
skillshare audit --quiet                # findings のある Skill のみ表示
skillshare audit --yes                  # 大規模スキャンの確認をスキップ
skillshare audit --no-tui               # プレーンテキスト出力（インタラクティブ TUI なし）
skillshare audit --profile strict       # strict プロファイルを使用（HIGH 以上でブロック）
skillshare audit --dedupe global        # 完全な composite-key 重複排除
skillshare audit --analyzer static      # static analyzer のみ実行
skillshare audit --analyzer static --analyzer dataflow  # 複数の analyzer
```

## 使うタイミング

- 新しい Skill をインストールした後にセキュリティの findings を確認する
- プロンプトインジェクション、データ流出、認証情報アクセスのパターンについて全 Skill をスキャンする
- 組織のセキュリティポリシーに合わせて audit ルールをカスタマイズする
- コンプライアンス向け（`--format json`）、静的解析ツール向け（`--format sarif`）、またはドキュメント向け（`--format markdown`）の audit レポートを生成する
- CI/CD パイプラインに組み込んで Skill のデプロイをゲートする
- SARIF の結果を GitHub Code Scanning にアップロードして PR レベルの注釈を行う

## 検出内容

audit エンジンは、Skill ディレクトリ内のすべてのテキストベースのファイルを 100 以上の組み込みルール（正規表現パターン、テーブル駆動の認証情報検出、構造チェック、コンテンツの整合性検証、サプライチェーンの信頼性分析）と照合し、5 段階の重大度（**CRITICAL**、**HIGH**、**MEDIUM**、**LOW**、**INFO**）に分類します。

完全な検出カタログ、脅威カテゴリの詳細、リスクスコアリングアルゴリズム、コマンド安全性のティア分類、Skill 間の相互作用分析については、[Audit Engine](/docs/understand/audit-engine) を参照してください。

## 出力例

```
skillshare audit
Audit  ~/.config/skillshare/skills
  global · blocks at CRITICAL · policy DEFAULT / dedupe:GLOBAL / analyzers:ALL

! ci-release-helper  HIGH · risk 25/100
✗ suspicious-skill   CRITICAL · risk 35/100

Summary
  Scanned      12 skills
✓ Passed       9
! Warning      2
✗ Failed       1
  Severity     1 critical, 2 high, 1 medium
  Threats      credential:1 exfiltration:1 injection:1 privilege:1
  Risk         HIGH 35/100 · 100% auditable

✗ Blocked 1 of 12 skills: findings at CRITICAL or above · 2.1s
  The risk score is informational; only the severity blocks

Next
  skillshare audit suspicious-skill  see its findings
```

`Failed` は、有効な閾値（`--threshold` または config の `audit.block_threshold`、デフォルトは `CRITICAL`）以上の findings を持つ Skill の数を数えます。

`Threats` は、短縮名を使ってすべての findings をカテゴリ別に内訳表示します: `inj`（injection）、`exfil`（exfiltration）、`cred`（credential）、`obfusc`（obfuscation）、`priv`（privilege）、`integ`（integrity）、`struct`（structure）、`risk`（risk）。findings がない場合、この行は省略されます。ターミナル出力では、各カテゴリは脅威タイプごとに色分けされます。

`audit.block_threshold` はブロックの閾値のみを制御します。スキャン自体を無効化することは**ありません**。

### インタラクティブ TUI モード

インタラクティブなターミナルで複数の Skill をスキャンすると、`audit` は結果を 1 行ずつ出力する代わりにフルスクリーンの画面を開きます。左側に Skill（問題のあるものが先頭）、右側に選択した Skill のリスク概要と検出結果が表示されます。Skill を開くと、検出結果ごとにそのファイルの該当行が行番号付きでハイライト表示されます。キーは画面下部に表示されます。

この画面が開くのは、ターミナルがインタラクティブで、出力が JSON ではなく、結果が複数ある場合だけです。プレーンテキストを強制するには `--no-tui` を使用します。

### 大規模スキャンの確認

インタラクティブなターミナルで 1,000 を超える Skill をスキャンする場合、コマンドは実行前に確認を求めます。TTY 環境（例: ローカルの自動化スクリプト）でこのプロンプトをスキップするには `--yes` を使用します。CI/CD パイプライン（非 TTY）では、このプロンプトは自動的にスキップされます。

## ポリシーとプロファイル

audit コマンドは、プロファイル、重複排除モード、analyzer の選択を通じた**ポリシー駆動**の設定をサポートします。これらは CLI フラグ、project config、またはグローバル config で設定できます。

### プロファイル

プロファイルは、閾値と重複排除の適切なデフォルトを設定するプリセットです。

| プロファイル | 閾値 | 重複排除 | 用途 |
|---------|-----------|--------|----------|
| `default` | `CRITICAL` | `global` | 標準的な動作 — critical な脅威のみをブロック |
| `strict` | `HIGH` | `global` | セキュリティ重視のチーム — high 以上の脅威をブロック |
| `permissive` | `CRITICAL` | `legacy` | アドバイザリ専用 — 最小限のブロック、グローバル重複排除なし |

```bash
skillshare audit --profile strict       # HIGH 以上でブロック、グローバル重複排除
skillshare audit --profile permissive   # アドバイザリモード
```

明示的なフラグは常にプロファイルのデフォルトより優先されます。

```bash
skillshare audit --profile strict --threshold medium  # strict プロファイルだが MEDIUM 以上でブロック
```

### 重複排除

同じ finding が複数の analyzer（例: static と dataflow の両方）で検出された場合、重複排除は冗長なエントリを削除します。

| モード | 動作 |
|------|------|
| `global` | すべての findings に対する完全な composite-key 重複排除（デフォルト） |
| `legacy` | analyzer 単位のみの重複排除（v0.16.9 以前の動作） |

### Analyzer の選択

デフォルトではすべての analyzer が実行されます。特定の analyzer のみを実行するには `--analyzer` を使用します。

```bash
skillshare audit --analyzer static                    # static パターンマッチングのみ
skillshare audit --analyzer static --analyzer dataflow # 複数の analyzer
```

| Analyzer | スコープ | 説明 |
|----------|-------|-------------|
| `static` | ファイル単位 | audit ルールに対する正規表現ベースのパターンマッチング |
| `dataflow` | ファイル単位 | shell スクリプトと markdown コードブロックの taint tracking |
| `tier` | Skill 単位 | Capability tier の組み合わせリスク分析 |
| `integrity` | Skill 単位 | コンテンツハッシュ検証（SKILL.md の `file_hashes`） |
| `metadata` | Skill 単位 | サプライチェーンの信頼性検証（publisher の不一致、権威の主張） |
| `structure` | Skill 単位 | markdown のダングリングリンク検出 |
| `cross-skill` | バンドル単位 | Skill 間のデータ流出・権限昇格分析 |

config で設定することもできます。

```yaml
audit:
  enabled_analyzers: [static, dataflow]
```

### 優先順位 {#precedence}

設定は次の順序で解決されます（最初に空でない値が優先されます）。

1. CLI フラグ（`--profile`, `--threshold`, `--dedupe`, `--analyzer`）
2. Project config（`.skillshare/config.yaml`）
3. グローバル config（`~/.config/skillshare/config.yaml`）
4. プロファイルのデフォルト

## 自動スキャン

### インストール時

Skill はインストール中に自動的にスキャンされます。`audit.block_threshold`（デフォルト: `CRITICAL`）以上の findings があるとインストールがブロックされます。

```bash
skillshare install /path/to/evil-skill
# Error: security audit failed: critical threats detected in skill

skillshare install /path/to/evil-skill --force
# 警告付きでインストールする（注意して使用）

skillshare install /path/to/skill --audit-threshold high
# コマンド単位のブロック閾値の上書き

skillshare install /path/to/skill -T h
# --audit-threshold high と同じ

skillshare install /path/to/skill --skip-audit
# スキャンをバイパスする（注意して使用）
```

`--force` はブロックの判定を上書きします。`--skip-audit` は、そのインストールコマンドに対するスキャンを無効化します。

install 時の audit をグローバルに無効化する config フラグはありません。意図的にスキャンをバイパスしたいコマンドに限って `--skip-audit` を使用してください。

差異のまとめ:

| Install フラグ | Audit は実行される? | Findings は利用可能? |
|--------------|-------------|---------------------|
| `--force` | はい | はい（インストールは続行される） |
| `--skip-audit` | いいえ | いいえ（スキャンはバイパスされる） |

両方が指定された場合、audit が実行されないため実質的に `--skip-audit` が優先されます。

### 更新時

`skillshare update` は、追跡中のリポジトリを pull した後にセキュリティ audit を実行します。有効な閾値（デフォルトは `audit.block_threshold`、または `--audit-threshold` / `--threshold` / `-T` による上書き）以上の findings があると、ロールバックが発生します。詳細は [`update --skip-audit`](/docs/reference/commands/update#security-audit-gate) を参照してください。

`--force` で受け入れた findings はその Skill について記憶されるため、後続の更新で同じルールが同じテキストに一致しても再びブロックされることはありません。新しい finding、または同じルールが異なるテキストに一致した場合は、再びブロックされます。詳細は [Accepted Findings](/docs/reference/commands/update#accepted-findings) を参照してください。

install 経由で追跡中のリポジトリを更新する場合（`skillshare install <repo> --track --update`）、ゲートは同じ閾値ポリシー（`audit.block_threshold` または `--audit-threshold` / `--threshold` / `-T`）を使用します。

## CI/CD 統合

`audit` コマンドはパイプライン自動化向けに設計されています。非 TTY 環境（CI ランナー、パイプ出力）では、インタラクティブ TUI と確認プロンプトは自動的に無効化されます — `--yes` や `--no-tui` は不要です。

完全な CI/CD ワークフロー（GitHub Actions、GitLab CI、SARIF アップロード、出力フォーマット）については、[CI/CD Skill Validation recipe](/docs/how-to/recipes/ci-cd-skill-validation) を参照してください。

### Pre-commit フック

[pre-commit](https://pre-commit.com/) フレームワークを使って、コミットごとに `skillshare audit` を自動実行します。このフックは `.skillshare/` または `skills/` ディレクトリに一致するファイルをスキャンし、findings が設定した閾値を超える場合にコミットをブロックします。

```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/runkids/skillshare
    rev: v0.16.11  # 最新のリリースタグを使用
    hooks:
      - id: skillshare-audit
```

完全なセットアップ手順は [Pre-commit Hook recipe](/docs/how-to/recipes/pre-commit-hook) を参照してください。

## ベストプラクティス

### 個人開発者向け

- **信頼する前に audit する** — 信頼できないソースから Skill をインストールした後は、必ず `skillshare audit` を実行する
- **合否だけでなく findings を確認する** — 「合格」した Skill でも、調査する価値のある LOW/MEDIUM の findings がある場合がある
- **Skill ファイルを読む** — 自動スキャンは既知のパターンを検出するが、新種の攻撃には人間によるレビューが必要

### チームおよび組織向け

- **`audit.block_threshold: HIGH` を設定する** — デフォルトの `CRITICAL` より厳しくし、難読化や破壊的なコマンドを検出する
- **組織全体のカスタムルールを作成する** — 社内のシークレット形式（例: `corp-api-key-*`）用のパターンを追加する
- **オーバーライドには project モードのルールを使う** — グローバルではなく project 単位で、想定されるパターンの重大度を引き下げる

### 推奨される Audit ワークフロー

1. **インストール**: Skill は自動的にスキャンされる — 閾値を超えるとブロックされる
2. **定期スキャン**: インストール後に更新されたルールを検出するために、定期的に `skillshare audit` を実行する
3. **Pre-commit フック**: [pre-commit フレームワーク](/docs/how-to/recipes/pre-commit-hook) でコミット前に問題を検出する
4. **CI ゲート**: 共有 Skill リポジトリのために、CI パイプラインに audit を追加する
5. **カスタムルール**: 組織の脅威モデルに合わせて検出をカスタマイズする
6. **レポートを確認する**: コンプライアンス向けには `--format json`、GitHub Code Scanning 向けには `--format sarif`、GitHub Issues/PR 向けには `--format markdown` を使用する

### 閾値の設定

config ファイルでブロック閾値を設定します。

```yaml
# ~/.config/skillshare/config.yaml
audit:
  block_threshold: HIGH  # HIGH 以上でブロック（デフォルトの CRITICAL より厳しい）
```

またはコマンド単位で設定します。

```bash
skillshare audit --threshold medium  # MEDIUM 以上でブロック
```

### 完全な Audit 設定

すべての audit 設定は `config.yaml` に永続化できます。

```yaml
# ~/.config/skillshare/config.yaml (project の場合は .skillshare/config.yaml)
audit:
  block_threshold: HIGH                         # ブロックの重大度ゲート
  profile: strict                               # プロファイルのプリセット（default/strict/permissive）
  dedupe_mode: global                           # 重複排除モード（global/legacy）
  enabled_analyzers: [static, dataflow, tier]   # 特定の analyzer に限定
```

CLI フラグは config の値より優先されます。完全な解決順序については [優先順位](#precedence) を参照してください。

`skillshare status` コマンドは、すべての優先順位のレイヤーを適用した後の、有効なプロファイル、閾値、重複排除モード、analyzer リストを表示する、解決済みの audit ポリシーを表示します。

## Web UI

audit 機能は、Web ダッシュボードの `/audit` でも利用できます。

```bash
skillshare ui
# Audit ページに移動 → 「Run Audit」をクリック
```

![Security Audit page in web dashboard](/img/web-audit-demo.png)

Dashboard ページには、クイックスキャンのサマリーを含む Security Audit セクションがあります。

### カスタムルールエディタ

Web ダッシュボードには、ブラウザ上で直接カスタムルールを作成・編集するための専用の **Audit Rules** ページが `/audit/rules` にあります。

- **作成**: `audit-rules.yaml` が存在しない場合、「Create Rules File」をクリックしてスキャフォールドする
- **編集**: シンタックスハイライトと検証機能付きの YAML エディタ
- **保存**: 保存前に YAML フォーマットと正規表現パターンを検証する

Audit ページの「Custom Rules」ボタンからアクセスできます。

## 終了コード

| コード | 意味 |
|------|---------|
| `0` | 有効な閾値以上の findings なし |
| `1` | 有効な閾値以上の findings が 1 件以上ある |

## スキャン対象ファイル

audit は Skill ディレクトリ内のテキストベースのファイルをスキャンします。

- `.md`, `.txt`, `.yaml`, `.yml`, `.json`, `.toml`
- `.sh`, `.bash`, `.zsh`, `.fish`
- `.py`, `.js`, `.ts`, `.rb`, `.go`, `.rs`
- 拡張子のないファイル（例: `Makefile`, `Dockerfile`）

スキャンは各 Skill ディレクトリ内で再帰的に行われるため、サポート対象のテキストファイルタイプに一致する限り、`SKILL.md`、ネストした `references/*.md`、`scripts/*.sh` もすべて検査されます。

バイナリファイル（画像、`.wasm` など）と隠しディレクトリ（`.git`）はスキップされます。

## オプション

| フラグ | 説明 |
|------|------------|
| `-G`, `--group` `<name>` | グループ内のすべての Skill をスキャン（複数指定可） |
| `-p`, `--project` | project レベルの Skill をスキャン |
| `-g`, `--global` | グローバルな Skill をスキャン |
| `--threshold` `<t>`, `-T` `<t>` | ブロック閾値: `critical`\|`high`\|`medium`\|`low`\|`info`（省略形: `c`\|`h`\|`m`\|`l`\|`i`、加えて `crit`, `med`） |
| `--profile` `<p>` | Audit プロファイルのプリセット: `default`, `strict`, `permissive` |
| `--dedupe` `<mode>` | 重複排除モード: `legacy`, `global`（デフォルト） |
| `--analyzer` `<id>` | 指定した analyzer のみ実行（複数指定可）。ID: `static`, `dataflow`, `tier`, `integrity`, `metadata`, `structure`, `cross-skill` |
| `--format` `<f>` | 出力フォーマット: `text`（デフォルト）, `json`, `sarif`, `markdown` |
| `--json` | JSON を出力（**非推奨**: `--format json` を使用） |
| `--yes`, `-y` | 大規模スキャンの確認プロンプトをスキップ（自動確認） |
| `--quiet`, `-q` | findings のある Skill とサマリーのみ表示（クリーンな ✓ 行を抑制） |
| `--no-tui` | インタラクティブ TUI を無効化し、プレーンテキストを出力 |
| `--init-rules` | スターター用の `audit-rules.yaml` を作成（`-p`/`-g` に対応） |
| `-h`, `--help` | ヘルプを表示 |

### サブコマンド

| サブコマンド | 説明 |
|-----------|-------------|
| `rules` | audit ルールの閲覧、有効化、無効化（[`audit rules`](/docs/reference/commands/audit-rules) を参照） |

## Agent サポート

`skillshare audit agents` はセキュリティスキャンの対象を agent のみに絞り、agent の source ディレクトリ内の `.md` ファイルをスキャンします。

```bash
skillshare audit agents                    # すべての agent をスキャン
skillshare audit agents --threshold high   # agent の HIGH 以上でブロック
skillshare audit agents --format sarif     # agent 向けの SARIF 出力
skillshare audit agents -p                 # project の agent をスキャン
```

Agent は、Skill と同じ audit ルール、重大度レベル、閾値ゲートの対象になります。`agents` 引数を指定しない場合、`audit` は Skill のみをスキャンします（デフォルトの動作）。背景については [Agents](/docs/understand/agents) を参照してください。

## 関連項目

- [Audit Engine](/docs/understand/audit-engine) — エンジンの仕組み（脅威モデル、リスクスコアリング、コマンドのティア分類）
- [`audit rules`](/docs/reference/commands/audit-rules) — ルールの管理とカスタマイズ
- [install](/docs/reference/commands/install) — Skill をインストール（自動スキャン付き）
- [check](/docs/reference/commands/check) — Skill の整合性と同期状態を検証
- [doctor](/docs/reference/commands/doctor) — セットアップの問題を診断
- [list](/docs/reference/commands/list) — インストール済みの Skill を一覧表示
- [Securing Your Skills](/docs/how-to/advanced/security) — チームや組織向けのセキュリティガイド
- [CI/CD Skill Validation](/docs/how-to/recipes/ci-cd-skill-validation) — パイプライン自動化のレシピ
- [Pre-commit Hook](/docs/how-to/recipes/pre-commit-hook) — コミットごとの自動 audit
- [Agents](/docs/understand/agents) — Agent の概念
