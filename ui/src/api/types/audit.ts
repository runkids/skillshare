// Audit types
export interface AuditFinding {
  severity: 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' | 'INFO';
  kind?: 'skill' | 'agent';
  pattern: string;
  message: string;
  file: string;
  line: number;
  snippet: string;
  ruleId?: string;
  analyzer?: string;
  category?: string;
  confidence?: number;
  fingerprint?: string;
}

export interface AuditResult {
  binaryFiles?: number;
  skippedFiles?: number;
  oversizedFiles?: number;
  skillName: string;
  kind?: 'skill' | 'agent';
  findings: AuditFinding[];
  riskScore: number;
  riskLabel: 'clean' | 'low' | 'medium' | 'high' | 'critical';
  threshold: string;
  isBlocked: boolean;
  scanTarget?: string;
}

export interface AuditSummary {
  binaryFiles?: number;
  skippedFiles?: number;
  oversizedFiles?: number;
  total: number;
  passed: number;
  warning: number;
  failed: number;
  critical: number;
  high: number;
  medium: number;
  low: number;
  info: number;
  threshold: string;
  riskScore: number;
  riskLabel: 'clean' | 'low' | 'medium' | 'high' | 'critical';
  byCategory?: Record<string, number>;
  scanErrors?: number;
}

export interface AuditAllResponse {
  results: AuditResult[];
  summary: AuditSummary;
  /** Source links the scan did not follow; incomplete when one could not be read. */
  warnings?: string[];
  incomplete?: boolean;
}

export interface AuditSkillResponse {
  result: AuditResult;
  summary: AuditSummary;
}

export interface AuditRulesResponse {
  exists: boolean;
  raw: string;
  path: string;
}

export interface CompiledRule {
  files?: string[];
  confidence: number;
  confidenceOverridden: boolean;
  id: string;
  severity: string;
  pattern: string;
  message: string;
  regex: string;
  exclude?: string;
  enabled: boolean;
  source: string;
}

export interface PatternGroup {
  pattern: string;
  total: number;
  enabled: number;
  disabled: number;
  maxSeverity: string;
}

export interface CompiledRulesResponse {
  rules: CompiledRule[];
  patterns: PatternGroup[];
  /** Preset the scan runs under: default, strict, or permissive. */
  profile: string;
}

export interface AuditPolicy {
  threshold: string;
  profile: string;
}
