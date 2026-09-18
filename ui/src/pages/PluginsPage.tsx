import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Package, Plus, Download, RefreshCw } from 'lucide-react';
import { pluginsApi, pluginTargets, type PluginPlan, type PluginRequest, type PluginResult, type PluginTarget } from '../api/plugins';
import Button from '../components/Button';
import Card from '../components/Card';
import DialogShell from '../components/DialogShell';
import EmptyState from '../components/EmptyState';
import PageHeader from '../components/PageHeader';
import { PageSkeleton } from '../components/Skeleton';
import { Checkbox } from '../components/Input';
import PluginAddDialog from '../components/plugins/PluginAddDialog';
import { useT } from '../i18n';
import { queryKeys } from '../lib/queryKeys';

export default function PluginsPage() {
  const t = useT();
  const cache = useQueryClient();
  const { data, error, isPending } = useQuery({ queryKey: queryKeys.plugins, queryFn: pluginsApi.list });
  const [adding, setAdding] = useState<{ source?: string; name?: string } | null>(null);
  const [importing, setImporting] = useState(false);
  const [review, setReview] = useState<{ request: PluginRequest; plan: PluginPlan } | null>(null);
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState('');
  const [result, setResult] = useState<PluginResult | null>(null);
  const refresh = () => { void cache.invalidateQueries({ queryKey: queryKeys.plugins }); void cache.invalidateQueries({ queryKey: queryKeys.config }); };
  const preview = async (request: PluginRequest) => {
    setBusy(true); setFailure(''); setResult(null);
    try { const plan = await pluginsApi.preview(request); setReview({ request, plan }); setAdding(null); setImporting(false); }
    catch (e) { setFailure((e as Error).message); throw e; }
    finally { setBusy(false); }
  };
  const begin = (request: PluginRequest) => { void preview(request).catch(() => {}); };
  const apply = async () => {
    if (!review) return;
    setBusy(true); setFailure('');
    try { const response = await pluginsApi.apply(review.request, review.plan.revision); setResult(response); setFailure(response.failure); setReview(null); refresh(); }
    catch (e) { setFailure((e as Error).message); refresh(); }
    finally { setBusy(false); }
  };
  const selectTarget = async (name: string, target: PluginTarget, selected: boolean) => {
    setBusy(true); setFailure(''); setResult(null);
    try {
      const request: PluginRequest = { action: selected ? 'enable' : 'disable', name, targets: [target] };
      const plan = await pluginsApi.preview(request);
      const response = await pluginsApi.apply(request, plan.revision);
      if (response.failure) setFailure(response.failure);
      refresh();
    } catch (e) { setFailure((e as Error).message); }
    finally { setBusy(false); }
  };
  if (isPending) return <PageSkeleton />;
  const actionText = (action: string) => t(({
    noop: 'plugins.noChanges', install: 'plugins.add', import: 'plugins.import', update: 'plugins.update',
    remove: 'plugins.remove', uninstall: 'plugins.remove', forget: 'plugins.remove', selection: 'common.save',
    blocked: 'plugins.blocked', 'update-available': 'plugins.update', 'native-check': 'plugins.unverified',
  } as Record<string, string>)[action] ?? 'plugins.pending');
  const packages = Object.entries(data?.packages ?? {});
  return <div className="space-y-5 animate-fade-in">
    <PageHeader icon={<Package />} title={t('plugins.title')} subtitle={t('plugins.subtitle')} actions={<>
      <Button variant="ghost" disabled={busy} onClick={refresh} aria-label={t('plugins.refresh')}><RefreshCw size={16} /></Button>
      <Button variant="secondary" disabled={busy} onClick={() => setImporting(true)}><Download size={16} />{t('plugins.import')}</Button>
      <Button disabled={busy} onClick={() => setAdding({})}><Plus size={16} />{t('plugins.add')}</Button>
    </>} />
    {(failure || error) && <p role="alert" className="text-danger text-sm">{failure || (error as Error).message}</p>}
    <p className="text-sm text-pencil-light">{t('plugins.selectionHelp')}</p>
    <details className="rounded-lg border border-muted p-3">
      <summary className="cursor-pointer text-sm font-medium">{t('plugins.targets')}</summary>
      <ul className="mt-3 space-y-3">{data?.hosts.map((h) => <li key={h.target} className="text-sm"><strong>{pluginTargets[h.target].label}</strong>{h.version && <span className="ms-2 text-pencil-light">{h.version}</span>}{h.error && <p className="text-warning">{h.error}</p>}{h.note && <p className="text-pencil-light">{h.note}</p>}</li>)}</ul>
    </details>
    {result?.result && <Card><ul className="space-y-2" aria-live="polite">{result.result.results.map((r) => <li key={`${r.name}:${r.target}`} className={r.status === 'failed' ? 'text-danger' : 'text-pencil'}><strong>{r.name} · {r.target}</strong> — {r.status}<p className="text-sm text-pencil-light">{r.message}</p></li>)}</ul></Card>}
    {packages.length === 0 ? <EmptyState icon={Package} title={t('plugins.empty')} description={t('plugins.emptyHelp')} action={<Button onClick={() => setAdding({})}>{t('plugins.add')}</Button>} /> : <>
      <div className="flex gap-2"><Button disabled={busy} onClick={() => begin({ action: 'sync' })}>{t('plugins.sync')}</Button><Button variant="secondary" disabled={busy} onClick={() => begin({ action: 'check' })}>{t('plugins.check')}</Button></div>
      {packages.map(([name, pack]) => <Card key={name}>
        <div className="flex flex-wrap justify-between gap-3"><h2 className="text-lg font-semibold">{name}</h2><div className="flex gap-2"><Button size="sm" variant="secondary" disabled={busy} onClick={() => setAdding({ name, source: Object.values(pack.bindings).find((b) => b.source)?.source })}>{t('plugins.targets')}</Button><Button size="sm" variant="secondary" disabled={busy} onClick={() => begin({ action: 'update', name })}>{t('plugins.update')}</Button><Button size="sm" variant="secondary" disabled={busy} onClick={() => begin({ action: 'remove', name })}>{t('plugins.remove')}</Button></div></div>
        <div className="mt-4 space-y-4">{Object.entries(pack.bindings).map(([target, b]) => {
          const host = data?.hosts.find((h) => h.target === target);
          const installed = host?.installed.find((i) => i.id === b.id);
          const state = b.pending ? t('plugins.pending') : host?.error ? t('plugins.unverified') : installed ? t('plugins.installed') : t('plugins.absent');
          return <div key={target} className="flex flex-wrap gap-3 items-start justify-between border-t border-dashed border-pencil-light/30 pt-3">
            <div className="space-y-1 min-w-0"><Checkbox label={pluginTargets[target as PluginTarget].label} checked={b.sync !== false} disabled={busy} onChange={(on) => void selectTarget(name, target as PluginTarget, on)} /><p className="text-xs text-pencil-light break-all">{b.id} {b.version && `· ${b.version}`}</p>{b.components?.length ? <p className="text-xs text-pencil-light">{b.components.join(' · ')}</p> : null}{b.source && <p className="text-xs text-muted-dark break-all">{b.source}</p>}</div>
            <div className="text-sm text-pencil-light text-end"><p>{state}</p>{installed && !installed.enabled && <p>{t('plugins.nativeDisabled')}</p>}
              <Button variant="link" size="sm" disabled={busy} onClick={() => begin({ action: 'sync', name, targets: [target as PluginTarget] })}>{t('plugins.sync')}</Button>
              <Button variant="link" size="sm" disabled={busy || b.sync === false} onClick={() => begin({ action: 'update', name, targets: [target as PluginTarget] })}>{t('plugins.update')}</Button>
            </div>
          </div>;
        })}</div>
      </Card>)}
    </>}
    {adding && <PluginAddDialog initialName={adding.name} initialSource={adding.source} onClose={() => setAdding(null)} onPreview={preview} />}
    <DialogShell open={importing} onClose={() => setImporting(false)} preventClose={busy} ariaLabel={t('plugins.import')} maxWidth="2xl">
      <div className="space-y-4"><h2 className="text-xl font-semibold">{t('plugins.import')}</h2><p className="text-sm text-pencil-light">{t('plugins.importHelp')}</p>
        {data?.hosts.map((h) => <div key={h.target}><h3 className="font-medium mb-2">{pluginTargets[h.target].label}</h3>{h.installed.length === 0 && <p className="text-sm text-pencil-light">{h.error || t('plugins.absent')}</p>}{h.installed.map((i) => <Button key={i.id} variant="secondary" className="mb-2 me-2" disabled={busy || i.filtered || h.target === 'cursor' || h.target === 'antigravity'} onClick={() => begin({ action: 'import', from: h.target, plugin: i.id })}>{i.id}</Button>)}</div>)}
        <Button variant="ghost" disabled={busy} onClick={() => setImporting(false)}>{t('common.cancel')}</Button>
      </div>
    </DialogShell>
    <DialogShell open={!!review} onClose={() => setReview(null)} preventClose={busy} ariaLabel={t('plugins.preview')} maxWidth="2xl">
      <div className="space-y-4"><h2 className="text-xl font-semibold">{t('plugins.preview')}</h2><p className="text-sm text-pencil-light">{t('plugins.nativeHelp')}</p>
        <ul className="space-y-3">{review?.plan.changes.map((c) => <li key={`${c.name}:${c.target}`}><strong>{c.name} · {c.target}</strong> — <span className={c.action === 'blocked' ? 'text-danger' : ''}>{actionText(c.action)}</span>{c.components?.length ? <p className="text-xs text-pencil-light">{c.components.join(' · ')}</p> : null}{c.message && <p className="text-sm text-pencil-light">{c.message}</p>}</li>)}</ul>
        {review?.plan.changes.length === 0 && <p>{t('plugins.noChanges')}</p>}
        <div className="flex gap-2"><Button variant="ghost" disabled={busy} onClick={() => setReview(null)}>{t('common.cancel')}</Button>{review?.request.action !== 'check' && <Button loading={busy} disabled={review?.plan.blocked || !review?.plan.changes.length} onClick={() => void apply()}>{t('plugins.apply')}</Button>}</div>
      </div>
    </DialogShell>
  </div>;
}
