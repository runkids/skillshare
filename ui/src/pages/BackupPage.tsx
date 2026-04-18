import { useState, useRef, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useT } from '../i18n';
import {
  Archive,
  Clock,
  RotateCcw,
  Trash2,
  Target,
  Plus,
  RefreshCw,
  ChevronDown,
} from 'lucide-react';
import { api } from '../api/client';
import type { BackupInfo, RestoreValidateResponse } from '../api/client';
import { useAppContext } from '../context/AppContext';
import Spinner from '../components/Spinner';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { formatSize } from '../lib/format';
import Card from '../components/Card';
import Button from '../components/Button';
import PageHeader from '../components/PageHeader';
import Badge from '../components/Badge';
import ConfirmDialog from '../components/ConfirmDialog';
import EmptyState from '../components/EmptyState';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';

function timeAgo(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  const diff = now - then;
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.floor(hrs / 24);
  if (days < 30) return `${days}d ago`;
  return `${Math.floor(days / 30)}mo ago`;
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  });
}

export default function BackupPage() {
  const t = useT();
  const { isProjectMode } = useAppContext();
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const { data, isPending, error } = useQuery({
    queryKey: queryKeys.backups,
    queryFn: () => api.listBackups(),
    staleTime: staleTimes.backups,
  });

  // All hooks must be called before any conditional return
  const [creating, setCreating] = useState(false);
  const [cleanupOpen, setCleanupOpen] = useState(false);
  const [cleaningUp, setCleaningUp] = useState(false);
  const [restoreTarget, setRestoreTarget] = useState<{ timestamp: string; target: string } | null>(null);
  const [restoring, setRestoring] = useState(false);
  const [validation, setValidation] = useState<{
    loading: boolean;
    result: RestoreValidateResponse | null;
  }>({ loading: false, result: null });

  const backups = data?.backups ?? [];

  const handleRefresh = () => {
    queryClient.invalidateQueries({ queryKey: queryKeys.backups });
  };

  const handleCreate = async () => {
    setCreating(true);
    try {
      const res = await api.createBackup();
      if (res.backedUpTargets?.length) {
        toast(t('backup.toast.backedUp', { count: res.backedUpTargets.length }), 'success');
      } else {
        toast(t('backup.toast.nothingToBackUp'), 'info');
      }
      queryClient.invalidateQueries({ queryKey: queryKeys.backups });
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      setCreating(false);
    }
  };

  const handleCleanup = async () => {
    setCleaningUp(true);
    try {
      const res = await api.cleanupBackups();
      toast(t('backup.toast.cleanedUp', { count: res.removed }), 'success');
      queryClient.invalidateQueries({ queryKey: queryKeys.backups });
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      setCleaningUp(false);
      setCleanupOpen(false);
    }
  };

  const openRestoreDialog = async (timestamp: string, target: string) => {
    setRestoreTarget({ timestamp, target });
    setValidation({ loading: true, result: null });
    try {
      const result = await api.validateRestore({ timestamp, target });
      setValidation({ loading: false, result });
    } catch {
      setValidation({ loading: false, result: null });
    }
  };

  const closeRestoreDialog = () => {
    setRestoreTarget(null);
    setValidation({ loading: false, result: null });
  };

  const handleRestore = async () => {
    if (!restoreTarget) return;
    setRestoring(true);
    const needsForce = (validation.result?.conflicts?.length ?? 0) > 0;
    try {
      await api.restore({ ...restoreTarget, force: needsForce });
      toast(t('backup.toast.restored', { target: restoreTarget.target }), 'success');
      queryClient.invalidateQueries({ queryKey: queryKeys.backups });
      queryClient.invalidateQueries({ queryKey: queryKeys.targets.all });
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      setRestoring(false);
      closeRestoreDialog();
    }
  };

  // Project mode guard — after all hooks
  if (isProjectMode) {
    return (
      <div className="animate-fade-in">
        <Card className="text-center py-12">
          <Archive size={40} strokeWidth={2} className="text-pencil-light mx-auto mb-4" />
          <h2 className="text-2xl font-bold text-pencil mb-2">
            {t('backup.projectMode.title')}
          </h2>
          <p className="text-pencil-light mb-4">
            {t('backup.projectMode.description')}
          </p>
          <Link
            to="/"
            className="text-blue hover:underline"
          >
            {t('common.back')}
          </Link>
        </Card>
      </div>
    );
  }

  if (isPending) return <PageSkeleton />;

  if (error) {
    return (
      <Card>
        <p className="text-danger">{error.message}</p>
      </Card>
    );
  }

  return (
    <div className="space-y-5 animate-fade-in">
      <PageHeader
        icon={<Archive size={24} strokeWidth={2.5} />}
        title={t('backup.title')}
        subtitle={t('backup.subtitle')}
        actions={
          <>
            <Button onClick={handleRefresh} variant="secondary" size="sm">
              <RefreshCw size={16} /> {t('backup.actions.refresh')}
            </Button>
            <Button
              variant="primary"
              size="sm"
              onClick={handleCreate}
              disabled={creating}
            >
              {creating ? (
                <><Spinner size="sm" /> {t('backup.actions.creating')}</>
              ) : (
                <><Plus size={16} strokeWidth={2.5} /> {t('backup.actions.createBackup')}</>
              )}
            </Button>
            {backups.length > 0 && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setCleanupOpen(true)}
              >
                <Trash2 size={16} strokeWidth={2.5} /> {t('backup.actions.cleanup')}
              </Button>
            )}
          </>
        }
      />

      {/* Summary line */}
      {backups.length > 0 && (
        <p className="text-sm text-pencil-light">
          {t('backup.summary.backupsOnFile', { count: backups.length, s: backups.length !== 1 ? 's' : '' })}
          {data && data.totalSizeBytes > 0 && ` · ${formatSize(data.totalSizeBytes)}`}
        </p>
      )}

      {/* Content */}
      {backups.length === 0 ? (
        <EmptyState
          icon={Archive}
          title={t('backup.empty.title')}
          description={t('backup.empty.description')}
          action={
            <Button variant="primary" onClick={handleCreate} disabled={creating}>
              <Archive size={16} strokeWidth={2.5} /> {t('backup.actions.createFirstBackup')}
            </Button>
          }
        />
      ) : (
        <div className="space-y-4">
          {backups.map((backup) => (
            <BackupCard
              key={backup.timestamp}
              backup={backup}
              onRestore={(target) =>
                openRestoreDialog(backup.timestamp, target)
              }
            />
          ))}
        </div>
      )}

      {/* Cleanup Dialog */}
      <ConfirmDialog
        open={cleanupOpen}
        title={t('backup.cleanup.title')}
        message={
          <span>
            {t('backup.cleanup.message')}
          </span>
        }
        confirmText={t('backup.cleanup.confirmText')}
        variant="danger"
        loading={cleaningUp}
        onConfirm={handleCleanup}
        onCancel={() => setCleanupOpen(false)}
      />

      {/* Restore Dialog */}
      <ConfirmDialog
        open={restoreTarget !== null}
        title={t('backup.restore.title')}
        wide
        message={
          restoreTarget ? (
            <div className="text-left space-y-3">
              <div className="space-y-1 text-sm">
                <div><strong>{t('backup.card.target')}</strong> {restoreTarget.target}</div>
                <div><strong>{t('backup.card.from')}</strong> <code className="text-xs bg-paper-dark/50 px-1 rounded">{restoreTarget.timestamp}</code></div>
                {validation.result && validation.result.backupSizeBytes > 0 && (
                  <div><strong>{t('backup.card.backupSize')}</strong> {formatSize(validation.result.backupSizeBytes)}</div>
                )}
              </div>

              {validation.loading && (
                <p className="text-pencil-light italic text-sm">{t('backup.restore.checkingTarget')}</p>
              )}

              {validation.result?.currentIsSymlink && (
                <p className="text-blue text-sm">
                  {t('backup.restore.symlinkNote')}
                </p>
              )}

              {(validation.result?.conflicts?.length ?? 0) > 0 && (
                <div className="bg-warning/10 border border-warning/30 rounded p-2 text-sm">
                  <p className="font-medium text-warning mb-1">
                    {t('backup.restore.overwriteWarning', { count: validation.result!.conflicts.length })}
                  </p>
                  <ul className="list-disc list-inside text-pencil-light max-h-24 overflow-y-auto">
                    {validation.result!.conflicts.slice(0, 10).map((f) => (
                      <li key={f}>{f}</li>
                    ))}
                    {validation.result!.conflicts.length > 10 && (
                      <li>...and {validation.result!.conflicts.length - 10} more</li>
                    )}
                  </ul>
                </div>
              )}

              {validation.result && !validation.result.currentIsSymlink && validation.result.conflicts.length === 0 && (
                <p className="text-green text-sm">
                  {t('backup.restore.emptyOrMissing')}
                </p>
              )}
            </div>
          ) : <span />
        }
        confirmText={
          (validation.result?.conflicts?.length ?? 0) > 0
            ? t('backup.actions.restoreOverwrite')
            : t('backup.actions.restore')
        }
        variant="danger"
        loading={restoring || validation.loading}
        onConfirm={handleRestore}
        onCancel={closeRestoreDialog}
      />
    </div>
  );
}

function BackupCard({
  backup,
  onRestore,
}: {
  backup: BackupInfo;
  onRestore: (target: string) => void;
}) {
  const t = useT();
  const [dropdownOpen, setDropdownOpen] = useState(false);
  const [openUpward, setOpenUpward] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const btnRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!dropdownOpen) return;
    const handleClick = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setDropdownOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [dropdownOpen]);

  const toggleDropdown = () => {
    if (!dropdownOpen && btnRef.current) {
      const rect = btnRef.current.getBoundingClientRect();
      const spaceBelow = window.innerHeight - rect.bottom;
      const estimatedHeight = backup.targets.length * 36 + 8;
      setOpenUpward(spaceBelow < estimatedHeight + 8);
    }
    setDropdownOpen((prev) => !prev);
  };

  const hasManyTargets = backup.targets.length > 3;

  return (
    <Card overflow className={dropdownOpen ? 'z-10' : ''}>
      <div className="space-y-3">
        {/* Timestamp row */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 text-pencil">
            <Clock size={16} strokeWidth={2.5} />
            <span className="font-medium">{formatDate(backup.date)}</span>
            <span className="text-sm text-pencil-light">
              {timeAgo(backup.date)}
            </span>
          </div>
          {backup.sizeBytes > 0 && (
            <span className="text-xs text-pencil-light">
              {formatSize(backup.sizeBytes)}
            </span>
          )}
        </div>

        {/* Targets */}
        <div className="flex items-center gap-2 flex-wrap">
          <Target size={14} strokeWidth={2.5} className="text-pencil-light" />
          {backup.targets.map((t) => (
            <Badge key={t} variant="info">{t}</Badge>
          ))}
        </div>

        {/* Actions */}
        <div className="border-t border-dashed border-pencil-light/30 pt-3 flex gap-2 flex-wrap">
          {hasManyTargets ? (
            <div className="relative" ref={dropdownRef}>
              <Button
                ref={btnRef}
                variant="secondary"
                size="sm"
                onClick={toggleDropdown}
              >
                <RotateCcw size={14} strokeWidth={2.5} />
                {t('backup.actions.restoreTarget')}
                <ChevronDown
                  size={14}
                  strokeWidth={2.5}
                  className={`transition-transform duration-150 ${dropdownOpen ? 'rotate-180' : ''}`}
                />
              </Button>
              {dropdownOpen && (
                <div className={`absolute left-0 z-50 min-w-[180px] bg-surface border border-muted rounded-[var(--radius-md)] shadow-md py-1 animate-fade-in ${openUpward ? 'bottom-full mb-1' : 'top-full mt-1'}`}>
                  {backup.targets.map((t) => (
                    <button
                      key={t}
                      className="w-full text-left px-3 py-2 text-sm text-pencil hover:bg-muted/40 transition-colors cursor-pointer flex items-center gap-2"
                      onClick={() => {
                        setDropdownOpen(false);
                        onRestore(t);
                      }}
                    >
                      <RotateCcw size={12} strokeWidth={2.5} className="text-pencil-light" />
                      {t}
                    </button>
                  ))}
                </div>
              )}
            </div>
          ) : (
            backup.targets.map((t) => (
              <Button
                key={t}
                variant="secondary"
                size="sm"
                onClick={() => onRestore(t)}
              >
                <RotateCcw size={14} strokeWidth={2.5} /> Restore {t}
              </Button>
            ))
          )}
        </div>
      </div>
    </Card>
  );
}
