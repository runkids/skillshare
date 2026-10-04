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

왼쪽 트리는 중첩 폴더를, 위쪽에 검색 상자가 있습니다. 오른쪽 패널은 **Preview** / **Source**를 전환하며, 긴 노트는 접힌 상태로 열리고 **Show all**로 펼칩니다. 노트 이름 옆에는 **Edit**와 **More actions** 메뉴가 있으며 **Copy file path**, **History**, **Move or rename**, **Delete note**가 들어 있습니다. **Use with agents**는 노트 아래에 있습니다. 기존 노트의 상대 링크는 같은 뷰어에서 열립니다.

![wiki 폴더를 펼친 영어 Memory 뷰어](/img/memory-tree-demo.png)

## 4. 검색, 편집, 복원

**Search names and content**에 `Retrieval`을 입력하세요. 검색은 대소문자를 구분하지 않으며 하위 폴더의 경로와 내용도 포함합니다. 검색을 지우면 모든 노트가 표시됩니다.

![중첩 노트를 표시하는 영어 검색 결과](/img/memory-search-demo.png)

텍스트 편집기도 사용할 수 있습니다. 외부 변경 사항은 dashboard를 새로 고쳐 확인하세요. 편집 중 노트가 바뀌면 오래된 version의 저장을 거부하고 초안을 보존합니다. **Latest saved version**을 비교하고 **Copy draft**로 복사할 수 있습니다. 직접 내용을 비교하거나 병합한 뒤 **Save my draft**를 선택하여 대체를 확인하세요. 갱신된 version으로 저장하고 기존 내용을 백업합니다. 외부 변경이 다시 발생하면 또 충돌이 발생합니다.

![최신 저장 내용과 초안을 함께 표시하는 영어 편집기](/img/memory-conflict-demo.png)

**History**는 노트의 절대 경로로 필터링된 **Backup Files**를 엽니다. 저장된 버전을 미리 보고 복원한 다음 Memory를 새로 고치세요. 삭제된 노트도 같은 페이지에서 복원할 수 있습니다.

![wiki/workflow-check.md의 영어 Backup Files 복원 미리 보기](/img/memory-restore-demo.png)

## 5. 새 에이전트 세션에서 검증

관련 노트에 `memory-check: demo-7429` 같은 임시 값을 추가하고 저장하세요. 연결된 도구에서 새 세션을 시작하세요. **Copy verification prompt**로 다음 프롬프트를 복사할 수 있습니다.

**Copy verification prompt** 위에 마우스를 올리면 복사하기 전에 내용을 미리 볼 수 있습니다.

![English verification prompt tooltip](/img/memory-verification-demo.png)

> 지침에 명시된 공유 메모리 INDEX.md와 이 작업에 관련된 노트를 읽어 주세요. 노트의 전체 경로와 제가 추가한 임시 검증 값을 알려 주세요. 읽기 이벤트를 확인할 수 있도록 파일 읽기 도구를 사용하세요.

실제 읽기 도구 이벤트에서 전체 경로와 임시 값을 확인하세요. 다른 연결 도구에서도 반복하고 임시 값을 제거하세요. 수동 검증이며 Skillshare는 읽기 telemetry를 보장하지 않습니다. 읽었다는 답변이나 **Configured** 표시만으로는 읽기의 증거가 되지 않습니다.

교훈을 저장하려면 배경, 결론, 증거를 `LEARNED.md`에 기록하도록 요청하세요. 두 모드 모두 각 작업 시작 시 `INDEX.md`를 읽습니다. 노트는 사용자 소유입니다. `passive` 안내는 남길 만한 사실을 알려 주되 사용자 요청이 있을 때만 업데이트하게 하고, `active` 안내는 위 규칙에 따라 그런 사실을 도구 자체 메모리 대신 여기에 저장하게 합니다. Native automatic memory, 자동 학습, Obsidian 통합은 활성화하지 않습니다.

## Project mode

프로젝트의 Skillshare 설정을 먼저 초기화하고 실행하세요.

```bash
skillshare extras memory init -p
skillshare ui -p
```

기본 소스는 `.skillshare/extras/memory/`이며 표시되는 설정 디렉터리를 사용하면 `skillshare/extras/memory/`입니다. 기존 extras source overrides도 적용됩니다. 같은 연결, 인덱스, 편집, 복원 기능을 제공합니다. 저장소 내부 소스는 **project root** 기준 상대 경로를 사용합니다. 지침 파일이 하위 폴더에 있어도 동일합니다. 프로젝트 외부 override는 절대 경로를 사용합니다. 절대 소스를 이동하거나 위치를 바꾸면 안내를 다시 생성하고 검토하세요.

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
