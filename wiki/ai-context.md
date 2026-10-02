# AI Context Router

This page is the complete in-repository manual for maintaining agent documentation routing without an external skill. Load it with `python3 scripts/ai-context.py ai-context`.

## Layers

1. **Kernel:** root `AGENTS.md`, loaded for every task. It contains only universal rules and topic triggers.
2. **Scoped instructions:** package- or app-specific nested `AGENTS.md` files. Every scoped file must be loaded by at least one topic.
3. **Topics:** `wiki/ai-context.json` maps topics to complete files or exact headings. `scripts/ai-context.py <topic>` prints only those sections.
4. **History:** `wiki/history/` stores milestone logs in their original language. History is not loaded by default and is indexed in `wiki/README.md`.

`wiki/ai-context.json` is the single source of truth. The tables in `AGENTS.md` and `wiki/README.md` are discovery mirrors.

## When Documentation Must Change

Update documentation in the same change that makes it stale:

| Change | Update |
|---|---|
| Changed behavior described by a topic | That topic page |
| Found documentation that contradicts code | Verify the code, fix the documentation, and mention it in the handoff |
| Learned a cross-task lesson from a failure | A one-line behavior rule in the kernel plus the procedure in its topic |
| Finished a milestone | `wiki/history/<milestone>.md` and the `wiki/README.md` history index |
| Added a recurring kind of task | A new topic, JSON entry, and both topic tables |
| Added a nested `AGENTS.md` | Map it to a topic |

Do not put volatile state in the wiki, including progress counts, current owners, or today's blocker. Keep that information in the tracker.

## Adding or Changing a Topic

1. Put the content in `wiki/<page>.md`. A topic should represent a recognizable kind of work, not one ticket or file.
2. Map it in `wiki/ai-context.json` using a complete file or exact heading.
3. Add it to the tables in `AGENTS.md` and `wiki/README.md`.
4. Run `python3 scripts/ai-context.py check`.
5. Render the topic and read it as an agent would; it must be sufficient on its own for the task.

Heading references must be unique and exact. A section ends at the next heading of the same or higher level. Never use line ranges.

## What Check Enforces

- Every path stays inside the repository and exists, and every heading matches exactly once.
- `AGENTS.md` and every topic stay within byte budgets. Split an over-budget topic instead of raising its cap.
- Every JSON topic appears in `AGENTS.md`.
- There are no orphans:
  - every non-history wiki page is loaded by a topic;
  - every history file is indexed in `wiki/README.md`;
  - every nested `AGENTS.md` is loaded by a topic.

Only genuine human-only exceptions belong in the JSON `unrouted` map, and every exception requires a reason.

## Skills and CI

Skills retain workflow phases, report templates, and helper invocation. They load the applicable topic before acting; repository rules must not exist only in a skill.

The router check runs through the `Makefile` `docs-check`/`check` targets and the test and website workflows. After changing the router, run:

```sh
python3 scripts/ai-context.py check
```
