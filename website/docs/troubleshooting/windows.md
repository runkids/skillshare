---
sidebar_position: 3
---

# Windows

Windows-specific issues and solutions.

## Installation

### How do I install on Windows?

**PowerShell:**
```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

**Or download manually:**
1. Go to [releases](https://github.com/runkids/skillshare/releases)
2. Download the `.zip` for Windows
3. Extract and add to PATH

---

## Permissions

### Does skillshare need admin privileges?

**No.** Skillshare uses NTFS junctions instead of symlinks, which don't require admin privileges.

NTFS junctions work like symlinks for directories but are available to all users.

---

## File Locations

### Where are config files on Windows?

```
%USERPROFILE%\.config\skillshare\config.yaml
%USERPROFILE%\.config\skillshare\skills\
%USERPROFILE%\.config\skillshare\backups\
```

Typically:
```
C:\Users\YourName\.config\skillshare\
```

### Where are target directories?

```
%USERPROFILE%\.claude\skills\
%USERPROFILE%\.cursor\skills\
%USERPROFILE%\.codex\skills\
```

---

## Environment Variables

### How do I set GITHUB_TOKEN on Windows?

**Current session only:**
```powershell
$env:GITHUB_TOKEN = "ghp_your_token"
```

**Permanent (user-level):**
```powershell
[Environment]::SetEnvironmentVariable("GITHUB_TOKEN", "ghp_your_token", "User")
```

**Then restart PowerShell.**

### How do I set SKILLSHARE_CONFIG?

```powershell
$env:SKILLSHARE_CONFIG = "C:\path\to\custom\config.yaml"
skillshare status
```

---

## Common Issues

### `junction creation failed`

**Cause:** Target path already exists as a file or incompatible type.

**Solution:**
```powershell
# Backup and remove existing
skillshare backup
Remove-Item -Path "$env:USERPROFILE\.claude\skills" -Recurse -Force
skillshare sync
```

### `path too long`

**Cause:** Windows has a 260 character path limit by default.

**Solution:** Enable long paths:
```powershell
# Run as Administrator
Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Control\FileSystem" -Name "LongPathsEnabled" -Value 1
```

Then restart.

### `access denied`

**Cause:** File or directory is in use or protected.

**Solutions:**
1. Close any programs using the files
2. Check antivirus isn't blocking
3. Run PowerShell as Administrator (rarely needed)

### `symlinks not working`

**Cause:** You're seeing symlinks instead of junctions.

**Note:** Skillshare uses NTFS junctions on Windows, not symlinks. If you see symlink errors, ensure you're using the Windows version of skillshare.

---

## PowerShell Tips

### Aliases

Add to your PowerShell profile (`$PROFILE`):
```powershell
Set-Alias -Name ss -Value skillshare
function sss { skillshare sync }
function ssp { param($m) skillshare push -m $m }
function ssl { skillshare pull }
```

### Check PowerShell version

Skillshare works with PowerShell 5.1+ and PowerShell Core 7+:
```powershell
$PSVersionTable.PSVersion
```

---

## WSL Compatibility

If you use Windows Subsystem for Linux:

### Separate installations

Keep separate skillshare installations for Windows and WSL:
- Windows: `%USERPROFILE%\.config\skillshare\`
- WSL: `~/.config/skillshare/`

### Share via git

Use the same git remote to sync between them:
```bash
# Windows
skillshare push -m "From Windows"

# WSL
skillshare pull
```

---

## Getting Help

Include in bug reports:
- Windows version: `winver`
- PowerShell version: `$PSVersionTable.PSVersion`
- skillshare version: `skillshare --version`
- Full error message

---

## Related

- [Common Errors](./common-errors) — General error solutions
- [Configuration](/docs/targets/configuration) — Config file reference
