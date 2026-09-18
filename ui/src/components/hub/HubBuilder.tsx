import { useEffect, useRef, useState } from 'react';
import { Link, useBeforeUnload, useBlocker } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, CheckCircle2, Download, Eye, FilePlus, Plus, Upload } from 'lucide-react';
import { api, ApiError, type HubSavedEntry } from '../../api/client';
import { hubAddCommand, hubDrafts, type DraftResponse, type HubDraft, type HubEntry, type HubProblem } from '../../api/hubDrafts';
import Button from '../Button';
import ConfirmDialog from '../ConfirmDialog';
import EmptyState from '../EmptyState';
import HubEntryEditor from './HubEntryEditor';
import InstalledSkillPicker from './InstalledSkillPicker';
import { queryKeys } from '../../lib/queryKeys';
import { useT } from '../../i18n';

type HubAction = { type: 'open'; id: string } | { type: 'import'; file: File } | { type: 'create' | 'save' | 'delete' | 'download' | 'copy' | 'chooseFile' | 'subscribe' };

export default function HubBuilder() {
  const t = useT();
  const queryClient = useQueryClient();
  const [drafts, setDrafts] = useState<HubDraft[]>([]);
  const [draft, setDraft] = useState<HubDraft | null>(null);
  const [saved, setSaved] = useState('');
  const [problems, setProblems] = useState<HubProblem[]>([]);
  const [candidates, setCandidates] = useState<HubEntry[]>([]);
  const [picker, setPicker] = useState(false);
  const [query, setQuery] = useState('');
  const [open, setOpen] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [confirm, setConfirm] = useState<{ kind: 'delete' | 'leave'; action: HubAction } | null>(null);
  const [busy, setBusy] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);
  const dirty = draft !== null && JSON.stringify(draft) !== saved;
  // Where the index is hosted belongs to the draft, so a reopened hub remembers it.
  const location = String(draft?.fields?.publishUrl ?? '');
  const setLocation = (value: string) =>
    setDraft(current => current && ({ ...current, fields: { ...current.fields, publishUrl: value } }));
  const blocker = useBlocker(dirty || busy);
  useBeforeUnload((event) => { if (dirty || busy) { event.preventDefault(); event.returnValue = ''; } });

  useEffect(() => {
    let active = true;
    Promise.all([hubDrafts.list(), hubDrafts.candidates()]).then(([items, skills]) => {
      if (active) { setDrafts(items); setCandidates(skills); }
    }).catch((err: Error) => { if (active) setError(err.message); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, []);

  function accept(response: DraftResponse) {
    setDraft(response.draft); setSaved(JSON.stringify(response.draft)); setProblems(response.problems);
    setDrafts(items => [response.draft, ...items.filter(item => item.id !== response.draft.id)]);
    setQuery(''); setOpen([]);
  }
  function requestAction(action: HubAction) {
    if (action.type === 'delete') setConfirm({ kind: 'delete', action });
    else if (dirty && ['open', 'create', 'chooseFile'].includes(action.type)) setConfirm({ kind: 'leave', action });
    else void performAction(action);
  }
  async function performAction(action: HubAction) {
    if (action.type === 'chooseFile') { fileInput.current?.click(); return; }
    setBusy(true); setError(''); setNotice('');
    try {
      switch (action.type) {
        case 'open': accept(await hubDrafts.get(action.id)); break;
        case 'create':
          accept(await hubDrafts.create({ name: t('hubBuilder.untitled'), description: '', entries: [], fields: {} })); break;
        case 'import':
          if (action.file.size > 4 * 1024 * 1024) throw new Error(t('hubBuilder.fileTooLarge'));
          accept(await hubDrafts.import(await action.file.text())); break;
        case 'save':
          if (draft) { accept(await hubDrafts.save(draft)); setNotice(t('hubBuilder.saved')); } break;
        case 'delete':
          if (draft) {
            await hubDrafts.remove(draft); setDrafts(items => items.filter(item => item.id !== draft.id));
            setDraft(null); setSaved('');
          } break;
        case 'download':
          if (draft) downloadIndex(await hubDrafts.export(draft)); break;
        case 'copy':
          if (draft) { await navigator.clipboard.writeText(hubAddCommand(location, draft.name)); setNotice(t('hubBuilder.copied')); } break;
        case 'subscribe':
          if (draft) {
            // Same config the CLI and the Browse tab read, so it shows up in both at once.
            const config = await api.getHubConfig();
            const url = location.trim();
            const next: HubSavedEntry[] = [...config.hubs.filter(h => h.url !== url), { label: draft.name, url }];
            await api.putHubConfig({ hubs: next, default: config.default });
            queryClient.setQueryData(queryKeys.hubConfig, { hubs: next, default: config.default });
            setNotice(t('hubBuilder.subscribed'));
          } break;
      }
    } catch (err) {
      setError(err instanceof ApiError && err.status === 409 ? t('hubBuilder.conflict') : err instanceof Error ? err.message : String(err));
    } finally { setBusy(false); }
  }
  function changeEntry(id: string, key: string, value: unknown) {
    setDraft(current => current && ({ ...current, entries: current.entries.map(entry => {
      if (entry.id !== id) return entry;
      const data = { ...entry.data, [key]: value };
      if (key === 'source' || key === 'skill') {
        delete data.riskScore; delete data.riskLabel; delete data.auditedAt;
      }
      return { ...entry, data };
    }) }));
  }
  function addEntries(entries: HubEntry[]) {
    const added = entries.map(entry => ({ ...entry, id: Array.from(crypto.getRandomValues(new Uint8Array(16)), byte => byte.toString(16).padStart(2, '0')).join('') }));
    setDraft(current => current && ({ ...current, entries: [...current.entries, ...added] }));
    setPicker(false); setQuery('');
    // A hand-added entry has nothing in it yet, so open it straight away.
    if (added.length === 1 && !added[0].data.source) setOpen(list => [...list, added[0].id]);
  }

  const shown = draft?.entries.filter(entry => `${entry.data.name ?? ''} ${entry.data.source ?? ''} ${(entry.data.tags ?? []).join(' ')}`.toLowerCase().includes(query.toLowerCase())) ?? [];
  const publishLocation = /^(https?:\/\/|ssh:\/\/|[^\s@]+@[^\s:]+:)/.test(location.trim()) && !Array.from(location).some(char => char.charCodeAt(0) < 32 || char.charCodeAt(0) === 127);
  const blocked = problems.length > 0;

  const fileField = (
    <input ref={fileInput} type="file" accept=".json,application/json" className="hidden" aria-label={t('hubBuilder.import')} onChange={event => {
      const file = event.target.files?.[0]; event.target.value = ''; if (file) requestAction({ type: 'import', file });
    }} />
  );

  if (loading) return <p role="status" className="text-[13px] text-ink-2">{t('hubBuilder.loading')}</p>;

  return (
    <div className="grid grid-cols-[220px_minmax(0,1fr)] items-start gap-6">
      {fileField}

      <aside aria-label={t('hubBuilder.drafts')} className="flex flex-col gap-3">
        <div className="ss-sec !mb-0">
          <h2>{t('hubBuilder.drafts')}</h2>
          <span className="ss-cnt">{drafts.length}</span>
        </div>
        {drafts.length === 0 ? (
          <p className="text-[13px] text-ink-2">{t('hubBuilder.emptyDrafts')}</p>
        ) : (
          <div className="ss-list">
            {drafts.map(item => {
              const published = String(item.fields?.publishUrl ?? '').trim();
              return (
                <div key={item.id} className={`ss-r !min-h-[52px] ${draft?.id === item.id ? 'sel' : ''}`}>
                  <button type="button" disabled={busy} aria-pressed={draft?.id === item.id}
                    className="flex min-w-0 w-full flex-1 flex-col items-start gap-0.5 text-start"
                    onClick={() => requestAction({ type: 'open', id: item.id })}>
                    <span className="nm m break-words">{item.name || t('hubBuilder.untitled')}</span>
                    <span className="text-xs text-ink-3">
                      {t('hubBuilder.count', { count: item.entries.length })}
                      {published && ` \u00b7 ${t('hubBuilder.published')}`}
                    </span>
                    {published && <span className="w-full truncate font-mono text-[11px] text-ink-3">{published}</span>}
                  </button>
                </div>
              );
            })}
          </div>
        )}
        <div className="flex flex-col gap-2">
          <Button size="sm" disabled={busy} onClick={() => requestAction({ type: 'create' })}><Plus size={14} />{t('hubBuilder.create')}</Button>
          <Button size="sm" variant="secondary" disabled={busy} onClick={() => requestAction({ type: 'chooseFile' })}><Upload size={14} />{t('hubBuilder.import')}</Button>
        </div>
      </aside>

      {!draft ? (
        <EmptyState icon={FilePlus} title={t('hubBuilder.startTitle')} description={t('hubBuilder.start')}
          action={<Button onClick={() => requestAction({ type: 'create' })} disabled={busy}><Plus size={15} />{t('hubBuilder.create')}</Button>} />
      ) : (
        <div className="flex min-w-0 flex-col gap-[18px]">
          {error && <div role="alert" className="ss-note bad"><span className="flex-1 break-words">{error}</span></div>}
          {notice && <p role="status" className="ss-note"><span className="flex-1">{notice}</span></p>}

          <div className={`ss-note ${blocked ? 'bad' : dirty ? 'warn' : 'ok'}`}>
            {blocked ? <AlertTriangle size={16} className="mt-0.5 shrink-0" /> : <CheckCircle2 size={16} className="mt-0.5 shrink-0" />}
            <span className="flex-1">{blocked ? t('hubBuilder.blocked', { count: problems.length }) : dirty ? t('hubBuilder.unsaved') : t('hubBuilder.ready')}</span>
          </div>

          <fieldset disabled={busy} className="flex min-w-0 flex-col gap-[18px]">
            <div className="ss-box flex flex-col gap-3">
              <div className="grid grid-cols-2 gap-3.5">
                <div className="ss-fld">
                  <label htmlFor="hub-name">{t('hubBuilder.name')}</label>
                  <span className="ss-inp"><input id="hub-name" value={draft.name} onChange={event => setDraft({ ...draft, name: event.target.value })} /></span>
                </div>
                <div className="ss-fld">
                  <label htmlFor="hub-description">{t('hubBuilder.description')}</label>
                  <span className="ss-inp"><input id="hub-description" value={draft.description} onChange={event => setDraft({ ...draft, description: event.target.value })} /></span>
                </div>
              </div>
              <span className="hp">{t('hubBuilder.metadataHint')}</span>
            </div>

            <div className="flex flex-col gap-3">
              <div className="ss-sec !mb-0">
                <h2>{t('hubBuilder.skills')}</h2>
                <span className="ss-cnt">{draft.entries.length}</span>
                <span className="ml-auto flex items-center gap-3">
                  {draft.entries.length > 0 && (
                    <span className="ss-inp !h-8 w-[200px]">
                      <input value={query} onChange={event => setQuery(event.target.value)} placeholder={t('hubBuilder.filter')} aria-label={t('hubBuilder.filter')} />
                    </span>
                  )}
                  <button type="button" className="ss-btn sm" onClick={() => setPicker(true)}><Plus size={14} />{t('hubBuilder.fromInstalled')}</button>
                  <button type="button" className="ss-btn sm ghost" onClick={() => addEntries([{ id: '', data: { name: '', source: '' } }])}>{t('hubBuilder.manual')}</button>
                </span>
              </div>

              {draft.entries.length === 0 ? (
                <div className="ss-empty"><p>{t('hubBuilder.noEntries')}</p></div>
              ) : (
                <div className="ss-list">
                  {shown.map(entry => (
                    <HubEntryEditor key={entry.id} entry={entry} showProblems={!dirty}
                      problems={problems.filter(problem => problem.entryId === entry.id)}
                      expanded={open.includes(entry.id)}
                      onToggle={() => setOpen(list => list.includes(entry.id) ? list.filter(id => id !== entry.id) : [...list, entry.id])}
                      onChange={(key, value) => changeEntry(entry.id, key, value)}
                      onRemove={() => setDraft({ ...draft, entries: draft.entries.filter(item => item.id !== entry.id) })} />
                  ))}
                </div>
              )}
            </div>
          </fieldset>

          <div className="ss-box flex flex-col gap-3">
            <span className="hp">{t('hubBuilder.accessHint')}</span>
            <div className="flex flex-wrap items-center gap-2.5">
              <Button disabled={!dirty || busy} loading={busy} onClick={() => requestAction({ type: 'save' })}>{t('hubBuilder.save')}</Button>
              <Button variant="secondary" disabled={dirty || blocked || busy} onClick={() => requestAction({ type: 'download' })}><Download size={15} />{t('hubBuilder.download')}</Button>
              <Link to={`/hubs?preview=draft:${encodeURIComponent(draft.id)}`} className="ss-btn"><Eye size={15} />{t('hubBuilder.previewInBrowse')}</Link>
              <Button variant="ghost" disabled={busy} onClick={() => requestAction({ type: 'open', id: draft.id })}>{t('hubBuilder.reload')}</Button>
              <Button variant="ghost" disabled={busy} onClick={() => requestAction({ type: 'delete' })}>{t('hubBuilder.delete')}</Button>
            </div>
          </div>

          <div className="ss-box flex flex-col gap-3">
            <div className="flex flex-col gap-1">
              <h2 className="ss-h2">{t('hubBuilder.shareTitle')}</h2>
              <span className="hp">{t('hubBuilder.shareHint')}</span>
            </div>
            <div className="ss-fld">
              <label htmlFor="hub-location">{t('hubBuilder.hostLocation')}</label>
              <span className="ss-inp font-mono"><input id="hub-location" placeholder="https://example.com/skillshare-hub.json" value={location} onChange={event => setLocation(event.target.value)} /></span>
            </div>
            {publishLocation && (
              <>
                <div className="ss-code !p-3 font-mono text-[12.5px] break-all">{hubAddCommand(location, draft.name)}</div>
                <div className="flex flex-wrap items-center gap-2.5">
                  <Button variant="secondary" disabled={busy} onClick={() => requestAction({ type: 'copy' })}>{t('hubBuilder.copyCommand')}</Button>
                  <Button variant="ghost" disabled={busy} onClick={() => requestAction({ type: 'subscribe' })}>{t('hubBuilder.subscribe')}</Button>
                </div>
              </>
            )}
          </div>
        </div>
      )}

      {picker && <InstalledSkillPicker candidates={candidates} onClose={() => setPicker(false)} onAdd={addEntries} />}
      <ConfirmDialog open={confirm !== null} title={t(confirm?.kind === 'delete' ? 'hubBuilder.delete' : 'hubBuilder.leaveTitle')}
        message={t(confirm?.kind === 'delete' ? 'hubBuilder.deleteHint' : 'hubBuilder.leaveHint')} variant={confirm?.kind === 'delete' ? 'danger' : 'default'}
        onCancel={() => setConfirm(null)} onConfirm={() => { const action = confirm?.action; setConfirm(null); if (action) void performAction(action); }} />
      <ConfirmDialog open={blocker.state === 'blocked'} title={t('hubBuilder.leaveTitle')} message={t('hubBuilder.leaveHint')}
        loading={busy} onCancel={() => blocker.state === 'blocked' && blocker.reset()} onConfirm={() => blocker.state === 'blocked' && blocker.proceed()} />
    </div>
  );
}

function downloadIndex(data: Record<string, unknown>) {
  const url = URL.createObjectURL(new Blob([JSON.stringify(data, null, 2) + '\n'], { type: 'application/json' }));
  const anchor = document.createElement('a'); anchor.href = url; anchor.download = 'skillshare-hub.json';
  document.body.append(anchor); anchor.click(); anchor.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
