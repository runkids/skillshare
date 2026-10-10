export interface TrackedRepo {
  name: string;
  skillCount: number;
  dirty: boolean;
}

export interface Overview {
  source: string;
  agentsSource?: string;
  extrasSource?: string;
  skillCount: number;
  agentCount: number;
  topLevelCount: number;
  targetCount: number;
  mode: string;
  version: string;
  trackedRepos: TrackedRepo[];
  isProjectMode: boolean;
  projectRoot?: string;
  /** Folder holding config.yaml */
  configDir: string;
}

export interface LinkedRepo {
  name: string;
  target: string;
}

export interface Skill {
  name: string;
  kind: 'skill' | 'agent';
  flatName: string;
  relPath: string;
  sourcePath: string;
  /** First-level source link followed by the current policy, and its resolved target. */
  linkName?: string;
  linkTarget?: string;
  isInRepo: boolean;
  /** Tracked repo root relative to the source (`_team`, or `org/_team` when installed with --into). */
  repoPath?: string;
  targets?: string[];
  installedAt?: string;
  source?: string;
  type?: string;
  repoUrl?: string;
  version?: string;
  disabled?: boolean;
  /** disable-model-invocation: invocable by name, never loaded by the model on its own. */
  manualOnly?: boolean;
  branch?: string;
}

export interface SourceLink {
  name: string;
  target: string;
  available?: boolean;
  warning?: string;
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
  description?: string;
  /** Folder under the source to create the skill in */
  into?: string;
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

export interface SourceLinkRequest {
  path: string;
  name?: string;
  enable?: boolean;
}

export interface SourceLinkResult {
  path: string;
  target: string;
  kind: 'symlink' | 'junction';
  warning: string;
}

export interface SkillFileContent {
  content: string;
  contentType: string;
  filename: string;
}

export interface BatchUninstallRequest {
  names: string[];
  kind?: 'skill' | 'agent';
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

export interface MoveRequest {
  /** Skills (flat name, relPath or basename) or folder relPaths. */
  names: string[];
  /** Folder under the skills source; '.' is the source root. */
  dest: string;
  /** Accepts a name collision; nothing else. */
  force?: boolean;
  /** Plan only: nothing on disk or in the store changes. */
  dryRun?: boolean;
}

export interface MoveItemResult {
  name: string;
  success: boolean;
  from?: string;
  to?: string;
  flatName?: string;
  /** The skill had an install record, and it moved with it. */
  record?: boolean;
  /** For a folder: how many skills moved with it. */
  skills?: number;
  error?: string;
  /** Stable refusal code; the dashboard shows its own text for it, never `error`. */
  error_code?: string;
}

export interface MoveResult {
  results: MoveItemResult[];
  summary: { succeeded: number; failed: number };
  warnings: string[];
  dryRun: boolean;
}

export interface BatchToggleItemResult {
  name: string;
  success: boolean;
  disabled: boolean;
  error?: string;
}

export interface BatchToggleResult {
  results: BatchToggleItemResult[];
  summary: { updated: number; unchanged: number; failed: number };
}

export interface LocalSkillInfo {
  name: string;
  path: string;
  targetName: string;
  size: number;
  modTime: string;
  kind?: 'skill' | 'agent';
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
