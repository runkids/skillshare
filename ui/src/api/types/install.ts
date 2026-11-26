import type { LinkedRepo } from './resources';

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
  /** Branch or tag the source names; empty or absent means the default branch. */
  ref?: string;
}

export interface SkillPreview {
  name: string;
  description: string;
  license?: string;
  tags?: string[];
  content: string;
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

export interface DiscoveredSkill {
  name: string;
  path: string;
  description?: string;
  kind?: 'skill' | 'agent';
}

export interface DiscoveredAgent {
  name: string;
  path: string;
  fileName: string;
  kind: 'agent';
}

export interface DiscoverResult {
  needsSelection: boolean;
  skills: DiscoveredSkill[];
  agents: DiscoveredAgent[];
  warnings?: string[];
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

// Check types
export interface RepoCheckResult {
  name: string;
  status: string;
  behind: number;
  message?: string;
}

export interface SkillCheckResult {
  name: string;
  kind?: 'skill' | 'agent';
  source: string;
  version: string;
  status: string;
  installed_at?: string;
  message?: string;
  /** The branch the remote HEAD points at, for a source that recorded no branch. */
  default_branch?: string;
}

export interface ConfigEntry {
  name: string;
  source: string;
  tracked: boolean;
  branch?: string;
}

export interface InstallFromConfigResult {
  installed: number;
  installedRepos: number;
  installedRepoSkills: number;
  skipped: number;
  failed: string[];
}

export interface CheckResult {
  linked_repos?: LinkedRepo[];
  tracked_repos: RepoCheckResult[];
  skills: SkillCheckResult[];
  /** Only a full stream check reports agents, each by its path without `.md`. */
  agents?: SkillCheckResult[];
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
