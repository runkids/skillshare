import { useState } from 'react';
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
import type { BackupInfo } from '../api/client';
import { useApi } from '../hooks/useApi';
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
  const { toast } = useToast();
  const { data, loading, error, refetch } = useApi(() => api.listBackups(), []);

  const [creating, setCreating] = useState(false);
  const [cleanupOpen, setCleanupOpen] = useState(false);
  const [cleaningUp, setCleaningUp] = useState(false);
  const [restoreTarget, setRestoreTarget] = useState<{ timestamp: string; target: string } | null>(null);
  const [restoring, setRestoring] = useState(false);

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

  const handleRestore = async () => {
    if (!restoreTarget) return;
    setRestoring(true);
    try {
      await api.restore({ ...restoreTarget, force: true });
      toast(`Restored ${restoreTarget.target} from backup`, 'success');
      refetch();
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      setRestoring(false);
      setRestoreTarget(null);
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
            {data && data.totalSizeMB > 0 && (
              <p className="text-sm text-pencil-light">
                Total size: {data.totalSizeMB.toFixed(1)} MB
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
                setRestoreTarget({ timestamp: backup.timestamp, target })
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
        message={
          restoreTarget ? (
            <span>
              Restore <strong>{restoreTarget.target}</strong> from backup{' '}
              <code className="text-sm">{restoreTarget.timestamp}</code>?
              This will overwrite the current target contents.
            </span>
          ) : <span />
        }
        confirmText="Restore"
        variant="danger"
        loading={restoring}
        onConfirm={handleRestore}
        onCancel={() => setRestoreTarget(null)}
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
          {backup.sizeMB > 0 && (
            <span className="text-xs text-pencil-light">
              {backup.sizeMB.toFixed(1)} MB
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
