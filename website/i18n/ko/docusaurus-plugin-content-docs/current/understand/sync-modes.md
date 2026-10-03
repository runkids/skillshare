---
sidebar_position: 3
---

# Sync Modes

skillshare가 source를 target에 연결하는 방식.

:::tip 언제 중요한가요?
skill별 symlink를 원하고 target의 로컬 skill을 보존하고 싶다면 merge mode(기본값)를 선택하세요. symlink 대신 실제 파일이 필요하다면(이식성, CI, 개인 취향) copy mode를 선택하세요. 디렉터리 전체를 연결하고 target의 로컬 skill이 필요 없다면 symlink mode를 선택하세요.
:::

## 개요

| Mode | 동작 | 사용 사례 |
|------|----------|----------|
| `merge` | 각 skill이 개별적으로 symlink됨 | **기본값.** 로컬 skill을 보존합니다. |
| `copy` | 각 skill이 실제 파일로 복사됨 | 이식성, CI/샌드박스 환경, 또는 symlink보다 실제 파일을 선호하는 경우. |
| `symlink` | 디렉터리 전체가 하나의 symlink | 모든 곳에서 정확히 동일한 사본. |

## 결정 매트릭스 (중립적)

target 브랜드 이름이 아니라 제약 조건에 따라 선택할 때 이 표를 사용하세요:

| 결정 축 | `merge` | `copy` | `symlink` |
|---|---|---|---|
| 서로 다른 AI CLI 간 호환성 | 중간 | 높음 | 낮음–중간 |
| 편집 즉시 반영 | 높음 | 낮음 (`sync` 필요) | 높음 |
| 디스크 사용량 | 낮음 | 높음 | 낮음 |
| target에서 실수로 삭제되는 것에 대한 안전성 | 높음 | 높음 | 낮음 |
| 운영상의 단순함 | 중간 | 중간 | 높음 |
| target별 필터링 (`include`/`exclude`) | 예 | 예 | 아니오 |

확신이 서지 않는다면 `merge`로 시작한 뒤 필요에 따라 특정 target을 `copy`로 전환하세요.

---

## Merge Mode (기본값)

각 skill이 개별적으로 symlink됩니다. target에 있는 로컬 skill은 보존됩니다.

```
Source                          Target (claude)
─────────────────────────────────────────────────────────────
skills/                         ~/.claude/skills/
├── my-skill/        ────────►  ├── my-skill/ → (symlink)
├── another/         ────────►  ├── another/  → (symlink)
└── ...                         ├── local-only/  (preserved)
                                └── .skillshare-manifest.json
```

**장점:**
- target 전용 skill을 유지 가능 (sync되지 않음)
- 설치된 skill과 로컬 skill을 혼용 가능
- 세밀한 제어
- target별 include/exclude 필터링
- manifest 기반 orphan 정리 (uninstall 후 symlink가 아닌 잔여물도 안전하게 제거)

:::info Project mode에서의 상대 symlink
project mode(`-p`)에서는 symlink가 절대 경로가 아닌 **상대 경로**로 생성됩니다(예: `../../.skillshare/skills/my-skill`). 이로 인해 프로젝트가 이식 가능해집니다 — 디렉터리를 옮기거나 이름을 바꿔도 symlink는 계속 작동합니다. global mode에서는 source와 target이 서로 다른 위치에 있으므로 절대 경로가 사용됩니다.
:::

**언제 사용하나요:**
- 일부 skill을 특정 AI CLI에만 두고 싶을 때
- sync하기 전에 로컬 skill을 먼저 시험해보고 싶을 때
- 하나의 source에서 target마다 다른 skill 집합을 원할 때

### Merge mode에서의 필터 전략

`include`와 `exclude`는 target별로 다음 순서로 평가됩니다:
1. `include`가 일치하는 이름을 유지
2. `exclude`가 유지된 집합에서 제거

빠른 선택 가이드:
- target이 작은 하위 집합만 받아야 한다면 `include` 사용
- target이 거의 전부를 받아야 한다면 `exclude` 사용
- 넓은 하위 집합에 명시적인 예외가 필요하다면 `include + exclude` 사용

규칙이 변경될 때의 동작:
- 이전에 sync되었던 source 연결 항목이 필터링되어 제외되면 다음 `sync` 시 제거됨
- target에 있는 기존 로컬 non-symlink 폴더는 보존됨

전체 예시는 [Target Configuration](/docs/reference/targets/configuration#include--exclude-target-filters)을 참고하세요.

---

## Copy Mode

각 skill이 target 디렉터리에 실제 파일로 복사됩니다. `.skillshare-manifest.json` 파일이 관리 중인 skill과 체크섬을 추적하므로 로컬 skill이 보존됩니다.

```
Source                          Target (cursor)
─────────────────────────────────────────────────────────────
skills/                         ~/.cursor/skills/
├── my-skill/        ────copy►  ├── my-skill/    (real files)
├── another/         ────copy►  ├── another/     (real files)
└── ...                         ├── local-only/  (preserved)
                                └── .skillshare-manifest.json
```

### 왜 copy mode인가?

AI CLI가 symlink를 제대로 처리하더라도, copy mode는 다음과 같은 가치를 제공합니다:

- **방어적 설계** — 모든 AI CLI가 symlink를 지원한다고 보장하지는 않습니다. 특히 Windows에서는 플랫폼과 권한 수준에 따라 symlink 동작이 달라집니다
- **샌드박스 환경** — 엄격한 CI 파이프라인, 컨테이너, 에어갭 환경은 파일시스템 경계를 넘는 symlink를 따라가지 못할 수 있습니다
- **사용자 선호** — 일부 사용자와 팀은 투명성과 이식성을 위해 symlink보다 실제 파일을 단순히 선호합니다

**장점:**
- 어디서나 동작 — AI CLI나 OS의 symlink 지원이 필요 없음
- 로컬 skill 보존 (merge mode와 동일)
- target별 include/exclude 필터링
- 체크섬 기반 스킵: 변경되지 않은 skill은 다시 복사되지 않음

**언제 사용하나요:**
- AI CLI가 "skill not found"를 보고하거나 symlink된 skill을 읽지 못할 때
- 프로젝트 저장소에 skill을 vendor로 넣고 싶을 때 — project mode에서의 copy mode는 팀이 실제 skill 파일을 git에 커밋하게 해주므로, 팀원이 skillshare를 설치할 필요가 없습니다
- 중앙 source 없이도 동작하는 독립적인 skill 디렉터리가 필요할 때 (이식 가능한 설정, CI 파이프라인, 에어갭 환경)
- merge mode와 동일한 필터링 동작을 실제 파일로 원할 때
- `copy`의 대표적인 첫 후보: `cursor`, `antigravity`, `copilot`, `opencode`

### 업데이트 동작 방식

`skillshare sync`를 실행할 때마다 각 source skill의 체크섬을 manifest에 저장된 값과 비교합니다:

- **체크섬이 같음** → skill을 건너뜀 (빠름)
- **체크섬이 다름** → skill을 새 버전으로 덮어씀
- **`--force`** → 체크섬과 관계없이 관리 중인 모든 skill을 덮어씀

### Manifest 생명주기

merge와 copy mode 모두 관리 중인 skill을 추적하기 위해 `.skillshare-manifest.json`을 작성합니다:

- **Merge mode**: skill 이름을 `"symlink"` 값으로 기록 — uninstall 후 orphan 실제 디렉터리(예: copy mode 잔여물)를 안전하게 정리하는 데 사용됨
- **Copy mode**: skill 이름을 SHA-256 체크섬과 함께 기록 — 증분 sync와 orphan 감지에 사용됨
- symlink mode로 전환하면 자동으로 제거됨
- 수동으로 삭제된 경우, 다음 `sync`에서 재생성됨

---

## Symlink Mode

전체 target 디렉터리가 source를 가리키는 하나의 symlink가 됩니다.

```
Source                          Target (claude)
─────────────────────────────────────────────────────────────
skills/              ────────►  ~/.claude/skills → (symlink to source)
├── my-skill/
├── another/
└── ...
```

**장점:**
- 모든 target이 동일함
- 관리가 더 간단함
- orphan symlink가 없음

**언제 사용하나요:**
- 모든 AI CLI가 정확히 동일한 skill을 갖기를 원할 때
- target별 skill이 필요 없을 때

**경고:** symlink mode에서는 target을 통해 삭제하면 source도 삭제됩니다!
```bash
rm -rf ~/.claude/skills/my-skill  # ❌ Deletes from SOURCE
skillshare target remove claude   # ✅ Safe way to unlink
```

---

## Mode 변경

### Target별

```bash
# Switch to copy mode (for AI CLIs that can't read symlinks)
skillshare target cursor --mode copy
skillshare sync

# Switch to symlink mode
skillshare target claude --mode symlink
skillshare sync

# Switch back to merge mode
skillshare target claude --mode merge
skillshare sync
```

### Target별 재정의 (권장)

모든 target에 대해 하나의 global mode를 사용할 필요는 없습니다. 흔한 패턴은 다음과 같습니다:

```yaml
mode: merge
targets:
  claude:
    path: ~/.claude/skills
    # inherits merge
  cursor:
    path: ~/.cursor/skills
    mode: copy
  codex:
    path: ~/.codex/skills
    mode: symlink
```

한 target은 호환성 우선 동작(`copy`)이 필요하고 다른 target은 즉시 반영(`merge`/`symlink`)을 유지하고 싶을 때 target별 재정의를 사용하세요.

### 기본 Mode

새 target에 대해 config에서 설정합니다:

```yaml
# ~/.config/skillshare/config.yaml
mode: merge  # or symlink or copy

targets:
  claude:
    path: ~/.claude/skills
    # inherits default mode

  cursor:
    path: ~/.cursor/skills
    mode: copy  # real files for Cursor

  codex:
    path: ~/.codex/skills
    mode: symlink  # override default
```

---

## Target Naming

merge나 copy mode를 사용할 때 target에서 skill 디렉터리 이름을 어떻게 지정할지 제어합니다.

| Naming | 동작 |
|--------|--------|
| `flat` (기본값) | 중첩된 skill이 `__` 구분자로 평탄화됨: `frontend/dev` → `frontend__dev` |
| `standard` | SKILL.md의 `name` 필드를 사용: `frontend/dev` → `dev` |

전역 또는 target별로 설정합니다:

```yaml
target_naming: standard    # global default
targets:
  claude:
    skills:
      target_naming: flat  # per-target override
```

또는 CLI를 통해:

```bash
skillshare target claude --target-naming standard
skillshare sync
```

**Standard mode**는 [Agent Skills specification](https://agentskills.io/specification)을 따르며, SKILL.md의 `name` 필드가 부모 디렉터리 이름과 일치해야 합니다. 이름이 유효하지 않거나 이름이 충돌하는 skill은 경고와 함께 건너뛰어집니다.

**마이그레이션**: `flat`에서 `standard`로 전환하면 기존에 관리되던 항목의 이름이 자동으로 변경됩니다. 로컬 skill이 이미 짧은 이름을 차지하고 있다면 기존 flat 항목이 보존됩니다.

**Symlink mode**: `target_naming`은 무시됩니다 — 디렉터리 전체가 그대로 연결됩니다.

---

## Mode 비교

| 측면 | Merge | Copy | Symlink |
|--------|-------|------|---------|
| 로컬 skill 보존 | ✅ 예 | ✅ 예 | ❌ 아니오 |
| Symlink 호환성 | ✅ 예 | ❌ 실제 파일 | ✅ 예 |
| 모든 target이 동일 | ❌ 다를 수 있음 | ❌ 다를 수 있음 | ✅ 예 |
| target별 include/exclude | ✅ 예 | ✅ 예 | ❌ 무시됨 |
| orphan 정리 필요 | ✅ 예 | ✅ 예 | ❌ 아니오 |
| 삭제 안전성 | ✅ 안전 | ✅ 안전 | ⚠️ 주의 필요 |
| 디스크 사용량 | 낮음 (symlink) | 높음 (사본) | 낮음 (symlink) |

---

## Orphan 정리

merge와 copy mode 모두, `sync`는 자동으로 orphan을 정리합니다:

- 삭제된 source skill을 가리키는 **symlink**는 항상 제거됨
- `.skillshare-manifest.json`에 등록된 (이전에 skillshare가 관리하던) **실제 디렉터리**는 제거됨
- manifest에 없는 **알 수 없는 디렉터리**는 경고와 함께 보존됨 (사용자가 만든 것으로 간주)

이는 `uninstall` + `sync` 이후, symlink가 아닌 잔여물(예: 이전 `copy` mode에서 남은 디렉터리)도 안전하게 정리된다는 뜻입니다.

```
$ skillshare sync
✓ claude    5 linked · 2 local · 1 pruned
✓ cursor    3 copied · 2 up to date · 1 pruned
```

:::info Agent도 동일한 mode를 따릅니다
세 가지 mode(merge, copy, symlink) 모두 agent sync에도 적용됩니다. Agent orphan 정리, target별 include/exclude 필터링, mode 전환은 skill과 동일하게 동작합니다 — 유일한 차이는 agent가 디렉터리가 아닌 단일 `.md` 파일이라는 점입니다. Agent를 지원하는 target(Claude, Cursor, Augment, OpenCode)은 `agents:` 하위 키에서 동일한 `mode` 설정을 따릅니다. 자세한 내용은 [Agents](./agents.md)를 참고하세요.
:::

---

## Extras Sync Modes

Extras(rules, commands, prompts 같은 skill이 아닌 리소스)도 merge와 copy mode를 사용합니다. 각 extras target은 자체 mode를 지정할 수 있습니다:

```yaml
extras:
  - name: rules
    targets:
      - path: ~/.claude/rules          # merge (default): per-file symlinks
      - path: ~/.cursor/rules
        mode: copy                     # copy: real file copies
```

동작 방식은 skill sync mode와 동일합니다 — merge는 파일별 symlink를 생성하고, copy는 실제 파일 사본을 생성합니다.

:::note Developer Mode가 없는 Windows
merge mode는 단일 파일을 링크하는데, Windows는 Developer Mode에서만 이를 허용합니다. Developer Mode가 없으면 merge mode의 agents와 extras는 대신 복사되며, 이 복사본은 링크처럼 업데이트되고 pruning됩니다. skills는 폴더이므로 어느 경우든 (junction으로) 링크됩니다. [Windows 문제 해결](/docs/troubleshooting/windows#file-links-need-windows-developer-mode-copying-instead)을 참고하세요.

skillshare가 소유하지 않는 내용이 같은 로컬 파일은 유지됩니다. copy fallback에서 agent 개수는 이를 `local preserved`로 따로 표시합니다(예: `0/1 linked, 1 local preserved`).
:::

---

## 참고

- [sync](/docs/reference/commands/sync) — sync를 실행해 mode 변경 사항을 적용
- [target](/docs/reference/commands/target) — target의 sync mode 변경
- [Source & Targets](./source-and-targets.md) — 핵심 아키텍처
- [Configuration](/docs/reference/targets/configuration) — target별 설정
