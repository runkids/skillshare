import { useState } from 'react';
import { useQueries } from '@tanstack/react-query';
import { Download, Pencil, Plus, Trash2, TriangleAlert, X } from 'lucide-react';
import { hookAgents, hooksApi } from '../../api/hooks';
import { ApiError } from '../../api/http';
import type { HookCandidate, HookInventory, HookPreview } from '../../api/hooks';
import { plural, useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import { queryKeys } from '../../lib/queryKeys';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import { Checkbox } from '../Checkbox';
import CodeEditor from '../CodeEditor';
import DialogShell from '../DialogShell';
import { Select } from '../Input';
import Spinner from '../Spinner';
import { bindingLines, suggestName } from './hookCatalog';
import HooksImportEditor from './HooksImportEditor';
import HooksImportRow from './HooksImportRow';
import { nativeEdits, rowEvents, rowTitle, sameEvents } from './hooksImport';
import type { ImportRow, NativeChange } from './hooksImport';
import HooksPreview from './HooksPreview';
import { HOOK_NAME, hookAccounts, agentOfKey, keyLabel, isCodeAgent, scopeEntries, scopePaths, scopeUnmanaged } from './hooksView';

interface Props {
  data: HookInventory;
  /** A root under hooks.projects: read that folder's target files and save into that project. */
  project?: string;
  onClose: () => void;
  /** How many hooks were imported, and how many were changed or removed in their target files instead. */
  onImported: (count: number, changed: number) => void;
  /** A save failed after it may have written something: what the page shows is out of date. */
  onChanged: () => void;
}

const PASTE = 'paste';

/**
 * Reads the hooks each target has that Skillshare does not manage yet, or pasted text, one target at a
 * time. Checked hooks are saved into the source, one hook each, as the owner of the registrations they
 * came from. A hook can also be edited or removed in its target file without importing it: that stays a
 * pending change until the review step, which shows the file diff and writes on confirmation.
 * Reading never executes anything.
 */
export default function HooksImportDialog({ data, project, onClose, onImported, onChanged }: Props) {
  const t = useT();
  const existing = scopeEntries(data, project);
  const paths = scopePaths(data, project);
  const unmanaged = scopeUnmanaged(data, project);
  const accounts = project ? {} : hookAccounts(data.targets);
  const agentOf = (key: string) => agentOfKey(accounts, key);
  const labelOf = (key: string) => keyLabel(accounts, key);
  const available = [...hookAgents.filter((a) => a !== 'git'), ...Object.keys(accounts).sort()];
  const targets = available.filter((a) => unmanaged.some((u) => u.target === a));
  const reads = useQueries({
    queries: targets.map((from) => ({
      queryKey: queryKeys.hooksImport(project ?? '', from),
      queryFn: () => hooksApi.import({ from, ...(project && { root: project }) }),
      gcTime: 0,
      retry: false,
    })),
  });
  const [picked, setPicked] = useState(targets[0] ?? PASTE);
  const [pasteFrom, setPasteFrom] = useState<string>(hookAgents[0]);
  const [content, setContent] = useState('');
  const [pasted, setPasted] = useState<HookCandidate[] | null>(null);
  const [reading, setReading] = useState(false);
  const [names, setNames] = useState<Record<string, string>>({});
  const [unchecked, setUnchecked] = useState<string[]>([]);
  const [opened, setOpened] = useState('');
  const [changes, setChanges] = useState<Record<string, NativeChange>>({});
  // Per target, the only rows still picked for import after a write to its file went through.
  const [kept, setKept] = useState<Record<string, string[]>>({});
  const [editing, setEditing] = useState('');
  const [footerHost, setFooterHost] = useState<HTMLElement | null>(null);
  const [plan, setPlan] = useState<HookPreview | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const busy = saving || reading;

  const rows: ImportRow[] = [
    ...targets.flatMap((target, i) => (reads[i].data ?? []).map((candidate) => ({ id: `${target}:${candidate.name}`, target, candidate, adopt: true }))),
    ...(pasted ?? []).map((candidate) => ({ id: `${PASTE}:${candidate.name}`, target: pasteFrom, candidate, adopt: false })),
  ];
  // Prefilled names never repeat a saved hook or each other.
  const defaults: Record<string, string> = {};
  for (const row of rows) {
    const line = bindingLines(row.candidate.entry.bindings[row.target])[0];
    const taken = (n: string) => n in existing || Object.values(defaults).includes(n);
    defaults[row.id] = suggestName(row.candidate.name, isCodeAgent(agentOf(row.target)) ? '' : line?.command ?? '', taken);
  }
  // An Antigravity hook is the block of that name, so taking one over keeps the block's name.
  const fixedName = (row: ImportRow) => row.adopt && row.target === 'antigravity';
  const nameOf = (row: ImportRow) => (fixedName(row) ? row.candidate.name : names[row.id] ?? defaults[row.id]);
  const nameError = (row: ImportRow) => {
    const name = nameOf(row).trim();
    if (!HOOK_NAME.test(name)) return t('hooks.nameHint');
    if (name in existing || rows.some((r) => r.id !== row.id && checked(r) && nameOf(r).trim() === name)) return t('mcp.nameTaken');
    return '';
  };
  const importable = (row: ImportRow) => row.candidate.problems.length === 0;
  // A change counts while the target still differs from it: once a write went through and the hook was
  // read again, an edit equals what is there and a removed hook has no row.
  const changeOf = (row: ImportRow): NativeChange | undefined => {
    const change = changes[row.id];
    return change && ('remove' in change || !sameEvents(change.events, rowEvents(row))) ? change : undefined;
  };
  // A hook with a pending edit or removal is written to its target file as it is, not imported in the same pass.
  const checked = (row: ImportRow) => importable(row) && !changeOf(row) && !unchecked.includes(row.id) && (!row.adopt || !kept[row.target] || kept[row.target].includes(row.id));
  const check = (ids: string[], on: boolean) => {
    setUnchecked((prev) => (on ? prev.filter((x) => !ids.includes(x)) : [...new Set([...prev, ...ids])]));
    if (on) setKept((prev) => Object.fromEntries(Object.entries(prev).map(([target, list]) => [target, [...list, ...rows.filter((r) => r.target === target && ids.includes(r.id)).map((r) => r.id)]])));
  };
  const chosen = rows.filter(checked);
  const blocked = chosen.some((r) => nameError(r));
  const changed = rows.filter(changeOf);
  const edits = nativeEdits(changed.map((r) => ({ target: r.target, root: project, before: rowEvents(r), change: changeOf(r)! })));
  // The hook being edited, while its target still has it.
  const row = rows.find((r) => r.id === editing);
  const undo = (id: string) => setChanges((prev) => Object.fromEntries(Object.entries(prev).filter(([key]) => key !== id)));

  const read = async () => {
    setReading(true);
    setError('');
    try {
      setPasted(await hooksApi.import({ from: pasteFrom, content, ...(project && { root: project }) }));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setReading(false);
    }
  };

  const review = async () => {
    setSaving(true);
    setError('');
    try {
      setPlan(await hooksApi.previewNative(edits));
    } catch (e) {
      setError((e as Error).message);
      // Usually a target file that changed since it was read: read again so the next review can pass.
      for (const r of reads) void r.refetch();
    } finally {
      setSaving(false);
    }
  };

  const save = async () => {
    setSaving(true);
    setError('');
    let written = false;
    try {
      // The target files first: an import changes ownership, which would outdate the revision just reviewed.
      if (plan) {
        await hooksApi.applyNative(edits, plan.revision);
        written = true;
      }
      // One at a time: each save previews and sends its own revision, so a parallel batch would be refused.
      for (const row of chosen) {
        await hooksApi.save({ ...(project && { project }), name: nameOf(row).trim(), entry: row.candidate.entry, ...(row.adopt && { adopt: true }) });
      }
      onImported(chosen.length, edits.length);
    } catch (e) {
      setError((e as Error).message);
      setSaving(false);
      setPlan(null);
      // A write that failed after its first file names the files it already wrote.
      const applied = !written && plan && e instanceof ApiError && Array.isArray(e.params?.applied) ? e.params.applied as string[] : [];
      const done = changed.filter((r) => written || applied.includes(paths[r.target] ?? ''));
      if (done.length > 0) {
        // Those files hold their changes now, so none of them is sent again; a file the write did not reach
        // keeps its changes pending. Read again, a change may show up as other rows than it was made in (an
        // edit can move a command to another event), so in the targets written only what was picked before
        // stays picked: a hook changed in place was not meant to be imported.
        const frozen = Object.fromEntries([...new Set(done.map((r) => r.target))].map((target) => [target, chosen.filter((r) => r.target === target).map((r) => r.id)]));
        setKept((prev) => ({ ...prev, ...frozen }));
        setChanges((prev) => Object.fromEntries(Object.entries(prev).filter(([id]) => !done.some((r) => r.id === id))));
      }
      // Otherwise nothing is known to be written; changeOf drops any change the files turn out to hold.
      for (const r of reads) void r.refetch();
      onChanged();
    }
  };

  const failure = error && <div className="ss-note bad" role="alert"><TriangleAlert size={16} /><span className="flex-1">{error}</span></div>;

  if (plan) {
    const title = t('hooks.importReview');
    const removed = changed.filter((r) => 'remove' in changeOf(r)!);
    const edited = changed.filter((r) => 'events' in changeOf(r)!);
    const tile = 'grid h-[26px] w-[26px] shrink-0 place-items-center rounded-sm';
    const hooksOf = (list: ImportRow[]) => list.map((r) => (
      <span key={r.id} className="inline-flex items-center gap-1.5"><AgentIcon target={agentOf(r.target)} size={13} /><span className="font-mono text-ink">{rowTitle(r)}</span></span>
    ));
    const sep = t('hooks.importSum.listSep');
    const labels = (list: string[]) => [...new Set(list)].map(labelOf).join(sep);
    const counts = [...new Set(chosen.map((r) => r.target))].map((target) => t('hooks.importSum.count', { target: labelOf(target), count: String(chosen.filter((r) => r.target === target).length) })).join(sep);
    const wrote = new Set(changed.map((r) => r.target));
    const others = labels(targets.filter((target) => !wrote.has(target)));
    const backupNote = t(plural('hooks.importBackupNote', plan.files?.length ?? wrote.size));
    return (
      <DialogShell open onClose={onClose} padding="none" preventClose ariaLabel={title} className="!max-w-[720px]">
        <div className="dh">
          <div className="flex flex-col gap-1">
            <h2 className="ss-h2">{title}</h2>
            <p className="text-[13px] text-ink-2">{t('hooks.importReviewHint')}</p>
          </div>
          <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={busy}><X size={16} /></button>
        </div>
        <div className="db !gap-5">
          <div className="overflow-hidden rounded-lg border border-line">
            {chosen.length > 0 && (
              <div className="flex min-h-[52px] items-center gap-3 px-4 py-2">
                <span className={`${tile} bg-sunken text-ink-2`}><Download size={15} /></span>
                <span className="flex min-w-0 flex-1 flex-col gap-px">
                  <span className="font-semibold">{t('hooks.importSum.import', { count: String(chosen.length) })}</span>
                  <span className="text-[13px] text-ink-2">{t('hooks.importSum.importHint', { list: counts })}</span>
                </span>
              </div>
            )}
            {edited.length > 0 && (
              <div className="flex min-h-[52px] items-center gap-3 border-t border-line px-4 py-2 first:border-t-0">
                <span className={`${tile} bg-sunken text-ink-2`}><Pencil size={15} /></span>
                <span className="flex min-w-0 flex-1 flex-col gap-px">
                  <span className="font-semibold">{t('hooks.importSum.edit', { count: String(edited.length) })}</span>
                  <span className="flex flex-wrap items-center gap-x-2.5 gap-y-0.5 text-[13px] text-ink-2">{hooksOf(edited)}</span>
                </span>
              </div>
            )}
            {removed.length > 0 && (
              <div className="flex min-h-[52px] items-center gap-3 border-t border-line px-4 py-2 first:border-t-0">
                <span className={`${tile} bg-bad-bg text-bad`}><Trash2 size={15} /></span>
                <span className="flex min-w-0 flex-1 flex-col gap-px">
                  <span className="font-semibold">{t('hooks.importSum.remove', { count: String(removed.length) })}</span>
                  <span className="flex flex-wrap items-center gap-x-2.5 gap-y-0.5 text-[13px] text-ink-2">{hooksOf(removed)}<span>{t('hooks.importSum.removeHint', { target: labels(removed.map((r) => r.target)) })}</span></span>
                </span>
              </div>
            )}
          </div>
          <div className="flex flex-col gap-2">
            <h3 className="text-[13px] font-semibold">{t('hooks.importFiles')}</h3>
            <HooksPreview plan={plan} />
            <p className="text-[13px] leading-normal text-ink-2">{backupNote}{others && `${/[.!?]$/.test(backupNote) ? ' ' : ''}${t('hooks.importBackupOthers', { targets: others })}`}</p>
          </div>
          {failure}
        </div>
        <div className="df">
          <Button variant="ghost" onClick={() => setPlan(null)} disabled={busy}>{t('common.back')}</Button>
          <Button variant="primary" loading={saving} onClick={() => void save()}>{t('hooks.importApply')}</Button>
        </div>
      </DialogShell>
    );
  }

  const list = (shown: ImportRow[]) => shown.map((row) => {
    const editable = row.candidate.editable;
    return (
      <HooksImportRow
        key={row.id}
        row={row}
        targetLabel={labelOf(row.target)}
        change={changeOf(row)}
        checked={checked(row)}
        importable={importable(row)}
        open={opened === row.id}
        busy={busy}
        name={nameOf(row)}
        nameFixed={fixedName(row)}
        nameError={checked(row) ? nameError(row) : ''}
        onCheck={(on) => check([row.id], on)}
        onToggle={() => setOpened((prev) => (prev === row.id ? '' : row.id))}
        onName={(name) => setNames((prev) => ({ ...prev, [row.id]: name }))}
        onEdit={editable ? () => setEditing(row.id) : undefined}
        onRemove={editable ? () => setChanges((prev) => ({ ...prev, [row.id]: { remove: true } })) : undefined}
        onUndo={() => undo(row.id)}
      />
    );
  });

  const pending = changed.length;
  const sources = new Set(chosen.map((r) => (r.adopt ? r.target : PASTE))).size;
  const active = picked === PASTE || targets.includes(picked) ? picked : PASTE;
  const at = targets.indexOf(active);
  const found = rows.filter((r) => r.adopt && r.target === active);
  const ready = found.filter((r) => importable(r) && !changeOf(r));
  const title = t('hooks.importTitle');
  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={busy || pending > 0 || Boolean(row)} ariaLabel={title} className="!max-w-[1120px] !bg-bg">
      <div className="dh">
        <div className="flex flex-col gap-1">
          <h2 className="ss-h2">{title}</h2>
          <p className="text-[13px] text-ink-2">{t('hooks.importHint')}</p>
        </div>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={busy}><X size={16} /></button>
      </div>
      <div className="flex h-[620px] max-h-[calc(100vh-240px)] min-h-0 gap-5 px-[22px] pt-[18px]">
        <nav aria-label={t('hooks.importTargets')} className="flex w-[188px] shrink-0 flex-col gap-px overflow-y-auto">
          <span className="px-2.5 pb-2 pt-0.5 text-xs font-medium text-ink-3">{t('hooks.importTargets')}</span>
          {targets.map((target, i) => {
            const mine = rows.filter((r) => r.adopt && r.target === target);
            return (
              <button key={target} type="button" className={`ss-nv shrink-0 text-left transition-colors duration-150 ${active === target ? 'on' : ''}`} aria-current={active === target || undefined} disabled={Boolean(row)} onClick={() => setPicked(target)}>
                <AgentIcon target={agentOf(target)} size={16} />
                <span className="min-w-0 flex-1 truncate">{labelOf(target)}</span>
                {mine.some((r) => checked(r) && nameError(r)) && <TriangleAlert size={13} className="shrink-0 text-bad" aria-label={t('mcp.nameTaken')} />}
                {reads[i].isPending ? <Spinner size="sm" /> : <span className="shrink-0 font-mono text-xs text-ink-3">{mine.filter(checked).length} / {mine.length}</span>}
              </button>
            );
          })}
          {!row && (
            <>
              <span className="flex-1" />
              <button type="button" className={`ss-nv !h-auto shrink-0 items-start py-2 text-left text-[13px] transition-colors duration-150 ${active === PASTE ? 'on' : ''}`} aria-current={active === PASTE || undefined} onClick={() => setPicked(PASTE)}>
                <Plus size={14} className="mt-0.5 shrink-0" /><span className="min-w-0 flex-1">{t('hooks.importPasteOpen')}</span>
              </button>
            </>
          )}
        </nav>
        {row ? (
          <HooksImportEditor
            key={row.id}
            agent={agentOf(row.target)}
            label={labelOf(row.target)}
            title={rowTitle(row)}
            path={paths[row.target]}
            footerHost={footerHost}
            events={(() => { const change = changeOf(row); return change && 'events' in change ? change.events : rowEvents(row); })()}
            original={rowEvents(row)}
            onCancel={() => setEditing('')}
            onDone={(events) => { if (events) setChanges((prev) => ({ ...prev, [row.id]: { events } })); else undo(row.id); setEditing(''); }}
            onRemove={() => { setChanges((prev) => ({ ...prev, [row.id]: { remove: true } })); setEditing(''); }}
          />
        ) : active === PASTE ? (
          <section aria-label={t('hooks.importPaste')} className="ss-list flex min-w-0 flex-1 flex-col">
            <div className="flex h-[50px] shrink-0 items-center px-4 font-semibold">{t('hooks.importPaste')}</div>
            <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
              <div className="flex flex-col gap-2.5 p-4">
                <Select
                  ariaLabel={t('hooks.importFrom')}
                  value={pasteFrom}
                  onChange={(v) => { setPasteFrom(v); setPasted(null); }}
                  disabled={busy}
                  options={available.map((a) => ({ value: a, label: labelOf(a), icon: <AgentIcon target={agentOf(a)} size={16} /> }))}
                />
                <CodeEditor value={content} onChange={(v) => { setContent(v); setPasted(null); }} lang={isCodeAgent(agentOf(pasteFrom)) ? 'typescript' : 'json'} ariaLabel={t('hooks.importPaste')} placeholder={t('hooks.importPastePlaceholder')} disabled={busy} minHeight="96px" />
                <span className="flex items-center gap-2">
                  <span className="hp flex-1">{t('hooks.importPasteHint')}</span>
                  <Button size="sm" variant="secondary" loading={reading} disabled={!content.trim()} onClick={() => void read()}>{t('hooks.importRead')}</Button>
                </span>
                {pasted && pasted.length === 0 && <p className="text-[13px] text-ink-2">{t('hooks.importNone')}</p>}
              </div>
              {pasted && pasted.length > 0 && <div>{list(rows.filter((r) => !r.adopt))}</div>}
            </div>
          </section>
        ) : (
          <section aria-label={labelOf(active)} className="ss-list flex min-w-0 flex-1 flex-col">
            <div className="flex h-[50px] shrink-0 items-center gap-3 px-4">
              <Checkbox hideLabel label={t('hooks.importSelectAll', { target: labelOf(active) })} checked={ready.length > 0 && ready.every(checked)} indeterminate={ready.some(checked) && !ready.every(checked)} disabled={ready.length === 0 || busy} onChange={(on) => check(ready.map((r) => r.id), on)} />
              <span className="shrink-0 font-semibold">{labelOf(active)}</span>
              {paths[active] ? <span className="min-w-0 flex-1 truncate font-mono text-xs text-ink-3" title={paths[active]}>{shortenHome(paths[active])}</span> : <span className="flex-1" />}
              <span className="shrink-0 text-xs text-ink-2">{t('hooks.importPickedOf', { picked: String(found.filter(checked).length), total: String(found.length) })}</span>
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto">
              {reads[at]?.isPending && <div className="p-4"><Spinner size="sm" /></div>}
              {reads[at]?.error && <div className="p-4"><div className="ss-note bad"><span className="flex-1">{(reads[at].error as Error).message}</span></div></div>}
              {reads[at]?.isSuccess && found.length === 0 && <p className="p-4 text-[13px] text-ink-2">{t('hooks.importNone')}</p>}
              {list(found)}
            </div>
          </section>
        )}
      </div>
      {failure && <div className="border-t border-line px-[22px] py-3">{failure}</div>}
      {/* While a hook is being edited the footer holds the editor's actions instead. */}
      <div className="df !border-t-0 !bg-transparent" ref={setFooterHost}>
        {!row && (
          <>
            <span className="flex-1 text-[13px] text-ink-2">
              {pending > 0
                ? t('hooks.importFooterPending', { count: String(chosen.length), changes: String(pending) })
                : t(plural('hooks.importFooter', sources), { count: String(chosen.length), targets: String(sources) })}
            </span>
            <Button variant="ghost" onClick={onClose} disabled={busy}>{t('common.cancel')}</Button>
            {pending > 0 && <Button variant="ghost" onClick={() => setChanges({})} disabled={busy}>{t('hooks.importDiscard')}</Button>}
            {pending > 0
              ? <Button variant="primary" loading={saving} disabled={blocked || reading} onClick={() => void review()}>{t('hooks.importReview')}</Button>
              : <Button variant="primary" loading={saving} disabled={chosen.length === 0 || blocked || reading} onClick={() => void save()}><Download size={15} />{t('hooks.importSave', { count: String(chosen.length) })}</Button>}
          </>
        )}
      </div>
    </DialogShell>
  );
}
