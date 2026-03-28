export interface FieldDoc {
  description: string;
  type: string;
  allowedValues?: string[];
  example: string;
}

export const fieldDocs: Record<string, FieldDoc> = {
  // --- Top-level ---
  sync_mode: {
    description: 'Alias for "mode". Controls how skills are synced from source to target directories.',
    type: 'string',
    allowedValues: ['merge', 'symlink', 'copy'],
    example: 'sync_mode: merge',
  },
  source: {
    description: 'Path to the skill source directory. This is the single source of truth for all skills.',
    type: 'string',
    example: 'source: ~/.config/skillshare/skills',
  },
  extras_source: {
    description: 'Default extras source directory. Individual extras can override this with their own source.',
    type: 'string',
    example: 'extras_source: ~/.config/skillshare/extras',
  },
  mode: {
    description: 'Default sync mode for all targets. Can be overridden per target.',
    type: 'string',
    allowedValues: ['merge', 'symlink', 'copy'],
    example: 'mode: merge',
  },
  tui: {
    description: 'Enable or disable interactive TUI prompts. Set to false for CI/scripting.',
    type: 'boolean',
    example: 'tui: false',
  },
  ignore: {
    description: 'List of skill name patterns to ignore globally. Uses gitignore-style patterns.',
    type: 'string[]',
    example: 'ignore:\n  - _deprecated*\n  - test-*',
  },
  gitlab_hosts: {
    description: 'List of self-hosted GitLab instances for skill installation and search.',
    type: 'string[]',
    example: 'gitlab_hosts:\n  - gitlab.company.com',
  },

  // --- Targets ---
  targets: {
    description: 'Map of target AI tools to configure. Each target uses skills: (and future agents:) sub-keys for per-resource-kind configuration.',
    type: 'object',
    example: 'targets:\n  claude:\n    skills:\n      mode: merge\n      include: ["team-*"]\n  cursor:\n    skills:\n      mode: symlink',
  },
  'targets.name': {
    description: 'Target name. Use a built-in name (e.g., claude, cursor, codex) for automatic path resolution, or any custom name with an explicit path under skills:.',
    type: 'string',
    example: '- name: claude\n  skills:\n    mode: merge',
  },
  'targets.skills': {
    description: 'Skills-specific target configuration. Controls path, sync mode, and include/exclude filters for skills.',
    type: 'object',
    example: 'skills:\n  path: ~/.claude/skills\n  mode: merge\n  include: ["team-*"]\n  exclude: ["wip-*"]',
  },
  'targets.skills.path': {
    description: 'Override the target skills directory path. If omitted, the built-in default for this target is used.',
    type: 'string',
    example: 'path: ~/.claude/skills',
  },
  'targets.skills.mode': {
    description: 'Sync mode for skills in this target. If omitted, inherits the top-level mode (defaults to merge).',
    type: 'string',
    allowedValues: ['merge', 'symlink', 'copy'],
    example: 'mode: symlink',
  },
  'targets.skills.include': {
    description: 'Glob patterns — only matching skills are synced to this target (merge and copy modes).',
    type: 'string[]',
    example: 'include: ["team-*", "shared-*"]',
  },
  'targets.skills.exclude': {
    description: 'Glob patterns — matching skills are excluded from sync to this target (merge and copy modes).',
    type: 'string[]',
    example: 'exclude: ["draft-*", "wip-*"]',
  },
  'targets.agents': {
    description: '(Reserved — not yet available) Agents-specific target configuration.',
    type: 'object',
    example: 'agents:\n  path: ~/.claude/agents',
  },
  'targets.agents.path': {
    description: '(Reserved) Override the target agents directory path.',
    type: 'string',
    example: 'path: ~/.claude/agents',
  },
  'targets.agents.mode': {
    description: '(Reserved) Sync mode for agents.',
    type: 'string',
    allowedValues: ['merge', 'symlink', 'copy'],
    example: 'mode: merge',
  },
  'targets.agents.include': {
    description: '(Reserved) Glob patterns — only matching agents are synced.',
    type: 'string[]',
    example: 'include: ["tutor-*"]',
  },
  'targets.agents.exclude': {
    description: '(Reserved) Glob patterns — matching agents are excluded from sync.',
    type: 'string[]',
    example: 'exclude: ["draft-*"]',
  },
  // Legacy flat fields (kept for backward compat display)
  'targets.include': {
    description: '[Legacy] Use skills.include instead. Glob patterns for skill include filter.',
    type: 'string[]',
    example: 'skills:\n  include: ["skill-a"]',
  },
  'targets.exclude': {
    description: '[Legacy] Use skills.exclude instead. Glob patterns for skill exclude filter.',
    type: 'string[]',
    example: 'skills:\n  exclude: ["debug-only"]',
  },
  'targets.mode': {
    description: '[Legacy] Use skills.mode instead. Sync mode override for this target.',
    type: 'string',
    allowedValues: ['merge', 'symlink', 'copy'],
    example: 'skills:\n  mode: symlink',
  },
  'targets.path': {
    description: '[Legacy] Use skills.path instead. Custom path for the target skills directory.',
    type: 'string',
    example: 'skills:\n  path: ~/.cursor/skills',
  },

  // --- Extras ---
  extras: {
    description: 'List of extra file bundles to sync alongside skills. Each extra has a name, source, and target list.',
    type: 'ExtraConfig[]',
    example: 'extras:\n  - name: prompts\n    source: ~/prompts\n    targets:\n      - path: ~/.claude\n        mode: merge',
  },
  'extras.name': {
    description: 'Unique name for this extra bundle.',
    type: 'string',
    example: 'name: my-prompts',
  },
  'extras.source': {
    description: 'Path to the source directory for this extra bundle.',
    type: 'string',
    example: 'source: ~/my-extras/prompts',
  },
  'extras.targets': {
    description: 'List of target directories for this extra bundle.',
    type: 'ExtraTarget[]',
    example: 'targets:\n  - path: ~/.claude\n    mode: merge',
  },
  'extras.targets.path': {
    description: 'Target directory path for this extra.',
    type: 'string',
    example: 'path: ~/.claude',
  },
  'extras.targets.mode': {
    description: 'Sync mode for this extra target.',
    type: 'string',
    allowedValues: ['merge', 'symlink', 'copy'],
    example: 'mode: merge',
  },
  'extras.targets.flatten': {
    description: 'When true, files from subdirectories are synced directly into the target root. Cannot be used with symlink mode.',
    type: 'boolean',
    example: 'flatten: true',
  },

  // --- Audit ---
  audit: {
    description: 'Configure security audit behavior for skill scanning.',
    type: 'object',
    example: 'audit:\n  block_threshold: HIGH\n  profile: strict',
  },
  'audit.block_threshold': {
    description: 'Minimum severity to block installation. Skills with findings at or above this level are blocked.',
    type: 'string',
    allowedValues: ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW', 'INFO'],
    example: 'block_threshold: HIGH',
  },
  'audit.profile': {
    description: 'Audit profile that controls which rules are active and their sensitivity.',
    type: 'string',
    allowedValues: ['default', 'strict', 'permissive'],
    example: 'profile: strict',
  },
  'audit.dedupe_mode': {
    description: 'How to handle duplicate findings across skills.',
    type: 'string',
    allowedValues: ['legacy', 'global'],
    example: 'dedupe_mode: global',
  },
  'audit.enabled_analyzers': {
    description: 'Allowlist of analyzers to run. Empty means all analyzers are active.',
    type: 'string[]',
    example: 'enabled_analyzers:\n  - shell-injection\n  - secrets',
  },

  // --- Hub ---
  hub: {
    description: 'Configure the skill hub for search and discovery.',
    type: 'object',
    example: 'hub:\n  default: github',
  },
  'hub.default': {
    description: 'Default hub to use for search commands.',
    type: 'string',
    example: 'default: github',
  },
  'hub.hubs': {
    description: 'List of custom hub endpoints for skill discovery.',
    type: 'HubEntry[]',
    example: 'hubs:\n  - label: internal\n    url: https://hub.company.com',
  },

  // --- Log ---
  log: {
    description: 'Configure the operations log.',
    type: 'object',
    example: 'log:\n  max_entries: 500',
  },
  'log.max_entries': {
    description: 'Maximum number of log entries to keep. 0 for unlimited.',
    type: 'number',
    example: 'max_entries: 1000',
  },
};
