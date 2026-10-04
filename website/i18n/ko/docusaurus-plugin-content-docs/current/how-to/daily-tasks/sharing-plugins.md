---
sidebar_position: 9
---

# 여러 도구에서 플러그인 관리하기

플러그인에는 Skill, MCP 연결, 훅, 스크립트 등 함께 작동하는 여러 파일이 포함될 수 있습니다.
Skillshare는 그 패키지를 온전히 유지하면서 어떤 도구가 그것을 받을지 선택할 수 있게 해줍니다.
시작하기 위해 YAML을 작성할 필요는 없습니다.

## 첫 번째 플러그인 추가하기

대시보드에서 **Plugins → Add plugin**을 여세요:

1. GitHub 저장소(`owner/repo`), HTTPS Git URL, 또는 로컬 디렉터리를 붙여넣습니다.
2. Source에 여러 개가 있으면 플러그인을 선택한 다음 호환되는 도구를 선택합니다.
3. 변경 사항을 검토하고 적용합니다.

![Add plugin 대화 상자: 발견된 plugin과 호환·미지원 target](/img/plugins-add-dialog.png)

대부분의 사용자는 저장소와 Target 체크박스만 있으면 됩니다. **Advanced options**에서는
릴리스를 선택할 수 있는 Git ref를 추가합니다. Discovery는 각 Target의 구성 요소와
호환성을 개별적으로 보여줍니다. OpenCode의 항목을 감지하지 못해 지원되지 않는다고
표시된 경우, 해당 행의 **Set entry path**를 사용하면 빌드된 파일을 가져와 이미 선택한
내용을 유지한 채 Source를 다시 검색합니다. 안전한 상대 저장소 심볼릭 링크는
보존됩니다.

동일한 안내형 흐름은 터미널에서도 사용할 수 있습니다:

```bash
skillshare plugin add
```

Claude Code, Codex, Copilot, Antigravity CLI, Grok, Pi, 또는 OpenCode CLI는
**Skillshare 백엔드가 실행되는 곳**에 설치되어 있어야 합니다.
Codex는 Codex 데스크톱 app에 포함된 CLI도 인정됩니다. Codex가 다른 위치에 있는 머신에서는 [`SKILLSHARE_CODEX_CLI`](/docs/reference/appendix/environment-variables#skillshare_codex_cli)를 설정하세요.
Cursor와 Antigravity 데스크톱은 대신 로컬 플러그인 디렉터리에 완전한 파일을 받습니다.
컨테이너 안에서 실행되는 대시보드는 호스트 머신에만 설치된 플러그인을 관리할 수
없습니다. 로컬 CLI를 사용하거나, 네이티브 클라이언트 옆에서 Skillshare를 실행하세요.

설치 Target은 **Claude Code, Codex, Cursor, Antigravity Desktop, Antigravity CLI,
GitHub Copilot CLI, Pi, OpenCode**입니다. Grok은 import 전에 네이티브 설치와 신뢰
설정이 필요합니다. Kimi, Hermes, Devin 형식은 검색은 가능하지만 자동 설치는 불가능하며
인터페이스가 그 이유를 설명해줍니다.
Target에 맞게 배포된 형식을 선택하세요. Skillshare는 도구 간에 플러그인을 변환하지
않습니다. 프로젝트 모드는 Claude, Antigravity Desktop, Pi, OpenCode를 지원합니다.
Antigravity Desktop(`antigravity` 또는 `agy`)과 CLI(`antigravity-cli`)는 별도의
저장소를 사용합니다. 실행하는 것을 선택하세요.

## 이미 무언가를 설치했다면?

**Import installed**를 선택하고 네이티브 설치를 선택하세요. Import는 재설치하거나,
인증 정보를 복사하거나, 해당 Agent 안에서의 활성화 여부를 변경하지 않고 그저
기록만 합니다. Import는 Claude, Codex, Copilot, Antigravity CLI, Grok, Pi, OpenCode에서
사용할 수 있으며, Cursor와 Antigravity 로컬 패키지에는 **Add plugin**을 사용하세요.

```bash
skillshare plugin import review@team --from claude --no-tui
```

논리적으로 하나의 패키지가 도구별로 서로 다른 네이티브 배포판을 사용한다면, 각
배포판을 동일한 `--name`과 해당 Target으로 추가/import하세요. Skillshare는 표시
이름으로부터 동등성을 추론하지 않습니다.

Import는 이 머신의 네이티브 설치에 묶여 있습니다. 같은 plugin을 다른 머신에서 쓰려면
대신 source에서 추가하세요. [Cross-Machine Sync — Plugins](/docs/how-to/sharing/cross-machine-sync#plugins)를 참고하세요.

## 동기화 대상 선택하기

관리되는 각 바인딩에는 체크박스가 있습니다. 체크박스는 **이 Target을 sync에 포함**
한다는 의미이지, "Agent 안에서 활성화"한다는 의미가 아닙니다.

- 체크한 다음 동기화하면 누락된 플러그인이 설치됩니다.
- 플러그인의 행을 열면 그 Source가 패키지를 가진 다른 Agent들도 체크되지 않은
  상태로 나열됩니다. 하나를 체크하면 설치 미리보기가 열립니다. Source가 제공할 수
  없는 Agent는 행 끝에 개수로 표시되며, 그 개수를 클릭하면 이유가 열립니다.
- Agent를 하나도 체크하지 않고 플러그인을 추가할 수 있습니다. Skillshare에 남아 **아직 Agent 없음**으로 표시되며, 행에서 Agent를 체크하기 전까지 아무것도 설치되지 않습니다.
- 체크 해제한 다음 동기화하면 해당 관리 설치가 제거됩니다.
- 패키지 정의는 그대로 남아있으므로 나중에 다시 해당 Target을 선택할 수 있습니다.
- Claude나 Codex 안에서 비활성화된 플러그인은 비활성화 상태를 유지합니다. 해당
  도구의 네이티브 설정에서 관리하세요.

대시보드에서, Plugins 페이지 오른쪽 상단의 **Sync** 박스는 다음 sync에서 각
Agent에 대해 무엇이 설치되거나 제거될지 나열합니다. 버튼을 누르면 미리보기가
열리며, 확인하기 전까지는 Agent에서 아무것도 바뀌지 않습니다. 플러그인 목록은
즉시 표시되고, 박스 아래의 **Agents** 열은 각 Agent의 CLI가 응답하는 대로
채워집니다.
실행 후 박스에는 결과가 표시됩니다. 실패가 먼저 오고 다른 변경이 뒤따르며, 같은 결과로 끝난 Agent는 한 행에 묶입니다. 바뀌지 않은 항목은 **Unchanged** 한 줄로 접히고, 펼치면 plugin마다 한 행씩, Pi 패키지는 따로 표시됩니다. 미리보기도 같은 방식으로, 적용할 변경 아래에 바뀌지 않는 항목을 접어 둡니다. 목록에는 Pi가 설치한 패키지에서 읽은 모든 Pi 패키지의 설치된 버전이 표시됩니다.

```bash
skillshare plugin disable review --target codex --no-tui
skillshare sync plugins --dry-run
skillshare sync plugins --no-tui
```

플러그인의 메뉴에는 **View files**가 있습니다: 렌더링된 Markdown과 함께 제공되는
읽기 전용 검토된 로컬 복사본입니다. Import된 플러그인은 로컬 복사본이 없으므로 이
항목도 없습니다.

플러그인은 일반 Skill 및 MCP 동기화와는 별개입니다. 이들의 번들 구성 요소는
독립적인 Skillshare Source에도 복사되지 않습니다.

## 업데이트와 복구 {#updates-and-recovery}

**Check updates**를 사용한 다음 지원되는 Target의 업데이트를 검토하세요. Claude는
네이티브 CLI를 통해 업데이트할 수 있습니다. Codex는 검토된 스냅샷에서 플러그인을 다시
add합니다. 단, Codex에서 비활성화된 플러그인은 add하면 다시 활성화되므로 업데이트하지
않습니다. Import한 Codex 플러그인은 마켓플레이스를 업그레이드하여 업데이트합니다. Cursor와 Antigravity는 로컬 편집을 확인한 후 관리되는 로컬
복사본을 교체합니다. Pi와 OpenCode는 검토된 스냅샷을 업데이트합니다. Copilot은 알려진
활성화 상태를 보존하면서 검토된 Source를 새로고침할 수 있습니다. Antigravity CLI와
Grok의 업데이트는 네이티브 도구 안에 머무릅니다. Import된 패키지의 제한 사항은
명령어 레퍼런스를 참고하세요. 업데이트가 처리할 수 없는 Target은 이유와 함께
건너뛰며, 해당 플러그인의 다른 Agent는 그대로 업데이트됩니다.

Pi의 npm 패키지에는 비교할 source가 없으므로 **Check updates**는 설치된 버전을 npm의 최신 버전과 비교합니다. 새 버전이 발견되면 행에 `old → new`와 **Update** 버튼이 표시되고, 확인 결과에도 발견된 plugin마다 버튼이 있습니다. 둘 다 먼저 그 plugin의 업데이트 미리보기를 엽니다.

한 Target이 실패하면 결과는 성공한 부분을 그대로 유지합니다. 네이티브 클라이언트의
인증이나 설정 문제를 해결한 다음 해당 Target을 다시 동기화하세요:

```bash
skillshare sync plugins review --target claude --no-tui
```

스냅샷은 Skillshare가 소유합니다. 외부에서 그 내용이 편집되었다면 Skillshare는
그 편집 내용을 먼저 보존할 수 있도록 교체를 차단합니다. Source 다이제스트가 변경을
감지하지만, 모든 네이티브 설치나 import된 마켓플레이스가 여러 머신에서 재현 가능함을
보장하지는 않습니다.

자동화, 스코프 세부 사항, 모든 플래그는 [plugin](/docs/reference/commands/plugin)을
참고하세요.
