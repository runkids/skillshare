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
export interface PluginCandidate { name: string; description: string; version: string; targets: PluginTarget[]; components: string[]; problem?: string; targetInfo?: Record<string, { manifest: string; version?: string; entry?: string; components: string[]; problem?: string }> }
export interface PluginDiscovery {
  warnings?: string[]; source: string; sourceRef?: string; commit?: string; targetDefinitions?: PluginTargetDefinition[]; digest: string; candidates: PluginCandidate[] }
export interface PluginPlan { revision: string; blocked: boolean; changes: { name: string; target: PluginTarget; id: string; action: string; message?: string; components?: string[] }[] }
export interface PluginOutcome { name: string; target: PluginTarget; status: string; message?: string }
export interface PluginResult { result: { results: PluginOutcome[] } | null; failure: string }
const post = <T,>(path: string, body: unknown) => apiFetch<T>(path, { method: 'POST', body: JSON.stringify(body) });
export const pluginsApi = {
  list: () => apiFetch<PluginInventory>('/plugins'),
  discover: (source: string, sourceRef?: string, entry?: string) => post<PluginDiscovery>('/plugins/discover', { source, sourceRef, entry }),
  preview: (request: PluginRequest) => post<PluginPlan>('/plugins/preview', { request }),
  apply: (request: PluginRequest, revision: string) => post<PluginResult>('/plugins/apply', { request, revision }),
};
