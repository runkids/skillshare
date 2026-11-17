---
sidebar_position: 2
---

# Skill 만들기

아이디어부터 게시된 skill까지.

:::tip
어떤 target이 skill을 받을지 제어하고 싶으신가요? [Skill 필터링](/docs/how-to/daily-tasks/filtering-skills)을 참고하세요.
:::

## 개요

```mermaid
flowchart LR
    IDEA["IDEA"] --> CREATE["CREATE"] --> WRITE["WRITE"] --> TEST["TEST"] --> PUBLISH["PUBLISH"]
```

---

## 1단계: Skill 생성

```bash
skillshare new my-skill
```

이것은 다음을 생성합니다.
```
~/.config/skillshare/skills/my-skill/
└── SKILL.md  (템플릿 포함)
```

---

## 2단계: Skill 작성

생성된 `SKILL.md`를 편집하세요.

```bash
$EDITOR ~/.config/skillshare/skills/my-skill/SKILL.md
```

### 기본 구조

```markdown
---
name: my-skill
description: Brief description (shown in skill lists)
---

# My Skill

What this skill does and when to use it.

## Instructions

1. Step one
2. Step two
3. Step three
```

### 좋은 Skill 작성 팁

**구체적으로 작성하세요:**
```markdown
# Good
When the user asks to review code, analyze for:
- Bugs and potential issues
- Style consistency
- Performance concerns

# Bad
Review the code and make it better.
```

**예시를 포함하세요:**
````markdown
## Example

User: "Review this function"
```python
def add(a, b):
    return a + b
```

Response: Suggest adding type hints...
````

**사용하지 않아야 할 때를 명시하세요:**
```markdown
## When NOT to Use

- Don't use for simple syntax questions
- Don't use for explaining code (use explain-code skill instead)
```

---

## 3단계: 배포 및 테스트

### 모든 target에 배포

```bash
skillshare sync
```

### AI CLI에서 테스트

skill을 사용해 보세요.
- 명시적으로 호출: `/skill:my-skill`
- 또는 작업을 설명하고 AI가 이를 인식하는지 확인

### 반복

작동이 잘 될 때까지 편집 → sync → 테스트를 반복하세요.

---

## 4단계: 게시 (선택 사항)

### 팀과 공유

git remote로 push하세요.
```bash
skillshare push -m "Add my-skill"
```

팀원은 pull할 수 있습니다.
```bash
skillshare pull
```

### 공개 공유

1. skill을 위한 GitHub repo 생성
2. skill 디렉터리 push
3. 다른 사람이 install할 수 있습니다.
   ```bash
   skillshare install github.com/you/my-skills/my-skill
   ```

---

## Skill 템플릿

### 단순 Skill

```markdown
---
name: simple-skill
description: Does one thing well
---

# Simple Skill

When the user asks to do X, follow these steps:

1. First, do Y
2. Then, do Z
3. Finally, confirm completion
```

### 작업 지향 Skill

```markdown
---
name: code-review
description: Reviews code for quality and issues
---

# Code Review

You are a code reviewer. Analyze code for quality issues.

## What to Check

- Bugs and edge cases
- Performance issues
- Security vulnerabilities
- Code style and readability

## Output Format

For each issue found:
1. **Location**: File and line
2. **Severity**: High/Medium/Low
3. **Issue**: What's wrong
4. **Fix**: Suggested solution

## Example

[Include an example input and expected output]
```

### Target 전용 Skill

```markdown
---
name: claude-prompts
description: Prompt patterns specific to Claude Code
targets: [claude]
---

# Claude Prompts

Patterns that work best with Claude Code's capabilities.

## When to Use

Use when crafting prompts for Claude Code specifically.
```

`targets`가 설정되면 skill은 일치하는 target에만 sync됩니다 — 다른 target은 받지 않습니다. 모든 곳에 sync하려면 `targets`를 생략하세요.

### 프로세스 Skill

```markdown
---
name: git-workflow
description: Guides through git commit workflow
---

# Git Workflow

Guide the user through proper git commit practices.

## Steps

1. **Check status**: Run `git status`
2. **Review changes**: Run `git diff`
3. **Stage files**: Add specific files, not `git add .`
4. **Write message**: Follow conventional commits
5. **Commit**: Create the commit
6. **Verify**: Run `git log -1`

## Commit Message Format

```text
type(scope): description

[optional body]
```

Types: feat, fix, docs, style, refactor, test, chore

---

## 고급 주제

### Skill 내 다중 파일

skill은 추가 파일을 포함할 수 있습니다.

```
my-skill/
├── SKILL.md
├── examples/
│   └── sample.py
└── templates/
    └── component.tsx
```

SKILL.md에서 이를 참조하세요.
```markdown
See the example in `examples/sample.py` for reference.
```

### 팀을 위한 네임스페이싱

네임스페이스가 적용된 이름으로 충돌을 피하세요.

```yaml
name: acme-code-review
```

### 버전 추적

버전 메타데이터를 추가하세요.

```yaml
---
name: my-skill
description: My skill
metadata:
  version: 1.0.0
  author: Your Name
---
```

### License 메타데이터

사용자가 install 전에 license 정보를 볼 수 있도록 `license` 필드를 추가하세요.

```yaml
---
name: my-skill
description: My reusable skill
license: MIT
---
```

이 필드가 있으면 `skillshare install`은 선택 프롬프트와 확인 화면에 license를 표시합니다. 이는 기업 사용자의 규정 준수 결정에 도움이 됩니다. 자세한 내용은 [Skill 형식](/docs/understand/skill-format#license)을 참고하세요.

### .skillignore로 discovery 제어하기

다중 skill repo를 게시할 때, 사용자가 발견하지 않았으면 하는 내부 도구나 작업 중인 skill이 있을 수 있습니다. repo 루트에 `.skillignore` 파일을 만드세요.

```text title=".skillignore"
# Internal tooling
validation-scripts
scaffold-template

# Exclude an entire group directory
internal-tools/

# Work in progress
prompt-eval-*

# Ignore temp at any depth
**/temp

# Exclude tests but keep test-critical
test-*
!test-critical
```

`.skillignore`는 [gitignore 문법](https://git-scm.com/docs/gitignore)을 사용합니다 — `*`, `**`, `?`, `[abc]`, `!negation`, `/anchored`, `pattern/` (디렉터리 전용), `\#`/`\!` 이스케이프를 지원합니다. `internal-tools`와 같은 그룹 이름은 해당 디렉터리 아래의 **모든** skill을 제외합니다. 그룹 내 특정 skill만 제외하려면 `internal-tools/helper`처럼 정확한 경로를 사용하세요.

이 패턴과 일치하는 skill은 `skillshare install <repo>` discovery에 나타나지 않습니다. 이는 서버 측(repo 내)에서 적용되므로 모든 사용자가 자동으로 혜택을 받습니다. 실제 예시는 [`runkids/my-skills`](https://github.com/runkids/my-skills)를, 사용자 측 제외는 [install --exclude](/docs/reference/commands/install#excluding-skills)를 참고하세요.

### Source-root .skillignore (로컬)

**source root**(`~/.config/skillshare/skills/.skillignore`)에도 `.skillignore`를 두어 모든 명령어 — `doctor`, `status`, `list`, `sync`, `audit`, `diff`, `check` — 에서 전역적으로 skill을 숨길 수 있습니다.

```text title="~/.config/skillshare/skills/.skillignore"
# Temporarily mute a skill without uninstalling
my-experimental-skill

# Exclude all draft skills
[Dd]raft*

# Hide an entire tracked repo
_archived-team-skills

# Ignore vendored deps at any depth
**/node_modules
*.venv
```

두 계층이 모두 적용됩니다. source-root 패턴은 모든 skill(tracked 및 non-tracked)에 영향을 미치고, repo 수준 패턴은 해당 repo의 skill에만 영향을 미칩니다. 어느 한쪽이라도 일치하면 skill이 제외됩니다.

### .skillignore.local (개인 재정의)

공유 repo의 `.skillignore`가 로컬에서 필요한 skill을 차단한다면 같은 디렉터리에 `.skillignore.local`을 만드세요. 이 패턴은 `.skillignore` 이후에 추가되므로, `!pattern` negation이 기본 파일을 재정의합니다.

```text title="_team-skills/.skillignore.local"
# The repo ignores private-*, but I need my own
!private-mine
```

이 파일은 커밋하지 **않아야** 합니다 — `.gitignore`에 추가하세요. source root와 repo 수준 모두에서 작동합니다.

---

## 체크리스트

게시하기 전:

- [ ] 명확하고 구체적인 이름
- [ ] 목적을 설명하는 description
- [ ] 실행 가능한 지시문
- [ ] 예시 포함
- [ ] AI CLI에서 테스트됨
- [ ] 기존 skill과 충돌 없음

---

## 참고

- [new](/docs/reference/commands/new) — 템플릿으로 skill 생성
- [Skill 형식](/docs/understand/skill-format) — SKILL.md 구조
- [Skill 설계](/docs/understand/philosophy/skill-design) — 복잡도 레벨, 결정성, CLI wrapper 패턴
- [모범 사례](./best-practices.md) — 네이밍과 조직화
- [Skill 조직화](./organizing-skills.md) — 폴더 구조
