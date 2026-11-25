import { describe, expect, it } from 'vitest';
import { diffExcerpt } from './diffExcerpt';

const same = (n: number, from = 1) => Array.from({ length: n }, (_, i) => `  line ${from + i}`);

describe('diffExcerpt', () => {
  it('shows a change deep in a long file instead of its unchanged start', () => {
    const lines = [...same(200), '+ new rule', ...same(100, 201)];
    expect(diffExcerpt(lines, 40)).toContain('+ new rule');
  });

  it('keeps three lines around a change and marks the skipped runs', () => {
    const lines = [...same(10), '- old', '+ new', ...same(10, 11)];
    expect(diffExcerpt(lines, 40)).toEqual([
      null,
      '  line 8',
      '  line 9',
      '  line 10',
      '- old',
      '+ new',
      '  line 11',
      '  line 12',
      '  line 13',
      null,
    ]);
  });

  it('cuts a file that changed throughout at the limit', () => {
    const lines = Array.from({ length: 60 }, (_, i) => `+ rule ${i}`);
    expect(diffExcerpt(lines, 40)).toHaveLength(40);
  });
});
