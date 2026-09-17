import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertCircle, Archive, ChevronRight, Copy, Download, Pencil, Plug, Plus, Trash2, X } from 'lucide-react';
import { mcpApi, mcpTargets, type MCPMutation, type MCPPlan } from '../api/mcp';
import AgentIcon from '../components/AgentIcon';
import Button from '../components/Button';
import DialogShell from '../components/DialogShell';
import EmptyState from '../components/EmptyState';
import PageHeader from '../components/PageHeader';
import { PageSkeleton } from '../components/Skeleton';
import { SkillContextMenu, type ContextMenuItem } from '../components/TargetMenu';
import { useToast } from '../components/Toast';
import MCPImportDialog from '../components/mcp/MCPImportDialog';
import MCPServerList from '../components/mcp/MCPServerList';
import MCPPreview, { type MCPResolve } from '../components/mcp/MCPPreview';
import MCPRemoveDialog from '../components/mcp/MCPRemoveDialog';
import MCPRestoreDialog from '../components/mcp/MCPRestoreDialog';
import MCPServerDialog from '../components/mcp/MCPServerDialog';
import { buildMatrix, countActions, describeMessage, isResolvable, targetLabel, type MCPChange } from '../components/mcp/mcpView';
import { useT } from '../i18n';
import { shortenHome } from '../lib/paths';
import { queryKeys } from '../lib/queryKeys';

type MCPList = Awaited<ReturnType<typeof mcpApi.list>>;

const copy = (text: string) => void navigator.clipboard?.writeText(text);

export default function MCPPage() {
  const t = useT();
  const { toast } = useToast();
  const navigate = useNavigate();
  const cache = useQueryClient();
  const { data, error, isPending } = useQuery({ queryKey: queryKeys.mcp, queryFn: mcpApi.list });
  const [editing, setEditing] = useState<string | null>(null); // '' adds a new server
  const [importing, setImporting] = useState<{ conflict?: { target: string; name: string } } | null>(null);
  const [removing, setRemoving] = useState('');
  const [backupsOpen, setBackupsOpen] = useState(false);
  const [replace, setReplace] = useState<{ plan: MCPPlan; mutation: MCPMutation } | null>(null);
  const [busy, setBusy] = useState(false);
  const [menu, setMenu] = useState<{ x: number; y: number; items: ContextMenuItem[] } | null>(null);
  const [allFiles, setAllFiles] = useState(false);

  const refresh = () => {
    void cache.invalidateQueries({ queryKey: queryKeys.mcp });
    void cache.invalidateQueries({ queryKey: queryKeys.config });
  };
  const done = (message: string) => {
    setEditing(null); setImporting(null); setRemoving(''); setBackupsOpen(false); setReplace(null);
    refresh();
    toast(message, 'success');
  };

  if (isPending) return <PageSkeleton />;

  const servers = data?.source.servers ?? {};
  const defaults = data?.source.targets ?? [];
  const targetsOf = (name: string) => servers[name]?.targets ?? defaults;
  const rows = data ? buildMatrix(servers, data.plan) : [];
  const changes = data?.plan?.changes ?? [];
  const counts = countActions(changes);
  const pending = (counts.add ?? 0) + (counts.update ?? 0) + (counts.remove ?? 0);
  const conflicts = changes.filter((c) => c.action === 'conflict');
  const detected = new Set(data?.detected);
  const files = mcpTargets.filter((x) => data?.paths[x]);
  const matrixTargets = new Set([...files, ...rows.flatMap((row) => [...targetsOf(row.name), ...Object.keys(row.cells)])]);
  const shownFiles = allFiles ? files : files.filter((x) => detected.has(x));
  const hiddenFiles = files.length - shownFiles.length;

  const toggle = async (name: string, target: string, on: boolean) => {
    const current = targetsOf(name);
    const next = mcpTargets.filter((x) => (x === target ? on : current.includes(x)));
    if (next.length === 0) {
      toast(t('mcp.needTarget'), 'warning');
      return;
    }
    const server = { ...servers[name], targets: next };
    // Tick right away; saving only touches the source, Sync writes the files
    const prev = cache.getQueryData<MCPList>(queryKeys.mcp);
    cache.setQueryData<MCPList>(queryKeys.mcp, (old) => old && { ...old, source: { ...old.source, servers: { ...old.source.servers, [name]: server } } });
    try {
      await mcpApi.save({ name, server, replace: true });
    } catch (e) {
      if (prev) cache.setQueryData(queryKeys.mcp, prev);
      toast((e as Error).message, 'error');
    }
    refresh();
  };

  const resolve: MCPResolve = async (target, name, action) => {
    if (action === 'import') {
      setImporting({ conflict: { target, name } });
      return;
    }
    const mutation: MCPMutation = { resolutions: [{ target, name, action: 'replace' }] };
    setBusy(true);
    try {
      setReplace({ plan: await mcpApi.preview(mutation), mutation });
    } catch (e) {
      toast((e as Error).message, 'error');
    } finally {
      setBusy(false);
    }
  };

  const applyReplace = async () => {
    if (!replace) return;
    setBusy(true);
    try {
      await mcpApi.configure(replace.mutation, replace.plan.revision, true);
      done(t('mcp.toast.replaced'));
    } catch (e) {
      toast((e as Error).message, 'error');
    } finally {
      setBusy(false);
    }
  };

  const openMenu = (e: React.MouseEvent<HTMLButtonElement>, name: string) => {
    const r = e.currentTarget.getBoundingClientRect();
    setMenu({
      x: r.left,
      y: r.bottom + 4,
      items: [
        { key: 'edit', label: t('mcp.edit'), icon: <Pencil size={14} />, onSelect: () => setEditing(name) },
        { key: 'remove', label: t('mcp.remove'), icon: <Trash2 size={14} />, danger: true, onSelect: () => setRemoving(name) },
      ],
    });
  };

  const conflictText = (c: MCPChange) => {
    const params = { target: targetLabel(c.target), name: c.name };
    if (c.message?.startsWith('existing entry is not managed')) return t('mcp.conflict.unmanaged', params);
    if (c.message?.startsWith('Agent configuration changed')) return t('mcp.conflict.changed', params);
    return `${params.target} · ${c.name}: ${describeMessage(t, c.message)}`;
  };

  return (
    <div className="animate-fade-in">
      <PageHeader
        title="MCP"
        subtitle={t('mcp.subtitle')}
        actions={<span className="flex items-center gap-2.5" data-tour="mcp-actions">
          {data?.backups.length ? <Button variant="ghost" onClick={() => setBackupsOpen(true)}><Archive size={15} />{t('mcp.backupsButton')}</Button> : null}
          <Button variant="secondary" onClick={() => setImporting({})}><Download size={15} />{t('mcp.importFromTarget')}</Button>
          <Button variant="primary" onClick={() => setEditing('')}><Plus size={15} />{t('mcp.addServer')}</Button>
        </span>}
      />

      {error && <div className="ss-note bad mb-4"><span className="flex-1">{error.message}</span></div>}
      {data?.previewError && <div className="ss-note bad mb-4"><AlertCircle size={16} /><span className="flex-1">{data.previewError}</span></div>}

      {data && (
        <div className="flex flex-col gap-7">
          {rows.length > 0 ? (
            <MCPServerList rows={rows} targets={mcpTargets.filter((x) => matrixTargets.has(x))} targetsOf={targetsOf} onToggle={(n, x, on) => void toggle(n, x, on)} onMenu={openMenu} />
          ) : (
            <EmptyState
              icon={Plug}
              title={t('mcp.empty')}
              description={t('mcp.emptyHint')}
              action={<div className="flex gap-2">
                <Button variant="secondary" onClick={() => setImporting({})}><Download size={15} />{t('mcp.importFromTarget')}</Button>
                <Button variant="primary" onClick={() => setEditing('')}><Plus size={15} />{t('mcp.addServer')}</Button>
              </div>}
            />
          )}

          {conflicts.length > 0 && (
            <div className="ss-note warn !items-center">
              <AlertCircle size={16} className="self-start mt-0.5" />
              <div className="flex flex-1 flex-col gap-2">
                {conflicts.map((c, i) => (
                  <div key={`${c.target}:${c.name}`} className="flex items-center gap-3">
                    <span className="flex-1">
                      {i === 0 && <b>{t(conflicts.length === 1 ? 'mcp.conflictLead.one' : 'mcp.conflictLead.other', { count: conflicts.length })} </b>}
                      {conflictText(c)}
                    </span>
                    {isResolvable(c) && <>
                      <Button size="sm" variant="secondary" disabled={busy} onClick={() => void resolve(c.target, c.name, 'import')}>{t('mcp.importFromAgent', { target: targetLabel(c.target) })}</Button>
                      <Button size="sm" variant="secondary" disabled={busy} onClick={() => void resolve(c.target, c.name, 'replace')}>{t('mcp.replace')}</Button>
                    </>}
                  </div>
                ))}
              </div>
            </div>
          )}

          {data.plan && rows.length > 0 && (
            <div className="flex items-center justify-between gap-3">
              {pending > 0 ? (
                <>
                  <span className="ss-st warn text-[13px]">{t(pending === 1 ? 'mcp.pending.one' : 'mcp.pending.other', { count: pending })}</span>
                  <Button variant="secondary" onClick={() => navigate('/sync')}>{t('mcp.reviewInSync')}<ChevronRight size={15} /></Button>
                </>
              ) : conflicts.length === 0 ? (
                <span className="ss-st ok text-[13px]">{t('mcp.allSynced')}</span>
              ) : null}
            </div>
          )}

          <section>
            <div className="ss-sec">
              <h2>{t('mcp.configFiles')}</h2>
              <span className="ss-cnt">{files.length}</span>
            </div>
            <div className="ss-list">
              {shownFiles.map((target) => (
                <div key={target} className="ss-r !min-h-10">
                  <span className="ss-at"><AgentIcon target={target} size={17} /></span>
                  <span className="min-w-0 flex-1 truncate font-mono text-[13px]" title={data.paths[target]}>{shortenHome(data.paths[target])}</span>
                  {detected.has(target) ? <span className="ss-st ok">{t('mcp.fileDetected')}</span> : <span className="ss-st off">{t('mcp.notDetected')}</span>}
                  <button type="button" className="ss-ib" aria-label={t('mcp.copyPath')} onClick={() => { copy(data.paths[target]); toast(t('mcp.copied'), 'success'); }}><Copy size={15} /></button>
                </div>
              ))}
              <div className="ss-r !min-h-[38px]">
                <span className="min-w-0 flex-1 truncate text-[13px] text-ink-3">
                  {hiddenFiles > 0 && <><button type="button" className="hover:text-ink" onClick={() => setAllFiles(true)}>{t('mcp.moreFiles', { count: hiddenFiles })}</button> · </>}
                  {t('mcp.source')}: <span className="font-mono" title={data.source.path}>{shortenHome(data.source.path)}</span>
                </span>
                <button type="button" className="ss-ib" aria-label={t('mcp.copySource')} onClick={() => { copy(data.source.path); toast(t('mcp.copied'), 'success'); }}><Copy size={15} /></button>
              </div>
            </div>
          </section>
        </div>
      )}

      {editing !== null && data && (
        <MCPServerDialog
          initial={editing ? { name: editing, server: servers[editing] } : undefined}
          defaultTargets={defaults}
          existingNames={Object.keys(servers)}
          availableTargets={files}
          onClose={() => setEditing(null)}
          onSaved={() => done(t('mcp.toast.saved'))}
        />
      )}
      {importing && data && (
        <MCPImportDialog
          servers={servers}
          defaultTargets={defaults}
          paths={data.paths}
          detected={data.detected}
          conflict={importing.conflict}
          onClose={() => setImporting(null)}
          onImported={() => { setImporting(null); refresh(); }}
        />
      )}
      {removing && <MCPRemoveDialog name={removing} onClose={() => setRemoving('')} onSaved={() => done(t('mcp.toast.removed', { name: removing }))} />}
      {backupsOpen && data && <MCPRestoreDialog backups={data.backups} onClose={() => setBackupsOpen(false)} onRestored={() => done(t('mcp.toast.restored'))} />}
      <DialogShell open={Boolean(replace)} onClose={() => setReplace(null)} padding="none" preventClose={busy} ariaLabel={t('mcp.replace')} className="!max-w-[640px]">
        {replace && <>
          <div className="dh">
            <h2 className="ss-h2">{t('mcp.replace')}</h2>
            <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={() => setReplace(null)} disabled={busy}><X size={16} /></button>
          </div>
          <div className="db">
            <MCPPreview plan={replace.plan} />
          </div>
          <div className="df">
            <span className="flex-1 text-[13px] text-ink-2">{t('mcp.backupNote')}</span>
            <Button variant="ghost" onClick={() => setReplace(null)} disabled={busy}>{t('common.cancel')}</Button>
            <Button variant="primary" loading={busy} disabled={replace.plan.blocked} onClick={applyReplace}>{t('mcp.saveSync')}</Button>
          </div>
        </>}
      </DialogShell>
      <SkillContextMenu open={!!menu} anchorPoint={menu ?? undefined} items={menu?.items ?? []} onClose={() => setMenu(null)} />
    </div>
  );
}
