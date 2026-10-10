# CLI E2E Runbook: Move Installed Skills and Folders

Verifies `skillshare move` and `POST /api/resources/batch/move`: installed skills
and whole folders change folder while their install records, audit state and
project lock pins follow, and nothing is downloaded again.

**Origin**: [#510](https://github.com/runkids/skillshare/issues/510) — moving an
installed skill with `mv` strands its record in `.metadata.json`.

## Scope

- Global mode: one local-path skill and one git skill, one edited after install
- Dry run changes nothing; `move` does not sync
- Records (source, commit, group) re-keyed; `doctor` finds no record problem
- `sync` renames the target links; `check` still resolves the source
- Whole-folder move; a folder holding a tracked checkout or a followed link is refused whole
- Project mode: `skills.lock.json` pin, `config.yaml` group and `.gitignore` entry
- Web API against `ss ui`

## Environment

Run inside the devcontainer with `ssenv` isolation (`--init`).
Requires `git`, `jq` and `curl`. mdproof replaces `$HOME`, so the source is read
through `$XDG_CONFIG_HOME` and the target path through `ss status --json`.

## Steps

### 1. Setup: create a local skill and a local git repo, install both, sync

```bash
rm -rf /tmp/ss-move-local /tmp/ss-move-git /tmp/ss-move-git.git /tmp/ss-move-linked /tmp/ss-move-proj
mkdir -p /tmp/ss-move-local/local-skill /tmp/ss-move-git/git-skill
printf -- "---\nname: local-skill\n---\n# Local\n" > /tmp/ss-move-local/local-skill/SKILL.md
printf -- "---\nname: git-skill\n---\n# Git\n" > /tmp/ss-move-git/git-skill/SKILL.md
cd /tmp/ss-move-git && git init -q -b main && git add . \
  && git -c user.name=t -c user.email=t@e.c -c commit.gpgsign=false commit -qm seed
git clone -q --bare /tmp/ss-move-git /tmp/ss-move-git.git
ss install /tmp/ss-move-local/local-skill -g
ss install file:///tmp/ss-move-git.git --skill git-skill -g
ss sync -g
CLAUDE=$(ss status -g --json | jq -r '.targets[] | select(.name=="claude") | .path')
ls "$CLAUDE"
```

Expected:
- exit_code: 0
- git-skill
- local-skill

### 2. Records exist before the move

```bash
jq -c '.entries | {local: .["local-skill"].type, git: .["git-skill"].type, commit: (.["git-skill"].commit | length)}' $XDG_CONFIG_HOME/skillshare/skills/.metadata.json
```

Expected:
- exit_code: 0
- jq: .local == "local"
- jq: .git == "git-https-subdir"
- jq: .commit == 40

### 3. Edit the git skill after install

```bash
echo "# edited after install" >> $XDG_CONFIG_HOME/skillshare/skills/git-skill/SKILL.md
tail -1 $XDG_CONFIG_HOME/skillshare/skills/git-skill/SKILL.md
```

Expected:
- exit_code: 0
- edited after install

### 4. Dry run reports the plan

```bash
ss move local-skill git-skill grp --dry-run --json -g
```

Expected:
- exit_code: 0
- jq: .dry_run == true
- jq: (.moved | length) == 2
- jq: .moved[0].to == "grp/local-skill"
- jq: (.failed | length) == 0

### 5. Dry run changed nothing

```bash
SRC="$XDG_CONFIG_HOME/skillshare/skills"
jq -n --arg dirs "$(ls "$SRC" | tr '\n' ' ')" --slurpfile m "$SRC/.metadata.json" \
  '{dirs: $dirs, keys: ($m[0].entries | keys)}'
```

Expected:
- exit_code: 0
- jq: (.keys | index("git-skill")) != null
- jq: (.keys | index("local-skill")) != null
- jq: (.keys | index("grp/git-skill")) == null
- jq: (.dirs | contains("grp")) == false

### 6. Move both into grp

```bash
ss move local-skill git-skill grp --json -g
```

Expected:
- exit_code: 0
- jq: (.moved | length) == 2
- jq: .moved[0].record == true
- jq: .moved[1].record == true
- jq: .moved[1].to == "grp/git-skill"
- jq: (.failed | length) == 0

### 7. Records moved with their skills

```bash
SRC="$XDG_CONFIG_HOME/skillshare/skills"
jq -n --arg dirs "$(ls "$SRC/grp" | tr '\n' ' ')" --slurpfile m "$SRC/.metadata.json" '{
  dirs: $dirs,
  keys: ($m[0].entries | keys),
  group: $m[0].entries["grp/git-skill"].group,
  source: $m[0].entries["grp/git-skill"].source,
  commit: ($m[0].entries["grp/git-skill"].commit | length),
  local_group: $m[0].entries["grp/local-skill"].group
}'
```

Expected:
- exit_code: 0
- jq: (.keys | index("grp/git-skill")) != null
- jq: (.keys | index("grp/local-skill")) != null
- jq: (.keys | index("git-skill")) == null
- jq: (.keys | index("local-skill")) == null
- jq: .group == "grp"
- jq: .local_group == "grp"
- jq: .source == "file:///tmp/ss-move-git.git/git-skill"
- jq: .commit == 40
- jq: (.dirs | contains("git-skill"))

### 8. doctor finds no record problem

The edited skill is the only integrity warning; nothing reports a missing record.

```bash
ss doctor -g --json || true
```

Expected:
- jq: ([.checks[] | select(.message | test("missing on disk"; "i"))] | length) == 0
- jq: ([.checks[] | select(.name == "skill_integrity")][0].details | length) == 1
- jq: ([.checks[] | select(.name == "skill_integrity")][0].details[0] | startswith("grp/git-skill"))

### 9. The old links stay until sync; sync renames them

```bash
CLAUDE=$(ss status -g --json | jq -r '.targets[] | select(.name=="claude") | .path')
echo "--- before sync"; ls "$CLAUDE"
ss sync -g
echo "--- after sync"; ls "$CLAUDE"
```

Expected:
- exit_code: 0
- --- before sync
- --- after sync
- grp__git-skill
- grp__local-skill

### 10. check still resolves the source of the moved git skill

```bash
ss check git-skill --json
```

Expected:
- exit_code: 0
- jq: .skills[0].name == "grp/git-skill"
- jq: .skills[0].source == "file:///tmp/ss-move-git.git/git-skill"

### 11. Move the whole folder

```bash
ss move grp archive --json -g
```

Expected:
- exit_code: 0
- jq: .moved[0].to == "archive/grp"
- jq: .moved[0].skills == 2
- jq: .moved[0].record == true

### 12. The folder arrived whole, records under the new prefix

```bash
SRC="$XDG_CONFIG_HOME/skillshare/skills"
jq -n --arg dirs "$(ls "$SRC/archive/grp" | tr '\n' ' ')" --arg top "$(ls "$SRC" | tr '\n' ' ')" --slurpfile m "$SRC/.metadata.json" \
  '{dirs: $dirs, top: $top, keys: ($m[0].entries | keys), group: $m[0].entries["archive/grp/git-skill"].group}'
```

Expected:
- exit_code: 0
- jq: (.dirs | contains("git-skill")) and (.dirs | contains("local-skill"))
- jq: (.top | contains("grp")) == false
- jq: (.keys | index("archive/grp/git-skill")) != null
- jq: .group == "archive/grp"

### 13. A folder holding a tracked checkout is refused whole

```bash
SRC="$XDG_CONFIG_HOME/skillshare/skills"
mkdir -p "$SRC/team/pdf" "$SRC/team/_vendor"
printf -- "---\nname: pdf\n---\n# Pdf\n" > "$SRC/team/pdf/SKILL.md"
git -C "$SRC/team/_vendor" init -q
ss move team archive --json -g
```

Expected:
- exit_code: 1
- jq: (.moved | length) == 0
- jq: .failed[0].code == "inside_tracked_repo"

### 14. Nothing of the refused folder moved

```bash
SRC="$XDG_CONFIG_HOME/skillshare/skills"
ls "$SRC/team"
ls "$SRC/archive"
```

Expected:
- exit_code: 0
- pdf
- _vendor
- grp

### 15. A skill below a followed link is refused

```bash
mkdir -p /tmp/ss-move-linked/linked-skill
printf -- "---\nname: linked-skill\n---\n# Linked\n" > /tmp/ss-move-linked/linked-skill/SKILL.md
ss link /tmp/ss-move-linked --name dev --enable -g
```

Expected:
- exit_code: 0
- Linked dev

### 16. The refusal is a linked_folder with exit code 1

```bash
ss move dev/linked-skill archive --json -g
```

Expected:
- exit_code: 1
- jq: (.moved | length) == 0
- jq: .failed[0].code == "linked_folder"

### 17. A destination that exists is never overwritten, not even with --force

```bash
SRC="$XDG_CONFIG_HOME/skillshare/skills"
mkdir -p "$SRC/archive/pdf" "$SRC/solo-src/pdf"
printf -- "---\nname: pdf\n---\n# other\n" > "$SRC/archive/pdf/SKILL.md"
printf -- "---\nname: pdf\n---\n# solo\n" > "$SRC/solo-src/pdf/SKILL.md"
ss move solo-src/pdf archive --force --json -g
```

Expected:
- exit_code: 1
- jq: .failed[0].code == "dest_exists"

### 18. The existing destination is untouched

```bash
cat "$XDG_CONFIG_HOME/skillshare/skills/archive/pdf/SKILL.md"
```

Expected:
- exit_code: 0
- # other

### 19. Project mode: initialize and install the git skill

```bash
mkdir -p /tmp/ss-move-proj && cd /tmp/ss-move-proj
ss init -p --targets claude
ss install file:///tmp/ss-move-git.git --skill git-skill -p
jq -c '{keys: (.skills | keys), commit: (.skills["git-skill"].commit | length)}' .skillshare/skills.lock.json
```

Expected:
- exit_code: 0
- "keys":["git-skill"]
- "commit":40

### 20. Project mode: the move keeps the pin, the group and the .gitignore entry

```bash
cd /tmp/ss-move-proj
PIN=$(jq -r '.skills["git-skill"].commit' .skillshare/skills.lock.json)
ss move git-skill grp -p --json > /tmp/ss-move-proj/move.json || exit 1
jq -n --arg pin "$PIN" --slurpfile move /tmp/ss-move-proj/move.json --slurpfile lock .skillshare/skills.lock.json \
  --arg cfg "$(grep -A4 '^skills:' .skillshare/config.yaml | tr '\n' ' ')" \
  --arg ign "$(grep 'skills/' .skillshare/.gitignore | tr '\n' ' ')" '{
    moved_to: $move[0].moved[0].to,
    record: $move[0].moved[0].record,
    lock_keys: ($lock[0].skills | keys),
    pin_kept: ($lock[0].skills["grp/git-skill"].commit == $pin),
    cfg: $cfg,
    ign: $ign
  }'
```

Expected:
- exit_code: 0
- jq: .moved_to == "grp/git-skill"
- jq: .record == true
- jq: .lock_keys == ["grp/git-skill"]
- jq: .pin_kept == true
- jq: .cfg | contains("group: grp")
- jq: (.ign | contains("skills/grp/git-skill")) and (.ign | contains("skills/git-skill/") | not)

### 21. Web API: start the dashboard server

```bash
cd /tmp
ss ui start --no-open --port 49555 -g
sleep 2
```

Expected:
- exit_code: 0
- UI running

### 22. Web API: partial failure is still 200 with a per-item error_code

```bash
curl -s -X POST localhost:49555/api/resources/batch/move \
  -H "Content-Type: application/json" \
  -d '{"names":["archive/grp/git-skill","ghost"],"dest":"api-dest"}'
```

Expected:
- exit_code: 0
- jq: .summary.succeeded == 1
- jq: .summary.failed == 1
- jq: .results[0].to == "api-dest/git-skill"
- jq: .results[0].flatName == "api-dest__git-skill"
- jq: .results[0].record == true
- jq: .results[1].error_code == "skill_not_found"

### 23. Web API: a malformed destination is 400

```bash
curl -s -i -X POST localhost:49555/api/resources/batch/move \
  -H "Content-Type: application/json" -d '{"names":["x"],"dest":"../out"}' | head -1
```

Expected:
- exit_code: 0
- 400

### 24. Web API: agents are not supported

```bash
curl -s -X POST localhost:49555/api/resources/batch/move \
  -H "Content-Type: application/json" -d '{"names":["x"],"dest":"grp","kind":"agent"}'
```

Expected:
- exit_code: 0
- jq: .error_code == "unsupported_kind"

### 25. oplog recorded the moves

```bash
ss log --cmd move --json -g
```

Expected:
- exit_code: 0
- "cmd":"move"

### 26. Cleanup

```bash
ss ui stop -g 2>/dev/null || true
ss unlink dev -g 2>/dev/null || true
rm -rf /tmp/ss-move-local /tmp/ss-move-git /tmp/ss-move-git.git /tmp/ss-move-linked /tmp/ss-move-proj
echo cleaned
```

Expected:
- exit_code: 0
- cleaned

## Pass Criteria

- Steps 6, 7, 11, 12: records, groups and the commit are present under the new keys, and no download happened
- Steps 13 to 18: refusals exit 1 and leave the source and records untouched
- Step 20: lock pin equals the pin before the move; `config.yaml` has the new group
- Steps 22 to 24: the endpoint answers with the documented codes
