import { describe, expect, it } from 'vitest';
import { splitStack } from './splitStack';

describe('splitStack', () => {
  it('draws only the first max items', () => {
    expect(splitStack(['a', 'b', 'c', 'd', 'e'], 3).shown).toEqual(['a', 'b', 'c']);
  });

  it('counts the items past max', () => {
    expect(splitStack(['a', 'b', 'c', 'd', 'e'], 3).rest).toBe(2);
  });

  it.each([2, 3])('leaves no rest for %i items within a max of 3', (count) => {
    expect(splitStack(['a', 'b', 'c'].slice(0, count), 3).rest).toBe(0);
  });

  it('draws every item without a max', () => {
    expect(splitStack(['a', 'b', 'c'])).toEqual({ shown: ['a', 'b', 'c'], rest: 0 });
  });
});
