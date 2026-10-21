---
sidebar_position: 21
---

# project

各フォルダーに `.skillshare/` の設定を作らずに、global config で project フォルダーを管理し、そこへ Skill と Agent を sync します。

`project` は project 内で実行した場合でも、常に global config を使います。これはリポジトリ自身の設定を選ぶ `-p` / `--project` とは異なり、このコマンドでは `-p` / `--project` は拒否されます。チームメイトにもリポジトリと一緒にセットアップを渡したい場合は、[project mode](/docs/understand/project-skills) を使ってください。

## 使い方 {#usage}

```bash
skillshare project                          # Same as project list
skillshare project list --json
skillshare project show <project> --json
skillshare project add <root> --target T [options]
skillshare project edit <project> [options]
skillshare project sync <project> [--dry-run] [--json]
skillshare project remove <project> [--yes] [--dry-run] [--json]
```

`add` には既存のフォルダーと 1 つ以上の Target が必要です。その他のコマンドは、有効な project 名（カスタム名またはフォルダーのベース名）か、そのルートパスを受け付けます。ルートを明示的に指定するには、相対パスの先頭に `./` または `../` を付けてください。プレフィックスのないトークンが、ある project の名前と別の project のルートの両方に一致する場合、コマンドはどちらかを選ばず両方の候補を報告します。シンボリックリンクのエイリアスは別の登録として扱われます。

## フラグ {#flags}

| フラグ | 対象 | 説明 |
|------|------------|-------------|
| `--target T` | add, edit | Target ツール。繰り返すかカンマ区切りで指定します。add では必須で、edit ではリスト全体を置き換えます |
| `--name N` | add, edit | カスタム名。`--name=` でフォルダーのベース名に戻します |
| `--skills`, `--no-skills` | add, edit | Skill を有効化または無効化します。add ではデフォルトで有効です |
| `--agents`, `--no-agents` | add, edit | Agent を有効化または無効化します。add ではデフォルトで無効です |
| `--skills-mode MODE`, `--agents-mode MODE` | add, edit | `merge`、`symlink`、`copy` のいずれか。空の値で上書き設定を解除します |
| `--skills-naming NAMING` | add, edit | `flat`、`standard`、`prefixed` のいずれか。空の値で継承に戻します |
| `--skills-include P`, `--skills-exclude P` | add, edit | 繰り返して複数の glob パターンを指定します。対応するリストを置き換え、空の値で解除します |
| `--agents-include P`, `--agents-exclude P` | add, edit | Agent 用で、置き換えの扱いは同じです。フィルタには merge または copy モードが必要です |
| `--sync` | add, edit | 保存してから、この project の Skill と Agent を sync します。force はオフです |
| `--dry-run`, `-n` | add, edit, sync, remove | 書き込みや確認なしでプレビューします |
| `--yes` | remove | 確認をスキップします。プレビュー以外で JSON 出力と一緒に削除するときは必須です |
| `--json` | all | 構造化出力。プロセスの終了コードも確認してください |
| `--global`, `-g` | all | global スコープを明示します（このコマンドのデフォルトでもあります） |
| `--help`, `-h` | all | ヘルプを表示します |

リソースの設定を指定するとそのリソースが有効になり、既存の `enabled: false` も解除されます。設定を省略した場合、無効状態は保持されます。edit は指定した設定だけを変更し、既存のリソースを有効化してもその設定は保持されます。相反するトグルや、`--no-skills --skills-mode copy` のような無効化と設定の組み合わせは拒否されます。Agent の命名方式のフラグはありません。[`target`](/docs/reference/commands/target) と異なり、これらの include/exclude フラグは個々のパターンを追加・削除するのではなく、リストを置き換えます。

## 例 {#examples}

### 登録、プレビュー、sync {#register-and-sync}

```bash
skillshare project add ~/work/shop-web --target claude,codex \
  --skills-mode copy --skills-include 'frontend-*' --agents --dry-run
skillshare project add ~/work/shop-web --target claude,codex \
  --skills-mode copy --skills-include 'frontend-*' --agents --sync
skillshare project show shop-web --json
```

保存されたエントリは [`projects`](/docs/reference/targets/configuration#projects) に属します。共有されたツールのフォルダーは 1 つの sync Target にまとめられます。通常の `skillshare sync` でも、登録済みの project が sync されます。

Skill を無効化した project では、global の symlink モードを含め、Skill の sync 処理は一切実行されません。要求された、または有効なリソースの選択が空の場合は、検出、バックアップ、Target への書き込みを行わずに成功します。

### フィルタの置き換えと上書き設定のリセット {#edit-settings}

```bash
skillshare project edit shop-web --skills-include 'frontend-*' --skills-include 'core-*'
skillshare project edit shop-web --skills-exclude= --skills-mode= --name=
skillshare project sync shop-web --dry-run --json
skillshare project sync shop-web --json
```

add と edit は、書き込む前に有効な設定全体を検証します。YAML アンカーは他のエントリにも影響することがあるため、変更の結果には影響を受けたすべてのルートが列挙されます。何も変わらない add や edit は `applied: false` と `unchanged: true` を返し、それでも `--sync` は実行されます。異なる設定で add を繰り返すと失敗し、`project edit` を案内します。カスタム名はルートに見える形（例: `~name`、`./name`、`C:name`）にはできません。

### 登録の削除 {#remove}

```bash
skillshare project remove shop-web --dry-run --json
skillshare project remove shop-web --yes --json
```

削除では、登録を外す前に Skill を切り離します。フォルダー全体のリンクは削除され、merge モードのリンクは Source の内容が存在する限りローカルコピーになり、copy モードのファイルは残してマニフェストだけを削除します。存続している有効な Target がまだ所有するパスは残されます。

CLI の edit は、外された Skill のパスも同じルールで切り離します。project の Skill が無効のままなら、名前変更や Agent 設定だけの編集を含む、どの CLI edit でも残っている Skill のパスを切り離します。ダッシュボードで以前に無効化した場合も同じで、コピーバックと存続している有効な Target によるパス所有のルールは引き続き適用されます。ダッシュボードでの保存は Skill を切り離しません。Agent のファイルはそのまま残ります。プレビューと結果には、削除・コピーバック・保持されたパスが列挙されます。

## JSON と失敗時の動作 {#json-and-failures}

| コマンド | 主なフィールド |
|---------|-------------|
| list | `projects` 配列（登録がない場合は空） |
| show | `project` オブジェクト |
| add, edit, remove | `action`、`project`、`applied`、`affected`、`warnings`。必要に応じて `sync` または `detach` |
| sync | `action`、`project`、`sync`。プレビューでは `applied: false` も含みます |

project オブジェクトには `root`（保存された表記）、`path`（展開済みのパス）、`name`、`targets`、`skills`、`agents`、`groups`、`missing`、`hasOwnConfig` が含まれます。省略または無効化されたリソースは `null` です。有効なリソースはその設定を公開します。テキスト形式の `project list` と `project show` はオン/オフの状態を表示します。sync の結果には `results`、`failed`、`warnings`、および検出・フィルタの診断情報が含まれます。

Target が失敗すると、sync は完了した Target を報告しつつ終了コード 1 で終了します。sync のプレビューは競合を `failed` に報告して終了コード 0 で終了しますが、不正な入力は引き続き失敗します。add/edit の保存に成功して sync が失敗した場合、登録は保存されたままです。Target を修正してから `project sync` で再試行してください。見つからない project フォルダーも edit はできますが、sync はできません。

edit と削除はトランザクションではありません。後続の切り離しや保存が失敗すると、それ以前のファイルシステムの変更が残ることがあります。JSON の結果には、完了した `detach`、`registration`（`kept`、`updated`、`removed`）、`error` が報告されます。edit の保存に失敗した場合は `applied: false` となり、永続化された project を記述します。保存に成功した後に sync が失敗した場合は `applied: true` のままです。再試行の前にこれらのフィールドを確認してください。変更が行われる前の失敗はエラーペイロードを使います。list、show、プレビュー、JSON 出力は起動時のマイグレーション、クリーンアップ、バージョン確認を行いません。テキスト出力の add、edit、sync、remove は他のコマンドと同様にそれらを実行します。

このコマンドは Skill と Agent のみを管理します。MCP の project 設定と hooks はそれぞれ専用のコマンドを使います。Target の変換や、project 自身の設定の編集は行いません。

## project の同時書き込み {#concurrency}

CLI とダッシュボードによる project の変更は、skillshare の state ディレクトリにある、解決済みの global config の保存先ごとの `gofrs/flock` ロックを共有し、最大 10 秒待機して、取得後に再読み込みします。config のシンボリックリンクも同じロックを共有します。読み取りと dry run はロックを作成も取得もしません。`target`、MCP、hooks、通常の `Config.Save()` 呼び出しなど、その他の config の書き込みはこの保護の対象外です。それらの経路での同時書き込みは、引き続き変更を上書きする可能性があります。

## 関連 {#related}

- [多数の Project を 1 つの Config で](/docs/how-to/recipes/many-projects-one-config)
- [project の設定フィールド](/docs/reference/targets/configuration#projects)
- [MCP の project 設定](/docs/reference/commands/mcp#manage-several-projects-from-the-global-config)
