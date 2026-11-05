---
sidebar_position: 6
---

# Playground에서 skillshare 체험하기

> 데모 Skill, Audit 규칙, 프로젝트가 미리 구성된 Docker 샌드박스 — 몇 초 만에 둘러볼 수 있습니다.

## 사전 준비

- Docker와 Docker Compose 설치
- skillshare 저장소 클론: `git clone https://github.com/runkids/skillshare.git`

## Playground 시작하기

```bash
cd skillshare
make playground
```

이 명령어 하나로:

1. 샌드박스 Docker 이미지를 빌드합니다 (Go 툴체인 포함)
2. 컨테이너 내부에서 `skillshare` 바이너리를 컴파일합니다
3. 모든 Target이 자동 감지된 상태로 Global mode를 초기화합니다
4. 여러 카테고리에 걸친 데모 Skill(clean, warning, critical)을 생성합니다
5. 프로젝트 단위 Skill과 커스텀 Audit 규칙을 갖춘 데모 프로젝트를 설정합니다
6. 대화형 셸로 진입시킵니다 — 바로 둘러볼 수 있습니다

## 내부 구성

### 데모 Skill (Global)

| Skill | 카테고리 | Audit 결과 |
|-------|----------|----------------|
| `audit-demo-clean` | root | 없음 (깨끗한 기준선) |
| `deploy-checklist` | `devops/` | 없음 |
| `audit-demo-ci-release` | `security/` | HIGH + MEDIUM (sudo, 외부 URL) |
| `audit-demo-debug-exfil` | `security/` | CRITICAL (자격 증명 유출) |
| `audit-demo-external-link` | `security/` | LOW (외부 URL) |
| `audit-demo-dangling-link` | `security/` | LOW (누락된 링크) + HIGH (루트 이탈) |

### 데모 프로젝트 (`~/demo-project`)

미리 구성된 `.skillshare/` 프로젝트로 다음을 포함합니다:
- `hello-world` — 깨끗한 프로젝트 Skill
- `demos/audit-demo-release` — Audit 경고가 있는 릴리스 헬퍼
- `guides/code-review` — 중첩된 코드 리뷰 가이드
- TODO/FIXME 정책 규칙이 포함된 커스텀 `audit-rules.yaml`

### 커스텀 Audit 규칙

Global 및 Project 레벨의 `audit-rules.yaml`이 모두 미리 구성되어 있어, 규칙을 커스터마이징하는 방법을 직접 확인할 수 있습니다 — 규칙 활성화/비활성화, 커스텀 패턴 추가, 허용 목록 설정.

## 시도해볼 것들

```bash
# 설치된 항목 확인
skillshare status
skillshare list

# 보안 Audit 실행 — 심각도별 결과 확인
skillshare audit

# Project mode 시도
cd ~/demo-project
skillshare status          # Project mode 자동 감지
skillshare audit           # 커스텀 규칙을 사용한 프로젝트 단위 스캔

# 웹 대시보드 실행 (포트 19420)
skillshare-ui              # Global mode
skillshare-ui-p            # Project mode

# 중첩된 Skill 탐색
ls ~/.config/skillshare/skills/security/
ls ~/.config/skillshare/skills/devops/
```

## Bare 모드

깨끗한 상태로 시작 — 자동 초기화도, 데모 콘텐츠도 없습니다:

```bash
./scripts/sandbox_playground_up.sh --bare
./scripts/sandbox_playground_shell.sh
```

`skillshare init`을 처음부터 테스트할 때 유용합니다.

## Playground 중지

```bash
make playground-down
```

데이터는 Docker 볼륨(`playground-home`)에 유지됩니다. 다음에 `make playground`를 실행하면 이전 상태에서 이어집니다.

## 아키텍처

Playground는 보안이 강화된 **읽기 전용(read-only)** Docker 컨테이너에서 실행됩니다:

- `read_only: true` — 지정된 볼륨을 제외한 파일 시스템이 변경 불가능합니다
- `cap_drop: ALL` — Linux capability가 전혀 없습니다
- `no-new-privileges` — 권한 상승을 방지합니다
- 쓰기 가능한 볼륨: `/sandbox-home` (영구), `/tmp` (tmpfs, 256 MB)
- 웹 대시보드용 포트 `19420` 포워딩

워크스페이스는 호스트 저장소에서 읽기 전용으로 마운트됩니다 — 사용자의 머신에서 코드를 수정하고 컨테이너 내부에서 다시 빌드할 수 있습니다.

## 다음 단계는?

- [시작하기 →](/docs/getting-started)
- [보안 Audit 가이드 →](/docs/how-to/advanced/security)
- [Docker 샌드박스 가이드 →](/docs/how-to/advanced/docker-sandbox)
