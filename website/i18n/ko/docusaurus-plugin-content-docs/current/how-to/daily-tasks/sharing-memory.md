---
sidebar_position: 12
---

# AI 도구 간 메모리 공유

결정, 교훈, 프로젝트 배경을 하나의 Markdown 노트 폴더에 보관하세요. 도구에 읽기 안내를 연결한 다음 새 세션에서 에이전트가 관련 노트를 읽는지 확인하세요.

이 가이드는 global mode를 사용하며 Claude와 Codex가 targets로 설정되어 있습니다.

```bash
skillshare ui -g
```

모든 스크린샷의 UI, 노트, 대화 상자는 영어입니다. `/tmp/skillshare-memory-docs` 아래의 격리된 데모 home을 사용하므로 실제 경로는 다릅니다.

## 1. 메모리 생성

**Extras → Memory**를 열고 **Create memory**를 클릭하세요.

![영어 Memory 탭의 초기 상태](/img/memory-empty-demo.png)

Targets가 없는 `memory` extra를 등록하고 없는 시작 노트를 생성합니다. `INDEX.md`는 짧은 진입점이며 `LEARNED.md`는 교훈의 날짜, 배경, 결론, 증거를 기록합니다. 기존 노트와 설정은 보존합니다.

![INDEX.md와 LEARNED.md를 표시하는 Memory](/img/memory-starters-demo.png)

기본 global 폴더는 `~/.config/skillshare/extras/memory/`이며 **Extras → Folders & files**에도 표시됩니다. 처음에는 targets가 없습니다. 같은 컴퓨터의 에이전트는 이 소스를 직접 읽으므로 노트 사본을 각 도구에 sync할 필요가 없습니다.

## 2. 도구 연결

**Use with agents**에서 **Connect to agents**를 클릭하세요. 도구를 직접 선택하고 각각의 업데이트 모드를 고른 뒤 **Review changes**를 클릭하세요.

- `passive`(기본값): 에이전트가 노트를 읽고, 요청할 때만 업데이트합니다.
- `active`: 명시한 선호, 이유가 있는 결정, 확인된 함정처럼 이후 세션에서도 쓸 사실을 스스로 저장합니다. 일회성 세부 사항과 추측은 건너뛰고, 확실하지 않으면 노트를 제안해 동의를 기다리며, 중복 대신 기존 노트를 업데이트하고 저장한 내용을 알려 줍니다.

같은 파일을 읽는 도구는 블록 하나를 공유하므로 하나를 바꾸면 함께 바뀝니다. 설정된 도구의 모드를 바꾸려면 **Connect to agents**를 다시 열어 전환하세요. 변경은 같은 검토를 거칩니다.

![영어 연결 대화 상자의 지침 파일 변경 미리 보기](/img/memory-connect-demo.png)

각 파일의 diff(삭제된 줄은 `−`, 추가된 줄은 `+`)를 검토하고 **Apply changes**를 클릭하세요. Skillshare는 도구의 기존 지침 파일 또는 도구가 이미 읽는 공유 소스에 관리되는 읽기 안내 블록을 추가합니다. 파일이 없으면 생성할 수 있습니다. 블록에는 scope와 내용 hash 마커가 있으며 다른 내용, 기존 할당, 연결 모드는 보존합니다. 기존 파일은 변경 전에 백업합니다. 다른 도구도 같은 파일을 읽거나 알려진 글자 수 제한이 있으면 검토 화면에 경고가 표시됩니다.

![설정된 도구를 표시하는 영어 Memory 탭](/img/memory-connected-demo.png)

**Configured**는 도구의 읽기 경로에 현재 안내가 있다는 뜻이며 에이전트가 읽었다는 뜻은 아닙니다. **Not configured**, **Outdated**, **Needs attention**은 지침 파일의 상태입니다. 수정되지 않은 오래된 블록은 다시 검토하여 업데이트할 수 있습니다. 수동으로 수정되었거나 마커가 잘못된 블록은 보존하며 직접 수정해야 합니다. 서로 다른 파일에서 두 모드의 블록을 모두 읽는 도구도 조치가 필요합니다. 해당 파일을 읽는 도구를 같은 모드로 설정하세요. 여러 파일에서 블록을 읽는 도구는 남는 블록을 삭제해야 모드를 바꿀 수 있습니다. 동기화되지 않은 공유 지침은 먼저 sync하세요. 읽을 수 없는 지침 파일은 건너뜁니다. 검토 후 파일이 변경되면 적용 전에 다시 검토해야 합니다.

**Open AGENTS.md**에서 지침을 확인하거나 수정하세요. 연결 검토는 dashboard 기능이며 새 CLI 연결 명령은 없습니다.

## 3. 노트와 인덱스 링크 추가

**New note**를 클릭하고 **File name**에 `wiki/architecture.md`를 입력하세요. **Link from INDEX.md**를 선택한 상태로 **Create**를 클릭하세요. 이 체크박스는 `INDEX.md`를 읽을 수 있을 때 표시되며 기본으로 선택되어 있습니다.

![중첩 경로와 인덱스 옵션을 표시하는 영어 New note 대화 상자](/img/memory-folder-demo.png)

없는 하위 폴더를 만들고 `INDEX.md` 끝에 상대 Markdown 링크를 추가합니다. 인덱스 업데이트는 version을 확인하고 변경 내용을 백업합니다. 링크 추가가 실패해도 노트는 유지되며 부분 완료 경고가 표시됩니다. 인덱스에 없는 노트를 선택하고 **Add to INDEX**를 클릭하여 재시도하세요. 인덱스는 직접 편집할 수도 있습니다. 짧게 유지하세요. 깨진 링크는 경고로 표시되며 자동 제거하지 않습니다.

노트는 UTF-8 `.md` 파일이어야 하며 최대 1 MiB입니다. 지원하지 않는 노트는 **Unsupported file** 표시와 함께 목록에 남고 다른 유효한 노트는 계속 사용할 수 있습니다. 소스 내부의 숨김 파일, 숨김 폴더, 심볼릭 링크는 제외합니다.

노트를 선택하고 **Edit**를 클릭하여 다음 영어 데모 내용을 입력한 후 **Save**하세요.

```markdown
# Architecture decisions

## Shared memory

Claude and Codex read the same Markdown notes from Skillshare.
Keep durable decisions here and verify facts that may have changed.

## Retrieval

Read INDEX.md first, then only the notes relevant to the current task.
Update notes when the user asks you to remember a decision.
```

![저장된 영어 노트의 Markdown 미리 보기](/img/memory-note-demo.png)

왼쪽 트리는 중첩 폴더를, 위쪽에 검색 상자가 있으며, 그 옆의 **Refresh** 버튼은 에이전트가 대시보드 밖에서 노트를 쓴 뒤 목록과 열려 있는 노트를 다시 불러옵니다. 오른쪽 패널은 **Preview** / **Source**를 전환하며, 긴 노트는 접힌 상태로 열리고 **Show all**로 펼칩니다. 노트 이름 옆에는 **Edit**와 **More actions** 메뉴가 있으며 **Copy file path**, **History**, **Move or rename**, **Delete note**가 들어 있습니다. **Use with agents**는 노트 아래에 있습니다. 기존 노트의 상대 링크는 같은 뷰어에서 열립니다.

![wiki 폴더를 펼친 영어 Memory 뷰어](/img/memory-tree-demo.png)

## 4. 검색, 편집, 복원

**Search names and content**에 `Retrieval`을 입력하세요. 검색은 대소문자를 구분하지 않으며 하위 폴더의 경로와 내용도 포함합니다. 검색을 지우면 모든 노트가 표시됩니다.

![중첩 노트를 표시하는 영어 검색 결과](/img/memory-search-demo.png)

텍스트 편집기도 사용할 수 있습니다. 외부 변경 사항은 dashboard를 새로 고쳐 확인하세요. 편집 중 노트가 바뀌면 오래된 version의 저장을 거부하고 초안을 보존합니다. **Latest saved version**을 비교하고 **Copy draft**로 복사할 수 있습니다. 직접 내용을 비교하거나 병합한 뒤 **Save my draft**를 선택하여 대체를 확인하세요. 갱신된 version으로 저장하고 기존 내용을 백업합니다. 외부 변경이 다시 발생하면 또 충돌이 발생합니다.

![최신 저장 내용과 초안을 함께 표시하는 영어 편집기](/img/memory-conflict-demo.png)

**History**는 노트의 절대 경로로 필터링된 **Backup Files**를 엽니다. 저장된 버전을 미리 보고 복원한 다음 Memory를 새로 고치세요. 삭제된 노트도 같은 페이지에서 복원할 수 있습니다.

![wiki/workflow-check.md의 영어 Backup Files 복원 미리 보기](/img/memory-restore-demo.png)

## 메모리 시험과 정리

**Use with agents → 메모리 도구**에서 **새 대화에서 시험**을 선택하세요.

1. 첫 프롬프트를 현재 대화에 붙여 넣으세요. 이 소스의 `INDEX.md`에 임시 `memory-check` 행을 추가하고 다른 내용은 보존합니다.
2. 같은 도구나 다른 연결된 도구에서 새 대화를 열고 두 번째 프롬프트를 붙여 넣으세요. 소스 경로는 있지만 검증 값은 없습니다. 실제 읽기 이벤트와 전체 경로를 확인하고 반환한 값을 1단계와 비교하세요.
3. 정리 프롬프트로 테스트 행만 제거하세요. 이 브라우저 탭에서는 대화 상자를 다시 열거나 페이지를 새로고침해도 값이 유지됩니다. 브라우저 저장소를 쓸 수 없다면 정리가 끝날 때까지 페이지를 열어 두세요.

버튼은 프롬프트만 복사합니다. Skillshare는 에이전트를 실행하거나 통과 여부를 표시하지 않습니다. 명시적 읽기는 데이터 검색을 검증하며 네이티브 자동 로딩의 증거는 아닙니다.

**이번 대화 회고**는 중복, 모순, 오래된 정보, 잘못된 색인 링크, 날짜·이유·출처가 없는 결정을 검토하는 프롬프트를 복사합니다. 에이전트는 경로, 증거, 수정 전후 내용을 제안합니다. 대화에서 구체적인 변경을 승인한 뒤에만 쓰기·이동·삭제합니다. 모르는 날짜와 없는 증거는 명시합니다.

Dashboard의 Memory 영역에서도 같은 **메모리 도구** 대화 상자를 열 수 있습니다. 정리 프롬프트를 돌아보고 싶은 대화에 붙여 넣으세요. 현재 대화, 확인 가능한 도구 결과, 사용자가 제공한 세션 기록만으로 수정과 반복된 실수에서 재사용할 교훈을 제안합니다. [retro 절차](https://github.com/mattpocock/skills/blob/main/skills/engineering/retro/SKILL.md)를 참고해 기존 문서와 검사를 먼저 확인하고 기계적 오류에는 자동 검사를 우선하며 영향 순서로 제안합니다. 승인된 메모 변경은 표시된 메모리 소스로 한정하고 프로젝트 검사나 지침 수정에는 별도 승인이 필요합니다. Skillshare는 비공개 대화 기록을 가져오거나 이 검토를 실행하지 않습니다.

**Dream**은 여러 대화에서 저장한 노트와 교훈을 비교해 반복되는 패턴을 찾고 중복, 오래된 정보, 모순, 끊어진 링크의 정리를 제안합니다. 여러 세션의 출처가 있는 노트만으로도 분석할 수 있으며 원본 transcript는 필수가 아닙니다. 증거가 부족하면 결론의 한계를 명시합니다. [Agent Memory Repo](https://cognition.com/agent-memory-repo)를 참고하며 사용 중인 호스트 에이전트가 실행합니다.

세 탭은 inline입니다. 전체 프롬프트는 기본적으로 접혀 있고 **전체 프롬프트 보기**로 펼칩니다. **Agent용 복사**는 소스 경로와 프로젝트／공유 범위를 포함해 복사합니다. 호스트에 업데이트된 내장 Skillshare skill이 설치되어 있다면 이번 대화 검토나 지정 노트의 Dreaming을 직접 요청할 수 있습니다. Skill은 해석된 소스를 확인하고 구체적인 경로와 변경을 승인용으로 제안한 뒤 version 검사와 백업으로 저장합니다. 새 journal이나 필수 transcript archive는 필요 없습니다. 호스트에서 예약한 검토는 제안만 반환합니다.

메모리 도구는 소스 범위가 확인될 때까지 비활성화되며 Project 모드에는 프로젝트 경로도 필요합니다. 회고와 Dream 프롬프트 모두 version 검사와 백업이 있는 Skillshare CLI를 사용하고 소스 불일치, CLI 사용 불가 또는 충돌 시 저장을 중단하도록 요청합니다.

업데이트된 읽기 지침은 반복된 수정이나 모순된 노트가 나타날 때 대화 중 한 번만 짧게 검토를 제안합니다. 검토를 자동으로 시작하지 않습니다. Skillshare는 에이전트를 시작하거나 예약하지 않습니다.

두 모드 모두 결정과 교훈에 날짜, 이유, 확인 가능한 출처나 증거를 기록하고 쓰기 성공 후 저장 내용, 프로젝트／공유 범위, 파일 경로를 한 줄로 보고하도록 요청합니다. 예: “저장: 이 프로젝트 테스트는 devcontainer에서 실행. Project memory: testing.md.” 일반 Markdown을 사용하며 metadata schema는 필요하지 않습니다. 편집기에도 같은 안내를 표시합니다.

기존 도구는 **Connect to agents → Review changes → Apply changes**로 지침을 업데이트합니다. 온전한 오래된 블록은 변경을 검토할 수 있고 수동 수정된 블록은 유지합니다. 도구나 기존 메모를 조용히 업데이트하지 않습니다.


교훈을 저장하려면 배경, 결론, 증거를 `LEARNED.md`에 기록하도록 요청하세요. 두 모드 모두 각 작업 시작 시 `INDEX.md`를 읽습니다. 노트는 사용자 소유입니다. `passive` 안내는 남길 만한 사실을 알려 주되 사용자 요청이 있을 때만 업데이트하게 하고, `active` 안내는 위 규칙에 따라 그런 사실을 도구 자체 메모리 대신 여기에 저장하게 합니다. 한두 문장으로 끝나는 사실은 `INDEX.md`의 `## Notes` 아래 항목 하나로 바로 적고, 긴 노트만 별도 파일로 만듭니다. Native automatic memory, 자동 학습, Obsidian 통합은 활성화하지 않습니다.

## Project mode

프로젝트의 Skillshare 설정을 먼저 초기화하고 실행하세요.

```bash
skillshare extras memory init -p
skillshare ui -p
```

기본 소스는 `.skillshare/extras/memory/`이며 표시되는 설정 디렉터리를 사용하면 `skillshare/extras/memory/`입니다. 기존 extras source overrides도 적용됩니다. 같은 연결, 인덱스, 편집, 복원 기능을 제공합니다. 저장소 내부 소스는 **project root** 기준 상대 경로를 사용합니다. 지침 파일이 하위 폴더에 있어도 동일합니다. Project 안내는 이 프로젝트에 관한 노트는 여기에, 사용자 자신이나 도구, 다른 프로젝트에 관한 노트는 여기에 속하지 않는다고 에이전트에게 알려 주므로, shared memory 안내도 읽는 에이전트는 각 사실을 어디에 저장할지 알 수 있습니다. 프로젝트 외부 override는 절대 경로를 사용합니다. 절대 소스를 이동하거나 위치를 바꾸면 안내를 다시 생성하고 검토하세요.

## 고급 대안: 안내를 직접 복사

**Copy guidance**를 열어 `passive` 또는 `active`를 고르고, 복사한 블록을 에이전트가 읽는 지침 파일에 붙여 넣으세요. **Open AGENTS.md**에서 기존 편집기를 열 수 있습니다.

![영어 Copy guidance 미리 보기](/img/memory-guidance-demo.png)

**Extras → AGENTS.md**에서 공유 지침을 만들고 해당 페이지의 기존 연결 절차를 사용할 수도 있습니다. [도구 간 하나의 AGENTS.md 공유](./sharing-instructions.md)의 연결 모드와 대체 경고를 확인하세요.

![연결된 도구를 표시하는 영어 공유 지침 페이지](/img/memory-agents-demo.png)

Scope와 hash 마커를 유지하세요. 생성된 본문을 직접 수정하면 수정됨으로 표시하고 이후 연결 검토에서도 보존합니다.

## CLI 대안

```bash
skillshare extras memory init -g
printf '# Architecture decisions\n\nRead relevant notes on demand.\n' |
  skillshare extras memory write wiki/architecture.md --from - -g
skillshare extras memory list --search architecture -g
skillshare extras memory show wiki/architecture.md -g
skillshare extras memory instructions -g
skillshare extras memory instructions --update-mode active -g
```

CLI는 읽기 안내(`--update-mode active`를 지정하지 않으면 `passive`)만 출력하므로 직접 붙여 넣어야 합니다. 도구 연결이나 인덱스 링크 추가는 하지 않습니다. 노트 업데이트에는 현재 `--version`이 필요합니다. [`extras memory` 참조](../../reference/commands/extras.md#extras-memory)를 확인하세요.

## 노트 이름 변경 및 이동

노트를 선택하고 **More actions**를 연 다음 **Move or rename**을 누른 뒤 새 상대 `.md` 경로를 입력하세요. `wiki/architecture.md` → `wiki/design.md`는 이름 변경이고, `projects/design.md`로 변경하면 다른 폴더로 이동합니다. 없는 폴더는 자동으로 생성합니다. **Move**로 적용하세요.

![영어 Move or rename 대화 상자에서 새 폴더 경로 지정](/img/memory-move-demo.png)

내용과 권한을 보존합니다. 대상이 이미 있거나 version이 오래되면 거부합니다. 이동 전 원래 경로를 백업하며 **Restore in Backup Files**에서 기록을 볼 수 있습니다. 거기서 복원하면 원래 노트를 다시 만들고 이동한 노트도 남습니다.

Markdown 링크는 노트 안의 상대 링크까지 자동으로 변경하지 않습니다. `INDEX.md`와 다른 노트의 링크를 직접 수정하세요. 깨진 인덱스 링크는 뷰어 위에 표시됩니다. Agent 지침은 source 루트의 `INDEX.md`를 가리키므로 해당 위치를 유지하세요.

## 노트 삭제

**More actions**나 편집기에서 **Delete note**를 선택하고 파일 이름을 확인하세요. 저장되지 않은 편집은 버립니다. 저장된 version을 확인하고 백업한 다음 선택한 노트만 삭제합니다. 폴더와 다른 노트는 보존합니다. `INDEX.md`의 오래된 링크는 직접 업데이트하세요. 삭제 후 **Restore in Backup Files**에서 필터링된 기록을 열 수 있습니다.

![영어 노트 삭제 확인](/img/memory-delete-demo.png)

CLI도 방금 읽은 version이 필요합니다.

```bash
version=$(skillshare extras memory show wiki/architecture.md --json -g | jq -r '.version')
skillshare extras memory delete wiki/architecture.md --version "$version" -g
```

복원하려면 [`backup files`](../../reference/commands/backup.md)에서 `skillshare backup files show <absolute-note-path>`를 실행한 후 `skillshare backup files restore <absolute-note-path> <id>`를 실행하세요.
