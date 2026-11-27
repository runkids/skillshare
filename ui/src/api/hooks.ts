import { http } from './http';

/** The Agents hooks can be managed for, in display order. */
export const hookAgents = [
  'claude',
  'codex',
  'gemini',
  'copilot',
  'cursor',
  'droid',
  'qwen',
  'antigravity',
  'pi',
  'omp',
  'amp',
  'opencode',
  'git',
] as const;
export type HookAgent = (typeof hookAgents)[number];
/** Agents whose hooks are one standalone native TypeScript/JavaScript file, not a command event map. */
export const hookCodeAgents: readonly string[] = ['pi', 'omp', 'amp', 'opencode'];

/** One Agent's part of a hook. Command Agents fill `events`, code Agents fill `code`. */
export interface HookBinding {
  commands?: Record<string, { events: string[]; command: string; parallel?: boolean }>;
  /** The Agent's own event map, passed through as written. */
  events?: Record<string, unknown>;
  code?: string;
  /** OpenCode v2 directory: relative filenames mapped to UTF-8 contents. */
  package?: Record<string, string>;
  /** Path, relative to the config's folder, that `code` was read from. */
  codeFile?: string;
  /** Relative script filename → UTF-8 contents. */
  files?: Record<string, string>;
  /** Script filename → path, relative to the config's folder, its contents were read from. */
  filesFrom?: Record<string, string>;
}
export interface HookEntry {
  description?: string;
  /** Left out means enabled. */
  enabled?: boolean;
  bindings: Record<string, HookBinding>;
}
export interface HookMutation {
  /** A root under hooks.projects; with `remove` and no `name`, the project itself. */
  project?: string;
  name?: string;
  entry?: HookEntry;
  remove?: boolean;
  /** With remove: stop managing the hook and leave its target entries as they are. */
  unmanage?: boolean;
  replace?: boolean;
  /** Save an imported entry as the owner of the native registrations it was read from, so sync adopts them instead of reporting a conflict. */
  adopt?: boolean;
}
/** `root`: the hooks.projects root a native file belongs to; left out for the global files. */
/** Events a change adds to, updates in, or removes from a shared hooks file. */
export interface HookEventChanges {
  added?: string[];
  updated?: string[];
  removed?: string[];
}
export interface HookChange {
  target: string;
  path: string;
  name: string;
  root?: string;
  action: string;
  message?: string;
  inactiveReason?: string;
  events?: HookEventChanges;
}
/** `fingerprint` covers the hook content the plan proposes, which `changes` (no contents) cannot show: equal shape, different command, different fingerprint. */
/** `warnings`: advisory only, such as event names an Agent does not document; they never block. */
export interface HookPlan {
  revision: string;
  fingerprint: string;
  sourcePath: string;
  blocked: boolean;
  changes: HookChange[];
  warnings?: string[];
}
export interface HookCatalogEvent {
  name: string;
  description: string;
  matcher: boolean;
}
/** A command Agent's documented events; code Agents are not listed. */
export interface HookAgentCatalog {
  events: HookCatalogEvent[];
  timeoutUnit: 'seconds' | 'milliseconds' | '';
}
/** A native file the preview would change: full text now (`""` when missing) and exactly what sync writes (`""` when removed). */
export interface HookFileDiff {
  target: string;
  path: string;
  root?: string;
  before: string;
  after: string;
}
/** Only POST /hooks/preview carries `files`. */
export type HookPreview = HookPlan & { files?: HookFileDiff[] };
export interface HookResult {
  plan?: HookPlan;
  applied: string[];
  backupIds: string[];
}
export interface HookTargetDef {
  name: string;
  agent?: string;
  kind: string;
  note?: string;
}
/** Hooks in an Agent's native configuration that Skillshare does not manage. */
export interface HookUnmanaged {
  target: string;
  path: string;
  names: string[];
  project?: string;
  /** False for sources Skillshare only lists, such as Codex inline TOML or Git config: import cannot take them over. */
  importable: boolean;
}
export interface HookBackup {
  id: string;
  target: string;
  path: string;
  root?: string;
  time?: string;
}
/** `editable`: the hook can be changed or removed in its target file without importing it. `source`: the Antigravity block or Copilot file it was read from. */
export interface HookCandidate {
  name: string;
  entry: HookEntry;
  problems: string[];
  warnings: string[];
  editable?: boolean;
  source?: string;
}
/** A change to hooks Skillshare does not manage: `before` as the import listed them, `after` their replacement (left out removes them). `source` is the candidate's. */
export interface HookNativeEdit {
  target: string;
  root?: string;
  source?: string;
  before: Record<string, unknown>;
  after?: Record<string, unknown>;
}
export interface HookImportRequest {
  from: string;
  content?: string;
  name?: string;
  /** Reads the Agent's file under this hooks.projects root. */ root?: string;
}
/** One root under hooks.projects: hooks that live in that folder's own native files. */
export interface HookProject {
  entries?: Record<string, HookEntry>;
}
export interface HookRendered {
  target: string;
  path?: string;
  content?: string;
  error?: string;
}
export interface HookInventory {
  git?: HookGitInfo;
  projectGit?: Record<string, HookGitInfo>;
  source: {
    path: string;
    configPath: string;
    entries: Record<string, HookEntry>;
    projects?: Record<string, HookProject>;
  };
  /** Project roots that also have their own .skillshare/config.yaml. */
  projectConfigs?: string[];
  targets: HookTargetDef[];
  paths: Record<string, string>;
  /** Project root → Agent → native path, for each root under hooks.projects. */
  projectPaths?: Record<string, Record<string, string>>;
  plan: HookPlan | null;
  previewError: string;
  backups: HookBackup[];
  unmanaged: HookUnmanaged[];
}

export interface HookGitInfo {
  version: string;
  configHooks: boolean;
  parallel: boolean;
  hooksFile: string;
  include: {
    target: string;
    resolved: string;
    present: boolean;
    owned: boolean;
    writable: boolean;
    lines: string;
    conditional?: boolean;
    active?: boolean;
  };
  hooksPath: { value: string; origin: string };
  reason?: string;
}

/** One Codex hook Skillshare owns. `needed`: not trusted yet or its command changed; `unchanged`: already trusted; `trusted`: recorded by this call. */
export interface CodexTrustHook {
  home?: string;
  target: string;
  entry: string;
  key: string;
  command?: string;
  action: 'needed' | 'unchanged' | 'trusted';
  disabled?: boolean;
}
export interface CodexTrustPlan {
  project?: string;
  hooks: CodexTrustHook[];
  revision: string;
  /** Homes Codex could not list; their hooks are left out. */ failed?: {
    target: string;
    error: string;
  }[];
}

export const hooksApi = {
  list: () => http.get<HookInventory>('/hooks'),
  catalog: () => http.get<Record<string, HookAgentCatalog>>('/hooks/catalog'),
  /** An empty mutation previews synchronizing the current source. */
  preview: (mutation: HookMutation = {}) => http.post<HookPreview>('/hooks/preview', { mutation }),
  configure: (mutation: HookMutation, revision: string, sync: boolean) =>
    http.post<HookResult>('/hooks', { mutation, revision, sync }),
  /** Save to the source only. The server refuses a write it has not previewed; the revision also catches concurrent edits. */
  save: async (mutation: HookMutation) =>
    http.post<HookResult>('/hooks', {
      mutation,
      revision: (await http.post<HookPlan>('/hooks/preview', { mutation })).revision,
      sync: false,
    }),
  /** Each Agent's native contribution of one hook, as sync would write it: its own file(s) and scripts only, never the shared settings. Writes nothing. */
  render: (mutation: HookMutation) =>
    http.post<{ rendered: HookRendered[] }>('/hooks/render', { mutation }),
  /** Apply only the changes of one hooks.projects root; the global scope and other projects stay pending. */
  syncProject: (root: string, revision: string) =>
    http.post<HookResult>('/hooks', { mutation: {}, revision, sync: true, root }),
  /** Reads native configuration or code, or the pasted `content`; nothing is executed. */
  import: async (body: HookImportRequest) => {
    const res = await http.post<{ candidates: HookCandidate[] } | HookCandidate[]>(
      '/hooks/import',
      body,
    );
    return Array.isArray(res) ? res : (res.candidates ?? []);
  },
  /** What editing or removing unmanaged hooks would write to their target files. Writes nothing. */
  previewNative: (edits: HookNativeEdit[]) =>
    http.post<HookPreview>('/hooks/native/preview', { edits }),
  /** Writes previewed edits, with a backup per file. The hooks stay unmanaged. */
  applyNative: (edits: HookNativeEdit[], revision: string) =>
    http.post<HookResult>('/hooks/native', { edits, revision }),
  /** Starts a Codex process to list the hooks, so call it only when the user asks. Global scope only. */
  previewCodexTrust: () => http.post<CodexTrustPlan>('/hooks/codex-trust/preview', {}),
  /** Records trust for the previewed hooks. Refused (409) when they changed since the preview. */
  trustCodex: (revision: string) => http.post<CodexTrustPlan>('/hooks/codex-trust', { revision }),
  previewRestore: (backupId: string) =>
    http.post<HookPlan>('/hooks/restore', { backupId, preview: true }),
  restore: (backupId: string, revision: string) =>
    http.post<HookResult>('/hooks/restore', { backupId, revision }),
};
