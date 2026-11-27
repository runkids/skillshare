# Feature Proposal: Read-only fleet inventory of skill deployments

## Problem

Issue: [#934](https://github.com/runkids/skillshare/issues/934).

The web UI shows targets on the machine it runs on. Someone who runs Skillshare
on several machines cannot see in one place which skills reached which
targets, because a project target can exist on only one of those machines.

What the configuration selects is also not proof that a skill was delivered.
The sync matrix (`internal/server/handler_sync_matrix.go`) reports `synced`
from filter rules alone, so it describes what is wanted, not what happened.
Evidence of delivery is in each target's `.skillshare-manifest.json`
(`internal/sync/manifest.go`): `symlink` for a link made in merge mode, or a
SHA-256 checksum for a copy. That file exists only on the machine that holds
the target.

Git or Syncthing can carry files between machines, but on their own they
cannot show which project target received a skill.

## Proposed Solution

A standalone, read-only companion that sits outside Skillshare core, following
the companion direction discussed in #239. It adds no Skillshare command,
server endpoint, package or configuration field, and does not commit
Skillshare to maintaining federation.

It has two parts:

- **Exporter**, run on each machine by its owner. It turns one explicitly
  supplied description of that machine's targets and manifest evidence into a
  small, versioned JSON snapshot. It does not discover files or read
  Skillshare configuration by itself.
- **Consumer**, run anywhere. It reads snapshots the operator has copied to it
  and prints one report per skill, project alias and machine.

Each entry in the report has one of three states:

| State | Meaning |
|---|---|
| `applied` | A manifest entry or a link the caller verified shows the skill is in the target |
| `configured` | The skill is wanted, but no evidence shows it was delivered |
| `unknown` | Evidence is missing, unreadable or ambiguous |

Wanting a skill never produces `applied`. The consumer passes recorded states
through unchanged and never upgrades one from another.

Freshness is reported separately from state. A snapshot is `fresh` or `stale`
by its age, or `unknown` when the snapshot's time is too far ahead of the
reader's clock to judge. A machine the operator expects but has supplied no
snapshot for is reported `missing`, which is different from a machine that
sent a snapshot with no matching deployment.

### Opt-in aliases

Export is off until the machine's owner turns it on, and a project is exported
only when the owner gives it an alias. Unlisted projects are left out
entirely.

Snapshots carry only aliases: no paths, usernames, hostnames, repository names
or skill contents. Every label (machine, skill, project, harness) must match
one grammar, `[A-Za-z][A-Za-z0-9_-]{0,63}`, so a path, hostname, URL or email
address cannot pass as one. A name that fits the grammar can still be a real
name, so the owner chooses what to publish. The exporter refuses the whole
export, rather than writing part of it, if an output value matches an input
path or the machine's hostname, username or home directory.

The consumer accepts only machines and labels listed in a registry the operator
owns. That registry says which machines are expected; it is not
authentication or enrollment.

Both programs reject malformed input, unknown fields, duplicate JSON keys and
duplicate entries, and produce no partial output.

### Transport

Snapshots are plain files the operator moves by any means (copying, Git,
Syncthing). The first version has no network listener, enrollment, credentials
or new sync protocol.

### Acceptance

Using only invented fixtures: one project target exists only on machine A and
another only on machine B. One report shows both, keeps `configured` and
`applied` apart, and makes a stale snapshot and a missing machine obvious.

## Alternatives Considered

- **Bidirectional federation in core**: each Skillshare instance would query
  the others. This needs a listener, trust between machines and a protocol
  Skillshare would have to maintain. Out of scope for this proposal.
- **Extending the web UI to remote hosts**: same transport and trust cost, and
  it couples the dashboard to machines it cannot see.
- **Sharing configuration through Git or Syncthing only**: shows what each
  machine wants, not what it received.
- **Reading the sync matrix**: describes filter results, not delivery.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

The estimate is for the companion and its tests, which live outside Skillshare
core. No Skillshare Go package, command, dashboard page or configuration file
changes.

## Open Questions

1. **Where should the companion live?** A separate repository, or a `contrib/`
   directory that Skillshare does not build, import or run?
2. **Skill names that do not fit the alias grammar.** Skillshare flat names
   can start with `_` (skills from tracked repositories), can start with a
   digit, and can exceed 64 characters when nested names are joined with
   `__`. Under the current grammar, one such name in a manifest stops the
   whole export, even when no target wants that skill. Options:
   - an explicit mapping, owned by the machine's owner, from real flat names
     to published aliases, with unmapped names left out of the snapshot;
   - widening the grammar to allow a leading `_` and longer names, which
     publishes more of the real name.

   The proposal leans towards the explicit mapping. Which would you accept?
3. **Gathering the exporter input.** The exporter needs a description of the
   machine's targets and manifests. Should a later version read this from
   Skillshare (for example a read-only `--json` output), or should that stay
   with the caller so the companion never reads Skillshare state directly?
4. **Signing.** Snapshots are unsigned, so anyone who can write a snapshot file
   can claim any state for a registered machine. Is that acceptable for a
   first, file-only version?

Nothing here has been validated against a real fleet of machines. Any
reference implementation offered with this proposal would be tested only with
invented fixtures.
