import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Check, ChevronDown, Ellipsis, Loader2, Package } from 'lucide-react';
import { bindingVersion, pluginsApi, syncAction, targetMap, type PluginInventory, type PluginRequest, type PluginTarget } from '../../api/plugins';
import AgentIcon from '../AgentIcon';
import Spinner from '../Spinner';
import { agentReasons } from './agentReasons';
import SourceHint from './SourceHint';
import { pluginErrorMessage } from './pluginError';
import { useT } from '../../i18n';
import { useAppContext } from '../../context/AppContext';
import { shortenPath } from '../../lib/paths';
import PiPackageIcon from '../PiPackageIcon';

const STACK = 6;

interface Props {
  inventory: PluginInventory;
  /** The packages this list shows, in order. */
  names: string[];
  /** Pi packages, which all show Pi's mark. */
  pi?: boolean;
  /** The version each plugin's source has, from the last update check. */
  updates?: Record<string, Record<PluginTarget, string>>;
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
      {props.names.map((name) => <Row key={name} name={name} {...props} />)}
    </div>
  );
}

function Row({ name, pi, inventory, updates, busy, working, onToggle, onMenu, onAdd, onBlocked }: Props & { name: string }) {
  const t = useT();
  const { isProjectMode } = useAppContext();
  const pluginTargets = targetMap(inventory.targetDefinitions);
  const [expanded, setExpanded] = useState(false);
  const pack = inventory.packages[name];
  const bindings = Object.entries(pack.bindings) as [PluginTarget, NonNullable<(typeof pack.bindings)[PluginTarget]>][];
  const selected = bindings.filter(([, b]) => b.sync !== false).map(([target]) => target);
  // The Agents the last check found an update for, until an update takes them off. A source change
  // without a new version counts too; the version only says what the update brings.
  const checked = updates?.[name] ?? {};
  const next = Object.values(checked).find(Boolean);
  const behind = selected.filter((target) => target in checked && pluginTargets[target]?.operations.includes('update'));
  const versions = [...new Set(bindings.map(([target, b]) => bindingVersion(inventory, target, b)).filter(Boolean))];
  // With no Agent yet, the version recorded when the plugin was added is all there is; a plugin
  // added before that was recorded asks its source, through the same query the row opens with.
  const parts = [...new Set(bindings.flatMap(([, b]) => b.components ?? []))];
  const from = bindings.find(([, b]) => b.source)?.[1];
  // A plugin with no Agent yet has only what was recorded when it was added.
  const source = from?.source ?? pack.source;
  const sourceRef = from?.sourceRef ?? pack.sourceRef;
  const entry = bindings.find(([, b]) => b.entry)?.[1].entry ?? pack.entry;
  const meta = parts.join(', ');
  const rowBusy = working === name;
  // Which other Agents can take it, and the logo its Codex manifest names, are the source's answer,
  // not config's, so the row asks for them. Its own key, outside `plugins`: a toggle must not send it back to the network.
  // ponytail: one discovery per plugin per session; give it a refresh control if sources change under an open dashboard.
  const found = useQuery({
    // The name picks the snapshot: two plugins of one source can be at different commits.
    queryKey: ['plugin-discover', source, sourceRef, entry, name],
    queryFn: () => pluginsApi.discover(source!, sourceRef, entry, name),
    enabled: !!source,
    staleTime: Infinity,
    retry: false,
  });
  const plugin = bindings.find(([, b]) => b.plugin)?.[1].plugin ?? pack.plugin;
  const candidate = found.data?.candidates.find((c) => c.name === plugin) ?? (found.data?.candidates.length === 1 ? found.data.candidates[0] : undefined);
  if (bindings.length === 0) {
    const known = pack.version ?? candidate?.version;
    if (known) versions.push(known);
  }
  const codex = candidate?.targets.includes('codex') ? candidate.targetInfo?.codex : undefined;
  const logo = codex && !codex.problem ? codex.logo : undefined;
  const others = candidate ? agentReasons(candidate, pluginTargets, isProjectMode, t).filter((r) => !pack.bindings[r.target]) : [];
  // An npm package, imported or added, records no source: Pi installs it from its identifier,
  // so the other Pi targets need no discovery, as in the add dialog.
  const npm = source ? undefined : bindings.find(([, b]) => b.id.startsWith('npm:'))?.[1].id;
  const npmOthers = npm ? (inventory.targetDefinitions ?? []).filter((d) => d.npm && d.operations.includes('add') && (!isProjectMode || d.project) && !pack.bindings[d.target]) : [];
  const blocked = others.filter((r) => r.reason).length;
  const total = bindings.length + others.length - blocked + npmOthers.length;
  return (
    <>
      <div className="ss-r">
        {pi ? (
          <span className="ss-cat sm bg-sunken text-ink"><PiPackageIcon size={22} /></span>
        ) : (
          <span className="ss-cat plugin sm">
            {logo ? <img src={logo} alt="" className="size-full rounded-[inherit] object-cover" /> : <Package size={14} />}
          </span>
        )}
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="flex items-center gap-2">
            <span title={name} className="truncate font-mono font-semibold">{name}</span>
            {/* Agents at different versions, as after an update that skipped one, show each. */}
            {versions.length > 1
              ? <span className="ss-tag shrink-0 font-mono" title={bindings.filter(([target, b]) => bindingVersion(inventory, target, b)).map(([target, b]) => `${pluginTargets[target]?.label ?? target} ${bindingVersion(inventory, target, b)}`).join(' · ')}>{versions.join(' / ')}</span>
              : <VersionChange from={versions[0]} to={next} />}
            {/* A check found another version: update it from here, through the same review as the menu. */}
            {behind.length > 0 && (
              <button type="button" className="ss-more min-h-6 shrink-0 disabled:opacity-50" disabled={busy} onClick={() => onAdd({ action: 'update', name, targets: behind }, name)}>{t('plugins.update')}</button>
            )}
            {bindings.some(([target, b]) => syncAction(b, inventory.hosts.find((h) => h.target === target))) && <span className="ss-tag warn">{t('plugins.pending')}</span>}
            {bindings.length === 0 && <span className="ss-tag">{t('plugins.noAgentsYet')}</span>}
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
          {npmOthers.map((definition) => (
            <Toggle
              key={definition.target} target={definition.target} label={definition.label} on={false} applying={working === `${name}:${definition.target}`} disabled={busy}
              onClick={() => onAdd({ action: 'add', source: npm, name, targets: [definition.target] }, `${name}:${definition.target}`)}
            />
          ))}
          {found.isFetching && <span className="flex items-center gap-2 text-[13px] text-ink-3"><Spinner size="sm" />{t('plugins.discovering')}</span>}
          {(found.isFetching || found.error) && <SourceHint active={found.isFetching} />}
          {found.error && <span className="text-xs text-ink-3">{pluginErrorMessage(found.error, t)}</span>}
          {blocked > 0 && <button type="button" className="ss-more" disabled={busy} onClick={() => onBlocked(name, source)}>{t('plugins.moreBlocked', { count: blocked })}</button>}
        </div>
      )}
    </>
  );
}

/** The installed version as a tag, and `old → new` when the source has another one. */
export function VersionChange({ from, to }: { from?: string; to?: string }) {
  const next = to && to !== from ? to : undefined;
  if (!from && !next) return null;
  return <span className={`ss-tag shrink-0 font-mono ${next ? 'inf' : ''}`}>{from ?? '?'}{next && ` → ${next}`}</span>;
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
