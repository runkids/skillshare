import type { SharedName, SyncResult, UnmatchedInclude } from './sync';

// Git types
export interface GitStatus {
  gitInstalled: boolean;
  isRepo: boolean;
  hasRemote: boolean;
  branch: string;
  isDirty: boolean;
  files: string[];
  sourceDir: string;
  scope: string;
  scopeMismatch: boolean;
  mismatchScope?: string;
  mismatchDir?: string;
  remoteURL?: string;
  headHash?: string;
  headMessage?: string;
  trackingBranch?: string;
  /** Commits no remote-tracking branch has yet (what a push uploads); 0 without a remote. */
  ahead: number;
  /** Upstream commits HEAD lacks as of the last fetch (what a pull brings in); 0 without an upstream. */
  behind: number;
  // Root-scope hazards (populated only when scope === 'root').
  nestedRepos: string[];
  configTracked: boolean;
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
  warnings?: string[];
  /** Targets whose include filter selects no skill, from the sync after the pull */
  unmatched?: UnmatchedInclude[];
  shared_names?: SharedName[];
}

export interface GitConflictVersion {
  deleted: boolean;
  content: string;
  noPreview: boolean;
}

export interface GitPullConflict {
  localHash: string;
  remoteHash: string;
  files: { path: string; local: GitConflictVersion; remote: GitConflictVersion }[];
}

export interface GitPullResolution {
  localHash: string;
  remoteHash: string;
  choices: Record<string, 'local' | 'remote'>;
}
