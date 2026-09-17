import { apiFetch } from './client';

export const mcpTargets = ['claude', 'codex', 'cursor', 'vscode', 'opencode', 'grok'] as const;
export type MCPValue = string | { fromEnv: string };
export interface MCPServer {
  command?: string;
  args?: string[];
  url?: string;
  transport?: 'stdio' | 'streamable-http';
  targets?: string[];
  env?: Record<string, MCPValue>;
  headers?: Record<string, MCPValue>;
  bearerToken?: { fromEnv: string };
}
export interface MCPMutation {
  name?: string;
  server?: MCPServer;
  remove?: boolean;
  replace?: boolean;
  resolutions?: { target: string; name: string; action: 'replace' | 'adopt' }[];
}
export interface MCPPlan {
  revision: string;
  sourcePath: string;
  blocked: boolean;
  changes: { target: string; path: string; name: string; action: string; message?: string }[];
}
export interface MCPResult { plan?: MCPPlan; applied: string[]; backupIds: string[] }
export interface MCPCandidate { name: string; server: MCPServer; problems: string[]; warnings: string[]; from?: string }
const post = <T,>(path: string, body: unknown) => apiFetch<T>(path, { method: 'POST', body: JSON.stringify(body) });
export const mcpApi = {
  list: () => apiFetch<{
    source: { path: string; configPath: string; targets: string[] | null; servers: Record<string, MCPServer> };
    paths: Record<string, string>; detected: string[]; plan: MCPPlan | null; previewError: string;
    backups: { id: string; target: string; path: string }[];
  }>('/mcp'),
  preview: (mutation: MCPMutation = {}) => post<MCPPlan>('/mcp/preview', { mutation }),
  configure: (mutation: MCPMutation, revision: string, sync: boolean) => post<MCPResult>('/mcp', { mutation, revision, sync }),
  import: (body: { from?: string; content?: string; name?: string }) => post<{ candidates: MCPCandidate[] }>('/mcp/import', body),
  previewRestore: (backupId: string) => post<MCPPlan>('/mcp/restore', { backupId, preview: true }),
  restore: (backupId: string, revision: string) => post<MCPResult>('/mcp/restore', { backupId, revision }),
};
