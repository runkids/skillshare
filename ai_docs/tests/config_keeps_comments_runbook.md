# CLI E2E Runbook: config.yaml Keeps Its Comments

Verifies that commands which write `config.yaml` keep every comment in it:
head, inline and trailing comments, comments with odd spacing or no space
after `#`, comments on nested keys and list items, and comments on a key the
command removed. It also checks that YAML anchors, aliases and merge keys
(`<<: *base`) survive.

**Origin**: [#525](https://github.com/runkids/skillshare/issues/525) — on
0.25.4 `target --mode copy --target-naming prefixed` dropped marker comments
a user's script relied on. [#546](https://github.com/runkids/skillshare/issues/546)
— on 0.25.5 the same write expanded `&anchor` / `*alias` pairs under `targets`
and `projects`. Merge keys were expanded the same way.

## Scope

- Global mode, `config.yaml` written by `target` (set and remove)
- Every comment survives, including the ones on the removed target
- The schema header appears once
- An anchored list keeps its anchor and its alias after an edit; editing the
  aliased list gives it its own copy and leaves the anchored one alone
- A merge key (`<<: *base`) stays after an edit and only the keys the mapping
  sets itself are written out; editing the anchored mapping reaches a mapping
  that merges it (a key it gains included), and removing the anchored mapping
  writes the merged keys out
- Comments on the items of a merge list, and on a key the command removed from
  a mapping that is left with merged keys only, are not lost

## Environment

Run inside the devcontainer with `ssenv` isolation (`--init`).
mdproof replaces `$HOME`, so the config is reached through `$XDG_CONFIG_HOME`.
mdproof's setup re-runs `init` before every step, so steps 6 to 13 each write
their own config instead of relying on an earlier step.

## Steps

### 1. Setup: a config full of comments

```bash
CFG=$XDG_CONFIG_HOME/skillshare/config.yaml
cat > "$CFG" <<'YAML'
#ddddsxx
sources:
  skills: SRC/skills #inline1
#   weird   spacing   #  here
mode: merge #ddddsxx2
targets:
  #ddddsxx3 nested
  claude:
    skills:
      path: HOME/.claude/skills # path note
  universal: # will be removed
    skills:
      path: HOME/.agents/skills #ddddsxx4
ignore:
  - .DS_Store #ddddsxx5
  # between items
  - .git/
#ddddsxx6 end
YAML
sed -i "s|SRC|$XDG_CONFIG_HOME/skillshare|; s|HOME|$HOME|g" "$CFG"
echo "comments=$(grep -c '#' "$CFG")"
```

Expected:
- exit_code: 0
- comments=11

### 2. Set a target option

```bash
ss target claude --mode copy --target-naming prefixed
```

Expected:
- exit_code: 0

### 3. Every comment is still there and the value changed

```bash
CFG=$XDG_CONFIG_HOME/skillshare/config.yaml
grep -c "target_naming: prefixed" "$CFG"
for c in '#ddddsxx$' '#inline1' 'weird   spacing' '#ddddsxx2' '#ddddsxx3 nested' '# path note' '# will be removed' '#ddddsxx4' '#ddddsxx5' '# between items' '#ddddsxx6 end'; do grep -q -- "$c" "$CFG" && echo "kept $c" || echo "LOST $c"; done
echo "schema-headers=$(grep -c 'yaml-language-server' "$CFG")"
echo "lost-count=$(grep -c 'LOST' <<<"$(for c in ddddsxx inline1 spacing note removed between; do grep -q "$c" "$CFG" || echo LOST; done)")"
```

Expected:
- exit_code: 0
- 1
- kept #ddddsxx$
- kept # will be removed
- kept # between items
- schema-headers=1
- lost-count=0

### 4. Remove a target

```bash
ss target remove universal
```

Expected:
- exit_code: 0

### 5. The removed target's comments moved to the end of targets

```bash
CFG=$XDG_CONFIG_HOME/skillshare/config.yaml
grep -c "universal:" "$CFG"
echo "lost-count=$(grep -c 'LOST' <<<"$(for c in ddddsxx ddddsxx2 ddddsxx3 ddddsxx4 ddddsxx5 ddddsxx6 inline1 spacing note removed between; do grep -q "$c" "$CFG" || echo LOST; done)")"
awk '/^targets:/{t=1} t&&/will be removed/{print "orphans-under-targets"; exit} /^ignore:/{exit}' "$CFG"
```

Expected:
- exit_code: 0
- 0
- lost-count=0
- orphans-under-targets

### 6. Editing the anchored target keeps the anchor and the alias

```bash
CFG=$XDG_CONFIG_HOME/skillshare/config.yaml
cat > "$CFG" <<'YAML'
sources:
  skills: SRC/skills
targets:
  claude:
    skills:
      path: HOME/.claude/skills
      include: &shared
        - demo
  codex:
    skills:
      path: HOME/.codex/skills
      include: *shared
YAML
sed -i "s|SRC|$XDG_CONFIG_HOME/skillshare|; s|HOME|$HOME|g" "$CFG"
ss target claude --add-exclude 'old-*'
echo "anchors=$(grep -c '&shared' "$CFG") aliases=$(grep -c '\*shared' "$CFG")"
grep -c -- "- old-\*" "$CFG"
```

Expected:
- exit_code: 0
- anchors=1 aliases=1
- 1

### 7. Editing the aliased target gives it its own list

```bash
CFG=$XDG_CONFIG_HOME/skillshare/config.yaml
cat > "$CFG" <<'YAML'
sources:
  skills: SRC/skills
targets:
  claude:
    skills:
      path: HOME/.claude/skills
      include: &shared
        - demo
  codex:
    skills:
      path: HOME/.codex/skills
      include: *shared
YAML
sed -i "s|SRC|$XDG_CONFIG_HOME/skillshare|; s|HOME|$HOME|g" "$CFG"
ss target codex --add-include extra
echo "aliases=$(grep -c '\*shared' "$CFG")"
echo "extra=$(grep -c -- '- extra' "$CFG")"
awk '/^  claude:/{c=1} /^  codex:/{c=0} c&&/- extra/{n++} END{print "claude-extras=" n+0}' "$CFG"
```

Expected:
- exit_code: 0
- aliases=0
- extra=1
- claude-extras=0

### 8. A merge key survives an edit and only local keys stay written out

`claude` merges `zshared` and sets its own `skills`; `codex` only merges.
Targets are written in name order, so the anchor moves onto the first merge key
that uses it.

```bash
CFG=$XDG_CONFIG_HOME/skillshare/config.yaml
cat > "$CFG" <<'YAML'
sources:
  skills: SRC/skills
targets:
  zshared: &shared
    skills:
      path: HOME/.shared/skills
      mode: copy
  claude:
    # inherits the shared settings
    <<: *shared
    skills:
      path: HOME/.claude/skills
  codex:
    <<: *shared
YAML
sed -i "s|SRC|$XDG_CONFIG_HOME/skillshare|; s|HOME|$HOME|g" "$CFG"
mode() { ss target list --json | tr -d ' \n' | grep -o "\"name\":\"$1\",\"path\":\"[^\"]*\",\"mode\":\"[a-z]*\"" | sed 's/.*"mode"://'; }
ss target claude --add-exclude 'old-*'
echo "merges=$(grep -c '<<:' "$CFG")"
echo "anchors=$(grep -c '&shared' "$CFG") aliases=$(grep -c '\*shared' "$CFG")"
echo "comment=$(grep -c '# inherits the shared settings' "$CFG")"
echo "codex-own-keys=$(awk '/^  codex:/{c=1;next} /^  [a-z]/{c=0} c' "$CFG" | grep -c -v '<<:')"
echo "codex-mode=$(mode codex)"
```

Expected:
- exit_code: 0
- merges=2
- anchors=1 aliases=2
- comment=1
- codex-own-keys=0
- codex-mode="copy"

### 9. Editing the anchored target reaches the merging target, an override stays local

```bash
CFG=$XDG_CONFIG_HOME/skillshare/config.yaml
cat > "$CFG" <<'YAML'
sources:
  skills: SRC/skills
targets:
  zshared: &shared
    skills:
      path: HOME/.shared/skills
      mode: copy
  claude:
    # inherits the shared settings
    <<: *shared
    skills:
      path: HOME/.claude/skills
  codex:
    <<: *shared
YAML
sed -i "s|SRC|$XDG_CONFIG_HOME/skillshare|; s|HOME|$HOME|g" "$CFG"
mode() { ss target list --json | tr -d ' \n' | grep -o "\"name\":\"$1\",\"path\":\"[^\"]*\",\"mode\":\"[a-z]*\"" | sed 's/.*"mode"://'; }
ss target zshared --mode symlink
echo "merges=$(grep -c '<<:' "$CFG")"
echo "codex-mode=$(mode codex)"
echo "claude-mode=$(mode claude)"
```

Expected:
- exit_code: 0
- merges=2
- codex-mode="symlink"
- claude-mode="merge"

### 10. Removing the anchored target writes the merged keys out

```bash
CFG=$XDG_CONFIG_HOME/skillshare/config.yaml
cat > "$CFG" <<'YAML'
sources:
  skills: SRC/skills
targets:
  zshared: &shared
    skills:
      path: HOME/.shared/skills
      mode: copy
  claude:
    # inherits the shared settings
    <<: *shared
    skills:
      path: HOME/.claude/skills
  codex:
    <<: *shared
YAML
sed -i "s|SRC|$XDG_CONFIG_HOME/skillshare|; s|HOME|$HOME|g" "$CFG"
mode() { ss target list --json | tr -d ' \n' | grep -o "\"name\":\"$1\",\"path\":\"[^\"]*\",\"mode\":\"[a-z]*\"" | sed 's/.*"mode"://'; }
ss target remove zshared
echo "merges=$(grep -c '<<:' "$CFG") anchors=$(grep -c '&shared' "$CFG") aliases=$(grep -c '\*shared' "$CFG")"
echo "comment=$(grep -c '# inherits the shared settings' "$CFG")"
echo "codex-mode=$(mode codex)"
```

Expected:
- exit_code: 0
- merges=0 anchors=0 aliases=0
- comment=1
- codex-mode="copy"

### 11. A key added to the anchored target reaches the merging targets

`base` has no `agents` block yet; `work` only merges and `home` sets its own
`skills`.

```bash
CFG=$XDG_CONFIG_HOME/skillshare/config.yaml
cat > "$CFG" <<'YAML'
sources:
  skills: SRC/skills
targets:
  base: &base
    agent: claude
    config_dir: HOME/.base
    skills:
      mode: copy
  work:
    <<: *base
    config_dir: HOME/.work
  home:
    <<: *base
    config_dir: HOME/.home
    skills:
      mode: symlink
YAML
sed -i "s|SRC|$XDG_CONFIG_HOME/skillshare|; s|HOME|$HOME|g" "$CFG"
ss target base --agent-mode copy
echo "merges=$(grep -c '<<:' "$CFG") anchors=$(grep -c '&base' "$CFG")"
echo "agents-blocks=$(grep -c 'agents:' "$CFG")"
echo "agent-copy=$(ss target list --json | tr -d ' \n' | grep -o '"agentMode":"copy"' | wc -l)"
```

Expected:
- exit_code: 0
- merges=2 anchors=1
- agents-blocks=1
- agent-copy=3

### 12. Comments on the items of a merge list stay on them

`alpha` and `beta` sort before `claude`, so their anchors stay where they are.

```bash
CFG=$XDG_CONFIG_HOME/skillshare/config.yaml
cat > "$CFG" <<'YAML'
sources:
  skills: SRC/skills
targets:
  alpha: &a
    skills:
      path: HOME/.alpha/skills
  beta: &b
    agents:
      path: HOME/.beta/agents
  claude:
    <<:
      # about the first source
      - *a # line a
      - *b # line b
    skills:
      path: HOME/.claude/skills
YAML
sed -i "s|SRC|$XDG_CONFIG_HOME/skillshare|; s|HOME|$HOME|g" "$CFG"
ss target claude --add-exclude 'old-*'
echo "merges=$(grep -c '<<:' "$CFG")"
echo "comments=$(grep -c -e '# about the first source' -e '# line a' -e '# line b' "$CFG")"
echo "line-a=$(grep -c -e '- \*a # line a' "$CFG")"
echo "line-b=$(grep -c -e '- \*b # line b' "$CFG")"
```

Expected:
- exit_code: 0
- merges=1
- comments=3
- line-a=1
- line-b=1

### 13. Comments of a removed key stay when only merged keys are left

`claude` sets one key of its own, `exclude`, with comments around it. Removing
it must not drop them, although the keys left in `claude` come from the merge.

```bash
CFG=$XDG_CONFIG_HOME/skillshare/config.yaml
cat > "$CFG" <<'YAML'
sources:
  skills: SRC/skills
targets:
  base: &shared
    skills:
      path: HOME/.shared/skills
      mode: copy
  claude:
    <<: *shared
    # why the exclude
    exclude:
      - old-* # drop the old ones
    # end of the exclude
YAML
sed -i "s|SRC|$XDG_CONFIG_HOME/skillshare|; s|HOME|$HOME|g" "$CFG"
ss target claude --remove-exclude 'old-*'
echo "merges=$(grep -c '<<:' "$CFG") exclude=$(grep -c 'exclude:' "$CFG")"
echo "comments=$(grep -c -e '# why the exclude' -e '# drop the old ones' -e '# end of the exclude' "$CFG")"
```

Expected:
- exit_code: 0
- merges=1 exclude=0
- comments=3

## Pass Criteria

- Step 3: all 11 comments present after `target` set, schema header once.
- Step 5: `universal` gone, all 11 comments still present, the removed
  target's comments sit inside the `targets` section.
- Step 6: the `&shared` anchor and its `*shared` alias are both still there.
- Step 7: `codex` no longer aliases `shared`, and `claude` did not gain `extra`.
- Step 8: both `<<:` keys are still there with one anchor and two aliases of
  it, `codex` writes nothing but its merge key and still loads as `copy`.
- Step 9: `codex` follows the edit of the anchored target; `claude`, which sets
  its own `skills`, does not.
- Step 10: no `<<:`, anchor or alias is left, `codex` still loads as `copy`, and
  the comment on the removed merge key is still in the file.
- Step 11: both `<<:` keys and the anchor are still there, the new `agents`
  block is written once under `base`, and all three targets load it.
- Step 12: the merge list is still there and each of its three comments is
  written once, the line comments on the item they were on.
- Step 13: the merge key is still there, `exclude` is gone, and its three
  comments are all still in the file.
