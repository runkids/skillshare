import { Ellipsis } from 'lucide-react';
import type { HookEntry, HookPlan } from '../../api/hooks';
import { useT } from '../../i18n';
import AgentIcon from '../AgentIcon';
import { bindingLines } from './hookCatalog';
import { boundAgents, hookLabel, isCodeAgent, syncState } from './hooksView';

interface Props {
  entries: Record<string, HookEntry>;
  plan: HookPlan | null;
  onToggle: (name: string, enabled: boolean) => void;
  onMenu: (e: React.MouseEvent<HTMLButtonElement>, name: string) => void;
  /** Holds the switches while a save is on its way: each sends the revision it previewed. */
  disabled?: boolean;
}

/**
 * What a hook runs, one line per distinct event, matcher and command, with the targets that run it.
 * Event names that differ only in case (SessionStart, sessionStart) are one line.
 */
function summaryLines(entry: HookEntry) {
  const rows = new Map<string, { event: string; matcher: string; command: string; agents: string[] }>();
  for (const agent of boundAgents(entry)) {
    const binding = entry.bindings[agent] ?? entry.bindings.factory;
    const lines = isCodeAgent(agent) ? [{ event: hookLabel(agent), matcher: '', command: '' }] : bindingLines(binding);
    for (const line of lines) {
      const key = `${line.event.toLowerCase()}\0${line.matcher}\0${line.command}`;
      const row = rows.get(key);
      if (!row) rows.set(key, { ...line, agents: [agent] });
      else if (!row.agents.includes(agent)) row.agents.push(agent);
    }
  }
  return [...rows.values()];
}

/**
 * One card per hook: its state, what it runs and the targets it goes to. The per-target sync state
 * is on each icon; whether a target trusts and loads the hook is that target's own call.
 */
export default function HooksList({ entries, plan, onToggle, onMenu, disabled = false }: Props) {
  const t = useT();
  return (
    <div className="flex flex-col gap-3">
      {Object.entries(entries).map(([name, entry]) => {
        const agents = boundAgents(entry);
        const enabled = entry.enabled !== false;
        const states = agents.map((a) => syncState(plan, name, a));
        const status = !enabled ? t('hooks.disabled')
          : agents.length === 0 ? t('plugins.noAgentsYet')
            : states.includes('conflict') ? t('hooks.status.conflict')
              : states.includes('pending') ? t('plugins.pending')
                : t('hooks.sync.synced');
        const lines = summaryLines(entry);
        // Each line names its targets unless every line runs on all of them.
        const perLine = lines.some((l) => l.agents.length < agents.length);
        return (
          <article key={name} aria-label={name} className="ss-box flex flex-col gap-2.5 !p-4">
            <div className="flex items-center gap-2.5">
              <span title={name} className={`min-w-0 truncate font-mono font-semibold ${enabled ? '' : 'text-ink-3'}`}>{name}</span>
              <span className={`ss-tag !font-sans ${states.includes('conflict') && enabled ? 'bad' : ''}`}>
                {enabled && states.includes('pending') && !states.includes('conflict') && <span className="h-[5px] w-[5px] rounded-full bg-ink" aria-hidden="true" />}
                {status}
              </span>
              <span className="flex-1" />
              <span className="flex items-center gap-1.5">
                {agents.map((a, i) => (
                  <span key={a} title={`${hookLabel(a)} · ${t(`hooks.sync.${states[i]}`)}`} className={enabled ? '' : 'opacity-45 grayscale'}>
                    <AgentIcon target={a} size={16} />
                    <span className="sr-only">{hookLabel(a)} · {t(`hooks.sync.${states[i]}`)}</span>
                  </span>
                ))}
              </span>
              <button type="button" role="switch" aria-checked={enabled} aria-label={t('hooks.enableSwitch', { name })} className="grid h-7 shrink-0 place-items-center disabled:opacity-50" disabled={disabled} onClick={() => onToggle(name, !enabled)}>
                <span className={`ss-sw ${enabled ? 'on' : ''}`}><i /></span>
              </button>
              <button type="button" className="ss-ib" aria-label={t('hooks.moreActions', { name })} onClick={(e) => onMenu(e, name)}>
                <Ellipsis size={16} />
              </button>
            </div>
            {entry.description && <span className="text-[13px] text-ink-2">{entry.description}</span>}
            {lines.length > 0 && (
              <div className={`flex flex-col gap-1 border-t border-line pt-2.5 font-mono text-[12.5px] ${enabled ? '' : 'text-ink-3'}`}>
                {lines.map((l) => (
                  <div key={`${l.event}\0${l.matcher}\0${l.command}`} className="flex min-w-0 items-baseline gap-2">
                    <span className="w-[150px] shrink-0 truncate" title={l.matcher ? `${l.event} · ${l.matcher}` : l.event}>{l.event}{l.matcher && <span className="text-ink-2"> · {l.matcher}</span>}</span>
                    {l.command ? <><span className="shrink-0 text-ink-3">→</span><span className="min-w-0 flex-1 truncate" title={l.command}>{l.command}</span></> : <span className="flex-1 font-sans text-xs text-ink-3">{t('hooks.summary.code')}</span>}
                    {perLine && (
                      <span className={`flex shrink-0 items-center gap-1 self-center ${enabled ? '' : 'opacity-45 grayscale'}`}>
                        {l.agents.map((a) => <span key={a} title={hookLabel(a)}><AgentIcon target={a} size={13} /><span className="sr-only">{hookLabel(a)}</span></span>)}
                      </span>
                    )}
                  </div>
                ))}
              </div>
            )}
          </article>
        );
      })}
    </div>
  );
}
