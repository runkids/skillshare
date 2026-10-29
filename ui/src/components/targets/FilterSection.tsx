import { Fragment, useState, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { ChevronDown, ChevronRight } from 'lucide-react';
import type { SyncMatrixEntry } from '../../api/client';
import AgentIcon from '../AgentIcon';
import Spinner from '../Spinner';
import { useI18n, useT, plural } from '../../i18n';
import PatternInput from './PatternInput';
import { joinList, patternName, togglePatterns } from './targetView';

type Kind = 'skill' | 'agent' | 'file';
const MODES = ['merge', 'copy', 'symlink'] as const;
// Sentences that end in a full-width stop run on without a space.
const CJK_STOP = /[。！？]$/;

interface Props {
  kind: Kind;
  mode: string;
  /** What the summary says the resources sync to: a target or a project */
  name: string;
  include: string[];
  exclude: string[];
  onChange: (next: { include: string[]; exclude: string[] }) => void;
  /** Preview rows of this kind for the draft filters */
  entries: SyncMatrixEntry[];
  loaded: boolean;
  loading: boolean;
  error: Error | null;
  disabled: boolean;
  /** Targets with skills off that read this folder, so the filters shape what they see too */
  alsoReadBy?: string[];
  /** Targets with skills on whose folder this tool also reads, so it sees their skills twice */
  readsFrom?: string[];
  /** Shown in place of the include hint, such as an include that matches nothing */
  includeWarning?: ReactNode;
}

/** Include and exclude patterns with a live preview; a click on a preview row edits them. */
export default function FilterSection({ kind, mode, name, include, exclude, onChange, entries, loaded, loading, error, disabled, alsoReadBy = [], readsFrom = [], includeWarning }: Props) {
  const { t, locale } = useI18n();
  const [help, setHelp] = useState(false);
  const files = kind === 'file';
  const synced = entries.filter((e) => e.status === 'synced').length;

  const reason = (e: SyncMatrixEntry) => {
    switch (e.status) {
      case 'skipped': return t('sync.skipped.ambiguousVariant');
      case 'synced': return t(include.length ? 'targetDetail.reason.included' : 'targetDetail.reason.noFilter');
      case 'excluded': return t('targetDetail.reason.excluded', { pattern: e.reason });
      case 'not_included': return t('targetDetail.reason.notIncluded');
      case 'skill_target_mismatch': return t('targetDetail.reason.declared', { targets: e.reason });
      default: return e.reason;
    }
  };

  const baseHint = t(`targetDetail.includeHint.${kind}s`);
  const includeHint = alsoReadBy.length > 0
    ? baseHint + (CJK_STOP.test(baseHint) ? '' : ' ') + t('targetDetail.alsoReadBy.filterHint', { names: joinList(alsoReadBy, locale) })
    : baseHint;
  const readers = alsoReadBy.length > 0 && (
    <TargetRow sentence={t('targetDetail.alsoReadBy.sentence', { names: NAMES })} names={alsoReadBy} />
  );
  const sources = readsFrom.length > 0 && (
    <TargetRow sentence={t('targetDetail.readsFrom.sentence', { names: NAMES })} names={readsFrom} />
  );

  return mode === 'symlink' ? (
    <>
      <div className="ss-note inf"><span className="flex-1">{t('targetDetail.symlinkNoFilters')}</span></div>
      {readers}
      {sources}
    </>
  ) : (
    <>
      {loaded && (
        <p className="text-[13.5px]">
          {t(plural(`targetDetail.summary.${kind}s`, entries.length), { synced, total: entries.length, name })}
        </p>
      )}
      {readers}
      {sources}
      <div className="ss-fld">
        <label htmlFor="filter-include">{t('targetDetail.include')}</label>
        <PatternInput id="filter-include" patterns={include} onChange={(next) => onChange({ include: next, exclude })} disabled={disabled} />
        {includeWarning ?? <span className="hp">{includeHint}</span>}
      </div>
      <div className="ss-fld">
        <label htmlFor="filter-exclude">{t('targetDetail.exclude')}</label>
        <PatternInput id="filter-exclude" patterns={exclude} onChange={(next) => onChange({ include, exclude: next })} disabled={disabled} />
        <span className="hp">{t(`targetDetail.excludeHint.${kind}s`)}</span>
      </div>
      <button type="button" className="ss-disc self-start" aria-expanded={help} onClick={() => setHelp(!help)}>
        {help ? <ChevronDown size={15} /> : <ChevronRight size={15} />}
        {t('targetDetail.patternHelp')}
      </button>
      {help && (
        <ul className="ml-[22px] -mt-2 flex list-disc flex-col gap-1 pl-4 text-[13px] text-ink-2">
          <li>{t('targetDetail.help.wildcards')}</li>
          {files ? (
            <>
              <li>{t('targetDetail.help.anyDepth')}</li>
              <li>{t('targetDetail.help.folders')}</li>
              <li>{t('targetDetail.help.orderFiles')}</li>
            </>
          ) : (
            <>
              <li>{t('targetDetail.help.nested')}</li>
              <li>{t('targetDetail.help.order')}</li>
              <li>{t('targetDetail.help.declared')}</li>
            </>
          )}
        </ul>
      )}

      {loading ? (
        <div className="flex items-center gap-2 text-[13px] text-ink-2"><Spinner size="sm" />{t('targetDetail.loadingPreview')}</div>
      ) : error ? (
        <div className="ss-note bad"><span className="flex-1">{error.message}</span></div>
      ) : entries.length > 0 && (
        // The result of the filters, set apart from the fields that shape it.
        <div className="mt-2 flex flex-col gap-3 border-t border-line pt-5">
          <div className="flex flex-col gap-0.5">
            <h3 className="text-[14px] font-semibold">{t('targetDetail.previewCount', { count: entries.length })}</h3>
            <p className="text-[13px] text-ink-3">{t(files ? 'targetDetail.clickHintFiles' : 'targetDetail.clickHint')}</p>
          </div>
          <div className="ss-list !shadow-none">
            <div className="ss-lh">
              <span className="flex-1">{t(files ? 'targetDetail.colFile' : 'resources.col.name')}</span>
              <span className="w-[170px]">{t('targetDetail.becauseOf')}</span>
              <span className="w-[96px]">{t('targetDetail.result')}</span>
            </div>
            <div className="max-h-[340px] overflow-y-auto">
              {entries.map((e) => {
                const next = togglePatterns(e, include, exclude);
                const on = e.status === 'synced';
                const cells = (
                  <>
                    <span className={`min-w-0 flex-1 truncate font-mono text-[13px] font-semibold ${on ? '' : 'text-ink-2'}`}>{patternName(e)}</span>
                    <span className="w-[170px] shrink-0 truncate font-mono text-[12px] text-ink-2" title={reason(e)}>{reason(e)}</span>
                    <span className="w-[96px] shrink-0"><span className={`ss-st ${on ? 'ok' : 'off'}`}>{t(on ? 'targetDetail.synced' : 'targetDetail.notSynced')}</span></span>
                  </>
                );
                return next ? (
                  <button key={e.skill} type="button" className="ss-r link !min-h-[42px] w-full text-left" onClick={() => onChange(next)} title={t(on || e.status === 'skipped' ? 'targetDetail.clickExclude' : 'targetDetail.clickInclude')} disabled={disabled}>
                    {cells}
                  </button>
                ) : (
                  <div key={e.skill} className="ss-r !min-h-[42px]">{cells}</div>
                );
              })}
            </div>
          </div>
        </div>
      )}
    </>
  );
}

/** merge, copy or symlink, with what each does to this kind of resource. */
export function ModePicker({ kind, mode, onChange, disabled }: { kind: Kind; mode: string; onChange: (mode: string) => void; disabled: boolean }) {
  const t = useT();
  return (
    <div role="radiogroup" aria-label={t('targetDetail.syncMode')} className="flex flex-col gap-2.5">
      {MODES.map((m) => {
        const on = mode === m;
        return (
          <button key={m} type="button" role="radio" aria-checked={on} className={`ss-pick text-left ${on ? 'on' : ''}`} onClick={() => onChange(m)} disabled={disabled}>
            <span className={`ss-chk rad ${on ? 'on' : ''}`} />
            <span className="flex flex-col gap-0.5">
              <span><span className="font-semibold">{m}</span>{m === 'merge' && <span className="text-ink-3"> · {t('targetDetail.default')}</span>}</span>
              <span className="text-[13px] text-ink-2">{t(m === 'merge' ? `targetDetail.mode.merge.${kind}` : `targetDetail.mode.${m}`)}</span>
            </span>
          </button>
        );
      })}
    </div>
  );
}

// Stands in for the names in a sentence, which are then put back as links.
export const NAMES = '\u0000';

/** A sentence with its NAMES placeholder replaced by links to those targets, or to `href(name)` when it gives one. */
export function TargetLinks({ sentence, names, href }: { sentence: string; names: string[]; href?: (name: string) => string | undefined }) {
  const t = useT();
  const [before, after = ''] = sentence.split(NAMES);
  return (
    <p className="min-w-0">
      {before}
      {names.map((n, i) => (
        <Fragment key={n}>
          {i > 0 && t('instructions.shared.listSep')}
          <Link to={href?.(n) ?? `/targets/${encodeURIComponent(n)}`} className="font-semibold underline-offset-2 hover:underline">{n}</Link>
        </Fragment>
      ))}
      {after}
    </p>
  );
}

/** Other targets tied to this folder, as a note so it does not read as a filter pattern. */
function TargetRow({ sentence, names }: { sentence: string; names: string[] }) {
  return (
    <div className="ss-note items-center">
      <span className="ss-stack shrink-0" aria-hidden="true">
        {names.slice(0, 5).map((n) => <span key={n} className="ss-at !h-6 !w-6"><AgentIcon target={n} size={13} /></span>)}
      </span>
      <TargetLinks sentence={sentence} names={names} />
    </div>
  );
}
