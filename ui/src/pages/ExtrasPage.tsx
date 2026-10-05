import { useEffect, useMemo, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Ellipsis, FileText, FoldVertical, Folder, FolderPlus, Link2, Plus, Puzzle, RefreshCw, Trash2, X, Zap } from 'lucide-react';
import { api } from '../api/client';
import type { AvailableTarget, Extra, ExtraTarget } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { useAppContext } from '../context/AppContext';
import { useToast } from '../components/Toast';
import AgentIcon from '../components/AgentIcon';
import Button from '../components/Button';
import MemoryNotes from '../components/memory/MemoryNotes';
import DialogShell from '../components/DialogShell';
import { Select } from '../components/Input';
import EmptyState from '../components/EmptyState';
import PageHeader from '../components/PageHeader';
import ConfirmDialog from '../components/ConfirmDialog';
import { PageSkeleton } from '../components/Skeleton';
import { SkillContextMenu, type ContextMenuItem } from '../components/TargetMenu';
import { useT } from '../i18n';
import { buildSyncToast, sumEntry, syncToastType } from '../lib/extrasSyncToast';
import { shortenHome } from '../lib/paths';
import ProjectInstructions from '../components/instructions/ProjectInstructions';
import SharedInstructions from '../components/instructions/SharedInstructions';
import { isAgentsExtra } from '../components/instructions/instructionsView';
import { useAvailableTargetsQuery, useOverviewQuery } from '../hooks/useSharedQueries';

const MODES = ['merge', 'copy', 'symlink'] as const;
// A single file can't be a directory symlink; import writes an @ line instead.
const FILE_MODES = ['merge', 'copy', 'import'] as const;

/** A file name typed where a path would be wrong. */
const isPathLike = (name: string) => /[\\/]/.test(name);

/** <folder>/<file>, for a single-file extra's source or target file, with the separator the folder already uses (a backslash on Windows). */
const joinFile = (dir: string, file: string) => {
  const sep = dir.lastIndexOf('\\') > dir.lastIndexOf('/') ? '\\' : '/';
  return `${dir.replace(/[\\/]+$/, '')}${sep}${file}`;
};

const STATUS: Record<string, { tone: string; labelKey: string }> = {
  synced: { tone: 'ok', labelKey: 'extras.status.synced' },
  drift: { tone: 'warn', labelKey: 'extras.status.drift' },
  modified: { tone: 'warn', labelKey: 'extras.status.modified' },
  'not synced': { tone: 'warn', labelKey: 'extras.status.notSynced' },
  'no source': { tone: 'bad', labelKey: 'extras.status.sourceMissing' },
};

/** The tool a target folder belongs to: the known target whose home folder (e.g. ~/.claude) holds the path. */
function targetOf(path: string, known: AvailableTarget[]): string | null {
  const p = shortenHome(path.replace(/\/+$/, ''));
  let best: { name: string; len: number } | null = null;
  for (const k of known) {
    const dir = shortenHome(k.path).replace(/\/[^/]+\/?$/, '');
    if (dir && (p === dir || p.startsWith(`${dir}/`)) && dir.length > (best?.len ?? 0)) best = { name: k.name, len: dir.length };
  }
  return best?.name ?? null;
}

function TargetMark({ path, known }: { path: string; known: AvailableTarget[] }) {
  const name = targetOf(path, known);
  return <span className="ss-at">{name ? <AgentIcon target={name} size={17} /> : <Folder size={15} className="text-ink-3" />}</span>;
}

interface Draft {
  id: string; // stable key: paths can be empty or repeated while editing
  path: string;
  mode: string;
  flatten: boolean;
  extension: string;
  as: string; // single-file extra: target file name, empty keeps the source name
}

const newDraft = (): Draft => ({ id: crypto.randomUUID(), path: '', mode: 'merge', flatten: false, extension: '', as: '' });

/** A single-file extra to start from, as a target page's "Share with Extras" asks for it. */
interface FilePrefill {
  file: string;
  target: string; // the folder holding the file
}

/** An extra name from a file name: APPEND_SYSTEM.md → APPEND_SYSTEM, .goosehints → goosehints. */
const nameFromFile = (file: string) => {
  const base = file.trim().replace(/^.*[\\/]/, '').replace(/^\.+/, '');
  return (base.replace(/\.[^.]*$/, '') || base).replace(/[^a-zA-Z0-9_-]+/g, '-').replace(/^[_-]+/, '');
};

/** Folder · File name · Mode for a single-file extra, shared by the Add extra dialog and the inline Add target row. */
function FileDraftFields({ draft, onChange, fileName, known, disabled }: {
  draft: Draft;
  onChange: (next: Draft) => void;
  fileName: string;
  known: AvailableTarget[];
  disabled: boolean;
}) {
  const t = useT();
  return (
    <>
      <TargetMark path={draft.path} known={known} />
      <span className="ss-inp min-w-0 flex-1">
        <input
          value={draft.path}
          onChange={(e) => onChange({ ...draft, path: e.target.value })}
          placeholder="~/.claude"
          aria-label={t('extras.modal.colPath')}
          disabled={disabled}
        />
      </span>
      <span className={`ss-inp w-[170px] shrink-0 ${isPathLike(draft.as) ? 'err' : ''}`}>
        <input
          value={draft.as}
          onChange={(e) => onChange({ ...draft, as: e.target.value })}
          placeholder={fileName || 'CONVENTIONS.md'}
          aria-label={t('extras.modal.fileName')}
          aria-invalid={isPathLike(draft.as)}
          title={isPathLike(draft.as) ? t('extras.modal.fileNameInvalid') : undefined}
          disabled={disabled}
        />
      </span>
      <Select
        className="w-[104px] shrink-0"
        value={draft.mode}
        onChange={(v) => onChange({ ...draft, mode: v })}
        options={FILE_MODES.map((m) => ({ value: m, label: m, description: t(`extras.fileModeDescription.${m}`) }))}
        disabled={disabled}
      />
    </>
  );
}

/** Folder · Extension · Mode · flatten, shared by the Add extra dialog and the inline Add target row. */
function DraftFields({ draft, onChange, extensions, known, disabled }: {
  draft: Draft;
  onChange: (next: Draft) => void;
  extensions: string[];
  known: AvailableTarget[];
  disabled: boolean;
}) {
  const t = useT();
  const locked = draft.extension !== '';
  return (
    <>
      <TargetMark path={draft.path} known={known} />
      <span className="ss-inp min-w-0 flex-1">
        <input
          value={draft.path}
          onChange={(e) => onChange({ ...draft, path: e.target.value })}
          placeholder="~/.claude/commands"
          aria-label={t('extras.modal.colPath')}
          disabled={disabled}
        />
      </span>
      <Select
        className="w-[170px] shrink-0"
        value={draft.extension}
        // An extension converts each file, so it always writes copies
        onChange={(v) => onChange({ ...draft, extension: v, ...(v ? { mode: 'copy' } : {}) })}
        options={[{ value: '', label: t('extras.noExtension') }, ...extensions.map((e) => ({ value: e, label: e }))]}
        disabled={disabled || extensions.length === 0}
      />
      <Select
        className="w-[104px] shrink-0"
        value={locked ? 'copy' : draft.mode}
        onChange={(v) => onChange({ ...draft, mode: v, ...(v === 'symlink' ? { flatten: false } : {}) })}
        options={MODES.map((m) => ({ value: m, label: m, description: t(`extras.modeDescription.${m}`) }))}
        disabled={disabled || locked}
      />
      <span className="flex w-[96px] shrink-0 items-center gap-2">
        <button
          type="button"
          role="switch"
          aria-checked={draft.flatten}
          aria-label={t('extras.flatten')}
          className={`ss-sw ${draft.flatten ? 'on' : ''} disabled:opacity-50`}
          onClick={() => onChange({ ...draft, flatten: !draft.flatten })}
          disabled={disabled || draft.mode === 'symlink'}
        >
          <i />
        </button>
        <span className="text-xs text-ink-2">flatten</span>
      </span>
    </>
  );
}

function DraftHints({ extensions }: { extensions: string[] }) {
  const t = useT();
  return (
    <div className="flex flex-col gap-0.5 text-[12.5px] text-ink-3">
      <span>
        {t('extras.hint.extension')}{' '}
        {extensions.length === 0 && <Link to="/config?tab=extensions" className="font-semibold text-ink-2 hover:text-ink">{t('extras.installExtensionHint')}</Link>}
      </span>
      <span>{t('extras.hint.flatten')}</span>
    </div>
  );
}

function FileDraftHints({ fileName }: { fileName: string }) {
  const t = useT();
  return (
    <div className="flex flex-col gap-0.5 text-[12.5px] text-ink-3">
      <span>{t('extras.hint.fileAs', { file: fileName || '…' })}</span>
      <span>{t('extras.hint.fileModes')}</span>
      <span>{t('extras.hint.fileBackup')}</span>
    </div>
  );
}

function AddExtraDialog({ onClose, onCreated, extensions, known, sharedDir, folders, initial }: {
  onClose: () => void;
  onCreated: (agents: boolean) => void;
  extensions: string[];
  known: AvailableTarget[];
  sharedDir: string;
  folders: string[]; // folders in sharedDir that single-file extras already use
  initial?: FilePrefill;
}) {
  const { toast } = useToast();
  const t = useT();
  const [name, setName] = useState(initial ? nameFromFile(initial.file) : '');
  // A single file's name follows its file name until the user types one.
  const [nameTyped, setNameTyped] = useState(false);
  const [single, setSingle] = useState(Boolean(initial));
  const [file, setFile] = useState(initial?.file ?? '');
  const changeFile = (next: string) => {
    setFile(next);
    if (!nameTyped) setName(nameFromFile(next));
  };
  const [custom, setCustom] = useState(false);
  const [source, setSource] = useState('');
  const [folder, setFolder] = useState('');
  const [drafts, setDrafts] = useState<Draft[]>(() => [{ ...newDraft(), path: initial?.target ?? '' }]);
  const [saving, setSaving] = useState(false);
  const title = t('extras.addExtraTitle');
  const valid = drafts.filter((d) => d.path.trim());
  const fileName = file.trim();
  // A single file in the shared folder can use another extra's folder; empty means the extra's name.
  const folderName = single && !custom ? folder.trim() : '';
  const fileOk = !single || (fileName !== '' && !isPathLike(fileName) && !isPathLike(folderName) && valid.every((d) => !isPathLike(d.as.trim())));
  const canCreate = name.trim() !== '' && valid.length > 0 && (!custom || source.trim() !== '') && fileOk && !saving;
  const agents = single && isAgentsExtra({ file: fileName });
  // The separator between the source folder and the file name, a backslash on Windows.
  const sourceSep = joinFile(custom ? source : sharedDir, '').slice(-1);

  // Keep each target's mode valid for the chosen kind.
  const switchKind = (toSingle: boolean) => {
    const modes: readonly string[] = toSingle ? FILE_MODES : MODES;
    setSingle(toSingle);
    setDrafts(drafts.map((d) => (modes.includes(d.mode) ? d : { ...d, mode: 'merge' })));
  };

  const create = async () => {
    if (!canCreate) return;
    setSaving(true);
    try {
      await api.createExtra({
        name: name.trim(),
        ...(custom && { source: source.trim() }),
        ...(folderName && folderName !== name.trim() && { folder: folderName }),
        ...(single && { file: fileName }),
        targets: valid.map((d) => (single
          ? { path: d.path.trim(), mode: d.mode, ...(d.as.trim() && { as: d.as.trim() }) }
          : { path: d.path.trim(), mode: d.extension ? 'copy' : d.mode, flatten: d.flatten, ...(d.extension && { extension: d.extension }) })),
      });
      toast(t('extras.toast.created', { name: name.trim() }), 'success');
      onCreated(agents);
    } catch (err) {
      toast((err as Error).message, 'error');
      setSaving(false);
    }
  };

  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={saving} ariaLabel={title} className="!max-w-[800px]">
      <div className="dh">
        <div className="flex flex-col gap-1">
          <h2 className="ss-h2">{title}</h2>
          <p className="text-[13px] text-ink-2">{t('extras.modal.subtitle')}</p>
        </div>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={saving}><X size={16} /></button>
      </div>
      <div className="db">
        <div className="grid grid-cols-2 gap-3.5">
          <div className="ss-fld">
            <label htmlFor="extra-name">{t('extras.modal.name')}</label>
            <span className="ss-inp">
              <input id="extra-name" autoFocus value={name} onChange={(e) => { setName(e.target.value); setNameTyped(e.target.value !== ''); }} placeholder={t('extras.modal.namePlaceholder')} disabled={saving} />
            </span>
            <span className="hp">{t(single ? 'extras.modal.nameHintFile' : custom ? 'extras.modal.nameHintCustom' : 'extras.modal.nameHint')}</span>
          </div>
          <div className="ss-fld">
            <span id="extra-kind" className="text-[13px] font-semibold">{t('extras.modal.sync')}</span>
            <div className="ss-seg self-start" role="radiogroup" aria-labelledby="extra-kind">
              {[false, true].map((on) => (
                <button key={String(on)} type="button" role="radio" aria-checked={single === on} className={single === on ? 'on' : ''} onClick={() => switchKind(on)} disabled={saving}>
                  {on ? <FileText size={14} /> : <Folder size={14} />}
                  {t(on ? 'extras.modal.syncFile' : 'extras.modal.syncFolder')}
                </button>
              ))}
            </div>
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
              disabled={saving}
            />
            {single ? null : custom ? (
              <span className="ss-inp">
                <input value={source} onChange={(e) => setSource(e.target.value)} placeholder={t('extras.modal.sourcePathPlaceholder')} aria-label={t('extras.sourceType.custom')} disabled={saving} />
              </span>
            ) : (
              <span className="hp truncate font-mono">{joinFile(shortenHome(sharedDir), name.trim() || '…')}</span>
            )}
          </div>
          {single && (
            <div className="ss-fld col-span-2">
              <span className="text-[13px] font-semibold">{t('extras.modal.sourceFile')}</span>
              {/* The source file as one path: <folder> <sep> <file name>. */}
              <div className="flex min-w-0 items-center gap-1.5">
                {custom ? (
                  <span className="ss-inp flex-[3]">
                    <input value={source} onChange={(e) => setSource(e.target.value)} placeholder={t('extras.modal.sourcePathPlaceholder')} aria-label={t('extras.sourceType.custom')} disabled={saving} />
                  </span>
                ) : (
                  <>
                    <span className="max-w-[40%] shrink-0 truncate font-mono text-[12.5px] text-ink-3" title={sharedDir}>{joinFile(shortenHome(sharedDir), '')}</span>
                    <span className={`ss-inp flex-1 ${isPathLike(folderName) ? 'err' : ''}`}>
                      <input value={folder} onChange={(e) => setFolder(e.target.value)} placeholder={name.trim() || t('extras.modal.sourceFolder')} aria-label={t('extras.modal.sourceFolder')} aria-invalid={isPathLike(folderName)} disabled={saving} />
                    </span>
                  </>
                )}
                <span className="font-mono text-ink-3">{sourceSep}</span>
                <span className={`ss-inp flex-[2] ${isPathLike(fileName) ? 'err' : ''}`}>
                  <input value={file} onChange={(e) => changeFile(e.target.value)} placeholder="CONVENTIONS.md" aria-label={t('extras.modal.fileName')} aria-invalid={isPathLike(fileName)} disabled={saving} />
                </span>
              </div>
              {/* Folders other single-file extras use, one click to share one. */}
              {!custom && folders.length > 0 && (
                <span className="flex flex-wrap items-center gap-1.5 text-[12.5px] text-ink-3">
                  {t('extras.modal.existingFolders')}
                  {folders.map((f) => (
                    <button key={f} type="button" className={`ss-tag font-mono hover:text-ink ${folderName === f ? '!text-ink' : ''}`} aria-pressed={folderName === f} onClick={() => setFolder(f)} disabled={saving}>{f}</button>
                  ))}
                </span>
              )}
              {isPathLike(folderName) ? <span className="hp text-bad">{t('extras.modal.sourceFolderInvalid')}</span>
                : isPathLike(fileName) ? <span className="hp text-bad">{t('extras.modal.fileNameInvalid')}</span>
                : !custom && <span className="hp">{t('extras.modal.sourceFolderHint')}</span>}
            </div>
          )}
          {agents && <div className="ss-note inf col-span-2"><span className="flex-1">{t('extras.modal.agentsNote')}</span></div>}
        </div>

        <div className="flex flex-col gap-1.5">
          <span className="text-[13px] font-semibold">{t('extras.modal.targets')}</span>
          <div className="ss-list !shadow-none">
            <div className="ss-lh !px-3">
              <span className="w-[30px]" />
              <span className="flex-1">{t('extras.modal.colPath')}</span>
              <span className="w-[170px]">{t(single ? 'extras.modal.fileName' : 'extras.modal.colExtension')}</span>
              <span className="w-[104px]">{t('extras.modal.colMode')}</span>
              {!single && <span className="w-[96px]" />}
              <span className="w-[30px]" />
            </div>
            {drafts.map((d, i) => (
              <div key={d.id} className="ss-r !min-h-[52px] !px-3 !py-1.5">
                {single
                  ? <FileDraftFields draft={d} onChange={(next) => setDrafts(drafts.map((x, j) => (j === i ? next : x)))} fileName={fileName} known={known} disabled={saving} />
                  : <DraftFields draft={d} onChange={(next) => setDrafts(drafts.map((x, j) => (j === i ? next : x)))} extensions={extensions} known={known} disabled={saving} />}
                <button
                  type="button"
                  className="ss-ib shrink-0 disabled:invisible"
                  aria-label={t('extras.removeTarget')}
                  onClick={() => setDrafts(drafts.filter((_, j) => j !== i))}
                  disabled={saving || drafts.length === 1}
                >
                  <X size={16} />
                </button>
              </div>
            ))}
            <div className="ss-r !min-h-10 !px-3">
              <button type="button" className="flex items-center gap-[7px] pl-4 text-[13px] text-ink-2 hover:text-ink" onClick={() => setDrafts([...drafts, newDraft()])} disabled={saving}>
                <Plus size={14} />
                {t('extras.addTarget')}
              </button>
            </div>
          </div>
          {single ? <FileDraftHints fileName={fileName} /> : <DraftHints extensions={extensions} />}
        </div>
      </div>
      <div className="df">
        <span className="flex-1" />
        <Button variant="ghost" onClick={onClose} disabled={saving}>{t('extras.cancel')}</Button>
        <Button variant="primary" loading={saving} disabled={!canCreate} onClick={create}>{t('extras.create')}</Button>
      </div>
    </DialogShell>
  );
}

function AddTargetRow({ onAdd, onCancel, extensions, known, file }: {
  onAdd: (draft: Draft) => Promise<boolean>;
  onCancel: () => void;
  extensions: string[];
  known: AvailableTarget[];
  file?: string; // single-file extra: the source file name
}) {
  const t = useT();
  const [draft, setDraft] = useState(newDraft);
  const [busy, setBusy] = useState(false);
  const ready = draft.path.trim() !== '' && !isPathLike(draft.as.trim());
  const add = async () => {
    if (!ready || busy) return;
    setBusy(true);
    if (!(await onAdd(draft))) setBusy(false);
  };
  return (
    <form className="ss-r !min-h-[52px] !py-1.5" onSubmit={(e) => { e.preventDefault(); void add(); }}>
      {file
        ? <FileDraftFields draft={draft} onChange={setDraft} fileName={file} known={known} disabled={busy} />
        : <DraftFields draft={draft} onChange={setDraft} extensions={extensions} known={known} disabled={busy} />}
      <Button type="submit" variant="primary" size="sm" loading={busy} disabled={!ready}>{t('extras.addTarget')}</Button>
      <button type="button" className="ss-ib shrink-0" aria-label={t('extras.cancel')} onClick={onCancel} disabled={busy}><X size={16} /></button>
    </form>
  );
}

function TargetTags({ target }: { target: ExtraTarget }) {
  return (
    <span className="flex w-[210px] shrink-0 items-center gap-1.5">
      {target.extension && <span className="ss-tag"><Puzzle size={11} />{target.extension}</span>}
      <span className="ss-tag">{target.mode}</span>
      {target.flatten && <span className="ss-tag">flatten</span>}
    </span>
  );
}

export default function ExtrasPage() {
  const { isProjectMode } = useAppContext();
  const { toast } = useToast();
  const t = useT();
  const queryClient = useQueryClient();
  const [params, setParams] = useSearchParams();
  const tab = params.get('tab') === 'memory' ? 'memory' : params.get('tab') === 'instructions' ? 'instructions' : 'folders';
  const [creatingNote, setCreatingNote] = useState(false);

  const { data, isPending, error } = useQuery({ queryKey: queryKeys.extras, queryFn: () => api.listExtras(), staleTime: staleTimes.extras });
  const { data: extData } = useQuery({ queryKey: ['extras', 'extensions'], queryFn: () => api.listExtraExtensions(), staleTime: staleTimes.extras });
  const { data: availData } = useAvailableTargetsQuery();
  const { data: overview } = useOverviewQuery();
  const extensions = extData?.extensions ?? [];
  const known = useMemo(() => availData?.targets ?? [], [availData]);
  // Mirrors config.ResolveExtrasSourceDir: extras_source, else "extras" next to the skills source
  const sharedDir = overview?.extrasSource ?? joinFile((overview?.source ?? '').replace(/[\\/][^\\/]*[\\/]?$/, ''), 'extras');
  // Folders of single-file extras directly inside sharedDir, offered when adding another file
  const sharedFolders = useMemo(() => {
    const parent = sharedDir.replace(/[\\/]+$/, '');
    const dirs = (data?.extras ?? []).filter((e) => e.file).map((e) => e.source_dir.replace(/[\\/]+$/, ''));
    return [...new Set(dirs.filter((d) => d.replace(/[\\/][^\\/]*$/, '') === parent).map((d) => d.slice(parent.length + 1)))];
  }, [data, sharedDir]);

  // ?add=file&target=<folder>&file=<name> opens Add extra ready to share that file.
  const [prefill, setPrefill] = useState<FilePrefill | null>(() => {
    const target = params.get('target') ?? '';
    const file = params.get('file') ?? '';
    return params.get('add') === 'file' && file ? { file, target } : null;
  });
  const [showAdd, setShowAdd] = useState(prefill !== null);
  const prefillInUrl = params.get('add') === 'file';
  useEffect(() => {
    if (!prefillInUrl) return;
    setParams((prev) => {
      const p = new URLSearchParams(prev);
      ['add', 'target', 'file'].forEach((k) => p.delete(k));
      return p;
    }, { replace: true });
  }, [prefillInUrl, setParams]);
  const [creatingShared, setCreatingShared] = useState(false);
  const [addingTo, setAddingTo] = useState<string | null>(null);
  const [menu, setMenu] = useState<{ x: number; y: number; items: ContextMenuItem[] } | null>(null);
  const [removeExtra, setRemoveExtra] = useState<string | null>(null);
  const [removeTarget, setRemoveTarget] = useState<{ name: string; path: string } | null>(null);

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: queryKeys.extras });
    queryClient.invalidateQueries({ queryKey: queryKeys.extrasDiff() });
    queryClient.invalidateQueries({ queryKey: queryKeys.config });
    queryClient.invalidateQueries({ queryKey: queryKeys.overview });
  };

  const openMenu = (e: React.MouseEvent<HTMLButtonElement>, items: ContextMenuItem[]) => {
    const r = e.currentTarget.getBoundingClientRect();
    setMenu({ x: r.left, y: r.bottom + 4, items });
  };

  const sync = async (name: string, force: boolean) => {
    try {
      const res = await api.syncExtras({ name, force });
      const totals = sumEntry(res.extras.find((e) => e.name === name));
      toast(buildSyncToast(t('extras.toast.syncOne', { name }), t('extras.toast.syncOneFailed', { name }), totals, force, t), syncToastType(totals));
      invalidate();
    } catch (err) {
      toast((err as Error).message, 'error');
    }
  };

  const changeTarget = async (name: string, target: ExtraTarget, patch: { mode?: string; flatten?: boolean; extension?: string }, message: string) => {
    // Show the choice right away instead of waiting for the PATCH and refetch
    const prev = queryClient.getQueryData<{ extras: Extra[] }>(queryKeys.extras);
    queryClient.setQueryData<{ extras: Extra[] }>(queryKeys.extras, (old) => old && {
      extras: old.extras.map((e) => e.name !== name ? e : { ...e, targets: e.targets.map((tg) => tg.path !== target.path ? tg : { ...tg, ...patch }) }),
    });
    try {
      await api.setExtraMode(name, target.path, patch.mode ?? target.mode, patch.flatten, patch.extension);
      toast(message, 'success');
      invalidate();
    } catch (err) {
      if (prev) queryClient.setQueryData(queryKeys.extras, prev);
      toast((err as Error).message, 'error');
    }
  };

  const addTarget = async (extra: Extra, d: Draft) => {
    const { name } = extra;
    const path = d.path.trim();
    try {
      await api.addExtraTarget(name, extra.file
        ? { path, mode: d.mode, ...(d.as.trim() && { as: d.as.trim() }) }
        : { path, mode: d.extension ? 'copy' : d.mode, flatten: d.flatten });
      // The add endpoint takes no extension, so set it on the new target afterwards
      if (d.extension) await api.setExtraMode(name, path, 'copy', undefined, d.extension);
      toast(t('extras.toast.targetAdded', { path }), 'success');
      setAddingTo(null);
      invalidate();
      return true;
    } catch (err) {
      toast((err as Error).message, 'error');
      invalidate();
      return false;
    }
  };

  const extraMenu = (extra: Extra): ContextMenuItem[] => [
    { key: 'sync', label: t('extras.sync'), icon: <RefreshCw size={14} />, onSelect: () => void sync(extra.name, false) },
    { key: 'force', label: t('extras.forceSync'), icon: <Zap size={14} />, onSelect: () => void sync(extra.name, true) },
    { key: 'remove', label: t('extras.removeConfirm.title'), icon: <Trash2 size={14} />, danger: true, onSelect: () => setRemoveExtra(extra.name) },
  ];

  // Single files: merge, copy or import; symlink only when a target already uses it.
  const fileTargetMenu = (extra: Extra, tg: ExtraTarget): ContextMenuItem[] => [
    {
      key: 'mode',
      label: t('extras.modal.colMode'),
      icon: <Link2 size={14} />,
      items: [...FILE_MODES, ...(tg.mode === 'symlink' ? ['symlink'] : [])].map((m) => ({
        key: m,
        label: m,
        selected: tg.mode === m,
        onSelect: () => void changeTarget(extra.name, tg, { mode: m }, t('extras.toast.modeChanged', { mode: m })),
      })),
    },
    ...(extra.targets.length > 1
      ? [{ key: 'remove', label: t('extras.removeTarget'), icon: <Trash2 size={14} />, danger: true, onSelect: () => setRemoveTarget({ name: extra.name, path: tg.path }) }]
      : []),
  ];

  const targetMenu = (extra: Extra, tg: ExtraTarget): ContextMenuItem[] => extra.file ? fileTargetMenu(extra, tg) : [
    ...(extensions.length > 0 || tg.extension
      ? [{
          key: 'extension',
          label: t('extras.modal.colExtension'),
          icon: <Puzzle size={14} />,
          items: [
            { key: '', label: t('extras.noExtension'), selected: !tg.extension, onSelect: () => void changeTarget(extra.name, tg, { extension: '' }, t('extras.toast.extensionCleared')) },
            ...[...new Set([...extensions, ...(tg.extension ? [tg.extension] : [])])].map((e) => ({
              key: e,
              label: extensions.includes(e) ? e : t('extras.extensionMissing', { extension: e }),
              selected: tg.extension === e,
              onSelect: () => void changeTarget(extra.name, tg, { mode: 'copy', extension: e }, t('extras.toast.extensionChanged', { extension: e })),
            })),
          ],
        }]
      : []),
    ...(tg.extension
      ? []
      : [{
          key: 'mode',
          label: t('extras.modal.colMode'),
          icon: <Link2 size={14} />,
          items: MODES.map((m) => ({
            key: m,
            label: m,
            selected: tg.mode === m,
            onSelect: () => void changeTarget(extra.name, tg, { mode: m, ...(m === 'symlink' && tg.flatten ? { flatten: false } : {}) }, t('extras.toast.modeChanged', { mode: m })),
          })),
        }]),
    ...(tg.mode === 'symlink'
      ? []
      : [{
          key: 'flatten',
          label: t('extras.flatten'),
          icon: <FoldVertical size={14} />,
          items: [true, false].map((on) => ({
            key: String(on),
            label: t(on ? 'extras.flattenOn' : 'extras.flattenOff'),
            selected: tg.flatten === on,
            onSelect: () => void changeTarget(extra.name, tg, { flatten: on }, t('extras.toast.flattenChanged', { flatten: String(on) })),
          })),
        }]),
    // The last target can't go: an extra needs somewhere to sync to
    ...(extra.targets.length > 1
      ? [{ key: 'remove', label: t('extras.removeTarget'), icon: <Trash2 size={14} />, danger: true, onSelect: () => setRemoveTarget({ name: extra.name, path: tg.path }) }]
      : []),
  ];

  // Shared AGENTS.md files have their own tab.
  const extras = (data?.extras ?? []).filter((e) => !isAgentsExtra(e));
  const sharedCount = (data?.extras ?? []).length - extras.length;

  return (
    <div className="animate-fade-in">
      <PageHeader
        title={t('extras.title')}
        subtitle={t(isProjectMode ? 'extras.subtitle.project' : 'extras.subtitle.global')}
        actions={tab === 'folders' ? <span data-tour="extras-list"><Button variant="primary" onClick={() => setShowAdd(true)}><Plus size={15} />{t('extras.addExtra')}</Button></span>
          : tab === 'instructions' ? <Button variant="primary" onClick={() => setCreatingShared(true)}><Plus size={15} />{t(isProjectMode ? 'instructions.projectShared.new' : 'instructions.shared.new')}</Button>
          : <>
            <Button variant="ghost" onClick={() => void queryClient.invalidateQueries({ queryKey: queryKeys.memory.all })}><RefreshCw size={15} />{t('memory.refresh')}</Button>
            <Button variant="primary" onClick={() => setCreatingNote(true)}><Plus size={15} />{t('memory.new')}</Button>
          </>}
      />

      <nav className="ss-tabs mb-7" aria-label={t('extras.tabs')}>
        <Link to="?" replace className={tab === 'folders' ? 'on' : ''} aria-current={tab === 'folders'}>{t('extras.tab.folders')}</Link>
        <Link to="?tab=instructions" replace className={tab === 'instructions' ? 'on' : ''} aria-current={tab === 'instructions'}>{t('extras.tab.instructions')}</Link>
        <Link to="?tab=memory" replace className={tab === 'memory' ? 'on' : ''} aria-current={tab === 'memory'}>{t('memory.title')}</Link>
      </nav>

      {tab === 'memory' ? <MemoryNotes creating={creatingNote} setCreating={setCreatingNote} /> : tab === 'instructions' ? (
        isProjectMode ? <ProjectInstructions creating={creatingShared} setCreating={setCreatingShared} /> : <SharedInstructions creating={creatingShared} setCreating={setCreatingShared} />
      ) : isPending ? (
        <PageSkeleton />
      ) : error ? (
        <div className="ss-note bad"><span className="flex-1">{error.message}</span></div>
      ) : extras.length === 0 ? (
        <EmptyState
          icon={FolderPlus}
          title={t(sharedCount > 0 ? 'extras.empty.foldersTitle' : 'extras.empty.title')}
          description={sharedCount > 0 ? t(isProjectMode ? 'extras.empty.sharedElsewhereProject' : 'extras.empty.sharedElsewhere', { count: sharedCount }) : t('extras.empty.description')}
          action={<Button variant="primary" onClick={() => setShowAdd(true)}><Plus size={15} />{t('extras.addExtra')}</Button>}
        />
      ) : (
        <>
          <div className="ss-list">
            {extras.map((extra) => (
              <div key={extra.name} className="contents">
                <div className="ss-gh !min-h-12">
                  <span className="ss-cat sm extra">{extra.file ? <FileText size={14} /> : <FolderPlus size={14} />}</span>
                  <span className="font-mono font-semibold">{extra.name}</span>
                  {extra.file ? (
                    <>
                      <span className="min-w-0 truncate font-mono text-ink-3" title={joinFile(extra.source_dir, extra.file)}>
                        {joinFile(shortenHome(extra.source_dir), '')}<span className="font-semibold text-ink">{extra.file}</span>
                      </span>
                      <span className="shrink-0 text-ink-3">· {t('extras.singleFile')}</span>
                      {!extra.source_exists && <span className="ss-tag bad shrink-0">{t('extras.sourceFileMissing')}</span>}
                    </>
                  ) : (
                    <>
                      <span className="min-w-0 truncate font-mono text-ink-3" title={extra.source_dir}>{shortenHome(extra.source_dir)}</span>
                      <span className="shrink-0 text-ink-3">· {t(extra.file_count === 1 ? 'extras.files.one' : 'extras.files.other', { count: extra.file_count })}</span>
                    </>
                  )}
                  <span className="flex-1" />
                  <button type="button" className="ss-ib" aria-label={t('extras.moreActions', { name: extra.name })} onClick={(e) => openMenu(e, extraMenu(extra))}>
                    <Ellipsis size={16} />
                  </button>
                </div>
                {extra.targets.map((tg) => {
                  const status = STATUS[tg.status];
                  const dest = extra.file ? joinFile(tg.path, tg.as || extra.file) : tg.path;
                  return (
                    <div key={tg.path} className="ss-r !min-h-[46px]">
                      <TargetMark path={tg.path} known={known} />
                      <span className="min-w-0 flex-1 truncate font-mono text-[13px]" title={dest}>{shortenHome(dest)}</span>
                      <TargetTags target={tg} />
                      <span className="w-[120px] shrink-0">
                        <span className={`ss-st ${status?.tone ?? ''}`}>{status ? t(status.labelKey) : tg.status}</span>
                      </span>
                      <button type="button" className="ss-ib" aria-label={t('extras.targetActions', { path: tg.path })} onClick={(e) => openMenu(e, targetMenu(extra, tg))}>
                        <Ellipsis size={16} />
                      </button>
                    </div>
                  );
                })}
                {addingTo === extra.name ? (
                  <AddTargetRow onAdd={(d) => addTarget(extra, d)} onCancel={() => setAddingTo(null)} extensions={extensions} known={known} file={extra.file} />
                ) : (
                  <div className="ss-r !min-h-10">
                    <button type="button" className="flex items-center gap-[7px] text-[13px] text-ink-2 hover:text-ink" onClick={() => setAddingTo(extra.name)}>
                      <Plus size={14} />
                      {t('extras.addTarget')}
                    </button>
                  </div>
                )}
              </div>
            ))}
          </div>
          <p className="mt-6 text-[13px] leading-relaxed text-ink-3">
            {t('extras.footnote')}{' '}
            <Link to="/config?tab=extensions" className="text-ink-2 hover:text-ink">{t('extras.footnoteLink')}</Link>
          </p>
        </>
      )}

      {showAdd && (
        <AddExtraDialog
          onClose={() => { setShowAdd(false); setPrefill(null); }}
          onCreated={(agents) => {
            setShowAdd(false);
            setPrefill(null);
            invalidate();
            if (agents) setParams({ tab: 'instructions' }, { replace: true });
          }}
          extensions={extensions}
          known={known}
          sharedDir={sharedDir}
          folders={sharedFolders}
          initial={prefill ?? undefined}
        />
      )}
      <SkillContextMenu open={!!menu} anchorPoint={menu ?? undefined} items={menu?.items ?? []} onClose={() => setMenu(null)} />
      <ConfirmDialog
        open={removeExtra !== null}
        title={t('extras.removeConfirm.title')}
        message={t('extras.removeConfirm.message', { name: removeExtra ?? '' })}
        confirmText={t('extras.removeConfirm.confirmText')}
        variant="danger"
        onConfirm={async () => {
          const name = removeExtra!;
          setRemoveExtra(null);
          try {
            await api.deleteExtra(name);
            toast(t('extras.toast.removed', { name }), 'success');
            invalidate();
          } catch (err) {
            toast((err as Error).message, 'error');
          }
        }}
        onCancel={() => setRemoveExtra(null)}
      />
      <ConfirmDialog
        open={removeTarget !== null}
        title={t('extras.removeTargetConfirm.title')}
        message={t('extras.removeTargetConfirm.message', { path: removeTarget?.path ?? '' })}
        confirmText={t('extras.removeConfirm.confirmText')}
        variant="danger"
        onConfirm={async () => {
          const target = removeTarget!;
          setRemoveTarget(null);
          try {
            await api.removeExtraTarget(target.name, target.path);
            toast(t('extras.toast.targetRemoved', { path: target.path }), 'success');
            invalidate();
          } catch (err) {
            toast((err as Error).message, 'error');
          }
        }}
        onCancel={() => setRemoveTarget(null)}
      />
    </div>
  );
}
