import { useId, useState } from 'react';
import { ChevronDown, ChevronRight } from 'lucide-react';
import CopyButton from '../CopyButton';
import { useT } from '../../i18n';
import { profileReason } from '../profiles/profileView';

/** Keep external diagnostics intact, but do not inline an entire stack trace. */
export default function SyncError({ error }: { error: string }) {
  const t = useT();
  const id = useId();
  const [open, setOpen] = useState(false);
  const lines = profileReason(error, t).trim().split(/\r?\n/);
  const first = lines.find((line) => /^\s*[\w.]*Error:\s/.test(line))?.trim() || lines[0];
  const summary = first.length > 240 ? `${first.slice(0, 240)}…` : first;
  const condensed = summary !== error.trim();
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-1">
      <span className="break-words font-mono text-[12px] text-ink-2">{summary}</span>
      {condensed && (
        <>
          <button type="button" className="ss-btn ghost sm self-start" aria-expanded={open} aria-controls={id} onClick={() => setOpen(!open)}>
            {open ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
            {t(open ? 'sync.error.hideDetails' : 'sync.error.showDetails')}
          </button>
          {open && (
            <div id={id} className="ss-pre min-w-0">
              <div className="mb-1 flex justify-end">
                <CopyButton value={error} unstyled className="ss-btn ghost sm" title={t('common.copy')} label={t('common.copy')} copiedLabel={t('doctor.copied')} errorMessage={t('memory.copyFailed')} />
              </div>
              <pre className="max-h-64 overflow-auto whitespace-pre-wrap break-words" tabIndex={0} aria-label={t('sync.error.showDetails')}>{error}</pre>
            </div>
          )}
        </>
      )}
    </div>
  );
}
