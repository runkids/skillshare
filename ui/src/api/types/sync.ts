export interface SyncMatrixEntry {
  skill: string;
  target: string;
  status: 'synced' | 'excluded' | 'not_included' | 'skill_target_mismatch' | 'na';
  reason: string;
  reasonCode?: string;
  reasonParams?: Record<string, string>;
  kind?: 'skill' | 'agent';
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
  agent_ignore_root?: string;
  agent_ignored_count?: number;
  agent_ignored_skills?: string[];
}

export interface ContextCostGroup {
  targets: string[];
  always_loaded_tokens: number;
  on_demand_tokens: number;
}

export interface ContextCostOffender {
  name: string;
  tokens: number;
}

export interface ContextCostWarning {
  type: 'always_loaded' | 'on_demand';
  target: string;
  actual: number;
  budget: number;
  top_offenders: ContextCostOffender[];
}

export interface ContextCost {
  groups: ContextCostGroup[];
  warnings?: ContextCostWarning[];
}

export function formatTokenK(n: number): string {
  if (n < 1000) return String(n);
  return (n / 1000).toFixed(1) + 'K';
}

/** A target that failed to sync while the others went ahead. */
export interface SyncTargetFailure {
  target: string;
  /** 'config': the target's settings are invalid, so it was skipped */
  part: 'skill' | 'agent' | 'config';
  error: string;
  /** The same text as its entry in warnings */
  message: string;
  /** A symlink points elsewhere; Force replaces it */
  conflict?: boolean;
}

export interface SyncResponse extends IgnoreSources {
  results: SyncResult[];
  warnings?: string[];
  /** Targets whose include filter selects no skill */
  unmatched?: UnmatchedInclude[];
  failed?: SyncTargetFailure[];
  folder_conflicts?: FolderConflict[];
  /** Targets whose skills path overlaps another's in a way folder_conflicts doesn't explain. */
  path_overlap?: number;
  context_cost?: ContextCost;
}

/** A target whose include filter selects no skill. */
export interface UnmatchedInclude {
  target: string;
  /** The project folder of a project target (`project@tool`) */
  root?: string;
  patterns: string[];
  /** Source path names the patterns likely meant */
  suggestions?: string[];
  /** No include pattern selects anything, so the target gets no skills */
  all: boolean;
}

/** A skills folder two or more targets sync into with different settings, so each sync undoes the other. */
export interface FolderConflict {
  path: string;
  targets: string[];
  /** The target to keep syncing skills */
  keep: string;
  /** The targets to stop syncing skills for */
  stop: string[];
}

export interface DiffTarget {
  target: string;
  items: { skill: string; action: string; reason?: string; kind?: 'skill' | 'agent' }[];
  skippedCount?: number;
  collisionCount?: number;
  /** How many of skippedCount break naming rules rather than collide */
  namingSkippedCount?: number;
}
