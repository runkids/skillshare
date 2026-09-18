import { apiFetch } from './client';

export type PluginTarget = string;
export interface PluginTargetDefinition { target: string; label: string; project: boolean; operations: string[]; reason?: string; reasonKey?: string }
export const targetMap = (definitions: PluginTargetDefinition[] = []) => Object.fromEntries(definitions.map((d) => [d.target, d]));
export type PluginAction = 'add' | 'import' | 'sync' | 'check' | 'update' | 'remove' | 'enable' | 'disable';
export interface PluginRequest { action: PluginAction; name?: string; source?: string; sourceRef?: string; entry?: string; plugin?: string; targets?: PluginTarget[]; from?: PluginTarget }
export interface PluginBinding { id: string; source?: string; sourceRef?: string; commit?: string; entry?: string; plugin?: string; digest?: string; version?: string; pending?: string; sync?: boolean; components?: string[] }
export interface NativePlugin { id: string; version?: string; enabled: boolean; enabledKnown?: boolean; scope?: string; filtered?: boolean }
export interface PluginInventory {
  targetDefinitions?: PluginTargetDefinition[];
  packages: Record<string, { bindings: Partial<Record<PluginTarget, PluginBinding>> }>;
  /** `status`: 'ready' | 'missing' (its CLI is not on PATH here) | 'blocked' (deal with it in the Agent). */
  hosts: { target: PluginTarget; version: string; status: string; error?: string; errorKey?: string; note?: string; noteKey?: string; installed: NativePlugin[] }[];
}
/**
 * What Sync would do to one binding, '' when nothing. Without `host` (the Agents have not
 * answered yet) only what config recorded is known.
 * ponytail: mirrors the sync case of Service.Preview in internal/plugin/service.go; return
 * the plan from the API instead if the two ever drift.
 */
export const syncAction = (b: PluginBinding, host?: PluginInventory['hosts'][number]) => {
  const exists = host?.installed.some((i) => i.id === b.id);
  if (b.sync === false) return exists ? 'uninstall' : '';
  if (b.pending) return b.pending === 'install' && exists ? '' : b.pending;
  return !host || host.error || exists ? '' : 'install';
};
export interface PluginCandidate { name: string; description: string; version: string; targets: PluginTarget[]; components: string[]; problem?: string; problemKey?: string; targetInfo?: Record<string, { manifest: string; version?: string; entry?: string; components: string[]; problem?: string; problemKey?: string; problemArgs?: Record<string, string> }> }
export interface PluginDiscovery {
  warnings?: string[]; source: string; sourceRef?: string; commit?: string; targetDefinitions?: PluginTargetDefinition[]; digest: string; candidates: PluginCandidate[] }
export interface PluginPlan { revision: string; blocked: boolean; changes: { name: string; target: PluginTarget; id: string; action: string; message?: string; components?: string[] }[] }
export interface PluginOutcome { name: string; target: PluginTarget; status: string; message?: string }
export interface PluginResult { result: { results: PluginOutcome[] } | null; failure: string }
const post = <T,>(path: string, body: unknown) => apiFetch<T>(path, { method: 'POST', body: JSON.stringify(body) });
export const pluginsApi = {
  /** `hosts: false` answers from config alone, without waiting on any Agent's CLI; `hosts` comes back empty. */
  list: (hosts = true) => apiFetch<PluginInventory>(hosts ? '/plugins' : '/plugins?hosts=false'),
  discover: (source: string, sourceRef?: string, entry?: string) => post<PluginDiscovery>('/plugins/discover', { source, sourceRef, entry }),
  /** The reviewed local copy of the plugin's source; empty for an imported plugin, which has none. */
  files: (name: string) => apiFetch<{ files: string[] }>(`/plugins/${encodeURIComponent(name)}/files`),
  file: (name: string, path: string) => apiFetch<{ content: string }>(`/plugins/${encodeURIComponent(name)}/files/${path.split('/').map(encodeURIComponent).join('/')}`),
  preview: (request: PluginRequest) => post<PluginPlan>('/plugins/preview', { request }),
  apply: (request: PluginRequest, revision: string) => post<PluginResult>('/plugins/apply', { request, revision }),
};
