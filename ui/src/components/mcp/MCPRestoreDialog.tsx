import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { RotateCcw, X } from 'lucide-react';
import { mcpApi } from '../../api/mcp';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import DialogShell from '../DialogShell';
import Spinner from '../Spinner';
import { formatDateTime, useI18n } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import { backupTime, describeMessage } from './mcpView';

interface Props {
  backups: { id: string; target: string; path: string }[];
  onClose: () => void;
  onRestored: () => void;
}

export default function MCPRestoreDialog({ backups, onClose, onRestored }: Props) {
  const { t, locale } = useI18n();
  const [selected, setSelected] = useState(backups[0]?.id ?? '');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const preview = useQuery({ queryKey: ['mcp-restore-preview', selected], queryFn: () => mcpApi.previewRestore(selected), enabled: Boolean(selected), gcTime: 0, retry: false });
  const conflict = preview.data?.changes.find((c) => c.action === 'conflict');
  const title = t('mcp.backups');

  const restore = async () => {
    if (!preview.data) return;
    setBusy(true);
    setError('');
    try {
      await mcpApi.restore(selected, preview.data.revision);
      onRestored();
    } catch (e) {
      setError((e as Error).message);
      setBusy(false);
    }
  };

  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={busy} ariaLabel={title} className="!max-w-[680px]">
      <div className="dh">
        <div className="flex flex-col gap-1">
          <h2 className="ss-h2">{title}</h2>
          <p className="text-[13px] text-ink-2">{t('mcp.backupsHint')}</p>
        </div>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={busy}><X size={16} /></button>
      </div>
      <div className="db">
        <div role="radiogroup" aria-label={title} className="ss-list max-h-[40vh] overflow-auto !shadow-none">
          {backups.map((b) => {
            const on = b.id === selected;
            return (
              <button
                key={b.id}
                type="button"
                role="radio"
                aria-checked={on}
                className={`ss-r w-full !min-h-[46px] text-left ${on ? 'sel' : ''}`}
                onClick={() => setSelected(b.id)}
                disabled={busy}
              >
                <span className={`ss-chk rad ${on ? 'on' : ''}`} />
                <span className="ss-at"><AgentIcon target={b.target} size={17} /></span>
                <span className="flex min-w-0 flex-1 flex-col gap-px">
                  <span className="truncate font-mono text-[13px]" title={b.path}>{shortenHome(b.path)}</span>
                  <span className={`text-xs ${on && conflict ? 'text-warn' : 'text-ink-3'}`}>
                    {formatDateTime(backupTime(b.id), locale, { weekday: 'short', day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })}
                    {on && conflict && ` · ${describeMessage(t, conflict.message)}`}
                  </span>
                </span>
              </button>
            );
          })}
        </div>
        <div className="ss-fld">
          <span className="text-[13px] font-semibold">{t('mcp.restorePreview')}</span>
          {preview.isPending ? (
            <Spinner size="sm" />
          ) : preview.error || error ? (
            <div className="ss-note bad" role="alert"><span className="flex-1">{error || preview.error?.message}</span></div>
          ) : (
            <div className="ss-code">
              {preview.data?.changes.map((c) => (
                <span key={c.name} className={`block ${c.action === 'conflict' ? 'del' : ''}`}>{`${c.action.padEnd(9)} ${c.name}`}</span>
              ))}
            </div>
          )}
          <span className="hp">{t('mcp.restoreHint')}</span>
        </div>
      </div>
      <div className="df">
        <span className="flex-1" />
        <Button variant="ghost" onClick={onClose} disabled={busy}>{t('common.close')}</Button>
        <Button variant="primary" loading={busy} disabled={!preview.data || preview.data.blocked} onClick={restore}>
          <RotateCcw size={15} />
          {t('mcp.restoreFile')}
        </Button>
      </div>
    </DialogShell>
  );
}
