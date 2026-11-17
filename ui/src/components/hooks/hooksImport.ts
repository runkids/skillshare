import type { HookCandidate, HookNativeEdit } from '../../api/hooks';
import { bindingLines } from './hookCatalog';

export type HookEvents = Record<string, unknown>;
/** What the import dialog will write to a hook it has not imported: new events, or its removal. */
export type NativeChange = { remove: true } | { events: HookEvents };

/** A candidate as the dialog shows it: where it was read from and whether saving takes over its native registrations. */
export interface ImportRow { id: string; target: string; candidate: HookCandidate; adopt: boolean }

export const rowEvents = (row: ImportRow): HookEvents => row.candidate.entry.bindings[row.target]?.events ?? {};

/** The event of a one-event candidate; a named block or a whole file goes by its own name. */
export const rowTitle = (row: ImportRow) => {
  const events = Object.keys(rowEvents(row));
  return events.length === 1 && row.target !== 'antigravity' ? events[0] : row.candidate.name;
};

const SCRIPT = /(?:"?\$\{?\w+\}?"?|~|\.{1,2})?(?:\/[\w.@+-]+)+\.\w+/g;

/** The script a command runs, else its first line: what a collapsed row shows of a long command. */
export function commandSummary(command: string) {
  const scripts = command.match(SCRIPT);
  if (scripts) return scripts[scripts.length - 1].replace(/"/g, '');
  return command.split('\n').map((l) => l.trim()).find(Boolean) ?? '';
}

const stable = (v: unknown): unknown =>
  Array.isArray(v) ? v.map(stable)
    : v && typeof v === 'object' ? Object.fromEntries(Object.keys(v).sort().map((k) => [k, stable((v as Record<string, unknown>)[k])]))
      : v;

/** Equal event maps, whatever order their keys are written in. */
export const sameEvents = (a: HookEvents | undefined, b: HookEvents | undefined) => JSON.stringify(stable(a ?? {})) === JSON.stringify(stable(b ?? {}));

/** The edits to send: each changed hook's events as read, and what replaces them. */
export function nativeEdits(rows: { target: string; root?: string; before: HookEvents; change: NativeChange }[]): HookNativeEdit[] {
  return rows.map(({ target, root, before, change }) => ({ target, ...(root && { root }), before, ...('events' in change && { after: change.events }) }));
}

/** One line on what an edit changed, as a translation key: a single matcher or command when that is all, else just "edited". */
export function describeChange(before: HookEvents, after: HookEvents): { key: string; params?: Record<string, string> } {
  const was = bindingLines({ events: before });
  const now = bindingLines({ events: after });
  const changed = was.length === now.length ? [...now.keys()].filter((i) => JSON.stringify(was[i]) !== JSON.stringify(now[i])) : [];
  if (changed.length === 1) {
    const [i] = changed;
    const [a, b] = [was[i], now[i]];
    if (a.event === b.event && a.command === b.command) return { key: 'hooks.importChange.matcher', params: { from: a.matcher || '*', to: b.matcher || '*' } };
    if (a.event === b.event && a.matcher === b.matcher) return { key: 'hooks.importChange.command', params: { n: String(i + 1) } };
  }
  return { key: 'hooks.importChange.edited' };
}

/**
 * Splits a command where a statement ends outside quotes and brackets. Null for anything this does not
 * model exactly (heredocs, backticks, comments, an expansion inside double quotes, ANSI-C strings, an escaped space or
 * line break, case fall-through): a wrong split would change what the command runs.
 */
function statements(command: string): string[] | null {
  if (/<<|`/.test(command)) return null;
  const out: string[] = [];
  let current = '';
  let quote = '';
  let depth = 0;
  const end = () => { if (current.trim()) out.push(current.trim()); current = ''; };
  for (let i = 0; i < command.length; i++) {
    const c = command[i];
    const next = command[i + 1];
    if (c === '\\' && quote !== "'") {
      if (next === undefined || /\s/.test(next)) return null;
      current += c + next;
      i++;
      continue;
    }
    if (quote) {
      if (quote === '"' && c === '$' && (next === '(' || next === '{')) return null;
      if (c === quote) quote = '';
      current += c;
      continue;
    }
    if (c === '$' && (next === "'" || next === '"')) return null;
    if (c === '"' || c === "'") { quote = c; current += c; continue; }
    if (c === '#') return null;
    if (c === '(' || (c === '{' && command[i - 1] === '$')) depth++;
    if (c === ')' || (c === '}' && depth > 0 && !/\s/.test(command[i - 1] ?? ' '))) depth = Math.max(0, depth - 1);
    if (depth === 0 && c === '\n') { end(); continue; }
    if (depth === 0 && c === ';' && next === ';') {
      if (command[i + 2] === '&') return null;
      current += ';;';
      i++;
      continue;
    }
    if (depth === 0 && c === ';') {
      if (next === '&') return null;
      end();
      continue;
    }
    if (depth === 0 && (c === '&' || c === '|') && next === c) { current += c + c; i++; end(); continue; }
    current += c;
  }
  if (quote) return null;
  end();
  return out;
}

const OPENER = /^(then|do|else)(?=\s|$)\s*([\s\S]*)$/;

/**
 * A one-line shell command laid out one statement per line, indented by block. Only line breaks and
 * leading spaces change; a command this cannot split safely is returned as is.
 */
export function formatCommand(command: string) {
  const parts = statements(command);
  if (!parts) return command;
  const lines: string[] = [];
  let level = 0;
  let continued = false;
  const push = (text: string) => {
    lines.push('  '.repeat(Math.max(level, 0) + (continued ? 1 : 0)) + text);
    continued = /(&&|\|\|)$/.test(text);
  };
  for (let s of parts) {
    for (let opener = OPENER.exec(s); opener; opener = OPENER.exec(s)) {
      if (opener[1] === 'else') { level--; push('else'); } else if (lines.length > 0 && !continued) lines[lines.length - 1] += `; ${opener[1]}`; else push(opener[1]);
      level++;
      s = opener[2];
    }
    if (!s) continue;
    if (/^(fi|done|esac|elif|\})(?![\w-])/.test(s)) level--;
    push(s);
    if (/^case\b/.test(s) && !/\besac\b/.test(s)) level++;
    if (/(^|\s)\{$/.test(s)) level++;
  }
  return lines.join('\n');
}
