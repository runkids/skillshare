import { describe, it, expect } from 'vitest';
import { computeLineDiff } from '../useLineDiff';

describe('computeLineDiff', () => {
  it('returns empty array when strings are identical', () => {
    const result = computeLineDiff('foo\nbar\n', 'foo\nbar\n');
    expect(result.lines).toEqual([]);
    expect(result.changeCount).toBe(0);
  });

  it('detects added lines', () => {
    const result = computeLineDiff('foo\n', 'foo\nbar\n');
    expect(result.lines.some(l => l.type === 'add')).toBe(true);
    expect(result.changeCount).toBeGreaterThan(0);
  });

  it('detects removed lines', () => {
    const result = computeLineDiff('foo\nbar\n', 'foo\n');
    expect(result.lines.some(l => l.type === 'remove')).toBe(true);
  });

  it('detects changed lines', () => {
    const result = computeLineDiff('sync_mode: merge\n', 'sync_mode: symlink\n');
    expect(result.changeCount).toBeGreaterThan(0);
  });

  it('handles empty strings', () => {
    expect(computeLineDiff('', '').changeCount).toBe(0);
    expect(computeLineDiff('', 'foo\n').changeCount).toBeGreaterThan(0);
    expect(computeLineDiff('foo\n', '').changeCount).toBeGreaterThan(0);
  });
});
