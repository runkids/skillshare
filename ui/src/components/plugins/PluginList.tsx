import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Check, ChevronDown, Ellipsis, Loader2, Package } from 'lucide-react';
import { pluginsApi, syncAction, targetMap, type PluginInventory, type PluginRequest, type PluginTarget } from '../../api/plugins';
import AgentIcon from '../AgentIcon';
import Spinner from '../Spinner';
import { agentReasons } from './agentReasons';
import { useT } from '../../i18n';
import { useAppContext } from '../../context/AppContext';
import { shortenPath } from '../../lib/paths';

const STACK = 6;

interface Props {
  inventory: PluginInventory;
  busy: boolean;
  /** `name:target` of the selection being applied right now. */
  working: string;
  onToggle: (name: string, target: PluginTarget, on: boolean) => void;
  onMenu: (e: React.MouseEvent<HTMLButtonElement>, name: string) => void;
  /** Ticking an Agent the plugin is not in yet: an install, so it goes through the preview. */
  onAdd: (request: PluginRequest, key: string) => void;
  /** The Agents that cannot take it, and why: the dialog has the room to say so. */
  onBlocked: (name: string, source?: string) => void;
}

/** One row per plugin, agents as toggles inside it: the same shape as the MCP server list, for the same reason. */
export default function PluginList(props: Props) {
  return (
    <div className="ss-list">
      {Object.keys(props.inventory.packages).map((name) => <Row key={name} name={name} {...props} />)}
    </div>
  );
}

function Row({ name, inventory, busy, working, onToggle, onMenu, onAdd, onBlocked }: Props & { name: string }) {
  const t = useT();
  const { isProjectMode } = useAppContext();
  const pluginTargets = targetMap(inventory.targetDefinitions);
  const [expanded, setExpanded] = useState(false);
  const pack = inventory.packages[name];
  const bindings = Object.entries(pack.bindings) as [PluginTarget, NonNullable<(typeof pack.bindings)[PluginTarget]>][];
  const selected = bindings.filter(([, b]) => b.sync !== false).map(([target]) => target);
  const versions = [...new Set(bindings.map(([, b]) => b.version).filter(Boolean))];
  const parts = [...new Set(bindings.flatMap(([, b]) => b.components ?? []))];
  const from = bindings.find(([, b]) => b.source)?.[1];
  const source = from?.source;
  const entry = bindings.find(([, b]) => b.entry)?.[1].entry;
  const meta = [versions.length === 1 && versions[0], parts.join(', ')].filter(Boolean).join(' · ');
  const rowBusy = working === name;
  // Which other Agents can take it is the source's answer, not config's, so it is asked when the
  // row opens. Its own key, outside `plugins`: a toggle must not send it back to the network.
  // ponytail: one discovery per plugin per session; give it a refresh control if sources change under an open dashboard.
  const found = useQuery({
    queryKey: ['plugin-discover', source, from?.sourceRef, entry],
    queryFn: () => pluginsApi.discover(source!, from?.sourceRef, entry),
    enabled: expanded && !!source,
    staleTime: Infinity,
    retry: false,
  });
  const plugin = bindings.find(([, b]) => b.plugin)?.[1].plugin;
  const candidate = found.data?.candidates.find((c) => c.name === plugin) ?? (found.data?.candidates.length === 1 ? found.data.candidates[0] : undefined);
  const others = candidate ? agentReasons(candidate, pluginTargets, isProjectMode, t).filter((r) => !pack.bindings[r.target]) : [];
  const blocked = others.filter((r) => r.reason).length;
  const total = bindings.length + others.length - blocked;
  return (
    <>
      <div className="ss-r">
        <span className="ss-cat plugin sm"><Package size={14} /></span>
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="flex items-center gap-2">
            <span title={name} className="truncate font-mono font-semibold">{name}</span>
            {bindings.some(([target, b]) => syncAction(b, inventory.hosts.find((h) => h.target === target))) && <span className="ss-tag warn">{t('plugins.pending')}</span>}
          </span>
          {/* One quiet line instead of a tag, a label and a path each asking to be read. */}
          <span className="truncate text-xs text-ink-3" title={source}>
            {meta}{meta && source && ' · '}{source && <span className="font-mono">{shortenPath(source)}</span>}
          </span>
        </span>
        <button
          type="button"
          className={`ss-btn ${selected.length > 0 ? '!pl-1.5' : ''}`}
          aria-expanded={expanded}
          aria-label={t('mcp.chooseAgents', { name })}
          onClick={() => setExpanded(!expanded)}
        >
          {selected.length > 0 && (
            <span className="ss-stack" aria-hidden="true">
              {selected.slice(0, STACK).map((target) => (
                <span key={target} className="ss-at"><AgentIcon target={target} size={13} /></span>
              ))}
            </span>
          )}
          {selected.length}/{total}
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
            const host = inventory.hosts.find((h) => h.target === target);
            const installed = host?.installed.find((i) => i.id === b.id);
            const todo = syncAction(b, host);
            // Only what the tick does not already say: a ticked Agent that has the plugin needs no label.
            const state = todo ? 'plugins.pending' : !host ? '' : host.error ? 'plugins.unverified' : installed && installed.enabledKnown !== false && !installed.enabled ? 'plugins.nativeDisabled' : '';
            return (
              <span key={target} className="inline-flex items-center gap-2">
                <Toggle target={target} label={pluginTargets[target]?.label ?? target} title={b.id} on={on} applying={working === `${name}:${target}`} disabled={busy} onClick={() => onToggle(name, target, !on)} />
                {state && <span className={`ss-tag ${todo ? 'warn' : ''}`}>{t(state)}</span>}
              </span>
            );
          })}
          {others.filter((r) => !r.reason).map(({ target, definition }) => (
            <Toggle
              key={target} target={target} label={definition.label} on={false} applying={working === `${name}:${target}`} disabled={busy}
              onClick={() => onAdd({ action: 'add', source: found.data!.source, sourceRef: found.data!.sourceRef || undefined, entry: entry || undefined, plugin: candidate!.name, name, targets: [target] }, `${name}:${target}`)}
            />
          ))}
          {found.isFetching && <span className="flex items-center gap-2 text-[13px] text-ink-3"><Spinner size="sm" />{t('plugins.discovering')}</span>}
          {found.error && <span className="text-xs text-ink-3">{(found.error as Error).message}</span>}
          {blocked > 0 && <button type="button" className="ss-more" disabled={busy} onClick={() => onBlocked(name, source)}>{t('plugins.moreBlocked', { count: blocked })}</button>}
        </div>
      )}
    </>
  );
}

interface ToggleProps { target: PluginTarget; label: string; title?: string; on: boolean; applying: boolean; disabled: boolean; onClick: () => void }

function Toggle({ target, label, title, on, applying, disabled, onClick }: ToggleProps) {
  return (
    <button type="button" role="checkbox" aria-checked={on} aria-label={label} title={title} className={`ss-tgl ${on ? 'on' : ''} ${applying ? 'working' : ''}`} onClick={onClick} disabled={disabled}>
      <span className="ic"><AgentIcon target={target} size={20} /><i>{applying ? <Loader2 size={9} className="animate-spin" /> : <Check size={9} strokeWidth={3.5} />}</i></span>
      {label}
    </button>
  );
}
