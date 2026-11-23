import { describe, expect, it } from 'vitest';
import type { TranslationParams } from '../i18n';
import { targetFailureText } from './targetFailures';

const t = (key: string, params?: TranslationParams) =>
  [key, ...Object.values(params ?? {})].join('|');

describe('targetFailureText', () => {
  it('names a refused file and adds how to fix it', () => {
    expect(
      targetFailureText(t, 1, [
        { detail: 'alpha', refused: true },
        { detail: 'beta: disk full', refused: false },
      ]),
    ).toBe(
      'resources.targets.toast.partialRefused|1|2|alpha; beta: disk full|resources.targets.error.target_frontmatter',
    );
  });

  it('lists other failures without the frontmatter hint', () => {
    expect(targetFailureText(t, 0, [{ detail: 'beta: disk full', refused: false }])).toBe(
      'resources.targets.toast.partialDetails|0|1|beta: disk full',
    );
  });
});
