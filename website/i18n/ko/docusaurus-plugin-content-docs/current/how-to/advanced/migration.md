---
sidebar_position: 5
---

# 마이그레이션

다른 skill 관리 방식에서 skillshare로 마이그레이션하세요.

## 수동 관리로부터

AI CLI 사이에서 skill을 수동으로 복사해왔다면:

### 1단계: skillshare 초기화

```bash
skillshare init
```

### 2단계: 기존 skill 수집

```bash
# 각 AI CLI에서 수집
skillshare collect claude
skillshare collect pi
skillshare collect codex

# 또는 한 번에 모두 수집
skillshare collect --all
```

### 3단계: 중복 처리

같은 skill이 여러 곳에 존재하면 `collect`가 경고합니다. 어느 것을 유지할지 선택하세요.

### 4단계: Sync

```bash
skillshare sync
```

이제 모든 target이 단일 source로 symlink됩니다.

---

## 다른 Install 도구로부터

`npx install-skill`이나 유사한 도구를 사용했다면:

### 1단계: skillshare 초기화

```bash
skillshare init
```

### 2단계: 기존 skill 백업

```bash
skillshare backup
```

### 3단계: 수집 또는 재설치

**옵션 A: 기존 항목 수집** (현재 버전 유지)
```bash
skillshare collect --all
```

**옵션 B: source에서 재설치** (최신 버전 가져오기)
```bash
# 메타데이터 확인
cat ~/.config/skillshare/skills/.metadata.json

# 재설치
skillshare install anthropics/skills/skills/pdf
```

### 4단계: Sync

```bash
skillshare sync
```

---

## `npx skills`로부터 {#from-npx-skills}

기존 skills를 `npx skills`로 계속 관리하거나, 선택한 skills를 Skillshare에 넘길 수 있습니다. 두 도구 모두 여러 agent와 전역/프로젝트 설치를 지원합니다. 이전 이유는 Skillshare의 더 넓은 리소스 관리가 필요해서이지, 다른 도구가 설치만 할 수 있어서가 아닙니다. [비교](/docs/understand/philosophy/comparison)를 참고하세요.

**npx lock 자동 가져오기나 자동 관리권 이전 기능은 없습니다.** Collect는 로컬 skill 파일을 보존하지만 원본 추적 정보는 가져오지 않습니다. 저장소에서 다시 설치하면 Skillshare 추적 정보가 생기지만 기존 설치와 다른 콘텐츠를 받을 수 있습니다.

아래 경로와 lock 필드는 **skills v1.8.1** 기준입니다: [전역 lock 소스](https://github.com/vercel-labs/skills/blob/e781cb4d07433dfe1d19e924ed45cc61738c55ec/src/skill-lock.ts), [프로젝트 lock 소스](https://github.com/vercel-labs/skills/blob/e781cb4d07433dfe1d19e924ed45cc61738c55ec/src/local-lock.ts), [CLI 문서](https://github.com/vercel-labs/skills/blob/e781cb4d07433dfe1d19e924ed45cc61738c55ec/docs/commands.md).

| 항목 | Global | Project |
|------|--------|---------|
| 공유 skill 위치 | `~/.agents/skills/<name>/` | `.agents/skills/<name>/` |
| Agent 디렉터리 | 보통 공유 skills를 가리키는 링크. `--copy`는 독립 복사본 생성 | 동일 |
| Lock 파일 | 설정되어 있으면 `$XDG_STATE_HOME/skills/.skill-lock.json`, 아니면 `~/.agents/.skill-lock.json` | `skills-lock.json` |

`experimental_sync`가 만든 skills는 실제 디렉터리가 아니라 npm 의존성을 가리키는 링크일 수 있습니다. 수집 전에 실제 경로를 확인하세요.

### 1단계: 범위를 정하고 원래 구성을 보존하기

2–5단계 명령은 명시적으로 **전역 모드**(`-g`)를 사용합니다. 프로젝트 설치에는 아래 프로젝트 설명을 사용하세요.

초기화 또는 동기화 전:

- 각 도구가 관리할 skills를 정하세요. 이전 중에는 업데이트, 제거, 자동 동기화 작업을 중지하세요.
- 원래 skill 디렉터리, 실제 콘텐츠, symlink 구성을 별도 스냅샷으로 저장하세요. 교체될 수 있는 기존 Skillshare source도 포함하세요.
- 해당 npx lock 파일을 별도로 보존하세요. npm 프로젝트에서는 `package.json`과 패키지 관리자의 lock도 저장하세요. 이 파일들은 소스와 의존성 기록이지 로컬 수정의 백업이 아닙니다.

인계를 검증할 때까지 스냅샷을 유지하세요. 이전 lock을 삭제하거나 정리 목적으로 `npx skills remove`를 실행하지 마세요.

### 2단계: 복사 없이 초기화하고 백업 확인하기

Skillshare가 아직 초기화되지 않았다면 **stdin과 stdout 모두 터미널에 연결된 대화형 환경**에서 다음 명령을 실행하세요. 파이프나 자동화로 실행하지 마세요. 이 플래그들은 초기 동기화를 비활성화하지 않으며 비대화형 초기화에서는 targets가 선택되면 자동으로 동기화됩니다.

```bash
skillshare init -g --no-copy --no-skill --no-git
```

관리할 targets를 선택하고 `~/.agents/skills`에 해당하는 `universal`을 포함하세요. 이 절차에서는 merge mode를 사용합니다. 초기 동기화를 묻는다면 거절하세요. 이미 설정했다면 초기화하지 말고 source, [targets](/docs/reference/commands/target), 모드를 확인하세요.

```bash
skillshare backup -g --dry-run
skillshare backup -g
skillshare backup -g --list
```

출력의 실패 여부와 실제 skill 파일 포함 여부를 확인하세요. Target 백업은 **symlink를 건너뛰며**, 링크 대상의 콘텐츠나 target 디렉터리 밖의 npx lock을 저장하지 않습니다. 1단계 스냅샷을 보완하는 것이지 대체하지 않습니다.

### 3단계: 로컬 복사본 또는 원본 추적 선택하기

**현재 파일 유지:** 실제 디렉터리를 가진 target에서 미리 보고 수집하세요.

```bash
skillshare collect universal -g --dry-run
skillshare collect universal -g
```

이는 해당 target의 수집 가능한 모든 skills를 수집하며 선택한 하나만 수집하는 명령이 아닙니다. 특정 agent 디렉터리에 복사본이 있다면 해당 설정된 target 이름으로 바꾸세요. 모든 설정된 targets에서 수집하려는 경우에만 `collect --all -g`를 사용하세요. 일부만 넘길 때는 선택한 실제 skill 디렉터리만 Skillshare source에 복사하세요.

Collect는 `node_modules` 링크를 포함한 skill symlink를 건너뜁니다. npm 관리를 유지하거나, 링크가 가리키는 실제 skill 디렉터리를 의도적으로 로컬 스냅샷으로 복사하세요. 이 복사본은 패키지 업데이트를 따라가지 않습니다. 강제하지 않으면 source에 이미 있는 이름도 건너뜁니다. 건너뜀/실패 결과를 살피고 중복 콘텐츠를 직접 비교하세요. 미리보기는 lock을 이해하는 충돌 보고서가 아닙니다. `collect --force`로 무조건 한쪽을 선택하지 마세요.

수집된 skills에는 저장소 추적 정보가 없으므로 `skillshare update`로 원래 소스에서 업데이트할 수 없습니다.

**원본 업데이트 유지:** 소스에서 설치하기 전에 저장한 lock 항목을 각각 확인하세요. `sourceType`은 Git, npm, 로컬 등의 소스를 구분하며 `sourceUrl`, `ref`, `skillPath`에 추가 정보가 있을 수 있습니다. 모든 항목에 저장소나 `skillPath`가 있는 것은 아닙니다.

예를 들어 GitHub 항목 `source: "anthropics/skills"`, `skillPath: "skills/pdf/SKILL.md"`는 해당 파일이 들어 있는 skill 디렉터리에 대응합니다.

```bash
skillshare install anthropics/skills/skills/pdf -g --dry-run
skillshare install anthropics/skills/skills/pdf -g
```

Lock에 Git ref가 기록되어 있다면 위 두 명령 모두에 `--branch <recorded-ref>`를 추가하세요([install](/docs/reference/commands/install) 참고). 지정하지 않으면 이 예시는 원격 기본 브랜치를 사용합니다. 이는 새 설치이며 npx 콘텐츠 hash를 재현하는 작업이 아닙니다. 저장한 파일과 비교하고 로컬 수정을 통합하세요. 수집한 skill이 이미 목적지를 차지한다면 먼저 멈추고 해결하세요. 강제 설치는 수정 내용을 덮어쓰고 audit 차단을 무시할 수도 있으므로 무작정 사용하지 마세요. npm과 로컬 항목은 별도로 소스를 판단해야 하며 단순히 `owner/repo`로 변환할 수 없습니다.

### 4단계: 미리 보고 명시적으로 관리권 넘기기

```bash
skillshare sync -g --dry-run
```

`universal`뿐 아니라 **설정된 모든 targets**를 검토하세요. Merge mode는 같은 이름의 로컬 디렉터리를 보존하지만 같은 이름의 agent symlink는 force 없이도 Skillshare source로 다시 연결될 수 있습니다. 넘기려는 skills에만 영향을 주는지 확인한 후 적용하세요.

```bash
skillshare sync -g
```

보존된 실제 디렉터리도 링크로 바꾸려면, source에 원하는 버전이 있고 원래 스냅샷이 완전한지 먼저 확인하세요.

```bash
skillshare sync -g --force --dry-run
# Apply only after reviewing every replacement:
skillshare sync -g --force
```

Force는 설정된 targets 전체에 적용되며 skill별 이전 스위치가 아닙니다. 아직 npx skills가 관리하는 리소스가 미리보기에 포함되면 중지하세요. 관리 대상을 분리하세요. [함께 사용하기](/docs/troubleshooting/faq#using-universal-alongside-npx-skills)를 참고하세요.

### 5단계: 검증하고 관리자를 하나로 유지하기

결과 파일/링크를 확인하고 각 AI 도구가 의도한 skills를 불러오는지 확인하세요. `skillshare status -g`를 실행해 보존된 로컬 skills나 경고를 살피세요. Sync 성공만으로 모든 원래 skill이 이전되었다고 볼 수는 없습니다.

Skillshare는 이전 npx lock 항목을 제거하지 않습니다. 인계 후 이전한 skills에 `npx skills update`, `remove`, `experimental_install`, `experimental_sync`를 사용하지 마세요. 일부만 이전했다면 해당 skills까지 포함하는 범위 미지정 일괄 명령을 피하세요.

되돌리려면 두 관리 도구를 중지하고 저장한 원래 파일, 링크 구성, lock을 함께 복원하세요. Skillshare target 백업은 로컬 파일 복구에 도움이 되지만 원래 npx symlink나 lock을 단독으로 재구성하지 못합니다.

### 프로젝트 설치와 npm 동기화

변경 전에 커밋하지 않았거나 무시된 로컬 수정까지 포함해 프로젝트 파일과 lock을 보존하세요. `skillshare init -p`로 초기화하고 프로젝트 targets를 설정한 뒤, 위 전역 명령 대신 `collect`, `install`, `sync`에 `-p`를 사용하세요. 원본 추적과 symlink 제한은 동일합니다. 프로젝트 모드에서는 skill-target 백업을 만들지 않으므로 검증된 프로젝트 스냅샷이나 해당 파일을 실제로 포함하는 버전 관리를 사용하세요.

`package.json` scripts 또는 CI에서 `experimental_sync`를 호출한다면, 겹치는 skills를 넘기기 전에 작업과 skill 선언을 조정하세요. 그렇지 않으면 다음 의존성 설치 때 다시 연결될 수 있습니다. npm skills를 npx skills에 계속 맡겨도 됩니다. 링크를 `.skillshare/skills/`에 복사했다고 독립적으로 관리되는 것은 아닙니다.

---

## Git Submodule로부터

git submodule을 사용해왔다면:

### 1단계: submodule 콘텐츠 내보내기

```bash
# 기존 skill repo에서
git submodule foreach 'cp -r $toplevel/$sm_path ~/temp-skills/$name'
```

### 2단계: skillshare 초기화

```bash
skillshare init
```

### 3단계: skill 가져오기

```bash
# source로 복사
cp -r ~/temp-skills/* ~/.config/skillshare/skills/

# 또는 tracked repo로 install
skillshare install github.com/org/skill-repo --track
```

### 4단계: Sync

```bash
skillshare sync
```

---

## 커밋된 Project Skill로부터

repo에 이미 `.claude/skills/`, `.cursor/skills/` 같은 디렉터리에 skill이 커밋되어 있다면:

### 1단계: Project mode 초기화

```bash
cd my-project
skillshare init -p
```

### 2단계: `.skillshare/skills/`로 skill 이동

```bash
# 기존 skill을 skillshare source로 복사
cp -r .claude/skills/my-skill .skillshare/skills/
cp -r .claude/skills/api-guide .skillshare/skills/

# 원본 제거 (sync가 symlink로 재생성함)
rm -rf .claude/skills/my-skill .claude/skills/api-guide
```

### 3단계: Sync

```bash
skillshare sync
```

이제 `.claude/skills/my-skill`은 `.skillshare/skills/my-skill`에 대한 symlink이며 — 다른 모든 target(Pi, Windsurf 등)도 자동으로 동일한 skill을 받습니다.

### 4단계: 마이그레이션 커밋

```bash
git add .skillshare/ .claude/skills/ .cursor/skills/
git commit -m "Migrate project skills to skillshare"
```

:::tip 멀티 도구 이점
이전: skill이 하나의 AI CLI에서만 작동했습니다. 이후: 같은 skill이 구성된 모든 target에서 자동으로 사용 가능합니다.
:::

---

## 팀 전용 솔루션으로부터

팀이 커스텀 skill 공유 방식을 사용하고 있다면:

### 1단계: 현재 방식 파악

- skill이 어디에 저장되어 있나요?
- 어떻게 공유되나요?
- 어떻게 업데이트되나요?

### 2단계: 마이그레이션 경로 선택

**옵션 A: Global mode** — 각 머신의 모든 프로젝트에서 skill 사용 가능.

```bash
# 팀 skill repo 생성
cp -r /current/team/skills ~/new-team-skills
cd ~/new-team-skills && git init && git add . && git commit -m "Migrate to skillshare"
git push origin main

# 팀원이 전역으로 install
skillshare install github.com/org/team-skills --track && skillshare sync
```

**옵션 B: Project mode** — skill이 특정 repo에 한정되며, git으로 공유.

```bash
cd my-project
skillshare init -p

# 팀 skill을 project source로 이동
cp -r /current/team/skills/* .skillshare/skills/

# Sync 및 커밋
skillshare sync
git add .skillshare/
git commit -m "Add team skills via skillshare"
```

새 팀원은 다음으로 모든 것을 받습니다.
```bash
git clone github.com/org/my-project
cd my-project
skillshare install -p && skillshare sync
```

**옵션 C: 둘 다** — 조직 전체 표준은 전역으로, 프로젝트별 skill은 repo마다.

```bash
# 조직 표준 (global)
skillshare install github.com/org/standards --track && skillshare sync

# 프로젝트별 skill (project mode)
cd my-project
skillshare init -p
skillshare install github.com/org/project-skills -p && skillshare sync
```

:::tip 무엇을 선택할까요?
- **Global**: 코딩 표준, 보안 감사 — 모든 프로젝트에 필요한 것들
- **Project**: API 규약, 도메인 규칙, 배포 가이드 — 한 repo에 특화된 것들
- **둘 다**: 대부분의 팀이 성장하며 결국 이렇게 됩니다
:::

---

## Global에서 Project로

특정 프로젝트에 속한 skill이 global mode에 있다면:

### 1단계: Project mode 초기화

```bash
cd my-project
skillshare init -p
```

### 2단계: global source에서 skill 복사

```bash
# 특정 skill 복사
cp -r ~/.config/skillshare/skills/api-guide .skillshare/skills/
cp -r ~/.config/skillshare/skills/deploy-rules .skillshare/skills/
```

### 3단계: global에서 제거 (선택 사항)

```bash
skillshare uninstall api-guide
skillshare uninstall deploy-rules
skillshare sync   # global symlink 정리
```

### 4단계: Sync 및 커밋

```bash
skillshare sync   # project mode를 자동 감지
git add .skillshare/
git commit -m "Move project-specific skills to project mode"
```

이후 skill은 이 repo에 한정되어 git을 통해 팀과 공유됩니다 — 더 이상 global 설정을 어지럽히지 않습니다.

---

## History 보존

git history를 유지하고 싶다면:

### 개인 skill의 경우

```bash
# 기존 repo를 skillshare 위치로 clone
git clone your-existing-repo ~/.config/skillshare/skills

# 기존 source로 skillshare 초기화
skillshare init --source ~/.config/skillshare/skills
```

### 팀 repo의 경우

```bash
# .git을 보존하려면 --track 사용
skillshare install github.com/team/skills --track
```

---

## Rollback

마이그레이션이 잘못되었다면:

### 백업에서 복원

```bash
skillshare restore claude
skillshare restore cursor
```

### 새로 시작

```bash
rm ~/.config/skillshare/config.yaml
skillshare init
```

---

## 체크리스트

마이그레이션 전:

- [ ] 현재 모든 skill 위치 나열
- [ ] 중복 항목 파악
- [ ] 커스텀 설정 기록
- [ ] 백업 생성

마이그레이션 후:

- [ ] `skillshare list`에 모든 skill이 표시되는지 확인
- [ ] 각 AI CLI에서 skill 테스트
- [ ] git remote 설정 (원하는 경우)
- [ ] 새 워크플로를 팀과 공유

---

## 참고

- [기존 Skill로부터](/docs/getting-started/from-existing-skills) — 빠른 마이그레이션 경로
- [collect](/docs/reference/commands/collect) — target에서 수집
- [비교](/docs/understand/philosophy/comparison) — 접근 방식 비교
