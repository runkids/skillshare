# OMP 18.6.1: shared plugin cache prevents a complete safe lifecycle

## Verified upstream contract

The inspected source is `can1357/oh-my-pi` tag `v18.6.1`, commit
`2a2c6dcbbb558c0f8145f67f28b3370984f2bf60`.

- [`marketplace/manager.ts`](https://github.com/can1357/oh-my-pi/blob/v18.6.1/packages/coding-agent/src/extensibility/plugins/marketplace/manager.ts)
  lists the user registry and only the current project's registry. Its uninstall
  code (around lines 616–636) deletes an install directory when neither of those
  two registries references it. Other projects are not considered.
- [`marketplace/cache.ts`](https://github.com/can1357/oh-my-pi/blob/v18.6.1/packages/coding-agent/src/extensibility/plugins/marketplace/cache.ts)
  identifies cached installations by marketplace, plugin name and version, not
  by project. Installing/upgrading the same identity can replace shared contents.

## Reproduced behavior

Native OMP 18.6.1 with Bun 1.3.14 and the Linux ARM64 native addon was exercised
inside the devcontainer, using the disposable `omp-plugins-409` ssenv, never a
real user's configuration or a running Agent session.

1. Install one local marketplace plugin in user scope and project B scope.
2. From the isolated HOME, native inventory shows only the user installation.
3. Native `omp plugin uninstall <id> --scope user` removes the shared cache,
   leaving project B's `node_modules` link dangling.
4. A same-version native upgrade replaces cache contents; an upgrade to another
   version can remove the old directory still referenced by project B.

Retaining the marketplace registration does **not** fix this: native uninstall
can delete the installed cache independently of marketplace removal. Checking
only the opposite scope visible from the current directory is also insufficient.

## Skillshare boundary

The adapter therefore does not provide a complete OMP plugin lifecycle:

- Reviewed local/Git marketplace installation is allowed only for OMP 18.6.1,
  a verified cache root, a previously absent cache identity and an absent runtime
  destination in the selected scope. The reviewed source establishes the runtime
  package name; preview binds the destination and apply rechecks it before native
  installation, which otherwise recursively removes that destination.
- An unknown root/version/source or unreadable/linked/non-directory destination
  blocks installation. Reinstall picks a fresh marketplace/cache identity when
  retained cache exists; old content and shared registrations are never replaced.
  Cached install paths from old registrations are not trusted as the next destination.
- Native update/uninstall commands are not invoked. Update remains blocked.
- Scoped removal uses `omp_remove.go`: delete only the verified scope's registry
  entry, runtime selection and link, retaining all cache, marketplaces and plugin
  settings. The plugin is genuinely uninstalled in that scope, not disabled.
  Ambiguous JSON/ownership, version mismatch, foreign links, real module folders
  and npm collisions refuse writes. A missing cache manifest requires a unique
  runtime-lock key whose link targets the exact cache; otherwise active removal
  fails closed instead of guessing the marketplace name. Windows native writes
  require private ACLs.
- Interrupted removal is retryable from retained cache and binding identity.
  Revisions include native registry, runtime lock, dependency/package metadata
  and link target. Skillshare removals share a lock, but native OMP plugin commands
  do not; concurrent native mutations are unsupported, not an atomic native CAS.
- Profile/root overrides block plugin management. Cache relocation invalidates
  reviewed revisions. npm/Git/link packages remain inventory-only.
- Standalone extension selection is separate: verified rows can still be changed
  with native locks and revision checks. Runtime lock-only plugin registrations
  are valid even without a root `package.json` and do not falsely block selection.

The UI uses the same Preview/Apply workflow as other Agents, without an OMP-only
execution confirmation. Resource loading notes are conditional, not a claim that
Markdown-only plugins execute code. Unsafe plans still disable Apply.
No native trust prompt is bypassed and no extension code runs during management.

## Evidence

- `TestOMPNeverRunsCacheDestructiveNativeCommands`: invisible-project regression;
  destructive requests emit no native mutation command.
- `TestOMPInstallGuardFailsClosed`: unknown version/root/source and unreadable
  destinations refuse installation.
- `TestOMPPreviewBindsNativeCacheRoot`: an XDG-root change invalidates approval.
- `TestOMPExtensionsLockOnlyPlugins`: real-directory and linked marketplace
  runtime entries work with or without the optional root package manifest.
- Native isolated tests reproduced the defect and verified blocked Skillshare
  operations preserve the affected registry, cache and project link.
- Review follow-up in `omp-review-fix-verified-409` verified that an undeclared runtime
  directory and its foreign file survive blocked preview/apply, an empty destination
  accepts the inert plugin, unverifiable missing-manifest ownership blocks removal,
  and missing cache metadata still permits complete scoped removal using the proven
  runtime link name. No Agent or extension code ran.
- `TestOMPRemoveRetainsCacheUsedByInvisibleProject`: user and project removal
  preserve foreign registry/link/cache, unrelated raw values and plugin settings.
- `TestOMPRemovalRefusesForeignLinkAndStaleNativeFile`,
  `TestOMPRemovalRefusesUnsafeMetadata`, and partial-removal recovery tests cover
  fail-closed behavior without destructive native commands.
- `TestOMPReinstallAllocatesFreshIdentityWithoutTouchingRetainedCache` verifies
  stable review and non-destructive reinstallation.
- Native `omp_plugins_removal_runbook.md` passed both real 18.6.1 scenarios:
  user removal/reinstall preserves an invisible project's cache; project
  deselection-plus-sync preserves the user installation.

The scoped removal adapter resolves removal, not native cache collection or
safe upgrades. Completing safe update still requires an upstream contract or a
separately reviewed non-destructive upgrade adapter. macOS and Windows runtime
behavior remains unverified.
