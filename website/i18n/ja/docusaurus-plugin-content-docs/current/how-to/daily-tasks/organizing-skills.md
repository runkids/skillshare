---
sidebar_position: 5.5
---

# フォルダで Skill を整理する

Skill のコレクションが増えるにつれて、フォルダに整理しておくと管理しやすくなります — 残りは skillshare が自動的に処理してくれます。

## なぜ整理するのか？

20個以上の Skill のフラットなリストはナビゲートしづらくなります。

```
~/.config/skillshare/skills/
├── accessibility/
├── ascii-box-check/
├── core-web-vitals/
├── frontend-design/
├── performance/
├── react-best-practices/
├── remotion/
├── seo/
├── skill-creator/
├── ui-skills/
├── vue-best-practices/
├── vue-debug-guides/
├── web-artifacts-builder/
└── ... 20+ more
```

フォルダを使うと論理的にグループ化でき、skillshare が AI CLI 向けに自動的にフラット化します。

```
SOURCE (organized)                     TARGET (auto-flattened)
───────────────────────────────────    ──────────────────────────────────
~/.config/skillshare/skills/           ~/.claude/skills/
├── frontend/                          ├── frontend__frontend-design
│   ├── frontend-design/               ├── frontend__react__react-best-..
│   ├── react/                         ├── frontend__ui-skills
│   │   └── react-best-practices/      ├── frontend__vue__vue-best-prac..
│   ├── ui-skills/                     ├── frontend__vue__vue-debug-gui..
│   └── vue/                           ├── utils__ascii-box-check
│       ├── vue-best-practices/        ├── utils__remotion
│       ├── vue-debug-guides/          ├── utils__skill-creator
│       └── ...                        ├── web-dev__accessibility
├── utils/                             ├── web-dev__core-web-vitals
│   ├── ascii-box-check/               └── ...
│   ├── remotion/
│   └── skill-creator/
└── web-dev/
    ├── accessibility/
    ├── core-web-vitals/
    └── ...
```

![Source vs Target comparison](/img/organizing-skills-comparison.png)

:::tip 実例
このパターンを使って整理された Skill コレクションの完全な例は [runkids/my-skills](https://github.com/runkids/my-skills) を参照してください。
:::

---

## 自動フラット化の仕組み

skillshare はフォルダのパスを `__`（アンダースコア2つ）を区切り文字にしてフラットな名前に変換します。

| Source のパス | Sync される Target 名 |
|---|---|
| `frontend/react/react-best-practices/` | `frontend__react__react-best-practices` |
| `utils/remotion/` | `utils__remotion` |
| `web-dev/accessibility/` | `web-dev__accessibility` |

**重要なポイント:**
- `SKILL.md` を含むディレクトリのみが Skill として扱われる
- 中間フォルダ（`frontend/` 自体など）は単なる整理用で、`SKILL.md` は不要
- `list` と `sync` はどんな深さのネストされた Skill も発見する
- `check` と `update` もネストされた Skill に対応する

:::note Agent はネストされない
このページは**Skill**の整理についてのものです。Agent は常に `~/.config/skillshare/agents/`（Project mode では `.skillshare/agents/`）直下に配置される単一の `.md` ファイルです — フォルダのネストや自動フラット化には対応していません。Agent を整理するには、命名規則（例: `frontend-reviewer.md`、`backend-auditor.md`）と `.agentignore` のパターンを使ってください。
:::

---

## ネストされた Skill の扱い

### list

同じディレクトリ内の Skill は自動的にグループ化されます。

```bash
$ skillshare list -g

  frontend/vue/
    → vue-best-practices     github.com/vuejs-ai/skills/...

  utils/
    → remotion               github.com/remotion-dev/skills/...

  web-dev/
    → accessibility          github.com/addyosmani/web-quality-...
```

各グループ内では、Skill はベース名（フラット化された完全な名前ではなく）で表示されます。トップレベルの Skill は末尾にグループ化されずに表示されます。すべての Skill がトップレベルの場合、出力は従来のフォーマットと同じフラットなリストになります。

### check

ネストされた Skill を検出し、相対パスを表示します。

```bash
$ skillshare check -g
! utils/remotion  update available · github.com/remotion-dev/skills

! Updates available for 1 skill, 14 up to date · 2.1s

Next
  skillshare update --all  pull the updates
```

### update

**完全なパス**と**短い名前**の両方に対応しています。

```bash
# 完全な相対パス
skillshare update -g frontend/react/react-best-practices

# 短い名前（ベース名） — 自動的に解決される
skillshare update -g react-best-practices

# すべてを更新する
skillshare update -g --all
```

短い名前が複数の Skill に一致する場合、skillshare はより具体的に指定するよう求めます。

```
'my-skill' matches multiple items:
  - frontend/my-skill
  - backend/my-skill
Please specify the full path
```

### enable / disable

フォルダを使うと、カテゴリ全体を一度に有効/無効に切り替えるのが簡単になります。`disable`/`enable` は glob パターンを受け付けるので、フォルダを指定できます。

```bash
# frontend/ 配下のすべての Skill を無効化する（任意の深さ）
skillshare disable "frontend/**"

# 同じパターンでフォルダ全体を再度有効化する
skillshare enable "frontend/**"

# Target に反映する
skillshare sync
```

これは `.skillignore` に `frontend/**` という1行を書き込み、後でそのフォルダに追加するものすべてをカバーし続けます。個々の Skill を切り替えるには、代わりにその名前を渡してください（`skillshare disable frontend/react/react-best-practices`）。

:::tip パターンを引用符で囲む
シェルが先に `*` を展開してしまわないよう、フォルダパターンは引用符（`"frontend/**"`）で囲んでください。
:::

詳しくは [enable / disable](/docs/reference/commands/enable) と [.skillignore の構文](/docs/reference/filtering#skillignore) を参照してください。

---

## フォルダへの直接インストール {#install-directly-into-folders}

`--into` を使うと、Skill を1ステップでサブディレクトリにインストールできます — 手動での `mv` は不要です。

```bash
# カテゴリフォルダにインストールする
skillshare install anthropics/skills -s pdf --into frontend
# → ~/.config/skillshare/skills/frontend/pdf/

# 複数階層のネスト
skillshare install ~/my-skill --into frontend/react
# → ~/.config/skillshare/skills/frontend/react/my-skill/

# --track でも使える
skillshare install github.com/team/skills --track --into devops
# → ~/.config/skillshare/skills/devops/_team-skills/

# Project mode でも使える
skillshare install anthropics/skills -s pdf --into tools -p
# → .skillshare/skills/tools/pdf/
```

`skillshare sync` の後、Target には自動フラット化された名前が表示されます。
- `frontend/pdf/` → `frontend__pdf`
- `frontend/react/my-skill/` → `frontend__react__my-skill`
- `devops/_team-skills/frontend/ui/` → `devops___team-skills__frontend__ui`

:::tip
`--into` は中間ディレクトリを自動的に作成します。先に `mkdir` する必要はありません。
:::

---

## 推奨されるフォルダ構造

### ドメイン別

```
skills/
├── frontend/
│   ├── react/
│   ├── vue/
│   └── css/
├── backend/
│   ├── api-design/
│   └── database/
├── devops/
│   ├── docker/
│   └── ci-cd/
└── utils/
    ├── git-workflow/
    └── code-review/
```

### ツールエコシステム別

```
skills/
├── vue/
│   ├── vue-best-practices/
│   ├── vue-debug-guides/
│   ├── vue-pinia-best-practices/
│   └── vue-router-best-practices/
├── react/
│   └── react-best-practices/
└── web/
    ├── accessibility/
    ├── performance/
    └── seo/
```

### 混合: 個人 + Tracked repos

```
skills/
├── frontend/              # 個人の整理された Skill
│   └── vue/
├── utils/                 # 個人のユーティリティ
│   └── ascii-box-check/
├── _team-skills/          # Tracked repo (自動更新)
│   ├── code-review/
│   └── deploy/
└── _org-standards/        # 別の Tracked repo
    └── security/
```

---

## Skill をバージョン管理する

フォルダで Skill を整理することは、自然に git と組み合わせられます。

```bash
skillshare init --remote git@github.com:yourname/my-skills.git
skillshare push -m "organize skills into categories"
```

これにより次のことが得られます。
- マシン間の Skill 変更の**履歴**
- GitHub/GitLab による**バックアップ**
- **共有** — 他の人があなたのコレクションを閲覧・fork できる
- `skillshare pull` による**クロスマシン Sync**（[Cross-Machine Sync](/docs/how-to/sharing/cross-machine-sync) を参照）

---

## フラットからフォルダへの移行 {#migrating-from-flat-to-folders}

:::tip 新規インストール
新しい Skill には、`--into` を使って正しいフォルダに直接インストールしてください — 上記の [フォルダへの直接インストール](#install-directly-into-folders) を参照。
:::

自分で作成した Skill（`skillshare new` や手作業で作成したもの）にはインストール記録がないため、`mv` で移動できます：

```bash
cd ~/.config/skillshare/skills
mkdir -p frontend/react
mv my-react-notes frontend/react/
skillshare sync
```

`sync` の後、Target は自動的に更新されます — 古いフラットなシンボリックリンクはクリーンアップされ、新しいフラット化された名前が作成されます。

:::warning `skillshare install` でインストールした Skill
インストール済みの Skill は `mv` で移動しないでください。`.metadata.json` のインストール記録はパスをキーにしているため、次に `sync`、`install` または **Install missing** が移動したコピー（同名・インストール後未変更・1 つだけ）を認識して記録を移すまで、記録は古いパスに残ります。それまで `update` と `uninstall` はその Skill を見つけられず、プロジェクトのロックピンと `.skillignore` の行は追従しません。代わりに [`skillshare move`](/docs/reference/commands/move) を使ってください。Skill とインストール記録を一緒に移動するため、再ダウンロードは行われません：

```bash
skillshare move react-best-practices frontend/react
skillshare sync
```

`move` は複数の Skill やフォルダ全体も受け付けます（`skillshare move frontend archive`）。`--dry-run` でプレビューできます。`mv` と同様、`skillshare sync` を実行するまで Target は更新されません。
:::

---

## 関連項目

- [Source と Targets](/docs/understand/source-and-targets) — フラット化の仕組み
- [Tracked Repositories](/docs/understand/tracked-repositories) — リポジトリ内のネストされた Skill
- [ベストプラクティス](./best-practices.md) — 命名規則
- [install](/docs/reference/commands/install) — サブディレクトリへの `--into` インストール
