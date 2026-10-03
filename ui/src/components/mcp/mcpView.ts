import { mcpOffTargets, mcpTargets } from '../../api/mcp';
import type { MCPPlan, MCPServer, MCPToolPolicy } from '../../api/mcp';

export type MCPChange = MCPPlan['changes'][number];

export interface MatrixRow {
  name: string;
  /** Undefined when the entry was removed from the source but is still in Agent files. */
  server?: MCPServer;
  cells: Record<string, MCPChange>;
}

export const statusVariant: Record<string, 'default' | 'success' | 'warning' | 'danger' | 'info'> = {
  add: 'info', adopt: 'info', update: 'info', restore: 'info', unchanged: 'success', conflict: 'warning', remove: 'danger',
};

/** One row per source server, plus rows for entries the plan will remove from Agents. */
export function buildMatrix(servers: Record<string, MCPServer>, plan: MCPPlan | null): MatrixRow[] {
  const rows = new Map<string, MatrixRow>(Object.entries(servers).map(([name, server]) => [name, { name, server, cells: {} }]));
  for (const change of plan?.changes ?? []) {
    const row = rows.get(change.name) ?? { name: change.name, cells: {} };
    const previous = row.cells[change.target];
    // A Pi migration can affect two files for one target; unchanged must not hide pending work.
    if (previous?.action !== 'conflict' && !(change.action === 'unchanged' && previous && writes(previous))) row.cells[change.target] = change;
    rows.set(change.name, row);
  }
  return [...rows.values()];
}

/** Every MCP target in display order; accounts of an Agent follow the Agents, by name. */
export const mcpOrder = (accounts?: Record<string, unknown>) => [...mcpTargets, ...Object.keys(accounts ?? {}).sort()];

/** A change that Sync applies; `adopt` only records an entry the Agent already has. `unchanged` and `conflict` do nothing. */
export const writes = (change: { action: string }) => ['add', 'adopt', 'update', 'remove'].includes(change.action);

/** The MCP client a skill target writes to: Factory's skills target is named droid, and agy shares Antigravity's MCP file. */
const MCP_CLIENTS: Record<string, string> = { droid: 'factory', 'antigravity-cli': 'antigravity' };
export const mcpClient = (target: string) => MCP_CLIENTS[target] ?? target;

/** How many servers a scope writes to one Agent, none when it has no MCP file there. A switch-only entry turns a server off, so it does not count. */
export const serverCount = (data: { source: { servers?: Record<string, MCPServer>; targets: string[] | null }; paths: Record<string, string> }, target: string) =>
  data.paths[target] ? Object.values(data.source.servers ?? {}).filter((s) => !s.disabled && (s.targets ?? data.source.targets ?? []).includes(target)).length : 0;

export const inside = (root: string, path: string) => path.startsWith(root + '/') || path.startsWith(root + '\\');

/** The mcp.projects root a change belongs to, if any. */
// The plan says so outright, because Claude Code's off list is written to the global file
// rather than to anything under the folder it turns a server off for.
export const projectOf = (roots: string[], change: MCPChange) =>
  change.root ?? roots.find((root) => inside(root, change.path));

export function countActions(changes: MCPChange[]): Record<string, number> {
  const counts: Record<string, number> = {};
  for (const change of changes) counts[change.action] = (counts[change.action] ?? 0) + 1;
  return counts;
}

/** The project a change turns a server off for, when it edits Claude Code's off list. */
// That list is the one project destination outside its root: it sits in ~/.claude.json, next
// to the user-scope servers, so the path alone reads as a change to the global server.
export const offListFor = (change: MCPChange) => (change.root && !inside(change.root, change.path) ? change.root : undefined);

export function groupByFile(changes: MCPChange[]) {
  const files = new Map<string, { key: string; target: string; path: string; offListFor?: string; changes: MCPChange[] }>();
  for (const change of changes) {
    const key = `${change.path}\0${offListFor(change) ?? ''}`;
    const file = files.get(key) ?? { key, target: change.target, path: change.path, offListFor: offListFor(change), changes: [] };
    file.changes.push(change);
    files.set(key, file);
  }
  return [...files.values()];
}

// ponytail: keyed on the backend Change.Message text; add a reason code to mcp.Change if these start drifting.
const shadowMessage = 'a local scope server of the same name in ~/.claude.json overrides this one in this project; remove it with:';

// Keyed by how the message starts: some end in a path or a command.
const conflictKeys: Record<string, string> = {
  [shadowMessage]: 'mcp.claudeLocalShadow',
  'managed by another Skillshare config': 'mcp.conflictOtherConfig',
  'left over from a Skillshare config that was removed; import it or explicitly replace this entry': 'mcp.conflictOrphaned',
  'Pi setting changed since sync; import it before removing the cleared setting': 'mcp.conflictChanged',
  'Agent configuration changed; import it or explicitly replace this entry': 'mcp.conflictChanged',
  'existing entry is not managed; import it to explicitly adopt it': 'mcp.conflictUnmanaged',
  'existing entry is a Pi project override of a global server; replace it, or remove the override with /mcp in Pi': 'mcp.conflictPiOverride',
  'entry changed after the backup; restore would overwrite newer changes': 'mcp.conflictAfterBackup',
};

const conflictPrefix = (message: string) => Object.keys(conflictKeys).find((k) => message.startsWith(k)) ?? '';

export const describeMessage = (t: (key: string) => string, message = '') => {
  const start = conflictPrefix(message);
  return start ? t(conflictKeys[start]) + message.slice(start.length) : message;
};

// What the server says about a snippet or file it cannot read. It words them for a sync,
// where "target was not changed" matters; a paste has no target to change.
const importErrorKeys: Record<string, string> = {
  'invalid JSON/JSONC; target was not changed': 'mcp.importError.json',
  'invalid server JSON': 'mcp.importError.json',
  'invalid TOML; target was not changed': 'mcp.importError.toml',
  'no MCP entries found': 'mcp.importError.noEntries',
  'no MCP entries found; select the matching client format': 'mcp.importError.noEntries',
};

const kiloProjectEnvPrefix = 'Kilo Code MCP ';
const kiloProjectEnvSuffix = ': Kilo does not allow environment references in project config and ignores the whole file when it finds one; remove fromEnv here or define this server in global mode';

/** Localizes the errors people can act on: an unreadable snippet, and the Kilo project error, whose leading project path is kept. */
export const describeError = (t: (key: string, params?: Record<string, string>) => string, message = '') => {
  if (importErrorKeys[message]) return t(importErrorKeys[message]);
  const start = message.indexOf(kiloProjectEnvPrefix);
  if (start < 0) return message;
  const nameStart = start + kiloProjectEnvPrefix.length;
  const end = message.indexOf(kiloProjectEnvSuffix, nameStart);
  if (end < 0) return message;
  return message.slice(0, start) + t('mcp.kilocodeProjectEnv', { name: message.slice(nameStart, end) });
};

/** Pi entry fields Skillshare writes from the server's own settings; the backend refuses them in piOptions. */
const piOwnFields = new Set(['command', 'args', 'env', 'url', 'headers', 'transport', 'disabled', 'type', 'settings', 'autoEnableCodemode']);

/** pi-mcp-adapter fields Pi's built-in MCP does not read; the backend refuses them. The first three are what tools now holds. The adapter's auth is a string; Pi's own (0.99.2) is an object. */
const adapterToolFields = new Set(['directTools', 'includeTools', 'excludeTools']);
const adapterFields = new Set(['approveTools', 'auth', 'bearerToken', 'bearerTokenEnv', 'bearerTokenStore', 'caFile', 'debug', 'exposeResources', 'idleTimeout', 'inheritEnv', 'lifecycle', 'protocolVersion', 'requestHeadersCommand', 'requestTimeoutMs', 'searchKeywords', 'socket', 'tasks', 'toolPrefix', 'trace']);

export const piExposures = ['codemode', 'codemode-deferred', 'deferred', 'direct', 'hidden'];

/** Mirrors validatePiBuiltinOptions: Pi 1.0 trusts oauth.authServerMetadataUrl, so it must be https except on loopback. */
const metadataURL = (value: unknown) => {
  if (typeof value !== 'string') return false;
  try {
    const u = new URL(value);
    const loopback = ['localhost', '127.0.0.1', '[::1]'].includes(u.hostname);
    return Boolean(u.host) && !u.username && !u.password && (u.protocol === 'https:' || (u.protocol === 'http:' && loopback));
  } catch {
    return false;
  }
};

/**
 * piOptions as typed. An empty box sets nothing; `invalid` is text that is not a JSON object, `taken` a field Skillshare writes,
 * `adapter`/`adapterTools` a pi-mcp-adapter field, and `overlap` Pi's toolExposure while the server has a tool policy (`tools`), which writes it.
 */
export const parsePiOptions = (text: string, tools = false): { value?: Record<string, unknown>; invalid?: true; taken?: string; bad?: string; adapter?: string; adapterTools?: string; overlap?: string } => {
  if (!text.trim()) return {};
  try {
    const value: unknown = JSON.parse(text);
    if (!value || typeof value !== 'object' || Array.isArray(value)) return { invalid: true };
    const keys = Object.keys(value);
    const taken = keys.find((key) => piOwnFields.has(key));
    if (taken) return { taken };
    const adapterTools = keys.find((key) => adapterToolFields.has(key));
    if (adapterTools) return { adapterTools };
    const options = value as Record<string, unknown>;
    const object = (v: unknown): v is Record<string, unknown> => Boolean(v) && typeof v === 'object' && !Array.isArray(v);
    const adapter = keys.find((key) => adapterFields.has(key) && !(key === 'auth' && object(options.auth)));
    if (adapter) return { adapter };
    const overlap = tools ? keys.find((key) => key === 'toolExposure') : undefined;
    if (overlap) return { overlap };
    if ('exposure' in options && !piExposures.includes(options.exposure as string)) return { bad: 'exposure' };
    if ('toolExposure' in options && (!object(options.toolExposure) || Object.values(options.toolExposure).some((v) => !piExposures.includes(v as string)))) return { bad: 'toolExposure' };
    if ('timeout' in options && (typeof options.timeout !== 'number' || options.timeout <= 0)) return { bad: 'timeout' };
    if ('cwd' in options && typeof options.cwd !== 'string') return { bad: 'cwd' };
    if ('enabled' in options && typeof options.enabled !== 'boolean') return { bad: 'enabled' };
    if ('oauth' in options) {
      if (!object(options.oauth)) return { bad: 'oauth' };
      for (const key of ['clientId', 'clientSecret', 'callbackUrl', 'scope', 'clientName']) {
        if (key in options.oauth && typeof options.oauth[key] !== 'string') return { bad: `oauth.${key}` };
      }
      const port = options.oauth.callbackPort;
      if (port !== undefined && (!Number.isInteger(port) || Number(port) < 1 || Number(port) > 65535)) return { bad: 'oauth.callbackPort' };
      if ('authServerMetadataUrl' in options.oauth && !metadataURL(options.oauth.authServerMetadataUrl)) return { bad: 'oauth.authServerMetadataUrl' };
    }
    if ('auth' in options && (!object(options.auth) || typeof options.auth.provider !== 'string' || !options.auth.provider)) return { bad: 'auth.provider' };
    return { value: options };
  } catch {
    return { invalid: true };
  }
};

/** Mirrors mcp.toolPattern: a tool name in which * matches any characters. */
export const toolNamePattern = /^[^\s,?[\]{}]+$/;

const matchTool = (pattern: string, tool: string) => new RegExp(`^${pattern.replace(/[.+?^${}()|[\]\\]/g, '\\$&').replace(/\*/g, '.*')}$`).test(tool);

/** Mirrors mcp.ToolPolicy.validate: Deny removes every tool Allow keeps, which the server refuses to save. */
export const denyRemovesAll = (tools: MCPToolPolicy) =>
  Boolean(tools.allow?.length) && tools.allow!.every((tool) => !tool.includes('*') && (tools.deny ?? []).some((pattern) => matchTool(pattern, tool)));

/** Whether a tool reaches the model: Allow is empty or keeps it, and Deny does not remove it. */
export const toolAllowed = (tools: MCPToolPolicy, tool: string) =>
  (!tools.allow?.length || tools.allow.some((pattern) => matchTool(pattern, tool))) && !(tools.deny ?? []).some((pattern) => matchTool(pattern, tool));

/** The Deny rule other than the tool's own name that removes it, which ticking the tool cannot undo. */
export const toolBlockedBy = (tools: MCPToolPolicy, tool: string) => (tools.deny ?? []).find((pattern) => pattern !== tool && matchTool(pattern, tool));

/**
 * Ticks or unticks one tool by its exact name. Unticking denies it; ticking drops its own Deny entry and,
 * when a non-empty Allow still leaves it out, allows it by name.
 */
export const setToolChecked = (tools: MCPToolPolicy, tool: string, on: boolean): MCPToolPolicy => {
  const deny = tools.deny ?? [];
  if (!on) return deny.includes(tool) ? tools : { ...tools, deny: [...deny, tool] };
  const next = { ...tools, deny: deny.filter((name) => name !== tool) };
  if (next.allow?.length && !next.allow.some((pattern) => matchTool(pattern, tool))) next.allow = [...next.allow, tool];
  return next;
};

/** Mirrors mcp.toolPolicyGaps: Codex and Copilot list exact tool names, so they hold part of a policy; another Agent with a gap holds none of it. */
const namedToolClients = new Set(['codex', 'copilot']);

/** What each selected Agent does with a tool policy: follows all of it, part of it (with the parts it drops), or none. */
export const toolOutcomes = (rendered: { target: string; toolGaps?: string[] }[]) => {
  const full: string[] = [];
  const partial: { target: string; gaps: string[] }[] = [];
  const none: string[] = [];
  for (const { target, toolGaps } of rendered) {
    const gaps = toolGaps ?? [];
    if (!gaps.length) full.push(target);
    else if (namedToolClients.has(mcpClient(target))) partial.push({ target, gaps });
    else none.push(target);
  }
  return { full, partial, none };
};

/** An example rule for the server's tools: their shared prefix, such as clickup_, before delete_*. */
export const toolRuleExample = (names: string[]) => {
  let prefix = names[0] ?? '';
  for (const name of names) while (!name.startsWith(prefix)) prefix = prefix.slice(0, -1);
  const cut = Math.max(prefix.lastIndexOf('_'), prefix.lastIndexOf('-'));
  return `${names.length > 1 && cut > 0 ? prefix.slice(0, cut + 1) : ''}delete_*`;
};

/** The Allow and Deny entries a loaded tool row does not show: patterns, and names the server does not list. */
export const toolRules = (tools: MCPToolPolicy, names: string[]) => ({
  allow: (tools.allow ?? []).filter((entry) => !names.includes(entry)),
  deny: (tools.deny ?? []).filter((entry) => !names.includes(entry)),
});

/** Whether a policy says anything; an empty one is no policy. */
export const hasToolPolicy = (tools?: MCPToolPolicy): tools is MCPToolPolicy => Boolean(tools?.allow?.length || tools?.deny?.length);

/** The policy without its empty parts, or undefined when nothing is left. */
export const cleanToolPolicy = (tools?: MCPToolPolicy): MCPToolPolicy | undefined =>
  hasToolPolicy(tools) ? { ...(tools.allow?.length && { allow: tools.allow }), ...(tools.deny?.length && { deny: tools.deny }) } : undefined;

/** A policy in plain words, e.g. "Only 3 allowed, 1 excluded". */
export const toolSummary = (t: (key: string, params?: Record<string, string | number>) => string, tools: MCPToolPolicy) => {
  const allow = tools.allow?.length ?? 0;
  const deny = tools.deny?.length ?? 0;
  if (allow && deny) return t('mcp.tools.summaryBoth', { allow, deny });
  const [key, count] = allow ? ['mcp.tools.summaryAllow', allow] as const : ['mcp.tools.summaryDeny', deny] as const;
  return t(`${key}.${count === 1 ? 'one' : 'other'}`, { count });
};

/** The parts of a tool policy an Agent does not apply, as the backend names them: allow, deny, allow patterns, deny patterns. */
export const toolGapKey = (gap: string) => `mcp.tools.gap.${gap.replace(' patterns', 'Patterns')}`;

/** Just past the "(" of the last top-level pair of parentheses, or -1. */
const topLevelGroupStart = (text: string) => {
  let depth = 0;
  for (let i = text.length - 1; i >= 0; i--) {
    if (text[i] === ')') depth++;
    else if (text[i] === '(' && --depth === 0) return i + 1;
  }
  return -1;
};

const toolNoticePrefix = 'tool policy not applied for ';

/** A plan notice or check finding about a tool policy, "tool policy not applied for codex: allow patterns (docs, wiki)", taken apart. */
export const parseToolNotice = (message: string) => {
  if (!message.startsWith(toolNoticePrefix)) return undefined;
  const rest = message.slice(toolNoticePrefix.length);
  const colon = rest.indexOf(': ');
  if (colon < 0) return undefined;
  // The servers are in the last parentheses; a project server's root is nested in its own.
  const open = rest.endsWith(')') ? topLevelGroupStart(rest) : -1;
  const names = open > 0 ? rest.slice(open, -1) : '';
  const end = open > 0 ? open - 2 : rest.length;
  return { target: rest.slice(0, colon), gaps: rest.slice(colon + 2, end).split(', '), names: names ? names.split(', ') : [] };
};

/** A synced Claude entry that a local-scope server of the same name hides in this project. */
export const isShadowed = (change: MCPChange) => change.action !== 'conflict' && Boolean(change.message?.startsWith(shadowMessage));

/**
 * Conflicts the user can settle by importing the Agent entry or replacing it with the
 * source. A live owning config is not one of them: it has to release the entry itself,
 * so a button here would do nothing. One that was removed never can, hence conflictOrphaned.
 */
export const isResolvable = (change: MCPChange) =>
  change.action === 'conflict' &&
  ['mcp.conflictChanged', 'mcp.conflictUnmanaged', 'mcp.conflictOrphaned', 'mcp.conflictPiOverride'].includes(conflictKeys[conflictPrefix(change.message ?? '')]);

/** Whether importing can settle a conflict. It reads the Agent's global file, which never holds a project's switch,
 * and Pi's project override has no server to import. */
export const canImportConflict = (change: MCPChange) =>
  isResolvable(change) && !change.switch && conflictKeys[conflictPrefix(change.message ?? '')] !== 'mcp.conflictPiOverride';

/** Agents a project can turn a global server off for: those it uses, that the server reaches and that have a switch.
 * Pi takes one when given the global server itself, whose command or url its switch carries. */
export const switchTargets = (server: MCPServer, defaults: string[], projectTargets: readonly string[]) =>
  (server.targets ?? defaults).filter((x) => (mcpOffTargets.includes(x) || (x === 'pi' && Boolean(server.command || server.url))) && projectTargets.includes(x));

/** The Agents a server of one scope goes to. A switch that names none follows the scope's, where the Agent has a switch, as sync works it out. */
export const reachOf = (server: MCPServer, defaults: string[]) =>
  server.disabled ? switchTargets(server, defaults, server.targets ?? defaults) : server.targets ?? defaults;

/** Display names for the MCP clients, as in their own docs. */
export const targetLabel = (target: string) =>
  ({ pi: 'Pi', claude: 'Claude', codex: 'Codex', cursor: 'Cursor', vscode: 'VS Code', opencode: 'OpenCode', kilocode: 'Kilo Code', grok: 'Grok', antigravity: 'Antigravity', amp: 'Amp', 'claude-desktop': 'Claude Desktop', cline: 'Cline', commandcode: 'Command Code', copilot: 'Copilot CLI', droid: 'Droid', factory: 'Factory', gemini: 'Gemini CLI', goose: 'Goose', junie: 'Junie', kiro: 'Kiro', lmstudio: 'LM Studio', warp: 'Warp', windsurf: 'Windsurf', zed: 'Zed' })[target] ?? target;

/** Splits a command line into words, honouring single and double quotes. */
// ponytail: no backslash escapes; a word holding both quote kinds needs the YAML config.
export function splitCommand(line: string): string[] {
  const words: string[] = [];
  let word = '';
  let quote = '';
  let started = false;
  for (const ch of line) {
    if (quote) {
      if (ch === quote) quote = '';
      else word += ch;
    } else if (ch === '"' || ch === "'") {
      quote = ch;
      started = true;
    } else if (/\s/.test(ch)) {
      if (started) words.push(word);
      word = '';
      started = false;
    } else {
      word += ch;
      started = true;
    }
  }
  if (started) words.push(word);
  return words;
}

export const joinCommand = (words: string[]) =>
  words.map(w => (w === '' || /[\s"']/.test(w) ? (w.includes("'") ? `"${w}"` : `'${w}'`) : w)).join(' ');

export const describeEndpoint = (server: MCPServer) => server.url ?? joinCommand([server.command ?? '', ...(server.args ?? [])]);

/** Backup IDs start with the Unix time in nanoseconds. */
export const backupTime = (id: string) => new Date(Number(id.split('-')[0]) / 1e6);

// The plan's notices come from internal/mcp worded in English; each known one maps to a key, and any
// other is shown as sent. A legacy notice ends in ": <servers>", a tool policy one in " (<servers>)".
const noticeKeys: Record<string, string> = {
  "Pi's built-in MCP needs Pi 0.99.0 or later; on older Pi these servers stop loading until Pi is updated. If pi-mcp-adapter or pi-mcp-extension is still installed in Pi, remove it, because it can take the place of Pi's built-in MCP": 'mcp.notice.piBuiltin',
};
const legacyNoticeKeys: Record<string, string> = {
  'Pi now uses its built-in MCP; the next sync updates the config': 'mcp.notice.piExtension',
  'piOptionsPrune is no longer used; the next sync removes it': 'mcp.notice.piOptionsPrune',
  'directTools, includeTools and excludeTools become tool settings; the next sync converts them': 'mcp.notice.piTools',
  'directTools, includeTools or excludeTools the server already covers, or that are not tool lists, are dropped; the next sync removes them': 'mcp.notice.piToolsDropped',
};
const ADAPTER_OPTIONS = /^Pi's built-in MCP does not read (.+); the next sync removes them: (.+)$/;
export const mcpNotice = (t: (key: string, params?: Record<string, string>) => string, notice: string) => {
  if (noticeKeys[notice]) return t(noticeKeys[notice]);
  const cut = notice.lastIndexOf(': ');
  const key = cut > 0 ? legacyNoticeKeys[notice.slice(0, cut)] : undefined;
  if (key) return t(key, { names: notice.slice(cut + 2) });
  const adapter = ADAPTER_OPTIONS.exec(notice);
  if (adapter) return t('mcp.notice.adapterOptions', { fields: adapter[1], names: adapter[2] });
  const policy = parseToolNotice(notice);
  if (!policy) return notice;
  const parts = policy.gaps.map((gap) => t(toolGapKey(gap))).join(t('mcp.tools.partSeparator'));
  return t('mcp.notice.toolPolicy', { agent: targetLabel(policy.target), parts, names: policy.names.join(', ') });
};
