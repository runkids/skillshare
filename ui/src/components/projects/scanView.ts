import type { ProjectScanRow, ProjectScanWarning } from '../../api/client';
import { baseName } from './projectView';

export interface ScanMessage {
  key: string;
  params?: Record<string, string>;
}

/** How many folders each result has, for the review summary and its add button. */
export function scanCounts(rows: ProjectScanRow[]) {
  const count = (result: ProjectScanRow['result']) =>
    rows.filter((r) => r.result === result).length;
  return { added: count('added'), kept: count('kept'), skipped: count('skipped') };
}

const SKIP: Record<string, string> = {
  different: 'projects.scan.skip.different',
  storage: 'projects.scan.skip.storage',
  removed: 'projects.scan.skip.removed',
  'own-config': 'projects.scan.skip.ownConfig',
  'name-taken': 'projects.scan.skip.nameTaken',
  'bad-name': 'projects.scan.skip.badName',
};

/** Skipped rows whose settings are changed on the project's own page. */
export const opensProject = (row: ProjectScanRow) =>
  row.skip === 'different' || row.skip === 'storage';

/** A checkout named from its path because its folder name is taken. */
export const renamed = (row: ProjectScanRow) =>
  row.result === 'added' && row.path !== '.' && row.name !== baseName(row.path);

/**
 * The line under a row. Null means no line, or, for a skipped row, a skip code
 * this dashboard does not know: show the server's reason then.
 */
export function scanNote(row: ProjectScanRow): ScanMessage | null {
  if (row.result === 'skipped') {
    const key = row.skip ? SKIP[row.skip] : undefined;
    return key ? { key, params: { name: row.name } } : null;
  }
  if (row.result === 'kept') return { key: 'projects.scan.kept' };
  if (row.path === '.') return { key: 'projects.scan.picked' };
  if (renamed(row)) return { key: 'projects.scan.renamed', params: { base: baseName(row.path) } };
  return null;
}

export function scanWarning(w: ProjectScanWarning): ScanMessage {
  return w.kind === 'missing'
    ? { key: 'projects.scan.missing', params: { folder: w.folder } }
    : { key: 'projects.scan.unreadable', params: { folder: w.folder, detail: w.detail ?? '' } };
}
