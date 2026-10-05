import { useEffect, useState } from 'react';
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import { Folder, Folders, Plug, RefreshCw, Webhook } from 'lucide-react';
import { hooksApi, type HookInventory } from '../api/hooks';
import { api, type ProjectList, type ProjectResource } from '../api/client';
import { mcpApi, mcpTargets } from '../api/mcp';
import AgentIcon from '../components/AgentIcon';
import Button from '../components/Button';
import ConfirmDialog from '../components/ConfirmDialog';
import EmptyState from '../components/EmptyState';
import PageHeader from '../components/PageHeader';
import { PageSkeleton } from '../components/Skeleton';
import SegmentedControl from '../components/SegmentedControl';
import { useToast } from '../components/Toast';
import HooksScope from '../components/hooks/HooksScope';
import MCPProjectView from '../components/mcp/MCPProjectView';
import { targetLabel } from '../components/mcp/mcpView';
import ProjectSyncDialog from '../components/projects/ProjectSyncDialog';
import ProjectTools from '../components/projects/ProjectTools';
import TargetOmpExtensions from '../components/targets/TargetOmpExtensions';
import TargetPiExtensions from '../components/targets/TargetPiExtensions';
import { projectHealth, projectRows, toolGroups, type ProjectRow } from '../components/projects/projectView';
import FilterSection, { ModePicker } from '../components/targets/FilterSection';
import { refreshTargets } from '../components/targets/targetView';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { shortenHome } from '../lib/paths';
import { useT } from '../i18n';
import { useAvailableTargetsQuery, useHooksQuery, useMcpQuery } from '../hooks/useSharedQueries';

type MCPList = Awaited<ReturnType<typeof mcpApi.list>>;
type Tab = 'skills' | 'agents' | 'mcp' | 'hooks' | 'extensions';
const TABS: Tab[] = ['skills', 'agents', 'mcp', 'hooks', 'extensions'];
const LABEL = { skills: 'Skills', agents: 'Agents', mcp: 'MCP', hooks: 'Hooks', extensions: 'Extensions' };
const EVERYTHING: ProjectResource = { mode: 'merge', include: [], exclude: [] };

export default function ProjectDetailPage() {
  const { root = '' } = useParams();
  const t = useT();
  const list = useQuery({ queryKey: queryKeys.projects, queryFn: () => api.listProjects(), staleTime: staleTimes.targets });
  const mcp = useMcpQuery();
  const hooks = useHooksQuery();
  const project = projectRows(list.data, mcp.data, hooks.data).find((p) => p.path === root);

  if (list.isPending || mcp.isPending || hooks.isPending) return <PageSkeleton />;
  if (list.error) return <div className="ss-note bad"><span className="flex-1">{list.error.message}</span></div>;
  if (!project || !list.data) {
    return (
      <EmptyState
        icon={Folders}
        title={t('projects.notFound', { name: shortenHome(root) })}
        action={<Link to="/projects"><Button variant="secondary">{t('projects.back')}</Button></Link>}
      />
    );
  }
  return <ProjectEditor key={root} project={project} tools={list.data.tools} mcp={mcp.data} hooks={hooks.data} hooksError={hooks.error?.message} />;
}

function ProjectEditor({ project, tools, mcp, hooks, hooksError }: { project: ProjectRow; tools: ProjectList['tools']; mcp: MCPList | undefined; hooks: HookInventory | undefined; hooksError?: string }) {
  const t = useT();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const [params] = useSearchParams();
  const targets = useQuery({ queryKey: queryKeys.targets.projects, queryFn: () => api.listTargets('projects'), staleTime: staleTimes.targets });
  // Declared tools do not always generate a target (for example, an agents-only Pi project).
  const hasTarget = (tool: string) => project.targets.includes(tool) && Boolean(targets.data?.targets.some((target) => target.name === `${project.name}@${tool}`));
  // Pi and Oh My Pi both switch their extensions here. With both, the tab picks one.
  const extensionTools = (['pi', 'omp'] as const).filter(hasTarget);
  const [extensionTool, setExtensionTool] = useState<'pi' | 'omp'>('pi');
  const shownExtensionTool = extensionTools.includes(extensionTool) ? extensionTool : extensionTools[0];
  const shownTabs = TABS.filter((x) => x !== 'extensions' || extensionTools.length > 0);
  const tab = shownTabs.find((x) => x === params.get('tab')) ?? 'skills';
  const available = useAvailableTargetsQuery();
  const common = (available.data?.targets ?? []).filter((a) => a.installed || a.detected).map((a) => a.name);

  const saved = { targets: project.targets, skills: project.skills, agents: project.agents };
  const [draft, setDraft] = useState(saved);
  const [saving, setSaving] = useState(false);
  const [removing, setRemoving] = useState(false);
  const [busy, setBusy] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const dirty = JSON.stringify(draft) !== JSON.stringify(saved);
  const canSave = dirty && draft.targets.length > 0 && Boolean(draft.skills || draft.agents);

  const agent = tab === 'agents';
  const resource = agent ? draft.agents : draft.skills;
  const setResource = (next: ProjectResource | null) => setDraft(agent ? { ...draft, agents: next } : { ...draft, skills: next });
  const agentTools = draft.targets.filter((x) => tools.find((tool) => tool.name === x)?.agentsPath);
  // ponytail: one tool stands in for the project. A skill whose `targets:` names another tool can differ per folder.
  const previewTool = (agent ? agentTools[0] : undefined) ?? draft.targets[0];

  const [filters, setFilters] = useState(draft);
  useEffect(() => {
    const id = setTimeout(() => setFilters(draft), 400);
    return () => clearTimeout(id);
  }, [draft]);
  const preview = useQuery({
    queryKey: ['sync-matrix-preview', project.name, previewTool, filters.skills, filters.agents],
    queryFn: () => api.previewSyncMatrix(`${project.name}@${previewTool}`, filters.skills?.include ?? [], filters.skills?.exclude ?? [], filters.agents?.include ?? [], filters.agents?.exclude ?? []),
    placeholderData: keepPreviousData,
    enabled: Boolean(previewTool) && tab !== 'mcp' && tab !== 'hooks' && tab !== 'extensions',
  });
  const entries = (preview.data?.entries ?? []).filter((e) => (e.kind === 'agent') === agent && e.status !== 'na');

  const refresh = () => {
    refreshTargets(queryClient);
    void queryClient.invalidateQueries({ queryKey: queryKeys.mcp });
    void queryClient.invalidateQueries({ queryKey: queryKeys.hooks });
  };
  const save = async () => {
    setSaving(true);
    try {
      await api.saveProject({ root: project.root, ...draft, create: !project.declared });
      refresh();
      toast(t('projects.saved', { name: project.name }), 'success');
    } catch (e) {
      toast((e as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  };
  const mcpEntry = mcp?.source.projects?.[project.path];
  const hooksEntry = hooks?.source.projects?.[project.path];
  const remove = async () => {
    setBusy(true);
    try {
      if (project.declared) await api.removeProject(project.root);
      if (mcpEntry) await mcpApi.save({ project: project.path, remove: true });
      if (hooksEntry) await hooksApi.save({ project: project.path, remove: true });
      refresh();
      toast(t('projects.removed', { name: project.name }), 'success');
      navigate('/projects');
    } catch (e) {
      toast((e as Error).message, 'error');
      setBusy(false);
    }
  };
  const manageHooks = async () => {
    setBusy(true);
    try {
      await hooksApi.save({ project: project.path });
      refresh();
    } catch (e) {
      toast((e as Error).message, 'error');
    } finally {
      setBusy(false);
    }
  };
  const manageMCP = async () => {
    setBusy(true);
    try {
      const clients = mcpTargets.filter((x) => project.targets.includes(x));
      await mcpApi.save({ project: project.path, settings: { targets: clients.length > 0 ? clients : undefined } });
      refresh();
    } catch (e) {
      toast((e as Error).message, 'error');
    } finally {
      setBusy(false);
    }
  };

  const tabs = (
    <nav className="ss-tabs" aria-label={project.name}>
      {shownTabs.map((x) => (
        <Link key={x} to={x === 'skills' ? '?' : `?tab=${x}`} replace className={tab === x ? 'on' : ''} aria-current={tab === x}>{LABEL[x]}</Link>
      ))}
    </nav>
  );
  const health = projectHealth(project, targets.data?.targets ?? [], mcp, hooks);
  const folders = agent
    ? draft.targets.map((tool) => ({ tools: [tool], path: tools.find((x) => x.name === tool)?.agentsPath ?? '' }))
    : toolGroups(tools, draft.targets).map((g) => ({ tools: g.tools, path: g.skillsPath }));

  return (
    <div className="animate-fade-in">
      <PageHeader
        crumbs={[{ label: t('projects.title'), to: '/projects' }, { label: project.name }]}
        title={project.name}
        subtitle={<span className="font-mono">{shortenHome(project.path)}</span>}
        actions={
          <>
            <Button variant="secondary" onClick={() => setSyncing(true)} disabled={project.missing}>
              <RefreshCw size={16} />
              {t('projects.sync.button')}
              {health.state === 'pending' && <span className="size-2 rounded-full bg-warn" role="img" aria-label={t(health.count === 1 ? 'projects.note.pending.one' : 'projects.note.pending.other', { count: health.count })} />}
            </Button>
            {tab === 'skills' && previewTool && (
              <Link to={`/skills?tab=analyze&target=${encodeURIComponent(`${project.name}@${previewTool}`)}`} className="ss-btn ghost">{t('analyze.open')}</Link>
            )}
            <Button variant="ghost" onClick={() => setRemoving(true)}>{t('projects.remove')}</Button>
            {tab !== 'mcp' && tab !== 'hooks' && tab !== 'extensions' && <Button variant="primary" onClick={save} loading={saving} disabled={!canSave}>{t('common.save')}</Button>}
          </>
        }
      />

      <div className="mb-6 flex flex-col gap-3 empty:hidden">
        {project.missing && <div className="ss-note bad"><span className="flex-1">{t('projects.note.missing')}</span></div>}
        {project.hasOwnConfig && <div className="ss-note warn"><span className="flex-1">{t('projects.note.ownConfig')}</span></div>}
      </div>

      {/* The Hooks tab carries its own actions, so it draws this bar itself with them at the right. */}
      {!(tab === 'hooks' && !hooksError && hooks && hooksEntry) && <div className="mb-7">{tabs}</div>}

      {tab === 'extensions' ? (
        <div className="flex flex-col gap-5">
          {extensionTools.length > 1 && (
            <SegmentedControl value={shownExtensionTool ?? 'pi'} onChange={setExtensionTool} options={[{ value: 'pi', label: 'Pi' }, { value: 'omp', label: 'Oh My Pi' }]} className="self-start" />
          )}
          {shownExtensionTool === 'omp' ? <TargetOmpExtensions name={`${project.name}@omp`} /> : <TargetPiExtensions name={`${project.name}@pi`} />}
        </div>
      ) : tab === 'hooks' ? (
        hooksError ? (
          <div className="ss-note bad"><span className="flex-1">{hooksError}</span></div>
        ) : hooks && hooksEntry ? (
          <HooksScope data={hooks} project={project.path} header={(actions) => <div className="ss-tabbar mb-7 flex-wrap">{tabs}{actions}</div>} />
        ) : (
          <EmptyState icon={Webhook} title={t('projects.hooks.emptyTitle')} description={t('projects.hooks.emptyDescription')} action={<Button variant="primary" onClick={() => void manageHooks()} loading={busy}>{t('projects.hooks.manage')}</Button>} />
        )
      ) : tab === 'mcp' ? (
        mcp && mcpEntry ? (
          <MCPProjectView data={mcp} root={project.path} offered={mcpTargets.filter((x) => mcp.paths[x])} onChanged={refresh} onRemoved={project.declared ? refresh : () => { refresh(); navigate('/projects'); }} />
        ) : (
          <EmptyState icon={Plug} title={t('projects.mcp.emptyTitle')} description={t('projects.mcp.emptyDescription')} action={<Button variant="primary" onClick={() => void manageMCP()} loading={busy}>{t('projects.mcp.manage')}</Button>} />
        )
      ) : (
        <div className="grid grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)] items-start gap-12">
          <section className="flex flex-col gap-5">
            <div className="flex items-center gap-4">
              <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                <h2 className="ss-h2" id="project-part">{t(`projects.part.${tab}`)}</h2>
                <span className="text-[13px] text-ink-2">{t(`projects.part.${tab}Hint`)}</span>
              </div>
              <button type="button" role="switch" aria-checked={Boolean(resource)} aria-labelledby="project-part" className={`ss-sw ${resource ? 'on' : ''} disabled:opacity-50`} disabled={saving} onClick={() => setResource(resource ? null : (agent ? saved.agents : saved.skills) ?? EVERYTHING)}><i /></button>
            </div>
            {resource && previewTool && (
              <FilterSection kind={agent ? 'agent' : 'skill'} mode={resource.mode || 'merge'} name={project.name} include={resource.include} exclude={resource.exclude} onChange={(next) => setResource({ ...resource, ...next })} entries={entries} loaded={Boolean(preview.data)} loading={preview.isPending} error={preview.error} disabled={saving} />
            )}
          </section>

          <aside className="flex flex-col gap-7">
            <div className="flex flex-col gap-3">
              <h2 className="ss-h2">{t('projects.targets')}</h2>
              <ProjectTools tools={tools} common={common} selected={draft.targets} onChange={(next) => setDraft({ ...draft, targets: next })} disabled={saving} />
            </div>
            {resource && folders.length > 0 && (
              <div className="flex flex-col gap-3">
                <h2 className="ss-h2">{t('projects.writes')}</h2>
                <div className="ss-list">
                  {folders.map((f) => (
                    <div key={f.tools.join()} className={`ss-r ${f.path ? '' : 'opacity-55'}`}>
                      <span className="ss-stack shrink-0">{f.tools.map((tool) => <span key={tool} className="ss-at" title={tool}><AgentIcon target={tool} size={13} /></span>)}</span>
                      {f.path
                        ? <span className="flex min-w-0 flex-1 items-center gap-2 font-mono text-[13px]"><Folder size={14} className="shrink-0 text-ink-3" /><span className="truncate">{f.path}</span></span>
                        : <span className="min-w-0 flex-1 text-[13px] text-ink-2">{t('projects.writes.noAgents', { name: targetLabel(f.tools[0]) })}</span>}
                      {f.tools.length > 1 && <span className="ss-tag" title={f.tools.join(', ')}>{t('projects.writes.shared')}</span>}
                    </div>
                  ))}
                </div>
              </div>
            )}
            {resource && (
              <div className="flex flex-col gap-3">
                <h2 className="ss-h2">{t('targetDetail.syncMode')}</h2>
                <ModePicker kind={agent ? 'agent' : 'skill'} mode={resource.mode || 'merge'} onChange={(mode) => setResource({ ...resource, mode })} disabled={saving} />
              </div>
            )}
          </aside>
        </div>
      )}

      <ProjectSyncDialog open={syncing} onClose={() => setSyncing(false)} project={project} targets={targets.data?.targets} />

      <ConfirmDialog
        open={removing}
        loading={busy}
        variant="danger"
        title={t('projects.removeTitle', { name: project.name })}
        message={<p>{t('projects.removeMessage')}</p>}
        confirmText={t('projects.remove')}
        onCancel={() => setRemoving(false)}
        onConfirm={() => void remove()}
      />
    </div>
  );
}
