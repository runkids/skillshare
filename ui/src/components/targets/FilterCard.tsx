import { useState, type PointerEvent, type ReactNode } from 'react';
import { SlidersHorizontal, X } from 'lucide-react';
import type { SyncMatrixEntry } from '../../api/client';
import { plural, useT } from '../../i18n';
import { InfoTip } from '../mcp/ProjectSectionHead';
import { usePatternDraft } from './usePatternDraft';
import { patternMatches, patternName } from './targetView';
import type { FilterFeedback } from './useFilterFeedback';

// A long list of rules folds after this many, so the rail keeps its height.
const SHOWN = 6;

interface Props {
  kind: 'skill' | 'agent';
  include: string[];
  exclude: string[];
  onChange: (next: { include: string[]; exclude: string[] }) => void;
  /** Preview rows, which each rule's count is taken from; undefined until the preview loads */
  entries: SyncMatrixEntry[] | undefined;
  /** Symlink mode links the whole folder; the rules stay, dimmed, and the preview says why. */
  symlink: boolean;
  disabled: boolean;
  feedback: FilterFeedback;
  /** A line under the head, such as other tools these rules also shape */
  note?: ReactNode;
}

/** Include and exclude rules for the rail. Each rule says how many rows it matches and lights them on hover. */
export default function FilterCard({
  kind,
  include,
  exclude,
  onChange,
  entries,
  symlink,
  disabled,
  feedback,
  note,
}: Props) {
  const t = useT();
  const part = kind === 'agent' ? 'agents' : 'skills';
  const help = [
    t('targetDetail.help.wildcards'),
    ...(kind === 'agent' ? [] : [t('targetDetail.help.nested')]),
    t('targetDetail.help.order'),
    ...(kind === 'agent' ? [] : [t('targetDetail.help.declared')]),
  ].join(' ');
  const { result } = feedback;
  const said = result && [
    result.skipped > 0 &&
      t(plural('targetDetail.feedback.skipped', result.skipped), { count: result.skipped }),
    result.synced > 0 &&
      t(plural('targetDetail.feedback.synced', result.synced), { count: result.synced }),
  ];
  const resultText =
    result && (said?.filter(Boolean).join(' · ') || t(`targetDetail.feedback.none.${part}`));
  return (
    <div className="ss-box flex flex-col gap-3.5">
      {/* A long card outgrows the window; its head, with the result note, stays in view while the card does. */}
      <div className="sticky top-0 z-[1] -mx-5 -my-2.5 flex items-center gap-2 bg-surface px-5 py-2.5">
        <SlidersHorizontal size={15} className="text-ink-2" />
        <b className="text-[15px]">{t('projects.filter.title')}</b>
        <InfoTip hint={help} />
        <span
          role="status"
          className={`ml-auto truncate text-xs font-semibold text-warn transition-opacity duration-150 ease-[cubic-bezier(.23,1,.32,1)] ${result?.shown ? 'opacity-100' : 'opacity-0'}`}
        >
          {resultText}
        </span>
      </div>
      {note}
      <div
        className={`flex flex-col gap-3.5 transition-opacity duration-200 ${symlink ? 'opacity-50' : ''}`}
      >
        {(['include', 'exclude'] as const).map((side) => (
          <PatternField
            key={side}
            id={`filter-${side}`}
            label={t(`targetDetail.${side}`)}
            hint={t(`targetDetail.${side}Short`)}
            kind={kind}
            patterns={side === 'include' ? include : exclude}
            onChange={(next) =>
              onChange(side === 'include' ? { include: next, exclude } : { include, exclude: next })
            }
            entries={entries}
            disabled={disabled}
            locked={symlink}
            feedback={feedback}
          />
        ))}
      </div>
    </div>
  );
}

function PatternField({
  id,
  label,
  hint,
  kind,
  patterns,
  onChange,
  entries,
  disabled,
  locked,
  feedback,
}: {
  id: string;
  label: string;
  hint: string;
  kind: 'skill' | 'agent';
  patterns: string[];
  onChange: (next: string[]) => void;
  entries: SyncMatrixEntry[] | undefined;
  disabled: boolean;
  /** No new rules while they cannot apply; the ones there can still go */
  locked: boolean;
  feedback: FilterFeedback;
}) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const field = usePatternDraft(patterns, onChange);
  const { lit, setLit } = feedback;
  // Touch screens send a pointer enter on tap; only a mouse hovers.
  const hover = (p: string) => (e: PointerEvent) => e.pointerType === 'mouse' && setLit(p);
  const names = entries?.map(patternName);
  const shown = open ? patterns : patterns.slice(0, SHOWN);
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="flex items-baseline gap-1.5 text-xs text-ink-3">
        <span className="text-[13px] font-semibold text-ink">{label}</span>
        {patterns.length > 0 && <span className="font-semibold">{patterns.length}</span>}
        <span>· {hint}</span>
      </label>
      <div className="ss-inp !h-auto flex-col !items-stretch !gap-1 !p-1.5">
        {patterns.length > 0 && (
          <div className="flex flex-wrap items-center gap-1.5">
            {shown.map((p) => {
              // Unknown until the preview loads.
              const count = names?.filter((n) => patternMatches(p, n)).length;
              return (
                <span
                  key={p}
                  className={`ss-tag ss-pat !h-[26px] max-w-full !pl-2.5 !pr-0.5 !text-[12px] ${lit === p ? 'hot' : ''} ${count === 0 ? 'zero' : ''}`}
                  title={
                    count === undefined
                      ? undefined
                      : count
                        ? t(plural('targetDetail.pattern.matches', count), { count })
                        : t(
                            `targetDetail.pattern.noMatch.${kind === 'agent' ? 'agents' : 'skills'}`,
                          )
                  }
                  onPointerEnter={hover(p)}
                  onPointerLeave={() => setLit('')}
                >
                  <span className="min-w-0 truncate">{p}</span>
                  {count !== undefined && (
                    <span className="font-sans text-[11px] font-semibold text-ink-3">{count}</span>
                  )}
                  <button
                    type="button"
                    className="grid h-6 w-6 shrink-0 place-items-center rounded-full text-ink-3 hover:bg-sunken hover:text-ink"
                    aria-label={t('targetDetail.removePattern', { pattern: p })}
                    onClick={() => {
                      setLit('');
                      onChange(patterns.filter((x) => x !== p));
                    }}
                    onFocus={() => setLit(p)}
                    onBlur={() => setLit('')}
                    disabled={disabled}
                  >
                    <X size={11} />
                  </button>
                </span>
              );
            })}
            {patterns.length > SHOWN && (
              <button
                type="button"
                className="ss-tag !h-[26px] cursor-pointer !border-dashed !px-2.5 !text-[12px] font-semibold"
                aria-expanded={open}
                onClick={() => setOpen(!open)}
              >
                {open
                  ? t('targetDetail.pattern.fewer')
                  : t('targetDetail.pattern.more', { count: patterns.length - SHOWN })}
              </button>
            )}
          </div>
        )}
        <input
          id={id}
          className="!h-[26px] w-full min-w-0 font-mono text-[12px]"
          {...field}
          placeholder={t('targetDetail.addPatternShort')}
          disabled={disabled || locked}
        />
      </div>
    </div>
  );
}
