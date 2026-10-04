# follow / unfollow Runbook

Verifies `skillshare follow` and `skillshare unfollow` end to end in global mode: `follow --to` creates the first-level link, the declaration, and the Git ignore line in one step; sync then links the followed skills; `unfollow` removes the declaration, the link, and the ignore line without touching the external tree, and the next sync prunes the managed links. Also covers `--local` and a partial `unfollow --local`. Proposal: `proposals/274-skillfollow.md` (step 4). Reference: `website/docs/reference/commands/follow.md`, `unfollow.md`.

## Scope

- `cmd/skillshare/follow.go`, `follow_handlers.go` — command parsing, link creation, ignore lines, state output
- `internal/skillfollow` — declaration and `.gitignore` edits through `sourcefs`
- Commands affected: `follow`, `unfollow`, `list`, `sync`

## Environment

- Devcontainer with `ss` binary (`make build`)
- Source and target paths come from `ss status --json`: ssenv moves the config through `XDG_CONFIG_HOME`, while mdproof isolates `HOME`, so `~` paths do not match the configured ones
- mdproof isolation (`isolation: per-runbook`); `ss init -g --force --all-targets --no-git --no-skill` runs before each step, so the source directory persists across steps but `config.yaml` is reset
- The external tree lives at `/tmp/sfc-ext`; Step 1 removes and recreates it

## Steps

### Step 1: follow --to creates the link, declaration, and ignore line

```bash
SOURCE=$(ss status --json -g | jq -r .source.path)
rm -rf /tmp/sfc-ext "$SOURCE/_ext" "$SOURCE/.skillfollow" "$SOURCE/.skillfollow.local" "$SOURCE/.gitignore" "$SOURCE/.git"
CLAUDE=$(ss status --json -g | jq -r '.targets[] | select(.name == "claude") | .path')
rm -rf "$CLAUDE"/*
mkdir -p /tmp/sfc-ext/a
printf -- "---\nname: a\ndescription: Fixture a\n---\n# a\n" > /tmp/sfc-ext/a/SKILL.md
git -C "$SOURCE" init -q
ss follow _ext --to /tmp/sfc-ext -g --json
```

Expected:
- jq: .added == true
- jq: .link_created == true
- jq: .ignore_lines_added == ["/_ext"]
- jq: .state == "followed"
- exit_code: 0

### Step 2: the declaration and ignore line are on disk, and Git ignores the link

```bash
SOURCE=$(ss status --json -g | jq -r .source.path)
readlink "$SOURCE/_ext"
cat "$SOURCE/.skillfollow"
git -C "$SOURCE" check-ignore _ext
```

Expected:
- /tmp/sfc-ext
- _ext
- exit_code: 0

### Step 3: sync links the followed skill

```bash
CLAUDE=$(ss status --json -g | jq -r '.targets[] | select(.name == "claude") | .path')
ss sync -g >/dev/null
ss list --json -g | jq -r '[.[] | .name | select(startswith("_ext"))] | join(",")'
test -L "$CLAUDE/_ext__a" && echo linked
```

Expected:
- _ext__a
- linked
- exit_code: 0

### Step 4: unfollow removes the link but not the external tree

```bash
ss unfollow _ext -g --json
```

Expected:
- jq: .files_edited == [".skillfollow"]
- jq: .link_removed == true
- jq: .ignore_line_removed == true
- exit_code: 0

### Step 5: the target link is pruned and the external tree is intact

```bash
SOURCE=$(ss status --json -g | jq -r .source.path)
CLAUDE=$(ss status --json -g | jq -r '.targets[] | select(.name == "claude") | .path')
ss sync -g >/dev/null
test -e "$SOURCE/_ext" || echo "source link gone"
test -e "$CLAUDE/_ext__a" || echo "target link pruned"
cat /tmp/sfc-ext/a/SKILL.md | head -2 | tail -1
```

Expected:
- source link gone
- target link pruned
- name: a
- exit_code: 0

### Step 6: unfollow --local reports an entry that .skillfollow still declares

```bash
ss follow _ext --to /tmp/sfc-ext -g >/dev/null
ss follow _ext --local -g >/dev/null
ss unfollow _ext --local -g --json
```

Expected:
- jq: .files_edited == [".skillfollow.local"]
- jq: .still_declared_in == [".skillfollow"]
- jq: .link_removed == false
- exit_code: 0

### Step 7: cleanup

```bash
SOURCE=$(ss status --json -g | jq -r .source.path)
ss unfollow _ext -g >/dev/null
rm -rf /tmp/sfc-ext "$SOURCE/.skillfollow" "$SOURCE/.skillfollow.local" "$SOURCE/.gitignore" "$SOURCE/.git"
ls -A "$SOURCE" | grep -c _ext || true
```

Expected:
- 0
- exit_code: 0

## Pass Criteria

- All 7 steps pass
- `follow --to` writes the link, `.skillfollow`, and the `/_ext` ignore line in one command, and prints `followed`
- Sync links the followed skill under its logical flat name
- `unfollow` removes the declaration, the link, and the ignore line; the external tree is untouched, and the next sync prunes the target link
- `unfollow --local` edits only `.skillfollow.local` and reports that `.skillfollow` still declares the entry
