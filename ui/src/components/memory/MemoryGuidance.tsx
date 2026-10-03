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
import { Checkbox } from '../Checkbox';
import CodeView from '../CodeView';
import CopyButton from '../CopyButton';
import DialogShell from '../DialogShell';
import Tooltip from '../Tooltip';

export default function MemoryGuidance({ initialized, instructions }: { initialized: boolean; instructions: string }) {
  const t = useT();
  const client = useQueryClient();
  const guidance = useQuery({ queryKey: [...queryKeys.memory.all, 'guidance'], queryFn: api.getMemoryGuidance, enabled: initialized });
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
  return (
    <aside className="ss-box flex min-w-0 flex-[1_1_280px] flex-col gap-3.5 !shadow-none md:max-w-[340px]" aria-label={t('memory.useWithAgents')}>
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-[15px] font-semibold">{t('memory.useWithAgents')}</h2>
        {!!guidance.data?.targets.length && <span className="text-[12px] text-ink-3">{t('memory.connectedCount', { count: guidance.data.targets.filter((target) => target.state === 'configured').length, total: guidance.data.targets.length })}</span>}
      </div>
      {guidance.error && <div className="ss-note bad">{guidance.error.message}</div>}
      {!!guidance.data?.targets.length && <ul className="flex flex-col">
        {guidance.data.targets.map((target) => <li key={target.name} className="flex items-center gap-2.5 border-b border-line-soft py-2.5 last:border-b-0">
          <AgentIcon target={target.name} size={20} />
          <div className="flex min-w-0 flex-1 flex-col gap-0.5">
            <span className="font-mono text-[13px] font-semibold">{target.name}</span>
            {target.detail && <span className="text-[12px] text-ink-3">{t(`memory.connectionDetail.${target.detail}`, undefined, target.detail)}</span>}
            {target.file && <span className="truncate font-mono text-[11.5px] text-ink-3" title={shortenHome(target.file)}>{target.file.split('/').slice(-2).join('/')}</span>}
          </div>
          {target.state === 'configured'
            ? <span role="img" aria-label={t('memory.connection.configured')} title={t('memory.connection.configured')}><Check size={16} className="text-ok" /></span>
            : <span className={`ss-st text-[12px] ${target.state === 'unconfigured' ? 'off' : 'warn'}`}>{t(`memory.connection.${target.state}`)}</span>}
        </li>)}
      </ul>}
      {initialized && guidance.data?.targets.length === 0 && <p className="text-[13px] text-ink-3">{t('memory.noTargets')}</p>}
      <Button variant="secondary" size="sm" disabled={!initialized || !guidance.data?.targets.length} onClick={() => { setSelected([]); setPlan(null); setOpen(true); }}>{t('memory.connect')}</Button>
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
            {(guidance.data?.targets ?? []).map((target) => <Checkbox key={target.name} label={`${target.name} · ${t(`memory.connection.${target.state}`)}`} checked={selected.includes(target.name)} disabled={busy || target.state === 'configured' || target.state === 'broken'}
              onChange={(checked) => setSelected((prev) => checked ? [...prev, target.name] : prev.filter((name) => name !== target.name))} />)}
          </> : <>
            {(plan.warnings ?? []).map((warning) => <div key={`${warning.code}:${warning.path}:${warning.target ?? ""}`} className="ss-note warn">{warning.path}: {warning.code === 'also_read_by' ? t('memory.alsoReadBy', { targets: (warning.targets ?? []).join(', ') }) : t('memory.overLimit', { target: warning.target ?? '', limit: warning.limit ?? 0, chars: warning.chars ?? 0 })}</div>)}
            {(plan.skipped ?? []).map((item) => <div key={item.target} className="ss-note warn">{item.target}: {t(`memory.skipped.${item.reason}`, undefined, item.reason)}</div>)}
            {plan.changes.length === 0 && <p>{t('memory.noChanges')}</p>}
            {plan.changes.map((change) => <div key={change.path} className="flex min-w-0 flex-col gap-2">
              <h3 className="break-all font-mono text-[13px] font-semibold">{change.path}</h3>
              <div className="grid min-w-0 grid-cols-1 gap-4 md:grid-cols-2">
                <div className="min-w-0"><p className="mb-2 text-[13px]">{t('memory.before')}</p><CodeView content={change.before} lang="md" className="max-h-[40vh]" /></div>
                <div className="min-w-0"><p className="mb-2 text-[13px]">{t('memory.after')}</p><CodeView content={change.after} lang="md" className="max-h-[40vh]" /></div>
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
