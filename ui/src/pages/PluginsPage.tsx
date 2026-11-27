import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  ChevronRight,
  Download,
  FolderOpen,
  Package,
  Pause,
  Play,
  Plus,
  RefreshCw,
  Share2,
  Trash2,
  X,
} from 'lucide-react';
import {
  byPlugin,
  piShareCommand,
  pluginShareCommand,
  pluginsApi,
  targetMap,
  type PluginTarget,
  type PluginInventory,
} from '../api/plugins';
import AgentTile from '../components/AgentTile';
import Button from '../components/Button';
import { Checkbox } from '../components/Checkbox';
import DialogHeader from '../components/DialogHeader';
import DialogShell from '../components/DialogShell';
import EmptyState from '../components/EmptyState';
import IconButton from '../components/IconButton';
import PageHeader from '../components/PageHeader';
import { PageSkeleton } from '../components/Skeleton';
import { SkillContextMenu, type ContextMenuItem } from '../components/TargetMenu';
import { Select } from '../components/Select';
import Spinner from '../components/Spinner';
import { RailLayout, SyncBox } from '../components/StatusRail';
import PluginAddDialog from '../components/plugins/PluginAddDialog';
import PluginFilesDialog from '../components/plugins/PluginFilesDialog';
import PluginShareDialog from '../components/plugins/PluginShareDialog';
import PluginUpdateDialog from '../components/plugins/PluginUpdateDialog';
import PluginAgents from '../components/plugins/PluginAgents';
import { keyedMessage, keyedText, outcomeStatus } from '../components/plugins/outcomeText';
import {
  bulkActions,
  groupKey,
  herdrPackageTest,
  importPackageNames,
  importRequest,
  installedVersion,
  usePluginsFlow,
} from '../components/plugins/pluginsFlow';
import PluginList, { VersionChange } from '../components/plugins/PluginList';
import { PluginRunLine, UnchangedRuns } from '../components/plugins/PluginRuns';
import { useT, plural } from '../i18n';
import { useSlow } from '../hooks/useSlow';
import { queryKeys } from '../lib/queryKeys';
import { invalidate } from '../lib/queryEvents';
import { bulkGhost } from '../lib/exitGhost';
import ConfirmDialog from '../components/ConfirmDialog';

/** The selection bar's `working` key: a package name has no `:`, so no row's key matches it. */
const BULK = ':bulk';

export default function PluginsPage() {
  const t = useT();
  const cache = useQueryClient();
  // Config answers at once; asking every Agent's CLI takes seconds. The plugin list is drawn
  // from the quick answer until full inventory supplies verified packages and Agents.
  const quick = useQuery({
    queryKey: queryKeys.pluginPackages,
    queryFn: () => pluginsApi.list(false),
  });
  const full = useQuery({ queryKey: queryKeys.plugins, queryFn: () => pluginsApi.list() });
  const hostsReady = !!full.data;
  const hostsSlow = useSlow(!hostsReady, 10000);
  const base = full.data ?? quick.data;
  const data = base && { ...base, hosts: full.data?.hosts ?? [] };
  const error = quick.error ?? full.error;
  const [adding, setAdding] = useState<{
    source?: string;
    name?: string;
    targets?: PluginTarget[];
    bound?: PluginInventory['packages'][string]['bindings'];
    recorded?: PluginInventory['packages'][string];
  } | null>(null);
  // A target page links here with ?add=<target> to add a plugin for that target.
  const [params, setParams] = useSearchParams();
  useEffect(() => {
    const target = params.get('add');
    if (target === null) return;
    setAdding({ targets: target ? [target] : [] });
    setParams({}, { replace: true });
  }, [params, setParams]);
  const [importing, setImporting] = useState(false);
  const [importNames, setImportNames] = useState<Record<string, string>>({});
  // Rows ticked for one batch import, as `${target}:${id}`.
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [sharing, setSharing] = useState<string[] | null>(null);
  const [browsing, setBrowsing] = useState<{ name: string; source?: string } | null>(null);
  const [menu, setMenu] = useState<{ x: number; y: number; items: ContextMenuItem[] } | null>(null);
  // Packages ticked for the selection bar, across every section.
  const [selected, setSelected] = useState<Set<string>>(() => new Set());
  // Removing several packages asks first; the review that follows shows what each Agent loses.
  const [confirmRemove, setConfirmRemove] = useState<{ names: string[]; key: string } | null>(null);
  const select = (names: string[], on: boolean) =>
    setSelected((prev) => {
      const next = new Set(prev);
      names.forEach((name) => (on ? next.add(name) : next.delete(name)));
      return next;
    });
  const refresh = () => void invalidate(cache, 'pluginsChanged');
  const {
    busy,
    working,
    failure,
    review,
    updates,
    outcomes,
    checked,
    checking,
    checkProgress,
    checkFailures,
    batch,
    pendingUpdates,
    preview,
    begin,
    apply,
    selectTarget,
    selectMany,
    check,
    updateMany,
    importMany,
    stopBatch,
    closeBatch,
    closeReview,
    clearResult,
    todo,
    active,
    idle,
    changed,
    isPi,
  } = usePluginsFlow({
    t,
    inventory: data,
    refresh,
    onPreviewed: () => {
      setAdding(null);
      setImporting(false);
    },
  });
  const [updating, setUpdating] = useState(false);
  // A change with no Agent is to Skillshare itself: the plugin was only added to it, or removed from it.
  const agentLabel = (target: string) =>
    pluginTargets[target]?.label ?? (target || t('plugins.skillshareOnly'));
  if (!data && !error) return <PageSkeleton />;
  const actionText = (action: string) =>
    t(
      (
        {
          noop: 'plugins.noChanges',
          skip: 'plugins.outcome.skipped',
          install: 'resources.install',
          import: 'plugins.import',
          update: 'plugins.update',
          remove: 'plugins.remove',
          uninstall: 'plugins.remove',
          forget: 'plugins.remove',
          selection: 'common.save',
          record: 'plugins.record',
          blocked: 'plugins.blocked',
        } as Record<string, string>
      )[action] ?? 'plugins.pending',
    );
  const pluginTargets = targetMap(data?.targetDefinitions);
  const packages = Object.entries(data?.packages ?? {});
  // A plugin with no Agent yet has only what was recorded when it was added.
  // Plugins first: a machine without Pi stops the pasted list at the first Pi package, with every plugin already added.
  const shareable = packages
    .map(([name, pack]) => {
      const bound = Object.values(pack.bindings);
      const command = pluginShareCommand(name, bound.find((b) => b?.source) ?? pack);
      if (command) return { name, pi: false, command, quiet: `${command} --no-tui` };
      // Pi's own source for the package: recorded as the source when Skillshare added it, as the binding's id when it was imported.
      const npm =
        [pack.source, ...bound.flatMap((b) => [b?.source, b?.id])].find((s) =>
          s?.startsWith('npm:'),
        ) ?? '';
      return {
        name,
        pi: true,
        command: piShareCommand(name, npm, true),
        quiet: `${piShareCommand(name, npm, false)} --no-tui`,
      };
    })
    .filter((p) => p.command)
    .sort((a, b) => Number(a.pi) - Number(b.pi));
  const pending = todo.length;
  const updatable = pendingUpdates.length;
  const actionTone = (action: string) =>
    action === 'blocked' ? 'bad' : action === 'noop' ? '' : action === 'skip' ? 'warn' : 'inf';
  // Not every Agent can be updated from here, and a deselected one has nothing installed; the preview names the ones that are.
  const updatableTargets = (name: string) => {
    const bindings = data?.packages[name]?.bindings ?? {};
    return Object.keys(bindings).filter(
      (key) => bindings[key]?.sync !== false && pluginTargets[key]?.operations.includes('update'),
    );
  };
  const openMenu = (e: React.MouseEvent<HTMLButtonElement>, name: string) => {
    const r = e.currentTarget.getBoundingClientRect();
    const bindings = data?.packages[name]?.bindings ?? {};
    const source = Object.values(bindings).find((b) => b?.source)?.source;
    const updatable = updatableTargets(name);
    setMenu({
      x: r.left,
      y: r.bottom + 4,
      items: [
        {
          key: 'sync',
          label: t('plugins.syncOne'),
          icon: <ChevronRight size={14} />,
          onSelect: () => begin({ action: 'sync', name }, name),
        },
        ...(updatable.length > 0
          ? [
              {
                key: 'update',
                label: t('plugins.updateLatest'),
                icon: <RefreshCw size={14} />,
                onSelect: () => begin({ action: 'update', name, targets: updatable }, name),
              },
            ]
          : []),
        // Only a plugin added from a source has a local copy to read; an imported one lives in its Agent.
        ...(source
          ? [
              {
                key: 'files',
                label: t('plugins.viewFiles'),
                icon: <FolderOpen size={14} />,
                onSelect: () => setBrowsing({ name, source }),
              },
            ]
          : []),
        ...(shareable.some((p) => p.name === name)
          ? [
              {
                key: 'share',
                label: t('plugins.share'),
                icon: <Share2 size={14} />,
                onSelect: () => setSharing([name]),
              },
            ]
          : []),
        {
          key: 'remove',
          label: t('plugins.remove'),
          icon: <Trash2 size={14} />,
          danger: true,
          onSelect: () => begin({ action: 'remove', name }, name),
        },
      ],
    });
  };
  /** The packages one source added, at once: each action is one request for all of them. */
  const openGroupMenu = (e: React.MouseEvent<HTMLButtonElement>, members: string[]) => {
    const r = e.currentTarget.getBoundingClientRect();
    const key = groupKey(members);
    if (!data) return;
    const { off, on, removable, picks } = bulkActions(data, members, pendingUpdates);
    setMenu({
      x: r.left,
      y: r.bottom + 4,
      items: [
        ...(picks.length > 0
          ? [
              {
                key: 'update',
                label: t('plugins.groupUpdate'),
                icon: <RefreshCw size={14} />,
                // The batch shows its progress and failures in the update dialog.
                onSelect: () => {
                  setUpdating(true);
                  void updateMany(picks);
                },
              },
            ]
          : []),
        ...(off.length > 0
          ? [
              {
                key: 'enable',
                label: t('plugins.groupEnable'),
                icon: <Play size={14} />,
                onSelect: () => void selectMany(off, true, key),
              },
            ]
          : []),
        ...(on.length > 0
          ? [
              {
                key: 'disable',
                label: t('plugins.groupDisable'),
                icon: <Pause size={14} />,
                onSelect: () => void selectMany(on, false, key),
              },
            ]
          : []),
        ...(removable.length > 0
          ? [
              {
                key: 'remove',
                label: t('plugins.groupRemove'),
                icon: <Trash2 size={14} />,
                danger: true,
                onSelect: () => setConfirmRemove({ names: removable, key }),
              },
            ]
          : []),
      ],
    });
  };
  const names = Object.keys(data?.packages ?? {});
  const piPackages = names.filter(isPi);
  const isHerdr = herdrPackageTest(data);
  const herdrPackages = names.filter(isHerdr);
  const plugins = names.filter((name) => !isPi(name) && !isHerdr(name));
  // A removed package leaves the selection with its row, so one added again later comes back unticked.
  const ticked = names.filter((name) => selected.has(name));
  if (data && ticked.length < selected.size) setSelected(new Set(ticked));
  const bulk = data && bulkActions(data, ticked, pendingUpdates);
  // A section's "select all" shows while some of it is not ticked.
  const selectAll = (shown: string[], name: string) =>
    shown.some((n) => !selected.has(n)) && (
      <button type="button" className="more" onClick={() => select(shown, true)}>
        {t('resources.select.group', { name })}
      </button>
    );
  const list = (shown: string[], kind?: 'pi' | 'herdr') =>
    data && (
      <PluginList
        inventory={data}
        names={shown}
        kind={kind}
        updates={updates}
        checkFailures={checkFailures}
        busy={busy}
        working={working}
        onToggle={(name, target, on) => void selectTarget(name, target, on)}
        onMenu={openMenu}
        onGroupMenu={openGroupMenu}
        ticked={selected}
        onTick={select}
        onAdd={begin}
        onBlocked={(name, source) =>
          setAdding({
            name,
            source,
            bound: data.packages[name]?.bindings,
            recorded: data.packages[name],
          })
        }
      />
    );
  const importHosts = (data?.hosts ?? []).map((h) => {
    const locked = !pluginTargets[h.target]?.operations.includes('import');
    // What Skillshare installed or already imported into this Agent has nothing left to import.
    const managed = new Set(
      Object.values(data?.packages ?? {}).map((p) => p.bindings[h.target]?.id),
    );
    const importable = h.installed.filter((i) => !managed.has(i.id));
    // A filtered install keeps native fields the user should see in its own review, so it is imported one at a time.
    const pickable = locked
      ? []
      : importable.filter((i) => !i.filtered).map((i) => `${h.target}:${i.id}`);
    return { h, locked, importable, pickable };
  });
  // A tick on a row that stopped being pickable since (a refetch filtered it) is not sent.
  const pickedRequests = importHosts.flatMap(({ h, importable, pickable }) =>
    importable
      .filter((i) => picked.has(`${h.target}:${i.id}`) && pickable.includes(`${h.target}:${i.id}`))
      .map((i) => importRequest(h.target, i.id, importNames[`${h.target}:${i.id}`])),
  );
  const togglePicks = (keys: string[], on: boolean) =>
    setPicked((prev) => {
      const next = new Set(prev);
      for (const k of keys) {
        if (on) next.add(k);
        else next.delete(k);
      }
      return next;
    });
  const addActions = (
    <>
      <Button variant="secondary" disabled={busy} onClick={() => setImporting(true)}>
        <Download size={15} />
        {t('plugins.import')}
      </Button>
      <Button disabled={busy} onClick={() => setAdding({})}>
        <Plus size={15} />
        {t('plugins.add')}
      </Button>
    </>
  );

  // State, not inventory: what Sync would do, then which Agents can take part.
  const rail = data && (
    <>
      {(packages.length > 0 || outcomes.length > 0) && (
        // What config recorded as pending is known at once; the rest, and "synced", only after the Agents answered.
        <SyncBox
          tone={pending > 0 ? 'warn' : hostsReady ? 'ok' : 'busy'}
          state={
            pending > 0
              ? t(plural('plugins.pendingCount', pending), { count: pending })
              : t(hostsReady ? 'targets.state.synced' : 'plugins.checking')
          }
        >
          {pending > 0 && (
            <>
              <div className="flex flex-col gap-1">
                {byPlugin(todo, (x) => x.word).map((x) => (
                  <PluginRunLine
                    key={`${x.name}:${x.word}`}
                    name={x.name}
                    targets={x.targets}
                    label={agentLabel}
                    right={<span className="ss-tag inf">{x.word}</span>}
                  />
                ))}
              </div>
              <Button
                className="w-full justify-center"
                disabled={busy}
                onClick={() => begin({ action: 'sync' }, 'sync')}
              >
                {t('plugins.sync')}
                {/* The spinner takes the arrow's place. Button's loading puts one before the label, which
                    pushes the centred label aside for as long as the preview takes. */}
                {working === 'sync' ? (
                  <span className="grid size-[15px] place-items-center" aria-hidden="true">
                    <span className="spin animate-spin" />
                  </span>
                ) : (
                  <ChevronRight size={15} />
                )}
              </Button>
              <p className="text-xs leading-normal text-ink-2">{t('plugins.syncHint')}</p>
            </>
          )}
          {/* Outcomes of the action just run, not plugin state, so they live with Sync instead of reading as more inventory. */}
          {outcomes.length > 0 && (
            <div aria-live="polite" className="flex flex-col gap-1.5">
              <div className="flex items-center text-xs font-semibold text-ink-3">
                {t('plugins.lastRun')}
                <IconButton
                  className="ml-auto"
                  size="sm"
                  icon={<X size={14} />}
                  label={t('common.close')}
                  onClick={clearResult}
                />
              </div>
              {changed.map((r) => (
                <PluginRunLine
                  key={`${r.name}:${r.status}:${r.note}`}
                  name={r.name}
                  targets={r.targets}
                  label={agentLabel}
                  note={r.note}
                  className={r.status === 'failed' ? 'rounded-lg bg-bad-bg px-3 py-2' : ''}
                  right={
                    <span
                      className={`ss-st ${r.status === 'failed' ? 'bad' : r.status === 'skipped' ? 'warn' : ''}`}
                    >
                      {outcomeStatus(t, r.status)}
                    </span>
                  }
                />
              ))}
              <UnchangedRuns
                runs={outcomes.filter((r) => r.status === 'unchanged' && !r.guidance)}
                isPi={isPi}
              />
            </div>
          )}
          {pending === 0 && (
            <>
              {outcomes.length === 0 && (
                <p className="text-[13px] leading-normal text-ink-2">
                  {t('plugins.selectionHelp')}
                </p>
              )}
              <button
                type="button"
                className="ss-more flex items-center gap-1.5 self-start disabled:opacity-50"
                disabled={busy}
                onClick={() => begin({ action: 'sync' }, 'sync')}
              >
                {working === 'sync' && <Spinner size="sm" />}
                {t('plugins.syncAgain')}
              </button>
            </>
          )}
        </SyncBox>
      )}
      <PluginAgents
        inventory={data}
        ready={hostsReady}
        refreshing={full.isFetching}
        disabled={busy}
        onRefresh={refresh}
      />
    </>
  );

  return (
    <div className="ss-wrap animate-page-in">
      <PageHeader
        title={t('plugins.title')}
        subtitle={t('plugins.subtitle')}
        actions={
          <>
            {shareable.length > 0 && (
              <Button
                variant="ghost"
                disabled={busy}
                onClick={() => setSharing(shareable.map((p) => p.name))}
              >
                <Share2 size={15} />
                {t('plugins.share')}
              </Button>
            )}
            {packages.length > 0 &&
              (checking ? (
                <span className="flex items-center gap-1.5 px-3 text-[13px] text-ink-3">
                  <Spinner size="sm" />
                  {t('plugins.checkingUpdates')}
                  {checkProgress.total > 0 && (
                    <span className="tabular-nums">
                      {checkProgress.done}/{checkProgress.total}
                    </span>
                  )}
                </span>
              ) : (
                <Button
                  variant="ghost"
                  disabled={busy}
                  onClick={() =>
                    updatable > 0
                      ? setUpdating(true)
                      : void check().then((ok) => ok && setUpdating(true))
                  }
                >
                  <RefreshCw size={15} />
                  {updatable > 0
                    ? t(plural('plugins.updateCount', updatable), { count: updatable })
                    : t('plugins.check')}
                </Button>
              ))}
            {addActions}
          </>
        }
      />

      {(failure || error) && (
        <div role="alert" className="ss-note bad">
          <span className="flex-1">{failure || (error as Error).message}</span>
        </div>
      )}

      {data && (
        <RailLayout rail={rail}>
          {/* Room below the last row for the selection bar, which floats over the page's foot. */}
          <div className={`flex flex-col gap-7 ${ticked.length > 0 ? 'pb-12' : ''}`}>
            {(packages.length === 0 || plugins.length > 0) && (
              <section aria-labelledby="plugins-managed-title" className="flex flex-col gap-3">
                <div className="ss-sec">
                  <h2 id="plugins-managed-title">{t('plugins.managedTitle')}</h2>
                  <span className="ss-cnt">{plugins.length}</span>
                  {plugins.length > 0 && selectAll(plugins, t('plugins.managedTitle'))}
                </div>
                {packages.length === 0 ? (
                  <EmptyState
                    icon={Package}
                    title={t('plugins.empty')}
                    description={t('plugins.emptyHelp')}
                    action={addActions}
                  />
                ) : (
                  list(plugins)
                )}
              </section>
            )}
            {herdrPackages.length > 0 && (
              <section aria-labelledby="plugins-herdr-title" className="flex flex-col gap-3">
                <div className="ss-sec">
                  <h2 id="plugins-herdr-title">{t('plugins.herdrTitle')}</h2>
                  <span className="ss-cnt">{herdrPackages.length}</span>
                  {selectAll(herdrPackages, t('plugins.herdrTitle'))}
                </div>
                {list(herdrPackages, 'herdr')}
              </section>
            )}
            {piPackages.length > 0 && (
              <section aria-labelledby="plugins-pi-title" className="flex flex-col gap-3">
                <div className="ss-sec">
                  <h2 id="plugins-pi-title">{t('plugins.piTitle')}</h2>
                  <span className="ss-cnt">{piPackages.length}</span>
                  {selectAll(piPackages, t('plugins.piTitle'))}
                </div>
                {list(piPackages, 'pi')}
              </section>
            )}
          </div>
        </RailLayout>
      )}

      {confirmRemove && (
        <ConfirmDialog
          open
          variant="danger"
          title={t(plural('plugins.bulkRemoveTitle', confirmRemove.names.length), {
            count: confirmRemove.names.length,
          })}
          message={
            <div className="flex flex-col gap-3">
              <p>{t('plugins.bulkRemoveBody')}</p>
              <ul className="flex flex-col gap-1 font-mono text-[13px] text-ink">
                {confirmRemove.names.map((name) => (
                  <li key={name}>{name}</li>
                ))}
              </ul>
            </div>
          }
          confirmText={t('plugins.remove')}
          onCancel={() => setConfirmRemove(null)}
          onConfirm={() => {
            const { names: picked, key } = confirmRemove;
            setConfirmRemove(null);
            begin(
              picked.length === 1
                ? { action: 'remove', name: picked[0] }
                : { action: 'remove', names: picked },
              key,
            );
          }}
        />
      )}

      {/* Each action sends one request for every ticked package it applies to, as a group's menu does. */}
      {bulk && ticked.length > 0 && (
        <div
          ref={bulkGhost}
          className="ss-bulk"
          role="toolbar"
          aria-label={t('resources.select.count', { count: ticked.length })}
        >
          <b>{t('resources.select.count', { count: ticked.length })}</b>
          {working === BULK && <Spinner size="sm" />}
          <span className="dv" />
          <Button
            variant="secondary"
            size="sm"
            disabled={busy || bulk.off.length === 0}
            // A failure keeps the selection, so the same packages can be tried again.
            onClick={() =>
              void selectMany(bulk.off, true, BULK).then((ok) => ok && setSelected(new Set()))
            }
          >
            <Play size={15} />
            {t('plugins.bulkEnable')}
          </Button>
          <Button
            variant="secondary"
            size="sm"
            disabled={busy || bulk.on.length === 0}
            // A failure keeps the selection, so the same packages can be tried again.
            onClick={() =>
              void selectMany(bulk.on, false, BULK).then((ok) => ok && setSelected(new Set()))
            }
          >
            <Pause size={15} />
            {t('plugins.bulkDisable')}
          </Button>
          <Button
            variant="secondary"
            size="sm"
            disabled={busy || bulk.picks.length === 0}
            onClick={() => {
              setUpdating(true);
              void updateMany(bulk.picks);
              setSelected(new Set());
            }}
          >
            <RefreshCw size={15} />
            {t('plugins.updateLatest')}
          </Button>
          <Button
            variant="secondary"
            size="sm"
            disabled={busy || bulk.removable.length === 0}
            onClick={() => setConfirmRemove({ names: bulk.removable, key: BULK })}
          >
            <Trash2 size={15} />
            {t('plugins.remove')}
          </Button>
          <span className="dv" />
          <button
            type="button"
            className="ss-ib"
            aria-label={t('resources.select.clear')}
            onClick={() => setSelected(new Set())}
          >
            <X size={16} />
          </button>
        </div>
      )}

      {sharing && (
        <PluginShareDialog plugins={shareable} initial={sharing} onClose={() => setSharing(null)} />
      )}
      {browsing && (
        <PluginFilesDialog
          name={browsing.name}
          source={browsing.source}
          onClose={() => setBrowsing(null)}
        />
      )}
      {data && (
        <PluginUpdateDialog
          open={updating}
          plan={checked}
          pending={pendingUpdates}
          inventory={data}
          batch={batch}
          busy={busy}
          isPi={isPi}
          onUpdate={(picks) => void updateMany(picks)}
          onStop={stopBatch}
          onClose={() => {
            setUpdating(false);
            closeBatch();
          }}
        />
      )}
      {adding && (
        <PluginAddDialog
          initialName={adding.name}
          initialSource={adding.source}
          initialTargets={adding.targets}
          definitions={data?.targetDefinitions}
          bound={adding.bound}
          recorded={adding.recorded}
          onClose={() => setAdding(null)}
          onPreview={preview}
        />
      )}

      <DialogShell
        open={importing}
        onClose={() => setImporting(false)}
        preventClose={busy}
        ariaLabel={t('plugins.import')}
        maxWidth="2xl"
        padding="none"
      >
        <DialogHeader
          title={t('plugins.import')}
          subtitle={t('plugins.importHelp')}
          onClose={() => setImporting(false)}
          closeDisabled={busy}
        />
        <div className="db overflow-y-auto">
          <div className="ss-list">
            {!hostsReady && (
              <div className="ss-r gap-2 text-[13px] text-ink-2">
                <Spinner size="sm" />
                {t('plugins.hostsAsking')}
              </div>
            )}
            {!hostsReady && hostsSlow && (
              <div className="ss-r !min-h-0 text-xs text-ink-3">{t('plugins.hostsSlow')}</div>
            )}
            {importHosts.map(({ h, locked, importable, pickable }) => {
              const all = pickable.length > 0 && pickable.every((k) => picked.has(k));
              return (
                <div key={h.target}>
                  <div className="ss-gh">
                    {pickable.length > 0 && (
                      <Checkbox
                        label={pluginTargets[h.target]?.label ?? h.target}
                        hideLabel
                        checked={all}
                        indeterminate={!all && pickable.some((k) => picked.has(k))}
                        disabled={busy}
                        onChange={(on) => togglePicks(pickable, on)}
                      />
                    )}
                    <AgentTile target={h.target} />
                    <span className="font-semibold">
                      {pluginTargets[h.target]?.label ?? h.target}
                    </span>
                    <span className="ss-cnt">{importable.length}</span>
                  </div>
                  {h.installed.length === 0 && (
                    <div className="ss-r !min-h-11">
                      <span className="text-[13px] text-ink-3">
                        {keyedText(t, h.errorKey, h.error, h.errorArgs) || t('plugins.absent')}
                      </span>
                    </div>
                  )}
                  {h.installed.length > 0 && importable.length === 0 && (
                    <div className="ss-r !min-h-11">
                      <span className="text-[13px] text-ink-3">{t('plugins.allManaged')}</span>
                    </div>
                  )}
                  {importable.map((i) => (
                    <div key={i.id} className="ss-r !min-h-11">
                      {pickable.length > 0 && (
                        <Checkbox
                          label={`${i.id} (${pluginTargets[h.target]?.label ?? h.target})`}
                          hideLabel
                          checked={picked.has(`${h.target}:${i.id}`)}
                          disabled={busy || !pickable.includes(`${h.target}:${i.id}`)}
                          onChange={(on) => togglePicks([`${h.target}:${i.id}`], on)}
                        />
                      )}
                      <span
                        className="min-w-0 flex-1 truncate font-mono text-[13px] font-semibold"
                        title={i.id}
                      >
                        {i.id}
                      </span>
                      {i.enabledKnown !== false && !i.enabled && (
                        <span className="ss-tag">{t('plugins.nativeDisabled')}</span>
                      )}
                      <span className="font-mono text-xs text-ink-3">{i.version}</span>
                      {importPackageNames(data, h.target).length > 0 && (
                        <Select
                          ariaLabel={t('resources.col.name')}
                          size="sm"
                          disabled={busy}
                          value={
                            importRequest(h.target, i.id, importNames[`${h.target}:${i.id}`])
                              .name ?? ''
                          }
                          options={[
                            { value: '', label: i.id },
                            ...importPackageNames(data, h.target).map((name) => ({
                              value: name,
                              label: name,
                            })),
                          ]}
                          onChange={(name) =>
                            setImportNames((old) => ({ ...old, [`${h.target}:${i.id}`]: name }))
                          }
                        />
                      )}
                      <Button
                        size="sm"
                        variant="secondary"
                        loading={working === `${h.target}:${i.id}`}
                        disabled={busy || (i.filtered && !i.importable) || locked}
                        onClick={() =>
                          begin(
                            importRequest(h.target, i.id, importNames[`${h.target}:${i.id}`]),
                            `${h.target}:${i.id}`,
                          )
                        }
                      >
                        {t('plugins.importOne')}
                      </Button>
                    </div>
                  ))}
                </div>
              );
            })}
          </div>
        </div>
        <div className="df">
          {/* The page's alert sits behind this dialog. */}
          {failure && (
            <span role="alert" className="mr-auto min-w-0 text-[13px] text-bad">
              {failure}
            </span>
          )}
          <Button variant="ghost" disabled={busy} onClick={() => setImporting(false)}>
            {t('common.close')}
          </Button>
          <Button
            loading={working === 'import'}
            disabled={busy || pickedRequests.length === 0}
            onClick={() => void importMany(pickedRequests)}
          >
            {t('plugins.importSelected', { count: pickedRequests.length })}
          </Button>
        </div>
      </DialogShell>

      <DialogShell
        open={!!review}
        onClose={closeReview}
        preventClose={busy}
        ariaLabel={t('plugins.preview')}
        maxWidth="2xl"
        padding="none"
      >
        <DialogHeader
          title={t('plugins.preview')}
          subtitle={t('plugins.previewHelp')}
          onClose={closeReview}
          closeDisabled={busy}
        />
        <div className="db overflow-y-auto">
          {review?.plan.changes.length === 0 ? (
            <p className="text-[13px] text-ink-2">{t('plugins.noChanges')}</p>
          ) : (
            <>
              {active.length > 0 && (
                <div className="ss-list">
                  {active.map((c) => (
                    <div key={`${c.name}:${c.target}`} className="ss-r">
                      <AgentTile
                        target={c.target}
                        fallback={
                          c.logo ? (
                            <img
                              src={c.logo}
                              alt=""
                              className="size-full rounded-[inherit] object-cover"
                            />
                          ) : (
                            <Package size={15} />
                          )
                        }
                      />
                      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                        <span className="flex items-center gap-2">
                          <span className="font-mono font-semibold">{c.name}</span>
                          <span className="text-[13px] text-ink-2">{agentLabel(c.target)}</span>
                          <VersionChange
                            from={installedVersion(data, c.name, c.target)}
                            to={c.action.startsWith('update') ? c.binding?.version : undefined}
                            fromCommit={data?.packages[c.name]?.bindings[c.target]?.commit}
                            toCommit={c.action.startsWith('update') ? c.binding?.commit : undefined}
                          />
                        </span>
                        {!!c.preservedKeys?.length && (
                          <span className="text-xs text-ink-3">
                            <span>{t('plugins.preservedKeys')}</span> {c.preservedKeys.join(' · ')}
                          </span>
                        )}
                        {c.action === 'record' ? (
                          <span className="text-xs text-ink-3">{t('plugins.recordHelp')}</span>
                        ) : (
                          (c.message || c.components?.length) && (
                            <span className="text-xs text-ink-3">
                              {c.message ? keyedMessage(t, c) : c.components!.join(' · ')}
                            </span>
                          )
                        )}
                      </span>
                      <span className={`ss-tag ${c.failed ? 'bad' : actionTone(c.action)}`}>
                        {c.failed ? t('plugins.outcome.failed') : actionText(c.action)}
                      </span>
                    </div>
                  ))}
                </div>
              )}
              {review?.plan.herdrPanel ? (
                <p className="ss-note info">
                  {t(
                    review.plan.herdrPanel.mod
                      ? 'plugins.herdrDockPreview'
                      : 'plugins.herdrPanelPreview',
                    review.plan.herdrPanel,
                  )}
                </p>
              ) : (
                active.some(
                  (c) =>
                    c.target === 'herdr-app' && (c.action === 'install' || c.action === 'update'),
                ) && <p className="text-xs text-ink-3">{t('plugins.herdrPreviewHelp')}</p>
              )}
              {active.some(
                (c) =>
                  c.target !== 'herdr-app' && (c.action === 'install' || c.action === 'update'),
              ) && <p className="text-xs text-ink-3">{t('plugins.nativeHelp')}</p>}
              <UnchangedRuns runs={idle} isPi={isPi} versions />
            </>
          )}
        </div>
        <div className="df">
          <Button variant="ghost" disabled={busy} onClick={closeReview}>
            {t('common.cancel')}
          </Button>
          <Button
            loading={busy}
            disabled={review?.plan.blocked || !review?.plan.changes.length}
            onClick={() => void apply()}
          >
            {t('plugins.apply')}
          </Button>
        </div>
      </DialogShell>

      <SkillContextMenu
        open={!!menu}
        anchorPoint={menu ?? undefined}
        items={menu?.items ?? []}
        onClose={() => setMenu(null)}
      />
    </div>
  );
}
