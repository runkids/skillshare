---
sidebar_position: 5.5
---

# 폴더로 Skill 정리하기

Skill 모음이 늘어나면 폴더로 정리해야 관리가 편해집니다 — 나머지는 skillshare가 자동으로 처리합니다.

## 왜 정리해야 할까요?

20개 이상의 Skill이 나열된 평면 목록은 탐색하기 어려워집니다:

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
└── ... 20개 이상 더
```

폴더를 사용하면 논리적으로 그룹화하면서도 skillshare가 AI CLI를 위해 자동으로 평탄화해줍니다:

```
SOURCE (정리된 상태)                     TARGET (자동 평탄화)
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

:::tip 실제 사용 예시
이 패턴을 사용해 Skill 모음을 체계적으로 구성한 완전한 예시는 [runkids/my-skills](https://github.com/runkids/my-skills)를 참고하세요.
:::

---

## 자동 평탄화 작동 방식

skillshare는 `__`(더블 언더스코어)를 구분자로 사용해 폴더 경로를 평탄한 이름으로 변환합니다:

| Source 경로 | 동기화된 Target 이름 |
|---|---|
| `frontend/react/react-best-practices/` | `frontend__react__react-best-practices` |
| `utils/remotion/` | `utils__remotion` |
| `web-dev/accessibility/` | `web-dev__accessibility` |

**핵심 포인트:**
- `SKILL.md`를 포함한 디렉터리만 Skill로 취급됩니다
- 중간 폴더(예: `frontend/` 자체)는 단순히 구조를 위한 것으로 — `SKILL.md`가 필요 없습니다
- `list`와 `sync`는 어떤 깊이든 중첩된 Skill을 찾아냅니다
- `check`와 `update`도 중첩된 Skill과 함께 작동합니다

:::note Agent는 중첩되지 않습니다
이 페이지는 **Skill** 정리에 관한 내용입니다. Agent는 항상 `~/.config/skillshare/agents/`(프로젝트 모드에서는 `.skillshare/agents/`) 바로 아래에 위치하는 단일 `.md` 파일이며 — 폴더 중첩이나 자동 평탄화를 지원하지 않습니다. Agent를 정리하려면 네이밍 규칙(예: `frontend-reviewer.md`, `backend-auditor.md`)과 `.agentignore` 패턴을 사용하세요.
:::

---

## 중첩된 Skill 다루기

### list

같은 디렉터리에 있는 Skill은 자동으로 그룹화됩니다:

```bash
$ skillshare list -g

  frontend/vue/
    → vue-best-practices     github.com/vuejs-ai/skills/...

  utils/
    → remotion               github.com/remotion-dev/skills/...

  web-dev/
    → accessibility          github.com/addyosmani/web-quality-...
```

각 그룹 안에서는 Skill이 전체 평탄화 이름이 아닌 기본 이름으로 표시됩니다. 최상위 Skill은 맨 아래에 그룹 없이 나타납니다. 모든 Skill이 최상위에 있다면 출력은 이전 형식과 동일한 평면 목록이 됩니다.

### check

중첩된 Skill을 감지하고 상대 경로를 보여줍니다:

```bash
$ skillshare check -g
! utils/remotion  update available · github.com/remotion-dev/skills

! Updates available for 1 skill, 14 up to date · 2.1s

Next
  skillshare update --all  pull the updates
```

### update

**전체 경로**와 **짧은 이름** 모두 지원합니다:

```bash
# 전체 상대 경로
skillshare update -g frontend/react/react-best-practices

# 짧은 이름(basename) — 자동으로 해석됨
skillshare update -g react-best-practices

# 전체 업데이트
skillshare update -g --all
```

짧은 이름이 여러 Skill과 일치하면 skillshare가 더 구체적으로 지정하도록 요청합니다:

```
'my-skill' matches multiple items:
  - frontend/my-skill
  - backend/my-skill
Please specify the full path
```

### enable / disable

폴더를 사용하면 전체 카테고리를 한 번에 켜고 끄기 쉬워집니다. `disable`/`enable`은 glob 패턴을 지원하므로 폴더를 지정하면 됩니다:

```bash
# frontend/ 아래 모든 Skill 비활성화(모든 깊이)
skillshare disable "frontend/**"

# 같은 패턴으로 폴더 전체 다시 활성화
skillshare enable "frontend/**"

# Target에 적용
skillshare sync
```

이렇게 하면 `.skillignore`에 `frontend/**` 한 줄이 기록되어, 이후 해당 폴더에 추가하는 항목까지 계속 적용됩니다. 개별 Skill만 전환하려면 이름을 지정하세요(`skillshare disable frontend/react/react-best-practices`).

:::tip 패턴을 따옴표로 감싸세요
셸이 `*`를 먼저 확장하지 않도록 폴더 패턴을 따옴표로 감싸세요(`"frontend/**"`).
:::

자세한 내용은 [enable / disable](/docs/reference/commands/enable)와 [.skillignore 문법](/docs/reference/filtering#skillignore)을 참고하세요.

---

## 폴더에 바로 설치하기 {#install-directly-into-folders}

`--into`를 사용하면 수동으로 `mv`할 필요 없이 한 번에 하위 디렉터리에 Skill을 설치할 수 있습니다:

```bash
# 카테고리 폴더에 설치
skillshare install anthropics/skills -s pdf --into frontend
# → ~/.config/skillshare/skills/frontend/pdf/

# 다중 레벨 중첩
skillshare install ~/my-skill --into frontend/react
# → ~/.config/skillshare/skills/frontend/react/my-skill/

# --track과 함께 사용도 가능
skillshare install github.com/team/skills --track --into devops
# → ~/.config/skillshare/skills/devops/_team-skills/

# 프로젝트 모드에서도 사용 가능
skillshare install anthropics/skills -s pdf --into tools -p
# → .skillshare/skills/tools/pdf/
```

`skillshare sync` 이후, Target에는 자동 평탄화된 이름이 표시됩니다:
- `frontend/pdf/` → `frontend__pdf`
- `frontend/react/my-skill/` → `frontend__react__my-skill`
- `devops/_team-skills/frontend/ui/` → `devops___team-skills__frontend__ui`

:::tip
`--into`는 중간 디렉터리를 자동으로 생성합니다. 먼저 `mkdir`을 할 필요가 없습니다.
:::

---

## 추천 폴더 구조

### 도메인별

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

### 도구 생태계별

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

### 혼합: 개인용 + 추적되는 저장소

```
skills/
├── frontend/              # 개인적으로 정리한 Skill
│   └── vue/
├── utils/                 # 개인 유틸리티
│   └── ascii-box-check/
├── _team-skills/          # 추적되는 저장소(자동 업데이트)
│   ├── code-review/
│   └── deploy/
└── _org-standards/        # 또 다른 추적되는 저장소
    └── security/
```

---

## Skill 버전 관리

폴더로 Skill을 정리하면 git과 자연스럽게 어우러집니다:

```bash
skillshare init --remote git@github.com:yourname/my-skills.git
skillshare push -m "organize skills into categories"
```

이를 통해 다음을 얻을 수 있습니다:
- 여러 기기에서 Skill 변경 내역의 **히스토리**
- GitHub/GitLab을 통한 **백업**
- 다른 사람이 컬렉션을 둘러보고 fork할 수 있는 **공유**
- `skillshare pull`을 통한 **크로스 머신 동기화**(자세한 내용은 [Cross-Machine Sync](/docs/how-to/sharing/cross-machine-sync) 참고)

---

## 평면 구조에서 폴더 구조로 마이그레이션

:::tip 새로 설치하는 경우
새 Skill의 경우, 위의 [폴더에 바로 설치하기](#install-directly-into-folders)에서 설명한 대로 `--into`를 사용해 원하는 폴더에 바로 설치하세요.
:::

직접 만든 Skill(`skillshare new` 또는 수동으로 만든 것)에는 설치 기록이 없으므로 `mv`로 옮길 수 있습니다:

```bash
cd ~/.config/skillshare/skills
mkdir -p frontend/react
mv my-react-notes frontend/react/
skillshare sync
```

`sync` 후에는 Target이 자동으로 업데이트됩니다 — 이전의 평면 심볼릭 링크는 정리되고 새로운 평탄화된 이름이 생성됩니다.

:::warning `skillshare install`로 설치한 Skill
설치한 Skill은 `mv`로 옮기지 마세요. `.metadata.json`의 설치 기록은 경로를 키로 사용하므로, 옮긴 복사본은 소스 연결을 잃고 기록은 계속 이전 위치를 가리킵니다. 대신 [`skillshare move`](/docs/reference/commands/move)를 사용하세요. Skill과 설치 기록을 함께 옮기므로 다시 다운로드하지 않습니다:

```bash
skillshare move react-best-practices frontend/react
skillshare sync
```

`move`는 여러 Skill과 폴더 전체도 받을 수 있습니다(`skillshare move frontend archive`). `--dry-run`으로 미리 볼 수 있습니다. `mv`와 마찬가지로 `skillshare sync`를 실행하기 전까지 Target은 업데이트되지 않습니다.
:::

---

## 참고

- [Source & Targets](/docs/understand/source-and-targets) — 평탄화 작동 방식
- [Tracked Repositories](/docs/understand/tracked-repositories) — 저장소 안의 중첩된 Skill
- [Best Practices](./best-practices.md) — 네이밍 규칙
- [install](/docs/reference/commands/install) — 하위 디렉터리에 설치할 때 `--into` 사용
