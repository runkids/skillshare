---
sidebar_position: 5
---

# Dev Container에서 skillshare 사용하기

> VS Code에서 열면 Skill이 바로 준비됩니다 — 로컬 설치가 필요 없습니다.

## 사전 준비

- [VS Code](https://code.visualstudio.com/)와 [Dev Containers 확장](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers)

## 동작 방식

VS Code Dev Containers를 사용하면 Docker 컨테이너 안에서 개발할 수 있습니다. `.devcontainer/`에 환경을 정의해두면, VS Code가 나머지를 처리합니다 — 프로젝트를 열고 "Reopen in Container"를 클릭하면 모든 준비가 끝납니다.

skillshare는 이 워크플로우에 자연스럽게 맞아 들어갑니다. `postCreateCommand`에 추가해두면 컨테이너가 시작될 때 Skill이 설치되고 Sync됩니다.

## 설정

`.devcontainer/devcontainer.json`에 다음 두 가지를 추가하세요:

```json
{
  "postCreateCommand": "curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh && export PATH=\"$HOME/.local/bin:$PATH\" && skillshare init --no-copy --all-targets --no-skill && skillshare sync"
}
```

이 PATH 설정은 해당 설정 명령에만 적용됩니다. 이후 터미널에서도 사용할 수 있도록 컨테이너 안의 shell 설정 파일(예: `~/.bashrc`)에 `export PATH="$HOME/.local/bin:$PATH"`를 추가하세요.

이게 전부입니다. 팀원이 VS Code에서 프로젝트를 열고 "Reopen in Container"를 클릭하면:

1. skillshare가 자동으로 설치됩니다
2. `init`이 비대화형으로 실행됩니다 — 감지된 모든 AI CLI Target을 추가하고, 복사 여부를 묻는 프롬프트와 내장 Skill 설치를 건너뜁니다
3. `sync`가 모든 Target에 Skill을 전달합니다

## 프로젝트 Skill 추가하기

팀이 공유하는 Skill을 위해서는 `.skillshare/` 설정을 저장소에 커밋하세요:

```bash
# 컨테이너 내부에서
skillshare init -p
skillshare install your-org/team-skills -p
```

그런 다음 커밋하고, 프로젝트 Skill도 Sync하도록 `postCreateCommand`를 업데이트하세요:

```json
{
  "postCreateCommand": "curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh && export PATH=\"$HOME/.local/bin:$PATH\" && skillshare init --no-copy --all-targets --no-skill && skillshare sync && skillshare sync -p"
}
```

이제 모든 팀원이 컨테이너를 열 때 동일한 Skill을 받게 됩니다.

## GitHub Codespaces

동일한 `.devcontainer/` 설정이 변경 없이 Codespaces에서도 작동합니다. Codespaces는 VS Code와 동일한 방식으로 `postCreateCommand`를 실행합니다.

## ssenv를 이용한 격리 테스트

devcontainer 내부에서 `ssenv`를 사용하면 병렬 테스트를 위한 격리된 skillshare 환경을 만들 수 있습니다. 각 환경은 별도의 설정, Skill, Target을 갖는 독립된 `HOME` 디렉터리를 가집니다.

| 명령어 | 동작 |
|---------|-------------|
| `ssnew <name>` | 새로운 격리 환경 생성 |
| `ssuse <name>` | 환경 전환 |
| `ssback` | 원래 환경으로 복귀 |
| `ssls` | 모든 환경 목록 표시 |
| `ssrm <name>` | 환경 삭제 |

```bash
ssnew demo && ssuse demo    # 생성 후 전환
ss init && ss sync          # 격리된 환경에서 명령어 실행
ssback                      # 원래 환경으로 복귀
```

기존 설정에 영향을 주지 않고 설정 변경이나 Skill 설치를 테스트할 때 유용합니다.

## 다음 단계는?

- [프로젝트 Skill 설정하기 →](/docs/how-to/sharing/project-setup)
- [팀 공유하기 →](/docs/how-to/sharing/organization-sharing)
- [Sync 모드 설명 →](/docs/understand/philosophy/sync-modes-explained)
