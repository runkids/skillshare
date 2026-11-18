# Discover Git checkouts for managed project skill sync

## Problem

A workspace can hold many independent repositories. Codex discovers local skills from the current directory up to its Git root. When someone opens one repository directly, skills synced only to the workspace folder are out of scope. Registering every repository by hand works and leaves another registration to make for each clone or worktree.

## Proposed Solution

Add an optional list of scan directories to a managed project in the global configuration.

```yaml
projects:
  ~/work/suite:
    targets: [codex]
    repository_dirs: [".", ".worktrees"]
    skills:
      mode: merge
      include: ["team-*"]
```

- Resolve these paths relative to the declared workspace. Walk ordinary grouping directories and stop at each Git checkout. Recognize directory and worktree-file forms of `.git`. Hidden grouping directories can be selected explicitly.
- Expand the checkouts into the existing target model so global sync, scoped project sync, and dry runs use the same destinations. Each checkout inherits the workspace's skill filters and merge or copy mode.
- Keep explicit registrations and target paths in control. Skip checkouts with their own project configuration and skip directory links during discovery. Recheck path components before writing to a discovered checkout.
- Keep discovery optional. Agents, MCP servers, hooks, and Git ignore rules keep their separate workflows. Disabling discovery leaves existing checkout output in place.

The first version is configured in YAML and uses the existing sync preview. It needs no new dependencies. The [reference implementation](https://github.com/amphetamlne/skillshare/commit/75c9b4fb86a7a551057103849188f5682c76c974) includes configuration and CLI integration tests.

## Alternatives Considered

- Register every checkout as a machine-local project. This works today. The list and skill filters need upkeep as checkouts come and go.
- Sync the workspace skills to global targets. Every project then sees them, including projects outside the workspace.
- Add a separate discovery command that writes project records. That fits a one-time import. Deriving targets during sync also covers later clones and worktrees.

## Scope

- [ ] Small (1–3 files, fewer than 200 lines)
- [ ] Medium (3–10 files, 200–500 lines)
- [x] Large (10+ files)

The reference implementation connects config expansion, target ownership, CLI and dashboard sync, and tests. This proposal itself changes one document.

## Open Questions

- Is an explicit list of scan directories appropriate for the first version?
- Should a follow-up track checkout ownership for optional cleanup after a root leaves the scan?

Tracking issue [#953](https://github.com/runkids/skillshare/issues/953).
