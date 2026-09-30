---
sidebar_position: 5
---

# completion

명령어, 하위 명령어, 플래그에 대한 tab-completion을 위한 shell completion 스크립트를 생성하고 설치합니다.

## 언제 사용하나요

- skillshare 명령과 플래그에 대한 tab-completion을 원할 때
- 새 머신이나 shell 환경을 설정할 때
- alias(예: `ss`)를 사용 중이며 completion이 함께 동작하길 원할 때

## 사용법

```bash
# 자동 설치 (권장)
skillshare completion bash --install
skillshare completion zsh --install
skillshare completion fish --install
skillshare completion powershell --install
skillshare completion nushell --install

# 스크립트를 stdout으로 출력 (고급)
skillshare completion bash
skillshare completion zsh > ~/.zsh/completions/_skillshare
```

## 지원되는 Shell

| Shell | 설치 경로 |
|-------|-------------|
| bash | `~/.local/share/bash-completion/completions/skillshare` |
| zsh | `~/.zsh/completions/_skillshare` |
| fish | `~/.config/fish/completions/skillshare.fish` |
| powershell | `~/.config/powershell/completions/skillshare.ps1` |
| nushell | `~/.config/nushell/completions/skillshare.nu` |

## 플래그

| 플래그 | 설명 |
|------|------|
| `--install` | 표준 설치 경로에 completion 스크립트를 작성 |
| `--help`, `-h` | 사용법 표시 |

## Completion 범위

생성된 스크립트는 다음에 대한 tab-completion을 제공합니다.

- **명령어** — 모든 최상위 명령(`sync`, `install`, `list` 등)
- **하위 명령어** — `target add/remove/list`, `trash list/restore/delete/empty`, `hub add/list/remove/default/index`, `extras init/list/remove/collect/source`, `audit rules disable/enable/severity/reset/init`, `backup files list/show/restore`, `ui start/stop`, `hooks add/disable/edit/enable/import/list/remove/restore/sync`
- **플래그** — short form을 포함한 명령별 플래그(`--dry-run`/`-n`, `--force`/`-f` 등)
- **전역 플래그** — `--project`/`-p`, `--global`/`-g`

## Alias 지원

bash, zsh, PowerShell 스크립트는 `skillshare`를 가리키는 alias를 자동으로 감지하여 해당 alias에 대한 completion을 등록합니다.

```bash
alias ss=skillshare
source <(skillshare completion bash)
ss sy<Tab>  # → ss sync
```

Fish의 alias(함수 wrapper)는 completion을 자동으로 상속받습니다.

Nushell의 경우, 설정에서 `alias ss = skillshare`를 사용하면 alias completion이 네이티브로 상속됩니다.

## 설치 후 단계

`--install` 이후, completion을 활성화하기 위해 추가 작업이 필요할 수 있습니다.

### Bash

```bash
# shell을 재시작하거나, 다음을 실행:
source ~/.local/share/bash-completion/completions/skillshare
```

### Zsh

`.zshrc`에 다음을 추가하세요 (아직 없는 경우).

```bash
fpath=(~/.zsh/completions $fpath)
autoload -Uz compinit && compinit
```

이후 shell을 재시작하거나 `exec zsh`를 실행하세요.

### Fish

새 fish 세션에서 completion이 자동으로 사용 가능합니다.

### PowerShell

PowerShell 프로필(`echo $PROFILE`)에 다음을 추가하세요.

```powershell
. ~/.config/powershell/completions/skillshare.ps1
```

### Nushell

Nushell 설정(`$nu.config-path`)에 다음을 추가하세요.

```nu
source ~/.config/nushell/completions/skillshare.nu
```

## 예시

```
$ skillshare completion bash --install
✔ Completion script installed to /home/user/.local/share/bash-completion/completions/skillshare

ℹ Restart your shell or run:
  source /home/user/.local/share/bash-completion/completions/skillshare
```

## 참고 항목

- [doctor](./doctor.md) — 환경 문제 진단
- [version](./version.md) — CLI 버전 표시
