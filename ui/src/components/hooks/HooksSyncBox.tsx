import { useState } from 'react';
import { Link } from 'react-router-dom';
import { AlertCircle, CircleCheck, RefreshCw } from 'lucide-react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { hooksApi, type HookChange, type HookEntry, type HookMutation, type HookPlan } from '../../api/hooks';
import { useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import { queryKeys } from '../../lib/queryKeys';
import Button from '../Button';
import DialogShell from '../DialogShell';
import Spinner from '../Spinner';
import { RailLine, SyncBox } from '../StatusRail';
import HooksPreview from './HooksPreview';
import { actionLabel, fileName, hookLabel, rootPlan, writes } from './hooksView';

/**
 * Confirm, then write. The dialog previews afresh and applies exactly that plan, so a source or native
 * file that changed since the page loaded is refused, not overwritten.
 */
export function HooksSyncDialog({ project, takeover, canTakeOver, onTakeover, onClose }: {
  project?: string;
  /** Replace this hook's conflicting native outputs. That plan applies as a whole, since `replace` is not kept in the source. */
  takeover?: { name: string; entry: HookEntry };
  /** Offered on the conflicts of a plain sync: opens the take-over preview for that hook instead. */
  canTakeOver?: (c: HookChange) => boolean;
  onTakeover?: (name: string) => void;
  onClose: () => void;
}) {
  const t = useT();
  const cache = useQueryClient();
  const mutation: HookMutation = takeover ? { ...(project && { project }), name: takeover.name, entry: takeover.entry, replace: true } : {};
  const { data: plan, error: previewError, isPending } = useQuery({ queryKey: ['hooks-sync-preview', project ?? '', takeover?.name ?? ''], queryFn: () => hooksApi.preview(mutation), gcTime: 0, retry: false });
  // A project sees and is blocked by only its own root, takeover included; the revision is still the whole plan's.
  const view = plan && rootPlan(plan, project);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState('');
  const [done, setDone] = useState(false);

  const sync = async () => {
    if (!plan) return;
    setRunning(true);
    setError('');
    try {
      // A project writes only its own root; the global scope writes the whole plan, as the MCP box does.
      if (project && !takeover) await hooksApi.syncProject(project, plan.revision);
      else await hooksApi.configure(mutation, plan.revision, true);
      setDone(true);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setRunning(false);
      for (const queryKey of [queryKeys.hooks, ['log']]) void cache.invalidateQueries({ queryKey });
    }
  };
  const title = takeover ? t('hooks.takeoverTitle', { name: takeover.name }) : t('hooks.syncButton');
  return (
    <DialogShell open onClose={onClose} padding="none" maxWidth="2xl" preventClose={running} ariaLabel={title}>
      <div className="dh">
        <div className="flex flex-col gap-1">
          <h2 className="ss-h2">{done ? t('syncPreview.titleComplete') : title}</h2>
          {!done && <p className="text-[13px] text-ink-2">{takeover ? t('hooks.takeoverMessage') : t('hooks.syncDialog.subtitle')}</p>}
        </div>
      </div>
      <div className="db">
        {done ? <div className="ss-note inf"><CircleCheck size={16} /><span className="flex-1">{t('hooks.syncDialog.done')}</span></div>
          : isPending ? <Spinner size="sm" />
          : view && <HooksPreview plan={view} canTakeOver={takeover ? undefined : canTakeOver} onTakeover={takeover ? undefined : onTakeover} />}
        {(previewError || error) && <div className="ss-note bad" role="alert"><AlertCircle size={16} /><span className="flex-1">{previewError?.message ?? error}</span></div>}
      </div>
      <div className="df">
        {done ? <Button variant="primary" onClick={onClose}>{t('syncPreview.closeButton')}</Button> : (
          <>
            <Button variant="secondary" onClick={onClose} disabled={running}>{t('syncPreview.cancelButton')}</Button>
            {/* A refused write means the plan moved: close and reopen for a fresh one rather than retry it. */}
            {!error && <Button onClick={() => void sync()} loading={running} disabled={!view || view.blocked}>{t('syncPreview.syncNowButton')}</Button>}
          </>
        )}
      </div>
    </DialogShell>
  );
}

/** What Sync would write. This is Skillshare's side only: whether an Agent trusts and loads it is that Agent's call. */
export default function HooksSyncBox({ plan, project, canTakeOver, onTakeover }: { plan: HookPlan; project?: string; canTakeOver?: (c: HookChange) => boolean; onTakeover?: (name: string) => void }) {
  const t = useT();
  const [reviewing, setReviewing] = useState(false);
  const pending = plan.changes.filter(writes);
  const conflicts = plan.changes.filter((c) => c.action === 'conflict');
  const state = pending.length > 0 ? t(pending.length === 1 ? 'mcp.pending.one' : 'mcp.pending.other', { count: pending.length }) : conflicts.length > 0 ? t('mcp.status.conflict') : t('targets.state.synced');
  return (
    <SyncBox tone={pending.length > 0 || conflicts.length > 0 ? 'warn' : 'ok'} state={state}>
      {pending.length > 0 && (
        <div className="flex flex-col gap-1.5">
          {pending.map((c) => <RailLine key={`${c.path}:${c.target}:${c.name}`} name={c.name} agent={[hookLabel(c.target), c.root && !project && shortenHome(c.root), fileName(c.path)].filter(Boolean).join(' · ')} word={actionLabel(t, c.action)} />)}
        </div>
      )}
      {conflicts.length > 0 && (
        <div className="flex flex-col gap-2">
          {conflicts.map((c) => (
            <div key={`${c.path}:${c.target}:${c.name}`} className="flex flex-col gap-0.5 text-xs">
              <span className="flex items-center gap-1.5"><AlertCircle size={13} className="shrink-0 text-warn" /><span className="truncate font-mono font-semibold" title={c.name}>{c.name}</span><span className="text-ink-2">{hookLabel(c.target)}</span></span>
              <span className="truncate font-mono text-ink-3" title={c.path}>{shortenHome(c.path)}</span>
              {c.root && !project && <Link to={`/projects/${encodeURIComponent(c.root)}?tab=hooks`} className="w-fit font-semibold text-ink-2 hover:text-ink">{shortenHome(c.root)}</Link>}
              {onTakeover && canTakeOver?.(c) && <button type="button" className="w-fit font-semibold text-ink-2 hover:text-ink" aria-label={`${t('hooks.takeoverMenu')} · ${c.name}`} onClick={() => onTakeover(c.name)}>{t('hooks.takeoverMenu')}</button>}
            </div>
          ))}
        </div>
      )}
      {(pending.length > 0 || conflicts.length > 0) && (
        <>
          {/* A blocked plan stays reviewable; the dialog is what refuses to write it. */}
          <Button className="w-full justify-center" variant={plan.blocked ? 'secondary' : undefined} onClick={() => setReviewing(true)}>{plan.blocked ? <AlertCircle size={15} /> : <RefreshCw size={15} />}{t(plan.blocked ? 'hooks.viewConflicts' : 'hooks.syncButton')}</Button>
          {plan.blocked && <p className="text-xs leading-normal text-warn">{t('hooks.blockedHint')}</p>}
        </>
      )}
      <p className="text-xs leading-normal text-ink-2">{t('hooks.syncHint')}</p>
      {reviewing && <HooksSyncDialog project={project} canTakeOver={canTakeOver} onTakeover={onTakeover && ((name) => { setReviewing(false); onTakeover(name); })} onClose={() => setReviewing(false)} />}
    </SyncBox>
  );
}
