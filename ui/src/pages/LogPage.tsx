import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ChevronDown, ChevronRight, ScrollText, Search, Trash2 } from 'lucide-react';
import { api, type LogStatsResponse } from '../api/client';
import ConfirmDialog from '../components/ConfirmDialog';
import CopyButton from '../components/CopyButton';
import EmptyState from '../components/EmptyState';
import PageHeader from '../components/PageHeader';
import Pagination from '../components/Pagination';
import { Select } from '../components/Select';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { useAppContext } from '../context/AppContext';
import { formatDateTime, formatRelativeTime, useI18n, useT } from '../i18n';
import { formatLogDetail } from '../lib/logFormat';
import { shortenHome } from '../lib/paths';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { SettingsTabs } from './SettingsPage';

type LogTab = 'all' | 'ops' | 'audit';

const TABS: LogTab[] = ['all', 'ops', 'audit'];
const STATUSES = ['ok', 'error', 'partial', 'blocked'];
const RANGES = ['', '1h', '24h', '7d', '30d'] as const;
const PAGE_SIZES = [10, 25, 50] as const;
const TONE: Record<string, string> = { ok: 'ok', partial: 'warn', error: 'bad', blocked: 'bad' };

const sinceFor = (range: string) => {
  const hours = { '1h': 1, '24h': 24, '7d': 24 * 7, '30d': 24 * 30 }[range];
  return hours ? new Date(Date.now() - hours * 3600_000).toISOString() : '';
};

const duration = (ms?: number) => (!ms || ms <= 0 ? '' : ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(1)} s`);

/** "targets_total" reads as "targets total" — the log writes whatever the command had on hand. */
const argLabel = (key: string) => key.replace(/_/g, ' ');
const argValue = (value: unknown) => (Array.isArray(value) ? value.join(', ') : typeof value === 'object' && value !== null ? JSON.stringify(value) : String(value));

export default function LogPage() {
  const t = useT();
  const { locale } = useI18n();
  const { toast } = useToast();
  const { isProjectMode } = useAppContext();
  const queryClient = useQueryClient();

  const [tab, setTab] = useState<LogTab>('all');
  const [search, setSearch] = useState('');
  const [cmdFilter, setCmdFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [range, setRange] = useState('');
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState<number>(25);
  const [open, setOpen] = useState<string | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);

  const overview = useQuery({ queryKey: queryKeys.overview, queryFn: () => api.getOverview(), staleTime: staleTimes.overview });

  const filters = useMemo(() => {
    const f: Record<string, string> = {};
    if (cmdFilter) f.cmd = cmdFilter;
    if (statusFilter) f.status = statusFilter;
    const since = sinceFor(range);
    if (since) f.since = since;
    return Object.keys(f).length > 0 ? f : undefined;
  }, [cmdFilter, statusFilter, range]);

  const wantOps = tab !== 'audit';
  const wantAudit = tab !== 'ops';
  const ops = useQuery({ queryKey: queryKeys.log('ops', 100, filters), queryFn: () => api.listLog('ops', 100, filters), enabled: wantOps, staleTime: staleTimes.log });
  const audit = useQuery({ queryKey: queryKeys.log('audit', 100, filters), queryFn: () => api.listLog('audit', 100, filters), enabled: wantAudit, staleTime: staleTimes.log });
  const opsStats = useQuery({ queryKey: queryKeys.logStats('ops', filters), queryFn: () => api.getLogStats('ops', filters), enabled: wantOps, staleTime: staleTimes.log });
  const auditStats = useQuery({ queryKey: queryKeys.logStats('audit', filters), queryFn: () => api.getLogStats('audit', filters), enabled: wantAudit, staleTime: staleTimes.log });

  const entries = useMemo(() => {
    const merged = [...(wantOps ? ops.data?.entries ?? [] : []), ...(wantAudit ? audit.data?.entries ?? [] : [])];
    merged.sort((a, b) => new Date(b.ts).getTime() - new Date(a.ts).getTime());
    const needle = search.trim().toLowerCase();
    if (!needle) return merged;
    return merged.filter((e) => `${e.cmd} ${e.msg ?? ''} ${formatLogDetail(e)}`.toLowerCase().includes(needle));
  }, [wantOps, wantAudit, ops.data, audit.data, search]);

  const stats = useMemo(() => mergeStats(wantOps ? opsStats.data : undefined, wantAudit ? auditStats.data : undefined), [wantOps, wantAudit, opsStats.data, auditStats.data]);
  const commands = useMemo(() => {
    const set = new Set([...(ops.data?.commands ?? []), ...(audit.data?.commands ?? [])]);
    return [...set].sort();
  }, [ops.data?.commands, audit.data?.commands]);

  const clear = useMutation({
    mutationFn: () => (tab === 'all' ? Promise.all([api.clearLog('ops'), api.clearLog('audit')]).then(() => undefined) : api.clearLog(tab).then(() => undefined)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['log'] });
      queryClient.invalidateQueries({ queryKey: ['log-stats'] });
      toast(t('log.toast.cleared'), 'success');
      setConfirmOpen(false);
    },
    onError: (e: Error) => { toast(e.message, 'error'); setConfirmOpen(false); },
  });

  const loading = (wantOps && ops.isPending) || (wantAudit && audit.isPending);
  const filtered = !!filters || !!search.trim();
  const totalPages = Math.max(1, Math.ceil(entries.length / pageSize));
  const start = Math.min(page, totalPages - 1) * pageSize;
  const visible = entries.slice(start, start + pageSize);

  const reset = <T,>(set: (v: T) => void) => (v: T) => { set(v); setPage(0); setOpen(null); };

  return (
    <div className="ss-wrap animate-fade-in">
      <PageHeader
        className="!mb-0"
        title={t('layout.nav.settings')}
        subtitle={`${t(isProjectMode ? 'app.project' : 'app.global')}${overview.data?.configDir ? ` · ${shortenHome(overview.data.configDir)}` : ''}`}
      />
      <SettingsTabs current="log" />

      <div className="flex flex-wrap items-center gap-2.5" data-tour="log-filters">
        <label className="ss-inp w-[210px]">
          <Search size={15} className="text-ink-3" />
          <input value={search} onChange={(e) => reset(setSearch)(e.target.value)} placeholder={t('log.search.placeholder')} aria-label={t('log.search.placeholder')} />
        </label>
        <div className="ss-seg" role="radiogroup" aria-label={t('log.title')}>
          {TABS.map((v) => (
            <button key={v} type="button" role="radio" aria-checked={tab === v} className={tab === v ? 'on' : ''} onClick={() => reset(setTab)(v)}>
              {t(`log.tab.${v}`)}
            </button>
          ))}
        </div>
        <Select
          value={cmdFilter}
          onChange={reset(setCmdFilter)}
          className="w-[170px]"
          options={[{ value: '', label: t('log.filter.anyCommand') }, ...commands.map((c) => ({ value: c, label: c }))]}
        />
        <Select
          value={statusFilter}
          onChange={reset(setStatusFilter)}
          className="w-[150px]"
          options={[{ value: '', label: t('log.filter.anyStatus') }, ...STATUSES.map((s) => ({ value: s, label: s }))]}
        />
        <Select
          value={range}
          onChange={reset(setRange)}
          className="w-[160px]"
          options={RANGES.map((r) => ({ value: r, label: t(`log.time.${r || 'any'}`) }))}
        />
        <span className="flex-1" />
        {entries.length > 0 && (
          <button type="button" className="ss-btn sm ghost" onClick={() => setConfirmOpen(true)}><Trash2 size={14} />{t('log.clear')}</button>
        )}
      </div>

      {loading && entries.length === 0 ? (
        <PageSkeleton />
      ) : entries.length === 0 ? (
        <EmptyState
          icon={ScrollText}
          title={filtered ? t('log.empty.filterTitle') : t('log.empty.noEntriesTitle')}
          description={filtered ? t('log.empty.filterDescription') : t('log.empty.noEntriesDescription')}
        />
      ) : (
        <>
          <div className="ss-list">
            <div className="ss-lh">
              <span className="w-4" />
              <span className="w-[128px]">{t('log.table.time')}</span>
              <span className="w-[110px]">{t('log.table.command')}</span>
              <span className="flex-1">{t('log.table.details')}</span>
              <span className="w-[90px]">{t('log.table.status')}</span>
              <span className="w-[64px] text-right">{t('log.table.duration')}</span>
            </div>
            {visible.map((entry, i) => {
              const id = `${entry.ts}-${entry.cmd}-${start + i}`;
              const expanded = open === id;
              const detail = formatLogDetail(entry);
              return (
                <div key={id}>
                  <button type="button" className="ss-r link w-full text-left" aria-expanded={expanded} onClick={() => setOpen(expanded ? null : id)}>
                    {expanded ? <ChevronDown size={15} className="shrink-0 text-ink-3" /> : <ChevronRight size={15} className="shrink-0 text-ink-3" />}
                    <span className="w-[128px] shrink-0 font-mono text-[12.5px] text-ink-2">{formatDateTime(entry.ts, locale, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}</span>
                    <span className="nm m w-[110px] shrink-0 truncate">{entry.cmd}</span>
                    <span className="min-w-0 flex-1 truncate text-[13px] text-ink-2">{detail || '—'}</span>
                    <span className="w-[90px] shrink-0"><span className={`ss-st ${TONE[entry.status] ?? 'off'}`}>{entry.status}</span></span>
                    <span className="w-[64px] shrink-0 text-right font-mono text-[12.5px] text-ink-3">{duration(entry.ms)}</span>
                  </button>
                  {expanded && (
                    <div className="ss-r fold !items-start !py-3.5 !pl-[43px]">
                      <div className="flex min-w-0 flex-1 flex-col gap-3">
                        {entry.msg && <p className="text-[13px] leading-relaxed text-ink-2">{entry.msg}</p>}
                        {entry.args && Object.keys(entry.args).length > 0 && (
                          <dl className="ss-kv !grid-cols-[130px_minmax(0,1fr)]">
                            {Object.entries(entry.args).map(([k, v]) => (
                              <div key={k} className="contents">
                                <dt>{argLabel(k)}</dt>
                                <dd className="break-words font-mono text-[12.5px]">{argValue(v)}</dd>
                              </div>
                            ))}
                          </dl>
                        )}
                      </div>
                      <CopyButton value={JSON.stringify(entry, null, 2)} size={14} title={t('common.copy')} />
                    </div>
                  )}
                </div>
              );
            })}
          </div>

          <div className="flex flex-col gap-1">
            <Pagination
              page={Math.min(page, totalPages - 1)}
              totalPages={totalPages}
              onPageChange={(p) => { setPage(p); setOpen(null); }}
              rangeText={`${start + 1}–${Math.min(start + pageSize, entries.length)} / ${entries.length}`}
              pageSize={{ value: pageSize, options: PAGE_SIZES, onChange: (s) => { setPageSize(s); setPage(0); } }}
            />
            {stats && stats.total > 0 && (
              <p className="text-[13px] text-ink-3">
                {t('log.summary.entries', { total: stats.total })}
                {` · ${t('log.summary.success', { rate: Math.round(stats.success_rate * 100) })}`}
                {stats.last_operation && ` · ${t('log.summary.lastCommand')} ${stats.last_operation.cmd} ${formatRelativeTime(stats.last_operation.ts, locale)}`}
                {filtered && ` · ${t('log.summary.filtered')}`}
              </p>
            )}
          </div>
        </>
      )}

      <ConfirmDialog
        open={confirmOpen}
        onConfirm={() => clear.mutate()}
        onCancel={() => setConfirmOpen(false)}
        title={t('log.clearConfirm.title')}
        message={t('log.clearConfirm.message', { logType: t(`log.clearConfirm.${tab}`) })}
        confirmText={t('log.clearConfirm.confirmText')}
        variant="danger"
        loading={clear.isPending}
      />
    </div>
  );
}

/** The two logs are separate files; the "All" tab adds their counts up. */
function mergeStats(ops?: LogStatsResponse, audit?: LogStatsResponse): LogStatsResponse | undefined {
  if (!ops) return audit;
  if (!audit) return ops;
  const total = ops.total + audit.total;
  const ok = ops.total * ops.success_rate + audit.total * audit.success_rate;
  const last = !audit.last_operation || (ops.last_operation && new Date(ops.last_operation.ts) > new Date(audit.last_operation.ts)) ? ops.last_operation : audit.last_operation;
  return { total, success_rate: total > 0 ? ok / total : 0, by_command: { ...ops.by_command, ...audit.by_command }, last_operation: last };
}

