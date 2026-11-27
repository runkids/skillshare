import { Ellipsis } from 'lucide-react';
import { useState } from 'react';
import type { HookEntry, HookPlan } from '../../api/hooks';
import { useT } from '../../i18n';
import AgentIcon from '../AgentIcon';
import { bindingLines } from './hookCatalog';
import {
  boundAgents,
  agentOfKey,
  keyLabel,
  isCodeAgent,
  syncState,
  type SyncState,
} from './hooksView';
import { SwitchMark } from '../Switch';

interface Props {
  entries: Record<string, HookEntry>;
  accounts?: Record<string, string>;
  plan: HookPlan | null;
  onToggle: (name: string, enabled: boolean) => void;
  onMenu: (e: React.MouseEvent<HTMLButtonElement>, name: string) => void;
  /** Holds the switches while a save is on its way: each sends the revision it previewed. */
  disabled?: boolean;
  /** One row per hook in a single list, its first line only, for a project's tab. */
  compact?: boolean;
}

/**
 * What a hook runs, one line per distinct event, matcher and command, with the targets that run it.
 * Event names that differ only in case (SessionStart, sessionStart) are one line.
 */
function summaryLines(entry: HookEntry, accounts: Record<string, string>) {
  const rows = new Map<
    string,
    { event: string; matcher: string; command: string; agents: string[] }
  >();
  for (const agent of boundAgents(entry, accounts)) {
    const binding = entry.bindings[agent] ?? entry.bindings.factory;
    const lines = isCodeAgent(agentOfKey(accounts, agent))
      ? [{ event: keyLabel(accounts, agent), matcher: '', command: '' }]
      : bindingLines(binding);
    for (const line of lines) {
      const key = `${line.event.toLowerCase()}\0${line.matcher}\0${line.command}`;
      const row = rows.get(key);
      if (!row) rows.set(key, { ...line, agents: [agent] });
      else if (!row.agents.includes(agent)) row.agents.push(agent);
    }
  }
  return [...rows.values()];
}

const hookStatus = (
  t: ReturnType<typeof useT>,
  enabled: boolean,
  agentCount: number,
  states: SyncState[],
) =>
  !enabled
    ? t('hooks.disabled')
    : agentCount === 0
      ? t('plugins.noAgentsYet')
      : states.includes('conflict')
        ? t('hooks.status.conflict')
        : states.includes('inactive')
          ? t('hooks.status.inactive')
          : states.includes('pending')
            ? t('plugins.pending')
            : t('hooks.sync.synced');

const STACK = 3;

/** The hooks of a project as rows of one list: name, state, targets and switch, then what the first line runs. */
function CompactList({
  entries,
  plan,
  onToggle,
  onMenu,
  disabled,
  accounts,
}: Required<Omit<Props, 'compact'>>) {
  const t = useT();
  return (
    <div className="ss-list">
      {Object.entries(entries).map(([name, entry]) => {
        const agents = boundAgents(entry, accounts);
        const enabled = entry.enabled !== false;
        const states = agents.map((a) => syncState(plan, name, a));
        const conflict = enabled && states.includes('conflict');
        // A synced hook needs no word; every other state does.
        const synced = enabled && agents.length > 0 && states.every((s) => s === 'synced');
        const lines = summaryLines(entry, accounts);
        const text = (l: (typeof lines)[number]) =>
          [l.matcher ? `${l.event} · ${l.matcher}` : l.event, l.command]
            .filter(Boolean)
            .join(' → ');
        const first = lines[0];
        return (
          <article
            key={name}
            aria-label={name}
            className="flex flex-col gap-1 border-t border-line px-4 py-3 first:border-t-0"
          >
            <div className="flex min-w-0 items-center gap-2.5">
              <span
                title={name}
                className={`min-w-0 shrink-0 truncate font-mono text-[13.5px] font-semibold ${enabled ? '' : 'text-ink-3'}`}
              >
                {name}
              </span>
              {!synced && (
                <span className={`ss-tag shrink-0 !font-sans ${conflict ? 'bad' : ''}`}>
                  {hookStatus(t, enabled, agents.length, states)}
                </span>
              )}
              <span
                className="min-w-0 flex-1 truncate text-[13px] text-ink-3"
                title={entry.description}
              >
                {entry.description}
              </span>
              <span className={`ss-stack shrink-0 ${enabled ? '' : 'opacity-45 grayscale'}`}>
                {agents.slice(0, STACK).map((a) => (
                  <span key={a} className="ss-at" title={keyLabel(accounts, a)} aria-hidden="true">
                    <AgentIcon target={agentOfKey(accounts, a)} size={13} />
                  </span>
                ))}
                {agents.length > STACK && (
                  <span className="ss-at text-[11px] font-semibold text-ink-2" aria-hidden="true">
                    +{agents.length - STACK}
                  </span>
                )}
                {/* Every target keeps its state for screen readers, those past the stack too. */}
                {agents.map((a, i) => (
                  <span key={a} className="sr-only">
                    {keyLabel(accounts, a)} · {t(`hooks.sync.${states[i]}`)}
                  </span>
                ))}
              </span>
              <button
                type="button"
                role="switch"
                aria-checked={enabled}
                aria-label={t('hooks.enableSwitch', { name })}
                className="grid h-7 shrink-0 place-items-center disabled:opacity-50"
                disabled={disabled}
                onClick={() => onToggle(name, !enabled)}
              >
                <SwitchMark checked={enabled} />
              </button>
              <button
                type="button"
                className="ss-ib shrink-0"
                aria-label={t('hooks.moreActions', { name })}
                onClick={(e) => onMenu(e, name)}
              >
                <Ellipsis size={16} />
              </button>
            </div>
            {first && (
              <div
                className={`flex min-w-0 items-baseline gap-2 font-mono text-[12px] ${enabled ? 'text-ink-2' : 'text-ink-3'}`}
                title={lines.map(text).join('\n')}
              >
                <span className="max-w-[40%] shrink-0 truncate">
                  {first.event}
                  {first.matcher && <span className="text-ink-3"> · {first.matcher}</span>}
                </span>
                {first.command ? (
                  <>
                    <span className="shrink-0 text-ink-3">→</span>
                    <span className="min-w-0 truncate">{first.command}</span>
                  </>
                ) : (
                  <span className="font-sans text-xs text-ink-3">{t('hooks.summary.code')}</span>
                )}
                {lines.length > 1 && (
                  <span className="shrink-0 font-sans text-xs text-ink-3">
                    {t('hooks.moreLines', { count: lines.length - 1 })}
                  </span>
                )}
              </div>
            )}
          </article>
        );
      })}
    </div>
  );
}

/**
 * One card per hook: its state, what it runs and the targets it goes to. The per-target sync state
 * is on each icon; whether a target trusts and loads the hook is that target's own call.
 */
export default function HooksList({
  entries,
  plan,
  onToggle,
  onMenu,
  disabled = false,
  accounts = {},
  compact = false,
}: Props) {
  const t = useT();
  // The line under the pointer lights up its own targets in the header; the rest fade.
  const [hovered, setHovered] = useState<{ name: string; agents: string[] } | null>(null);
  if (compact) {
    return (
      <CompactList
        entries={entries}
        plan={plan}
        onToggle={onToggle}
        onMenu={onMenu}
        disabled={disabled}
        accounts={accounts}
      />
    );
  }
  return (
    <div className="flex flex-col gap-3">
      {Object.entries(entries).map(([name, entry]) => {
        const agents = boundAgents(entry, accounts);
        const enabled = entry.enabled !== false;
        const states = agents.map((a) => syncState(plan, name, a));
        const status = hookStatus(t, enabled, agents.length, states);
        const lines = summaryLines(entry, accounts);
        // Each line names its targets unless every line runs on all of them.
        const perLine = lines.some((l) => l.agents.length < agents.length);
        return (
          <article key={name} aria-label={name} className="ss-box flex flex-col gap-2.5 !p-4">
            <div className="flex items-center gap-2.5">
              <span
                title={name}
                className={`min-w-0 truncate font-mono font-semibold ${enabled ? '' : 'text-ink-3'}`}
              >
                {name}
              </span>
              <span
                className={`ss-tag !font-sans ${states.includes('conflict') && enabled ? 'bad' : ''}`}
              >
                {enabled && states.includes('pending') && !states.includes('conflict') && (
                  <span className="h-[5px] w-[5px] rounded-full bg-ink" aria-hidden="true" />
                )}
                {status}
              </span>
              <span className="flex-1" />
              <span className={`ss-stack ${enabled ? '' : 'opacity-45 grayscale'}`}>
                {agents.map((a, i) => (
                  <span
                    key={a}
                    title={`${keyLabel(accounts, a)} · ${t(`hooks.sync.${states[i]}`)}`}
                    className={`ss-at transition-opacity ${hovered?.name === name && !hovered.agents.includes(a) ? 'opacity-25' : ''}`}
                  >
                    <AgentIcon target={agentOfKey(accounts, a)} size={14} />
                    <span className="sr-only">
                      {keyLabel(accounts, a)} · {t(`hooks.sync.${states[i]}`)}
                    </span>
                  </span>
                ))}
              </span>
              <button
                type="button"
                role="switch"
                aria-checked={enabled}
                aria-label={t('hooks.enableSwitch', { name })}
                className="grid h-7 shrink-0 place-items-center disabled:opacity-50"
                disabled={disabled}
                onClick={() => onToggle(name, !enabled)}
              >
                <SwitchMark checked={enabled} />
              </button>
              <button
                type="button"
                className="ss-ib"
                aria-label={t('hooks.moreActions', { name })}
                onClick={(e) => onMenu(e, name)}
              >
                <Ellipsis size={16} />
              </button>
            </div>
            {entry.description && (
              <span className="text-[13px] text-ink-2">{entry.description}</span>
            )}
            {lines.length > 0 && (
              <div
                className={`flex flex-col gap-1 border-t border-line pt-2.5 font-mono text-[12.5px] ${enabled ? '' : 'text-ink-3'}`}
              >
                {lines.map((l) => (
                  <div
                    key={`${l.event}\0${l.matcher}\0${l.command}`}
                    className={`flex min-w-0 items-baseline gap-2 ${perLine ? '-mx-2 rounded-md px-2 hover:bg-sunken' : ''}`}
                    onMouseEnter={
                      perLine ? () => setHovered({ name, agents: l.agents }) : undefined
                    }
                    onMouseLeave={perLine ? () => setHovered(null) : undefined}
                  >
                    <span
                      className="w-[150px] shrink-0 truncate"
                      title={l.matcher ? `${l.event} · ${l.matcher}` : l.event}
                    >
                      {l.event}
                      {l.matcher && <span className="text-ink-2"> · {l.matcher}</span>}
                    </span>
                    {l.command ? (
                      <>
                        <span className="shrink-0 text-ink-3">→</span>
                        <span className="min-w-0 flex-1 truncate" title={l.command}>
                          {l.command}
                        </span>
                      </>
                    ) : (
                      <span className="flex-1 font-sans text-xs text-ink-3">
                        {t('hooks.summary.code')}
                      </span>
                    )}
                    {perLine &&
                      l.agents.map((a) => (
                        <span key={a} className="sr-only">
                          {keyLabel(accounts, a)}
                        </span>
                      ))}
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
