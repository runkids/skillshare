import { useState, type ReactNode } from 'react';
import { ChevronDown, Package } from 'lucide-react';
import AgentIcon from '../AgentIcon';
import Tooltip from '../Tooltip';
import { byPlugin, type PluginRun } from '../../api/plugins';
import { useT } from '../../i18n';

/** Agents as overlapping logos. Two Agents can share a logo, so each names itself on hover or focus. */
function AgentStack({ targets, label }: { targets: string[]; label: (target: string) => string }) {
  return (
    <span className="ss-stack shrink-0">
      {targets.map((target) => (
        <Tooltip key={target} side="top" content={label(target)}>
          <span className="ss-at" role="img" tabIndex={0} aria-label={label(target)}>{target ? <AgentIcon target={target} size={13} /> : <Package size={13} />}</span>
        </Tooltip>
      ))}
    </span>
  );
}

/** One plugin and the Agents a change or outcome applies to, with a status on the right and an optional note below. */
export function PluginRunLine({ name, targets, label, right, note, className = '' }: { name: string; targets: string[]; label: (target: string) => string; right: ReactNode; note?: string; className?: string }) {
  return (
    <div className={`flex flex-col gap-1 ${className}`}>
      <div className="flex min-h-7 items-center gap-2">
        <span className="min-w-0 flex-1 truncate font-mono text-[12.5px] font-semibold" title={name}>{name}</span>
        <AgentStack targets={targets} label={label} />
        {right}
      </div>
      {note && <span className="break-words text-xs leading-normal text-ink-3">{note}</span>}
    </div>
  );
}

/**
 * What a run left alone, folded into one line until asked for: listed in full it buries the
 * few rows that changed. Open, it is one row per plugin, with Pi packages apart from plugins.
 * Its rows carry no logos: which Agents a plugin left alone tells nothing, and they crowd out the rows that need a look.
 */
export function UnchangedRuns({ runs, isPi, versions }: { runs: PluginRun[]; isPi: (name: string) => boolean; versions?: boolean }) {
  const t = useT();
  const [open, setOpen] = useState(false);
  if (runs.length === 0) return null;
  const groups = byPlugin(runs, (r) => r.version ?? '');
  const parts = [
    { key: 'plugins', title: t('plugins.title'), rows: groups.filter((g) => !isPi(g.name)) },
    { key: 'pi', title: t('plugins.piTitle'), rows: groups.filter((g) => isPi(g.name)) },
  ].filter((part) => part.rows.length > 0);
  return (
    <div className="flex flex-col">
      <button type="button" aria-expanded={open} className="flex min-h-8 items-center gap-1.5 text-left text-xs font-semibold text-ink-3" onClick={() => setOpen(!open)}>
        <span>{t('plugins.outcome.unchanged')} · {runs.length}</span>
        <ChevronDown size={14} className={`ml-auto shrink-0 ${open ? 'rotate-180' : ''}`} />
      </button>
      {open && parts.map((part) => (
        <div key={part.key} className="flex flex-col pt-2">
          <div className="pb-1 text-[11px] font-semibold text-ink-3">{part.title}</div>
          {part.rows.map((g) => (
            <div key={`${g.name}:${g.version ?? ''}`} className="flex min-h-9 items-center gap-2 [border-top:var(--sep)]">
              <span className="min-w-0 flex-1 truncate font-mono text-[12.5px] font-semibold" title={g.name}>{g.name}</span>
              {versions && g.version && <span className="shrink-0 font-mono text-xs text-ink-3">{g.version}</span>}
            </div>
          ))}
        </div>
      ))}
    </div>
  );
}
