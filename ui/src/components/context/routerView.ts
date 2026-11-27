import type { RouterOrigin, RouterReport, RouterTopic } from '../../api/contextRouter';
import { ApiError } from '../../api/http';
import { plural } from '../../i18n';

export type TopicState = 'ok' | 'over' | 'unavailable';

/** A size at the cap is within it, as the router judges it. */
export const topicState = (t: Pick<RouterTopic, 'bytes' | 'cap'>): TopicState =>
  t.bytes === null ? 'unavailable' : t.bytes > t.cap ? 'over' : 'ok';

type Params = Record<string, string | number>;
type Sentence = { key: string; params: Params };

/** A sentence to translate. `reason`, when set, is translated first and passed in as `{reason}`. */
export interface RouterProblem {
  key: string;
  params: Params;
  reason?: Sentence;
}

/** Exact, since a topic can sit a few bytes under its cap. Digits are not localized: they are compared by eye with the router's own output. */
export const exactBytes = (n: number): string => n.toLocaleString('en-US');

/** What is wrong with this folder's own kernel and topics, in the config's order, which is the order the router prints. Mounts are not looked at. */
export function routerProblems(r: RouterReport): RouterProblem[] {
  if (r.error) {
    const reason: Sentence =
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
  for (const t of r.topics) {
    if (t.bytes !== null && t.bytes > t.cap) {
      out.push({
        key: 'context.router.problem.topicOver',
        params: { topic: t.name, actual: exactBytes(t.bytes), cap: exactBytes(t.cap) },
      });
    }
    for (const s of t.sources) {
      if (!s.error) continue;
      const reason: Sentence =
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

/** The header line: which script the folder carries and whose size rule was applied. */
export const versionLine = (
  r: Pick<RouterReport, 'scriptVersion' | 'semanticsVersion'>,
): { key: string; params: Params } =>
  r.scriptVersion
    ? {
        key: 'context.router.version.known',
        params: { version: r.scriptVersion, semantics: r.semanticsVersion },
      }
    : { key: 'context.router.version.unknown', params: { semantics: r.semanticsVersion } };

/** The note a version mismatch needs, or null when the script's rule is the one applied. */
export const versionNote = (
  r: Pick<RouterReport, 'scriptVersion' | 'semanticsVersion' | 'versionStatus'>,
): { key: string; params: Params } | null =>
  r.versionStatus === 'ok'
    ? null
    : {
        key: `context.router.version.note.${r.versionStatus}`,
        params: { version: r.scriptVersion, semantics: r.semanticsVersion },
      };
