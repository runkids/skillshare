import type { SkillFrontmatter } from './skillMarkdown';

export type SkillFrontmatterField = {
  key: string;
  required: 'No' | 'Recommended' | 'Yes';
  description: string;
};

export type FrontmatterSchema = 'skill' | 'agent';

type FrontmatterFieldQueryOptions = {
  excludeKeys?: string[];
  schema?: FrontmatterSchema;
};

export const SKILL_FRONTMATTER_REFERENCE_URL = 'https://code.claude.com/docs/en/skills#frontmatter-reference';
export const AGENT_FRONTMATTER_REFERENCE_URL = 'https://code.claude.com/docs/en/sub-agents#supported-frontmatter-fields';

export const SKILL_FRONTMATTER_FIELDS: SkillFrontmatterField[] = [
  {
    key: 'name',
    required: 'No',
    description: 'Display name for the skill. If omitted, Claude uses the directory name.',
  },
  {
    key: 'description',
    required: 'Recommended',
    description: 'What the skill does and when to use it. Claude uses this to decide when to apply the skill.',
  },
  {
    key: 'argument-hint',
    required: 'No',
    description: 'Hint shown during autocomplete to indicate expected arguments such as [issue-number] or [filename] [format].',
  },
  {
    key: 'agent',
    required: 'No',
    description: 'Which subagent type to use when forked execution is enabled.',
  },
  {
    key: 'allowed-tools',
    required: 'No',
    description: 'Tools Claude can use without asking permission while this skill is active. Accepts a space-separated string or YAML list.',
  },
  {
    key: 'context',
    required: 'No',
    description: 'Set to fork to run the skill in a forked subagent context instead of the main conversation.',
  },
  {
    key: 'disable-model-invocation',
    required: 'No',
    description: 'Set to true to prevent automatic loading. Use this for workflows you only want to trigger manually.',
  },
  {
    key: 'effort',
    required: 'No',
    description: 'Effort level for the active skill. Overrides the session effort. Options: low, medium, high, max.',
  },
  {
    key: 'hooks',
    required: 'No',
    description: 'Hooks scoped to the skill lifecycle.',
  },
  {
    key: 'model',
    required: 'No',
    description: 'Model to use when this skill is active.',
  },
  {
    key: 'paths',
    required: 'No',
    description: 'Glob patterns that limit when the skill is activated automatically. Accepts a comma-separated string or YAML list.',
  },
  {
    key: 'shell',
    required: 'No',
    description: 'Shell used for inline shell commands. Accepts bash or powershell.',
  },
  {
    key: 'user-invocable',
    required: 'No',
    description: 'Set to false to hide the skill from the slash-command menu. Defaults to true.',
  },
];

export const AGENT_FRONTMATTER_FIELDS: SkillFrontmatterField[] = [
  {
    key: 'name',
    required: 'Yes',
    description: 'Unique identifier for the subagent. Use lowercase letters and hyphens.',
  },
  {
    key: 'description',
    required: 'Yes',
    description: 'When Claude should delegate to this subagent.',
  },
  {
    key: 'tools',
    required: 'No',
    description: 'Tools the subagent can use. Inherits all tools from the parent session if omitted.',
  },
  {
    key: 'disallowedTools',
    required: 'No',
    description: 'Tools to explicitly deny from the inherited or configured tool list.',
  },
  {
    key: 'model',
    required: 'No',
    description: 'Model to use: sonnet, opus, haiku, inherit, or a full model ID.',
  },
  {
    key: 'permissionMode',
    required: 'No',
    description: 'Permission mode for the subagent. Options include default, acceptEdits, auto, dontAsk, bypassPermissions, and plan.',
  },
  {
    key: 'maxTurns',
    required: 'No',
    description: 'Maximum number of agentic turns before the subagent stops.',
  },
  {
    key: 'skills',
    required: 'No',
    description: 'Skills to preload into the subagent context at startup. Accepts a string or YAML list.',
  },
  {
    key: 'mcpServers',
    required: 'No',
    description: 'MCP servers available to this subagent. Accepts configured server names or inline server definitions.',
  },
  {
    key: 'hooks',
    required: 'No',
    description: 'Lifecycle hooks scoped to this subagent.',
  },
  {
    key: 'memory',
    required: 'No',
    description: 'Persistent memory scope for this subagent. Options: user, project, or local.',
  },
  {
    key: 'background',
    required: 'No',
    description: 'Set to true to always run this subagent as a background task.',
  },
  {
    key: 'effort',
    required: 'No',
    description: 'Effort level when this subagent is active. Options: low, medium, high, max.',
  },
  {
    key: 'isolation',
    required: 'No',
    description: 'Set to worktree to run the subagent in a temporary isolated git worktree.',
  },
  {
    key: 'color',
    required: 'No',
    description: 'Display color for the subagent in the task list and transcript.',
  },
  {
    key: 'initialPrompt',
    required: 'No',
    description: 'Initial user turn submitted automatically when this agent runs as the main session agent.',
  },
];

const FRONTMATTER_FIELDS_BY_SCHEMA: Record<FrontmatterSchema, SkillFrontmatterField[]> = {
  skill: SKILL_FRONTMATTER_FIELDS,
  agent: AGENT_FRONTMATTER_FIELDS,
};

const FRONTMATTER_REFERENCE_URLS: Record<FrontmatterSchema, string> = {
  skill: SKILL_FRONTMATTER_REFERENCE_URL,
  agent: AGENT_FRONTMATTER_REFERENCE_URL,
};

export function getFrontmatterFields(schema: FrontmatterSchema = 'skill') {
  return FRONTMATTER_FIELDS_BY_SCHEMA[schema];
}

export function getFrontmatterFieldOrder(schema: FrontmatterSchema = 'skill') {
  return getFrontmatterFields(schema).map((field) => field.key);
}

export function getFrontmatterReferenceUrl(schema: FrontmatterSchema = 'skill') {
  return FRONTMATTER_REFERENCE_URLS[schema];
}

function getReferenceKeySet(schema: FrontmatterSchema = 'skill') {
  return new Set(getFrontmatterFieldOrder(schema));
}

export function isBuiltInFrontmatterKey(key: string, schema: FrontmatterSchema = 'skill'): boolean {
  return getReferenceKeySet(schema).has(key);
}

function getFilteredFrontmatterFields(options?: FrontmatterFieldQueryOptions) {
  const excludedKeys = new Set(options?.excludeKeys ?? []);
  return getFrontmatterFields(options?.schema).filter((field) => !excludedKeys.has(field.key));
}

export function buildFrontmatterTemplate(options?: FrontmatterFieldQueryOptions): string {
  return [
    '---',
    ...getFilteredFrontmatterFields(options).map((field) => `${field.key}:`),
    '---',
  ].join('\n');
}

export function formatFrontmatterValue(value: unknown): string {
  if (value === null || value === undefined || value === '') return 'Not set';
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  if (Array.isArray(value)) {
    return value.length > 0 ? value.map((item) => formatFrontmatterValue(item)).join(', ') : 'Not set';
  }
  return JSON.stringify(value, null, 2);
}

export function isFrontmatterValueSet(value: unknown): boolean {
  if (value === null || value === undefined) return false;
  if (typeof value === 'string') return value.trim().length > 0;
  if (Array.isArray(value)) return value.length > 0;
  return true;
}

export function getReferenceFrontmatterEntries(frontmatter: SkillFrontmatter, options?: FrontmatterFieldQueryOptions) {
  return getFilteredFrontmatterFields(options).map((field) => {
    const value = frontmatter[field.key];
    return {
      ...field,
      value,
      isSet: isFrontmatterValueSet(value),
    };
  });
}

export function getAdditionalFrontmatterEntries(frontmatter: SkillFrontmatter, schema: FrontmatterSchema = 'skill') {
  const referenceKeySet = getReferenceKeySet(schema);

  return Object.entries(frontmatter)
    .filter(([key, value]) => !referenceKeySet.has(key) && isFrontmatterValueSet(value))
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, value]) => ({
      key,
      value,
    }));
}
