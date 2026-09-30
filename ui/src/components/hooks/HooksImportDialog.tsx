import { useState } from 'react';
import { Download, TriangleAlert, X } from 'lucide-react';
import { hookAgents, hooksApi, type HookCandidate, type HookInventory } from '../../api/hooks';
import { useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import { Checkbox } from '../Checkbox';
import CodeEditor from '../CodeEditor';
import DialogShell from '../DialogShell';
import { Select } from '../Input';
import Spinner from '../Spinner';
import { boundAgents, hookLabel, hookMessage, scopeEntries, scopePaths, scopeUnmanaged } from './hooksView';

interface Props {
  data: HookInventory;
  /** The Agent selected first, such as one found to hold hooks Skillshare does not manage. */
  defaultFrom?: string;
  /** A root under hooks.projects: read that folder's Agent files and save into that project. */
  project?: string;
  onClose: () => void;
  onImported: (count: number) => void;
}

/**
 * Reads an Agent's native configuration or extension, or pasted text, and saves the chosen hooks into
 * the source. Reading never executes anything; saving does not write native files, Sync does.
 */
export default function HooksImportDialog({ data, defaultFrom, project, onClose, onImported }: Props) {
  const t = useT();
  const [from, setFrom] = useState(defaultFrom ?? scopeUnmanaged(data, project)[0]?.target ?? hookAgents[0]);
  const [content, setContent] = useState('');
  const [name, setName] = useState('');
  const [candidates, setCandidates] = useState<HookCandidate[] | null>(null);
  const [picked, setPicked] = useState<string[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const existing = scopeEntries(data, project);
  const busy = loading || saving;

  const importable = (c: HookCandidate) => c.problems.length === 0 && !(c.name in existing);
  const selected = picked ?? (candidates ?? []).filter(importable).map((c) => c.name);
  const chosen = (candidates ?? []).filter((c) => importable(c) && selected.includes(c.name));
  const source = scopePaths(data, project)[from];

  // Changing the source invalidates what was read from the old one.
  const reset = () => { setCandidates(null); setPicked(null); setError(''); };

  const read = async () => {
    setLoading(true);
    reset();
    try {
      setCandidates(await hooksApi.import({ from, ...(project && { root: project }), ...(content.trim() && { content }), ...(name.trim() && { name: name.trim() }) }));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const save = async () => {
    setSaving(true);
    setError('');
    try {
      // One at a time: each save previews and sends its own revision, so a parallel batch would be refused.
      for (const c of chosen) await hooksApi.save({ ...(project && { project }), name: c.name, entry: c.entry });
      onImported(chosen.length);
    } catch (e) {
      setError((e as Error).message);
      setSaving(false);
    }
  };

  const title = t('hooks.importTitle');
  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={busy} ariaLabel={title} className="!max-w-[720px]">
      <div className="dh">
        <div className="flex flex-col gap-1">
          <h2 className="ss-h2">{title}</h2>
          <p className="text-[13px] text-ink-2">{t('hooks.importHint')}</p>
        </div>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={busy}><X size={16} /></button>
      </div>
      <div className="db">
        <div className="ss-fld">
          <Select
            label={t('hooks.importFrom')}
            value={from}
            onChange={(v) => { setFrom(v); reset(); }}
            disabled={busy}
            options={hookAgents.map((a) => ({ value: a, label: hookLabel(a), icon: <AgentIcon target={a} size={16} />, note: scopeUnmanaged(data, project).some((u) => u.target === a) ? `· ${t('hooks.hasUnmanaged')}` : undefined }))}
          />
          {source && <span className="hp truncate font-mono" title={source}>{shortenHome(source)}</span>}
        </div>
        <div className="ss-fld">
          <label>{t('hooks.importPaste')}</label>
          <CodeEditor value={content} onChange={(v) => { setContent(v); reset(); }} ariaLabel={t('hooks.importPaste')} placeholder={t('hooks.importPastePlaceholder')} disabled={busy} minHeight="96px" />
          <span className="hp">{t('hooks.importPasteHint')}</span>
        </div>
        {content.trim() && (
          <div className="ss-fld">
            <label htmlFor="hook-import-name">{t('hooks.importName')}</label>
            <span className="ss-inp"><input id="hook-import-name" value={name} onChange={(e) => { setName(e.target.value); reset(); }} placeholder="my-hook" disabled={busy} /></span>
          </div>
        )}
        {loading && <Spinner size="sm" />}
        {candidates && candidates.length === 0 && <div className="ss-note inf"><span className="flex-1">{t('hooks.importNone')}</span></div>}
        {candidates && candidates.length > 0 && (
          <div className="ss-list !shadow-none" role="group" aria-label={t('hooks.importCandidates')}>
            {candidates.map((c) => {
              const ok = importable(c);
              const taken = c.name in existing;
              return (
                <div key={c.name} className="ss-r !items-start !py-2.5">
                  <Checkbox hideLabel label={c.name} checked={ok && selected.includes(c.name)} disabled={!ok || busy} onChange={(on) => setPicked(on ? [...selected, c.name] : selected.filter((x) => x !== c.name))} />
                  <span className="flex min-w-0 flex-1 flex-col gap-1">
                    <span className="flex items-center gap-2">
                      <span className="truncate font-mono font-semibold">{c.name}</span>
                      <span className="ss-stack">{boundAgents(c.entry).map((a) => <span key={a} className="ss-at"><AgentIcon target={a} size={13} /></span>)}</span>
                      {taken && <span className="ss-tag warn">{t('hooks.importTaken')}</span>}
                    </span>
                    {c.problems.map((p) => <span key={p} className="text-xs text-bad">{p}</span>)}
                    {c.warnings.map((w) => <span key={w} className="flex items-center gap-1.5 text-xs text-warn"><TriangleAlert size={12} />{hookMessage(t, w)}</span>)}
                  </span>
                </div>
              );
            })}
          </div>
        )}
        {error && <div className="ss-note bad" role="alert"><span className="flex-1">{error}</span></div>}
      </div>
      <div className="df">
        <span className="flex-1 text-[13px] text-ink-2">{candidates && chosen.length > 0 ? t('hooks.importSaveNote') : ''}</span>
        <Button variant="ghost" onClick={onClose} disabled={busy}>{t('common.cancel')}</Button>
        {candidates && candidates.length > 0
          ? <Button variant="primary" loading={saving} disabled={chosen.length === 0 || loading} onClick={() => void save()}>{t('hooks.importSave', { count: chosen.length })}</Button>
          : <Button variant="primary" loading={loading} onClick={() => void read()}><Download size={15} />{t('hooks.importRead')}</Button>}
      </div>
    </DialogShell>
  );
}
