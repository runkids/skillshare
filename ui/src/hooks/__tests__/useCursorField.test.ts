import { describe, it, expect } from 'vitest';
import { resolveFieldPath } from '../useCursorField';

describe('resolveFieldPath', () => {
  const sampleLines = [
    'sync_mode: merge',
    'targets:',
    '  claude:',
    '    mode: merge',
    '    include:',
    '      - skill-a',
    '  cursor:',
    '    mode: symlink',
    'extras:',
    '  source: ~/extras',
  ];

  it('resolves root-level key', () => {
    expect(resolveFieldPath(sampleLines, 0)).toBe('sync_mode');
  });

  it('resolves nested key under targets', () => {
    expect(resolveFieldPath(sampleLines, 3)).toBe('targets.claude.mode');
  });

  it('resolves first-level nested key', () => {
    expect(resolveFieldPath(sampleLines, 2)).toBe('targets.claude');
  });

  it('resolves second target', () => {
    expect(resolveFieldPath(sampleLines, 7)).toBe('targets.cursor.mode');
  });

  it('returns null for blank line', () => {
    expect(resolveFieldPath(['', 'foo: bar'], 0)).toBeNull();
  });

  it('returns null for out-of-bounds index', () => {
    expect(resolveFieldPath(sampleLines, -1)).toBeNull();
    expect(resolveFieldPath(sampleLines, 100)).toBeNull();
  });

  it('returns null for bare list values (no key)', () => {
    expect(resolveFieldPath(sampleLines, 5)).toBeNull();
  });

  // YAML list items with keys: "- name: rules"
  const extrasLines = [
    'extras:',              // 0
    '  - name: rules',     // 1
    '    targets:',         // 2
    '      - path: ~/.x',  // 3
    '        mode: merge',  // 4
  ];

  it('resolves list item key under extras', () => {
    expect(resolveFieldPath(extrasLines, 1)).toBe('extras.name');
  });

  it('resolves nested key under extras list item', () => {
    // resolveFieldPath returns the raw indentation-based path;
    // FieldDocs handles fallback (extras.name.targets.path → extras.targets.path)
    expect(resolveFieldPath(extrasLines, 3)).toBe('extras.name.targets.path');
  });

  it('resolves deeply nested key', () => {
    expect(resolveFieldPath(extrasLines, 4)).toBe('extras.name.targets.path.mode');
  });
});
