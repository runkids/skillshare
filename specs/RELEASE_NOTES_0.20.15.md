# skillshare v0.20.15 Release Notes

## TL;DR

1. **Git operations fail visibly** — dashboard branch refreshes, checkouts, and source URL edits now stop when remote updates fail instead of continuing with stale state.
2. **Target cleanup is safer** — target removal preserves config when filesystem cleanup fails, so users can fix the issue and retry.
3. **Automation output is cleaner** — version checks handle release-tag formats correctly and JSON-mode cleanup warnings no longer pollute parseable output.

## Git operations fail visibly

Dashboard branch refreshes and checkouts now surface fetch failures. If a remote cannot be reached or authentication fails, Skillshare reports the problem instead of showing stale branch data or continuing a checkout with outdated information.

Source URL edits for tracked skills and agents are stricter too. Skillshare updates the repository remote before saving metadata, keeping the displayed source and the repository's remote in sync.

## Target cleanup is safer

Removing a target from the dashboard now stops when Skillshare cannot inspect the target, remove managed files, or clean up symlinks. The target remains in config so users can fix permissions or filesystem state and try again without recreating it.

## Version checks and JSON output are cleaner

Update checks now accept release versions with a leading `v`, while malformed versions still fail closed. Local metadata builds only advertise release versions when built from a clean exact tag; other builds stay in `dev` mode.

JSON-mode commands also stay quiet when temporary Git clone cleanup warnings occur, keeping automation-friendly output parseable.

## Skill analysis failures are explicit

Skill linting now reports rule load problems instead of crashing or dropping the error after the first run. Analysis commands get a clear failure when lint rules cannot be loaded.

## Dashboard polish

Audit finding severity dots are vertically centered next to their badges and messages.
