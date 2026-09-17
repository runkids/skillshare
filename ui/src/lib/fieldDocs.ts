export interface FieldDoc {
  description: string;
  type: string;
  allowedValues?: string[];
  example: string;
}

export const fieldDocs: Record<string, FieldDoc> = {
  'sources.mcp': {
    description: 'Optional MCP YAML file, relative to config.yaml or an absolute path. Its root contains servers. Use this instead of inline mcp.servers; do not define both.',
    type: 'string', example: 'sources:\n  mcp: ./mcp.yaml',
  },
  mcp: {
    description: 'MCP connection settings. Define servers here, or use sources.mcp for a separate file. Skillshare writes client configuration files; it does not run servers.',
    type: 'object', example: 'mcp:\n  servers:\n    docs:\n      url: https://example.com/mcp\n      targets: [claude]',
  },
  'mcp.targets': {
    description: 'Default MCP clients. Individual servers can override this list.',
    type: 'string[]', example: 'targets: [claude, codex, cursor, vscode, opencode, grok]',
  },
  'mcp.servers': {
    description: 'Named MCP connections. Names such as docs are your own labels, not built-in services. Each connection needs a command or a URL.',
    type: 'object', example: 'servers:\n  docs:\n    url: https://example.com/mcp',
  },
  'mcp.servers.command': {
    description: 'Executable launched locally by the client for stdio. Use command or url, never both.',
    type: 'string', example: 'command: my-mcp-server',
  },
  'mcp.servers.args': {
    description: 'Arguments passed to the local command, one list item per argument.',
    type: 'string[]', example: 'args: [--directory, /workspace]',
  },
  'mcp.servers.url': {
    description: 'HTTP or HTTPS MCP endpoint. The client connects to this service. Do not put credentials in the URL.',
    type: 'string', example: 'url: https://example.com/mcp',
  },
  'mcp.servers.transport': {
    description: 'Optional transport. Inferred from command (stdio) or url (streamable-http).',
    type: 'string', allowedValues: ['stdio', 'streamable-http'], example: 'transport: stdio',
  },
  'mcp.servers.targets': {
    description: 'Clients receiving this connection; overrides mcp.targets. An explicit list must not be empty.',
    type: 'string[]', example: 'targets: [claude, codex]',
  },
  'mcp.servers.env': {
    description: 'Environment variables for the local command. Use fromEnv for secrets, rather than storing their values.',
    type: 'object', example: 'env:\n  API_TOKEN:\n    fromEnv: API_TOKEN',
  },
  'mcp.servers.env.fromEnv': {
    description: 'Name of an environment variable available to the client. Skillshare stores a reference and does not read its secret value.',
    type: 'string', example: 'fromEnv: API_TOKEN',
  },
  'mcp.servers.headers': {
    description: 'HTTP request headers. Values can be literals or environment references; sensitive values require fromEnv.',
    type: 'object', example: 'headers:\n  X-API-Key:\n    fromEnv: API_KEY',
  },
  'mcp.servers.headers.fromEnv': {
    description: 'Environment variable supplying this HTTP header when the client connects.',
    type: 'string', example: 'fromEnv: API_KEY',
  },
  'mcp.servers.bearerToken': {
    description: 'Bearer authentication using an environment variable reference. Keep the token itself outside this file.',
    type: 'object', example: 'bearerToken:\n  fromEnv: MCP_TOKEN',
  },
  'mcp.servers.bearerToken.fromEnv': {
    description: 'Environment variable containing the bearer token, available to the client.',
    type: 'string', example: 'fromEnv: MCP_TOKEN',
  },
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
  sources: {
    description: 'Custom source paths. skills, agents, and extras select directories; mcp selects an optional YAML file instead of inline mcp.servers. Directory defaults are .skillshare/<type>/ in project mode and <base>/<type>/ in global mode (or the legacy source fields).',
    type: 'object',
    example: 'sources:\n  skills: ./docs/skills\n  agents: ./docs/agents\n  extras: ./docs/extras',
  },
  'sources.skills': {
    description: 'Custom skills source directory. Relative paths resolve from the project root (project mode); absolute paths and ~ are supported (both modes). Project mode default: .skillshare/skills/. Global mode default: <base>/skills/ (or the legacy `source` field). Project-mode sync rejects configs where this aliases or nests with a target path.',
    type: 'string',
    example: 'sources:\n  skills: ./docs/skills',
  },
  'sources.agents': {
    description: 'Custom agents source directory. Relative paths resolve from the project root (project mode). Project mode default: .skillshare/agents/. Global mode default: <base>/agents/ (or the legacy `agents_source` field).',
    type: 'string',
    example: 'sources:\n  agents: ./docs/agents',
  },
  'sources.extras': {
    description: 'Custom extras source directory. Relative paths resolve from the project root (project mode). Project mode default: .skillshare/extras/. Global mode default: the `extras` sibling of the skills source (or the legacy `extras_source` field).',
    type: 'string',
    example: 'sources:\n  extras: ./docs/extras',
  },
  mode: {
    description: 'Default sync mode for all targets. Can be overridden per target.',
    type: 'string',
    allowedValues: ['merge', 'symlink', 'copy'],
    example: 'mode: merge',
  },
  target_naming: {
    description: 'Default target entry naming strategy for merge/copy sync. "flat" keeps flattened parent directory prefixes; "standard" uses the SKILL.md name and enforces the Agent Skills naming rules.',
    type: 'string',
    allowedValues: ['flat', 'standard'],
    example: 'target_naming: standard',
  },
  git_root: {
    description: 'Directory that "skillshare commit/push/pull" version-controls. "skills" (default) tracks only the skills source; "agents" and "extras" track those sources; "root" tracks the whole skillshare base directory (skills, agents, and extras together, excluding config.yaml). Changing this does NOT move an existing repository — re-run "skillshare init" or move .git yourself.',
    type: 'string',
    allowedValues: ['skills', 'agents', 'extras', 'root'],
    example: 'git_root: root',
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
  azure_hosts: {
    description: 'List of self-hosted Azure DevOps Server instances for skill installation.',
    type: 'string[]',
    example: 'azure_hosts:\n  - azuredevops.mycompany.com',
  },

  // --- Targets ---
  targets: {
    description: 'Map of target AI tools to configure. Each target uses skills: and agents: sub-keys for per-resource-kind configuration.',
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
  'targets.skills.target_naming': {
    description: 'Target entry naming strategy for skills in this target. If omitted, inherits the top-level target_naming. Ignored in symlink mode.',
    type: 'string',
    allowedValues: ['flat', 'standard'],
    example: 'target_naming: standard',
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
    description: 'Agents-specific target configuration. Controls path, sync mode, and include/exclude filters for agents.',
    type: 'object',
    example: 'agents:\n  path: ~/.claude/agents\n  mode: merge\n  include: ["tutor-*"]\n  exclude: ["draft-*"]',
  },
  'targets.agents.path': {
    description: 'Override the target agents directory path. If omitted, the built-in default for this target is used.',
    type: 'string',
    example: 'path: ~/.claude/agents',
  },
  'targets.agents.mode': {
    description: 'Sync mode for agents in this target. If omitted, inherits the top-level mode (defaults to merge).',
    type: 'string',
    allowedValues: ['merge', 'symlink', 'copy'],
    example: 'mode: merge',
  },
  'targets.agents.target_naming': {
    description: 'Target entry naming strategy for agents in this target. If omitted, inherits the top-level target_naming. Ignored in symlink mode.',
    type: 'string',
    allowedValues: ['flat', 'standard'],
    example: 'target_naming: standard',
  },
  'targets.agents.include': {
    description: 'Glob patterns — only matching agents are synced to this target (merge and copy modes).',
    type: 'string[]',
    example: 'include: ["tutor-*", "team-*"]',
  },
  'targets.agents.exclude': {
    description: 'Glob patterns — matching agents are excluded from sync to this target (merge and copy modes).',
    type: 'string[]',
    example: 'exclude: ["draft-*", "wip-*"]',
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
