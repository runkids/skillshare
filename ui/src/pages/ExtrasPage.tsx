import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { FolderOpen, FolderPlus, Plus, RefreshCw, Target, Trash2, X } from 'lucide-react';
import { api } from '../api/client';
import type { Extra } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { useAppContext } from '../context/AppContext';
import { useToast } from '../components/Toast';
import Card from '../components/Card';
import Button from '../components/Button';
import IconButton from '../components/IconButton';
import DialogShell from '../components/DialogShell';
import { Input, Select } from '../components/Input';
import Badge from '../components/Badge';
import EmptyState from '../components/EmptyState';
import PageHeader from '../components/PageHeader';
import ConfirmDialog from '../components/ConfirmDialog';
import { PageSkeleton } from '../components/Skeleton';

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
    <DialogShell open={true} onClose={onClose} maxWidth="2xl" preventClose={saving}>
        <Card overflow className="p-6">
          <div className="flex items-center justify-between mb-4">
            <h3
              className="text-xl font-bold text-pencil"
            >
              Add Extra
            </h3>
            <IconButton
              icon={<X size={20} strokeWidth={2.5} />}
              label="Close"
              size="sm"
              variant="ghost"
              onClick={onClose}
              disabled={saving}
            />
          </div>

          <div className="space-y-4">
            {/* Name */}
            <Input
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
                      <Input
                        placeholder="Target path (e.g. ~/.cursor/scripts)"
                        value={t.path}
                        onChange={(e) => updateTarget(i, 'path', e.target.value)}
                        disabled={saving}
                      />
                    </div>
                    <div className="w-36 shrink-0">
                      <Select
                        value={t.mode}
                        onChange={(v) => updateTarget(i, 'mode', v)}
                        options={MODE_OPTIONS}
                      />
                    </div>
                    {targets.length > 1 && (
                      <IconButton
                        icon={<X size={16} strokeWidth={2.5} />}
                        label="Remove target"
                        size="sm"
                        variant="ghost"
                        onClick={() => removeTarget(i)}
                        disabled={saving}
                        className="mt-2.5 hover:text-danger"
                      />
                    )}
                  </div>
                ))}
              </div>
              <Button
                variant="ghost"
                size="sm"
                onClick={addTarget}
                disabled={saving}
                className="mt-2"
              >
                <Plus size={14} strokeWidth={2.5} /> Add Target
              </Button>
            </div>
          </div>

          <div className="flex gap-3 justify-end mt-6">
            <Button variant="secondary" size="sm" onClick={onClose} disabled={saving}>
              Cancel
            </Button>
            <Button variant="primary" size="sm" onClick={handleCreate} disabled={saving}>
              {saving ? 'Creating...' : 'Create'}
            </Button>
          </div>
        </Card>
    </DialogShell>
  );
}

// ─── ExtraCard ────────────────────────────────────────────────────────────────

function ExtraCard({
  extra,
  onSync,
  onRemove,
  onModeChange,
}: {
  extra: Extra;
  index?: number;
  onSync: (name: string) => Promise<void>;
  onRemove: (name: string) => void;
  onModeChange: (name: string, target: string, mode: string) => Promise<void>;
}) {
  const [syncing, setSyncing] = useState(false);
  const [changingMode, setChangingMode] = useState<string | null>(null);

  const handleSync = async () => {
    setSyncing(true);
    try {
      await onSync(extra.name);
    } finally {
      setSyncing(false);
    }
  };

  return (
    <Card overflow>
      {/* Header */}
      <div className="flex items-center justify-between gap-4 mb-1">
        <div className="flex items-center gap-2 flex-wrap min-w-0">
          <FolderPlus size={16} strokeWidth={2.5} className="text-blue shrink-0" />
          <span className="font-bold text-pencil">{extra.name}</span>
          <Badge variant={extra.source_exists ? 'success' : 'warning'}>
            {extra.file_count} {extra.file_count === 1 ? 'file' : 'files'}
          </Badge>
          {!extra.source_exists && (
            <Badge variant="danger">source missing</Badge>
          )}
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <Button variant="secondary" size="sm" onClick={handleSync} disabled={syncing}>
            <RefreshCw size={12} strokeWidth={2.5} className={syncing ? 'animate-spin' : ''} />
            {syncing ? 'Syncing...' : 'Sync'}
          </Button>
          <IconButton
            icon={<Trash2 size={16} strokeWidth={2.5} />}
            label="Remove extra"
            size="md"
            variant="danger-outline"
            onClick={() => onRemove(extra.name)}
          />
        </div>
      </div>

      {/* Source */}
      <div className="flex items-center gap-1.5 mt-3">
        <FolderOpen size={13} strokeWidth={2.5} className="text-warning shrink-0" />
        <span className="text-xs text-pencil-light uppercase tracking-wider">Source</span>
      </div>
      <p className="font-mono text-sm text-pencil-light truncate ml-5 mt-1">{extra.source_dir}</p>

      {/* Targets */}
      <div className="flex items-center gap-1.5 mt-3 pt-3 border-t border-dashed border-pencil-light/30">
        <Target size={13} strokeWidth={2.5} className="text-success shrink-0" />
        <span className="text-xs text-pencil-light uppercase tracking-wider">
          {extra.targets.length > 0 ? `Targets (${extra.targets.length})` : 'Targets'}
        </span>
      </div>
      <div className="ml-5 mt-1 space-y-1.5">
        {extra.targets.length > 0 ? (
          extra.targets.map((t, ti) => (
            <div key={ti} className="flex items-center gap-3">
              <div className="flex items-center gap-2 min-w-0 flex-1">
                <span className="font-mono text-sm truncate text-pencil-light">{t.path}</span>
                <Badge
                  variant={
                    t.status === 'synced'
                      ? 'success'
                      : t.status === 'drift'
                      ? 'warning'
                      : 'danger'
                  }
                  size="sm"
                >
                  {t.status}
                </Badge>
              </div>
              <Select
                value={t.mode}
                onChange={async (v) => {
                  if (v === t.mode) return;
                  setChangingMode(t.path);
                  try {
                    await onModeChange(extra.name, t.path, v);
                  } finally {
                    setChangingMode(null);
                  }
                }}
                options={MODE_OPTIONS}
                size="sm"
                className="w-36 shrink-0"
                disabled={changingMode === t.path}
              />
            </div>
          ))
        ) : (
          <p className="text-sm text-pencil-light italic">No targets configured</p>
        )}
      </div>
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
  const [syncingAll, setSyncingAll] = useState(false);

  const invalidate = () => queryClient.invalidateQueries({ queryKey: queryKeys.extras });

  const handleSyncAll = async () => {
    setSyncingAll(true);
    try {
      await api.syncExtras();
      toast('All extras synced', 'success');
      invalidate();
    } catch (err: any) {
      toast(err.message, 'error');
    } finally {
      setSyncingAll(false);
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

  const handleModeChange = async (name: string, target: string, mode: string) => {
    try {
      await api.setExtraMode(name, target, mode);
      toast(`Mode changed to "${mode}"`, 'success');
      invalidate();
    } catch (err: any) {
      toast(err.message, 'error');
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
      <PageHeader
        icon={<FolderPlus size={24} strokeWidth={2.5} />}
        title="Extras"
        subtitle={isProjectMode
          ? 'Sync arbitrary directories to project targets'
          : 'Sync arbitrary directories to AI tool targets'}
        actions={
          <>
            {extras.length > 0 && (
              <Button variant="secondary" size="sm" onClick={handleSyncAll} disabled={syncingAll}>
                <RefreshCw size={14} strokeWidth={2.5} className={syncingAll ? 'animate-spin' : ''} />
                {syncingAll ? 'Syncing...' : 'Sync All'}
              </Button>
            )}
            <Button variant="primary" size="sm" onClick={() => setShowAdd(true)}>
              <Plus size={14} strokeWidth={2.5} /> Add Extra
            </Button>
          </>
        }
      />

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
            <Button variant="primary" size="md" onClick={() => setShowAdd(true)}>
              <Plus size={16} strokeWidth={2.5} /> Add Extra
            </Button>
          }
        />
      )}

      {/* Extras list */}
      {!isPending && !error && extras.length > 0 && (
        <div data-tour="extras-list" className="space-y-4">
          {extras.map((extra, i) => (
            <ExtraCard
              key={extra.name}
              extra={extra}
              index={i}
              onSync={handleSync}
              onRemove={(name) => setRemoveName(name)}
              onModeChange={handleModeChange}
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
