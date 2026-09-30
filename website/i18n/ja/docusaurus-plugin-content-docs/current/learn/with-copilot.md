---
sidebar_position: 2
---

# GitHub Copilot で skillshare を使う

> インストールから最初の Sync まで — 5分。

## 前提条件

- VS Code または JetBrains で [GitHub Copilot](https://github.com/features/copilot) の coding agent が
  有効になっていること
- macOS、Linux、または Windows

## ステップ 1: skillshare をインストールする

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

インストーラーが PATH 設定の案内を表示した場合のみ、その案内に従ってから以下のコマンドを実行してください。PATH の警告がなければ追加の設定は不要です。

## ステップ 2: 初期化する

```bash
skillshare init
```

これは Copilot の Skill ディレクトリ（`~/.copilot/skills/`）を検出し、自動的に Target として
追加します。

## ステップ 3: Copy モードに切り替える（推奨）

Copilot がシンボリックリンクを正しく辿れないことがあるという報告を受けています。問題を避けるため、
Copilot の Target を**copy モード**に切り替えてください。

```bash
skillshare target copilot --mode copy
```

Copy モードは、シンボリックリンクを作成する代わりに Skill ファイルを物理的に `~/.copilot/skills/`
にコピーします。トレードオフとして Source の編集が即座には反映されなくなります — 変更を反映するには
`skillshare sync` を実行する必要があります。しかしプラットフォーム間でより信頼性が高くなります。

:::tip merge（シンボリックリンク）モードを使うべきとき
macOS や Linux 上で、あなたのマシンで Copilot がシンボリックリンクを正しく読み取れる場合、デフォルトの
merge モードで問題なく動作します。いつでも元に戻せます。

```bash
skillshare target copilot --mode merge
```
:::

## ステップ 4: 最初の Skill をインストールする

```bash
skillshare install runkids/my-skills
```

## ステップ 5: Sync する

```bash
skillshare sync
```

Skill は `~/.copilot/skills/` にコピーされます。Copilot はこれらをカスタム指示として認識します。

## ステップ 6: 確認する

```bash
ls ~/.copilot/skills/
```

インストールした Skill が（copy モードでは）実際のディレクトリとして、（merge モードでは）
シンボリックリンクとして見えるはずです。

## Copilot 固有の注意事項

- **Skill のパス**: `~/.copilot/skills/`（Global）または `.github/skills/`（Project）
- **Agent のパス**: `~/.copilot/agents/`（Global）または `.github/agents/`（Project）— Copilot CLI は
  skillshare が管理するのと同じ `.agent.md` フォーマットでカスタム Agent を読み取るため、
  `skillshare sync agents` は変換なしでそれらを配布します。[Agents](/docs/understand/agents) を
  参照してください。
- **Project mode**: プロジェクトレベルの Copilot Skill を管理するには `skillshare init -p` を実行
  します — これらはコードベースと共に `.github/skills/` に入ります
- **シンボリックリンクの問題**: Copilot が Skill を認識しない場合、Target が merge モードかどうか
  （`skillshare status`）を確認し、上記の通り copy モードに切り替えてください
- **`.github/copilot-instructions.md`**: 既存の instructions ファイルがある場合、skillshare の
  Skill はそれを補完します — 置き換えることはありません

## 次のステップ

- [複数の Skill を管理する →](/docs/how-to/daily-tasks/organizing-skills)
- [チームと共有する →](/docs/how-to/sharing/organization-sharing)
- [さらに Skill を探す →](/docs/reference/commands/search)
