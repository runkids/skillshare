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

const expectedAgentOrderedKeys = [
  'name',
  'description',
  'tools',
  'disallowedTools',
  'model',
  'permissionMode',
  'maxTurns',
  'skills',
  'mcpServers',
  'hooks',
  'memory',
  'background',
  'effort',
  'isolation',
  'color',
  'initialPrompt',
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

  it('uses the Claude subagent frontmatter schema when requested', () => {
    expect(getReferenceFrontmatterEntries({}, { schema: 'agent' }).map((entry) => entry.key)).toEqual(expectedAgentOrderedKeys);
  });

  it('builds the agent template in the same order as the agent reference entries', () => {
    expect(buildFrontmatterTemplate({ schema: 'agent' })).toBe([
      '---',
      ...expectedAgentOrderedKeys.map((key) => `${key}:`),
      '---',
    ].join('\n'));
  });
});
