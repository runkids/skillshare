export const queryKeys = {
  overview: ['overview'] as const,
  versionCheck: ['version-check'] as const,

  skills: {
    all: ['skills'] as const,
    detail: (name: string) => ['skills', name] as const,
  },

  targets: {
    all: ['targets'] as const,
    available: ['targets', 'available'] as const,
    /** Own and project targets together */
    synced: ['targets', 'all'] as const,
    projects: ['targets', 'projects'] as const,
    skillsOffPreview: (name: string) => ['targets', 'skills-off-preview', name] as const,
  },
  projects: ['projects'] as const,

  diff: (target?: string) => ['diff', target ?? '__all'] as const,
  collectScan: (target?: string) => ['collect-scan', target ?? '__all'] as const,

  backups: ['backups'] as const,
  restoreValidate: (timestamp: string, target: string) => ['restore-validate', timestamp, target] as const,
  fileBackups: {
    all: ['file-backups'] as const,
    versions: (path: string) => ['file-backups', 'versions', path] as const,
    version: (path: string, id: string) => ['file-backups', 'version', path, id] as const,
  },
  trash: ['trash'] as const,
  gitStatus: ['git-status'] as const,
  gitBranches: ['git-branches'] as const,

  audit: {
    all: (kind?: string) => ['audit', kind ?? 'skills'] as const,
    skill: (name: string) => ['audit', 'skill', name] as const,
    rules: ['audit', 'rules'] as const,
    compiled: ['audit', 'rules', 'compiled'] as const,
  },

  log: (type: string, limit: number, filters?: Record<string, string>) =>
    ['log', type, limit, filters ?? {}] as const,
  logStats: (type: string, filters?: Record<string, string>) =>
    ['log-stats', type, filters ?? {}] as const,

  config: ['config'] as const,
  check: ['check'] as const,
  missingTrackedRepos: ['missing-tracked-repos'] as const,
  // Last update check, kept client-side (see UpdatePage) and shared by the Updates tab and its count.
  updateCheck: ['update-check'] as const,
  syncMatrix: (target?: string) => ['sync-matrix', target ?? '__all'] as const,

  templates: ['templates'] as const,
  skillPreview: (req: object) => ['skill-preview', req] as const,
  extras: ['extras'] as const,
  memory: {
    all: ['memory'] as const,
    list: (search: string) => ['memory', 'notes', search] as const,
    content: (path: string) => ['memory', 'content', path] as const,
  },
  instructions: {
    all: ['instructions'] as const,
    target: (name: string) => ['instructions', 'target', name] as const,
    shared: ['instructions', 'shared'] as const,
    sharedContent: (name: string) => ['instructions', 'shared', name] as const,
    restorePreview: (name: string, target: string) => ['instructions', 'restore-preview', name, target] as const,
    locationRestorePreview: (name: string, path: string) => ['instructions', 'location-restore-preview', name, path] as const,
    project: ['instructions', 'project'] as const,
  },
  // Everything under `target-files`, so invalidating a target's list also refreshes its files.
  targetFiles: {
    list: (name: string) => ['target-files', name] as const,
    content: (name: string, path: string) => ['target-files', name, 'content', path] as const,
  },
  mcp: ['mcp'] as const,
  piExtensions: (name: string) => ['pi-extensions', name] as const,
  // Every Pi target's Extensions tab, which lists the packages Plugins syncs to Pi.
  piExtensionsAll: ['pi-extensions'] as const,
  ompExtensions: (name: string) => ['omp-extensions', name] as const,
  ompExtensionsAll: ['omp-extensions'] as const,
  hooks: ['hooks'] as const,
  plugins: ['plugins'] as const,
  // Under `plugins`, so invalidating that key refreshes both.
  pluginPackages: ['plugins', 'packages'] as const,
  hubConfig: ['hub-config'] as const,
  // Everything under `hub`, so invalidating `hub.drafts` also refreshes each draft.
  hub: {
    drafts: ['hub', 'drafts'] as const,
    draft: (id: string) => ['hub', 'drafts', id] as const,
    contents: (url: string) => ['hub', 'contents', url] as const,
    refs: (source: string) => ['hub', 'refs', source] as const,
    candidates: ['hub', 'candidates'] as const,
  },
  preview: (source: string) => ['preview', source] as const,
  extrasDiff: (name?: string) => ['extras-diff', name ?? '__all'] as const,
  analyze: ['analyze'] as const,
  doctor: ['doctor'] as const,
  skillignore: ['skillignore'] as const,
  agentignore: ['agentignore'] as const,
};

// Stale times per data type
export const staleTimes = {
  overview: 30 * 1000,       // 30s — dashboard, refreshed often
  skills: 2 * 60 * 1000,     // 2min — large payload
  diff: 30 * 1000,            // 30s — fast-changing
  gitStatus: 30 * 1000,       // 30s
  log: 30 * 1000,             // 30s
  targets: 60 * 1000,         // 1min — changes after sync
  version: 5 * 60 * 1000,     // 5min — rarely changes
  config: 5 * 60 * 1000,      // 5min
  auditRules: 5 * 60 * 1000,  // 5min
  backups: 2 * 60 * 1000,     // 2min
  trash: 2 * 60 * 1000,       // 2min
  audit: 5 * 60 * 1000,         // 5min — full audit scan, expensive
  auditSkill: 5 * 60 * 1000,   // 5min — per-skill audit, rarely changes
  check: 60 * 1000,            // 1min
  missingTrackedRepos: 60 * 1000, // 1min
  syncMatrix: 30 * 1000,       // 30s — changes after filter edits
  extras: 30 * 1000,        // 30s — fast-changing like diff
  analyze: 2 * 60 * 1000,   // 2min — walks every skill file
  doctor: 60 * 1000,        // 1min — health checks
  skillignore: 5 * 60 * 1000, // 5min — rarely changes
  agentignore: 5 * 60 * 1000, // 5min — rarely changes
};
