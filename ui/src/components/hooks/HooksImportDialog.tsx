import { useState } from 'react';
import { useQueries } from '@tanstack/react-query';
import { Download, Plus, TriangleAlert, X } from 'lucide-react';
import { hookAgents, hooksApi } from '../../api/hooks';
import type { HookCandidate, HookInventory } from '../../api/hooks';
import { useT } from '../../i18n';
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
import { HOOK_NAME, hookLabel, hookMessage, isCodeAgent, scopeEntries, scopePaths, scopeUnmanaged } from './hooksView';

interface Props {
  data: HookInventory;
  /** A root under hooks.projects: read that folder's target files and save into that project. */
  project?: string;
  onClose: () => void;
  onImported: (count: number) => void;
}

/** A candidate as the dialog shows it: where it was read from and whether saving takes over its native registrations. */
interface Row { id: string; target: string; candidate: HookCandidate; adopt: boolean }

// Said once in the dialog's subtitle rather than on every row.
const ADOPT_WARNING = 'saving the import takes over the existing registrations in place; sync leaves them as they are instead of adding duplicates';

/**
 * Reads the hooks each target has that Skillshare does not manage yet, or pasted text, and saves the
 * checked ones into the source, one hook each. Reading never executes anything; a read candidate is
 * saved as the owner of the registrations it came from, so sync does not write them twice.
 */
export default function HooksImportDialog({ data, project, onClose, onImported }: Props) {
  const t = useT();
  const existing = scopeEntries(data, project);
  const paths = scopePaths(data, project);
  const unmanaged = scopeUnmanaged(data, project);
  const targets = hookAgents.filter((a) => unmanaged.some((u) => u.target === a));
  const reads = useQueries({
    queries: targets.map((from) => ({
      queryKey: [...queryKeys.hooks, 'import', project ?? '', from],
      queryFn: () => hooksApi.import({ from, ...(project && { root: project }) }),
      gcTime: 0,
      retry: false,
    })),
  });
  const [pasting, setPasting] = useState(targets.length === 0);
  const [pasteFrom, setPasteFrom] = useState<string>(hookAgents[0]);
  const [content, setContent] = useState('');
  const [pasted, setPasted] = useState<HookCandidate[] | null>(null);
  const [reading, setReading] = useState(false);
  const [names, setNames] = useState<Record<string, string>>({});
  const [unchecked, setUnchecked] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const busy = saving || reading;

  const rows: Row[] = [
    ...targets.flatMap((target, i) => (reads[i].data ?? []).map((candidate) => ({ id: `${target}:${candidate.name}`, target, candidate, adopt: true }))),
    ...(pasted ?? []).map((candidate) => ({ id: `paste:${candidate.name}`, target: pasteFrom, candidate, adopt: false })),
  ];
  // Prefilled names never repeat a saved hook or each other.
  const defaults: Record<string, string> = {};
  for (const row of rows) {
    const line = bindingLines(row.candidate.entry.bindings[row.target])[0];
    const taken = (n: string) => n in existing || Object.values(defaults).includes(n);
    defaults[row.id] = suggestName(row.candidate.name, isCodeAgent(row.target) ? '' : line?.command ?? '', taken);
  }
  const nameOf = (row: Row) => names[row.id] ?? defaults[row.id];
  const nameError = (row: Row) => {
    const name = nameOf(row).trim();
    if (!HOOK_NAME.test(name)) return t('hooks.nameHint');
    if (name in existing || rows.some((r) => r.id !== row.id && checked(r) && nameOf(r).trim() === name)) return t('mcp.nameTaken');
    return '';
  };
  const importable = (row: Row) => row.candidate.problems.length === 0;
  const checked = (row: Row) => importable(row) && !unchecked.includes(row.id);
  const chosen = rows.filter(checked);
  const blocked = chosen.some((r) => nameError(r));

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

  const save = async () => {
    setSaving(true);
    setError('');
    try {
      // One at a time: each save previews and sends its own revision, so a parallel batch would be refused.
      for (const row of chosen) {
        await hooksApi.save({ ...(project && { project }), name: nameOf(row).trim(), entry: row.candidate.entry, ...(row.adopt && { adopt: true }) });
      }
      onImported(chosen.length);
    } catch (e) {
      setError((e as Error).message);
      setSaving(false);
    }
  };

  const group = (key: string, target: string, path: string | undefined, list: Row[]) => (
    <section key={key} aria-label={hookLabel(target)} className="overflow-hidden rounded-[12px] border border-line">
      <div className="flex min-h-10 items-center gap-2 bg-sunken px-3.5">
        <AgentIcon target={target} size={16} />
        <span className="font-semibold">{hookLabel(target)}</span>
        {path && <span className="min-w-0 truncate font-mono text-xs text-ink-3" title={path}>{shortenHome(path)}</span>}
      </div>
      {list.map((row) => {
        const lines = bindingLines(row.candidate.entry.bindings[row.target]);
        const notes = [...row.candidate.problems, ...row.candidate.warnings.filter((w) => w !== ADOPT_WARNING).map((w) => hookMessage(t, w))];
        const error = checked(row) ? nameError(row) : '';
        return (
          <div key={row.id} className="flex items-start gap-3 border-t border-line px-3.5 py-3">
            <span className="pt-1"><Checkbox hideLabel label={row.candidate.name} checked={checked(row)} disabled={!importable(row) || busy} onChange={(on) => setUnchecked((prev) => (on ? prev.filter((x) => x !== row.id) : [...prev, row.id]))} /></span>
            <span className="flex min-w-0 flex-1 flex-col gap-1 pt-1">
              {lines.length > 0 ? lines.map((l, i) => (
                <span key={i} className="flex min-w-0 items-baseline gap-2 font-mono text-[13px]">
                  <span className="w-[150px] shrink-0 truncate">{l.event}{l.matcher && ` · ${l.matcher}`}</span>
                  <span className="shrink-0 text-ink-3">→</span>
                  <span className="min-w-0 truncate" title={l.command}>{l.command}</span>
                </span>
              )) : <span className="font-mono text-[13px]">{row.candidate.name}</span>}
              {notes.map((n) => <span key={n} className={`text-xs leading-normal ${row.candidate.problems.includes(n) ? 'text-bad' : 'text-ink-2'}`}>{n}</span>)}
            </span>
            <span className="flex shrink-0 flex-col items-end gap-1">
              <span className="flex items-center gap-2">
                <span className="text-xs text-ink-2">{t('hooks.name')}</span>
                <span className={`ss-inp w-[170px] font-mono ${error ? 'err' : ''}`}>
                  <input value={nameOf(row)} onChange={(e) => setNames((prev) => ({ ...prev, [row.id]: e.target.value }))} aria-label={t('hooks.importNameFor', { name: row.candidate.name })} disabled={!checked(row) || busy} />
                </span>
              </span>
              {error && <span className="text-xs text-bad">{error}</span>}
            </span>
          </div>
        );
      })}
    </section>
  );

  const loading = reads.some((r) => r.isPending);
  const failed = reads.map((r, i) => r.error && `${hookLabel(targets[i])}: ${(r.error as Error).message}`).filter(Boolean);
  const found = rows.filter((r) => r.adopt);
  const title = t('hooks.importTitle');
  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={busy} ariaLabel={title} className="!max-w-[880px]">
      <div className="dh">
        <div className="flex flex-col gap-1">
          <h2 className="ss-h2">{title}</h2>
          <p className="text-[13px] text-ink-2">{t('hooks.importHint')}</p>
        </div>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={busy}><X size={16} /></button>
      </div>
      <div className="db">
        {loading && <Spinner size="sm" />}
        {failed.map((f) => <div key={f as string} className="ss-note bad"><span className="flex-1">{f}</span></div>)}
        {targets.map((target) => {
          const list = found.filter((r) => r.target === target);
          return list.length > 0 && group(target, target, paths[target], list);
        })}
        {!loading && targets.length > 0 && found.length === 0 && failed.length === 0 && <p className="text-[13px] text-ink-2">{t('hooks.importNone')}</p>}
        {pasted && pasted.length > 0 && group('paste', pasteFrom, undefined, rows.filter((r) => !r.adopt))}
        {pasted && pasted.length === 0 && <p className="text-[13px] text-ink-2">{t('hooks.importNone')}</p>}
        {pasting ? (
          <div className="flex flex-col gap-2.5 rounded-[12px] border border-dashed border-line-2 p-3.5">
            <span className="text-[13px] font-semibold">{t('hooks.importPaste')}</span>
            <Select
              ariaLabel={t('hooks.importFrom')}
              value={pasteFrom}
              onChange={(v) => { setPasteFrom(v); setPasted(null); }}
              disabled={busy}
              options={hookAgents.map((a) => ({ value: a, label: hookLabel(a), icon: <AgentIcon target={a} size={16} /> }))}
            />
            <CodeEditor value={content} onChange={(v) => { setContent(v); setPasted(null); }} lang={isCodeAgent(pasteFrom) ? 'typescript' : 'json'} ariaLabel={t('hooks.importPaste')} placeholder={t('hooks.importPastePlaceholder')} disabled={busy} minHeight="96px" />
            <span className="flex items-center gap-2">
              <span className="hp flex-1">{t('hooks.importPasteHint')}</span>
              <Button size="sm" variant="secondary" loading={reading} disabled={!content.trim()} onClick={() => void read()}>{t('hooks.importRead')}</Button>
            </span>
          </div>
        ) : (
          <button type="button" className="flex w-fit items-center gap-[7px] text-[13px] text-ink-2 hover:text-ink" onClick={() => setPasting(true)}><Plus size={14} />{t('hooks.importPasteOpen')}</button>
        )}
        {error && <div className="ss-note bad" role="alert"><TriangleAlert size={16} /><span className="flex-1">{error}</span></div>}
      </div>
      <div className="df">
        <span className="flex-1 text-[13px] text-ink-2">{t('hooks.importSelected', { count: String(chosen.length) })}</span>
        <Button variant="ghost" onClick={onClose} disabled={busy}>{t('common.cancel')}</Button>
        <Button variant="primary" loading={saving} disabled={chosen.length === 0 || blocked || reading} onClick={() => void save()}><Download size={15} />{t('hooks.importSave', { count: String(chosen.length) })}</Button>
      </div>
    </DialogShell>
  );
}

