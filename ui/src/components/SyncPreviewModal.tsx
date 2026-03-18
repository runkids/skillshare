import { useCallback, useEffect, useState } from 'react';
import { RefreshCw } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';

import type { SyncResult } from '../api/client';

import { api } from '../api/client';
import { formatSyncToast, invalidateAfterSync } from '../lib/sync';
import { useToast } from './Toast';
import Button from './Button';
import Card from './Card';
import DialogShell from './DialogShell';
import Spinner from './Spinner';
import SyncResultList from './SyncResultList';

interface SyncPreviewModalProps {
  open: boolean;
  onClose: () => void;
}

export default function SyncPreviewModal({ open, onClose }: SyncPreviewModalProps) {
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const [loading, setLoading] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [results, setResults] = useState<SyncResult[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const runDryRun = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await api.sync({ dryRun: true });
      setResults(res.results);
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
      toast(formatSyncToast(res.results), 'success');
      invalidateAfterSync(queryClient);
      onClose();
    } catch (e: unknown) {
      setError((e as Error).message);
    } finally {
      setSyncing(false);
    }
  };

  // Auto-run dry-run when modal opens; clear stale data on close
  useEffect(() => {
    if (open) {
      setResults(null);
      setError(null);
      runDryRun();
    } else {
      setResults(null);
      setError(null);
    }
  }, [open, runDryRun]);

  const allUpToDate =
    results !== null &&
    results.every(
      (r) =>
        (r.linked?.length ?? 0) === 0 &&
        (r.updated?.length ?? 0) === 0 &&
        (r.pruned?.length ?? 0) === 0,
    );

  const noTargets = results !== null && results.length === 0;

  return (
    <DialogShell open={open} onClose={onClose} maxWidth="2xl" preventClose={syncing}>
      <Card>
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-bold text-pencil">Sync Preview</h2>
            {results !== null && !loading && (
              <button
                onClick={runDryRun}
                className="text-pencil-light hover:text-pencil transition-colors"
                title="Refresh preview"
              >
                <RefreshCw size={16} />
              </button>
            )}
          </div>

          {loading && (
            <div className="flex items-center justify-center py-8">
              <Spinner />
              <span className="ml-3 text-pencil-light">Running dry-run...</span>
            </div>
          )}

          {error && (
            <div className="text-center py-4 space-y-3">
              <p className="text-danger text-sm">{error}</p>
              <Button variant="secondary" size="sm" onClick={runDryRun}>
                Retry
              </Button>
            </div>
          )}

          {!loading && !error && noTargets && (
            <p className="text-pencil-light text-center py-4">
              No targets configured. Check your config to add targets.
            </p>
          )}

          {!loading && !error && allUpToDate && !noTargets && (
            <p className="text-pencil-light text-center py-4">
              Everything is up to date. No sync needed.
            </p>
          )}

          {!loading && !error && results && !allUpToDate && !noTargets && (
            <SyncResultList results={results} />
          )}

          <div className="flex justify-end gap-3 pt-2">
            <Button variant="secondary" onClick={onClose} disabled={syncing}>
              Cancel
            </Button>
            {!allUpToDate && !noTargets && results && !error && (
              <Button onClick={handleSync} loading={syncing}>
                Sync Now
              </Button>
            )}
          </div>
        </div>
      </Card>
    </DialogShell>
  );
}
