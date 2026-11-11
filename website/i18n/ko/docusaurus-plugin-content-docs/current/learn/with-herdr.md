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

### Mod: 현재 레이아웃 유지 (권장)

독립 [Mod 패키지](https://github.com/runkids/skillshare/tree/main/herdr/skillshare-mod)를 사용하면 전체 창을 교체하지 않고 패널을 추가할 수 있습니다. Herdr 안의 에이전트에 다음 프롬프트를 붙여 넣습니다.

```text
Read ~/.config/herdr-app/AGENTS.md, then add this mod from https://github.com/runkids/skillshare/tree/main/herdr/skillshare-mod. Follow its RECIPE.md: add only the skillshare toggle and panel to my current layout, preserving my status bar, usage meters and all other customizations. Reuse existing skillshare nodes instead of duplicating them.
```

이 패키지에는 `layouts/` 폴더가 없습니다. 레시피는 skillshare 버튼과 패널만 추가하거나 재사용하며, 상태 표시줄, CLI 사용량 표시, 테마와 다른 사용자 설정을 보존합니다. Herdr 기본 레이아웃은 복제한 뒤 수정합니다.

### Herdr layouts에서

이 레이아웃은 [Herdr layouts](https://herdr.dev/layouts/runkids/skillshare--herdr--skillshare/)에 등록되어 있습니다. Herdr 안의 에이전트에 다음 프롬프트를 붙여 넣습니다.

```text
Read ~/.config/herdr-app/AGENTS.md, then add this layout: herdr.dev/layouts/runkids/skillshare--herdr--skillshare
```

에이전트는 모든 파일을 읽은 뒤 레이아웃을 추가하며, 기존 레이아웃은 그대로 유지됩니다. 레이아웃은 `ctrl-shift-l`로 전환합니다.

### 직접 설치

패널은 skillshare 저장소의 [`herdr/skillshare`](https://github.com/runkids/skillshare/tree/main/herdr/skillshare)에 있습니다.

1. `mods/skillshare/`를 `~/.config/herdr-app/mods/skillshare/`에 복사합니다.
2. `layouts/herdr-skillshare.json`과 `layouts/herdr-skillshare.png`를 `~/.config/herdr-app/layouts/`에 복사한 뒤, Herdr 설정에서 `herdr-skillshare` 레이아웃으로 전환합니다. 기존 레이아웃을 유지하려면 폴더의 `README.md`에 나온 두 블록을 추가합니다.
3. `herdr-app check ~/.config/herdr-app/mods/skillshare`를 실행합니다.

탭 바에 **skillshare** 버튼이 나타나 패널을 표시하거나 숨깁니다. space를 전환해도 패널의 열림 상태는 그대로 유지됩니다.

## 패널 업데이트

어느 방법으로 설치했든 에이전트로 업데이트할 때는 설치된 skillshare mod만 백업하고 교체합니다. 현재 선택된 레이아웃, 모든 레이아웃, 사용자 상태 표시줄, 사용량 표시, 설정, 키 바인딩과 다른 mod를 보존하세요. 레이아웃 조정이 필요해도 전체 레이아웃을 다시 적용하지 말고 skillshare 자체 노드만 병합합니다.

패널은 표시되는 동안 한 시간에 한 번 GitHub에 최신 skillshare 릴리스를 확인합니다. 새 릴리스가 게시되면 패널 헤더에 **Update panel**이 나타납니다. 먼저 확인을 받은 뒤 `~/.config/herdr-app/mods/skillshare/`의 파일을 해당 릴리스에 포함된 파일로 교체하고, Herdr가 패널을 다시 불러옵니다. 교체된 파일은 그 폴더의 `.bak-update-<version>`에 보관됩니다. 그 파일에 직접 한 변경은 유지되지 않습니다. 레이아웃은 바뀌지 않습니다.

이 버튼이 생기기 전에 설치한 패널은 업데이트를 확인하지 않습니다. 최신 릴리스의 `mods/skillshare/`를 `~/.config/herdr-app/mods/skillshare/`에 한 번 직접 덮어써 주세요. 이후 업데이트는 버튼으로 할 수 있습니다.

## 패널 기능

패널은 Herdr가 보여 주는 space를 따라가며, 그 space의 머신에서 포커스된 터미널의 폴더를 기준으로 skillshare를 실행합니다.

- **Overview**는 먼저 답을 보여 줍니다. "Everything is in sync"이거나, 동기화가 필요한 타깃의 수와 Sync 버튼 하나입니다. 그 아래에는 source에 있는 종류별 개수와 각 타깃으로 이어지는 줄이 있어 어느 타깃이 대기 중인지 알 수 있습니다. 탭의 나머지에는 처리가 필요한 것만 나열합니다. 동기화를 기다리는 항목, 업데이트가 있는 항목, hooks 알림입니다. `.skillshare/config.yaml`이 있는 프로젝트에서는 탭 옆의 All · Project · Global로 범위를 고릅니다.
- **Library**는 skills, agents, MCP 서버, plugins, hooks를 "Needs sync", "Update available", "In sync"로 나누고, 비활성화된 항목은 "Off"로 묶어 보여 줍니다. skills와 agents는 "In sync"에서 같은 저장소의 항목을 접을 수 있는 그룹으로 묶으며, List로 바꾸면 목록으로 보여 줍니다.
- **행**을 열면 미리보기가 나타납니다. skill은 `SKILL.md`와 파일을, MCP 서버는 비밀 값을 가린 설정을 보여 주고, 미리보기의 Targets 탭은 타깃별 상태를 보여 줍니다. skills와 agents에는 Update, Diff, Disable(비활성화된 항목은 Enable, 다음 sync에서 적용), Uninstall, 그리고 확인 후 해당 항목을 Herdr에서 포커스된 에이전트에 넘기는 "Ask &lt;agent&gt; to audit…"가 있습니다. 최상위에 선언된 hooks는 확인 후 Enable, Disable, Remove를 제공하며, 저장된 변경은 Sync로 적용합니다.
- hooks 개수에는 이름이 있는 항목과 정리 대기 항목이 포함되며, 진단 알림은 별도의 행으로 표시됩니다. hooks 충돌은 CLI나 대시보드에서 해결할 때까지 모든 리소스의 동기화를 차단합니다. 계획을 읽을 수 없으면 항목은 그룹 밖에서 동기화 상태를 알 수 없음으로 표시됩니다.
- **Install**은 저장소 URL을 받습니다. skills가 여러 개인 저장소는 목록에서 고를 수 있습니다. 패널은 클론에 [`skillshare audit`](/docs/reference/commands/audit)을 실행하고 통과한 경우에만 설치합니다. audit에 막히면 findings와 함께 "Install anyway"(`--force`)와 "Ask &lt;agent&gt; to review…"를 보여 줍니다.
- **Source**는 source 저장소의 상태를 보여 주며, 대화상자에서 Pull, Commit and push, Sync both ways를 제공합니다. 두 머신이 모두 바꾼 파일 때문에 pull이 멈추면, 좌우 비교 미리보기에서 파일마다 남길 버전을 고릅니다. `.metadata.json`은 [`skillshare pull`](/docs/reference/commands/pull)과 같은 방식으로 항목별로 병합됩니다.
- **Machines**는 Herdr 각 머신의 skillshare 버전, remote, source, 타깃 상태와 doctor 경고를 보여 주고, Pull and sync와 새 릴리스가 있을 때 Update를 제공합니다. 커밋하지 않았거나 push하지 않은 변경이 있는 머신에는 일반 pull이 거부되므로 대신 Sync both ways를 제공합니다.

패널은 표시되는 동안 20초마다 새로 고치고 10분마다 `skillshare check`를 실행하며, 숨겨져 있을 때는 아무것도 하지 않습니다.

## 관련 문서

- [Sync](/docs/reference/commands/sync)
- [Push](/docs/reference/commands/push)와 [Pull](/docs/reference/commands/pull)
- [Audit](/docs/reference/commands/audit)
