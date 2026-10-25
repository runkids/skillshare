import { describe, expect, it } from 'vitest';
import { updateCheckTitle } from './updateCheckSummary';

describe('updateCheckTitle', () => {
  it.each([
    [true, true, 0, 'update.check.error'],
    [true, true, 2, 'update.check.error'],
    [true, false, 0, 'update.check.error'],
    [false, false, 0, 'update.tab.notChecked'],
    [false, true, 2, 'update.hero.available'],
    [false, true, 0, 'update.hero.allCurrent'],
  ] as const)('failed=%s checked=%s available=%s', (failed, checked, available, expected) => {
    expect(updateCheckTitle(failed, checked, available)).toBe(expected);
  });
});
