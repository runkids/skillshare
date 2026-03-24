import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { FolderOpen, FolderPlus, Plus, RefreshCw, Target, Trash2, X, Zap } from 'lucide-react';
import { api } from '../api/client';
import type { Extra, ExtrasSyncResult } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { useAppContext } from '../context/AppContext';
import { useToast } from '../components/Toast';
import Card from '../components/Card';
import Button from '../components/Button';
import IconButton from '../components/IconButton';
import SplitButton from '../components/SplitButton';
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
  flatten: boolean;
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
  const [source, setSource] = useState('');
  const [targets, setTargets] = useState<TargetEntry[]>([{ path: '', mode: 'merge', flatten: false }]);
  const [saving, setSaving] = useState(false);

  const addTarget = () => setTargets((prev) => [...prev, { path: '', mode: 'merge', flatten: false }]);

  const updateTarget = (i: number, field: keyof TargetEntry, value: string | boolean) => {
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
        ...(source.trim() && { source: source.trim() }),
        targets: validTargets.map((t) => ({ path: t.path.trim(), mode: t.mode, flatten: t.flatten })),
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

            {/* Source path (optional) */}
            <Input
              label="Source path (optional)"
              placeholder="e.g. ~/my-extras/scripts"
              value={source}
              onChange={(e) => setSource(e.target.value)}
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
                        onChange={(v) => {
                          updateTarget(i, 'mode', v);
                          if (v === 'symlink') updateTarget(i, 'flatten', false);
                        }}
                        options={MODE_OPTIONS}
                      />
                    </div>
                    <label className="flex items-center gap-1.5 shrink-0 cursor-pointer select-none mt-2.5">
                      <input
                        type="checkbox"
                        checked={t.flatten}
                        onChange={(e) => updateTarget(i, 'flatten', e.target.checked)}
                        disabled={saving || t.mode === 'symlink'}
                        className="accent-primary"
                      />
                      <span className={`text-xs ${t.mode === 'symlink' ? 'text-pencil-light/50' : 'text-pencil-light'}`}>
                        Flatten
                      </span>
                    </label>
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
  onForceSync,
  onRemove,
  onModeChange,
}: {
  extra: Extra;
  index?: number;
  onSync: (name: string) => Promise<void>;
  onForceSync: (name: string) => Promise<void>;
  onRemove: (name: string) => void;
  onModeChange: (name: string, target: string, mode: string, flatten?: boolean) => Promise<void>;
}) {
  const [syncing, setSyncing] = useState(false);
  const [changingMode, setChangingMode] = useState<string | null>(null);

  const handleSync = async (force?: boolean) => {
    setSyncing(true);
    try {
      if (force) {
        await onForceSync(extra.name);
      } else {
        await onSync(extra.name);
      }
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
          {extra.source_type !== "default" && (
            <span className="ml-2 text-xs text-gray-500">({extra.source_type})</span>
          )}
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <SplitButton
            variant="secondary"
            size="sm"
            onClick={() => handleSync()}
            loading={syncing}
            dropdownAlign="right"
            items={[
              {
                label: 'Force Sync',
                icon: <Zap size={14} strokeWidth={2.5} />,
                onClick: () => handleSync(true),
                confirm: true,
              },
            ]}
          >
            <RefreshCw size={12} strokeWidth={2.5} />
            {syncing ? 'Syncing...' : 'Sync'}
          </SplitButton>
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
              <label className="flex items-center gap-1 shrink-0 cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={t.flatten}
                  onChange={async (e) => {
                    const newFlatten = e.target.checked;
                    setChangingMode(t.path);
                    try {
                      await onModeChange(extra.name, t.path, t.mode, newFlatten);
                    } finally {
                      setChangingMode(null);
                    }
                  }}
                  disabled={changingMode === t.path || t.mode === 'symlink'}
                  className="accent-primary"
                />
                <span className={`text-xs ${t.mode === 'symlink' ? 'text-pencil-light/50' : 'text-pencil-light'}`}>
                  Flatten
                </span>
              </label>
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

// ─── Sync result helpers ─────────────────────────────────────────────────────

type SyncTotals = { synced: number; skipped: number; targets: number; errors: number };

function sumEntry(entry: ExtrasSyncResult | undefined): SyncTotals {
  if (!entry) return { synced: 0, skipped: 0, targets: 0, errors: 0 };
  let synced = 0, skipped = 0, errors = 0;
  for (const t of entry.targets) {
    if (t.error) {
      errors++;
    } else {
      synced += t.synced;
      skipped += t.skipped;
      errors += t.errors?.length ?? 0;
    }
  }
  return { synced, skipped, targets: entry.targets.length, errors };
}

function sumAll(extras: ExtrasSyncResult[]): SyncTotals {
  const totals: SyncTotals = { synced: 0, skipped: 0, targets: 0, errors: 0 };
  for (const e of extras) {
    const s = sumEntry(e);
    totals.synced += s.synced;
    totals.skipped += s.skipped;
    totals.targets += s.targets;
    totals.errors += s.errors;
  }
  return totals;
}

function syncToastType(t: SyncTotals): 'success' | 'warning' | 'error' {
  if (t.errors > 0 && t.synced === 0) return 'error';
  if (t.errors > 0) return 'warning';
  if (t.skipped > 0 && t.synced === 0) return 'warning';
  return 'success';
}

function buildSyncToast(label: string, failLabel: string, t: SyncTotals, isForce: boolean): string {
  if (t.errors > 0 && t.synced === 0)
    return `${failLabel} \u2014 ${t.errors} error${t.errors > 1 ? 's' : ''}`;
  if (t.synced === 0 && t.skipped === 0 && t.errors === 0)
    return `${label} \u2014 no files in source`;
  const parts: string[] = [];
  parts.push(`${t.synced} file${t.synced !== 1 ? 's' : ''} to ${t.targets} target${t.targets !== 1 ? 's' : ''}`);
  if (t.skipped > 0)
    parts.push(`${t.skipped} skipped${!isForce ? ' (enable Force to override)' : ''}`);
  if (t.errors > 0)
    parts.push(`${t.errors} error${t.errors > 1 ? 's' : ''}`);
  return `${label} \u2014 ${parts.join(', ')}`;
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

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: queryKeys.extras });
    queryClient.invalidateQueries({ queryKey: queryKeys.extrasDiff() });
    queryClient.invalidateQueries({ queryKey: queryKeys.config });
    queryClient.invalidateQueries({ queryKey: queryKeys.overview });
  };

  const handleSyncAll = async (force = false) => {
    setSyncingAll(true);
    try {
      const res = await api.syncExtras({ force });
      const t = sumAll(res.extras);
      toast(buildSyncToast('All extras synced', 'Extras sync failed', t, force), syncToastType(t));
      invalidate();
    } catch (err: any) {
      toast(err.message, 'error');
    } finally {
      setSyncingAll(false);
    }
  };

  const handleSync = async (name: string, force = false) => {
    try {
      const res = await api.syncExtras({ name, force });
      const entry = res.extras.find((e) => e.name === name);
      const t = sumEntry(entry);
      toast(buildSyncToast(`"${name}" synced`, `"${name}" sync failed`, t, force), syncToastType(t));
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

  const handleModeChange = async (name: string, target: string, mode: string, flatten?: boolean) => {
    try {
      await api.setExtraMode(name, target, mode, flatten);
      const msg = flatten !== undefined ? `Updated (flatten=${flatten})` : `Mode changed to "${mode}"`;
      toast(msg, 'success');
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
              <SplitButton
                variant="secondary"
                size="sm"
                onClick={() => handleSyncAll()}
                loading={syncingAll}
                dropdownAlign="right"
                items={[
                  {
                    label: 'Force Sync All',
                    icon: <Zap size={14} strokeWidth={2.5} />,
                    onClick: () => handleSyncAll(true),
                    confirm: true,
                  },
                ]}
              >
                <RefreshCw size={14} strokeWidth={2.5} />
                {syncingAll ? 'Syncing...' : 'Sync All'}
              </SplitButton>
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

      {/* Empty state / Extras list */}
      {!isPending && !error && (
        <div data-tour="extras-list">
          {extras.length === 0 ? (
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
          ) : (
            <div className="space-y-4">
              {extras.map((extra, i) => (
                <ExtraCard
                  key={extra.name}
                  extra={extra}
                  index={i}
                  onSync={(name) => handleSync(name)}
                  onForceSync={(name) => handleSync(name, true)}
                  onRemove={(name) => setRemoveName(name)}
                  onModeChange={handleModeChange}
                />
              ))}
            </div>
          )}
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
