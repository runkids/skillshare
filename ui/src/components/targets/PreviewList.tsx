import { useState, type PointerEvent } from 'react';
import { Search } from 'lucide-react';
import type { SyncMatrixEntry } from '../../api/client';
import { useT } from '../../i18n';
import Button from '../Button';
import SegmentedControl from '../SegmentedControl';
import Spinner from '../Spinner';
import { inSegment, type PreviewSegment } from '../projects/projectView';
import { Highlight } from './FilterSection';
import { patternMatches, patternName, previewReason, rowAction, searchEntries } from './targetView';
import type { FilterFeedback } from './useFilterFeedback';

interface Props {
  kind: 'skill' | 'agent';
  /** Preview rows of this kind for the draft filters */
  entries: SyncMatrixEntry[];
  include: string[];
  exclude: string[];
  onChange: (next: { include: string[]; exclude: string[] }) => void;
  loading: boolean;
  error: Error | null;
  disabled: boolean;
  feedback: FilterFeedback;
}

/** What the filters let through: every row with its result, and an action on hover that edits the filters by name. */
export default function PreviewList({
  kind,
  entries,
  include,
  exclude,
  onChange,
  loading,
  error,
  disabled,
  feedback,
}: Props) {
  const t = useT();
  const [segment, setSegment] = useState<PreviewSegment>('all');
  const [query, setQuery] = useState('');
  const part = kind === 'agent' ? 'agents' : 'skills';
  const synced = entries.filter((e) => e.status === 'synced').length;
  const shown = searchEntries(entries, query).filter((e) => inSegment(e.status, segment));
  const { lit, setLit, changed } = feedback;
  // Touch screens send a pointer enter on tap; only a mouse hovers.
  const hover = (p: string) => (e: PointerEvent) => e.pointerType === 'mouse' && setLit(p);

  return (
    <div className="flex flex-col gap-2.5">
      <div className="flex flex-wrap items-center gap-2.5">
        <SegmentedControl
          value={segment}
          onChange={setSegment}
          options={[
            { value: 'all', label: t('projects.preview.all'), count: entries.length },
            { value: 'sync', label: t('targetDetail.synced'), count: synced },
            { value: 'skip', label: t('projects.preview.skipped'), count: entries.length - synced },
          ]}
        />
        <label className="ss-inp ml-auto h-[34px] w-full sm:w-[220px]">
          <Search size={14} className="shrink-0 text-ink-3" />
          <input
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t(`targetDetail.search.${part}`)}
            aria-label={t(`targetDetail.search.${part}`)}
          />
        </label>
      </div>
      {loading ? (
        <div className="flex items-center gap-2 text-[13px] text-ink-2">
          <Spinner size="sm" />
          {t('targetDetail.loadingPreview')}
        </div>
      ) : error ? (
        <div className="ss-note bad">
          <span className="flex-1">{error.message}</span>
        </div>
      ) : (
        <>
          <div className="ss-list">
            {shown.length === 0 && (
              <p className="px-4 py-3 text-[13px] text-ink-3">
                {query.trim()
                  ? t('targetDetail.searchNoMatches', { query: query.trim() })
                  : t(`projects.preview.none.${part}`)}
              </p>
            )}
            {shown.map((e) => {
              const name = patternName(e);
              const on = e.status === 'synced';
              const act = rowAction(e, include, exclude);
              const reason = previewReason(t, e, include.length > 0);
              const tint = changed.has(e.skill)
                ? 'ss-chg'
                : lit && patternMatches(lit, name)
                  ? 'ss-lit'
                  : '';
              return (
                <div key={e.skill} className={`ss-r ss-fr group !min-h-[44px] !py-1 ${tint}`}>
                  <span
                    className={`min-w-0 flex-1 truncate font-mono text-[13px] font-semibold ${on ? '' : 'text-ink-2'}`}
                  >
                    <Highlight text={name} query={query} />
                  </span>
                  {!on && (
                    <span
                      className="max-w-[45%] truncate text-xs text-ink-3"
                      title={reason}
                      onPointerEnter={e.status === 'excluded' ? hover(e.reason) : undefined}
                      onPointerLeave={() => setLit('')}
                    >
                      {reason}
                    </span>
                  )}
                  <span className={`ss-st shrink-0 ${on ? 'ok' : 'off'}`}>
                    {t(on ? 'targetDetail.synced' : 'projects.preview.skipped')}
                  </span>
                  <span className="flex w-[92px] shrink-0 justify-end">
                    {act && (
                      <Button
                        variant="ghost"
                        size="sm"
                        className="opacity-0 transition-opacity duration-[120ms] group-focus-within:opacity-100 group-hover:opacity-100 [@media(hover:none)]:opacity-100"
                        aria-label={`${t(`targetDetail.action.${act.action}`)} ${name}`}
                        onClick={() => onChange(act.next)}
                        disabled={disabled}
                      >
                        {t(`targetDetail.action.${act.action}`)}
                      </Button>
                    )}
                  </span>
                </div>
              );
            })}
          </div>
          <p className="text-xs text-ink-3">{t('targetDetail.rowHint')}</p>
        </>
      )}
    </div>
  );
}
