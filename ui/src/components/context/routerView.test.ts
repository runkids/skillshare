import { describe, expect, it } from 'vitest';
import type { RouterReport, RouterTopic } from '../../api/contextRouter';
import { ApiError } from '../../api/http';
import {
  exactBytes,
  isStaleRoot,
  orderTopics,
  originKey,
  routerProblems,
  topicState,
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
  reference: '19c94f9',
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

describe('orderTopics', () => {
  it('puts topics without a size first, then the largest share of the cap, then by name', () => {
    const topics = [topic('a', 10), topic('b', null), topic('d', 90), topic('c', 90)];
    expect(orderTopics(topics).map((t) => t.name)).toEqual(['b', 'c', 'd', 'a']);
    expect(topics.map((t) => t.name)).toEqual(['a', 'b', 'd', 'c']);
  });

  it('compares shares, not sizes, when caps differ', () => {
    const topics = [topic('big', 500, { cap: 1000 }), topic('small', 9, { cap: 10 })];
    expect(orderTopics(topics).map((t) => t.name)).toEqual(['small', 'big']);
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

  it('reports the kernel, then each topic in display order', () => {
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
      {
        key: 'context.router.problem.topicOver',
        params: { topic: 'over', actual: '16,385', cap: '16,384' },
      },
      { key: 'context.router.problem.notListed', params: { topic: 'hidden', file: 'AGENTS.md' } },
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
