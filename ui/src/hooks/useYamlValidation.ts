// ui/src/hooks/useYamlValidation.ts
import { useState, useEffect, useCallback, useRef } from 'react';
import { parseDocument } from 'yaml';

export interface ValidationError {
  line: number;
  message: string;
  severity: 'error' | 'warning';
}

const VALID_SYNC_MODES = ['merge', 'symlink', 'copy'];
const VALID_BLOCK_THRESHOLDS = ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW', 'INFO'];
const VALID_AUDIT_PROFILES = ['default', 'strict', 'permissive'];
const VALID_DEDUPE_MODES = ['legacy', 'global'];

/** Helper: push a warning if value is not in allowed list */
function validateEnum(
  errors: ValidationError[],
  sourceLines: string[],
  value: unknown,
  key: string,
  allowed: string[],
  label: string,
  afterKey?: string,
) {
  if (typeof value === 'string' && !allowed.includes(value)) {
    errors.push({
      line: findKeyLine(sourceLines, key, afterKey),
      message: `Invalid ${label} "${value}". Valid: ${allowed.join(', ')}`,
      severity: 'warning',
    });
  }
}

/** Pure validation function (testable without React) */
export function validateYaml(
  source: string,
): ValidationError[] {
  if (!source.trim()) return [];

  const errors: ValidationError[] = [];
  const doc = parseDocument(source);

  // YAML syntax errors
  for (const err of doc.errors) {
    const line = err.linePos?.[0]?.line ?? 1;
    errors.push({ line, message: err.message, severity: 'error' });
  }

  // Skip schema validation if syntax errors exist
  if (errors.length > 0) return errors;

  const parsed = doc.toJS();
  if (!parsed || typeof parsed !== 'object') return errors;

  const sourceLines = source.split('\n');

  // Validate top-level mode (and legacy sync_mode alias)
  validateEnum(errors, sourceLines, parsed.mode, 'mode', VALID_SYNC_MODES, 'mode');
  validateEnum(errors, sourceLines, parsed.sync_mode, 'sync_mode', VALID_SYNC_MODES, 'sync_mode');

  // Validate per-target mode
  if (parsed.targets) {
    for (const [name, cfg] of Object.entries(parsed.targets)) {
      if (cfg && typeof cfg === 'object' && 'mode' in cfg) {
        validateEnum(errors, sourceLines, (cfg as Record<string, unknown>).mode, 'mode', VALID_SYNC_MODES, `mode for target "${name}"`, name);
      }
    }
  }

  // Validate audit config
  if (parsed.audit && typeof parsed.audit === 'object') {
    validateEnum(errors, sourceLines, parsed.audit.block_threshold, 'block_threshold', VALID_BLOCK_THRESHOLDS, 'block_threshold', 'audit');
    validateEnum(errors, sourceLines, parsed.audit.profile, 'profile', VALID_AUDIT_PROFILES, 'audit profile', 'audit');
    validateEnum(errors, sourceLines, parsed.audit.dedupe_mode, 'dedupe_mode', VALID_DEDUPE_MODES, 'dedupe_mode', 'audit');
  }

  return errors;
}

/** Find line number of a key in YAML source lines */
function findKeyLine(sourceLines: string[], key: string, afterKey?: string): number {
  let afterFound = !afterKey;
  for (let i = 0; i < sourceLines.length; i++) {
    if (afterKey && sourceLines[i].trimStart().startsWith(`${afterKey}:`)) {
      afterFound = true;
      continue;
    }
    if (afterFound && sourceLines[i].trimStart().startsWith(`${key}:`)) {
      return i + 1;
    }
  }
  return 1;
}

/** React hook: debounced YAML validation */
export function useYamlValidation(source: string) {
  const [errors, setErrors] = useState<ValidationError[]>([]);
  const timerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  const validate = useCallback(() => {
    setErrors(validateYaml(source));
  }, [source]);

  useEffect(() => {
    clearTimeout(timerRef.current);
    timerRef.current = setTimeout(validate, 300);
    return () => clearTimeout(timerRef.current);
  }, [validate]);

  return { errors, hasErrors: errors.some(e => e.severity === 'error') };
}
