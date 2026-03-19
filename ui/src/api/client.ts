import { BASE_PATH } from '../lib/basePath';

const BASE = BASE_PATH + '/api';

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response;
  try {
    res = await fetch(BASE + path, {
      headers: { 'Content-Type': 'application/json' },
      ...init,
    });
  } catch {
    throw new ApiError(0, 'Server connection lost — try restarting with "skillshare ui".');
  }
  const text = await res.text();
  if (!text) {
    throw new ApiError(res.status || 502, 'Empty response from server (request may have timed out)');
  }
  let data: any;
  try {
    data = JSON.parse(text);
  } catch {
    throw new ApiError(res.status || 502, `Invalid JSON response: ${text.slice(0, 200)}`);
  }
  if (!res.ok) {
    throw new ApiError(res.status, data.error ?? res.statusText);
  }
  return data as T;
}

// createSSEStream creates an EventSource with the standard done/error lifecycle.
// The `handlers` map registers named SSE event listeners; the special key "done"
// is treated as the terminal event that closes the connection.
function createSSEStream(
  url: string,
  handlers: Record<string, (data: any) => void>,
  onError: (err: Error) => void,
  errorMessage: string,
): EventSource {
  const es = new EventSource(url);
  let completed = false;
  for (const [event, handler] of Object.entries(handlers)) {
    if (event === 'done') {
      es.addEventListener('done', (e) => {
        completed = true;
        es.close();
        handler(JSON.parse((e as MessageEvent).data));
      });
    } else {
      es.addEventListener(event, (e) => {
        handler(JSON.parse((e as MessageEvent).data));
      });
    }
  }
  es.addEventListener('error', () => {
    if (completed) return;
    es.close();
    onError(new Error(errorMessage));
  });
  return es;
}

// Extras types
export interface ExtraTarget {
  path: string;
  mode: string;
  status: string;  // "synced" | "drift" | "not synced" | "no source"
}

export interface Extra {
  name: string;
  source_dir: string;
  source_type: "per-extra" | "extras_source" | "default";
  file_count: number;
  source_exists: boolean;
  targets: ExtraTarget[];
}

export interface ExtraDiffItem {
  action: string;  // "create" | "update" | "prune"
  file: string;
  reason: string;
}

export interface ExtraDiffResult {
  name: string;
  target: string;
  mode: string;
  synced: boolean;
  items: ExtraDiffItem[];
}

export interface ExtrasSyncResult {
  name: string;
  targets: Array<{
    path: string;
    mode: string;
    synced: number;
    skipped: number;
    pruned: number;
    error?: string;
  }>;
}

export interface SyncMatrixEntry {
  skill: string;
  target: string;
  status: 'synced' | 'excluded' | 'not_included' | 'skill_target_mismatch' | 'na';
  reason: string;
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
  batchUninstall: (opts: BatchUninstallRequest) =>
    apiFetch<BatchUninstallResult>('/uninstall/batch', {
      method: 'POST',
      body: JSON.stringify(opts),
    }),
  getTemplates: async () => {
    const res = await apiFetch<TemplatesResponse>('/skills/templates');
    // Normalize: Go omits nil slices, so scaffoldDirs may be undefined
    for (const p of res.patterns) {
      if (!p.scaffoldDirs) p.scaffoldDirs = [];
    }
    return res;
  },
  createSkill: (data: CreateSkillRequest) =>
    apiFetch<CreateSkillResponse>('/skills', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  // Targets
  listTargets: () => apiFetch<{ targets: Target[]; sourceSkillCount: number }>('/targets'),
  addTarget: (name: string, path: string) =>
    apiFetch<{ success: boolean }>('/targets', {
      method: 'POST',
      body: JSON.stringify({ name, path }),
    }),
  removeTarget: (name: string) =>
    apiFetch<{ success: boolean }>(`/targets/${encodeURIComponent(name)}`, { method: 'DELETE' }),
  updateTarget: (name: string, opts: { include?: string[]; exclude?: string[]; mode?: string }) =>
    apiFetch<{ success: boolean }>(`/targets/${encodeURIComponent(name)}`, {
      method: 'PATCH',
      body: JSON.stringify(opts),
    }),

  // Sync Matrix
  getSyncMatrix: (target?: string) =>
    apiFetch<{ entries: SyncMatrixEntry[] }>(
      `/sync-matrix${target ? '?target=' + encodeURIComponent(target) : ''}`
    ),
  previewSyncMatrix: (target: string, include: string[], exclude: string[]) =>
    apiFetch<{ entries: SyncMatrixEntry[] }>('/sync-matrix/preview', {
      method: 'POST',
      body: JSON.stringify({ target, include, exclude }),
    }),

  // Sync
  sync: (opts: { dryRun?: boolean; force?: boolean }) =>
    apiFetch<SyncResponse>('/sync', {
      method: 'POST',
      body: JSON.stringify(opts),
    }),
  diff: (target?: string) =>
    apiFetch<{ diffs: DiffTarget[] } & IgnoreSources>(`/diff${target ? '?target=' + encodeURIComponent(target) : ''}`),
  diffStream: (
    onDiscovering: () => void,
    onStart: (total: number) => void,
    onResult: (diff: DiffTarget, checked: number) => void,
    onDone: (data: { diffs: DiffTarget[] } & IgnoreSources) => void,
    onError: (err: Error) => void,
  ): EventSource =>
    createSSEStream(BASE + '/diff/stream', {
      discovering: () => onDiscovering(),
      start: (d) => onStart(d.total),
      result: (d) => onResult(d.diff, d.checked),
      done: onDone,
    }, onError, 'Diff stream failed'),

  // Hub
  hubIndex: () => apiFetch<HubIndex>('/hub/index'),
  getHubConfig: () => apiFetch<HubConfigResponse>('/hub/saved'),
  putHubConfig: (data: { hubs: HubSavedEntry[]; default: string }) =>
    apiFetch<{ success: boolean }>('/hub/saved', {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
  addHub: (hub: { label: string; url: string }) =>
    apiFetch<{ success: boolean }>('/hub/saved', {
      method: 'POST',
      body: JSON.stringify(hub),
    }),
  removeHub: (label: string) =>
    apiFetch<{ success: boolean }>(`/hub/saved/${encodeURIComponent(label)}`, {
      method: 'DELETE',
    }),

  // Search & Install
  search: (q: string, limit = 20) =>
    apiFetch<{ results: SearchResult[] }>(`/search?q=${encodeURIComponent(q)}&limit=${limit}`),
  searchHub: (q: string, hubURL: string) =>
    apiFetch<{ results: SearchResult[] }>(`/search?q=${encodeURIComponent(q)}&hub=${encodeURIComponent(hubURL)}`),
  check: () => apiFetch<CheckResult>('/check'),
  checkStream: (
    onDiscovering: () => void,
    onStart: (total: number) => void,
    onProgress: (checked: number) => void,
    onDone: (data: CheckResult) => void,
    onError: (err: Error) => void,
  ): EventSource =>
    createSSEStream(BASE + '/check/stream', {
      discovering: () => onDiscovering(),
      start: (d) => onStart(d.total),
      progress: (d) => onProgress(d.checked),
      done: onDone,
    }, onError, 'Check stream failed'),
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
  update: (opts: { name?: string; force?: boolean; all?: boolean; skipAudit?: boolean }) =>
    apiFetch<{ results: UpdateResultItem[] }>('/update', {
      method: 'POST',
      body: JSON.stringify(opts),
    }),
  updateAllStream: (
    onStart: (total: number) => void,
    onResult: (item: UpdateResultItem) => void,
    onDone: (data: { results: UpdateResultItem[]; summary: UpdateStreamSummary }) => void,
    onError: (err: Error) => void,
    opts?: { names?: string[]; force?: boolean; skipAudit?: boolean },
  ): EventSource => {
    const params = new URLSearchParams();
    if (opts?.names?.length) params.set('names', opts.names.join(','));
    if (opts?.force) params.set('force', 'true');
    if (opts?.skipAudit) params.set('skipAudit', 'true');
    return createSSEStream(`${BASE}/update/stream?${params.toString()}`, {
      start: (d) => onStart(d.total),
      result: onResult,
      done: onDone,
    }, onError, 'Update stream failed');
  },

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
    apiFetch<ConfigSaveResponse>('/config', {
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
  validateRestore: (opts: { timestamp: string; target: string }) =>
    apiFetch<RestoreValidateResponse>('/restore/validate', {
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

  // Extras
  listExtras: () => apiFetch<{ extras: Extra[] }>('/extras'),
  diffExtras: (name?: string) =>
    apiFetch<{ extras: ExtraDiffResult[] }>(`/extras/diff${name ? '?name=' + encodeURIComponent(name) : ''}`),
  createExtra: (data: {
    name: string;
    source?: string;
    targets: Array<{ path: string; mode: string }>;
  }) =>
    apiFetch<{ success: boolean }>('/extras', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  syncExtras: (opts?: { name?: string; dry_run?: boolean; force?: boolean }) =>
    apiFetch<{ extras: ExtrasSyncResult[] }>('/extras/sync', {
      method: 'POST',
      body: JSON.stringify(opts ?? {}),
    }),
  deleteExtra: (name: string) =>
    apiFetch<{ success: boolean }>(`/extras/${encodeURIComponent(name)}`, { method: 'DELETE' }),
  setExtraMode: (name: string, target: string, mode: string) =>
    apiFetch<{ success: boolean }>(`/extras/${encodeURIComponent(name)}/mode`, {
      method: 'PATCH',
      body: JSON.stringify({ target, mode }),
    }),

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
  getLogStats: (type?: string, filters?: { cmd?: string; status?: string; since?: string }) => {
    const params = new URLSearchParams();
    params.set('type', type ?? 'ops');
    if (filters?.cmd) params.set('cmd', filters.cmd);
    if (filters?.status) params.set('status', filters.status);
    if (filters?.since) params.set('since', filters.since);
    return apiFetch<LogStatsResponse>(`/log/stats?${params.toString()}`);
  },

  // Audit
  auditAll: () => apiFetch<AuditAllResponse>('/audit'),
  auditSkill: (name: string) => apiFetch<AuditSkillResponse>(`/audit/${encodeURIComponent(name)}`),
  auditAllStream: (
    onStart: (total: number) => void,
    onProgress: (scanned: number) => void,
    onDone: (data: AuditAllResponse) => void,
    onError: (err: Error) => void,
  ): EventSource =>
    createSSEStream(BASE + '/audit/stream', {
      start: (d) => onStart(d.total),
      progress: (d) => onProgress(d.scanned),
      done: onDone,
    }, onError, 'Audit stream failed'),

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
  getCompiledRules: () => apiFetch<CompiledRulesResponse>('/audit/rules/compiled'),
  toggleRule: (req: { id?: string; pattern?: string; enabled: boolean; severity?: string }) =>
    apiFetch<{ success: boolean }>('/audit/rules/toggle', {
      method: 'POST',
      body: JSON.stringify(req),
    }),
  resetRules: () =>
    apiFetch<{ success: boolean }>('/audit/rules/reset', {
      method: 'POST',
    }),

  // Git
  gitStatus: () => apiFetch<GitStatus>('/git/status'),
  gitBranches: (opts?: { fetch?: boolean }) =>
    apiFetch<GitBranches>(`/git/branches${opts?.fetch ? '?fetch=true' : ''}`),
  gitCheckout: (branch: string) =>
    apiFetch<GitCheckoutResponse>('/git/checkout', {
      method: 'POST',
      body: JSON.stringify({ branch }),
    }),
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

  // Doctor
  doctor: () => apiFetch<DoctorResponse>('/doctor'),

  // Skillignore
  getSkillignore: () => apiFetch<SkillignoreResponse>('/skillignore'),
  putSkillignore: (raw: string) =>
    apiFetch<{ success: boolean }>('/skillignore', {
      method: 'PUT',
      body: JSON.stringify({ raw }),
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
  targets?: string[];
  installedAt?: string;
  source?: string;
  type?: string;
  repoUrl?: string;
  version?: string;
}

export interface SkillPattern {
  name: string;
  description: string;
  scaffoldDirs: string[];
}

export interface SkillCategory {
  key: string;
  label: string;
}

export interface TemplatesResponse {
  patterns: SkillPattern[];
  categories: SkillCategory[];
}

export interface CreateSkillRequest {
  name: string;
  pattern: string;
  category?: string;
  scaffoldDirs?: string[];
}

export interface CreateSkillResponse {
  skill: {
    name: string;
    flatName: string;
    relPath: string;
    sourcePath: string;
  };
  createdFiles: string[];
}

export interface Target {
  name: string;
  path: string;
  mode: string;
  status: string;
  linkedCount: number;
  localCount: number;
  include: string[];
  exclude: string[];
  expectedSkillCount: number;
}

export interface SyncResult {
  target: string;
  linked: string[];
  updated: string[];
  skipped: string[];
  pruned: string[];
  dir_created?: string;
}

export interface IgnoreSources {
  ignored_count: number;
  ignored_skills: string[];
  ignore_root: string;
  ignore_repos: string[];
}

export interface SyncResponse extends IgnoreSources {
  results: SyncResult[];
  warnings?: string[];
}

export interface ConfigSaveResponse {
  success: boolean;
  warnings?: string[];
}

export interface DiffTarget {
  target: string;
  items: { skill: string; action: string; reason?: string }[];
}

export interface HubIndex {
  schemaVersion: number;
  generatedAt: string;
  sourcePath?: string;
  skills: { name: string; description?: string; source?: string }[];
}

export interface SearchResult {
  name: string;
  description: string;
  source: string;
  skill?: string;
  stars: number;
  owner: string;
  repo: string;
  tags?: string[];
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
  action: string; // "updated", "up-to-date", "skipped", "error", "blocked"
  message?: string;
  isRepo: boolean;
  auditRiskScore?: number;
  auditRiskLabel?: string;
}

export interface UpdateStreamSummary {
  updated: number;
  upToDate: number;
  blocked: number;
  errors: number;
  skipped: number;
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
  description?: string;
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

export interface BatchUninstallRequest {
  names: string[];
  force?: boolean;
}

export interface BatchUninstallItemResult {
  name: string;
  success: boolean;
  movedToTrash?: boolean;
  error?: string;
}

export interface BatchUninstallResult {
  results: BatchUninstallItemResult[];
  summary: { succeeded: number; failed: number };
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
  sizeBytes: number;
}

export interface BackupListResponse {
  backups: BackupInfo[];
  totalSizeBytes: number;
}

export interface RestoreValidateResponse {
  valid: boolean;
  error: string;
  conflicts: string[];
  backupSizeBytes: number;
  currentIsSymlink: boolean;
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
  remoteURL?: string;
  headHash?: string;
  headMessage?: string;
  trackingBranch?: string;
}

export interface GitBranches {
  current: string;
  local: string[];
  remote: string[];
  isDirty: boolean;
  dirtyFiles: string[];
}

export interface GitCheckoutResponse {
  success: boolean;
  branch: string;
  message: string;
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

export interface CommandStats {
  total: number;
  ok: number;
  error: number;
  partial: number;
  blocked: number;
}

export interface LogStatsResponse {
  total: number;
  success_rate: number;
  by_command: Record<string, CommandStats>;
  last_operation?: LogEntry;
}

// Audit types
export interface AuditFinding {
  severity: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' | 'INFO';
  pattern: string;
  message: string;
  file: string;
  line: number;
  snippet: string;
  ruleId?: string;
  analyzer?: string;
  category?: string;
  confidence?: number;
  fingerprint?: string;
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
  byCategory?: Record<string, number>;
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

export interface CompiledRule {
  id: string;
  severity: string;
  pattern: string;
  message: string;
  regex: string;
  exclude?: string;
  enabled: boolean;
  source: string;
}

export interface PatternGroup {
  pattern: string;
  total: number;
  enabled: number;
  disabled: number;
  maxSeverity: string;
}

export interface CompiledRulesResponse {
  rules: CompiledRule[];
  patterns: PatternGroup[];
}

// Doctor health check types
export interface DoctorCheck {
  name: string;
  status: 'pass' | 'warning' | 'error' | 'info';
  message: string;
  details?: string[];
}

export interface DoctorSummary {
  total: number;
  pass: number;
  warnings: number;
  errors: number;
  info: number;
}

export interface DoctorVersion {
  current: string;
  latest?: string;
  update_available: boolean;
}

export interface DoctorResponse {
  checks: DoctorCheck[];
  summary: DoctorSummary;
  version?: DoctorVersion;
}

// Hub saved config types
export interface HubSavedEntry {
  label: string;
  url: string;
  builtIn?: boolean;
}

export interface HubConfigResponse {
  hubs: HubSavedEntry[];
  default: string;
}

// Skillignore types
export interface SkillignoreStats {
  pattern_count: number;
  ignored_count: number;
  patterns: string[];
  ignored_skills: string[];
}

export interface SkillignoreResponse {
  exists: boolean;
  path: string;
  raw: string;
  stats?: SkillignoreStats;
}
