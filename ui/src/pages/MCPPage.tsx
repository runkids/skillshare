import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Plug } from 'lucide-react';
import { mcpApi, type MCPMutation, type MCPPlan, type MCPServer } from '../api/mcp';
import PageHeader from '../components/PageHeader';
import Card from '../components/Card';
import Button from '../components/Button';
import EmptyState from '../components/EmptyState';
import DialogShell from '../components/DialogShell';
import { Select } from '../components/Input';
import { PageSkeleton } from '../components/Skeleton';
import MCPPreview from '../components/mcp/MCPPreview';
import MCPSetup from '../components/mcp/MCPSetup';
import { useToast } from '../components/Toast';
import { useT } from '../i18n';
import { queryKeys } from '../lib/queryKeys';

export default function MCPPage() {
  const t = useT();
  const { toast } = useToast();
  const cache = useQueryClient();
  const { data, error, isPending } = useQuery({ queryKey: queryKeys.mcp, queryFn: mcpApi.list });
  const [setup, setSetup] = useState<{ name: string; server: MCPServer } | 'new' | null>(null);
  const [preview, setPreview] = useState<{ plan: MCPPlan; mutation: MCPMutation; restore?: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const [backup, setBackup] = useState('');
  const [backups, setBackups] = useState<string[]>([]);

  const reportError = (e: unknown) => toast(e instanceof Error ? e.message : t('common.error.generic'), 'error');
  const saved = (ids: string[]) => {
    setBackups(ids); setSetup(null); setPreview(null);
    void cache.invalidateQueries({ queryKey: queryKeys.mcp });
    void cache.invalidateQueries({ queryKey: queryKeys.config });
    toast(t('mcp.saved'), 'success');
  };
  const showPreview = async (mutation: MCPMutation = {}) => {
    setBusy(true);
    try { setPreview({ plan: await mcpApi.preview(mutation), mutation }); }
    catch (e) { reportError(e); } finally { setBusy(false); }
  };
  const showRestore = async () => {
    setBusy(true);
    try { setPreview({ plan: await mcpApi.previewRestore(backup), mutation: {}, restore: backup }); }
    catch (e) { reportError(e); } finally { setBusy(false); }
  };
  const apply = async () => {
    if (!preview) return;
    setBusy(true);
    try {
    const result = preview.restore
      ? await mcpApi.restore(preview.restore, preview.plan.revision)
      : await mcpApi.configure(preview.mutation, preview.plan.revision, true);
    saved(result.backupIds ?? []);
    } catch (e) { reportError(e); }
    finally { setBusy(false); void cache.invalidateQueries({ queryKey: queryKeys.mcp }); }
  };

  if (isPending) return <PageSkeleton />;
  return <div className="space-y-5 animate-fade-in">
    <PageHeader icon={<Plug />} title="MCP" subtitle={t('mcp.subtitle')} actions={<Button onClick={() => setSetup('new')}>{t('mcp.add')}</Button>} />
    <div className="flex flex-wrap items-end gap-3">
      <Button variant="secondary" loading={busy} onClick={() => showPreview()}>{t('mcp.preview')}</Button>
      {data?.backups.length ? <Select label={t('mcp.restore')} value={backup} onChange={setBackup} options={[{ value: '', label: '—' }, ...data.backups.map(b => ({ value: b.id, label: `${b.target} · ${b.id}` }))]} /> : null}
      {data?.backups.length ? <Button variant="secondary" disabled={!backup} loading={busy} onClick={showRestore}>{t('mcp.restore')}</Button> : null}
    </div>
    {error ? <p role="alert" className="text-danger">{error.message}</p> : null}
    {data?.previewError ? <p role="alert" className="text-warning">{data.previewError}</p> : null}
    {data ? <p className="text-sm text-pencil-light break-all">{t('mcp.source')}: {data.source.path}</p> : null}
    {data && !Object.keys(data.source.servers).length ? <EmptyState icon={Plug} title={t('mcp.empty')} description={t('mcp.setupHint')} action={<Button onClick={() => setSetup('new')}>{t('mcp.add')}</Button>} /> : null}
    {data ? Object.entries(data.source.servers).map(([name, server]) => <Card key={name}>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div><h2 className="font-semibold">{name}</h2><p className="text-sm text-pencil-light">{(server.targets ?? data.source.targets ?? []).join(', ')}</p></div>
        <div className="flex gap-2">
          <Button variant="secondary" onClick={() => setSetup({ name, server })}>{t('mcp.edit')}</Button>
          <Button variant="secondary" loading={busy} onClick={() => showPreview({ name, remove: true })}>{t('mcp.remove')}</Button>
        </div>
      </div>
    </Card>) : null}
    {data?.plan ? <MCPPreview plan={data.plan} /> : null}
    {backups.length ? <Card><p>{t('mcp.backupId')}</p>{backups.map(id => <Button key={id} variant="link" onClick={() => setBackup(id)}>{id}</Button>)}</Card> : null}
    {setup ? <MCPSetup initial={setup === 'new' ? undefined : setup} defaultTargets={data?.source.targets ?? []} existingNames={Object.keys(data?.source.servers ?? {})} onClose={() => setSetup(null)} onSaved={saved} /> : null}
    <DialogShell open={Boolean(preview)} onClose={() => setPreview(null)} preventClose={busy} maxWidth="2xl" ariaLabel={t('mcp.preview')}>
      {preview ? <div className="space-y-4">
        <h2 className="text-xl font-semibold">{preview.restore ? t('mcp.restore') : t('mcp.preview')}</h2>
        <MCPPreview plan={preview.plan} />
        {preview.plan.blocked && !preview.restore ? <>
          <p className="text-warning">{t('mcp.conflictHint')}</p>
          {preview.plan.changes.filter(c => c.action === 'conflict').map(c => <Button key={`${c.target}:${c.name}`} variant="secondary" loading={busy} onClick={() => showPreview({ ...preview.mutation, resolutions: [...(preview.mutation.resolutions ?? []), { target: c.target, name: c.name, action: 'replace' }] })}>{t('mcp.replace')}: {c.target} / {c.name}</Button>)}
        </> : null}
        <div className="flex gap-2"><Button variant="ghost" onClick={() => setPreview(null)}>{t('common.cancel')}</Button><Button loading={busy} disabled={preview.plan.blocked} onClick={apply}>{preview.restore ? t('mcp.restore') : t('mcp.saveSync')}</Button></div>
      </div> : null}
    </DialogShell>
  </div>;
}
