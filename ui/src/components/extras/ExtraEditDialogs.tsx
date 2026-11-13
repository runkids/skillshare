import { useState, type ReactNode } from 'react';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { X } from 'lucide-react';
import { api } from '../../api/client';
import type { Extra, ExtraEditResult, ExtraTarget } from '../../api/client';
import { queryKeys } from '../../lib/queryKeys';
import { buildSyncToast, sumEntry, syncToastType } from '../../lib/extrasSyncToast';
import { shortenHome } from '../../lib/paths';
import { useToast } from '../Toast';
import { useT, plural } from '../../i18n';
import Button from '../Button';
import DialogShell from '../DialogShell';
import { Select } from '../Input';
import FilterSection from '../targets/FilterSection';

/** <folder>/<file>, for a single-file extra's source or target file, with the separator the folder already uses (a backslash on Windows). */
export const joinFile = (dir: string, file: string) => {
  const sep = dir.lastIndexOf('\\') > dir.lastIndexOf('/') ? '\\' : '/';
  return `${dir.replace(/[\\/]+$/, '')}${sep}${file}`;
};

export const MODES = ['merge', 'copy', 'symlink'] as const;
// A single file can't be a directory symlink; import writes an @ line instead.
export const FILE_MODES = ['merge', 'copy', 'import', 'prepend', 'append'] as const;

/** The target fields whose rules Add target and Edit target share. */
export interface TargetFields {
  mode: string;
  flatten: boolean;
  extension: string;
}

/** Applies a field change with the rules that follow from it: an extension converts each file, so it always writes copies; symlink links the whole folder, so flatten turns off. */
export function applyDraftChange<T extends TargetFields>(draft: T, patch: Partial<TargetFields>): T {
  const next = { ...draft, ...patch };
  if (patch.extension) next.mode = 'copy';
  if (next.mode === 'symlink') next.flatten = false;
  return next;
}

const Mono = ({ children }: { children: ReactNode }) => <span className="font-mono">{children}</span>;

/** Reports a save: the sync it ran when files moved, else a plain saved toast. */
function useSaveToast() {
  const { toast } = useToast();
  const t = useT();
  return (name: string, res: ExtraEditResult) => {
    if (res.prune_errors?.length) toast(t('extras.edit.pruneFailed', { error: res.prune_errors.join('; ') }), 'error');
    if (!res.extras) {
      toast(t('extras.edit.saved', { name }), 'success');
      return;
    }
    const totals = sumEntry(res.extras.find((e) => e.name === name));
    toast(buildSyncToast(t('extras.toast.syncOne', { name }), t('extras.toast.syncOneFailed', { name }), totals, false, t), syncToastType(totals));
  };
}

function DialogFrame({ title, subtitle, saving, canSave, saveLabel, onClose, onSave, children }: {
  title: string;
  subtitle: ReactNode;
  saving: boolean;
  canSave: boolean;
  saveLabel: string;
  onClose: () => void;
  onSave: () => void;
  children: ReactNode;
}) {
  const t = useT();
  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={saving} ariaLabel={title} className="!max-w-[720px]">
      <div className="dh">
        <div className="flex min-w-0 flex-col gap-1">
          <h2 className="ss-h2">{title}</h2>
          <p className="truncate text-[13px] text-ink-2">{subtitle}</p>
        </div>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={saving}><X size={16} /></button>
      </div>
      <div className="db">{children}</div>
      <div className="df">
        <span className="flex-1" />
        <Button variant="ghost" onClick={onClose} disabled={saving}>{t('extras.cancel')}</Button>
        <Button variant="primary" loading={saving} disabled={!canSave} onClick={onSave}>{saveLabel}</Button>
      </div>
    </DialogShell>
  );
}

/** Where a target syncs, how, and which files: everything Add target sets, plus the file filters. */
export function EditTargetDialog({ extra, target, extensions, markFor, onClose, onSaved }: {
  extra: Extra;
  target: ExtraTarget;
  extensions: string[];
  markFor: (path: string) => ReactNode;
  onClose: () => void;
  onSaved: () => void;
}) {
  const t = useT();
  const { toast } = useToast();
  const report = useSaveToast();
  const single = Boolean(extra.file);
  const [path, setPath] = useState(target.path);
  const [fields, setFields] = useState<TargetFields>({ mode: target.mode, flatten: target.flatten, extension: target.extension ?? '' });
  const { mode, flatten, extension } = fields;
  const change = (patch: Partial<TargetFields>) => setFields(applyDraftChange(fields, patch));
  const [as, setAs] = useState(target.as ?? '');
  const [filters, setFilters] = useState({ include: target.include ?? [], exclude: target.exclude ?? [] });
  const [saving, setSaving] = useState(false);

  const filtered = !single && mode !== 'symlink';
  const preview = useQuery({
    queryKey: queryKeys.extrasPreview(extra.name, filters.include, filters.exclude),
    queryFn: () => api.previewExtraFilter(extra.name, filters.include, filters.exclude),
    placeholderData: keepPreviousData,
    enabled: filtered && extra.source_exists,
  });
  const files = preview.data?.files ?? [];
  const entries = files.map((f) => ({ skill: f.file, target: path, status: f.status, reason: f.reason ?? '' }));
  const dropped = files.filter((f) => f.status !== 'synced').length;
  const unmatched = preview.data?.unmatched ?? [];

  const trimmed = path.trim();
  const moved = trimmed !== target.path || as.trim() !== (target.as ?? '');
  const patterns = (target.include?.length ?? 0) + (target.exclude?.length ?? 0);
  const flat = flatten && mode !== 'symlink';
  const filtersChanged = filters.include.join('\n') !== (target.include ?? []).join('\n') || filters.exclude.join('\n') !== (target.exclude ?? []).join('\n');
  const canSave = trimmed !== '' && !saving;

  const save = async () => {
    setSaving(true);
    try {
      const res = await api.editExtraTarget(extra.name, target.path, {
        path: trimmed, mode: extension ? 'copy' : mode, flatten: flat, extension, as: as.trim(),
        include: filtered ? filters.include : [], exclude: filtered ? filters.exclude : [],
      });
      report(extra.name, res);
      onSaved();
    } catch (err) {
      toast((err as Error).message, 'error');
      setSaving(false);
    }
  };

  const was = (value: string) => <span className="hp font-mono">{t('extras.edit.was', { value: shortenHome(value) })}</span>;

  return (
    <DialogFrame
      title={t('extras.edit.targetTitle')}
      subtitle={<><Mono>{extra.name}</Mono> → <Mono>{shortenHome(target.path)}</Mono></>}
      saving={saving}
      canSave={canSave}
      saveLabel={t(moved ? 'extras.edit.saveAndSync' : 'common.save')}
      onClose={onClose}
      onSave={() => void save()}
    >
      <div className="flex items-start gap-3">
        <span className="mt-[27px]">{markFor(trimmed)}</span>
        <div className="ss-fld min-w-0 flex-1">
          <label htmlFor="edit-target-path">{t('extras.modal.colPath')}</label>
          <span className="ss-inp"><input id="edit-target-path" className="font-mono" value={path} onChange={(e) => setPath(e.target.value)} disabled={saving} /></span>
          {trimmed !== target.path && was(target.path)}
        </div>
      </div>
      <div className="flex flex-wrap items-start gap-3">
        {single ? (
          <div className="ss-fld min-w-0 flex-1">
            <label htmlFor="edit-target-as">{t('extras.modal.fileName')}</label>
            <span className="ss-inp"><input id="edit-target-as" className="font-mono" value={as} placeholder={extra.file} onChange={(e) => setAs(e.target.value)} disabled={saving} /></span>
          </div>
        ) : (
          <div className="ss-fld min-w-0 flex-1">
            <span className="text-[13px] font-semibold">{t('extras.modal.colExtension')}</span>
            <Select
              value={extension}
              onChange={(v) => change({ extension: v })}
              options={[{ value: '', label: t('extras.noExtension') }, ...extensions.map((e) => ({ value: e, label: e }))]}
              disabled={saving || mode === 'symlink' || (extensions.length === 0 && !extension)}
            />
          </div>
        )}
        <div className="ss-fld w-[180px]">
          <span className="text-[13px] font-semibold">{t('extras.modal.colMode')}</span>
          <Select
            value={extension ? 'copy' : mode}
            onChange={(v) => change({ mode: v })}
            options={(single ? FILE_MODES : MODES).map((m) => ({ value: m, label: m, description: t(single ? `extras.fileModeDescription.${m}` : `extras.modeDescription.${m}`) }))}
            disabled={saving || Boolean(extension)}
          />
        </div>
        {!single && (
          <div className="ss-fld w-[110px]">
            <span className="text-[13px] font-semibold">{t('extras.flatten')}</span>
            <span className="flex h-9 items-center gap-2">
              <button type="button" role="switch" aria-checked={flat} aria-label={t('extras.flatten')} className={`ss-sw ${flat ? 'on' : ''} disabled:opacity-50`} onClick={() => change({ flatten: !flatten })} disabled={saving || mode === 'symlink'}><i /></button>
              <span className="text-xs text-ink-2">{t(flat ? 'extras.flattenOn' : 'extras.flattenOff')}</span>
            </span>
          </div>
        )}
      </div>
      {moved && (
        <div className="ss-note inf">
          <span className="flex-1">{t(target.mode === 'copy' ? 'extras.edit.moveCopyNote' : 'extras.edit.moveNote', { from: shortenHome(target.path), to: shortenHome(trimmed) })}</span>
        </div>
      )}

      {!single && (
        <div className="mt-2 flex flex-col gap-4 border-t border-line pt-5">
          <h3 className="text-[14px] font-semibold">{t('extras.edit.files')}</h3>
          {mode === 'symlink' ? (
            <div className="ss-note inf">
              <span className="flex-1">{patterns > 0 ? t(plural('extras.edit.symlinkClears', patterns), { count: patterns }) : t('targetDetail.symlinkNoFilters')}</span>
            </div>
          ) : (
            <>
              <FilterSection
                kind="file"
                mode={mode}
                name={shortenHome(trimmed)}
                include={filters.include}
                exclude={filters.exclude}
                onChange={setFilters}
                entries={entries}
                loaded={preview.isSuccess}
                loading={preview.isPending && preview.fetchStatus !== 'idle'}
                error={preview.error}
                disabled={saving}
                includeWarning={unmatched.length > 0 ? (
                  <div className="ss-note warn">
                    <span className="flex-1">{unmatched.map((p) => t('extras.edit.unmatched', { pattern: p, name: extra.name })).join(' ')}</span>
                  </div>
                ) : undefined}
              />
              {dropped > 0 && filtersChanged && !moved && (
                <div className="ss-note">
                  <span className="flex-1">{mode === 'copy'
                    ? t('extras.edit.copyNote', { path: shortenHome(target.path) })
                    : t(plural('extras.edit.pruneNote', dropped), { count: dropped, name: extra.name })}</span>
                </div>
              )}
            </>
          )}
        </div>
      )}
    </DialogFrame>
  );
}

/** Name and source folder of an extra. A new folder relinks every target; a new name changes only the config. */
export function EditExtraDialog({ extra, sharedDir, onClose, onSaved }: {
  extra: Extra;
  sharedDir: string;
  onClose: () => void;
  onSaved: () => void;
}) {
  const t = useT();
  const { toast } = useToast();
  const report = useSaveToast();
  const [name, setName] = useState(extra.name);
  const wasCustom = extra.source_type === 'per-extra';
  const [custom, setCustom] = useState(wasCustom);
  const [source, setSource] = useState(wasCustom ? shortenHome(extra.source_dir) : '');
  const [saving, setSaving] = useState(false);

  const remote = Boolean(extra.remote);
  const trimmedName = name.trim();
  const trimmedSource = source.trim();
  const renamed = trimmedName !== extra.name;
  // Back to the shared folder means the folder named after the extra there.
  const sourceChanged = custom ? trimmedSource !== '' && trimmedSource !== shortenHome(extra.source_dir) : wasCustom;
  const canSave = !remote && trimmedName !== '' && (renamed || sourceChanged) && !(custom && trimmedSource === '') && !saving;

  const save = async () => {
    if (remote || !canSave) return;
    setSaving(true);
    try {
      const res = await api.editExtra(extra.name, {
        ...(renamed && { name: trimmedName }),
        ...(sourceChanged && { source: custom ? trimmedSource : '' }),
      });
      report(trimmedName, res);
      onSaved();
    } catch (err) {
      toast((err as Error).message, 'error');
      setSaving(false);
    }
  };

  const sharedPath = joinFile(shortenHome(sharedDir), wasCustom ? trimmedName || '…' : extra.name);
  const newPath = custom ? trimmedSource : sharedPath;

  return (
    <DialogFrame
      title={t('extras.edit.extraTitle')}
      subtitle={<Mono>{shortenHome(extra.file ? joinFile(extra.source_dir, extra.file) : extra.source_dir)}</Mono>}
      saving={saving}
      canSave={canSave}
      saveLabel={t(sourceChanged ? 'extras.edit.saveAndSync' : 'common.save')}
      onClose={onClose}
      onSave={() => void save()}
    >
      {remote && <div className="ss-note inf"><span className="flex-1">{t('extras.remote.identityLocked')}</span></div>}
      <div className="ss-fld">
        <label htmlFor="edit-extra-name">{t('extras.modal.name')}</label>
        <span className="ss-inp"><input id="edit-extra-name" className="font-mono" value={name} onChange={(e) => setName(e.target.value)} disabled={saving || remote} /></span>
        {renamed && <span className="hp font-mono">{t('extras.edit.was', { value: extra.name })}</span>}
      </div>
      <div className="ss-fld">
        <span className="text-[13px] font-semibold">{t('extras.modal.source')}</span>
        <Select
          value={custom ? 'custom' : 'shared'}
          onChange={(v) => setCustom(v === 'custom')}
          options={[
            { value: 'shared', label: t('extras.sourceType.shared') },
            { value: 'custom', label: t('extras.sourceType.custom') },
          ]}
          disabled={saving || remote}
        />
        {custom ? (
          <span className="ss-inp">
            <input className="font-mono" value={source} onChange={(e) => setSource(e.target.value)} placeholder={t('extras.modal.sourcePathPlaceholder')} aria-label={t('extras.sourceType.custom')} disabled={saving || remote} />
          </span>
        ) : (
          <span className="hp truncate font-mono">{sharedPath}</span>
        )}
        {sourceChanged && <span className="hp font-mono">{t('extras.edit.was', { value: shortenHome(extra.source_dir) })}</span>}
      </div>
      {sourceChanged ? (
        <div className="ss-note inf"><span className="flex-1">{t('extras.edit.relinkNote', { path: newPath })}</span></div>
      ) : renamed && (
        <div className="ss-note"><span className="flex-1">{t('extras.edit.renameNote', { path: shortenHome(extra.source_dir) })}</span></div>
      )}
    </DialogFrame>
  );
}
