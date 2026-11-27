import { useEffect, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertCircle, ChevronDown, Trash2 } from 'lucide-react';
import { hubRefs } from '../../api/hubDrafts';
import type { HubEntry, HubProblem } from '../../api/hubDrafts';
import { Select } from '../Select';
import { refOptions } from './hubShared';
import { queryKeys } from '../../lib/queryKeys';
import { useT } from '../../i18n';

interface Props {
  entry: HubEntry;
  /** Only problems that still apply: the entry is unchanged since the server reported them. */
  problems: HubProblem[];
  /** Branch or tag the source names, as last known; "" is the default branch, undefined not known yet. */
  version?: string;
  expanded: boolean;
  onToggle: () => void;
  onChange: (key: string, value: unknown) => void;
  /** A version was picked: the source rewritten to it, and the version itself. */
  onVersion: (source: string, version: string) => void;
  onRemove: () => void;
}

/**
 * One skill per row: name, source and version on one line. The rarely edited
 * fields fold out underneath, so a long hub stays a list rather than a stack of forms.
 */
export default function HubEntryEditor({
  entry,
  problems,
  version,
  expanded,
  onToggle,
  onChange,
  onVersion,
  onRemove,
}: Props) {
  const t = useT();
  const queryClient = useQueryClient();
  const name = entry.data.name || t('hubBuilder.newEntry');
  const source = entry.data.source ?? '';
  const field = (key: string) => `hub-${entry.id}-${key}`;
  // Branches and tags are only listed once someone reaches for the dropdown.
  const [wanted, setWanted] = useState(false);
  const [error, setError] = useState('');
  // A new or re-typed source has no known version, so it is looked up once typing pauses.
  const [settled, setSettled] = useState(source);
  useEffect(() => {
    const timer = setTimeout(() => setSettled(source), 500);
    return () => clearTimeout(timer);
  }, [source]);
  const refs = useQuery({
    queryKey: queryKeys.hub.refs(source),
    queryFn: () => hubRefs(source),
    enabled: (wanted || (version === undefined && settled === source)) && Boolean(source.trim()),
    staleTime: 5 * 60 * 1000,
  });
  const current = refs.data?.current ?? version ?? '';
  const fixed = refs.data?.pinnable === false;

  const pick = async (next: string) => {
    setError('');
    try {
      const res = await hubRefs(source, next);
      // A source that cannot be pinned comes back without one; it stays as it is.
      if (!res.pinnable || res.source === undefined) return;
      const pinned = res.source;
      if (refs.data)
        queryClient.setQueryData(queryKeys.hub.refs(pinned), {
          ...refs.data,
          current: next,
          source: undefined,
        });
      onVersion(pinned, next);
    } catch (e) {
      setError((e as Error).message);
    }
  };

  const message =
    problems.length > 0
      ? problems.map((p) => t(`hubBuilder.problem.${p.code}`)).join(' ')
      : error || (refs.isError ? (refs.error as Error).message : '');

  return (
    <div className={`ss-r !block !py-2.5 ${problems.length > 0 ? 'bg-bad-bg' : ''}`}>
      <div className="grid grid-cols-[180px_minmax(0,1fr)_150px_30px_30px] items-center gap-2.5">
        <span className="ss-inp !h-8 font-mono">
          <input
            aria-label={t('hubs.edit.name')}
            value={entry.data.name ?? ''}
            placeholder={t('hubBuilder.newEntry')}
            onChange={(e) => onChange('name', e.target.value)}
          />
        </span>
        <span className={`ss-inp !h-8 font-mono ${problems.length > 0 ? 'err' : ''}`}>
          <input
            aria-label={t('hubs.edit.source')}
            value={source}
            placeholder="github.com/owner/repo/skills/review"
            onChange={(e) => onChange('source', e.target.value)}
          />
        </span>
        <div
          onPointerDownCapture={() => setWanted(true)}
          onFocusCapture={() => setWanted(true)}
          title={fixed ? t('hubs.ref.fixed') : undefined}
        >
          <Select
            size="sm"
            ariaLabel={t('hubs.view.version')}
            value={current}
            onChange={(v) => void pick(v)}
            options={refOptions(t, refs.data, current, refs.isFetching)}
            disabled={!source.trim() || fixed}
          />
        </div>
        <button
          type="button"
          className="ss-ib"
          aria-label={t('hubs.edit.removeEntry', { name })}
          title={t('hubs.edit.removeEntry', { name })}
          onClick={onRemove}
        >
          <Trash2 size={15} />
        </button>
        <button
          type="button"
          className="ss-ib"
          aria-expanded={expanded}
          aria-label={t('hubs.edit.moreFields', { name })}
          title={t('hubs.edit.moreFields', { name })}
          onClick={onToggle}
        >
          <ChevronDown size={16} className={expanded ? 'rotate-180' : ''} />
        </button>
      </div>
      {message && (
        <span className="mt-1.5 flex items-center gap-1.5 pl-[190px] text-[12.5px] text-bad">
          <AlertCircle size={13} className="shrink-0" />
          {message}
        </span>
      )}
      {!message && fixed && (
        <span className="mt-1.5 block pl-[190px] text-[12.5px] text-ink-3">
          {t('hubs.ref.fixed')}
        </span>
      )}

      {expanded && (
        <div className="mt-3 flex flex-col gap-3.5 pl-[190px] pb-1.5">
          <div className="ss-fld">
            <label htmlFor={field('description')}>{t('hubBuilder.entryDescription')}</label>
            <span className="ss-inp area">
              <textarea
                id={field('description')}
                rows={2}
                value={entry.data.description ?? ''}
                onChange={(e) => onChange('description', e.target.value)}
              />
            </span>
          </div>
          <div className="grid grid-cols-2 gap-3.5">
            <div className="ss-fld">
              <label htmlFor={field('tags')}>{t('hubBuilder.tags')}</label>
              <span className="ss-inp">
                <input
                  id={field('tags')}
                  value={(entry.data.tags ?? []).join(',')}
                  onChange={(e) => onChange('tags', e.target.value.split(','))}
                />
              </span>
            </div>
            <div className="ss-fld">
              <label htmlFor={field('selector')}>{t('hubBuilder.selector')}</label>
              <span className="ss-inp font-mono">
                <input
                  id={field('selector')}
                  value={entry.data.skill ?? ''}
                  onChange={(e) => onChange('skill', e.target.value)}
                />
              </span>
              <span className="hp">{t('hubBuilder.selectorHint')}</span>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
