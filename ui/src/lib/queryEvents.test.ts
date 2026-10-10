import { QueryClient } from '@tanstack/react-query';
import { expect, it } from 'vitest';

import { invalidate, type QueryEvent } from './queryEvents';
import { queryKeys as k } from './queryKeys';

// One cached query per label. Several sit under another key, or beside it for a second target,
// to show which events reach them by prefix.
const cached = {
  overview: k.overview,
  version: k.versionCheck,
  skills: k.skills.all,
  skill: k.skills.detailOfKind('a', 'skill'),
  targets: k.targets.all,
  targetsAvailable: k.targets.available,
  projects: k.projects,
  diff: k.diff(),
  diffClaude: k.diff('claude'),
  collectScan: k.collectScanScope(undefined, 'global'),
  collectScanClaude: k.collectScanScope('claude', 'global'),
  backups: k.backups,
  fileBackups: k.fileBackups.versions('/a'),
  trash: k.trash,
  gitStatus: k.gitStatus,
  gitBranches: k.gitBranches,
  auditRules: k.audit.rules,
  auditCompiled: k.audit.compiled,
  log: k.log('ops', 10),
  logStats: k.logStats('ops'),
  config: k.config,
  missingConfigEntries: k.missingConfigEntries,
  syncMatrix: k.syncMatrix(),
  syncMatrixClaude: k.syncMatrix('claude'),
  extras: k.extras,
  extrasExtensions: k.extrasExtensions,
  extensions: k.extensions,
  extrasDiff: k.extrasDiff(),
  memory: k.memory.list(''),
  instructions: k.instructions.shared,
  targetFiles: k.targetFiles.list('claude'),
  targetFile: k.targetFiles.content('claude', 'a.md'),
  targetFilesCodex: k.targetFiles.list('codex'),
  mcp: k.mcp,
  mcpRender: k.mcpRender('{}'),
  pi: k.piExtensions('pi'),
  pi2: k.piExtensions('pi2'),
  omp: k.ompExtensions('omp'),
  hooks: k.hooks,
  hooksCatalog: k.hooksCatalog,
  plugins: k.plugins,
  pluginPackages: k.pluginPackages,
  hubDrafts: k.hub.drafts,
  hubDraft1: k.hub.draft('1'),
  hubDraft2: k.hub.draft('2'),
  doctor: k.doctor,
  skillignore: k.skillignore,
  agentignore: k.agentignore,
};
type Label = keyof typeof cached;

const S: Label[] = ['skills', 'skill'];
const T: Label[] = ['targets', 'targetsAvailable'];
const X: Label[] = ['extras', 'extrasExtensions'];
const M: Label[] = ['mcp', 'mcpRender'];
const H: Label[] = ['hooks', 'hooksCatalog'];
const P: Label[] = ['plugins', 'pluginPackages'];
const SM: Label[] = ['syncMatrix', 'syncMatrixClaude'];
const TARGETS: Label[] = [...T, 'projects', 'config', 'overview', 'diff', 'syncMatrix'];

const stale: Record<QueryEvent, Label[] | { args: string[]; stale: Label[] }> = {
  skillsChanged: [...S, 'overview'],
  skillEdited: { args: ['a'], stale: ['skill'] },
  installDialogClosed: S,
  reposChanged: [...S, 'overview', 'trash'],
  skillsMoved: [...S, 'overview', ...SM, 'diff'],
  skillsUninstalled: [...S, 'overview', 'trash', ...SM, 'diff'],
  trashChanged: ['trash', ...S, ...SM],
  skillSyncChanged: [...S, ...SM],
  skillsToggled: [...S, ...SM, 'overview'],
  skillTargetFilterChanged: [...T, ...SM, 'diff', 'diffClaude'],
  sourceLinked: [...S, 'overview', 'config', 'diff', 'syncMatrix'],
  sourceUnlinked: [...S, 'overview', 'config', 'diff', 'syncMatrix', 'trash'],
  collected: { args: ['claude'], stale: [...S, 'overview', ...T, 'diff', 'syncMatrix', 'collectScanClaude'] },
  configInstalled: ['missingConfigEntries', ...S, 'overview', ...SM, 'diff'],

  targetsChanged: [...TARGETS, 'instructions'],
  synced: [...T, 'overview', 'diff'],
  syncRan: [...TARGETS, 'extrasDiff', ...X, ...M, ...H, 'log'],
  projectSynced: [...TARGETS, ...M, ...H, 'log'],
  projectChanged: [...TARGETS, ...M, ...H],
  projectAdded: [...TARGETS, ...M],
  targetFilesChanged: { args: ['claude'], stale: ['targetFiles', 'targetFile'] },
  targetFileRemoved: { args: ['claude'], stale: ['targetFiles'] },

  configSaved: ['config', ...M, 'overview', ...T, ...S, ...X, 'extrasDiff', 'diff', 'syncMatrix', 'doctor', 'instructions'],
  settingsSaved: ['config'],
  skillignoreSaved: ['skillignore', 'diff', 'overview', ...S, 'doctor'],
  agentignoreSaved: ['agentignore', 'diff', 'overview', ...S, 'doctor'],
  extensionsChanged: ['extensions', 'extrasExtensions'],
  auditRulesChanged: ['auditRules', 'auditCompiled'],

  extrasChanged: [...X, 'extrasDiff', 'config', 'overview'],
  instructionsChanged: ['instructions', ...X, 'extrasDiff', 'config'],
  managedFilesChanged: ['fileBackups', 'instructions', 'memory'],
  memoryNotesChanged: ['memory', ...X, 'fileBackups'],
  memoryRefreshed: ['memory'],

  mcpChanged: [...M, 'config'],
  mcpSynced: [...M, 'log'],
  mcpRestored: M,
  hooksChanged: [...H, 'config'],
  hooksSynced: [...H, 'log'],
  hooksRestored: H,
  pluginsChanged: [...P, 'config', 'pi', 'pi2', 'omp'],
  piExtensionsStale: { args: ['pi'], stale: ['pi'] },
  piExtensionsChanged: { args: ['pi'], stale: ['pi', ...P] },
  ompExtensionsStale: { args: ['omp'], stale: ['omp'] },
  ompExtensionsChanged: { args: ['omp'], stale: ['omp', ...P] },

  backupsChanged: ['backups'],
  backupLimitsSaved: ['backups', 'config'],
  backupRestored: ['backups', ...T],
  logsCleared: ['log', 'logStats'],
  gitFetched: ['gitStatus'],
  gitChanged: ['gitStatus', 'gitBranches', ...S, 'overview', 'config', ...T, 'diff'],
  sourceDiscarded: Object.keys(cached) as Label[],
  appUpgraded: ['version'],

  hubDraftsChanged: ['hubDrafts'],
  hubDraftReverted: { args: ['1'], stale: ['hubDraft1'] },
};

it.each(Object.entries(stale))('%s makes exactly its queries stale', async (event, want) => {
  const client = new QueryClient();
  for (const key of Object.values(cached)) client.setQueryData(key, 1);
  const { args, stale: labels } = Array.isArray(want) ? { args: [], stale: want } : want;

  await (invalidate as (c: QueryClient, e: string, ...a: string[]) => Promise<unknown>)(client, event, ...args);

  const got = (Object.keys(cached) as Label[]).filter((label) => client.getQueryState(cached[label])?.isInvalidated);
  expect(got.sort()).toEqual([...labels].sort());
});
