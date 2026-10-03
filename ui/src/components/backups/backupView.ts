import { ApiError } from '../../api/client';
import type { BackupRetention, FileBackupVersion } from '../../api/client';
import type { MCPBackup } from '../../api/mcp';
import type { useT } from '../../i18n';
import type { Locale } from '../../i18n/locales';
import { shortenHome } from '../../lib/paths';

/** One folder in a snapshot: an agents snapshot is stored under `<target>-agents`. */
export interface BackupItem { name: string; target: string; kind: 'skills' | 'agents' }

export function backupItem(name: string): BackupItem {
  return name.endsWith('-agents') ? { name, target: name.slice(0, -'-agents'.length), kind: 'agents' } : { name, target: name, kind: 'skills' };
}

/** `''` keeps everything, `agents` only agents folders, anything else one target's folders. */
export function filterItems(items: BackupItem[], filter: string): BackupItem[] {
  if (!filter) return items;
  return items.filter((i) => (filter === 'agents' ? i.kind === 'agents' : i.target === filter));
}

const HISTORY_REASONS = ['convert', 'shim', 'import', 'edit', 'delete', 'collect', 'attach', 'restore', 'migrate'];
const DRIFT_REASONS = ['overwrite', 'mode', 'restore'];

/** The i18n key of the sentence saying why a version was kept. */
export function reasonKey(v: FileBackupVersion): string {
  if (v.kind === 'origin') return v.none ? 'backup.files.origin.none' : v.link_to ? 'backup.files.origin.link' : 'backup.files.origin.content';
  if (v.kind === 'drift') return `backup.files.drift.${DRIFT_REASONS.includes(v.reason) ? v.reason : 'other'}`;
  return `backup.files.reason.${HISTORY_REASONS.includes(v.reason) ? v.reason : 'write'}`;
}

export const kindTone = { history: '', drift: 'warn', origin: 'ok' } as const;

/** Server names a backup's write changed, by change. */
export function mcpChanges(servers: MCPBackup['servers']) {
  const by = { added: [] as string[], changed: [] as string[], removed: [] as string[] };
  for (const s of servers ?? []) by[s.change]?.push(s.name);
  return (['added', 'changed', 'removed'] as const).filter((c) => by[c].length).map((change) => ({ change, names: by[change] }));
}

/** The message for a failed file restore, from the server's error code when it sent one. */
export function fileBackupErrorMessage(error: unknown, t: (key: string, params?: Record<string, string>, fallback?: string) => string): string {
  if (error instanceof ApiError && error.code?.startsWith('file_backup_')) {
    const params = Object.fromEntries(Object.entries(error.params ?? {}).map(([k, v]) => [k, typeof v === 'string' ? shortenHome(v) : String(v)]));
    return t(`backup.files.error.${error.code}`, params, error.message);
  }
  return (error as Error).message;
}

/** "30 days · 10 backups · 500 MB", for the footer and the cleanup confirmation. */
export function retentionSummary(t: ReturnType<typeof useT>, r: BackupRetention, locale: Locale) {
  const num = (n: number) => new Intl.NumberFormat(locale).format(n);
  return t('backup.retention.summary', {
    days: num(r.maxAgeDays),
    count: r.maxCount === 0 ? t('backup.retention.anyCount') : t('backup.retention.count', { count: num(r.maxCount) }),
    size: r.maxSizeMB === 0 ? t('backup.retention.anySize') : `${num(r.maxSizeMB)} MB`,
  });
}
