---
sidebar_position: 3
---

# 필터링 레퍼런스

어떤 Skill이 어떤 Target에 도달할지를 제어하는 세 가지 필터링 레이어에 대한 완전한 명세입니다.

:::tip 빠른 가이드가 필요하신가요?
시나리오 중심의 가이드는 [Filtering Skills](/docs/how-to/daily-tasks/filtering-skills)를 참고하세요.
:::

## 개요

| 레이어 | 범위 | 설정 위치 | 문법 | 평가 시점 |
|-------|-------|-------------|--------|-------------|
| `.skillignore` | 모든 Target에서 숨김 | Source 디렉터리 또는 Tracked 저장소 루트 | [gitignore](https://git-scm.com/docs/gitignore) | Discovery |
| SKILL.md `metadata.targets` | 나열된 Target으로만 Skill 제한 | Skill별 frontmatter | YAML 목록 | Sync (Discovery 시 파싱) |
| Agent `targets` | 나열된 Target으로만 Agent 제한 | Agent별 frontmatter | YAML 목록 | Sync (Discovery 시 파싱) |
| Target include/exclude | Target별, 리소스별 | `config.yaml` 또는 CLI 플래그 | Go [`filepath.Match`](https://pkg.go.dev/path/filepath#Match) glob | Sync |

:::note Sync 모드 주의사항
이 세 가지 레이어는 모두 **merge** 및 **copy** Sync 모드에만 적용됩니다.
**symlink** 모드에서는 전체 Source 디렉터리가 하나의 단위로 링크되므로 — Skill별 필터링은 효과가 없습니다.
:::

## 평가 순서와 우선순위

Skill이 Target에 도달하려면 **모든** 레이어를 통과해야 합니다:

1. **`.skillignore`** — Discovery 시 평가됩니다. 일치하는 Skill은 Sync 파이프라인에 진입하지 않습니다.
2. **Target include/exclude** — Sync 시(`FilterSkills`) 평가됩니다. Skill은 Discovery되지만 일치하지 않는 Target에서는 건너뜁니다.
3. **SKILL.md `metadata.targets`** — Sync 시(`FilterSkillsByTarget`) 평가됩니다. Skill은 선언된 Target으로만 제한됩니다.

## .skillfollow

[`.skillfollow`](./skillfollow.md)는 필터가 아닌 검색 opt-in입니다. Skills source 루트에서 첫 계층 외부 그룹/tracked repo 링크 이름을 선언합니다. Local은 이름 합집합이며 gitignore pattern이 아닙니다. 링크의 루트 기준/끝 `/` 없는 ignore와 `/.skillfollow.local`을 추가하세요. Root/repo `.skillignore`는 계속 적용됩니다. [설정과 모든 상태](./skillfollow.md#states)를 확인하세요.

## .skillignore

**위치:**
- Source 루트: `~/.config/skillshare/skills/.skillignore` — 모든 Skill에 적용
- Tracked 저장소 루트: `_team-repo/.skillignore` — 해당 저장소 내에서만 적용. source 안의 깊이는 무관(`--into group/sub`로 설치한 저장소나 followed 그룹 안의 저장소도 포함). 저장소가 중첩되면 가장 안쪽 저장소의 규칙이 적용

**문법:** 전체 [gitignore](https://git-scm.com/docs/gitignore) — `*`(단일 세그먼트), `**`(임의 깊이), `?`, `[abc]`, `!pattern`(부정), `/pattern`(경로 고정), `pattern/`(디렉터리 전용).

**`.skillignore.local`:** `.skillignore`와 함께 위치시킵니다. 패턴은 기본 파일 뒤에 추가되며 — 마지막에 일치하는 규칙이 우선합니다. `!pattern`으로 제외를 해제할 수 있습니다. 이 파일은 커밋하지 마세요. `/`가 없는 패턴은 어느 깊이의 같은 이름에도 일치하므로 `!feature-radar`는 `feature-radar/feature-radar`도 다시 활성화합니다. 최상위 폴더만 일치시키려면 `!/feature-radar`라고 쓰세요. 대시보드는 이 파일의 규칙을 **Settings → Files**에 보여 주고, 이 파일이 덮어쓴 전환을 실패로 보고합니다.

**CLI에서의 표시:**

| 명령어 | 출력 |
|---------|--------|
| `skillshare sync` | 개수 + Skill 이름 |
| `skillshare status --json` | 패턴과 제외된 목록이 담긴 `source.skillignore` 객체 |
| `skillshare doctor` | 패턴 개수와 제외된 개수 |

📖 [File structure reference](/docs/reference/appendix/file-structure#skillignore-optional)

## SKILL.md targets 필드 {#skillmd-targets-field}

**형식:** 최상위 또는 `metadata` 아래에 중첩:

```yaml
# 권장
metadata:
  targets: [claude, cursor]

# 레거시 대체 형식
targets: [claude, cursor]
```

**동작:** 화이트리스트 — 해당 Skill은 나열된 Target에만 Sync됩니다. 필드를 생략하면 모든 Target에 Sync됩니다. `metadata.targets`와 최상위 `targets`가 모두 존재하는 경우 `metadata.targets`가 우선합니다.

**Tracked repo:** dashboard에서 tracked repo 안의 skill에 설정한 targets는 해당 SKILL.md가 아니라 source의 `.metadata.json`에 저장됩니다. 그래서 clone은 깨끗하게 유지되고 `update`도 정상적으로 동작합니다. 이 설정은 skill의 `metadata.targets`보다 우선합니다.

**별칭(Alias):** Target 이름은 별칭을 지원합니다. `claude`는 `claude-code`로 설정된 Target과도 일치합니다. [Supported Targets](/docs/reference/targets/supported-targets)를 참고하세요.

📖 [Skill format — targets field](/docs/understand/skill-format#targets)

**Agent**도 Agent frontmatter의 최상위 `targets` 목록을 통해 동일한 화이트리스트를 지원합니다. 필드가 없는 Agent는 Agent를 지원하는 모든 Target에 Sync됩니다. [Agents — Agent File Format](/docs/understand/agents#agent-file-format)을 참고하세요.

## Target include/exclude 필터 {#target-includeexclude-filters}

**CLI로 설정:**

```bash
# Skill
skillshare target claude --add-include "team-*"
skillshare target cursor --add-exclude "legacy-*"
skillshare target claude --remove-include "team-*"

# Agent
skillshare target claude --add-agent-include "team-*"
skillshare target claude --add-agent-exclude "draft-*"
skillshare target claude --remove-agent-include "team-*"
```

**저장 위치:** Skill의 경우 `config.yaml`의 `targets.<name>.include` / `targets.<name>.exclude`, Agent의 경우 `targets.<name>.agents.include` / `targets.<name>.agents.exclude`.

**문법:** 평탄화된 리소스 이름에 대해 매칭되는 Go [`filepath.Match`](https://pkg.go.dev/path/filepath#Match) glob 패턴입니다. Skill은 평탄화된 Skill 이름(예: `_team__frontend__ui`)을 사용하고, Agent는 평탄화된 `.md` 파일 이름을 사용합니다.

| 지원됨 | 지원되지 않음 |
|-----------|--------------|
| `*` (임의 문자) | `**` (재귀) |
| `?` (단일 문자) | `{a,b}` (중괄호 확장) |
| `[abc]` (문자 클래스) | |

**우선순위:** `include`와 `exclude`가 모두 설정된 경우, `include`가 먼저 적용된 다음 `exclude`가 적용됩니다. 둘 다에 일치하는 리소스는 제외됩니다.

**시각적 편집기:** `skillshare ui` → Targets 페이지 → "Customize filters" 버튼.

📖 [Target command](/docs/reference/commands/target#target-filters-includeexclude) · [Filter behavior examples](/docs/reference/commands/sync#filter-behavior-examples) · [Configuration](/docs/reference/targets/configuration#include--exclude-target-filters)

## 참고

- [Filtering Skills](/docs/how-to/daily-tasks/filtering-skills) — 시나리오 중심의 how-to 가이드
