---
sidebar_position: 5
---

# completion

コマンド、サブコマンド、フラグをタブ補完するためのシェル補完スクリプトを生成・インストールします。

## 使うタイミング

- skillshare のコマンドとフラグをタブ補完したい
- 新しいマシンやシェル環境をセットアップする
- エイリアス（例: `ss`）を使っていて、それでも補完を効かせたい

## 構文

```bash
# 自動インストール（推奨）
skillshare completion bash --install
skillshare completion zsh --install
skillshare completion fish --install
skillshare completion powershell --install
skillshare completion nushell --install

# スクリプトを標準出力に出力する（上級者向け）
skillshare completion bash
skillshare completion zsh > ~/.zsh/completions/_skillshare
```

## 対応シェル

| シェル | インストールパス |
|-------|-------------|
| bash | `~/.local/share/bash-completion/completions/skillshare` |
| zsh | `~/.zsh/completions/_skillshare` |
| fish | `~/.config/fish/completions/skillshare.fish` |
| powershell | `~/.config/powershell/completions/skillshare.ps1` |
| nushell | `~/.config/nushell/completions/skillshare.nu` |

## フラグ

| フラグ | 説明 |
|------|-------------|
| `--install` | 標準のインストールパスに補完スクリプトを書き込む |
| `--help`, `-h` | 使い方を表示 |

## 補完の範囲

生成されるスクリプトは、以下のタブ補完を提供します。

- **コマンド** — すべてのトップレベルコマンド（`sync`、`install`、`list` など）
- **サブコマンド** — `target add/remove/list`、`trash list/restore/delete/empty`、`hub add/list/remove/default/index`、`extras init/list/remove/collect/source`、`audit rules disable/enable/severity/reset/init`、`backup files list/show/restore`、`ui start/stop`、`hooks add/disable/edit/enable/import/list/remove/restore/sync`
- **フラグ** — コマンドごとのフラグと短縮形（`--dry-run`/`-n`、`--force`/`-f` など）
- **グローバルフラグ** — `--project`/`-p`、`--global`/`-g`

## エイリアス対応

bash、zsh、PowerShell のスクリプトは、`skillshare` を指すエイリアスを自動的に検出し、それらに対しても補完を登録します。

```bash
alias ss=skillshare
source <(skillshare completion bash)
ss sy<Tab>  # → ss sync
```

Fish のエイリアス（関数のラッパーとして実装されている）は自動的に補完を引き継ぎます。

Nushell では、設定で `alias ss = skillshare` を使っている場合、エイリアスの補完はネイティブに引き継がれます。

## インストール後の手順

`--install` の後、補完を有効化するために追加の作業が必要になる場合があります。

### Bash

```bash
# シェルを再起動するか、以下を実行してください:
source ~/.local/share/bash-completion/completions/skillshare
```

### Zsh

`.zshrc` に以下を追加してください（まだ無ければ）。

```bash
fpath=(~/.zsh/completions $fpath)
autoload -Uz compinit && compinit
```

その後、シェルを再起動するか `exec zsh` を実行してください。

### Fish

新しい fish セッションでは補完が自動的に利用可能になります。

### PowerShell

PowerShell のプロファイル（`echo $PROFILE`）に以下を追加してください。

```powershell
. ~/.config/powershell/completions/skillshare.ps1
```

### Nushell

Nushell の設定（`$nu.config-path`）に以下を追加してください。

```nu
source ~/.config/nushell/completions/skillshare.nu
```

## 例

```
$ skillshare completion bash --install
✔ Completion script installed to /home/user/.local/share/bash-completion/completions/skillshare

ℹ Restart your shell or run:
  source /home/user/.local/share/bash-completion/completions/skillshare
```

## 関連項目

- [doctor](./doctor.md) — 環境の問題を診断
- [version](./version.md) — CLI のバージョンを表示
