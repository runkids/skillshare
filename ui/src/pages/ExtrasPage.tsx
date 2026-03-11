import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { FolderPlus, Plus, RefreshCw, Trash2, X } from 'lucide-react';
import { api } from '../api/client';
import type { Extra } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { useAppContext } from '../context/AppContext';
import { useToast } from '../components/Toast';
import Card from '../components/Card';
import HandButton from '../components/HandButton';
import { HandInput, HandSelect } from '../components/HandInput';
import Badge from '../components/Badge';
import EmptyState from '../components/EmptyState';
import ConfirmDialog from '../components/ConfirmDialog';
import { PageSkeleton } from '../components/Skeleton';
import { radius } from '../design';

// ─── AddExtraModal ────────────────────────────────────────────────────────────

interface TargetEntry {
  path: string;
  mode: string;
}

const MODE_OPTIONS = [
  { value: 'merge', label: 'merge', description: 'Per-file symlinks, preserves local files' },
  { value: 'copy', label: 'copy', description: 'Copy files to target directory' },
  { value: 'symlink', label: 'symlink', description: 'Symlink entire directory' },
];

function AddExtraModal({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: () => void;
}) {
  const { toast } = useToast();
  const [name, setName] = useState('');
  const [targets, setTargets] = useState<TargetEntry[]>([{ path: '', mode: 'merge' }]);
  const [saving, setSaving] = useState(false);

  const addTarget = () => setTargets((prev) => [...prev, { path: '', mode: 'merge' }]);

  const updateTarget = (i: number, field: keyof TargetEntry, value: string) => {
    setTargets((prev) => prev.map((t, idx) => (idx === i ? { ...t, [field]: value } : t)));
  };

  const removeTarget = (i: number) => {
    setTargets((prev) => prev.filter((_, idx) => idx !== i));
  };

  const handleCreate = async () => {
    if (!name.trim()) {
      toast('Name is required', 'error');
      return;
    }
    const validTargets = targets.filter((t) => t.path.trim());
    if (validTargets.length === 0) {
      toast('At least one target path is required', 'error');
      return;
    }
    setSaving(true);
    try {
      await api.createExtra({
        name: name.trim(),
        targets: validTargets.map((t) => ({ path: t.path.trim(), mode: t.mode })),
      });
      toast(`Extra "${name.trim()}" created`, 'success');
      onCreated();
    } catch (err: any) {
      toast(err.message, 'error');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      role="dialog"
      aria-modal="true"
      onClick={(e) => {
        if (e.target === e.currentTarget && !saving) onClose();
      }}
    >
      {/* Backdrop */}
      <div className="absolute inset-0 bg-pencil/30" />

      {/* Dialog */}
      <div
        className="relative w-full max-w-lg animate-fade-in"
        style={{ borderRadius: radius.md }}
      >
        <Card>
          <div className="flex items-center justify-between mb-4">
            <h3
              className="text-xl font-bold text-pencil"
            >
              Add Extra
            </h3>
            <button
              onClick={onClose}
              disabled={saving}
              className="text-pencil-light hover:text-pencil transition-colors disabled:opacity-50"
              aria-label="Close"
            >
              <X size={20} strokeWidth={2.5} />
            </button>
          </div>

          <div className="space-y-4">
            {/* Name */}
            <HandInput
              label="Name"
              placeholder="e.g. my-scripts"
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={saving}
            />

            {/* Targets */}
            <div>
              <label
                className="block text-base text-pencil-light mb-2"
              >
                Targets
              </label>
              <div className="space-y-2">
                {targets.map((t, i) => (
                  <div key={i} className="flex gap-2 items-start">
                    <div className="flex-1">
                      <HandInput
                        placeholder="Target path (e.g. ~/.cursor/scripts)"
                        value={t.path}
                        onChange={(e) => updateTarget(i, 'path', e.target.value)}
                        disabled={saving}
                      />
                    </div>
                    <div className="w-36 shrink-0">
                      <HandSelect
                        value={t.mode}
                        onChange={(v) => updateTarget(i, 'mode', v)}
                        options={MODE_OPTIONS}
                      />
                    </div>
                    {targets.length > 1 && (
                      <button
                        onClick={() => removeTarget(i)}
                        disabled={saving}
                        className="mt-2.5 text-pencil-light hover:text-danger transition-colors disabled:opacity-50"
                        aria-label="Remove target"
                      >
                        <X size={16} strokeWidth={2.5} />
                      </button>
                    )}
                  </div>
                ))}
              </div>
              <HandButton
                variant="ghost"
                size="sm"
                onClick={addTarget}
                disabled={saving}
                className="mt-2"
              >
                <Plus size={14} strokeWidth={2.5} /> Add Target
              </HandButton>
            </div>
          </div>

          <div className="flex gap-3 justify-end mt-6">
            <HandButton variant="ghost" size="sm" onClick={onClose} disabled={saving}>
              Cancel
            </HandButton>
            <HandButton variant="primary" size="sm" onClick={handleCreate} disabled={saving}>
              {saving ? 'Creating...' : 'Create'}
            </HandButton>
          </div>
        </Card>
      </div>
    </div>
  );
}

// ─── ExtraCard ────────────────────────────────────────────────────────────────

function ExtraCard({
  extra,
  onSync,
  onRemove,
}: {
  extra: Extra;
  index?: number;
  onSync: (name: string) => void;
  onRemove: (name: string) => void;
}) {
  return (
    <Card>
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2 flex-wrap">
          <FolderPlus size={18} strokeWidth={2.5} className="text-blue shrink-0" />
          <h3
            className="text-lg font-bold text-pencil"
          >
            {extra.name}
          </h3>
          <Badge variant={extra.source_exists ? 'success' : 'warning'}>
            {extra.file_count} {extra.file_count === 1 ? 'file' : 'files'}
          </Badge>
          {!extra.source_exists && (
            <Badge variant="danger">source missing</Badge>
          )}
        </div>
        <div className="flex gap-2 shrink-0">
          <HandButton variant="secondary" size="sm" onClick={() => onSync(extra.name)}>
            <RefreshCw size={12} strokeWidth={2.5} /> Sync
          </HandButton>
          <HandButton variant="danger" size="sm" onClick={() => onRemove(extra.name)}>
            <Trash2 size={12} strokeWidth={2.5} />
          </HandButton>
        </div>
      </div>

      {/* Source path */}
      <p
        className="text-sm text-pencil-light mb-3"
        style={{ fontFamily: "'Courier New', monospace" }}
      >
        {extra.source_dir}
      </p>

      {/* Targets */}
      {extra.targets.length > 0 ? (
        <div className="space-y-1.5">
          {extra.targets.map((t, ti) => (
            <div
              key={ti}
              className="flex items-center justify-between py-1.5 px-3 bg-paper-warm border border-muted"
              style={{ borderRadius: radius.sm }}
            >
              <span
                style={{ fontFamily: "'Courier New', monospace" }}
                className="text-sm truncate text-pencil-light mr-2"
              >
                {t.path}
              </span>
              <div className="flex items-center gap-2 shrink-0">
                <Badge variant="info">{t.mode}</Badge>
                <Badge
                  variant={
                    t.status === 'synced'
                      ? 'success'
                      : t.status === 'drift'
                      ? 'warning'
                      : 'danger'
                  }
                >
                  {t.status}
                </Badge>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <p
          className="text-sm text-pencil-light italic"
        >
          No targets configured
        </p>
      )}
    </Card>
  );
}

// ─── ExtrasPage ───────────────────────────────────────────────────────────────

export default function ExtrasPage() {
  const { isProjectMode } = useAppContext();
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const { data, isPending, error } = useQuery({
    queryKey: queryKeys.extras,
    queryFn: () => api.listExtras(),
    staleTime: staleTimes.extras,
  });

  const [showAdd, setShowAdd] = useState(false);
  const [removeName, setRemoveName] = useState<string | null>(null);
  const [removing, setRemoving] = useState(false);

  const invalidate = () => queryClient.invalidateQueries({ queryKey: queryKeys.extras });

  const handleSyncAll = async () => {
    try {
      await api.syncExtras();
      toast('All extras synced', 'success');
      invalidate();
    } catch (err: any) {
      toast(err.message, 'error');
    }
  };

  const handleSync = async (name: string) => {
    try {
      await api.syncExtras({ name });
      toast(`"${name}" synced`, 'success');
      invalidate();
    } catch (err: any) {
      toast(err.message, 'error');
    }
  };

  const handleRemove = async () => {
    if (!removeName) return;
    setRemoving(true);
    try {
      await api.deleteExtra(removeName);
      toast(`"${removeName}" removed`, 'success');
      invalidate();
    } catch (err: any) {
      toast(err.message, 'error');
    } finally {
      setRemoving(false);
      setRemoveName(null);
    }
  };

  const handleCreated = () => {
    setShowAdd(false);
    invalidate();
  };

  const extras = data?.extras ?? [];

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <div className="flex items-center justify-between">
          <h2
            className="text-3xl font-bold text-pencil"
          >
            Extras
          </h2>
          <div className="flex gap-2">
            {extras.length > 0 && (
              <HandButton variant="secondary" size="sm" onClick={handleSyncAll}>
                <RefreshCw size={14} strokeWidth={2.5} /> Sync All
              </HandButton>
            )}
            <HandButton variant="primary" size="sm" onClick={() => setShowAdd(true)}>
              <Plus size={14} strokeWidth={2.5} /> Add Extra
            </HandButton>
          </div>
        </div>
        <p
          className="text-pencil-light mt-1"
        >
          {isProjectMode
            ? 'Sync arbitrary directories to project targets'
            : 'Sync arbitrary directories to AI tool targets'}
        </p>
      </div>

      {/* Loading */}
      {isPending && <PageSkeleton />}

      {/* Error */}
      {error && (
        <Card>
          <p className="text-danger">{error.message}</p>
        </Card>
      )}

      {/* Empty state */}
      {!isPending && !error && extras.length === 0 && (
        <EmptyState
          icon={FolderPlus}
          title="No extras configured"
          description="Extras let you sync any directory to your AI tool targets alongside your skills."
          action={
            <HandButton variant="primary" size="md" onClick={() => setShowAdd(true)}>
              <Plus size={16} strokeWidth={2.5} /> Add Extra
            </HandButton>
          }
        />
      )}

      {/* Extras list */}
      {!isPending && !error && extras.length > 0 && (
        <div className="space-y-4">
          {extras.map((extra, i) => (
            <ExtraCard
              key={extra.name}
              extra={extra}
              index={i}
              onSync={handleSync}
              onRemove={(name) => setRemoveName(name)}
            />
          ))}
        </div>
      )}

      {/* Add Extra modal */}
      {showAdd && (
        <AddExtraModal onClose={() => setShowAdd(false)} onCreated={handleCreated} />
      )}

      {/* Remove confirm dialog */}
      <ConfirmDialog
        open={removeName !== null}
        title="Remove Extra"
        message={
          removeName ? (
            <span>
              Remove extra <strong>{removeName}</strong>? This will not delete the source
              directory or synced files.
            </span>
          ) : (
            <span />
          )
        }
        confirmText="Remove"
        variant="danger"
        loading={removing}
        onConfirm={handleRemove}
        onCancel={() => setRemoveName(null)}
      />
    </div>
  );
}
