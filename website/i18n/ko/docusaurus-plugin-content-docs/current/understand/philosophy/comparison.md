---
sidebar_position: 9
---

# Skill 관리 방식 비교

**Vercel의 `npx skills`**와 **Skillshare**는 모두 여러 AI 도구에서 skills를 관리합니다. 차이는 “하나는 설치만, 다른 하나는 동기화”가 아닙니다. Skills를 관리할지, 더 넓은 AI 작업 환경을 관리할지에 따라 선택하세요.

이 비교는 배포된 **skills v1.8.1**의 [패키지 정보](https://registry.npmjs.org/skills/1.8.1)와 [공식 CLI 문서](https://github.com/vercel-labs/skills/blob/e781cb4d07433dfe1d19e924ed45cc61738c55ec/docs/commands.md)를 기준으로 합니다. 실험적 명령은 변경되거나 사라질 수 있습니다. 모든 설치 도구에 대한 일반화가 아니라 특정 버전의 비교입니다.

## 아키텍처 개요

### npx skills: Skills 발견과 관리

[skills.sh](https://skills.sh)는 skills를 찾는 출발점입니다. CLI는 여러 agent, 프로젝트 및 전역 설치, 공통 복사본을 가리키는 symlink 또는 독립 복사본, 업데이트 확인과 업데이트를 지원합니다. Agent 선택과 확인은 플래그로 지정할 수 있으므로 대화형 입력은 필수가 아닙니다.

설치된 skills는 lock 파일에도 기록됩니다. 실험적 명령은 프로젝트 복원과 npm 기반 동기화를 지원하므로 상태가 없거나 설치 전용이라고 설명하면 정확하지 않습니다.

### Skillshare: 도구 간 작업 환경 관리

Skillshare는 source 리소스와 target 설정을 분리한 뒤 각 도구에 동기화합니다. Skills 외에도 agent 정의, rules 같은 extras, MCP 연결 설정, 지원 클라이언트의 네이티브 hooks를 관리합니다. 로컬 Web dashboard에서 시각적으로 관리할 수 있습니다.

차이는 이러한 리소스를 도구, 머신, 팀 워크플로우 전반에서 함께 관리하는 데 있습니다. Symlink나 여러 agent 설치가 독점 기능이라는 뜻은 아닙니다. 리소스 형식과 클라이언트 기능이 다르므로 모든 리소스가 모든 도구에서 작동하지는 않습니다.

## 기능 비교

| 기능 | npx skills v1.8.1 | Skillshare |
|------|------------------|------------|
| **발견** | skills.sh 및 `skills find` | CLI 검색 및 로컬 Web dashboard |
| **여러 agent와 범위** | 여러 agent, 프로젝트 및 전역 설치 | 설정된 targets, 프로젝트 및 전역 모드 |
| **Skill 파일 공유** | 공통 복사본을 가리키는 symlink, copy 옵션 | Source 디렉터리와 merge, copy, 전체 디렉터리 symlink 모드 |
| **업데이트** | `skills check`(업데이트나 로컬 변경이 있으면 종료 코드 1) 및 `skills update` | `skillshare check` 및 `skillshare update` |
| **프로젝트 복원** | 실험적 `skills experimental_install`(소스 없이 `skills install`로도 실행)이 `skills-lock.json`의 지원 소스를 복원 | 선언적 skill 설정과 source 리소스. [manifest](/docs/understand/declarative-manifest) 참고 |
| **npm 프로젝트 동기화** | 실험적 `skills experimental_sync`가 설치된 의존성과 `package.json`의 skill 선언을 읽음 | Source에서 target으로 동기화. 동일한 npm 의존성 워크플로우는 아님 |
| **머신 간 워크플로우** | 프로젝트 lock 복원과 npm 동기화로 프로젝트 skills 재구성 지원 | Git `push` / `pull`, 범위는 `git_root`로 결정 |
| **Skills 외 리소스** | 인용한 CLI 문서는 skills 중심 | Agent 정의, rules 같은 extras, MCP 설정, 지원 클라이언트의 hooks |
| **로컬 관리와 복구** | 인용한 CLI 문서에는 로컬 dashboard나 target 백업 명령이 명시되어 있지 않음 | `skillshare ui`, target 백업과 복원, 관리 파일 복구 |
| **실행 환경** | Node.js 및 npm/npx | Go 바이너리. Git 작업에는 Git 필요 |

### 실험적 명령의 범위

- **`experimental_install`**(소스 없이 `skills install`로도 실행)은 지원하는 lock 항목을 `.agents/skills/`에 복원하고 agent 디렉터리에 링크합니다(copy 모드로 기록된 skills는 복사). 대상은 `--agent`로 지정한 agent이며, 지정하지 않으면 마지막으로 선택한 agent, 그것도 없으면 감지된 agent입니다. npm 의존성은 설치하지 않습니다. 이미 설치되어 있고 파일이 lock의 콘텐츠 hash와 일치하는 skills는 다시 가져오지 않고 빠진 agent 링크만 추가합니다. 내용이 다른 skills는 소스에서 복원하며, `--force`는 모든 skills를 재설치합니다. Frozen-lock 모드는 콘텐츠 hash를 검사하며 CI에서 기본값입니다.
- **`experimental_sync`**는 설치된 npm 의존성에 포함된 skills를 연결하고 `package.json`의 skill 선언을 처리합니다. `skills-lock.json`으로 추적하고 더 이상 제공되지 않는 skills를 제거하며 별도로 추가한 skills는 보존합니다. Copy를 사용하면 의존성 업데이트 후 다시 동기화해야 합니다.

이는 실제 동기화 및 복원 기능이지만 AI 도구 설정 전체의 백업은 아닙니다.

## 적절한 도구 선택

**npx skills 유지**: 주된 목적이 공개 또는 비공개 skills를 찾고 설치하고 업데이트하는 것이라면 여러 agent를 사용하더라도 합리적입니다. Skills가 npm 프로젝트 의존성을 따르기를 원하고 실험적 워크플로우를 수용할 수 있는 경우에도 적합합니다.

**Skillshare 선택**: 예를 들어 “Claude Code와 Codex를 쓰는데, 관리 중인 skills, rules, MCP 설정, 지원되는 hooks를 다른 컴퓨터로 가져가고 싶다”는 경우입니다. Source/target 설정, Git 워크플로우, 로컬 dashboard와 복구 도구는 이러한 더 넓은 관리 요구를 위한 것입니다.

이식성에도 준비가 필요합니다. Git 범위를 선택하고 새 머신의 targets를 설정하며 자격 증명과 외부 의존성을 별도로 준비해야 합니다. `push`는 기본적으로 skills source만 처리하며 모든 설정을 자동 게시하거나 머신 전체를 백업하지 않습니다. [Git root 범위](/docs/reference/commands/push)와 [sync](/docs/reference/commands/sync)를 참고하세요.

## Skillshare로 마이그레이션

잘 작동하는 설치 도구를 바로 대체할 필요는 없습니다. Skillshare가 관리할 리소스만 이전하고, 같은 관리 대상 skill을 두 도구에서 동시에 업데이트하거나 제거하지 마세요.

현재 마이그레이션은 정보를 모두 보존하는 lock 가져오기가 아닙니다. 수집한 skills는 원래 저장소의 추적 정보가 없는 로컬 복사본입니다. 이후 Skillshare로 업데이트하려면 기록된 소스에서 다시 설치하세요. 관리권을 넘기기 전에 백업과 미리보기를 하고, 수집만으로 완료했다고 생각하지 말고 [npx skills 마이그레이션 가이드](/docs/how-to/advanced/migration)를 따르세요.

## 참고

- [마이그레이션](/docs/how-to/advanced/migration)
- [핵심 개념](/docs/understand)
- [백업](/docs/reference/commands/backup)
