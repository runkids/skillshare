import { Fragment } from 'react';
import { ChevronDown, X } from 'lucide-react';
import type { HubEntry, HubProblem } from '../../api/hubDrafts';
import { useT } from '../../i18n';

interface Props {
  entry: HubEntry;
  problems: HubProblem[];
  showProblems: boolean;
  expanded: boolean;
  onToggle: () => void;
  onChange: (key: string, value: unknown) => void;
  onRemove: () => void;
}

/**
 * One row per skill. The fields only appear when the row is open, so a hub with a
 * dozen entries stays a list instead of a dozen stacked forms.
 */
export default function HubEntryEditor({ entry, problems, showProblems, expanded, onToggle, onChange, onRemove }: Props) {
  const t = useT();
  const name = entry.data.name || t('hubBuilder.newEntry');
  const source = entry.data.source ?? '';
  const blocked = showProblems && problems.length > 0;
  const field = (key: string) => `hub-${entry.id}-${key}`;

  return (
    <Fragment>
      <div className="ss-r">
        <span className={`h-[7px] w-[7px] shrink-0 rounded-full ${blocked ? 'bg-bad' : source ? 'bg-ok' : 'bg-ink-3'}`} />
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="nm m break-words">{name}</span>
          {blocked ? (
            <span className="text-[12.5px] leading-snug text-bad">{problems.map(problem => t(`hubBuilder.problem.${problem.code}`)).join(' ')}</span>
          ) : (
            <span className="truncate font-mono text-[11.5px] text-ink-3">{source || t('hubBuilder.noSource')}</span>
          )}
        </span>
        {!blocked && source && <span className="ss-tag ok">{t('hubBuilder.installable')}</span>}
        <button type="button" className="ss-ib" aria-expanded={expanded} aria-label={t(expanded ? 'hubBuilder.collapseEntry' : 'hubBuilder.expandEntry', { name })} onClick={onToggle}>
          <ChevronDown size={16} className={expanded ? 'rotate-180' : ''} />
        </button>
      </div>

      {expanded && (
        <div className="ss-r fold !min-h-0 flex-col !items-stretch gap-3.5 !py-4">
          <div className="grid grid-cols-2 gap-3.5">
            <div className="ss-fld">
              <label htmlFor={field('name')}>{t('hubBuilder.displayName')}</label>
              <span className="ss-inp"><input id={field('name')} value={entry.data.name ?? ''} onChange={event => onChange('name', event.target.value)} /></span>
            </div>
            <div className="ss-fld">
              <label htmlFor={field('source')}>{t('hubBuilder.source')}</label>
              <span className={`ss-inp font-mono ${blocked ? 'err' : ''}`}>
                <input id={field('source')} value={source} placeholder="owner/repo/skills/review" onChange={event => onChange('source', event.target.value)} />
              </span>
              <span className="hp">{t('hubBuilder.sourceHint')}</span>
            </div>
          </div>

          <div className="ss-fld">
            <label htmlFor={field('description')}>{t('hubBuilder.entryDescription')}</label>
            <span className="ss-inp area"><textarea id={field('description')} rows={2} value={entry.data.description ?? ''} onChange={event => onChange('description', event.target.value)} /></span>
          </div>

          <div className="grid grid-cols-2 items-end gap-3.5">
            <div className="ss-fld">
              <label htmlFor={field('tags')}>{t('hubBuilder.tags')}</label>
              <span className="ss-inp"><input id={field('tags')} value={(entry.data.tags ?? []).join(',')} onChange={event => onChange('tags', event.target.value.split(','))} /></span>
            </div>
            <div className="flex items-center justify-end gap-3">
              <details className="mr-auto">
                <summary className="text-[12.5px] text-ink-2">{t('hubBuilder.advanced')}</summary>
                <div className="ss-fld mt-3">
                  <label htmlFor={field('selector')}>{t('hubBuilder.selector')}</label>
                  <span className="ss-inp font-mono"><input id={field('selector')} value={entry.data.skill ?? ''} onChange={event => onChange('skill', event.target.value)} /></span>
                  <span className="hp">{t('hubBuilder.selectorHint')}</span>
                </div>
              </details>
              <button type="button" className="ss-btn sm dng" onClick={onRemove}>
                <X size={13} />
                {t('hubBuilder.remove')}
              </button>
            </div>
          </div>
        </div>
      )}
    </Fragment>
  );
}
