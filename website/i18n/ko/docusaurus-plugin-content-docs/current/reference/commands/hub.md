---
sidebar_position: 8
---

# hub

skill hub를 관리합니다 — 조직 전체의 skill 검색을 위해 저장된 hub 소스입니다.

## 사용 시점

- `search`가 조회할 조직의 skill 카탈로그 설정
- 여러 hub 간 전환 (예: 회사 전체 vs 팀별)
- 저장된 hub 소스 목록 확인 또는 제거

## hub add

hub 소스를 config에 저장하여 [`search --hub`](./search.md)에서 재사용합니다.

```bash
skillshare hub add <url> [options]
```

| Flag | 설명 |
|------|-------------|
| `--label`, `-l` | 사용자 지정 label (기본값: URL 호스트명에서 도출) |
| `--project`, `-p` | 프로젝트 config에 저장 |
| `--global`, `-g` | 전역 config에 저장 |

처음 추가된 hub는 자동으로 기본값으로 설정됩니다. Label은 대소문자를 구분하지 않습니다.

hub URL은 HTTP(S) URL, 로컬 파일 경로, 또는 SSH URL(scp 스타일 `git@host:org/repo.git` 또는 scheme 스타일 `ssh://git@host/org/repo.git`)이 될 수 있습니다. SSH 소스의 경우 index 파일은 `//path` 접미사를 통해 저장소에서 읽어오며, 기본값은 저장소 루트의 `skillshare-hub.json`입니다. SSH 소스는 사용자의 SSH agent/키를 사용하여 클론되므로 비공개 저장소와 GitHub Enterprise 호스트에서도 작동합니다.

SSH hub에 동일 호스트의 GitHub 또는 GitHub Enterprise 도메인 접두사 항목이 포함된 경우, Skillshare는 해당 hub의 SSH identity를 통해 설치합니다. 예를 들어 `acme@acme.ghe.com:Org/skills.git//hubs/team.json`으로 추가된 hub에 `acme.ghe.com/Org/skills/skills/reviewer`가 포함될 수 있으며, 검색 결과는 이를 `acme@acme.ghe.com:Org/skills.git//skills/reviewer`로 설치합니다. SSH hub 외부에서는 도메인 접두사 소스가 여전히 HTTPS를 의미합니다.

```bash
skillshare hub add https://internal.corp/hub.json --label team
skillshare hub add ./local-hub.json                          # label derived: "local-hub"
skillshare hub add git@ghe.corp.com:team/skills.git --label ghe
skillshare hub add git@ghe.corp.com:team/skills.git//hubs/team.json --label ghe-team
```

## hub list

저장된 hub 목록을 표시합니다. `*`는 기본 hub를 표시합니다.

```bash
skillshare hub list [options]
```

| Flag | 설명 |
|------|-------------|
| `--project`, `-p` | 프로젝트 hub 표시 |
| `--global`, `-g` | 전역 hub 표시 |

```
$ skillshare hub list
  team      https://internal.corp/hub.json · default
  local     ./local-hub.json

2 hubs
```

별칭: `hub ls`

## hub remove

label로 저장된 hub를 제거합니다.

```bash
skillshare hub remove <label> [options]
```

| Flag | 설명 |
|------|-------------|
| `--project`, `-p` | 프로젝트 config에서 제거 |
| `--global`, `-g` | 전역 config에서 제거 |

제거된 hub가 기본값이었다면 기본값이 초기화됩니다.

별칭: `hub rm`

## hub default

`search --hub`(플래그 단독 사용 시)가 사용하는 기본 hub를 표시하거나 설정합니다.

```bash
skillshare hub default [label] [options]
```

| Flag | 설명 |
|------|-------------|
| `--reset` | 기본값 초기화 (community hub로 되돌림) |
| `--project`, `-p` | 프로젝트 config 사용 |
| `--global`, `-g` | 전역 config 사용 |

```bash
skillshare hub default              # Show current default
skillshare hub default team         # Set default to "team"
skillshare hub default --reset      # Clear default → community hub
```

## hub index

설치된 skill로부터 `skillshare-hub.json` index 파일을 빌드합니다. 생성된 index는 비공개 오프라인 skill 검색을 위해 [`search --hub`](./search.md#private-index-search)에서 사용할 수 있습니다.

### Usage

```bash
skillshare hub index [options]
```

### Options

| Flag | 설명 |
|------|-------------|
| `--source`, `-s` | 스캔할 소스 디렉터리 (기본값: 자동 감지) |
| `--output`, `-o` | 출력 파일 경로 (기본값: `<source>/skillshare-hub.json`) |
| `--full` | 전체 메타데이터 포함 (flatName, type, version 등) |
| `--audit` | 각 skill에 대해 보안 audit을 실행하고 위험 점수 포함 |
| `--project`, `-p` | 프로젝트 모드 사용 (`.skillshare/`) |
| `--global`, `-g` | 전역 모드 사용 (`~/.config/skillshare`) |
| `--help`, `-h` | 도움말 표시 |

### Output Modes

**Minimal (기본값)** — 검색 및 설치에 필요한 필수 필드만 포함:

```json
{
  "schemaVersion": 1,
  "generatedAt": "2026-02-12T10:00:00Z",
  "sourcePath": "/home/user/.config/skillshare/skills",
  "skills": [
    {
      "name": "my-skill",
      "description": "A useful skill",
      "source": "owner/repo/.claude/skills/my-skill",
      "tags": ["workflow"]
    }
  ]
}
```

**Full (`--full`)** — audit 및 관리를 위한 메타데이터 포함:

```json
{
  "name": "my-skill",
  "description": "A useful skill",
  "source": "github.com/owner/repo/.claude/skills/my-skill",
  "tags": ["workflow"],
  "flatName": "my-skill",
  "type": "github-subdir",
  "repoUrl": "https://github.com/owner/repo.git",
  "version": "abc1234",
  "installedAt": "2026-02-10T03:49:06Z",
  "isInRepo": false
}
```

**Audit (`--audit`)** — `skillshare audit`에서 보안 위험 점수 추가:

```json
{
  "name": "my-skill",
  "description": "A useful skill",
  "source": "owner/repo/.claude/skills/my-skill",
  "riskScore": 0,
  "riskLabel": "clean",
  "auditedAt": "2026-02-22T10:00:00Z"
}
```

`--audit`은 `--full`과 함께 사용하여 메타데이터와 위험 점수를 모두 포함할 수 있습니다. 위험 label: `clean` (0), `low` (1–25), `medium` (26–50), `high` (51–75), `critical` (76–100).

메타데이터 필드는 `omitempty`를 사용합니다 — 중복되는 값은 생략됩니다:
- `flatName`은 `name`과 같으면 생략
- `relPath`는 `source`와 같으면 생략
- `isInRepo`는 `false`이면 생략

### Examples

```bash
# Build minimal index (default)
skillshare hub index

# Build with full metadata
skillshare hub index --full

# Build with security risk scores
skillshare hub index --audit

# Full metadata + risk scores
skillshare hub index --full --audit

# Custom output path
skillshare hub index -o /shared/team/skillshare-hub.json

# Custom source directory
skillshare hub index -s ~/my-skills

# Project mode
skillshare hub index -p
```

### Workflow

일반적인 비공개 hub 워크플로우:

```
1. Install skills           → skillshare install ...
2. Build index              → skillshare hub index
3. Share the index file     → Commit/host skillshare-hub.json (HTTP, file, or Git repo)
4. Teammates search         → skillshare search --hub [path-url-or-ssh]
```

index가 비공개 또는 GitHub Enterprise 저장소에 있는 경우, 팀원들은 각자 로컬로 클론할 필요 없이 SSH를 통해 `--hub`를 직접 지정할 수 있습니다 (`git@host:org/repo.git`).

자세한 내용은 [Hub Index Guide](/docs/how-to/sharing/hub-index)를 참고하세요.

## Config Format

저장된 hub는 `config.yaml`의 `hub:` 키 아래에 저장됩니다:

```yaml
hub:
  default: team
  hubs:
    - label: team
      url: https://internal.corp/hub.json
    - label: local
      url: ./local-hub.json
    - label: ghe
      url: git@ghe.corp.com:team/skills.git//hubs/team.json
```

[public hub](https://github.com/runkids/skillshare-hub)는 내장된 기본값이며 별도로 저장할 필요가 없습니다. 사용자 지정 기본값이 설정되지 않은 경우, `search --hub`는 자동으로 여기로 폴백됩니다. 이 저장소를 fork하여 자체 조직의 hub를 구축하세요.
