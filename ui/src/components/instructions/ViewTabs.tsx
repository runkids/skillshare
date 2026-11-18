import type { ReactNode } from 'react';
import MarkdownView from '../MarkdownView';
import { useT, plural } from '../../i18n';
import { formatSize, lineCount, previewParts } from './instructionsView';

/**
 * Underline tabs that switch how a box shows an instruction file (Preview /
 * Edit, or Source / Preview when it is read-only). className replaces the
 * strip's own divider and padding, for a header that already has them.
 */
export function ViewTabs<V extends string>({
  view,
  views: given,
  onChange,
  className = 'border-b border-line-soft px-4',
}: {
  view: V;
  /** Defaults to Preview / Edit (values preview and edit). */
  views?: { value: V; label: string }[];
  onChange: (view: V) => void;
  className?: string;
}) {
  const t = useT();
  const views =
    given ??
    ([
      { value: 'preview', label: t('instructions.target.view.preview') },
      { value: 'edit', label: t('instructions.target.view.edit') },
    ] as { value: V; label: string }[]);
  return (
    <div
      role="tablist"
      aria-label={t('instructions.target.view.label')}
      className={`flex gap-5 ${className}`}
      onKeyDown={(e) => {
        if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return;
        e.preventDefault();
        const i = views.findIndex((v) => v.value === view);
        const next =
          views[(i + (e.key === 'ArrowRight' ? 1 : views.length - 1)) % views.length].value;
        onChange(next);
        e.currentTarget.querySelector<HTMLElement>(`[data-view="${next}"]`)?.focus();
      }}
    >
      {views.map((v) => (
        <button
          key={v.value}
          type="button"
          role="tab"
          data-view={v.value}
          aria-selected={view === v.value}
          tabIndex={view === v.value ? 0 : -1}
          onClick={() => onChange(v.value)}
          className={`-mb-px h-9 border-b-2 text-[12.5px] font-semibold ${view === v.value ? 'border-ink text-ink' : 'border-transparent text-ink-2 hover:text-ink'}`}
        >
          {v.label}
        </button>
      ))}
    </div>
  );
}

/**
 * The grey strip on top of every box that shows an instruction file: line
 * count and size of content (live, so a draft counts as typed; … while it
 * loads) on the left, the view switch and then any actions on the right.
 */
export function BoxHeader<V extends string>({
  content,
  view,
  views,
  onChange,
  children,
}: {
  content: string | undefined;
  view: V;
  views?: { value: V; label: string }[];
  onChange: (view: V) => void;
  children?: ReactNode;
}) {
  const t = useT();
  return (
    // Body font even inside a code box, so every box header reads the same.
    <div
      className="flex h-[38px] shrink-0 items-center gap-2 border-b border-line bg-sunken pr-2 pl-4"
      style={{ fontFamily: 'var(--f)' }}
    >
      <span className="flex-1 text-[12.5px] text-ink-2">
        {content === undefined
          ? '…'
          : t(plural('instructions.preview.stats', lineCount(content)), {
              lines: lineCount(content),
              size: formatSize(new TextEncoder().encode(content).length),
            })}
      </span>
      {/* Shorter tabs, centred, so the underline sits clear of the strip's divider. */}
      <ViewTabs
        view={view}
        views={views}
        onChange={onChange}
        className="mr-2 [&>button]:mb-0 [&>button]:h-7"
      />
      {children}
    </div>
  );
}

/**
 * An instruction file rendered as Markdown: HTML comments are dropped and the
 * managed import block shows as the shared files it imports (names: known
 * shared file names).
 */
export function InstructionsPreview({
  content,
  names,
  className = 'min-h-0 flex-1 overflow-auto',
}: {
  content: string;
  names: string[];
  className?: string;
}) {
  const t = useT();
  const { before, imports, after } = previewParts(content, names);
  const empty = !before.trim() && imports.length === 0 && !after.trim();
  return (
    <div
      className={`flex flex-col gap-3 px-6 py-5 text-[13.5px] ${className}`}
      style={{ fontFamily: 'var(--f)' }}
      role="tabpanel"
      aria-label={t('instructions.target.view.preview')}
    >
      {before.trim() && <MarkdownView breaks>{before}</MarkdownView>}
      {imports.length > 0 && (
        <p className="font-mono text-[12px] text-ink-3">
          {imports.map((n) => `@import ${n}`).join(' · ')}
        </p>
      )}
      {after.trim() && <MarkdownView breaks>{after}</MarkdownView>}
      {empty && <p className="text-[13px] text-ink-3">{t('instructions.target.previewEmpty')}</p>}
    </div>
  );
}
