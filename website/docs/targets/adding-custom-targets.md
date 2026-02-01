---
sidebar_position: 3
---

# Adding Custom Targets

Add any tool with a skills directory to skillshare.

## Overview

If your AI CLI isn't in the [supported list](./supported-targets), you can add it manually.

---

## Add a Target

```bash
skillshare target add <name> <path>
```

### Example

```bash
skillshare target add aider ~/.aider/skills
skillshare sync
```

---

## Requirements

### Path must exist

Create the directory first if needed:

```bash
mkdir -p ~/.myapp/skills
skillshare target add myapp ~/.myapp/skills
```

### Path should end with `/skills`

This is recommended but not required:

```bash
# Recommended
skillshare target add myapp ~/.myapp/skills

# Also works
skillshare target add myapp ~/.myapp/prompts
```

---

## Verify

After adding:

```bash
# Check target
skillshare target myapp

# Sync to new target
skillshare sync

# Verify
skillshare status
```

---

## Common Scenarios

### Add new AI CLI tool

```bash
# 1. Find where the tool stores skills
# (Check tool documentation)

# 2. Create directory if needed
mkdir -p ~/.newtool/skills

# 3. Add as target
skillshare target add newtool ~/.newtool/skills

# 4. Sync
skillshare sync
```

### Add project-specific target

```bash
# Sync skills to a specific project
skillshare target add myproject ~/projects/myapp/.ai/skills
skillshare sync
```

### Add multiple tools

```bash
skillshare target add tool1 ~/.tool1/skills
skillshare target add tool2 ~/.tool2/skills
skillshare target add tool3 ~/.tool3/skills
skillshare sync
```

---

## Change Sync Mode

After adding, you can change the sync mode:

```bash
# Default is merge mode
skillshare target myapp --mode symlink
skillshare sync
```

See [Sync Modes](/docs/concepts/sync-modes) for details.

---

## Remove Target

If you no longer need a target:

```bash
skillshare target remove myapp
```

This:
1. Creates a backup
2. Replaces symlinks with real files
3. Removes from config

---

## Troubleshooting

### "path does not exist"

Create the directory first:

```bash
mkdir -p ~/.myapp/skills
skillshare target add myapp ~/.myapp/skills
```

### Target not syncing

Check if the target is enabled:

```bash
skillshare target list
skillshare target myapp
```

### Wrong path

Remove and re-add:

```bash
skillshare target remove myapp
skillshare target add myapp /correct/path/skills
```

---

## Related

- [Supported Targets](./supported-targets) — Built-in targets
- [Configuration](./configuration) — Edit config directly
- [Sync Modes](/docs/concepts/sync-modes) — Merge vs symlink
