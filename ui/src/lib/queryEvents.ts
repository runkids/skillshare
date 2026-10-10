import type { InvalidateQueryFilters, QueryClient, QueryKey } from '@tanstack/react-query';

import { queryKeys as k } from './queryKeys';

const keys = (...list: QueryKey[]): InvalidateQueryFilters[] => list.map((queryKey) => ({ queryKey }));

/** Everything that shows a target's config or its effect. */
const targets = [k.targets.all, k.projects, k.config, k.overview, k.diff(), k.syncMatrix()];
const sourceLinks = [k.skills.all, k.overview, k.config, k.diff(), k.syncMatrix()];
const ignoreFile = [k.diff(), k.overview, k.skills.all, k.doctor];

/**
 * What each event makes stale. A mutation names what happened; the queries to refetch are listed here, once.
 * `skills.all` is a large payload: add it to an event only when the list really changes.
 * `syncMatrixAll` and `diffAll` are prefixes that reach every target; `syncMatrix()` and `diff()` are the all-targets query only.
 */
const events = {
  // Skills and agents
  skillsChanged: () => keys(k.skills.all, k.overview),
  skillEdited: (name: string) => keys(k.skills.detail(name)),
  installDialogClosed: () => keys(k.skills.all),
  reposChanged: () => keys(k.overview, k.skills.all, k.trash),
  skillsMoved: () => keys(k.skills.all, k.overview, k.syncMatrixAll, k.diff()),
  skillsUninstalled: () => keys(k.skills.all, k.overview, k.trash, k.syncMatrixAll, k.diff()),
  trashChanged: () => keys(k.trash, k.skills.all, k.syncMatrixAll),
  skillSyncChanged: () => keys(k.skills.all, k.syncMatrixAll),
  skillsToggled: () => keys(k.skills.all, k.syncMatrixAll, k.overview),
  skillTargetFilterChanged: () => keys(k.targets.all, k.syncMatrixAll, k.diffAll),
  sourceLinked: () => keys(...sourceLinks),
  sourceUnlinked: () => keys(...sourceLinks, k.trash),
  collected: (target?: string) => keys(k.skills.all, k.overview, k.targets.all, k.diff(), k.syncMatrix(), k.collectScan(target)),
  configInstalled: () => keys(k.missingConfigEntries, k.skills.all, k.overview, k.syncMatrixAll, k.diff()),

  // Targets, projects and sync
  targetsChanged: () => keys(...targets, k.instructions.all),
  synced: () => keys(k.targets.all, k.overview, k.diff()),
  syncRan: () => keys(...targets, k.extrasDiff(), k.extras, k.mcp, k.hooks, k.logAll),
  projectSynced: () => keys(...targets, k.mcp, k.hooks, k.logAll),
  projectChanged: () => keys(...targets, k.mcp, k.hooks),
  projectAdded: () => keys(...targets, k.mcp),
  targetFilesChanged: (target: string) => keys(k.targetFiles.list(target)),
  // Only the list: the removed file's content query is still mounted and would fetch again.
  targetFileRemoved: (target: string): InvalidateQueryFilters[] => [{ queryKey: k.targetFiles.list(target), exact: true }],

  // Config
  configSaved: () => keys(k.config, k.mcp, k.overview, k.targets.all, k.skills.all, k.extras, k.extrasDiff(), k.diff(), k.syncMatrix(), k.doctor, k.instructions.all),
  settingsSaved: () => keys(k.config),
  skillignoreSaved: () => keys(k.skillignore, ...ignoreFile),
  agentignoreSaved: () => keys(k.agentignore, ...ignoreFile),
  extensionsChanged: () => keys(k.extensions, k.extrasExtensions),
  auditRulesChanged: () => keys(k.audit.rules, k.audit.compiled),

  // Extras, instructions and memory
  extrasChanged: () => keys(k.extras, k.extrasDiff(), k.config, k.overview),
  instructionsChanged: () => keys(k.instructions.all, k.extras, k.extrasDiff(), k.config),
  managedFilesChanged: () => keys(k.fileBackups.all, k.instructions.all, k.memory.all),
  memoryNotesChanged: () => keys(k.memory.all, k.extras, k.fileBackups.all),
  memoryRefreshed: () => keys(k.memory.all),

  // MCP, hooks and plugins
  mcpChanged: () => keys(k.mcp, k.config),
  mcpSynced: () => keys(k.mcp, k.logAll),
  mcpRestored: () => keys(k.mcp),
  hooksChanged: () => keys(k.hooks, k.config),
  hooksSynced: () => keys(k.hooks, k.logAll),
  hooksRestored: () => keys(k.hooks),
  pluginsChanged: () => keys(k.plugins, k.config, k.piExtensionsAll, k.ompExtensionsAll),
  piExtensionsStale: (target: string) => keys(k.piExtensions(target)),
  piExtensionsChanged: (target: string) => keys(k.piExtensions(target), k.plugins),
  ompExtensionsStale: (target: string) => keys(k.ompExtensions(target)),
  ompExtensionsChanged: (target: string) => keys(k.ompExtensions(target), k.plugins),

  // Backups, logs, git and the app itself
  backupsChanged: () => keys(k.backups),
  backupLimitsSaved: () => keys(k.backups, k.config),
  backupRestored: () => keys(k.backups, k.targets.all),
  logsCleared: () => keys(k.logAll, k.logStatsAll),
  gitFetched: () => keys(k.gitStatus),
  gitChanged: () => keys(k.gitStatus, k.gitBranches, k.skills.all, k.overview, k.config, k.targets.all, k.diff()),
  // Root scope can change any of the source resources shown elsewhere.
  sourceDiscarded: (): InvalidateQueryFilters[] => [{}],
  appUpgraded: () => keys(k.versionCheck),

  // Hub drafts: the list only, since each draft's query sits under it and is written directly.
  hubDraftsChanged: (): InvalidateQueryFilters[] => [{ queryKey: k.hub.drafts, exact: true }],
  hubDraftReverted: (id: string) => keys(k.hub.draft(id)),
};

export type QueryEvent = keyof typeof events;

/** Mark what `event` made stale and refetch the queries on screen. Resolves when those refetches finish. */
export function invalidate<E extends QueryEvent>(queryClient: QueryClient, event: E, ...args: Parameters<(typeof events)[E]>) {
  const filters = (events[event] as (...a: unknown[]) => InvalidateQueryFilters[])(...args);
  return Promise.all(filters.map((f) => queryClient.invalidateQueries(f)));
}
