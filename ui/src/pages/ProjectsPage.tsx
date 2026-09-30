import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowRight, ChevronRight, Folder, Folders, Plus } from 'lucide-react';
import { api, type ProjectList } from '../api/client';
import AgentIcon from '../components/AgentIcon';
import Button from '../components/Button';
import ConfirmDialog from '../components/ConfirmDialog';
import EmptyState from '../components/EmptyState';
import PageHeader from '../components/PageHeader';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import AddProjectDialog from '../components/projects/AddProjectDialog';
import { projectHealth, projectRows, projectUrl, type ProjectRow } from '../components/projects/projectView';
import { refreshTargets } from '../components/targets/targetView';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { shortenHome } from '../lib/paths';
import { useT } from '../i18n';
import { useAvailableTargetsQuery, useHooksQuery, useMcpQuery } from '../hooks/useSharedQueries';

const TONE = { missing: 'bad', conflict: 'warn', pending: 'warn', synced: 'ok', idle: 'off' } as const;
const STACK = 6;

export default function ProjectsPage() {
  const t = useT();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const list = useQuery({ queryKey: queryKeys.projects, queryFn: () => api.listProjects(), staleTime: staleTimes.targets });
  const targets = useQuery({ queryKey: queryKeys.targets.projects, queryFn: () => api.listTargets('projects'), staleTime: staleTimes.targets });
  const mcp = useMcpQuery();
  const hooks = useHooksQuery();
  const available = useAvailableTargetsQuery();
  const [adding, setAdding] = useState(false);
  const [converting, setConverting] = useState<ProjectList['convertible'][number] | null>(null);
  const [busy, setBusy] = useState(false);

  if (list.isPending) return <PageSkeleton />;

  const rows = projectRows(list.data, mcp.data, hooks.data);
  const convertible = list.data?.convertible ?? [];
  const common = (available.data?.targets ?? []).filter((a) => a.installed || a.detected).map((a) => a.name);

  const content = (p: ProjectRow) => {
    const mine = (targets.data?.targets ?? []).filter((tg) => tg.project === p.path);
    const skills = Math.max(0, ...mine.map((tg) => tg.expectedSkillCount));
    const agents = Math.max(0, ...mine.map((tg) => tg.agentExpectedCount ?? 0));
    const servers = Object.keys(mcp.data?.source.projects?.[p.path]?.servers ?? {}).length;
    const hookCount = Object.keys(hooks.data?.source.projects?.[p.path]?.entries ?? {}).length;
    const filtered = (p.skills?.include.length ?? 0) + (p.skills?.exclude.length ?? 0) > 0;
    return [
      p.skills && t(filtered ? (skills === 1 ? 'projects.content.skills.one' : 'projects.content.skills.other') : 'projects.content.allSkills', { count: skills }),
      p.agents && t(agents === 1 ? 'projects.content.agents.one' : 'projects.content.agents.other', { count: agents }),
      mcp.data?.source.projects?.[p.path] && (servers > 0 ? t('projects.content.mcp', { count: servers }) : 'MCP'),
      hooks.data?.source.projects?.[p.path] && (hookCount > 0 ? t('projects.content.hooks', { count: hookCount }) : 'Hooks'),
    ].filter(Boolean).join(' · ') || t('projects.content.none');
  };

  const convert = async () => {
    if (!converting) return;
    setBusy(true);
    try {
      await api.convertProject(converting.root);
      refreshTargets(queryClient);
      toast(t('projects.convert.done', { name: shortenHome(converting.root) }), 'success');
      setConverting(null);
    } catch (e) {
      toast((e as Error).message, 'error');
    } finally {
      setBusy(false);
    }
  };

  const addButton = <Button variant="primary" onClick={() => setAdding(true)}><Plus size={15} />{t('projects.add.title')}</Button>;

  return (
    <div className="animate-fade-in">
      <PageHeader title={t('projects.title')} subtitle={t('projects.subtitle')} actions={addButton} />

      {list.error ? (
        <div className="ss-note bad"><span className="flex-1">{list.error.message}</span></div>
      ) : rows.length === 0 ? (
        <EmptyState icon={Folders} title={t('projects.emptyTitle')} description={t('projects.emptyDescription')} action={addButton} />
      ) : (
        <div className="ss-list">
          <div className="ss-lh">
            <span className="w-[30px]" />
            <span className="flex-1">{t('projects.col.project')}</span>
            <span className="w-[120px]">{t('projects.targets')}</span>
            <span className="w-[270px]">{t('projects.col.content')}</span>
            <span className="w-[130px]">{t('projects.col.status')}</span>
            <span className="w-4" />
          </div>
          {rows.map((p) => {
            const { state, count } = projectHealth(p, targets.data?.targets ?? [], mcp.data, hooks.data);
            const tools = p.targets.length > 0 ? p.targets : mcp.data?.source.projects?.[p.path]?.targets ?? [];
            return (
              <Link key={p.path} to={projectUrl(p.path, p.declared ? undefined : hooks.data?.source.projects?.[p.path] && !mcp.data?.source.projects?.[p.path] ? 'hooks' : 'mcp')} className="ss-r link !min-h-[56px]">
                <span className="ss-cat target"><Folder size={16} /></span>
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="font-semibold">{p.name}</span>
                  <span className="truncate font-mono text-[12px] text-ink-3" title={p.path}>{shortenHome(p.path)}</span>
                </span>
                <span className="w-[120px] shrink-0">
                  {tools.length > 0 && (
                    <span className="ss-stack" title={tools.join(', ')}>
                      {tools.slice(0, STACK).map((tool) => <span key={tool} className="ss-at"><AgentIcon target={tool} size={13} /></span>)}
                    </span>
                  )}
                </span>
                <span className="w-[270px] shrink-0 truncate text-[13px] text-ink-2">{content(p)}</span>
                <span className="w-[130px] shrink-0"><span className={`ss-st ${TONE[state]}`}>{t(state === 'conflict' ? `projects.state.conflict.${count === 1 ? 'one' : 'other'}` : `projects.state.${state}`, { count })}</span></span>
                <ChevronRight size={16} className="shrink-0 text-ink-3" />
              </Link>
            );
          })}
        </div>
      )}

      {convertible.length > 0 && (
        <section className="mt-10">
          <div className="ss-sec">
            <h2 className="ss-h2">{t('projects.convert.heading')}</h2>
            <span className="ss-cnt">{convertible.length}</span>
          </div>
          <div className="ss-list">
            {convertible.map((c) => (
              <div key={c.root} className="ss-r !min-h-[56px]">
                <span className="ss-cat target"><Folder size={16} /></span>
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="truncate font-mono text-[13px] font-semibold" title={c.root}>{shortenHome(c.root)}</span>
                  <span className="truncate text-[12px] text-ink-3">{t('projects.convert.from', { targets: c.targets.join(', ') })}</span>
                </span>
                <span className="ss-stack" title={c.tools.join(', ')}>
                  {c.tools.slice(0, STACK).map((tool) => <span key={tool} className="ss-at"><AgentIcon target={tool} size={13} /></span>)}
                </span>
                <Button variant="secondary" size="sm" onClick={() => setConverting(c)}><ArrowRight size={14} />{t('projects.convert.button')}</Button>
              </div>
            ))}
          </div>
          <p className="mt-3 text-[13px] text-ink-3">{t('projects.convert.hint')}</p>
        </section>
      )}

      {adding && list.data && (
        <AddProjectDialog
          tools={list.data.tools}
          common={common}
          existing={rows.filter((p) => p.declared).map((p) => p.path)}
          onClose={() => setAdding(false)}
          onAdded={(root) => {
            setAdding(false);
            refreshTargets(queryClient);
            void queryClient.invalidateQueries({ queryKey: queryKeys.mcp });
            toast(t('projects.added', { name: shortenHome(root) }), 'success');
            // The server answers with the key as typed; the page is addressed by the absolute folder.
            void api.listProjects().then((fresh) => {
              const added = fresh.projects.find((p) => p.root === root);
              if (added) navigate(projectUrl(added.path));
            });
          }}
        />
      )}
      <ConfirmDialog
        open={Boolean(converting)}
        loading={busy}
        title={t(converting?.targets.length === 1 ? 'projects.convert.title.one' : 'projects.convert.title.other', { count: converting?.targets.length ?? 0 })}
        message={<>
          <p className="font-mono text-[13px]">{converting?.targets.join(', ')} → {shortenHome(converting?.root ?? '')}</p>
          <p>{t('projects.convert.keeps')}</p>
          <p>{t('projects.convert.moves')}</p>
        </>}
        confirmText={t('projects.convert.button')}
        onCancel={() => setConverting(null)}
        onConfirm={() => void convert()}
      />
    </div>
  );
}
