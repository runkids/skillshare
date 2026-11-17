import { useState } from 'react';
import { AlertCircle, CircleCheck } from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { hooksApi } from '../../api/hooks';
import { useT, plural } from '../../i18n';
import { queryKeys } from '../../lib/queryKeys';
import Button from '../Button';
import DialogShell from '../DialogShell';
import Spinner from '../Spinner';

/**
 * Show each Codex hook's full command, then record trust for exactly that list. A change since the preview
 * is refused, not trusted: close and reopen for a fresh list.
 */
export default function CodexTrustDialog({ onClose }: { onClose: () => void }) {
  const t = useT();
  const { data: plan, error: previewError, isPending } = useQuery({ queryKey: queryKeys.hooksCodexTrustPreview(), queryFn: hooksApi.previewCodexTrust, gcTime: 0, retry: false });
  const [running, setRunning] = useState(false);
  const [error, setError] = useState('');
  const [trusted, setTrusted] = useState<number | null>(null);
  const needed = plan?.hooks.filter((h) => h.action === 'needed') ?? [];
  const unchanged = plan ? plan.hooks.length - needed.length : 0;

  const trust = async () => {
    if (!plan) return;
    setRunning(true);
    setError('');
    try {
      setTrusted((await hooksApi.trustCodex(plan.revision)).hooks.filter((h) => h.action === 'trusted').length);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setRunning(false);
    }
  };
  const done = trusted !== null;
  return (
    <DialogShell open onClose={onClose} padding="none" maxWidth="2xl" preventClose={running} ariaLabel={t('hooks.codexTrust.title')}>
      <div className="dh">
        <div className="flex flex-col gap-1">
          <h2 className="ss-h2">{t('hooks.codexTrust.title')}</h2>
          {!done && <p className="text-[13px] text-ink-2">{t('hooks.codexTrust.subtitle')}</p>}
        </div>
      </div>
      <div className="db">
        {done ? <div className="ss-note inf"><CircleCheck size={16} /><span className="flex-1">{t(plural('hooks.codexTrust.done', trusted), { count: trusted })}</span></div>
          : isPending ? <Spinner size="sm" />
          : plan && (
            <div className="flex flex-col gap-3">
              {needed.length === 0 && <p className="text-[13px] text-ink-2">{t('hooks.codexTrust.none')}</p>}
              {needed.map((h) => (
                <div key={h.key} className="flex flex-col gap-1">
                  <span className="flex items-baseline gap-1.5 text-[13px]">
                    <span className="min-w-0 break-all font-mono font-semibold text-ink">{h.entry}</span>
                    <span className="shrink-0 text-xs text-ink-2">{h.target}</span>
                    {h.disabled && <span className="shrink-0 font-mono text-xs text-warn">{t('hooks.codexTrust.disabled')}</span>}
                  </span>
                  <pre className="m-0 whitespace-pre-wrap break-all rounded-[10px] border border-line bg-sunken px-2.5 py-2 font-mono text-xs text-ink">{h.command}</pre>
                </div>
              ))}
              {needed.some((h) => h.disabled) && <p className="text-xs leading-normal text-ink-2">{t('hooks.codexTrust.disabledNote')}</p>}
              {unchanged > 0 && <p className="text-xs leading-normal text-ink-2">{t(plural('hooks.codexTrust.unchanged', unchanged), { count: unchanged })}</p>}
            </div>
          )}
        {(previewError || error) && <div className="ss-note bad" role="alert"><AlertCircle size={16} /><span className="flex-1">{previewError?.message ?? error}</span></div>}
      </div>
      <div className="df">
        {done || (plan && needed.length === 0) ? <Button variant="primary" onClick={onClose}>{t('syncPreview.closeButton')}</Button> : (
          <>
            <Button variant="secondary" onClick={onClose} disabled={running}>{t('syncPreview.cancelButton')}</Button>
            {/* A refused write means the hooks moved: close and reopen for a fresh list rather than retry it. */}
            {!error && <Button onClick={() => void trust()} loading={running} disabled={needed.length === 0}>{t('hooks.codexTrust.confirm')}</Button>}
          </>
        )}
      </div>
    </DialogShell>
  );
}
