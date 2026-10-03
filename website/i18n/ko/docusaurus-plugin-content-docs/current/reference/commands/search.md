---
sidebar_position: 5
---

# search

GitHub 저장소에서 skill을 검색하고 설치합니다.

## 언제 사용하나요

- hub 인덱스에서 커뮤니티 skill을 발견할 때
- 이름, 태그, 설명으로 skill을 찾을 때
- 설치 전에 사용 가능한 skill을 둘러볼 때

## Quick Start

```bash
skillshare search vercel       # Search by keyword
skillshare search              # Browse popular skills
```

이는 쿼리와 일치하는 `SKILL.md` 파일을 포함한 저장소를 GitHub에서 검색합니다.

## Browse Mode

쿼리가 제공되지 않으면 `search`는 GitHub에서 인기 있는 skill을 둘러봅니다.

```bash
skillshare search              # Browse popular skills
skillshare search --list       # List popular skills
```

이는 GitHub 쿼리로 `filename:SKILL.md`를 사용하고 star 수로 결과를 정렬하여 가장 인기 있는 skill 저장소를 먼저 보여줍니다.

## 동작 방식

```
skillshare search [query]
        │
        ▼
GitHub Code Search API (filename:SKILL.md + query)
        │
        ▼
Fetch star counts for each repository
        │
        ▼
Sort by stars (most popular first)
        │
        ▼
Interactive selector → Install selected skill
```

## 설치할 항목 고르기

터미널에서는 결과가 선택기로 열리고, 커서가 있는 skill의 상세 정보가 목록 옆에 표시됩니다. 여러 개를 선택해 함께 설치할 수 있으며, 아무것도 선택하지 않으면 `Enter`가 커서가 있는 skill을 설치합니다. 키는 화면 하단에 표시됩니다.

## 옵션

| Flag | Description |
|------|-------------|
| `--project`, `-p` | project 레벨 config에 설치(`.skillshare/`) |
| `--global`, `-g` | global config에 설치(`~/.config/skillshare`) |
| `--hub [URL]` | hub 인덱스에서 검색(기본값: [skillshare-hub](https://github.com/runkids/skillshare-hub); 또는 커스텀 URL/경로) |
| `--list`, `-l` | 결과만 나열, 설치 프롬프트 없음 |
| `--json` | JSON으로 출력(스크립팅용) |
| `--limit N`, `-n N` | 최대 결과 수(기본값: 20, 최대: 100) |
| `--help`, `-h` | 도움말 표시 |

:::tip 자동 감지
`--project`와 `--global` 둘 다 지정하지 않으면 skillshare가 자동으로 감지합니다: 현재 디렉터리에 `.skillshare/config.yaml`이 존재하면 project mode를 기본값으로 사용하고, 그렇지 않으면 global mode를 사용합니다.
:::

## 예시

### 인기 항목 둘러보기

```bash
skillshare search              # Browse popular skills (no query)
```

### 기본 검색

```bash
skillshare search pdf           # Interactive search and install
skillshare search "code review" # Multi-word search
```

### List Mode

```bash
skillshare search commit --list
```

출력:
```
  fix            facebook/react/.claude/skills/fix         ★ 242.7k
                 Use when you have lint errors, formatting issues...
  verify         facebook/react/.claude/skills/verify      ★ 242.7k
                 Use when you want to validate changes before committing...
  commit-helper  ...ockroach/.claude/skills/commit-helper  ★ 31.8k
                 Help create git commits and PRs with properly formatted messages...

Found 3 skills

Next
  skillshare install <source>  install one
```

### JSON Output

```bash
skillshare search react --json --limit 5
```

```json
[
  {
    "Name": "react-patterns",
    "Description": "React and Next.js performance optimization...",
    "Source": "facebook/react/.claude/skills/react-patterns",
    "Stars": 242700,
    "Owner": "facebook",
    "Repo": "react",
    "Path": ".claude/skills/react-patterns"
  }
]
```

### Project Mode

```bash
skillshare search pdf -p           # Search and install to project
skillshare search react --project  # Same thing, long flag
```

설치된 skill은 `.skillshare/skills/`로 이동하며 project config가 자동으로 업데이트됩니다. project가 아직 초기화되지 않은 경우, skillshare가 먼저 `init -p`를 실행합니다.

### 결과 수 제한

```bash
skillshare search frontend -n 5   # Show only top 5 results
```

## 인증 {#authentication}

GitHub Code Search API는 인증이 필요합니다. skillshare는 자격 증명을 자동으로 감지합니다.

1. **GitHub CLI**(권장) — `gh`로 로그인한 경우:
   ```bash
   gh auth login
   ```

2. **환경 변수** — `GITHUB_TOKEN` 또는 `GH_TOKEN` 설정:
   ```bash
   export GITHUB_TOKEN=ghp_your_token_here
   ```

### 토큰 생성

`gh` CLI를 사용하지 않는 경우:

1. [GitHub Settings → Tokens](https://github.com/settings/tokens)로 이동
2. 새 토큰 생성(classic)
3. public repo에는 scope가 필요 없음
4. 토큰 설정:
   ```bash
   export GITHUB_TOKEN=ghp_your_token_here
   ```

## 결과 순위 산정 방식

1. **Search** — GitHub Code Search가 쿼리와 일치하는 `SKILL.md` 파일을 찾습니다
2. **Filter** — fork된 저장소(중복)를 제거합니다
3. **Fetch Stars** — 각 고유 저장소의 star 수를 가져옵니다
4. **Sort** — star 수로 정렬합니다(가장 인기 있는 것부터)
5. **Limit** — 상위 N개 결과를 반환합니다

이를 통해 고품질의 인기 있는 skill이 먼저 표시됩니다.

## Community Hub

[skillshare-hub](https://github.com/runkids/skillshare-hub)에서 커뮤니티가 선별한 skill을 둘러보고 설치합니다.

```bash
skillshare search --hub                # Browse all skills in skillshare-hub
skillshare search react --hub          # Search "react" in skillshare-hub
```

`--hub`가 URL 없이 사용되면 커뮤니티 [skillshare-hub](https://github.com/runkids/skillshare-hub) 인덱스를 기본값으로 사용합니다.

skill을 커뮤니티와 공유하고 싶으신가요? [PR을 열어](https://github.com/runkids/skillshare-hub) skill을 추가하세요 — CI가 모든 제출물에 대해 `skillshare audit`을 실행합니다.

## Saved Hub Labels

[`hub add`](./hub.md#hub-add)로 hub를 저장하고 전체 URL을 입력하는 대신 레이블로 검색하세요.

```bash
# Save a hub once
skillshare hub add https://internal.corp/hub.json --label team

# Search by label
skillshare search react --hub team

# Set as default for bare --hub
skillshare hub default team
skillshare search --hub              # Uses "team" hub
```

`--hub <value>`의 해석 순서:
1. URL 또는 경로(`http`, `/`, `.`, `~`, `file://` 또는 `git@…`/`ssh://…` 같은 SSH URL로 시작) → 그대로 사용
2. 그 외 → 저장된 hub에서 레이블 조회
3. 값 없는 `--hub` → config 기본값 → 커뮤니티 hub로 대체

저장된 hub 관리는 [`hub`](./hub.md)를 참조하세요.

## Private Index Search {#private-index-search}

GitHub 대신 private hub 인덱스에서 검색합니다.

```bash
# Local file
skillshare search react --hub ./skillshare-hub.json

# HTTP URL
skillshare search react --hub https://internal.corp/skills/skillshare-hub.json

# SSH URL — clones the repo and reads the index (works with private/GHE hosts)
skillshare search react --hub git@github.com:org/skills.git
skillshare search react --hub git@ghe.corp.com:team/skills.git//hubs/team.json

# Browse all skills (empty query)
skillshare search --hub ./skillshare-hub.json --json

# Equals syntax also works
skillshare search react --hub=./skillshare-hub.json
```

:::note SSH hub sources
SSH `--hub` 값은 저장소를 shallow-clone하여(SSH agent/키 사용) 그 안의 인덱스 파일을 읽는 방식으로 해석됩니다. 저장소 내부의 파일 경로는 `//path` 접미사에서 가져오며 — `git@host:org/repo.git//hubs/team.json` — 생략된 경우 저장소 루트의 `skillshare-hub.json`을 기본값으로 사용합니다. scp 스타일(`git@host:org/repo.git`)과 scheme 스타일(`ssh://git@host/org/repo.git`) URL 모두 지원됩니다.

[web dashboard](./ui.md)에서는 SSH hub source를 [먼저 저장](./hub.md#hub-add)해야 합니다; 서버는 저장된 hub만 clone합니다.
:::

[`hub index`](./hub.md)로 인덱스를 구축하세요.

```bash
skillshare hub index                           # Generate skillshare-hub.json
skillshare search --hub ./skillshare-hub.json  # Search it
```

:::tip Default hub
`skillshare search --hub`(URL 없이)는 커뮤니티 [skillshare-hub](https://github.com/runkids/skillshare-hub) 인덱스를 기본값으로 사용하므로 매번 전체 URL을 입력할 필요가 없습니다. 또는 `skillshare hub default <label>`로 자신만의 기본값을 설정하세요.
:::

자세한 내용은 [Hub Index Guide](/docs/how-to/sharing/hub-index)를 참조하세요.

## 팁

### 공식 Skill 찾기

잘 알려진 조직을 검색하세요.
```bash
skillshare search anthropic    # Anthropic's skills
skillshare search facebook     # Meta/Facebook skills
skillshare search vercel       # Vercel's skills
```

### 특정 기능 찾기

원하는 작업으로 검색하세요.
```bash
skillshare search "pull request"
skillshare search deployment
skillshare search testing
skillshare search database
```

### 연속 검색

결과가 없거나 선택기에서 `Esc`를 누르면 새 키워드를 묻습니다.

```
? Search again (or press Enter to quit): react
```

## 문제 해결

### "GitHub Code Search API requires authentication"

`gh auth login`을 실행하거나 `GITHUB_TOKEN`을 설정하세요. [인증](#authentication)을 참조하세요.

### "GitHub API rate limit exceeded"

- 인증된 사용자: Code Search 기준 분당 30 요청
- 1분 정도 기다린 후 다시 시도하세요
- API 호출을 줄이려면 `--limit`을 사용하세요

### 새 저장소를 찾을 수 없음

GitHub는 새 저장소를 지연을 두고 인덱싱합니다(몇 시간에서 며칠). 저장소를 찾을 수 없다면:
- 직접 설치: `skillshare install owner/repo/path/to/skill`
- GitHub가 저장소를 인덱싱할 때까지 기다리기

### 결과가 쿼리와 일치하지 않음

GitHub Code Search는 `SKILL.md` 파일 내부의 콘텐츠를 매칭합니다. 설명에 "vercel"을 언급하는 skill은 Vercel에 관한 것이 아니더라도 vercel 검색 결과에 나타날 수 있습니다.

설치 전에 결과를 검토하려면 `--list`를 사용하세요.
