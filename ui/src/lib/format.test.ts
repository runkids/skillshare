import { describe, expect, it } from 'vitest';
import { formatVersion } from './format';

describe('formatVersion', () => {
  it.each([
    ['1.10', 'v1.10'],
    ['v2.0.0', 'v2.0.0'],
    ['beta', 'beta'],
    ['', ''],
    [undefined, undefined],
  ])('%s', (input, expected) => {
    expect(formatVersion(input)).toBe(expected);
  });
});
