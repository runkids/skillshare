---
sidebar_position: 9
---

# Skill 관리 방식 비교

이 페이지는 AI CLI skill 관리에 대한 두 가지 주요 아키텍처 접근 방식을 비교합니다: **imperative**(명령별 install)와 **declarative**(config + sync).

도구를 평가 중이거나 전환을 고려 중이라면, 이 분석이 근본적인 설계 차이를 이해하는 데 도움이 될 것입니다.

## 아키텍처 한눈에 보기

### Imperative (명령별 Install)

Imperative 도구는 명령별 install 모델을 사용합니다 — 각 install이 독립적인 작업입니다:

```
tool add owner/repo → select agents → choose method → done
tool add owner/repo → select agents → choose method → done
tool add owner/repo → select agents → choose method → done
```

모든 작업에 사용자 입력이 필요합니다. "무엇이 어디에 설치되어야 하는가"를 설명하는 영구적인 상태가 없습니다.

### Declarative (Config + Sync)

skillshare는 declarative 모델을 사용합니다 — 원하는 상태를 한 번 정의한 다음 sync합니다:

```yaml
# config.yaml — define once
source: ~/.config/skillshare/skills
targets:
  claude: ~/.claude
  cursor: ~/.cursor/skills
  codex: ~/.codex/skills
```

```bash
skillshare sync  # reconcile actual state to desired state
```

하나의 명령, 프롬프트 없음, 매번 결정론적인 결과.

## 기능 비교

| 기능 | Imperative (명령별 install) | Declarative (skillshare) |
|------------|------------------------|--------------------------|
| **설정** | Config 파일 없음. 실행마다 프롬프트 | `config.yaml` — 한 번 설정하고 계속 재사용 |
| **Agent 선택** | 매번 대화형 프롬프트 | config에 정의됨. `sync`가 모두 처리 |
| **설치 방식** | 작업마다 copy/symlink 선택 | config의 `sync_mode` (merge, copy, symlink) |
| **단일 source of truth** | 각 agent에 독립적으로 skill 복사 | source 디렉터리 → 모든 target에 symlink |
| **하나의 agent에서 skill 제거** | source 파일을 삭제해 다른 agent에 영향을 줄 수 있음 | 해당 target의 symlink에만 영향 |
| **재현 가능한 설정** | 새 머신에서 복원할 내장 방법 없음 | `config.yaml` + source dir = 전체 복원 |
| **프로젝트 범위 skill** | Lock 파일이 global만 추적 | 저장소별 skill을 위한 `skillshare init -p` |
| **여러 머신 간 sync** | 수동 (dotfiles를 통한 lock 파일 sync) | git을 사용한 내장 `push` / `pull` |
| **양방향 흐름** | 단방향 (install만) | `collect`가 target에서 개선 사항을 다시 가져옴 |
| **자신의 skill과 설치한 skill 구분** | 같은 디렉터리에 혼재 | Tracked repo는 `_` 접두사 사용 |
| **오프라인 작동** | CLI 자체에 npx와 네트워크 필요 | 단일 바이너리, 설치 후 오프라인 작동 |
| **웹 대시보드** | 없음 | `skillshare ui` — 시각적 관리 |
| **백업 / 복원** | 없음 | `skillshare backup` / `skillshare restore` |
| **Git 플랫폼 지원** | update/check는 GitHub 전용 (GitHub Trees API에 하드코딩) | 모든 Git remote — GitHub, GitLab, Bitbucket, Azure DevOps, Gitea, AtomGit, Gitee, self-hosted |
| **런타임 의존성** | Node.js + npm | 없음 (단일 Go 바이너리) |

## 자주 겪는 문제 해결

### "설치할 때마다 agent를 매번 선택해야 함"

skillshare에서는 target을 한 번만 설정하면 됩니다:

```yaml
targets:
  claude: ~/.claude
  cursor: ~/.cursor/skills
```

그러면 모든 `sync`, `install`, `collect`가 어디로 가야 할지 알고 있습니다. 프롬프트 없음.

### "하나의 agent에서 skill을 제거하면 다른 agent가 깨짐"

Imperative 도구에서는 하나의 agent에서 skill을 제거하면 공유된 source 파일이 삭제되어 다른 agent에 깨진 symlink가 남을 수 있습니다.

skillshare의 아키텍처는 이를 완전히 방지합니다 — source 디렉터리가 유일한 진실입니다. Target symlink는 source를 **가리킵니다**. Target을 제거해도 그 target의 symlink만 제거되며, source 파일은 그대로입니다.

```
Source: ~/.config/skillshare/skills/my-skill/SKILL.md  (always preserved)
  ├── ~/.claude/skills/my-skill → symlink to source  ✓
  ├── ~/.cursor/skills/my-skill  → symlink to source  ✓  (unaffected)
  └── ~/.codex/skills/my-skill  → symlink to source  ✓  (unaffected)
```

### "새 머신에서 내 설정을 복원할 수 없음"

skillshare를 사용하면 전체 설정이 이식 가능합니다:

1. `~/.config/skillshare/`(source + config)를 버전 관리에 등록
2. 새 머신에서: 설정 repo를 `git clone`
3. `skillshare sync` 실행

모든 target이 즉시 재생성됩니다.

### "Update와 check가 GitLab / Bitbucket / Azure DevOps에서 작동하지 않음"

Imperative 도구는 종종 update 확인을 위해 GitHub Trees API에 의존하는데, 이는 `update`와 `check`가 GitHub가 아닌 소스의 skill을 조용히 건너뛴다는 뜻입니다.

skillshare는 **로컬 git 작업**(`git fetch` + tree hash 비교)을 사용합니다 — GitLab, Bitbucket, Azure DevOps, Gitea, AtomGit, Gitee, 그리고 모든 self-hosted 인스턴스를 포함한 모든 Git remote와 함께 작동합니다. 플랫폼 특화 API가 필요하지 않습니다.

```bash
# All of these support install, update, and check:
skillshare install https://gitlab.com/team/skills
skillshare install git@bitbucket.org:company/private-skills.git
skillshare install https://git.mycompany.com/org/repo
skillshare update   # checks all sources, regardless of host
```

### "큰 저장소에서 clone이 너무 오래 걸림"

skillshare는 tracked가 아닌 install에 기본적으로 shallow clone(`--depth 1`)을 사용해 다운로드 시간을 크게 줄입니다. 전체 히스토리가 필요한 tracked repo는 `--track`을 사용하세요.

### "내 skill들이 여러 agent 디렉터리에 흩어져 있음"

skillshare는 모든 것을 한곳에 유지합니다:

```
~/.config/skillshare/skills/
├── my-custom-skill/          # Your own skills
├── react-best-practices/     # Installed skills
├── _team-repo/               # Tracked repos (prefixed with _)
│   ├── frontend-guidelines/
│   └── code-review/
└── _another-org-repo/
```

`_` 접두사는 tracked(팀/조직) repo를 개인 skill과 명확하게 구분합니다.

## skillshare로 마이그레이션하기

이미 다른 skill 관리자를 사용 중이라면:

### 1단계: skillshare 설치

```bash
# macOS / Linux
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

설치 프로그램이 PATH 설정 안내를 표시할 때만 안내에 따라 설정한 후 아래 명령을 실행하세요. PATH 경고가 없으면 추가 설정은 필요하지 않습니다.

```bash

# Homebrew
brew install skillshare
```

### 2단계: 초기화 및 기존 skill 수집

```bash
skillshare init              # Creates config and detects targets
skillshare collect --all     # Imports existing skills from all detected targets
```

### 3단계: Sync

```bash
skillshare sync              # Symlinks source skills to all targets
```

이제 기존 skill들이 한곳에서 관리됩니다. 자세한 안내는 [Migration Guide](/docs/how-to/advanced/migration)를 참고하세요.

## 올바른 도구 선택하기

**다음의 경우 imperative 도구를 선택하세요:**
- skill을 드물게 설치하고 대화형 프롬프트에 개의치 않는 경우
- AI CLI를 하나만 사용하는 경우
- 여러 머신이나 팀 워크플로우가 필요 없는 경우

**다음의 경우 skillshare를 선택하세요:**
- 여러 AI CLI를 사용하며 서로 동기화되기를 원하는 경우
- 설정하고 잊어버릴 수 있는(set-and-forget) 구성을 원하는 경우
- 여러 머신에서 작업하는 경우
- 팀이나 조직과 skill을 공유하는 경우
- skill에 대한 백업, 복원, 버전 관리를 원하는 경우
- GitLab, Bitbucket, Azure DevOps, 또는 self-hosted Git에 skill을 호스팅하는 경우
- 런타임 의존성이 없는 단일 바이너리를 선호하는 경우
- 로컬 워크플로우 밖에서 설치/다운로드 활동이 추적되는 것을 원하지 않는 경우

---

## 참고

- [Migration](/docs/how-to/advanced/migration) — 마이그레이션 가이드
- [Core Concepts](/docs/understand) — skillshare 동작 방식
