# Shared MCP servers for chosen projects

## Problem

Issue: [#976](https://github.com/runkids/skillshare/issues/976).

A user with several projects keeps the same MCP servers in each of them. Today there are
three ways to set that up, and none of them works for a server that only some projects
should get:

- **Global `mcp.servers`.** The server is written to every Agent's global MCP file, so it
  loads in every folder on the machine. A per-project `disabled` entry can turn it off in a
  project. That only works for Claude Code, OpenCode, Kilo Code, Pi and Oh My Pi, and the
  user has to add one entry for every project that should *not* have the server.
- **A copy per project** under `mcp.projects.<root>.servers`. The dashboard's
  **From another project** tab makes the copy. Changing the server then means editing every
  copy. With five projects and five servers, that is twenty-five entries to keep in step.
- **YAML anchors**, as the `mcp` reference suggests. They hold only until the dashboard
  saves. Verified on `115b50a3d` with a config where `~/work/api` holds the anchor and
  `~/work/web` and `~/work/docs` alias it:
  - Editing a server in `~/work/api`, which holds the anchor, wrote `~/work/docs` out in full
    with the old values.
  - Editing one in `~/work/web`, which aliases it, changed only `~/work/web`.
  - With a server-level alias (`internal-docs: *idocs`), editing the anchored server left
    the alias with the old values.

  The UI never shows that two entries are linked.

The reporter asks for the model skills already have: define each server once, decide where
it applies (globally or in chosen projects), and have an edit reach every place it applies.

Skills work this way. There is one source, and each project under `projects` picks skills
with `include` and `exclude`. MCP has no equivalent of that per-project pick.

## Proposed Solution

Keep one server list, `mcp.servers`. Let a server opt out of the global files, and let each
project root name the servers it uses.

```yaml
mcp:
  targets: [claude]
  servers:
    context7:                    # global, as today: loads in every folder
      command: npx
      args: ["-y", "@upstash/context7-mcp@4.4.1"]
    internal-docs:
      global: false              # written to no global file
      url: https://docs.example.com/mcp
  projects:
    ~/work/api:
      use: [internal-docs]       # written to this project's files
      servers:
        local-db:                # this project only, as today
          command: ./scripts/db-mcp
    ~/work/web:
      use: [internal-docs]
```

Editing `internal-docs` once changes both projects on the next `skillshare sync mcp`.

### Rules

1. **`global: false`.**
   - It is allowed only on a server in the global config's `mcp.servers`, including servers
     the global config reads from its `sources.mcp` file.
   - It is an error in a project's own `config.yaml` (project mode, including its
     `sources.mcp` file), on a server under `mcp.projects`, and together with `disabled`.
   - Leaving it out, or `global: true`, keeps today's behavior. A save drops `global: true`.
   - **Edits keep it.** A request that replaces a server without saying `global` keeps the
     stored value. Several writers rebuild the server from its fields, so this matters:
     - the dashboard dialog, which saves `{ ...build(), targets }` from the draft;
     - `skillshare mcp add --replace`;
     - `mcp_edit.go` and `mcp_interactive.go`;
     - import with replace.

     Only an explicit `global` in the request changes the value. Otherwise an edit could
     silently make a shared server global and unassign it everywhere (rule 8).
2. **Global files.** A `global: false` server is never written to an Agent's global file.
   Turning `global` off on an existing server removes Skillshare's entries from the global
   files on the next sync, through the existing ownership ledger.
3. **`use: [name, ...]`** sits under a root in `mcp.projects`, next to `targets` and
   `servers`. Each name must be a `global: false` server in `mcp.servers`. These are errors
   that name the root:
   - a name that does not exist;
   - a name that is a global server ("already loads in every project");
   - a name listed twice.
4. **No silent precedence.** In one root, a name may not be both in `use` and in that
   root's `servers`, whether as a server or as a `disabled` switch. This is an error.
   Per-project overrides are left for later (see Open Questions).
5. **Targets.** A used server is rendered in a root exactly as if it were declared in that
   root's `servers`:
   - its own `targets` if set;
   - otherwise the project's `targets`;
   - otherwise `mcp.targets`.

   An explicit `targets: []` keeps it in Skillshare only, as today.

   **Account targets.** Project entries refuse account targets such as `claude-work`
   (`Source.checkTargets`; inside `mcp.projects` the Agent's own name is used). Global
   servers may name them. A `global: false` server's `targets` is therefore refused when it
   names an account target. The error says to use the Agent's name, as a project server
   would.
6. **Unused shared servers.** A `global: false` server that no root uses is written nowhere.
   `mcp list`, `mcp check` and the dashboard say "not used by any project".
7. **Removing a shared server** also removes its name from every `use` list, in the same
   save. The preview lists the affected projects and files.
8. **Switching a used server back to global** removes it from every `use` list in the same
   save, because it will load in every project anyway.
9. **Unchanged behavior.**
   - Removing a name from `use`, or removing the root, removes the written entries on the
     next sync, as it does for a project server today. Ledger keys are already per file
     and name.
   - Anchors in `mcp.projects` still parse as before.
   - Roots that rules 7 and 8 edit are re-encoded, like any touched root today. They lose
     comments, and their aliases are expanded.
10. **Pending changes and sync scope.** Editing a shared server changes every root that
    uses it, so its pending changes belong to those roots:
    - The MCP page's sync box counts them with the projects, as it does for project
      servers today.
    - The project tab shows them for its own root.
    - Syncing one project (`syncProject`) applies that root only and leaves the other roots
      pending.
    - The MCP page lists shared servers, so the docs line "The MCP page lists global servers
      only" changes.

### Implementation shape

The model needs two new fields:

- `Server.Global *bool` (`yaml:"global,omitempty" json:"global,omitempty"`). The JSON
  decoder rejects unknown fields, so the tag is required.
- `Project.Use []string`. The `ParseProjects` error message becomes "takes only targets,
  use and servers".

Resolution happens in one place, the `Source`, so no consumer reimplements it:

- `Source.globalServers()`: `mcp.servers` without the `global: false` servers. It feeds the
  global `renderScope`.
- `Source.globalServer(name)`: one server of that list, for the readers that look up a
  single name (rendering a switch, following a switch).
- `Source.projectScopes()`: each root with its own servers plus the servers it uses. It
  feeds the per-root `renderScope`.

These places switch to the two helpers:

- `renderWithRoots`;
- `toolPolicyNotices`;
- the unmanaged-server finder used by import (so a used server in a project file is not
  reported as unmanaged).

`mcp check` and tool pins keep one entry per shared server, under the global list (decided
for v1, see Open Question 3). `checkClientRules` renders that entry in each root that uses
it, and the sync findings of every using root attach to that one entry.
`mcpServersWithoutTargets` reports a shared server's `targets: []` once.

A root that holds only `use` is still a key under `mcp.projects`, so the project-removal
warning (`leftoverWarnings`, which reads root keys) needs no change.

Saving keeps the definitions and the references apart. `use` is written back with its root,
and a shared server stays once in `mcp.servers`; references are never expanded into copies.
The plan revision already hashes `Servers` and `Projects` as JSON, so the new fields take
part through their tags.

On the Web API, `Mutation` gains `Using *bool`. With `Project` and `Name` set, it adds that
one name to the root's `use` list or takes it out. It changes one name rather than replacing
the list, so two switches flipped from pages loaded at the same time both stay. It takes no
`Server` and no `Remove`.

The config editor runs the same rules before it saves: it reads the servers of an external
`sources.mcp` file to check `use` against them, and it refuses `global: false` in a project's
own config.

### Dashboard

- **MCP page.** The server dialog gets **Loads in: Every project / Only projects that turn
  it on**.
  - A shared server's row shows **Opt-in · N projects**, with the folders.
  - Saving a shared server says that it updates N projects.
  - Deleting it, or turning **Every project** back on, previews the `use` lists it changes
    (rules 7 and 8).
- **Project MCP tab.** Three sections, each with a one-line hint, so the difference reads
  as on by default / off by default / defined here:
  - **On in every project**: the global servers, with the off switches as before.
  - **Opt-in servers** (new): every `global: false` server with a switch; on means the
    server is in this root's `use`. The names "Shared" and "Global" alone did not say
    which way the default went.
  - **Only in this project**: the root's own servers, as before.
  - **From another project** stays, for one-off copies.

### CLI

There is no CLI path to `mcp.projects` today: no CLI code sets `Mutation.Project`. To stay
consistent, v1 adds no assignment command. Shared servers are set up in the dashboard or in
YAML.

The CLI does read them:

- `mcp list` (text, `--json` and the TUI) marks `global: false` servers and lists their
  projects. `--json` adds a top-level `shared` array of `{name, projects}`; the plan itself
  is unchanged. The TUI detail lists the projects that use the server, shows their targets, and names
  the project of each sync line.
- `mcp check` and doctor report each shared server once (see Open Questions).
- `skillshare mcp add` accepts `global: false` through `--file` and pasted config like any
  other field. A dedicated flag is an open question: `--global` is already the mode flag.

### Docs

- `reference/commands/mcp.md`: in "Manage several projects from the global config", replace
  the YAML-anchor advice with `global: false` and `use`, and keep anchors only as a note.
- In the same file, the MCP page's `--json` and summary text: shared servers are listed
  there now.
- `how-to/daily-tasks/sharing-mcp.md`, `reference/targets/configuration.md`, and the
  ja/ko/zh-Hans/zh-Hant copies.
- The built-in skill's MCP reference.

### Tests (state matrix before the first review)

- Each server kind (global, `global: false` used, `global: false` unused) in each project
  setup:
  - a root with only `use`;
  - a root with `use` and its own servers;
  - a root whose project targets are set, and one where they are unset.
- The same name in `use` and in `servers`: as a server, and as a `disabled` switch.
- Each refused place: `global: false` in project mode, and inside `mcp.projects`.
- Removing a name from `use`; deleting a used shared server; turning `global` back on.
- Editing a used shared server from each writer keeps `global: false` and every `use`
  list. The writers are the dashboard dialog, `mcp add --replace`, `mcp_edit.go`,
  `mcp_interactive.go` and import with replace.
- A shared server whose `targets` names Agents the project does not use, and one that
  names an account target (refused).
- `global: false` in the global `sources.mcp` file (allowed) and in a project's
  `sources.mcp` file (refused).
- An edited shared server used by two roots: both show pending. Syncing one root leaves
  the other pending.
- Claude's off list and OpenCode switches are unaffected when a global server shares a name
  with nothing.
- A config with anchors in `mcp.projects` loads and saves as before, except in roots that
  rules 7 and 8 re-encode.
- Check and doctor list a shared server once, and import does not report it as unmanaged.

## Alternatives Considered

- **A separate `mcp.library` section.** It is the same model, but it gives two server lists
  and two places to look for a name. The MCP page would need a second list.
- **Reusing `targets: []`.** On a server it already means "kept in Skillshare only, written
  nowhere". It would then need a second meaning ("but write it where a project uses it"),
  and an empty `targets` could no longer pause a shared server.
- **Assignment on the server (`projects: [roots]`).** This duplicates root keys that
  already live under `mcp.projects`. It drifts when a project is moved or removed, and it
  puts the choice on the MCP page instead of the project tab, where users decide what a
  project gets. `use` on the root mirrors `projects.<root>.skills.include`.
- **Keeping anchors and making saves preserve them.** Links stay invisible in the UI, and
  an edit on the alias side would still need a guess: change every project, or only this
  one.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

## Open Questions

1. **Key names.** Are `global: false` and `use` the right names? Alternatives are
   `scope: projects` and `shared: true` on the server, and `servers_from` on the root.
2. **Converting existing copies.** Should v1 include it? A project server that two or more
   roots hold with the same name and the same settings (the comparison the copy picker
   already uses) would get **Share across N projects**. That moves the definition to
   `mcp.servers` with `global: false` and replaces each copy with a `use` entry. The
   rendered files stay the same unless targets differ.
3. **Checks.** Should `mcp check` and its live tool check run a shared server once, or once
   per root? A project server starts in its project folder, so a once-only check would
   start in the first using root, and tool pins would be keyed once instead of per root.
   v1 decision: once, in the global list, with client rules and sync findings taken from
   every using root. Revisit if per-root `env` overrides (question 4) land.
4. **Per-project overrides.** Should a later version allow a root to change a used server,
   for example its `env`, instead of refusing the name in both places?
5. **CLI.** Should v1 add assignment commands (`mcp use` / `mcp unuse --root`), or wait
   until the CLI can address `mcp.projects` at all? Either way, `mcp remove` and
   `mcp add --replace` already change `mcp.servers`, so under rules 7 and 8 they edit
   `use` lists. Their preview and `--json` output must list the projects affected.
6. **Mixed versions.** Servers and project roots are decoded strictly, and `config.yaml`
   is often shared across machines. Once `global` or `use` appears, an older Skillshare
   rejects the whole MCP section. Options:
   - accept that and note the minimum version in the docs and the changelog;
   - write `use` lists and the `global` flag somewhere older versions ignore.
