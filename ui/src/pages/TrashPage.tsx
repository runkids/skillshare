import { useState } from 'react';
import {
  Trash2,
  Clock,
  RotateCcw,
  X,
} from 'lucide-react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import type { TrashedSkill } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
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

export default function TrashPage() {
  const { isProjectMode } = useAppContext();
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const { data, isPending, error } = useQuery({
    queryKey: queryKeys.trash,
    queryFn: () => api.listTrash(),
    staleTime: staleTimes.trash,
  });

  const [restoreName, setRestoreName] = useState<string | null>(null);
  const [restoring, setRestoring] = useState(false);
  const [deleteName, setDeleteName] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [emptyOpen, setEmptyOpen] = useState(false);
  const [emptying, setEmptying] = useState(false);

  const items = data?.items ?? [];

  const handleRestore = async () => {
    if (!restoreName) return;
    setRestoring(true);
    try {
      await api.restoreTrash(restoreName);
      toast(`Restored "${restoreName}" from trash`, 'success');
      queryClient.invalidateQueries({ queryKey: queryKeys.trash });
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      setRestoring(false);
      setRestoreName(null);
    }
  };

  const handleDelete = async () => {
    if (!deleteName) return;
    setDeleting(true);
    try {
      await api.deleteTrash(deleteName);
      toast(`Permanently deleted "${deleteName}"`, 'success');
      queryClient.invalidateQueries({ queryKey: queryKeys.trash });
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      setDeleting(false);
      setDeleteName(null);
    }
  };

  const handleEmpty = async () => {
    setEmptying(true);
    try {
      const res = await api.emptyTrash();
      toast(`Emptied trash (${res.removed} item${res.removed !== 1 ? 's' : ''} removed)`, 'success');
      queryClient.invalidateQueries({ queryKey: queryKeys.trash });
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      setEmptying(false);
      setEmptyOpen(false);
    }
  };

  if (isPending) return <PageSkeleton />;

  if (error) {
    return (
      <Card>
        <p className="text-danger">{error.message}</p>
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
          Trash
        </h2>
        <p
          className="text-pencil-light mt-1"
          style={{ fontFamily: 'var(--font-hand)' }}
        >
          {isProjectMode
            ? 'Recently deleted project skills are kept for 7 days before automatic cleanup'
            : 'Recently deleted skills are kept for 7 days before automatic cleanup'}
        </p>
      </div>

      {/* Summary Card */}
      <Card variant="postit">
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
          <div>
            <p
              className="text-lg font-medium text-pencil"
              style={{ fontFamily: 'var(--font-hand)' }}
            >
              {items.length === 0
                ? 'Trash is empty'
                : `${items.length} item${items.length !== 1 ? 's' : ''} in trash`}
            </p>
            {data && data.totalSize > 0 && (
              <p className="text-sm text-pencil-light">
                Total size: {formatSize(data.totalSize)}
              </p>
            )}
          </div>
          {items.length > 0 && (
            <HandButton
              variant="danger"
              size="lg"
              onClick={() => setEmptyOpen(true)}
            >
              <Trash2 size={18} strokeWidth={2.5} /> Empty Trash
            </HandButton>
          )}
        </div>
      </Card>

      {/* Item List */}
      {items.length === 0 ? (
        <EmptyState
          icon={Trash2}
          title="Trash is empty"
          description="Deleted skills will appear here for 7 days"
        />
      ) : (
        <div className="space-y-4">
          {items.map((item, i) => (
            <TrashCard
              key={`${item.name}-${item.timestamp}`}
              item={item}
              index={i}
              onRestore={() => setRestoreName(item.name)}
              onDelete={() => setDeleteName(item.name)}
            />
          ))}
        </div>
      )}

      {/* Restore Dialog */}
      <ConfirmDialog
        open={restoreName !== null}
        title="Restore Skill"
        message={
          restoreName ? (
            <span>
              Restore <strong>{restoreName}</strong> back to your skills directory?
            </span>
          ) : <span />
        }
        confirmText="Restore"
        variant="default"
        loading={restoring}
        onConfirm={handleRestore}
        onCancel={() => setRestoreName(null)}
      />

      {/* Delete Dialog */}
      <ConfirmDialog
        open={deleteName !== null}
        title="Permanently Delete"
        message={
          deleteName ? (
            <span>
              Permanently delete <strong>{deleteName}</strong>? This cannot be undone.
            </span>
          ) : <span />
        }
        confirmText="Delete Forever"
        variant="danger"
        loading={deleting}
        onConfirm={handleDelete}
        onCancel={() => setDeleteName(null)}
      />

      {/* Empty Trash Dialog */}
      <ConfirmDialog
        open={emptyOpen}
        title="Empty Trash"
        message={
          <span>
            Permanently delete all <strong>{items.length}</strong> item{items.length !== 1 ? 's' : ''} from trash?
            This cannot be undone.
          </span>
        }
        confirmText="Empty Trash"
        variant="danger"
        loading={emptying}
        onConfirm={handleEmpty}
        onCancel={() => setEmptyOpen(false)}
      />
    </div>
  );
}

function TrashCard({
  item,
  index,
  onRestore,
  onDelete,
}: {
  item: TrashedSkill;
  index: number;
  onRestore: () => void;
  onDelete: () => void;
}) {
  return (
    <Card
      decoration={index === 0 ? 'tape' : 'none'}
      style={{
        transform: `rotate(${index % 2 === 0 ? '-0.2' : '0.2'}deg)`,
      }}
    >
      <div className="space-y-3">
        {/* Name + time */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 text-pencil">
            <Trash2 size={16} strokeWidth={2.5} />
            <span
              className="font-medium"
              style={{ fontFamily: 'var(--font-hand)' }}
            >
              {item.name}
            </span>
            <span className="text-sm text-pencil-light">
              {timeAgo(item.date)}
            </span>
          </div>
          <Badge variant="default">{formatSize(item.size)}</Badge>
        </div>

        {/* Deleted at */}
        <div className="flex items-center gap-2 text-sm text-pencil-light">
          <Clock size={14} strokeWidth={2.5} />
          <span>Deleted {new Date(item.date).toLocaleString(undefined, {
            year: 'numeric',
            month: 'short',
            day: 'numeric',
            hour: 'numeric',
            minute: '2-digit',
          })}</span>
        </div>

        {/* Actions */}
        <div className="border-t border-dashed border-pencil-light/40 pt-3 flex gap-2">
          <HandButton variant="secondary" size="sm" onClick={onRestore}>
            <RotateCcw size={14} strokeWidth={2.5} /> Restore
          </HandButton>
          <HandButton variant="ghost" size="sm" onClick={onDelete}>
            <X size={14} strokeWidth={2.5} /> Delete
          </HandButton>
        </div>
      </div>
    </Card>
  );
}
