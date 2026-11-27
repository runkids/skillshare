import { useLayoutEffect, useRef, useState, type ReactNode } from 'react';
import { ChevronDown } from 'lucide-react';
import AgentIcon from './AgentIcon';
import Spinner from './Spinner';
import Reveal from './Reveal';
import { useT } from '../i18n';

/**
 * The right-hand column of the Plugins and MCP pages. The list on the left is what you
 * manage; the rail shows what Sync would do and each tool's native inventory.
 * Pages label the two separately when their inventory counts can differ.
 */

/**
 * List on the left, state on the right, both in the page's own scroll. The rail sticks at the
 * top while it fits the window; a taller rail scrolls with the page until its bottom shows, then
 * stays, so its last section is never out of reach and the page keeps one scrollbar. A rail held
 * to the window's height instead squeezed its Agents list to a few rows on a short window (#1053).
 */
export function RailLayout({
  rail,
  children,
  className = '',
}: {
  rail: ReactNode;
  children: ReactNode;
  /** Extra classes on the grid, for a page that needs its own narrow-width layout. */ className?: string;
}) {
  const railRef = useRef<HTMLElement>(null);
  useLayoutEffect(() => {
    const el = railRef.current;
    if (!el || typeof ResizeObserver !== 'function') return;
    const ro = new ResizeObserver(() => el.style.setProperty('--rail-h', `${el.offsetHeight}px`));
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  return (
    <div className={`grid grid-cols-[minmax(0,1fr)_320px] items-start gap-8 ${className}`}>
      <div className="flex min-w-0 flex-col gap-3">{children}</div>
      <aside
        ref={railRef}
        className="sticky flex flex-col gap-7"
        style={{ top: 'min(1.5rem, calc(100vh - var(--rail-h, 0px) - 1.5rem))' }}
      >
        {rail}
      </aside>
    </div>
  );
}

/** A titled part of the rail: a heading with a count, then its groups. */
export function RailSection({
  title,
  count,
  action,
  children,
}: {
  title: string;
  count: number;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-3.5">
      <div className="flex min-h-6 items-center gap-2">
        <h3 className="text-[15px] font-semibold">{title}</h3>
        <span className="ss-cnt">{count}</span>
        {action && <span className="ml-auto flex">{action}</span>}
      </div>
      <div className="flex flex-col gap-3.5">{children}</div>
    </div>
  );
}

/** `warn` tints the box. It is the only colour in the rail, so pending work is seen first. `plain` marks pending work without colour. */
export function SyncBox({
  tone,
  state,
  children,
}: {
  tone: 'ok' | 'warn' | 'busy' | 'plain';
  state: string;
  children: ReactNode;
}) {
  const t = useT();
  const box = useRef<HTMLDivElement>(null);
  const height = useRef(0);
  const grow = useRef<Animation | null>(null);
  const shown = useRef(`${tone}:${state}`);
  // A new state swaps the box's contents at once; grow or shrink it from the old height instead of jumping.
  // Other renders only note the height. Only this animation counts: the tint's transition runs on the box too.
  useLayoutEffect(() => {
    const el = box.current;
    if (!el || typeof el.animate !== 'function') return;
    const key = `${tone}:${state}`;
    const running = grow.current?.playState === 'running' ? grow.current : null;
    if (key === shown.current) {
      if (!running) height.current = el.offsetHeight;
      return;
    }
    shown.current = key;
    // Caught midway, it starts again from where it is.
    const from = running ? el.offsetHeight : height.current;
    running?.cancel();
    const to = (height.current = el.offsetHeight);
    if (!from || from === to || window.matchMedia?.('(prefers-reduced-motion: reduce)').matches)
      return;
    grow.current = el.animate(
      { height: [`${from}px`, `${to}px`], overflow: ['hidden', 'hidden'] },
      // The curve and length of a Reveal growing open.
      { duration: 250, easing: 'cubic-bezier(0.3, 0.7, 0.3, 1)' },
    );
  });
  return (
    <div
      ref={box}
      className={`ss-box ss-syncbox flex shrink-0 flex-col gap-3 ${tone === 'warn' ? 'bg-warn-bg' : ''}`}
    >
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-[15px] font-semibold">{t('sync.title')}</h3>
        {tone === 'busy' ? (
          <span className="flex items-center gap-1.5 text-[13px] text-ink-3">
            <Spinner size="sm" />
            {state}
          </span>
        ) : (
          <span className={`ss-st ${tone}`}>{state}</span>
        )}
      </div>
      {children}
    </div>
  );
}

/** What Sync will do, or did, to one Agent. `word` is the CLI's own term, so it stays English. */
export function RailLine({
  name,
  agent,
  word,
  bad,
}: {
  name: string;
  agent: string;
  word: string;
  bad?: boolean;
}) {
  return (
    <div className="flex items-baseline gap-1.5 text-[13px] text-ink-2">
      <span
        className="max-w-[45%] shrink-0 truncate font-mono text-[12.5px] font-semibold text-ink"
        title={name}
      >
        {name}
      </span>
      <span className="min-w-0 flex-1 truncate">→ {agent}</span>
      <span className={`shrink-0 font-mono text-xs ${bad ? 'text-bad' : 'text-ink'}`}>{word}</span>
    </div>
  );
}

/** A labelled run of rows. Groups are told apart by a separator and a small label, not a box each. */
export function RailGroup({
  label,
  count,
  right,
  foot,
  fold,
  children,
}: {
  label: string;
  count?: number;
  right?: ReactNode;
  foot?: string;
  /** Starts as just its heading, opened from it: for a group nobody needs while working, so the rail fits the window and stays in view. */
  fold?: boolean;
  children?: ReactNode;
}) {
  const [open, setOpen] = useState(!fold);
  const heading = (
    <>
      <span>{label}</span>
      {count !== undefined && <span>· {count}</span>}
    </>
  );
  const body = (
    <>
      {children && <div className="flex flex-col">{children}</div>}
      {foot && <p className="text-xs leading-normal text-ink-3">{foot}</p>}
    </>
  );
  return (
    <div className="flex flex-col gap-1.5 pt-3.5 [border-top:var(--sep)] first:pt-0 first:[border-top:0]">
      {fold ? (
        <button
          type="button"
          aria-expanded={open}
          onClick={() => setOpen((o) => !o)}
          className="flex min-h-5 items-center gap-1.5 text-left text-xs font-semibold text-ink-3 hover:text-ink-2"
        >
          {heading}
          <ChevronDown
            size={14}
            className={`ml-auto transition-transform duration-200 ease-[cubic-bezier(.3,.7,.3,1)] motion-reduce:transition-none ${open ? 'rotate-180' : ''}`}
            aria-hidden="true"
          />
        </button>
      ) : (
        <div className="flex min-h-5 items-center gap-1.5 text-xs font-semibold text-ink-3">
          {heading}
          {right && <span className="ml-auto flex">{right}</span>}
        </div>
      )}
      {fold ? (
        <Reveal open={open}>
          <div className="flex flex-col gap-1.5">{body}</div>
        </Reveal>
      ) : (
        body
      )}
    </div>
  );
}

/**
 * One Agent per line. With `detail` the row is a disclosure: the rail stays a list of names
 * until one is asked about. `right` sits inside that button, so it must not be interactive.
 * `path` shows in full under the name, wrapping instead of being cut short.
 */
export function RailRow({
  target,
  label,
  dim,
  sub,
  path,
  right,
  detail,
}: {
  target: string;
  label: string;
  dim?: boolean;
  sub?: string;
  path?: string;
  right?: ReactNode;
  detail?: ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const head = (
    <>
      <span className={`ss-at !h-[22px] !w-[22px] !rounded-md ${dim ? 'opacity-55' : ''}`}>
        <AgentIcon target={target} size={13} />
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-px">
        <span className={`truncate text-[13.5px] font-medium ${dim ? 'text-ink-2' : ''}`}>
          {label}
        </span>
        {sub && !open && <span className="truncate text-xs text-ink-3">{sub}</span>}
        {path && <span className="break-all font-mono text-xs text-ink-3">{path}</span>}
      </span>
      {right}
    </>
  );
  if (!detail) return <div className="flex min-h-8 items-center gap-2.5 py-1">{head}</div>;
  return (
    <>
      <button
        type="button"
        aria-expanded={open}
        aria-label={label}
        className="group flex min-h-8 w-full items-center gap-2.5 py-1 text-left"
        onClick={() => setOpen(!open)}
      >
        {head}
        <ChevronDown
          size={14}
          className={`shrink-0 text-ink-3 ${open ? 'rotate-180' : 'opacity-0 group-hover:opacity-100 group-focus-visible:opacity-100'}`}
        />
      </button>
      {open && (
        <div className="mb-2 ml-8 mt-0.5 flex flex-col items-start gap-1.5 rounded-md bg-sunken px-3 py-2.5 text-[12.5px] leading-relaxed text-ink-2">
          {detail}
        </div>
      )}
    </>
  );
}
