import { apiFetch } from './client';

export type PluginTarget = string;
/** `npm`: the target installs npm:<package> sources (Pi, or a Pi account that runs pi). */
export interface PluginTargetDefinition { target: string; label: string; project: boolean; operations: string[]; reason?: string; reasonKey?: string; npm?: boolean }
export const targetMap = (definitions: PluginTargetDefinition[] = []) => Object.fromEntries(definitions.map((d) => [d.target, d]));
export type PluginAction = 'add' | 'import' | 'sync' | 'check' | 'update' | 'remove' | 'enable' | 'disable';
export interface PluginRequest { action: PluginAction; name?: string; source?: string; sourceRef?: string; entry?: string; plugin?: string; targets?: PluginTarget[]; from?: PluginTarget }
export interface PluginBinding { piRegistration?: string; id: string; source?: string; sourceRef?: string; commit?: string; entry?: string; plugin?: string; digest?: string; version?: string; pending?: string; sync?: boolean; components?: string[] }
export interface NativePlugin { id: string; version?: string; enabled: boolean; enabledKnown?: boolean; scope?: string; filtered?: boolean; importable?: boolean }
export interface PluginPackage { version?: string; source?: string; sourceRef?: string; plugin?: string; entry?: string; bindings: Partial<Record<PluginTarget, PluginBinding>> }
export interface PluginInventory {
  targetDefinitions?: PluginTargetDefinition[];
  /** `source` and the rest are set when the plugin was added, so it stays managed with no Agent bound. */
  packages: Record<string, PluginPackage>;
  /** `status`: 'ready' | 'missing' (its CLI is not on PATH here) | 'blocked' (deal with it in the Agent). */
  /** `managedMarketplaces`: the Claude/Codex marketplaces Skillshare registered from its own state directory. */
  hosts: { target: PluginTarget; version: string; status: string; error?: string; errorKey?: string; note?: string; noteKey?: string; installed: NativePlugin[]; managedMarketplaces?: string[] }[];
}
/**
 * What Sync would do to one binding, '' when nothing. Without `host` (the Agents have not
 * answered yet) only what config recorded is known.
 * ponytail: mirrors the sync case of Service.Preview in internal/plugin/service.go; return
 * the plan from the API instead if the two ever drift.
 */
export const syncAction = (b: PluginBinding, host?: PluginInventory['hosts'][number]) => {
  const exists = host?.installed.some((i) => i.id === b.id);
  // The plugin is gone but the marketplace Skillshare registered for it is still there.
  const leftover = !exists && !!b.source && b.id.includes('@') && !!host?.managedMarketplaces?.includes(b.id.slice(b.id.indexOf('@') + 1));
  if (b.sync === false) return exists || leftover ? 'uninstall' : '';
  if (b.pending) return b.pending === 'install' && exists && !b.piRegistration ? '' : b.pending;
  return !host || host.error || exists ? '' : 'install';
};
/**
 * The command that adds this plugin from its source on another machine, '' when the source is a
 * local directory, which only exists here. Agents are left for whoever runs it to choose. It always
 * adds globally: run inside a project with its own config, it would otherwise land in that project.
 */
export const pluginShareCommand = (name: string, p: { source?: string; sourceRef?: string; plugin?: string; entry?: string }) => {
  if (!p.source?.startsWith('https://')) return '';
  const quote = (s: string) => (/^[\w@%+=:,./-]+$/.test(s) ? s : `'${s.replace(/'/g, `'"'"'`)}'`);
  const flags: [string, string | undefined][] = [['--plugin', p.plugin], ['--name', p.plugin && p.plugin !== name ? name : undefined], ['--source-ref', p.sourceRef], ['--entry', p.entry]];
  return ['skillshare plugin add', quote(p.source), ...flags.filter(([, v]) => v).map(([flag, v]) => `${flag} ${quote(v!)}`), '-g'].join(' ');
};
export interface PluginCandidate { name: string; description: string; version: string; targets: PluginTarget[]; components: string[]; problem?: string; problemKey?: string; problemArgs?: Record<string, string>; targetInfo?: Record<string, { manifest: string; version?: string; logo?: string; entry?: string; components: string[]; problem?: string; problemKey?: string; problemArgs?: Record<string, string> }> }
export interface PluginDiscovery {
  warnings?: string[]; source: string; sourceRef?: string; commit?: string; targetDefinitions?: PluginTargetDefinition[]; digest: string; candidates: PluginCandidate[] }
export interface PluginPlan { revision: string; blocked: boolean; changes: { name: string; target: PluginTarget; id: string; action: string; message?: string; messageKey?: string; messageArgs?: Record<string, string>; components?: string[]; preservedKeys?: string[]; logo?: string; binding?: PluginBinding }[] }
export interface PluginOutcome { name: string; target: PluginTarget; status: string; message?: string; messageKey?: string; messageArgs?: Record<string, string> }
export interface PluginResult { result: { results: PluginOutcome[] } | null; failure: string }
const post = <T,>(path: string, body: unknown) => apiFetch<T>(path, { method: 'POST', body: JSON.stringify(body) });
export const pluginsApi = {
  /** `hosts: false` answers from config alone, without waiting on any Agent's CLI; `hosts` comes back empty. */
  list: (hosts = true) =>
    apiFetch<PluginInventory>(hosts ? '/plugins' : '/plugins?hosts=false').then((inv) => ({
      ...inv,
      // A failed inventory could arrive as null; one Agent must not break the whole page.
      hosts: inv.hosts.map((h) => ({ ...h, installed: (h.installed as NativePlugin[] | null) ?? [] })),
    })),
  /** `name` is a plugin already managed here: its reviewed copy answers instead of downloading the source again. */
  discover: (source: string, sourceRef?: string, entry?: string, name?: string) => post<PluginDiscovery>('/plugins/discover', { source, sourceRef, entry, name }),
  /** The reviewed local copy of the plugin's source; empty for an imported plugin, which has none. */
  files: (name: string) => apiFetch<{ files: string[] }>(`/plugins/${encodeURIComponent(name)}/files`),
  file: (name: string, path: string) => apiFetch<{ content: string }>(`/plugins/${encodeURIComponent(name)}/files/${path.split('/').map(encodeURIComponent).join('/')}`),
  preview: (request: PluginRequest) => post<PluginPlan>('/plugins/preview', { request }),
  apply: (request: PluginRequest, revision: string) => post<PluginResult>('/plugins/apply', { request, revision }),
};
