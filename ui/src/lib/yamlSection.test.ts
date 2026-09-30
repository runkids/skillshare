import { describe, expect, it } from 'vitest';
import { yamlKeyOffset } from './yamlSection';

describe('yamlKeyOffset', () => {
  const doc = 'source: ~/skills\nmcp:\n  hooks: nested\nhooks:\n  entries: {}\n';

  it('finds the top-level key, not a nested one of the same name', () => {
    expect(yamlKeyOffset(doc, 'hooks')).toBe(doc.indexOf('\nhooks:') + 1);
  });

  it('is undefined when the key is absent', () => {
    expect(yamlKeyOffset(doc, 'targets')).toBeUndefined();
  });
});
