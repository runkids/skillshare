import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { AlertCircle, ArrowRight, RefreshCw, Webhook } from 'lucide-react';
import type { HookInventory } from '../../api/hooks';
import Button from '../Button';
import EmptyState from '../EmptyState';
import { useToast } from '../Toast';
import HookDialog from '../hooks/HookDialog';
import { HooksSyncDialog } from '../hooks/HooksSyncBox';
import { agentOfKey, blockedHint, boundAgents, hookAccounts, keyLabel, hookMessage, hookNote, isCodeAgent, scopeEntries, scopePlan, syncState, writes } from '../hooks/hooksView';
import { useT } from '../../i18n';
import { queryKeys } from '../../lib/queryKeys';

/**
 * One Agent's hooks of the current scope, laid out like the MCP tab: a row per hook bound to it.
 * `project` is the hooks.projects root of a project target; global targets pass none.
 */
export default function TargetHooks({ agent, data, project }: { agent: string; data: HookInventory; project?: string }) {
  const t = useT();
  const { toast } = useToast();
  const cache = useQueryClient();
  const [editing, setEditing] = useState('');
  const [syncing, setSyncing] = useState(false);
  const entries = scopeEntries(data, project);
  const plan = scopePlan(data, project);
  const rows = Object.entries(entries).filter(([, e]) => boundAgents(e).includes(agent));
  const mine = (plan?.changes ?? []).filter((c) => c.target === agent);
  const conflicts = mine.filter((c) => c.action === 'conflict');
  const pending = mine.filter(writes).length;
  // Sync all applies the whole plan globally (projects included) and only the root for a project.
  const all = ((project ? plan : data.plan)?.changes ?? []).filter(writes).length;
  const accounts = hookAccounts(data.targets);
  const nativeAgent = agentOfKey(accounts, agent);
  const label = keyLabel(accounts, agent);
  const note = hookNote(t, nativeAgent, data.targets.find((x) => x.name === agent)?.note);
  const manage = project ? `/projects/${encodeURIComponent(project)}?tab=hooks` : '/hooks';

  if (rows.length === 0) {
    return <EmptyState icon={Webhook} title={t('targetDetail.hooks.emptyTitle', { name: label })} description={t('targetDetail.hooks.emptyDescription', { name: label })} action={<Link to={manage} className="ss-btn pri">{t('targetDetail.hooks.open')}</Link>} />;
  }

  return (
    <div className="grid grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)] items-start gap-12">
      <section className="flex flex-col gap-5">
        <h2 className="ss-h2">{t('targetDetail.whatSyncs')}</h2>
        <p className="text-[13.5px]">{t(rows.length === 1 ? 'targetDetail.hooks.summary.one' : 'targetDetail.hooks.summary.other', { count: rows.length, name: label })}</p>
        {conflicts.map((c) => (
          <div key={`${c.path}:${c.name}`} className="ss-note warn">
            <AlertCircle size={16} />
            <span className="flex-1"><span className="font-mono">{c.name}</span>: {c.message ? hookMessage(t, c.message) : t('hooks.status.conflict')}</span>
          </div>
        ))}
        <div className="ss-list !shadow-none">
          <div className="ss-lh">
            <span className="flex-1">{t('targetDetail.previewCount', { count: rows.length })}</span>
            <span className="w-[96px]">{t('targetDetail.result')}</span>
          </div>
          {rows.map(([name, entry]) => {
            const state = syncState(plan, name, agent);
            const enabled = entry.enabled !== false;
            return (
              <button key={name} type="button" className="ss-r link !min-h-[42px] w-full text-left" aria-label={t('hooks.editHook', { name })} onClick={() => setEditing(name)}>
                <span className="flex min-w-0 flex-1 items-center gap-2">
                  <span title={name} className={`truncate font-mono text-[13px] font-semibold ${enabled ? '' : 'text-ink-3'}`}>{name}</span>
                  {!enabled && <span className="ss-tag">{t('hooks.disabled')}</span>}
                  <span className="ss-tag">{t(isCodeAgent(nativeAgent) ? 'hooks.kind.code' : 'hooks.kind.command')}</span>
                </span>
                {/* Synchronized only: whether the Agent trusts and loads it is the Agent's call. */}
                <span className="w-[96px] shrink-0"><span className={`ss-st ${state === 'synced' ? 'ok' : state === 'pending' ? 'warn' : state === 'conflict' ? 'bad' : 'off'}`}>{t(`hooks.sync.${state}`)}</span></span>
              </button>
            );
          })}
        </div>
        <p className="text-[13px] text-ink-3">{t('hooks.nativeNote')}</p>
      </section>

      <aside className="flex flex-col gap-7">
        <div className="flex flex-col gap-3">
          <div className="flex items-center justify-between gap-2">
            <h2 className="ss-h2">{t('sync.title')}</h2>
            <span className={`ss-st ${pending > 0 || conflicts.length > 0 ? 'warn' : 'ok'}`}>{pending > 0 ? t(pending === 1 ? 'mcp.pending.one' : 'mcp.pending.other', { count: pending }) : conflicts.length > 0 ? t('hooks.status.conflict') : t('targets.state.synced')}</span>
          </div>
          {pending > 0 && (
            <>
              <p className="text-[13px] text-ink-2">{t(all === 1 ? 'targetDetail.hooks.syncHint.one' : 'targetDetail.hooks.syncHint.other', { count: all })}</p>
              {/* A conflict anywhere in the plan holds this sync too; the review shows where, and its Sync Now stays disabled. */}
              <Button variant="secondary" className="self-start" onClick={() => setSyncing(true)}>{plan?.blocked ? <AlertCircle size={15} /> : <RefreshCw size={15} />}{t(plan?.blocked ? 'hooks.viewConflicts' : 'targetDetail.hooks.syncAll')}</Button>
              {plan?.blocked && <p className="text-[13px] text-warn">{blockedHint(t, (project ? plan : data.plan) ?? plan)}</p>}
            </>
          )}
          {note && <p className="text-[13px] text-ink-2">{note}</p>}
        </div>
        <div className="flex flex-col gap-3">
          <h2 className="ss-h2">{t('targetDetail.hooks.manage')}</h2>
          <p className="text-[13px] text-ink-2">{t('targetDetail.hooks.manageHint', { name: label })}</p>
          <Link to={manage} className="ss-btn self-start">{t('targetDetail.hooks.open')}<ArrowRight size={14} /></Link>
        </div>
      </aside>

      {syncing && <HooksSyncDialog project={project} onClose={() => setSyncing(false)} />}
      {editing && (
        <HookDialog
          accounts={accounts}
          initial={{ name: editing, entry: entries[editing] }}
          existingNames={Object.keys(entries)}
          project={project}
          onClose={() => setEditing('')}
          onSaved={(synced) => {
            setEditing('');
            for (const queryKey of [queryKeys.hooks, queryKeys.config]) void cache.invalidateQueries({ queryKey });
            toast(t(synced ? 'hooks.toast.savedSynced' : 'hooks.toast.saved'), 'success');
          }}
        />
      )}
    </div>
  );
}
