import { useState } from 'react';
import { AlertCircle, Archive, ChevronRight, CircleCheck, ListChecks, RefreshCw } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import type { MCPPlan } from '../../api/mcp';
import Button from '../Button';
import DialogShell from '../DialogShell';
import MCPNotices from './MCPNotices';
import { RailLine, SyncBox } from '../StatusRail';
import SyncResultList from '../SyncResultList';
import { MCP_CHANGED, mcpGroups, runSync } from '../sync/syncView';
import { useT } from '../../i18n';
import { queryKeys } from '../../lib/queryKeys';
import { shortenHome } from '../../lib/paths';
import { offListFor, projectOf, targetLabel, writes, type MCPChange } from './mcpView';

function ChangeLines({ changes, roots }: { changes: MCPChange[]; roots: string[] }) {
  const t = useT();
  return (
    <div className="flex flex-col gap-1.5">
      {changes.map((c) => {
        const root = projectOf(roots, c);
        return <RailLine key={`${c.path}:${root ?? ''}:${c.target}:${c.name}`} name={c.name} agent={root ? `${targetLabel(c.target)} · ${shortenHome(root)}` : targetLabel(c.target)} word={t(`mcp.status.${c.action}`)} />;
      })}
    </div>
  );
}

/**
 * Confirm, then write the whole MCP plan, laid out like the Skills sync dialog: a row per Agent file.
 * The plan is global, so a project's box still syncs every project.
 */
export function MCPSyncDialog({ plan, shown, onClose }: { plan: MCPPlan; shown: number; onClose: () => void }) {
  const t = useT();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [running, setRunning] = useState(false);
  const [error, setError] = useState('');
  const [done, setDone] = useState(false);
  const pending = plan.changes.filter(writes);
  // A file under an mcp.projects root is named by its project, as the Claude off list already is.
  // The plan's root says which, as projectOf reads it: the path alone is ambiguous under nested roots.
  const groups = mcpGroups({ ...plan, changes: pending }).map((g) => ({ ...g, project: g.project ?? pending.find((c) => c.path === g.path && !offListFor(c))?.root }));
  // Agents that get servers but have nothing to write fold into the closing line.
  const inSync = [...new Set(plan.changes.map((c) => c.target))].filter((x) => plan.changes.every((c) => c.target !== x || c.action === 'unchanged')).map(targetLabel);
  const outside = pending.length - shown;
  const title = t('mcp.syncButton');

  const sync = async () => {
    setRunning(true);
    setError('');
    try {
      await runSync({ resources: null, extras: false, mcp: plan, force: false });
      setDone(true);
    } catch (e) {
      const message = (e as Error).message;
      setError(message === MCP_CHANGED ? t('sync.mcpChanged') : message);
    } finally {
      setRunning(false);
      for (const queryKey of [queryKeys.mcp, ['log']]) void queryClient.invalidateQueries({ queryKey });
    }
  };

  return (
    <DialogShell open onClose={onClose} padding="none" maxWidth="2xl" preventClose={running} ariaLabel={title}>
      <div className="dh">
        <div className="flex flex-col gap-1">
          <h2 className="ss-h2">{done ? t('syncPreview.titleComplete') : title}</h2>
          {!done && <p className="text-[13px] text-ink-2">{t('mcp.syncDialog.subtitle')}</p>}
        </div>
      </div>
      <div className="db">
        {done ? (
          <div className="ss-note inf"><CircleCheck size={16} /><span className="flex-1">{t('mcp.syncDialog.done')}</span></div>
        ) : (
          <>
            <MCPNotices notices={plan.notices} />
            {/* One row per Agent file can outgrow the dialog; scroll the list so the buttons stay reachable. */}
            <SyncResultList groups={groups} inSync={inSync} className="max-h-[50vh] !overflow-y-auto" />
            {outside > 0 && <p className="text-[13px] text-ink-2">{t(outside === 1 ? 'mcp.syncDialog.outside.one' : 'mcp.syncDialog.outside.other', { count: outside })}</p>}
          </>
        )}
        {error && <div className="ss-note bad"><AlertCircle size={16} /><span className="flex-1">{error}</span></div>}
      </div>
      <div className="df">
        {done ? (
          <Button variant="primary" onClick={onClose}>{t('syncPreview.closeButton')}</Button>
        ) : (
          <>
            <Button variant="secondary" onClick={onClose} disabled={running}>{t('syncPreview.cancelButton')}</Button>
            {/* A moved plan is stale: close and reopen from the refreshed box instead of retrying it. */}
            {!error && <Button onClick={() => void sync()} loading={running}>{t('syncPreview.syncNowButton')}</Button>}
          </>
        )}
        <button type="button" className="ss-more order-first mr-auto" onClick={() => { onClose(); navigate('/sync'); }} disabled={running}>{t('syncPreview.openSyncPage')}</button>
      </div>
    </DialogShell>
  );
}

export interface MCPRailActionsProps {
  /** Offered only while there are servers to check. */
  check?: { running: boolean; run: () => unknown };
  /** Offered only when backups exist. */
  onBackups?: () => void;
}

/** The page's quiet upkeep actions, kept with the sync state rather than in the page header. */
export function MCPRailActions({ check, onBackups }: MCPRailActionsProps) {
  const t = useT();
  if (!check && !onBackups) return null;
  return (
    <div className="flex flex-col items-start gap-0.5">
      {check && <Button className="flush" size="sm" variant="ghost" loading={check.running} onClick={() => void check.run()}>{!check.running && <ListChecks size={14} />}{t(check.running ? 'mcp.check.running' : 'mcp.check.button')}</Button>}
      {onBackups && <Button className="flush" size="sm" variant="ghost" onClick={onBackups}><Archive size={14} />{t('mcp.backups')}</Button>}
    </div>
  );
}

/** What Sync would write. A change inside a project names the folder, since one server can land in several. */
export default function MCPSyncBox({ changes, roots, plan, check, onBackups }: { changes: MCPChange[]; roots: string[]; plan: MCPPlan } & MCPRailActionsProps) {
  const t = useT();
  const navigate = useNavigate();
  // Held while the dialog is open, so the list it confirms stays put when the queries refresh.
  const [reviewing, setReviewing] = useState<{ plan: MCPPlan; shown: number } | null>(null);
  const pending = changes.filter(writes);
  return (
    <SyncBox tone={pending.length > 0 ? 'warn' : 'ok'} state={pending.length > 0 ? t(pending.length === 1 ? 'mcp.pending.one' : 'mcp.pending.other', { count: pending.length }) : t('targets.state.synced')}>
      {pending.length > 0 && (
        <>
          <ChangeLines changes={pending} roots={roots} />
          {/* A blocked plan applies nothing; the Sync page shows why. */}
          {plan.blocked
            ? <Button className="w-full justify-center" onClick={() => navigate('/sync')}>{t('mcp.reviewInSync')}<ChevronRight size={15} /></Button>
            : <Button className="w-full justify-center" onClick={() => setReviewing({ plan, shown: pending.length })}><RefreshCw size={15} />{t('mcp.syncButton')}</Button>}
        </>
      )}
      {/* Ticks only change the source; say where the files get written, next to the state. */}
      <p className={pending.length > 0 ? 'text-xs leading-normal text-ink-2' : 'text-[13px] leading-normal text-ink-2'}>{t('mcp.syncHint')}</p>
      {pending.length === 0 && <button type="button" className="ss-more self-start" onClick={() => navigate('/sync')}>{t('mcp.reviewInSync')}</button>}
      {(check || onBackups) && <>
        <div className="[border-top:var(--sep)]" />
        <MCPRailActions check={check} onBackups={onBackups} />
      </>}
      {reviewing && <MCPSyncDialog {...reviewing} onClose={() => setReviewing(null)} />}
    </SyncBox>
  );
}
