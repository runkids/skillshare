import { useMemo, useState } from 'react';
import { Navigate, useSearchParams } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { AlertCircle, ChevronDown, Copy, Download, Eye, Pencil, Plug, Plus, PowerOff, Trash2, X } from 'lucide-react';
import { mcpApi, type MCPMutation, type MCPPlan, type MCPSettings } from '../api/mcp';
import Button from '../components/Button';
import { useAppContext } from '../context/AppContext';
import DialogShell from '../components/DialogShell';
import EmptyState from '../components/EmptyState';
import PageHeader from '../components/PageHeader';
import { PageSkeleton } from '../components/Skeleton';
import { RailGroup, RailLayout, RailRow, RailSection } from '../components/StatusRail';
import { SkillContextMenu, type ContextMenuItem } from '../components/TargetMenu';
import { useToast } from '../components/Toast';
import MCPDefaults from '../components/mcp/MCPDefaults';
import MCPImportDialog from '../components/mcp/MCPImportDialog';
import MCPUnmanagedNote from '../components/mcp/MCPUnmanagedNote';
import MCPCheckNote from '../components/mcp/MCPCheckNote';
import { problemsByServer, useMCPCheck } from '../components/mcp/useMCPCheck';
import { projectUrl } from '../components/projects/projectView';
import MCPSyncBox, { MCPRailActions } from '../components/mcp/MCPSyncBox';
import MCPServerList from '../components/mcp/MCPServerList';
import MCPPreview, { type MCPResolve } from '../components/mcp/MCPPreview';
import { MCPConfigDialog } from '../components/mcp/MCPConfigView';
import MCPRemoveDialog from '../components/mcp/MCPRemoveDialog';
import MCPRestoreDialog from '../components/mcp/MCPRestoreDialog';
import MCPServerDialog from '../components/mcp/MCPServerDialog';
import { buildMatrix, canImportConflict, describeError, describeMessage, isShadowed, isResolvable, mcpOrder, projectOf, targetLabel, writes, type MCPChange } from '../components/mcp/mcpView';
import { MCPTargetOrder } from '../components/mcp/targetOrder';
import { useMCPToggle } from '../components/mcp/useMCPToggle';
import { useT } from '../i18n';
import { shortenHome } from '../lib/paths';
import { queryKeys } from '../lib/queryKeys';
import { useMcpQuery } from '../hooks/useSharedQueries';

const copy = (text: string) => void navigator.clipboard?.writeText(text);
const inGlobalScope = (change: MCPChange) => !change.root;

function mcpPageModel(data: MCPList | undefined, order: readonly string[]) {
  const servers = data?.source.servers ?? {};
  const defaults = data?.source.targets ?? [];
  const targetsOf = (name: string) => servers[name]?.targets ?? defaults;
  const changes = data?.plan?.changes ?? [];
  // mcp.projects puts a server of the same name into other folders; this list is the global
  // files only. A project's off switch for Claude Code lands in a global path, so ask the
  // change which scope it came from rather than trusting the path alone.
  const rows = data ? buildMatrix(servers, data.plan && { ...data.plan, changes: changes.filter(inGlobalScope) }) : [];
  // The plan still covers every project's files, so the sync box counts them.
  const roots = Object.keys(data?.source.projects ?? {});
  const conflicts = changes.filter((c) => c.action === 'conflict');
  const detected = new Set(data?.detected);
  const files = order.filter((x) => data?.paths[x]);
  const matrixTargets = new Set([...files, ...rows.flatMap((row) => [...targetsOf(row.name), ...Object.keys(row.cells)])]);
  const undetected = files.filter((x) => !detected.has(x));
  const showSync = Boolean(data?.plan && (rows.length > 0 || roots.length > 0) && (changes.some(writes) || conflicts.length === 0));
  return { servers, defaults, targetsOf, changes, rows, roots, conflicts, detected, files, matrixTargets, undetected, showSync };
}

type MCPList = Awaited<ReturnType<typeof mcpApi.list>>;
type PageModel = ReturnType<typeof mcpPageModel>;
type MCPCheck = ReturnType<typeof useMCPCheck>;
type ImportRequest = { conflict?: { target: string; name: string }; from?: string; project?: string };

// Check and Backups live in the sync card, and the off switch with the server list: the header keeps the two ways to add.
function MCPHeader({ onImport, onAdd }: { onImport: () => void; onAdd: () => void }) {
  const t = useT();
  return (
    <PageHeader
      title="MCP"
      subtitle={t('mcp.subtitle')}
      actions={<span className="flex items-center gap-2.5" data-tour="mcp-actions">
        <Button variant="secondary" onClick={onImport}><Download size={15} />{t('mcp.importFromTarget')}</Button>
        <Button variant="primary" onClick={onAdd}><Plus size={15} />{t('mcp.addServer')}</Button>
      </span>}
    />
  );
}

function MCPFilesRail({ data, model, allFiles, onShowAll }: { data: MCPList; model: PageModel; allFiles: boolean; onShowAll: () => void }) {
  const t = useT();
  const { toast } = useToast();
  const { files, undetected, detected } = model;
  return (
    <RailSection title={t('layout.nav.agents')} count={files.length}>
      {/* The file name is enough to recognise; the full path is one hover or one copy away. */}
      <RailGroup label={t('mcp.fileDetected')} count={files.length - undetected.length}>
        {files.filter((x) => detected.has(x)).map((target) => (
          <RailRow key={target} target={target} label={targetLabel(target)} right={<>
            <span className="max-w-[130px] truncate font-mono text-xs text-ink-3" title={data.paths[target]}>{data.paths[target].split(/[\\/]/).pop()}</span>
            <button type="button" className="ss-ib" aria-label={`${t('mcp.copyPath')} · ${targetLabel(target)}`} onClick={() => { copy(data.paths[target]); toast(t('mcp.copied'), 'success'); }}><Copy size={14} /></button>
          </>} />
        ))}
      </RailGroup>
      {undetected.length > 0 && (
        <RailGroup label={t('mcp.notDetected')} count={undetected.length} right={
          <button type="button" className="ss-ib !h-6 !w-6" aria-expanded={allFiles} aria-label={t('mcp.moreFiles', { count: undetected.length })} onClick={() => onShowAll()}><ChevronDown size={14} className={allFiles ? 'rotate-180' : ''} /></button>
        }>
          {allFiles && <div className="grid grid-cols-2 gap-x-3 gap-y-1.5 pt-0.5 text-[13px] text-ink-2">{undetected.map((target) => <span key={target} className="truncate" title={data.paths[target]}>{targetLabel(target)}</span>)}</div>}
        </RailGroup>
      )}
    </RailSection>
  );
}

interface ContentProps {
  data: MCPList;
  model: PageModel;
  order: readonly string[];
  allFiles: boolean;
  onShowAll: () => void;
  busy: boolean;
  onToggle: (name: string, target: string, on: boolean) => void;
  onMenu: (e: React.MouseEvent<HTMLButtonElement>, name: string) => void;
  onImport: (request: ImportRequest) => void;
  onAdd: () => void;
  onSettings: (settings: MCPSettings) => void;
  resolve: MCPResolve;
  check: MCPCheck;
  isProjectMode: boolean;
  onOff: () => void;
  onBackups: () => void;
}

function MCPContent({ data, model, order, allFiles, onShowAll, busy, onToggle, onMenu, onImport, onAdd, onSettings, resolve, check, isProjectMode, onOff, onBackups }: ContentProps) {
  const t = useT();
  const { toast } = useToast();
  const { rows, roots, changes, conflicts, servers, defaults, matrixTargets, files, targetsOf, showSync } = model;
  const conflictText = (c: MCPChange) => {
    const params = { target: targetLabel(c.target), name: c.name };
    if (c.message?.startsWith('existing entry is not managed')) return t('mcp.conflict.unmanaged', params);
    if (c.message?.startsWith('Agent configuration changed')) return t('mcp.conflict.changed', params);
    return `${params.target} · ${c.name}: ${describeMessage(t, c.message)}`;
  };
  const actions = { check: Object.keys(servers).length > 0 ? check : undefined, onBackups: data.backups.length > 0 ? onBackups : undefined };

  return (
    <RailLayout rail={<>
      {showSync && data.plan
        ? <MCPSyncBox changes={changes} roots={roots} plan={data.plan} {...actions} />
        : (actions.check || actions.onBackups) && <div className="ss-box"><MCPRailActions {...actions} /></div>}

      <MCPFilesRail data={data} model={model} allFiles={allFiles} onShowAll={onShowAll} />
    </>}>
      {changes.filter(isShadowed).map((c) => (
        <div key={`${c.target}:${c.name}`} className="ss-note warn">
          <AlertCircle size={16} />
          <span className="flex-1">{targetLabel(c.target)} · <span className="font-mono">{c.name}</span>: {describeMessage(t, c.message)}</span>
        </div>
      ))}
      {conflicts.length > 0 && (
        <div className="ss-note warn !items-center">
          <AlertCircle size={16} className="self-start mt-0.5" />
          <div className="flex flex-1 flex-col gap-2">
            {conflicts.map((c, i) => (
              <div key={`${c.path}:${c.target}:${c.name}`} className="flex items-center gap-3">
                <span className="flex-1">
                  {i === 0 && <b>{t(conflicts.length === 1 ? 'mcp.conflictLead.one' : 'mcp.conflictLead.other', { count: conflicts.length })} </b>}
                  {conflictText(c)}
                  {/* One server can conflict in several folders, so a project's row says which. */}
                  {projectOf(roots, c) && <span className="text-ink-2"> · {shortenHome(projectOf(roots, c)!)}</span>}
                </span>
                {isResolvable(c) && <>
                  {canImportConflict(c) && <Button size="sm" variant="secondary" disabled={busy} onClick={() => void resolve(c.target, c.name, 'import', projectOf(roots, c))}>{t('mcp.importFromAgent', { target: targetLabel(c.target) })}</Button>}
                  <Button size="sm" variant="secondary" disabled={busy} onClick={() => void resolve(c.target, c.name, 'replace')}>{t('mcp.replace')}</Button>
                </>}
              </div>
            ))}
          </div>
        </div>
      )}
      {rows.length > 0 && <MCPCheckNote report={check.report} checkedAt={check.checkedAt} error={check.error} running={check.running} onRun={() => void check.run()} />}
      <MCPUnmanagedNote entries={data.unmanaged.filter((u) => !u.project)} onImport={(from) => onImport({ from })} />
      {(rows.length > 0 || isProjectMode) && (
        <div className="ss-sec !mb-0 !items-center"><h2>{t('mcp.serverList')}</h2><span className="ss-cnt">{rows.length}</span>
          {/* Only a project file can turn off a server the Agent defines globally. */}
          {isProjectMode && <Button className="ml-auto" size="sm" variant="ghost" onClick={onOff}><PowerOff size={14} />{t('mcp.addOff')}</Button>}
        </div>
      )}
      {rows.length > 0 ? (
        <MCPServerList rows={rows} targets={order.filter((x) => matrixTargets.has(x))} targetsOf={targetsOf} onToggle={onToggle} onMenu={onMenu} disabled={busy} problems={problemsByServer(check.report)} />
      ) : (
        <EmptyState
          icon={Plug}
          title={t('mcp.empty')}
          description={t('mcp.emptyHint')}
          action={<div className="flex gap-2">
            <Button variant="secondary" onClick={() => onImport({})}><Download size={15} />{t('mcp.importFromTarget')}</Button>
            <Button variant="primary" onClick={onAdd}><Plus size={15} />{t('mcp.addServer')}</Button>
          </div>}
        />
      )}
      <MCPDefaults targets={defaults} offered={files} onSave={onSettings} />
      <div className="flex items-center gap-1 px-1 text-xs text-ink-3">
        <span className="min-w-0 truncate">{t('mcp.source')}: <span className="font-mono" title={data.source.path}>{shortenHome(data.source.path)}</span></span>
        <button type="button" className="ss-ib" aria-label={t('mcp.copySource')} onClick={() => { copy(data.source.path); toast(t('mcp.copied'), 'success'); }}><Copy size={14} /></button>
      </div>
    </RailLayout>
  );
}

function MCPEditDialog({ data, model, editing, addingOff, addMode, onMode, onClose, onSaved }: { data: MCPList; model: PageModel; editing: string; addingOff: boolean; addMode: 'form' | 'paste'; onMode: (mode: 'form' | 'paste') => void; onClose: () => void; onSaved: () => void }) {
  const { servers, defaults, files } = model;
  return (
    editing === '' && addMode === 'paste' ? (
      <MCPImportDialog
        source="paste"
        servers={servers}
        defaultTargets={defaults}
        paths={data.paths}
        detected={data.detected}
        onMode={onMode}
        onClose={onClose}
        onImported={onSaved}
      />
    ) : (
      <MCPServerDialog
        off={editing === '' && addingOff}
        initial={editing ? { name: editing, server: servers[editing] } : undefined}
        defaultTargets={defaults}
        existingNames={Object.keys(servers)}
        availableTargets={files}
        onMode={editing === '' ? onMode : undefined}
        onClose={onClose}
        onSaved={onSaved}
      />
    )
  );
}

function importContext(data: MCPList, model: PageModel, importing: ImportRequest) {
  const { servers, defaults, roots } = model;
  const defaultPath = importing.conflict ? data.plan?.changes.find((c) => c.target === importing.conflict?.target && c.name === importing.conflict?.name && projectOf(roots, c) === importing.project)?.path : data.unmanaged.find((u) => u.target === importing.from && u.project === importing.project)?.path;
  const importServers = importing.project ? data.source.projects?.[importing.project]?.servers ?? {} : servers;
  const defaultTargets = importing.project ? data.source.projects?.[importing.project]?.targets ?? defaults : defaults;
  return { defaultPath, servers: importServers, defaultTargets };
}

function MCPAgentImport({ data, model, importing, onClose, onImported }: { data: MCPList; model: PageModel; importing: ImportRequest; onClose: () => void; onImported: () => void }) {
  const context = importContext(data, model, importing);
  return (
    <MCPImportDialog
      source="target"
      // A project's conflict is read from and imported into that project, not the global source.
      project={importing.project}
      importSources={data.importSources?.[importing.project ?? '']}
      defaultPath={context.defaultPath}
      servers={context.servers}
      defaultTargets={context.defaultTargets}
      paths={data.paths}
      detected={data.detected}
      conflict={importing.conflict}
      defaultFrom={importing.from}
      onClose={onClose}
      onImported={onImported}
    />
  );
}

function MCPPageErrors({ error, previewError }: { error: Error | null; previewError?: string }) {
  const t = useT();
  return <>
      {error && <div className="ss-note bad mb-4"><span className="flex-1">{error.message}</span></div>}
      {previewError && <div className="ss-note bad mb-4"><AlertCircle size={16} /><span className="flex-1">{describeError(t, previewError)}</span></div>}
  </>;
}

function legacyMCPPath(params: URLSearchParams) {
  const opened = params.get('project');
  if (opened) return projectUrl(opened, 'mcp');
  return params.get('tab') === 'projects' ? '/projects' : null;
}

function MCPReplaceDialog({ replace, busy, onClose, onApply }: { replace: { plan: MCPPlan; mutation: MCPMutation } | null; busy: boolean; onClose: () => void; onApply: () => Promise<void> }) {
  const t = useT();
  return (
    <DialogShell open={Boolean(replace)} onClose={onClose} padding="none" preventClose={busy} ariaLabel={t('mcp.replace')} className="!max-w-[640px]">
      {replace && <>
        <div className="dh">
          <h2 className="ss-h2">{t('mcp.replace')}</h2>
          <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={busy}><X size={16} /></button>
        </div>
        <div className="db">
          <MCPPreview plan={replace.plan} />
        </div>
        <div className="df">
          <span className="flex-1 text-[13px] text-ink-2">{t('mcp.backupNote')}</span>
          <Button variant="ghost" onClick={onClose} disabled={busy}>{t('common.cancel')}</Button>
          <Button variant="primary" loading={busy} disabled={replace.plan.blocked} onClick={onApply}>{t('mcp.saveSync')}</Button>
        </div>
      </>}
    </DialogShell>
  );
}

export default function MCPPage() {
  const t = useT();
  const { toast } = useToast();
  const cache = useQueryClient();
  const [params] = useSearchParams();
  const { data, error, isPending } = useMcpQuery();
  const { isProjectMode } = useAppContext();
  const [addingOff, setAddingOff] = useState(false); // the new entry is a switch, not a server
  const [editing, setEditing] = useState<string | null>(null); // '' adds a new server
  // Adding takes two shapes: fill the fields, or paste a snippet. Both end up saving one source server.
  const [addMode, setAddMode] = useState<'form' | 'paste'>('form');
  const [importing, setImporting] = useState<ImportRequest | null>(null);
  const [removing, setRemoving] = useState('');
  const [viewing, setViewing] = useState('');
  const [backupsOpen, setBackupsOpen] = useState(false);
  const [replace, setReplace] = useState<{ plan: MCPPlan; mutation: MCPMutation } | null>(null);
  const [busy, setBusy] = useState(false);
  const [menu, setMenu] = useState<{ x: number; y: number; items: ContextMenuItem[] } | null>(null);
  const [allFiles, setAllFiles] = useState(false);
  const check = useMCPCheck();

  const refresh = () => {
    void cache.invalidateQueries({ queryKey: queryKeys.mcp });
    void cache.invalidateQueries({ queryKey: queryKeys.config });
  };
  const done = (message: string) => {
    setEditing(null); setAddMode('form'); setImporting(null); setRemoving(''); setBackupsOpen(false); setReplace(null);
    refresh();
    toast(message, 'success');
  };

  const order = useMemo(() => mcpOrder(data?.source.accounts), [data?.source.accounts]);
  const toggleTarget = useMCPToggle(order);

  if (isPending) return <PageSkeleton />;

  const model = mcpPageModel(data, order);
  const { servers, targetsOf } = model;

  const toggle = (name: string, target: string, on: boolean) => {
    setBusy(true);
    void toggleTarget(name, target, on).finally(() => setBusy(false));
  };

  const saveSettings = async (settings: MCPSettings) => {
    try {
      await mcpApi.save({ settings, replace: true });
    } catch (e) {
      toast(describeError(t, (e as Error).message), 'error');
    }
    refresh();
  };

  const resolve: MCPResolve = async (target, name, action, project) => {
    if (action === 'import') {
      setImporting({ conflict: { target, name }, project });
      return;
    }
    const mutation: MCPMutation = { resolutions: [{ target, name, action: 'replace' }] };
    setBusy(true);
    try {
      setReplace({ plan: await mcpApi.preview(mutation), mutation });
    } catch (e) {
      toast(describeError(t, (e as Error).message), 'error');
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
      toast(describeError(t, (e as Error).message), 'error');
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
        ...(targetsOf(name).length > 0 ? [{ key: 'view', label: t('mcp.viewConfig'), icon: <Eye size={14} />, onSelect: () => setViewing(name) }] : []),
        { key: 'remove', label: t('mcp.remove'), icon: <Trash2 size={14} />, danger: true, onSelect: () => setRemoving(name) },
      ],
    });
  };

  // Projects moved to their own page; links to the old tab still land somewhere useful.
  const redirected = legacyMCPPath(params);
  if (redirected) return <Navigate to={redirected} replace />;

  return (
    <MCPTargetOrder.Provider value={order}>
    <div className="animate-fade-in">
      <MCPHeader onImport={() => setImporting({})} onAdd={() => { setAddingOff(false); setAddMode('form'); setEditing(''); }} />

      <MCPPageErrors error={error} previewError={data?.previewError} />

      {data && (
        <MCPContent data={data} model={model} order={order} allFiles={allFiles} onShowAll={() => setAllFiles(!allFiles)} busy={busy} onToggle={(n, x, on) => void toggle(n, x, on)} onMenu={openMenu} onImport={setImporting} onAdd={() => { setAddingOff(false); setAddMode('form'); setEditing(''); }} onSettings={(settings) => void saveSettings(settings)} resolve={resolve} check={check} isProjectMode={isProjectMode} onOff={() => { setAddingOff(true); setAddMode('form'); setEditing(''); }} onBackups={() => setBackupsOpen(true)} />
      )}

      {editing !== null && data && (<MCPEditDialog data={data} model={model} editing={editing} addingOff={addingOff} addMode={addMode} onMode={setAddMode} onClose={() => setEditing(null)} onSaved={() => done(t('mcp.toast.saved'))} />
      )}
      {importing && data && (
        <MCPAgentImport data={data} model={model} importing={importing} onClose={() => setImporting(null)} onImported={() => { setImporting(null); refresh(); }} />
      )}
      {viewing && servers[viewing] && <MCPConfigDialog mutation={{ name: viewing, server: { ...servers[viewing], targets: order.filter((x) => targetsOf(viewing).includes(x)) } }} onClose={() => setViewing('')} />}
      {removing && <MCPRemoveDialog name={removing} inScope={inGlobalScope} onClose={() => setRemoving('')} onSaved={() => done(t('mcp.toast.removed', { name: removing }))} />}
      {backupsOpen && data && <MCPRestoreDialog backups={data.backups} onClose={() => setBackupsOpen(false)} onRestored={() => done(t('mcp.toast.restored'))} />}
      <MCPReplaceDialog replace={replace} busy={busy} onClose={() => setReplace(null)} onApply={applyReplace} />
      <SkillContextMenu open={!!menu} anchorPoint={menu ?? undefined} items={menu?.items ?? []} onClose={() => setMenu(null)} />
    </div>
    </MCPTargetOrder.Provider>
  );
}
