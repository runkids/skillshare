---
sidebar_position: 3
---

# Sync Modes

skillshare が source と target をどのようにリンクするか。

:::tip これが重要になる場面
Skill ごとの symlink を使い、target 内のローカル Skill を保持したい場合は merge mode(デフォルト)を選びます。symlink ではなく実ファイルが必要な場合(可搬性、CI、個人の好みなど)は copy mode を選びます。ディレクトリ全体をリンクしたく、target 側のローカル Skill が不要な場合は symlink mode を選びます。
:::

## 概要

| モード | 動作 | ユースケース |
|------|----------|----------|
| `merge` | 各 Skill が個別に symlink される | **デフォルト。** ローカル Skill を保持する。 |
| `copy` | 各 Skill が実ファイルとしてコピーされる | 可搬性、CI/サンドボックス環境、または symlink より実ファイルを好む場合。 |
| `symlink` | ディレクトリ全体が 1 つの symlink になる | どこでも完全に同一のコピーにする。 |

## 判断マトリクス(中立)

target のブランド名ではなく、自分の制約条件をもとに選ぶための表です。

| 判断軸 | `merge` | `copy` | `symlink` |
|---|---|---|---|
| 異なる AI CLI 間の互換性 | 中 | 高 | 低〜中 |
| 1 回の編集の即時反映 | 高 | 低(`sync` が必要) | 高 |
| ディスク使用量 | 低 | 高 | 低 |
| target 側での誤削除への安全性 | 高 | 高 | 低 |
| 運用のシンプルさ | 中 | 中 | 高 |
| target ごとのフィルタリング(`include`/`exclude`) | あり | あり | なし |

迷ったら `merge` から始め、必要に応じて特定の target だけ `copy` に切り替えてください。

---

## Merge Mode(デフォルト)

各 Skill が個別に symlink されます。target 内のローカル Skill は保持されます。

```
Source                          Target (claude)
─────────────────────────────────────────────────────────────
skills/                         ~/.claude/skills/
├── my-skill/        ────────►  ├── my-skill/ → (symlink)
├── another/         ────────►  ├── another/  → (symlink)
└── ...                         ├── local-only/  (preserved)
                                └── .skillshare-manifest.json
```

**利点:**
- target 固有の Skill(同期されないもの)を残せる
- インストールされた Skill とローカル Skill を混在できる
- 細かい制御ができる
- target ごとの include/exclude フィルタリング
- マニフェストベースの孤立ファイル削除(uninstall 後、symlink 以外の残骸を安全に削除)

:::info Project mode における相対 symlink
project mode(`-p`)では、symlink は絶対パスではなく **相対パス**(例: `../../.skillshare/skills/my-skill`)として作成されます。これにより、ディレクトリを移動・リネームしても symlink が機能し続けるため、プロジェクトの可搬性が高まります。global mode では、source と target が異なる場所にあるため絶対パスが使われます。
:::

**使うべき場面:**
- 一部の Skill を特定の AI CLI にだけ入れたい場合
- 同期する前にローカル Skill を試したい場合
- 1 つの source から、target ごとに異なる Skill の組み合わせにしたい場合

### Merge Mode でのフィルタ戦略

`include` と `exclude` は target ごとに次の順序で評価されます。

1. `include` がマッチする名前を残す
2. `exclude` がその残った集合から取り除く

簡単な選び方:
- target に少数のサブセットだけを渡したいときは `include` を使う
- target にほぼすべてを渡したいときは `exclude` を使う
- 広いサブセットに明示的な除外を加えたいときは `include + exclude` を使う

ルールを変更したときの挙動:
- これまで同期されていた source 由来のエントリがフィルタで除外されると、次回の `sync` で削除される
- target 内の既存のローカルな非 symlink フォルダは保持される

完全な例については [Target Configuration](/docs/reference/targets/configuration#include--exclude-target-filters) を参照してください。

---

## Copy Mode

各 Skill は実ファイルとして target ディレクトリにコピーされます。`.skillshare-manifest.json` ファイルが管理対象の Skill とそのチェックサムを追跡するため、ローカル Skill は保持されます。

```
Source                          Target (cursor)
─────────────────────────────────────────────────────────────
skills/                         ~/.cursor/skills/
├── my-skill/        ────copy►  ├── my-skill/    (real files)
├── another/         ────copy►  ├── another/     (real files)
└── ...                         ├── local-only/  (preserved)
                                └── .skillshare-manifest.json
```

### なぜ Copy Mode なのか

使用している AI CLI が symlink を正しく扱える場合でも、copy mode には価値があります。

- **防御的な設計** — すべての AI CLI が symlink サポートを保証しているわけではなく、特に Windows では symlink の挙動がプラットフォームや権限レベルによって異なる
- **サンドボックス環境** — 厳格な CI パイプライン、コンテナ、エアギャップ環境ではファイルシステム境界をまたぐ symlink をたどれないことがある
- **ユーザーの好み** — 一部のユーザーやチームは、透明性や可搬性のために symlink よりも実ファイルを単純に好む

**利点:**
- どこでも動作する — AI CLI や OS 側に symlink サポートは不要
- ローカル Skill を保持する(merge mode と同様)
- target ごとの include/exclude フィルタリング
- チェックサムによるスキップ: 変更のない Skill は再コピーされない

**使うべき場面:**
- 使用している AI CLI が「Skill が見つからない」と報告する、または symlink された Skill を読み込めない場合
- プロジェクトリポジトリに Skill をベンダリングしたい場合 — project mode の copy mode を使うと、チームは実際の Skill ファイルを git にコミットできるため、チームメイトは skillshare をインストールする必要がなくなる
- 中央の source なしで動作する自己完結型の Skill ディレクトリが必要な場合(可搬な構成、CI パイプライン、エアギャップ環境)
- merge mode と同じフィルタリングの挙動を実ファイルで実現したい場合
- `copy` の最初の候補としてよく挙がるもの: `cursor`、`antigravity`、`copilot`、`opencode`

### 更新の仕組み

`skillshare sync` を実行するたびに、各 source Skill のチェックサムがマニフェストに保存された値と比較されます。

- **チェックサムが同じ** → Skill はスキップされる(高速)
- **チェックサムが異なる** → Skill は新しいバージョンで上書きされる
- **`--force`** → チェックサムに関係なく、管理対象のすべての Skill が上書きされる

### マニフェストのライフサイクル

merge mode と copy mode の両方が、管理対象の Skill を追跡するために `.skillshare-manifest.json` を書き込みます。

- **Merge mode**: Skill 名を値 `"symlink"` として記録 — uninstall 後に孤立した実ディレクトリ(例: copy mode の残骸)を安全に削除するために使われる
- **Copy mode**: Skill 名を SHA-256 チェックサムとともに記録 — 差分同期と孤立検出に使われる
- symlink mode に切り替えると自動的に削除される
- 手動で削除された場合、次の `sync` で再構築される

---

## Symlink Mode

target ディレクトリ全体が source への単一の symlink になります。

```
Source                          Target (claude)
─────────────────────────────────────────────────────────────
skills/              ────────►  ~/.claude/skills → (symlink to source)
├── my-skill/
├── another/
└── ...
```

**利点:**
- すべての target が完全に同一になる
- 管理がシンプル
- 孤立した symlink が発生しない

**使うべき場面:**
- すべての AI CLI にまったく同じ Skill を持たせたい場合
- target 固有の Skill が不要な場合

**警告:** symlink mode では、target 経由で削除すると source も削除されます!
```bash
rm -rf ~/.claude/skills/my-skill  # ❌ SOURCE から削除されてしまう
skillshare target remove claude   # ✅ 安全にリンクを解除する方法
```

---

## モードの変更

### target ごと

```bash
# copy mode に切り替える(symlink を読めない AI CLI 向け)
skillshare target cursor --mode copy
skillshare sync

# symlink mode に切り替える
skillshare target claude --mode symlink
skillshare sync

# merge mode に戻す
skillshare target claude --mode merge
skillshare sync
```

### target ごとの上書き(推奨)

すべての target に対して 1 つのグローバルモードを使う必要はありません。よくあるパターンは次の通りです。

```yaml
mode: merge
targets:
  claude:
    path: ~/.claude/skills
    # inherits merge
  cursor:
    path: ~/.cursor/skills
    mode: copy
  codex:
    path: ~/.codex/skills
    mode: symlink
```

ある target には互換性優先の挙動(`copy`)が必要で、他の target は即時反映(`merge`/`symlink`)を維持したい場合に、target ごとの上書きを使います。

### デフォルトモード

新しい target 向けに設定で指定します。

```yaml
# ~/.config/skillshare/config.yaml
mode: merge  # or symlink or copy

targets:
  claude:
    path: ~/.claude/skills
    # inherits default mode

  cursor:
    path: ~/.cursor/skills
    mode: copy  # real files for Cursor

  codex:
    path: ~/.codex/skills
    mode: symlink  # override default
```

---

## Target の命名規則 {#target-naming}

merge mode または copy mode を使うとき、target 内で Skill ディレクトリがどう命名されるかを制御します。

| 命名方式 | 動作 |
|--------|------|
| `flat`(デフォルト) | ネストした Skill は `__` 区切りでフラット化される: `frontend/dev` → `frontend__dev` |
| `standard` | SKILL.md の `name` フィールドを使う: `frontend/dev` → `dev` |
| `prefixed` | copy mode 専用。`standard` に tracked repo 名を前置する: `_mattpocock-skills/skills/prototype` → `mattpocock-skills-prototype` |

グローバルまたは target ごとに設定します。

```yaml
target_naming: standard    # global default
targets:
  claude:
    skills:
      target_naming: flat  # per-target override
```

または CLI 経由で設定します。

```bash
skillshare target claude --target-naming standard
skillshare sync
```

**Standard mode** は [Agent Skills specification](https://agentskills.io/specification) に従い、SKILL.md の `name` フィールドが親ディレクトリ名と一致することを要求します。有効な名前は最大 64 文字の小文字（どの文字体系でも可）、数字、単独のハイフンで構成され、`-` で始まったり終わったりしません。アンダースコアは使えません。名前が無効な Skill や名前の衝突は警告され、スキップされます。

**Prefixed mode** は、同じ名前の Skill を提供する tracked repo のためのものです。`standard` では、別々の repo にある 2 つの `prototype` Skill が衝突して両方ともスキップされますが、`prefixed` では tracked repo 内の各 Skill が `<repo>-<name>` になるため、両方がツールに届きます。`<repo>` は tracked repo フォルダー名から先頭の `_` を除き、小文字にし、文字と数字（任意の文字体系）以外をすべて `-` に置き換えたものです。prefixed 名はフォルダー名として使われ、コピーされた `SKILL.md` の `name:` にも書き込まれます。source は変更されません。

- 名前がすでに repo 名で始まっている場合は、再度プレフィックスを付けません: `_bmad/skills/bmad-ux` は `bmad-ux` のままです。
- tracked repo の外にある Skill は名前をそのまま保ちます。
- source の Skill は、まず `standard` のチェックを通過する必要があります。prefixed 名が 64 文字を超える場合は警告付きでスキップされ、それでも名前が衝突する場合は `standard` と同様にスキップされます。tracked Skill は `SKILL.md` で名前を変更できないため、sync はもう一方の Skill の名前を変えるか、`--name` で repo を再度 track することを提案します。
- より短いプレフィックスにしたい場合は、repo を短い名前で track します: `skillshare install <repo> --track --name mp` は `mp-prototype` になります。
- 兄弟 Skill への相対リンク（`../other-skill/`）は書き換えられません。これは copy mode の `flat` と同じです。
- この名前がツールに表示される名前になります。Claude Code ではスラッシュコマンドになります。例: `/mattpocock-skills-prototype`。

`prefixed` は copy mode が必要です。merge のリンクは source を指しており、そこでは `name:` を変更できないためです。merge mode または symlink mode で `prefixed` に解決される Target は、バリデーションに失敗し、sync でスキップされます。新しい Target が copy 以外の mode で `prefixed` を継承する場合（プロジェクトの Target は既定で merge）、`target add` はその Target を copy mode にします。`target add` はそうしたときにその旨を表示します。また `status` と `doctor` は、copy 以外の mode で `prefixed` に解決される Target を sync の前に警告します。`targets:` にある Target については `target list` も同様です。対処は、その Target に `mode: copy` を設定することです。`projects:` エントリから展開された Target の場合は `projects.<root>.skills.mode: copy` を設定します。

```yaml
targets:
  universal:
    skills:
      mode: copy
      target_naming: prefixed
```

**移行**: `flat`、`standard`、`prefixed` の間で切り替えると、既存の管理対象エントリはその場でリネームされます。copy mode では manifest が各コピーを作成した naming を記録しているため、source が変更されていなくても、リネームされたコピーは再度コピーされ、その `name:` は新しい naming に一致します。ローカル Skill が既に新しい名前を占有している場合、古い管理対象エントリは保持されます。

**Symlink mode**: `flat` と `standard` は無視されます — ディレクトリ全体がそのままリンクされます。`prefixed` は上記のとおり検証エラーになります。

---

## モードの比較

| 観点 | Merge | Copy | Symlink |
|--------|-------|------|---------|
| ローカル Skill の保持 | ✅ あり | ✅ あり | ❌ なし |
| symlink 互換性 | ✅ あり | ❌ 実ファイル | ✅ あり |
| すべての target が同一 | ❌ 異なる場合がある | ❌ 異なる場合がある | ✅ 同一 |
| target ごとの include/exclude | ✅ あり | ✅ あり | ❌ 無視される |
| 孤立ファイル削除の必要性 | ✅ あり | ✅ あり | ❌ なし |
| 削除の安全性 | ✅ 安全 | ✅ 安全 | ⚠️ 注意が必要 |
| ディスク使用量 | 低(symlink) | 高め(コピー) | 低(symlink) |

---

## 孤立ファイルの削除

merge mode と copy mode の両方で、`sync` は自動的に孤立ファイルを削除します。

- 削除された source Skill を指す **symlink** は常に削除される
- `.skillshare-manifest.json` に記録されている(以前 skillshare が管理していた)**実ディレクトリ** は削除される
- マニフェストにない **未知のディレクトリ** はユーザーが作成したものとみなされ、警告とともに保持される

つまり `uninstall` + `sync` の後は、symlink 以外の残骸(例: 以前の `copy` mode から残ったディレクトリ)も安全にクリーンアップされます。

```
$ skillshare sync
✓ claude    5 linked · 2 local · 1 pruned
✓ cursor    3 copied · 2 up to date · 1 pruned
```

:::info エージェントも同じモードに従う
merge、copy、symlink の 3 つのモードすべてが、エージェントの同期にも適用されます。エージェントの孤立ファイル削除、target ごとの include/exclude フィルタリング、モード変換の挙動は Skill とまったく同じです。唯一の違いは、エージェントがディレクトリではなく単一の `.md` ファイルであることです。エージェント対応の target(Claude、Cursor、Augment、OpenCode)は、`agents:` サブキーに対しても同じ `mode` 設定に従います。詳細は [Agents](./agents.md) を参照してください。
:::

---

## Extras の Sync Modes

Extras(rules、commands、prompts のような Skill 以外のリソース)も merge mode と copy mode を使用します。各 extras の target は独自のモードを指定できます。

```yaml
extras:
  - name: rules
    targets:
      - path: ~/.claude/rules          # merge (default): per-file symlinks
      - path: ~/.cursor/rules
        mode: copy                     # copy: real file copies
```

挙動は Skill の sync modes と同じです。merge はファイルごとの symlink を作成し、copy は実ファイルのコピーを作成します。

:::note Developer Mode がオフの Windows
merge mode は単一のファイルをリンクしますが、Windows では Developer Mode がオンの場合にしか許可されません。Developer Mode がない場合、merge mode の agents と extras は代わりにコピーされ、それらのコピーはリンクと同じように更新・削除されます。Skills はフォルダーなので、どちらの場合も（ジャンクションで）リンクされます。[Windows のトラブルシューティング](/docs/troubleshooting/windows#file-links-need-windows-developer-mode-copying-instead) を参照してください。

skillshare が所有していない、内容が同じローカルファイルは保持されます。copy fallback では agent の件数に `local preserved` として別に表示されます（例：`0/1 linked, 1 local preserved`）。
:::

---

## 関連ページ

- [sync](/docs/reference/commands/sync) — sync を実行してモードの変更を適用する
- [target](/docs/reference/commands/target) — target の sync mode を変更する
- [Source & Targets](./source-and-targets.md) — 中核となるアーキテクチャ
- [Configuration](/docs/reference/targets/configuration) — target ごとの設定リファレンス
