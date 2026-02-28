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
  },

  diff: (target?: string) => ['diff', target ?? '__all'] as const,
  collectScan: (target?: string) => ['collect-scan', target ?? '__all'] as const,

  backups: ['backups'] as const,
  trash: ['trash'] as const,
  gitStatus: ['git-status'] as const,

  audit: {
    all: ['audit'] as const,
    rules: ['audit', 'rules'] as const,
  },

  log: (type: string, limit: number, filters?: Record<string, string>) =>
    ['log', type, limit, filters ?? {}] as const,
  logStats: (type: string, filters?: Record<string, string>) =>
    ['log-stats', type, filters ?? {}] as const,

  config: ['config'] as const,
  check: ['check'] as const,
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
  check: 60 * 1000,            // 1min
};
