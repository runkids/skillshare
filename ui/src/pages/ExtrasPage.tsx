import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Ellipsis, FoldVertical, Folder, FolderPlus, Link2, Plus, Puzzle, RefreshCw, Trash2, X, Zap } from 'lucide-react';
import { api } from '../api/client';
import type { AvailableTarget, Extra, ExtraTarget } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { useAppContext } from '../context/AppContext';
import { useToast } from '../components/Toast';
import AgentIcon from '../components/AgentIcon';
import Button from '../components/Button';
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

const MODES = ['merge', 'copy', 'symlink'] as const;

// Status and mode labels stay in English, like the CLI
const STATUS: Record<string, { tone: string; label: string }> = {
  synced: { tone: 'ok', label: 'In sync' },
  drift: { tone: 'warn', label: 'Drift' },
  'not synced': { tone: 'warn', label: 'Not synced' },
  'no source': { tone: 'bad', label: 'Source missing' },
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
}

const newDraft = (): Draft => ({ id: crypto.randomUUID(), path: '', mode: 'merge', flatten: false, extension: '' });

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

function AddExtraDialog({ onClose, onCreated, extensions, known, sharedDir }: {
  onClose: () => void;
  onCreated: () => void;
  extensions: string[];
  known: AvailableTarget[];
  sharedDir: string;
}) {
  const { toast } = useToast();
  const t = useT();
  const [name, setName] = useState('');
  const [custom, setCustom] = useState(false);
  const [source, setSource] = useState('');
  const [drafts, setDrafts] = useState<Draft[]>(() => [newDraft()]);
  const [saving, setSaving] = useState(false);
  const title = t('extras.addExtraTitle');
  const valid = drafts.filter((d) => d.path.trim());
  const canCreate = name.trim() !== '' && valid.length > 0 && (!custom || source.trim() !== '') && !saving;

  const create = async () => {
    if (!canCreate) return;
    setSaving(true);
    try {
      await api.createExtra({
        name: name.trim(),
        ...(custom && { source: source.trim() }),
        targets: valid.map((d) => ({ path: d.path.trim(), mode: d.extension ? 'copy' : d.mode, flatten: d.flatten, ...(d.extension && { extension: d.extension }) })),
      });
      toast(t('extras.toast.created', { name: name.trim() }), 'success');
      onCreated();
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
              <input id="extra-name" autoFocus value={name} onChange={(e) => setName(e.target.value)} placeholder={t('extras.modal.namePlaceholder')} disabled={saving} />
            </span>
            <span className="hp">{t('extras.modal.nameHint')}</span>
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
            {custom ? (
              <span className="ss-inp">
                <input value={source} onChange={(e) => setSource(e.target.value)} placeholder={t('extras.modal.sourcePathPlaceholder')} aria-label={t('extras.sourceType.custom')} disabled={saving} />
              </span>
            ) : (
              <span className="hp truncate font-mono">{`${shortenHome(sharedDir)}/${name.trim() || '…'}`}</span>
            )}
          </div>
        </div>

        <div className="flex flex-col gap-1.5">
          <span className="text-[13px] font-semibold">{t('extras.modal.targets')}</span>
          <div className="ss-list !shadow-none">
            <div className="ss-lh !px-3">
              <span className="w-[30px]" />
              <span className="flex-1">{t('extras.modal.colPath')}</span>
              <span className="w-[170px]">{t('extras.modal.colExtension')}</span>
              <span className="w-[104px]">{t('extras.modal.colMode')}</span>
              <span className="w-[96px]" />
              <span className="w-[30px]" />
            </div>
            {drafts.map((d, i) => (
              <div key={d.id} className="ss-r !min-h-[52px] !px-3 !py-1.5">
                <DraftFields draft={d} onChange={(next) => setDrafts(drafts.map((x, j) => (j === i ? next : x)))} extensions={extensions} known={known} disabled={saving} />
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
          <DraftHints extensions={extensions} />
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

function AddTargetRow({ onAdd, onCancel, extensions, known }: {
  onAdd: (draft: Draft) => Promise<boolean>;
  onCancel: () => void;
  extensions: string[];
  known: AvailableTarget[];
}) {
  const t = useT();
  const [draft, setDraft] = useState(newDraft);
  const [busy, setBusy] = useState(false);
  const add = async () => {
    if (!draft.path.trim() || busy) return;
    setBusy(true);
    if (!(await onAdd(draft))) setBusy(false);
  };
  return (
    <form className="ss-r !min-h-[52px] !py-1.5" onSubmit={(e) => { e.preventDefault(); void add(); }}>
      <DraftFields draft={draft} onChange={setDraft} extensions={extensions} known={known} disabled={busy} />
      <Button type="submit" variant="primary" size="sm" loading={busy} disabled={!draft.path.trim()}>{t('extras.addTarget')}</Button>
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

  const { data, isPending, error } = useQuery({ queryKey: queryKeys.extras, queryFn: () => api.listExtras(), staleTime: staleTimes.extras });
  const { data: extData } = useQuery({ queryKey: ['extras', 'extensions'], queryFn: () => api.listExtraExtensions(), staleTime: staleTimes.extras });
  const { data: availData } = useQuery({ queryKey: queryKeys.targets.available, queryFn: () => api.availableTargets(), staleTime: staleTimes.targets });
  const { data: overview } = useQuery({ queryKey: queryKeys.overview, queryFn: () => api.getOverview(), staleTime: staleTimes.overview });
  const extensions = extData?.extensions ?? [];
  const known = useMemo(() => availData?.targets ?? [], [availData]);
  // Mirrors config.ResolveExtrasSourceDir: extras_source, else "extras" next to the skills source
  const sharedDir = overview?.extrasSource ?? `${(overview?.source ?? '').replace(/\/[^/]*\/?$/, '')}/extras`;

  const [showAdd, setShowAdd] = useState(false);
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

  const addTarget = async (name: string, d: Draft) => {
    const path = d.path.trim();
    try {
      await api.addExtraTarget(name, { path, mode: d.extension ? 'copy' : d.mode, flatten: d.flatten });
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

  const targetMenu = (extra: Extra, tg: ExtraTarget): ContextMenuItem[] => [
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

  const extras = data?.extras ?? [];

  return (
    <div className="animate-fade-in">
      <PageHeader
        title={t('extras.title')}
        subtitle={t(isProjectMode ? 'extras.subtitle.project' : 'extras.subtitle.global')}
        actions={<span data-tour="extras-list"><Button variant="primary" onClick={() => setShowAdd(true)}><Plus size={15} />{t('extras.addExtra')}</Button></span>}
      />

      {isPending ? (
        <PageSkeleton />
      ) : error ? (
        <div className="ss-note bad"><span className="flex-1">{error.message}</span></div>
      ) : extras.length === 0 ? (
        <EmptyState
          icon={FolderPlus}
          title={t('extras.empty.title')}
          description={t('extras.empty.description')}
          action={<Button variant="primary" onClick={() => setShowAdd(true)}><Plus size={15} />{t('extras.addExtra')}</Button>}
        />
      ) : (
        <>
          <div className="ss-list">
            {extras.map((extra) => (
              <div key={extra.name} className="contents">
                <div className="ss-gh !min-h-12">
                  <span className="ss-cat sm extra"><FolderPlus size={14} /></span>
                  <span className="font-mono font-semibold">{extra.name}</span>
                  <span className="min-w-0 truncate font-mono text-ink-3" title={extra.source_dir}>{shortenHome(extra.source_dir)}</span>
                  <span className="shrink-0 text-ink-3">· {t(extra.file_count === 1 ? 'extras.files.one' : 'extras.files.other', { count: extra.file_count })}</span>
                  <span className="flex-1" />
                  <button type="button" className="ss-ib" aria-label={t('extras.moreActions', { name: extra.name })} onClick={(e) => openMenu(e, extraMenu(extra))}>
                    <Ellipsis size={16} />
                  </button>
                </div>
                {extra.targets.map((tg) => {
                  const status = STATUS[tg.status];
                  return (
                    <div key={tg.path} className="ss-r !min-h-[46px]">
                      <TargetMark path={tg.path} known={known} />
                      <span className="min-w-0 flex-1 truncate font-mono text-[13px]" title={tg.path}>{shortenHome(tg.path)}</span>
                      <TargetTags target={tg} />
                      <span className="w-[120px] shrink-0">
                        <span className={`ss-st ${status?.tone ?? ''}`}>{status?.label ?? tg.status}</span>
                      </span>
                      <button type="button" className="ss-ib" aria-label={t('extras.targetActions', { path: tg.path })} onClick={(e) => openMenu(e, targetMenu(extra, tg))}>
                        <Ellipsis size={16} />
                      </button>
                    </div>
                  );
                })}
                {addingTo === extra.name ? (
                  <AddTargetRow onAdd={(d) => addTarget(extra.name, d)} onCancel={() => setAddingTo(null)} extensions={extensions} known={known} />
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
          onClose={() => setShowAdd(false)}
          onCreated={() => { setShowAdd(false); invalidate(); }}
          extensions={extensions}
          known={known}
          sharedDir={sharedDir}
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
