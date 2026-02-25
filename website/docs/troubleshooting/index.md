---
sidebar_position: 1
---

# Troubleshooting

Having issues? Start here.

## Quick Diagnosis

Run the doctor command:

```bash
skillshare doctor
```

This checks:
- Source directory
- Config file
- Target accessibility
- Symlink health
- Git status

---

## What's happening?

| Problem | Go To |
|---------|-------|
| I see an error message | [Common Errors](./common-errors.md) |
| Something isn't working on Windows | [Windows](./windows.md) |
| I need a step-by-step debugging process | [Troubleshooting Workflow](./troubleshooting-workflow.md) |
| I have a general question | [FAQ](./faq.md) |

---

## Quick Fixes

### Skills not appearing

```bash
skillshare sync
```

### Broken symlinks

```bash
skillshare sync --force
```

### Config issues

```bash
skillshare doctor
```

### Start fresh

```bash
rm ~/.config/skillshare/config.yaml
skillshare init
```

---

## Getting Help

If you can't resolve the issue:

1. **Gather information:**
   ```bash
   skillshare doctor
   skillshare status
   ```

2. **Search existing issues:** [GitHub Issues](https://github.com/runkids/skillshare/issues)

3. **Open a new issue** with:
   - Doctor output
   - Error messages
   - What you were trying to do
   - Operating system

---

## Related

- [Troubleshooting Workflow](./troubleshooting-workflow.md) — Step-by-step debugging
- [Commands: doctor](/docs/reference/commands/doctor) — Doctor command details
