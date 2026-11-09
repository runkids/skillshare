import { createContext, useContext, useEffect, useId, useMemo, useRef, useState } from 'react';
import { Link, useBeforeUnload, useBlocker, useSearchParams } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowRight, Check, ChevronDown, ChevronUp, FileText, FileX, Info, Link2, Lock, TriangleAlert, X } from 'lucide-react';
import { api } from '../../api/client';
import type { TargetInstructions as Data } from '../../api/client';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import CodeEditor from '../CodeEditor';
import ConfirmDialog from '../ConfirmDialog';
import DialogShell from '../DialogShell';
import EmptyState from '../EmptyState';
import { Checkbox, Input } from '../Input';
import { targetLabel } from '../mcp/mcpView';
import { PageSkeleton } from '../Skeleton';
import { useToast } from '../Toast';
import { useAppContext } from '../../context/AppContext';
import { useT, plural } from '../../i18n';
import { queryKeys } from '../../lib/queryKeys';
import { fileName, shortenHome } from '../../lib/paths';
import ConvertDialog from './ConvertDialog';
import InstructionFileList from './InstructionFileList';
import { useFillHeight } from './useFillHeight';
import { BoxHeader, InstructionsPreview } from './ViewTabs';
import { useSaveShortcut } from './useSaveShortcut';
import { connectedTo, instructionsErrorMessage, importDecor, setupPathOf, setupPathProblem, sharedOfImport } from './instructionsView';
import { invalidate } from '../../lib/queryEvents';
import { useMenuPresence } from '../../hooks/usePresence';


// The page's bottom padding, and the least height the tab keeps on a short window.
const PAGE_BOTTOM = 40;
const MIN_TAB = 480;

// The preview can show the whole file; the tab then drops its fixed height and the page grows.
const Expanded = createContext<{ expanded: boolean; setExpanded: (on: boolean) => void }>({ expanded: false, setExpanded: () => {} });

/**
 * ① A target's Instructions tab, filling the window down to the page bottom so
 * the editor gets the room. Tools that read this target's skills but keep
 * their own instruction file (riders: Codex under universal) are listed beside
 * it; the pick lives in ?tool= so it survives a reload.
 */
export default function TargetInstructions({ name }: { name: string }) {
  const t = useT();
  const [params, setParams] = useSearchParams();
  const { data } = useQuery({ queryKey: queryKeys.instructions.target(name), queryFn: () => api.getTargetInstructions(name) });
  const [fillRef, height] = useFillHeight<HTMLDivElement>(PAGE_BOTTOM, MIN_TAB);
  const [expanded, setExpanded] = useState(false);
  const fixed = expanded ? undefined : height;
  const riders = data?.riders ?? [];
  if (!data?.supported || riders.length === 0) {
    return <Expanded.Provider value={{ expanded, setExpanded }}><div ref={fillRef} style={{ height: fixed }} className="flex flex-col"><Panel name={name} /></div></Expanded.Provider>;
  }

  const tool = riders.find((r) => r.name === params.get('tool'))?.name ?? name;
  // Switching tools is a navigation, so the editor's guard asks before an unsaved edit is dropped.
  const pick = (next: string) => setParams((prev) => {
    const p = new URLSearchParams(prev);
    if (next === name) p.delete('tool');
    else p.set('tool', next);
    return p;
  }, { replace: true });
  // Tools by the name people know them by (Codex, not codex); the target itself keeps its own name.
  const items = [
    { id: name, label: name, path: data.path ?? '', exists: data.exists },
    ...riders.map((r) => ({ id: r.name, label: targetLabel(r.name), path: r.path, exists: r.exists })),
  ];

  return (
    <Expanded.Provider value={{ expanded, setExpanded }}>
    <div ref={fillRef} style={{ height: fixed }} className="grid grid-cols-[220px_minmax(0,1fr)] gap-7">
      <InstructionFileList items={items} selected={tool} onSelect={pick} divider={1} caption={t('instructions.files.caption', { name })} />
      <div className="flex min-h-0 min-w-0 flex-col">
        <Panel key={tool} name={tool} />
      </div>
    </div>
    </Expanded.Provider>
  );
}

/** The files a target (or rider) reads, in order, and an editor for its own file. */
function Panel({ name }: { name: string }) {
  const t = useT();
  const { data, error, isPending } = useQuery({ queryKey: queryKeys.instructions.target(name), queryFn: () => api.getTargetInstructions(name) });
  if (isPending) return <PageSkeleton />;
  if (error) return <div className="ss-note bad"><span className="flex-1">{instructionsErrorMessage(error, t)}</span></div>;
  // skillshare does not know the file: let the user say which one the tool reads.
  if (!data.supported && name !== 'cursor') return <SetupFormInline data={data} />;
  if (!data.supported) {
    return (
      <EmptyState
        icon={FileX}
        title={t('instructions.target.none.title', { name })}
        description={t(name === 'cursor' && !data.project ? 'instructions.target.none.cursor' : 'instructions.target.none.description', { name })}
      />
    );
  }
  // Remount on a new file version so the draft starts from it.
  return <Editor key={`${data.path}:${data.content}`} data={data} />;
}

/**
 * Which file the target reads: for a tool skillshare does not know, or to move
 * a known tool's file elsewhere (builtIn: it has a default to go back to).
 * onDone runs after a save or reset.
 */
function useSetupForm(data: Data, onDone?: () => void) {
  const t = useT();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const { projectRoot } = useAppContext();
  const id = useId();
  const builtIn = Boolean(data.default_path);
  const [path, setPath] = useState(data.setup?.path ?? (data.supported && data.path ? setupPathOf(data.path, data.project, projectRoot) : ''));
  const [imports, setImports] = useState(data.setup?.import ?? (data.supported && data.import));
  const [saving, setSaving] = useState(false);
  const [resetting, setResetting] = useState(false);
  // What the server said about the last save or reset, shown in the form as it came.
  const [failure, setFailure] = useState('');
  const problem = setupPathProblem(path, data.project);
  const example = data.project ? `.${data.target}/AGENTS.md` : `~/.${data.target}/AGENTS.md`;


  const save = async () => {
    setSaving(true);
    setFailure('');
    try {
      await api.setTargetInstructionsSetup(data.target, { path: path.trim(), import: imports });
      void invalidate(queryClient, 'instructionsChanged');
      toast(t('instructions.setup.saved'), 'success');
      onDone?.();
    } catch (err) {
      setFailure(instructionsErrorMessage(err, t));
    } finally {
      setSaving(false);
    }
  };

  // Back to the built-in file, or, for a tool skillshare does not know, no file.
  const reset = async () => {
    setResetting(true);
    setFailure('');
    try {
      await api.removeTargetInstructionsSetup(data.target);
      void invalidate(queryClient, 'instructionsChanged');
      toast(t('instructions.setup.removed'), 'success');
      onDone?.();
    } catch (err) {
      setFailure(instructionsErrorMessage(err, t));
    } finally {
      setResetting(false);
    }
  };

  const title = t(builtIn ? 'instructions.setup.builtInTitle' : 'instructions.setup.title', { name: data.target });
  const description = builtIn ? t('instructions.setup.builtInDescription', { path: shortenHome(data.default_path ?? '') }) : t('instructions.setup.description', { name: data.target });
  const fields = (
    <>
      <div className="ss-fld">
        <label htmlFor={id}>{t(builtIn ? 'instructions.setup.locationLabel' : 'instructions.setup.pathLabel')}</label>
        <Input id={id} value={path} onChange={(e) => setPath(e.target.value)} placeholder={example} className="font-mono" spellCheck={false} autoComplete="off" />
        <span className={`hp ${problem ? '!text-bad' : ''}`}>
          {problem ? t(`instructions.setup.problem.${problem}`)
            : builtIn && !data.project ? t('instructions.setup.pathHelpBuiltIn')
              : t(data.project ? 'instructions.setup.pathHelpProject' : 'instructions.setup.pathHelp', { example })}
        </span>
      </div>
      <Checkbox label={t('instructions.setup.import')} checked={imports} onChange={setImports} size="sm" />
      {failure && <div className="ss-note bad"><span className="flex-1">{failure}</span></div>}
    </>
  );
  const busy = saving || resetting;
  const saveButton = <Button variant="primary" size="sm" onClick={save} loading={saving} disabled={!path.trim() || problem !== null || resetting}>{t('common.save')}</Button>;
  const resetButton = data.custom && (
    <Button variant="ghost" size="sm" onClick={reset} loading={resetting} disabled={saving}>{t(builtIn ? 'instructions.setup.reset' : 'instructions.setup.remove')}</Button>
  );
  return { title, description, fields, busy, saveButton, resetButton };
}

/** The location form in the page, for a target with no known file: there is nothing else to show. */
function SetupFormInline({ data }: { data: Data }) {
  const f = useSetupForm(data);
  return (
    <div className="ss-box flex max-w-[560px] flex-col gap-4">
      <div className="flex flex-col gap-1">
        <h3 className="font-semibold text-ink">{f.title}</h3>
        <p className="text-[13px] text-ink-2">{f.description}</p>
      </div>
      {f.fields}
      <div className="flex items-center gap-2">{f.saveButton}</div>
    </div>
  );
}

/** Change which file a target reads. */
function PathDialog({ data, onClose }: { data: Data; onClose: () => void }) {
  const t = useT();
  const f = useSetupForm(data, onClose);
  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={f.busy} ariaLabel={f.title} className="!max-w-[540px]">
      <div className="dh">
        <div className="flex min-w-0 flex-col gap-1">
          <h2 className="ss-h2">{f.title}</h2>
          <p className="text-[13px] text-ink-2">{f.description}</p>
        </div>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={f.busy}><X size={16} /></button>
      </div>
      <div className="db flex flex-col gap-4">{f.fields}</div>
      <div className="df">
        {f.resetButton}
        <span className="flex-1" />
        <Button variant="ghost" size="sm" onClick={onClose} disabled={f.busy}>{t('common.cancel')}</Button>
        {f.saveButton}
      </div>
    </DialogShell>
  );
}

function Editor({ data }: { data: Data }) {
  const t = useT();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState(data.content);
  const [saving, setSaving] = useState(false);
  const [converting, setConverting] = useState(false);
  const [changingPath, setChangingPath] = useState(false);
  // Preview renders the draft, so switching keeps unsaved edits.
  // A file with content opens as it reads; an empty or missing one opens ready to write.
  const [view, setView] = useState<'edit' | 'preview'>(data.exists && data.content.trim() ? 'preview' : 'edit');
  const { expanded, setExpanded } = useContext(Expanded);
  const path = data.path ?? '';
  const file = fileName(path);
  const linked = Boolean(data.link_shared);
  const shared = data.shared;
  const tooLong = Boolean(data.max_chars) && [...draft].length > (data.max_chars ?? 0);
  const importNote = !data.project && data.convert.includes('import') && shared.length === 0;

  // Says at the end of each @import line who expands it; the import-target
  // wording is about converting, so it drops that part once nothing is left to convert.
  const lineDecor = useMemo(() => {
    const names = shared.map((s) => s.name);
    const why = !data.import ? t('instructions.target.lineNote.other')
      : data.convert.length > 0 ? t('instructions.target.lineNote.import', { name: data.target, file })
        : t('instructions.target.lineNote.importOnly', { name: data.target });
    return (lines: string[]) => importDecor(lines, (line, inBlock) => {
      const name = inBlock ? sharedOfImport(line, names) : undefined;
      return name ? `${name} · ${why}` : why;
    });
  }, [shared, data.import, data.convert.length, data.target, file, t]);

  // Leaving with an unsaved edit asks first: links, the sidebar, other tools in the list, a reload.
  const dirty = draft !== data.content;
  const blocker = useBlocker(({ currentLocation, nextLocation }) => dirty
    && (currentLocation.pathname !== nextLocation.pathname || currentLocation.search !== nextLocation.search));
  useBeforeUnload((e) => { if (dirty) { e.preventDefault(); e.returnValue = ''; } });

  const save = async () => {
    setSaving(true);
    try {
      await api.putTargetInstructions(data.target, draft);
      void invalidate(queryClient, 'instructionsChanged');
      toast(t('instructions.saved', { path: shortenHome(path) }), 'success');
    } catch (err) {
      toast(instructionsErrorMessage(err, t), 'error');
    } finally {
      setSaving(false);
    }
  };
  // Only an editable file with changes; the same in Edit and Preview.
  useSaveShortcut(() => {
    if (!saving && draft !== data.content) void save();
  }, !linked);


  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4">
      {importNote && (
        <div className="ss-note inf">
          <Info size={16} />
          <span className="flex-1">{t('instructions.target.importNote', { name: data.target, file })}</span>
          <Button variant="secondary" size="sm" onClick={() => setConverting(true)}>{t('instructions.convert.open')}</Button>
        </div>
      )}

      {/* The page header shows the target's own file; another tool's file (picked on universal's page) names itself here. */}
      {data.rider_of && <span className="min-w-0 truncate font-mono text-[14px] font-semibold" title={path}>{shortenHome(path)}</span>}
      <SourceCard data={data} onChangeLocation={() => setChangingPath(true)} onConvert={() => setConverting(true)} convertable={data.convert.length > 0 && !linked && !importNote} dirty={draft !== data.content} />

      <div className="ss-code flex min-h-[360px] flex-1 flex-col !bg-surface !overflow-hidden !p-0 !whitespace-normal">
        {/* A linked file is read only here: its second tab shows the text, it does not edit it. */}
        <BoxHeader content={draft} view={view} onChange={setView} views={linked ? [{ value: 'preview', label: t('instructions.target.view.preview') }, { value: 'edit', label: t('instructions.target.view.source') }] : undefined}>
          <Button variant="ghost" size="sm" aria-expanded={expanded} onClick={() => setExpanded(!expanded)}>
            {t(expanded ? 'instructions.preview.collapse' : 'instructions.preview.expand')}
            {expanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
          </Button>
          {/* Only once there is something to save; Cmd+S works either way. */}
          {!linked && draft !== data.content && <Button variant="primary" size="sm" onClick={save} loading={saving}>{t('common.save')}</Button>}
        </BoxHeader>
        {view === 'edit' ? (
          <CodeEditor value={draft} onChange={setDraft} ariaLabel={file} lineDecor={lineDecor} disabled={saving || linked} wrap fill={!expanded} minHeight="360px" maxHeight="none"
            className="min-h-0 flex-1 !rounded-none !border-0 !bg-surface" placeholder={t('instructions.target.placeholder', { file })} />
        ) : (
          <InstructionsPreview content={draft} names={shared.map((s) => s.name)} className={expanded ? '' : undefined} />
        )}
      </div>
      {tooLong && (
        <p className="flex items-center gap-2 text-[13px] text-warn">
          <TriangleAlert size={15} className="shrink-0" />
          {t('instructions.tooLong', { name: data.target, max: data.max_chars?.toLocaleString() ?? '' })}
        </p>
      )}

      {converting && <ConvertDialog data={{ ...data, content: draft }} onClose={() => setConverting(false)} />}
      {changingPath && <PathDialog data={data} onClose={() => setChangingPath(false)} />}
      <ConfirmDialog
        open={blocker.state === 'blocked'}
        title={t('config.discard.title')}
        message={t('config.discard.message')}
        confirmText={t('config.discard.confirmText')}
        variant="danger"
        onConfirm={() => blocker.state === 'blocked' && blocker.proceed()}
        onCancel={() => blocker.state === 'blocked' && blocker.reset()}
      />
    </div>
  );
}

/**
 * Where the target's file comes from, as one card: a shared AGENTS.md it follows
 * (linked, read only) or imports, with who else uses it, or the target's own file.
 */
function SourceCard({ data, onChangeLocation, onConvert, convertable, dirty }: {
  data: Data;
  onChangeLocation: () => void;
  onConvert: () => void;
  convertable: boolean;
  dirty: boolean;
}) {
  const t = useT();
  const list = useQuery({ queryKey: queryKeys.instructions.shared, queryFn: () => api.listSharedInstructions(), enabled: !data.project });
  const linked = data.link_shared;
  const names = linked ? [linked] : data.shared.map((s) => s.name);
  const source = names[0];
  // Everyone on the source except this target; one name for a link, the first import otherwise.
  const users = source ? connectedTo(list.data?.targets ?? [], source).map((tg) => tg.name) : [];
  const others = users.filter((n) => n !== data.target);
  const extras = `/extras?tab=instructions${source ? `&file=${encodeURIComponent(source)}` : ''}`;
  const readBy = data.read_by.map(targetLabel).join(t('instructions.shared.listSep'));
  const unread = data.read_order.some((e) => e.kind === 'unread');
  // The server refuses to move a file a shared AGENTS.md is written into.
  const canMove = !data.rider_of && names.length === 0;
  const notes = [
    linked ? t('instructions.card.linkedNote', { name: linked, count: Math.max(users.length, 1) })
      : source ? t('instructions.card.importNote', { names: names.join(t('instructions.shared.listSep')), count: Math.max(users.length, 1) })
        : t('instructions.card.ownNote', { name: data.target }),
    !data.exists && t('instructions.card.notCreated'),
    readBy && t('instructions.target.readBy', { names: readBy }),
    unread && t('instructions.target.unreadShort'),
  ].filter(Boolean).reduce<string>((all, next) => (all ? all + (/[。！？]$/.test(all) ? '' : ' ') + next : next as string), '');
  return (
    <div className="flex flex-col gap-1.5 rounded-xl border border-line bg-surface px-4 py-3">
      <div className="flex min-h-8 flex-wrap items-center gap-2.5">
        {source ? <Link2 size={15} className="shrink-0 text-ink-3" /> : <FileText size={15} className="shrink-0 text-ink-3" />}
        {source && names.map((n) => (
          <Link key={n} to={`/extras?tab=instructions&file=${encodeURIComponent(n)}`} className="font-mono font-semibold hover:underline">{n}</Link>
        ))}
        {source && <ArrowRight size={14} className="shrink-0 text-ink-3" />}
        <span className="inline-flex items-center gap-1.5 font-semibold"><AgentIcon target={data.target} size={15} />{data.target}</span>
        {!source && <span className="ss-tag">{t('instructions.card.own')}</span>}
        {others.length > 0 && (
          <>
            <span className="mx-1 h-4 w-px bg-line" aria-hidden="true" />
            <span className="ss-stack" role="img" aria-label={others.join(', ')} title={others.join(', ')}>
              {others.slice(0, 5).map((n) => <span key={n} className="ss-at !h-5 !w-5"><AgentIcon target={n} size={11} /></span>)}
            </span>
            <span className="text-[12.5px] text-ink-3">{t(plural('instructions.card.alsoUsed', others.length), { count: others.length })}</span>
          </>
        )}
        <span className="flex-1" />
        {convertable && <button type="button" className="ss-btn ghost sm" onClick={onConvert} disabled={dirty}>{t('instructions.convert.open')}</button>}
        {canMove && <button type="button" className="ss-btn ghost sm" onClick={onChangeLocation}>{t('instructions.setup.changeLocation')}</button>}
        {!data.project && !data.rider_of && (source
          ? (
            <>
              {names.length === 1
                ? <ChangeMenu target={data.target} current={source} linked={!!linked} files={list.data?.files.map((f) => f.name) ?? []} />
                : <Link to="/extras?tab=instructions" className="ss-btn ghost sm">{t('instructions.card.change')}</Link>}
              {names.length === 1 && <Link to={extras} className="ss-btn sm">{t('instructions.card.edit', { name: source })}<ArrowRight size={13} /></Link>}
            </>
          )
          : <Link to="/extras?tab=instructions" className="ss-btn sm"><Link2 size={13} />{t('instructions.card.connect')}</Link>)}
      </div>
      <p className="flex items-start gap-1.5 pl-[25px] text-[12.5px] text-ink-3">
        {linked && <Lock size={12} className="mt-[3px] shrink-0" />}
        <span>{notes}</span>
      </p>
    </div>
  );
}

/**
 * Switches the target to another shared AGENTS.md in place. A link can hold only
 * one file, so leaving the current one asks first; an import just swaps the line.
 */
function ChangeMenu({ target, current, linked, files }: { target: string; current: string; linked: boolean; files: string[] }) {
  const t = useT();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const menu = useMenuPresence(open);
  const [picked, setPicked] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  const switchTo = async (name: string) => {
    setBusy(true);
    try {
      const res = await api.assignSharedInstructions([target], [name]);
      if (!res.success) throw new Error(res.errors.join('; '));
      toast(t('instructions.shared.assigned', { targets: target, files: name }), 'success');
    } catch (err) {
      toast(instructionsErrorMessage(err, t), 'error');
    } finally {
      setBusy(false);
      setPicked(null);
      void invalidate(queryClient, 'instructionsChanged');
    }
  };
  const pick = (name: string) => {
    setOpen(false);
    if (name === current) return;
    if (linked) setPicked(name);
    else void switchTo(name);
  };

  return (
    <div ref={ref} className="relative shrink-0">
      <button type="button" className="ss-btn ghost sm" aria-haspopup="menu" aria-expanded={open} disabled={busy} onClick={() => setOpen(!open)}>
        {t('instructions.card.change')}<ChevronDown size={13} />
      </button>
      {menu.shown && (
        <div role="menu" aria-label={t('instructions.card.change')} {...menu.exitProps} className={`ss-menu absolute right-0 top-full z-50 mt-1 !w-60 origin-top-right ${menu.motion}`}>
          {files.map((n) => (
            <button key={n} type="button" role="menuitemradio" aria-checked={n === current} onClick={() => pick(n)}>
              <span className="min-w-0 flex-1 truncate font-mono">{n}</span>
              {n === current && <Check size={14} />}
            </button>
          ))}
          <hr />
          <Link to="/extras?tab=instructions" role="menuitem">{t('instructions.card.manage')}</Link>
        </div>
      )}
      <ConfirmDialog
        open={picked !== null}
        title={t('instructions.switchFrom.title', { target, name: picked ?? '' })}
        message={t('instructions.switchFrom.message', { target, other: current })}
        confirmText={t('instructions.switchFrom.confirm', { name: picked ?? '' })}
        loading={busy}
        onConfirm={() => picked && void switchTo(picked)}
        onCancel={() => setPicked(null)}
      />
    </div>
  );
}
