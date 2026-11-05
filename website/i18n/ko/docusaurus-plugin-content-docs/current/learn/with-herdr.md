---
sidebar_position: 8
---

# Herdr에서 skillshare 사용하기

> 에이전트 옆에 skillshare 패널을 두고, [Herdr](https://herdr.dev)를 벗어나지 않고 업데이트나 sync가 필요한 항목을 확인하고 바로 처리합니다.

![Herdr의 skillshare 패널](/img/herdr-panel-demo.png)

## 사전 요구 사항

- Herdr 0.1.12 이상
- Herdr가 연결하는 모든 머신에 skillshare가 설치되어 있고, `~/.local/bin`, `~/go/bin`, `/opt/homebrew/bin`, `/usr/local/bin` 중 한 곳에 있을 것

## 설정

패널은 skillshare 저장소의 [`herdr/skillshare`](https://github.com/runkids/skillshare/tree/main/herdr/skillshare)에 있습니다.

1. `mods/skillshare/`를 `~/.config/herdr-app/mods/skillshare/`에 복사합니다.
2. `layouts/herdr-skillshare.json`과 `layouts/herdr-skillshare.png`를 `~/.config/herdr-app/layouts/`에 복사한 뒤, Herdr 설정에서 `herdr-skillshare` 레이아웃으로 전환합니다. 기존 레이아웃을 유지하려면 폴더의 `README.md`에 나온 두 블록을 추가합니다.
3. `herdr-app check ~/.config/herdr-app/mods/skillshare`를 실행합니다.

탭 바에 **skillshare** 버튼이 나타나 패널을 표시하거나 숨깁니다. space를 전환해도 패널의 열림 상태는 그대로 유지됩니다.

## 패널 기능

패널은 Herdr가 보여 주는 space를 따라가며, 그 space의 머신에서 포커스된 터미널의 폴더를 기준으로 skillshare를 실행합니다.

- **Resources**는 skills, agents, MCP 서버, plugins, hooks를 "Updates available", "Needs sync", "In sync"로 나누어 보여 줍니다. `.skillshare/config.yaml`이 있는 프로젝트에서는 All · Project · Global로 범위를 고릅니다.
- **행**은 그 자리에서 펼쳐집니다. skills와 agents에는 Update, Diff, Uninstall, 그리고 확인 후 해당 항목을 Herdr에서 포커스된 에이전트에 넘기는 "Ask &lt;agent&gt; to audit…"가 있습니다. 최상위에 선언된 hooks는 확인 후 Enable, Disable, Remove를 제공하며, 저장된 변경은 Sync로 적용합니다.
- hooks 개수에는 이름이 있는 항목과 정리 대기 항목이 포함되며, 진단 알림은 같은 필터를 따르고 항목이 0개여도 별도로 표시됩니다. 파일이 동기화되어도 비활성 이유를 표시하며, 중첩 행에는 전체 프로젝트 경로를 표시합니다. 네이티브 파일만 변경된 경우에도 Sync를 제공하며, hooks 충돌은 CLI나 대시보드에서 해결할 때까지 모든 리소스의 동기화를 차단합니다. 미리보기에 실패하면 항목은 그룹 밖에서 동기화 상태를 알 수 없음으로 표시되며, 범위별 오류 알림을 하나 표시합니다.
- **Install**은 저장소 URL을 받습니다. skills가 여러 개인 저장소는 목록에서 고를 수 있습니다. 패널은 클론에 [`skillshare audit`](/docs/reference/commands/audit)을 실행하고 통과한 경우에만 설치합니다. audit에 막히면 findings와 함께 "Install anyway"(`--force`)와 "Ask &lt;agent&gt; to review…"를 보여 줍니다.
- **Source**는 source 저장소의 상태를 보여 주며, 대화상자에서 Pull, Commit and push, Sync both ways를 제공합니다. 두 머신이 모두 바꾼 파일 때문에 pull이 멈추면, 좌우 비교 미리보기에서 파일마다 남길 버전을 고릅니다. `.metadata.json`은 [`skillshare pull`](/docs/reference/commands/pull)과 같은 방식으로 항목별로 병합됩니다.
- **Machines**는 Herdr 각 머신의 skillshare 버전, source 상태, doctor 경고를 보여 주고, Pull and sync와 새 릴리스가 있을 때 Update를 제공합니다.

패널은 표시되는 동안 20초마다 새로 고치고 10분마다 `skillshare check`를 실행하며, 숨겨져 있을 때는 아무것도 하지 않습니다.

## 관련 문서

- [Sync](/docs/reference/commands/sync)
- [Push](/docs/reference/commands/push)와 [Pull](/docs/reference/commands/pull)
- [Audit](/docs/reference/commands/audit)
