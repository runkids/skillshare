import type { MCPPlan, MCPServer } from '../../api/mcp';
import { formatDateTime, type Locale } from '../../i18n';

export type MCPChange = MCPPlan['changes'][number];

export interface MatrixRow {
  name: string;
  /** Undefined when the entry was removed from the source but is still in Agent files. */
  server?: MCPServer;
  cells: Record<string, MCPChange>;
}

export const statusVariant: Record<string, 'default' | 'success' | 'warning' | 'danger' | 'info'> = {
  add: 'info', update: 'info', restore: 'info', unchanged: 'success', conflict: 'warning', remove: 'danger',
};

/** One row per source server, plus rows for entries the plan will remove from Agents. */
export function buildMatrix(servers: Record<string, MCPServer>, plan: MCPPlan | null): MatrixRow[] {
  const rows = new Map<string, MatrixRow>(Object.entries(servers).map(([name, server]) => [name, { name, server, cells: {} }]));
  for (const change of plan?.changes ?? []) {
    const row = rows.get(change.name) ?? { name: change.name, cells: {} };
    row.cells[change.target] = change;
    rows.set(change.name, row);
  }
  return [...rows.values()];
}

export function countActions(changes: MCPChange[]): Record<string, number> {
  const counts: Record<string, number> = {};
  for (const change of changes) counts[change.action] = (counts[change.action] ?? 0) + 1;
  return counts;
}

export function groupByFile(changes: MCPChange[]) {
  const files = new Map<string, { target: string; path: string; changes: MCPChange[] }>();
  for (const change of changes) {
    const file = files.get(change.path) ?? { target: change.target, path: change.path, changes: [] };
    file.changes.push(change);
    files.set(change.path, file);
  }
  return [...files.values()];
}

// ponytail: keyed on the backend Change.Message text; add a reason code to mcp.Change if these start drifting.
const conflictKeys: Record<string, string> = {
  'managed by another Skillshare config': 'mcp.conflictOtherConfig',
  'Agent configuration changed; import it or explicitly replace this entry': 'mcp.conflictChanged',
  'existing entry is not managed; import it to explicitly adopt it': 'mcp.conflictUnmanaged',
  'entry changed after the backup; restore would overwrite newer changes': 'mcp.conflictAfterBackup',
};

export const describeMessage = (t: (key: string) => string, message = '') =>
  conflictKeys[message] ? t(conflictKeys[message]) : message;

/** Conflicts the user can settle by importing the Agent entry or replacing it with the source. */
export const isResolvable = (change: MCPChange) =>
  change.action === 'conflict' && ['mcp.conflictChanged', 'mcp.conflictUnmanaged'].includes(conflictKeys[change.message ?? '']);

export const describeEndpoint = (server: MCPServer) => server.url ?? [server.command, ...(server.args ?? [])].join(' ');

/** Credential references only; literal values are never shown. */
export function describeCredentials(server: MCPServer): string[] {
  const ref = (key: string, value: string | { fromEnv: string }) => typeof value === 'string' ? key : `${key} ← $${value.fromEnv}`;
  return [
    ...(server.bearerToken ? [`Bearer ← $${server.bearerToken.fromEnv}`] : []),
    ...Object.entries(server.headers ?? {}).map(([key, value]) => ref(key, value)),
    ...Object.entries(server.env ?? {}).map(([key, value]) => ref(key, value)),
  ];
}

/** Backup IDs start with the Unix time in nanoseconds. */
export const backupTime = (id: string) => new Date(Number(id.split('-')[0]) / 1e6);

export function groupBackupsByDay<T extends { id: string }>(backups: T[]) {
  const days = new Map<string, { date: Date; backups: T[] }>();
  for (const backup of backups) {
    const date = backupTime(backup.id);
    const day = days.get(date.toDateString()) ?? { date, backups: [] };
    day.backups.push(backup);
    days.set(date.toDateString(), day);
  }
  return [...days.values()];
}

export function dayLabel(date: Date, locale: Locale, now = new Date()) {
  const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
  const diff = Math.round((startOfDay(date) - startOfDay(now)) / 86_400_000);
  const day = formatDateTime(date, locale, { month: 'long', day: 'numeric' });
  return diff === 0 || diff === -1 ? `${new Intl.RelativeTimeFormat(locale, { numeric: 'auto' }).format(diff, 'day')} · ${day}` : day;
}
