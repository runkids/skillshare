# CLI E2E Runbook: Git Token Auth Stays Out of Process Arguments

Validates that HTTPS token auth (issue #565) reaches git through a credential helper that reads the token from the environment, so the token never appears in a git process's arguments, a stale token does not break public repos, and the user's own credential helper (their keychain) is never asked or erased for that host.

**Origin**: v0.25.11 — auth used a `url.<user>:<token>@host.insteadOf` rewrite; git passed the rewritten URL to `git-remote-https` as an argument, visible to any user through `ps`.

## Scope

- Installing a public repo with a stale `GITHUB_TOKEN` succeeds, and the git trace never contains the token
- The user's global credential helper is not called for the token's host
- A repo that needs auth (401) fails with the token-rejected hint, without prompting, and still without calling the user's helper
- Not covered: a successful private clone with a valid token (needs a real private repo and token; covered by `TestAuthEnv_GitAsksOnlySkillshareHelper` with real `git credential fill`)

## Environment

Run inside the devcontainer with mdproof isolation. Network access required (clones from `github.com`). `GIT_TRACE=<file>` records every command git runs, including `git-remote-https` and credential helpers, with their arguments.

## Steps

### 1. Public install with a stale token keeps the token out of git arguments

```bash
rm -f "$HOME/user-helper.log" "$HOME/git-trace.log"
git config --global credential.helper "!f() { echo \"\$1\" >> $HOME/user-helper.log; echo username=realuser; echo password=stored; }; f"
GITHUB_TOKEN=ghp_FAKE_runbook_565 GIT_TRACE="$HOME/git-trace.log" \
  ss install https://github.com/JuliusBrussee/caveman.git/skills/caveman --name caveman-565 --skip-audit -y >/dev/null 2>&1
echo "install_exit=$?"
echo "remote_https_ran=$(grep -c remote-https "$HOME/git-trace.log")"
echo "token_in_trace=$(grep -c ghp_FAKE_runbook_565 "$HOME/git-trace.log")"
echo "user_helper_calls=$(cat "$HOME/user-helper.log" 2>/dev/null | wc -l)"
```

**Expected**:
- regex: (?m)^install_exit=0$
- regex: remote_https_ran=[1-9]
- regex: (?m)^token_in_trace=0$
- regex: (?m)^user_helper_calls=0$

### 2. Rejected token fails with the hint, without a prompt or the user's helper

```bash
rm -f "$HOME/user-helper.log" "$HOME/git-trace.log"
git config --global credential.helper "!f() { echo \"\$1\" >> $HOME/user-helper.log; echo username=realuser; echo password=stored; }; f"
GITHUB_TOKEN=ghp_FAKE_runbook_565 GIT_TRACE="$HOME/git-trace.log" \
  timeout 120 ss install https://github.com/runkids/does-not-exist-565 --skip-audit -y </dev/null 2>&1
echo "install_exit=$?"
echo "skillshare_helper_ran=$(grep -c SKILLSHARE_GIT_AUTH_PASSWORD "$HOME/git-trace.log")"
echo "token_in_trace=$(grep -c ghp_FAKE_runbook_565 "$HOME/git-trace.log")"
echo "user_helper_calls=$(cat "$HOME/user-helper.log" 2>/dev/null | wc -l)"
```

**Expected**:
- token rejected
- regex: (?m)^install_exit=1$
- regex: skillshare_helper_ran=[1-9]
- regex: (?m)^token_in_trace=0$
- regex: (?m)^user_helper_calls=0$

## Pass Criteria

- All steps marked PASS
- `install_exit=1` in Step 2 comes from the auth failure, not from `timeout` (exit 124 would mean git waited on a prompt)
