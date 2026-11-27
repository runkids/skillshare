# CLI E2E Runbook: Install Keeps Symlinks Inside the Repository

Validates that installing from a Git repository refuses a symlink that leads outside the repository or into its `.git` folder, before writing anything, while a link to another file in the same repository is still copied.

## Scope

- `ss install <repo>//<subdir>` refuses a skill with a link outside the repository
- `ss install <repo> --all` (discovery) refuses the same skill
- `ss install <repo> -a <name>` refuses an agent file that is such a link
- A link into the clone's `.git` folder is refused
- A link to another file in the repository is copied as that file's content
- `ss install <local path>` (a folder the user names) keeps its links' content as before
- `ss install <repo> --track` refuses a repository with a link outside it
- A link to a folder in the repository is copied as a folder (#1036)
- A subdirectory install brings a linked file from elsewhere in the repository (#1037)

## Environment

Run inside the devcontainer with `ssenv` isolation. No network needed: the repository is a local Git repository installed through `file://`.

## Steps

### 1. Create a fake private file and a repository that links to it

```bash
mkdir -p ~/.fake-ssh && echo "PRIVATE KEY" > ~/.fake-ssh/id_ed25519
REPO=$(mktemp -d)/linked-repo
mkdir -p "$REPO/skills/demo" "$REPO/shared"
printf -- '---\nname: demo\n---\n# Demo\n' > "$REPO/skills/demo/SKILL.md"
echo "shared reference" > "$REPO/shared/ref.md"
ln -s ~/.fake-ssh/id_ed25519 "$REPO/skills/demo/notes.md"
git -C "$REPO" init -q && git -C "$REPO" add . && git -C "$REPO" -c user.name=t -c user.email=t@t commit -qm init
echo "$REPO"
```

**Expected**: exit_code 0, prints the repository path.

### 2. Subdirectory install is refused

```bash
ss install "file://$REPO//skills/demo" --skip-audit
```

**Expected**: exit_code non-zero, output contains `links outside the repository`.

### 3. Discovery install is refused

```bash
ss install "file://$REPO" --all --skip-audit
```

**Expected**: output contains `links outside the repository`; `demo` is not installed.

### 4. Agent install is refused

A repository with skills installs its skills first, so use one that holds only agents.

```bash
AGENTS=$(mktemp -d)/agent-repo
mkdir -p "$AGENTS/agents" && ln -s ~/.fake-ssh/id_ed25519 "$AGENTS/agents/notes.md"
echo "# Reviewer" > "$AGENTS/agents/reviewer.md"
git -C "$AGENTS" init -q && git -C "$AGENTS" add . && git -C "$AGENTS" -c user.name=t -c user.email=t@t commit -qm init
ss install "file://$AGENTS" -a notes --skip-audit
```

**Expected**: output contains `agents/notes.md links outside the repository` and `1 failed`.

### 5. Nothing from outside reached the source

```bash
grep -rl "PRIVATE KEY" ~/.config/skillshare/ || echo "clean"
```

**Expected**: output is `clean`.

### 6. A link into the clone's .git folder is refused

```bash
rm "$REPO/skills/demo/notes.md"
ln -s ../../.git/config "$REPO/skills/demo/cfg"
git -C "$REPO" add -A && git -C "$REPO" -c user.name=t -c user.email=t@t commit -qm git-link
ss install "file://$REPO//skills/demo" --skip-audit
```

**Expected**: exit_code non-zero, output contains `cfg links outside the repository`.

### 7. A link inside the repository is copied

```bash
rm "$REPO/skills/demo/cfg"
ln -s ../../shared/ref.md "$REPO/skills/demo/ref.md"
git -C "$REPO" add -A && git -C "$REPO" -c user.name=t -c user.email=t@t commit -qm inside
ss install "file://$REPO" --all --skip-audit
cat ~/.config/skillshare/skills/demo/ref.md
```

**Expected**: exit_code 0, output contains `shared reference`.

### 8. A local folder keeps its links' content

```bash
LOCAL=$(mktemp -d)/local-skill && mkdir -p "$LOCAL"
printf -- '---\nname: local-demo\n---\n# Local\n' > "$LOCAL/SKILL.md"
ln -s ~/.fake-ssh/id_ed25519 "$LOCAL/notes.md"
ss install "$LOCAL" --skip-audit
cat ~/.config/skillshare/skills/local-skill/notes.md
```

**Expected**: exit_code 0, output contains `PRIVATE KEY` (a folder the user names is trusted as before).

### 9. Tracked install is refused

```bash
TRACKED=$(mktemp -d)/tracked-repo
mkdir -p "$TRACKED/skills/one" && printf -- '---\nname: one\n---\n' > "$TRACKED/skills/one/SKILL.md"
ln -s ~/.fake-ssh/id_ed25519 "$TRACKED/skills/one/notes.md"
git -C "$TRACKED" init -q && git -C "$TRACKED" add . && git -C "$TRACKED" -c user.name=t -c user.email=t@t commit -qm init
ss install "file://$TRACKED" --track --skip-audit
ls ~/.config/skillshare/skills/ | grep -c tracked-repo
```

**Expected**: install fails with `skills/one/notes.md links outside the repository`; the last line prints `0`.

### 10. A linked folder is copied as a folder

```bash
ln -s ../../shared "$REPO/skills/demo/refs"
git -C "$REPO" add -A && git -C "$REPO" -c user.name=t -c user.email=t@t commit -qm folder-link
ss install "file://$REPO" --all --force --skip-audit
cat ~/.config/skillshare/skills/demo/refs/ref.md
```

**Expected**: exit_code 0, output contains `shared reference`.

### 11. A subdirectory install brings a linked file from elsewhere

```bash
ss install "file://$REPO//skills/demo" --name demo-sub --skip-audit
cat ~/.config/skillshare/skills/demo-sub/ref.md
```

**Expected**: exit_code 0, output contains `shared reference`.

### 12. A tracked update that adds an outside link is refused

```bash
UPD=$(mktemp -d)/update-repo
mkdir -p "$UPD/skills/one" && printf -- '---\nname: one\n---\n' > "$UPD/skills/one/SKILL.md"
git -C "$UPD" init -q && git -C "$UPD" add . && git -C "$UPD" -c user.name=t -c user.email=t@t commit -qm init
ss install "file://$UPD" --track
ln -s ~/.fake-ssh/id_ed25519 "$UPD/skills/one/notes.md"
git -C "$UPD" add . && git -C "$UPD" -c user.name=t -c user.email=t@t commit -qm link
ss update _update-repo; echo "single exit=$?"
ss update --all; echo "all exit=$?"
ls ~/.config/skillshare/skills/_update-repo/skills/one/
```

**Expected**: both updates print `update refused: skills/one/notes.md links outside the repository` and exit non-zero; neither suggests `--force` or `--skip-audit`; the last line lists only `SKILL.md`.

## Pass Criteria

- All steps marked PASS
- Steps 2–4, 6 and 9 fail before any file lands in the source; step 5 finds nothing
- Step 12 leaves the tracked checkout at its first commit
