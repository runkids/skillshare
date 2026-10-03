---
sidebar_position: 8
---

# Recipe: Many Projects, One Config

> Send skills and MCP servers into several project folders from the global config, with one sync.

## Scenario

Global targets such as `~/.claude/skills` are read in every project, so every project sees the same skills. If that is what you want, you do not need this recipe.

This recipe is for when projects should get **different** things:

- You have many skills installed, and a frontend project only needs `frontend-*`. Fewer skills in a session means less context spent on descriptions and fewer wrong picks.
- One project must not load an MCP server that is fine everywhere else, such as a client's repository.
- A project should hold real copies of its skills so they can be committed, without committing any Skillshare config.
- A tool you use only reads a folder inside the project.

[Project mode](/docs/how-to/recipes/skill-per-project-workflow) also gives each project its own set. It keeps a `.skillshare/config.yaml` in every project, and you sync from inside each folder. `projects` in the global config gives the same result from one file on your machine:

| | Global targets | Global `projects` | Project mode |
|---|---|---|---|
| **Who gets the skills** | Every project, the same set | The folders you list, a set each | That one project |
| **Where the setup lives** | Your machine | Your machine | The project's repo |
| **Teammates get it** | No | No | Yes, by cloning |
| **Files added to the project** | None | Only the synced skills and agents | `.skillshare/` plus the synced files |
| **Sync** | One `sync` from anywhere | One `sync` from anywhere | `sync` inside each project |

Choose project mode when the setup should travel with the repo. Choose `projects` for your own projects, for client or open-source repos where you cannot add a `.skillshare/`, and when you want one `sync` to update them all.

## Solution

### Skills and agents: `projects`

```yaml
# ~/.config/skillshare/config.yaml
projects:
  ~/work/project01:
    targets: [claude, codex]
    skills:
      mode: copy
      include:
        - myskill-*
    agents: {}
```

```bash
skillshare sync --dry-run   # preview
skillshare sync
```

- `targets` names the tools you use in that project. Skillshare writes to each tool's project path, here `.claude/skills` and `.agents/skills`, so there is no path to type.
- `skills` and `agents` switch that part on. Left empty, they sync everything; `include` and `exclude` narrow it down. See [Filtering skills](/docs/how-to/daily-tasks/filtering-skills).
- `copy` writes real files, so the project can commit them. Leave the default `merge` if symlinks are fine.

In the dashboard, the **Projects** page does the same: **Add project**, pick the targets, and choose what to sync. See [`projects`](/docs/reference/targets/configuration#projects) for every field.

### MCP servers: `mcp.projects`

MCP servers are written into each Agent's own config file, so they are listed by project folder instead of by path:

```yaml
# ~/.config/skillshare/config.yaml
mcp:
  servers:
    context7:
      command: npx
      args: ["-y", "@upstash/context7-mcp"]
      targets: [opencode]
  projects:
    ~/work/project01:
      servers:
        context7:            # loaded everywhere else, off here
          disabled: true
          targets: [opencode]
```

```bash
skillshare sync mcp --dry-run   # preview every file
skillshare sync mcp
```

See [`mcp`: manage several projects](/docs/reference/commands/mcp#manage-several-projects-from-the-global-config) for the fields and limits.

## Verification

- `skillshare sync` reports the project's targets, for example `project01@claude  1 copied`
- `~/work/project01/.claude/skills/` contains only the skills matched by `include`
- `skillshare sync mcp --dry-run` lists one line per project file
- A second `skillshare sync mcp` reports every entry as `unchanged`

## Variations

- **A folder outside the tool paths**: a target is just a name and a path, so `skillshare target add project01 ~/work/project01/some/folder` still works for a folder no tool's project path covers. When such a target does point at a tool's project path, the dashboard's **Projects** page offers to convert it.
- **Commit or ignore**: in `copy` mode Skillshare also writes `.skillshare-manifest.json` into the target folder to track what it copied. Commit it with the skills, or add it to `.gitignore`.
- **Path overlap warning**: if a project folder is one another target already uses, `sync` prints a path overlap warning. Run `skillshare doctor` to see which targets share it.
- **Same server in several projects**: define it once under one project with a YAML anchor (`docs: &docs`) and reuse it in the others (`docs: *docs`). See the [`mcp` reference](/docs/reference/commands/mcp#manage-several-projects-from-the-global-config).
- **Shared projects**: teammates who clone the project do not get your global config. When the setup must travel with the repo, use [project mode](/docs/how-to/recipes/skill-per-project-workflow).

## Related

- [`target` command reference](/docs/reference/commands/target)
- [`mcp` command reference](/docs/reference/commands/mcp)
- [Sharing MCP servers](/docs/how-to/daily-tasks/sharing-mcp)
