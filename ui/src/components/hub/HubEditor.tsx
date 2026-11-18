import { useMemo, useState } from 'react';
import { useBeforeUnload, useBlocker } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { Plus } from 'lucide-react';
import { ApiError } from '../../api/client';
import { hubDrafts } from '../../api/hubDrafts';
import type { DraftResponse, HubDraft, HubEntry } from '../../api/hubDrafts';
import Button from '../Button';
import ConfirmDialog from '../ConfirmDialog';
import AddSkillDialog from './AddSkillDialog';
import HubEntryEditor from './HubEntryEditor';
import { newEntryId } from './hubShared';
import { queryKeys } from '../../lib/queryKeys';
import { useT, plural } from '../../i18n';
import { invalidate } from '../../lib/queryEvents';

interface Props {
  response: DraftResponse;
  onDone: () => void;
}

/** Counts what differs from the saved hub: the name, the description, and each added, removed or edited skill. */
function countChanges(saved: HubDraft, draft: HubDraft) {
  const before = new Map(saved.entries.map((e) => [e.id, JSON.stringify(e)]));
  const now = new Set(draft.entries.map((e) => e.id));
  let n = Number(saved.name !== draft.name) + Number(saved.description !== draft.description);
  for (const e of draft.entries) if (before.get(e.id) !== JSON.stringify(e)) n++;
  for (const id of before.keys()) if (!now.has(id)) n++;
  return n;
}

/** The user's own hub, edited in place where it is viewed. */
export default function HubEditor({ response, onDone }: Props) {
  const t = useT();
  const queryClient = useQueryClient();
  const [base, setBase] = useState(response);
  const [draft, setDraft] = useState(response.draft);
  const [versions, setVersions] = useState<Record<string, string>>(response.refs ?? {});
  const [open, setOpen] = useState<string[]>([]);
  const [adding, setAdding] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const savedJSON = useMemo(() => JSON.stringify(base.draft), [base]);
  const dirty = JSON.stringify(draft) !== savedJSON;
  const blocker = useBlocker(dirty || busy);
  useBeforeUnload((event) => {
    if (dirty || busy) {
      event.preventDefault();
      event.returnValue = '';
    }
  });

  // A problem the server reported only holds while its entry is unchanged.
  const savedEntries = useMemo(
    () => new Map(base.draft.entries.map((e) => [e.id, JSON.stringify(e)])),
    [base],
  );
  const savedSources = useMemo(
    () => new Map(base.draft.entries.map((e) => [e.id, e.data.source])),
    [base],
  );
  const problemsOf = (entry: HubEntry) =>
    savedEntries.get(entry.id) === JSON.stringify(entry)
      ? base.problems.filter((p) => p.entryId === entry.id)
      : [];
  const blocked = draft.entries.filter((e) => problemsOf(e).length > 0).length;
  const changes = Math.max(1, countChanges(base.draft, draft));

  const setEntries = (update: (entries: HubEntry[]) => HubEntry[]) =>
    setDraft((current) => ({ ...current, entries: update(current.entries) }));

  function changeEntry(id: string, key: string, value: unknown) {
    setEntries((entries) =>
      entries.map((entry) => {
        if (entry.id !== id) return entry;
        const data = { ...entry.data, [key]: value };
        if (key === 'source' || key === 'skill') {
          delete data.riskScore;
          delete data.riskLabel;
          delete data.auditedAt;
        }
        return { ...entry, data };
      }),
    );
    // A hand-edited source may name another version; it is read again when the dropdown opens.
    if (key === 'source')
      setVersions((v) => {
        const rest = { ...v };
        delete rest[id];
        return rest;
      });
  }

  function addEntries(added: HubEntry['data'][]) {
    setEntries((entries) => [...entries, ...added.map((data) => ({ id: newEntryId(), data }))]);
    setAdding(false);
  }

  async function save() {
    setBusy(true);
    setError('');
    try {
      const res = await hubDrafts.save(draft);
      queryClient.setQueryData(queryKeys.hub.draft(res.draft.id), res);
      void invalidate(queryClient, 'hubDraftsChanged');
      setBase(res);
      setDraft(res.draft);
      setVersions(res.refs ?? {});
      // Stay while something still needs fixing, so the red rows say what.
      if (!res.problems.some((p) => p.entryId)) onDone();
    } catch (err) {
      setError(
        err instanceof ApiError && err.status === 409
          ? t('hubs.conflict')
          : err instanceof Error
            ? err.message
            : String(err),
      );
    } finally {
      setBusy(false);
    }
  }

  function cancel() {
    // Reload what is saved, which also recovers from an edit made in another window.
    void invalidate(queryClient, 'hubDraftReverted', draft.id);
    onDone();
  }

  return (
    <div className="flex min-w-0 flex-col gap-3.5">
      <div className="ss-sec !mb-0 !items-center">
        <h2>{t('hubs.edit.title', { name: draft.name || t('hubBuilder.untitled') })}</h2>
        <span className="ml-auto flex items-center gap-2">
          <Button variant="secondary" onClick={() => setAdding(true)} disabled={busy}>
            <Plus size={14} />
            {t('hubs.edit.addSkill')}
          </Button>
          {!dirty && <Button onClick={onDone}>{t('hubs.edit.done')}</Button>}
        </span>
      </div>

      {error && (
        <div role="alert" className="ss-note bad">
          <span className="flex-1 break-words">{error}</span>
        </div>
      )}

      <fieldset disabled={busy} className="flex min-w-0 flex-col gap-3.5">
        <div className="ss-box grid grid-cols-2 gap-3.5 !py-4">
          <div className="ss-fld">
            <label htmlFor="hub-name">{t('hubs.edit.name')}</label>
            <span className="ss-inp">
              <input
                id="hub-name"
                value={draft.name}
                onChange={(e) => setDraft({ ...draft, name: e.target.value })}
              />
            </span>
          </div>
          <div className="ss-fld">
            <label htmlFor="hub-description">{t('hubs.edit.description')}</label>
            <span className="ss-inp">
              <input
                id="hub-description"
                value={draft.description}
                onChange={(e) => setDraft({ ...draft, description: e.target.value })}
              />
            </span>
          </div>
        </div>

        {draft.entries.length === 0 ? (
          <div className="ss-empty">
            <p>{t('hubBuilder.noEntries')}</p>
          </div>
        ) : (
          <div className="ss-list">
            <div className="ss-lh !grid grid-cols-[180px_minmax(0,1fr)_150px_30px_30px] gap-2.5">
              <span>{t('hubs.edit.name')}</span>
              <span>{t('hubs.edit.source')}</span>
              <span>{t('hubs.view.version')}</span>
            </div>
            {draft.entries.map((entry) => (
              <HubEntryEditor
                key={entry.id}
                entry={entry}
                problems={problemsOf(entry)}
                version={
                  versions[entry.id] ??
                  (savedSources.get(entry.id) === entry.data.source
                    ? (base.refs?.[entry.id] ?? '')
                    : undefined)
                }
                expanded={open.includes(entry.id)}
                onToggle={() =>
                  setOpen((list) =>
                    list.includes(entry.id)
                      ? list.filter((id) => id !== entry.id)
                      : [...list, entry.id],
                  )
                }
                onChange={(key, value) => changeEntry(entry.id, key, value)}
                onVersion={(source, version) => {
                  changeEntry(entry.id, 'source', source);
                  setVersions((v) => ({ ...v, [entry.id]: version }));
                }}
                onRemove={() =>
                  setEntries((entries) => entries.filter((item) => item.id !== entry.id))
                }
              />
            ))}
          </div>
        )}
      </fieldset>

      {dirty && (
        <div className="ss-box sticky bottom-4 z-10 flex items-center gap-2.5 !py-3 shadow-[var(--sh-float)]">
          <span className="h-2 w-2 shrink-0 rounded-full bg-warn" />
          <span className="flex-1 text-[13px] text-ink-2">
            {t(plural('hubs.edit.changes', changes), { count: changes })}
            {blocked > 0 && ` · ${t(plural('hubs.edit.blocked', blocked), { count: blocked })}`}
          </span>
          <Button variant="secondary" onClick={cancel} disabled={busy}>
            {t('common.cancel')}
          </Button>
          <Button onClick={() => void save()} loading={busy}>
            {t('common.save')}
          </Button>
        </div>
      )}

      {adding && (
        <AddSkillDialog
          hubName={draft.name || t('hubBuilder.untitled')}
          entries={draft.entries}
          onAdd={addEntries}
          onManual={() => addEntries([{ name: '', source: '' }])}
          onClose={() => setAdding(false)}
        />
      )}
      <ConfirmDialog
        open={blocker.state === 'blocked'}
        title={t('hubBuilder.leaveTitle')}
        message={t('hubBuilder.leaveHint')}
        loading={busy}
        onCancel={() => blocker.state === 'blocked' && blocker.reset()}
        onConfirm={() => blocker.state === 'blocked' && blocker.proceed()}
      />
    </div>
  );
}
