# Adopting Externally Installed Skills

`adopt` is an ownership transfer. Use it when an external installer placed a real skill directory in the universal target and you want Skillshare—not that installer—to become the canonical source.

Preview first. Adoption copies the selected skill into Skillshare's source, removes only the external links identified in that preview, moves the external original to recoverable trash, and distributes the new canonical copy using each target's normal sync mode. Project-mode links retain the same portable relative-link behavior as `sync`, and copy-mode targets retain their configured file ignores and unrelated local content. If cleanup or target sync fails, the result is reported as partial rather than claiming that every target was updated.

The external installer's lockfile remains read-only. After a successful adoption, release the lingering entry through the owning tool so a future tool update does not recreate the old copy.

Conflicts are conservative: an unmanaged same-name source skill is skipped unless `--force` is explicit. A source entry with install metadata must be uninstalled first so a later update or project replay cannot reclaim the adopted copy. Machine-readable `--json` suppresses prompts and diagnostics, but does not authorize either kind of overwrite.
