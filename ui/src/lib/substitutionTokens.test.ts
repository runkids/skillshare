import { describe, expect, it } from 'vitest';
import {
  getFirstSubstitutionTokenMatch,
  isFullSubstitutionToken,
} from './substitutionTokens';

describe('substitutionTokens', () => {
  it('finds the first substitution token inside surrounding text', () => {
    expect(getFirstSubstitutionTokenMatch('Release $ARGUMENTS before Friday.')).toEqual({
      start: 8,
      end: 18,
      value: '$ARGUMENTS',
    });
  });

  it('supports braced and indexed substitution token forms', () => {
    expect(getFirstSubstitutionTokenMatch('Use ${FILE[path]} next.')).toEqual({
      start: 4,
      end: 17,
      value: '${FILE[path]}',
    });
  });

  it('distinguishes whole substitution tokens from ordinary inline code', () => {
    expect(isFullSubstitutionToken('$ARGUMENTS')).toBe(true);
    expect(isFullSubstitutionToken('v0.19.0')).toBe(false);
  });
});
