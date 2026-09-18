import { apiFetch } from './client';

export const pluginTargets = {
  claude: { label: 'Claude Code', project: true },
  codex: { label: 'Codex', project: false },
  cursor: { label: 'Cursor', project: false },
  antigravity: { label: 'Antigravity', project: true },
  pi: { label: 'Pi', project: true },
  opencode: { label: 'OpenCode', project: true },
} as const;
export type PluginTarget = keyof typeof pluginTargets;
export type PluginAction = 'add' | 'import' | 'sync' | 'check' | 'update' | 'remove' | 'enable' | 'disable';
export interface PluginRequest { action: PluginAction; name?: string; source?: string; plugin?: string; targets?: PluginTarget[]; from?: PluginTarget }
export interface PluginBinding { id: string; source?: string; plugin?: string; digest?: string; version?: string; pending?: string; sync?: boolean; components?: string[] }
export interface NativePlugin { id: string; version?: string; enabled: boolean; scope?: string; filtered?: boolean }
export interface PluginInventory {
  packages: Record<string, { bindings: Partial<Record<PluginTarget, PluginBinding>> }>;
  hosts: { target: PluginTarget; version: string; error?: string; note?: string; installed: NativePlugin[] }[];
}
export interface PluginCandidate { name: string; description: string; version: string; targets: PluginTarget[]; components: string[]; problem?: string }
export interface PluginDiscovery { source: string; digest: string; candidates: PluginCandidate[] }
export interface PluginPlan { revision: string; blocked: boolean; changes: { name: string; target: PluginTarget; id: string; action: string; message?: string; components?: string[] }[] }
export interface PluginOutcome { name: string; target: PluginTarget; status: string; message?: string }
export interface PluginResult { result: { results: PluginOutcome[] } | null; failure: string }
const post = <T,>(path: string, body: unknown) => apiFetch<T>(path, { method: 'POST', body: JSON.stringify(body) });
export const pluginsApi = {
  list: () => apiFetch<PluginInventory>('/plugins'),
  discover: (source: string) => post<PluginDiscovery>('/plugins/discover', { source }),
  preview: (request: PluginRequest) => post<PluginPlan>('/plugins/preview', { request }),
  apply: (request: PluginRequest, revision: string) => post<PluginResult>('/plugins/apply', { request, revision }),
};
