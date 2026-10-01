/** One resource kind of a project; null means the project does not take it. */
export interface ProjectResource {
  mode: string;
  targetNaming?: string;
  include: string[];
  exclude: string[];
}

export interface Project {
  /** The key in config.yaml, as written */
  root: string;
  /** Absolute folder */
  path: string;
  name: string;
  customName?: string;
  targets: string[];
  skills: ProjectResource | null;
  agents: ProjectResource | null;
  /** Tools that share a skills folder are one group, written once */
  groups: { target: string; tools: string[]; skillsPath: string; agentsPath: string }[];
  missing: boolean;
  hasOwnConfig: boolean;
}

export type ProjectInput = Pick<Project, 'root' | 'targets' | 'skills' | 'agents'> & { name?: string; create?: boolean };

export interface ProjectList {
  projects: Project[];
  /** Ordinary targets whose folders are one project's tool paths */
  convertible: { root: string; targets: string[]; tools: string[]; agents: boolean }[];
  /** Every tool with a project skills path, relative to the project folder */
  tools: { name: string; skillsPath: string; agentsPath: string }[];
}

export interface Target {
  name: string;
  /** Root of the project this target belongs to */
  project?: string;
  /** The built-in Agent this target is another config folder of */
  agent?: string;
  /** That config folder; the skills and agents paths follow it */
  configDir?: string;
  /** The executable that runs its plugin commands, when not the Agent's own */
  cli?: string;
  path: string;
  mode: string;
  targetNaming: string;
  status: string;
  linkedCount: number;
  localCount: number;
  include: string[];
  exclude: string[];
  expectedSkillCount: number;
  skippedSkillCount?: number;
  collisionCount?: number;
  agentPath?: string;
  agentMode?: string;
  agentExtension?: string; // converts each agent; implies copy mode
  agentInclude?: string[];
  agentExclude?: string[];
  agentLinkedCount?: number;
  agentLocalCount?: number;
  agentExpectedCount?: number;
  /** False when skillshare writes no skills to this target; its other content still syncs. */
  skillsEnabled: boolean;
  /** With skills off: enabled targets whose skills folder this tool reads. */
  skillsReadFrom?: string[];
  /** Targets with skills off that read this target's skills folder. */
  skillsAlsoReadBy?: string[];
}

export interface SkillsDetach {
  removed: string[];
  kept: string[];
  copies?: string[];
}

export interface SkillsOffPreview {
  remove: string[];
  keep: string[];
  /** Copies copy mode made: kept, but loaded twice if the tool reads the skills elsewhere too. */
  copies?: string[];
  /** The enabled target that uses the same folder; nothing is removed then. */
  sharedWith?: string;
}
