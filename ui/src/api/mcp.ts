import { apiFetch } from './client';

export const mcpTargets = ['claude', 'codex', 'cursor', 'vscode', 'opencode', 'kilocode', 'grok', 'antigravity', 'amp', 'claude-desktop', 'cline', 'copilot', 'factory', 'gemini', 'goose', 'junie', 'kiro', 'lmstudio', 'warp', 'windsurf', 'pi', 'omp'] as const;
export type MCPValue = string | { fromEnv: string };
/** A scope's defaults. A save replaces them, so a value left out is cleared. */
export interface MCPSettings { targets?: string[] }
/** One root under the global config's mcp.projects. */
export interface MCPProject extends MCPSettings { servers?: Record<string, MCPServer> }
/** Which of a server's tools reach the model, written once and translated per Agent. Names may use `*`. */
export interface MCPToolPolicy { allow?: string[]; deny?: string[] }
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
  tools?: MCPToolPolicy;
  /** Project mode: the whole entry, turning off a server the Agent's global config defines. */
  disabled?: boolean;
}
/** Agents with a per-project switch: a field merged over the global entry, or Claude Code's own off list. */
export const mcpOffTargets: readonly string[] = ['claude', 'opencode', 'kilocode', 'pi', 'omp'];
export interface MCPMutation {
  /** A root under mcp.projects; with `remove` and no `name`, the project itself. */
  project?: string;
  settings?: MCPSettings;
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
  changes: { target: string; path: string; name: string; root?: string; switch?: boolean; action: string; message?: string; fields?: { added?: string[]; updated?: string[]; removed?: string[] } }[];
}
/** `migrated`: config files the sync also saved without the settings 0.23.0 retired, and each one's backup. */
export interface MCPResult { plan?: MCPPlan; applied: string[]; backupIds: string[]; migrated?: { path: string; backup?: string }[] }
/** Servers in one Agent file that skillshare does not manage; `project` is a root under mcp.projects. */
export interface MCPUnmanaged { target: string; project?: string; path: string; names: string[] }
/** `servers`: what the write the backup was taken before changed; `time`: when it was taken. */
export interface MCPBackup { id: string; target: string; path: string; time?: string; servers?: { name: string; change: 'added' | 'changed' | 'removed' }[] }
export interface MCPCandidate { name: string; server: MCPServer; problems: string[]; warnings: string[]; from?: string }
/** `piExtension` only tells Pi's files apart: `builtin` is mcp.json, `pi-mcp-adapter` the legacy mcp-adapter.json. */
export interface MCPImportSource { target: string; path: string; piExtension?: string }
const post = <T,>(path: string, body: unknown) => apiFetch<T>(path, { method: 'POST', body: JSON.stringify(body) });
export const mcpApi = {
  list: () => apiFetch<{
    source: { path: string; configPath: string; targets: string[] | null; servers: Record<string, MCPServer>; projects?: Record<string, MCPProject>; /** Targets that are another config folder of an Agent, by name */ accounts?: Record<string, { agent: string; configDir: string }>;
      /** Settings the config still has that no longer apply, worded by the server in English. */
      notices?: string[] };
    /** Project roots that also have their own .skillshare/config.yaml. */
    projectConfigs: string[];
    paths: Record<string, string>; detected: string[]; plan: MCPPlan | null; previewError: string;
    /** Native import files by scope: the empty key is this config, others are project roots. */
    importSources?: Record<string, MCPImportSource[]>;
    backups: MCPBackup[];
    unmanaged: MCPUnmanaged[];
  }>('/mcp'),
  preview: (mutation: MCPMutation = {}) => post<MCPPlan>('/mcp/preview', { mutation }),
  configure: (mutation: MCPMutation, revision: string, sync: boolean) => post<MCPResult>('/mcp', { mutation, revision, sync }),
  /** Apply only the changes of one mcp.projects root; the global scope and other projects stay pending. */
  syncProject: (root: string, revision: string) => post<MCPResult>('/mcp', { mutation: {}, revision, sync: true, root }),
  /** Save to the source only. The server refuses a write it has not previewed; the revision also catches concurrent edits. */
  save: async (mutation: MCPMutation) =>
    post<MCPResult>('/mcp', { mutation, revision: (await post<MCPPlan>('/mcp/preview', { mutation })).revision, sync: false }),
  /** One server as each of its targets' config files would hold it. Reads configuration only; no server is executed and no file is written. */
  /** `toolGaps`: the parts of the server's tool policy that Agent does not apply. */
  render: (mutation: MCPMutation) => post<{ rendered: { target: string; path: string; content?: string; error?: string; toolGaps?: string[] }[] }>('/mcp/render', { mutation }),
  /** `root` reads the `from` target's file in that mcp.projects root. */
  import: (body: { from?: string; content?: string; name?: string; root?: string; piExtension?: string }) => post<{ candidates: MCPCandidate[] }>('/mcp/import', body),
  previewRestore: (backupId: string) => post<MCPPlan>('/mcp/restore', { backupId, preview: true }),
  restore: (backupId: string, revision: string) => post<MCPResult>('/mcp/restore', { backupId, revision }),
};
