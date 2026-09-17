import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Archive, Link2, Plus, Trash2 } from 'lucide-react';
import { api, type BackupInfo, type RestoreValidateResponse } from '../api/client';
import AgentIcon from '../components/AgentIcon';
import Button from '../components/Button';
import ConfirmDialog from '../components/ConfirmDialog';
import DialogShell from '../components/DialogShell';
import EmptyState from '../components/EmptyState';
import PageHeader from '../components/PageHeader';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { useAppContext } from '../context/AppContext';
import { formatDateTime, formatRelativeTime, formatSize, useI18n, useT } from '../i18n';
import { shortenHome } from '../lib/paths';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { SettingsTabs } from './SettingsPage';

const CONFLICTS_SHOWN = 6;

/** Backups all sit side by side, so any one of them names the folder they share. */
const backupsDir = (path: string) => path.replace(/[/\\][^/\\]+$/, '');

export default function BackupPage() {
  const t = useT();
  const { locale } = useI18n();
  const { isProjectMode } = useAppContext();
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const overview = useQuery({ queryKey: queryKeys.overview, queryFn: () => api.getOverview(), staleTime: staleTimes.overview });
  const { data, isPending, error } = useQuery({ queryKey: queryKeys.backups, queryFn: () => api.listBackups(), staleTime: staleTimes.backups, enabled: !isProjectMode });
  const [cleanupOpen, setCleanupOpen] = useState(false);
  const [restore, setRestore] = useState<{ backup: BackupInfo; target: string | null } | null>(null);

  const backups = data?.backups ?? [];
  const refresh = () => queryClient.invalidateQueries({ queryKey: queryKeys.backups });

  const create = useMutation({
    mutationFn: () => api.createBackup(),
    onSuccess: (res) => {
      toast(res.backedUpTargets?.length ? t('backup.toast.backedUp', { count: res.backedUpTargets.length }) : t('backup.toast.nothingToBackUp'), res.backedUpTargets?.length ? 'success' : 'info');
      refresh();
    },
    onError: (e: Error) => toast(e.message, 'error'),
  });
  const cleanup = useMutation({
    mutationFn: () => api.cleanupBackups(),
    onSuccess: (res) => { toast(t('backup.toast.cleanedUp', { count: res.removed }), 'success'); refresh(); setCleanupOpen(false); },
    onError: (e: Error) => { toast(e.message, 'error'); setCleanupOpen(false); },
  });

  if (isProjectMode) {
    return (
      <div className="ss-wrap animate-fade-in">
        <SettingsHeader />
        <EmptyState icon={Archive} title={t('backup.projectMode.title')} description={t('backup.projectMode.description')} action={<Link to="/" className="ss-btn">{t('common.back')}</Link>} />
      </div>
    );
  }

  return (
    <div className="ss-wrap animate-fade-in">
      <SettingsHeader />

      <div className="flex items-center justify-between gap-6">
        <p className="max-w-[620px] text-[13px] leading-relaxed text-ink-2">{t('backup.intro')}</p>
        <Button variant="primary" onClick={() => create.mutate()} loading={create.isPending}><Plus size={15} />{t('backup.actions.backUpNow')}</Button>
      </div>

      {isPending ? (
        <PageSkeleton />
      ) : error ? (
        <div className="ss-note bad"><span className="flex-1">{error.message}</span></div>
      ) : backups.length === 0 ? (
        <EmptyState icon={Archive} title={t('backup.empty.title')} description={t('backup.empty.description')} />
      ) : (
        <>
          <div className="ss-list">
            <div className="ss-lh">
              <span className="flex-1">{t('backup.col.taken')}</span>
              <span className="w-[180px]">{t('backup.col.targets')}</span>
              <span className="w-[80px] text-right">{t('backup.col.size')}</span>
              <span className="w-[86px]" />
            </div>
            {backups.map((b) => (
              <div key={b.timestamp} className="ss-r">
                <span className="flex min-w-0 flex-1 items-baseline gap-2.5">
                  <span className="nm">{formatDateTime(b.date, locale, { dateStyle: 'medium', timeStyle: 'short' })}</span>
                  <span className="text-[13px] text-ink-3">{formatRelativeTime(b.date, locale)}</span>
                </span>
                <span className="ss-stack flex w-[180px] shrink-0 items-center">
                  {b.targets.map((name) => (
                    <span key={name} className="ss-at" title={name}><AgentIcon target={name} size={14} /></span>
                  ))}
                </span>
                <span className="w-[80px] shrink-0 text-right font-mono text-[13px] text-ink-2">{b.sizeBytes > 0 ? formatSize(b.sizeBytes, locale) : '—'}</span>
                <span className="w-[86px] shrink-0 text-right">
                  <Button variant="secondary" size="sm" onClick={() => setRestore({ backup: b, target: b.targets.length === 1 ? b.targets[0] : null })}>
                    {t('backup.actions.restore')}
                  </Button>
                </span>
              </div>
            ))}
          </div>
          <div className="flex items-center gap-3 text-[13px] text-ink-3">
            <span>
              {t('backup.footer.count', { count: backups.length })}
              {data && data.totalSizeBytes > 0 ? ` · ${formatSize(data.totalSizeBytes, locale)}` : ''}
              {` · ${shortenHome(backupsDir(backups[0].path))}`}
            </span>
            <span className="flex-1" />
            <button type="button" className="ss-btn sm ghost" onClick={() => setCleanupOpen(true)}><Trash2 size={14} />{t('backup.actions.cleanup')}</button>
          </div>
        </>
      )}

      {restore && (
        <RestoreDialog
          backup={restore.backup}
          target={restore.target}
          onPick={(target) => setRestore({ backup: restore.backup, target })}
          onClose={() => setRestore(null)}
          onDone={() => {
            setRestore(null);
            refresh();
            queryClient.invalidateQueries({ queryKey: queryKeys.targets.all });
          }}
        />
      )}

      <ConfirmDialog
        open={cleanupOpen}
        title={t('backup.cleanup.title')}
        message={t('backup.cleanup.message')}
        confirmText={t('backup.cleanup.confirmText')}
        variant="danger"
        loading={cleanup.isPending}
        onConfirm={() => cleanup.mutate()}
        onCancel={() => setCleanupOpen(false)}
      />
    </div>
  );

  function SettingsHeader() {
    return (
      <>
        <PageHeader
          className="!mb-0"
          title={t('layout.nav.settings')}
          subtitle={`${t(isProjectMode ? 'app.project' : 'app.global')}${overview.data?.configDir ? ` · ${shortenHome(overview.data.configDir)}` : ''}`}
        />
        <SettingsTabs current="backup" />
      </>
    );
  }
}

function RestoreDialog({ backup, target, onPick, onClose, onDone }: {
  backup: BackupInfo;
  target: string | null;
  onPick: (target: string) => void;
  onClose: () => void;
  onDone: () => void;
}) {
  const t = useT();
  const { locale } = useI18n();
  const { toast } = useToast();
  const [more, setMore] = useState(false);

  const check = useQuery<RestoreValidateResponse>({
    queryKey: queryKeys.restoreValidate(backup.timestamp, target ?? ''),
    queryFn: () => api.validateRestore({ timestamp: backup.timestamp, target: target! }),
    enabled: target !== null,
    staleTime: 0,
  });
  const conflicts = check.data?.conflicts ?? [];
  const run = useMutation({
    mutationFn: () => api.restore({ timestamp: backup.timestamp, target: target!, force: conflicts.length > 0 }),
    onSuccess: () => { toast(t('backup.toast.restored', { target: target! }), 'success'); onDone(); },
    onError: (e: Error) => toast(e.message, 'error'),
  });

  const taken = formatDateTime(backup.date, locale, { dateStyle: 'medium', timeStyle: 'short' });

  return (
    <DialogShell open onClose={onClose} padding="none" preventClose={run.isPending} ariaLabel={t('backup.restore.title')}>
      <div className="dh">
        <div>
          <h2 className="ss-h2">{t('backup.restore.title')}</h2>
          <p className="mt-1 text-[13px] text-ink-3">{t('backup.restore.taken', { date: taken })}</p>
        </div>
      </div>

      {target === null ? (
        <>
          <div className="db">
            <p className="text-[13px] text-ink-2">{t('backup.restore.pickTarget')}</p>
            <div className="ss-list">
              {backup.targets.map((name) => (
                <button key={name} type="button" className="ss-r link w-full text-left" onClick={() => onPick(name)}>
                  <span className="ss-at"><AgentIcon target={name} size={16} /></span>
                  <span className="nm flex-1">{name}</span>
                </button>
              ))}
            </div>
          </div>
          <div className="df"><Button variant="ghost" onClick={onClose}>{t('common.cancel')}</Button></div>
        </>
      ) : (
        <>
          <div className="db">
            <div className="ss-r !min-h-0 !px-0">
              <span className="ss-at"><AgentIcon target={target} size={16} /></span>
              <span className="nm flex-1">{target}</span>
            </div>
            {check.isPending ? (
              <p className="text-[13px] text-ink-3">{t('backup.restore.checkingTarget')}</p>
            ) : check.error ? (
              <div className="ss-note bad"><span className="flex-1">{check.error.message}</span></div>
            ) : conflicts.length > 0 ? (
              <div className="ss-note warn flex-col !items-stretch">
                <span>{t('backup.restore.overwriteWarning', { count: conflicts.length })}</span>
                <ul className="mt-2 flex flex-col gap-1 font-mono text-[12px]">
                  {(more ? conflicts : conflicts.slice(0, CONFLICTS_SHOWN)).map((f) => <li key={f}>{f}</li>)}
                </ul>
                {!more && conflicts.length > CONFLICTS_SHOWN && (
                  <button type="button" className="ss-btn sm ghost mt-1 self-start !px-0" onClick={() => setMore(true)}>
                    {t('backup.restore.moreConflicts', { count: conflicts.length - CONFLICTS_SHOWN })}
                  </button>
                )}
              </div>
            ) : (
              <p className="text-[13px] text-ink-2">{t('backup.restore.emptyOrMissing')}</p>
            )}
            {check.data?.currentIsSymlink && (
              <div className="ss-note inf"><Link2 size={15} /><span className="flex-1">{t('backup.restore.symlinkNote')}</span></div>
            )}
          </div>
          <div className="df">
            <Button variant="ghost" onClick={onClose} disabled={run.isPending}>{t('common.cancel')}</Button>
            <Button variant="danger" onClick={() => run.mutate()} loading={run.isPending} disabled={check.isPending || !!check.error}>
              {conflicts.length > 0 ? t('backup.actions.restoreOverwrite') : t('backup.actions.restore')}
            </Button>
          </div>
        </>
      )}
    </DialogShell>
  );
}
