export interface ConfigSaveResponse {
  success: boolean;
  warnings?: string[];
}

export interface AvailableTarget {
  name: string;
  path: string;
  agentPath?: string;
  /** Set when a target can be another config folder of this Agent. */
  configDir?: string;
  installed: boolean;
  detected: boolean;
  /** Enabled targets whose skills folder this tool already reads. */
  readsFrom?: string[];
  /** File name of the tool's instructions file (AGENTS.md, GEMINI.md, ...). */
  instructionsFile?: string;
  /** That file's default path. */
  instructionsPath?: string;
  /** Other tools, on this machine or configured, that read this tool's skills folder. */
  readBy?: string[];
}

// Machine-only .local companion of an ignore file; its rules override the shared file.
export interface IgnoreLocalFile {
  path: string;
  raw: string;
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
  local?: IgnoreLocalFile;
  stats?: SkillignoreStats;
}

// Agentignore types
export interface AgentignoreStats {
  pattern_count: number;
  ignored_count: number;
  patterns: string[];
  ignored_agents: string[];
}

export interface AgentignoreResponse {
  exists: boolean;
  path: string;
  raw: string;
  local?: IgnoreLocalFile;
  stats?: AgentignoreStats;
}
