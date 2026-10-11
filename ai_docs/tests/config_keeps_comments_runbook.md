# CLI E2E Runbook: config.yaml Keeps Its Comments

Verifies that commands which write `config.yaml` keep every comment in it:
head, inline and trailing comments, comments with odd spacing or no space
after `#`, comments on nested keys and list items, and comments on a key the
command removed.

**Origin**: [#525](https://github.com/runkids/skillshare/issues/525) — on
0.25.4 `target --mode copy --target-naming prefixed` dropped marker comments
a user's script relied on.

## Scope

- Global mode, `config.yaml` written by `target` (set and remove)
- Every comment survives, including the ones on the removed target
- The schema header appears once

## Environment

Run inside the devcontainer with `ssenv` isolation (`--init`).
mdproof replaces `$HOME`, so the config is reached through `$XDG_CONFIG_HOME`.

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

## Pass Criteria

- Step 3: all 11 comments present after `target` set, schema header once.
- Step 5: `universal` gone, all 11 comments still present, the removed
  target's comments sit inside the `targets` section.
