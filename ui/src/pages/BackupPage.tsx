import { useState } from 'react';
import { Link } from 'react-router-dom';
import {
  Archive,
  Clock,
  RotateCcw,
  Trash2,
  RefreshCw,
  Target,
  Plus,
} from 'lucide-react';
import { api } from '../api/client';
import type { BackupInfo, RestoreValidateResponse } from '../api/client';
import { useApi } from '../hooks/useApi';
import { useAppContext } from '../context/AppContext';
import { formatSize } from '../lib/format';
import Card from '../components/Card';
import HandButton from '../components/HandButton';
import Badge from '../components/Badge';
import ConfirmDialog from '../components/ConfirmDialog';
import EmptyState from '../components/EmptyState';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';

function timeAgo(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diff = now - then;
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.floor(hrs / 24);
  if (days < 30) return `${days}d ago`;
  return `${Math.floor(days / 30)}mo ago`;
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  });
}

export default function BackupPage() {
  const { isProjectMode } = useAppContext();
  const { toast } = useToast();
  const { data, loading, error, refetch } = useApi(() => api.listBackups(), []);

  if (isProjectMode) {
    return (
      <div className="animate-sketch-in">
        <Card decoration="tape" className="text-center py-12">
          <Archive size={40} strokeWidth={2} className="text-pencil-light mx-auto mb-4" />
          <h2
            className="text-2xl font-bold text-pencil mb-2"
            style={{ fontFamily: 'var(--font-heading)' }}
          >
            Backup & Restore is not available in project mode
          </h2>
          <p className="text-pencil-light mb-4">
            Project skills are managed through your project's own version control.
          </p>
          <Link
            to="/"
            className="text-blue hover:underline"
            style={{ fontFamily: 'var(--font-hand)' }}
          >
            Back to Dashboard
          </Link>
        </Card>
      </div>
    );
  }

  const [creating, setCreating] = useState(false);
  const [cleanupOpen, setCleanupOpen] = useState(false);
  const [cleaningUp, setCleaningUp] = useState(false);
  const [restoreTarget, setRestoreTarget] = useState<{ timestamp: string; target: string } | null>(null);
  const [restoring, setRestoring] = useState(false);
  const [validation, setValidation] = useState<{
    loading: boolean;
    result: RestoreValidateResponse | null;
  }>({ loading: false, result: null });

  const backups = data?.backups ?? [];

  const handleCreate = async () => {
    setCreating(true);
    try {
      const res = await api.createBackup();
      if (res.backedUpTargets?.length) {
        toast(`Backed up ${res.backedUpTargets.length} target(s)`, 'success');
      } else {
        toast('Nothing to back up', 'info');
      }
      refetch();
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      setCreating(false);
    }
  };

  const handleCleanup = async () => {
    setCleaningUp(true);
    try {
      const res = await api.cleanupBackups();
      toast(`Cleaned up ${res.removed} old backup(s)`, 'success');
      refetch();
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      setCleaningUp(false);
      setCleanupOpen(false);
    }
  };

  const openRestoreDialog = async (timestamp: string, target: string) => {
    setRestoreTarget({ timestamp, target });
    setValidation({ loading: true, result: null });
    try {
      const result = await api.validateRestore({ timestamp, target });
      setValidation({ loading: false, result });
    } catch {
      setValidation({ loading: false, result: null });
    }
  };

  const closeRestoreDialog = () => {
    setRestoreTarget(null);
    setValidation({ loading: false, result: null });
  };

  const handleRestore = async () => {
    if (!restoreTarget) return;
    setRestoring(true);
    const needsForce = (validation.result?.conflicts?.length ?? 0) > 0;
    try {
      await api.restore({ ...restoreTarget, force: needsForce });
      toast(`Restored ${restoreTarget.target} from backup`, 'success');
      refetch();
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      setRestoring(false);
      closeRestoreDialog();
    }
  };

  if (loading) return <PageSkeleton />;

  if (error) {
    return (
      <Card>
        <p className="text-danger">{error}</p>
      </Card>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h2
          className="text-3xl font-bold text-pencil"
          style={{ fontFamily: 'var(--font-heading)' }}
        >
          Backup & Restore
        </h2>
        <p
          className="text-pencil-light mt-1"
          style={{ fontFamily: 'var(--font-hand)' }}
        >
          Create snapshots of your targets and restore them when needed
        </p>
      </div>

      {/* Action Card */}
      <Card variant="postit">
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
          <div>
            <p
              className="text-lg font-medium text-pencil"
              style={{ fontFamily: 'var(--font-hand)' }}
            >
              {backups.length === 0
                ? 'No backups yet'
                : `${backups.length} backup${backups.length !== 1 ? 's' : ''} on file`}
            </p>
            {data && data.totalSizeBytes > 0 && (
              <p className="text-sm text-pencil-light">
                Total size: {formatSize(data.totalSizeBytes)}
              </p>
            )}
          </div>
          <div className="flex gap-3">
            <HandButton
              variant="primary"
              size="lg"
              onClick={handleCreate}
              disabled={creating}
            >
              {creating ? (
                <><RefreshCw size={18} strokeWidth={2.5} className="animate-spin" /> Creating...</>
              ) : (
                <><Plus size={18} strokeWidth={2.5} /> Create Backup</>
              )}
            </HandButton>
            {backups.length > 0 && (
              <HandButton
                variant="ghost"
                size="sm"
                onClick={() => setCleanupOpen(true)}
              >
                <Trash2 size={16} strokeWidth={2.5} /> Cleanup
              </HandButton>
            )}
          </div>
        </div>
      </Card>

      {/* Backup List */}
      {backups.length === 0 ? (
        <EmptyState
          icon={Archive}
          title="No backups found"
          description="Create your first backup to protect your target configurations"
          action={
            <HandButton variant="primary" onClick={handleCreate} disabled={creating}>
              <Archive size={16} strokeWidth={2.5} /> Create First Backup
            </HandButton>
          }
        />
      ) : (
        <div className="space-y-4">
          {backups.map((backup, i) => (
            <BackupCard
              key={backup.timestamp}
              backup={backup}
              isNewest={i === 0}
              index={i}
              onRestore={(target) =>
                openRestoreDialog(backup.timestamp, target)
              }
            />
          ))}
        </div>
      )}

      {/* Cleanup Dialog */}
      <ConfirmDialog
        open={cleanupOpen}
        title="Cleanup Old Backups"
        message={
          <span>
            This will remove old backups based on retention policy
            (max 30 days, max 10 backups, max 500 MB).
          </span>
        }
        confirmText="Cleanup"
        variant="danger"
        loading={cleaningUp}
        onConfirm={handleCleanup}
        onCancel={() => setCleanupOpen(false)}
      />

      {/* Restore Dialog */}
      <ConfirmDialog
        open={restoreTarget !== null}
        title="Restore Backup"
        wide
        message={
          restoreTarget ? (
            <div className="text-left space-y-3">
              <div className="space-y-1 text-sm">
                <div><strong>Target:</strong> {restoreTarget.target}</div>
                <div><strong>From:</strong> <code className="text-xs bg-paper-dark/50 px-1 rounded">{restoreTarget.timestamp}</code></div>
                {validation.result && validation.result.backupSizeBytes > 0 && (
                  <div><strong>Backup size:</strong> {formatSize(validation.result.backupSizeBytes)}</div>
                )}
              </div>

              {validation.loading && (
                <p className="text-pencil-light italic text-sm">Checking target state...</p>
              )}

              {validation.result?.currentIsSymlink && (
                <p className="text-blue text-sm">
                  Current target is a symlink — it will be safely replaced.
                </p>
              )}

              {(validation.result?.conflicts?.length ?? 0) > 0 && (
                <div className="bg-warning/10 border border-warning/30 rounded p-2 text-sm">
                  <p className="font-medium text-warning mb-1">
                    Target has {validation.result!.conflicts.length} existing item(s) that will be overwritten:
                  </p>
                  <ul className="list-disc list-inside text-pencil-light max-h-24 overflow-y-auto">
                    {validation.result!.conflicts.slice(0, 10).map((f) => (
                      <li key={f}>{f}</li>
                    ))}
                    {validation.result!.conflicts.length > 10 && (
                      <li>...and {validation.result!.conflicts.length - 10} more</li>
                    )}
                  </ul>
                </div>
              )}

              {validation.result && !validation.result.currentIsSymlink && validation.result.conflicts.length === 0 && (
                <p className="text-green text-sm">
                  Target is empty or does not exist — safe to restore.
                </p>
              )}
            </div>
          ) : <span />
        }
        confirmText={
          (validation.result?.conflicts?.length ?? 0) > 0
            ? 'Restore (overwrite)'
            : 'Restore'
        }
        variant="danger"
        loading={restoring || validation.loading}
        onConfirm={handleRestore}
        onCancel={closeRestoreDialog}
      />
    </div>
  );
}

function BackupCard({
  backup,
  isNewest,
  index,
  onRestore,
}: {
  backup: BackupInfo;
  isNewest: boolean;
  index: number;
  onRestore: (target: string) => void;
}) {
  return (
    <Card
      decoration={isNewest ? 'tape' : 'none'}
      style={{
        transform: `rotate(${index % 2 === 0 ? '-0.2' : '0.2'}deg)`,
      }}
    >
      <div className="space-y-3">
        {/* Timestamp row */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 text-pencil">
            <Clock size={16} strokeWidth={2.5} />
            <span
              className="font-medium"
              style={{ fontFamily: 'var(--font-hand)' }}
            >
              {formatDate(backup.date)}
            </span>
            <span className="text-sm text-pencil-light">
              {timeAgo(backup.date)}
            </span>
          </div>
          {backup.sizeBytes > 0 && (
            <span className="text-xs text-pencil-light">
              {formatSize(backup.sizeBytes)}
            </span>
          )}
        </div>

        {/* Targets */}
        <div className="flex items-center gap-2 flex-wrap">
          <Target size={14} strokeWidth={2.5} className="text-pencil-light" />
          {backup.targets.map((t) => (
            <Badge key={t} variant="info">{t}</Badge>
          ))}
        </div>

        {/* Actions */}
        <div className="border-t border-dashed border-pencil-light/40 pt-3 flex gap-2">
          {backup.targets.map((t) => (
            <HandButton
              key={t}
              variant="secondary"
              size="sm"
              onClick={() => onRestore(t)}
            >
              <RotateCcw size={14} strokeWidth={2.5} /> Restore {t}
            </HandButton>
          ))}
        </div>
      </div>
    </Card>
  );
}
