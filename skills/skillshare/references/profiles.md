# Profiles

Profiles select deployed skills without uninstalling source skills. They are separate from `audit --profile` security policies.

```bash
skillshare profile list --json
skillshare profile create writing --add-include 'docs__*'
skillshare profile show writing --json
skillshare profile use writing --dry-run --json
skillshare profile use writing
skillshare profile use --none
```

Use `-g` or `-p` to choose a config. Definitions are shared YAML under `profiles`; active state and pending are local to the canonical config identity. No `skills` key is unrestricted; `skills: {}` or empty/blank includes select nothing. Match flat source names with Go globs. Excludes require an include key; `include: ['*']` means everything before exclusions. New profiles without patterns are empty.

`edit` accepts `--description`, `--add-include`, `--remove-include`, `--add-exclude`, `--remove-exclude`; pattern flags repeat. `rename` follows the locally active name; `delete` refuses an active definition. Definition changes affecting active selection require a later sync.

Raw rename and inactive delete also work for unusable definitions with future keys. Rename retains unknown contents/comments and pending state; it does not enable unsupported selection. Active delete, edit and use still validate or refuse as appropriate.

`use` accepts `--dry-run`, `--no-sync`, `--none`, `--allow-empty`, `--discard-edits`, `--json`. It syncs valid enabled merge/copy skill targets, excluding registered projects and every other resource kind. Profiles narrow target filters and metadata routing. Symlink-mode roots remain unchanged.

A refused switch changes no state or target. An incomplete switch leaves the new active name pending and exits nonzero; retry `skillshare sync`. Modified managed copies and unmanaged collisions are kept. `--discard-edits` snapshots and verifies each listed modified copy and rechecks before deletion; it cannot discard unmanaged content. Ordinary `sync --discard-edits` uses the same rule while a profile or pending switch exists. Text and JSON report snapshot paths, retention and restore commands. Collect authored changes when they should become source content.

Project `sync --discard-edits` also covers selected eligible targets’ declared former skills folders. Use each reported moved restore command to recover that folder while preserving the current one. Profile switching does not run former-folder cleanup.

Profile-aware diff keeps protected entries visible with a reason and no executable-change count, including former folders. CLI text/JSON/TUI show preservation as kept. API selected entries keep force-sensitive skip behavior; departing kept entries never count under Force. Collect or remove unmanaged content manually; only modified managed copies offer verified discard.

Output fingerprints use versioned, framed path/content identities; source checksums are unchanged. A legacy profile copy is clean only if available source matches its recorded source checksum and reproduces the deployed bytes, including naming rewrites. This proves present reproducibility, not original deployment history. Missing source, failed replay and unknown/malformed/empty output encodings remain protected. No-profile source-free legacy verification remains compatible; binaries predating output fingerprints do not supply this protection after a downgrade.

Reads/dry runs create no lock. Non-dry operations retain an empty local lock file and its directories even on refusal; never delete it to bypass a running command. Unavailable active state refuses skill operations; repair with `profile use <name>` or `profile use --none`. Older binaries ignore the local selection and may drop definitions when saving. No-profile behavior stays unchanged except copy output fingerprint bookkeeping.
