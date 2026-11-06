---
sidebar_position: 21
---

# project

global config が Skill と Agent を同期する project フォルダーを、各フォルダーに `.skillshare/` を作らずに管理します。project は、このマシンだけに保存する場合を除き `config.yaml` に保存されます。

`project` は project 内で実行した場合でも、常に global config を使います。これはリポジトリ自身の設定を選ぶ `-p` / `--project` とは異なり、このコマンドでは `-p` / `--project` は拒否されます。チームメイトにもリポジトリと一緒にセットアップを渡したい場合は、[project mode](/docs/understand/project-skills) を使ってください。

## 使い方 {#usage}

```bash
skillshare project                          # Same as project list
skillshare project list --json
skillshare project show <project> --json
skillshare project add <root> --target T [options]
skillshare project edit <project> [options]
skillshare project sync <project> [--dry-run] [--json]
skillshare project localize <project> [--dry-run] [--json]
skillshare project share <project> [--yes] [--dry-run] [--json]
skillshare project remove <project> [--shared] [--yes] [--dry-run] [--json]
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
| `--machine-local` | add | `config.yaml` ではなく、このマシンだけに保存します |
| `--shared` | add, remove | add: [`projects_storage`](#local-registrations) が `machine-local` でも `config.yaml` に保存します。remove: このマシンが上書きしている project の `config.yaml` 側のコピーも削除します |
| `--sync` | add, edit | 保存してから、この project の Skill と Agent を sync します。force はオフです |
| `--dry-run`, `-n` | add, edit, sync, remove, localize, share | 書き込みや確認なしでプレビューします |
| `--yes` | remove, share | 確認をスキップします。プレビュー以外で JSON 出力と一緒に削除するときは必須です。share は、`config.yaml` に同じ project の異なる設定があるときだけ確認します |
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

project は `config.yaml` の [`projects`](/docs/reference/targets/configuration#projects) に保存されるため、その設定ファイルを使うすべてのマシンに反映されます。このマシンにしかないフォルダーには `--machine-local` を付けてください。詳しくは [project の保存先](#local-registrations)を参照してください。共有されたツールフォルダーは 1 つの sync Target にまとめられます。通常の `skillshare sync` もすべての project を sync します。

Skill を無効化した project では、global の symlink モードを含め、Skill の sync 処理は一切実行されません。要求された、または有効なリソースの選択が空の場合は、検出、バックアップ、Target への書き込みを行わずに成功します。

### フィルタの置き換えと上書き設定のリセット {#edit-settings}

```bash
skillshare project edit shop-web --skills-include 'frontend-*' --skills-include 'core-*'
skillshare project edit shop-web --skills-exclude= --skills-mode= --name=
skillshare project sync shop-web --dry-run --json
skillshare project sync shop-web --json
```

add と edit は、書き込む前に有効な設定全体を検証します。編集は project が保存されている場所（`config.yaml`、またはこのマシンだけの project ならこのマシン）に保存されます。`config.yaml` では、YAML アンカーで宣言された project を編集すると、そのアンカーのエイリアスも変更されます。変更の結果には影響を受けたルートが列挙されます。何も変わらない add や edit は `applied: false` と `unchanged: true` を返し、それでも `--sync` は実行されます。異なる設定で add を繰り返すと失敗し、`project edit` を案内します。カスタム名はルートに見える形（例: `~name`、`./name`、`C:name`）にはできません。

### project の削除 {#remove}

```bash
skillshare project remove shop-web --dry-run --json
skillshare project remove shop-web --yes --json
```

削除は project が保存されている場所から行われます。`config.yaml` から project を削除すると、その設定ファイルを使うすべてのマシンから削除され、確認メッセージにもその旨が表示されます。このマシンが `config.yaml` の project を上書きしている場合、削除はこのマシンのコピーだけを消します。`config.yaml` 側のバージョンが再び適用され、JSON の結果は `remaining` に報告します。`--shared` を付けると両方を削除します。

削除では、project を外す前に Skill を切り離します。フォルダー全体のリンクは削除され、merge モードのリンクは Source の内容が存在する限りローカルコピーになり、copy モードのファイルは残してマニフェストだけを削除します。存続している有効な Target がまだ所有するパスは残されます。

CLI の edit は、外された Skill のパスも同じルールで切り離します。project の Skill が無効のままなら、名前変更や Agent 設定だけの編集を含む、どの CLI edit でも残っている Skill のパスを切り離します。ダッシュボードで以前に無効化した場合も同じで、コピーバックと存続している有効な Target によるパス所有のルールは引き続き適用されます。ダッシュボードでの保存は Skill を切り離しません。Agent のファイルはそのまま残ります。プレビューと結果には、削除・コピーバック・保持されたパスが列挙されます。

## JSON と失敗時の動作 {#json-and-failures}

| コマンド | 主なフィールド |
|---------|-------------|
| list | `projects` 配列（登録がない場合は空） |
| show | `project` オブジェクト |
| add, edit, remove, localize, share | `action`、`project`、`applied`、`affected`、`warnings`。必要に応じて `sync`、`detach`、または `remaining`（remove） |
| sync | `action`、`project`、`sync`。プレビューでは `applied: false` も含みます |

project オブジェクトには `root`（保存された表記）、`path`（展開済みのパス）、`name`、`targets`、`skills`、`agents`、`groups`、`missing`、`hasOwnConfig`、`origin`（`shared` または `local`）、`originPath`（所有ファイル）、`sharedCopy`（`config.yaml` にも宣言されているローカルの project）が含まれます。省略または無効化されたリソースは `null` です。有効なリソースはその設定を公開します。テキスト形式の `project list` と `project show` はオン/オフの状態を表示します。sync の結果には `results`、`failed`、`warnings`、および検出・フィルタの診断情報が含まれます。

Target が失敗すると、sync は完了した Target を報告しつつ終了コード 1 で終了します。sync のプレビューは競合を `failed` に報告して終了コード 0 で終了しますが、不正な入力は引き続き失敗します。add/edit の保存に成功して sync が失敗した場合、登録は保存されたままです。Target を修正してから `project sync` で再試行してください。見つからない project フォルダーも edit はできますが、sync はできません。

edit と削除はトランザクションではありません。後続の切り離しや保存が失敗すると、それ以前のファイルシステムの変更が残ることがあります。JSON の結果には、完了した `detach`、`registration`（`kept`、`updated`、`removed`）、`error` が報告されます。edit の保存に失敗した場合は `applied: false` となり、永続化された project を記述します。保存に成功した後に sync が失敗した場合は `applied: true` のままです。再試行の前にこれらのフィールドを確認してください。変更が行われる前の失敗はエラーペイロードを使います。すべての project コマンドは起動時の移行、クリーンアップ、バージョン確認を省略します。

このコマンドは Skill と Agent のみを管理します。MCP の project 設定と hooks はそれぞれ専用のコマンドを使います。Target の変換や、project 自身の設定の編集は行いません。

## project の保存先 {#local-registrations}

project は次の 2 か所のどちらかに保存されます。

- **`config.yaml`**（`origin: shared`、デフォルト）。この設定ファイルを使うすべてのマシンに project が反映されます。たとえば dotfiles や `skillshare push` を使う場合です。
- **このマシンだけ**（`origin: local`）。設定ディレクトリの外にあるレジストリで、1 台のマシンにしかないフォルダー向けです。

`project add` は、`--machine-local` を付けない限り `config.yaml` に保存します。マシン固有をデフォルトにするには、`config.yaml` に [`projects_storage: machine-local`](/docs/reference/targets/configuration#projects-storage) を設定します。その場合、`--shared` で 1 つの project だけを `config.yaml` に保存できます。ダッシュボードの **プロジェクトを追加** ダイアログも同じ選択肢を提供し、この設定を初期値にします。編集と削除は project が保存されている場所で行われます。

保存済みの project を 2 か所の間で移動します。

```bash
skillshare project localize shop-web --dry-run --json   # keep a copy on this machine only
skillshare project share shop-web --dry-run --json      # move it into config.yaml
```

`localize` は `config.yaml` の project を、`config.yaml` の変更、切り離し、同期なしでこのマシンにコピーします。他のマシンは `config.yaml` のバージョンを使い続け、このマシンはそのコピーを使います。`share` はマシン固有の project を `config.yaml` に書き込み、ローカルのコピーを削除します。`config.yaml` に同じ project が異なる設定ですでにある場合、`share` は置き換える前に確認します。`--yes` なら確認せずに置き換えます。どちらも、project がすでにその場所にあれば何もせず、同期もしません。

ローカルの project は、同じ正規化ルート（`~` を展開しパスを整理したもの。project のシンボリックリンクルートは別扱い）の `config.yaml` の項目全体を置き換えます。v0.29.0 で保存された削除は、そのマシン上の `config.yaml` の project を隠しており、project が再び追加されるまで隠れたままです。異なるルートの名前重複と同じ層の正規化ルート重複は拒否されます。project の MCP サーバーと hooks は、project 自体の保存先にかかわらず、常に `config.yaml` の `mcp.projects` と `hooks.projects` にあります。

保存先は `$XDG_STATE_HOME/skillshare/projects/<identity>.yaml`、未指定なら Unix では `~/.local/state/skillshare/projects`、Windows では `%LOCALAPPDATA%\skillshare\state\projects` です。状態ディレクトリはローカルに保ってください。識別子は解決済み設定ファイルの保存先に基づき、設定シンボリックリンクの別名は状態を共有します。保存先の移動は新しい登録領域になります。読み取りはファイルを作成せず、不正な内容やバージョンは拒否します。

ダッシュボードの **Convert** は、通常の Target を `config.yaml` の `projects` 宣言に 1 回の書き込みで置き換えます。すでに宣言済み、ローカルで上書き済み、またはこのマシンで隠されているルートは変換できません。生の設定エディターは、提出された YAML とこのマシンの project の統合結果を、書き込み前に検証します。

## project の同時書き込み {#concurrency}

CLI とダッシュボードはローカル状態のロックを共有し、最大 10 秒待ち、取得後に再読み込みします。切り離しと書き込み前に共有・ローカルのスナップショットを確認します。他の共有設定書き込みには適用されません。ダッシュボードの Convert は明示的に共有設定を変更し、既存、非表示、ローカル上書きのルートを拒否します。生の設定エディターは提出された内容とローカル項目の統合結果を保存前に検証します。 `share` のように両方の場所を変更する場合は、先に `config.yaml` を、次にローカルレジストリを書き込むため、中断されても project が失われることはありません。

## 関連 {#related}

- [多数の Project を 1 つの Config で](/docs/how-to/recipes/many-projects-one-config)
- [project の設定フィールド](/docs/reference/targets/configuration#projects)
- [MCP の project 設定](/docs/reference/commands/mcp#manage-several-projects-from-the-global-config)
