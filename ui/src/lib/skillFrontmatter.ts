import type { SkillFrontmatter } from './skillMarkdown';

export type SkillFrontmatterField = {
  key: string;
  required: 'No' | 'Recommended';
  description: string;
};

type FrontmatterFieldQueryOptions = {
  excludeKeys?: string[];
};

export const SKILL_FRONTMATTER_REFERENCE_URL = 'https://code.claude.com/docs/en/skills#frontmatter-reference';

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

const referenceKeySet = new Set(SKILL_FRONTMATTER_FIELDS.map((field) => field.key));

export function isBuiltInFrontmatterKey(key: string): boolean {
  return referenceKeySet.has(key);
}

function getFilteredFrontmatterFields(options?: FrontmatterFieldQueryOptions) {
  const excludedKeys = new Set(options?.excludeKeys ?? []);
  return SKILL_FRONTMATTER_FIELDS.filter((field) => !excludedKeys.has(field.key));
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

export function getAdditionalFrontmatterEntries(frontmatter: SkillFrontmatter) {
  return Object.entries(frontmatter)
    .filter(([key, value]) => !referenceKeySet.has(key) && isFrontmatterValueSet(value))
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, value]) => ({
      key,
      value,
    }));
}
