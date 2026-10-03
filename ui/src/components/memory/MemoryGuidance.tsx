import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Check } from 'lucide-react';
import { Link } from 'react-router-dom';
import { api, ApiError } from '../../api/client';
import type { MemoryGuidancePlan } from '../../api/memory';
import { useT } from '../../i18n';
import { queryKeys } from '../../lib/queryKeys';
import { shortenHome } from '../../lib/paths';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import CodeView from '../CodeView';
import CopyButton from '../CopyButton';
import DialogShell from '../DialogShell';
import Tooltip from '../Tooltip';

export default function MemoryGuidance({ initialized, instructions }: { initialized: boolean; instructions: string }) {
  const t = useT();
  const client = useQueryClient();
  // States follow assignments and files that other tabs and the CLI change, so never trust a cached copy.
  const guidance = useQuery({ queryKey: [...queryKeys.memory.all, 'guidance'], queryFn: api.getMemoryGuidance, enabled: initialized, staleTime: 0 });
  const [open, setOpen] = useState(false);
  const [selected, setSelected] = useState<string[]>([]);
  const [plan, setPlan] = useState<MemoryGuidancePlan | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const close = () => { setOpen(false); setPlan(null); setError(''); };
  const review = async () => {
    setBusy(true); setError('');
    try { setPlan(await api.planMemoryGuidance(selected)); }
    catch (err) { setError((err as Error).message); }
    finally { setBusy(false); }
  };
  const apply = async () => {
    if (!plan || busy) return;
    setBusy(true); setError('');
    try {
      const result = await api.applyMemoryGuidance(selected, plan.token);
      void client.invalidateQueries({ queryKey: queryKeys.memory.all });
      void client.invalidateQueries({ queryKey: queryKeys.instructions.all });
      void client.invalidateQueries({ queryKey: queryKeys.fileBackups.all });
      if (result.success) close();
      else { setPlan(null); setError(result.errors.map((e) => `${e.path}: ${e.error}`).join('\n')); }
    } catch (err) { setPlan(null); setError(err instanceof ApiError && err.code === 'memory_guidance_stale' ? t('memory.guidanceStale') : (err as Error).message); }
    finally { setBusy(false); }
  };
  const prompt = t('memory.checkPrompt');
  const targets = guidance.data?.targets ?? [];
  // The specific reason beats the generic state; it must be readable without hovering.
  const statusText = (target: (typeof targets)[number]) => target.detail ? t(`memory.connectionDetail.${target.detail}`, undefined, target.detail) : t(`memory.connection.${target.state}`);
  return (
    <aside className="ss-box flex min-w-0 flex-[1_1_280px] flex-col gap-3.5 !shadow-none md:max-w-[340px]" aria-label={t('memory.useWithAgents')}>
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-[15px] font-semibold">{t('memory.useWithAgents')}</h2>
        {!!guidance.data?.targets.length && <span className="text-[12px] text-ink-3">{t('memory.connectedCount', { count: guidance.data.targets.filter((target) => target.state === 'configured').length, total: guidance.data.targets.length })}</span>}
      </div>
      {guidance.error && <div className="ss-note bad">{guidance.error.message}</div>}
      {!!targets.length && <span className="ss-stack flex-wrap gap-y-1">
        {targets.map((target) => <span key={target.name} title={`${target.name} · ${t(`memory.connection.${target.state}`)}`} className={`ss-at ${target.state === 'configured' ? '' : 'opacity-45 grayscale'}`}>
          <AgentIcon target={target.name} size={14} />
          <span className="sr-only">{target.name} · {t(`memory.connection.${target.state}`)}</span>
        </span>)}
      </span>}
      {/* Only targets that need a fix get a row; the stack already shows the rest. */}
      {targets.filter((target) => target.state === 'outdated' || target.state === 'broken').map((target) => <div key={target.name} className="flex items-center gap-2.5 text-[12.5px]">
        <AgentIcon target={target.name} size={16} />
        <span className="min-w-0 flex-1 truncate font-mono font-semibold" title={target.file ? shortenHome(target.file) : undefined}>{target.name}</span>
        <span className="ss-st warn text-right text-[12px]">{statusText(target)}</span>
      </div>)}
      {initialized && guidance.data?.targets.length === 0 && <p className="text-[13px] text-ink-3">{t('memory.noTargets')}</p>}
      <Button variant="secondary" size="sm" disabled={!initialized || !guidance.data?.targets.length} onClick={() => { void guidance.refetch(); setSelected([]); setPlan(null); setOpen(true); }}>{t('memory.connect')}</Button>
      <div className="flex flex-col items-start gap-1 border-t border-line-soft pt-3 text-[13px]">
        <Tooltip content={<span className="block whitespace-pre-wrap break-words font-mono">{instructions}</span>}>
          <CopyButton value={instructions} title={t('memory.copyInstructions')} label={t('memory.copyInstructions')} copiedLabel={t('memory.copied')} errorMessage={t('memory.copyFailed')} unstyled className="ss-btn ghost sm !px-1.5" />
        </Tooltip>
        <Tooltip content={<span className="block whitespace-pre-wrap break-words font-mono">{prompt}</span>}>
          <CopyButton value={prompt} title={t('memory.check')} label={t('memory.check')} copiedLabel={t('memory.copied')} errorMessage={t('memory.copyFailed')} unstyled className="ss-btn ghost sm !px-1.5" />
        </Tooltip>
        <Link to="?tab=instructions" className="ss-more px-1.5 py-1">{t('memory.openInstructions')}</Link>
      </div>
      <p className="text-[12px] leading-relaxed text-ink-3">{t('memory.connectionHint')}</p>
      <DialogShell open={open} onClose={close} preventClose={busy} padding="none" maxWidth={plan ? "full" : "lg"} ariaLabel={t('memory.connect')}>
        <div className="dh"><h2 className="ss-h2">{t(plan ? 'memory.reviewConnections' : 'memory.connect')}</h2></div>
        <div className="db flex flex-col gap-4">
          {error && <div role="alert" className="ss-note bad whitespace-pre-wrap">{error}</div>}
          {!plan ? <>
            <p className="text-[13px] text-ink-2">{t('memory.connectHint')}</p>
            <div className="ss-list !shadow-none">
              {targets.map((target) => {
                const on = selected.includes(target.name) || target.state === 'configured';
                const disabled = busy || target.state === 'configured' || target.state === 'broken';
                return <label key={target.name} title={target.file ? shortenHome(target.file) : undefined} className={`ss-r !min-h-11 !gap-2.5 !py-1.5 ${disabled ? 'cursor-not-allowed' : 'cursor-pointer'}`}>
                  <input type="checkbox" className="ss-chk-input sr-only" checked={on} disabled={disabled}
                    onChange={(e) => setSelected((prev) => e.target.checked ? [...prev, target.name] : prev.filter((name) => name !== target.name))} />
                  <span className={`ss-chk ${on ? 'on' : ''} ${disabled ? 'opacity-40' : ''}`}>{on && <Check size={12} strokeWidth={3} />}</span>
                  <span className="ss-at !h-6 !w-6"><AgentIcon target={target.name} size={14} /></span>
                  <span className="min-w-0 flex-1 truncate font-mono text-[13px] font-semibold text-ink">{target.name}</span>
                  <span className={`text-right text-[12.5px] ${target.state === 'outdated' || target.state === 'broken' ? 'text-warn' : 'text-ink-3'}`}>{statusText(target)}</span>
                </label>;
              })}
            </div>
          </> : <>
            {(plan.warnings ?? []).map((warning) => <div key={`${warning.code}:${warning.path}:${warning.target ?? ""}`} className="ss-note warn">{warning.path}: {warning.code === 'also_read_by' ? t('memory.alsoReadBy', { targets: (warning.targets ?? []).join(', ') }) : t('memory.overLimit', { target: warning.target ?? '', limit: warning.limit ?? 0, chars: warning.chars ?? 0 })}</div>)}
            {(plan.skipped ?? []).map((item) => <div key={item.target} className="ss-note warn">{item.target}: {t(`memory.skipped.${item.reason}`, undefined, item.reason)}</div>)}
            {plan.changes.length === 0 && <p>{t('memory.noChanges')}</p>}
            {plan.changes.map((change) => <div key={change.path} className="flex min-w-0 flex-col gap-2">
              <h3 className="break-all font-mono text-[13px] font-semibold">{change.path}</h3>
              <div className="grid min-w-0 grid-cols-1 gap-4 md:grid-cols-2">
                <div className="min-w-0"><p className="mb-2 text-[13px]">{t('memory.before')}</p><CodeView content={change.before} lang="md" className="h-[40vh]" /></div>
                <div className="min-w-0"><p className="mb-2 text-[13px]">{t('memory.after')}</p><CodeView content={change.after} lang="md" className="h-[40vh]" /></div>
              </div>
            </div>)}
          </>}
        </div>
        <div className="df">
          <Button variant="ghost" disabled={busy} onClick={close}>{t('common.cancel')}</Button>
          {plan && <Button variant="ghost" disabled={busy} onClick={() => setPlan(null)}>{t('common.back')}</Button>}
          <Button variant="primary" loading={busy} disabled={plan ? plan.changes.length === 0 : selected.length === 0} onClick={() => plan ? void apply() : void review()}>{t(plan ? 'memory.applyConnections' : 'memory.reviewConnections')}</Button>
        </div>
      </DialogShell>
    </aside>
  );
}
