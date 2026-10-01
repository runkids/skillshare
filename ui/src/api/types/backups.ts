// Trash types
export interface TrashedSkill {
  name: string;
  kind?: 'skill' | 'agent';
  timestamp: string;
  date: string;
  size: number;
  path: string;
}

export interface TrashListResponse {
  items: TrashedSkill[];
  totalSize: number;
}

// Backup types
export interface BackupInfo {
  timestamp: string;
  path: string;
  targets: string[];
  /** Each snapshot folder's size and file count, in `targets` order. */
  entries?: { name: string; sizeBytes: number; files: number }[];
  date: string;
  sizeBytes: number;
}

/** Limits applied after each sync and by Clean up; 0 means no limit. */
export interface BackupRetention {
  maxAgeDays: number;
  maxCount: number;
  maxSizeMB: number;
}

export interface BackupListResponse {
  backups: BackupInfo[];
  totalSizeBytes: number;
  retention: BackupRetention;
}

/** A file with earlier versions kept. `target`/`extra` name who uses the path, when known. */
export interface FileBackup {
  path: string;
  versions: number;
  latest: string;
  target?: string;
  extra?: string;
  /** The shared file itself, not a place it is put. */
  source?: boolean;
}

/** `history`: saved before a write; `drift`: edits a write replaced; `origin`: the file before it was first attached. */
export interface FileBackupVersion {
  id: string;
  kind: 'history' | 'drift' | 'origin';
  reason: string;
  time: string;
  size: number;
  preview: string;
  /** The path was a link to this destination. */
  link_to?: string;
  /** There was no file. */
  none?: boolean;
}

export interface FileBackupVersions {
  path: string;
  current: { exists: boolean; link_to?: string };
  versions: FileBackupVersion[];
}

export interface RestoreValidateResponse {
  valid: boolean;
  error: string;
  conflicts: string[];
  backupSizeBytes: number;
  currentIsSymlink: boolean;
}
