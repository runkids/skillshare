import { useCallback, useEffect, useState } from 'react';
import { AlertTriangle, CheckCircle, RefreshCw } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';

import type { SyncResult } from '../api/client';

import { api } from '../api/client';
import { invalidateAfterSync } from '../lib/sync';
import Button from './Button';
import DialogShell from './DialogShell';
import Spinner from './Spinner';
import SyncResultList from './SyncResultList';
import { useT } from '../i18n';

interface SyncPreviewModalProps {
  open: boolean;
  onClose: () => void;
}

export default function SyncPreviewModal({ open, onClose }: SyncPreviewModalProps) {
  const t = useT();
  const queryClient = useQueryClient();

  const [loading, setLoading] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [synced, setSynced] = useState(false);
  const [results, setResults] = useState<SyncResult[] | null>(null);
  const [warnings, setWarnings] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);

  const runDryRun = useCallback(async () => {
    setLoading(true);
    setError(null);
    setWarnings([]);
    try {
      const res = await api.sync({ dryRun: true });
      setResults(res.results);
      setWarnings(res.warnings ?? []);
    } catch (e: unknown) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, []);

  const handleSync = async () => {
    setSyncing(true);
    try {
      const res = await api.sync({ dryRun: false });
      setResults(res.results);
      setSynced(true);
      invalidateAfterSync(queryClient);
    } catch (e: unknown) {
      setError((e as Error).message);
    } finally {
      setSyncing(false);
    }
  };

  // Clear stale data on open/close; auto-run dry-run when opening
  useEffect(() => {
    setResults(null);
    setError(null);
    setWarnings([]);
    setSynced(false);
    if (open) {
      runDryRun();
    }
  }, [open, runDryRun]);

  const allUpToDate =
    results !== null &&
    results.every(
      (r) =>
        (r.linked?.length ?? 0) === 0 &&
        (r.updated?.length ?? 0) === 0 &&
        (r.pruned?.length ?? 0) === 0 &&
        !r.dir_created,
    );

  const noTargets = results !== null && results.length === 0;

  return (
    <DialogShell open={open} onClose={onClose} maxWidth="2xl" preventClose={syncing}>
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-bold text-pencil">{synced ? t('syncPreview.titleComplete') : t('syncPreview.titlePreview')}</h2>
            {results !== null && !loading && !synced && (
              <button
                onClick={runDryRun}
                className="text-pencil-light hover:text-pencil transition-colors"
                title={t('syncPreview.refreshPreview')}
              >
                <RefreshCw size={16} />
              </button>
            )}
          </div>

          {synced && (
            <div className="flex items-center gap-2 rounded-lg border border-success/30 bg-success/5 px-3 py-2 text-sm font-medium text-success">
              <CheckCircle size={16} className="shrink-0" />
              {t('syncPreview.completed')}
            </div>
          )}

          {warnings.length > 0 && (
            <div className="flex items-start gap-2 rounded-lg border border-warning bg-warning-light px-3 py-2 text-sm text-pencil">
              <AlertTriangle size={16} className="mt-0.5 shrink-0 text-warning" />
              <div className="space-y-1">
                {warnings.map((w, i) => <p key={i}>{w}</p>)}
              </div>
            </div>
          )}

          {loading && (
            <div className="flex items-center justify-center py-8">
              <Spinner />
              <span className="ml-3 text-pencil-light">{t('syncPreview.dryRunning')}</span>
            </div>
          )}

          {error && (
            <div className="text-center py-4 space-y-3">
              <p className="text-danger text-sm">{error}</p>
              <Button variant="secondary" size="sm" onClick={runDryRun}>
                {t('syncPreview.retryButton')}
              </Button>
            </div>
          )}

          {!loading && !error && noTargets && (
            <p className="text-pencil-light text-center py-4">
              {t('syncPreview.noTargets')}
            </p>
          )}

          {!loading && !error && allUpToDate && !noTargets && (
            <p className="text-pencil-light text-center py-4">
              {t('syncPreview.allUpToDate')}
            </p>
          )}

          {!loading && !error && results && !allUpToDate && !noTargets && (
            <SyncResultList results={results} />
          )}

          <div className="flex justify-end gap-3 pt-2">
            {synced ? (
              <Button variant="primary" onClick={onClose}>
                {t('syncPreview.closeButton')}
              </Button>
            ) : (
              <>
                <Button variant="secondary" onClick={onClose} disabled={syncing}>
                  {t('syncPreview.cancelButton')}
                </Button>
                {!allUpToDate && !noTargets && results && !error && (
                  <Button onClick={handleSync} loading={syncing}>
                    {t('syncPreview.syncNowButton')}
                  </Button>
                )}
              </>
            )}
          </div>
        </div>
    </DialogShell>
  );
}
