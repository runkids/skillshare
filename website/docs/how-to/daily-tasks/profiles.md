---
sidebar_position: 4
---

# Work with skill profiles

Profiles save named selections of installed skills. Switching changes which skills
reach this configuration's tools; it keeps source skills available for editing,
updating and auditing. Profiles are separate from security policies used by
`audit --profile`.

## Choose the scope {#scope}

Open the dashboard for the configuration you want to change:

```bash
skillshare ui -g
# Or, from a project root:
skillshare ui -p
```

Global and project definitions are independent. Definitions live in shared config
YAML, while the active name and pending state are local to this configuration.
Another checkout can select a different profile from the same definitions.

## Create a selection {#create}

1. Open **Profiles** in the Library navigation, after Skills.
2. Choose **Create profile**, enter a name and a one-line description.
3. Add include patterns and optional exclude patterns. Press Enter or leave the
   pattern field to commit a pattern, then review **Selected skills**.
4. Choose **Create profile**. Creating a definition does not activate or sync it.

Names contain lowercase letters, digits, underscores or hyphens, starting with a
letter or digit, up to 64 characters. `none` is reserved. Patterns match flat
source names, such as `frontend__button`, with Go globs. Use `frontend__*` for a
group or `*` for all skills before exclusions. The preview comes from the server;
it lists selected names and warns about include patterns matching nothing.

An empty include selects **no skills**. A definition without a `skills` section
passes all skills through; editing only its description preserves that behavior.
Choose **Select specific skills** to deliberately restrict it. Excludes retained
with an empty include are inert until an include is added again.

## Assign from Skills {#assign}

Select skills in list or card view, or select a folder and its skills in tree
view. Choose **Add to profile…** or **Remove from profile…**, then choose a usable
profile. Folder selections expand to source skill names; overlapping selections
send each skill once. These actions are unavailable for agents.

Assignment changes only the definition. It does not activate a profile, sync,
remove source files, or override ignore rules, target filters or skill routing.
Adding removes an exact exclusion when possible. A matching glob exclusion
refuses the whole batch and names the skill and pattern. Your selection remains
available to retry after resolving the error. Unusable profiles show their reason
and cannot be chosen.

## Edit, rename and delete {#edit}

Open a usable profile from **Profiles**. Edit its description or patterns and
review the live selected list. **Save profile** is enabled only for changes;
**Cancel** resets the draft to the last saved form. A failed save keeps your draft
for retry. Saving the active definition offers **Sync now**: open the skills
preview, inspect it, then explicitly choose **Sync Now** to apply.

**Rename profile** changes its name without replacing its patterns or activating
an inactive profile. Save or cancel a dirty draft before renaming. Renaming the
locally active definition follows that name in local state.

**Delete profile** requires confirmation and is disabled for the active profile.
Switch away first. If it becomes active elsewhere before your request arrives,
the server refuses deletion. Deleting an inactive definition leaves installed
source skills and target files in place. Definitions with unsupported fields show
their reason and offer only rename or inactive delete. Their unknown contents and
comments survive rename; pattern editing and activation remain unavailable.

## Switch and recover {#switch}

Use the profile picker in the dashboard header to preview a switch. Target filters
still apply after the profile selection. Only this scope's enabled merge/copy
skill targets participate; symlink targets and other resource kinds stay separate.

A modified managed copy requires snapshot-first consent before it can be
discarded. An empty selection has its own confirmation. Collect edits into source
when you want to keep them there. Review returned snapshot paths, retention and
restore commands; restore replaces the destination as a whole.

An incomplete switch retains its target diagnostics and pending state. Repair
the reported target, then use the pending banner's **Sync now** action. Even an
up-to-date preview offers an explicit sync to settle pending state. Missing or
unusable active definitions offer **Choose profile** and **All skills** repairs;
there is no silent fallback.

See the [profile command](/docs/reference/commands/profile),
[filtering reference](/docs/reference/filtering#profiles) and
[backup and restore guide](/docs/how-to/daily-tasks/backup-restore).
