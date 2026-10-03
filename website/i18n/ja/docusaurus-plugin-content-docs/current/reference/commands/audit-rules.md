---
sidebar_position: 4
---

# audit rules

audit ルールを閲覧、有効化、無効化、カスタマイズします。

```bash
skillshare audit rules                          # インタラクティブ TUI ルールブラウザ
skillshare audit rules --no-tui                 # プレーンテキストのテーブル
skillshare audit rules --pattern credential-access  # パターンでフィルタ
skillshare audit rules --severity high          # 重大度でフィルタ
skillshare audit rules --disabled               # 無効化されたルールのみ表示
skillshare audit rules --format json            # JSON 出力

skillshare audit rules disable prompt-injection-0           # 単一ルールを無効化
skillshare audit rules disable --pattern credential-access  # グループ全体を無効化
skillshare audit rules enable prompt-injection-0            # ルールを再有効化
skillshare audit rules enable --pattern credential-access   # グループを再有効化

skillshare audit rules severity destructive-commands-2 medium      # 単一ルールを引き下げ
skillshare audit rules severity --pattern destructive-commands low  # グループ全体を引き下げ
skillshare audit rules reset                    # カスタムルールをすべて削除し、デフォルトに戻す

skillshare audit rules init                     # スターター用の audit-rules.yaml を作成
skillshare audit rules init -p                  # project レベルのルールファイルを作成
```

## パターン単位のルール

`audit-rules.yaml` で、パターングループ全体を無効化またはオーバーライドできます。

```yaml
rules:
  # すべての credential-access ルールを無効化する
  - pattern: credential-access
    enabled: false

  # ただし .env の検出は維持する
  - id: credential-access-env-file
    enabled: true

  # すべての destructive-commands を MEDIUM に引き下げる
  - pattern: destructive-commands
    severity: MEDIUM
```

パターン単位のエントリは `id` なしで `pattern` を使用します。マージ順序: まずパターン単位のルールが適用され、その後 id 単位のルールが、無効化されたグループ内の個々のエントリを上書きできます。

## カスタムルール {#custom-rules}

YAML ファイルを使って audit ルールを追加、オーバーライド、無効化できます。ルールは **built-in → global user → project user** の順にマージされます。

コメント付きの例を含むスターターファイルを作成するには `--init-rules`（または `audit rules init`）を使用します。

```bash
skillshare audit --init-rules         # グローバルなルールファイルを作成
skillshare audit -p --init-rules      # project のルールファイルを作成
```

### ファイルの場所

| スコープ | パス |
|-------|------|
| グローバル | `~/.config/skillshare/audit-rules.yaml` |
| Project | `.skillshare/audit-rules.yaml` |

### フォーマット

```yaml
rules:
  # 新しいルールを追加する
  - id: my-custom-rule
    severity: HIGH
    pattern: custom-check
    message: "Custom pattern detected"
    regex: 'DANGEROUS_PATTERN'

  # exclude 付きのルールを追加する（特定の行での一致を抑制する）
  - id: url-check
    severity: MEDIUM
    pattern: url-usage
    message: "External URL detected"
    regex: 'https?://\S+'
    exclude: 'https?://(localhost|127\.0\.0\.1)'

  # 既存の built-in ルールをオーバーライドする（id で一致させる）
  - id: destructive-commands-2
    severity: MEDIUM
    pattern: destructive-commands
    message: "Sudo usage (downgraded to MEDIUM)"
    regex: '(?i)\bsudo\s+'

  # built-in ルールを無効化する
  - id: insecure-http-0
    enabled: false

  # dangling-link 構造チェックを無効化する
  - id: dangling-link
    enabled: false
```

### フィールド

| フィールド | 必須 | 説明 |
|-------|----------|------|
| `id` | はい | 安定した識別子。一致する ID は built-in ルールを上書きする。 |
| `severity` | はい* | `CRITICAL`, `HIGH`, `MEDIUM`, `LOW`, または `INFO` |
| `pattern` | はい* | ルールのカテゴリ名（例: `prompt-injection`） |
| `message` | はい* | findings に表示される人間可読な説明 |
| `regex` | はい* | 各行と照合する正規表現 |
| `exclude` | いいえ | 行が `regex` と `exclude` の両方に一致した場合、finding は抑制される |
| `enabled` | いいえ | `false` に設定するとルールを無効化する。無効化する場合は `id` のみが必要。 |

*`enabled: false` の場合を除き必須。

### マージのセマンティクス

各レイヤー（global、次に project）は、前のレイヤーの上に適用されます。

- **同じ `id`** + `enabled: false` → ルールを無効化する
- **同じ `id`** + 他のフィールド → ルール全体を置き換える
- **新しい `id`** → カスタムルールとして追加する
- **`pattern` のみ**（`id` なし）+ `enabled: false` → そのパターンに一致するすべてのルールを無効化する
- **`pattern` のみ** + `severity` → 一致するすべてのルールの重大度を上書きする
- **パターンの後に id** → id 単位のエントリは、無効化されたパターングループ内の個々のルールを再有効化できる

### 実用的なテンプレート

実際のポリシー調整のための出発点として、これを使用してください。

```yaml
rules:
  # 教育用/リファレンス用の Skill 向けに hardcoded-secret を MEDIUM に引き下げる
  - pattern: hardcoded-secret
    severity: MEDIUM

  # 内部の許可リストで built-in の suspicious-fetch を上書きする
  - id: suspicious-fetch-0
    severity: MEDIUM
    pattern: suspicious-fetch
    message: "External URL used in command context"
    regex: '(?i)(curl|wget|invoke-webrequest|iwr)\s+https?://'
    exclude: '(?i)https?://(localhost|127\.0\.0\.1|artifacts\.company\.internal|registry\.company\.internal)'

  # ガバナンス例外: ノイズの多い insecure-http のシグナルを無効化する
  - id: insecure-http-0
    enabled: false
```

### `init` で始める

`audit rules init`（または `audit --init-rules`）は、コメントアウトされた例を含むスターター用の `audit-rules.yaml` を作成します。これをアンコメントして調整できます。

```bash
skillshare audit rules init          # → ~/.config/skillshare/audit-rules.yaml
skillshare audit rules init -p       # → .skillshare/audit-rules.yaml
```

生成されるファイルは次のようになります。

```yaml
# Custom audit rules for skillshare.
# Rules are merged on top of built-in rules in order:
#   built-in → global (~/.config/skillshare/audit-rules.yaml)
#            → project (.skillshare/audit-rules.yaml)
#
# Each rule needs: id, severity, pattern, message, regex.
# Optional: exclude (suppress match), enabled (false to disable).

rules:
  # Example: flag TODO comments as informational
  # - id: flag-todo
  #   severity: MEDIUM
  #   pattern: todo-comment
  #   message: "TODO comment found"
  #   regex: '(?i)\bTODO\b'

  # Example: disable a built-in rule by id
  # - id: insecure-http-0
  #   enabled: false

  # Example: disable the dangling-link structural check
  # - id: dangling-link
  #   enabled: false

  # Example: override a built-in rule (match by id, change severity)
  # - id: destructive-commands-2
  #   severity: MEDIUM
  #   pattern: destructive-commands
  #   message: "Sudo usage (downgraded)"
  #   regex: '(?i)\bsudo\s+'
```

ファイルが既に存在する場合、`init` はエラーで終了します — 既存のルールを上書きすることはありません。

## ワークフロー: 誤検知の修正

ルールをカスタマイズする一般的な理由は、正当な Skill が built-in ルールに引っかかる場合です。ステップバイステップの例を示します。

**1. audit を実行して誤検知を確認する:**

```bash
$ skillshare audit ci-helper
Audit  ~/.config/skillshare/skills
  global · blocks at CRITICAL · policy DEFAULT / dedupe:GLOBAL / analyzers:ALL

HIGH      Sudo escalation  SKILL.md:42
  destructive-commands-2 / static
  "sudo apt-get install -y jq"

  Risk       HIGH 15/100
  Auditable  100%

! ci-helper has findings below CRITICAL · 0.2s
```

**2. [built-in ルールのテーブル](#built-in-rule-ids)からルール ID を特定する:**

`sudo` を含む `destructive-commands` パターンは、ルール `destructive-commands-2` に一致します。

**3. カスタムルールファイルを作成する（まだ作成していない場合）:**

```bash
skillshare audit rules init
```

**4. ルールのオーバーライドを追加して抑制または引き下げる:**

```yaml
# ~/.config/skillshare/audit-rules.yaml
rules:
  # CI 自動化用の Skill 向けに sudo を MEDIUM に引き下げる
  - id: destructive-commands-2
    severity: MEDIUM
    pattern: destructive-commands
    message: "Sudo usage (downgraded for CI automation)"
    regex: '(?i)\bsudo\s+'
```

または完全に無効化する:

```yaml
rules:
  - id: destructive-commands-2
    enabled: false
```

**5. audit を再実行して確認する:**

```bash
$ skillshare audit ci-helper
Audit  ~/.config/skillshare/skills
  global · blocks at CRITICAL · policy DEFAULT / dedupe:GLOBAL / analyzers:ALL

✓ No issues found in ci-helper · 0.1s   # 合格になる（または HIGH の代わりに MEDIUM が表示される）
```

### 変更を検証する

ルールを編集した後、audit を再実行して確認してください。

```bash
skillshare audit                     # すべての Skill をチェック
skillshare audit <name>              # 特定の Skill をチェック
skillshare audit --json | jq '.skills[].findings'  # findings をプログラムから確認
```

サマリーの解釈:

- `Failed` は、有効な閾値以上の findings を持つ Skill を数える。
- `Warning` は、閾値未満だが clean より上の findings を持つ Skill を数える（例えば閾値が `CRITICAL` のときの `HIGH/MEDIUM/LOW/INFO`）。

## Built-in ルール ID {#built-in-rule-ids}

`id` の値を使って、特定の built-in ルールをオーバーライドまたは無効化します。

正規表現ベースのルールの信頼できる情報源:
[`internal/audit/rules.yaml`](https://github.com/runkids/skillshare/blob/main/internal/audit/rules.yaml)

:::note 構造チェック、tier チェック、Skill 間チェック

`dangling-link`、`content-tampered`、`content-oversize`、`content-missing`、`content-unexpected` は**構造チェック**です（正規表現ではなく、ファイルシステムの検索とハッシュ比較）。`low-analyzability` は [Analyzability Score](/docs/understand/audit-engine#analyzability-score) から生成される**分析可能性の finding** です。`tier-stealth`、`tier-destructive-network`、`tier-network-heavy`、`tier-interpreter`、`tier-interpreter-network` は、[Command Safety Tiering](/docs/understand/audit-engine#command-safety-tiering) プロファイルから生成される**ティアの組み合わせの finding** です。`cross-skill-*` の findings は、[Cross-Skill Interaction Detection](/docs/understand/audit-engine#cross-skill-interaction-detection) から生成されます。これらはすべて下記の表に記載されていますが、`rules.yaml` には定義されていません。

:::

| ID | パターン | 重大度 |
|----|---------|----------|
| `prompt-injection-0` | prompt-injection | CRITICAL |
| `prompt-injection-1` | prompt-injection | CRITICAL |
| `prompt-injection-2` | prompt-injection | HIGH |
| `prompt-injection-3` | prompt-injection | CRITICAL |
| `prompt-injection-4` | prompt-injection | CRITICAL |
| `hidden-unicode-1` | invisible-payload | CRITICAL |
| `data-exfiltration-0` | data-exfiltration | CRITICAL |
| `data-exfiltration-1` | data-exfiltration | CRITICAL |
| `data-exfiltration-2` | data-exfiltration | MEDIUM |
| `data-exfiltration-3` | data-exfiltration | HIGH |
| `credential-access-ssh-private-key` | credential-access | CRITICAL |
| `credential-access-env-file` | credential-access | CRITICAL |
| `credential-access-aws-credentials` | credential-access | CRITICAL |
| `credential-access-etc-shadow` | credential-access | CRITICAL |
| `credential-access-git-credentials` | credential-access | CRITICAL |
| `credential-access-netrc` | credential-access | CRITICAL |
| `credential-access-gnupg` | credential-access | CRITICAL |
| `credential-access-kube-config` | credential-access | CRITICAL |
| `credential-access-vault-token` | credential-access | CRITICAL |
| `credential-access-terraform-creds` | credential-access | CRITICAL |
| `credential-access-gnome-keyring` | credential-access | CRITICAL |
| `credential-access-npmrc` | credential-access | CRITICAL |
| `credential-access-pypirc` | credential-access | CRITICAL |
| `credential-access-gem-credentials` | credential-access | CRITICAL |
| `credential-access-ssl-private` | credential-access | CRITICAL |
| `credential-access-ssh-host-key` | credential-access | CRITICAL |
| `credential-access-pgpass` | credential-access | CRITICAL |
| `credential-access-mysql-cnf` | credential-access | CRITICAL |
| `credential-access-etc-passwd` | credential-access | MEDIUM |
| `credential-access-azure-creds` | credential-access | HIGH |
| `credential-access-gcloud-creds` | credential-access | HIGH |
| `credential-access-docker-config` | credential-access | HIGH |
| `credential-access-gh-cli-token` | credential-access | HIGH |
| `credential-access-password-store` | credential-access | HIGH |
| `credential-access-macos-keychain-user` | credential-access | HIGH |
| `credential-access-macos-keychain-sys` | credential-access | HIGH |
| `credential-access-terraformrc` | credential-access | HIGH |
| `credential-access-cargo-credentials` | credential-access | HIGH |
| `credential-access-op-cli` | credential-access | HIGH |
| `credential-access-age-keys` | credential-access | HIGH |
| `credential-access-shell-history` | credential-access | LOW |
| `credential-access-openvpn` | credential-access | LOW |
| `credential-access-auth-log` | credential-access | INFO |
| `credential-access-unknown-dotdir` | credential-access | INFO |

> **注:** 上記の各 credential エントリは、アクセス方法ごとに派生 ID も生成します: `-copy`、`-redirect`、`-dd`、`-exfil`（例: `credential-access-ssh-private-key-copy`）。特定の派生バリアントを無効化するには、`audit-rules.yaml` でその完全な ID を使用してください。

| ID | パターン | 重大度 |
|----|---------|----------|
| `hidden-unicode-0` | hidden-unicode | HIGH |
| `hidden-unicode-2` | hidden-unicode | HIGH |
| `config-manipulation-0` | config-manipulation | HIGH |
| `hidden-comment-injection-1` | hidden-comment-injection | HIGH |
| `self-propagation-0` | self-propagation | HIGH |
| `destructive-commands-0` | destructive-commands | HIGH |
| `destructive-commands-1` | destructive-commands | HIGH |
| `destructive-commands-2` | destructive-commands | HIGH |
| `destructive-commands-3` | destructive-commands | HIGH |
| `destructive-commands-4` | destructive-commands | HIGH |
| `dynamic-code-exec-0` | dynamic-code-exec | HIGH |
| `dynamic-code-exec-1` | dynamic-code-exec | HIGH |
| `shell-execution-0` | shell-execution | HIGH |
| `hidden-comment-injection-0` | hidden-comment-injection | HIGH |
| `obfuscation-0` | obfuscation | HIGH |
| `fetch-with-pipe-0` | fetch-with-pipe | HIGH |
| `fetch-with-pipe-1` | fetch-with-pipe | HIGH |
| `fetch-with-pipe-2` | fetch-with-pipe | HIGH |
| `hardcoded-secret-0` | hardcoded-secret | HIGH |
| `hardcoded-secret-1` | hardcoded-secret | HIGH |
| `hardcoded-secret-2` | hardcoded-secret | HIGH |
| `hardcoded-secret-3` | hardcoded-secret | HIGH |
| `hardcoded-secret-4` | hardcoded-secret | HIGH |
| `hardcoded-secret-5` | hardcoded-secret | HIGH |
| `hardcoded-secret-6` | hardcoded-secret | HIGH |
| `hardcoded-secret-7` | hardcoded-secret | HIGH |
| `hardcoded-secret-8` | hardcoded-secret | HIGH |
| `hardcoded-secret-9` | hardcoded-secret | HIGH |
| `data-uri-0` | data-uri | MEDIUM |
| `escape-obfuscation-0` | escape-obfuscation | MEDIUM |
| `suspicious-fetch-0` | suspicious-fetch | MEDIUM |
| `ip-address-url-0` | ip-address-url | MEDIUM |
| `hidden-unicode-3` | hidden-unicode | MEDIUM |
| `untrusted-install-0` | untrusted-install | MEDIUM |
| `untrusted-install-1` | untrusted-install | MEDIUM |
| `insecure-http-0` | insecure-http | LOW |
| `external-link-0` | external-link | LOW |
| `dangling-link` | dangling-link | LOW |
| `content-tampered` | content-tampered | MEDIUM |
| `content-oversize` | content-oversize | MEDIUM |
| `content-missing` | content-missing | LOW |
| `content-unexpected` | content-unexpected | LOW |
| `shell-chain-0` | shell-chain | INFO |
| `low-analyzability` | low-analyzability | INFO |
| `tier-stealth` | tier-stealth | CRITICAL |
| `tier-destructive-network` | tier-destructive-network | HIGH |
| `tier-network-heavy` | tier-network-heavy | MEDIUM |
| `tier-interpreter` | tier-interpreter | INFO |
| `tier-interpreter-network` | tier-interpreter-network | MEDIUM |
| `cross-skill-exfiltration` | cross-skill-exfiltration | HIGH |
| `cross-skill-privilege-network` | cross-skill-privilege-network | MEDIUM |
| `cross-skill-stealth` | cross-skill-stealth | HIGH |
| `cross-skill-cred-interpreter` | cross-skill-cred-interpreter | MEDIUM |

## サブコマンド

| サブコマンド | 説明 |
|-----------|-------------|
| `rules` | audit ルールの閲覧、有効化、無効化 |
| `rules disable <id>` | ID で単一ルールを無効化 |
| `rules disable --pattern <p>` | パターンに一致するすべてのルールを無効化 |
| `rules enable <id>` | ID で単一ルールを再有効化 |
| `rules enable --pattern <p>` | パターンに一致するすべてのルールを再有効化 |
| `rules severity <id> <level>` | 単一ルールの重大度を上書き |
| `rules severity --pattern <p> <level>` | パターングループ内のすべてのルールの重大度を上書き |
| `rules reset` | すべてのカスタムルールを削除（built-in のデフォルトに戻す） |
| `rules init` | スターター用の `audit-rules.yaml` を作成（`audit --init-rules` と同じ） |

## 関連項目

- [`audit`](/docs/reference/commands/audit) — メインの audit コマンドリファレンス
- [Audit Engine](/docs/understand/audit-engine) — エンジンの仕組み（脅威モデル、リスクスコアリング、ティア分類）
- [Securing Your Skills](/docs/how-to/advanced/security) — チーム向けセキュリティガイド
- [CI/CD Skill Validation](/docs/how-to/recipes/ci-cd-skill-validation) — パイプライン自動化のレシピ
