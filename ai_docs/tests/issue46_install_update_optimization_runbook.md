# CLI E2E Runbook: Issue #46 Install/Update Optimization

Validates large-repo install/update optimization behavior for:
- `install` subdirectory source from GitHub
- `install --track` optimized clone path
- `update` for both regular skill and tracked repo (including `--force`)
- sparse checkout path and sparse->full-clone fallback
- GitHub/GHE API base handling
- non-TTY quiet behavior (no raw git progress spam)
- manual TTY progress sanity
- devcontainer + `ssenv` isolation/mode guardrails
- token and credential-helper preconditions

## Scope

- Subdir install from GitHub URL succeeds and installs only target skill
- API fallback path still succeeds when unauthenticated/rate-limited
- Tracked install keeps `.git` and uses shallow/partial clone optimization
- Regular skill update (`update <skill>`) succeeds for subdir source metadata
- Tracked repo update (`update _<name>`) succeeds
- Tracked repo force-update path discards local dirty changes
- Non-GitHub subdir install uses sparse checkout when possible
- Fuzzy subdir falls back from sparse checkout to full clone and still succeeds
- GHE API base derivation is validated (`https://<host>/api/v3`)
- Non-TTY install does not print raw git progress lines
- `ssenv enter` command mode keeps cwd isolated (not `/workspace`)

## Environment

Run inside devcontainer with `ssenv` HOME isolation.

Container target:
- `skillshare_devcontainer-skillshare-devcontainer-1`

Token envs forwarded by compose (optional but recommended for GitHub API quota):
- `GITHUB_TOKEN`
- `GH_TOKEN`
- `SKILLSHARE_GIT_TOKEN`
- plus other host-specific vars (`GITLAB_TOKEN`, `BITBUCKET_TOKEN`, etc.)

## Steps

### 1. Create isolated environment and baseline init

```bash
CONTAINER="skillshare_devcontainer-skillshare-devcontainer-1"
ENV_NAME="e2e-issue46-$(date +%Y%m%d-%H%M%S)"
echo "$ENV_NAME"

docker exec "$CONTAINER" ssenv create "$ENV_NAME"
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- mkdir -p ~/.claude ~/.codex
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- ss init -g --no-copy --targets claude,codex --mode merge --no-git --no-skill
```

Expected:
- `ssenv` env created
- `ss init` succeeds

### 2. Isolation + mode sanity (prevents false project/global confusion)

```bash
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- bash -c '
  set -e
  test "$(pwd)" = "$HOME"
  ss status -g >/dev/null
  cd /workspace
  ss list >/tmp/issue46-auto-mode.log 2>&1 || true
  ss list -g >/tmp/issue46-global-mode.log 2>&1 || true
  test -s /tmp/issue46-auto-mode.log
  test -s /tmp/issue46-global-mode.log
'
```

Expected:
- `ssenv enter <env> -- <cmd>` runs from isolated HOME (`pwd == $HOME`)
- In `/workspace`, auto mode and `-g` mode are both callable (outputs may differ)
- This verifies runbook commands are not accidentally bound to workspace project mode

### 3. Token + credential-helper precheck

```bash
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- bash -c '
  set -e
  if [ -n "${GITHUB_TOKEN:-}${GH_TOKEN:-}${SKILLSHARE_GIT_TOKEN:-}" ]; then
    echo "TOKEN_PRESENT"
  else
    echo "TOKEN_ABSENT"
  fi
  if command -v credential-helper >/dev/null 2>&1; then
    credential-helper status || true
  else
    echo "credential-helper command not found (skip)"
  fi
'
```

Expected:
- Shows whether token-based auth is available (`TOKEN_PRESENT` / `TOKEN_ABSENT`)
- Prints credential-helper state (or explicit skip)

### 4. Install GitHub subdir skill (Issue #46 reference scenario)

```bash
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- \
  ss install -g https://github.com/runkids/claude-skill-registry/tree/main/skills/documents/atlassian-search
```

Expected:
- Install command succeeds
- `~/.config/skillshare/skills/atlassian-search/SKILL.md` exists
- `~/.config/skillshare/skills/atlassian-search/.skillshare-meta.json` exists

Verify:

```bash
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- bash -c '
  test -f ~/.config/skillshare/skills/atlassian-search/SKILL.md
  test -f ~/.config/skillshare/skills/atlassian-search/.skillshare-meta.json
  find ~/.config/skillshare/skills -mindepth 1 -maxdepth 1 -type d | sort
'
```

### 5. (Optional but recommended) No-token fallback behavior

Runs the same install without token env to ensure fallback path still succeeds.

```bash
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- bash -c '
  env -u GITHUB_TOKEN -u GH_TOKEN -u SKILLSHARE_GIT_TOKEN \
    ss install -g https://github.com/runkids/claude-skill-registry/tree/main/skills/documents/atlassian-search \
    --name atlassian-search-no-token --force 2>&1 | tee /tmp/issue46-no-token.log
  test -f ~/.config/skillshare/skills/atlassian-search-no-token/SKILL.md
'
```

Expected:
- Install succeeds even without token envs
- Output may include API warning/fallback messages (acceptable)

### 6. Update the regular installed skill

```bash
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- ss update -g atlassian-search
```

Expected:
- Update succeeds (reinstall-from-source path for subdir metadata)
- Skill files remain present

Verify:

```bash
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- \
  bash -c 'test -f ~/.config/skillshare/skills/atlassian-search/SKILL.md'
```

### 7. Install tracked repo with optimization path

```bash
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- \
  ss install -g https://github.com/runkids/skillshare --track --name issue46-track --force
```

Expected:
- Tracked repo installed at `~/.config/skillshare/skills/_issue46-track`
- `.git` directory exists
- Optimization markers exist for shallow/partial clone (for supported remotes)

Verify:

```bash
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- bash -c '
  TRACK=~/.config/skillshare/skills/_issue46-track
  test -d "$TRACK/.git"
  test -f "$TRACK/.git/shallow"
  grep -Eq "partialclonefilter = blob:none|promisor = true" "$TRACK/.git/config"
'
```

### 7b. Install tracked repo from GitHub subdir URL (primary Issue #46 case)

```bash
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- \
  ss install -g https://github.com/majiayu000/claude-skill-registry/tree/main/skills/documents/atlassian-search \
  --track --name issue46-track-subdir --force
```

Expected:
- Command succeeds (no hang)
- Tracked repo exists with `.git`
- On sparse-capable git, `.git/info/sparse-checkout` usually contains `skills/documents/atlassian-search`

Verify:

```bash
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- bash -c '
  TRACK=~/.config/skillshare/skills/_issue46-track-subdir
  test -d "$TRACK/.git"
  test -f "$TRACK/.git/HEAD"
  if [ -f "$TRACK/.git/info/sparse-checkout" ]; then
    grep -q "skills/documents/atlassian-search" "$TRACK/.git/info/sparse-checkout"
  fi
'
```

### 8. Update tracked repo

```bash
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- ss update -g _issue46-track
```

Expected:
- Update command succeeds (up-to-date or updated)

### 9. Non-TTY quiet check for tracked install

```bash
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- bash -c '
  ss install -g https://github.com/runkids/skillshare --track --name issue46-quiet --force 2>&1 | tee /tmp/issue46-quiet.log
  ! grep -E "(Enumerating objects|Counting objects|Receiving objects|Resolving deltas)" /tmp/issue46-quiet.log
'
```

Expected:
- Install succeeds
- No raw git progress lines in piped output

### 10. Non-GitHub sparse checkout path (deterministic, local file:// repo)

```bash
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- bash -c '
  set -e
  REPO=~/tmp-repo-sparse-ok
  rm -rf "$REPO"
  mkdir -p "$REPO/skills/alpha"
  cat > "$REPO/skills/alpha/SKILL.md" <<EOF
# alpha
EOF
  git -C "$REPO" init
  git -C "$REPO" add .
  git -C "$REPO" -c user.name=e2e -c user.email=e2e@example.com commit -m init
  ss install -g "file://$REPO/skills/alpha" --name sparse-alpha --force
  test -f ~/.config/skillshare/skills/sparse-alpha/SKILL.md
'
```

Expected:
- Install succeeds from `file://` subdir source
- Installed skill exists at `~/.config/skillshare/skills/sparse-alpha/SKILL.md`

### 11. Sparse failure -> full clone fallback (fuzzy subdir)

```bash
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- bash -c '
  set -e
  REPO=~/tmp-repo-fuzzy-fallback
  rm -rf "$REPO"
  mkdir -p "$REPO/skills/pdf"
  cat > "$REPO/skills/pdf/SKILL.md" <<EOF
# pdf
EOF
  git -C "$REPO" init
  git -C "$REPO" add .
  git -C "$REPO" -c user.name=e2e -c user.email=e2e@example.com commit -m init
  ss install -g "file://$REPO/pdf" --name fuzzy-pdf --force 2>&1 | tee /tmp/issue46-fuzzy.log
  test -f ~/.config/skillshare/skills/fuzzy-pdf/SKILL.md
  grep -q "sparse checkout install fallback" /tmp/issue46-fuzzy.log || true
'
```

Expected:
- Install succeeds even though input subdir is fuzzy (`/pdf`)
- Skill ends up installed (full-clone fallback + fuzzy resolve path)
- Fallback warning may appear: `sparse checkout install fallback`

### 12. GHE API base handling

Always run routing sanity test:

```bash
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- \
  go test /workspace/internal/install -run TestGitHubAPIBase -count=1
```

Expected:
- Test passes and validates:
  - `github.com -> https://api.github.com`
  - `github.<corp>.com -> https://<host>/api/v3`

Optional live GHE check (if you have a reachable repo + token):

```bash
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- bash -c '
  if [ -n "${ISSUE46_GHE_SUBDIR_SOURCE:-}" ]; then
    ss install -g "$ISSUE46_GHE_SUBDIR_SOURCE" --force
  else
    echo "ISSUE46_GHE_SUBDIR_SOURCE not set; skip live GHE install check"
  fi
'
```

### 13. `update --force` tracked repo branch

```bash
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- bash -c '
  set -e
  TRACK=~/.config/skillshare/skills/_issue46-track
  echo "# local dirty change" >> "$TRACK/README.md"
  test -n "$(git -C "$TRACK" status --porcelain)"
  ss update -g _issue46-track --force
  test -z "$(git -C "$TRACK" status --porcelain)"
'
```

Expected:
- Local dirty changes are discarded by force-update path
- Command succeeds

### 14. (Manual) TTY progress sanity

Run manually in an interactive terminal (TTY) to verify progress readability:

```bash
docker exec -it "$CONTAINER" ssenv enter "$ENV_NAME" -- \
  ss install -g https://github.com/runkids/skillshare --track --name issue46-tty --force
```

Expected:
- Progress lines are readable and stable (no rapid MiB/s flicker spam)
- Stage-like updates remain visible (for example `Receiving objects: 42%`)

## Pass Criteria

- [ ] Step 1 setup/init succeeds
- [ ] Step 2 isolation/mode sanity succeeds
- [ ] Step 3 token + credential-helper precheck executed
- [ ] Step 4 subdir install succeeds and files exist
- [ ] Step 5 no-token fallback install succeeds (optional but recommended)
- [ ] Step 6 regular `update` succeeds
- [ ] Step 7 tracked install succeeds and optimization markers are present
- [ ] Step 7b tracked subdir URL install succeeds (sparse preferred)
- [ ] Step 8 tracked update succeeds
- [ ] Step 9 non-TTY output has no raw git progress lines
- [ ] Step 10 local non-GitHub subdir install succeeds
- [ ] Step 11 fuzzy subdir install succeeds via fallback path
- [ ] Step 12 GHE API base routing test passes
- [ ] Step 13 tracked `update --force` branch succeeds and cleans dirty state
- [ ] Step 14 manual TTY sanity checked (optional in CI)
