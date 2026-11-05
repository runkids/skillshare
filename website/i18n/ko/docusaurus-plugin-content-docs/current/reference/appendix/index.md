---
sidebar_position: 1
---

# 부록

skillshare 기술 레퍼런스 부록입니다.

## 무엇을 찾고 계신가요?

| 주제 | 참고 |
|-------|------|
| skillshare에 영향을 주는 환경 변수 | [Environment Variables](/docs/reference/appendix/environment-variables) |
| skillshare가 설정, Skill, 로그, 캐시를 저장하는 위치 | [File Structure](/docs/reference/appendix/file-structure) |
| 지원되는 git URL 형식 | [URL Formats](/docs/reference/appendix/url-formats) |
| 설정 파일 형식 및 옵션 | [Configuration](/docs/reference/targets/configuration) |
| 모든 CLI 명령어 | [Commands](/docs/reference/commands) |

## 빠른 참조

### 주요 경로 (Unix)

| 경로 | 용도 |
|------|------|
| `~/.config/skillshare/config.yaml` | 설정 파일 |
| `~/.config/skillshare/skills/` | Source 디렉터리 (사용자의 Skill) |
| `~/.config/skillshare/skills/.metadata.json` | 설치된 Skill 메타데이터 (자동 관리) |
| `~/.local/share/skillshare/backups/` | 백업 디렉터리 |
| `~/.local/share/skillshare/trash/` | 소프트 삭제된 Skill |
| `~/.local/state/skillshare/logs/` | 작업 및 Audit 로그 |
| `~/.local/state/skillshare/projects/` | 머신 로컬 프로젝트 등록 및 잠금 |
| `~/.cache/skillshare/ui/` | 다운로드된 웹 대시보드 |

### 환경 변수

| 변수 | 용도 |
|----------|---------|
| `SKILLSHARE_CONFIG` | 설정 경로 재정의 |
| `GITHUB_TOKEN` | GitHub API 인증 |

## 참고

- [Configuration](/docs/reference/targets/configuration) — 설정 파일 상세
- [Commands](/docs/reference/commands) — 모든 명령어
