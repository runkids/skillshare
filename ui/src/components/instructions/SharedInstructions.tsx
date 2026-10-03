import Tooltip from '../Tooltip';
import { useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { useQueries, useQuery, useQueryClient } from '@tanstack/react-query';
import { ChevronDown, ChevronUp, Copy, Ellipsis, FilePlus, Info, Plus, Search, Trash2, TriangleAlert, X } from 'lucide-react';
import { api, ApiError } from '../../api/client';
import type { InstructionsWarning, SharedInstructionsFile, SharedInstructionsTarget } from '../../api/client';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import { Checkbox } from '../Checkbox';
import ConfirmDialog from '../ConfirmDialog';
import EmptyState from '../EmptyState';
import { targetLabel } from '../mcp/mcpView';
import { Select } from '../Select';
import { PageSkeleton } from '../Skeleton';
import { SkillContextMenu } from '../TargetMenu';
import { useToast } from '../Toast';
import { useT } from '../../i18n';
import { queryKeys } from '../../lib/queryKeys';
import { fileName, shortenHome } from '../../lib/paths';
import AddLocationDialog from './AddLocationDialog';
import InstructionsEditorDialog from './InstructionsEditorDialog';
import LocationRows from './LocationRows';
import NewSharedDialog from './NewSharedDialog';
import RestorePreviewDialog from './RestorePreviewDialog';
import { BoxHeader, InstructionsPreview } from './ViewTabs';
import type { ConnectStep, ModeOption, RestoreStep, RowHint } from './instructionsView';
import {
  instructionsErrorMessage, instructionsWarningMessage, connectExtras, connectPlan, connectedTo, modeOptions, needsSync, pickedMode, refreshInstructions, restorePlan, rowHint, saveCopiesSummary, staleLocations, statusTone, usesOf,
} from './instructionsView';

const PREVIEW_LINES = 8;
type Pending = { title: string; message: ReactNode; confirm: string; danger?: boolean; run: () => Promise<void> };

/** ④ Extras › AGENTS.md (global): shared files on the left, the selected file and its targets on the right. */
export default function SharedInstructions({ creating, setCreating }: { creating: boolean; setCreating: (open: boolean) => void }) {
  const t = useT();
  const queryClient = useQueryClient();
  const [params, setParams] = useSearchParams();
  const [query, setQuery] = useState('');
  const { data, error, isPending } = useQuery({ queryKey: queryKeys.instructions.shared, queryFn: () => api.listSharedInstructions() });

  // A file just created: the dialog stays until the page shows it, since the list
  // and the address reach the page in separate renders.
  const created = useRef<string | null>(null);
  const pick = (name: string | null) => setParams((prev) => {
    const next = new URLSearchParams(prev);
    if (name) next.set('file', name);
    else next.delete('file');
    return next;
  }, { replace: true });

  // A ?file= naming no shared file shows the first one, and the address says so.
  const asked = params.get('file');
  const unknown = Boolean(data && asked && !data.files.some((f) => f.name === asked));
  useEffect(() => {
    if (unknown && data) pick(data.files[0]?.name ?? null);
  });
  useEffect(() => {
    if (created.current && asked === created.current && data?.files.some((f) => f.name === created.current)) {
      created.current = null;
      setCreating(false);
    }
  }, [asked, data, setCreating]);

  if (isPending) return <PageSkeleton />;
  if (error) return <div className="ss-note bad"><span className="flex-1">{instructionsErrorMessage(error, t)}</span></div>;
  const { files, targets } = data;
  // An older server does not say; file links then work as before.
  const fileLinks = data.file_links ?? true;
  const current = files.find((f) => f.name === params.get('file')) ?? files[0];
  const shown = files.filter((f) => f.name.toLowerCase().includes(query.trim().toLowerCase()));

  return (
    <>
      {!current ? (
        <EmptyState
          icon={FilePlus}
          title={t('instructions.shared.empty.title')}
          description={t('instructions.shared.empty.description')}
          action={<Button variant="primary" onClick={() => setCreating(true)}><Plus size={15} />{t('instructions.shared.new')}</Button>}
        />
      ) : (
        <div className="grid grid-cols-[220px_minmax(0,1fr)] items-start gap-6">
          <div className="sticky top-6 flex max-h-[calc(100dvh-48px)] min-w-0 flex-col gap-2.5">
            <label className="ss-inp h-[34px] w-full">
              <Search size={15} className="shrink-0 text-ink-3" />
              <input type="search" value={query} onChange={(e) => setQuery(e.target.value)} placeholder={t('instructions.shared.search')} aria-label={t('instructions.shared.search')} />
            </label>
            <nav className="ss-list min-h-0 !overflow-y-auto" aria-label={t('instructions.shared.files')}>
              {shown.length === 0 && <p className="px-4 py-3 text-[13px] text-ink-3">{t('instructions.shared.noMatches')}</p>}
              {shown.map((f) => {
                const using = connectedTo(targets, f.name);
                const on = f.name === current.name;
                return (
                  <button key={f.name} type="button" aria-pressed={on} onClick={() => pick(f.name)}
                    className={`ss-r link w-full !flex-col !items-start !gap-2 !py-3 text-left ${on ? 'sel' : ''}`}>
                    <span className="max-w-full truncate font-mono text-[13.5px] font-semibold">{f.name}</span>
                    <span className="flex items-center gap-2">
                      {using.length > 0 && (
                        <span className="ss-stack" aria-hidden="true">
                          {using.slice(0, 5).map((tg) => <span key={tg.name} className="ss-at !h-5 !w-5"><AgentIcon target={tg.name} size={11} /></span>)}
                        </span>
                      )}
                      <span className="text-[12px] text-ink-3">{t(using.length === 1 ? 'instructions.shared.count.one' : 'instructions.shared.count.other', { count: using.length })}</span>
                    </span>
                  </button>
                );
              })}
            </nav>
          </div>
          <FilePanel key={current.name} file={current} targets={targets} fileLinks={fileLinks}
            onDeleted={() => pick(files.find((f) => f.name !== current.name)?.name ?? null)} />
        </div>
      )}

      {creating && (
        <NewSharedDialog
          targets={targets}
          onClose={() => setCreating(false)}
          onCreated={async (name) => {
            // The dialog closes once the page shows the file, so it does not show another one meanwhile.
            refreshInstructions(queryClient);
            await queryClient.refetchQueries({ queryKey: queryKeys.instructions.shared });
            created.current = name;
            pick(name);
          }}
        />
      )}
    </>
  );
}

function FilePanel({ file, targets, fileLinks, onDeleted }: {
  file: SharedInstructionsFile;
  targets: SharedInstructionsTarget[];
  fileLinks: boolean;
  onDeleted: () => void;
}) {
  const t = useT();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const name = file.name;
  // Off once deletion starts, so the refresh afterwards does not ask for a file that is gone.
  const [deleting, setDeleting] = useState(false);
  const content = useQuery({ queryKey: queryKeys.instructions.sharedContent(name), queryFn: () => api.getSharedInstructionsContent(name), enabled: !deleting });
  const [expanded, setExpanded] = useState(false);
  // The card is read-only: Preview renders it (the default), Source shows the text as written.
  const [view, setView] = useState<'source' | 'preview'>('preview');
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [menu, setMenu] = useState<{ x: number; y: number } | null>(null);
  const [pending, setPending] = useState<Pending | null>(null);
  const [busy, setBusy] = useState(false);
  const [editing, setEditing] = useState(false);
  const [restoring, setRestoring] = useState<string | null>(null);
  const [addingLocation, setAddingLocation] = useState(false);
  // Refusals the server gave for one target (another shared file holds it), shown in its row.
  const [held, setHeld] = useState<Record<string, string>>({});

  const connected = connectedTo(targets, name);
  const isOn = (tg: SharedInstructionsTarget) => !tg.same_as && usesOf(tg).includes(name);
  const selectable = targets.filter((tg) => !tg.same_as);
  const chosen = selectable.filter((tg) => selected.has(tg.name));
  const allOn = selectable.length > 0 && selectable.every((tg) => selected.has(tg.name));
  const connectAll = connectPlan(targets, file);
  const restoreAll = restorePlan(targets, name);
  const stale = needsSync(targets, name);
  const locations = file.locations ?? [];
  const staleLoc = staleLocations(locations);
  const staleCount = stale.length + staleLoc.length;
  const list = (names: string[]) => names.join(t('instructions.shared.listSep'));

  // Runs one change with the page busy, reporting failures and refetching afterwards.
  const act = async (fn: () => Promise<void>) => {
    setBusy(true);
    setHeld({});
    try {
      await fn();
    } catch (err) {
      toast(instructionsErrorMessage(err, t), 'error');
    } finally {
      setBusy(false);
      refreshInstructions(queryClient);
    }
  };
  const ask = (p: Pending) => setPending(p);
  // A target another shared file holds is refused (409); its row says why, and the rest go on.
  const heldText = (err: unknown) => (err instanceof ApiError && err.code === 'instructions_target_held'
    ? instructionsErrorMessage(err, t) : null);
  const warn = (warnings?: InstructionsWarning[]) => warnings?.forEach((w) => toast(instructionsWarningMessage(w, t), 'warning'));

  const connect = async (steps: { target: string; extras: string[] }[]) => {
    const errors: string[] = [];
    const done: string[] = [];
    const refused: Record<string, string> = {};
    for (const s of steps) {
      try {
        const res = await api.assignSharedInstructions([s.target], s.extras);
        warn(res.warnings);
        if (res.success) done.push(s.target);
        else errors.push(...res.errors);
      } catch (err) {
        const why = heldText(err);
        if (!why) throw err;
        refused[s.target] = why;
      }
    }
    setHeld(refused);
    if (done.length) toast(t('instructions.shared.assigned', { targets: list(done), files: name }), 'success');
    if (errors.length) throw new Error(errors.join('; '));
  };
  // Restore takes only this file off each target; an import target keeps its other files.
  const detach = async (names: string[]) => {
    const errors: string[] = [];
    const done: string[] = [];
    for (const n of names) {
      const res = await api.restoreSharedInstructions(name, n);
      if (res.success) done.push(n);
      else errors.push(...res.errors);
    }
    if (done.length) toast(t('instructions.shared.detached', { targets: list(done), name }), 'success');
    if (errors.length) throw new Error(errors.join('; '));
  };

  const toggle = (tg: SharedInstructionsTarget) => {
    if (isOn(tg)) {
      setRestoring(tg.name);
    } else if (!tg.import && tg.assigned.length > 0) {
      const other = tg.assigned[0].name;
      ask({
        title: t('instructions.switchFrom.title', { target: tg.name, name }),
        message: t('instructions.switchFrom.message', { target: tg.name, other }),
        confirm: t('instructions.switchFrom.confirm', { name }),
        run: () => connect([{ target: tg.name, extras: [name] }]),
      });
    } else {
      void act(() => connect([{ target: tg.name, extras: connectExtras(tg, name) }]));
    }
  };

  const setMode = (tg: SharedInstructionsTarget, mode: string) => act(async () => {
    try {
      warn((await api.setSharedInstructionsMode(name, tg.name, mode)).warnings);
    } catch (err) {
      const why = heldText(err);
      if (!why) throw err;
      setHeld({ [tg.name]: why });
      return;
    }
    toast(t('instructions.mode.changed', { target: tg.name, name, mode }), 'success');
  });
  const modeText = (o: ModeOption, tg: SharedInstructionsTarget) => {
    if (o.blocked) return t(`instructions.mode.blocked.${o.blocked}`, { target: tg.name });
    return t(`instructions.mode.${o.mode}`, { name, file: fileName(tg.path) });
  };

  const connectNote = (s: ConnectStep) => (s.note === 'held' ? t('instructions.plan.held', { other: s.other ?? '' })
    : s.note === 'tooLong' ? t('instructions.plan.tooLong', { max: (s.max ?? 0).toLocaleString() })
      : t(`instructions.plan.${s.note}`));
  const restoreNote = (s: RestoreStep) => (s.note === 'importKeep' ? t((s.others ?? []).length === 1 ? 'instructions.plan.importKeep.one' : 'instructions.plan.importKeep.other', { name, others: list(s.others ?? []) })
    : s.note === 'import' ? t('instructions.plan.dropImport', { name })
      : s.note === 'modified' ? t('instructions.plan.modified') : t('instructions.plan.restoreLink'));
  const planList = (rows: { target: string; note: string; warn: boolean }[], footer: string) => (
    <div className="flex flex-col gap-3">
      <div className="ss-list">
        {rows.map((r) => (
          <div key={r.target} className="ss-r !min-h-11 !gap-2.5 !py-1.5">
            <span className="ss-at !h-6 !w-6"><AgentIcon target={r.target} size={14} /></span>
            <span className="min-w-0 flex-1 truncate font-mono text-[13px] font-semibold text-ink">{r.target}</span>
            <span className={`text-right text-[12.5px] ${r.warn ? 'text-warn' : 'text-ink-3'}`}>{r.note}</span>
          </div>
        ))}
      </div>
      <p className="text-[13px]">{footer}</p>
    </div>
  );
  // Held targets are listed as skipped and left out of the run.
  const askConnect = (steps: ConnectStep[], confirm: string) => {
    const run = steps.filter((s) => s.note !== 'held');
    ask({
      title: t(run.length === 1 ? 'instructions.connectAll.title.one' : 'instructions.connectAll.title.other', { count: run.length, name }),
      message: planList(steps.map((s) => ({ target: s.target, note: connectNote(s), warn: s.note !== 'import' && s.note !== 'link' })), t('instructions.connectAll.message')),
      confirm,
      run: () => connect(run),
    });
  };
  const askRestore = (steps: RestoreStep[], confirm: string) => ask({
    title: t(steps.length === 1 ? 'instructions.restoreAll.title.one' : 'instructions.restoreAll.title.other', { count: steps.length, name }),
    message: <RestorePlan name={name} steps={steps} note={restoreNote} planList={planList} footer={t('instructions.restoreAll.message', { name })} />,
    confirm,
    run: () => detach(steps.map((s) => s.target)),
  });

  // label names the row in the messages: a target's name or a location's path.
  const resolve = (label: string, on: { target: string } | { path: string }, action: 'collect' | 'reapply') => ask({
    title: t(`instructions.resolve.${action}.title`, { name, target: label }),
    message: t(action === 'collect' ? `instructions.resolve.collect.message.${connected.length === 1 ? 'one' : 'other'}` : 'instructions.resolve.reapply.message', { name, target: label, count: connected.length }),
    confirm: t(`instructions.resolve.${action}.item`, { name }),
    run: async () => {
      await api.resolveSharedInstructions(name, on, action);
      toast(t(`instructions.resolve.${action}.done`, { name, target: label }), 'success');
    },
  });

  const sync = () => act(async () => {
    const res = await api.syncExtras({ name });
    const failed = res.extras.flatMap((e) => e.targets).find((r) => r.error);
    if (failed) throw new Error(failed.error);
    toast(t('instructions.shared.synced', { name, targets: list([...stale.map((tg) => tg.name), ...staleLoc.map((l) => shortenHome(l.file))]) }), 'success');
  });

  const remove = () => ask({
    title: t('instructions.delete.title', { name }),
    message: connected.length
      ? t(connected.length === 1 ? 'instructions.delete.message.one' : 'instructions.delete.message.other', { count: connected.length, targets: list(connected.map((tg) => tg.name)), name })
      : t('instructions.delete.unused', { name }),
    confirm: t('instructions.detail.delete.confirm'),
    danger: true,
    run: async () => {
      setDeleting(true);
      try {
        await api.deleteExtra(name);
      } catch (err) {
        setDeleting(false);
        throw err;
      }
      toast(t('instructions.detail.delete.done', { name }), 'success');
      onDeleted();
    },
  });

  const copyPath = async () => {
    try {
      await navigator.clipboard.writeText(file.path);
      toast(t('instructions.shared.pathCopied', { path: shortenHome(file.path) }), 'success');
    } catch (err) {
      toast(instructionsErrorMessage(err, t), 'error');
    }
  };

  const hintText = (h: RowHint) => {
    switch (h.kind) {
      case 'sameAs': return t('instructions.hint.sameAs', { name: h.name });
      case 'folderLink': return t('instructions.hint.folderLink');
      case 'directory': return t('instructions.hint.directory');
      case 'notSynced': return t(h.mode === 'symlink' ? 'instructions.hint.missingLink' : 'instructions.hint.missingFile');
      case 'drift': return h.mode === 'import' ? t('instructions.hint.driftImport') : t('instructions.hint.drift', { name });
      case 'noSource': return t('instructions.hint.noSource', { name });
      case 'tooLong': return t('instructions.shared.tooLong', { name, chars: file.chars.toLocaleString(), max: h.max.toLocaleString() });
      case 'usesOther': return t('instructions.hint.usesOther', { name: h.name });
      case 'alsoUses': return t('instructions.hint.alsoUses', { names: list(h.names) });
    }
  };

  const text = content.data?.content ?? '';
  const lines = text ? text.replace(/\n$/, '').split('\n') : [];
  const bulkOff = chosen.filter((tg) => !isOn(tg));
  const bulkOn = chosen.filter(isOn);

  return (
    <section className="flex min-w-0 flex-col gap-4" aria-label={name}>
      <div className="flex items-start gap-3">
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <h2 className="truncate font-mono text-[20px] font-bold">{name}</h2>
          <span className="truncate font-mono text-[12px] text-ink-3" title={file.path}>{shortenHome(file.path)}</span>
        </div>
        <Button variant="secondary" size="sm" onClick={() => setEditing(true)} disabled={!content.data}>{t('instructions.shared.edit')}</Button>
        <button type="button" className="ss-ib" aria-label={t('instructions.shared.more')} aria-haspopup="menu" aria-expanded={menu !== null}
          onClick={(e) => { const r = e.currentTarget.getBoundingClientRect(); setMenu({ x: r.right - 200, y: r.bottom + 4 }); }}>
          <Ellipsis size={16} />
        </button>
      </div>

      <div className="ss-list">
        <BoxHeader content={content.data ? text : undefined} view={view} onChange={setView} views={[
          { value: 'preview', label: t('instructions.target.view.preview') },
          { value: 'source', label: t('instructions.target.view.source') },
        ]}>
          {lines.length > PREVIEW_LINES && (
            <Button variant="ghost" size="sm" aria-expanded={expanded} onClick={() => setExpanded(!expanded)}>
              {t(expanded ? 'instructions.preview.collapse' : 'instructions.preview.expand')}
              {expanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
            </Button>
          )}
        </BoxHeader>
        {content.error ? (
          <div className="px-[18px] py-3 text-[13px] text-bad">{instructionsErrorMessage(content.error, t)}</div>
        ) : view === 'preview' ? (
          <InstructionsPreview content={text} names={[]} className={expanded ? '' : 'max-h-[230px] overflow-y-auto'} />
        ) : (
          // Collapsed, both views scroll inside the same height; expanded shows everything.
          <pre className={`px-[18px] pt-3 pb-3.5 font-mono text-[12.5px] leading-[1.7] whitespace-pre-wrap text-ink ${expanded ? '' : 'max-h-[230px] overflow-y-auto'}`} style={{ overflowWrap: 'anywhere' }}>
            {lines.join('\n') || ' '}
          </pre>
        )}
      </div>

      <div className="flex items-center gap-2 pt-1.5 pl-4">
        <Checkbox label={t('instructions.shared.selectAll')} hideLabel checked={allOn} indeterminate={!allOn && chosen.length > 0}
          onChange={() => setSelected(allOn ? new Set() : new Set(selectable.map((tg) => tg.name)))} disabled={selectable.length === 0} />
        <h3 className="ml-2 text-[15px] font-bold">{t('instructions.detail.targets')}</h3>
        {!fileLinks && <Tooltip content={t('instructions.fileLinks.tooltip')}>
          <button type="button" className="ss-ib" aria-label={t('instructions.fileLinks.tooltip')}><Info size={16} /></button>
        </Tooltip>}
        <span className="text-[12.5px] text-ink-3">{t('instructions.shared.connectedCount', { count: connected.length })}</span>
        <span className="flex-1" />
        {staleCount > 0 && (
          <>
            <span className="text-[12.5px] text-warn">{t(staleCount === 1 ? 'instructions.shared.needSync.one' : 'instructions.shared.needSync.other', { count: staleCount })}</span>
            <Button variant="primary" size="sm" onClick={() => void sync()} loading={busy}>{t('extras.sync')}</Button>
          </>
        )}
        {connectAll.some((s) => s.note !== 'held') && <Button variant="ghost" size="sm" disabled={busy} onClick={() => askConnect(connectAll, t('instructions.shared.connectAll'))}>{t('instructions.shared.connectAll')}</Button>}
        {restoreAll.length > 0 && <Button variant="ghost" size="sm" disabled={busy} onClick={() => askRestore(restoreAll, t('instructions.shared.restoreAll'))}>{t('instructions.shared.restoreAll')}</Button>}
      </div>

      <div className="ss-list">
        {targets.map((tg) => {
          const on = isOn(tg);
          const a = tg.assigned.find((x) => x.name === name);
          const hint = rowHint(tg, file);
          const label = t('instructions.shared.switch', { target: tg.name, name });
          return (
            <div key={tg.name} className={`ss-r !block !p-0 ${selected.has(tg.name) ? 'sel' : ''}`}>
              <div className="flex min-h-[56px] items-center gap-3 px-4 py-2">
                <Checkbox label={t('instructions.shared.select', { name: tg.name })} hideLabel checked={selected.has(tg.name)} disabled={Boolean(tg.same_as)}
                  onChange={(v) => { const next = new Set(selected); if (v) next.add(tg.name); else next.delete(tg.name); setSelected(next); }} />
                <span className="ss-at"><AgentIcon target={tg.name} size={17} /></span>
                {/* Not a target of its own: where it comes from is on hover, not a line on every row. */}
                <span className="w-[110px] shrink-0">
                  {tg.rider_of ? (
                    <Tooltip block content={t('instructions.hint.riderOf', { name: tg.rider_of })}>
                      <Link to={`/targets/${encodeURIComponent(tg.rider_of)}?tab=instructions&tool=${encodeURIComponent(tg.name)}`} className="block truncate font-mono text-[13px] font-semibold hover:underline">{targetLabel(tg.name)}</Link>
                    </Tooltip>
                  ) : (
                    <Link to={`/targets/${encodeURIComponent(tg.name)}?tab=instructions`} className="block truncate font-mono text-[13px] font-semibold hover:underline">{tg.name}</Link>
                  )}
                </span>
                <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <span className="truncate font-mono text-[12.5px] text-ink-2" title={tg.path}>{shortenHome(tg.path)}</span>
                  {hint && <span className={`text-[12px] ${hint.kind === 'tooLong' || hint.kind === 'noSource' || hint.kind === 'folderLink' || hint.kind === 'directory' ? 'text-warn' : 'text-ink-3'}`}>{hintText(hint)}</span>}
                </span>
                {on && a && (
                  <Select size="sm" align="end" className="w-[104px] shrink-0 font-mono" ariaLabel={t('instructions.mode.label', { target: tg.name, name })} value={pickedMode(a.mode)} disabled={busy}
                    onChange={(m) => { if (m !== pickedMode(a.mode)) void setMode(tg, m); }}
                    options={modeOptions(tg, fileLinks).map((o) => ({
                      value: o.mode, label: o.mode, note: o.isDefault ? t('instructions.mode.default') : undefined, disabled: Boolean(o.blocked), description: modeText(o, tg),
                    }))} />
                )}
                <span className="w-[90px] shrink-0">{on && a && <span className={`ss-st ${statusTone(a.status)}`}>{a.status}</span>}</span>
                <button type="button" role="switch" aria-checked={on} aria-label={label} title={label}
                  className={`ss-sw ${on ? 'on' : ''} disabled:opacity-50`} disabled={busy || Boolean(tg.same_as)} onClick={() => toggle(tg)}><i /></button>
              </div>
              {held[tg.name] && (
                <div className="ss-note bad mr-4 mb-3 ml-[82px]" role="alert">
                  <TriangleAlert size={16} />
                  <span className="flex-1">{held[tg.name]}</span>
                </div>
              )}
              {on && a?.status === 'modified' && (
                <div className="ss-note warn mr-4 mb-3 ml-[82px] !items-center">
                  <TriangleAlert size={16} className="!mt-0" />
                  <span className="flex-1">{t('instructions.row.modified')}</span>
                  <Button variant="secondary" size="sm" disabled={busy} onClick={() => resolve(tg.name, { target: tg.name }, 'collect')}>{t('instructions.resolve.collect.item', { name })}</Button>
                  <Button variant="secondary" size="sm" disabled={busy} onClick={() => resolve(tg.name, { target: tg.name }, 'reapply')}>{t('instructions.resolve.reapply.item', { name })}</Button>
                </div>
              )}
            </div>
          );
        })}
      </div>

      <div className="flex items-center gap-2 pt-2.5 pl-1">
        <h3 className="text-[15px] font-bold">{t('instructions.locations.title')}</h3>
        {locations.length > 0 && <span className="text-[12.5px] text-ink-3">{t(locations.length === 1 ? 'instructions.locations.count.one' : 'instructions.locations.count.other', { count: locations.length })}</span>}
        <span className="flex-1" />
        <Button variant="secondary" size="sm" disabled={busy} onClick={() => setAddingLocation(true)}><Plus size={14} />{t('instructions.locations.add')}</Button>
      </div>
      {locations.length > 0 ? (
        <div className="ss-list" aria-label={t('instructions.locations.title')}>
          <LocationRows name={name} locations={locations} fileLinks={fileLinks} busy={busy} act={act} warn={warn}
            onResolve={(label, path, action) => resolve(label, { path }, action)} />
        </div>
      ) : (
        <div className="ss-empty !gap-1.5 !p-[26px]">
          <span className="text-[13.5px] font-semibold text-ink">{t('instructions.locations.empty.title')}</span>
          <span className="max-w-[460px] text-[12.5px] leading-normal text-ink-3">{t('instructions.locations.empty.description', { name })}</span>
        </div>
      )}

      {chosen.length > 0 && (
        <div className="ss-bulk" role="toolbar" aria-label={t('instructions.shared.selected', { count: chosen.length })}>
          <b>{t('instructions.shared.selectedShort', { count: chosen.length })}</b>
          <span className="dv" />
          <Button variant="secondary" size="sm" disabled={busy || bulkOff.length === 0} title={bulkOff.length === 0 ? t('instructions.shared.noneOff', { name }) : undefined}
            onClick={() => askConnect(connectPlan(bulkOff, file), t('instructions.shared.connect', { name }))}>
            {t('instructions.shared.connect', { name })}
          </Button>
          <Button variant="secondary" size="sm" disabled={busy || bulkOn.length === 0} title={bulkOn.length === 0 ? t('instructions.shared.noneOn', { name }) : undefined}
            onClick={() => askRestore(restorePlan(bulkOn, name), t('instructions.detail.restore'))}>
            {t('instructions.detail.restore')}
          </Button>
          <span className="dv" />
          <button type="button" className="ss-ib" aria-label={t('instructions.shared.clear')} onClick={() => setSelected(new Set())}><X size={16} /></button>
        </div>
      )}

      {restoring && (
        <RestorePreviewDialog name={name} target={restoring} label={targets.find((tg) => tg.name === restoring)?.rider_of ? targetLabel(restoring) : restoring}
          mode={targets.find((tg) => tg.name === restoring)?.assigned.find((x) => x.name === name)?.mode ?? ''} busy={busy} onClose={() => setRestoring(null)}
          onConfirm={async () => { await act(() => detach([restoring])); setRestoring(null); }} />
      )}
      {addingLocation && (
        <AddLocationDialog name={name} file={file.file} fileLinks={fileLinks} onClose={() => setAddingLocation(false)}
          onAdded={(path, warnings) => {
            warn(warnings);
            toast(t('instructions.locations.added', { path: shortenHome(path), name }), 'success');
            setAddingLocation(false);
            refreshInstructions(queryClient);
          }} />
      )}
      {editing && content.data && (
        <InstructionsEditorDialog
          title={`${name} · ${file.file}`}
          path={file.path}
          content={content.data.content}
          readers={connected.map((tg) => tg.name)}
          warnings={connected.filter((tg) => tg.max_chars && file.chars > tg.max_chars).map((tg) =>
            t('instructions.editor.tooLong', { target: tg.name, chars: file.chars.toLocaleString(), max: tg.max_chars!.toLocaleString() }))}
          onSave={async (next) => {
            const res = await api.putSharedInstructionsContent(name, next);
            refreshInstructions(queryClient);
            // Copy targets were rewritten; say which, and any problem, like the sync button.
            const copies = saveCopiesSummary(res.copies ?? []);
            copies.warnings.forEach((w) => toast(w, 'warning'));
            if (copies.errors.length) toast(copies.errors.join('; '), 'error');
            return copies.updated.length ? t('instructions.savedCopies', { path: shortenHome(file.path), targets: list(copies.updated) }) : undefined;
          }}
          onClose={() => setEditing(false)}
        />
      )}
      <SkillContextMenu open={menu !== null} anchorPoint={menu ?? undefined} onClose={() => setMenu(null)} items={[
        { key: 'copy', label: t('instructions.shared.copyPath'), icon: <Copy size={14} />, onSelect: () => void copyPath() },
        { key: 'delete', label: t('instructions.shared.delete', { name }), icon: <Trash2 size={14} />, danger: true, onSelect: remove },
      ]} />
      <ConfirmDialog
        open={pending !== null}
        title={pending?.title ?? ''}
        message={pending?.message ?? ''}
        confirmText={pending?.confirm}
        variant={pending?.danger ? 'danger' : 'default'}
        loading={busy}
        onCancel={() => setPending(null)}
        onConfirm={async () => {
          const p = pending!;
          await act(p.run);
          setPending(null);
          if (!p.danger) setSelected(new Set());
        }}
      />
    </section>
  );
}

/**
 * The restore plan's rows. A link or copy target goes back to what the record
 * kept, which may be no file at all: the rows ask, as the one-target preview does.
 */
function RestorePlan({ name, steps, note, planList, footer }: {
  name: string;
  steps: RestoreStep[];
  note: (s: RestoreStep) => string;
  planList: (rows: { target: string; note: string; warn: boolean }[], footer: string) => ReactNode;
  footer: string;
}) {
  const t = useT();
  const previews = useQueries({
    queries: steps.map((s) => ({
      queryKey: queryKeys.instructions.restorePreview(name, s.target),
      queryFn: () => api.getSharedRestorePreview(name, s.target),
      enabled: s.note === 'link',
      gcTime: 0,
    })),
  });
  return planList(steps.map((s, i) => {
    if (s.note !== 'link') return { target: s.target, note: note(s), warn: s.note === 'modified' };
    const p = previews[i];
    const text = p.data?.kind === 'delete' ? t('instructions.plan.delete') : p.data || p.error ? note(s) : '…';
    return { target: s.target, note: text, warn: false };
  }), footer);
}
