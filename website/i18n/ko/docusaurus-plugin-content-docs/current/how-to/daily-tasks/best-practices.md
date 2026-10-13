---
sidebar_position: 6
---

# 모범 사례

Skill을 위한 네이밍 규칙, 조직화, 버전 관리.

## 네이밍

### Skill 이름

**할 것:**
- 하이픈을 사용한 소문자: `code-review`, `pdf-tools`
- 설명적으로: `rcg`가 아닌 `react-component-generator`
- 팀을 위한 네임스페이스: `acme-code-review`

**하지 말 것:**
- 공백이나 특수문자 사용
- 일반적인 이름 사용: `helper`, `utils`, `tools`
- 흔한 skill 이름과 충돌

### Repository 이름

**개인용:**
```
my-skills
ai-skills
```

**팀용:**
```
<team>-skills
<org>-skills
```

---

## 조직화

### 개인 skill

```
~/.config/skillshare/skills/
├── code-review/
├── pdf-tools/
├── git-workflow/
└── _team-skills/      # Tracked repo
```

### 팀 repo

```
team-skills/
├── frontend/
│   ├── react/
│   ├── vue/
│   └── testing/
├── backend/
│   ├── api/
│   └── database/
├── devops/
│   ├── deploy/
│   └── monitoring/
└── README.md
```

### Skill 디렉터리

```
my-skill/
├── SKILL.md           # 필수
├── README.md          # 선택: 사람을 위한 문서
├── examples/          # 선택: 예시 파일
└── templates/         # 선택: 코드 템플릿
```

---

## 버전 관리

### 커밋 메시지

Conventional commits를 따르세요.
```
feat(code-review): add security check
fix(pdf-tools): handle empty files
docs(readme): update installation
```

### 브랜칭

**개인용:**
- 단일 `main` 브랜치로 충분
- 실험용으로는 브랜치 사용

**팀용:**
- 안정적인 skill을 위한 `main`
- 개발용 feature 브랜치
- 병합 전 PR 리뷰

### 태그

안정적인 릴리스에 태그를 붙이세요.
```bash
git tag v1.0.0
git push --tags
```

---

## Skill 작성

### 구조

```markdown
---
name: skill-name
description: One-line description
---

# Skill Name

Brief overview.

## When to Use

Clear trigger conditions.

## Instructions

1. Step one
2. Step two

## Examples

Concrete input/output examples.

## When NOT to Use

Explicit exclusions.
```

### License

게시된 skill에는 `license` 필드를 추가하세요 — 기업 환경에서는 특히 중요합니다.

```yaml
---
name: code-review
description: Reviews code for quality
license: MIT
---
```

이는 `skillshare install` 중에 표시되어 사용자가 정보에 기반한 규정 준수 결정을 내릴 수 있게 합니다.

### 콘텐츠

**할 것:**
- 명확하고 실행 가능한 지시문 작성
- 예시 포함
- edge case 명시
- 초점 유지 (skill 하나 = 목적 하나)

**하지 말 것:**
- 모호한 지시문 작성
- 너무 많은 책임 포함
- 오류 처리 잊어버리기
- 테스트 생략

---

## 팀 협업

### Repo 전용 skill에는 project mode(`-p`) 사용

skill이 하나의 코드베이스(아키텍처, 도메인 규칙, 배포 흐름)와 밀접하게 결합되어 있다면 project mode를 선호하세요.

```bash
skillshare init -p
skillshare install <source> -p
skillshare sync
```

**이것이 도움이 되는 이유:**
- **재현 가능한 온보딩**: `.skillshare/config.yaml`은 repo를 clone하는 누구에게나 이동 가능한 skill manifest 역할을 합니다.
- **명확한 범위**: project skill은 global 개인 워크플로로 새어 나가지 않고 `.skillshare/skills/`에 머무릅니다.
- **더 안전한 협업**: 변경 사항이 프로젝트 코드와 함께 일반적인 git PR 흐름을 통해 검토됩니다.
- **커밋 노이즈 감소**: `.skillshare/logs/`는 project mode에서 기본적으로 무시됩니다.

개인의 프로젝트 간 skill에는 global mode를, repo 전용 팀 컨텍스트에는 `-p`를 사용하세요.

### 내부 도구에는 .skillignore 사용

팀 repo에 내부 툴링이나 작업 중인 skill이 있다면, 우발적인 discovery를 방지하기 위해 `.skillignore`를 추가하세요.

```text title=".skillignore"
# 공개 discovery에서 숨김
_internal-scripts
test-*
wip-feature
```

이는 `skillshare install <repo> --all`을 실행하는 외부 기여자나 자동화가 내부 skill을 가져가지 않도록 보장합니다.

**`.skillignore.local`을 사용한 로컬 재정의**: 공유 repo의 `.skillignore`가 로컬에서 필요한 skill을 차단한다면, 공유 파일을 수정하지 않고 재정의하기 위해 같은 디렉터리에 `.skillignore.local`을 만드세요.

```text title="_team-skills/.skillignore.local"
# 내 개인 private skill의 ignore 해제
!private-mine
```

`.skillignore.local`을 `.gitignore`에 추가하세요 — 이것은 로컬에만 머물러야 합니다.

### 소유권

- skill 카테고리에 소유자 지정
- README에 누가 무엇을 관리하는지 문서화
- 병합 전 PR 리뷰

### 문서화

```
team-skills/
├── README.md           # 설정 안내
├── CONTRIBUTING.md     # skill 추가 방법
├── CHANGELOG.md        # 변경 내역
└── skills/
    └── ...
```

### 커뮤니케이션

- 팀 채팅에 새 skill 공지
- breaking change 문서화
- 사용자로부터 피드백 수집

---

## 유지 관리

### 정기 작업

```bash
# 주간
skillshare update --all     # tracked repo 업데이트
skillshare doctor           # 문제 확인
skillshare backup --cleanup # 오래된 스냅샷과 방치된 미완성 디렉터리 제거

# 월간
skillshare list             # 설치된 skill 검토
# 사용하지 않는 항목 제거: skillshare uninstall <name>...
```

### 사용하지 않는 skill 정리

```bash
# 모든 skill 목록
skillshare list

# 사용하지 않는 것 제거
skillshare uninstall unused-skill
skillshare sync
```

### 의존성 업데이트

```bash
# CLI 업데이트
skillshare upgrade --cli

# 내장 skill 업데이트
skillshare upgrade --skill

# tracked repo 업데이트
skillshare update --all
```

---

## 보안

### 민감한 정보

**skill에 절대 넣지 말 것:**
- API 키
- 비밀번호
- 개인 정보
- 내부 URL

**대신:**
- 환경 변수 사용
- 외부 config 참조
- skill을 범용적으로 유지

### 설치 전 검토

서드파티 skill을 install하기 전에:
- source 확인
- SKILL.md 읽기
- 먼저 `--dry-run` 사용

포괄적인 보안 워크플로는 [Skill 보안 강화하기](/docs/how-to/advanced/security) 가이드를 참고하세요.

---

## 체크리스트

### 새 skill

- [ ] 설명적인 이름
- [ ] 명확한 설명
- [ ] 실행 가능한 지시문
- [ ] 예시 포함
- [ ] AI CLI에서 테스트됨
- [ ] 이름 충돌 없음

### 팀 repo

- [ ] 명확한 폴더 구조
- [ ] 설정 안내가 담긴 README
- [ ] 네임스페이스가 적용된 skill 이름
- [ ] 내부 도구를 위한 `.skillignore`
- [ ] PR 리뷰 프로세스
- [ ] CHANGELOG 유지

---

## 참고

- [Skill 만들기](./creating-skills.md) — Skill 생성 가이드
- [Skill 설계](/docs/understand/philosophy/skill-design) — 복잡도 레벨, 결정성, CLI wrapper 패턴
- [Skill 형식](/docs/understand/skill-format) — SKILL.md 레퍼런스
- [조직 전체 Skill](/docs/how-to/sharing/organization-sharing) — 팀 공유 패턴
