import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ChevronRight, Download, FolderOpen, Package, Plus, RefreshCw, Share2, Trash2, X } from 'lucide-react';
import { bindingVersion, byPlugin, pluginShareCommand, pluginsApi, syncAction, targetMap, type PluginPlan, type PluginRequest, type PluginResult, type PluginTarget, type PluginInventory } from '../api/plugins';
import AgentIcon from '../components/AgentIcon';
import Button from '../components/Button';
import DialogShell from '../components/DialogShell';
import EmptyState from '../components/EmptyState';
import IconButton from '../components/IconButton';
import PageHeader from '../components/PageHeader';
import { PageSkeleton } from '../components/Skeleton';
import { SkillContextMenu, type ContextMenuItem } from '../components/TargetMenu';
import Spinner from '../components/Spinner';
import { RailLayout, SyncBox } from '../components/StatusRail';
import PluginAddDialog from '../components/plugins/PluginAddDialog';
import PluginFilesDialog from '../components/plugins/PluginFilesDialog';
import PluginShareDialog from '../components/plugins/PluginShareDialog';
import PluginAgents from '../components/plugins/PluginAgents';
import { keyedMessage, outcomeStatus } from '../components/plugins/outcomeText';
import PluginList, { VersionChange } from '../components/plugins/PluginList';
import { PluginRunLine, UnchangedRuns } from '../components/plugins/PluginRuns';
import { useT } from '../i18n';
import { useSlow } from '../hooks/useSlow';
import { queryKeys } from '../lib/queryKeys';

export default function PluginsPage() {
  const t = useT();
  const cache = useQueryClient();
  // Config answers at once; asking every Agent's CLI takes seconds. The plugin list is drawn
  // from the quick answer and the Agents are filled in when the full one arrives.
  const quick = useQuery({ queryKey: queryKeys.pluginPackages, queryFn: () => pluginsApi.list(false) });
  const full = useQuery({ queryKey: queryKeys.plugins, queryFn: () => pluginsApi.list() });
  const hostsReady = !!full.data;
  const hostsSlow = useSlow(!hostsReady, 10000);
  const base = quick.data ?? full.data;
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
  const [sharing, setSharing] = useState<string[] | null>(null);
  const [browsing, setBrowsing] = useState<{ name: string; source?: string } | null>(null);
  const [review, setReview] = useState<{ request: PluginRequest; plan: PluginPlan } | null>(null);
  const [busy, setBusy] = useState(false);
  const [working, setWorking] = useState('');
  const [failure, setFailure] = useState('');
  const [result, setResult] = useState<PluginResult | null>(null);
  // The server joins each failed target's English text; show the translated messages once each instead.
  const failureText = (response: PluginResult) => {
    const failed = new Set((response.result?.results ?? []).filter((r) => r.status === 'failed').map((r) => keyedMessage(t, r)));
    return response.failure && failed.size ? [...failed].join(' ') : response.failure;
  };
  // What the last check found to update, per plugin and Agent, with the version it would bring
  // ('' when the source changed without a new one). An applied update takes its Agents off.
  const [updates, setUpdates] = useState<Record<string, Record<PluginTarget, string>>>({});
  const [menu, setMenu] = useState<{ x: number; y: number; items: ContextMenuItem[] } | null>(null);
  // A change with no Agent is to Skillshare itself: the plugin was only added to it, or removed from it.
  const agentLabel = (target: string) => pluginTargets[target]?.label ?? (target || t('plugins.skillshareOnly'));
  // The backend keys its fixed sentences; a message it assembled at runtime has no key
  // and is shown as it came, which is also what the CLI prints.
  const message = (key: string | undefined, text: string | undefined, args?: Record<string, string>) => (key ? t(key, args, text) : text ?? '');
  const refresh = () => {
    void cache.invalidateQueries({ queryKey: queryKeys.plugins });
    void cache.invalidateQueries({ queryKey: queryKeys.config });
    void cache.invalidateQueries({ queryKey: queryKeys.piExtensionsAll });
    void cache.invalidateQueries({ queryKey: queryKeys.ompExtensionsAll });
  };
  // `key` names the control that started this, so only it shows a spinner.
  const preview = async (request: PluginRequest, key = '') => {
    setBusy(true); setWorking(key); setFailure(''); setResult(null);
    try {
      const plan = await pluginsApi.preview(request);
      if (request.action === 'check') {
        const found: Record<string, Record<PluginTarget, string>> = {};
        for (const c of plan.changes) if (c.action === 'update-available') (found[c.name] ??= {})[c.target] = c.binding?.version ?? '';
        setUpdates(found);
      }
      setReview({ request, plan }); setAdding(null); setImporting(false);
    }
    catch (e) { setFailure((e as Error).message); throw e; }
    finally { setBusy(false); setWorking(''); }
  };
  const begin = (request: PluginRequest, key = '') => { void preview(request, key).catch(() => {}); };
  const apply = async () => {
    if (!review) return;
    setBusy(true); setFailure('');
    try {
      const response = await pluginsApi.apply(review.request, review.plan.revision);
      setResult(response); setFailure(failureText(response)); setReview(null); refresh();
      if (review.request.action === 'update') {
        // An Agent comes off once the result reports it updated, or already there (noop); one that
        // failed, was skipped, or never ran because the apply stopped first keeps its Update action.
        const done = new Set((response.result?.results ?? []).filter((r) => r.status !== 'failed').map((r) => `${r.name}:${r.target}`));
        setUpdates((prev) => {
          const next = { ...prev };
          for (const c of review.plan.changes) {
            if ((c.action !== 'update' && c.action !== 'noop') || !done.has(`${c.name}:${c.target}`) || !next[c.name]) continue;
            next[c.name] = { ...next[c.name] };
            delete next[c.name][c.target];
          }
          return next;
        });
      }
    }
    catch (e) { setFailure((e as Error).message); refresh(); }
    finally { setBusy(false); }
  };
  const selectTarget = async (name: string, target: PluginTarget, selected: boolean) => {
    setBusy(true); setWorking(`${name}:${target}`); setFailure(''); setResult(null);
    try {
      const request: PluginRequest = { action: selected ? 'enable' : 'disable', name, targets: [target] };
      const plan = await pluginsApi.preview(request);
      const response = await pluginsApi.apply(request, plan.revision);
      if (response.failure) setFailure(failureText(response));
      refresh();
    } catch (e) { setFailure((e as Error).message); }
    finally { setBusy(false); setWorking(''); }
  };
  if (!data && !error) return <PageSkeleton />;
  const actionText = (action: string) => t(({
    noop: 'plugins.noChanges', skip: 'plugins.outcome.skipped', install: 'resources.install', import: 'plugins.import', update: 'plugins.update',
    remove: 'plugins.remove', uninstall: 'plugins.remove', forget: 'plugins.remove', selection: 'common.save', record: 'plugins.record',
    blocked: 'plugins.blocked', 'update-available': 'plugins.update', 'native-check': 'plugins.unverified',
  } as Record<string, string>)[action] ?? 'plugins.pending');
  const pluginTargets = targetMap(data?.targetDefinitions);
  const packages = Object.entries(data?.packages ?? {});
  const outcomes = result?.result?.results ?? [];
  // A plugin with no Agent yet has only what was recorded when it was added.
  const shareable = packages.map(([name, pack]) => ({ name, command: pluginShareCommand(name, Object.values(pack.bindings).find((b) => b?.source) ?? pack) })).filter((p) => p.command);
  const todo = packages.flatMap(([name, pack]) => Object.entries(pack.bindings).map(([target, b]) => ({ name, target, word: syncAction(b!, data?.hosts.find((h) => h.target === target)) }))).filter((x) => x.word);
  const pending = todo.length;
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
  // A package Pi installs itself (no source Skillshare copies from) and bound only to Pi targets
  // is a Pi package: it gets its own list, in the same rows. A plugin Skillshare installs stays above.
  const piTargets = new Set((data?.targetDefinitions ?? []).filter((d) => d.npm).map((d) => d.target));
  const names = Object.keys(data?.packages ?? {});
  const isPi = (name: string) => {
    // A run can name a package config no longer has, such as one it just removed.
    const pack = data?.packages[name];
    if (!pack) return false;
    const bound = Object.entries(pack.bindings);
    return !pack.source && bound.length > 0 && bound.every(([target, b]) => piTargets.has(target) && !b?.source);
  };
  const piPackages = names.filter(isPi);
  const installedVersion = (name: string, target: string) => {
    const b = data?.packages[name]?.bindings[target];
    return b && data ? bindingVersion(data, target, b) : undefined;
  };
  // A preview lists every binding; the ones it leaves alone fold away under the ones it changes.
  const active = review?.plan.changes.filter((c) => c.action !== 'noop') ?? [];
  const idle = (review?.plan.changes ?? []).filter((c) => c.action === 'noop').map((c) => ({ name: c.name, target: c.target, version: c.binding?.version ?? installedVersion(c.name, c.target) }));
  // What the last run changed, failures first; Agents that ended the same way share a row.
  const changed = byPlugin(outcomes.filter((r) => r.status !== 'unchanged').map((r) => ({ ...r, note: r.message ? keyedMessage(t, r) : '' })), (r) => `${r.status}\0${r.note}`)
    .sort((a, b) => Number(b.status === 'failed') - Number(a.status === 'failed'));
  const plugins = names.filter((name) => !isPi(name));
  const list = (shown: string[], pi?: boolean) => data && (
    <PluginList inventory={data} names={shown} pi={pi} updates={updates} busy={busy} working={working} onToggle={(name, target, on) => void selectTarget(name, target, on)} onMenu={openMenu} onAdd={begin} onBlocked={(name, source) => setAdding({ name, source, bound: data.packages[name]?.bindings, recorded: data.packages[name] })} />
  );
  const addActions = <>
    <Button variant="secondary" disabled={busy} onClick={() => setImporting(true)}><Download size={15} />{t('plugins.import')}</Button>
    <Button disabled={busy} onClick={() => setAdding({})}><Plus size={15} />{t('plugins.add')}</Button>
  </>;

  // State, not inventory: what Sync would do, then which Agents can take part.
  const rail = data && (
    <>
      {(packages.length > 0 || outcomes.length > 0) && (
        // What config recorded as pending is known at once; the rest, and "synced", only after the Agents answered.
        <SyncBox tone={pending > 0 ? 'warn' : hostsReady ? 'ok' : 'busy'} state={pending > 0 ? t(pending === 1 ? 'plugins.pendingCount.one' : 'plugins.pendingCount.other', { count: pending }) : t(hostsReady ? 'targets.state.synced' : 'plugins.checking')}>
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
              <div className="flex items-center text-xs font-semibold text-ink-3">{t('plugins.lastRun')}<IconButton className="ml-auto" size="sm" icon={<X size={14} />} label={t('common.close')} onClick={() => setResult(null)} /></div>
              {changed.map((r) => (
                <PluginRunLine key={`${r.name}:${r.status}:${r.note}`} name={r.name} targets={r.targets} label={agentLabel} note={r.note}
                  className={r.status === 'failed' ? 'rounded-lg bg-bad-bg px-3 py-2' : ''}
                  right={<span className={`ss-st ${r.status === 'failed' ? 'bad' : r.status === 'skipped' ? 'warn' : ''}`}>{outcomeStatus(t, r.status)}</span>} />
              ))}
              <UnchangedRuns runs={outcomes.filter((r) => r.status === 'unchanged')} isPi={isPi} />
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
        {packages.length > 0 && <Button variant="ghost" loading={working === 'check'} disabled={busy} onClick={() => begin({ action: 'check' }, 'check')}><RefreshCw size={15} />{t('plugins.check')}</Button>}
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
            {data?.hosts.map((h) => {
              const locked = !pluginTargets[h.target]?.operations.includes('import');
              // What Skillshare installed or already imported into this Agent has nothing left to import.
              const managed = new Set(Object.values(data?.packages ?? {}).map((p) => p.bindings[h.target]?.id));
              const importable = h.installed.filter((i) => !managed.has(i.id));
              return (
                <div key={h.target}>
                  <div className="ss-gh"><span className="ss-at"><AgentIcon target={h.target} size={17} /></span><span className="font-semibold">{(pluginTargets[h.target]?.label ?? h.target)}</span><span className="ss-cnt">{importable.length}</span></div>
                  {h.installed.length === 0 && <div className="ss-r !min-h-11"><span className="text-[13px] text-ink-3">{message(h.errorKey, h.error, h.errorArgs) || t('plugins.absent')}</span></div>}
                  {h.installed.length > 0 && importable.length === 0 && <div className="ss-r !min-h-11"><span className="text-[13px] text-ink-3">{t('plugins.allManaged')}</span></div>}
                  {importable.map((i) => (
                    <div key={i.id} className="ss-r !min-h-11">
                      <span className="min-w-0 flex-1 truncate font-mono text-[13px] font-semibold" title={i.id}>{i.id}</span>
                      {i.enabledKnown !== false && !i.enabled && <span className="ss-tag">{t('plugins.nativeDisabled')}</span>}
                      <span className="font-mono text-xs text-ink-3">{i.version}</span>
                      <Button size="sm" variant="secondary" loading={working === `${h.target}:${i.id}`} disabled={busy || (i.filtered && !i.importable) || locked} onClick={() => begin({ action: 'import', from: h.target, plugin: i.id }, `${h.target}:${i.id}`)}>{t('plugins.importOne')}</Button>
                    </div>
                  ))}
                </div>
              );
            })}
          </div>
        </div>
        <div className="df"><Button variant="ghost" disabled={busy} onClick={() => setImporting(false)}>{t('common.close')}</Button></div>
      </DialogShell>

      <DialogShell open={!!review} onClose={() => setReview(null)} preventClose={busy} ariaLabel={t('plugins.preview')} maxWidth="2xl" padding="none">
        <div className="dh">
          <div className="flex flex-col gap-1"><h2 className="ss-h2">{t('plugins.preview')}</h2><p className="text-[13px] text-ink-2">{t('plugins.previewHelp')}</p></div>
          <IconButton icon={<X size={16} />} label={t('common.close')} disabled={busy} onClick={() => setReview(null)} />
        </div>
        <div className="db overflow-y-auto">
          {review?.plan.changes.length === 0 ? <p className="text-[13px] text-ink-2">{t('plugins.noChanges')}</p> : (
            <>
            {active.length > 0 && <div className="ss-list">
              {active.map((c) => (
                <div key={`${c.name}:${c.target}`} className="ss-r">
                  <span className="ss-at">{c.target ? <AgentIcon target={c.target} size={17} /> : c.logo ? <img src={c.logo} alt="" className="size-full rounded-[inherit] object-cover" /> : <Package size={15} />}</span>
                  <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span className="flex items-center gap-2"><span className="font-mono font-semibold">{c.name}</span><span className="text-[13px] text-ink-2">{agentLabel(c.target)}</span><VersionChange from={installedVersion(c.name, c.target)} to={c.action.startsWith('update') ? c.binding?.version : undefined} /></span>
                    {!!c.preservedKeys?.length && <span className="text-xs text-ink-3"><span>{t('plugins.preservedKeys')}</span>{' '}{c.preservedKeys.join(' · ')}</span>}
                    {c.action === 'record' ? <span className="text-xs text-ink-3">{t('plugins.recordHelp')}</span> : (c.message || c.components?.length) && <span className="text-xs text-ink-3">{c.message ? keyedMessage(t, c) : c.components!.join(' · ')}</span>}
                  </span>
                  {/* What a check found can be updated from here: just that plugin on that Agent, after its own review. */}
                  {c.action === 'update-available'
                    ? <Button size="sm" variant="secondary" loading={working === `${c.name}:${c.target}`} disabled={busy} onClick={() => begin({ action: 'update', name: c.name, targets: [c.target] }, `${c.name}:${c.target}`)}>{t('plugins.update')}</Button>
                    : <span className={`ss-tag ${actionTone(c.action)}`}>{actionText(c.action)}</span>}
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
          <Button variant="ghost" disabled={busy} onClick={() => setReview(null)}>{t('common.cancel')}</Button>
          {review?.request.action !== 'check' && <Button loading={busy} disabled={review?.plan.blocked || !review?.plan.changes.length} onClick={() => void apply()}>{t('plugins.apply')}</Button>}
        </div>
      </DialogShell>

      <SkillContextMenu open={!!menu} anchorPoint={menu ?? undefined} items={menu?.items ?? []} onClose={() => setMenu(null)} />
    </div>
  );
}
