import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Braces, Check, Download, History, Info, Link, Plug, Plus, RefreshCw, ShieldCheck, X } from 'lucide-react';
import { mcpApi, mcpTargets, type MCPMutation, type MCPPlan } from '../api/mcp';
import PageHeader from '../components/PageHeader';
import Card from '../components/Card';
import Button from '../components/Button';
import CopyButton from '../components/CopyButton';
import EmptyState from '../components/EmptyState';
import DialogShell from '../components/DialogShell';
import IconButton from '../components/IconButton';
import { PageSkeleton } from '../components/Skeleton';
import AgentIcon from '../components/AgentIcon';
import MCPMatrix from '../components/mcp/MCPMatrix';
import MCPPreview, { type MCPResolve } from '../components/mcp/MCPPreview';
import MCPRemoveDialog from '../components/mcp/MCPRemoveDialog';
import MCPRestoreDialog from '../components/mcp/MCPRestoreDialog';
import MCPSetup, { type MCPSetupMode } from '../components/mcp/MCPSetup';
import { buildMatrix, countActions } from '../components/mcp/mcpView';
import { useToast } from '../components/Toast';
import Badge from '../components/Badge';
import { useT } from '../i18n';
import { queryKeys } from '../lib/queryKeys';
import { shortenHome } from '../lib/paths';

type SetupState = { name?: string; mode?: MCPSetupMode; importFrom?: { target: string; name: string } };

const methods = [
  { mode: 'url', icon: Link, title: 'mcp.url', description: 'mcp.urlDesc' },
  { mode: 'json', icon: Braces, title: 'mcp.paste', description: 'mcp.pasteDesc' },
  { mode: 'import', icon: Download, title: 'mcp.import', description: 'mcp.importDesc' },
] as const;

export default function MCPPage() {
  const t = useT();
  const { toast } = useToast();
  const cache = useQueryClient();
  const { data, error, isPending } = useQuery({ queryKey: queryKeys.mcp, queryFn: mcpApi.list });
  const [setup, setSetup] = useState<SetupState | null>(null);
  const [preview, setPreview] = useState<{ plan: MCPPlan; mutation: MCPMutation } | null>(null);
  const [removing, setRemoving] = useState('');
  const [backupsOpen, setBackupsOpen] = useState(false);
  const [busy, setBusy] = useState(false);

  const reportError = (e: unknown) => toast(e instanceof Error ? e.message : t('common.error.generic'), 'error');
  const saved = () => {
    setSetup(null); setPreview(null); setRemoving(''); setBackupsOpen(false);
    void cache.invalidateQueries({ queryKey: queryKeys.mcp });
    void cache.invalidateQueries({ queryKey: queryKeys.config });
    toast(t('mcp.saved'), 'success');
  };
  const showPreview = async (mutation: MCPMutation = {}) => {
    setBusy(true);
    try { setPreview({ plan: await mcpApi.preview(mutation), mutation }); }
    catch (e) { reportError(e); } finally { setBusy(false); }
  };
  const resolve: MCPResolve = (target, name, action) => {
    if (action === 'import') { setPreview(null); setSetup({ importFrom: { target, name } }); return; }
    const mutation = preview?.mutation ?? {};
    void showPreview({ ...mutation, resolutions: [...(mutation.resolutions ?? []), { target, name, action: 'replace' }] });
  };
  const apply = async () => {
    if (!preview) return;
    setBusy(true);
    try {
      await mcpApi.configure(preview.mutation, preview.plan.revision, true);
      saved();
    } catch (e) { reportError(e); }
    finally { setBusy(false); void cache.invalidateQueries({ queryKey: queryKeys.mcp }); }
  };

  if (isPending) return <PageSkeleton />;
  const servers = data?.source.servers ?? {};
  const detected = new Set(data?.detected);
  const rows = data ? buildMatrix(servers, data.plan) : [];
  const counts = countActions(data?.plan?.changes ?? []);
  const pending = (counts.add ?? 0) + (counts.update ?? 0) + (counts.remove ?? 0);
  const conflicts = counts.conflict ?? 0;
  const targetsOf = (name: string) => servers[name]?.targets ?? data?.source.targets ?? [];
  const setupName = setup?.name ?? setup?.importFrom?.name;

  return <div className="space-y-5 animate-fade-in">
    <PageHeader icon={<Plug />} title="MCP" subtitle={t('mcp.subtitle')} actions={<>
      {data?.backups.length ? <Button variant="secondary" onClick={() => setBackupsOpen(true)}><History size={16} aria-hidden="true" />{t('mcp.backups')}</Button> : null}
      {rows.length ? <Button onClick={() => setSetup({})}><Plus size={16} aria-hidden="true" />{t('mcp.add')}</Button> : null}
    </>} />
    {error ? <p role="alert" className="text-danger">{error.message}</p> : null}
    {data?.previewError ? <p role="alert" className="text-warning">{data.previewError}</p> : null}

    {data && rows.length ? <Card>
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="flex items-center gap-3 min-w-0">
          {data.plan ? <span className={`w-10 h-10 shrink-0 rounded-full flex items-center justify-center ${pending || conflicts ? 'bg-warning-light text-warning' : 'bg-success-light text-success'}`}>
            {pending || conflicts ? <RefreshCw size={18} aria-hidden="true" /> : <Check size={18} aria-hidden="true" />}
          </span> : null}
          <div className="min-w-0 space-y-0.5">
            {data.plan ? <p className="font-semibold">
              {pending ? t('mcp.pending', { count: pending }) : conflicts ? null : t('mcp.allSynced')}
              {conflicts ? <span className="font-medium text-warning">{pending ? ' · ' : ''}{t('mcp.conflicts', { count: conflicts })}</span> : null}
            </p> : null}
            <p className="flex items-center gap-1.5 min-w-0 text-sm text-pencil-light">
              {t('mcp.source')}
              <span className="font-mono text-xs truncate" title={data.source.path}>{data.source.path}</span>
              <CopyButton value={data.source.path} title={t('mcp.copySource')} copiedLabel={t('mcp.copied')} />
            </p>
          </div>
        </div>
        <Button variant={pending || conflicts ? 'primary' : 'secondary'} loading={busy} onClick={() => showPreview()}>{t('mcp.previewSync')}</Button>
      </div>
    </Card> : null}

    {data && rows.length ? <MCPMatrix rows={rows} busy={busy} onEdit={name => setSetup({ name })} onRemove={setRemoving} onResolve={resolve} /> : null}
    {data && !rows.length ? <Card>
      <EmptyState icon={Plug} title={t('mcp.empty')} description={t('mcp.emptyHint')} action={<div className="w-full max-w-3xl space-y-4">
        <div className="grid gap-3 sm:grid-cols-3 text-left">
          {methods.map(method => <button key={method.mode} type="button" onClick={() => setSetup({ mode: method.mode })} className="flex flex-col items-start gap-2 p-4 bg-surface border border-muted rounded-[var(--radius-lg)] cursor-pointer transition-all duration-150 hover:border-pencil hover:shadow-sm focus-visible:ring-2 focus-visible:ring-pencil/20">
            <span className="w-9 h-9 flex items-center justify-center bg-muted/50 rounded-[var(--radius-md)]"><method.icon size={18} aria-hidden="true" /></span>
            <span className="font-semibold text-pencil">{t(method.title)}</span>
            <span className="text-sm text-pencil-light">{t(method.description)}</span>
          </button>)}
        </div>
        <p className="flex items-center justify-center gap-2 text-sm text-pencil-light"><Info size={14} className="shrink-0" aria-hidden="true" />{t('mcp.adoptHint')}</p>
      </div>} />
    </Card> : null}

    {data ? <section aria-labelledby="mcp-agent-files" className="space-y-3">
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
        <h3 id="mcp-agent-files" className="font-semibold">{t('mcp.agentFiles')}</h3>
        <p className="text-sm text-pencil-light">{t('mcp.agentFilesHint')}</p>
      </div>
      <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
        {mcpTargets.filter(target => data.paths[target]).sort((a, b) => Number(detected.has(b)) - Number(detected.has(a))).map(target => <Card key={target} padding="sm" className={detected.has(target) ? '' : 'opacity-60'}>
          <div className="flex items-center justify-between gap-2">
            <p className="inline-flex items-center gap-2 text-sm font-semibold"><AgentIcon target={target} />{target}</p>
            {detected.has(target) ? <Badge variant="success">{t('targets.detected')}</Badge> : <span className="text-xs text-pencil-light">{t('mcp.notDetected')}</span>}
          </div>
          <p className="font-mono text-xs text-pencil-light truncate" title={data.paths[target]}>{shortenHome(data.paths[target])}</p>
        </Card>)}
      </div>
    </section> : null}

    {setup ? <MCPSetup
      initial={setup.name ? { name: setup.name, server: servers[setup.name] } : undefined}
      initialMode={setup.mode}
      importFrom={setup.importFrom}
      defaultTargets={data?.source.targets ?? []}
      explicitTargets={setupName ? servers[setupName]?.targets : undefined}
      existingNames={Object.keys(servers)}
      paths={data?.paths}
      onClose={() => setSetup(null)}
      onSaved={saved}
    /> : null}
    {removing ? <MCPRemoveDialog name={removing} targets={targetsOf(removing)} onClose={() => setRemoving('')} onSaved={saved} /> : null}
    {backupsOpen && data ? <MCPRestoreDialog backups={data.backups} onClose={() => setBackupsOpen(false)} onRestored={saved} /> : null}
    <DialogShell open={Boolean(preview)} onClose={() => setPreview(null)} preventClose={busy} maxWidth="2xl" ariaLabel={t('mcp.preview')}>
      {preview ? <div className="space-y-4">
        <div className="flex items-start justify-between gap-3">
          <h2 className="text-xl font-semibold">{t('mcp.preview')}</h2>
          <IconButton icon={<X size={16} strokeWidth={2.5} />} label={t('common.close')} disabled={busy} onClick={() => setPreview(null)} />
        </div>
        <MCPPreview plan={preview.plan} busy={busy} onResolve={resolve} />
        <div className="flex flex-wrap items-center justify-between gap-3 pt-4 border-t border-dashed border-pencil-light/30">
          <p className="flex items-center gap-1.5 text-xs text-pencil-light"><ShieldCheck size={14} className="text-success" aria-hidden="true" />{t('mcp.backupNote')}</p>
          <div className="flex gap-2">
            <Button variant="ghost" disabled={busy} onClick={() => setPreview(null)}>{t('common.cancel')}</Button>
            <Button loading={busy} disabled={preview.plan.blocked} onClick={apply}>{t('mcp.saveSync')}</Button>
          </div>
        </div>
      </div> : null}
    </DialogShell>
  </div>;
}
