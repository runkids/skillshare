import { useId, useState } from 'react';
import { ChevronDown, X } from 'lucide-react';
import { useT } from '../../i18n';
import { useOverviewQuery } from '../../hooks/useSharedQueries';
import Button from '../Button';
import CopyButton from '../CopyButton';
import DialogShell from '../DialogShell';
import SegmentedControl from '../SegmentedControl';

export default function MemoryTools({ root, disabled }: { root: string; disabled: boolean }) {
  const t = useT();
  const { data: overview } = useOverviewQuery();
  const scopeReady = typeof overview?.isProjectMode === 'boolean' && (!overview.isProjectMode || !!overview.projectRoot);
  const promptId = useId();
  const [marker, setMarker] = useState('');
  const [open, setOpen] = useState(false);
  const [mode, setMode] = useState<'trial' | 'organize' | 'dream'>('trial');
  const [showPrompt, setShowPrompt] = useState(false);
  const copy = (value: string, primary = false, title = t('memory.copyPrompt')) => <CopyButton value={value} title={title} label={t('memory.copyPrompt')}
    copiedLabel={t('memory.copied')} errorMessage={t('memory.copyFailed')} unstyled className={primary ? 'ss-btn pri sm' : 'ss-btn ghost sm'} />;
  const close = () => setOpen(false);
  const steps = mode === 'trial' ? (['save', 'read', 'cleanup'] as const).map((step) => ({
    title: t(`memory.trial.${step}.title`), hint: t(`memory.trial.${step}.hint`),
    // The fresh-session prompt must not disclose the value it asks the agent to retrieve.
    prompt: t(`memory.trial.${step}.prompt`, step === 'read' ? { root } : { root, marker }),
  })) : [`memory.${mode}.paste`, `memory.${mode}.propose`, 'memory.approveStep'].map((key) => ({ title: t(key), hint: '', prompt: '' }));
  const prompt = mode === 'trial' ? steps.map((step, i) => `${i + 1}. ${step.title}\n${step.prompt}`).join('\n\n')
    : `${t('memory.reviewContext', { root, scope: t(overview?.isProjectMode ? 'app.project' : 'app.global'), projectRoot: overview?.projectRoot ?? '—' })}\n\n${t(`memory.${mode}Prompt`, { root })}`;
  return <>
    <Button variant="ghost" size="sm" disabled={disabled || !root || !scopeReady} onClick={() => {
      let value = `ss-${crypto.getRandomValues(new Uint32Array(2)).join('-')}`;
      try {
        const key = `skillshare:memory-trial:${root}`;
        value = sessionStorage.getItem(key) || value;
        sessionStorage.setItem(key, value);
      } catch { /* The trial still works while this page stays open. */ }
      setMarker(value);
      setOpen(true); setMode('trial'); setShowPrompt(false);
    }}>{t('memory.tools')}</Button>
    <DialogShell open={open && !!root && scopeReady} onClose={close} padding="none" maxWidth="xl" ariaLabel={t('memory.tools')}>
      <div className="dh"><h2 className="ss-h2">{t('memory.tools')}</h2><button type="button" className="ss-ib" onClick={close} aria-label={t('common.close')}><X size={16} /></button></div>
      <div className="db !gap-5">
        <SegmentedControl className="self-start" value={mode} onChange={(value) => { setMode(value); setShowPrompt(false); }} options={[
          { value: 'trial', label: t('memory.trial') }, { value: 'organize', label: t('memory.organize') },
          { value: 'dream', label: t('memory.dream') },
        ]} />
        <p className="text-[14px] leading-relaxed text-ink">{t(`memory.${mode}Hint`)}</p>
        <ol className="flex flex-col gap-4">
          {steps.map((step, i) => <li key={step.title} className="flex items-start gap-3">
            <span aria-hidden className="grid size-6 shrink-0 place-items-center rounded-full border border-line bg-sunken text-[12px] font-semibold text-ink-2">{i + 1}</span>
            <div className="min-w-0 flex-1 pt-0.5">
              <h3 className="text-[14px] font-semibold leading-snug">{step.title}</h3>
              {step.hint && <p className="mt-1 text-[13px] leading-relaxed text-ink-2">{step.hint}</p>}
            </div>
            {step.prompt && copy(step.prompt, false, `${t('memory.copyPrompt')}: ${step.title}`)}
          </li>)}
        </ol>
        {showPrompt && <div id={promptId} className="ss-code whitespace-pre-wrap break-words max-h-72 !overflow-y-auto">{prompt}</div>}
      </div>
      <div className="df justify-between">
        <Button variant="ghost" size="sm" aria-expanded={showPrompt} aria-controls={promptId} onClick={() => setShowPrompt(!showPrompt)}>
          <ChevronDown size={14} className={showPrompt ? 'rotate-180' : ''} />{t(showPrompt ? 'memory.hidePrompt' : 'memory.showPrompt')}
        </Button>
        {mode !== 'trial' && <span key={mode}>{copy(prompt, true)}</span>}
      </div>
    </DialogShell>
  </>;
}
