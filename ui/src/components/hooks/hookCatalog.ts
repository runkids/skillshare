import type { HookAgentCatalog, HookBinding, HookChange } from '../../api/hooks';
import { emptyBinding, eventsToRows, isCodeAgent, newRow, parseNative, rowBlank } from './hooksView';
import type { BindingDraft, HookRow } from './hooksView';

/** GET /api/hooks/catalog: each command target's documented events. Code targets are not listed. */
export type HookCatalog = Record<string, HookAgentCatalog>;

const lower = (s: string) => s.toLowerCase();

export const catalogEvent = (catalog: HookCatalog | undefined, agent: string, event: string) =>
  catalog?.[agent]?.events.find((e) => e.name === event);

/** Whether an event takes a matcher. An event the catalog does not list keeps the field. */
export const eventHasMatcher = (catalog: HookCatalog | undefined, agent: string, event: string) =>
  catalogEvent(catalog, agent, event)?.matcher ?? true;

/** An event's description in the dashboard's language; the catalog's English one when there is no translation. */
export const eventDescription = (t: (key: string, params?: undefined, fallback?: string) => string, name: string, fallback: string) =>
  t(`hooks.events.${name.toLowerCase()}`, undefined, fallback);
export const timeoutUnit = (catalog: HookCatalog | undefined, agent: string) => catalog?.[agent]?.timeoutUnit ?? 'seconds';

// ---- Copying between targets ------------------------------------------------------------------

/**
 * Events that mean the same moment in each target's own spelling. Consulted only when the target
 * has no event of the same name; a target missing from a row has no such moment.
 */
const EQUIVALENTS: Record<string, string>[] = [
  { claude: 'PreToolUse', codex: 'PreToolUse', gemini: 'BeforeTool', copilot: 'preToolUse', cursor: 'preToolUse', droid: 'PreToolUse', qwen: 'PreToolUse', antigravity: 'PreToolUse' },
  { claude: 'PostToolUse', codex: 'PostToolUse', gemini: 'AfterTool', copilot: 'postToolUse', cursor: 'postToolUse', droid: 'PostToolUse', qwen: 'PostToolUse', antigravity: 'PostToolUse' },
  { claude: 'UserPromptSubmit', codex: 'UserPromptSubmit', gemini: 'BeforeAgent', copilot: 'userPromptSubmitted', cursor: 'beforeSubmitPrompt', droid: 'UserPromptSubmit', qwen: 'UserPromptSubmit' },
  { claude: 'Stop', codex: 'Stop', gemini: 'AfterAgent', copilot: 'agentStop', cursor: 'stop', droid: 'Stop', qwen: 'Stop', antigravity: 'Stop' },
  { claude: 'SessionStart', codex: 'SessionStart', gemini: 'SessionStart', copilot: 'sessionStart', cursor: 'sessionStart', droid: 'SessionStart', qwen: 'SessionStart' },
  { claude: 'SessionEnd', codex: 'SessionEnd', gemini: 'SessionEnd', copilot: 'sessionEnd', cursor: 'sessionEnd', droid: 'SessionEnd', qwen: 'SessionEnd' },
  { claude: 'SubagentStop', codex: 'SubagentStop', copilot: 'subagentStop', cursor: 'subagentStop', droid: 'SubagentStop', qwen: 'SubagentStop' },
  { claude: 'PreCompact', codex: 'PreCompact', gemini: 'PreCompress', copilot: 'preCompact', cursor: 'preCompact', droid: 'PreCompact', qwen: 'PreCompact' },
  { claude: 'Notification', gemini: 'Notification', copilot: 'notification', droid: 'Notification', qwen: 'Notification' },
];

/** `same`: the target has this event; `closest`: another name for the same moment; `none`: nothing close. */
export interface EventMatch { event: string; kind: 'same' | 'closest' | 'none' }

export function matchEvent(catalog: HookCatalog | undefined, to: string, event: string): EventMatch {
  const events = catalog?.[to]?.events;
  if (!events) return { event, kind: 'same' };
  const same = events.find((e) => lower(e.name) === lower(event));
  if (same) return { event: same.name, kind: 'same' };
  const row = EQUIVALENTS.find((r) => Object.values(r).some((name) => lower(name) === lower(event)));
  const closest = row?.[to] && events.find((e) => e.name === row[to]);
  return closest ? { event: closest.name, kind: 'closest' } : { event: '', kind: 'none' };
}

/** A timeout in one unit written in another; blank stays blank. */
export function convertTimeout(value: string, from: string, to: string) {
  const n = Number(value.trim());
  if (!value.trim() || from === to || !Number.isFinite(n)) return value;
  return String(to === 'milliseconds' ? n * 1000 : Math.max(1, Math.round(n / 1000)));
}

/** The rows a command draft holds, from whichever editor is current. Null when they do not fit rows. */
export function draftRows(draft: BindingDraft): HookRow[] | null {
  if (draft.mode === 'simple') return draft.rows.filter((r) => !rowBlank(r));
  const parsed = parseNative(draft.native);
  return parsed.error ? null : eventsToRows(parsed.value);
}

export interface CopyNote { from: string; to: string; kind: EventMatch['kind'] }

/**
 * One target's draft rewritten for another: events through the catalog, the target's own field
 * names and shape, matchers only where the event takes one, timeouts in the target's unit.
 */
export function copyDraft(catalog: HookCatalog | undefined, from: string, to: string, source: BindingDraft): { draft: BindingDraft; notes: CopyNote[] } {
  const files = source.files.map((f) => ({ ...f, id: `${f.id}-${to}` }));
  const rows = draftRows(source);
  if (!rows) return { draft: { ...emptyBinding(to), mode: 'native', native: source.native, files }, notes: [] };
  const notes: CopyNote[] = [];
  const copied = rows.map((r) => {
    const match = matchEvent(catalog, to, r.event);
    if (match.kind !== 'same' && !notes.some((n) => n.from === r.event)) notes.push({ from: r.event, to: match.event, kind: match.kind });
    return {
      ...newRow(to, match.event),
      matcher: match.event && eventHasMatcher(catalog, to, match.event) ? r.matcher : '',
      command: r.command,
      timeout: convertTimeout(r.timeout, timeoutUnit(catalog, from), timeoutUnit(catalog, to)),
    };
  });
  return { draft: { ...emptyBinding(to), rows: copied.length > 0 ? copied : [newRow(to)], files }, notes };
}

/** Whether a draft holds anything yet: an untouched tab is offered copy or start-from-scratch. */
export const draftEmpty = (agent: string, d: BindingDraft) =>
  isCodeAgent(agent) ? !d.code.trim() && d.files.length === 0 : d.files.length === 0 && (d.mode === 'native' ? !d.native.trim() : d.rows.every(rowBlank));

// ---- Per-target editing aids ------------------------------------------------------------------

/** A command placeholder in the target's own style; only documented path variables are used. */
export const commandPlaceholder = (agent: string) =>
  ({ claude: '$CLAUDE_PROJECT_DIR/.claude/hooks/check.sh', gemini: '$GEMINI_PROJECT_DIR/.gemini/hooks/check.sh', droid: '$FACTORY_PROJECT_DIR/.factory/hooks/check.sh' })[agent] ?? '~/bin/check.sh';

/** Environment variables a target's official hooks reference documents for commands. Others get none. */
export const commandVariables = (agent: string): string[] =>
  ({ claude: ['$CLAUDE_PROJECT_DIR'], gemini: ['$GEMINI_PROJECT_DIR'], droid: ['$FACTORY_PROJECT_DIR'] })[agent] ?? [];

/** A starting point for a code target's hook file, in that target's own API. */
export const codeTemplate = (agent: string) =>
  ({
    pi: `import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

export default function (pi: ExtensionAPI) {
  pi.on("tool_call", async (event, ctx) => {
    // Inspect event and return { block: true, reason } to stop the call.
  });
}
`,
    amp: `export default function (amp) {
  amp.on("tool.call", async (event) => {
    // Return { action: "allow" } to let the call run.
    return { action: "allow" };
  });
}
`,
    opencode: `import type { Plugin } from "@opencode-ai/plugin";

export const SkillsharePlugin: Plugin = async ({ project, client, $, directory }) => {
  return {
    "tool.execute.before": async (input, output) => {
      // Throw an error to stop the tool call.
    },
  };
};
`,
  })[agent] ?? '';

/** Native field names offered inside an event's entries. */
export const fieldNames = (agent: string) =>
  agent === 'copilot' ? ['type', 'bash', 'powershell', 'timeoutSec']
    : agent === 'cursor' ? ['command', 'timeout', 'matcher']
      : agent === 'antigravity' ? ['matcher', 'hooks', 'command', 'timeout']
        : ['matcher', 'hooks', 'type', 'command', 'timeout'];

// ---- JSON: exact error positions, properties, completion context ------------------------------

export type JsonErrorKind = 'end' | 'comma' | 'key' | 'colon' | 'value' | 'string' | 'trailing';
export interface JsonProp { key: string; from: number; to: number; depth: number; valueFrom: number; valueType: 'object' | 'array' | 'string' | 'number' | 'other' }
export interface JsonScan { error?: { at: number; kind: JsonErrorKind }; props: JsonProp[]; rootType?: JsonProp['valueType'] }

/** A strict JSON reader that reports where and why it stops, and every object property it read. */
export function scanJson(text: string): JsonScan {
  const props: JsonProp[] = [];
  let i = 0;
  const fail = (kind: JsonErrorKind): never => { throw { at: Math.min(i, text.length), kind: i >= text.length ? 'end' : kind }; };
  const ws = () => { while (i < text.length && ' \t\n\r'.includes(text[i])) i++; };
  const str = () => {
    i++;
    while (i < text.length && text[i] !== '"') {
      if (text[i] === '\n') fail('string');
      i += text[i] === '\\' ? 2 : 1;
    }
    if (i >= text.length) fail('string');
    i++;
  };
  const value = (depth: number): JsonProp['valueType'] => {
    ws();
    const c = text[i];
    if (c === '{') {
      i++; ws();
      if (text[i] === '}') { i++; return 'object'; }
      for (;;) {
        ws();
        if (text[i] !== '"') fail('key');
        const from = i;
        str();
        const to = i;
        ws();
        if (text[i] !== ':') fail('colon');
        i++; ws();
        const prop: JsonProp = { key: JSON.parse(text.slice(from, to)) as string, from, to, depth, valueFrom: i, valueType: 'other' };
        props.push(prop);
        prop.valueType = value(depth + 1);
        ws();
        if (text[i] === ',') { i++; continue; }
        if (text[i] === '}') { i++; return 'object'; }
        fail('comma');
      }
    }
    if (c === '[') {
      i++; ws();
      if (text[i] === ']') { i++; return 'array'; }
      for (;;) {
        value(depth + 1);
        ws();
        if (text[i] === ',') { i++; continue; }
        if (text[i] === ']') { i++; return 'array'; }
        fail('comma');
      }
    }
    if (c === '"') { str(); return 'string'; }
    const m = /^-?(0|[1-9]\d*)(\.\d+)?([eE][+-]?\d+)?/.exec(text.slice(i));
    if (m && m[0]) { i += m[0].length; return 'number'; }
    for (const word of ['true', 'false', 'null']) if (text.startsWith(word, i)) { i += word.length; return 'other'; }
    return fail('value');
  };
  try {
    const rootType = value(1);
    ws();
    if (i < text.length) fail('trailing');
    return { props, rootType };
  } catch (e) {
    return { props, error: e as JsonScan['error'] };
  }
}

const TIMEOUT_KEYS = ['timeout', 'timeoutSec', 'timeout_ms', 'timeoutMs'];

function distance(a: string, b: string) {
  const d = Array.from({ length: a.length + 1 }, (_, i) => [i, ...Array<number>(b.length).fill(0)]);
  for (let j = 1; j <= b.length; j++) d[0][j] = j;
  for (let i = 1; i <= a.length; i++) for (let j = 1; j <= b.length; j++) d[i][j] = Math.min(d[i - 1][j] + 1, d[i][j - 1] + 1, d[i - 1][j - 1] + (a[i - 1] === b[j - 1] ? 0 : 1));
  return d[a.length][b.length];
}

/** The documented event closest to a misspelled one, if any is close enough to be a typo. */
export function suggestEvent(names: string[], event: string) {
  let best = '';
  let score = Infinity;
  for (const name of names) {
    const d = distance(lower(name), lower(event));
    if (d < score) { best = name; score = d; }
  }
  return score <= Math.max(2, Math.floor(event.length / 4)) ? best : '';
}

/** One lint finding; `key` is an i18n key under hooks.lint and `params` fill it. */
export interface HookDiagnostic { from: number; to: number; severity: 'error' | 'warning'; key: string; params?: Record<string, string> }

/** Checks a command target's native event map: JSON syntax, known event names and timeout types. */
export function lintNative(text: string, agent: string, catalog: HookCatalog | undefined): HookDiagnostic[] {
  if (!text.trim()) return [];
  const scan = scanJson(text);
  if (scan.error) {
    const { at, kind } = scan.error;
    return [{ from: at, to: Math.min(at + 1, text.length), severity: 'error', key: `hooks.lint.${kind}` }];
  }
  if (scan.rootType !== 'object') return [{ from: 0, to: text.length, severity: 'error', key: 'hooks.lint.notObject' }];
  const out: HookDiagnostic[] = [];
  const names = catalog?.[agent]?.events.map((e) => e.name);
  for (const p of scan.props) {
    if (p.depth === 1 && names && !names.includes(p.key)) {
      const suggestion = suggestEvent(names, p.key);
      out.push({ from: p.from, to: p.to, severity: 'warning', key: suggestion ? 'hooks.lint.unknownEventSuggest' : 'hooks.lint.unknownEvent', params: { event: p.key, suggestion } });
    }
    if (p.depth > 1 && TIMEOUT_KEYS.includes(p.key) && p.valueType !== 'number') {
      out.push({ from: p.from, to: p.to, severity: 'error', key: 'hooks.lint.timeoutType', params: { unit: timeoutUnit(catalog, agent) } });
    }
  }
  return out;
}

/** 1-based line of an offset, for messages that name the line. */
export const lineOf = (text: string, at: number) => text.slice(0, at).split('\n').length;

/** Pretty-prints a native JSON map; text that does not parse is returned unchanged. */
export function formatJson(text: string) {
  try {
    return `${JSON.stringify(JSON.parse(text), null, 2)}\n`;
  } catch {
    return text;
  }
}

/**
 * What can be completed at a cursor in partial JSON: a property name (`depth` 1 is an event name)
 * or the value of `key`. `from` is where the replaced text starts; `quoted` if it starts at a quote.
 */
export type CompletionContext = { kind: 'key'; depth: number; from: number; quoted: boolean } | { kind: 'value'; key: string; from: number; quoted: boolean };

export function completionContext(text: string, pos: number): CompletionContext | null {
  const stack: { obj: boolean; expectKey: boolean; key: string }[] = [];
  let str = -1; // start of the open string, or -1
  for (let i = 0; i < pos; i++) {
    const c = text[i];
    const top = stack[stack.length - 1];
    if (str >= 0) {
      if (c === '\\') i++;
      else if (c === '"') {
        if (top?.obj && top.expectKey) top.key = text.slice(str + 1, i);
        str = -1;
      } else if (c === '\n') str = -1;
      continue;
    }
    if (c === '"') str = i;
    else if (c === '{' || c === '[') stack.push({ obj: c === '{', expectKey: c === '{', key: '' });
    else if (c === '}' || c === ']') stack.pop();
    else if (c === ':' && top?.obj) top.expectKey = false;
    else if (c === ',' && top?.obj) { top.expectKey = true; top.key = ''; }
  }
  const top = stack[stack.length - 1];
  if (!top?.obj) return null;
  const depth = stack.length;
  if (str >= 0) return top.expectKey ? { kind: 'key', depth, from: str, quoted: true } : { kind: 'value', key: top.key, from: str, quoted: true };
  let from = pos;
  while (from > 0 && /[A-Za-z_]/.test(text[from - 1])) from--;
  return top.expectKey ? { kind: 'key', depth, from, quoted: false } : null;
}

// ---- Import names -------------------------------------------------------------------------------

const slugPart = (s: string) => s.toLowerCase().replace(/[^a-z0-9_.-]+/g, '-').replace(/^[-.]+|[-.]+$/g, '');

/**
 * An import's default name: the script's basename when the command runs a file by path (`~/.claude/guard.sh` → guard),
 * otherwise the CLI's candidate name, `<target>-<event>` (claude-stop). Numbered when taken.
 */
export function suggestName(candidate: string, command: string, taken: (name: string) => boolean) {
  const first = command.trim().split(/\s+/)[0] ?? '';
  const script = /[\\/]/.test(first) ? slugPart((first.split(/[\\/]/).pop() ?? '').replace(/\.[A-Za-z0-9]+$/, '')) : '';
  const stem = script.slice(0, 60) || candidate;
  let name = stem;
  for (let n = 2; taken(name); n++) name = `${stem}-${n}`;
  return name;
}

// ---- Diff ---------------------------------------------------------------------------------------

export interface DiffLine { op: ' ' | '+' | '-'; text: string }

// A line that only gained or lost a trailing comma or whitespace, because a neighbour was added or removed, is unchanged.
const sameLine = (line: string) => line.replace(/,?\s*$/, '');

/**
 * A line diff of two small files (LCS); files too large for that show as replaced. Lines align ignoring a
 * trailing comma, so an added sibling does not shift the rest, but a line whose text still differs shows
 * as removed and added. Indentation counts, so a closing bracket never pairs with a nested one. Within a
 * change, removed lines come first.
 */
export function diffLines(before: string, after: string): DiffLine[] {
  const oldLines = before ? before.replace(/\n$/, '').split('\n') : [];
  const b = after ? after.replace(/\n$/, '').split('\n') : [];
  const a = oldLines.map(sameLine);
  const keys = b.map(sameLine);
  if (a.length * b.length > 4_000_000) return [...oldLines.map((text) => ({ op: '-' as const, text })), ...b.map((text) => ({ op: '+' as const, text }))];
  const lcs = Array.from({ length: a.length + 1 }, () => new Uint32Array(b.length + 1));
  for (let i = a.length - 1; i >= 0; i--) for (let j = b.length - 1; j >= 0; j--) lcs[i][j] = a[i] === keys[j] ? lcs[i + 1][j + 1] + 1 : Math.max(lcs[i + 1][j], lcs[i][j + 1]);
  const out: DiffLine[] = [];
  let i = 0;
  let j = 0;
  while (i < a.length || j < b.length) {
    if (i < a.length && j < b.length && a[i] === keys[j]) {
      if (oldLines[i] === b[j]) out.push({ op: ' ', text: b[j] });
      else out.push({ op: '-', text: oldLines[i] }, { op: '+', text: b[j] });
      i++; j++;
    } else if (i < a.length && (j >= b.length || lcs[i + 1][j] >= lcs[i][j + 1])) out.push({ op: '-', text: oldLines[i++] });
    else out.push({ op: '+', text: b[j++] });
  }
  // A comma-only pair can land after an added line; each run of changes lists its removals first.
  for (let start = 0; start < out.length; start++) {
    if (out[start].op === ' ') continue;
    let end = start;
    while (end < out.length && out[end].op !== ' ') end++;
    const run = out.slice(start, end);
    out.splice(start, run.length, ...run.filter((l) => l.op === '-'), ...run.filter((l) => l.op === '+'));
    start = end;
  }
  return out;
}

/** Diff lines with long unchanged runs folded to `context` lines on each side; `skip` counts folded lines. */
export function foldDiff(lines: DiffLine[], context = 2): (DiffLine | { skip: number })[] {
  const near = lines.map((_, i) => lines.slice(Math.max(0, i - context), i + context + 1).some((l) => l.op !== ' '));
  const out: (DiffLine | { skip: number })[] = [];
  lines.forEach((line, i) => {
    if (near[i]) { out.push(line); return; }
    const last = out[out.length - 1];
    if (last && 'skip' in last) last.skip++;
    else out.push({ skip: 1 });
  });
  return out;
}

/** The event a diff line opens (`"Stop": [`), for marking a user's own hooks as untouched. */
export const lineEvent = (text: string) => /^\s*"([^"]+)"\s*:\s*\[/.exec(text)?.[1];

/** Summary lines of a binding: event, matcher and command of each registration. */
export function bindingLines(binding: HookBinding | undefined): { event: string; matcher: string; command: string }[] {
  const out: { event: string; matcher: string; command: string }[] = [];
  for (const command of Object.values(binding?.commands ?? {})) {
    for (const event of command.events) out.push({ event, matcher: '', command: command.command });
  }
  const isObj = (v: unknown): v is Record<string, unknown> => Boolean(v) && typeof v === 'object' && !Array.isArray(v);
  for (const [event, list] of Object.entries(binding?.events ?? {})) {
    for (const item of Array.isArray(list) ? list : []) {
      if (!isObj(item)) continue;
      const matcher = typeof item.matcher === 'string' ? item.matcher : '';
      const handlers = Array.isArray(item.hooks) ? item.hooks : [item];
      for (const h of handlers) {
        if (!isObj(h)) continue;
        const command = ['command', 'bash', 'powershell'].map((k) => h[k]).find((v) => typeof v === 'string') as string | undefined;
        out.push({ event, matcher, command: command ?? '' });
      }
    }
  }
  return out;
}

/** `+ Stop  − PreToolUse`: what a change does to each event of a shared file. */
export function eventLabels(change: Pick<HookChange, 'events'>) {
  const e = change.events;
  return [...(e?.added ?? []).map((x) => `+ ${x}`), ...(e?.updated ?? []).map((x) => `~ ${x}`), ...(e?.removed ?? []).map((x) => `− ${x}`)];
}
