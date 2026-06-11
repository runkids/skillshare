import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { FolderOpen, FolderPlus, Lock, Plus, Puzzle, RefreshCw, Target, Trash2, X, Zap } from 'lucide-react';
import { api } from '../api/client';
import type { Extra } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { useAppContext } from '../context/AppContext';
import { useToast } from '../components/Toast';
import Card from '../components/Card';
import Button from '../components/Button';
import IconButton from '../components/IconButton';
import SplitButton from '../components/SplitButton';
import DialogShell from '../components/DialogShell';
import { Input, Select, type SelectOption } from '../components/Input';
import Badge from '../components/Badge';
import EmptyState from '../components/EmptyState';
import PageHeader from '../components/PageHeader';
import ConfirmDialog from '../components/ConfirmDialog';
import { PageSkeleton } from '../components/Skeleton';
import Tooltip from '../components/Tooltip';
import { useT } from '../i18n';
import { buildSyncToast, sumAll, sumEntry, syncToastType } from '../lib/extrasSyncToast';

// ─── AddExtraModal ────────────────────────────────────────────────────────────

interface TargetEntry {
  id: string; // stable React key — paths can be empty/duplicate while editing
  path: string;
  mode: string;
  flatten: boolean;
  extension: string;
}

const newTarget = (): TargetEntry => ({
  id: crypto.randomUUID(),
  path: '',
  mode: 'merge',
  flatten: false,
  extension: '',
});

type TFunction = ReturnType<typeof useT>;

const MODE_VALUES = ['merge', 'copy', 'symlink'] as const;

type ModeValue = (typeof MODE_VALUES)[number];

function isModeValue(mode: string): mode is ModeValue {
  return (MODE_VALUES as readonly string[]).includes(mode);
}

function formatMode(t: TFunction, mode: string) {
  return isModeValue(mode) ? t(`extras.mode.${mode}`) : mode;
}

function getModeOptions(t: TFunction): SelectOption[] {
  return MODE_VALUES.map((mode) => ({
    value: mode,
    label: formatMode(t, mode),
    description: t(`extras.modeDescription.${mode}`),
  }));
}

// Flatten is a boolean, but rendered as a Select so it visually matches the
// Extension / Mode dropdowns it sits beside.
function getFlattenOptions(t: TFunction): SelectOption[] {
  return [
    { value: 'on', label: t('extras.flattenOn', {}, 'On'), description: t('extras.flattenTitle') },
    { value: 'off', label: t('extras.flattenOff', {}, 'Off') },
  ];
}

function formatTargetStatus(t: TFunction, status: string) {
  switch (status) {
    case 'synced':
      return t('extras.status.synced');
    case 'drift':
      return t('extras.status.drift');
    case 'not synced':
      return t('extras.status.notSynced');
    case 'no source':
      return t('extras.status.noSource');
    default:
      return status;
  }
}

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
  const [targets, setTargets] = useState<TargetEntry[]>(() => [newTarget()]);
  const [saving, setSaving] = useState(false);
  const modeOptions = getModeOptions(t);
  const flattenOptions = getFlattenOptions(t);

  const addTarget = () => setTargets((prev) => [...prev, newTarget()]);

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
                  <div key={tgt.id} className="rounded-[var(--radius-md)] border border-muted bg-muted/10 p-3 space-y-2.5">
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
                            <span>{formatMode(t, 'copy')}</span>
                          </div>
                        ) : (
                          <Select
                            value={tgt.mode}
                            onChange={(v) => {
                              updateTarget(i, 'mode', v);
                              if (v === 'symlink') updateTarget(i, 'flatten', false);
                            }}
                            options={modeOptions}
                            disabled={saving}
                          />
                        )}
                      </div>
                      <div className="w-32">
                        <label className={fieldLabel}>{t('extras.flatten')}</label>
                        <Select
                          value={tgt.flatten ? 'on' : 'off'}
                          onChange={(v) => updateTarget(i, 'flatten', v === 'on')}
                          options={flattenOptions}
                          disabled={saving || tgt.mode === 'symlink'}
                        />
                      </div>
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

// ─── AddTargetRow ─────────────────────────────────────────────────────────────

function AddTargetRow({
  onAdd,
  onCancel,
}: {
  onAdd: (path: string, mode: string) => Promise<void>;
  onCancel: () => void;
}) {
  const t = useT();
  const [path, setPath] = useState('');
  const [mode, setMode] = useState('merge');
  const [busy, setBusy] = useState(false);
  const modeOptions = getModeOptions(t);
  return (
    <div className="mt-2 flex items-center gap-2 rounded-[var(--radius-md)] border border-dashed border-pencil-light/30 bg-muted/10 p-2">
      <div className="flex-1 min-w-0">
        <Input
          value={path}
          onChange={(e) => setPath(e.target.value)}
          placeholder={t('extras.modal.targetPathPlaceholder')}
          size="sm"
          autoFocus
        />
      </div>
      <Select
        value={mode}
        onChange={(v) => setMode(v)}
        options={modeOptions}
        size="sm"
        className="w-32 shrink-0"
      />
      <Button
        variant="ghost"
        size="sm"
        disabled={busy || path.trim() === ''}
        onClick={async () => {
          setBusy(true);
          try {
            await onAdd(path.trim(), mode);
            setPath('');
          } finally {
            setBusy(false);
          }
        }}
      >
        <Plus size={14} strokeWidth={2.5} /> {t('extras.addTarget')}
      </Button>
      <IconButton
        icon={<X size={16} strokeWidth={2.5} />}
        label={t('extras.cancel')}
        size="sm"
        variant="ghost"
        onClick={onCancel}
        disabled={busy}
      />
    </div>
  );
}

// ─── ExtraCard ────────────────────────────────────────────────────────────────

function ExtraCard({
  extra,
  onSync,
  onForceSync,
  onRemove,
  onModeChange,
  onAddTarget,
  onRemoveTarget,
  availableExtensions,
}: {
  extra: Extra;
  index?: number;
  onSync: (name: string) => Promise<void>;
  onForceSync: (name: string) => Promise<void>;
  onRemove: (name: string) => void;
  onModeChange: (name: string, target: string, mode: string, flatten?: boolean, extension?: string) => Promise<void>;
  onAddTarget: (name: string, path: string, mode: string) => Promise<void>;
  onRemoveTarget: (name: string, path: string) => Promise<void>;
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
  const [addingTarget, setAddingTarget] = useState(false);
  const [confirmRemoveTarget, setConfirmRemoveTarget] = useState<string | null>(null);
  const modeOptions = getModeOptions(t);
  const flattenOptions = getFlattenOptions(t);

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
    <Card overflow className="hover:shadow-md">
      {/* Header */}
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-2.5 min-w-0">
          <span className="shrink-0 inline-flex items-center justify-center w-8 h-8 rounded-[var(--radius-md)] bg-blue/10 text-blue">
            <FolderPlus size={16} strokeWidth={2.5} />
          </span>
          <div className="min-w-0 flex items-center gap-2 flex-wrap">
            <span className="font-bold text-pencil truncate">{extra.name}</span>
            <Badge variant={extra.source_exists ? 'success' : 'warning'} size="sm">
              {t('extras.fileCount', { count: extra.file_count })}
            </Badge>
            {!extra.source_exists && (
              <Badge variant="danger" size="sm">{t('extras.sourceMissing')}</Badge>
            )}
            {sourceTypeLabel && (
              <span className="text-xs text-pencil-light/60">· {sourceTypeLabel}</span>
            )}
          </div>
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
          <span className="w-px h-5 bg-pencil-light/20 mx-0.5" aria-hidden="true" />
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
      <div className="mt-4">
        <div className="flex items-center gap-1.5 mb-1.5">
          <FolderOpen size={12} strokeWidth={2.5} className="text-warning shrink-0" />
          <span className="text-[10px] font-medium text-pencil-light/70 uppercase tracking-wider">{t('preview.source')}</span>
        </div>
        <div className="rounded-[var(--radius-md)] border border-muted bg-muted/20 px-3 py-2">
          <p className="font-mono text-sm text-pencil-light truncate">{extra.source_dir}</p>
        </div>
      </div>

      {/* Targets */}
      <div className="flex items-center gap-1.5 mt-4 mb-1">
        <Target size={12} strokeWidth={2.5} className="text-success shrink-0" />
        <span className="text-[10px] font-medium text-pencil-light/70 uppercase tracking-wider">{t('extras.modal.targets')}</span>
        {extra.targets.length > 0 && (
          <span className="text-[10px] text-pencil-light/45 tabular-nums">· {extra.targets.length}</span>
        )}
      </div>
      <div className="mt-1">
        {extra.targets.length > 0 ? (
          <>
            {/* Column headers — label the per-target controls so the dropdowns are self-explanatory */}
            <div className="flex items-center gap-3 px-3 pb-1 text-[10px] font-medium uppercase tracking-wider text-pencil-light/45">
              <div className="min-w-0 flex-1" />
              <div className="w-28 shrink-0">{t('extras.flatten')}</div>
              <div className="w-40 shrink-0">{t('extras.modal.colExtension')}</div>
              <div className="w-36 shrink-0">{t('extras.modal.colMode')}</div>
              {extra.targets.length > 1 && <div className="w-8 shrink-0" />}
            </div>
            <div className="space-y-0.5">
              {extra.targets.map((tgt, ti) => (
                <div
                  key={`${tgt.path}::${ti}`}
                  className="group/row flex items-center gap-3 rounded-[var(--radius-md)] px-3 py-2 transition-colors hover:bg-muted/25"
                >
                  {/* Data: path + status (status omitted when the source itself is missing) */}
                  <div className="flex items-center gap-2 min-w-0 flex-1">
                    <span className="font-mono text-sm truncate text-pencil">{tgt.path}</span>
                    {extra.source_exists && (
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
                        {formatTargetStatus(t, tgt.status)}
                      </Badge>
                    )}
                  </div>
                  {/* Settings: flatten · extension · mode */}
                  <div className="w-28 shrink-0">
                    <Select
                      value={tgt.flatten ? 'on' : 'off'}
                      onChange={async (v) => {
                        const next = v === 'on';
                        if (next === tgt.flatten) return;
                        setChangingMode(tgt.path);
                        try {
                          await onModeChange(extra.name, tgt.path, tgt.mode, next);
                        } finally {
                          setChangingMode(null);
                        }
                      }}
                      options={flattenOptions}
                      size="sm"
                      className="w-full"
                      disabled={changingMode === tgt.path || tgt.mode === 'symlink'}
                    />
                  </div>
                  <div className="w-40 shrink-0">
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
                            ? [{ value: tgt.extension, label: t('extras.extensionMissing', { extension: tgt.extension }) }]
                            : []),
                        ]}
                        size="sm"
                        className="w-full"
                        disabled={changingMode === tgt.path}
                      />
                    ) : (
                      // None installed: guide the user to install one in Config.
                      <Link
                        to="/config?tab=extensions"
                        className="inline-flex items-center gap-1.5 text-xs text-blue hover:underline"
                      >
                        <Puzzle size={12} strokeWidth={2.5} className="shrink-0" />
                        {t('extras.installExtensionHint', {}, 'Install an extension')}
                      </Link>
                    )}
                  </div>
                  <div className="w-36 shrink-0">
                    {tgt.extension ? (
                      // Extension forces copy mode: read-only locked chip, matching the Add Extra modal.
                      <Tooltip content={t('extras.modal.extensionLockedHint', {}, 'Extensions run in copy mode')} side="bottom">
                        <div className="w-full flex items-center gap-1.5 px-3 py-1.5 rounded-[var(--radius-sm)] border-2 border-muted bg-muted/40 text-xs text-pencil-light">
                          <Lock size={12} strokeWidth={2.5} className="shrink-0" />
                          <span>{formatMode(t, 'copy')}</span>
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
                        options={modeOptions}
                        size="sm"
                        className="w-full"
                        disabled={changingMode === tgt.path}
                      />
                    )}
                  </div>
                  {extra.targets.length > 1 && (
                    <div className="w-8 shrink-0 flex justify-end">
                      <IconButton
                        icon={<Trash2 size={14} strokeWidth={2.5} />}
                        label={t('extras.removeTarget')}
                        size="sm"
                        variant="ghost"
                        onClick={() => setConfirmRemoveTarget(tgt.path)}
                        className="hover:text-danger"
                      />
                    </div>
                  )}
                </div>
              ))}
            </div>
          </>
        ) : (
          <p className="text-sm text-pencil-light/70 italic px-3 py-2">{t('extras.noTargets')}</p>
        )}
      </div>
      {addingTarget ? (
        <AddTargetRow
          onAdd={async (path, mode) => {
            await onAddTarget(extra.name, path, mode);
            setAddingTarget(false);
          }}
          onCancel={() => setAddingTarget(false)}
        />
      ) : (
        <Button
          variant="ghost"
          size="sm"
          className="mt-1"
          onClick={() => setAddingTarget(true)}
        >
          <Plus size={14} strokeWidth={2.5} /> {t('extras.addTarget')}
        </Button>
      )}

      <ConfirmDialog
        open={confirmRemoveTarget !== null}
        title={t('extras.removeTargetConfirm.title')}
        message={
          confirmRemoveTarget ? (
            <span>{t('extras.removeTargetConfirm.message', { path: confirmRemoveTarget })}</span>
          ) : (
            <span />
          )
        }
        confirmText={t('extras.removeConfirm.confirmText')}
        variant="danger"
        onConfirm={async () => {
          if (confirmRemoveTarget) await onRemoveTarget(extra.name, confirmRemoveTarget);
          setConfirmRemoveTarget(null);
        }}
        onCancel={() => setConfirmRemoveTarget(null)}
      />
    </Card>
  );
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
        msg = tr('extras.toast.modeChanged', { mode: formatMode(tr, mode) });
      }
      toast(msg, 'success');
      invalidate();
    } catch (err: any) {
      // Roll back the optimistic cache update on failure.
      if (prev) queryClient.setQueryData(queryKeys.extras, prev);
      toast(err.message, 'error');
    }
  };

  const handleAddTarget = async (name: string, path: string, mode: string) => {
    try {
      await api.addExtraTarget(name, { path, mode });
      toast(tr('extras.toast.targetAdded', { path }, `Added target ${path}`), 'success');
      invalidate();
    } catch (err: any) {
      toast(err.message, 'error');
    }
  };

  const handleRemoveTarget = async (name: string, path: string) => {
    try {
      await api.removeExtraTarget(name, path);
      toast(tr('extras.toast.targetRemoved', { path }, `Removed target ${path}`), 'success');
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
                  onAddTarget={handleAddTarget}
                  onRemoveTarget={handleRemoveTarget}
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
