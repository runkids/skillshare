---
sidebar_position: 1
---

# 워크플로

skillshare의 일반적인 사용 패턴.

## 워크플로 선택하기

| 하고 싶은 것... | 워크플로 |
|-------------|----------|
| 매일 skill 사용하기 | [일상 워크플로](./daily-workflow.md) |
| AI 도구 간 메모리 공유 | [메모리 공유 가이드](./sharing-memory) |
| 새 skill 찾고 설치하기 | [Skill 발견](./skill-discovery.md) |
| skill 보호하기 | [백업 및 복원](./backup-restore.md) |
| project 범위 skill 관리하기 | [Project 워크플로](./project-workflow.md) |
| 고장 난 것 고치기 | [문제 해결](/docs/troubleshooting) |

---

## 빠른 참조

### 일상 사이클
```bash
# skill 편집 (source 또는 아무 target에서)
$EDITOR ~/.config/skillshare/skills/my-skill/SKILL.md

# 모든 target에 sync (필요한 경우)
skillshare sync

# remote로 push (크로스 머신 sync를 사용하는 경우)
skillshare push -m "Update my-skill"
```

### 발견 사이클
```bash
# skill 검색
skillshare search pdf

# repository 탐색
skillshare install anthropics/skills

# install 및 sync
skillshare install anthropics/skills/skills/pdf
skillshare sync
```

### 안전 사이클
```bash
# 위험한 변경 전
skillshare backup

# 문제가 생기면
skillshare restore claude

# 상태 확인
skillshare doctor
```
