import { Fragment, useState } from 'react';
import { Check, ChevronDown, Ellipsis, Loader2, Package } from 'lucide-react';
import { targetMap, type PluginInventory, type PluginTarget } from '../../api/plugins';
import AgentIcon from '../AgentIcon';
import { useT } from '../../i18n';

const STACK = 6;

interface Props {
  inventory: PluginInventory;
  busy: boolean;
  /** `name:target` of the selection being applied right now. */
  working: string;
  onToggle: (name: string, target: PluginTarget, on: boolean) => void;
  /** With a target, the menu acts on that one agent. */
  onMenu: (e: React.MouseEvent<HTMLButtonElement>, name: string, target?: PluginTarget) => void;
}

/** One row per plugin, agents as toggles inside it: the same shape as the MCP server list, for the same reason. */
export default function PluginList({ inventory, busy, working, onToggle, onMenu }: Props) {
  const t = useT();
  const pluginTargets = targetMap(inventory.targetDefinitions);
  const [open, setOpen] = useState<string[]>([]);

  return (
    <div className="ss-list">
      {Object.entries(inventory.packages).map(([name, pack]) => {
        const bindings = Object.entries(pack.bindings) as [PluginTarget, NonNullable<(typeof pack.bindings)[PluginTarget]>][];
        const selected = bindings.filter(([, b]) => b.sync !== false).map(([target]) => target);
        const versions = [...new Set(bindings.map(([, b]) => b.version).filter(Boolean))];
        const parts = [...new Set(bindings.flatMap(([, b]) => b.components ?? []))];
        const source = bindings.find(([, b]) => b.source)?.[1].source;
        const expanded = open.includes(name);
        const rowBusy = working === name;
        return (
          <Fragment key={name}>
            <div className="ss-r">
              <span className="ss-cat plugin sm"><Package size={14} /></span>
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                <span className="flex items-center gap-2">
                  <span title={name} className="truncate font-mono font-semibold">{name}</span>
                  {versions.length === 1 && <span className="ss-tag">{versions[0]}</span>}
                  {parts.length > 0 && <span className="truncate text-xs text-ink-3">{parts.join(' · ')}</span>}
                </span>
                {source && <span className="truncate font-mono text-xs text-ink-3" title={source}>{source}</span>}
              </span>
              {bindings.some(([, b]) => b.pending) && <span className="ss-st warn">{t('plugins.pending')}</span>}
              {!expanded && selected.length > 0 && (
                <span className="ss-stack" aria-hidden="true">
                  {selected.slice(0, STACK).map((target) => (
                    <span key={target} className="ss-at"><AgentIcon target={target} size={13} /></span>
                  ))}
                  {selected.length > STACK && <span className="ss-at text-[10px] font-semibold">+{selected.length - STACK}</span>}
                </span>
              )}
              <button
                type="button"
                className="ss-btn sm"
                aria-expanded={expanded}
                aria-label={t('mcp.chooseAgents', { name })}
                onClick={() => setOpen((prev) => (expanded ? prev.filter((x) => x !== name) : [...prev, name]))}
              >
                {selected.length}/{bindings.length}
                <ChevronDown size={14} className={expanded ? 'rotate-180' : ''} />
              </button>
              <button type="button" className="ss-ib" aria-label={t('mcp.moreActions', { name })} onClick={(e) => onMenu(e, name)} disabled={busy}>
                {rowBusy ? <Loader2 size={16} className="animate-spin" /> : <Ellipsis size={16} />}
              </button>
            </div>
            {expanded && (
              <div className="ss-r fold !min-h-0 flex-wrap gap-x-6 gap-y-3.5 !py-3.5">
                {bindings.map(([target, b]) => {
                  const on = b.sync !== false;
                  const applying = working === `${name}:${target}`;
                  const host = inventory.hosts.find((h) => h.target === target);
                  const installed = host?.installed.find((i) => i.id === b.id);
                  const state = b.pending ? 'plugins.pending' : host?.error ? 'plugins.unverified' : !installed ? (on ? 'plugins.absent' : '') : installed.enabledKnown !== false && !installed.enabled ? 'plugins.nativeDisabled' : 'plugins.registered';
                  return (
                    <span key={target} className="inline-flex items-center gap-2">
                      <button
                        type="button"
                        role="checkbox"
                        aria-checked={on}
                        aria-label={(pluginTargets[target]?.label ?? target)}
                        title={b.id}
                        className={`ss-tgl ${on ? 'on' : ''} ${applying ? 'working' : ''}`}
                        onClick={() => onToggle(name, target, !on)}
                        disabled={busy}
                      >
                        <span className="ic"><AgentIcon target={target} size={20} /><i>{applying ? <Loader2 size={9} className="animate-spin" /> : <Check size={9} strokeWidth={3.5} />}</i></span>
                        {(pluginTargets[target]?.label ?? target)}
                      </button>
                      {state && <span className={`ss-tag ${b.pending ? 'warn' : ''}`}>{t(state)}</span>}
                      <button type="button" className="ss-ib" aria-label={t('mcp.moreActions', { name: `${name} · ${(pluginTargets[target]?.label ?? target)}` })} onClick={(e) => onMenu(e, name, target)}>
                        <Ellipsis size={16} />
                      </button>
                    </span>
                  );
                })}
              </div>
            )}
          </Fragment>
        );
      })}
    </div>
  );
}
