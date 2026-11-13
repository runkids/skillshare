---
sidebar_position: 8
---

# Declarative Skill Manifest

skill 컬렉션을 코드로 정의하세요 — 단일 manifest 파일로 설치하고, 공유하고, 설정을 재현합니다.

:::tip 언제 중요한가요?
여러 머신에서 재현 가능한 skill 설정, 단일 명령으로 팀 온보딩, 또는 오픈소스 프로젝트 부트스트랩이 필요할 때 declarative manifest를 사용하세요.
:::

## Skill Manifest란?

skill manifest는 skill 컬렉션의 **이식 가능한 선언**입니다. skill을 하나씩 수동으로 설치하는 대신, manifest 파일에 나열하고 `skillshare install`을 실행하면 모든 것을 갖출 수 있습니다.

manifest 위치는 mode에 따라 다릅니다:

| Mode | Manifest 위치 | 커밋 가능? |
|------|------------------|-------------|
| **Project** | `.skillshare/config.yaml` (`skills:` 섹션) | 예 — 팀과 공유하려면 커밋 |
| **Global** | `~/.config/skillshare/skills/.metadata.json` | 아니오 — 개인 머신 상태 |

### Project Mode Manifest

project mode에서는 `skills:`가 `targets:`와 함께 `config.yaml`에 존재합니다:

```yaml
# .skillshare/config.yaml
targets:
  - claude
  - cursor

skills:
  - name: react-best-practices
    source: anthropics/skills/skills/react-best-practices
    group: frontend
  - name: _team-skills
    source: my-org/shared-skills
    tracked: true
  - name: commit
    source: anthropics/skills/skills/commit
```

이 파일은 git에 커밋됩니다 — 팀원이 repo를 clone하고 `skillshare install -p`를 실행하면 나열된 모든 skill이 설치됩니다.

manifest는 *무엇을* 설치할지를 기록합니다. 각 skill이 실제로 해석된 정확한 커밋은 그 옆의 `.skillshare/skills.lock.json`에 기록되며, 이 파일은 자동으로 작성되고 함께 커밋해야 합니다. 두 파일이 모두 있으면 upstream이 그 사이 진행되었더라도 `skillshare install -p`는 모든 팀원에게 동일한 커밋을 제공합니다. [Lockfile](./project-skills.md#lockfile)을 참고하세요.

### Global Mode Manifest

global mode에서는 skill 기록이 `.metadata.json`(중앙화된 메타데이터 저장소)에 저장됩니다. 이 파일에는 런타임 추적 데이터(해시, 타임스탬프)도 포함되며 자동으로 관리됩니다.

## 동작 방식

### Manifest로부터 설치하기

인자 없이 `skillshare install`을 실행하면 manifest를 읽고 나열된 모든 skill을 설치합니다:

```bash
# Global mode — installs all skills from ~/.config/skillshare/skills/.metadata.json
skillshare install

# Project mode — installs all skills from .skillshare/config.yaml skills: section
skillshare install -p

# Preview without installing
skillshare install --dry-run
```

이미 존재하는 skill은 자동으로 건너뜁니다. project mode에서는 설치된 커밋이 lockfile과 다른 skill은 건너뛰는 대신 고정된 커밋으로 이동됩니다.

### 자동 조정(Reconciliation)

manifest는 실제 skill 컬렉션과 동기화 상태를 유지합니다:

- **`skillshare install <source>`** — 설치된 skill을 manifest에 자동으로 추가
- **`skillshare uninstall <name>...`** — manifest에서 해당 항목을 자동으로 제거

project mode에서는 `config.yaml`과 `skills.lock.json`이 업데이트됩니다. global mode에서는 `.metadata.json`이 업데이트됩니다. manifest를 수동으로 편집할 필요는 없습니다 (물론 할 수는 있습니다).

Global tracked 선언과 감사 승인 기록은 checkout이 없어도 유지되므로 소스에 접근할 수 있고 대상 경로가 비어 있으면 `skillshare install`로 복원할 수 있습니다. 다른 폴더나 소스 링크가 대상 경로를 차지한다면 먼저 폴더를 옮기거나 링크를 해제하세요. 선언을 제거하려면 checkout이 있을 때 `skillshare uninstall <name>`을 사용하세요. checkout이 없다면 먼저 `skillshare install`로 복원한 다음 제거하세요. Project mode에서는 로컬 설치 기록이 없는 config 선언이 유지되어 새로 clone한 곳에서 `skillshare install -p`로 설치할 수 있습니다.

## Skill 항목 필드

`skills:` 목록의 각 항목에는 다음 필드가 있습니다:

| 필드 | 필수 | 설명 |
|-------|----------|-------------|
| `name` | 예 | skill 이름 (source 안의 디렉터리 이름) |
| `source` | 예 | install 소스 (GitHub shorthand, HTTPS URL, SSH URL) |
| `tracked` | 아니오 | tracked repository의 경우 `true` (`.git` 보존) |
| `group` | 아니오 | 하위 디렉터리 경로 (예: `frontend` 또는 `frontend/vue`). install 중 `--into`에 해당. |

## 사용 사례

### 개인 설정

여러 머신에서 개인 skill 컬렉션을 유지하세요:

```bash
# On machine A — skills are already installed and tracked in registry
skillshare push   # backup config + registry to git

# On machine B — fresh machine
skillshare pull   # restore config + registry from git
skillshare install  # install all skills from manifest
skillshare sync   # distribute to all targets
```

### 팀 온보딩

새 팀원이 한 명령으로 동일한 AI 컨텍스트를 얻습니다:

```bash
# .skillshare/config.yaml skills: section is committed to the repo
git clone <project-repo>
cd <project-repo>
skillshare install -p   # installs all declared skills
skillshare sync -p      # links to project targets
```

### 오픈소스 부트스트랩

프로젝트 유지관리자가 `config.yaml`에 권장 skill을 선언합니다:

```yaml
# .skillshare/config.yaml
targets:
  - claude
  - cursor

skills:
  - name: react-best-practices
    source: anthropics/skills/skills/react-best-practices
    group: frontend
  - name: commit
    source: anthropics/skills/skills/commit
```

:::info Group 필드와 `--into`
`--into`로 설치하면 group이 자동으로 기록됩니다:

```bash
skillshare install anthropics/skills/skills/pdf --into frontend -p
# config.yaml will contain: name: pdf, group: frontend
```

`skillshare install -p`(인자 없이)를 실행하면 manifest로부터 동일한 디렉터리 구조가 재생성됩니다.
:::

기여자는 clone한 뒤 `skillshare install -p`를 실행하여 즉시 프로젝트 전용 AI 컨텍스트를 얻습니다.

## 워크플로우 요약

```
Project mode:
1. Install skills normally      →  config.yaml skills: auto-updates
2. Commit config.yaml and skills.lock.json via git  →  same skills, same commits for the team
3. Run `skillshare install -p`  →  reproduce on clone
4. Run `skillshare sync`        →  distribute to all targets

Global mode:
1. Install skills normally      →  .metadata.json auto-updates
2. Push/pull config via git     →  portable across machines
3. Run `skillshare install`     →  reproduce on new machine
4. Run `skillshare sync`        →  distribute to all targets
```

## Extras 설정

skill 외에도, `config.yaml`은 **extras**를 선언할 수 있습니다 — 별도 디렉터리로 sync되는 skill이 아닌 리소스(rules, commands, prompts)입니다. Extras는 `config.yaml`의 `extras:` 섹션에서 설정됩니다 (global과 project 모두):

```yaml
extras:
  - name: rules
    targets:
      - path: ~/.claude/rules
      - path: ~/.cursor/rules
        mode: copy
```

자세한 내용은 [sync extras](/docs/reference/commands/sync#sync-extras)를 참고하세요.

## 관련 문서

- [Install command](/docs/reference/commands/install) — 인자가 있는/없는 `skillshare install`
- [Push/Pull](/docs/reference/commands/push) — git을 통한 config 백업 및 복원
- [Project Skills](./project-skills.md) — 프로젝트 레벨 manifest
