import { useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Info } from 'lucide-react';
import { api } from '../../api/client';
import type { FileBackupVersion, FileBackupVersions } from '../../api/client';
import Button from '../Button';
import DialogHeader from '../DialogHeader';
import DialogShell from '../DialogShell';
import Spinner from '../Spinner';
import { formatDateTime, useI18n } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import { queryKeys } from '../../lib/queryKeys';
import { lineDiff } from '../instructions/instructionsView';
import { fileBackupErrorMessage, reasonKey } from './backupView';
import Tabs, { TabButton } from '../Tabs';

/** Shows what a file goes back to, then restores that version. */
export default function FileRestoreDialog({
  path,
  current,
  version,
  onClose,
  onDone,
}: {
  path: string;
  current: FileBackupVersions['current'];
  version: FileBackupVersion;
  onClose: () => void;
  onDone: () => void;
}) {
  const { t, locale } = useI18n();
  const [tab, setTab] = useState<'diff' | 'full'>('diff');
  // A version that was no file or a link has no text to compare.
  const hasText = !version.none && !version.link_to;
  const { data, error } = useQuery({
    queryKey: queryKeys.fileBackups.version(path, version.id),
    queryFn: () => api.getFileBackupVersion(path, version.id),
    enabled: hasText,
    gcTime: 0,
  });
  const unlink = Boolean(current.link_to);
  const run = useMutation({
    mutationFn: () =>
      api.restoreFileBackup({ path, id: version.id, ...(unlink ? { unlink: true } : {}) }),
    onSuccess: onDone,
  });

  const short = shortenHome(path);
  const title = t('backup.files.restore.title', { path: short });
  // A link is cut rather than saved; a regular file is saved first; a version that was no file deletes it.
  const note = [
    unlink
      ? t('backup.files.restore.link', { dest: shortenHome(current.link_to ?? '') })
      : current.exists
        ? t('backup.files.restore.saveFirst')
        : '',
    version.none && current.exists ? t('backup.files.restore.removes') : '',
  ]
    .filter(Boolean)
    .join(' ');
  const lines = data ? data.content.replace(/\n$/, '').split('\n') : [];

  return (
    <DialogShell
      open
      onClose={onClose}
      padding="none"
      preventClose={run.isPending}
      ariaLabel={title}
      className="!max-w-[720px]"
    >
      <DialogHeader
        title={title}
        subtitle={t('backup.files.restore.subtitle', {
          time: formatDateTime(version.time, locale, {
            dateStyle: 'medium',
            timeStyle: 'short',
          }),
        })}
        onClose={onClose}
        closeDisabled={run.isPending}
      />
      <div className="db flex flex-col gap-3">
        {!hasText ? (
          <p className="text-[13px]">
            {t(reasonKey(version), { dest: shortenHome(version.link_to ?? '') })}
          </p>
        ) : error ? (
          <div className="ss-note bad">
            <span className="flex-1">{fileBackupErrorMessage(error, t)}</span>
          </div>
        ) : !data ? (
          <Spinner className="self-center" />
        ) : (
          <>
            <Tabs role="tablist">
              {(['diff', 'full'] as const).map((k) => (
                <TabButton key={k} on={tab === k} onClick={() => setTab(k)}>
                  {t(`backup.files.restore.tab.${k}`)}
                </TabButton>
              ))}
            </Tabs>
            <div className="ss-list !shadow-none" role="tabpanel">
              <pre
                className="ss-code !max-h-[320px] !overflow-auto !rounded-none !border-0 !whitespace-pre-wrap"
                style={{ overflowWrap: 'anywhere' }}
              >
                {tab === 'diff'
                  ? lineDiff(data.current, data.content).map((l, i) => (
                      <span key={i} className={l.kind === 'same' ? 'block' : l.kind}>
                        {l.kind === 'add' ? '+ ' : l.kind === 'del' ? '− ' : '  '}
                        {l.text || ' '}
                      </span>
                    ))
                  : lines.map((l, i) => (
                      <span key={i} className="flex">
                        <span className="ln w-6 shrink-0 text-right">{i + 1}</span>
                        <span className="min-w-0 flex-1">{l || ' '}</span>
                      </span>
                    ))}
              </pre>
            </div>
          </>
        )}
        {note && (
          <div className="ss-note inf">
            <Info size={15} />
            <span className="flex-1">{note}</span>
          </div>
        )}
        {run.error && (
          <div className="ss-note bad" role="alert">
            <span className="flex-1">{fileBackupErrorMessage(run.error, t)}</span>
          </div>
        )}
      </div>
      <div className="df">
        <Button variant="ghost" onClick={onClose} disabled={run.isPending}>
          {t('common.cancel')}
        </Button>
        <Button
          variant="primary"
          onClick={() => run.mutate()}
          loading={run.isPending}
          disabled={hasText && !data}
        >
          {t(unlink ? 'backup.files.restore.confirmUnlink' : 'backup.files.restore.confirm')}
        </Button>
      </div>
    </DialogShell>
  );
}
