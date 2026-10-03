# CLI E2E Runbook: Shared Memory Notes

## Scope

Verify source-only initialization, Markdown search, versioned updates, stale
edit protection, project source overrides, and exclusion of linked/hidden notes.
Native automatic memory is separate. Initialization creates missing `INDEX.md`
and `LEARNED.md` templates without overwriting user notes.

## Environment

Run inside the devcontainer with mdproof's per-runbook HOME isolation.
`MEMORY_CLI` may point to a worktree's built binary; otherwise use `skillshare`.
Every step initializes the scope it uses and does not depend on previous steps.

## Steps

### 1. Initialize global memory and search real Markdown

```bash
memory_cli=${MEMORY_CLI:-skillshare}
"$memory_cli" extras memory init -g >/dev/null
"$memory_cli" extras memory show INDEX.md -g | grep -Fq '[Lessons learned](LEARNED.md)'
"$memory_cli" extras memory show LEARNED.md -g | grep -q 'Evidence:'
printf '# Architecture\nShared seam and agent instructions.\n' > "$HOME/memory-input.md"
"$memory_cli" extras memory write architecture.md --from "$HOME/memory-input.md" -g >/dev/null
"$memory_cli" extras memory list --search 'SHARED SEAM' --json -g
```

Expected:
- exit_code: 0
- jq: .notes | length == 1
- jq: .notes[0].path == "architecture.md"
- jq: .notes[0].title == "Architecture"

### 2. Update with the last read version; reject stale writes

```bash
memory_cli=${MEMORY_CLI:-skillshare}
"$memory_cli" extras memory init -g >/dev/null
printf '# Before\n' > "$HOME/version-input.md"
"$memory_cli" extras memory write versioned.md --from "$HOME/version-input.md" -g >/dev/null
memory_version=$("$memory_cli" extras memory show versioned.md --json -g | jq -r '.version')
printf '# After\n' > "$HOME/version-input.md"
"$memory_cli" extras memory write versioned.md --from "$HOME/version-input.md" --version "$memory_version" -g >/dev/null
if "$memory_cli" extras memory write versioned.md --from "$HOME/version-input.md" --version "$memory_version" -g > "$HOME/conflict.txt" 2>&1; then
  exit 1
fi
grep -q 'reload it before saving' "$HOME/conflict.txt"
"$memory_cli" extras memory show versioned.md --json -g
```

Expected:
- exit_code: 0
- jq: .content == "# After\n"

### 3. Preserve a project index and use the per-extra source

```bash
memory_cli=${MEMORY_CLI:-skillshare}
mkdir -p "$HOME/memory-project/.skillshare" "$HOME/memory-project/notes"
printf 'targets: []\nextras:\n  - name: memory\n    source: notes\n    targets: []\n' > "$HOME/memory-project/.skillshare/config.yaml"
printf '# My index\n' > "$HOME/memory-project/notes/INDEX.md"
printf '# My lessons\n' > "$HOME/memory-project/notes/LEARNED.md"
cd "$HOME/memory-project"
"$memory_cli" extras memory init -p >/dev/null
grep -q 'My index' notes/INDEX.md
grep -q 'My lessons' notes/LEARNED.md
"$memory_cli" extras memory instructions --json -p
```

Expected:
- exit_code: 0
- jq: .root | endswith("/memory-project/notes")
- jq: .instructions | contains("INDEX.md")
- jq: .instructions | contains("Update these notes only when the user requests it.")

### 4. Exclude hidden state and links; reject traversal

```bash
memory_cli=${MEMORY_CLI:-skillshare}
memory_root=$("$memory_cli" extras memory init --json -g | jq -r '.root')
mkdir -p "$memory_root/.obsidian"
printf '# Private hidden evidence\n' > "$memory_root/.obsidian/private.md"
printf '# Private linked evidence\n' > "$HOME/private.md"
ln -s "$HOME/private.md" "$memory_root/private-link.md"
if "$memory_cli" extras memory write ../escape.md --from "$HOME/private.md" -g >/dev/null 2>&1; then
  exit 1
fi
"$memory_cli" extras memory list --search private --json -g
```

Expected:
- exit_code: 0
- jq: .notes == []

### 5. Back up and delete a note with its current version

```bash
memory_cli=${MEMORY_CLI:-skillshare}
memory_root=$("$memory_cli" extras memory init --json -g | jq -r '.root')
printf '# Delete demo\n' | "$memory_cli" extras memory write wiki/delete-demo.md --from - -g >/dev/null
version=$("$memory_cli" extras memory show wiki/delete-demo.md --json -g | jq -r '.version')
if "$memory_cli" extras memory delete wiki/delete-demo.md --version stale -g >/dev/null 2>&1; then
  exit 1
fi
"$memory_cli" extras memory delete wiki/delete-demo.md --version "$version" --json -g >/dev/null
test ! -f "$memory_root/wiki/delete-demo.md"
"$memory_cli" backup files show "$memory_root/wiki/delete-demo.md" | grep -q delete
"$memory_cli" extras memory list --search delete-demo --json -g
```

Expected:
- exit_code: 0
- jq: .notes == []

### 6. Keep valid notes available beside unsupported external files

```bash
memory_cli=${MEMORY_CLI:-skillshare}
memory_root=$("$memory_cli" extras memory init --json -g | jq -r '.root')
printf '# Valid external note\n' > "$memory_root/valid.md"
printf '\377' > "$memory_root/invalid.md"
head -c 1048577 /dev/zero > "$memory_root/large.md"
"$memory_cli" extras memory list --json -g
```

Expected:
- exit_code: 0
- jq: [.notes[] | select(.invalid != null)] | length == 2
- jq: [.notes[] | select(.path == "valid.md")] | length == 1

## Pass Criteria

All six steps pass. Existing files survive rejected writes and stale deletions, the project index
and lessons are preserved, and no native agent memory directory is modified.
