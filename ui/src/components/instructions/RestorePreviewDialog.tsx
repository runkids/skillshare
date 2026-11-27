import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { X } from 'lucide-react';
import { api } from '../../api/client';
import Button from '../Button';
import DialogShell from '../DialogShell';
import Spinner from '../Spinner';
import { useT, plural } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import { queryKeys } from '../../lib/queryKeys';
import { instructionsErrorMessage, formatSize, lineCount, lineDiff } from './instructionsView';
import Tabs, { TabButton } from '../Tabs';

/** Restore one target, or remove one other location: shows what its file goes back to before taking the shared file off it. */
export default function RestorePreviewDialog({
  name,
  target,
  label,
  mode,
  busy,
  onConfirm,
  onClose,
  location = false,
}: {
  name: string;
  /** A target's name, or with location the location's folder as stored. */
  target: string;
  /** How the target gets the file: an import target only loses the import line. */
  mode: string;
  /** The target as the row names it (Codex for a tool that is not a target). */
  label: string;
  busy: boolean;
  onConfirm: () => void;
  onClose: () => void;
  /** target is another location: removing it also takes it out of the list. */
  location?: boolean;
}) {
  const t = useT();
  const [tab, setTab] = useState<'content' | 'diff'>('content');
  const { data, error } = useQuery({
    queryKey: location
      ? queryKeys.instructions.locationRestorePreview(name, target)
      : queryKeys.instructions.restorePreview(name, target),
    queryFn: () =>
      location
        ? api.getLocationRestorePreview(name, target)
        : api.getSharedRestorePreview(name, target),
    gcTime: 0,
  });
  const title = t(
    location ? 'instructions.locations.removeTitle' : 'instructions.restorePreview.title',
    { name, target: label },
  );
  const lines = data ? data.content.replace(/\n$/, '').split('\n') : [];
  // The record behind the preview is kept per target file, not per shared file, so
  // its time can be another shared file's; the line says what happens, without a time.
  const imported = mode === 'import' && data?.kind === 'content';

  return (
    <DialogShell
      open
      onClose={onClose}
      padding="none"
      preventClose={busy}
      ariaLabel={title}
      className="!max-w-[720px]"
    >
      <div className="dh">
        <div className="flex min-w-0 flex-col gap-1">
          <h2 className="ss-h2">{title}</h2>
          {data && (
            <p className="text-[13px] text-ink-2">
              {t(
                imported
                  ? 'instructions.restorePreview.import'
                  : 'instructions.restorePreview.whenUnknown',
                { path: shortenHome(data.path), name },
              )}
            </p>
          )}
        </div>
        <button
          type="button"
          className="ss-ib"
          aria-label={t('common.close')}
          onClick={onClose}
          disabled={busy}
        >
          <X size={16} />
        </button>
      </div>
      <div className="db flex flex-col gap-3">
        {error ? (
          <div className="ss-note bad">
            <span className="flex-1">{instructionsErrorMessage(error, t)}</span>
          </div>
        ) : !data ? (
          <Spinner className="self-center" />
        ) : data.kind === 'delete' ? (
          <p className="text-[13px]">{t('instructions.restorePreview.delete')}</p>
        ) : data.kind === 'link' ? (
          <p className="text-[13px]">
            {t('instructions.restorePreview.link')}{' '}
            <span className="font-mono">{shortenHome(data.link_to ?? '')}</span>
          </p>
        ) : (
          <>
            <Tabs role="tablist">
              {(['content', 'diff'] as const).map((k) => (
                <TabButton key={k} on={tab === k} onClick={() => setTab(k)}>
                  {t(`instructions.restorePreview.tab.${k}`)}
                </TabButton>
              ))}
            </Tabs>
            <div className="ss-list !shadow-none" role="tabpanel">
              <div className="ss-lh !normal-case">
                <span className="flex-1 text-[12.5px]">
                  {tab === 'content'
                    ? t(plural('instructions.preview.stats', lineCount(data.content)), {
                        lines: lineCount(data.content),
                        size: formatSize(new TextEncoder().encode(data.content).length),
                      })
                    : shortenHome(data.path)}
                </span>
              </div>
              {tab === 'content' ? (
                <pre
                  className="ss-code !max-h-[320px] !overflow-auto !rounded-none !border-0 !whitespace-pre-wrap"
                  style={{ overflowWrap: 'anywhere' }}
                >
                  {/* Long lines wrap beside their number. */}
                  {lines.map((l, i) => (
                    <span key={i} className="flex">
                      <span className="ln w-6 shrink-0 text-right">{i + 1}</span>
                      <span className="min-w-0 flex-1">{l || ' '}</span>
                    </span>
                  ))}
                </pre>
              ) : (
                <pre
                  className="ss-code !max-h-[320px] !overflow-auto !rounded-none !border-0 !whitespace-pre-wrap"
                  style={{ overflowWrap: 'anywhere' }}
                >
                  {lineDiff(data.current, data.content).map((l, i) => (
                    <span key={i} className={l.kind === 'same' ? 'block' : l.kind}>
                      {l.kind === 'add' ? '+ ' : l.kind === 'del' ? '− ' : '  '}
                      {l.text || ' '}
                    </span>
                  ))}
                </pre>
              )}
            </div>
          </>
        )}
        {data?.drift && !imported && (
          <p className="text-[12.5px] text-ink-2">
            {t('instructions.restorePreview.drift', { target: label })}
          </p>
        )}
      </div>
      <div className="df">
        <Button variant="ghost" onClick={onClose} disabled={busy}>
          {t('common.cancel')}
        </Button>
        <Button
          variant={location ? 'danger' : 'primary'}
          onClick={onConfirm}
          loading={busy}
          disabled={!data}
        >
          {t(location ? 'instructions.locations.removeConfirm' : 'instructions.detail.restore')}
        </Button>
      </div>
    </DialogShell>
  );
}
