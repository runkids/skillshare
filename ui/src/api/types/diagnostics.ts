export interface VersionCheck {
  cliVersion: string;
  cliLatest?: string;
  cliUpdateAvailable: boolean;
  cliDevMode?: boolean;
  /** Set for a Nix install: the command that updates the CLI, or "" when a Nix configuration provides it. */
  nixUpgradeCommand?: string | null;
  skillVersion: string;
  skillLatest?: string;
  skillUpdateAvailable: boolean;
}

// Doctor health check types
export interface DoctorCheck {
  name: string;
  status: 'pass' | 'warning' | 'error' | 'info';
  message: string;
  details?: string[];
  suggestions?: string[];
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
  dev_mode?: boolean;
}

export interface DoctorResponse {
  checks: DoctorCheck[];
  summary: DoctorSummary;
  version?: DoctorVersion;
}

// Analyze types
export interface AnalyzeLintIssue {
  rule: string;
  severity: 'error' | 'warning';
  category: string;
  message: string;
}

export interface AnalyzeSkill {
  name: string;
  description_chars: number;
  description_tokens: number;
  body_chars: number;
  body_tokens: number;
  lint_issues?: AnalyzeLintIssue[];
  /** Lives in the target folder itself, not in the skillshare source. */
  local?: boolean;
  /** Disabled in .skillignore but still exposed by a symlink-mode target. */
  disabled?: boolean;
  path: string;
  is_tracked: boolean;
  targets?: string[];
  description?: string;
}

export interface AnalyzeTarget {
  name: string;
  skill_count: number;
  always_loaded: { chars: number; estimated_tokens: number };
  on_demand_max: { chars: number; estimated_tokens: number };
  skills: AnalyzeSkill[];
}

export interface AnalyzeResponse {
  targets: AnalyzeTarget[];
}
