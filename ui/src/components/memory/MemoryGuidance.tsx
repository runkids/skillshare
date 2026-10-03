import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
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
    <div className="ss-box flex flex-col gap-3 p-5">
      <div className="ss-sec"><h2>{t('memory.useWithAgents')}</h2></div>
      <p className="text-[13px] text-ink-2">{t('memory.connectionHint')}</p>
      {guidance.error && <div className="ss-note bad">{guidance.error.message}</div>}
      {(guidance.data?.targets ?? []).map((target) => <div key={target.name} className="flex flex-wrap items-center gap-2.5 text-[13px]">
        <AgentIcon target={target.name} size={16} />
        <span className="font-mono font-semibold">{target.name}</span>
        <span className={`ss-st ${target.state === 'configured' ? 'ok' : target.state === 'unconfigured' ? 'off' : 'warn'}`}>{t(`memory.connection.${target.state}`)}</span>
        {target.detail && <span className="text-ink-3">{t(`memory.connectionDetail.${target.detail}`, undefined, target.detail)}</span>}
        {target.file && <span className="break-all font-mono text-[12px] text-ink-3">{shortenHome(target.file)}</span>}
      </div>)}
      {initialized && guidance.data?.targets.length === 0 && <p className="text-[13px] text-ink-3">{t('memory.noTargets')}</p>}
      <div className="flex flex-wrap items-center gap-3">
        <Button variant="primary" size="sm" disabled={!initialized || !guidance.data?.targets.length} onClick={() => { setSelected([]); setPlan(null); setOpen(true); }}>{t('memory.connect')}</Button>
        <Tooltip content={<span className="block whitespace-pre-wrap break-words font-mono">{instructions}</span>}>
          <CopyButton value={instructions} title={t('memory.copyInstructions')} label={t('memory.copyInstructions')} copiedLabel={t('memory.copied')} errorMessage={t('memory.copyFailed')} unstyled className="ss-btn ghost sm" />
        </Tooltip>
        <Tooltip content={<span className="block whitespace-pre-wrap break-words font-mono">{prompt}</span>}>
          <CopyButton value={prompt} title={t('memory.check')} label={t('memory.check')} copiedLabel={t('memory.copied')} errorMessage={t('memory.copyFailed')} unstyled className="ss-btn ghost sm" />
        </Tooltip>
        <Link to="?tab=instructions" className="ss-more">{t('memory.openInstructions')}</Link>
      </div>
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
    </div>
  );
}
