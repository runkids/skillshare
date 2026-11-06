---
sidebar_position: 21
---

# project

각 폴더에 `.skillshare/`를 만들지 않고 공유 설정과 로컬 등록으로 프로젝트의 스킬과 agent를 동기화합니다.

`project`는 프로젝트 안에서 실행하더라도 항상 global 설정을 사용합니다. 이는 저장소 자체의 설정을 선택하는 `-p` / `--project`와 다르며, 이 명령어에서 `-p` / `--project`는 거부됩니다. 팀원이 저장소와 함께 설정을 받아야 한다면 [project mode](/docs/understand/project-skills)를 사용하세요.

## 사용법 {#usage}

```bash
skillshare project                          # Same as project list
skillshare project list --json
skillshare project show <project> --json
skillshare project add <root> --target T [options]
skillshare project edit <project> [options]
skillshare project sync <project> [--dry-run] [--json]
skillshare project localize <project> [--dry-run] [--json]
skillshare project remove <project> [--yes] [--dry-run] [--json]
```

`add`에는 존재하는 폴더와 하나 이상의 target이 필요합니다. 나머지 명령어는 유효한 프로젝트 이름(사용자 지정 이름 또는 폴더 basename)이나 루트 경로를 받습니다. 루트를 명시적으로 선택하려면 상대 경로 앞에 `./` 또는 `../`를 붙이세요. 접두사 없는 토큰이 한 프로젝트의 이름과 다른 프로젝트의 루트에 모두 일치하면, 명령어는 하나를 고르지 않고 두 후보를 모두 보고합니다. 심볼릭 링크 별칭은 별도의 등록으로 유지됩니다.

## 플래그 {#flags}

| 플래그 | 적용 대상 | 설명 |
|------|------------|-------------|
| `--target T` | add, edit | Target tool. 반복하거나 쉼표 목록을 사용합니다. add에서는 필수이며, edit에서는 목록 전체를 교체합니다 |
| `--name N` | add, edit | 사용자 지정 이름. `--name=`은 폴더 basename으로 되돌립니다 |
| `--skills`, `--no-skills` | add, edit | skill을 켜거나 끕니다. add에서는 기본적으로 켜져 있습니다 |
| `--agents`, `--no-agents` | add, edit | agent를 켜거나 끕니다. add에서는 기본적으로 꺼져 있습니다 |
| `--skills-mode MODE`, `--agents-mode MODE` | add, edit | `merge`, `symlink`, `copy` 중 하나. 빈 값은 override를 지웁니다 |
| `--skills-naming NAMING` | add, edit | `flat`, `standard`, `prefixed` 중 하나. 빈 값은 상속으로 돌아갑니다 |
| `--skills-include P`, `--skills-exclude P` | add, edit | 여러 glob 패턴은 반복해서 지정합니다. 해당 목록을 교체하며, 빈 값은 목록을 지웁니다 |
| `--agents-include P`, `--agents-exclude P` | add, edit | agent에 대해 같은 교체 규칙을 적용합니다. 필터에는 merge 또는 copy mode가 필요합니다 |
| `--sync` | add, edit | 저장한 뒤 이 프로젝트의 skill과 agent를 동기화합니다. force는 꺼져 있습니다 |
| `--dry-run`, `-n` | add, edit, sync, remove, localize | 쓰기나 확인 없이 미리 봅니다 |
| `--yes` | remove | 확인을 건너뜁니다. 미리보기가 아닌 상태에서 JSON으로 제거할 때 필수입니다 |
| `--json` | all | 구조화된 출력. 프로세스 종료 코드도 함께 확인하세요 |
| `--global`, `-g` | all | global 범위를 명시합니다(이 명령어의 기본값이기도 합니다) |
| `--help`, `-h` | all | 도움말을 표시합니다 |

리소스 설정을 지정하면 해당 리소스가 켜지며, 기존의 `enabled: false`도 해제됩니다. 설정을 생략하면 꺼진 상태가 유지됩니다. edit은 지정한 설정만 변경하며, 기존 리소스를 켜도 그 설정은 유지됩니다. 서로 반대되는 토글이나 `--no-skills --skills-mode copy`처럼 끄기와 설정을 함께 쓰는 조합은 거부됩니다. agent 이름 지정 방식 플래그는 없습니다. [`target`](/docs/reference/commands/target)과 달리, 이 include/exclude 플래그는 개별 패턴을 추가하거나 제거하는 대신 목록을 교체합니다.

## 예시 {#examples}

### 등록, 미리보기, 동기화 {#register-and-sync}

```bash
skillshare project add ~/work/shop-web --target claude,codex \
  --skills-mode copy --skills-include 'frontend-*' --agents --dry-run
skillshare project add ~/work/shop-web --target claude,codex \
  --skills-mode copy --skills-include 'frontend-*' --agents --sync
skillshare project show shop-web --json
```

저장한 등록은 이 기기에 유지됩니다. 공유 [`projects`](/docs/reference/targets/configuration#projects) 선언도 계속 지원됩니다.

skill이 꺼진 프로젝트는 global symlink mode를 포함해 skill 동기화 단계를 전혀 실행하지 않습니다. 요청되었거나 켜진 리소스 선택이 비어 있으면 탐색, 백업, target 쓰기 없이 성공합니다.

### 필터 교체 또는 override 초기화 {#edit-settings}

```bash
skillshare project edit shop-web --skills-include 'frontend-*' --skills-include 'core-*'
skillshare project edit shop-web --skills-exclude= --skills-mode= --name=
skillshare project sync shop-web --dry-run --json
skillshare project sync shop-web --json
```

add와 edit은 쓰기 전에 유효한 전체 설정을 검증합니다. 로컬 편집은 선택한 항목만 복사하므로 공유 YAML 앵커의 다른 루트는 변경되지 않습니다. 변경 결과에는 영향을 받은 루트가 나열됩니다. 아무것도 바뀌지 않는 add나 edit은 `applied: false`와 `unchanged: true`를 반환하며, `--sync`는 그대로 수행합니다. 다른 설정으로 add를 반복하면 실패하고 `project edit`을 안내합니다. 사용자 지정 이름은 루트처럼 보이면 안 됩니다(예: `~name`, `./name`, `C:name`).

### 등록 제거 {#remove}

```bash
skillshare project remove shop-web --dry-run --json
skillshare project remove shop-web --yes --json
```

제거는 등록을 지우기 전에 skill을 분리합니다. 폴더 전체 링크는 제거되고, merge mode 링크는 source 내용이 존재하는 동안 로컬 사본이 되며, copy mode 파일은 남기고 manifest만 제거합니다. 남아 있는 활성 target이 여전히 소유한 경로는 유지됩니다.

CLI edit은 빠진 skill 경로를 같은 규칙으로 분리합니다. 프로젝트의 skill이 꺼진 상태라면 이름 변경이나 agent 설정만 바꾸는 편집을 포함한 모든 CLI edit이 남은 skill 경로도 분리합니다. 앞서 대시보드에서 skill을 껐더라도 동일하며, 복사 복원과 남아 있는 활성 target의 경로 소유권 규칙은 그대로 적용됩니다. 대시보드 저장은 skill을 분리하지 않습니다. Agent 파일은 그대로 남습니다. 미리보기와 결과에는 제거, 복사 복원, 유지된 경로가 나열됩니다.

## JSON과 실패 처리 {#json-and-failures}

| 명령어 | 주요 필드 |
|---------|-------------|
| list | `projects` 배열(등록된 프로젝트가 없으면 비어 있음) |
| show | `project` 객체 |
| add, edit, remove, localize | `action`, `project`, `applied`, `affected`, `warnings`; 선택적으로 `sync` 또는 `detach` |
| sync | `action`, `project`, `sync`; 미리보기에는 `applied: false`도 포함 |

프로젝트 객체에는 `root`(저장된 표기), `path`(확장된 경로), `name`, `targets`, `skills`, `agents`, `groups`, `missing`, `hasOwnConfig`가 포함됩니다. 생략되었거나 꺼진 리소스는 `null`입니다. 켜진 리소스는 해당 설정을 노출합니다. 텍스트 형식의 `project list`와 `project show`는 켜짐/꺼짐 상태를 보고합니다. sync 결과에는 `results`, `failed`, `warnings`, 그리고 탐색/필터 진단 정보가 포함됩니다.

target 하나가 실패하면 sync는 완료된 target을 보고하면서 종료 코드 1로 끝납니다. sync 미리보기는 충돌을 `failed`에 보고하고 종료 코드 0으로 끝나지만, 잘못된 입력은 여전히 실패합니다. add/edit 저장은 성공했지만 sync가 실패하면 등록은 저장된 상태로 남습니다. target을 수정한 뒤 `project sync`로 다시 시도하세요. 없는 프로젝트 폴더도 edit은 할 수 있지만 sync는 할 수 없습니다.

edit과 제거는 트랜잭션이 아닙니다. 이후의 분리나 저장이 실패하면 그 이전의 파일 시스템 변경이 남을 수 있습니다. JSON 결과는 완료된 `detach`, `registration`(`kept`, `updated`, `removed`), `error`를 보고합니다. edit 저장이 실패하면 `applied: false`이며 영속화된 프로젝트를 설명합니다. 저장이 성공한 뒤 sync가 실패하면 `applied: true`가 유지됩니다. 다시 시도하기 전에 이 필드들을 확인하세요. 변경이 일어나기 전의 실패는 오류 payload를 사용합니다. 모든 project 명령은 시작 시 마이그레이션, 정리 및 버전 확인을 건너뜁니다.

이 명령어는 skill과 agent만 관리합니다. MCP 프로젝트 설정과 hook은 각각 전용 명령어를 사용하며, target을 변환하거나 프로젝트 자체 설정을 편집하지 않습니다.

## 로컬 등록과 공유 선언 {#local-registrations}

CLI add/edit/remove와 대시보드 저장은 로컬 등록만 기록하며 공유 설정을 다시 쓰지 않습니다. 공유 `projects:` 선언은 계속 유효합니다. 정규화된 루트가 같으면 로컬 항목이 공유 항목 전체를 대체합니다. 삭제는 이 기기에서만 공유 항목을 숨기며 다시 추가하면 해제됩니다. 서로 다른 루트의 이름 중복과 같은 계층의 정규화된 루트 중복은 거부됩니다.

```bash
skillshare project localize shop-web --dry-run --json
skillshare project localize shop-web --json
```

`localize`는 공유 항목을 로컬로 복사하며 공유 YAML 삭제, 연결 해제 또는 동기화를 하지 않습니다. 반복 실행은 변경하지 않습니다. list/show의 텍스트와 JSON은 `origin`(shared/local)과 `originPath`를 제공합니다. 모든 project 명령은 시작 시 마이그레이션, 정리 및 버전 확인을 건너뜁니다.

저장 위치는 `$XDG_STATE_HOME/skillshare/projects/<identity>.yaml`입니다. 지정하지 않으면 Unix는 `~/.local/state/skillshare/projects`, Windows는 `%LOCALAPPDATA%\skillshare\state\projects`를 사용합니다. 상태 디렉터리는 로컬에 두세요. 식별자는 해석된 설정 파일 대상에 따르므로 설정 심볼릭 링크 별칭은 상태를 공유합니다. 대상을 이동하면 새 등록 영역을 사용합니다. 읽기는 파일을 만들지 않으며 잘못된 내용이나 버전은 거부합니다.

CLI와 대시보드는 로컬 상태 잠금을 공유하고 최대 10초 기다린 후 다시 로드합니다. 연결 해제 및 쓰기 전에 공유 및 로컬 스냅샷을 확인합니다. 다른 공유 설정 작성자는 이 보호 밖에 있습니다. 대시보드 Convert는 명시적으로 공유 설정을 변경하며 이미 선언되었거나 숨겨졌거나 로컬에서 재정의된 루트를 거부합니다. 원시 설정 편집기는 저장 전에 제출 내용과 로컬 항목의 병합 결과를 검증합니다.

## 동시 프로젝트 쓰기 {#concurrency}

CLI와 대시보드는 로컬 상태 잠금을 공유하고 최대 10초 기다린 후 다시 로드합니다. 연결 해제 및 쓰기 전에 공유 및 로컬 스냅샷을 확인합니다. 다른 공유 설정 작성자는 이 보호 밖에 있습니다. 대시보드 Convert는 명시적으로 공유 설정을 변경하며 이미 선언되었거나 숨겨졌거나 로컬에서 재정의된 루트를 거부합니다. 원시 설정 편집기는 저장 전에 제출 내용과 로컬 항목의 병합 결과를 검증합니다.

## 관련 문서 {#related}

- [Many Projects, One Config](/docs/how-to/recipes/many-projects-one-config)
- [프로젝트 설정 필드](/docs/reference/targets/configuration#projects)
- [MCP 프로젝트 설정](/docs/reference/commands/mcp#manage-several-projects-from-the-global-config)
