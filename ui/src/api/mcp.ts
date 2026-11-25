import { http } from './http';

export const mcpTargets = [
  'claude',
  'codex',
  'cursor',
  'vscode',
  'opencode',
  'kilocode',
  'grok',
  'antigravity',
  'amp',
  'claude-desktop',
  'cline',
  'copilot',
  'factory',
  'gemini',
  'goose',
  'hermes',
  'junie',
  'kiro',
  'lmstudio',
  'warp',
  'windsurf',
  'pi',
  'omp',
] as const;
export type MCPValue = string | { fromEnv: string };
/** A scope's defaults. A save replaces them, so a value left out is cleared. */
export interface MCPSettings {
  targets?: string[];
}
/** One root under the global config's mcp.projects. */
export interface MCPProject extends MCPSettings {
  /** Shared servers (`global: false` in mcp.servers) this root loads. */
  use?: string[];
  /** What this root changes in a shared server it uses, keyed by the server's name. */
  overrides?: Record<string, MCPOverride>;
  servers?: Record<string, MCPServer>;
}
/** A root's change to a shared server. targets and tools replace the server's; piOptions and options are merged into it key by key. */
export interface MCPOverride {
  targets?: string[];
  tools?: MCPToolPolicy;
  piOptions?: Record<string, unknown>;
  options?: Record<string, Record<string, unknown>>;
}
/** Which of a server's tools reach the model, written once and translated per Agent. Names may use `*`. */
export interface MCPToolPolicy {
  allow?: string[];
  deny?: string[];
}
export interface MCPServer {
  command?: string;
  args?: string[];
  url?: string;
  transport?: 'stdio' | 'streamable-http';
  targets?: string[];
  env?: Record<string, MCPValue>;
  headers?: Record<string, MCPValue>;
  bearerToken?: { fromEnv: string };
  /** Per-server fields written into Pi's built-in MCP as given. */
  piOptions?: Record<string, unknown>;
  /** Other Agents' native per-server fields, keyed by Agent, written as given. */
  options?: Record<string, Record<string, unknown>>;
  tools?: MCPToolPolicy;
  /** Project mode: the whole entry, turning off a server the Agent's global config defines. */
  disabled?: boolean;
  /** `false`: written to no global file, only to the mcp.projects roots that list it under `use`. */
  global?: boolean;
}
/** A server of the global config that loads only in the projects that use it. */
export const isShared = (server?: MCPServer) => server?.global === false;
/** Agents with a per-project switch: a field merged over the global entry, or Claude Code's own off list. */
export const mcpOffTargets: readonly string[] = ['claude', 'opencode', 'kilocode', 'pi', 'omp'];
export interface MCPMutation {
  /** A root under mcp.projects; with `remove` and no `name`, the project itself. */
  project?: string;
  settings?: MCPSettings;
  /** With `project` and `name`: turns that shared server on or off for the root. */
  using?: boolean;
  name?: string;
  server?: MCPServer;
  remove?: boolean;
  /** With `remove`: also forget which Agent entries it wrote. Files stay as they are and sync leaves them alone. */
  unmanage?: boolean;
  replace?: boolean;
  resolutions?: { target: string; name: string; action: 'replace' | 'adopt' }[];
}
export interface MCPPlan {
  revision: string;
  sourcePath: string;
  blocked: boolean;
  /** The source's notices, then one per Agent and set of tool policy parts it does not apply, worded in English. */
  notices?: string[];
  /** Syncing also saves the config without the settings 0.23.0 retired, even when no Agent file changes. */
  migrates?: boolean;
  /** `switch`: the entry only turns a global server off for one project, so adding it turns the server off there. */
  changes: {
    target: string;
    path: string;
    name: string;
    root?: string;
    switch?: boolean;
    action: string;
    message?: string;
    fields?: { added?: string[]; updated?: string[]; removed?: string[] };
  }[];
}
/** `migrated`: config files the sync also saved without the settings 0.23.0 retired, and each one's backup. */
export interface MCPResult {
  plan?: MCPPlan;
  applied: string[];
  backupIds: string[];
  migrated?: { path: string; backup?: string }[];
}
/** Servers in one Agent file that skillshare does not manage; `project` is a root under mcp.projects. */
export interface MCPUnmanaged {
  target: string;
  project?: string;
  path: string;
  names: string[];
}
/** `servers`: what the write the backup was taken before changed; `time`: when it was taken. */
export interface MCPBackup {
  id: string;
  target: string;
  path: string;
  time?: string;
  servers?: { name: string; change: 'added' | 'changed' | 'removed' }[];
}
export interface MCPCandidate {
  name: string;
  server: MCPServer;
  problems: string[];
  warnings: string[];
  from?: string;
}
/** `piExtension` only tells Pi's files apart: `builtin` is mcp.json, `pi-mcp-adapter` the legacy mcp-adapter.json. */
export interface MCPImportSource {
  target: string;
  path: string;
  piExtension?: string;
}

export const mcpApi = {
  list: () =>
    http.get<{
      source: {
        path: string;
        configPath: string;
        targets: string[] | null;
        servers: Record<string, MCPServer>;
        projects?: Record<string, MCPProject>;
        /** Targets that are another config folder of an Agent, by name */ accounts?: Record<
          string,
          { agent: string; configDir: string }
        >;
        /** Settings the config still has that no longer apply, worded by the server in English. */
        notices?: string[];
      };
      /** Project roots that also have their own .skillshare/config.yaml. */
      projectConfigs: string[];
      paths: Record<string, string>;
      detected: string[];
      /** This browser runs on the dashboard's machine, so it can open files and folders there. */
      local: boolean;
      /** Targets whose file in `paths` exists and this browser, on the dashboard's machine, can open. */
      openable: string[];
      /** For each mcp.projects root: its Agents' paths, and the targets whose file exists there, whoever views them. */
      projectPaths: Record<string, Record<string, string>>;
      projectExisting: Record<string, string[]>;
      plan: MCPPlan | null;
      previewError: string;
      /** Native import files by scope: the empty key is this config, others are project roots. */
      importSources?: Record<string, MCPImportSource[]>;
      backups: MCPBackup[];
      unmanaged: MCPUnmanaged[];
    }>('/mcp'),
  preview: (mutation: MCPMutation = {}) => http.post<MCPPlan>('/mcp/preview', { mutation }),
  configure: (mutation: MCPMutation, revision: string, sync: boolean) =>
    http.post<MCPResult>('/mcp', { mutation, revision, sync }),
  /** Apply only the changes of one mcp.projects root; the global scope and other projects stay pending. */
  syncProject: (root: string, revision: string) =>
    http.post<MCPResult>('/mcp', { mutation: {}, revision, sync: true, root }),
  /** Save to the source only. The server refuses a write it has not previewed; the revision also catches concurrent edits. */
  save: async (mutation: MCPMutation) =>
    http.post<MCPResult>('/mcp', {
      mutation,
      revision: (await http.post<MCPPlan>('/mcp/preview', { mutation })).revision,
      sync: false,
    }),
  /** Save several changes to the source together, under one preview's revision; none is saved if one is refused. */
  saveBatch: async (mutations: MCPMutation[]) =>
    http.post<MCPResult>('/mcp', {
      mutations,
      revision: (await http.post<MCPPlan>('/mcp/preview', { mutations })).revision,
      sync: false,
    }),
  /** One server as each of its targets' config files would hold it. Reads configuration only; no server is executed and no file is written. */
  /** `toolGaps`: the parts of the server's tool policy that Agent does not apply. */
  render: (mutation: MCPMutation) =>
    http.post<{
      rendered: {
        target: string;
        path: string;
        content?: string;
        error?: string;
        toolGaps?: string[];
      }[];
    }>('/mcp/render', { mutation }),
  /** `root` reads the `from` target's file in that mcp.projects root. */
  import: (body: {
    from?: string;
    content?: string;
    name?: string;
    root?: string;
    piExtension?: string;
  }) => http.post<{ candidates: MCPCandidate[] }>('/mcp/import', body),
  previewRestore: (backupId: string) =>
    http.post<MCPPlan>('/mcp/restore', { backupId, preview: true }),
  restore: (backupId: string, revision: string) =>
    http.post<MCPResult>('/mcp/restore', { backupId, revision }),
  /** Open one Agent's config file, as listed in `paths` or `projectPaths`, the way the skill page opens a skill; `folder` opens its directory instead. */
  open: (target: string, opts: { project?: string; folder?: boolean } = {}) =>
    http.post<{ editor: string; path: string; pid: number }>('/mcp/open', { target, ...opts }),
};
