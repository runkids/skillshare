import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { X } from 'lucide-react';
import { mcpApi } from '../../api/mcp';
import Badge from '../Badge';
import Button from '../Button';
import DialogShell from '../DialogShell';
import IconButton from '../IconButton';
import Spinner from '../Spinner';
import { formatDateTime, useI18n } from '../../i18n';
import AgentIcon from '../AgentIcon';
import { backupTime, dayLabel, describeMessage, groupBackupsByDay, statusVariant } from './mcpView';

interface Props {
  backups: { id: string; target: string; path: string }[];
  onClose: () => void;
  onRestored: (backups: string[]) => void;
}

export default function MCPRestoreDialog({ backups, onClose, onRestored }: Props) {
  const { t, locale } = useI18n();
  const [expanded, setExpanded] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const preview = useQuery({ queryKey: ['mcp-restore-preview', expanded], queryFn: () => mcpApi.previewRestore(expanded), enabled: Boolean(expanded), gcTime: 0 });

  const restore = async () => {
    if (!preview.data) return;
    setBusy(true); setError('');
    try { onRestored((await mcpApi.restore(expanded, preview.data.revision)).backupIds ?? []); }
    catch (e) { setError(e instanceof Error ? e.message : t('common.error.generic')); }
    finally { setBusy(false); }
  };

  return <DialogShell open onClose={onClose} preventClose={busy} maxWidth="2xl" ariaLabel={t('mcp.backups')}>
    <div className="space-y-5">
      <div className="flex items-start justify-between gap-3">
        <div className="space-y-1">
          <h2 className="text-xl font-semibold">{t('mcp.backups')}</h2>
          <p className="text-sm text-pencil-light">{t('mcp.backupsHint')}</p>
        </div>
        <IconButton icon={<X size={16} strokeWidth={2.5} />} label={t('common.close')} disabled={busy} onClick={onClose} />
      </div>
      {error || preview.error ? <p role="alert" className="text-sm text-danger">{error || preview.error?.message}</p> : null}
      <div className="space-y-5 max-h-[65vh] overflow-auto">
        {groupBackupsByDay(backups).map(day => {
          const label = dayLabel(day.date, locale);
          return <section key={label} aria-label={label} className="space-y-2">
            <h3 className="text-xs font-medium text-pencil-light">{label}</h3>
            <ul className="space-y-2">
              {day.backups.map(backup => {
                const open = expanded === backup.id;
                return <li key={backup.id} className={`border rounded-[var(--radius-md)] overflow-hidden ${open ? 'border-pencil ring-1 ring-pencil' : 'border-muted'}`}>
                  <div className="flex flex-wrap items-center gap-3 px-3 py-2.5">
                    <span className="min-w-12 shrink-0 whitespace-nowrap font-semibold tabular-nums">{formatDateTime(backupTime(backup.id), locale, { hour: '2-digit', minute: '2-digit' })}</span>
                    <span className="w-24 inline-flex items-center gap-1.5 font-semibold"><AgentIcon target={backup.target} />{backup.target}</span>
                    <span className="flex-1 min-w-0 font-mono text-xs text-pencil-light truncate" title={backup.path}>{backup.path}</span>
                    <Button size="sm" variant={open ? 'ghost' : 'secondary'} aria-expanded={open} disabled={busy} onClick={() => setExpanded(open ? '' : backup.id)}>
                      {open ? t('mcp.collapse') : t('mcp.previewRestore')}
                    </Button>
                  </div>
                  {open ? <div className="space-y-3 px-3 py-3 bg-paper border-t border-dashed border-pencil-light/30">
                    {preview.isPending ? <Spinner size="sm" /> : preview.data ? <>
                      <p className="text-sm">{t('mcp.restoreHint')}</p>
                      <ul className="space-y-1.5">
                        {preview.data.changes.map(change => <li key={change.name} className="flex flex-wrap items-center gap-2">
                          <Badge size="md" variant={statusVariant[change.action]}>{t(`mcp.status.${change.action}`)}</Badge>
                          <span className="font-medium">{change.name}</span>
                          {change.message ? <span className="text-sm text-pencil-light">{describeMessage(t, change.message)}</span> : null}
                        </li>)}
                      </ul>
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <span className="font-mono text-[11px] text-pencil-light">{t('mcp.backupId')} {backup.id}</span>
                        <Button size="sm" loading={busy} disabled={preview.data.blocked} onClick={restore}>{t('mcp.restoreFile')}</Button>
                      </div>
                    </> : null}
                  </div> : null}
                </li>;
              })}
            </ul>
          </section>;
        })}
      </div>
    </div>
  </DialogShell>;
}
