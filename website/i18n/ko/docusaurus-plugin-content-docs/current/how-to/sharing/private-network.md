---
sidebar_position: 5
---

# Machines on a Private Network

내 컴퓨터들이 이미 SSH로 서로 접근할 수 있다면(예: Tailscale, WireGuard, 집 네트워크) skillshare를 여러 머신에서 함께 사용할 수 있습니다. GitHub나 호스팅 서비스가 필요 없습니다.

이 페이지는 세 가지를 다룹니다:

- 호스팅 Git 서비스 대신 [내 머신 중 하나에 있는 저장소를 통해 동기화](#ssh-remote)
- 지금 앞에 있는 머신에서 [다른 머신의 대시보드 열기](#remote-dashboard)
- MCP로 [다른 머신의 Agent에 내 Skill 제공](#remote-skills)

예시에서는 머신 이름을 `mac-mini`와 `laptop`으로 부릅니다. Tailscale MagicDNS 이름처럼 네트워크에서 각 머신에 부여한 이름을 사용하세요.

## 시작하기 전에 {#before-you-start}

비밀번호 프롬프트 없이 SSH가 동작하는지, 그리고 상대 머신에서 skillshare를 찾을 수 있는지 확인하세요:

```bash
ssh you@laptop skillshare version
```

SSH로 실행한 명령은 대화형 셸 프로필을 불러오지 않으므로 skillshare가 설치된 디렉터리를 찾지 못할 수 있습니다. 명령을 찾을 수 없다면 해당 머신에서 `which skillshare`가 출력하는 전체 경로(예: Apple silicon의 Homebrew라면 `/opt/homebrew/bin/skillshare`)를 사용하거나, 비대화형 셸이 읽는 파일(예: `~/.zshenv`)에서 그 디렉터리를 `PATH`에 추가하세요.

## 내 머신 중 하나의 저장소를 통해 동기화 {#ssh-remote}

`push`와 `pull`은 모든 Git remote와 동작하므로, 내 머신 중 하나에 있는 bare repository가 GitHub를 대신할 수 있습니다.

저장소를 보관할 머신에서(동기화하는 머신 중 하나여도 됩니다):

```bash
git init --bare ~/skillshare.git
```

첫 번째 머신에서 SSH로 연결하고 push합니다:

```bash
skillshare init --remote you@mac-mini:skillshare.git
skillshare push
```

`you@mac-mini:skillshare.git`은 해당 사용자의 홈 디렉터리 기준 경로입니다. 전체 경로를 쓴 `ssh://you@mac-mini/home/you/skillshare.git`도 동작합니다. 저장소를 가진 머신에서는 대신 로컬 경로(`~/skillshare.git`)를 지정할 수 있습니다. skillshare가 이미 GitHub 같은 다른 Remote로 설정되어 있으면 `init`은 이를 건드리지 않습니다. `git -C ~/.config/skillshare remote set-url origin you@mac-mini:skillshare.git`로 바꾼 뒤 `skillshare push`를 실행하세요.

새 bare repository는 기본 브랜치를 `master`(또는 `init.defaultBranch`)로 지정하는데, 첫 번째 머신이 push한 브랜치와 다를 수 있습니다. 다른 머신은 기본 브랜치에서 시작하므로, push된 브랜치를 가리키도록 설정하세요:

```bash
git --git-dir ~/skillshare.git branch                            # push된 브랜치, 예: main
git --git-dir ~/skillshare.git symbolic-ref HEAD refs/heads/main
```

이 단계를 건너뛰면 다음 머신의 `init`이 pull할 내용을 찾지 못합니다.

그런 다음 나머지 각 머신에서 같은 `init --remote`를 실행합니다. 이후에는 평소 명령을 그대로 사용합니다:

```bash
skillshare push    # Skill을 변경한 머신에서
skillshare pull    # 다른 머신에서; pull은 sync도 실행합니다
```

다른 머신에 로그인하지 않고 업데이트하려면 SSH로 해당 머신의 `pull`을 실행하세요:

```bash
skillshare push && ssh you@laptop skillshare pull
```

그 외의 모든 내용은 [Cross-Machine Sync](/docs/how-to/sharing/cross-machine-sync)가 그대로 적용됩니다: 첫 pull 동작, 충돌, `push`와 `pull`이 옮기는 항목.

:::tip
이제 bare repository는 내가 편집하는 머신에 없는 유일한 Skill 사본입니다. 다른 데이터와 마찬가지로 백업하거나, 비공개 호스팅 저장소 같은 두 번째 Remote를 추가하세요.
:::

## 다른 머신의 대시보드 열기 {#remote-dashboard}

각 머신에는 자체 대시보드가 있으며, 그 머신만 관리합니다. 다른 곳에서 여는 방법은 두 가지입니다.

### SSH 터널 사용

상대 머신에서는 자기 자신 외에는 아무것도 listen하지 않습니다. 해당 머신의 대시보드를 백그라운드로 시작한 다음, 로컬 포트를 그쪽으로 포워딩합니다:

```bash
ssh you@laptop skillshare ui start
ssh -N -L 19421:127.0.0.1:19420 you@laptop
```

그 대시보드가 이미 사설 주소(아래 참고)에서 실행 중이면 `ui start`는 그 주소에서 실행 중이라고 알려 줍니다. 이 경우 `-L 19421:100.101.102.103:19420`처럼 그 주소로 포워딩하세요.

`http://localhost:19421`을 엽니다. 이 머신에서 이미 `19420`으로 실행 중인 대시보드와 충돌하지 않도록 로컬 포트는 `19421`을 사용합니다.

### 사설 네트워크 주소 사용

그 머신이 사설 네트워크에서 받은 주소(예: Tailscale IP, `tailscale ip -4`)로 대시보드를 시작합니다:

```bash
skillshare ui start --host 100.101.102.103
```

네트워크의 어느 머신에서든 `http://100.101.102.103:19420`을 엽니다. 대시보드는 IP 주소로 접속하면 그대로 응답합니다. 이름으로 열려면 그 이름으로 시작하거나, 입력하는 주소를 정확히 trusted로 지정하세요:

```bash
skillshare ui start --host laptop.example.ts.net
skillshare ui start --host 100.101.102.103 --trusted-origin http://laptop:19420
```

그 밖의 이름은 403을 받으며, 이는 DNS rebinding으로부터 대시보드를 보호합니다. 전체 규칙은 [리버스 프록시](/docs/reference/commands/ui#reverse-proxy)를 참고하세요.

:::warning
대시보드에는 로그인이 없습니다. 주소에 접근할 수 있는 사람은 Agent가 실행하는 명령을 기록하는 MCP, Hooks, Plugins 페이지를 포함해 대시보드 전체를 사용할 수 있습니다. 직접 제어하는 네트워크의 주소에만 바인딩하고, 접근할 수 있는 대상을 제한하세요(예: Tailscale 접근 규칙). 신뢰할 수 없는 네트워크에도 연결되는 머신에서는 `--host 0.0.0.0`을 사용하지 마세요.
:::

대시보드에서 연 에디터는 내가 접속한 머신이 아니라 대시보드가 실행 중인 머신에서 시작됩니다.

## 다른 머신의 Agent에 내 Skill 제공 {#remote-skills}

자체 skillshare 설정이 없는 머신에서 Agent가 실행될 때는, 동기화된 폴더 대신 네트워크를 통해 `skillshare mcp serve`에서 Skill을 읽을 수 있습니다:

```bash
SKILLSHARE_MCP_TOKEN=change-me skillshare mcp serve --http 100.101.102.103:8765 --http-allow-insecure
```

`--http-allow-insecure`는 Tailscale이나 WireGuard처럼 네트워크 자체가 트래픽을 암호화하는 경우에만 사용하세요. 그 외에는 HTTPS로 제공하세요. Agent의 MCP client가 토큰을 bearer token으로 사용해 `http://100.101.102.103:8765/`에 연결하도록 설정합니다. transport 규칙, TLS 옵션, 제공되는 항목은 [Serve skills over MCP](/docs/reference/commands/mcp#serve-skills-over-mcp)를 참고하세요.

## 제한 사항 {#limits}

- `push`와 `pull`은 skills, agents, extras를 옮기며 각 머신의 `config.yaml`은 옮기지 않습니다. Plugins, MCP 서버, hooks는 각 머신에서 따로 설정해야 합니다. [Plugins, MCP, Hooks](/docs/how-to/sharing/cross-machine-sync#plugins-mcp-hooks)를 참고하세요.
- 사용자 이름이 다른 머신은 서로 다른 홈 경로를 저장합니다. `config.yaml`을 다른 방법으로 공유한다면 `preserve_tilde_on_save`를 켜세요([Configuration](/docs/reference/targets/configuration) 참고).
- macOS에서는 SSH로 시작한 명령이 로그인 키체인에 접근하지 못할 수 있습니다. HTTPS remote의 자격 증명 문제로 `ssh you@laptop skillshare pull`이 실패하면, 그 머신에서는 SSH remote나 토큰 환경 변수를 사용하세요.

## 참고 자료

- [Cross-Machine Sync](/docs/how-to/sharing/cross-machine-sync) — 호스팅 Git 서비스를 통한 동기화
- [ui](/docs/reference/commands/ui) — 대시보드 플래그와 백그라운드 서버
- [mcp](/docs/reference/commands/mcp#serve-skills-over-mcp) — MCP로 Skill 제공
- [init](/docs/reference/commands/init) — `--remote`로 init
