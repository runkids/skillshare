import type { FieldDoc } from './fieldDocs';

export const auditFieldDocs: Record<string, FieldDoc> = {
  rules: {
    description: 'List of audit rule definitions. Each rule can define a new custom rule or override a built-in rule by matching its ID.',
    type: 'yamlRule[]',
    example: 'rules:\n  - id: no-secrets\n    severity: CRITICAL\n    pattern: secrets\n    regex: \'(?i)(api_key|secret)\\s*=\\s*\\S+\'',
  },
  'rules.id': {
    description: 'Unique identifier for the rule. Use a built-in rule ID to override its settings, or a new ID to define a custom rule.',
    type: 'string',
    example: 'id: no-shell-exec',
  },
  'rules.severity': {
    description: 'Severity level assigned to findings from this rule. Aliases are also accepted (CRIT/C for CRITICAL, H for HIGH, MED/M for MEDIUM, L for LOW, I for INFO).',
    type: 'string',
    allowedValues: ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW', 'INFO'],
    example: 'severity: HIGH',
  },
  'rules.pattern': {
    description: 'Pattern group name used to categorize this rule. Groups related rules together for filtering and reporting.',
    type: 'string',
    example: 'pattern: secrets',
  },
  'rules.message': {
    description: 'Human-readable description shown when this rule triggers a finding.',
    type: 'string',
    example: 'message: Possible hardcoded secret detected',
  },
  'rules.regex': {
    description: 'Regular expression pattern to match against skill content. Required when defining a new custom rule.',
    type: 'string',
    example: 'regex: \'(?i)(password|secret)\\s*[=:]\\s*\\S+\'',
  },
  'rules.exclude': {
    description: 'Optional regular expression to exclude lines from matching. Lines matching this pattern are ignored even if they match the main regex.',
    type: 'string',
    example: 'exclude: \'#.*example\'',
  },
  'rules.enabled': {
    description: 'Enable or disable this rule. Set to false to disable a built-in rule without removing it.',
    type: 'boolean',
    allowedValues: ['true', 'false'],
    example: 'enabled: false',
  },
};
