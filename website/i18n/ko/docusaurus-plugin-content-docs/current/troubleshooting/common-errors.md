---
sidebar_position: 2
---

# Common Errors

오류 메시지와 그 해결 방법입니다.

## Config Errors

### `config not found: run 'skillshare init' first`

**Cause:** Config 파일이 존재하지 않습니다.

**Solution:**
```bash
skillshare init
```

Custom 경로를 원한다면 `--source`를 추가하세요.
```bash
skillshare init --source ~/my-skills
```

---

### `failed to load project config: ...`

**Cause:** `.skillshare/config.yaml`이 존재하지만 파싱할 수 없습니다 (잘못된 YAML, 잘못된 타입 등). 변경을 가하는 명령어(`uninstall`, `new`, `enable`/`disable`, `check`)는 이 상태에서 진행을 거부합니다. custom `sources` 설정이 있을 때 기본 `.skillshare/skills/` 디렉터리를 실수로 건드리지 않기 위해서입니다.

**Solution:** YAML을 수정한 뒤 명령어를 다시 실행하세요. 흔한 문제:

```yaml
# WRONG — targets는 리스트여야 함
targets: {}

# RIGHT
targets: []
```

```yaml
# WRONG — skills는 리스트여야 함
skills: my-skill

# RIGHT
skills:
  - name: my-skill
    source: github.com/org/my-skill
```

아무 YAML linter로든 파일을 검증하거나, 백업이 있다면 `.skillshare/backups/`에서 임시로 복원하세요.

---

### `target "<name>": skills target path X overlaps skills source Y`

**Cause:** `sources.skills`가 어떤 Target의 skill 경로와 동일한 디렉터리로 해석되거나 (또는 한쪽이 다른 쪽을 포함) 합니다. 예를 들어, `claude` Target과 함께 `sources.skills: .claude/skills`를 설정하면 — 둘 다 `.claude/skills/`를 가리킵니다. 이 안전장치가 없다면, `sync --force`가 Source를 Target 디렉터리로 취급해 그 내용을 삭제하게 됩니다.

**Solution:** 어떤 Target과도 겹치지 않는 Source 경로를 선택하세요. 흔히 사용하는 안전한 선택지:

```yaml
# 프로젝트 문서와 함께 배치
sources:
  skills: ./docs/skills

# .skillshare/ 아래 유지 (기본값 — sources 키를 완전히 제거)
```

같은 검사가 agent Target 경로에 대해 `sources.agents`에도 적용됩니다.

---

## Target Errors

### `target add: path does not exist`

**Cause:** skill 디렉터리가 아직 존재하지 않습니다.

**Solution:**
```bash
mkdir -p ~/.myapp/skills
skillshare target add myapp ~/.myapp/skills
```

### `target path does not end with 'skills'`

**Cause:** 경로가 관례를 따르지 않는다는 경고입니다.

**Solution:** 이것은 오류가 아니라 경고입니다. 경로가 의도한 것이라면 그대로 진행하거나, 수정하세요.
```bash
skillshare target add myapp ~/.myapp/skills  # 권장
```

### `target directory already exists with files`

**Cause:** Target에 덮어써질 수 있는 기존 파일이 있습니다.

**Solution:**
```bash
skillshare backup
skillshare sync
```

---

## Sync Errors

### `deleting a symlinked target removed source files`

**Cause:** symlink 모드에서 Target에 `rm -rf`를 실행했습니다.

**Solution:**
```bash
# git이 초기화되어 있다면
cd ~/.config/skillshare/skills
git checkout -- .

# 또는 백업에서 복원
skillshare restore <target>
```

**Prevention:** 수동 삭제 대신 `skillshare target remove`를 사용하세요.

### `sync` keeps showing the same changes {#sync-keeps-showing-the-same-changes}

**Cause:** 두 target이 서로 다른 `include`/`exclude` filter, `mode` 또는 `target_naming`으로 (두 target 모두 `symlink` 모드이면 폴더 전체를 링크하므로 필터와 이름 규칙은 영향을 주지 않습니다) 같은 폴더에 skill을 sync합니다. 매번 sync할 때마다 한 target의 설정대로 폴더를 다시 쓰고 다른 target의 결과를 되돌리므로(필터된 skill 추가나 제거, 이름 변경, 링크와 복사본 전환), 폴더가 안정되지 않습니다. `sync`가 해당 target을 알려줍니다:

```
! codex and universal sync skills to ~/.agents/skills with different settings, so each sync undoes the other
  keep one: skillshare target codex --skills=false
```

**Solution:** 한 target만 폴더에 쓰도록 하고 다른 target의 skills를 끄세요. 그 target의 agents, MCP 서버, 지침은 계속 관리되며, 도구는 여전히 공유 폴더의 skill을 읽습니다:

```bash
skillshare target codex --skills=false --dry-run
skillshare target codex --skills=false
```

대시보드의 **Sync** 페이지에도 같은 경고가 표시되며, 해당 target의 skill 동기화를 중지하는 버튼이 함께 제공됩니다. 두 target에 같은 filter, mode, target naming을 지정해도 해결됩니다.

### `sync seems stuck or slow`

**Cause:** skill 디렉터리에 큰 파일이 있습니다.

**Solution:** ignore 패턴을 추가하세요.
```yaml
# ~/.config/skillshare/config.yaml
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
  - "**/node_modules/**"
```

### `no space left on device` / `ENOSPC` during sync

**Cause:** 무언가가 볼륨을 채우고 있습니다. 먼저 백업 디렉터리를, 그다음 Source를 확인하세요.

**Solution:**
```bash
df -h ~                                     # 볼륨이 꽉 찼는지 확인
du -sh ~/.local/share/skillshare/backups    # 백업 사용량
du -sh ~/.config/skillshare/skills          # Source 사용량
```

백업이 크다면 정리하세요 — 정리 작업은 각 `sync` 이후 자동으로 실행되지만, 그 전에 이미 커진 디렉터리는 필요할 때 수동으로 비울 수 있습니다.

```bash
skillshare backup --cleanup --dry-run   # 미리보기
skillshare backup --cleanup
```

볼륨이 100%로 꽉 차면, 약간의 공간이 확보되기 전까지 `rm`이 "Permission denied"로 실패할 수 있습니다. 먼저 큰 파일 하나를 지운 뒤 정리하세요.

**Source**가 크다면, 문제의 아티팩트는 여러분의 skill 안에 있습니다. 백업은 이를 복사하지 않지만 (symlink된 skill은 건너뜀), copy 모드의 모든 Target은 복사합니다. 런타임 캐시, 모델 가중치, 브라우저 프로필은 skill 트리 밖으로 옮기거나 `ignore:`로 제외하세요.

Backup 범위가 `.gitignore` 및 `ignore:`와 어떻게 다른지는 [Backups & Disk Space](/docs/reference/commands/backup#backups--disk-space)를 참고하세요.

---

## Git Errors

### `Could not read from remote repository`

**Cause:** SSH 키가 설정되지 않았거나, remote URL이 잘못되었습니다.

**Solution:**
```bash
# SSH 접근 확인
ssh -T git@github.com

# SSH가 설정되지 않았다면 대신 HTTPS 사용
git -C ~/.config/skillshare/skills remote set-url origin https://github.com/you/my-skills.git

# 또는 SSH 키 설정
ssh-keygen -t ed25519 -C "you@example.com"
# 그런 다음 공개 키를 GitHub → Settings → SSH keys에 추가
```

### `push: remote has changes`

**Cause:** Remote repository가 로컬보다 앞서 있습니다.

**Solution:**
```bash
skillshare pull   # 먼저 remote 변경 사항 받기
skillshare push   # 이제 push가 동작함
```

### `pull: local has uncommitted changes`

**Cause:** 아직 push되지 않은 로컬 변경 사항이 있습니다.

**Solution:**
```bash
# 옵션 1: 먼저 변경 사항 push
skillshare push -m "Local changes"
skillshare pull

# 옵션 2: 로컬 변경 사항 폐기
cd ~/.config/skillshare/skills
git checkout -- .
skillshare pull
```

### `pull stopped: this machine and the remote both changed ...`

**Cause:** 동일한 파일이 두 머신에서 수정되었습니다. `pull`이 병합을 되돌렸으므로 저장소는 변경되지 않았습니다. `.metadata.json`만의 충돌은 자동으로 해결되므로 이 오류의 원인이 되지 않습니다.

**Solution:**
```bash
cd ~/.config/skillshare/skills
git pull --no-rebase          # 병합을 다시 실행하고 충돌을 남겨 둠
# 충돌한 파일 수정
git add .
git commit --no-edit
skillshare push
skillshare sync
```

### `Git had no identity`

**Cause:** `skillshare init`이 source repo를 만들 때 git에 `user.name` / `user.email`이 없었습니다. skillshare는 commit이 가능하도록 해당 repo 자체 설정에 폴백(`skillshare@local`)을 기록합니다. repo 설정이 `git config --global`보다 우선하므로, 나중에 global identity를 설정해도 대체되지 않습니다.

직접 만든 repo는 변경하지 않습니다. skillshare는 최초 commit 한 번에만 폴백을 사용합니다.

**Solution:** 해당 repo에서 identity를 설정하세요(경로는 메시지에 표시된 것을 사용하세요. 아래는 기본값):
```bash
git -C ~/.config/skillshare/skills config user.name "Your Name"
git -C ~/.config/skillshare/skills config user.email "you@example.com"
```

또는 repo 설정을 제거하고 global identity를 사용하세요: `git -C ~/.config/skillshare/skills config --unset user.name`(`user.email`도 동일).

### `Git root mismatch`

**Cause:** `config.yaml`의 `git_root`가 git repo가 없는 scope 디렉터리를 가리키는데, 다른 scope 디렉터리에는 repo가 있습니다. 이는 repository를 옮기지 않고 `git_root`를 변경할 때 발생합니다 — scope 전환은 "다른 디렉터리를 버전 관리하기 시작한다"는 의미이지, "기존 히스토리를 옮긴다"는 의미가 아닙니다. [`git_root`](/docs/reference/targets/configuration#git-root)를 참고하세요.

**Solution:** 오류 메시지가 출력하는 세 가지 옵션 중 하나를 선택하세요.
```bash
# 설정된 scope에 새 repo 시작 (히스토리 없음)
skillshare init --git-root <scope>

# 히스토리를 유지하며 기존 repo 이동
mv <old-scope>/.git <new-scope>/.git

# 또는 기존 repo를 계속 사용: config.yaml에서 git_root를 되돌리기
#   git_root: <scope-that-has-the-repo>
```

### `tracked repository clone is missing`

**Cause:** tracked repo가 `.metadata.json`에 선언되어 있지만, 클론 디렉터리(예: `skills/_team-skills/`)가 로컬에 없습니다. 이는 tracked repo 디렉터리가 관리형 `.gitignore` 블록에 의도적으로 포함되어 있기 때문에, 새 머신에서 skillshare source repo를 클론한 뒤 자주 발생합니다.

**Solution:** 메타데이터로부터 누락된 tracked repo 클론을 다시 만드세요.
```bash
skillshare install
skillshare sync
```

Project mode의 경우:
```bash
skillshare install -p
skillshare sync -p
```

`status`, `check`, `update --all`, `doctor`는 이 상태를 보고하고 `skillshare install`을 제안합니다.

### `nested git repositories must be disabled first`

**Cause:** `git_root: root`에서, 하위 디렉터리(예: `skills/_org/` 아래의 tracked skill repo)가 자체 `.git`을 가지고 있습니다. Git은 이를 **빈 submodule**로 업로드해 조용히 파일을 누락시키므로, 각 중첩 repo가 비활성화될 때까지 `commit`/`push`가 중단됩니다.

**Solution:**
```bash
# 보고된 각 중첩 repo 비활성화 (되돌릴 수 있음 — 이름을 다시 바꾸면 재활성화됨)
mv ~/.config/skillshare/<dir>/.git ~/.config/skillshare/<dir>/.git.disabled
```
또는 웹 UI의 Git Sync 페이지에서 원클릭으로 비활성화하세요. skillshare는 또한 머신별 경로를 담고 있는 `config.yaml`을 root-scope repo에서 자동으로 제외합니다.

### `Invalid git_root`

**Cause:** `config.yaml`의 `git_root`가 인식되지 않는 값(예: 오타)으로 설정되어 있습니다.

**Solution:** `skills`, `agents`, `extras`, `root` 중 하나를 사용하거나, 비워 두세요 (기본값은 `skills`).

---

## Install Errors

### `skill already exists`

**Cause:** 동일한 이름의 skill이 이미 설치되어 있습니다.

**Solution:**
```bash
# 기존 skill 업데이트
skillshare install <source> --update

# 또는 강제로 덮어쓰기
skillshare install <source> --force
```

### `git failed (exit 128): repository not found or authentication required`

**Cause:** repository URL이 잘못되었거나, repo가 존재하지 않거나, 인증이 누락되었습니다.

이제 skillshare는 원시 종료 코드 대신 흔한 git 실패에 대한 실행 가능한 오류 메시지를 제공합니다. 오류 메시지에는 제안이 포함됩니다.

```
Error: git failed (exit 128): repository not found or authentication required
```

토큰을 사용했지만 거부된 경우:

```
Error: git failed (exit 128): authentication token was rejected — check permissions and expiry
```

**Solution:** 아래의 인증 옵션을 참고하세요.

### `Authentication failed` / `Access denied`

**Cause:** HTTPS 자격 증명이 없거나, 만료되었거나, 잘못된 토큰 유형입니다.

**Solution — Option 1: 토큰 환경 변수 설정:**

```bash
# GitHub
export GITHUB_TOKEN=ghp_xxxx

# GitLab (반드시 Personal Access Token, 접두사 glpat-)
export GITLAB_TOKEN=glpat-xxxx

# Bitbucket
export BITBUCKET_TOKEN=your_app_password
```

**Windows (PowerShell):**
```powershell
$env:GITLAB_TOKEN = "glpat-xxxx"

# 영구 설정 (재시작 후에도 유지)
[Environment]::SetEnvironmentVariable("GITLAB_TOKEN", "glpat-xxxx", "User")
```

**Solution — Option 2: SSH URL 사용:**
```bash
skillshare install git@github.com:team/private-skills.git
skillshare install git@gitlab.com:team/skills.git
skillshare install git@bitbucket.org:team/skills.git
```

**Solution — Option 3: Git credential helper:**
```bash
gh auth login          # GitHub CLI
git credential approve # 또는 플랫폼별 credential manager
```

**Required token permissions:**

| Platform | Token type | Scopes / Permissions |
|----------|-----------|---------------------|
| GitHub | Personal Access Token (`ghp_`) | `repo` (private repo), 없음 (public) |
| GitLab | Personal Access Token (`glpat-`) | `read_repository` + `write_repository` |
| Bitbucket | Repository Access Token | Read + Write |
| Bitbucket | App Password + `BITBUCKET_USERNAME` | Repositories: Read + Write |

:::warning GitLab token types
**Personal Access Token** (`glpat-`)만 git 작업에 사용할 수 있습니다. Feed Token(`glft-`)은 git 접근 권한이 **없습니다**.
:::

[Environment Variables](/docs/reference/appendix/environment-variables#git-authentication)와 [Private Repositories](/docs/reference/commands/install#private-repositories)를 참고하세요.

### `SSL certificate problem` / `certificate verification failed`

**Cause:** Git 서버가 자체 서명 인증서 또는 시스템이 신뢰하지 않는 내부 CA를 사용합니다. self-hosted GitLab, Gitea, Gogs 인스턴스에서 흔합니다.

**Solution — Option 1: Custom CA bundle (권장):**
```bash
export GIT_SSL_CAINFO=/path/to/company-ca-bundle.crt
skillshare install https://gitlab.internal.company.com/team/skills.git --track
```

**Solution — Option 2: 대신 SSH 사용 (SSL을 완전히 회피):**
```bash
skillshare install git@gitlab.internal.company.com:team/skills.git --track
```

**Solution — Option 3: SSL 검증 비활성화 (권장하지 않음):**
```bash
GIT_SSL_NO_VERIFY=true skillshare install https://gitlab.internal.company.com/team/skills.git --track
```

:::warning
SSL 검증 비활성화는 보안 위험입니다. Option 1 또는 2를 사용하세요.
:::

[Environment Variables — Git SSL / TLS](/docs/reference/appendix/environment-variables#git-ssl--tls)를 참고하세요.

### `invalid skill: SKILL.md not found`

**Cause:** Source에 유효한 SKILL.md 파일이 없습니다.

**Solution:** Source 경로가 올바르며 skill 디렉터리를 가리키는지 확인하세요.

---

## Update Errors

### `pull stopped: this machine and the remote both changed ...` (tracked 저장소)

**Cause:** tracked 저장소에 remote와 충돌하는 로컬 커밋이 있습니다. `update`는 갈라진 히스토리를 병합하지만, 충돌하는 파일이 있으면 중단되고 병합이 되돌려집니다.

**Solution:**
```bash
# 강제 업데이트 (로컬을 remote로 대체)
skillshare update --force

# 또는 수동으로 해결
cd ~/.config/skillshare/skills/_repo-name
git pull --no-rebase
# 충돌한 파일을 수정한 뒤
git add . && git commit --no-edit
```

:::tip
`skillshare update`와 `skillshare install`은 이제 원시 종료 코드 대신 git 실패(인증, SSL, 갈라진 브랜치)에 대한 실행 가능한 오류 메시지를 보여줍니다.
:::

---

## Audit Errors

### `security audit failed — critical threats detected`

**Cause:** skill이 중요 보안 위협(prompt injection, 데이터 유출, 자격 증명 접근)과 일치하는 패턴을 포함하고 있습니다.

**Solution:**
```bash
# 발견 항목 검토
skillshare audit <skill-name>

# Source를 신뢰한다면, 강제 설치
skillshare install <source> --force
```

### `audit HIGH: Hidden zero-width Unicode characters detected`

**Cause:** skill에 보이지 않는 유니코드 문자가 포함되어 있으며, 이는 복사-붙여넣기 흔적이거나 의도적인 난독화일 수 있습니다.

**Solution:** 숨겨진 문자를 표시하는 에디터에서 파일을 열어 제거하거나, Source를 신뢰한다면 강제 설치하세요.

---

## Upgrade Errors

### `GitHub API rate limit exceeded`

**Cause:** 인증되지 않은 API 요청이 너무 많습니다.

**Solution:**
```bash
# 옵션 1: GitHub 토큰 설정 (권장)
export GITHUB_TOKEN=ghp_your_token_here
skillshare upgrade

# 옵션 2: 강제 업그레이드
skillshare upgrade --cli --force
```

다음에서 토큰을 생성하세요: https://github.com/settings/tokens (public repo에는 scope가 필요 없음)

---

## Skill Errors

### `skill not appearing in AI CLI`

**Causes:**
1. skill이 동기화되지 않음
2. 잘못된 SKILL.md 형식
3. AI CLI 캐싱

**Solutions:**
```bash
# 1. 동기화
skillshare sync

# 2. 형식 확인
skillshare doctor

# 3. AI CLI 재시작
```

### Antigravity does not load synced skills {#antigravity-does-not-load-synced-skills}

**Cause:** Antigravity 앱의 skill scanner는 **실제 디렉터리**만 탐색합니다 — symlink는 건너뜁니다. skillshare의 기본 `merge` 모드는 skill마다 하나의 symlink를 생성하므로(Windows에서는 NTFS junction), 어느 것도 인식되지 않습니다. Windows에서는 `Incorrect function` 오류로 나타나며, macOS와 Linux에서는 skill이 조용히 사라진 것처럼 보입니다.

이는 skillshare의 버그가 아니라 Antigravity 측의 제약입니다. `antigravity` target(앱, `~/.gemini/config/skills`)에만 해당하며, 독립 실행형 `agy` CLI는 `~/.gemini/antigravity-cli/skills`를 읽는 별도의 `antigravity-cli` target입니다. 두 가지 우회 방법이 있습니다.

**Option 1 — Target을 `copy` 모드로 전환**

```bash
skillshare target antigravity --mode copy
skillshare sync --force
```

symlink 대신 실제 디렉터리가 작성됩니다. Trade-off: Source skill을 편집한 뒤 `skillshare sync`를 다시 실행해야 합니다.

**Option 2 — Antigravity가 Source 디렉터리를 가리키게 하기**

Antigravity에서: **Settings → Customizations → Skill Custom Paths → "+ Add"**로 이동한 뒤, skillshare Source의 **절대** 경로(예: `/Users/you/.config/skillshare/skills`)를 입력하세요. `~` 축약형은 확장되지 않으므로 전체 경로가 필요합니다.

어느 방법을 쓰든, skill을 다시 불러오려면 Antigravity를 재시작하세요.

### `skill name 'X' is defined in multiple places`

**Cause:** 여러 skill이 동일한 `name` 필드를 가지고 있으며 같은 Target에 도달합니다.

**Solution:** SKILL.md에서 하나의 이름을 바꾸거나, `include`/`exclude` 필터로 서로 다른 Target으로 라우팅하세요.
```yaml
# 옵션 1: SKILL.md에서 네임스페이스 지정
name: team-a-skill-name

# 옵션 2: 필터로 라우팅 (global config)
targets:
  codex:
    path: ~/.codex/skills
    include: [_team-a__*]
  claude:
    path: ~/.claude/skills
    include: [_team-b__*]

# 옵션 2: 필터로 라우팅 (project config)
targets:
  - name: claude
    exclude: [codex-*]
  - name: codex
    include: [codex-*]
```

:::tip
필터가 이미 중복 항목을 분리하고 있다면, sync는 이에 대해 아무것도 출력하지 않습니다 — 별도 조치가 필요 없습니다.
전체 문법은 [Target Filters](/docs/reference/targets/configuration#include--exclude-target-filters)를 참고하세요.
:::

---

## Agent Errors

### Warning: `No agents folder: <targets>`

**Cause:** `skillshare sync` (또는 `skillshare sync agents`)를 실행했는데, 설정된 Target 중 하나 이상이 agents 디렉터리를 정의하지 않았습니다. 내장 agent 경로를 가진 것은 Claude, Cursor, Augment, OpenCode뿐이며, 다른 Target은 조용히 건너뜁니다.

**Solutions:**

1. 해당 Target에 agent가 필요 없다면 경고를 무시하세요.
2. `config.yaml`의 Target에 `agents:` 하위 키를 추가해 agent sync를 활성화하세요.

```yaml
targets:
  myapp:
    path: ~/myapp/skills
    agents:
      path: ~/myapp/agents
```

그런 다음 `skillshare sync agents`를 다시 실행하세요.

### `backup is not supported in project mode (except for agents)`

**Cause:** `agents` 필터 없이 `skillshare backup -p` (또는 `skillshare backup -p <target>`)를 실행했습니다. Project mode에서는 agent 백업만 지원됩니다 — skill 백업은 global 전용입니다.

**Solution:** `agents` 위치 인자를 추가하거나 `--all`을 사용하세요.

```bash
skillshare backup -p agents          # Project agent target
skillshare backup -p agents claude   # 특정 Target
skillshare backup -p --all           # 위와 동일 (agent로 좁혀짐)
```

`restore`에도 동일한 규칙이 적용됩니다: `restore is not supported in project mode (except for agents)`.

### `agent name 'X' has invalid characters`

**Cause:** agent 파일 이름 또는 `name:` frontmatter 필드에 허용되지 않은 문자가 포함되어 있습니다.

**Solution:** Agent 이름은 `a-z`, `0-9`, `_`, `-`, `.`만 사용해야 합니다. 파일 이름을 바꾸고(`name:` 필드도 함께 업데이트해) 동일한 정식 이름을 공유하도록 하세요.

### `.agentignore` patterns not taking effect

**Causes:**

1. 파일이 잘못된 위치에 있습니다. agents source root에 있어야 합니다: `~/.config/skillshare/agents/.agentignore` (global) 또는 `.skillshare/agents/.agentignore` (project).
2. 패턴이 예상과 다른 세그먼트와 일치합니다 — 이 파일은 [gitignore 문법](https://git-scm.com/docs/gitignore)을 사용합니다.

**Solution:** `skillshare doctor`로 파일 경로를 확인하고 패턴을 다시 점검하세요. Agent는 (`.md`를 뺀) basename으로 매칭되므로, `draft-*`는 `draft-experiment.md`와 일치합니다. CLI가 항목을 대신 작성하게 하려면 `skillshare disable <agent> --kind agent`를 사용하세요.

---

## Plugin Errors {#plugin-errors}

### `<agent> CLI is not installed or not on PATH` {#plugin-cli-not-on-path}

**Cause:** Plugin 명령은 Skillshare가 실행되는 머신에서 Agent의 네이티브 CLI(`claude`, `codex` 등)를 실행하는데, 그 CLI를 찾지 못했습니다. 예약 작업이나 서비스에서 시작한 dashboard는 터미널보다 `PATH`가 짧은 경우가 많습니다.

**Solutions:**

1. 해당 머신에 Agent의 CLI를 설치하세요.
2. 이미 설치되어 있다면, Skillshare를 시작하는 쪽(예: 예약 작업의 환경)의 `PATH`에 그 디렉터리를 추가하세요.
3. [account target](/docs/reference/commands/plugin#accounts)이라면 대신 `cli`에 실행 파일의 절대 경로를 설정할 수 있습니다.

### `Codex CLI not found on the machine running Skillshare` {#plugin-codex-not-found}

**Cause:** Skillshare가 `PATH`, Homebrew 폴더, Codex 데스크톱 app 안에서 Codex CLI를 찾았지만 찾지 못했습니다. 메시지에 찾아본 모든 위치가 표시됩니다.

**Solutions:**

1. 해당 머신에 Codex app 또는 Codex CLI를 설치하세요.
2. Codex가 다른 곳에 있다면 Skillshare를 시작하는 환경(예: 예약 작업)에서 [`SKILLSHARE_CODEX_CLI`](/docs/reference/appendix/environment-variables#skillshare_codex_cli)를 그 경로로 설정하세요. 이 설정은 해당 머신에만 남으므로 다른 OS와 공유하는 config에는 영향을 주지 않습니다.

### `The native marketplace X is gone` {#plugin-marketplace-gone}

**Cause:** 이 plugin은 import된 것이므로 Skillshare는 원래의 네이티브 marketplace에서 다시 설치하는데, 그 marketplace가 Agent에 등록되어 있지 않습니다. 두 번째 머신에서 흔히 발생합니다. import는 첫 번째 머신에만 기록되기 때문입니다.

**Solutions:**

1. Agent에서 marketplace를 다시 추가한 뒤 `skillshare sync plugins`를 실행하세요.
2. 또는 해당 plugin에서 그 Agent를 제거하고 source에서 plugin을 다시 추가하세요. [Cross-Machine Sync — Plugins](/docs/how-to/sharing/cross-machine-sync#plugins)를 참고하세요.

---

## Binary Errors

### `integration tests cannot find the binary`

**Cause:** 바이너리가 빌드되지 않았거나 경로가 잘못되었습니다.

**Solution:**
```bash
go build -o bin/skillshare ./cmd/skillshare
# 또는 설정
export SKILLSHARE_TEST_BINARY=/path/to/skillshare
```

---

## Still Having Issues?

체계적인 디버깅 접근법은 [Troubleshooting Workflow](./troubleshooting-workflow.md)를 참고하세요.
