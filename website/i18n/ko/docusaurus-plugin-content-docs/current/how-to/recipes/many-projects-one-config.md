---
sidebar_position: 8
---

# Recipe: Many Projects, One Config

> 한 번의 sync로 global 설정에서 여러 프로젝트 폴더에 Skill과 MCP 서버를 보내세요.

## 시나리오 {#scenario}

`~/.claude/skills`와 같은 Global target은 모든 프로젝트에서 읽히므로, 모든 프로젝트가 같은 skill을 보게 됩니다. 그것이 원하는 바라면 이 recipe는 필요하지 않습니다.

이 recipe는 프로젝트마다 **다른** 것을 받아야 할 때를 위한 것입니다:

- 설치된 skill이 많은데 frontend 프로젝트에는 `frontend-*`만 필요합니다. 세션의 skill이 적을수록 설명에 쓰이는 context가 줄고 잘못 선택할 가능성도 낮아집니다.
- 클라이언트의 저장소처럼, 다른 곳에서는 문제없는 MCP 서버를 특정 프로젝트 하나에는 로드하면 안 됩니다.
- 프로젝트가 Skillshare 설정은 커밋하지 않으면서, skill의 실제 사본을 커밋할 수 있어야 합니다.
- 사용하는 tool이 프로젝트 내부의 폴더 하나만 읽습니다.

[Project mode](/docs/how-to/recipes/skill-per-project-workflow)도 프로젝트마다 고유한 세트를 제공합니다. 모든 프로젝트에 `.skillshare/config.yaml`을 두고, 각 폴더 안에서 sync합니다. global 설정의 `projects`는 여러분의 머신에 있는 파일 하나로 같은 결과를 냅니다:

| | Global targets | Global `projects` | Project mode |
|---|---|---|---|
| **Skill을 받는 대상** | 모든 프로젝트, 동일한 세트 | 나열한 폴더들, 폴더마다 별도 세트 | 해당 프로젝트 하나 |
| **설정이 저장되는 위치** | 내 머신 | 내 머신 | 프로젝트 저장소 |
| **팀원도 받는지** | 아니요 | 아니요 | 예, clone하면 받음 |
| **프로젝트에 추가되는 파일** | 없음 | 동기화된 skill과 agent만 | `.skillshare/`와 동기화된 파일 |
| **Sync** | 어디서든 `sync` 한 번 | 어디서든 `sync` 한 번 | 각 프로젝트 안에서 `sync` |

설정이 저장소와 함께 이동해야 한다면 project mode를 선택하세요. 여러분 자신의 프로젝트, `.skillshare/`를 추가할 수 없는 클라이언트나 오픈소스 저장소, 그리고 한 번의 `sync`로 모두 업데이트하고 싶을 때는 `projects`를 선택하세요.

## 해결 방법

### Skill과 Agent: `projects`

```yaml
# ~/.config/skillshare/config.yaml
projects:
  ~/work/project01:
    targets: [claude, codex]
    skills:
      mode: copy
      include:
        - myskill-*
    agents: {}
```

```bash
skillshare sync --dry-run   # 미리보기
skillshare sync
```

- `targets`는 해당 프로젝트에서 사용하는 tool을 지정합니다. Skillshare는 각 tool의 프로젝트 경로(여기서는 `.claude/skills`와 `.agents/skills`)에 기록하므로 따로 입력할 경로가 없습니다.
- `skills`와 `agents`는 해당 부분을 켜는 스위치입니다. 비워 두면 모든 것을 동기화하며, `include`와 `exclude`로 범위를 좁힐 수 있습니다. [Filtering skills](/docs/how-to/daily-tasks/filtering-skills)를 참고하세요.
- `copy`는 실제 파일을 작성하므로 프로젝트에서 커밋할 수 있습니다. 심볼릭 링크로 충분하다면 기본값인 `merge`를 그대로 두세요.

대시보드에서도 동일한 작업을 할 수 있습니다: **프로젝트** 페이지에서 **프로젝트 추가**로 target을 선택하고 동기화할 항목을 고르세요. 모든 필드는 [`projects`](/docs/reference/targets/configuration#projects)를 참고하세요.

### MCP 서버: `mcp.projects`

MCP 서버는 각 Agent의 자체 설정 파일에 작성되므로, 경로가 아니라 프로젝트 폴더 단위로 나열합니다:

```yaml
# ~/.config/skillshare/config.yaml
mcp:
  servers:
    context7:
      command: npx
      args: ["-y", "@upstash/context7-mcp"]
      targets: [opencode]
  projects:
    ~/work/project01:
      servers:
        context7:            # 다른 곳에서는 로드되고, 여기서만 꺼짐
          disabled: true
          targets: [opencode]
```

```bash
skillshare sync mcp --dry-run   # 모든 파일 미리보기
skillshare sync mcp
```

필드와 제한 사항은 [`mcp`: manage several projects](/docs/reference/commands/mcp#manage-several-projects-from-the-global-config)를 참고하세요.

## 검증

- `skillshare sync`가 프로젝트의 target들을 보고함. 예: `project01@claude  1 copied`
- `~/work/project01/.claude/skills/`에 `include`와 일치하는 Skill만 들어 있음
- `skillshare sync mcp --dry-run`이 프로젝트 파일마다 한 줄씩 나열함
- `skillshare sync mcp`를 다시 실행하면 모든 항목이 `unchanged`로 보고됨

## 변형

- **Tool 경로 밖의 폴더**: Target은 이름과 경로일 뿐이므로, 어떤 tool의 프로젝트 경로도 해당하지 않는 폴더에는 `skillshare target add project01 ~/work/project01/some/folder`가 여전히 사용됩니다. 이런 Target이 실제로 어떤 tool의 프로젝트 경로를 가리키면, 대시보드의 **프로젝트** 페이지가 이를 변환하도록 제안합니다.
- **커밋 또는 무시**: `copy` 모드에서는 Skillshare가 복사한 내용을 추적하기 위해 Target 폴더에 `.skillshare-manifest.json`도 작성합니다. Skill과 함께 커밋하거나 `.gitignore`에 추가하세요.
- **경로 중복 경고**: 프로젝트 폴더가 다른 Target이 이미 사용하는 폴더이면 `sync`가 경로 중복(path overlap) 경고를 출력합니다. 어떤 Target이 같은 폴더를 공유하는지 보려면 `skillshare doctor`를 실행하세요.
- **여러 프로젝트에 같은 서버**: 한 프로젝트 아래에 YAML anchor(`docs: &docs`)로 한 번 정의하고 다른 프로젝트에서 재사용하세요(`docs: *docs`). [`mcp` reference](/docs/reference/commands/mcp#manage-several-projects-from-the-global-config)를 참고하세요.
- **공유 프로젝트**: 프로젝트를 clone한 팀원은 여러분의 global 설정을 받지 못합니다. 설정이 저장소와 함께 이동해야 한다면 [project mode](/docs/how-to/recipes/skill-per-project-workflow)를 사용하세요.

## 관련 문서

- [`target` command reference](/docs/reference/commands/target)
- [`mcp` command reference](/docs/reference/commands/mcp)
- [Sharing MCP servers](/docs/how-to/daily-tasks/sharing-mcp)
