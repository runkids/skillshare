import { useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Check, Copy, Plus, RefreshCw, X } from 'lucide-react';
import { hookAgents, hooksApi } from '../../api/hooks';
import type { HookEntry, HookMutation, HookPreview, HookUnmanaged } from '../../api/hooks';
import { useT } from '../../i18n';
import { queryKeys } from '../../lib/queryKeys';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import { Checkbox } from '../Input';
import DialogShell from '../DialogShell';
import SegmentedControl from '../SegmentedControl';
import { useSaveShortcut } from '../instructions/useSaveShortcut';
import HookBindingEditor from './HookBindingEditor';
import HooksPreview from './HooksPreview';
import {
  HOOK_NAME, bindingInvalid, bindingToDraft, boundAgents, checkBinding, draftToBinding, emptyBinding, agentOfKey, keyLabel, isCodeAgent, rootPlan, writes,
} from './hooksView';
import type { BindingDraft } from './hooksView';
import { catalogEvent, codeTemplate, copyDraft, draftEmpty, draftRows, eventDescription } from './hookCatalog';
import type { HookCatalog } from './hookCatalog';

interface Props {
  initial?: { name: string; entry: HookEntry };
  existingNames: string[];
  /** A root under hooks.projects: the hook is saved there instead of in the global source. */
  project?: string;
  /** Targets Skillshare can manage hooks for; defaults to all of them. */
  agents?: readonly string[];
  accounts?: Record<string, string>;
  /** Native hooks Skillshare does not manage, marked as untouched in the preview. */
  unmanaged?: HookUnmanaged[];
  onClose: () => void;
  /** `synced` when the save also wrote the target files. */
  onSaved: (synced: boolean) => void;
}

const catalogQuery = { queryKey: [...queryKeys.hooks, 'catalog'], queryFn: () => hooksApi.catalog(), staleTime: Infinity };

/** The filled command target a new target can copy from: the first one, in display order. */
const copySource = (order: string[], agent: string, draftOf: (a: string) => BindingDraft, agentOf: (key: string) => string) =>
  isCodeAgent(agentOf(agent)) || agent === 'git' ? undefined : order.find((a) => a !== agent && a !== 'git' && !isCodeAgent(agentOf(a)) && !draftEmpty(agentOf(a), draftOf(a)) && (draftRows(draftOf(a))?.length ?? 0) > 0);

/** Add or edit one source hook. Saving only changes the source; Sync writes the native files. */
export default function HookDialog({ initial, existingNames, project, agents = hookAgents, accounts: configuredAccounts = {}, unmanaged = [], onClose, onSaved }: Props) {
  const t = useT();
  const accounts = project ? {} : configuredAccounts;
  const agentOf = (key: string) => agentOfKey(accounts, key);
  const labelOf = (key: string) => keyLabel(accounts, key);
  const picker = agents.flatMap((agent) => [agent, ...Object.keys(accounts).filter((key) => accounts[key] === agent).sort()]);
  const editing = Boolean(initial);
  const { data: catalog } = useQuery<HookCatalog>(catalogQuery);
  const [name, setName] = useState(initial?.name ?? '');
  const [description, setDescription] = useState(initial?.entry.description ?? '');
  const [selected, setSelected] = useState<string[]>(() => (initial ? boundAgents(initial.entry, accounts) : []));
  // A drafted target keeps its content when it is unticked and ticked again.
  const [drafts, setDrafts] = useState<Record<string, BindingDraft>>(() => Object.fromEntries((initial ? boundAgents(initial.entry, accounts) : []).map((a) => [a, bindingToDraft(agentOf(a), initial?.entry.bindings[a] ?? initial?.entry.bindings.factory)])));
  // Targets whose empty tab was opened with "start from scratch", so the form shows instead of the offer.
  const [started, setStarted] = useState<string[]>([]);
  const [active, setActive] = useState(selected[0] ?? '');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [preview, setPreview] = useState<{ key: string; plan: HookPreview } | null>(null);
  const [showPreview, setShowPreview] = useState(false);
  const [confirmClose, setConfirmClose] = useState(false);
  // Off unless asked for: taking over a target's conflicting native hooks is never implied by an edit.
  const [takeover, setTakeover] = useState(false);

  const trimmed = name.trim();
  const nameError = trimmed && !HOOK_NAME.test(trimmed) ? t('hooks.nameHint') : !editing && existingNames.includes(trimmed) ? t('mcp.nameTaken') : '';
  const order = picker.filter((a) => selected.includes(a));
  const draftOf = (agent: string) => drafts[agent] ?? emptyBinding(agentOf(agent));
  const checks = Object.fromEntries(order.map((a) => [a, checkBinding(agentOf(a), draftOf(a))]));
  const unfilled = order.filter((a) => draftEmpty(agentOf(a), draftOf(a)));
  const invalid = order.filter((a) => bindingInvalid(checks[a]));
  const valid = Boolean(trimmed) && !nameError && invalid.length === 0;
  const canSave = valid && !saving;

  const entry = (): HookEntry => ({
    ...(description.trim() && { description: description.trim() }),
    ...(initial?.entry.enabled === false && { enabled: false }),
    bindings: Object.fromEntries(order.map((a) => [a, draftToBinding(agentOf(a), draftOf(a))])),
  });
  const mutation = (): HookMutation => ({ ...(project && { project }), name: trimmed, entry: entry(), ...(takeover && { replace: true }) });
  // A preview is only good for the exact hook it was taken of; any edit makes it stale. Being busy is not an edit.
  const key = valid ? JSON.stringify(mutation()) : '';
  const fresh = preview !== null && preview.key === key;
  const shown = preview && rootPlan(preview.plan, project);

  // What the dialog holds, to tell an edit from an untouched dialog when closing.
  const snapshot = JSON.stringify([trimmed, description.trim(), order, order.map((a) => draftToBinding(agentOf(a), draftOf(a)))]);
  const [initialSnapshot] = useState(snapshot);
  const dirty = snapshot !== initialSnapshot;
  const requestClose = () => {
    if (saving) return;
    if (dirty && !confirmClose) setConfirmClose(true);
    else onClose();
  };

  const toggle = (agent: string) => {
    if (selected.includes(agent)) {
      setSelected(selected.filter((x) => x !== agent));
      if (active === agent) setActive(order.find((x) => x !== agent) ?? '');
    } else {
      setSelected([...selected, agent]);
      setActive(agent);
    }
  };
  const setDraft = (agent: string, d: BindingDraft) => setDrafts((prev) => ({ ...prev, [agent]: d }));
  const copyFrom = (from: string, to: string) => {
    setDraft(to, copyDraft(catalog, agentOf(from), agentOf(to), draftOf(from)).draft);
    setStarted((prev) => [...prev, to]);
  };
  const startFresh = (agent: string) => {
    if (isCodeAgent(agentOf(agent))) setDraft(agent, { ...draftOf(agent), code: codeTemplate(agentOf(agent)) });
    setStarted((prev) => [...prev, agent]);
  };

  const openPreview = async () => {
    if (!canSave) return;
    setSaving(true);
    setError('');
    const at = key;
    try {
      setPreview({ key: at, plan: await hooksApi.preview(mutation()) });
      setShowPreview(true);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  };

  const save = async (sync: boolean) => {
    if (!canSave) return;
    setSaving(true);
    setError('');
    try {
      // Sync only applies the plan the user has just seen; without a fresh one, save the source alone.
      // A project mutation syncs only its own root on the server, so this is one call for both scopes.
      const synced = sync && fresh && preview !== null;
      if (synced) await hooksApi.configure(mutation(), preview.plan.revision, true);
      else await hooksApi.save(mutation());
      onSaved(synced);
    } catch (e) {
      setError((e as Error).message);
      setSaving(false);
    }
  };

  // Cmd/Ctrl+S saves the source only, from any field or editor in the dialog.
  const scope = useRef<HTMLDivElement>(null);
  useSaveShortcut(() => void save(false), true, scope);

  const current = order.includes(active) ? active : order[0];
  const title = t(editing ? 'hooks.editTitle' : 'hooks.addTitle');
  const writing = shown ? new Set(shown.changes.filter(writes).map((c) => c.path)).size : 0;

  // Unfilled command targets that would not get the same event names when copied: say what they get instead.
  const copyNotes = unfilled.flatMap((to) => {
    const from = copySource(order, to, draftOf, agentOf);
    if (!from) return [];
    const { notes } = copyDraft(catalog, agentOf(from), agentOf(to), draftOf(from));
    return notes.map((n) => ({ agent: to, source: from, event: n.from, closest: n.to, kind: n.kind }));
  });

  const emptyTab = (agent: string) => {
    const from = copySource(order, agent, draftOf, agentOf);
    const rows = from ? draftRows(draftOf(from)) ?? [] : [];
    const events = [...new Set(rows.map((r) => r.event))];
    const notes = from ? copyDraft(catalog, agentOf(from), agentOf(agent), draftOf(from)).notes : [];
    const same = from !== undefined && notes.length === 0;
    // Some events with no close match must be picked by hand; the line says so instead of promising the closest.
    const noClose = notes.filter((n) => n.kind === 'none').length;
    const copyNote = same ? 'hooks.emptyTarget.same' : noClose === 0 ? 'hooks.emptyTarget.differs' : noClose === notes.length ? 'hooks.emptyTarget.none' : 'hooks.emptyTarget.partial';
    return (
      <div className="flex flex-col items-center gap-3 rounded-[12px] border border-dashed border-line-2 px-6 py-8 text-center">
        <h3 className="text-[15px] font-semibold">{t('hooks.emptyTarget.title', { agent: labelOf(agent) })}</h3>
        <p className="text-[13px] text-ink-2">
          {from
            ? t(copyNote, { from: labelOf(from), agent: labelOf(agent), events: events.join(', ') })
            : t(isCodeAgent(agentOf(agent)) ? 'hooks.emptyTarget.code' : 'hooks.emptyTarget.scratch', { agent: labelOf(agent) })}
        </p>
        <div className="flex flex-wrap justify-center gap-2">
          {from && <Button variant="secondary" size="sm" onClick={() => copyFrom(from, agent)} disabled={saving}><Copy size={14} />{t('hooks.copyFrom', { agent: labelOf(from) })}</Button>}
          <Button variant="ghost" size="sm" onClick={() => startFresh(agent)} disabled={saving}><Plus size={14} />{t('hooks.startFresh')}</Button>
        </div>
      </div>
    );
  };

  return (
    <DialogShell open onClose={requestClose} padding="none" preventClose={saving} ariaLabel={title} className="min-w-0 !max-w-[min(880px,calc(100vw-2rem))]">
      <div className="dh" ref={scope}>
        <div className="flex min-w-0 flex-col gap-1">
          <h2 className="ss-h2">{showPreview ? t('hooks.previewTitle') : title}</h2>
          {showPreview && shown && <p className="text-[13px] text-ink-2">{t('hooks.previewSummary', { name: trimmed, count: String(writing) })}</p>}
        </div>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={requestClose} disabled={saving}><X size={16} /></button>
      </div>
      {showPreview && preview ? (
        <div className="db">
          {!fresh && <div className="ss-note warn" role="alert"><span className="flex-1">{t('hooks.previewStale')}</span></div>}
          <HooksPreview plan={shown ?? preview.plan} unmanaged={unmanaged} />
          {shown?.changes.some((c) => c.action === 'conflict') && (
            <Checkbox label={t('hooks.takeover')} checked={takeover} disabled={saving} onChange={() => { setTakeover(!takeover); setPreview(null); setShowPreview(false); }} />
          )}
          {error && <div className="ss-note bad" role="alert"><span className="flex-1">{error}</span></div>}
        </div>
      ) : (
        <form id="hook-form" className="db" onSubmit={(e) => { e.preventDefault(); void save(false); }}>
          <div className="grid grid-cols-1 gap-3.5 sm:grid-cols-2">
            <div className="ss-fld">
              <label htmlFor="hook-name">{t('hooks.name')}</label>
              <span className={`ss-inp font-mono ${nameError ? 'err' : ''}`}>
                <input id="hook-name" autoFocus={!editing} value={name} onChange={(e) => setName(e.target.value)} placeholder="block-force-push" disabled={editing || saving} />
              </span>
              {nameError && <span className="hp !text-bad">{nameError}</span>}
            </div>
            <div className="ss-fld">
              <label htmlFor="hook-description">{t('hooks.description')}</label>
              <span className="ss-inp">
                <input id="hook-description" value={description} onChange={(e) => setDescription(e.target.value)} disabled={saving} />
              </span>
            </div>
          </div>

          <div className="ss-fld">
            <span className="text-[13px] font-semibold">{t('hooks.agents')}</span>
            <div className="flex flex-wrap gap-x-5 gap-y-3">
              {picker.map((agent) => {
                const on = selected.includes(agent);
                return (
                  <button key={agent} type="button" role="checkbox" aria-checked={on} className={`ss-tgl ${on ? 'on' : ''}`} onClick={() => toggle(agent)} disabled={saving}>
                    <span className="ic"><AgentIcon target={agentOf(agent)} size={20} /><i><Check size={9} strokeWidth={3.5} /></i></span>
                    {labelOf(agent)}
                  </button>
                );
              })}
            </div>
            {order.length === 0 && <span className="hp">{t('hooks.noAgentsNote')}</span>}
          </div>

          {current && (
            <div className="flex flex-col gap-3.5">
              <SegmentedControl
                className="self-start"
                value={current}
                onChange={setActive}
                options={order.map((a) => ({
                  value: a,
                  label: (
                    <span className="inline-flex items-center gap-1.5">
                      <AgentIcon target={agentOf(a)} size={14} />{labelOf(a)}
                      {unfilled.includes(a)
                        ? <span className="h-[5px] w-[5px] rounded-full bg-ink-2" aria-label={t('hooks.tabUnfilled')} />
                        : bindingInvalid(checks[a]) && <span className="ss-tag bad" aria-label={t('hooks.tabInvalid')}>!</span>}
                    </span>
                  ),
                }))}
              />
              {unfilled.includes(current) && !started.includes(current) && (isCodeAgent(agentOf(current)) || copySource(order, current, draftOf, agentOf))
                ? emptyTab(current)
                : <HookBindingEditor key={current} agent={agentOf(current)} name={trimmed} draft={draftOf(current)} check={checks[current]} catalog={catalog} onChange={(d) => setDraft(current, d)} disabled={saving} />}
              {copyNotes.map((n) => (
                <div key={`${n.agent}:${n.event}`} className="flex items-center gap-2.5 rounded-[10px] bg-sunken px-3.5 py-3 text-[13px] text-ink-2">
                  <AgentIcon target={agentOf(n.agent)} size={16} />
                  <span className="flex-1">
                    {n.kind === 'closest'
                      ? t('hooks.copyNote.closest', { agent: labelOf(n.agent), event: n.event, closest: n.closest, description: eventDescription(t, n.closest, catalogEvent(catalog, agentOf(n.agent), n.closest)?.description ?? ''), from: labelOf(n.source) })
                      : t('hooks.copyNote.none', { agent: labelOf(n.agent), event: n.event, from: labelOf(n.source) })}
                  </span>
                </div>
              ))}
            </div>
          )}
          {error && <div className="ss-note bad" role="alert"><span className="flex-1">{error}</span></div>}
        </form>
      )}
      <div className="df flex-wrap [&>span]:basis-full sm:[&>span]:basis-auto">
        {confirmClose ? (
          <>
            <span className="flex-1 text-[13px] font-semibold" role="alert">{t('hooks.unsaved')}</span>
            <Button variant="ghost" onClick={() => setConfirmClose(false)}>{t('hooks.keepEditing')}</Button>
            <Button variant="danger" onClick={onClose}>{t('hooks.discard')}</Button>
          </>
        ) : showPreview ? (
          <>
            <span className="flex-1 text-[13px] text-ink-2">{t('hooks.previewNote')}</span>
            <Button variant="ghost" onClick={() => setShowPreview(false)} disabled={saving}>{t('common.back')}</Button>
            <Button variant="secondary" loading={saving} disabled={!canSave} onClick={() => void save(false)}>{t('hooks.saveOnly')}</Button>
            <Button variant="primary" loading={saving} disabled={!fresh || !shown || shown.blocked} onClick={() => void save(true)}><RefreshCw size={15} />{t('mcp.saveSync')}</Button>
          </>
        ) : (
          <>
            <span className="flex-1 text-[13px] text-ink-2">
              {unfilled.length > 0 ? t('hooks.unfilled', { count: String(unfilled.length), agents: unfilled.map(labelOf).join('、') })
                : invalid.length > 0 ? t('hooks.incomplete', { agents: invalid.map(labelOf).join('、') })
                  : t('hooks.saveNote')}
            </span>
            <Button variant="ghost" onClick={requestClose} disabled={saving}>{t('common.cancel')}</Button>
            <Button variant="secondary" disabled={!canSave || order.length === 0} onClick={() => void openPreview()}>{t('hooks.preview')}</Button>
            <Button type="submit" form="hook-form" variant="primary" loading={saving} disabled={!canSave}>{t('common.save')}</Button>
          </>
        )}
      </div>
    </DialogShell>
  );
}
