import { describe, it, expect } from 'vitest';
import { validateAuditYaml } from '../useAuditYamlValidation';

describe('validateAuditYaml', () => {
  it('returns empty for valid YAML', () => {
    const y = 'rules:\n  - id: test\n    severity: HIGH\n    pattern: test\n    message: "test"\n    regex: "foo"';
    expect(validateAuditYaml(y)).toEqual([]);
  });

  it('returns syntax error for invalid YAML', () => {
    const errors = validateAuditYaml('rules:\n  bad yaml\n  ::');
    expect(errors.length).toBeGreaterThan(0);
    expect(errors[0].severity).toBe('error');
  });

  it('returns empty for empty input', () => {
    expect(validateAuditYaml('')).toEqual([]);
  });

  it('warns on invalid severity', () => {
    const y = 'rules:\n  - id: t\n    severity: ULTRA\n    pattern: t\n    message: "t"\n    regex: "f"';
    const errors = validateAuditYaml(y);
    expect(errors.some(e => e.message.includes('ULTRA'))).toBe(true);
  });

  it('accepts severity aliases', () => {
    const y = 'rules:\n  - id: t\n    severity: CRIT\n    pattern: t\n    message: "t"\n    regex: "f"';
    expect(validateAuditYaml(y)).toEqual([]);
  });

  it('warns on invalid regex', () => {
    const y = 'rules:\n  - id: t\n    severity: HIGH\n    pattern: t\n    message: "t"\n    regex: "[invalid"';
    const errors = validateAuditYaml(y);
    expect(errors.some(e => e.message.toLowerCase().includes('regex'))).toBe(true);
  });

  it('skips Go-specific regex patterns', () => {
    const y = 'rules:\n  - id: t\n    severity: HIGH\n    pattern: t\n    message: "t"\n    regex: "[\\\\x{E0001}-\\\\x{E007F}]"';
    const errors = validateAuditYaml(y);
    expect(errors.filter(e => e.message.toLowerCase().includes('regex'))).toEqual([]);
  });
});
