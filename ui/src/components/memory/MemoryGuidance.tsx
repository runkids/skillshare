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
    <section className="flex min-w-0 flex-col gap-3" aria-label={t('memory.useWithAgents')}>
      <div className="flex flex-wrap items-center gap-2 pt-1.5 pl-1">
        <h3 className="text-[15px] font-bold">{t('memory.useWithAgents')}</h3>
        {!!targets.length && <span className="text-[12.5px] text-ink-3">{t('memory.connectedCount', { count: targets.filter((target) => target.state === 'configured').length, total: targets.length })}</span>}
        <span className="flex-1" />
        <Tooltip content={<span className="block whitespace-pre-wrap break-words font-mono">{instructions}</span>}>
          <CopyButton value={instructions} title={t('memory.copyInstructions')} label={t('memory.copyInstructions')} copiedLabel={t('memory.copied')} errorMessage={t('memory.copyFailed')} unstyled className="ss-btn ghost sm" />
        </Tooltip>
        <Tooltip content={<span className="block whitespace-pre-wrap break-words font-mono">{prompt}</span>}>
          <CopyButton value={prompt} title={t('memory.check')} label={t('memory.check')} copiedLabel={t('memory.copied')} errorMessage={t('memory.copyFailed')} unstyled className="ss-btn ghost sm" />
        </Tooltip>
        <Button variant="secondary" size="sm" disabled={!initialized || !targets.length} onClick={() => { void guidance.refetch(); setSelected([]); setPlan(null); setOpen(true); }}>{t('memory.connect')}</Button>
      </div>
      {guidance.error && <div className="ss-note bad">{guidance.error.message}</div>}
      {!!targets.length && <div className="ss-list">
        {targets.map((target) => <div key={target.name} className="ss-r !min-h-[56px]">
          <span className="ss-at"><AgentIcon target={target.name} size={17} /></span>
          <span className="w-[110px] shrink-0 truncate font-mono text-[13px] font-semibold">{target.name}</span>
          <span className="min-w-0 flex-1 truncate font-mono text-[12.5px] text-ink-2" title={target.file}>{target.file ? shortenHome(target.file) : ''}</span>
          <span className={`ss-st ${target.state === 'configured' ? 'ok' : target.state === 'unconfigured' ? 'off' : 'warn'}`}>{statusText(target)}</span>
        </div>)}
      </div>}
      {initialized && guidance.data?.targets.length === 0 && <p className="text-[13px] text-ink-3">{t('memory.noTargets')}</p>}
      <p className="pl-1 text-[12.5px] leading-relaxed text-ink-3">
        {t('memory.connectionHint')} <Link to="?tab=instructions" className="ss-more">{t('memory.openInstructions')}</Link>
      </p>
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
    </section>
  );
}
