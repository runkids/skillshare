import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ChevronRight, Plus, Target as TargetIcon } from 'lucide-react';
import { api, type Target } from '../api/client';
import { isPiTarget } from '../api/piExtensions';
import { isOmpTarget } from '../api/ompExtensions';
import AgentIcon from '../components/AgentIcon';
import Button from '../components/Button';
import EmptyState from '../components/EmptyState';
import PageHeader from '../components/PageHeader';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import AddTargetDialog from '../components/targets/AddTargetDialog';
import { mcpClient, serverCount } from '../components/mcp/mcpView';
import { hookAgentOf, hookCount } from '../components/hooks/hooksView';
import { refreshTargets, targetHealth, type TargetState } from '../components/targets/targetView';
import { useAppContext } from '../context/AppContext';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { fileName, shortenHome } from '../lib/paths';
import { useT } from '../i18n';
import { useAvailableTargetsQuery, useHooksQuery, useMcpQuery } from '../hooks/useSharedQueries';

const TONE = { synced: 'ok', pending: 'warn', missing: 'warn', migrate: 'warn', problem: 'bad', unknown: 'off' } as const;
const PROBLEM_TEXT: Record<string, string> = { 'not exist': 'targets.syncing.missing', conflict: 'targets.syncing.conflict', broken: 'targets.syncing.broken' };

/** One door into a target page: what the target gets, and the tab that manages it. */
interface Door { key: string; label: string; count?: number; to: string; title?: string }

export default function TargetsPage() {
  const t = useT();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const { isProjectMode } = useAppContext();
  const { data, isPending, error } = useQuery({ queryKey: queryKeys.targets.all, queryFn: () => api.listTargets(), staleTime: staleTimes.targets });
  const available = useAvailableTargetsQuery();
  const mcp = useMcpQuery();
  const hooks = useHooksQuery();
  // Which instruction file each target reads, and whether a shared AGENTS.md is connected to it. Global mode only: in a project every target reads ./AGENTS.md.
  const shared = useQuery({ queryKey: queryKeys.instructions.shared, queryFn: () => api.listSharedInstructions(), enabled: !isProjectMode });
  const [adding, setAdding] = useState<{ initial?: string } | null>(null);

  // The API walks a map, so the order changes between requests.
  const targets = [...(data?.targets ?? [])].sort((a, b) => a.name.localeCompare(b.name));
  const known = available.data?.targets ?? [];
  const found = known.filter((a) => a.detected && !a.installed).sort((a, b) => a.name.localeCompare(b.name));

  const syncing = (tg: Target, state: TargetState, pending: number) => {
    if (PROBLEM_TEXT[tg.status]) return t(PROBLEM_TEXT[tg.status]);
    if (state === 'migrate') return t('targets.syncing.migrate');
    if (tg.mode === 'symlink') return t('targets.syncing.folderLinked');
    return [
      // "0 linked" beside "In sync" reads as a contradiction when the source simply has nothing for this target.
      tg.linkedCount === 0 && pending === 0 ? t('targets.syncing.nothing') : t(tg.mode === 'copy' ? 'targets.syncing.copied' : 'targets.syncing.linked', { count: tg.linkedCount }),
      pending > 0 && t('targets.syncing.pending', { count: pending }),
      tg.localCount > 0 && t('targets.syncing.local', { count: tg.localCount }),
    ].filter(Boolean).join(' · ');
  };
  // Each thing a target gets is a door straight into the tab that manages it. MCP files have their own sync, on the MCP page.
  // Hooks follow the Agent, not the skills switch: a target with skills off still shows its hooks.
  const doors = (tg: Target, state: TargetState, pending: number): Door[] => {
    const base = `/targets/${encodeURIComponent(tg.name)}`;
    const servers = mcp.data ? serverCount(mcp.data, mcpClient(tg.name)) : 0;
    const agent = hookAgentOf(tg.name, tg.agent);
    const hookN = hooks.data && agent ? hookCount(hooks.data, agent, tg.project) : 0;
    const file = shared.data?.targets.find((s) => s.name === tg.name && !s.rider_of);
    const out: Door[] = [];
    if (tg.skillsEnabled !== false) out.push({ key: 'skills', label: 'Skills', count: tg.linkedCount, to: base, title: syncing(tg, state, pending) });
    if (tg.agentPath && tg.agentLinkedCount) out.push({ key: 'agents', label: 'Agents', count: tg.agentLinkedCount, to: `${base}?tab=agents` });
    if (servers > 0) out.push({ key: 'mcp', label: 'MCP', count: servers, to: `${base}?tab=mcp` });
    if (hookN > 0) out.push({ key: 'hooks', label: 'Hooks', count: hookN, to: `${base}?tab=hooks` });
    if (isPiTarget(tg) || isOmpTarget(tg)) out.push({ key: 'extensions', label: 'Extensions', to: `${base}?tab=extensions` });
    // Only a connected shared file is worth a door; a target's own file is the page's business.
    if (file && file.assigned.length > 0) {
      out.push({
        key: 'instructions', label: fileName(file.path), to: `${base}?tab=instructions`,
        title: t('targets.doors.shared', { name: file.assigned.map((a) => a.name).join(', ') }),
      });
    }
    return out;
  };
  const addButton = <Button variant="primary" onClick={() => setAdding({})}><Plus size={15} />{t('targets.addTarget')}</Button>;

  return (
    <div className="animate-fade-in">
      <PageHeader title={t('targets.title')} subtitle={t('targets.subtitle')} actions={<span data-tour="targets-grid">{addButton}</span>} />

      {isPending ? (
        <PageSkeleton />
      ) : error ? (
        <div className="ss-note bad"><span className="flex-1">{error.message}</span></div>
      ) : targets.length === 0 ? (
        <EmptyState icon={TargetIcon} title={t('targets.emptyTitle')} description={t('targets.emptyDescription')} action={addButton} />
      ) : (
        <div className="ss-list">
          <div className="ss-lh">
            <span className="w-[30px]" />
            <span className="flex-1">{t('targets.col.target')}</span>
            <span className="w-[92px]">{t('targets.col.mode')}</span>
            <span className="w-[340px]">{t('targets.col.syncing')}</span>
            <span className="w-[110px]">{t('targets.col.status')}</span>
            <span className="w-4" />
          </div>
          {targets.map((tg) => {
            const { state, pending } = targetHealth(tg);
            return (
              // The name opens the page; each door opens the tab for that one thing, so a row can't be a single link.
              <div key={tg.name} className="ss-r !min-h-[56px]">
                <Link to={`/targets/${encodeURIComponent(tg.name)}`} className="ss-rl -mx-4 -my-2 flex min-w-0 flex-1 items-center gap-3 self-stretch px-4 py-2" aria-label={tg.name}>
                  <span className="ss-at"><AgentIcon target={tg.name} size={17} /></span>
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span className="flex items-center gap-2">
                      <span className="font-semibold">{tg.name}</span>
                      {tg.name === 'universal' && <span className="ss-tag">{t('targets.sharedPath')}</span>}
                      {tg.agentExtension && <span className="ss-tag">extension: {tg.agentExtension}</span>}
                    </span>
                    <span className="truncate font-mono text-[12px] text-ink-3" title={tg.path}>{shortenHome(tg.path)}</span>
                  </span>
                </Link>
                <span className="w-[92px] shrink-0">
                  {tg.skillsEnabled === false ? <span className="ss-tag !text-ink-3">{t('targets.skillsOff')}</span> : <span className="ss-tag">{tg.mode}</span>}
                </span>
                <span className="flex w-[340px] shrink-0 flex-wrap items-center gap-0.5 -ml-2">
                  {doors(tg, state, pending).map((d) => (
                    <Link key={d.key} to={d.to} className="ss-door" title={d.title}>
                      {d.label}{d.count !== undefined && <b>{d.count}</b>}
                    </Link>
                  ))}
                </span>
                <span className="w-[110px] shrink-0">
                  <span className={`ss-st ${TONE[state]}`}>
                    {state === 'pending' ? t('targets.state.pending', { count: pending }) : state === 'unknown' ? tg.status : t(`targets.state.${state}`)}
                  </span>
                </span>
                <ChevronRight size={16} className="shrink-0 text-ink-3" />
              </div>
            );
          })}
        </div>
      )}

      {found.length > 0 && (
        <section className="mt-10">
          <div className="ss-sec">
            <h2 className="ss-h2">{t('targets.foundOnMachine')}</h2>
            <span className="ss-cnt">{found.length}</span>
          </div>
          <div className="ss-list">
            {found.map((a) => (
              <div key={a.name} className="ss-r">
                <span className="ss-at"><AgentIcon target={a.name} size={17} /></span>
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="font-semibold">{a.name}</span>
                  <span className="truncate font-mono text-[12px] text-ink-3">{shortenHome(a.path)}</span>
                </span>
                <Button variant="secondary" size="sm" onClick={() => setAdding({ initial: a.name })} aria-label={t('targets.add.addNamed', { name: a.name })}>
                  <Plus size={14} />{t('targets.add.button')}
                </Button>
              </div>
            ))}
          </div>
          <p className="mt-3 text-[13px] text-ink-3">{t('targets.foundHint', { count: known.length })}</p>
        </section>
      )}

      {adding && (
        <AddTargetDialog
          available={known}
          initial={adding.initial}
          existing={targets.map((tg) => tg.name)}
          targets={targets}
          onClose={() => setAdding(null)}
          onAdded={(name) => {
            setAdding(null);
            refreshTargets(queryClient);
            toast(t('targets.targetAdded', { name }), 'success');
          }}
        />
      )}
    </div>
  );
}
