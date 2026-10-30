import { useEffect, useRef, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ChevronRight, Download, FolderOpen, Package, Plus, RefreshCw, Share2, Trash2, X } from 'lucide-react';
import { byPlugin, pluginShareCommand, pluginsApi, targetMap, type PluginTarget, type PluginInventory } from '../api/plugins';
import AgentIcon from '../components/AgentIcon';
import Button from '../components/Button';
import { Checkbox } from '../components/Checkbox';
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
import { importPackageNames, importRequest, installedVersion, usePluginsFlow } from '../components/plugins/pluginsFlow';
import PluginList, { VersionChange } from '../components/plugins/PluginList';
import { PluginRunLine, UnchangedRuns } from '../components/plugins/PluginRuns';
import { useT, plural } from '../i18n';
import { useSlow } from '../hooks/useSlow';
import { queryKeys } from '../lib/queryKeys';
import { invalidate } from '../lib/queryEvents';

export default function PluginsPage() {
  const t = useT();
  const cache = useQueryClient();
  // Config answers at once; asking every Agent's CLI takes seconds. The plugin list is drawn
  // from the quick answer until full inventory supplies verified packages and Agents.
  const quick = useQuery({ queryKey: queryKeys.pluginPackages, queryFn: () => pluginsApi.list(false) });
  const full = useQuery({ queryKey: queryKeys.plugins, queryFn: () => pluginsApi.list() });
  const hostsReady = !!full.data;
  const hostsSlow = useSlow(!hostsReady, 10000);
  const base = full.data ?? quick.data;
  const data = base && { ...base, hosts: full.data?.hosts ?? [] };
  const error = quick.error ?? full.error;
  const [adding, setAdding] = useState<{ source?: string; name?: string; targets?: PluginTarget[]; bound?: PluginInventory['packages'][string]['bindings']; recorded?: PluginInventory['packages'][string] } | null>(null);
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
  const refresh = () => void invalidate(cache, 'pluginsChanged');
  const { busy, working, failure, review, updates, outcomes, checked, checking, batch, pendingUpdates, preview, begin, apply, selectTarget, check, updateMany, importMany, stopBatch, closeBatch, closeReview, clearResult, todo, active, idle, changed, isPi } =
    usePluginsFlow({ t, inventory: data, refresh, onPreviewed: () => { setAdding(null); setImporting(false); } });
  const [updating, setUpdating] = useState(false);
  // Once per visit, as soon as there is a plugin to ask about: the rows say what has a new version without a click.
  const hasPackages = Object.keys(data?.packages ?? {}).length > 0;
  const autoChecked = useRef(false);
  useEffect(() => {
    if (!hasPackages || autoChecked.current) return;
    autoChecked.current = true;
    void check(true);
  // eslint-disable-next-line react-hooks/exhaustive-deps -- `check` is remade every render; the ref keeps this to one run
  }, [hasPackages]);
  // A change with no Agent is to Skillshare itself: the plugin was only added to it, or removed from it.
  const agentLabel = (target: string) => pluginTargets[target]?.label ?? (target || t('plugins.skillshareOnly'));
  if (!data && !error) return <PageSkeleton />;
  const actionText = (action: string) => t(({
    noop: 'plugins.noChanges', skip: 'plugins.outcome.skipped', install: 'resources.install', import: 'plugins.import', update: 'plugins.update',
    remove: 'plugins.remove', uninstall: 'plugins.remove', forget: 'plugins.remove', selection: 'common.save', record: 'plugins.record',
    blocked: 'plugins.blocked',
  } as Record<string, string>)[action] ?? 'plugins.pending');
  const pluginTargets = targetMap(data?.targetDefinitions);
  const packages = Object.entries(data?.packages ?? {});
  // A plugin with no Agent yet has only what was recorded when it was added.
  const shareable = packages.map(([name, pack]) => ({ name, command: pluginShareCommand(name, Object.values(pack.bindings).find((b) => b?.source) ?? pack) })).filter((p) => p.command);
  const pending = todo.length;
  const updatable = pendingUpdates.length;
  const actionTone = (action: string) => (action === 'blocked' ? 'bad' : action === 'noop' ? '' : action === 'skip' ? 'warn' : 'inf');
  const openMenu = (e: React.MouseEvent<HTMLButtonElement>, name: string) => {
    const r = e.currentTarget.getBoundingClientRect();
    const bindings = data?.packages[name]?.bindings ?? {};
    const source = Object.values(bindings).find((b) => b?.source)?.source;
    // Not every Agent can be updated from here, and a deselected one has nothing installed; the preview names the ones that are.
    const updatable = Object.keys(bindings).filter((key) => bindings[key]?.sync !== false && pluginTargets[key]?.operations.includes('update'));
    setMenu({
      x: r.left,
      y: r.bottom + 4,
      items: [
        { key: 'sync', label: t('plugins.syncOne'), icon: <ChevronRight size={14} />, onSelect: () => begin({ action: 'sync', name }, name) },
        ...(updatable.length > 0 ? [
          { key: 'update', label: t('plugins.updateLatest'), icon: <RefreshCw size={14} />, onSelect: () => begin({ action: 'update', name, targets: updatable }, name) },
        ] : []),
        // Only a plugin added from a source has a local copy to read; an imported one lives in its Agent.
        ...(source ? [{ key: 'files', label: t('plugins.viewFiles'), icon: <FolderOpen size={14} />, onSelect: () => setBrowsing({ name, source }) }] : []),
        ...(shareable.some((p) => p.name === name) ? [{ key: 'share', label: t('plugins.share'), icon: <Share2 size={14} />, onSelect: () => setSharing([name]) }] : []),
        { key: 'remove', label: t('plugins.remove'), icon: <Trash2 size={14} />, danger: true, onSelect: () => begin({ action: 'remove', name }, name) },
      ],
    });
  };
  const names = Object.keys(data?.packages ?? {});
  const piPackages = names.filter(isPi);
  const plugins = names.filter((name) => !isPi(name));
  const list = (shown: string[], pi?: boolean) => data && (
    <PluginList inventory={data} names={shown} pi={pi} updates={updates} busy={busy} working={working} onToggle={(name, target, on) => void selectTarget(name, target, on)} onMenu={openMenu} onAdd={begin} onBlocked={(name, source) => setAdding({ name, source, bound: data.packages[name]?.bindings, recorded: data.packages[name] })} />
  );
  const importHosts = (data?.hosts ?? []).map((h) => {
    const locked = !pluginTargets[h.target]?.operations.includes('import');
    // What Skillshare installed or already imported into this Agent has nothing left to import.
    const managed = new Set(Object.values(data?.packages ?? {}).map((p) => p.bindings[h.target]?.id));
    const importable = h.installed.filter((i) => !managed.has(i.id));
    // A filtered install keeps native fields the user should see in its own review, so it is imported one at a time.
    const pickable = locked ? [] : importable.filter((i) => !i.filtered).map((i) => `${h.target}:${i.id}`);
    return { h, locked, importable, pickable };
  });
  // A tick on a row that stopped being pickable since (a refetch filtered it) is not sent.
  const pickedRequests = importHosts.flatMap(({ h, importable, pickable }) => importable.filter((i) => picked.has(`${h.target}:${i.id}`) && pickable.includes(`${h.target}:${i.id}`)).map((i) => importRequest(h.target, i.id, importNames[`${h.target}:${i.id}`])));
  const togglePicks = (keys: string[], on: boolean) => setPicked((prev) => { const next = new Set(prev); for (const k of keys) { if (on) next.add(k); else next.delete(k); } return next; });
  const addActions = <>
    <Button variant="secondary" disabled={busy} onClick={() => setImporting(true)}><Download size={15} />{t('plugins.import')}</Button>
    <Button disabled={busy} onClick={() => setAdding({})}><Plus size={15} />{t('plugins.add')}</Button>
  </>;

  // State, not inventory: what Sync would do, then which Agents can take part.
  const rail = data && (
    <>
      {(packages.length > 0 || outcomes.length > 0) && (
        // What config recorded as pending is known at once; the rest, and "synced", only after the Agents answered.
        <SyncBox tone={pending > 0 ? 'warn' : hostsReady ? 'ok' : 'busy'} state={pending > 0 ? t(plural('plugins.pendingCount', pending), { count: pending }) : t(hostsReady ? 'targets.state.synced' : 'plugins.checking')}>
          {pending > 0 && (
            <>
              <div className="flex flex-col gap-1">
                {byPlugin(todo, (x) => x.word).map((x) => <PluginRunLine key={`${x.name}:${x.word}`} name={x.name} targets={x.targets} label={agentLabel} right={<span className="ss-tag inf">{x.word}</span>} />)}
              </div>
              <Button className="w-full justify-center" loading={working === 'sync'} disabled={busy} onClick={() => begin({ action: 'sync' }, 'sync')}>{t('plugins.sync')}<ChevronRight size={15} /></Button>
              <p className="text-xs leading-normal text-ink-2">{t('plugins.syncHint')}</p>
            </>
          )}
          {/* Outcomes of the action just run, not plugin state, so they live with Sync instead of reading as more inventory. */}
          {outcomes.length > 0 && (
            <div aria-live="polite" className="flex flex-col gap-1.5">
              <div className="flex items-center text-xs font-semibold text-ink-3">{t('plugins.lastRun')}<IconButton className="ml-auto" size="sm" icon={<X size={14} />} label={t('common.close')} onClick={clearResult} /></div>
              {changed.map((r) => (
                <PluginRunLine key={`${r.name}:${r.status}:${r.note}`} name={r.name} targets={r.targets} label={agentLabel} note={r.note}
                  className={r.status === 'failed' ? 'rounded-lg bg-bad-bg px-3 py-2' : ''}
                  right={<span className={`ss-st ${r.status === 'failed' ? 'bad' : r.status === 'skipped' ? 'warn' : ''}`}>{outcomeStatus(t, r.status)}</span>} />
              ))}
              <UnchangedRuns runs={outcomes.filter((r) => r.status === 'unchanged' && !r.guidance)} isPi={isPi} />
            </div>
          )}
          {pending === 0 && (
            <>
              {outcomes.length === 0 && <p className="text-[13px] leading-normal text-ink-2">{t('plugins.selectionHelp')}</p>}
              <button type="button" className="ss-more flex items-center gap-1.5 self-start disabled:opacity-50" disabled={busy} onClick={() => begin({ action: 'sync' }, 'sync')}>{working === 'sync' && <Spinner size="sm" />}{t('plugins.syncAgain')}</button>
            </>
          )}
        </SyncBox>
      )}
      <PluginAgents inventory={data} ready={hostsReady} refreshing={full.isFetching} disabled={busy} onRefresh={refresh} />
    </>
  );

  return (
    <div className="ss-wrap animate-fade-in">
      <PageHeader title={t('plugins.title')} subtitle={t('plugins.subtitle')} actions={<>
        {shareable.length > 0 && <Button variant="ghost" disabled={busy} onClick={() => setSharing(shareable.map((p) => p.name))}><Share2 size={15} />{t('plugins.share')}</Button>}
        {packages.length > 0 && (checking
          ? <span className="flex items-center gap-1.5 px-3 text-[13px] text-ink-3"><Spinner size="sm" />{t('plugins.checkingUpdates')}</span>
          : <Button variant="ghost" disabled={busy} onClick={() => (updatable > 0 ? setUpdating(true) : void check().then((ok) => ok && setUpdating(true)))}><RefreshCw size={15} />{updatable > 0 ? t(plural('plugins.updateCount', updatable), { count: updatable }) : t('plugins.check')}</Button>)}
        {addActions}
      </>} />

      {(failure || error) && <div role="alert" className="ss-note bad"><span className="flex-1">{failure || (error as Error).message}</span></div>}

      {data && <RailLayout rail={rail}>
        <div className="flex flex-col gap-7">
        {(packages.length === 0 || plugins.length > 0) && (
          <section aria-labelledby="plugins-managed-title" className="flex flex-col gap-3">
            <div className="ss-sec"><h2 id="plugins-managed-title">{t('plugins.managedTitle')}</h2><span className="ss-cnt">{plugins.length}</span></div>
            {packages.length === 0 ? <EmptyState icon={Package} title={t('plugins.empty')} description={t('plugins.emptyHelp')} action={addActions} /> : list(plugins)}
          </section>
        )}
        {piPackages.length > 0 && (
          <section aria-labelledby="plugins-pi-title" className="flex flex-col gap-3">
            <div className="ss-sec"><h2 id="plugins-pi-title">{t('plugins.piTitle')}</h2><span className="ss-cnt">{piPackages.length}</span></div>
            {list(piPackages, true)}
          </section>
        )}
        </div>
      </RailLayout>}

      {sharing && <PluginShareDialog plugins={shareable} initial={sharing} onClose={() => setSharing(null)} />}
      {browsing && <PluginFilesDialog name={browsing.name} source={browsing.source} onClose={() => setBrowsing(null)} />}
      {data && <PluginUpdateDialog open={updating} plan={checked} pending={pendingUpdates} inventory={data} batch={batch} busy={busy} isPi={isPi} onUpdate={(picks) => void updateMany(picks)} onStop={stopBatch} onClose={() => { setUpdating(false); closeBatch(); }} />}
      {adding && <PluginAddDialog initialName={adding.name} initialSource={adding.source} initialTargets={adding.targets} definitions={data?.targetDefinitions} bound={adding.bound} recorded={adding.recorded} onClose={() => setAdding(null)} onPreview={preview} />}

      <DialogShell open={importing} onClose={() => setImporting(false)} preventClose={busy} ariaLabel={t('plugins.import')} maxWidth="2xl" padding="none">
        <div className="dh">
          <div className="flex flex-col gap-1"><h2 className="ss-h2">{t('plugins.import')}</h2><p className="text-[13px] text-ink-2">{t('plugins.importHelp')}</p></div>
          <IconButton icon={<X size={16} />} label={t('common.close')} disabled={busy} onClick={() => setImporting(false)} />
        </div>
        <div className="db overflow-y-auto">
          <div className="ss-list">
            {!hostsReady && <div className="ss-r gap-2 text-[13px] text-ink-2"><Spinner size="sm" />{t('plugins.hostsAsking')}</div>}
            {!hostsReady && hostsSlow && <div className="ss-r !min-h-0 text-xs text-ink-3">{t('plugins.hostsSlow')}</div>}
            {importHosts.map(({ h, locked, importable, pickable }) => {
              const all = pickable.length > 0 && pickable.every((k) => picked.has(k));
              return (
                <div key={h.target}>
                  <div className="ss-gh">{pickable.length > 0 && <Checkbox label={pluginTargets[h.target]?.label ?? h.target} hideLabel checked={all} indeterminate={!all && pickable.some((k) => picked.has(k))} disabled={busy} onChange={(on) => togglePicks(pickable, on)} />}<span className="ss-at"><AgentIcon target={h.target} size={17} /></span><span className="font-semibold">{(pluginTargets[h.target]?.label ?? h.target)}</span><span className="ss-cnt">{importable.length}</span></div>
                  {h.installed.length === 0 && <div className="ss-r !min-h-11"><span className="text-[13px] text-ink-3">{keyedText(t, h.errorKey, h.error, h.errorArgs) || t('plugins.absent')}</span></div>}
                  {h.installed.length > 0 && importable.length === 0 && <div className="ss-r !min-h-11"><span className="text-[13px] text-ink-3">{t('plugins.allManaged')}</span></div>}
                  {importable.map((i) => (
                    <div key={i.id} className="ss-r !min-h-11">
                      {pickable.length > 0 && <Checkbox label={`${i.id} (${pluginTargets[h.target]?.label ?? h.target})`} hideLabel checked={picked.has(`${h.target}:${i.id}`)} disabled={busy || !pickable.includes(`${h.target}:${i.id}`)} onChange={(on) => togglePicks([`${h.target}:${i.id}`], on)} />}
                      <span className="min-w-0 flex-1 truncate font-mono text-[13px] font-semibold" title={i.id}>{i.id}</span>
                      {i.enabledKnown !== false && !i.enabled && <span className="ss-tag">{t('plugins.nativeDisabled')}</span>}
                      <span className="font-mono text-xs text-ink-3">{i.version}</span>
                      {importPackageNames(data, h.target).length > 0 && <Select
                        ariaLabel={t('resources.col.name')} size="sm" disabled={busy}
                        value={importRequest(h.target, i.id, importNames[`${h.target}:${i.id}`]).name ?? ''}
                        options={[{ value: '', label: i.id }, ...importPackageNames(data, h.target).map((name) => ({ value: name, label: name }))]}
                        onChange={(name) => setImportNames((old) => ({ ...old, [`${h.target}:${i.id}`]: name }))}
                      />}
                      <Button size="sm" variant="secondary" loading={working === `${h.target}:${i.id}`} disabled={busy || (i.filtered && !i.importable) || locked} onClick={() => begin(importRequest(h.target, i.id, importNames[`${h.target}:${i.id}`]), `${h.target}:${i.id}`)}>{t('plugins.importOne')}</Button>
                    </div>
                  ))}
                </div>
              );
            })}
          </div>
        </div>
        <div className="df">
          {/* The page's alert sits behind this dialog. */}
          {failure && <span role="alert" className="mr-auto min-w-0 text-[13px] text-bad">{failure}</span>}
          <Button variant="ghost" disabled={busy} onClick={() => setImporting(false)}>{t('common.close')}</Button>
          <Button loading={working === 'import'} disabled={busy || pickedRequests.length === 0} onClick={() => void importMany(pickedRequests)}>{t('plugins.importSelected', { count: pickedRequests.length })}</Button>
        </div>
      </DialogShell>

      <DialogShell open={!!review} onClose={closeReview} preventClose={busy} ariaLabel={t('plugins.preview')} maxWidth="2xl" padding="none">
        <div className="dh">
          <div className="flex flex-col gap-1"><h2 className="ss-h2">{t('plugins.preview')}</h2><p className="text-[13px] text-ink-2">{t('plugins.previewHelp')}</p></div>
          <IconButton icon={<X size={16} />} label={t('common.close')} disabled={busy} onClick={closeReview} />
        </div>
        <div className="db overflow-y-auto">
          {review?.plan.changes.length === 0 ? <p className="text-[13px] text-ink-2">{t('plugins.noChanges')}</p> : (
            <>
            {active.length > 0 && <div className="ss-list">
              {active.map((c) => (
                <div key={`${c.name}:${c.target}`} className="ss-r">
                  <span className="ss-at">{c.target ? <AgentIcon target={c.target} size={17} /> : c.logo ? <img src={c.logo} alt="" className="size-full rounded-[inherit] object-cover" /> : <Package size={15} />}</span>
                  <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span className="flex items-center gap-2"><span className="font-mono font-semibold">{c.name}</span><span className="text-[13px] text-ink-2">{agentLabel(c.target)}</span><VersionChange from={installedVersion(data, c.name, c.target)} to={c.action.startsWith('update') ? c.binding?.version : undefined} fromCommit={data?.packages[c.name]?.bindings[c.target]?.commit} toCommit={c.action.startsWith('update') ? c.binding?.commit : undefined} /></span>
                    {!!c.preservedKeys?.length && <span className="text-xs text-ink-3"><span>{t('plugins.preservedKeys')}</span>{' '}{c.preservedKeys.join(' · ')}</span>}
                    {c.action === 'record' ? <span className="text-xs text-ink-3">{t('plugins.recordHelp')}</span> : (c.message || c.components?.length) && <span className="text-xs text-ink-3">{c.message ? keyedMessage(t, c) : c.components!.join(' · ')}</span>}
                  </span>
                  <span className={`ss-tag ${actionTone(c.action)}`}>{actionText(c.action)}</span>
                </div>
              ))}
            </div>}
            {/* Only an install or update can end in a prompt inside the Agent; on a removal the line would be noise. */}
            {active.some((c) => c.action === 'install' || c.action === 'update') && <p className="text-xs text-ink-3">{t('plugins.nativeHelp')}</p>}
            <UnchangedRuns runs={idle} isPi={isPi} versions />
            </>
          )}
        </div>
        <div className="df">
          <Button variant="ghost" disabled={busy} onClick={closeReview}>{t('common.cancel')}</Button>
          <Button loading={busy} disabled={review?.plan.blocked || !review?.plan.changes.length} onClick={() => void apply()}>{t('plugins.apply')}</Button>
        </div>
      </DialogShell>

      <SkillContextMenu open={!!menu} anchorPoint={menu ?? undefined} items={menu?.items ?? []} onClose={() => setMenu(null)} />
    </div>
  );
}
