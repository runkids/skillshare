---
sidebar_position: 2
---

# 환경 변수

skillshare가 인식하는 모든 환경 변수입니다.

## 설정

### SKILLSHARE_CONFIG

설정 파일 경로를 재정의합니다.

```bash
SKILLSHARE_CONFIG=~/custom-config.yaml skillshare status
```

**기본값:** `~/.config/skillshare/config.yaml`

---

### XDG_CONFIG_HOME

[XDG Base Directory Specification](https://specifications.freedesktop.org/basedir/latest/)에 따라 기본 설정 디렉터리를 재정의합니다.

```bash
export XDG_CONFIG_HOME=~/my-config
# skillshare는 ~/my-config/skillshare/를 사용합니다
```

**기본 동작:**

| 플랫폼 | 기본값 |
|----------|---------|
| Linux | `~/.config/skillshare/` |
| macOS | `~/.config/skillshare/` |
| Windows | `%AppData%\skillshare\` |

**우선순위:** `SKILLSHARE_CONFIG` > `XDG_CONFIG_HOME` > 플랫폼 기본값.

:::note
초기 설정 이후 `XDG_CONFIG_HOME`을 설정한 경우, 기존 `~/.config/skillshare/` 디렉터리를 새 위치로 직접 이동해야 합니다.
:::

---

### XDG_DATA_HOME

데이터 디렉터리(백업, 휴지통)를 재정의합니다.

```bash
export XDG_DATA_HOME=~/my-data
# skillshare는 ~/my-data/skillshare/backups/ 및 ~/my-data/skillshare/trash/를 사용합니다
```

**기본값:** `~/.local/share/skillshare/`

---

### XDG_STATE_HOME

상태 디렉터리(작업 로그)를 재정의합니다.

```bash
export XDG_STATE_HOME=~/my-state
# skillshare는 ~/my-state/skillshare/logs/를 사용합니다
```

**기본값:** `~/.local/state/skillshare/`

---

### XDG_CACHE_HOME

캐시 디렉터리(버전 확인 캐시, UI dist 캐시)를 재정의합니다.

```bash
export XDG_CACHE_HOME=~/my-cache
# skillshare는 ~/my-cache/skillshare/를 사용합니다
```

**기본값:** `~/.cache/skillshare/`

:::tip 자동 마이그레이션
v0.13.0부터 skillshare는 백업, 휴지통, 로그에 대해 XDG Base Directory Specification을 따릅니다. 이전 버전에서 업그레이드하는 경우, 첫 실행 시 이 디렉터리들이 `~/.config/skillshare/`에서 올바른 XDG 위치로 자동 마이그레이션됩니다.
:::

---

### SKILLSHARE_GITLAB_HOSTS

중첩된 하위 그룹(subgroup)을 지원하는 방식으로 처리할, 자체 관리형 GitLab 호스트명을 쉼표로 구분한 목록입니다. 설정 파일이 없을 수도 있는 CI/CD 환경에서 유용합니다.

```bash
SKILLSHARE_GITLAB_HOSTS=git.company.com,code.internal.io skillshare install git.company.com/team/frontend/ui
```

환경 변수와 설정 파일의 [`gitlab_hosts`](/docs/reference/targets/configuration#gitlab_hosts)가 모두 설정된 경우, 두 값은 **병합**됩니다(중복 제거). 잘못된 항목(스킴, 경로, 포트를 포함하거나 비어 있는 경우)은 조용히 건너뜁니다.

**기본값:** 없음 (설정 파일의 값만 사용됨)

---

### SKILLSHARE_AZURE_HOSTS

자체 호스팅 Azure DevOps Server 호스트명을 쉼표로 구분한 목록입니다.

```bash
SKILLSHARE_AZURE_HOSTS=azuredevops.mycompany.com,tfs.internal.io
```

환경 변수와 설정 파일의 [`azure_hosts`](/docs/reference/targets/configuration#azure_hosts)가 모두 설정된 경우, 두 값은 **병합**됩니다(중복 제거). 환경 변수 내 잘못된 항목은 조용히 건너뜁니다.

**기본값:** _(없음)_

### SKILLSHARE_GITEA_HOSTS

자체 호스팅 Gitea 호스트명을 쉼표로 구분한 목록입니다. 이름에 `gitea`가 포함된 호스트는 이 값 없이도 감지됩니다.

```bash
SKILLSHARE_GITEA_HOSTS=git.company.com,code.internal.io
```

설정 파일의 [`gitea_hosts`](/docs/reference/targets/configuration#gitea_hosts)와 병합됩니다.

**기본값:** _(없음)_

### SKILLSHARE_CNB_HOSTS

자체 호스팅 CNB 호스트명을 쉼표로 구분한 목록입니다. `cnb.cool`은 이 값 없이도 감지됩니다.

```bash
SKILLSHARE_CNB_HOSTS=cnb.company.com
```

설정 파일의 [`cnb_hosts`](/docs/reference/targets/configuration#cnb_hosts)와 병합됩니다.

**기본값:** _(없음)_

---

## 웹 UI

### SKILLSHARE_UI_BASE_PATH

리버스 프록시 뒤에서 웹 대시보드를 제공할 URL 하위 경로를 설정합니다.

```bash
SKILLSHARE_UI_BASE_PATH=/skillshare skillshare ui --host 0.0.0.0 --no-open
```

`--base-path /skillshare`와 동일합니다. 둘 다 설정된 경우 플래그가 우선합니다.

**기본값:** 없음 (대시보드는 루트 `/`에서 제공됨)

Nginx 및 Caddy 예제는 [리버스 프록시](/docs/reference/commands/ui#reverse-proxy)를 참고하세요.

---

## GitHub API

### GITHUB_TOKEN

GitHub 개인 액세스 토큰입니다.

**사용처:**
- GitHub API 요청(`skillshare search`, `skillshare upgrade`, 버전 확인)
- **Git clone 인증** — HTTPS를 통해 비공개 저장소를 설치할 때 자동으로 주입됩니다

**토큰 생성 방법:**
1. https://github.com/settings/tokens 로 이동
2. 새 토큰 생성 (classic)
3. Scope: 비공개 저장소는 `repo`, 공개 저장소는 없음
4. 토큰 복사

공식 문서: [개인 액세스 토큰 관리](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)

**사용법:**
```bash
export GITHUB_TOKEN=ghp_your_token_here
skillshare install https://github.com/org/private-skills.git --track
```

**Windows:**
```powershell
# 현재 세션
$env:GITHUB_TOKEN = "ghp_your_token"

# 영구 설정
[Environment]::SetEnvironmentVariable("GITHUB_TOKEN", "ghp_your_token", "User")
```

---

### GH_TOKEN

대체 GitHub 토큰 변수입니다. [GitHub CLI (`gh`)](https://cli.github.com/)에서 사용되며 skillshare에서도 대체 값으로 인식합니다.

**토큰 확인 순서:** `GITHUB_TOKEN` → `GH_TOKEN` → `gh auth token`

이미 `gh`를 사용 중이고 `gh auth login`으로 인증했다면, skillshare가 별도의 환경 변수 없이 자동으로 토큰을 가져옵니다.

```bash
export GH_TOKEN=ghp_your_token_here
skillshare search "react patterns"
```

---

## Git 인증 {#git-authentication}

이 변수들은 비공개 저장소에 대한 HTTPS 인증을 활성화합니다. 설정되어 있으면 skillshare는 `install`과 `update` 시 URL 수정 없이 토큰을 자동으로 주입합니다.

자세한 내용과 CI/CD 예제는 [비공개 저장소](/docs/reference/commands/install#private-repositories)를 참고하세요.


### GITLAB_TOKEN

GitLab 개인 액세스 토큰 또는 CI job 토큰입니다. GitLab에 호스팅된 비공개 저장소를 HTTPS로 clone할 때 사용됩니다.

**토큰 생성 방법:**
1. https://gitlab.com/-/user_settings/personal_access_tokens 로 이동
2. 새 토큰 추가
3. Scope: `read_repository`(pull 전용) 또는 `read_repository` + `write_repository`(push & pull)
4. 토큰 복사 (접두사 `glpat-`)

공식 문서: [토큰 개요](https://docs.gitlab.com/security/tokens/)

:::warning 토큰 유형
git 작업에는 **Personal Access Token**(`glpat-`)과 **Project/Group Access Token**만 사용할 수 있습니다. Feed Token(`glft-`)은 git 접근 권한이 **없습니다**.
:::

```bash
export GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx
skillshare install https://gitlab.com/org/skills.git --track
```

**Windows:**
```powershell
$env:GITLAB_TOKEN = "glpat-xxxxxxxxxxxxxxxxxxxx"
```

### BITBUCKET_TOKEN

Bitbucket 저장소 액세스 토큰, 또는 앱 비밀번호입니다. Bitbucket에 호스팅된 비공개 저장소를 HTTPS로 clone할 때 사용됩니다.

**토큰 생성 방법 (저장소 액세스 토큰):**
1. Repository → Settings → Access tokens로 이동
2. **Read** 권한(pull 전용) 또는 **Read + Write**(push & pull)로 토큰 생성
3. 토큰 복사 — `x-token-auth`를 자동으로 사용합니다(사용자 이름 불필요)

**토큰 생성 방법 (앱 비밀번호):**
1. https://bitbucket.org/account/settings/app-passwords/ 로 이동
2. **Repositories: Read**(pull 전용) 또는 **Repositories: Read + Write**(push & pull)로 앱 비밀번호 생성
3. `BITBUCKET_USERNAME`도 함께 설정하세요 (또는 URL에 `https://<username>@bitbucket.org/...` 형태로 포함)

공식 문서: [액세스 토큰](https://support.atlassian.com/bitbucket-cloud/docs/access-tokens/)

```bash
export BITBUCKET_USERNAME=your_bitbucket_username
export BITBUCKET_TOKEN=your_app_password
skillshare install https://bitbucket.org/team/skills.git --track
```

**Windows:**
```powershell
$env:BITBUCKET_USERNAME = "your_bitbucket_username"
$env:BITBUCKET_TOKEN = "your_app_password"
```

### BITBUCKET_USERNAME

`BITBUCKET_TOKEN`이 앱 비밀번호일 때 함께 사용하는 Bitbucket 사용자 이름입니다.

```bash
export BITBUCKET_USERNAME=your_bitbucket_username
export BITBUCKET_TOKEN=your_app_password
skillshare install https://bitbucket.org/team/skills.git --track
```

### AZURE_DEVOPS_TOKEN

Azure DevOps 개인 액세스 토큰(PAT)입니다. Azure DevOps에 호스팅된 비공개 저장소를 HTTPS로 clone할 때 사용됩니다.

**토큰 생성 방법:**
1. `https://dev.azure.com/{org}/_usersSettings/tokens`로 이동
2. **+ New Token** 선택
3. Scope: **Code → Read**(pull 전용) 또는 **Code → Read & Write**(push & pull)
4. 토큰 복사 (`AZDO` 시그니처가 포함된 84자 문자열)

공식 문서: [개인 액세스 토큰 사용하기](https://learn.microsoft.com/en-us/azure/devops/organizations/accounts/use-personal-access-tokens-to-authenticate?view=azure-devops)

```bash
export AZURE_DEVOPS_TOKEN=your_pat_here
skillshare install https://dev.azure.com/org/project/_git/repo --track
```

**Windows:**
```powershell
$env:AZURE_DEVOPS_TOKEN = "your_pat_here"
```

### GITEA_TOKEN

Gitea 액세스 토큰입니다. `gitea.com` 및 자체 호스팅 Gitea의 비공개 저장소를 HTTPS로 clone할 때, 그리고 전체 clone 없이 하위 디렉터리를 설치할 때 Gitea Contents API에 사용됩니다.

**토큰 생성 방법:**
1. Gitea에서 **Settings → Applications**로 이동
2. 새 토큰 생성
3. 권한: `repository: Read`(pull 전용) 또는 `repository: Read and Write`(push & pull)

```bash
export GITEA_TOKEN=your_token
skillshare install https://gitea.com/org/skills.git --track
```

**Windows:**
```powershell
$env:GITEA_TOKEN = "your_token"
```

토큰은 설치 대상 호스트에만 전송됩니다. 이름에 `gitea`가 포함되지 않은 자체 호스팅 인스턴스는 [`gitea_hosts`](/docs/reference/targets/configuration#gitea_hosts) 설정이 필요하며, 그렇지 않으면 해당 호스트에는 `SKILLSHARE_GIT_TOKEN`이 사용됩니다.

### CNB_TOKEN

저장소에 대한 read 권한을 가진 [CNB](https://cnb.cool) 액세스 토큰입니다. `cnb.cool` 및 자체 호스팅 CNB의 비공개 저장소를 HTTPS로 clone할 때 사용됩니다.

```bash
export CNB_TOKEN=your_token
skillshare install https://cnb.cool/org/skills --track
```

**Windows:**
```powershell
$env:CNB_TOKEN = "your_token"
```

다른 도메인에 있는 비공개 배포에는 [`cnb_hosts`](/docs/reference/targets/configuration#cnb_hosts) 설정이 필요합니다.

### SKILLSHARE_GIT_TOKEN

모든 HTTPS git 호스트에 사용할 수 있는 범용 대체 토큰입니다. 플랫폼별 토큰이 설정되지 않은 경우 사용됩니다.

```bash
export SKILLSHARE_GIT_TOKEN=your_token
skillshare install https://git.example.com/org/skills.git --track
```

**Windows:**
```powershell
$env:SKILLSHARE_GIT_TOKEN = "your_token"
```

**토큰 우선순위:** 플랫폼별 토큰(`GITHUB_TOKEN`, `GITLAB_TOKEN`, `BITBUCKET_TOKEN`, `AZURE_DEVOPS_TOKEN`, `GITEA_TOKEN`, `CNB_TOKEN`) > `SKILLSHARE_GIT_TOKEN`.

---

## Plugin {#plugins}

### SKILLSHARE_CODEX_CLI

[plugin 명령](/docs/reference/commands/plugin)이 실행할 Codex CLI를 이 머신에서만 지정합니다. Skillshare가 Codex를 스스로 찾지 못할 때 설정하세요. 환경 변수에서 읽으므로 macOS와 Windows가 같은 `config.yaml`을 공유해도 다른 쪽에서 틀린 경로를 적을 필요가 없습니다.

설정하지 않으면 Skillshare는 먼저 `PATH`의 `codex`를 실행하고, 그다음 아래 위치를 차례로 시도합니다.

| OS | 위치 |
|---|---|
| macOS | `/opt/homebrew/bin/codex`, `/usr/local/bin/codex`, 그리고 Codex app에 포함된 CLI(`/Applications` 또는 `~/Applications`의 `ChatGPT.app` 또는 `Codex.app`) |
| Windows | 공식 PowerShell 설치 프로그램의 `%LOCALAPPDATA%\Programs\OpenAI\Codex\bin\codex.exe`, 그다음 Codex app이 설치하는 CLI: `%LOCALAPPDATA%\OpenAI\Codex\bin\<version>\codex.exe`(가장 최신 것) |

```bash
export SKILLSHARE_CODEX_CLI=/Applications/ChatGPT.app/Contents/Resources/codex-cli/bin/codex
```

**Windows:**
```powershell
$env:SKILLSHARE_CODEX_CLI = "C:\Tools\codex.exe"
```

자체 `cli`를 설정한 [account target](/docs/reference/commands/plugin#accounts)은 그 `cli`를 계속 사용합니다.

---

## Git SSL / TLS {#git-ssl--tls}

이 표준 Git 환경 변수들은 모든 git 작업에 전달됩니다. 자체 서명 인증서나 내부 CA를 사용하는 자체 호스팅 Git 서버(GitLab, Gitea 등)에서 유용합니다.

### GIT_SSL_CAINFO

커스텀 CA 인증서 번들의 경로입니다. 자체 호스팅 Git 서버가 내부 CA로 서명된 인증서를 사용하는 경우 사용하세요.

```bash
export GIT_SSL_CAINFO=/path/to/ca-bundle.crt
skillshare install https://gitlab.internal.company.com/team/skills.git --track
```

### GIT_SSL_NO_VERIFY

SSL 인증서 검증을 완전히 비활성화합니다. CA 인증서 설정이 불가능할 때 최후의 수단으로만 사용하세요.

:::warning 보안 위험
SSL 검증을 비활성화하면 연결이 중간자 공격(man-in-the-middle)에 취약해집니다. 가급적 적절한 CA 번들과 함께 `GIT_SSL_CAINFO`를 사용하거나, SSH를 사용하세요.
:::

```bash
GIT_SSL_NO_VERIFY=true skillshare install https://gitlab.internal.company.com/team/skills.git --track
```

**대안: SSL을 아예 사용하지 않으려면 SSH를 사용하세요:**
```bash
skillshare install git@gitlab.internal.company.com:team/skills.git --track
```

---

## 테스트

### SKILLSHARE_TEST_BINARY

통합 테스트에서 사용할 CLI 바이너리 경로를 재정의합니다.

```bash
SKILLSHARE_TEST_BINARY=/path/to/skillshare go test ./tests/integration
```

**기본값:** 프로젝트 루트의 `bin/skillshare`

---

## 사용 예시

### 임시 재정의

```bash
# 단일 명령어
SKILLSHARE_CONFIG=/tmp/test-config.yaml skillshare status

# 여러 명령어
export SKILLSHARE_CONFIG=/tmp/test-config.yaml
skillshare status
skillshare list
unset SKILLSHARE_CONFIG
```

### 영구 설정 (macOS/Linux)

`~/.bashrc` 또는 `~/.zshrc`에 추가:
```bash
export GITHUB_TOKEN="ghp_your_token_here"
```

### 영구 설정 (Windows)

```powershell
[Environment]::SetEnvironmentVariable("GITHUB_TOKEN", "ghp_your_token", "User")
```

---

## 요약

| 변수 | 용도 | 기본값 |
|----------|---------|---------|
| `SKILLSHARE_CONFIG` | 설정 파일 경로 | `~/.config/skillshare/config.yaml` |
| `SKILLSHARE_UI_BASE_PATH` | 리버스 프록시용 웹 UI 하위 경로 | 없음 |
| `SKILLSHARE_GITLAB_HOSTS` | 커스텀 GitLab 호스트명 (쉼표로 구분) | 없음 |
| `SKILLSHARE_AZURE_HOSTS` | 커스텀 Azure DevOps Server 호스트명 (쉼표로 구분) | 없음 |
| `SKILLSHARE_GITEA_HOSTS` | 커스텀 Gitea 호스트명 (쉼표로 구분) | 없음 |
| `SKILLSHARE_CNB_HOSTS` | 커스텀 CNB 호스트명 (쉼표로 구분) | 없음 |
| `XDG_CONFIG_HOME` | 기본 설정 디렉터리 | `~/.config` (Linux/macOS), `%AppData%` (Windows) |
| `XDG_DATA_HOME` | 데이터 디렉터리 (백업, 휴지통) | `~/.local/share` |
| `XDG_STATE_HOME` | 상태 디렉터리 (로그) | `~/.local/state` |
| `XDG_CACHE_HOME` | 캐시 디렉터리 (버전 확인, UI) | `~/.cache` |
| `GITHUB_TOKEN` | GitHub API + git clone 인증 | 없음 |
| `GH_TOKEN` | GitHub API (`GITHUB_TOKEN`의 대체 값) | 없음 |
| `GITLAB_TOKEN` | GitLab git clone 인증 | 없음 |
| `BITBUCKET_TOKEN` | Bitbucket git clone 인증 | 없음 |
| `BITBUCKET_USERNAME` | 앱 비밀번호 인증용 Bitbucket 사용자 이름 | 없음 |
| `AZURE_DEVOPS_TOKEN` | Azure DevOps git clone 인증 | 없음 |
| `GITEA_TOKEN` | Gitea git clone + Contents API 인증 | 없음 |
| `CNB_TOKEN` | CNB git clone + contents API 인증 | 없음 |
| `SKILLSHARE_GIT_TOKEN` | 범용 git clone 인증 (대체 값) | 없음 |
| `SKILLSHARE_CODEX_CLI` | 이 머신에서 plugin 명령이 사용하는 Codex CLI | `PATH`의 `codex`, 그다음 알려진 설치 위치 |
| `GIT_SSL_CAINFO` | 커스텀 CA 인증서 번들 경로 | 시스템 기본값 |
| `GIT_SSL_NO_VERIFY` | SSL 인증서 검증 비활성화 | `false` |
| `SKILLSHARE_TEST_BINARY` | 테스트 바이너리 경로 | `bin/skillshare` |

---

## 관련 문서

- [Configuration](/docs/reference/targets/configuration) — 설정 파일 레퍼런스
- [Windows Issues](/docs/troubleshooting/windows) — Windows 환경 설정
