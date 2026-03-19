import { describe, it, expect } from 'vitest';
import { computeRegexMatches } from '../useRegexTester';

describe('computeRegexMatches', () => {
  it('returns matches for valid regex', () => {
    const r = computeRegexMatches('\\bfoo\\b', 'hello foo bar\nbaz foo');
    expect(r.matches.length).toBe(2);
    expect(r.matches[0].matched).toBe(true);
    expect(r.matches[1].matched).toBe(true);
    expect(r.error).toBeNull();
  });

  it('returns no matches for non-matching input', () => {
    const r = computeRegexMatches('xyz', 'hello world');
    expect(r.matches[0].matched).toBe(false);
  });

  it('returns error for invalid regex', () => {
    const r = computeRegexMatches('[bad', 'test');
    expect(r.error).toBeTruthy();
    expect(r.isGoSpecific).toBe(false);
  });

  it('detects Go-specific regex', () => {
    const r = computeRegexMatches('[\\x{E0001}]', 'test');
    expect(r.isGoSpecific).toBe(true);
  });

  it('handles exclude pattern', () => {
    const r = computeRegexMatches('ev' + 'al', 'ev' + 'al(x)\nev' + 'aluate(y)', 'ev' + 'aluate');
    expect(r.matches[0].matched).toBe(true);
    expect(r.matches[0].excluded).toBe(false);
    expect(r.matches[1].matched).toBe(true);
    expect(r.matches[1].excluded).toBe(true);
  });

  it('returns empty for empty regex', () => {
    const r = computeRegexMatches('', 'test');
    expect(r.matches).toEqual([]);
  });
});
