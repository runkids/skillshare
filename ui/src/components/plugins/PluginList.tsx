import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Check, ChevronDown, Ellipsis, Loader2, Package } from 'lucide-react';
import {
  bindingStatus,
  bindingVersion,
  pluginsApi,
  syncAction,
  targetMap,
  type PluginBinding,
  type PluginInventory,
  type PluginRequest,
  type PluginTarget,
} from '../../api/plugins';
import type { PluginUpdate } from '../../api/plugins';
import AgentIcon from '../AgentIcon';
import { Checkbox } from '../Checkbox';
import Reveal from '../Reveal';
import Spinner from '../Spinner';
import { agentReasons } from './agentReasons';
import SourceHint from './SourceHint';
import { pluginErrorMessage } from './pluginError';
import { groupBySource, groupKey, packageSource } from './pluginsFlow';
import { useT } from '../../i18n';
import { useAppContext } from '../../context/AppContext';
import { shortenPath } from '../../lib/paths';
import { parseRemoteURL } from '../../lib/parseRemoteURL';
import PiPackageIcon from '../PiPackageIcon';
import { queryKeys } from '../../lib/queryKeys';

const STACK = 6;

interface Props {
  inventory: PluginInventory;
  /** The packages this list shows, in order. */
  names: string[];
  /** Pi or Herdr packages, which all show that product's mark. */
  kind?: 'pi' | 'herdr';
  /** The version and commit each plugin's source has, from the last update check. */
  updates?: Record<string, Record<PluginTarget, PluginUpdate>>;
  /** Why the last update check could not check a plugin, by name. */
  checkFailures?: Record<string, string>;
  busy: boolean;
  /** `name:target` of the selection being applied right now. */
  working: string;
  onToggle: (name: string, target: PluginTarget, on: boolean) => void;
  onMenu: (e: React.MouseEvent<HTMLButtonElement>, name: string) => void;
  /** Ticking an Agent the plugin is not in yet: an install, so it goes through the preview. */
  onAdd: (request: PluginRequest, key: string) => void;
  /** The Agents that cannot take it, and why: the dialog has the room to say so. */
  onBlocked: (name: string, source?: string) => void;
  /** The menu of the packages one source added, shown as a group. */
  onGroupMenu: (e: React.MouseEvent<HTMLButtonElement>, names: string[]) => void;
  /** Packages ticked for the selection bar; without `onTick` the rows have no boxes. */
  ticked?: Set<string>;
  onTick?: (names: string[], on: boolean) => void;
}

/**
 * One row per plugin, agents as toggles inside it: the same shape as the MCP server list, for the same reason.
 * Packages added from one source fold into one group row, so a repo of ten mods is one line until opened.
 */
export default function PluginList(props: Props) {
  return (
    <div className="ss-list">
      {groupBySource(props.inventory, props.names).map((members) =>
        members.length > 1 ? (
          <Group key={groupKey(members)} members={members} {...props} />
        ) : (
          <Row key={members[0]} name={members[0]} {...props} />
        ),
      )}
    </div>
  );
}

/** A row's tile: what its section holds, or a plugin's own logo when it has one. */
function Tile({ kind, logo }: { kind: Props['kind']; logo?: string }) {
  if (kind === 'pi')
    return (
      <span className="ss-cat sm bg-sunken text-ink">
        <PiPackageIcon size={22} />
      </span>
    );
  if (kind === 'herdr')
    return (
      <span className="ss-cat sm bg-sunken">
        <AgentIcon target="herdr-app" size={18} />
      </span>
    );
  return (
    <span className="ss-cat plugin sm">
      {logo ? (
        <img src={logo} alt="" className="size-full rounded-[inherit] object-cover" />
      ) : (
        <Package size={14} />
      )}
    </span>
  );
}

function Group({ members, ...props }: Props & { members: string[] }) {
  const { inventory, busy, working, onGroupMenu, ticked, onTick } = props;
  const t = useT();
  const [expanded, setExpanded] = useState(false);
  const source = packageSource(inventory, members[0])?.source ?? '';
  const label = parseRemoteURL(source)?.ownerRepo ?? shortenPath(source);
  const bindings = members.map(
    (name) => Object.entries(inventory.packages[name].bindings) as [PluginTarget, PluginBinding][],
  );
  const on = bindings.filter((list) => list.some(([, b]) => b.sync !== false)).length;
  const targets = [
    ...new Set(
      bindings.flatMap((list) => list.filter(([, b]) => b.sync !== false).map(([at]) => at)),
    ),
  ];
  const pending = bindings.some((list) =>
    list.some(([target, b]) =>
      syncAction(
        b,
        inventory.hosts.find((h) => h.target === target),
      ),
    ),
  );
  return (
    <>
      <div className={`ss-r ${members.every((name) => ticked?.has(name)) ? 'sel' : ''}`}>
        {onTick && (
          <Checkbox
            hideLabel
            className="-m-1 p-1"
            label={t('resources.select.group', { name: label })}
            checked={members.every((name) => ticked?.has(name))}
            indeterminate={
              members.some((name) => ticked?.has(name)) &&
              !members.every((name) => ticked?.has(name))
            }
            onChange={(on) => onTick(members, on)}
          />
        )}
        <Tile kind={props.kind} />
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="flex items-center gap-2">
            <span title={source} className="truncate font-mono font-semibold">
              {label}
            </span>
            {pending && <span className="ss-tag warn">{t('plugins.pending')}</span>}
          </span>
          {/* Closed, the line says what is inside; open, the rows do, and it says where they come from. */}
          <span
            className="truncate text-xs text-ink-3"
            title={expanded ? source : members.join(', ')}
          >
            {expanded ? (
              <span className="font-mono">{shortenPath(source)}</span>
            ) : (
              members.join(', ')
            )}
          </span>
        </span>
        <button
          type="button"
          className={`ss-btn ${targets.length > 0 ? '!pl-1.5' : ''}`}
          aria-expanded={expanded}
          aria-label={t('plugins.groupShow', { name: label })}
          onClick={() => setExpanded(!expanded)}
        >
          {targets.length > 0 && (
            <span className="ss-stack" aria-hidden="true">
              {targets.slice(0, STACK).map((target) => (
                <span key={target} className="ss-at">
                  <AgentIcon target={target} size={13} />
                </span>
              ))}
            </span>
          )}
          {on}/{members.length}
          <ChevronDown
            size={14}
            className={`transition-transform duration-200 ease-[cubic-bezier(.3,.7,.3,1)] motion-reduce:transition-none ${expanded ? 'rotate-180' : ''}`}
          />
        </button>
        <button
          type="button"
          className="ss-ib"
          aria-label={t('mcp.moreActions', { name: label })}
          onClick={(e) => onGroupMenu(e, members)}
          disabled={busy}
        >
          {working === groupKey(members) ? (
            <Loader2 size={16} className="animate-spin" />
          ) : (
            <Ellipsis size={16} />
          )}
        </button>
      </div>
      <Reveal open={expanded} className="ss-rv">
        {members.map((name) => (
          <Row key={name} name={name} nested {...props} />
        ))}
      </Reveal>
    </>
  );
}

function Row({
  name,
  nested,
  kind,
  inventory,
  updates,
  checkFailures,
  busy,
  working,
  onToggle,
  onMenu,
  onAdd,
  onBlocked,
  ticked,
  onTick,
}: Props & { name: string; nested?: boolean }) {
  const t = useT();
  const { isProjectMode } = useAppContext();
  const pluginTargets = targetMap(inventory.targetDefinitions);
  const [expanded, setExpanded] = useState(false);
  const pack = inventory.packages[name];
  const bindings = Object.entries(pack.bindings) as [
    PluginTarget,
    NonNullable<(typeof pack.bindings)[PluginTarget]>,
  ][];
  const selected = bindings.filter(([, b]) => b.sync !== false).map(([target]) => target);
  // The Agents the last check found an update for, until an update takes them off. A source change
  // without a new version counts too; the version only says what the update brings.
  const checked = updates?.[name] ?? {};
  const next = Object.values(checked).find((u) => u.version)?.version;
  const nextCommit = Object.values(checked).find((u) => u.commit)?.commit;
  const commits = [...new Set(bindings.map(([, b]) => b.commit).filter(Boolean))];
  const behind = selected.filter(
    (target) => target in checked && pluginTargets[target]?.operations.includes('update'),
  );
  const versions = [
    ...new Set(bindings.map(([target, b]) => bindingVersion(inventory, target, b)).filter(Boolean)),
  ];
  // With no Agent yet, the version recorded when the plugin was added is all there is; a plugin
  // added before that was recorded asks its source, through the same query the row opens with.
  const parts = [...new Set(bindings.flatMap(([, b]) => b.components ?? []))];
  const from = bindings.find(([, b]) => b.source)?.[1];
  // A plugin with no Agent yet has only what was recorded when it was added.
  const recipe = from?.source ?? pack.source;
  const source = recipe?.startsWith('npm:') ? undefined : recipe;
  const sourceRef = from?.sourceRef ?? pack.sourceRef;
  const entry = bindings.find(([, b]) => b.entry)?.[1].entry ?? pack.entry;
  const meta = parts.join(', ');
  const rowBusy = working === name;
  // Which other Agents can take it, and the logo its Codex manifest names, are the source's answer,
  // not config's, so the row asks for them. Its own key, outside `plugins`: a toggle must not send it back to the network.
  // ponytail: one discovery per plugin per session; give it a refresh control if sources change under an open dashboard.
  const found = useQuery({
    // The name picks the snapshot: two plugins of one source can be at different commits.
    queryKey: queryKeys.pluginDiscover(source, sourceRef, entry, name),
    queryFn: () => pluginsApi.discover(source!, sourceRef, entry, name),
    enabled: !!source,
    staleTime: Infinity,
    retry: false,
  });
  const plugin = bindings.find(([, b]) => b.plugin)?.[1].plugin ?? pack.plugin;
  const candidate =
    found.data?.candidates.find((c) => c.name === plugin) ??
    (found.data?.candidates.length === 1 ? found.data.candidates[0] : undefined);
  if (bindings.length === 0) {
    const known = pack.version ?? candidate?.version;
    if (known) versions.push(known);
  }
  const codex = candidate?.targets.includes('codex') ? candidate.targetInfo?.codex : undefined;
  const logo = codex && !codex.problem ? codex.logo : undefined;
  const others = candidate
    ? agentReasons(candidate, pluginTargets, isProjectMode, t).filter(
        (r) => !pack.bindings[r.target],
      )
    : [];
  // An npm package, imported or added, records no source: Pi installs it from its identifier,
  // so the other Pi targets need no discovery, as in the add dialog.
  const npm = recipe?.startsWith('npm:')
    ? recipe
    : source
      ? undefined
      : bindings.find(([, b]) => b.id.startsWith('npm:'))?.[1].id;
  const npmOthers = npm
    ? (inventory.targetDefinitions ?? []).filter(
        (d) =>
          d.npm &&
          d.operations.includes('add') &&
          (!isProjectMode || d.project) &&
          !pack.bindings[d.target],
      )
    : [];
  const blocked = others.filter((r) => r.reason).length;
  const total = bindings.length + others.length - blocked + npmOthers.length;
  // A Herdr mod or a Pi package that only one Agent can take: the row already names it, so a 1/1
  // picker and a fold repeating that Agent say nothing. A switch keeps turning it off and on again.
  const only = kind && total === 1 && bindings.length === 1 ? bindings[0] : undefined;
  const onlyHost = only && inventory.hosts.find((h) => h.target === only[0]);
  // The fold's state tag, minus the two the row already shows.
  const onlyState = only ? bindingStatus(only[1], onlyHost) : '';
  const onlyTag =
    onlyState !== 'plugins.pending' && onlyState !== 'plugins.importRetained' ? onlyState : '';
  return (
    <>
      {/* Inside a group the group row has the tile and the source, so the row keeps neither. */}
      <div className={`ss-r ${ticked?.has(name) ? 'sel' : ''}`}>
        {onTick && (
          <Checkbox
            hideLabel
            className="-m-1 p-1"
            label={name}
            checked={!!ticked?.has(name)}
            onChange={(on) => onTick([name], on)}
          />
        )}
        {/* Under its group's tile, so its name lines up with the group's. */}
        {nested ? <span className="w-[26px] flex-none" /> : <Tile kind={kind} logo={logo} />}
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="flex items-center gap-2">
            <span title={name} className="truncate font-mono font-semibold">
              {name}
            </span>
            {/* Agents at different versions, as after an update that skipped one, show each. */}
            {versions.length > 1 ? (
              <span
                className="ss-tag shrink-0 font-mono"
                title={bindings
                  .filter(([target, b]) => bindingVersion(inventory, target, b))
                  .map(
                    ([target, b]) =>
                      `${pluginTargets[target]?.label ?? target} ${bindingVersion(inventory, target, b)}`,
                  )
                  .join(' · ')}
              >
                {versions.join(' / ')}
              </span>
            ) : (
              <VersionChange
                from={versions[0]}
                to={next}
                fromCommit={commits.length === 1 ? commits[0] : undefined}
                toCommit={nextCommit}
              />
            )}
            {/* A check found another version: update it from here, through the same review as the menu. */}
            {behind.length > 0 && (
              <button
                type="button"
                className="ss-more min-h-6 shrink-0 disabled:opacity-50"
                disabled={busy}
                onClick={() => onAdd({ action: 'update', name, targets: behind }, name)}
              >
                {t('plugins.update')}
              </button>
            )}
            {checkFailures?.[name] && (
              <span className="ss-tag warn shrink-0" title={checkFailures[name]}>
                {t('plugins.checkFailed')}
                <span className="sr-only">: {checkFailures[name]}</span>
              </span>
            )}
            {bindings.some(([target, b]) =>
              syncAction(
                b,
                inventory.hosts.find((h) => h.target === target),
              ),
            ) && <span className="ss-tag warn">{t('plugins.pending')}</span>}
            {bindings.some(
              ([target, b]) =>
                bindingStatus(
                  b,
                  inventory.hosts.find((h) => h.target === target),
                ) === 'plugins.importRetained',
            ) && <span className="ss-tag">{t('plugins.importRetained')}</span>}
            {bindings.length === 0 && <span className="ss-tag">{t('plugins.noAgentsYet')}</span>}
            {onlyTag && <span className="ss-tag">{t(onlyTag)}</span>}
          </span>
          {/* One quiet line instead of a tag, a label and a path each asking to be read. */}
          {/* In a group the group row has the source, and a Herdr mod's line says only `mods`, as all its siblings do. */}
          {(!nested || (kind !== 'herdr' && meta)) && (
            <span className="truncate text-xs text-ink-3" title={source}>
              {meta}
              {meta && source && !nested && ' · '}
              {source && !nested && <span className="font-mono">{shortenPath(source)}</span>}
            </span>
          )}
        </span>
        {only && (
          // The same pill as a row with more Agents, without the fold: a click turns the one Agent off or on.
          <button
            type="button"
            role="switch"
            aria-checked={only[1].sync !== false}
            aria-label={`${t('plugins.targets')} ${pluginTargets[only[0]]?.label ?? only[0]}`}
            // Busy elsewhere, it stays as it is, like a tick in a fold; only a row that cannot change dims.
            className={`ss-btn !pl-1.5 ${only[1].cleanup || only[1].contextChanged ? '' : 'disabled:!opacity-100'}`}
            disabled={busy || !!only[1].cleanup || !!only[1].contextChanged}
            onClick={() => onToggle(name, only[0], only[1].sync === false)}
          >
            {/* The logo stays mounted, so turning it off greys it and turning it on brings it back. */}
            <span className="ss-stack" aria-hidden="true">
              <span className="ss-at ss-one">
                {working === `${name}:${only[0]}` ? (
                  <Loader2 size={13} className="animate-spin" />
                ) : (
                  <AgentIcon target={only[0]} size={13} />
                )}
              </span>
            </span>
            {selected.length}/1
          </button>
        )}
        {!only && (
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
                  <span key={target} className="ss-at">
                    <AgentIcon target={target} size={13} />
                  </span>
                ))}
              </span>
            )}
            {selected.length}/{total}
            <ChevronDown
              size={14}
              className={`transition-transform duration-200 ease-[cubic-bezier(.3,.7,.3,1)] motion-reduce:transition-none ${expanded ? 'rotate-180' : ''}`}
            />
          </button>
        )}
        <button
          type="button"
          className="ss-ib"
          aria-label={t('mcp.moreActions', { name })}
          onClick={(e) => onMenu(e, name)}
          disabled={busy}
        >
          {rowBusy ? <Loader2 size={16} className="animate-spin" /> : <Ellipsis size={16} />}
        </button>
      </div>
      <Reveal open={expanded && !only} className="ss-rv">
        <div className="ss-r fold !min-h-0 flex-wrap gap-x-6 gap-y-3.5 !py-3.5">
          {bindings.map(([target, b]) => {
            const on = b.sync !== false;
            const host = inventory.hosts.find((h) => h.target === target);
            const todo = syncAction(b, host);
            // Only what the tick does not already say: a ticked Agent that has the plugin needs no label.
            const state = bindingStatus(b, host);
            return (
              <span key={target} className="inline-flex items-center gap-2">
                <Toggle
                  target={target}
                  label={pluginTargets[target]?.label ?? target}
                  title={b.id}
                  on={on}
                  applying={working === `${name}:${target}`}
                  disabled={busy || !!b.cleanup || !!b.contextChanged}
                  onClick={() => onToggle(name, target, !on)}
                />
                {state && <span className={`ss-tag ${todo ? 'warn' : ''}`}>{t(state)}</span>}
              </span>
            );
          })}
          {others
            .filter((r) => !r.reason)
            .map(({ target, definition }) => (
              <Toggle
                key={target}
                target={target}
                label={definition.label}
                on={false}
                applying={working === `${name}:${target}`}
                disabled={busy}
                onClick={() =>
                  onAdd(
                    {
                      action: 'add',
                      source: found.data!.source,
                      sourceRef: found.data!.sourceRef || undefined,
                      entry: entry || undefined,
                      plugin: candidate!.name,
                      name,
                      targets: [target],
                    },
                    `${name}:${target}`,
                  )
                }
              />
            ))}
          {npmOthers.map((definition) => (
            <Toggle
              key={definition.target}
              target={definition.target}
              label={definition.label}
              on={false}
              applying={working === `${name}:${definition.target}`}
              disabled={busy}
              onClick={() =>
                onAdd(
                  { action: 'add', source: npm, name, targets: [definition.target] },
                  `${name}:${definition.target}`,
                )
              }
            />
          ))}
          {found.isFetching && (
            <span className="flex items-center gap-2 text-[13px] text-ink-3">
              <Spinner size="sm" />
              {t('plugins.discovering')}
            </span>
          )}
          {(found.isFetching || found.error) && <SourceHint active={found.isFetching} />}
          {found.error && (
            <span className="text-xs text-ink-3">{pluginErrorMessage(found.error, t)}</span>
          )}
          {blocked > 0 && (
            <button
              type="button"
              className="ss-more"
              disabled={busy}
              onClick={() => onBlocked(name, source)}
            >
              {t('plugins.moreBlocked', { count: blocked })}
            </button>
          )}
        </div>
      </Reveal>
    </>
  );
}

/** The installed version as a tag, and `old → new` when the source has another one. */
export function VersionChange({
  from,
  to,
  fromCommit,
  toCommit,
}: {
  from?: string;
  to?: string;
  fromCommit?: string;
  toCommit?: string;
}) {
  const next = to && to !== from ? to : undefined;
  // A source that changed without a new version: the commit is what moved.
  const moved =
    !next && fromCommit && toCommit && fromCommit !== toCommit
      ? `${fromCommit.slice(0, 7)} → ${toCommit.slice(0, 7)}`
      : undefined;
  if (!from && !next && !moved) return null;
  return (
    <span className={`ss-tag shrink-0 font-mono ${next || moved ? 'inf' : ''}`}>
      {next ? `${from ?? '?'} → ${next}` : [from, moved].filter(Boolean).join(' · ')}
    </span>
  );
}

interface ToggleProps {
  target: PluginTarget;
  label: string;
  title?: string;
  on: boolean;
  applying: boolean;
  disabled: boolean;
  onClick: () => void;
}

export function Toggle({ target, label, title, on, applying, disabled, onClick }: ToggleProps) {
  return (
    <button
      type="button"
      role="checkbox"
      aria-checked={on}
      aria-label={label}
      title={title}
      className={`ss-tgl ${on ? 'on' : ''} ${applying ? 'working' : ''}`}
      onClick={onClick}
      disabled={disabled}
    >
      <span className="ic">
        <AgentIcon target={target} size={20} />
        <i>
          {applying ? (
            <Loader2 size={9} className="animate-spin" />
          ) : (
            <Check size={9} strokeWidth={3.5} />
          )}
        </i>
      </span>
      {label}
    </button>
  );
}
