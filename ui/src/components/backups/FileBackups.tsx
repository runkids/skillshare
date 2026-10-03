import { useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { FileText } from 'lucide-react';
import { api } from '../../api/client';
import type { FileBackup, FileBackupVersion } from '../../api/client';
import AgentIcon from '../AgentIcon';
import Button from '../Button';
import EmptyState from '../EmptyState';
import { PageSkeleton } from '../Skeleton';
import Spinner from '../Spinner';
import { useToast } from '../Toast';
import { formatDateTime, formatSize, useI18n } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import { queryKeys } from '../../lib/queryKeys';
import { fileBackupErrorMessage, kindTone, reasonKey } from './backupView';
import FileRestoreDialog from './FileRestoreDialog';

function comparablePath(path: string) {
  // Preserve literal backslashes in POSIX names while accepting Windows history links.
  return /^[A-Za-z]:[\\/]|^\\\\/.test(path) ? path.replace(/\\/g, '/') : path;
}

/** Earlier versions of single files skillshare rewrote: files on the left, one file's versions on the right. */
export default function FileBackups() {
  const { t } = useI18n();
  const { data, isPending, error } = useQuery({ queryKey: queryKeys.fileBackups.all, queryFn: () => api.listFileBackups() });
  const [picked, setPicked] = useState<string | null>(null);
  const [params] = useSearchParams();

  const files = data?.files ?? [];
  const requested = picked ?? params.get('path');
  const selected = requested === null ? files[0] : files.find((f) => comparablePath(f.path) === comparablePath(requested));

  return (
    <>
      {isPending ? (
        <PageSkeleton />
      ) : error ? (
        <div className="ss-note bad"><span className="flex-1">{fileBackupErrorMessage(error, t)}</span></div>
      ) : !selected ? (
        <EmptyState icon={FileText} title={t('backup.files.empty.title')} description={t('backup.files.empty.description')} />
      ) : (
        <div className="grid grid-cols-[360px_minmax(0,1fr)] items-start gap-5">
          <div className="ss-list" role="list" aria-label={t('backup.files.listLabel')}>
            {files.map((f) => (
              <button
                key={f.path}
                type="button"
                role="listitem"
                aria-current={f === selected}
                className={`ss-r link w-full text-left ${f === selected ? 'sel' : ''}`}
                onClick={() => setPicked(f.path)}
              >
                <span className="ss-at">{f.target ? <AgentIcon target={f.target} size={16} /> : <FileText size={15} />}</span>
                <span className="flex min-w-0 flex-1 flex-col gap-px">
                  <span className="truncate font-mono text-[13px] font-semibold" title={f.path}>{shortenHome(f.path)}</span>
                  <span className="truncate text-xs text-ink-3">{ownerLine(f)}</span>
                </span>
                <span className="ss-cnt">{f.versions}</span>
              </button>
            ))}
          </div>
          <Versions key={selected.path} file={selected} />
        </div>
      )}
    </>
  );

  function ownerLine(f: FileBackup) {
    const shared = f.extra ? t('backup.files.owner.shared', { name: f.extra }) : '';
    if (f.source) return shared;
    return [f.target ?? t('backup.files.owner.other'), shared].filter(Boolean).join(' · ');
  }
}

function Versions({ file }: { file: FileBackup }) {
  const { t, locale } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const { data, error } = useQuery({ queryKey: queryKeys.fileBackups.versions(file.path), queryFn: () => api.getFileBackupVersions(file.path) });
  const [restoring, setRestoring] = useState<FileBackupVersion | null>(null);

  return (
    <div className="ss-list">
      <div className="ss-gh !min-h-[52px]">
        <span className="flex min-w-0 flex-1 flex-col gap-px">
          <span className="truncate font-mono font-semibold" title={file.path}>{shortenHome(file.path)}</span>
          <span className="text-xs text-ink-3">{t((data?.versions.length ?? file.versions) === 1 ? 'backup.files.versionsCount.one' : 'backup.files.versionsCount.other', { count: data?.versions.length ?? file.versions })}</span>
        </span>
        {file.extra && (
          <Link to={`/extras?tab=instructions&file=${encodeURIComponent(file.extra)}`} className="ss-btn sm ghost">{t('backup.files.toShared')}</Link>
        )}
      </div>
      {error ? (
        <div className="p-4"><div className="ss-note bad"><span className="flex-1">{fileBackupErrorMessage(error, t)}</span></div></div>
      ) : !data ? (
        <div className="flex justify-center p-6"><Spinner /></div>
      ) : (
        data.versions.map((v) => (
          <div key={v.id} className="ss-r !items-start gap-3 !py-3">
            <span className={`ss-st mt-[3px] ${kindTone[v.kind]}`} aria-hidden="true" />
            <span className="flex min-w-0 flex-1 flex-col gap-1">
              <span className="flex items-center gap-2">
                <span className="text-[13px] font-semibold">{formatDateTime(v.time, locale, { dateStyle: 'medium', timeStyle: 'short' })}</span>
                <span className={`ss-tag ${kindTone[v.kind]}`}>{t(`backup.files.kind.${v.kind}`)}</span>
                {!v.none && !v.link_to && <span className="font-mono text-xs text-ink-3">{formatSize(v.size, locale)}</span>}
              </span>
              {/* An old backup has no reason; its pill already says it all. */}
              {(v.kind !== 'history' || v.reason) && <span className="text-[13px] text-ink-2">{t(reasonKey(v), { dest: shortenHome(v.link_to ?? '') })}</span>}
              {v.none ? (
                <span className="text-xs text-ink-3">{t('backup.files.noFile')}</span>
              ) : v.preview && !v.link_to ? (
                <span className="truncate font-mono text-xs text-ink-3">{v.preview}</span>
              ) : null}
            </span>
            <Button variant="secondary" size="sm" onClick={() => setRestoring(v)}>{t('backup.files.previewRestore')}</Button>
          </div>
        ))
      )}
      {restoring && data && (
        <FileRestoreDialog
          path={file.path}
          current={data.current}
          version={restoring}
          onClose={() => setRestoring(null)}
          onDone={() => {
            setRestoring(null);
            toast(t('backup.files.toast.restored', { path: shortenHome(file.path) }), 'success');
            void queryClient.invalidateQueries({ queryKey: queryKeys.fileBackups.all });
            void queryClient.invalidateQueries({ queryKey: queryKeys.instructions.all });
            void queryClient.invalidateQueries({ queryKey: queryKeys.memory.all });
          }}
        />
      )}
    </div>
  );
}
