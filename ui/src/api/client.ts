const BASE = '/api';

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(BASE + path, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  });
  const data = await res.json();
  if (!res.ok) {
    throw new ApiError(res.status, data.error ?? res.statusText);
  }
  return data as T;
}

// Typed API helpers
export const api = {
  // Overview
  getOverview: () => apiFetch<Overview>('/overview'),

  // Skills
  listSkills: () => apiFetch<{ skills: Skill[] }>('/skills'),
  getSkill: (name: string) =>
    apiFetch<{ skill: Skill; skillMdContent: string; files: string[] }>(`/skills/${encodeURIComponent(name)}`),
  deleteSkill: (name: string) =>
    apiFetch<{ success: boolean }>(`/skills/${encodeURIComponent(name)}`, { method: 'DELETE' }),

  // Targets
  listTargets: () => apiFetch<{ targets: Target[]; sourceSkillCount: number }>('/targets'),
  addTarget: (name: string, path: string) =>
    apiFetch<{ success: boolean }>('/targets', {
      method: 'POST',
      body: JSON.stringify({ name, path }),
    }),
  removeTarget: (name: string) =>
    apiFetch<{ success: boolean }>(`/targets/${encodeURIComponent(name)}`, { method: 'DELETE' }),

  // Sync
  sync: (opts: { dryRun?: boolean; force?: boolean }) =>
    apiFetch<{ results: SyncResult[] }>('/sync', {
      method: 'POST',
      body: JSON.stringify(opts),
    }),
  diff: (target?: string) =>
    apiFetch<{ diffs: DiffTarget[] }>(`/diff${target ? '?target=' + encodeURIComponent(target) : ''}`),

  // Search & Install
  search: (q: string, limit = 20) =>
    apiFetch<{ results: SearchResult[] }>(`/search?q=${encodeURIComponent(q)}&limit=${limit}`),
  check: () => apiFetch<CheckResult>('/check'),
  discover: (source: string) =>
    apiFetch<DiscoverResult>('/discover', {
      method: 'POST',
      body: JSON.stringify({ source }),
    }),
  install: (opts: { source: string; name?: string; force?: boolean; skipAudit?: boolean; track?: boolean; into?: string }) =>
    apiFetch<InstallResult>('/install', {
      method: 'POST',
      body: JSON.stringify(opts),
    }),
  installBatch: (opts: { source: string; skills: DiscoveredSkill[]; force?: boolean; skipAudit?: boolean; into?: string }) =>
    apiFetch<BatchInstallResult>('/install/batch', {
      method: 'POST',
      body: JSON.stringify(opts),
    }),

  // Update
  update: (opts: { name?: string; force?: boolean; all?: boolean }) =>
    apiFetch<{ results: UpdateResultItem[] }>('/update', {
      method: 'POST',
      body: JSON.stringify(opts),
    }),

  // Repo uninstall
  deleteRepo: (name: string) =>
    apiFetch<{ success: boolean; name: string }>(`/repos/${encodeURIComponent(name)}`, { method: 'DELETE' }),

  // Skill file content
  getSkillFile: (skillName: string, filepath: string) =>
    apiFetch<SkillFileContent>(`/skills/${encodeURIComponent(skillName)}/files/${filepath}`),

  // Collect
  collectScan: (target?: string) =>
    apiFetch<CollectScanResult>(`/collect/scan${target ? '?target=' + encodeURIComponent(target) : ''}`),
  collect: (opts: { skills: { name: string; targetName: string }[]; force?: boolean }) =>
    apiFetch<CollectResult>('/collect', {
      method: 'POST',
      body: JSON.stringify(opts),
    }),

  // Version check
  getVersionCheck: () => apiFetch<VersionCheck>('/version'),

  // Config
  getConfig: () => apiFetch<{ config: unknown; raw: string }>('/config'),
  putConfig: (raw: string) =>
    apiFetch<{ success: boolean }>('/config', {
      method: 'PUT',
      body: JSON.stringify({ raw }),
    }),
  availableTargets: () => apiFetch<{ targets: AvailableTarget[] }>('/config/available-targets'),

  // Backups
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

  // Trash
  listTrash: () => apiFetch<TrashListResponse>('/trash'),
  restoreTrash: (name: string) =>
    apiFetch<{ success: boolean }>(`/trash/${encodeURIComponent(name)}/restore`, { method: 'POST' }),
  deleteTrash: (name: string) =>
    apiFetch<{ success: boolean }>(`/trash/${encodeURIComponent(name)}`, { method: 'DELETE' }),
  emptyTrash: () =>
    apiFetch<{ success: boolean; removed: number }>('/trash/empty', { method: 'POST' }),

  // Log
  listLog: (type?: string, limit?: number, filters?: { cmd?: string; status?: string; since?: string }) => {
    const params = new URLSearchParams();
    params.set('type', type ?? 'ops');
    params.set('limit', String(limit ?? 100));
    if (filters?.cmd) params.set('cmd', filters.cmd);
    if (filters?.status) params.set('status', filters.status);
    if (filters?.since) params.set('since', filters.since);
    return apiFetch<LogListResponse>(`/log?${params.toString()}`);
  },
  clearLog: (type?: string) =>
    apiFetch<{ success: boolean }>(`/log?type=${type ?? 'ops'}`, { method: 'DELETE' }),

  // Audit
  auditAll: () => apiFetch<AuditAllResponse>('/audit'),
  auditSkill: (name: string) => apiFetch<AuditSkillResponse>(`/audit/${encodeURIComponent(name)}`),

  // Audit Rules
  getAuditRules: () => apiFetch<AuditRulesResponse>('/audit/rules'),
  putAuditRules: (raw: string) =>
    apiFetch<{ success: boolean }>('/audit/rules', {
      method: 'PUT',
      body: JSON.stringify({ raw }),
    }),
  initAuditRules: () =>
    apiFetch<{ success: boolean; path: string }>('/audit/rules', {
      method: 'POST',
    }),

  // Git
  gitStatus: () => apiFetch<GitStatus>('/git/status'),
  push: (opts: { message?: string; dryRun?: boolean }) =>
    apiFetch<PushResponse>('/push', {
      method: 'POST',
      body: JSON.stringify(opts),
    }),
  pull: (opts?: { dryRun?: boolean }) =>
    apiFetch<PullResponse>('/pull', {
      method: 'POST',
      body: JSON.stringify(opts ?? {}),
    }),
};

// Types
export interface TrackedRepo {
  name: string;
  skillCount: number;
  dirty: boolean;
}

export interface Overview {
  source: string;
  skillCount: number;
  topLevelCount: number;
  targetCount: number;
  mode: string;
  version: string;
  trackedRepos: TrackedRepo[];
  isProjectMode: boolean;
  projectRoot?: string;
}

export interface VersionCheck {
  cliVersion: string;
  cliLatest?: string;
  cliUpdateAvailable: boolean;
  skillVersion: string;
  skillLatest?: string;
  skillUpdateAvailable: boolean;
}

export interface Skill {
  name: string;
  flatName: string;
  relPath: string;
  sourcePath: string;
  isInRepo: boolean;
  installedAt?: string;
  source?: string;
  type?: string;
  repoUrl?: string;
  version?: string;
}

export interface Target {
  name: string;
  path: string;
  mode: string;
  status: string;
  linkedCount: number;
  localCount: number;
}

export interface SyncResult {
  target: string;
  linked: string[];
  updated: string[];
  skipped: string[];
  pruned: string[];
}

export interface DiffTarget {
  target: string;
  items: { skill: string; action: string; reason?: string }[];
}

export interface SearchResult {
  name: string;
  description: string;
  source: string;
  stars: number;
  owner: string;
  repo: string;
}

export interface InstallResult {
  skillName?: string;
  repoName?: string;
  action: string;
  warnings: string[];
  skillCount?: number;
  skills?: string[];
}

export interface UpdateResultItem {
  name: string;
  action: string; // "updated", "up-to-date", "skipped", "error"
  message?: string;
  isRepo: boolean;
}

export interface AvailableTarget {
  name: string;
  path: string;
  installed: boolean;
  detected: boolean;
}

export interface SkillFileContent {
  content: string;
  contentType: string;
  filename: string;
}

export interface DiscoveredSkill {
  name: string;
  path: string;
}

export interface DiscoverResult {
  needsSelection: boolean;
  skills: DiscoveredSkill[];
}

export interface BatchInstallResultItem {
  name: string;
  action?: string;
  warnings?: string[];
  error?: string;
}

export interface BatchInstallResult {
  results: BatchInstallResultItem[];
  summary: string;
}

export interface LocalSkillInfo {
  name: string;
  path: string;
  targetName: string;
  size: number;
  modTime: string;
}

export interface CollectScanTarget {
  targetName: string;
  skills: LocalSkillInfo[];
}

export interface CollectScanResult {
  targets: CollectScanTarget[];
  totalCount: number;
}

export interface CollectResult {
  pulled: string[];
  skipped: string[];
  failed: Record<string, string>;
}

// Trash types
export interface TrashedSkill {
  name: string;
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
  date: string;
  sizeMB: number;
}

export interface BackupListResponse {
  backups: BackupInfo[];
  totalSizeMB: number;
}

// Check types
export interface RepoCheckResult {
  name: string;
  status: string;
  behind: number;
  message?: string;
}

export interface SkillCheckResult {
  name: string;
  source: string;
  version: string;
  status: string;
  installed_at?: string;
}

export interface CheckResult {
  tracked_repos: RepoCheckResult[];
  skills: SkillCheckResult[];
}

// Git types
export interface GitStatus {
  isRepo: boolean;
  hasRemote: boolean;
  branch: string;
  isDirty: boolean;
  files: string[];
  sourceDir: string;
}

export interface PushResponse {
  success: boolean;
  message: string;
  dryRun?: boolean;
}

export interface PullResponse {
  success: boolean;
  upToDate: boolean;
  commits: { hash: string; message: string }[];
  stats: { filesChanged: number; insertions: number; deletions: number };
  syncResults: SyncResult[];
  dryRun?: boolean;
  message?: string;
}

// Log types
export interface LogEntry {
  ts: string;
  cmd: string;
  args?: Record<string, any>;
  status: string;
  msg?: string;
  ms?: number;
}

export interface LogListResponse {
  entries: LogEntry[];
  total: number;
  totalAll: number;
  commands: string[];
}

// Audit types
export interface AuditFinding {
  severity: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' | 'INFO';
  pattern: string;
  message: string;
  file: string;
  line: number;
  snippet: string;
}

export interface AuditResult {
  skillName: string;
  findings: AuditFinding[];
  riskScore: number;
  riskLabel: 'clean' | 'low' | 'medium' | 'high' | 'critical';
  threshold: string;
  isBlocked: boolean;
  scanTarget?: string;
}

export interface AuditSummary {
  total: number;
  passed: number;
  warning: number;
  failed: number;
  critical: number;
  high: number;
  medium: number;
  low: number;
  info: number;
  threshold: string;
  riskScore: number;
  riskLabel: 'clean' | 'low' | 'medium' | 'high' | 'critical';
  scanErrors?: number;
}

export interface AuditAllResponse {
  results: AuditResult[];
  summary: AuditSummary;
}

export interface AuditSkillResponse {
  result: AuditResult;
  summary: AuditSummary;
}

export interface AuditRulesResponse {
  exists: boolean;
  raw: string;
  path: string;
}
