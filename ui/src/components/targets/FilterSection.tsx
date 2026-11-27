import { Fragment, useState, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { ChevronDown, ChevronRight, Search } from 'lucide-react';
import type { SyncMatrixEntry } from '../../api/client';
import AgentStack from '../AgentStack';
import Spinner from '../Spinner';
import { useT, plural } from '../../i18n';
import PatternInput from './PatternInput';
import { patternName, previewReason, searchEntries, togglePatterns } from './targetView';

interface Props {
  /** What the summary says the files sync to */
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
  /** Shown in place of the include hint, such as an include that matches nothing */
  includeWarning?: ReactNode;
}

/** An extra's include and exclude patterns with a live preview; a click on a preview row edits them. */
export default function FilterSection({
  name,
  include,
  exclude,
  onChange,
  entries,
  loaded,
  loading,
  error,
  disabled,
  includeWarning,
}: Props) {
  const t = useT();
  const [help, setHelp] = useState(false);
  const [query, setQuery] = useState('');
  const synced = entries.filter((e) => e.status === 'synced').length;
  const shown = searchEntries(entries, query);
  const reason = (e: SyncMatrixEntry) => previewReason(t, e, include.length > 0);

  return (
    <>
      {loaded && (
        <p className="text-[13.5px]">
          {t(plural('targetDetail.summary.files', entries.length), {
            synced,
            total: entries.length,
            name,
          })}
        </p>
      )}
      <div className="ss-fld">
        <label htmlFor="filter-include">{t('targetDetail.include')}</label>
        <PatternInput
          id="filter-include"
          patterns={include}
          onChange={(next) => onChange({ include: next, exclude })}
          disabled={disabled}
        />
        {includeWarning ?? <span className="hp">{t('targetDetail.includeHint.files')}</span>}
      </div>
      <div className="ss-fld">
        <label htmlFor="filter-exclude">{t('targetDetail.exclude')}</label>
        <PatternInput
          id="filter-exclude"
          patterns={exclude}
          onChange={(next) => onChange({ include, exclude: next })}
          disabled={disabled}
        />
        <span className="hp">{t('targetDetail.excludeHint.files')}</span>
      </div>
      <button
        type="button"
        className="ss-disc self-start"
        aria-expanded={help}
        onClick={() => setHelp(!help)}
      >
        {help ? <ChevronDown size={15} /> : <ChevronRight size={15} />}
        {t('targetDetail.patternHelp')}
      </button>
      {help && (
        <ul className="ml-[22px] -mt-2 flex list-disc flex-col gap-1 pl-4 text-[13px] text-ink-2">
          <li>{t('targetDetail.help.wildcards')}</li>
          <li>{t('targetDetail.help.anyDepth')}</li>
          <li>{t('targetDetail.help.folders')}</li>
          <li>{t('targetDetail.help.orderFiles')}</li>
        </ul>
      )}

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
        entries.length > 0 && (
          // The result of the filters, set apart from the fields that shape it.
          <div className="mt-2 flex flex-col gap-3 border-t border-line pt-5">
            <div className="flex flex-wrap items-end justify-between gap-3">
              <div className="flex flex-col gap-0.5">
                <h3 className="text-[14px] font-semibold">
                  {query.trim()
                    ? t('targetDetail.previewCountSearch', {
                        shown: shown.length,
                        count: entries.length,
                      })
                    : t('targetDetail.previewCount', { count: entries.length })}
                </h3>
                <p className="text-[13px] text-ink-3">{t('targetDetail.clickHintFiles')}</p>
              </div>
              <label className="ss-inp h-[34px] w-full sm:w-[240px]">
                <Search size={15} className="shrink-0 text-ink-3" />
                <input
                  type="search"
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  placeholder={t('targetDetail.search.files')}
                  aria-label={t('targetDetail.search.files')}
                />
              </label>
            </div>
            <div className="ss-list !shadow-none">
              <div className="ss-lh">
                <span className="flex-1">{t('targetDetail.colFile')}</span>
                <span className="w-[170px]">{t('targetDetail.becauseOf')}</span>
                <span className="w-[96px]">{t('targetDetail.result')}</span>
              </div>
              <div className="max-h-[340px] overflow-y-auto">
                {shown.length === 0 && (
                  <p className="px-4 py-3 text-[13px] text-ink-3">
                    {t('targetDetail.searchNoMatches', { query: query.trim() })}
                  </p>
                )}
                {shown.map((e) => {
                  const next = togglePatterns(e, include, exclude);
                  const on = e.status === 'synced';
                  const cells = (
                    <>
                      <span
                        className={`min-w-0 flex-1 truncate font-mono text-[13px] font-semibold ${on ? '' : 'text-ink-2'}`}
                      >
                        <Highlight text={patternName(e)} query={query} />
                      </span>
                      <span
                        className="w-[170px] shrink-0 truncate font-mono text-[12px] text-ink-2"
                        title={reason(e)}
                      >
                        {reason(e)}
                      </span>
                      <span className="w-[96px] shrink-0">
                        <span className={`ss-st ${on ? 'ok' : 'off'}`}>
                          {t(on ? 'targetDetail.synced' : 'targetDetail.notSynced')}
                        </span>
                      </span>
                    </>
                  );
                  return next ? (
                    <button
                      key={e.skill}
                      type="button"
                      className="ss-r link !min-h-[42px] w-full text-left"
                      onClick={() => onChange(next)}
                      title={t(
                        on || e.status === 'skipped'
                          ? 'targetDetail.clickExclude'
                          : 'targetDetail.clickInclude',
                      )}
                      disabled={disabled}
                    >
                      {cells}
                    </button>
                  ) : (
                    <div key={e.skill} className="ss-r !min-h-[42px]">
                      {cells}
                    </div>
                  );
                })}
              </div>
            </div>
          </div>
        )
      )}
    </>
  );
}

/** Marks the searched text inside a row name. */
export function Highlight({ text, query }: { text: string; query: string }) {
  const q = query.trim();
  const i = q ? text.toLowerCase().indexOf(q.toLowerCase()) : -1;
  return i < 0 ? (
    text
  ) : (
    <>
      {text.slice(0, i)}
      <mark className="ss-hit">{text.slice(i, i + q.length)}</mark>
      {text.slice(i + q.length)}
    </>
  );
}

// Stands in for the names in a sentence, which are then put back as links.
export const NAMES = '\u0000';

/** A sentence with its NAMES placeholder replaced by links to those targets, or to `href(name)` when it gives one. */
export function TargetLinks({
  sentence,
  names,
  href,
}: {
  sentence: string;
  names: string[];
  href?: (name: string) => string | undefined;
}) {
  const t = useT();
  const [before, after = ''] = sentence.split(NAMES);
  return (
    <p className="min-w-0">
      {before}
      {names.map((n, i) => (
        <Fragment key={n}>
          {i > 0 && t('instructions.shared.listSep')}
          <Link
            to={href?.(n) ?? `/targets/${encodeURIComponent(n)}`}
            className="font-semibold underline-offset-2 hover:underline"
          >
            {n}
          </Link>
        </Fragment>
      ))}
      {after}
    </p>
  );
}

/** Other targets tied to this folder, as a note so it does not read as a filter pattern. */
export function TargetRow({ sentence, names }: { sentence: string; names: string[] }) {
  return (
    <div className="ss-note items-center">
      <AgentStack
        targets={names}
        max={5}
        size="md"
        iconSize={13}
        className="shrink-0"
        aria-hidden="true"
      />
      <TargetLinks sentence={sentence} names={names} />
    </div>
  );
}
