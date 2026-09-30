import { Fragment, useState } from 'react';
import { Ellipsis, Info, Webhook } from 'lucide-react';
import type { HookEntry, HookPlan } from '../../api/hooks';
import { useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import AgentIcon from '../AgentIcon';
import { TargetPill } from '../mcp/TargetPicker';
import { boundAgents, hookLabel, isCodeAgent, syncState, type SyncState } from './hooksView';

interface Props {
  entries: Record<string, HookEntry>;
  plan: HookPlan | null;
  paths: Record<string, string>;
  onToggle: (name: string, enabled: boolean) => void;
  onMenu: (e: React.MouseEvent<HTMLButtonElement>, name: string) => void;
  /** Holds the switches while a save is on its way: each sends the revision it previewed. */
  disabled?: boolean;
}

const stateTone: Record<SyncState, string> = { synced: 'ok', pending: 'warn', conflict: 'bad', none: '' };

/** The command count or file of one Agent binding, on one quiet line. */
function summary(entry: HookEntry, agent: string, t: ReturnType<typeof useT>) {
  const binding = entry.bindings[agent] ?? entry.bindings.factory;
  if (isCodeAgent(agent)) return t('hooks.summary.code');
  const events = Object.keys(binding?.events ?? {});
  return events.length > 0 ? events.join(', ') : t('hooks.summary.noEvents');
}

/**
 * One row per hook. The Agents it is bound to fold out into a per-Agent line that says two separate
 * things: whether Skillshare has written the native file (synchronized), and that the Agent itself
 * decides whether to trust and load it (native).
 */
export default function HooksList({ entries, plan, paths, onToggle, onMenu, disabled = false }: Props) {
  const t = useT();
  const [open, setOpen] = useState<string[]>([]);
  return (
    <div className="ss-list">
      {Object.entries(entries).map(([name, entry]) => {
        const agents = boundAgents(entry);
        const enabled = entry.enabled !== false;
        const expanded = open.includes(name);
        const states = agents.map((a) => syncState(plan, name, a));
        return (
          <Fragment key={name}>
            <div className="ss-r">
              <span className="ss-cat sm extra"><Webhook size={14} /></span>
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                <span className="flex items-center gap-2">
                  <span title={name} className={`truncate font-mono font-semibold ${enabled ? '' : 'text-ink-3'}`}>{name}</span>
                  {!enabled && <span className="ss-tag">{t('hooks.disabled')}</span>}
                  {states.includes('pending') && <span className="ss-tag warn">{t('plugins.pending')}</span>}
                  {states.includes('conflict') && <span className="ss-tag bad">{t('hooks.status.conflict')}</span>}
                  {agents.length === 0 && <span className="ss-tag">{t('plugins.noAgentsYet')}</span>}
                </span>
                <span className="truncate text-xs text-ink-3">{entry.description || t('hooks.noDescription')}</span>
              </span>
              <button type="button" role="switch" aria-checked={enabled} aria-label={t('hooks.enableSwitch', { name })} className="grid h-7 shrink-0 place-items-center disabled:opacity-50" disabled={disabled} onClick={() => onToggle(name, !enabled)}>
                <span className={`ss-sw ${enabled ? 'on' : ''}`}><i /></span>
              </button>
              <TargetPill selected={agents} text={String(agents.length)} expanded={expanded} label={t('hooks.showAgents', { name })} onClick={() => setOpen((prev) => (expanded ? prev.filter((x) => x !== name) : [...prev, name]))} />
              <button type="button" className="ss-ib" aria-label={t('hooks.moreActions', { name })} onClick={(e) => onMenu(e, name)}>
                <Ellipsis size={16} />
              </button>
            </div>
            {expanded && (
              <div className="ss-r fold !min-h-0 flex-col !items-stretch gap-2.5 !py-3.5">
                {agents.length === 0 && <span className="text-[13px] text-ink-2">{t('hooks.noAgentsHint')}</span>}
                {agents.map((agent, i) => (
                  <div key={agent} className="flex min-w-0 items-center gap-2.5 text-[13px]">
                    <span className="ss-at"><AgentIcon target={agent} size={17} /></span>
                    <span className="w-[92px] shrink-0 font-semibold">{hookLabel(agent)}</span>
                    <span className="ss-tag">{t(isCodeAgent(agent) ? 'hooks.kind.code' : 'hooks.kind.command')}</span>
                    <span className="min-w-0 flex-1 truncate text-xs text-ink-3" title={paths[agent]}>{summary(entry, agent, t)}{paths[agent] && <> · <span className="font-mono">{shortenHome(paths[agent])}</span></>}</span>
                    {/* Synchronized: Skillshare has (or has not yet) written the native registration or file. */}
                    <span className={`ss-st ${stateTone[states[i]]}`}>{t(`hooks.sync.${states[i]}`)}</span>
                  </div>
                ))}
                {agents.length > 0 && <span className="flex items-start gap-1.5 text-xs text-ink-2"><Info size={13} className="mt-px shrink-0" />{t('hooks.nativeNote')}</span>}
              </div>
            )}
          </Fragment>
        );
      })}
    </div>
  );
}
