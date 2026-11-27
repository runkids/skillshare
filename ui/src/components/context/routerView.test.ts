import { describe, expect, it } from 'vitest';
import type { RouterReport, RouterTopic } from '../../api/contextRouter';
import { ApiError } from '../../api/http';
import {
  exactBytes,
  isStaleRoot,
  originKey,
  routerProblems,
  topicState,
  versionLine,
  versionNote,
} from './routerView';

const topic = (
  name: string,
  bytes: number | null,
  extra: Partial<RouterTopic> = {},
): RouterTopic => ({
  name,
  description: '',
  bytes,
  cap: 100,
  listed: true,
  sources: [],
  ...extra,
});

const report = (extra: Partial<RouterReport> = {}): RouterReport => ({
  root: '/r',
  config: 'wiki/ai-context.json',
  scriptVersion: '0.6.0',
  semanticsVersion: '0.6.0',
  versionStatus: 'ok',
  kernel: { path: 'AGENTS.md', bytes: 10, cap: 100 },
  topics: [],
  truncated: false,
  mounts: [],
  ...extra,
});

describe('topicState', () => {
  it('counts a size at the cap as within it and one byte more as over', () => {
    expect(topicState({ bytes: 16384, cap: 16384 })).toBe('ok');
    expect(topicState({ bytes: 16385, cap: 16384 })).toBe('over');
  });

  it('gives no verdict without a size', () => {
    expect(topicState({ bytes: null, cap: 16384 })).toBe('unavailable');
  });
});

describe('routerProblems', () => {
  it('finds nothing in a clean router', () => {
    expect(routerProblems(report({ topics: [topic('a', 100)] }))).toEqual([]);
  });

  it('reports an unusable config alone', () => {
    expect(
      routerProblems(
        report({
          error: { code: 'config_invalid', detail: 'topics: expected an object' },
          kernel: undefined,
        }),
      ),
    ).toEqual([
      {
        key: 'context.router.problem.config',
        params: {},
        reason: {
          key: 'context.router.reason.invalid',
          params: { detail: 'topics: expected an object' },
        },
      },
    ]);
    expect(
      routerProblems(
        report({ error: { code: 'config_unreadable', detail: 'outside' }, kernel: undefined }),
      )[0].reason,
    ).toEqual({ key: 'context.router.reason.outside', params: {} });
  });

  it('reports the kernel, then each topic in the order the config lists it', () => {
    const problems = routerProblems(
      report({
        kernel: { path: 'AGENTS.md', bytes: 8193, cap: 8192 },
        topics: [
          topic('hidden', 10, { listed: false }),
          topic('over', 16385, { cap: 16384 }),
          topic('broken', null, {
            reason: 'source',
            sources: [
              { path: 'wiki/a.md', bytes: 5 },
              { path: 'wiki/b.md', heading: 'Setup', bytes: null, error: 'heading', count: 0 },
              { path: 'wiki/c.md', bytes: null, error: 'too_large' },
            ],
          }),
          topic('cut', null, { reason: 'limit' }),
        ],
      }),
    );
    expect(problems).toEqual([
      {
        key: 'context.router.problem.kernelOver',
        params: { file: 'AGENTS.md', actual: '8,193', cap: '8,192' },
      },
      { key: 'context.router.problem.notListed', params: { topic: 'hidden', file: 'AGENTS.md' } },
      {
        key: 'context.router.problem.topicOver',
        params: { topic: 'over', actual: '16,385', cap: '16,384' },
      },
      {
        key: 'context.router.problem.source',
        params: { topic: 'broken', path: 'wiki/b.md' },
        reason: {
          key: 'context.router.reason.heading.other',
          params: { count: 0, heading: 'Setup' },
        },
      },
      {
        key: 'context.router.problem.source',
        params: { topic: 'broken', path: 'wiki/c.md' },
        reason: { key: 'context.router.reason.too_large', params: {} },
      },
      { key: 'context.router.problem.limit', params: { topic: 'cut' } },
    ]);
  });

  it('says why the kernel is unreadable and does not guess which topics it names', () => {
    expect(
      routerProblems(
        report({
          kernel: { path: 'AGENTS.md', bytes: null, cap: 8192, error: 'missing' },
          topics: [topic('a', 10, { listed: null })],
        }),
      ),
    ).toEqual([
      {
        key: 'context.router.problem.kernel',
        params: { file: 'AGENTS.md' },
        reason: { key: 'context.router.reason.missing', params: {} },
      },
    ]);
  });

  it('counts a duplicated heading', () => {
    const [problem] = routerProblems(
      report({
        topics: [
          topic('a', null, {
            reason: 'source',
            sources: [{ path: 'p.md', heading: 'H', bytes: null, error: 'heading', count: 2 }],
          }),
        ],
      }),
    );
    expect(problem.reason).toEqual({
      key: 'context.router.reason.heading.other',
      params: { count: 2, heading: 'H' },
    });
  });
});

describe('isStaleRoot', () => {
  it('is a 404 for a folder the URL asked for', () => {
    expect(isStaleRoot(new ApiError(404, 'gone'), '/a')).toBe(true);
    expect(isStaleRoot(new ApiError(404, 'gone'), '')).toBe(false);
    expect(isStaleRoot(new ApiError(500, 'boom'), '/a')).toBe(false);
    expect(isStaleRoot(new Error('x'), '/a')).toBe(false);
  });
});

it('formats bytes exactly and names the origin key', () => {
  expect(exactBytes(16377)).toBe('16,377');
  expect(originKey('skills_source')).toBe('context.router.origin.skills_source');
});

describe('router script version', () => {
  it('names the folder script and the rule applied', () => {
    expect(versionLine({ scriptVersion: '0.2.1', semanticsVersion: '0.2.1' })).toEqual({
      key: 'context.router.version.known',
      params: { version: '0.2.1', semantics: '0.2.1' },
    });
    expect(versionLine({ scriptVersion: '', semanticsVersion: '0.6.0' })).toEqual({
      key: 'context.router.version.unknown',
      params: { semantics: '0.6.0' },
    });
  });

  it('adds a note for every status but a matching script', () => {
    const report = { scriptVersion: '0.7.0', semanticsVersion: '0.6.0' };
    expect(versionNote({ ...report, versionStatus: 'ok' })).toBeNull();
    for (const status of ['legacy', 'newer', 'unknown'] as const)
      expect(versionNote({ ...report, versionStatus: status })).toEqual({
        key: `context.router.version.note.${status}`,
        params: { version: '0.7.0', semantics: '0.6.0' },
      });
  });
});
