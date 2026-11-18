import { useState } from 'react';
import type { ReactNode } from 'react';
import { useQuery, useQueryClient, type QueryKey } from '@tanstack/react-query';
import { AlertCircle, X } from 'lucide-react';
import { ApiError } from '../../api/client';
import { invalidate } from '../../lib/queryEvents';
import { useT } from '../../i18n';
import Button from '../Button';
import DialogShell from '../DialogShell';

/**
 * The reviewed write of a Pi or Oh My Pi target's extension settings: preview once, apply only that
 * plan's revision, and send the user back to review again when the server says the plan is stale.
 * How a preview is asked for (with or without the view's revision) is the target's own protocol.
 */
export default function ExtensionsReviewDialog<Plan extends { revision: string }>({
  kind,
  name,
  text,
  previewKey,
  preview: fetchPreview,
  apply: applyPlan,
  children,
  onClose,
  onApplied,
}: {
  kind: 'pi' | 'omp';
  name: string;
  text: {
    title: string;
    subtitle: string;
    loading: string;
    apply: string;
    reviewAgain: string;
    stale: string;
    busy: string;
  };
  previewKey: QueryKey;
  preview: () => Promise<Plan>;
  apply: (revision: string) => Promise<Plan>;
  /** What the plan would write, above the failure note. */
  children: (plan: Plan) => ReactNode;
  onClose: () => void;
  onApplied: (plan: Plan) => void;
}) {
  const t = useT();
  const queryClient = useQueryClient();
  const preview = useQuery({
    queryKey: previewKey,
    queryFn: fetchPreview,
    retry: false,
    gcTime: 0,
    staleTime: Infinity,
  });
  const [applying, setApplying] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const plan = preview.data;
  const apply = async () => {
    if (!plan) return;
    setApplying(true);
    setError(null);
    try {
      const done = await applyPlan(plan.revision);
      await invalidate(queryClient, `${kind}ExtensionsChanged`, name);
      onApplied(done);
    } catch (err) {
      setError(err as Error);
      setApplying(false);
    }
  };
  const failure = error ?? preview.error;
  const code = failure instanceof ApiError ? failure.code : undefined;
  const stale = code === `${kind}_extensions_stale`;
  const busy = code === `${kind}_extensions_busy`;
  return (
    <DialogShell
      open
      onClose={onClose}
      padding="none"
      preventClose={applying}
      ariaLabel={text.title}
      className="!max-w-[680px]"
    >
      <div className="dh">
        <div className="flex flex-col gap-1">
          <h2 className="ss-h2">{text.title}</h2>
          <p className="text-[13px] text-ink-2">{text.subtitle}</p>
        </div>
        <button
          type="button"
          className="ss-ib"
          aria-label={t('common.close')}
          onClick={onClose}
          disabled={applying}
        >
          <X size={16} />
        </button>
      </div>
      <div className="db !gap-3 text-[13.5px]">
        {!plan && !failure && <p className="text-ink-3">{text.loading}</p>}
        {plan && children(plan)}
        {failure && (
          <div className={`ss-note ${stale || busy ? 'warn' : 'bad'}`} role="alert">
            <AlertCircle size={16} />
            <span className="flex-1">
              {stale ? text.stale : busy ? text.busy : failure.message}
            </span>
          </div>
        )}
      </div>
      <div className="df">
        <Button variant="ghost" onClick={onClose} disabled={applying}>
          {t('common.cancel')}
        </Button>
        {stale ? (
          <Button
            variant="secondary"
            onClick={() => {
              void invalidate(queryClient, `${kind}ExtensionsStale`, name);
              onClose();
            }}
          >
            {text.reviewAgain}
          </Button>
        ) : (
          <Button
            variant="primary"
            onClick={apply}
            loading={applying}
            disabled={!plan || Boolean(preview.error)}
          >
            {text.apply}
          </Button>
        )}
      </div>
    </DialogShell>
  );
}
