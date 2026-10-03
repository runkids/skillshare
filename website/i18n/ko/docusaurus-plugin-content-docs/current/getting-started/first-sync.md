---
sidebar_position: 2
---

# 첫 Sync

처음 설정하는 전체 과정을 순서대로 안내합니다. 설치부터 Sync가 동작하기까지 대략 5분이면 충분합니다. 다른 머신에서 복원하는 경우와 TTY 없는 머신에서 무인 실행하는 경우, 두 가지 변형은 이 페이지 끝부분에 정리해 두었습니다.

## 사전 준비

- macOS, Linux 또는 Windows
- AI CLI가 최소 하나 설치되어 있어야 합니다 (Claude Code, Pi, Codex 등)

## 1. CLI 설치

**Homebrew (macOS / Linux):**
```bash
brew install skillshare
```

:::note
Homebrew 릴리스는 며칠 늦어질 수 있습니다. 최신 버전을 원한다면 설치 스크립트를 사용하세요.
:::

**설치 스크립트 (macOS / Linux):**
```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

스크립트는 기본적으로 `~/.local/bin`에 설치하므로 일반적인 설치와 업데이트에는 `sudo`가 필요하지 않습니다. 설치 프로그램이 PATH 설정 안내를 표시할 때만 안내에 따라 설정한 후 `skillshare`를 실행하세요. 이후 터미널에서도 사용할 수 있도록 안내된 명령을 shell 설정 파일(예: `~/.zshrc` 또는 `~/.bashrc`)에 추가할 수 있습니다. 다른 설치 위치는 `INSTALL_DIR`로 지정할 수 있습니다.

기존 설치는 이동하거나 삭제하지 않습니다. PATH에서 이전 실행 파일이 우선하면 설치 프로그램이 해당 위치를 표시합니다. 새 설치 디렉터리를 PATH 앞에 두세요. 보호된 사용자 지정 디렉터리는 여전히 `sudo`가 필요할 수 있습니다.

시스템 전체에서 공유하는 설치가 필요하면:

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | INSTALL_DIR=/usr/local/bin sh
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

:::tip 나중에 업데이트하기
`skillshare upgrade`는 설치 방식(Homebrew, 스크립트, 수동)을 감지해 CLI를 제자리에서 업데이트합니다.
:::

## 2. 초기화

```bash
skillshare init
```

<p>
  <img src="/img/init-demo.png" alt="skillshare init: 답한 질문, 요약, 첫 sync" width="720" />
</p>

`init`은 짧은 질문 몇 가지를 묻고 요약을 보여 줍니다. 확인하기 전에는 아무것도 쓰지 않습니다.

1. **시작 방법** — 이 머신에서 새로 설정하거나 [기존 skillshare 저장소에 연결](/docs/how-to/sharing/cross-machine-sync#second-machine-setup)합니다.
2. **Targets** — 감지된 AI CLI가 모두 선택되어 있습니다. Space 키로 하나씩 뺄 수 있습니다.
3. **가져오기** — 그 도구들에 이미 있는 Skill을 source로 복사합니다.
4. **Git** — 기본으로 켜져 있어 나중에 Skill을 저장소로 push할 수 있습니다.
5. **원격 저장소** — Skill 저장소 URL을 붙여넣습니다 (예: `git@github.com:you/skills.git`). Enter로 건너뛰고 나중에 추가해도 됩니다.

요약에는 source 디렉터리(`~/.config/skillshare/skills/`), sync mode, 내장 Skill(AI CLI에 `/skillshare` 명령 추가)도 표시됩니다. **Change settings**로 바꿀 수 있고 <kbd>Esc</kbd>로 취소할 수 있습니다. 확인하면 `init`이 바로 sync할지 묻습니다.

### Sync 모드 선택하기

`init`은 새로 추가되는 Target의 기본값을 정하는 `--mode <merge|copy|symlink>` 옵션을 받습니다.

- `merge` (기본값) — Skill 단위 symlink. Target에 원래 있던 로컬 Skill이 그대로 보존됩니다
- `symlink` — Target 디렉터리 전체가 하나의 symlink가 됩니다 (가장 빠르지만 디렉터리를 대체합니다)
- `copy` — 실제 파일을 복사합니다. 변경 사항은 다음 `sync`에서 반영됩니다

Target별 재정의는 나중에 `skillshare target <name> --mode <mode>`로 할 수 있습니다.

## 3. Skill 설치

```bash
skillshare install anthropics/skills/skills/pdf
```

모든 설치는 보안 감사를 거칩니다. Critical 등급이 발견되면 설치가 차단되며, 내용을 검토하고 위험을 감수하기로 했을 때만 `--force`를 사용하세요.

## 4. Sync

```bash
skillshare sync
```

이제 설정된 모든 Target이 Source를 가리킵니다.

## 5. 확인

```bash
skillshare status
```

```text
$ skillshare status

Source
─────────────────────────────────────────
✓ ~/.config/skillshare/skills (43 skills, 2026-09-28 12:39)
✓ ~/.config/skillshare/agents (2 agents, 2026-09-28 12:39)

Targets
─────────────────────────────────────────
claude
  skills   merged       [merge] ~/.claude/skills (43 shared, 0 local)
  agents   merged       [merge] 2/2 linked
cursor
  skills   merged       [merge] ~/.cursor/skills (43 shared, 0 local)
  agents   merged       [merge] 2/2 linked
gemini
  skills   merged       [merge] ~/.gemini/skills (43 shared, 0 local)
…
```

출력에 Source 경로와 모든 Target이 나타납니다. `merge` 모드에서 동기화된 Target은 `merged`로 표시되며, shared 개수에 방금 설치한 Skill도 포함됩니다.

대시보드(`skillshare ui`)에서도 같은 상태를 한눈에 볼 수 있습니다.

![첫 동기화 후의 Dashboard: 하나의 Source가 모든 Target에 연결되어 모두 동기화됨](/img/web-dashboard-demo.png)

---

## 방금 무슨 일이 일어났나요

1. **`init`**이 `~/.config/skillshare/config.yaml`과 `~/.config/skillshare/skills/`를 만들고, AI CLI를 자동 감지했으며, 원격 저장소를 지정했다면 거기에 있던 기존 Skill을 클론해 왔습니다.
2. **`install`**이 Skill을 Source 디렉터리로 클론하고 보안 감사를 실행했습니다. `.metadata.json`에 upstream URL과 커밋이 기록되므로 `skillshare update`로 이후 변경 사항을 받아올 수 있습니다.
3. **`sync`**가 각 Target에 설정된 모드를 적용했습니다. 예를 들어 `merge` 모드에서는 다음과 같습니다.
   ```
   ~/.claude/skills/pdf → ~/.config/skillshare/skills/pdf  (symlink)
   ```

`merge`와 `symlink` 모드에서는 Source를 편집하면 모든 Target에 즉시 나타납니다. `copy` 모드에서는 다음 `sync` 때 반영됩니다. Target에 원래 있던 로컬 Skill은 `merge`와 `copy`에서 보존됩니다. `skillshare backup`은 파괴적인 작업 전에 스냅샷을 만들고, `skillshare restore <target>`으로 되돌릴 수 있습니다.

특정 Target 하나만 다른 모드가 필요한가요? Target별로 재정의하세요.

```bash
skillshare target <name> --mode copy
skillshare sync
```

전체 결정 매트릭스는 [Sync 모드](/docs/understand/sync-modes)를 참고하세요.

---

## 변형: 다른 머신에서 복원하기

이미 다른 곳에서 skillshare를 쓰고 있고 GitHub에 개인 Skill 리포지터리가 있는 경우입니다. 새 노트북, devcontainer 또는 VM에서 네 개의 명령이면 전부 복원됩니다. 프롬프트도 선택도 없고, 다시 실행해도 결과가 같습니다.

```bash
# 1. CLI 설치 (Homebrew 또는 curl|sh — 위 1단계와 동일)
brew install skillshare

# 2. Skill 리포지터리를 클론하고 감지된 Target 추가
skillshare init \
  --remote git@github.com:<you>/skills.git \
  --all-targets \
  --no-skill

# 3. Tracked 의존성 재설치
#    (_ 접두어 디렉터리는 gitignore되어 있으므로 클론한 리포지터리에 없습니다)
skillshare install https://github.com/<your-company>/skills --track --force

# 4. Sync
skillshare sync
```

`--no-skill`은 내장 Skill을 건너뜁니다. 이 머신에서도 필요하다면 나중에 `skillshare upgrade --skill`로 추가하세요.

---

## 변형: 헤드리스 설정 (TTY 없음)

CI 작업, devcontainer post-create 훅, 클라우드 VM 프로비저너를 위해 모든 프롬프트에 비대화형 플래그가 준비되어 있습니다.

```bash
skillshare init \
  --source ~/.config/skillshare/skills \
  --remote https://github.com/<you>/skills \
  --targets codex \
  --mode merge \
  --no-copy \
  --no-skill

skillshare install https://github.com/<your-company>/skills --track --force
skillshare sync
```

| 플래그 | 효과 |
|---|---|
| `--source <path>` | Source 경로 프롬프트를 건너뜁니다 |
| `--remote <url>` | 원격 저장소 프롬프트를 건너뜁니다. 원격에 내용이 있으면 클론합니다 |
| `--targets <name>` | 나열한 Target만 추가합니다 (감지된 전부를 추가하려면 `--all-targets` 사용) |
| `--mode merge` | 새 Target의 기본 Sync 모드 |
| `--no-copy` | "기존 Target Skill을 복사할까요?" 프롬프트를 건너뛰고 빈 상태로 시작합니다 |
| `--no-skill` | 내장 Skill 프롬프트를 건너뜁니다 |

`--targets`, `--all-targets`, `--no-targets`는 서로 배타적이므로 하나만 선택하세요.

---

## 다음 단계

- [나만의 Skill 만들기](/docs/how-to/daily-tasks/creating-skills)
- [머신 간 Sync](/docs/how-to/sharing/cross-machine-sync)
- [조직 전체 Skill](/docs/how-to/sharing/organization-sharing)
- [Agent](/docs/understand/agents) — Skill과 함께 단일 파일 `.md` Agent 관리하기
- [Sync 모드](/docs/understand/sync-modes) — 결정 매트릭스와 장단점
