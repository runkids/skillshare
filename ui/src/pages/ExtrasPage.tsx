import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { FolderOpen, FolderPlus, Lock, Plus, Puzzle, RefreshCw, Target, Trash2, X, Zap } from 'lucide-react';
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
import { Input, Select, Checkbox } from '../components/Input';
import Badge from '../components/Badge';
import EmptyState from '../components/EmptyState';
import PageHeader from '../components/PageHeader';
import ConfirmDialog from '../components/ConfirmDialog';
import { PageSkeleton } from '../components/Skeleton';
import Tooltip from '../components/Tooltip';
import { useT } from '../i18n';

// ─── AddExtraModal ────────────────────────────────────────────────────────────

interface TargetEntry {
  path: string;
  mode: string;
  flatten: boolean;
  extension: string;
}

const MODE_OPTIONS = [
  { value: 'merge', label: 'merge', description: 'Per-file symlinks, preserves local files' },
  { value: 'copy', label: 'copy', description: 'Copy files to target directory' },
  { value: 'symlink', label: 'symlink', description: 'Symlink entire directory' },
];

function AddExtraModal({
  onClose,
  onCreated,
  availableExtensions,
}: {
  onClose: () => void;
  onCreated: () => void;
  availableExtensions: string[];
}) {
  const { toast } = useToast();
  const t = useT();
  const [name, setName] = useState('');
  const [source, setSource] = useState('');
  const [targets, setTargets] = useState<TargetEntry[]>([{ path: '', mode: 'merge', flatten: false, extension: '' }]);
  const [saving, setSaving] = useState(false);

  const addTarget = () => setTargets((prev) => [...prev, { path: '', mode: 'merge', flatten: false, extension: '' }]);

  const updateTarget = (i: number, field: keyof TargetEntry, value: string | boolean) => {
    setTargets((prev) => prev.map((t, idx) => (idx === i ? { ...t, [field]: value } : t)));
  };

  const removeTarget = (i: number) => {
    setTargets((prev) => prev.filter((_, idx) => idx !== i));
  };

  const handleCreate = async () => {
    if (!name.trim()) {
      toast(t('extras.error.nameRequired'), 'error');
      return;
    }
    const validTargets = targets.filter((tgt) => tgt.path.trim());
    if (validTargets.length === 0) {
      toast(t('extras.error.targetRequired'), 'error');
      return;
    }
    setSaving(true);
    try {
      await api.createExtra({
        name: name.trim(),
        ...(source.trim() && { source: source.trim() }),
        targets: validTargets.map((tgt) => ({
          path: tgt.path.trim(),
          mode: tgt.mode,
          flatten: tgt.flatten,
          ...(tgt.extension && { extension: tgt.extension }),
        })),
      });
      toast(t('extras.toast.created', { name: name.trim() }), 'success');
      onCreated();
    } catch (err: any) {
      toast(err.message, 'error');
    } finally {
      setSaving(false);
    }
  };

  return (
    <DialogShell open={true} onClose={onClose} maxWidth="2xl" preventClose={saving}>
          <div className="flex items-center justify-between mb-4">
            <h3
              className="text-xl font-bold text-pencil"
            >
              {t('extras.addExtraTitle')}
            </h3>
            <IconButton
              icon={<X size={20} strokeWidth={2.5} />}
              label={t('common.close')}
              size="sm"
              variant="ghost"
              onClick={onClose}
              disabled={saving}
            />
          </div>

          <div className="space-y-4">
            {/* Name */}
            <Input
              label={t('extras.modal.name')}
              placeholder={t('extras.modal.namePlaceholder')}
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={saving}
            />

            {/* Source path (optional) */}
            <div>
              <Input
                id="extra-source-path"
                label={t('extras.modal.sourcePath')}
                placeholder={t('extras.modal.sourcePathPlaceholder')}
                value={source}
                onChange={(e) => setSource(e.target.value)}
                disabled={saving}
                aria-describedby="extra-source-path-help"
              />
              <p id="extra-source-path-help" className="mt-1.5 text-xs leading-relaxed text-pencil-light/70">
                {t('extras.modal.sourcePathHelp')}
              </p>
            </div>

            {/* Targets */}
            <div>
              <label
                className="block text-base text-pencil-light mb-2"
              >
                {t('extras.modal.targets')}
              </label>
              <div className="space-y-3">
                {targets.map((tgt, i) => {
                  const fieldLabel = 'block text-xs font-medium text-pencil-light mb-1';
                  return (
                  <div key={i} className="rounded-[var(--radius-md)] border border-muted bg-muted/10 p-3 space-y-2.5">
                    {/* Path — full width, like Name / Source above */}
                    <div>
                      <label className={fieldLabel}>{t('extras.modal.colPath', {}, 'Path')}</label>
                      <Input
                        placeholder={t('extras.modal.targetPathPlaceholder')}
                        value={tgt.path}
                        onChange={(e) => updateTarget(i, 'path', e.target.value)}
                        disabled={saving}
                      />
                    </div>
                    {/* Extension · Mode · Flatten — second row with room to breathe */}
                    <div className="flex flex-wrap items-end gap-3">
                      <div className="w-44">
                        <label className={fieldLabel}>{t('extras.modal.colExtension', {}, 'Extension')}</label>
                        {availableExtensions.length > 0 || tgt.extension ? (
                          <Select
                            value={tgt.extension}
                            onChange={(v) => {
                              // selecting an extension forces copy mode
                              setTargets((prev) =>
                                prev.map((te, idx) =>
                                  idx === i ? { ...te, extension: v, ...(v ? { mode: 'copy' } : {}) } : te,
                                ),
                              );
                            }}
                            options={[
                              { value: '', label: t('extras.noExtension', {}, 'no extension') },
                              ...availableExtensions.map((e) => ({ value: e, label: e })),
                            ]}
                            disabled={saving}
                          />
                        ) : (
                          // None installed: guide the user to install one in Config.
                          <Link
                            to="/config?tab=extensions"
                            className="inline-flex items-center gap-1.5 py-2 text-sm text-blue hover:underline"
                          >
                            <Puzzle size={14} strokeWidth={2.5} className="shrink-0" />
                            {t('extras.installExtensionHint', {}, 'Install an extension')}
                          </Link>
                        )}
                      </div>
                      <div className="w-36">
                        <label className={fieldLabel}>{t('extras.modal.colMode', {}, 'Mode')}</label>
                        {tgt.extension ? (
                          // Extension forces copy mode: read-only locked chip, not a greyed-out select.
                          <div className="flex items-center gap-1.5 px-4 py-2 rounded-[var(--radius-sm)] border-2 border-muted bg-muted/40 text-sm text-pencil-light">
                            <Lock size={13} strokeWidth={2.5} className="shrink-0" />
                            <span>copy</span>
                          </div>
                        ) : (
                          <Select
                            value={tgt.mode}
                            onChange={(v) => {
                              updateTarget(i, 'mode', v);
                              if (v === 'symlink') updateTarget(i, 'flatten', false);
                            }}
                            options={MODE_OPTIONS}
                            disabled={saving}
                          />
                        )}
                      </div>
                      <Tooltip content={t('extras.flattenTitle')} side="bottom">
                        <span className="h-[2.6rem] flex items-center">
                          <Checkbox
                            label={t('extras.flatten')}
                            checked={tgt.flatten}
                            onChange={(c) => updateTarget(i, 'flatten', c)}
                            disabled={saving || tgt.mode === 'symlink'}
                            size="sm"
                          />
                        </span>
                      </Tooltip>
                      {targets.length > 1 && (
                        <div className="ml-auto h-[2.6rem] flex items-center">
                          <IconButton
                            icon={<X size={16} strokeWidth={2.5} />}
                            label={t('extras.removeTarget')}
                            size="sm"
                            variant="ghost"
                            onClick={() => removeTarget(i)}
                            disabled={saving}
                            className="hover:text-danger"
                          />
                        </div>
                      )}
                    </div>
                    {tgt.extension && (
                      <p className="flex items-center gap-1 text-xs text-pencil-light/70">
                        <Lock size={11} strokeWidth={2.5} className="shrink-0" />
                        {t('extras.modal.extensionLockedHint', {}, 'Extensions run in copy mode')}
                      </p>
                    )}
                  </div>
                  );
                })}
              </div>
              <Button
                variant="ghost"
                size="sm"
                onClick={addTarget}
                disabled={saving}
                className="mt-2"
              >
                <Plus size={14} strokeWidth={2.5} /> {t('extras.addTarget')}
              </Button>
            </div>
          </div>

          <div className="flex gap-3 justify-end mt-6">
            <Button variant="secondary" size="sm" onClick={onClose} disabled={saving}>
              {t('extras.cancel')}
            </Button>
            <Button variant="primary" size="sm" onClick={handleCreate} disabled={saving}>
              {saving ? t('extras.creating') : t('extras.create')}
            </Button>
          </div>
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
  availableExtensions,
}: {
  extra: Extra;
  index?: number;
  onSync: (name: string) => Promise<void>;
  onForceSync: (name: string) => Promise<void>;
  onRemove: (name: string) => void;
  onModeChange: (name: string, target: string, mode: string, flatten?: boolean, extension?: string) => Promise<void>;
  availableExtensions: string[];
}) {
  const t = useT();
  const sourceTypeLabel = extra.source_type === 'per-extra'
    ? t('extras.sourceType.custom')
    : extra.source_type === 'extras_source'
      ? t('extras.sourceType.shared')
      : '';
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
          {sourceTypeLabel && (
            <Badge variant="default">{sourceTypeLabel}</Badge>
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
                label: t('extras.forceSync'),
                icon: <Zap size={14} strokeWidth={2.5} />,
                onClick: () => handleSync(true),
                confirm: true,
              },
            ]}
          >
            <RefreshCw size={12} strokeWidth={2.5} />
            {syncing ? t('extras.syncing') : t('extras.sync')}
          </SplitButton>
          <IconButton
            icon={<Trash2 size={16} strokeWidth={2.5} />}
            label={t('extras.removeConfirm.title')}
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
          {extra.targets.length > 0 ? `${t('extras.modal.targets')} (${extra.targets.length})` : t('extras.modal.targets')}
        </span>
      </div>
      <div className="ml-5 mt-1 space-y-1.5">
        {extra.targets.length > 0 ? (
          extra.targets.map((tgt, ti) => (
            <div key={ti} className="flex items-center gap-3">
              <div className="flex items-center gap-2 min-w-0 flex-1">
                <span className="font-mono text-sm truncate text-pencil-light">{tgt.path}</span>
                <Badge
                  variant={
                    tgt.status === 'synced'
                      ? 'success'
                      : tgt.status === 'drift'
                      ? 'warning'
                      : 'danger'
                  }
                  size="sm"
                >
                  {tgt.status}
                </Badge>
              </div>
              <Tooltip content={t('extras.flattenTitle')} side="bottom">
                <span className="shrink-0">
                  <Checkbox
                    label={t('extras.flatten')}
                    checked={tgt.flatten}
                    onChange={async (c) => {
                      setChangingMode(tgt.path);
                      try {
                        await onModeChange(extra.name, tgt.path, tgt.mode, c);
                      } finally {
                        setChangingMode(null);
                      }
                    }}
                    disabled={changingMode === tgt.path || tgt.mode === 'symlink'}
                    size="sm"
                  />
                </span>
              </Tooltip>
              {availableExtensions.length > 0 || tgt.extension ? (
                <Select
                  value={tgt.extension ?? ''}
                  onChange={async (v) => {
                    if (v === (tgt.extension ?? '')) return;
                    setChangingMode(tgt.path);
                    try {
                      // selecting an extension forces copy; clearing keeps the current mode
                      await onModeChange(extra.name, tgt.path, v ? 'copy' : tgt.mode, undefined, v);
                    } finally {
                      setChangingMode(null);
                    }
                  }}
                  options={[
                    { value: '', label: t('extras.noExtension', {}, 'no extension') },
                    ...availableExtensions.map((e) => ({ value: e, label: e })),
                    ...(tgt.extension && !availableExtensions.includes(tgt.extension)
                      ? [{ value: tgt.extension, label: `${tgt.extension} (missing)` }]
                      : []),
                  ]}
                  size="sm"
                  className="w-40 shrink-0"
                  disabled={changingMode === tgt.path}
                />
              ) : (
                // None installed: guide the user to install one in Config.
                <Link
                  to="/config?tab=extensions"
                  className="shrink-0 inline-flex items-center gap-1.5 text-xs text-blue hover:underline"
                >
                  <Puzzle size={12} strokeWidth={2.5} className="shrink-0" />
                  {t('extras.installExtensionHint', {}, 'Install an extension')}
                </Link>
              )}
              {tgt.extension ? (
                // Extension forces copy mode: read-only locked chip, matching the Add Extra modal.
                <Tooltip content={t('extras.modal.extensionLockedHint', {}, 'Extensions run in copy mode')} side="bottom">
                  <div className="w-36 shrink-0 flex items-center gap-1.5 px-3 py-1.5 rounded-[var(--radius-sm)] border-2 border-muted bg-muted/40 text-xs text-pencil-light">
                    <Lock size={12} strokeWidth={2.5} className="shrink-0" />
                    <span>copy</span>
                  </div>
                </Tooltip>
              ) : (
                <Select
                  value={tgt.mode}
                  onChange={async (v) => {
                    if (v === tgt.mode) return;
                    setChangingMode(tgt.path);
                    try {
                      await onModeChange(extra.name, tgt.path, v);
                    } finally {
                      setChangingMode(null);
                    }
                  }}
                  options={MODE_OPTIONS}
                  size="sm"
                  className="w-36 shrink-0"
                  disabled={changingMode === tgt.path}
                />
              )}
            </div>
          ))
        ) : (
          <p className="text-sm text-pencil-light italic">{t('extras.noTargets')}</p>
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

type TFunc = (key: string, params?: Record<string, any>, fallback?: string) => string;

function buildSyncToast(label: string, failLabel: string, totals: SyncTotals, isForce: boolean, t: TFunc): string {
  if (totals.errors > 0 && totals.synced === 0)
    return `${failLabel} \u2014 ${t('extras.toast.nErrors', { errors: totals.errors }, `${totals.errors} error${totals.errors > 1 ? 's' : ''}`)}`;
  if (totals.synced === 0 && totals.skipped === 0 && totals.errors === 0)
    return `${label} \u2014 ${t('extras.toast.noFilesInSource', {}, 'no files in source')}`;
  const parts: string[] = [];
  parts.push(t('extras.toast.syncedNFiles', { synced: totals.synced, targets: totals.targets }, `${totals.synced} file${totals.synced !== 1 ? 's' : ''} to ${totals.targets} target${totals.targets !== 1 ? 's' : ''}`));
  if (totals.skipped > 0)
    parts.push(!isForce
      ? t('extras.toast.skippedForce', { skipped: totals.skipped }, `${totals.skipped} skipped (enable Force to override)`)
      : t('extras.toast.skipped', { skipped: totals.skipped }, `${totals.skipped} skipped`));
  if (totals.errors > 0)
    parts.push(t('extras.toast.nErrors', { errors: totals.errors }, `${totals.errors} error${totals.errors > 1 ? 's' : ''}`));
  return `${label} \u2014 ${parts.join(', ')}`;
}

// ─── ExtrasPage ───────────────────────────────────────────────────────────────

export default function ExtrasPage() {
  const { isProjectMode } = useAppContext();
  const { toast } = useToast();
  const tr = useT();
  const queryClient = useQueryClient();

  const { data, isPending, error } = useQuery({
    queryKey: queryKeys.extras,
    queryFn: () => api.listExtras(),
    staleTime: staleTimes.extras,
  });

  // Available transform extensions for the current mode (-g/-p), used to
  // populate the per-target extension picker.
  const { data: extData } = useQuery({
    queryKey: ['extras', 'extensions'],
    queryFn: () => api.listExtraExtensions(),
    staleTime: staleTimes.extras,
  });
  const availableExtensions = extData?.extensions ?? [];

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
      const totals = sumAll(res.extras);
      toast(buildSyncToast(tr('extras.toast.syncAll'), tr('extras.toast.syncAllFailed'), totals, force, tr), syncToastType(totals));
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
      const totals = sumEntry(entry);
      toast(buildSyncToast(tr('extras.toast.syncOne', { name }), tr('extras.toast.syncOneFailed', { name }), totals, force, tr), syncToastType(totals));
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
      toast(tr('extras.toast.removed', { name: removeName }), 'success');
      invalidate();
    } catch (err: any) {
      toast(err.message, 'error');
    } finally {
      setRemoving(false);
      setRemoveName(null);
    }
  };

  const handleModeChange = async (name: string, target: string, mode: string, flatten?: boolean, extension?: string) => {
    // Optimistically patch the cache so the dropdown reflects the choice
    // immediately, instead of waiting for the PATCH + refetch round-trip.
    const prev = queryClient.getQueryData<{ extras: Extra[] }>(queryKeys.extras);
    queryClient.setQueryData<{ extras: Extra[] }>(queryKeys.extras, (old) =>
      old
        ? {
            extras: old.extras.map((e) =>
              e.name !== name
                ? e
                : {
                    ...e,
                    targets: e.targets.map((tg) =>
                      tg.path !== target
                        ? tg
                        : {
                            ...tg,
                            mode,
                            ...(flatten !== undefined && { flatten }),
                            ...(extension !== undefined && { extension }),
                          },
                    ),
                  },
            ),
          }
        : old,
    );
    try {
      await api.setExtraMode(name, target, mode, flatten, extension);
      let msg: string;
      if (extension !== undefined) {
        msg = extension
          ? tr('extras.toast.extensionChanged', { extension }, `Extension set to ${extension} (copy mode)`)
          : tr('extras.toast.extensionCleared', {}, 'Extension cleared');
      } else if (flatten !== undefined) {
        msg = tr('extras.toast.flattenChanged', { flatten: String(flatten) });
      } else {
        msg = tr('extras.toast.modeChanged', { mode });
      }
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
        title={tr('extras.title')}
        subtitle={isProjectMode
          ? tr('extras.subtitle.project')
          : tr('extras.subtitle.global')}
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
                    label: tr('extras.syncForceSyncAll'),
                    icon: <Zap size={14} strokeWidth={2.5} />,
                    onClick: () => handleSyncAll(true),
                    confirm: true,
                  },
                ]}
              >
                <RefreshCw size={14} strokeWidth={2.5} />
                {syncingAll ? tr('extras.syncing') : tr('extras.syncAll')}
              </SplitButton>
            )}
            <Button variant="primary" size="sm" onClick={() => setShowAdd(true)}>
              <Plus size={14} strokeWidth={2.5} /> {tr('extras.addExtra')}
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
              title={tr('extras.empty.title')}
              description={tr('extras.empty.description')}
              action={
                <Button variant="primary" size="md" onClick={() => setShowAdd(true)}>
                  <Plus size={16} strokeWidth={2.5} /> {tr('extras.addExtra')}
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
                  availableExtensions={availableExtensions}
                />
              ))}
            </div>
          )}
        </div>
      )}

      {/* Add Extra modal */}
      {showAdd && (
        <AddExtraModal
          onClose={() => setShowAdd(false)}
          onCreated={handleCreated}
          availableExtensions={availableExtensions}
        />
      )}

      {/* Remove confirm dialog */}
      <ConfirmDialog
        open={removeName !== null}
        title={tr('extras.removeConfirm.title')}
        message={
          removeName ? (
            <span>
              {tr('extras.removeConfirm.message', { name: removeName })}
            </span>
          ) : (
            <span />
          )
        }
        confirmText={tr('extras.removeConfirm.confirmText')}
        variant="danger"
        loading={removing}
        onConfirm={handleRemove}
        onCancel={() => setRemoveName(null)}
      />
    </div>
  );
}
