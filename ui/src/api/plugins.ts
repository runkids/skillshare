import { apiFetch } from './client';
import { BASE, parseApiErrorPayload } from './http';

export type PluginTarget = string;
/** `npm`: the target installs npm:<package> sources (Pi, or a Pi account that runs pi). */
export interface PluginTargetDefinition { target: string; label: string; project: boolean; operations: string[]; reason?: string; reasonKey?: string; npm?: boolean }
export const targetMap = (definitions: PluginTargetDefinition[] = []) => Object.fromEntries(definitions.map((d) => [d.target, d]));
export type PluginAction = 'add' | 'import' | 'unimport' | 'sync' | 'check' | 'update' | 'remove' | 'enable' | 'disable';
export interface PluginRequest { action: PluginAction; name?: string; source?: string; sourceRef?: string; entry?: string; plugin?: string; targets?: PluginTarget[]; from?: PluginTarget }
export interface PluginBinding { refChanged?: boolean; imported?: boolean; importRetained?: boolean; cleanup?: boolean; contextChanged?: boolean; piRegistration?: string; id: string; source?: string; sourceRef?: string; commit?: string; entry?: string; plugin?: string; digest?: string; version?: string; pending?: string; sync?: boolean; components?: string[] }
export interface NativePlugin { id: string; version?: string; enabled: boolean; enabledKnown?: boolean; scope?: string; filtered?: boolean; importable?: boolean }
export interface PluginPackage { version?: string; source?: string; sourceRef?: string; plugin?: string; entry?: string; bindings: Partial<Record<PluginTarget, PluginBinding>> }
export interface PluginInventory {
  targetDefinitions?: PluginTargetDefinition[];
  /** `source` and the rest are set when the plugin was added, so it stays managed with no Agent bound. */
  packages: Record<string, PluginPackage>;
  /** `status`: 'ready' | 'missing' (its CLI is not on PATH here) | 'blocked' (deal with it in the Agent). */
  /** `managedMarketplaces`: the Claude/Codex marketplaces Skillshare registered from its own state directory. */
  hosts: { target: PluginTarget; version: string; status: string; error?: string; errorKey?: string; errorArgs?: Record<string, string>; note?: string; noteKey?: string; installed: NativePlugin[]; managedMarketplaces?: string[] }[];
}
/**
 * What Sync would do to one binding, '' when nothing. Without `host` (the Agents have not
 * answered yet) only what config recorded is known.
 * ponytail: mirrors the sync case of Service.Preview in internal/plugin/service.go; return
 * the plan from the API instead if the two ever drift.
 */
export const syncAction = (b: PluginBinding, host?: PluginInventory['hosts'][number]) => {
  if (b.contextChanged) return '';
  if (b.imported && !b.id) return '';
  const exists = host?.installed.some((i) => i.id === b.id);
  // The plugin is gone but the marketplace Skillshare registered for it is still there.
  const leftover = !exists && !!b.source && b.id.includes('@') && !!host?.managedMarketplaces?.includes(b.id.slice(b.id.indexOf('@') + 1));
  if (b.cleanup) return host && !host.error ? 'remove' : '';
  if (b.sync === false) return exists || leftover ? 'uninstall' : '';
  if (b.refChanged && host?.status === 'ready' && !host.error) return exists ? 'update' : 'install';
  if (b.pending) return b.pending === 'install' && exists && !b.piRegistration ? '' : b.pending;
  return !host || host.error || exists ? '' : 'install';
};
/** Local binding state shown beside the existing target checkbox. */
export const bindingStatus = (b: PluginBinding, host?: PluginInventory['hosts'][number]) => {
  if (b.contextChanged) return 'plugins.accountChanged';
  if (b.cleanup) return 'plugins.cleanup';
  if (b.imported && !b.id) return 'plugins.importRequired';
  if (syncAction(b, host)) return 'plugins.pending';
  if (b.importRetained) return 'plugins.importRetained';
  if (!host) return '';
  if (host.error) return 'plugins.unverified';
  const installed = host.installed.find((i) => i.id === b.id);
  return installed && installed.enabledKnown !== false && !installed.enabled ? 'plugins.nativeDisabled' : '';
};
/**
 * A binding's version: what config recorded, else what the Agent reports installed. On a Pi
 * target Pi's answer comes first: it reads each installed package's package.json, npm, git or
 * local, and a version recorded at import goes stale with the next pi update.
 */
export const bindingVersion = (inventory: Pick<PluginInventory, 'hosts' | 'targetDefinitions'>, target: PluginTarget, b: PluginBinding) => {
  const installed = inventory.hosts.find((h) => h.target === target)?.installed.find((i) => i.id === b.id)?.version;
  const pi = inventory.targetDefinitions?.some((d) => d.target === target && d.npm);
  return pi ? installed ?? b.version : b.version ?? installed;
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
export interface PluginCandidate { name: string; description: string; version: string; targets: PluginTarget[]; components: string[]; problem?: string; problemKey?: string; problemArgs?: Record<string, string>; targetInfo?: Record<string, { manifest: string; path?: string; version?: string; logo?: string; entry?: string; components: string[]; problem?: string; problemKey?: string; problemArgs?: Record<string, string> }> }
export interface PluginDiscovery {
  warnings?: string[]; source: string; sourceRef?: string; commit?: string; targetDefinitions?: PluginTargetDefinition[]; digest: string; candidates: PluginCandidate[] }
export interface PluginPlan { revision: string; blocked: boolean; changes: { name: string; target: PluginTarget; id: string; action: string; guidance?: boolean; message?: string; messageKey?: string; messageArgs?: Record<string, string>; components?: string[]; preservedKeys?: string[]; logo?: string; binding?: PluginBinding }[] }
export interface PluginOutcome { name: string; target: PluginTarget; status: string; guidance?: boolean; message?: string; messageKey?: string; messageArgs?: Record<string, string> }

export interface PluginRun { name: string; target: string; version?: string }

/** Rows that differ only by Agent become one row naming every Agent; `key` says what else must match. */
export function byPlugin<R extends PluginRun>(rows: R[], key: (r: R) => string = () => '') {
  const groups = new Map<string, R & { targets: string[] }>();
  for (const r of rows) {
    const k = `${r.name}\0${key(r)}`;
    const group = groups.get(k);
    if (group) group.targets.push(r.target);
    else groups.set(k, { ...r, targets: [r.target] });
  }
  return [...groups.values()];
}
export interface PluginResult { result: { results: PluginOutcome[] } | null; failure: string }
/** What a check showed for one Agent of a plugin: the version an update would bring, and the source it came from. */
export interface PluginUpdate { version: string; commit?: string; digest?: string; id?: string; source?: string; sourceRef?: string; plugin?: string; entry?: string }
/** One plugin of a batch update, with what the check showed per Agent so a source that moved since is not installed. */
export interface PluginUpdateItem { name: string; targets: PluginTarget[]; seen?: Record<PluginTarget, PluginUpdate> }
export type PluginUpdateEvent =
  | { event: 'start'; name: string }
  | { event: 'plan'; name: string; plan: PluginPlan }
  | { event: 'result'; name: string; plan: PluginPlan | null; result: PluginResult['result']; failure: string; stale: boolean }
  | { event: 'done'; failure: string };

/** Reads the server's event stream for a batch update until `done`. EventSource (createSSEStream) cannot POST the picks, so fetch reads the stream. */
async function updateStream(items: PluginUpdateItem[], on: (e: PluginUpdateEvent) => void) {
  const res = await fetch(BASE + '/plugins/update/stream', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ items }) });
  if (!res.ok || !res.body) throw new Error(parseApiErrorPayload(await res.json().catch(() => null), res.status, res.statusText).message);
  const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
  let buffer = '';
  for (;;) {
    const { value, done } = await reader.read();
    if (done) throw new Error('The update stream closed before it finished; refresh to see what changed.');
    buffer += value;
    for (let end = buffer.indexOf('\n\n'); end >= 0; end = buffer.indexOf('\n\n')) {
      const block = buffer.slice(0, end);
      buffer = buffer.slice(end + 2);
      const event = /^event: (.*)$/m.exec(block)?.[1];
      const data = /^data: (.*)$/m.exec(block)?.[1];
      if (!event || data === undefined) continue;
      on({ event, ...JSON.parse(data) } as PluginUpdateEvent);
      if (event === 'done') return;
    }
  }
}

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
  /** Updates the plugins one after another on the server; each one's progress arrives through `on`. */
  update: updateStream,
  /** The plugin under way finishes; no other starts. */
  stopUpdate: () => post<{ stopping: boolean }>('/plugins/update/stop', {}),
};
