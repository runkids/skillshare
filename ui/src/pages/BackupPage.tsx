import { useContext, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Archive, ChevronDown, ChevronRight, Copy, Link2, Plus, Trash2 } from 'lucide-react';
import { api } from '../api/client';
import type { BackupInfo, RestoreValidateResponse } from '../api/client';
import AgentIcon from '../components/AgentIcon';
import Button from '../components/Button';
import ConfirmDialog from '../components/ConfirmDialog';
import DialogShell from '../components/DialogShell';
import EmptyState from '../components/EmptyState';
import PageHeader from '../components/PageHeader';
import { Select } from '../components/Select';
import Skeleton from '../components/Skeleton';
import Spinner from '../components/Spinner';
import { useToast } from '../components/Toast';
import FileBackups from '../components/backups/FileBackups';
import HooksBackups from '../components/backups/HooksBackups';
import MCPBackups from '../components/backups/MCPBackups';
import RetentionPopover from '../components/backups/RetentionPopover';
import { backupItem, filterItems, retentionSummary } from '../components/backups/backupView';
import { TargetAgents } from '../components/targetAgents';
import { useAppContext } from '../context/AppContext';
import { formatDateTime, formatRelativeTime, formatSize, useI18n, useT } from '../i18n';
import type { Locale } from '../i18n/locales';
import { shortenHome } from '../lib/paths';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { SettingsTabs } from './SettingsPage';
import { useHooksQuery, useMcpQuery, useOverviewQuery } from '../hooks/useSharedQueries';

const CONFLICTS_SHOWN = 6;
const TABS = ['folders', 'files', 'mcp', 'hooks'] as const;

/** Backups all sit side by side, so any one of them names the folder they share. */
const backupsDir = (path: string) => path.replace(/[/\\][^/\\]+$/, '');

/** Consecutive rows (newest first) under one heading per day: today and yesterday by name, older days by date. */
function dayGroups<T extends { backup: BackupInfo }>(rows: T[], locale: Locale) {
  const groups: { key: string; label: string; sub: string; rows: T[] }[] = [];
  const today = new Date().setHours(0, 0, 0, 0);
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' });
  for (const r of rows) {
    const d = new Date(r.backup.date);
    const key = d.toDateString();
    let g = groups.at(-1);
    if (g?.key !== key) {
      const ago = Math.round((today - new Date(d).setHours(0, 0, 0, 0)) / 86_400_000);
      const date = formatDateTime(d, locale, { dateStyle: 'medium' });
      const named = rtf.format(-ago, 'day');
      g = ago <= 1
        ? { key, label: named.charAt(0).toLocaleUpperCase(locale) + named.slice(1), sub: date, rows: [] }
        : { key, label: date, sub: formatDateTime(d, locale, { weekday: 'long' }), rows: [] };
      groups.push(g);
    }
    g.rows.push(r);
  }
  return groups;
}

export default function BackupPage() {
  const t = useT();
  const { isProjectMode } = useAppContext();
  const [params] = useSearchParams();
  const tab = TABS.find((k) => k === params.get('tab')) ?? 'folders';

  const overview = useOverviewQuery();
  // Each tab reads its own list; the counts in the tab bar share those queries.
  const folders = useQuery({ queryKey: queryKeys.backups, queryFn: () => api.listBackups(), staleTime: staleTimes.backups });
  const files = useQuery({ queryKey: queryKeys.fileBackups.all, queryFn: () => api.listFileBackups() });
  const mcp = useMcpQuery();
  const hooks = useHooksQuery();
  const counts = { folders: folders.data?.backups.length, files: files.data?.files.length, mcp: mcp.data?.backups.length, hooks: hooks.data?.backups.length };
  const create = useCreateBackup();

  return (
    <div className="ss-wrap animate-fade-in">
      <PageHeader
        className="!mb-0"
        title={t('layout.nav.settings')}
        subtitle={`${t(isProjectMode ? 'app.project' : 'app.global')}${overview.data?.configDir ? ` · ${shortenHome(overview.data.configDir)}` : ''}`}
      />
      <SettingsTabs current="backup" />

      <div className="flex items-center gap-4">
        <nav className="ss-seg" aria-label={t('backup.tabs')}>
          {TABS.map((k) => (
            <Link key={k} to={k === 'folders' ? '?' : `?tab=${k}`} replace className={tab === k ? 'on' : ''} aria-current={tab === k ? 'page' : undefined}>
              {t(`backup.tab.${k}`)}
              {counts[k] !== undefined && <span className="ss-cnt">{counts[k]}</span>}
            </Link>
          ))}
        </nav>
        <span className="flex-1" />
        {tab === 'folders' && (
          <Button variant="primary" onClick={() => create.mutate()} loading={create.isPending}><Plus size={15} />{t('backup.actions.backUpNow')}</Button>
        )}
      </div>

      {tab === 'files' ? <FileBackups /> : tab === 'mcp' ? <MCPBackups /> : tab === 'hooks' ? <HooksBackups /> : <FolderBackups creating={create.isPending} />}
    </div>
  );
}

function useCreateBackup() {
  const t = useT();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => api.createBackup(),
    onSuccess: (res) => {
      toast(res.backedUpTargets?.length ? t('backup.toast.backedUp', { count: res.backedUpTargets.length }) : t('backup.toast.nothingToBackUp'), res.backedUpTargets?.length ? 'success' : 'info');
      void queryClient.invalidateQueries({ queryKey: queryKeys.backups });
    },
    onError: (e: Error) => toast(e.message, 'error'),
  });
}

/** Snapshots of whole target folders, taken before each sync. */
function FolderBackups({ creating }: { creating: boolean }) {
  const t = useT();
  const { isProjectMode } = useAppContext();
  const { locale } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const { data, isPending, error } = useQuery({ queryKey: queryKeys.backups, queryFn: () => api.listBackups(), staleTime: staleTimes.backups });
  const [filter, setFilter] = useState('');
  const [cleanupOpen, setCleanupOpen] = useState(false);
  const [deleteAllOpen, setDeleteAllOpen] = useState(false);
  const [deleting, setDeleting] = useState<BackupInfo | null>(null);
  const [open, setOpen] = useState<string | null>(null);
  const [restore, setRestore] = useState<{ backup: BackupInfo; target: string } | null>(null);
  const agentOf = useContext(TargetAgents);

  const backups = data?.backups ?? [];
  const refresh = () => queryClient.invalidateQueries({ queryKey: queryKeys.backups });
  const date = (b: BackupInfo) => formatDateTime(b.date, locale, { dateStyle: 'medium', timeStyle: 'short' });

  const cleanup = useMutation({
    mutationFn: () => api.cleanupBackups(),
    onSuccess: (res) => { toast(t('backup.toast.cleanedUp', { count: res.removed }), 'success'); refresh(); setCleanupOpen(false); },
    onError: (e: Error) => { toast(e.message, 'error'); setCleanupOpen(false); },
  });
  const removeAll = useMutation({
    mutationFn: () => api.deleteAllBackups(),
    onSuccess: (res) => { toast(t('backup.toast.deletedAll', { count: res.removed }), 'success'); refresh(); setDeleteAllOpen(false); },
    onError: (e: Error) => { toast(e.message, 'error'); refresh(); setDeleteAllOpen(false); },
  });
  const limits = useMutation({
    mutationFn: (l: { maxCount: number; maxSizeMB: number }) => api.patchConfig({ backupMaxCount: l.maxCount, backupMaxSizeMB: l.maxSizeMB }),
    onSuccess: () => { toast(t('settings.toast.saved'), 'success'); refresh(); void queryClient.invalidateQueries({ queryKey: queryKeys.config }); },
    onError: (e: Error) => toast(e.message, 'error'),
  });
  const remove = useMutation({
    mutationFn: (b: BackupInfo) => api.deleteBackup(b.timestamp),
    onSuccess: () => { toast(t('backup.toast.deleted'), 'success'); refresh(); setDeleting(null); },
    onError: (e: Error) => { toast(e.message, 'error'); setDeleting(null); },
  });

  const items = backups.flatMap((b) => b.targets.map(backupItem));
  const targets = [...new Set(items.map((i) => i.target))].sort();
  const bothKinds = items.some((i) => i.kind === 'agents') && items.some((i) => i.kind === 'skills');
  const rows = backups.map((b) => ({ backup: b, items: filterItems(b.targets.map(backupItem), filter) })).filter((r) => r.items.length > 0);

  const copyPath = (b: BackupInfo) => { void navigator.clipboard?.writeText(b.path); toast(t('backup.toast.pathCopied'), 'success'); };

  return (
    <>
      {creating && (
        <div className="ss-note" role="status">
          <span className="flex flex-1 flex-col gap-2">
            <span>{t('backup.creating')}</span>
            <span className="ss-bar indet"><span /></span>
          </span>
        </div>
      )}

      {isPending ? (
        <div className="ss-list" role="status">
          <div className="ss-lh gap-2"><Spinner size="sm" />{t('backup.loading')}</div>
          {[0, 1, 2, 3].map((i) => (
            <div key={i} className="ss-r">
              <Skeleton className="h-4 w-[180px]" />
              <Skeleton className="h-4 flex-1" />
            </div>
          ))}
        </div>
      ) : error ? (
        <div className="ss-note bad"><span className="flex-1">{error.message}</span></div>
      ) : backups.length === 0 ? (
        <EmptyState icon={Archive} title={t('backup.empty.title')} description={t('backup.empty.description')} />
      ) : (
        <>
          <div className="flex flex-wrap items-center gap-2">
            {(targets.length > 1 || bothKinds) && (
              <Select
                className="self-start"
                prefix={t('backup.filter.label')}
                chip={{ clearValue: '', clearLabel: t('backup.filter.clear') }}
                value={filter}
                onChange={setFilter}
                options={[
                  { value: '', label: t('backup.filter.all') },
                  ...(targets.length > 1 ? targets.map((name) => ({ value: name, label: name, icon: <AgentIcon target={name} size={14} /> })) : []),
                  ...(bothKinds ? [{ value: 'agents', label: t('backup.filter.agents') }] : []),
                ]}
              />
            )}
            <span className="flex-1" />
            {data?.retention && <RetentionPopover retention={data.retention} editable={!isProjectMode} saving={limits.isPending} onSave={(l) => limits.mutateAsync(l)} />}
            <button type="button" className="ss-btn sm ghost" onClick={() => setCleanupOpen(true)}><Trash2 size={14} />{t('backup.actions.cleanup')}</button>
            <button type="button" className="ss-btn sm ghost !text-bad" onClick={() => setDeleteAllOpen(true)}><Trash2 size={14} />{t('backup.actions.deleteAll')}</button>
          </div>
          <div className="ss-list">
            {rows.length === 0 && <div className="ss-r text-[13px] text-ink-3">{t('backup.filter.empty')}</div>}
            {dayGroups(rows, locale).map((g, gi) => (
              <div key={g.key}>
                <div className="ss-lh !h-auto gap-2 py-2.5" style={gi ? { borderTop: 'var(--sep)' } : undefined}>
                  <span className="font-semibold text-ink">{g.label}</span>
                  <span>{g.sub}</span>
                </div>
                {g.rows.map(({ backup: b, items: shown }, ri) => {
                  const expanded = open === b.timestamp;
                  const entries = new Map((b.entries ?? []).map((e) => [e.name, e]));
                  const icons = [...new Map(shown.map((it) => [agentOf[it.target] ?? it.target, it.target])).values()];
                  return (
                    <div key={b.timestamp} style={ri ? { borderTop: 'var(--sep)' } : undefined}>
                      <button type="button" className="ss-r link w-full text-left" aria-expanded={expanded} onClick={() => setOpen(expanded ? null : b.timestamp)}>
                        <span className="flex w-[120px] shrink-0 flex-col gap-px">
                          <span className="font-mono text-[13px] font-semibold">{formatDateTime(b.date, locale, { timeStyle: 'short' })}</span>
                          <span className="text-xs text-ink-3">{formatRelativeTime(b.date, locale)}</span>
                        </span>
                        <span className="ss-stack shrink-0">
                          {icons.map((target) => <span key={target} className="ss-at"><AgentIcon target={target} size={14} /></span>)}
                        </span>
                        <span className="min-w-0 flex-1 truncate text-[13px] text-ink-2">{shown.map((it) => (it.kind === 'agents' ? `${it.target} agents` : it.target)).join(', ')}</span>
                        <span className="w-[80px] shrink-0 text-right font-mono text-[13px] text-ink-2">{b.sizeBytes > 0 ? formatSize(b.sizeBytes, locale) : '—'}</span>
                        {expanded ? <ChevronDown size={15} className="shrink-0 text-ink-3" /> : <ChevronRight size={15} className="shrink-0 text-ink-3" />}
                      </button>
                      {expanded && (
                        <div className="ss-r fold !flex-col !items-stretch !gap-0 !py-2 !pl-[148px]">
                          {shown.map((it) => {
                            const e = entries.get(it.name);
                            return (
                              <div key={it.name} className="flex items-center gap-2.5 py-1.5">
                                <span className="ss-at"><AgentIcon target={it.target} size={14} /></span>
                                <span className="font-mono text-[13px]">{it.target}</span>
                                <span className="ss-tag">{it.kind}</span>
                                {e && <span className="text-xs text-ink-3">{t(e.files === 1 ? 'backup.item.files.one' : 'backup.item.files.other', { count: e.files, size: formatSize(e.sizeBytes, locale) })}</span>}
                                <span className="flex-1" />
                                <Button variant="secondary" size="sm" onClick={() => setRestore({ backup: b, target: it.name })}>{t('backup.actions.restore')}</Button>
                              </div>
                            );
                          })}
                          <div className="mt-1 flex items-center gap-1 pt-2" style={{ borderTop: 'var(--sep)' }}>
                            <span className="min-w-0 flex-1 truncate font-mono text-xs text-ink-3">{shortenHome(b.path)}</span>
                            <button type="button" className="ss-btn sm ghost" onClick={() => copyPath(b)}><Copy size={14} />{t('common.copyPath')}</button>
                            <button type="button" className="ss-btn sm ghost !text-bad" onClick={() => setDeleting(b)}><Trash2 size={14} />{t('backup.actions.delete')}</button>
                          </div>
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            ))}
          </div>
          <div className="flex items-center gap-3 text-[13px] text-ink-3">
            <span>
              {t(backups.length === 1 ? 'backup.footer.count.one' : 'backup.footer.count.other', { count: backups.length })}
              {data && data.totalSizeBytes > 0 ? ` · ${formatSize(data.totalSizeBytes, locale)}` : ''}
              {` · ${shortenHome(backupsDir(backups[0].path))}`}
            </span>
          </div>
        </>
      )}

      <div className="ss-note">
        <Trash2 size={15} />
        <span className="flex-1">{t('backup.trashNote')}</span>
        <Link to="/skills?tab=trash" className="font-semibold text-ink-2 hover:text-ink">{t('backup.trashNote.skills')}</Link>
        <Link to="/agents?tab=trash" className="font-semibold text-ink-2 hover:text-ink">{t('backup.trashNote.agents')}</Link>
      </div>

      {restore && (
        <RestoreDialog
          backup={restore.backup}
          target={restore.target}
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
        message={t('backup.cleanup.message', { policy: data?.retention ? retentionSummary(t, data.retention, locale) : '' })}
        confirmText={t('backup.cleanup.confirmText')}
        variant="danger"
        loading={cleanup.isPending}
        onConfirm={() => cleanup.mutate()}
        onCancel={() => setCleanupOpen(false)}
      />
      <ConfirmDialog
        open={deleteAllOpen}
        title={t('backup.deleteAll.title', { count: String(backups.length) })}
        message={t('backup.deleteAll.message', { size: formatSize(data?.totalSizeBytes ?? 0, locale) })}
        confirmText={t('backup.deleteAll.confirmText')}
        variant="danger"
        loading={removeAll.isPending}
        onConfirm={() => removeAll.mutate()}
        onCancel={() => setDeleteAllOpen(false)}
      />
      <ConfirmDialog
        open={deleting !== null}
        title={t('backup.delete.title')}
        message={deleting ? t('backup.delete.message', { date: date(deleting) }) : ''}
        confirmText={t('backup.delete.confirmText')}
        variant="danger"
        loading={remove.isPending}
        onConfirm={() => deleting && remove.mutate(deleting)}
        onCancel={() => setDeleting(null)}
      />
    </>
  );
}

function RestoreDialog({ backup, target, onClose, onDone }: {
  backup: BackupInfo;
  target: string;
  onClose: () => void;
  onDone: () => void;
}) {
  const t = useT();
  const { locale } = useI18n();
  const { toast } = useToast();
  const [more, setMore] = useState(false);

  const check = useQuery<RestoreValidateResponse>({
    queryKey: queryKeys.restoreValidate(backup.timestamp, target),
    queryFn: () => api.validateRestore({ timestamp: backup.timestamp, target }),
    staleTime: 0,
  });
  const conflicts = check.data?.conflicts ?? [];
  const run = useMutation({
    mutationFn: () => api.restore({ timestamp: backup.timestamp, target, force: conflicts.length > 0 }),
    onSuccess: () => { toast(t('backup.toast.restored', { target }), 'success'); onDone(); },
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

      <div className="db">
        <div className="ss-r !min-h-0 !px-0">
          <ItemLabel name={target} />
        </div>
        {check.isPending ? (
          <p className="text-[13px] text-ink-3">{t('backup.restore.checkingTarget')}</p>
        ) : check.error ? (
          <div className="ss-note bad"><span className="flex-1">{check.error.message}</span></div>
        ) : conflicts.length > 0 ? (
          <div className="ss-note warn flex-col !items-stretch">
            <span>{t(conflicts.length === 1 ? 'backup.restore.overwriteWarning.one' : 'backup.restore.overwriteWarning.other', { count: conflicts.length })}</span>
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
    </DialogShell>
  );
}

/** A snapshot folder as target and kind: `claude-agents` reads as claude with an agents tag. */
function ItemLabel({ name }: { name: string }) {
  const it = backupItem(name);
  return (
    <>
      <span className="ss-at"><AgentIcon target={it.target} size={16} /></span>
      <span className="nm">{it.target}</span>
      <span className="ss-tag">{it.kind}</span>
      <span className="flex-1" />
    </>
  );
}
