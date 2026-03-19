// ui/src/hooks/useAuditYamlValidation.ts
import { useState, useEffect, useRef } from 'react';
import { parseDocument } from 'yaml';
import type { ValidationError } from './useYamlValidation';

export type { ValidationError };

const VALID_SEVERITIES = [
  'CRITICAL', 'HIGH', 'MEDIUM', 'LOW', 'INFO',
  'CRIT', 'C', 'H', 'MED', 'M', 'L', 'I',
];

/** Find the line number of a field within a specific rule (by index) in the source */
function findRuleFieldLine(
  sourceLines: string[],
  ruleIndex: number,
  fieldName: string,
): number {
  // Find the nth rule entry (lines starting with "  - ")
  let ruleCount = -1;
  let ruleStart = -1;
  for (let i = 0; i < sourceLines.length; i++) {
    if (/^\s+-\s/.test(sourceLines[i])) {
      ruleCount++;
      if (ruleCount === ruleIndex) {
        ruleStart = i;
        break;
      }
    }
  }
  if (ruleStart === -1) return 1;

  // From ruleStart, find the field line
  for (let i = ruleStart; i < sourceLines.length; i++) {
    // Stop at next rule entry
    if (i > ruleStart && /^\s+-\s/.test(sourceLines[i])) break;
    if (sourceLines[i].trimStart().startsWith(`${fieldName}:`)) {
      return i + 1;
    }
  }
  return ruleStart + 1;
}

/** Returns true if the regex pattern uses Go-specific syntax that JS cannot parse */
function isGoSpecificRegex(pattern: string): boolean {
  // Go uses \x{HHHH} for Unicode code points and (?flags) prefixes
  return pattern.includes('\\x{') || /^\(\?[a-zA-Z]+\)/.test(pattern);
}

/** Pure validation function for audit YAML (testable without React) */
export function validateAuditYaml(source: string): ValidationError[] {
  if (!source.trim()) return [];

  const errors: ValidationError[] = [];
  const doc = parseDocument(source);

  // Collect YAML syntax errors
  for (const err of doc.errors) {
    const line = err.linePos?.[0]?.line ?? 1;
    errors.push({ line, message: err.message, severity: 'error' });
  }

  // Skip schema validation if syntax errors exist
  if (errors.length > 0) return errors;

  const parsed = doc.toJS();
  if (!parsed || typeof parsed !== 'object') return errors;

  const sourceLines = source.split('\n');
  const rules = parsed.rules;

  if (!Array.isArray(rules)) return errors;

  rules.forEach((rule: unknown, index: number) => {
    if (!rule || typeof rule !== 'object') return;
    const r = rule as Record<string, unknown>;

    // Validate severity if present
    if ('severity' in r) {
      const severity = r.severity;
      if (typeof severity === 'string' && !VALID_SEVERITIES.includes(severity)) {
        const line = findRuleFieldLine(sourceLines, index, 'severity');
        errors.push({
          line,
          message: `Invalid severity "${severity}". Valid: ${VALID_SEVERITIES.join(', ')}`,
          severity: 'warning',
        });
      }
    }

    // Validate regex if present
    if ('regex' in r) {
      const regex = r.regex;
      if (typeof regex === 'string' && !isGoSpecificRegex(regex)) {
        try {
          new RegExp(regex);
        } catch (e) {
          const line = findRuleFieldLine(sourceLines, index, 'regex');
          const msg = e instanceof Error ? e.message : String(e);
          errors.push({
            line,
            message: `Invalid regex: ${msg}`,
            severity: 'warning',
          });
        }
      }
    }
  });

  return errors;
}

/** React hook: debounced audit YAML validation */
export function useAuditYamlValidation(source: string) {
  const [errors, setErrors] = useState<ValidationError[]>([]);
  const timerRef = useRef<ReturnType<typeof setTimeout>>();

  useEffect(() => {
    clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => {
      setErrors(validateAuditYaml(source));
    }, 300);
    return () => clearTimeout(timerRef.current);
  }, [source]);

  return { errors, hasErrors: errors.some(e => e.severity === 'error') };
}
