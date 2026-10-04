---
sidebar_position: 4
---

# .skillfollow（実験的）

Skills source の第一階層にある symlink や Windows junction を明示的に宣言し、外部グループ/repo を論理パスで discovery します。作業 repo を移動せず使えますが、宣言はファイルの所有権や書き込み権を skillshare に与えません。

## 設定

ファイルは**設定された skills source のルート**に置きます。通常は `~/.config/skillshare/skills/`、Windows は `%AppData%\skillshare\skills\`、project mode は `.skillshare/skills/` です。カスタム `sources.skills` も使えます。Agents/extras には適用せず、入れ子の repo ルートでは読みません。

外部 repo に `.git` と `review/SKILL.md` があり、ルート自体には `SKILL.md` がない例：

```text
~/work/team-skills/
~/.config/skillshare/skills/
├── _team-skills -> ~/work/team-skills/
├── .skillfollow
└── .gitignore
```

第一階層のリンクを自分で作成します。macOS/Linux：

```bash
ln -s "$HOME/work/team-skills" "$HOME/.config/skillshare/skills/_team-skills"
```

Windows の Command Prompt では symlink 権限不要の directory junction を作れます（外部パスを置換）：

```text
mklink /J "%AppData%\skillshare\skills\_team-skills" "C:\work\team-skills"
```

外部パスではなく**エントリ名**を記入します：

```text title=".skillfollow"
# One direct child of the skills source per line
_team-skills
```

Skills source の `.gitignore` に、ルートに固定した**末尾 `/` なし**の行を追加します：

```text title=".gitignore"
/_team-skills
/.skillfollow.local
```

Git は symlink をディレクトリでなくファイルとして保存するため、`/_team-skills/` では不十分です。`.skillfollow` はコミットし、リンクと `.skillfollow.local` は追跡しません。index 登録済みならパスを確認して skills source で実行します。index の登録だけを外し、作業リンクは残します：

```bash
git rm --cached -- '_team-skills'
```

`skillshare doctor`、`skillshare list --no-tui`、`skillshare sync --dry-run` で確認し、正しければ `skillshare sync`。`-g`/`-p` で対象を選びます。宣言/ignore は手動編集です。Discovery、status、doctor、dry run は自動作成・修復しません。`follow`/`unfollow` コマンドはまだありません。

`_` 接頭辞と `.git` を持つディレクトリは tracked repo、それ以外はグループです。Skills は `_team-skills/review`（flat name `_team-skills__review`）などの論理パスを維持します。Source-root/repo の `.skillignore` は引き続き適用され（followed グループ内に入れ子の tracked repo も含む）、入れ子の tracked repo（`--track --into` で入れたものも）は自分の skills を所有します（`list` に repo 名、`status` と Dashboard の件数、`.metadata.json` の target override が適用、Dashboard は単一 skill の uninstall を拒否）。未宣言リンクは非表示のままです。

## 形式

`.skillfollow.local` は `.skillfollow` の隣に置き、マシン固有の名前を追加します。両者は和集合で、base の後に local、重複は統合します。`.skillignore.local` と違い否定や上書きはありません。

- 前後の空白を除去。空行と `#` で始まる行は無視します。コメントは別の行に置きます。
- 直接の子の名前のみ。`.`、`..`、絶対パス、`C:` 等の drive/volume、UNC、`/`、`\`、path cleaning で変わる名前は禁止です。
- Glob/否定は禁止：`*`、`?`、`[`、`]`、`{`、`}`、`!`、NUL は拒否。無効行は警告され、followed エントリになりません。
- 宣言された第一階層だけをたどり、そのツリー内の入れ子リンクは走査しません。
- 宣言されたエントリを skillshare が作成することはありません（リンクがオフラインでも同じ）。`install`（通常、`--into`、`--track`、引数なしの再インストール）、`new`、`collect`、`trash restore`、`init` の取り込み、symlink モードの sync 移行、Dashboard の create/install/collect/restore は宣言エントリ内の宛先を `<source>/<entry> is a link; edit its target directly`（Dashboard は 409）で拒否し、宣言エントリ内に記録された tracked repo は missing 扱いにも rehydrate 対象にもなりません。宣言ファイルが存在するのに読み取れない間は、これらの書き込みはすべて読み取りエラーで拒否されます（読めないファイルが宛先を宣言している可能性があるため）。

## 状態と復旧 {#states}

Canonical path で安全性を検査し、最初に該当する状態を採用します。エントリ間の重複は両者を拒否し、宣言順に左右されません。

| 状態 | 意味と対応 |
|---|---|
| `missing` | 不在、リンク切れ、読み取り不能、安全境界の解決失敗。走査中の読み取り失敗も該当。ドライブ/リンク/読み取り権限や境界を修復し、不要な宣言は削除 |
| `not-link` | 実ディレクトリで通常 discovery。所有権は変わらず修復不要 |
| `invalid-target` | 対象がディレクトリでない、またはエントリがリンク/ディレクトリでない。ディレクトリリンクに直すか宣言削除 |
| `cycle` | 対象が source、その内部、または祖先。独立した外部ディレクトリへ変更 |
| `target-overlap` | 有効な skills target と同一、内部、または包含関係。入力/出力を分離 |
| `inside-git-root` | 有効な Git staging tree 内の対象。外部ツリーを staging tree 外へ移動。リンクを ignore しても実ファイルは隠れない |
| `entry-overlap` | 宣言対象が同じか包含関係。宣言を削除/変更して重複を解消 |
| `single-skill` | 対象ルートに `SKILL.md`。まだ未対応。親グループ/repo を使うか宣言削除 |
| `followed` | 安全で読めるグループ/tracked repo。Discovery/sync 可能 |
| `undeclared-link` | 両ファイルにない第一階層リンク。非表示のままにするか、宣言と ignore を追加 |

`followed`/`not-link` は doctor の pass。それ以外の宣言状態は warning でクリーンアップを停止。`undeclared-link` は info のみで停止しません。Parser 警告は別途表示します。

`.skillfollow` または `.skillfollow.local` が存在するのに読み取れない場合、discovery は不完全な結果で続行せず停止します。`sync` は拒否して既存 target を保持し、`check` と `status` は空のカウントではなく読み取りエラーを報告し、`doctor` は `skillfollow` で報告しつつ、discovered skills を必要とする検査（`skills_validity`、`skill_integrity`、`skill_targets_field`、`sync_drift`）を空の source として判定せず skipped にし、すべての `update`（CLI、Dashboard、`install --update`）は `--force` でも拒否され、Dashboard の update-all は全体が失敗し、source の Git staging も拒否されます。ファイルの読み取り権限を復旧するか、ファイルを削除してください。同じ規則は followed エントリの内側にも適用されます。その下のグループを選ぶ `check` と `update`（`--group <name>` または位置引数のグループ名）は、グループ配下のディレクトリが読み取れないとき、読み取れる部分だけを処理せず `incomplete discovery of <entry>: <read error>` で拒否します。

## コマンドの表示 {#visibility}

- **Status**：`.skillfollow: N entries, M skipped`、local 有効時は `(.local active)`、各 prune 停止の復旧メッセージ。JSON は `source.skillfollow` に `active`、`local_active`、`entry_count`、`followed_count`、`skipped_count`、宣言 `entries`（`name`、`state`、任意の `resolved_target`、`reason`）、任意の `warnings`/`prune_paused`。宣言も宣言警告もない場合は省略。
- **Doctor**：`skillfollow` は各宣言状態、`skillfollow_prune` は停止理由。未宣言リンクは `undeclared_source_links` info。Git repo 内では indexed/`not-ignored` リンクと安全でない local ファイルも検査しますが変更しません。
- **`list --no-tui`**：followed tracked repo に `→ <resolved>` を追加（ホームは `~` に短縮可能）。Skills は論理パス、JSON 形式は不変。
- **Diff**：sync と同じ規則でプレビューします。宣言が利用できない間は削除を報告せず、`<target>: prune paused; unavailable .skillfollow entry: <name> (<state>)` を表示し、sync が残す standard naming の managed copy を **Kept** として表示します。`diff --json` は target ごとに `prune_paused` と `keep` 項目を追加します。Dashboard diff は `prune_paused` を追加し、残す copy を `skip` で表示します。followed の orphan link も sync の prune と同じ判断で表示します。followed エントリの解決先を指す managed merge link は、その skill が discovery から外れると `prune` に、自分で作った同じ先への link は `local` になります。
- **Dashboard**：Skills、Overview、Check、Update、Audit、Hub で論理パスを表示（audit は resolved root 経由で followed skill を走査）。内容編集、uninstall、切替、target 上書き、source URL 変更は拒否。外部ツリーを直接編集し、非表示には **source-root `.skillignore`** を使います。宣言専用エディタはまだありません。Dashboard sync は CLI と同じ prune/copy 安全方針で、target ごとの `prune_paused`/`kept` と警告を表示。Targets は managed followed link を local でなく linked と数えます。

実際の診断文字列：

```text
_team-skills: not-ignored; add "/_team-skills" to <source>/.gitignore
_team-skills: indexed; run git rm --cached -- '_team-skills' and add "/_team-skills" to <source>/.gitignore
.skillfollow.local: tracked; run git rm --cached -- .skillfollow.local
.skillfollow.local: not-ignored; add "/.skillfollow.local" to <source>/.gitignore
```

## クリーンアップの安全性 {#cleanup}

**いずれかの宣言**が利用不能（`followed`/`not-link` 以外）なら merge/copy の全 skills target で prune を停止し、`sync --force` も解除せず、source を pull した直後の init 初回 sync でも同様です。新しいリンク/コピーは作成可能。Standard naming の既存 managed copy は出所が証明できないと置換せず、flat naming は続行可能。Merge link の置換は可能ですが、エントリ復帰時の名前衝突を警告します。

Status/doctor は各停止理由を表示します：

```text
prune paused: <name> is <state>; restore or fix <path>, or remove <name> from .skillfollow[.local], to resume cleanup
```

修復するか、名前がある**すべての宣言ファイル**から削除し、再 sync。不要な宣言を放置すると停止は無期限です。宣言削除は外部ツリーを削除しません。論理 source 経由の managed orphan link は prune できますが、follow 解除後の外部パスへ直接向く managed link は残し、`managed link resolves outside the source after unfollow; remove it or re-run with --force` と警告します。

## 更新の安全性 {#updates}

CLI、Dashboard（all/streaming 含む）、`install --update` は同じ followed tracked repo 方針です：clean tree と **fast-forward-only** pull（`--ff-only --no-rebase`）。明示 `--force` は dry run でも拒否。Dirty、status-check error、fast-forward 失敗（履歴分岐含む）は項目ごとに失敗し、`resolve in` と解決先の実パスを表示。他の batch 項目は続行します。外部 repo で解決し、force 再試行はしないでください。通常 installed repo は従来どおりです。Agent repo はこの方針の対象外です。`.skillfollow` は skills source のファイルなので、Dashboard からの repo ベース agent の update は skills の宣言が読めない場合でも参照しません。

followed entry 配下の通常 skill は再インストールされません。`update` はすべての選択方法（`--all`、名前、glob、group、project mode、dry run）と Dashboard の単一更新・update-all で、各項目を `followed repository update refused: skill <path> is inside followed entry <name>` として失敗にし、他の項目は続行します。更新されるのは上記方針に従う followed repository 自体だけです。

**Audit 失敗時は pull 前の commit に hard-reset します。** Resolved root をスキャンして論理パスで報告し、scan error も更新を止めます。更新中は編集、リンク先変更、別の Git 操作をしないでください。検査は snapshot で lock ではなく、rollback が同時変更を失う可能性があります。Skillshare 外の pull/編集は自動 audit されないため `skillshare audit` を実行してください。

## Source Git の安全性 {#git-safety}

`commit`、`push`、dry run、Dashboard staging、init source commit は Git が到達できる宣言リンクが indexed/未 ignore なら拒否します。Doctor の正確な末尾 `/` なし ignore と `git rm --cached` 指示に従います。自動で追跡解除しません。物理的 Git 到達性を使うため、skills の宣言だけで無関係な agents/extras repo は止めません。

Source **pull/reset/checkout** は indexed 宣言（不在だが indexed のリンクも含む）、または作業ツリーのリンク要素を持つ incoming path を拒否します。**宣言の有無は問いません**。宣言エントリ内の path は、そのリンクが不在でも拒否します（エラーはエントリ名を示します。remote から path を外すか、宣言からエントリを外してください）。Ignore だけでは Git の置換を防げません。エラーの commit/path に従い、indexed なら追跡解除と ignore、または remote の修正が必要です。Pull は fetch 後に固定 revision を検査。Dashboard checkout は既存 local/remote-tracking revision を検査し、暗黙の fetch は追加しません。Dashboard discard は ignored followed link を残します。

これらは skillshare 操作の guard で、自分で実行する Git は保護しません。

## 制限

単一 skill、`follow`/`unfollow`、宣言エディタは将来の対応です。入れ子リンクはたどりません。Developer Mode オフの Windows 11 ARM64 で、global mode の discovery、status、sync、prune の一時停止と再開、update の拒否、unfollow、`.skillfollow.local`、`invalid-target` を、追跡対象の junction（管理者と basic-user の token）と directory symlink（管理者 token）で検証済みです。project mode の相対リンク、Developer Mode の相対 symlink、リンクされた source root や target の親ディレクトリ、dashboard は Windows で**未検証**です。

## 関連項目

- [フィルタリング](./filtering.md#skillignore)
- [Source と Targets](../understand/source-and-targets.md)
- [Update](./commands/update.md)
- [Sync](./commands/sync.md)
