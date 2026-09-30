import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { AlertCircle, CircleCheck, TriangleAlert } from 'lucide-react';
import { type SyncResponse, type Target } from '../../api/client';
import { hookLabel } from '../hooks/hooksView';
import { describeMessage, targetLabel } from '../mcp/mcpView';
import Button from '../Button';
import DialogShell from '../DialogShell';
import Spinner from '../Spinner';
import SyncResultList, { SyncUpToDate } from '../SyncResultList';
import { countChanges, HOOKS_CHANGED, hooksGroups, MCP_CHANGED, mcpGroups, otherWarnings, projectChanges, resourceGroups, runSync, type ChangeGroup, type SyncFailure } from '../sync/syncView';
import SyncResult from '../sync/SyncResult';
import { refreshTargets } from '../targets/targetView';
import { useT } from '../../i18n';
import { queryKeys } from '../../lib/queryKeys';
import type { ProjectRow } from './projectView';
import { useDiffQuery, useHooksQuery, useMcpQuery } from '../../hooks/useSharedQueries';

interface Props {
  open: boolean;
  onClose: () => void;
  project: ProjectRow;
  /** Project targets; undefined while they load */
  targets: Target[] | undefined;
}

/** Previews and syncs one project: its skills, agents and MCP, nothing else. */
export default function ProjectSyncDialog({ open, onClose, project, targets }: Props) {
  const t = useT();
  const queryClient = useQueryClient();
  const diff = useDiffQuery({ enabled: open });
  const mcp = useMcpQuery({ enabled: open });
  const hooks = useHooksQuery({ enabled: open });
  const [running, setRunning] = useState(false);
  const [error, setError] = useState('');
  // undefined until the sync ran
  const [done, setDone] = useState<SyncResponse | null>();
  const [failures, setFailures] = useState<SyncFailure[]>([]);

  const mine = (targets ?? []).filter((tg) => tg.project === project.path);
  const names = new Set(mine.map((tg) => tg.name));
  const diffs = (diff.data?.diffs ?? []).filter((d) => names.has(d.target));
  const ignored = { skill: diff.data?.ignored_skills, agent: diff.data?.agent_ignored_skills };
  const plan = mcp.data?.plan;
  const changes = projectChanges(plan, project.path);
  const conflicts = changes.filter((c) => c.action === 'conflict');
  const mcpBlocked = conflicts.length > 0;
  const hookPlan = hooks.data?.plan;
  const hookChanges = (hookPlan?.changes ?? []).filter((c) => c.root === project.path);
  const hooksBlocked = hookChanges.some((c) => c.action === 'conflict');
  const sections: { label: string; groups: ChangeGroup[] }[] = [
    { label: 'Skills', groups: resourceGroups(diffs, mine, new Set(['skill']), false, ignored).groups },
    { label: 'Agents', groups: resourceGroups(diffs, mine, new Set(['agent']), false, ignored).groups },
    { label: 'MCP', groups: plan ? mcpGroups({ ...plan, changes }) : [] },
    { label: 'Hooks', groups: hookPlan ? hooksGroups({ ...hookPlan, changes: hookChanges }) : [] },
  ].filter((s) => s.groups.length > 0);
  const count = sections.reduce((n, s) => n + ((s.label === 'MCP' && mcpBlocked) || (s.label === 'Hooks' && hooksBlocked) ? 0 : countChanges(s.groups)), 0);
  const loading = diff.isPending || mcp.isPending || (hooks.isPending && !hooks.error) || !targets;

  const close = () => {
    onClose();
    setDone(undefined);
    setFailures([]);
    setError('');
  };
  const sync = async () => {
    setRunning(true);
    setError('');
    try {
      const { resources, failures: failed } = await runSync({
        resources: project.declared ? 'both' : null,
        extras: false,
        mcp: plan && !mcpBlocked && (plan.migrates || changes.some((c) => c.action !== 'unchanged')) ? plan : null,
        hooks: hookPlan && !hooksBlocked && hookChanges.some((c) => c.action !== 'unchanged') ? hookPlan : null,
        force: false,
        project: { root: project.root, path: project.path },
      });
      setDone(resources ?? null);
      setFailures(failed);
    } catch (err) {
      const message = (err as Error).message;
      setError(message === MCP_CHANGED ? t('sync.mcpChanged') : message === HOOKS_CHANGED ? t('sync.hooksChanged') : message);
    } finally {
      setRunning(false);
      refreshTargets(queryClient);
      for (const queryKey of [queryKeys.mcp, queryKeys.hooks, ['log']]) void queryClient.invalidateQueries({ queryKey });
    }
  };

  const failedTargets = new Set(failures.map((f) => f.target));
  const synced = new Set((done?.results ?? []).map((r) => r.target).filter((name) => !failedTargets.has(name))).size;

  const title = t('projects.sync.title', { name: project.name });
  return (
    <DialogShell open={open} onClose={close} maxWidth="2xl" padding="none" preventClose={running} ariaLabel={title}>
      <div className="dh">
        <div className="flex flex-col gap-1">
          <h2 className="ss-h2">{title}</h2>
          <p className="text-[13px] text-ink-2">{t('projects.sync.subtitle')}</p>
        </div>
      </div>
      <div className="db">
        {done !== undefined ? (
          <>
            {failures.length === 0 && <div className="ss-note inf"><CircleCheck size={16} /><span className="flex-1">{t('projects.sync.done', { name: project.name })}</span></div>}
            {/* No Force switch here, so a symlink conflict offers Open target only. */}
            <SyncResult failures={failures} warnings={otherWarnings(done)} synced={synced} />
          </>
        ) : (
          <>
            {error && <div className="ss-note bad"><AlertCircle size={16} /><span className="flex-1">{error}</span></div>}
            {mcpBlocked && (
              <div className="ss-note warn">
                <TriangleAlert size={16} />
                {/* The rows below count conflicts per file; say which entry and why here. */}
                <span className="flex flex-1 flex-col gap-1">
                  {t('projects.sync.mcpConflict')}
                  {conflicts.map((c) => <span key={`${c.path}:${c.name}`}><span className="font-mono">{targetLabel(c.target)} · {c.name}</span>: {describeMessage(t, c.message)}</span>)}
                </span>
              </div>
            )}
            {hooks.error && <div className="ss-note bad"><AlertCircle size={16} /><span className="flex-1">{hooks.error.message}</span></div>}
            {hooksBlocked && (
              <div className="ss-note warn">
                <TriangleAlert size={16} />
                <span className="flex flex-1 flex-col gap-1">
                  {t('sync.hooksBlocked')}
                  {hookChanges.filter((c) => c.action === 'conflict').map((c) => <span key={`${c.path}:${c.name}`}><span className="font-mono">{hookLabel(c.target)} · {c.name}</span>{c.message && `: ${c.message}`}</span>)}
                </span>
              </div>
            )}
            {loading ? (
              <div className="ss-list"><div className="ss-r gap-2 text-[13px] text-ink-2"><Spinner size="sm" />{t('sync.checking')}</div></div>
            ) : diff.error ? (
              <div className="ss-note bad"><AlertCircle size={16} /><span className="flex-1">{diff.error.message}</span></div>
            ) : sections.length === 0 ? (
              <SyncUpToDate text={t('sync.nothing')} />
            ) : (
              // One row per target, as the Skills sync dialog shows it; the Sync page lists each item.
              sections.map((s) => (
                <section key={s.label} className="flex flex-col gap-2">
                  <h3 className="text-[13px] font-semibold">{s.label}</h3>
                  <SyncResultList groups={s.groups} inSync={[]} />
                </section>
              ))
            )}
          </>
        )}
      </div>
      <div className="df">
        {done !== undefined || (!loading && !diff.error && sections.length === 0) ? (
          <Button variant="primary" onClick={close}>{t('projects.sync.close')}</Button>
        ) : (
          <>
            <Button variant="secondary" onClick={close} disabled={running}>{t('common.cancel')}</Button>
            <Button variant="primary" onClick={() => void sync()} loading={running} disabled={loading || count === 0}>
              {t('syncPreview.syncNowButton')}
            </Button>
          </>
        )}
        {/* Last in DOM so the dialog's initial focus lands on a button; order-first keeps it on the left. */}
        <Link to="/sync" className="ss-more order-first mr-auto" onClick={close}>{t('projects.sync.openPage')}</Link>
      </div>
    </DialogShell>
  );
}
