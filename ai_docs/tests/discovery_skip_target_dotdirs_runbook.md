# Discovery: Skip Target Dotdirs Runbook

Verifies that `discoverSkills` skips known target directories (`.claude/`, `.cursor/`, `.skillshare/`) during install discovery, while still finding skills in non-target hidden directories (`.curated/`, `.system/`). A repository-root `.agents/skills/` is discovered only when the repository has no other skills.

## Scope

- `internal/install/install_discovery.go` — `discoverSkills` skip logic
- `internal/sourcewalk/walk.go` — `AgentsSkillsDir` repository-root `.agents/skills` rule
- `internal/sync/discover_walk.go` — the same rule for tracked repos during sync
- `internal/config/targets.go` — `ProjectTargetDotDirs()` function
- `cmd/skillshare/main.go` — injection of target dotdirs at startup

## Environment

- Devcontainer with `ss` binary
- ssenv-isolated HOME (created with `--init`)
- Local `file://` bare repos at `/tmp/e2e-dotdir-test/`

## Steps

### Step 1: Create repos for testing

Creates five bare repos:
1. `repo1.git` — source skills + `.claude/` and `.cursor/` target copies
2. `repo2.git` — non-target hidden dirs (`.curated/`, `.system/`) + `.claude/` target copy
3. `repo3.git` — normal skill + `.skillshare/` config dir with stray SKILL.md
4. `repo4.git` — `skills/` + `.agents/skills/` copies of the same names
5. `repo5.git` — skills only in `.agents/skills/` (issue #800)

```bash
BASE=/tmp/e2e-dotdir-test
rm -rf "$BASE"
mkdir -p "$BASE"

# --- Repo 1: target dotdir duplicates ---
git init --bare "$BASE/repo1.git"
W1=$(mktemp -d)
cd "$W1" && git init && git config user.email "t@t" && git config user.name "T"
for name in adapt polish optimize; do
  mkdir -p "source/skills/$name"
  echo "---
name: $name
---
# $name" > "source/skills/$name/SKILL.md"
  mkdir -p ".claude/skills/$name"
  cp "source/skills/$name/SKILL.md" ".claude/skills/$name/SKILL.md"
done
mkdir -p ".cursor/skills/adapt"
cp source/skills/adapt/SKILL.md .cursor/skills/adapt/SKILL.md
git add -A && git commit -m "init"
git remote add origin "$BASE/repo1.git" && git push origin master

# --- Repo 2: non-target hidden dirs ---
git init --bare "$BASE/repo2.git"
W2=$(mktemp -d)
cd "$W2" && git init && git config user.email "t@t" && git config user.name "T"
for name in skill-alpha skill-beta; do
  mkdir -p ".curated/$name"
  echo "---
name: $name
---
# $name" > ".curated/$name/SKILL.md"
done
mkdir -p ".system/skill-gamma"
echo "---
name: skill-gamma
---
# g" > ".system/skill-gamma/SKILL.md"
mkdir -p ".claude/skills/skill-alpha"
echo "---
name: skill-alpha
---
# copy" > ".claude/skills/skill-alpha/SKILL.md"
git add -A && git commit -m "init"
git remote add origin "$BASE/repo2.git" && git push origin master

# --- Repo 3: .skillshare dir ---
git init --bare "$BASE/repo3.git"
W3=$(mktemp -d)
cd "$W3" && git init && git config user.email "t@t" && git config user.name "T"
mkdir -p "my-skill"
echo "---
name: my-skill
---
# ok" > "my-skill/SKILL.md"
mkdir -p ".skillshare/stray"
echo "---
name: stray
---
# no" > ".skillshare/stray/SKILL.md"
git add -A && git commit -m "init"
git remote add origin "$BASE/repo3.git" && git push origin master

# --- Repos 4 and 5: repository-root .agents/skills ---
mkrepo() { # repo name, then path:body pairs
  local repo=$1; shift
  git init -q --bare "$BASE/$repo.git"
  local W=$(mktemp -d)
  cd "$W" && git init -q && git config user.email "t@t" && git config user.name "T"
  for spec in "$@"; do
    p=${spec%%:*}; body=${spec#*:}
    mkdir -p "$p"
    printf -- "---\nname: %s\n---\n# %s\n" "$(basename "$p")" "$body" > "$p/SKILL.md"
  done
  git add -A && git commit -qm init && git push -q "$BASE/$repo.git" HEAD:master
}
mkrepo repo4 skills/alpha:authored skills/beta:authored .agents/skills/alpha:copy .agents/skills/beta:copy
mkrepo repo5 .agents/skills/psychopomp:authored .agents/skills/explainer-motion:authored

echo "ALL REPOS CREATED"
```

Expected:
- ALL REPOS CREATED

### Step 2: Install repo1 — only source skills, no target copies

```bash
ss install "file:///tmp/e2e-dotdir-test/repo1.git" --all --json 2>/dev/null | jq -r '.skills | sort | join(",")'
```

Expected:
- adapt,optimize,polish

### Step 3: Verify repo1 skill count is exactly 3

```bash
ss install "file:///tmp/e2e-dotdir-test/repo1.git" --all --json --force 2>/dev/null | jq '.skills | length'
```

Expected:
- 3

### Step 4: Install repo2 — non-target hidden dirs discovered, target copy skipped

```bash
ss install "file:///tmp/e2e-dotdir-test/repo2.git" --all --json 2>/dev/null | jq -r '.skills | sort | join(",")'
```

Expected:
- skill-alpha,skill-beta,skill-gamma

### Step 5: Verify repo2 has exactly 3 skills (not 4)

The `.claude/skills/skill-alpha` copy must not be counted.

```bash
ss install "file:///tmp/e2e-dotdir-test/repo2.git" --all --json --force 2>/dev/null | jq '.skills | length'
```

Expected:
- 3

### Step 6: Install repo3 — .skillshare dir skipped

```bash
ss install "file:///tmp/e2e-dotdir-test/repo3.git" --all --json 2>/dev/null | jq -r '.skills | sort | join(",")'
```

Expected:
- my-skill
- Not stray

### Step 7: Verify repo3 has exactly 1 skill

```bash
ss install "file:///tmp/e2e-dotdir-test/repo3.git" --all --json --force 2>/dev/null | jq '.skills | length'
```

Expected:
- 1

### Step 8: Install repo4 — authored skills win over `.agents/skills` copies

```bash
ss install "file:///tmp/e2e-dotdir-test/repo4.git" --all --json 2>/dev/null | jq -r '.skills | sort | join(",")'
SRC=$(ss status --json 2>/dev/null | jq -r .source.path)
tail -1 "$SRC/alpha/SKILL.md"
```

Expected:
- alpha,beta
- # authored

### Step 9: Track repo5 — skills only in `.agents/skills` are counted and synced

```bash
ss install "file:///tmp/e2e-dotdir-test/repo5.git" --track --json 2>/dev/null | jq -r '.skills | sort | join(",")'
ss sync >/dev/null 2>&1
ss list --json 2>/dev/null | jq -r '.[].name' | grep '^_repo5'
```

Expected:
- explainer-motion,psychopomp
- _repo5.git__.agents__skills__explainer-motion
- _repo5.git__.agents__skills__psychopomp

### Step 10: Track repo4 — sync lists no `.agents/skills` duplicates

```bash
ss install "file:///tmp/e2e-dotdir-test/repo4.git" --track --json 2>/dev/null | jq -r '.skills | sort | join(",")'
ss sync >/dev/null 2>&1
ss list --json 2>/dev/null | jq -r '.[].name' | grep '^_repo4'
```

Expected:
- alpha,beta
- _repo4.git__skills__alpha
- _repo4.git__skills__beta
- No `_repo4.git__.agents__` entries

## Pass Criteria

- All 10 steps pass
- Repo 1: Only 3 source skills discovered (`.claude/` and `.cursor/` copies excluded)
- Repo 2: 3 skills from `.curated/` and `.system/` discovered; `.claude/` copy excluded
- Repo 3: Only 1 skill; `.skillshare/` directory entirely skipped
- Repo 4: Only the `skills/` copies are installed and synced; `.agents/skills/` copies excluded
- Repo 5: Both `.agents/skills/` skills are counted on tracked install and synced
