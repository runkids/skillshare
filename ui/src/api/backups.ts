import { apiFetch, kindQuery } from './http';
import type { BackupListResponse, FileBackup, FileBackupVersions, RestoreValidateResponse, TrashListResponse } from './types/backups';

export const backupsApi = {
  listBackups: () => apiFetch<BackupListResponse>('/backups'),
  createBackup: (target?: string) =>
    apiFetch<{ success: boolean; backedUpTargets: string[] }>('/backup', {
      method: 'POST',
      body: JSON.stringify({ target: target ?? '' }),
    }),
  cleanupBackups: () =>
    apiFetch<{ success: boolean; removed: number }>('/backup/cleanup', { method: 'POST' }),
  restore: (opts: { timestamp: string; target: string; force?: boolean }) =>
    apiFetch<{ success: boolean; target: string; timestamp: string }>('/restore', {
      method: 'POST',
      body: JSON.stringify(opts),
    }),
  validateRestore: (opts: { timestamp: string; target: string }) =>
    apiFetch<RestoreValidateResponse>('/restore/validate', {
      method: 'POST',
      body: JSON.stringify(opts),
    }),
  deleteAllBackups: () =>
    apiFetch<{ success: boolean; removed: number }>('/backups', { method: 'DELETE' }),
  deleteBackup: (timestamp: string) =>
    apiFetch<{ success: boolean }>(`/backups/${encodeURIComponent(timestamp)}`, { method: 'DELETE' }),
  // File history: earlier versions of single files skillshare rewrote
  listFileBackups: () => apiFetch<{ files: FileBackup[] }>('/file-backups'),
  getFileBackupVersions: (path: string) =>
    apiFetch<FileBackupVersions>(`/file-backups/versions?path=${encodeURIComponent(path)}`),
  getFileBackupVersion: (path: string, id: string) =>
    apiFetch<{ content: string; current: string }>(`/file-backups/version?path=${encodeURIComponent(path)}&id=${encodeURIComponent(id)}`),
  restoreFileBackup: (body: { path: string; id: string; unlink?: boolean }) =>
    apiFetch<{ success: boolean; backup_id?: string }>('/file-backups/restore', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  listTrash: () => apiFetch<TrashListResponse>('/trash'),
  restoreTrash: (name: string, kind?: 'skill' | 'agent') =>
    apiFetch<{ success: boolean }>(
      `/trash/${encodeURIComponent(name)}/restore${kindQuery(kind)}`,
      { method: 'POST' },
    ),
  deleteTrash: (name: string, kind?: 'skill' | 'agent') =>
    apiFetch<{ success: boolean }>(
      `/trash/${encodeURIComponent(name)}${kindQuery(kind)}`,
      { method: 'DELETE' },
    ),
  emptyTrash: (kind: 'skill' | 'agent' | 'all' = 'all') =>
    apiFetch<{ success: boolean; removed: number }>(
      `/trash/empty${kindQuery(kind)}`,
      { method: 'POST' },
    ),
};
