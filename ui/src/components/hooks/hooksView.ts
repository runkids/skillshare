import { hookAgents, hookCodeAgents, type HookChange, type HookEntry, type HookPlan, type HookTargetDef } from '../../api/hooks';
import { parseDocument, stringify as stringifyYaml } from 'yaml';

export const hookLabel = (agent: string) =>
  ({ claude: 'Claude', codex: 'Codex', gemini: 'Gemini CLI', copilot: 'Copilot CLI', cursor: 'Cursor', droid: 'Droid', qwen: 'Qwen Code', antigravity: 'Antigravity', pi: 'Pi', omp: 'Oh My Pi', amp: 'Amp', opencode: 'OpenCode', git: 'Git' })[agent] ?? agent;

/** The dashboard's own wording of an Agent's native loading and trust guidance. Only an Agent this build does not know falls back to the server's English note. */
export const hookNote = (t: (key: string) => string, agent: string, fallback?: string) =>
  (hookAgents as readonly string[]).includes(agent) ? t(`hooks.native.${agent}`) : fallback;

export const isCodeAgent = (agent: string) => hookCodeAgents.includes(agent);

/** Mirrors the source's entry-name rule: a stable, file-safe key. */
export const HOOK_NAME = /^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$/;

/** Bound Agents of an entry in display order; `factory` is accepted as an alias of `droid`. */
export const hookAccounts = (targets: HookTargetDef[]): Record<string, string> =>
  Object.fromEntries(targets.filter((t) => t.agent).map((t) => [t.name, t.agent!]));
export const agentOfKey = (accounts: Record<string, string>, key: string): string => accounts[key] ?? key;
export const keyLabel = (accounts: Record<string, string>, key: string): string => accounts[key] ? `${key} (${hookLabel(accounts[key])})` : hookLabel(key);

export const boundAgents = (entry: HookEntry, accounts: Record<string, string> = {}) =>
  Object.keys(entry.bindings ?? {}).map((x) => (x === 'factory' ? 'droid' : x)).sort((a, b) =>
    order(agentOfKey(accounts, a)) - order(agentOfKey(accounts, b)) || Number(Boolean(accounts[a])) - Number(Boolean(accounts[b])) || a.localeCompare(b));

const order = (agent: string) => {
  const i = (hookAgents as readonly string[]).indexOf(agent);
  return i < 0 ? hookAgents.length : i;
};

/** A change Sync applies; `adopt` only records a registration the Agent already has. */
export const writes = (change: { action: string }) => ['add', 'adopt', 'update', 'remove', 'restore'].includes(change.action);

/** The synchronized state of one entry in one Agent, from the plan. `none` when the plan says nothing (not published). */
export type SyncState = 'synced' | 'pending' | 'conflict' | 'inactive' | 'none';
export function syncState(plan: HookPlan | null | undefined, name: string, agent: string): SyncState {
  const changes = (plan?.changes ?? []).filter((c) => c.name === name && c.target === agent);
  if (changes.some((c) => c.action === 'conflict')) return 'conflict';
  if (changes.some(writes)) return 'pending';
  if (changes.some((c) => c.action === 'inactive' || c.inactiveReason)) return 'inactive';
  return changes.length > 0 ? 'synced' : 'none';
}

export const statusTone: Record<string, 'ok' | 'warn' | 'bad' | ''> = {
  unchanged: 'ok', add: 'warn', update: 'warn', adopt: 'warn', restore: 'warn', remove: 'bad', conflict: 'bad',
  inactive: 'warn',
};

const KNOWN_ACTIONS = new Set(['add', 'adopt', 'update', 'restore', 'unchanged', 'conflict', 'remove', 'inactive']);
/** Localized action word; an action the UI does not know is shown as the server sent it. */
export const actionLabel = (t: (key: string) => string, action: string) => (KNOWN_ACTIONS.has(action) ? t(`hooks.status.${action}`) : action);

// Domain notes the user can act on in this UI, reworded around its own take-over action; any other note is shown as sent.
const messageKeys: Record<string, string> = {
  'an identical hook exists that Skillshare does not manage; import it or explicitly replace it': 'hooks.message.identicalUnmanaged',
  'saving the import takes over the existing registrations in place; sync leaves them as they are instead of adding duplicates': 'hooks.message.importUnmanaged',
};
const UNKNOWN_EVENT = /^hook (.+): (\S+) does not document the event "(.+)"; check its spelling$/;
export const hookMessage = (t: (key: string, params?: Record<string, string>) => string, message: string) => {
  if (messageKeys[message]) return t(messageKeys[message]);
  const unknown = UNKNOWN_EVENT.exec(message);
  return unknown ? t('hooks.message.unknownEvent', { name: unknown[1], agent: hookLabel(unknown[2]), event: unknown[3] }) : message;
};
export const needsTakeover = (change: HookChange) => change.action === 'conflict' && messageKeys[change.message ?? ''] === 'hooks.message.identicalUnmanaged';

export const blockedHint = (t: (key: string) => string, plan: HookPlan) => {
  const conflicts = plan.changes.filter((c) => c.action === 'conflict');
  return t(conflicts.length > 0 && conflicts.every(needsTakeover) ? 'hooks.unmanagedHint' : 'hooks.blockedHint');
};

/** The last part of a native path, which tells a hook's settings file from its script. */
export const fileName = (path: string) => path.split(/[\\/]/).pop() ?? path;

export function groupByFile(changes: HookChange[]) {
  const files = new Map<string, { target: string; path: string; root?: string; changes: HookChange[] }>();
  for (const change of changes) {
    const file = files.get(change.path) ?? { target: change.target, path: change.path, root: change.root, changes: [] };
    file.changes.push(change);
    files.set(change.path, file);
  }
  return [...files.values()];
}

/** Backup IDs start with the Unix time in nanoseconds. */
export const backupTime = (id: string) => new Date(Number(id.split('-')[0]) / 1e6);

// ---- Command events: the simple editor -------------------------------------------------------

/**
 * One row of the simple editor. Command Agents write either a matcher group
 * `{ matcher?, hooks: [handler] }` or a bare handler; `raw*` carry every other native field
 * so a round trip through the editor never drops one.
 */
export interface HookRow {
  id: string;
  event: string;
  matcher: string;
  command: string;
  timeout: string;
  shape: 'group' | 'flat';
  /** The native key that holds the command (`command`, `bash`, ...), and the one that holds the timeout. */
  commandKey: string;
  timeoutKey: string;
  handler: Record<string, unknown>;
  group: Record<string, unknown>;
}

const COMMAND_KEYS = ['command', 'bash', 'powershell'];
const TIMEOUT_KEYS = ['timeout', 'timeoutSec', 'timeout_ms', 'timeoutMs'];
const isObject = (v: unknown): v is Record<string, unknown> => Boolean(v) && typeof v === 'object' && !Array.isArray(v);
let seq = 0;
const rowId = () => `row-${++seq}`;
const omit = (o: Record<string, unknown>, keys: string[]) => Object.fromEntries(Object.entries(o).filter(([k]) => !keys.includes(k)));

/** Whether an Agent's registration for an event is a matcher group or a bare handler. Antigravity groups only its tool events. */
export const rowShape = (agent: string, event: string): HookRow['shape'] =>
  agent === 'copilot' || agent === 'cursor' || (agent === 'antigravity' && event !== 'PreToolUse' && event !== 'PostToolUse') ? 'flat' : 'group';

/** How a fresh row is written for an Agent: the native shape, not a translation of another Agent's. */
export function newRow(agent: string, event = ''): HookRow {
  return {
    id: rowId(), event, matcher: '', command: '', timeout: '',
    shape: rowShape(agent, event),
    commandKey: agent === 'copilot' ? 'bash' : 'command',
    timeoutKey: agent === 'copilot' ? 'timeoutSec' : 'timeout',
    handler: agent === 'cursor' || agent === 'antigravity' ? {} : { type: 'command' },
    group: {},
  };
}

function rowFromHandler(event: string, handler: Record<string, unknown>, group: Record<string, unknown>, shape: HookRow['shape']): HookRow | null {
  const commandKey = COMMAND_KEYS.find((k) => typeof handler[k] === 'string');
  if (!commandKey) return null;
  const timeoutKey = TIMEOUT_KEYS.find((k) => k in handler) ?? (shape === 'flat' ? 'timeoutSec' : 'timeout');
  const timeout = handler[timeoutKey];
  if (timeout !== undefined && typeof timeout !== 'number') return null;
  const rest = omit(handler, [commandKey, timeoutKey]);
  const command = handler[commandKey] as string;
  const matcher = group.matcher;
  const groupRest = omit(group, ['hooks', 'matcher']);
  if (matcher !== undefined && typeof matcher !== 'string') return null;
  return {
    id: rowId(), event, matcher: (matcher as string | undefined) ?? '', command, timeout: timeout === undefined ? '' : String(timeout),
    shape, commandKey, timeoutKey, handler: rest, group: groupRest,
  };
}

/**
 * The events as simple rows, or null when they use a shape the editor cannot round-trip
 * (several handlers in one group, non-command handlers, ...). The native JSON editor takes those.
 */
export function eventsToRows(events: Record<string, unknown> | undefined): HookRow[] | null {
  const rows: HookRow[] = [];
  for (const [event, list] of Object.entries(events ?? {})) {
    if (!Array.isArray(list)) return null;
    for (const item of list) {
      if (!isObject(item)) return null;
      if (Array.isArray(item.hooks)) {
        if (item.hooks.length !== 1 || !isObject(item.hooks[0])) return null;
        const row = rowFromHandler(event, item.hooks[0], item, 'group');
        if (!row) return null;
        rows.push(row);
      } else {
        const row = rowFromHandler(event, item, {}, 'flat');
        if (!row) return null;
        rows.push(row);
      }
    }
  }
  return rows;
}

/** A row with nothing typed in any field; only such rows are left out. Any other incomplete row is flagged. */
export const rowBlank = (r: HookRow) => !r.event.trim() && !r.matcher.trim() && !r.command.trim() && !r.timeout.trim();

/** Rows back to the Agent's own event map. Rows of one event keep their order; blank rows are skipped. */
export function rowsToEvents(rows: HookRow[]): Record<string, unknown> | undefined {
  const events: Record<string, unknown[]> = {};
  for (const r of rows) {
    if (rowBlank(r)) continue;
    const timeout = r.timeout.trim();
    const handler: Record<string, unknown> = { ...r.handler, [r.commandKey]: r.command, ...(timeout && { [r.timeoutKey]: Number(timeout) }) };
    const item = r.shape === 'group'
      ? { ...(r.matcher.trim() && { matcher: r.matcher.trim() }), ...r.group, hooks: [handler] }
      : handler;
    (events[r.event.trim()] ??= []).push(item);
  }
  return Object.keys(events).length > 0 ? events : undefined;
}

export interface RowProblems { event?: boolean; command?: boolean; timeout?: boolean }
export const rowProblems = (r: HookRow): RowProblems => ({
  event: !r.event.trim(),
  command: !r.command.trim(),
  timeout: r.timeout.trim() !== '' && !(Number.isFinite(Number(r.timeout)) && Number(r.timeout) > 0),
});

/** Event names an Agent documents, offered as suggestions only; any name is accepted. */
export const eventSuggestions = (agent: string): string[] => {
  if (agent === 'gemini') return ['BeforeTool', 'AfterTool', 'BeforeAgent', 'AfterAgent', 'SessionStart', 'SessionEnd'];
  if (agent === 'copilot') return ['sessionStart', 'userPromptSubmitted', 'preToolUse', 'postToolUse', 'sessionEnd'];
  if (agent === 'cursor') return ['beforeShellExecution', 'afterFileEdit', 'beforeSubmitPrompt', 'stop'];
  return ['PreToolUse', 'PostToolUse', 'UserPromptSubmit', 'SessionStart', 'Stop'];
};

// ---- Draft <-> entry --------------------------------------------------------------------------

export interface FileRow { id: string; name: string; content: string }

/** What the dialog edits for one Agent. `mode` picks which of rows/native is the source of truth. */
export interface BindingDraft {
  mode: 'simple' | 'native';
  rows: HookRow[];
  /** The Agent's native event map as JSON text. */
  native: string;
  code: string;
  files: FileRow[];
}

export const emptyBinding = (agent: string): BindingDraft => ({ mode: agent === 'git' ? 'native' : 'simple', rows: isCodeAgent(agent) || agent === 'git' ? [] : [newRow(agent)], native: '', code: '', files: [] });

export function bindingToDraft(agent: string, binding: HookEntry['bindings'][string] | undefined): BindingDraft {
  if (!binding) return emptyBinding(agent);
  if (agent === 'git') return { ...emptyBinding(agent), native: stringifyYaml(binding) };
  const rows = eventsToRows(binding.events);
  return {
    mode: rows ? 'simple' : 'native',
    rows: rows ?? [],
    native: binding.events ? JSON.stringify(binding.events, null, 2) : '',
    code: binding.code ?? '',
    files: Object.entries(binding.files ?? {}).map(([name, content]) => ({ id: rowId(), name, content })),
  };
}

/** Parses the native JSON box: empty is no events, anything but an object is an error. */
export function parseNative(text: string): { value?: Record<string, unknown>; error?: true } {
  if (!text.trim()) return {};
  try {
    const value: unknown = JSON.parse(text);
    return isObject(value) ? { value } : { error: true };
  } catch {
    return { error: true };
  }
}

/** Git edits the complete binding, including helpers, as YAML. Domain validation
 * remains on the server; unknown properties are preserved so it can reject them. */
export function parseGitBinding(text: string): { value?: HookEntry['bindings'][string]; error?: true } {
  if (!text.trim()) return {};
  try {
    const doc = parseDocument(text);
    if (doc.errors.length > 0) return { error: true };
    const value: unknown = doc.toJS({ maxAliasCount: 100 });
    return isObject(value) ? { value } : { error: true };
  } catch { return { error: true }; }
}

/** Switching to the other editor carries the current content over; native text that is not simple stays native. */
export function switchMode(draft: BindingDraft, mode: BindingDraft['mode']): BindingDraft {
  if (mode === draft.mode) return draft;
  if (mode === 'native') return { ...draft, mode, native: JSON.stringify(rowsToEvents(draft.rows) ?? {}, null, 2) };
  const parsed = parseNative(draft.native);
  const rows = parsed.error ? null : eventsToRows(parsed.value);
  return rows ? { ...draft, mode, rows } : draft;
}

const SCRIPT_NAME = /^[A-Za-z0-9_][A-Za-z0-9_.-]*$/;

export interface BindingCheck {
  nativeError: boolean;
  rowErrors: number;
  fileErrors: boolean;
  codeMissing: boolean;
  empty: boolean;
}

export function checkBinding(agent: string, d: BindingDraft): BindingCheck {
  if (agent === 'git') {
    const parsed = parseGitBinding(d.native);
    return { nativeError: Boolean(parsed.error), rowErrors: 0, fileErrors: false, codeMissing: false, empty: !isObject(parsed.value?.commands) || Object.keys(parsed.value.commands).length === 0 };
  }
  if (isCodeAgent(agent)) return { nativeError: false, rowErrors: 0, fileErrors: false, codeMissing: !d.code.trim(), empty: !d.code.trim() };
  const nativeError = d.mode === 'native' && Boolean(parseNative(d.native).error);
  const active = d.rows.filter((r) => !rowBlank(r));
  const rowErrors = d.mode === 'simple' ? active.filter((r) => Object.values(rowProblems(r)).some(Boolean)).length : 0;
  const names = d.files.map((f) => f.name.trim());
  const fileErrors = names.some((n) => !SCRIPT_NAME.test(n) || n === '.' || n === '..') || new Set(names).size !== names.length;
  const events = d.mode === 'native' ? parseNative(d.native).value : rowsToEvents(d.rows);
  return { nativeError, rowErrors, fileErrors, codeMissing: false, empty: !events || Object.keys(events).length === 0 };
}

export const bindingInvalid = (c: BindingCheck) => c.nativeError || c.rowErrors > 0 || c.fileErrors || c.codeMissing || c.empty;

export function draftToBinding(agent: string, d: BindingDraft) {
  if (agent === 'git') return parseGitBinding(d.native).value ?? {};
  const files = Object.fromEntries(d.files.filter((f) => f.name.trim()).map((f) => [f.name.trim(), f.content]));
  const withFiles = Object.keys(files).length > 0 ? { files } : {};
  if (isCodeAgent(agent)) return { code: d.code, ...withFiles };
  const events = d.mode === 'native' ? parseNative(d.native).value : rowsToEvents(d.rows);
  return { ...(events && { events }), ...withFiles };
}

export const newFileRow = (): FileRow => ({ id: rowId(), name: '', content: '' });

// ---- Scopes: the global source and each hooks.projects root ---------------------------------

interface Scoped {
  source: { entries: Record<string, HookEntry>; projects?: Record<string, { entries?: Record<string, HookEntry> }> };
  plan: HookPlan | null;
}

/** Roots the global source manages hooks for. */
export const hookRoots = (data: Pick<Scoped, 'source'>) => Object.keys(data.source.projects ?? {});

/** The hooks of one scope: a project root's own, or the global source's. Never one for the other. */
export const scopeEntries = (data: Scoped, project?: string): Record<string, HookEntry> =>
  project ? data.source.projects?.[project]?.entries ?? {} : data.source.entries;

/** The plan's changes of one scope; a project's files carry its root, so global lists leave them out. */
export function scopeChanges(data: Scoped, project?: string): HookChange[] {
  const roots = hookRoots(data);
  return (data.plan?.changes ?? []).filter((c) => (project ? c.root === project : !(c.root && roots.includes(c.root))));
}

/** Native paths of a scope; a project never shows the global files. */
export const scopePaths = (data: { paths: Record<string, string>; projectPaths?: Record<string, Record<string, string>> }, project?: string): Record<string, string> =>
  project ? data.projectPaths?.[project] ?? {} : data.paths;

type Rooted = { source: { projects?: Record<string, unknown> }; projectPaths?: Record<string, unknown> };
const under = (path: string, root: string) => path === root || path.startsWith(`${root.replace(/[\\/]+$/, '')}/`) || path.startsWith(`${root.replace(/[\\/]+$/, '')}\\`);

/** The hooks.projects root a native file sits under (the deepest one), or undefined for the global files. Fallback for payloads without project/root metadata; linked-worktree outputs may sit outside their source root. */
export const ownerRoot = (data: Rooted, path: string) =>
  [...new Set([...Object.keys(data.source.projects ?? {}), ...Object.keys(data.projectPaths ?? {})])]
    .filter((root) => under(path, root)).sort((a, b) => b.length - a.length)[0];

/** Native hooks of one scope that Skillshare does not manage. */
export const scopeUnmanaged = <T extends { path: string; project?: string }>(data: Rooted & { unmanaged: T[] }, project?: string) =>
  data.unmanaged.filter((u) => (u.project ?? ownerRoot(data, u.path)) === project);

/** Backups of the files of one scope. */
export const scopeBackups = <T extends { path: string; root?: string }>(data: Rooted & { backups: T[] }, project?: string) =>
  data.backups.filter((b) => (b.root || ownerRoot(data, b.path)) === project);

/**
 * A plan seen from one project root: only that root's changes, blocked only by its own conflicts. The
 * revision stays the whole plan's, since the server still checks it against everything. Global scope: unchanged.
 */
export const rootPlan = (plan: HookPlan, project?: string): HookPlan => {
  if (!project) return plan;
  const changes = plan.changes.filter((c) => c.root === project);
  return { ...plan, changes, blocked: changes.some((c) => c.action === 'conflict') };
};

/** The scope's plan, narrowed to its own changes. `blocked` follows the narrowed changes for a project. */
export function scopePlan(data: Scoped, project?: string): HookPlan | null {
  if (!data.plan) return null;
  return project ? rootPlan(data.plan, project) : { ...data.plan, changes: scopeChanges(data), blocked: data.plan.blocked };
}

/** Enabled hooks of a scope that publish to one Agent. A disabled hook is kept but not counted. */
export const hookCount = (data: Scoped, agent: string, project?: string) =>
  Object.values(scopeEntries(data, project)).filter((e) => e.enabled !== false && boundAgents(e).includes(agent)).length;

/** Bare folder name of a project root. */
export const rootName = (root: string) => root.replace(/[\\/]+$/, '').split(/[\\/]/).pop() || root;

/** The hooks binding key: account names stay distinct; project targets use their tool. */
export const hookAgentOf = (target: string, agent?: string) => {
  if (agent) return (hookAgents as readonly string[]).includes(agent) ? target : undefined;
  const tool = target.slice(target.lastIndexOf('@') + 1);
  // The Antigravity IDE and CLI share one hooks file, so both are the one hooks target.
  const name = tool === 'antigravity-cli' || tool === 'agy' ? 'antigravity' : tool;
  return (hookAgents as readonly string[]).includes(name) ? name : undefined;
};
