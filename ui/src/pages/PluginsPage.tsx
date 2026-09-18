import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertCircle, ChevronRight, Download, Package, Plus, RefreshCw, Trash2, Users, X } from 'lucide-react';
import { pluginsApi, targetMap, type PluginPlan, type PluginRequest, type PluginResult, type PluginTarget } from '../api/plugins';
import AgentIcon from '../components/AgentIcon';
import Button from '../components/Button';
import DialogShell from '../components/DialogShell';
import EmptyState from '../components/EmptyState';
import IconButton from '../components/IconButton';
import PageHeader from '../components/PageHeader';
import { PageSkeleton } from '../components/Skeleton';
import { SkillContextMenu, type ContextMenuItem } from '../components/TargetMenu';
import PluginAddDialog from '../components/plugins/PluginAddDialog';
import PluginList from '../components/plugins/PluginList';
import PluginDocsLink from '../components/plugins/PluginDocsLink';
import { useT } from '../i18n';
import { queryKeys } from '../lib/queryKeys';

export default function PluginsPage() {
  const t = useT();
  const cache = useQueryClient();
  const { data, error, isPending, isFetching } = useQuery({ queryKey: queryKeys.plugins, queryFn: pluginsApi.list });
  const [adding, setAdding] = useState<{ source?: string; name?: string } | null>(null);
  const [importing, setImporting] = useState(false);
  const [review, setReview] = useState<{ request: PluginRequest; plan: PluginPlan } | null>(null);
  const [busy, setBusy] = useState(false);
  const [working, setWorking] = useState('');
  const [failure, setFailure] = useState('');
  const [result, setResult] = useState<PluginResult | null>(null);
  const [menu, setMenu] = useState<{ x: number; y: number; items: ContextMenuItem[] } | null>(null);
  // Agents split by what you would do about them: use it, install its CLI, or open the Agent.
  const manualHosts = data?.hosts.filter((h) => data.targetDefinitions?.find((d) => d.target === h.target)?.operations.length === 0) ?? [];
  const byStatus = (status: string) => data?.hosts.filter((h) => h.status === status && !manualHosts.includes(h)) ?? [];
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
  if (isPending) return <PageSkeleton />;
  const actionText = (action: string) => t(({
    noop: 'plugins.noChanges', install: 'resources.install', import: 'plugins.import', update: 'plugins.update',
    remove: 'plugins.remove', uninstall: 'plugins.remove', forget: 'plugins.remove', selection: 'common.save',
    blocked: 'plugins.blocked', 'update-available': 'plugins.update', 'native-check': 'plugins.unverified',
  } as Record<string, string>)[action] ?? 'plugins.pending');
  const pluginTargets = targetMap(data?.targetDefinitions);
  const packages = Object.entries(data?.packages ?? {});
  const pending = packages.reduce((n, [, pack]) => n + Object.values(pack.bindings).filter((b) => b?.pending).length, 0);
  const actionTone = (action: string) => (action === 'blocked' ? 'bad' : action === 'noop' ? '' : 'inf');
  const openMenu = (e: React.MouseEvent<HTMLButtonElement>, name: string, target?: PluginTarget) => {
    const r = e.currentTarget.getBoundingClientRect();
    const targets = target ? [target] : undefined;
    const bindings = data?.packages[name]?.bindings ?? {};
    const source = Object.values(bindings).find((b) => b?.source)?.source;
    const canUpdate = target
      ? bindings[target]?.sync !== false && pluginTargets[target]?.operations.includes('update')
      : Object.keys(bindings).every((key) => pluginTargets[key]?.operations.includes('update'));
    setMenu({
      x: r.left,
      y: r.bottom + 4,
      items: [
        { key: 'sync', label: t('plugins.sync'), icon: <ChevronRight size={14} />, onSelect: () => begin({ action: 'sync', name, targets }, target ? `${name}:${target}` : name) },
        // A deselected agent has nothing installed to update.
        ...(canUpdate ? [
          { key: 'update', label: t('plugins.update'), icon: <RefreshCw size={14} />, onSelect: () => begin({ action: 'update', name, targets }, target ? `${name}:${target}` : name) },
        ] : []),
        ...(target ? [] : [
          { key: 'targets', label: t('plugins.targets'), icon: <Users size={14} />, onSelect: () => setAdding({ name, source }) },
          { key: 'remove', label: t('plugins.remove'), icon: <Trash2 size={14} />, danger: true, onSelect: () => begin({ action: 'remove', name }, name) },
        ]),
      ],
    });
  };
  const addActions = <>
    <Button variant="secondary" disabled={busy} onClick={() => setImporting(true)}><Download size={15} />{t('plugins.import')}</Button>
    <Button disabled={busy} onClick={() => setAdding({})}><Plus size={15} />{t('plugins.add')}</Button>
  </>;

  return (
    <div className="ss-wrap animate-fade-in">
      <PageHeader title={t('plugins.title')} subtitle={t('plugins.subtitle')} actions={<>
        {packages.length > 0 && <Button variant="ghost" loading={working === 'check'} disabled={busy} onClick={() => begin({ action: 'check' }, 'check')}><RefreshCw size={15} />{t('plugins.check')}</Button>}
        {addActions}
      </>} />

      {(failure || error) && <div role="alert" className="ss-note bad"><span className="flex-1">{failure || (error as Error).message}</span></div>}

      {result?.result && result.result.results.length > 0 && (
        <section aria-live="polite">
          {/* These are outcomes of the action just run, not plugin state. Unlabelled they read as more inventory. */}
          <div className="ss-sec">
            <h2>{t('plugins.lastRun')}</h2>
            <span className="ss-cnt">{result.result.results.length}</span>
            <IconButton className="ml-auto" icon={<X size={16} />} label={t('common.close')} onClick={() => setResult(null)} />
          </div>
          <div className="ss-list">
            {result.result.results.map((r) => (
              <div key={`${r.name}:${r.target}`} className="ss-r">
                <span className="ss-at"><AgentIcon target={r.target} size={17} /></span>
                <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <span className="flex items-center gap-2"><span className="font-mono font-semibold">{r.name}</span><span className="text-[13px] text-ink-2">{pluginTargets[r.target]?.label ?? r.target}</span></span>
                  {r.message && <span className="text-xs text-ink-3">{r.message}</span>}
                </span>
                <span className={`ss-st ${r.status === 'failed' ? 'bad' : 'ok'}`}>{r.status}</span>
              </div>
            ))}
          </div>
        </section>
      )}

      {packages.length === 0 ? (
        <EmptyState icon={Package} title={t('plugins.empty')} description={t('plugins.emptyHelp')} action={addActions} />
      ) : (
        <>
          <div className="flex flex-col gap-3">
            <PluginList inventory={data!} busy={busy} working={working} onToggle={(name, target, on) => void selectTarget(name, target, on)} onMenu={openMenu} />
            <p className="text-xs text-ink-3">{t('plugins.selectionHelp')}</p>
          </div>
          <div className="flex items-center justify-between gap-3">
            <span>{pending > 0 && <span className="ss-st warn text-[13px]">{t(pending === 1 ? 'mcp.pending.one' : 'mcp.pending.other', { count: pending })}</span>}</span>
            <Button variant="secondary" loading={working === 'sync'} disabled={busy} onClick={() => begin({ action: 'sync' }, 'sync')}>{t('plugins.sync')}<ChevronRight size={15} /></Button>
          </div>
        </>
      )}

      {data && data.hosts.length > 0 && (
        <section>
          <div className="ss-sec">
            <h2>{t('layout.nav.agents')}</h2>
            <span className="ss-cnt">{data.hosts.length}</span>
            <IconButton className="ml-auto" icon={<RefreshCw size={15} className={isFetching ? 'animate-spin' : ''} />} label={t('plugins.refresh')} disabled={busy || isFetching} onClick={refresh} />
          </div>
          <div className="flex flex-col gap-5">
            {byStatus('ready').length > 0 && (
              <div className="flex flex-col gap-2">
                <span className="flex items-baseline gap-2"><span className="text-[13px] font-semibold">{t('plugins.hostReady')}</span><span className="ss-cnt">{byStatus('ready').length}</span></span>
                <div className="ss-list">
                  {byStatus('ready').map((h) => (
                    <div key={h.target} className="ss-r !min-h-11">
                      <span className="ss-at"><AgentIcon target={h.target} size={17} /></span>
                      <span className="w-32 shrink-0 font-semibold">{agentLabel(h.target)}</span>
                      <span className="w-40 shrink-0 truncate font-mono text-xs text-ink-3" title={h.version}>{h.version}</span>
                      <span className="min-w-0 flex-1 py-1 text-[13px] text-ink-2">{h.target === 'grok' ? t('plugins.reason.grok') : message(h.noteKey || 'plugins.note.native', h.note)}</span>
                      <PluginDocsLink target={h.target} label={agentLabel(h.target)} />
                      {h.installed.length > 0 && <span className="ss-tag ok">{t(h.installed.length === 1 ? 'plugins.hostInstalled.one' : 'plugins.hostInstalled.other', { count: h.installed.length })}</span>}
                    </div>
                  ))}
                </div>
              </div>
            )}
            {byStatus('missing').length > 0 && (
              <div className="flex flex-col gap-2">
                <span className="flex items-baseline gap-2"><span className="text-[13px] font-semibold text-ink-2">{t('plugins.hostMissing')}</span><span className="ss-cnt">{byStatus('missing').length}</span></span>
                {/* One cause shared by every Agent here, so it is stated once instead of per row. */}
                <div className="ss-note warn !items-start">
                  <AlertCircle size={16} className="mt-0.5" />
                  <div className="flex min-w-0 flex-1 flex-col gap-2.5">
                    <span>{t('plugins.hostMissingHelp')}</span>
                    <div className="flex flex-wrap gap-2">
                      {byStatus('missing').map((h) => (
                        <span key={h.target} className="ss-chip"><AgentIcon target={h.target} size={16} />{agentLabel(h.target)}<PluginDocsLink target={h.target} label={agentLabel(h.target)} /></span>
                      ))}
                    </div>
                  </div>
                </div>
              </div>
            )}
            {byStatus('blocked').length > 0 && (
              <div className="flex flex-col gap-2">
                <span className="flex items-baseline gap-2"><span className="text-[13px] font-semibold text-ink-2">{t('plugins.hostBlocked')}</span><span className="ss-cnt">{byStatus('blocked').length}</span></span>
                <div className="ss-list">
                  {byStatus('blocked').map((h) => (
                    <div key={h.target} className="ss-r !min-h-10">
                      <span className="ss-at"><AgentIcon target={h.target} size={17} /></span>
                      <span className="w-32 shrink-0 font-semibold text-ink-2">{agentLabel(h.target)}</span>
                      <span className="ss-st warn wrap min-w-0 flex-1 py-1">{message(h.errorKey, h.error)}</span>
                      <PluginDocsLink target={h.target} label={agentLabel(h.target)} />
                    </div>
                  ))}
                </div>
              </div>
            )}
            {manualHosts.length > 0 && (
              <div className="flex flex-col gap-2">
                <span className="text-[13px] font-semibold text-ink-2">{t('plugins.hostManual')}</span>
                <p className="text-xs text-ink-3">{t('plugins.hostManualHelp')}</p>
                <div className="ss-list">
                  {manualHosts.map((h) => (
                    <div key={h.target} className="ss-r !min-h-11">
                      <span className="ss-at"><AgentIcon target={h.target} size={17} /></span>
                      <span className="w-32 shrink-0 font-semibold">{agentLabel(h.target)}</span>
                      <span className="min-w-0 flex-1 py-1 text-[13px] text-ink-2">{message(h.errorKey, h.error)}</span>
                      <PluginDocsLink target={h.target} label={agentLabel(h.target)} />
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        </section>
      )}

      {adding && <PluginAddDialog initialName={adding.name} initialSource={adding.source} onClose={() => setAdding(null)} onPreview={preview} />}

      <DialogShell open={importing} onClose={() => setImporting(false)} preventClose={busy} ariaLabel={t('plugins.import')} maxWidth="2xl" padding="none">
        <div className="dh">
          <div className="flex flex-col gap-1"><h2 className="ss-h2">{t('plugins.import')}</h2><p className="text-[13px] text-ink-2">{t('plugins.importHelp')}</p></div>
          <IconButton icon={<X size={16} />} label={t('common.close')} disabled={busy} onClick={() => setImporting(false)} />
        </div>
        <div className="db overflow-y-auto">
          <div className="ss-list">
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
          <div className="flex flex-col gap-1"><h2 className="ss-h2">{t('plugins.preview')}</h2><p className="text-[13px] text-ink-2">{t('plugins.nativeHelp')}</p></div>
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
