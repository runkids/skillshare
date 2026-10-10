import { useEffect, useState } from 'react';
import { AlertCircle, CircleCheck, RefreshCw, TriangleAlert } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';

import { api, type UnmatchedInclude } from '../api/client';
import Button from './Button';
import DialogShell from './DialogShell';
import Spinner from './Spinner';
import SyncResultList, { SyncUpToDate } from './SyncResultList';
import UnmatchedNotices from './sync/UnmatchedNotices';
import { countChanges, resourceGroups, type Part } from './sync/syncView';
import { useT } from '../i18n';
import { useDiffQuery, useSyncedTargetsQuery } from '../hooks/useSharedQueries';
import { invalidate } from '../lib/queryEvents';

interface SyncPreviewModalProps {
  open: boolean;
  onClose: () => void;
  /** Syncs only this kind and says so; unset syncs skills and agents together. */
  kind?: 'skill' | 'agent';
  /** Extra classes for the dialog, e.g. a slide-in when it replaces another dialog. */
  className?: string;
}

/**
 * Previews from /api/diff, the same data as the Sync page and the pending dots, so the counts agree:
 * a dry-run reports every existing link as linked again.
 */
export default function SyncPreviewModal({ open, onClose, kind, className }: SyncPreviewModalProps) {
  const t = useT();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const diff = useDiffQuery({ enabled: open });
  const targets = useSyncedTargetsQuery({ enabled: open });

  const [syncing, setSyncing] = useState(false);
  const [synced, setSynced] = useState(false);
  const [warnings, setWarnings] = useState<string[]>([]);
  const [unmatched, setUnmatched] = useState<UnmatchedInclude[]>([]);
  const [error, setError] = useState<string | null>(null);

  // Fresh numbers on every open; the cached diff may predate an install or uninstall.
  useEffect(() => {
    setError(null);
    setWarnings([]);
    setUnmatched([]);
    setSynced(false);
    if (open) void diff.refetch();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const handleSync = async () => {
    setSyncing(true);
    setError(null);
    try {
      const res = await api.sync({ dryRun: false, kind });
      setWarnings(res.warnings ?? []);
      setUnmatched(res.unmatched ?? []);
      setSynced(true);
      void invalidate(queryClient, 'synced');
    } catch (e: unknown) {
      setError((e as Error).message);
    } finally {
      setSyncing(false);
    }
  };

  const list = targets.data?.targets ?? [];
  const parts = new Set<Part>(kind ? [kind] : ['skill', 'agent']);
  const ignored = { skill: diff.data?.ignored_skills, agent: diff.data?.agent_ignored_skills };
  const { groups, inSync } = resourceGroups(diff.data?.diffs ?? [], list, parts, false, ignored);
  // Targets without an agents folder never get agents, so they are not "up to date" for them.
  const upToDate = kind === 'agent' ? inSync.filter((n) => list.find((tg) => tg.name === n)?.agentPath) : inSync;
  const loading = diff.isFetching || targets.isPending;
  const loadError = error ?? diff.error?.message ?? targets.error?.message;
  const count = countChanges(groups);
  const noTargets = !loading && list.length === 0;
  const canSync = !loading && !loadError && count > 0;

  const title = synced ? t('syncPreview.titleComplete') : kind ? t(`syncPreview.title.${kind}`) : t('syncPreview.titlePreview');
  return (
    <DialogShell open={open} onClose={onClose} maxWidth="2xl" padding="none" preventClose={syncing} ariaLabel={title} className={className}>
      <div className="dh">
        <div className="flex flex-col gap-1">
          <h2 className="ss-h2">{title}</h2>
          {kind && !synced && <p className="text-[13px] text-ink-2">{t(`syncPreview.scope.${kind}`)}</p>}
        </div>
        {!loading && !synced && (
          <button type="button" className="ss-ib" onClick={() => void diff.refetch()} title={t('syncPreview.refreshPreview')} aria-label={t('syncPreview.refreshPreview')}>
            <RefreshCw size={16} />
          </button>
        )}
      </div>

      <div className="db">
        {synced ? (
          <>
            <div className="ss-note inf"><CircleCheck size={16} /><span className="flex-1">{t('syncPreview.completed')}</span></div>
            {warnings.map((w) => <div key={w} className="ss-note warn"><TriangleAlert size={16} /><span className="flex-1">{w}</span></div>)}
            <UnmatchedNotices items={unmatched} />
          </>
        ) : loading ? (
          <div className="ss-list"><div className="ss-r gap-2 text-[13px] text-ink-2"><Spinner size="sm" />{t('sync.checking')}</div></div>
        ) : loadError ? (
          <div className="ss-note bad">
            <AlertCircle size={16} />
            <span className="flex-1">{loadError}</span>
            <Button variant="secondary" size="sm" onClick={() => { setError(null); void diff.refetch(); }}>{t('syncPreview.retryButton')}</Button>
          </div>
        ) : noTargets ? (
          <SyncUpToDate text={t('syncPreview.noTargets')} />
        ) : groups.length === 0 ? (
          <SyncUpToDate text={t('syncPreview.allUpToDate')} />
        ) : (
          // One row per target can outgrow the dialog; scroll the list so the buttons stay reachable
          <SyncResultList groups={groups} inSync={upToDate} className="max-h-[50vh] !overflow-y-auto" />
        )}
      </div>

      <div className="df">
        {synced || (!loading && !loadError && !canSync) ? (
          <Button variant="primary" onClick={onClose}>{t('syncPreview.closeButton')}</Button>
        ) : (
          <>
            <Button variant="secondary" onClick={onClose} disabled={syncing}>{t('syncPreview.cancelButton')}</Button>
            {canSync && <Button onClick={handleSync} loading={syncing}>{t('syncPreview.syncNowButton')}</Button>}
          </>
        )}
        {/* Last in DOM so the dialog's initial focus lands on a button; order-first keeps it on the left. */}
        {kind && <button type="button" className="ss-more order-first mr-auto" onClick={() => { onClose(); navigate('/sync'); }} disabled={syncing}>{t('syncPreview.openSyncPage')}</button>}
      </div>
    </DialogShell>
  );
}
