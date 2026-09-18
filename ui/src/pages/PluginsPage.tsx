import { Fragment, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ChevronRight, Download, FolderOpen, Package, Plus, RefreshCw, Trash2, X } from 'lucide-react';
import { pluginsApi, syncAction, targetMap, type PluginPlan, type PluginRequest, type PluginResult, type PluginTarget, type PluginInventory } from '../api/plugins';
import AgentIcon from '../components/AgentIcon';
import Button from '../components/Button';
import DialogShell from '../components/DialogShell';
import EmptyState from '../components/EmptyState';
import IconButton from '../components/IconButton';
import PageHeader from '../components/PageHeader';
import { PageSkeleton } from '../components/Skeleton';
import { SkillContextMenu, type ContextMenuItem } from '../components/TargetMenu';
import Spinner from '../components/Spinner';
import { RailLayout, RailLine, SyncBox } from '../components/StatusRail';
import PluginAddDialog from '../components/plugins/PluginAddDialog';
import PluginFilesDialog from '../components/plugins/PluginFilesDialog';
import PluginAgents from '../components/plugins/PluginAgents';
import PluginList from '../components/plugins/PluginList';
import { useT } from '../i18n';
import { queryKeys } from '../lib/queryKeys';

export default function PluginsPage() {
  const t = useT();
  const cache = useQueryClient();
  // Config answers at once; asking every Agent's CLI takes seconds. The plugin list is drawn
  // from the quick answer and the Agents are filled in when the full one arrives.
  const quick = useQuery({ queryKey: queryKeys.pluginPackages, queryFn: () => pluginsApi.list(false) });
  const full = useQuery({ queryKey: queryKeys.plugins, queryFn: () => pluginsApi.list() });
  const hostsReady = !!full.data;
  const base = quick.data ?? full.data;
  const data = base && { ...base, hosts: full.data?.hosts ?? [] };
  const error = quick.error ?? full.error;
  const [adding, setAdding] = useState<{ source?: string; name?: string; bound?: PluginInventory['packages'][string]['bindings'] } | null>(null);
  const [importing, setImporting] = useState(false);
  const [browsing, setBrowsing] = useState<{ name: string; source?: string } | null>(null);
  const [review, setReview] = useState<{ request: PluginRequest; plan: PluginPlan } | null>(null);
  const [busy, setBusy] = useState(false);
  const [working, setWorking] = useState('');
  const [failure, setFailure] = useState('');
  const [result, setResult] = useState<PluginResult | null>(null);
  const [menu, setMenu] = useState<{ x: number; y: number; items: ContextMenuItem[] } | null>(null);
  const agentLabel = (target: string) => pluginTargets[target]?.label ?? target;
  // The backend keys its fixed sentences; a message it assembled at runtime has no key
  // and is shown as it came, which is also what the CLI prints.
  const message = (key: string | undefined, text: string | undefined) => (key ? t(key, undefined, text) : text ?? '');
  const refresh = () => { void cache.invalidateQueries({ queryKey: queryKeys.plugins }); void cache.invalidateQueries({ queryKey: queryKeys.config }); };
  // `key` names the control that started this, so only it shows a spinner.
  const preview = async (request: PluginRequest, key = '') => {
    setBusy(true); setWorking(key); setFailure(''); setResult(null);
    try { const plan = await pluginsApi.preview(request); setReview({ request, plan }); setAdding(null); setImporting(false); }
    catch (e) { setFailure((e as Error).message); throw e; }
    finally { setBusy(false); setWorking(''); }
  };
  const begin = (request: PluginRequest, key = '') => { void preview(request, key).catch(() => {}); };
  const apply = async () => {
    if (!review) return;
    setBusy(true); setFailure('');
    try { const response = await pluginsApi.apply(review.request, review.plan.revision); setResult(response); setFailure(response.failure); setReview(null); refresh(); }
    catch (e) { setFailure((e as Error).message); refresh(); }
    finally { setBusy(false); }
  };
  const selectTarget = async (name: string, target: PluginTarget, selected: boolean) => {
    setBusy(true); setWorking(`${name}:${target}`); setFailure(''); setResult(null);
    try {
      const request: PluginRequest = { action: selected ? 'enable' : 'disable', name, targets: [target] };
      const plan = await pluginsApi.preview(request);
      const response = await pluginsApi.apply(request, plan.revision);
      if (response.failure) setFailure(response.failure);
      refresh();
    } catch (e) { setFailure((e as Error).message); }
    finally { setBusy(false); setWorking(''); }
  };
  if (!data && !error) return <PageSkeleton />;
  const actionText = (action: string) => t(({
    noop: 'plugins.noChanges', install: 'resources.install', import: 'plugins.import', update: 'plugins.update',
    remove: 'plugins.remove', uninstall: 'plugins.remove', forget: 'plugins.remove', selection: 'common.save',
    blocked: 'plugins.blocked', 'update-available': 'plugins.update', 'native-check': 'plugins.unverified',
  } as Record<string, string>)[action] ?? 'plugins.pending');
  const pluginTargets = targetMap(data?.targetDefinitions);
  const packages = Object.entries(data?.packages ?? {});
  const outcomes = result?.result?.results ?? [];
  const todo = packages.flatMap(([name, pack]) => Object.entries(pack.bindings).map(([target, b]) => ({ name, target, word: syncAction(b!, data?.hosts.find((h) => h.target === target)) }))).filter((x) => x.word);
  const pending = todo.length;
  const actionTone = (action: string) => (action === 'blocked' ? 'bad' : action === 'noop' ? '' : 'inf');
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
        { key: 'remove', label: t('plugins.remove'), icon: <Trash2 size={14} />, danger: true, onSelect: () => begin({ action: 'remove', name }, name) },
      ],
    });
  };
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
              <div className="flex flex-col gap-1.5">
                {todo.map((x) => <RailLine key={`${x.name}:${x.target}`} name={x.name} agent={agentLabel(x.target)} word={x.word} />)}
              </div>
              <Button className="w-full justify-center" loading={working === 'sync'} disabled={busy} onClick={() => begin({ action: 'sync' }, 'sync')}>{t('plugins.sync')}<ChevronRight size={15} /></Button>
              <p className="text-xs leading-normal text-ink-2">{t('plugins.syncHint')}</p>
            </>
          )}
          {/* Outcomes of the action just run, not plugin state, so they live with Sync instead of reading as more inventory. */}
          {outcomes.length > 0 && (
            <div aria-live="polite" className="flex flex-col gap-1.5">
              <div className="flex items-center text-xs font-semibold text-ink-3">{t('plugins.lastRun')}<IconButton className="ml-auto" size="sm" icon={<X size={14} />} label={t('common.close')} onClick={() => setResult(null)} /></div>
              {outcomes.map((r) => (
                <Fragment key={`${r.name}:${r.target}`}>
                  <RailLine name={r.name} agent={agentLabel(r.target)} word={r.status} bad={r.status === 'failed'} />
                  {r.message && <span className="text-xs text-ink-3">{r.message}</span>}
                </Fragment>
              ))}
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
        {packages.length > 0 && <Button variant="ghost" loading={working === 'check'} disabled={busy} onClick={() => begin({ action: 'check' }, 'check')}><RefreshCw size={15} />{t('plugins.check')}</Button>}
        {addActions}
      </>} />

      {(failure || error) && <div role="alert" className="ss-note bad"><span className="flex-1">{failure || (error as Error).message}</span></div>}

      {data && <RailLayout rail={rail}>
        {packages.length === 0 ? (
          <EmptyState icon={Package} title={t('plugins.empty')} description={t('plugins.emptyHelp')} action={addActions} />
        ) : (
          <PluginList inventory={data} busy={busy} working={working} onToggle={(name, target, on) => void selectTarget(name, target, on)} onMenu={openMenu} onAdd={begin} onBlocked={(name, source) => setAdding({ name, source, bound: data.packages[name]?.bindings })} />
        )}
      </RailLayout>}

      {browsing && <PluginFilesDialog name={browsing.name} source={browsing.source} onClose={() => setBrowsing(null)} />}
      {adding && <PluginAddDialog initialName={adding.name} initialSource={adding.source} bound={adding.bound} onClose={() => setAdding(null)} onPreview={preview} />}

      <DialogShell open={importing} onClose={() => setImporting(false)} preventClose={busy} ariaLabel={t('plugins.import')} maxWidth="2xl" padding="none">
        <div className="dh">
          <div className="flex flex-col gap-1"><h2 className="ss-h2">{t('plugins.import')}</h2><p className="text-[13px] text-ink-2">{t('plugins.importHelp')}</p></div>
          <IconButton icon={<X size={16} />} label={t('common.close')} disabled={busy} onClick={() => setImporting(false)} />
        </div>
        <div className="db overflow-y-auto">
          <div className="ss-list">
            {!hostsReady && <div className="ss-r gap-2 text-[13px] text-ink-2"><Spinner size="sm" />{t('plugins.hostsAsking')}</div>}
            {data?.hosts.map((h) => {
              const locked = !pluginTargets[h.target]?.operations.includes('import');
              return (
                <div key={h.target}>
                  <div className="ss-gh"><span className="ss-at"><AgentIcon target={h.target} size={17} /></span><span className="font-semibold">{(pluginTargets[h.target]?.label ?? h.target)}</span><span className="ss-cnt">{h.installed.length}</span></div>
                  {h.installed.length === 0 && <div className="ss-r !min-h-11"><span className="text-[13px] text-ink-3">{message(h.errorKey, h.error) || t('plugins.absent')}</span></div>}
                  {h.installed.map((i) => (
                    <div key={i.id} className="ss-r !min-h-11">
                      <span className="min-w-0 flex-1 truncate font-mono text-[13px] font-semibold" title={i.id}>{i.id}</span>
                      {i.enabledKnown !== false && !i.enabled && <span className="ss-tag">{t('plugins.nativeDisabled')}</span>}
                      <span className="font-mono text-xs text-ink-3">{i.version}</span>
                      <Button size="sm" variant="secondary" loading={working === `${h.target}:${i.id}`} disabled={busy || i.filtered || locked} onClick={() => begin({ action: 'import', from: h.target, plugin: i.id }, `${h.target}:${i.id}`)}>{t('plugins.importOne')}</Button>
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
            <div className="ss-list">
              {review?.plan.changes.map((c) => (
                <div key={`${c.name}:${c.target}`} className="ss-r">
                  <span className="ss-at"><AgentIcon target={c.target} size={17} /></span>
                  <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span className="flex items-center gap-2"><span className="font-mono font-semibold">{c.name}</span><span className="text-[13px] text-ink-2">{pluginTargets[c.target]?.label ?? c.target}</span></span>
                    {(c.message || c.components?.length) && <span className="text-xs text-ink-3">{c.message || c.components!.join(' · ')}</span>}
                  </span>
                  <span className={`ss-tag ${actionTone(c.action)}`}>{actionText(c.action)}</span>
                </div>
              ))}
            </div>
          )}
          {/* Only an install or update can end in a prompt inside the Agent; on a removal the line would be noise. */}
          {review?.plan.changes.some((c) => c.action === 'install' || c.action === 'update') && <p className="text-xs text-ink-3">{t('plugins.nativeHelp')}</p>}
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
