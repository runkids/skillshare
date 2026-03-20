// ui/src/hooks/__tests__/useYamlValidation.test.ts
import { describe, it, expect } from 'vitest';
import { validateYaml } from '../useYamlValidation';

describe('validateYaml', () => {
  it('returns empty array for valid YAML', () => {
    const errors = validateYaml('sync_mode: merge\n');
    expect(errors).toEqual([]);
  });

  it('returns syntax error with line number for invalid YAML', () => {
    const errors = validateYaml('foo:\n  bar\n  baz: invalid');
    expect(errors.length).toBeGreaterThan(0);
    expect(errors[0].severity).toBe('error');
    expect(errors[0].line).toBeGreaterThan(0);
  });

  it('returns empty array for empty input', () => {
    const errors = validateYaml('');
    expect(errors).toEqual([]);
  });

  it('accepts any target name without warnings', () => {
    const yaml = 'targets:\n  my-custom-target:\n    mode: merge\n';
    const errors = validateYaml(yaml);
    expect(errors).toEqual([]);
  });

  it('detects invalid sync_mode values', () => {
    const yaml = 'sync_mode: invalid_mode\n';
    const errors = validateYaml(yaml);
    expect(errors.length).toBe(1);
    expect(errors[0].severity).toBe('warning');
    expect(errors[0].message).toContain('invalid_mode');
  });

  it('detects invalid per-target mode', () => {
    const yaml = 'targets:\n  claude:\n    mode: bad\n';
    const errors = validateYaml(yaml);
    expect(errors.length).toBe(1);
    expect(errors[0].message).toContain('bad');
  });
});
