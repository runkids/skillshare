import { useState } from 'react';
import { Check, X } from 'lucide-react';
import { hookAgents, hooksApi, type HookEntry, type HookMutation, type HookPlan } from '../../api/hooks';
import { useT } from '../../i18n';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import { Checkbox } from '../Input';
import DialogShell from '../DialogShell';
import SegmentedControl from '../SegmentedControl';
import HookBindingEditor from './HookBindingEditor';
import HooksPreview from './HooksPreview';
import {
  HOOK_NAME, bindingInvalid, bindingToDraft, boundAgents, checkBinding, draftToBinding, emptyBinding, hookLabel, isCodeAgent, rootPlan,
  type BindingDraft,
} from './hooksView';

interface Props {
  initial?: { name: string; entry: HookEntry };
  existingNames: string[];
  /** A root under hooks.projects: the hook is saved there instead of in the global source. */
  project?: string;
  /** Agents Skillshare can manage hooks for; defaults to all ten. */
  agents?: readonly string[];
  onClose: () => void;
  onSaved: () => void;
}

/** Add or edit one source hook. Saving only changes the source; Sync writes the native files. */
export default function HookDialog({ initial, existingNames, project, agents = hookAgents, onClose, onSaved }: Props) {
  const t = useT();
  const editing = Boolean(initial);
  const [name, setName] = useState(initial?.name ?? '');
  const [description, setDescription] = useState(initial?.entry.description ?? '');
  const [selected, setSelected] = useState<string[]>(() => (initial ? boundAgents(initial.entry) : []));
  // A drafted Agent keeps its content when it is unticked and ticked again.
  const [drafts, setDrafts] = useState<Record<string, BindingDraft>>(() => Object.fromEntries((initial ? boundAgents(initial.entry) : []).map((a) => [a, bindingToDraft(a, initial?.entry.bindings[a] ?? initial?.entry.bindings.factory)])));
  const [active, setActive] = useState(selected[0] ?? '');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [preview, setPreview] = useState<{ key: string; plan: HookPlan } | null>(null);
  const [showPreview, setShowPreview] = useState(false);
  // Off unless asked for: taking over an Agent's conflicting native hooks is never implied by an edit.
  const [takeover, setTakeover] = useState(false);

  const trimmed = name.trim();
  const nameError = trimmed && !HOOK_NAME.test(trimmed) ? t('hooks.nameHint') : !editing && existingNames.includes(trimmed) ? t('mcp.nameTaken') : '';
  const order = agents.filter((a) => selected.includes(a));
  const draftOf = (agent: string) => drafts[agent] ?? emptyBinding(agent);
  const checks = Object.fromEntries(order.map((a) => [a, checkBinding(a, draftOf(a))]));
  const invalid = order.filter((a) => bindingInvalid(checks[a]));
  const valid = Boolean(trimmed) && !nameError && invalid.length === 0;
  const canSave = valid && !saving;

  const entry = (): HookEntry => ({
    ...(description.trim() && { description: description.trim() }),
    ...(initial?.entry.enabled === false && { enabled: false }),
    bindings: Object.fromEntries(order.map((a) => [a, draftToBinding(a, draftOf(a))])),
  });
  const mutation = (): HookMutation => ({ ...(project && { project }), name: trimmed, entry: entry(), ...(takeover && { replace: true }) });
  // A preview is only good for the exact hook it was taken of; any edit makes it stale. Being busy is not an edit.
  const key = valid ? JSON.stringify(mutation()) : '';
  const fresh = preview !== null && preview.key === key;
  const shown = preview && rootPlan(preview.plan, project);

  const toggle = (agent: string) => {
    if (selected.includes(agent)) {
      setSelected(selected.filter((x) => x !== agent));
      if (active === agent) setActive(order.find((x) => x !== agent) ?? '');
    } else {
      setSelected([...selected, agent]);
      setActive(agent);
    }
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
      if (sync && fresh && preview) await hooksApi.configure(mutation(), preview.plan.revision, true);
      else await hooksApi.save(mutation());
      onSaved();
    } catch (e) {
      setError((e as Error).message);
      setSaving(false);
    }
  };

  const title = t(editing ? 'hooks.editTitle' : 'hooks.addTitle');
  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={saving} ariaLabel={title} className="!max-w-[760px]">
      <div className="dh">
        <h2 className="ss-h2">{showPreview ? t('hooks.previewTitle') : title}</h2>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={saving}><X size={16} /></button>
      </div>
      {showPreview && preview ? (
        <div className="db">
          {!fresh && <div className="ss-note warn" role="alert"><span className="flex-1">{t('hooks.previewStale')}</span></div>}
          <HooksPreview plan={shown ?? preview.plan} />
          {shown?.changes.some((c) => c.action === 'conflict') && (
            <Checkbox label={t('hooks.takeover')} checked={takeover} disabled={saving} onChange={() => { setTakeover(!takeover); setPreview(null); setShowPreview(false); }} />
          )}
          {error && <div className="ss-note bad" role="alert"><span className="flex-1">{error}</span></div>}
        </div>
      ) : (
        <form id="hook-form" className="db" onSubmit={(e) => { e.preventDefault(); void save(false); }}>
          <div className="grid grid-cols-2 gap-3.5">
            <div className="ss-fld">
              <label htmlFor="hook-name">{t('hooks.name')}</label>
              <span className={`ss-inp ${nameError ? 'err' : ''}`}>
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
              {agents.map((agent) => {
                const on = selected.includes(agent);
                return (
                  <button key={agent} type="button" role="checkbox" aria-checked={on} className={`ss-tgl ${on ? 'on' : ''}`} onClick={() => toggle(agent)} disabled={saving}>
                    <span className="ic"><AgentIcon target={agent} size={20} /><i><Check size={9} strokeWidth={3.5} /></i></span>
                    {hookLabel(agent)}{isCodeAgent(agent) && <span className="ss-tag">{t('hooks.kind.code')}</span>}
                  </button>
                );
              })}
            </div>
            {order.length === 0 && <span className="hp">{t('hooks.noAgentsNote')}</span>}
          </div>

          {order.length > 0 && (
            <div className="flex flex-col gap-3.5">
              <SegmentedControl
                className="self-start"
                value={order.includes(active) ? active : order[0]}
                onChange={setActive}
                options={order.map((a) => ({ value: a, label: <span className="inline-flex items-center gap-1.5"><AgentIcon target={a} size={14} />{hookLabel(a)}{bindingInvalid(checks[a]) && <span className="ss-tag bad">!</span>}</span> }))}
              />
              {(() => {
                const agent = order.includes(active) ? active : order[0];
                return <HookBindingEditor key={agent} agent={agent} name={trimmed} draft={draftOf(agent)} check={checks[agent]} onChange={(d) => setDrafts((prev) => ({ ...prev, [agent]: d }))} disabled={saving} />;
              })()}
            </div>
          )}
          {error && <div className="ss-note bad" role="alert"><span className="flex-1">{error}</span></div>}
        </form>
      )}
      <div className="df">
        <span className="flex-1 text-[13px] text-ink-2">
          {showPreview ? t('hooks.previewNote') : invalid.length > 0 ? <span className="ss-st warn">{t('hooks.incomplete', { agents: invalid.map(hookLabel).join(', ') })}</span> : t('hooks.saveNote')}
        </span>
        {showPreview ? (
          <>
            <Button variant="secondary" onClick={() => setShowPreview(false)} disabled={saving}>{t('common.back')}</Button>
            <Button variant="primary" loading={saving} disabled={!fresh || !shown || shown.blocked} onClick={() => void save(true)}>{t('mcp.saveSync')}</Button>
          </>
        ) : (
          <>
            <Button variant="ghost" onClick={onClose} disabled={saving}>{t('common.cancel')}</Button>
            <Button variant="secondary" disabled={!canSave || order.length === 0} onClick={() => void openPreview()}>{t('hooks.preview')}</Button>
            <Button type="submit" form="hook-form" variant="primary" loading={saving} disabled={!canSave}>{t('common.save')}</Button>
          </>
        )}
      </div>
    </DialogShell>
  );
}
