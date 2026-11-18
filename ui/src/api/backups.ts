import { http, kindQuery } from './http';
import type {
  BackupListResponse,
  FileBackup,
  FileBackupVersions,
  RestoreValidateResponse,
  TrashListResponse,
} from './types/backups';

export const backupsApi = {
  listBackups: () => http.get<BackupListResponse>('/backups'),
  createBackup: (target?: string) =>
    http.post<{ success: boolean; backedUpTargets: string[] }>('/backup', { target: target ?? '' }),
  cleanupBackups: () => http.post<{ success: boolean; removed: number }>('/backup/cleanup'),
  restore: (opts: { timestamp: string; target: string; force?: boolean }) =>
    http.post<{ success: boolean; target: string; timestamp: string }>('/restore', opts),
  validateRestore: (opts: { timestamp: string; target: string }) =>
    http.post<RestoreValidateResponse>('/restore/validate', opts),
  deleteAllBackups: () => http.delete<{ success: boolean; removed: number }>('/backups'),
  deleteBackup: (timestamp: string) =>
    http.delete<{ success: boolean }>(`/backups/${encodeURIComponent(timestamp)}`),
  // File history: earlier versions of single files skillshare rewrote
  listFileBackups: () => http.get<{ files: FileBackup[] }>('/file-backups'),
  getFileBackupVersions: (path: string) =>
    http.get<FileBackupVersions>(`/file-backups/versions?path=${encodeURIComponent(path)}`),
  getFileBackupVersion: (path: string, id: string) =>
    http.get<{ content: string; current: string }>(
      `/file-backups/version?path=${encodeURIComponent(path)}&id=${encodeURIComponent(id)}`,
    ),
  restoreFileBackup: (body: { path: string; id: string; unlink?: boolean }) =>
    http.post<{ success: boolean; backup_id?: string }>('/file-backups/restore', body),
  listTrash: () => http.get<TrashListResponse>('/trash'),
  restoreTrash: (name: string, kind?: 'skill' | 'agent') =>
    http.post<{ success: boolean }>(`/trash/${encodeURIComponent(name)}/restore${kindQuery(kind)}`),
  deleteTrash: (name: string, kind?: 'skill' | 'agent') =>
    http.delete<{ success: boolean }>(`/trash/${encodeURIComponent(name)}${kindQuery(kind)}`),
  emptyTrash: (kind: 'skill' | 'agent' | 'all' = 'all') =>
    http.post<{ success: boolean; removed: number }>(`/trash/empty${kindQuery(kind)}`),
};
