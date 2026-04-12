import { describe, expect, it } from 'vitest';
import {
  buildFrontmatterTemplate,
  getReferenceFrontmatterEntries,
} from './skillFrontmatter';

const expectedOrderedKeys = [
  'name',
  'description',
  'argument-hint',
  'agent',
  'allowed-tools',
  'context',
  'disable-model-invocation',
  'effort',
  'hooks',
  'model',
  'paths',
  'shell',
  'user-invocable',
];

describe('skillFrontmatter ordering', () => {
  it('keeps reference entries in the user-facing field order', () => {
    expect(getReferenceFrontmatterEntries({}).map((entry) => entry.key)).toEqual(expectedOrderedKeys);
  });

  it('builds the template in the same order as the reference entries', () => {
    expect(buildFrontmatterTemplate()).toBe([
      '---',
      ...expectedOrderedKeys.map((key) => `${key}:`),
      '---',
    ].join('\n'));
  });

  it('can exclude manifest fields from the workspace reference list', () => {
    expect(
      getReferenceFrontmatterEntries({}, { excludeKeys: ['name', 'description'] }).map((entry) => entry.key),
    ).toEqual(expectedOrderedKeys.slice(2));
  });

  it('can exclude manifest fields from the workspace template block', () => {
    expect(buildFrontmatterTemplate({ excludeKeys: ['name', 'description'] })).toBe([
      '---',
      ...expectedOrderedKeys.slice(2).map((key) => `${key}:`),
      '---',
    ].join('\n'));
  });
});
