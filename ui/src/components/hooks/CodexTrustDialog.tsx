import { useState } from 'react';
import { AlertCircle, CircleCheck } from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { hooksApi, type CodexTrustPlan } from '../../api/hooks';
import { useT, plural } from '../../i18n';
import { queryKeys } from '../../lib/queryKeys';
import Button from '../Button';
import DialogHeader from '../DialogHeader';
import DialogShell from '../DialogShell';
import Spinner from '../Spinner';

/**
 * Show each Codex hook's full command, then record trust for exactly that list. A change since the preview
 * is refused, not trusted: close and reopen for a fresh list.
 */
export default function CodexTrustDialog({ onClose }: { onClose: () => void }) {
  const t = useT();
  const {
    data: plan,
    error: previewError,
    isPending,
  } = useQuery({
    queryKey: queryKeys.hooksCodexTrustPreview(),
    queryFn: hooksApi.previewCodexTrust,
    gcTime: 0,
    retry: false,
  });
  const [running, setRunning] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState<CodexTrustPlan | null>(null);
  const needed = plan?.hooks.filter((h) => h.action === 'needed') ?? [];
  const unchanged = plan ? plan.hooks.length - needed.length : 0;

  const trust = async () => {
    if (!plan) return;
    setRunning(true);
    setError('');
    try {
      const applied = await hooksApi.trustCodex(plan.revision);
      setResult(applied);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setRunning(false);
    }
  };
  const done = result !== null;
  const trusted = result?.hooks.filter((h) => h.action === 'trusted').length ?? 0;
  const reason = (message: string) => {
    if (message.includes('inside the checkout')) return t('hooks.codexTrust.inside');
    if (message.includes('review in Codex /hooks')) return t('hooks.codexTrust.unverified');
    if (message.startsWith('Codex home is unavailable: '))
      return t('hooks.codexTrust.unavailable', {
        home: message.slice('Codex home is unavailable: '.length),
      });
    return message;
  };
  return (
    <DialogShell
      open
      onClose={onClose}
      padding="none"
      maxWidth="2xl"
      preventClose={running}
      ariaLabel={t('hooks.codexTrust.title')}
    >
      <DialogHeader
        title={t('hooks.codexTrust.title')}
        subtitle={
          !done
            ? plan?.project
              ? t('hooks.codexTrust.project', { project: plan.project })
              : t('hooks.codexTrust.subtitle')
            : null
        }
      />
      <div className="db">
        {done && result?.failed?.length ? (
          <div className="ss-note warn" role="status">
            <AlertCircle size={16} />
            <span>
              {t('hooks.codexTrust.partial', {
                done: [
                  ...new Set(
                    result.hooks.filter((h) => h.action === 'trusted').map((h) => h.target),
                  ),
                ].join(', '),
                skipped: result.failed.map((f) => f.target).join(', '),
              })}
            </span>
          </div>
        ) : done ? (
          <div className="ss-note inf">
            <CircleCheck size={16} />
            <span className="flex-1">
              {t(plural('hooks.codexTrust.done', trusted), { count: trusted })}
            </span>
          </div>
        ) : isPending ? (
          <Spinner size="sm" />
        ) : (
          plan && (
            <div className="flex flex-col gap-3">
              {plan.failed?.map((f) => (
                <div key={f.target} className="ss-note warn">
                  <AlertCircle size={16} />
                  <span className="flex-1">
                    {t('hooks.codexTrust.failed', { target: f.target, error: reason(f.error) })}
                  </span>
                </div>
              ))}
              {needed.length === 0 && !plan.failed?.length && (
                <p className="text-[13px] text-ink-2">{t('hooks.codexTrust.none')}</p>
              )}
              {needed.map((h) => (
                <div key={JSON.stringify([h.target, h.key])} className="flex flex-col gap-1">
                  <span className="flex items-baseline gap-1.5 text-[13px]">
                    <span className="min-w-0 break-all font-mono font-semibold text-ink">
                      {h.entry}
                    </span>
                    <span className="shrink-0 text-xs text-ink-2">{h.target}</span>
                    {h.home && <code className="break-all text-xs">{h.home}</code>}
                    {h.disabled && (
                      <span className="shrink-0 font-mono text-xs text-warn">
                        {t('hooks.codexTrust.disabled')}
                      </span>
                    )}
                  </span>
                  <pre className="m-0 whitespace-pre-wrap break-all rounded-[var(--r-inner)] border border-line bg-sunken px-2.5 py-2 font-mono text-xs text-ink">
                    {h.command}
                  </pre>
                </div>
              ))}
              {needed.some((h) => h.disabled) && (
                <p className="text-xs leading-normal text-ink-2">
                  {t('hooks.codexTrust.disabledNote')}
                </p>
              )}
              {unchanged > 0 && (
                <p className="text-xs leading-normal text-ink-2">
                  {t(plural('hooks.codexTrust.unchanged', unchanged), { count: unchanged })}
                </p>
              )}
            </div>
          )
        )}
        {(previewError || error) && (
          <div className="ss-note bad" role="alert">
            <AlertCircle size={16} />
            <span className="flex-1">{previewError?.message ?? error}</span>
          </div>
        )}
      </div>
      <div className="df">
        {done || (plan && needed.length === 0) ? (
          <Button variant="primary" onClick={onClose}>
            {t('syncPreview.closeButton')}
          </Button>
        ) : (
          <>
            <Button variant="secondary" onClick={onClose} disabled={running}>
              {t('syncPreview.cancelButton')}
            </Button>
            {/* A refused write means the hooks moved: close and reopen for a fresh list rather than retry it. */}
            {!error && (
              <Button onClick={() => void trust()} loading={running} disabled={needed.length === 0}>
                {t('hooks.codexTrust.confirm')}
              </Button>
            )}
          </>
        )}
      </div>
    </DialogShell>
  );
}
