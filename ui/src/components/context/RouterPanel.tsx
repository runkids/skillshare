import { useEffect } from 'react';
import { createPortal } from 'react-dom';
import { useSearchParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { AlertCircle, AlertTriangle, ExternalLink, FileText, Signpost } from 'lucide-react';
import { contextRouterApi } from '../../api/contextRouter';
import type { RouterSource, RouterTopic } from '../../api/contextRouter';
import EmptyState from '../EmptyState';
import { Select } from '../Select';
import Spinner from '../Spinner';
import { useAppContext } from '../../context/AppContext';
import { plural, useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import { queryKeys, staleTimes } from '../../lib/queryKeys';
import {
  exactBytes,
  isStaleRoot,
  originKey,
  routerProblems,
  topicState,
  versionLine,
  versionNote,
} from './routerView';
import type { RouterProblem } from './routerView';

const ROUTER_README = 'https://github.com/runkids/agents-context-router#readme';

/** One folder's agents-context-router setup, measured against its byte caps. Read-only: the folder's own files are read, its script is never run. The folder picker goes in the page's `actions` slot. */
export default function RouterPanel({ actions }: { actions: HTMLElement | null }) {
  const t = useT();
  const { isProjectMode } = useAppContext();
  // The folder lives in the URL so the tab can be linked to.
  const [params, setParams] = useSearchParams();
  const root = params.get('root') ?? '';
  const setRoot = (next: string) =>
    setParams(
      (p) => {
        if (next) p.set('root', next);
        else p.delete('root');
        p.delete('gone');
        return p;
      },
      { replace: true },
    );
  const { data, isPending, error } = useQuery({
    queryKey: queryKeys.contextRouter.root(root),
    queryFn: ({ signal }) => contextRouterApi.get(root, signal),
    staleTime: staleTimes.check,
    retry: false,
  });
  // A bookmarked folder that lost its router: fall back to the default folder and say so.
  // The lost folder moves to `gone`, so the note survives the reload of the default.
  const stale = params.get('gone') ?? '';
  const staleNow = isStaleRoot(error, root);
  useEffect(() => {
    if (!staleNow) return;
    setParams(
      (p) => {
        p.set('gone', root);
        p.delete('root');
        return p;
      },
      { replace: true },
    );
  }, [staleNow, root, setParams]);

  if (isPending || staleNow)
    return (
      <div className="ss-note inf !items-center">
        <Spinner size="sm" />
        {t('context.router.loading')}
      </div>
    );
  if (error)
    return (
      <div className="ss-note bad">
        <AlertCircle size={16} />
        <span className="flex-1 break-words">{(error as Error).message}</span>
      </div>
    );

  const roots = data?.roots ?? [];
  const router = data?.router;
  const current = roots.find((r) => r.path === data?.selected);
  if (!router || !current)
    return (
      <EmptyState
        icon={Signpost}
        title={t('context.router.empty.title')}
        description={t(
          isProjectMode ? 'context.router.empty.project' : 'context.router.empty.global',
        )}
        action={
          <a className="ss-btn" href={ROUTER_README} target="_blank" rel="noreferrer">
            {t('context.router.learn')}
            <ExternalLink size={13} />
          </a>
        }
      />
    );

  const say = (p: RouterProblem) =>
    t(p.key, { ...p.params, reason: p.reason ? t(p.reason.key, p.reason.params) : '' });
  const problems = routerProblems(router);
  const kernel = router.kernel;
  const kernelOver = !!kernel && kernel.bytes !== null && kernel.bytes > kernel.cap;
  // The config's order, which is the order the router's own list and check print.
  const topics = router.topics;
  const version = versionLine(router);
  const note = versionNote(router);

  const sourceSize = (s: RouterSource) => {
    if (s.bytes !== null) return `${exactBytes(s.bytes)} B`;
    if (s.error === 'heading')
      return t(plural('context.router.reason.heading', s.count ?? 0), {
        count: s.count ?? 0,
        heading: s.heading ?? '',
      });
    return s.error ? t(`context.router.reason.${s.error}`) : t('context.router.unavailable');
  };
  const topicSize = (topic: RouterTopic) => {
    const state = topicState(topic);
    if (topic.bytes === null)
      return (
        <span className="flex w-[330px] justify-end">
          <span className="ss-tag warn">{t('context.router.unavailable')}</span>
        </span>
      );
    return (
      <span className="flex w-[330px] items-center gap-2.5">
        <span className="ss-prog flex-1">
          <i
            style={{
              width: `${topic.cap ? Math.min(100, (topic.bytes / topic.cap) * 100) : 100}%`,
            }}
          />
        </span>
        <span
          className={`w-[150px] text-right font-mono text-[13px] ${state === 'over' ? 'text-warn' : ''}`}
        >
          {exactBytes(topic.bytes)} / {exactBytes(topic.cap)} B
        </span>
      </span>
    );
  };

  return (
    <div className="flex flex-col gap-5">
      {actions &&
        roots.length > 1 &&
        createPortal(
          <Select
            value={current.path}
            onChange={setRoot}
            prefix={t('context.router.folder')}
            className="w-[320px]"
            options={roots.map((r) => ({ value: r.path, label: shortenHome(r.path) }))}
          />,
          actions,
        )}

      {/* Always shown: the server may have picked a folder above a source by itself. */}
      <p className="text-[13px] text-ink-2">
        {t('context.router.folder')}{' '}
        <span className="font-mono text-ink" title={current.path}>
          {shortenHome(current.path)}
        </span>{' '}
        · {t(originKey(current.origin))}
      </p>

      {stale && (
        <div className="ss-note inf">
          <span className="flex-1 break-words">
            {t('context.router.stale', {
              path: shortenHome(stale),
              current: shortenHome(current.path),
            })}
          </span>
        </div>
      )}

      {/* Which script's numbers these are compared with, and whose rule produced them. */}
      <p className="-mt-3 text-[13px] text-ink-2">{t(version.key, version.params)}</p>
      {note && (
        <div className={`ss-note ${router.versionStatus === 'newer' ? 'warn' : 'inf'}`}>
          {router.versionStatus === 'newer' && <AlertTriangle size={16} />}
          <span className="flex-1 break-words">{t(note.key, note.params)}</span>
        </div>
      )}

      {!router.error && kernel && (
        <div className="ss-counts !grid-cols-3">
          <div>
            <span className="flex flex-col gap-[3px]">
              <b className={kernelOver || kernel.bytes === null ? 'text-warn' : ''}>
                {kernel.bytes === null ? (
                  t('context.router.unavailable')
                ) : (
                  <>
                    {exactBytes(kernel.bytes)}
                    <span className="text-[15px] font-normal text-ink-3">
                      {' '}
                      / {exactBytes(kernel.cap)} B
                    </span>
                  </>
                )}
              </b>
              <span className="text-[13px] text-ink-2">
                {t('context.router.stat.kernel', { file: kernel.path })}
              </span>
            </span>
          </div>
          <div>
            <span className="flex flex-col gap-[3px]">
              <b>{topics.length}</b>
              <span className="text-[13px] text-ink-2">
                {t(plural('context.router.stat.topics', topics.length))}
              </span>
            </span>
          </div>
          <div>
            <span className="flex flex-col gap-[3px]">
              <b className={problems.length > 0 ? 'text-warn' : ''}>{problems.length}</b>
              <span className="text-[13px] text-ink-2">
                {t(plural('context.router.stat.problems', problems.length))}
              </span>
            </span>
          </div>
        </div>
      )}

      {problems.map((p, i) => (
        <div key={i} className={`ss-note ${router.error ? 'bad' : 'warn'}`}>
          {router.error ? <AlertCircle size={16} /> : <AlertTriangle size={16} />}
          <span className="flex-1 break-words">{say(p)}</span>
        </div>
      ))}
      {router.mounts.length > 0 && (
        <div className="ss-note inf">
          <span className="flex-1 break-words">
            {t(plural('context.router.mounts', router.mounts.length), {
              count: router.mounts.length,
              names: router.mounts.map((m) => `${m.name} (${m.root})`).join(', '),
            })}
          </span>
        </div>
      )}

      {!router.error && topics.length > 0 && (
        <div className="ss-list">
          <div className="ss-lh max-md:!hidden">
            <span className="w-[34px]" />
            <span className="flex-1">{t('context.router.col.topic')}</span>
            <span className="w-[80px]">{t('context.router.col.sources')}</span>
            <span className="w-[330px]">{t('context.router.col.size')}</span>
          </div>
          {topics.map((topic) => (
            <div key={topic.name} className="ss-r flex-wrap">
              <span className="ss-cat sm">
                <FileText size={14} />
              </span>
              <span className="nm m shrink-0">{topic.name}</span>
              <span
                className="min-w-0 flex-1 truncate text-[13px] text-ink-3"
                title={topic.description}
              >
                {topic.description}
              </span>
              <span className="w-[80px] font-mono text-[13px]">{topic.sources.length}</span>
              {topicSize(topic)}
              <span className="flex w-full flex-col gap-1 pb-1 pl-[46px]">
                {topic.sources.map((s, i) => (
                  <span key={i} className="flex items-baseline gap-3 text-[13px]">
                    <span className="min-w-0 flex-1 truncate font-mono text-ink-2">
                      {s.path}
                      {s.heading ? ` § ${s.heading}` : ''}
                    </span>
                    <span
                      className={`shrink-0 ${s.bytes === null ? 'text-warn' : 'font-mono text-ink-3'}`}
                    >
                      {sourceSize(s)}
                    </span>
                  </span>
                ))}
              </span>
            </div>
          ))}
        </div>
      )}

      {!router.error && (
        <p className="text-[13px] leading-relaxed text-ink-3">{t('context.router.footnote')}</p>
      )}
    </div>
  );
}
