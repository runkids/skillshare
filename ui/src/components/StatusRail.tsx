import { useState, type ReactNode } from 'react';
import { ChevronDown } from 'lucide-react';
import AgentIcon from './AgentIcon';
import Spinner from './Spinner';
import { useT } from '../i18n';

/**
 * The right-hand column of the Plugins and MCP pages. The list on the left is what you
 * have; everything here is state: what Sync would do, then which Agents can take part.
 * Position tells the two apart, so neither needs a heading to say which it is.
 */

// Below the page header, above the page's bottom padding: what is left of the viewport.
const PANE = 'sticky top-6 max-h-[calc(100vh-11rem)]';

/**
 * List on the left, state on the right. Each side keeps to the viewport and scrolls on its
 * own, so a long Agents column never drags the list, or the Sync box, out of view.
 * The list side is padded by what it is pulled out by, so the scroll box does not clip shadows.
 * Its children must not shrink: `.ss-list` clips its corners with overflow:hidden, which lets a
 * flex item go below its content height, and it would be squashed instead of scrolled.
 */
export function RailLayout({ rail, children }: { rail: ReactNode; children: ReactNode }) {
  return (
    <div className="grid grid-cols-[minmax(0,1fr)_320px] items-start gap-8">
      <div className={`${PANE} -m-2 flex min-w-0 flex-col gap-3 overflow-y-auto p-2 [&>*]:shrink-0`}>{children}</div>
      <aside className={`${PANE} flex flex-col gap-7`}>{rail}</aside>
    </div>
  );
}

/** A titled part of the rail. The title stays; the groups under it take the height that is left and scroll. */
export function RailSection({ title, count, action, children }: { title: string; count: number; action?: ReactNode; children: ReactNode }) {
  return (
    <div className="flex min-h-0 flex-col gap-3.5">
      <div className="flex min-h-6 items-center gap-2">
        <h3 className="text-[15px] font-semibold">{title}</h3>
        <span className="ss-cnt">{count}</span>
        {action && <span className="ml-auto flex">{action}</span>}
      </div>
      <div className="-mx-1 flex min-h-0 flex-col gap-3.5 overflow-y-auto px-1">{children}</div>
    </div>
  );
}

/** `warn` tints the box. It is the only colour in the rail, so pending work is seen first. */
export function SyncBox({ tone, state, children }: { tone: 'ok' | 'warn' | 'busy'; state: string; children: ReactNode }) {
  const t = useT();
  return (
    <div className={`ss-box flex shrink-0 flex-col gap-3 ${tone === 'warn' ? 'bg-warn-bg' : ''}`}>
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-[15px] font-semibold">{t('sync.title')}</h3>
        {tone === 'busy'
          ? <span className="flex items-center gap-1.5 text-[13px] text-ink-3"><Spinner size="sm" />{state}</span>
          : <span className={`ss-st ${tone}`}>{state}</span>}
      </div>
      {children}
    </div>
  );
}

/** What Sync will do, or did, to one Agent. `word` is the CLI's own term, so it stays English. */
export function RailLine({ name, agent, word, bad }: { name: string; agent: string; word: string; bad?: boolean }) {
  return (
    <div className="flex items-baseline gap-1.5 text-[13px] text-ink-2">
      <span className="max-w-[45%] shrink-0 truncate font-mono text-[12.5px] font-semibold text-ink" title={name}>{name}</span>
      <span className="min-w-0 flex-1 truncate">→ {agent}</span>
      <span className={`shrink-0 font-mono text-xs ${bad ? 'text-bad' : 'text-ink'}`}>{word}</span>
    </div>
  );
}

/** A labelled run of rows. Groups are told apart by a separator and a small label, not a box each. */
export function RailGroup({ label, count, right, foot, children }: { label: string; count?: number; right?: ReactNode; foot?: string; children?: ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5 pt-3.5 [border-top:var(--sep)] first:pt-0 first:[border-top:0]">
      <div className="flex min-h-5 items-center gap-1.5 text-xs font-semibold text-ink-3">
        <span>{label}</span>
        {count !== undefined && <span>· {count}</span>}
        {right && <span className="ml-auto flex">{right}</span>}
      </div>
      {children && <div className="flex flex-col">{children}</div>}
      {foot && <p className="text-xs leading-normal text-ink-3">{foot}</p>}
    </div>
  );
}

/**
 * One Agent per line. With `detail` the row is a disclosure: the rail stays a list of names
 * until one is asked about. `right` sits inside that button, so it must not be interactive.
 */
export function RailRow({ target, label, dim, sub, right, detail }: { target: string; label: string; dim?: boolean; sub?: string; right?: ReactNode; detail?: ReactNode }) {
  const [open, setOpen] = useState(false);
  const head = (
    <>
      <span className={`ss-at !h-[22px] !w-[22px] !rounded-md ${dim ? 'opacity-55' : ''}`}><AgentIcon target={target} size={13} /></span>
      <span className="flex min-w-0 flex-1 flex-col gap-px">
        <span className={`truncate text-[13.5px] font-medium ${dim ? 'text-ink-2' : ''}`}>{label}</span>
        {sub && !open && <span className="truncate text-xs text-ink-3">{sub}</span>}
      </span>
      {right}
    </>
  );
  if (!detail) return <div className="flex min-h-8 items-center gap-2.5 py-1">{head}</div>;
  return (
    <>
      <button type="button" aria-expanded={open} className="group flex min-h-8 w-full items-center gap-2.5 py-1 text-left" onClick={() => setOpen(!open)}>
        {head}
        <ChevronDown size={14} className={`shrink-0 text-ink-3 ${open ? 'rotate-180' : 'opacity-0 group-hover:opacity-100 group-focus-visible:opacity-100'}`} />
      </button>
      {open && <div className="mb-2 ml-8 mt-0.5 flex flex-col items-start gap-1.5 rounded-md bg-sunken px-3 py-2.5 text-[12.5px] leading-relaxed text-ink-2">{detail}</div>}
    </>
  );
}
