import type { RouterOrigin, RouterReport, RouterTopic } from '../../api/contextRouter';
import { ApiError } from '../../api/http';
import { plural } from '../../i18n';

export type TopicState = 'ok' | 'over' | 'unavailable';

/** A size at the cap is within it, as the router judges it. */
export const topicState = (t: Pick<RouterTopic, 'bytes' | 'cap'>): TopicState =>
  t.bytes === null ? 'unavailable' : t.bytes > t.cap ? 'over' : 'ok';

const share = (t: RouterTopic) =>
  t.bytes === null ? Infinity : t.cap > 0 ? t.bytes / t.cap : t.bytes;

/** Topics that need a look first: those without a size, then the largest share of the cap. */
export const orderTopics = (topics: RouterTopic[]): RouterTopic[] =>
  [...topics].sort((a, b) => share(b) - share(a) || a.name.localeCompare(b.name));

type Params = Record<string, string | number>;

/** A sentence to translate. `reason`, when set, is translated first and passed in as `{reason}`. */
export interface RouterProblem {
  key: string;
  params: Params;
  reason?: { key: string; params: Params };
}

/** Exact, since a topic can sit a few bytes under its cap. Digits are not localized: they are compared by eye with the router's own output. */
export const exactBytes = (n: number): string => n.toLocaleString('en-US');

/** What is wrong with this folder's own kernel and topics, most pressing first. Mounts are not looked at. */
export function routerProblems(r: RouterReport): RouterProblem[] {
  if (r.error) {
    const reason =
      r.error.code === 'config_invalid'
        ? { key: 'context.router.reason.invalid', params: { detail: r.error.detail ?? '' } }
        : { key: `context.router.reason.${r.error.detail ?? 'missing'}`, params: {} };
    return [{ key: 'context.router.problem.config', params: {}, reason }];
  }
  const out: RouterProblem[] = [];
  const file = r.kernel?.path ?? '';
  if (r.kernel?.error) {
    out.push({
      key: 'context.router.problem.kernel',
      params: { file },
      reason: { key: `context.router.reason.${r.kernel.error}`, params: {} },
    });
  } else if (r.kernel && r.kernel.bytes !== null && r.kernel.bytes > r.kernel.cap) {
    out.push({
      key: 'context.router.problem.kernelOver',
      params: { file, actual: exactBytes(r.kernel.bytes), cap: exactBytes(r.kernel.cap) },
    });
  }
  for (const t of orderTopics(r.topics)) {
    if (t.bytes !== null && t.bytes > t.cap) {
      out.push({
        key: 'context.router.problem.topicOver',
        params: { topic: t.name, actual: exactBytes(t.bytes), cap: exactBytes(t.cap) },
      });
    }
    for (const s of t.sources) {
      if (!s.error) continue;
      const reason =
        s.error === 'heading'
          ? {
              key: plural('context.router.reason.heading', s.count ?? 0),
              params: { count: s.count ?? 0, heading: s.heading ?? '' },
            }
          : { key: `context.router.reason.${s.error}`, params: {} };
      out.push({
        key: 'context.router.problem.source',
        params: { topic: t.name, path: s.path },
        reason,
      });
    }
    if (t.reason === 'limit') {
      out.push({ key: 'context.router.problem.limit', params: { topic: t.name } });
    }
    if (t.listed === false) {
      out.push({ key: 'context.router.problem.notListed', params: { topic: t.name, file } });
    }
  }
  return out;
}

export const originKey = (o: RouterOrigin): string => `context.router.origin.${o}`;

/** The folder in the URL is no longer one the server lists: a bookmark, or a router that was removed. */
export const isStaleRoot = (error: unknown, root: string): boolean =>
  root !== '' && error instanceof ApiError && error.status === 404;
