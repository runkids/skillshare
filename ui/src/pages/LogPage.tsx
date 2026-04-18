import { useState, useEffect, useMemo } from 'react';
import { ScrollText, Trash2, RefreshCw } from 'lucide-react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import type { LogEntry, LogStatsResponse } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { useT } from '../i18n';
import Card from '../components/Card';
import Button from '../components/Button';
import PageHeader from '../components/PageHeader';
import ConfirmDialog from '../components/ConfirmDialog';
import EmptyState from '../components/EmptyState';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import Spinner from '../components/Spinner';
import { Select } from '../components/Input';
import SegmentedControl from '../components/SegmentedControl';
import Pagination from '../components/Pagination';

/* ─── Types & Constants ─── */

type LogTab = 'all' | 'ops' | 'audit';
type TimeRange = '' | '1h' | '24h' | '7d' | '30d';

const TIME_RANGES: { label: string; value: TimeRange }[] = [
  { label: 'All', value: '' },
  { label: '1h', value: '1h' },
  { label: '24h', value: '24h' },
  { label: '7d', value: '7d' },
  { label: '30d', value: '30d' },
];

const STATUS_OPTIONS = ['', 'ok', 'error', 'partial', 'blocked'] as const;
const PAGE_SIZES = [10, 25, 50] as const;

/* ─── Helpers ─── */

function timeRangeToSince(range: TimeRange): string {
  if (!range) return '';
  const now = new Date();
  switch (range) {
    case '1h': now.setHours(now.getHours() - 1); break;
    case '24h': now.setHours(now.getHours() - 24); break;
    case '7d': now.setDate(now.getDate() - 7); break;
    case '30d': now.setDate(now.getDate() - 30); break;
  }
  return now.toISOString();
}

function statusColor(status: string): string {
  switch (status) {
    case 'ok': return 'var(--color-success)';
    case 'error': return 'var(--color-danger)';
    case 'partial': return 'var(--color-warning)';
    case 'blocked': return 'var(--color-danger)';
    default: return 'var(--color-pencil-light)';
  }
}

function formatTimestamp(ts: string): string {
  try {
    const d = new Date(ts);
    return d.toLocaleString(undefined, {
      month: 'short', day: 'numeric',
      hour: '2-digit', minute: '2-digit',
    });
  } catch { return ts; }
}

function formatDuration(ms?: number): string {
  if (!ms || ms <= 0) return '';
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

function timeAgo(ts: string): string {
  const diff = Date.now() - new Date(ts).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function asInt(v: unknown): number | undefined {
  if (typeof v === 'number' && Number.isFinite(v)) return Math.trunc(v);
  if (typeof v === 'string') {
    const n = Number.parseInt(v, 10);
    if (Number.isFinite(n)) return n;
  }
  return undefined;
}

function asString(v: unknown): string | undefined {
  if (typeof v === 'string') {
    const s = v.trim();
    return s.length > 0 ? s : undefined;
  }
  if (v == null) return undefined;
  return String(v);
}

function asStringArray(v: unknown): string[] {
  if (Array.isArray(v)) return v.map((it) => String(it).trim()).filter(Boolean);
  const s = asString(v);
  return s ? [s] : [];
}

function summarizeNames(names: string[], limit = 3): string {
  if (names.length <= limit) return names.join(', ');
  return `${names.slice(0, limit).join(', ')} (+${names.length - limit})`;
}

/* ── Detail formatters ─── */

function formatSyncDetail(args: Record<string, any>): string {
  const parts: string[] = [];
  const total = asInt(args.targets_total ?? args.targets);
  if (total != null) parts.push(`targets=${total}`);
  const failed = asInt(args.targets_failed);
  if (failed != null && failed > 0) parts.push(`failed=${failed}`);
  if (args.dry_run === true || args.dry_run === 'true') parts.push('dry-run');
  if (args.force === true || args.force === 'true') parts.push('force');
  const scope = asString(args.scope);
  if (scope) parts.push(`scope=${scope}`);
  return parts.join(', ');
}

function formatAuditDetail(args: Record<string, any>): string {
  const parts: string[] = [];
  const scope = asString(args.scope);
  const name = asString(args.name);
  if (scope === 'single' && name) parts.push(`skill=${name}`);
  else if (scope === 'all') parts.push('all-skills');
  else if (name) parts.push(name);
  const mode = asString(args.mode);
  if (mode) parts.push(`mode=${mode}`);
  const threshold = asString(args.threshold);
  if (threshold) parts.push(`threshold=${threshold.toUpperCase()}`);
  const scanned = asInt(args.scanned);
  if (scanned != null) parts.push(`scanned=${scanned}`);
  const passed = asInt(args.passed);
  if (passed != null) parts.push(`passed=${passed}`);
  const warning = asInt(args.warning);
  if (warning != null && warning > 0) parts.push(`warning=${warning}`);
  const failed = asInt(args.failed);
  if (failed != null && failed > 0) parts.push(`failed=${failed}`);
  const critical = asInt(args.critical) ?? 0;
  const high = asInt(args.high) ?? 0;
  const medium = asInt(args.medium) ?? 0;
  const low = asInt(args.low) ?? 0;
  const info = asInt(args.info) ?? 0;
  if (critical > 0 || high > 0 || medium > 0 || low > 0 || info > 0) {
    parts.push(`sev(c/h/m/l/i)=${critical}/${high}/${medium}/${low}/${info}`);
  }
  const riskScore = asInt(args.risk_score);
  const riskLabel = asString(args.risk_label);
  if (riskScore != null) {
    if (riskLabel) parts.push(`risk=${riskLabel.toUpperCase()}(${riskScore}/100)`);
    else parts.push(`risk=${riskScore}/100`);
  }
  const scanErrors = asInt(args.scan_errors);
  if (scanErrors != null && scanErrors > 0) parts.push(`scan-errors=${scanErrors}`);
  return parts.join(', ');
}

function formatUpdateDetail(args: Record<string, any>): string {
  const parts: string[] = [];
  const mode = asString(args.mode);
  if (mode) parts.push(`mode=${mode}`);
  if (args.all === true || args.all === 'true') parts.push('all');
  const name = asString(args.name);
  if (name) parts.push(name);
  const names = asStringArray(args.names);
  if (names.length > 0) parts.push(summarizeNames(names));
  const threshold = asString(args.threshold);
  if (threshold) parts.push(`threshold=${threshold.toUpperCase()}`);
  if (args.force === true || args.force === 'true') parts.push('force');
  if (args.dry_run === true || args.dry_run === 'true') parts.push('dry-run');
  if (args.skip_audit === true || args.skip_audit === 'true') parts.push('skip-audit');
  if (args.diff === true || args.diff === 'true') parts.push('diff');
  return parts.join(', ');
}

function formatGenericDetail(args: Record<string, any>): string {
  const parts: string[] = [];
  if (args.source) parts.push(String(args.source));
  if (args.name) parts.push(String(args.name));
  if (args.targets) parts.push(`${args.targets} target(s)`);
  if (args.target) parts.push(String(args.target));
  if (args.message) parts.push(String(args.message));
  if (args.summary) parts.push(String(args.summary));
  return parts.join(' ');
}

function formatDetail(entry: LogEntry): string {
  const detail = entry.args
    ? entry.cmd === 'sync'
      ? formatSyncDetail(entry.args)
      : entry.cmd === 'update'
        ? formatUpdateDetail(entry.args)
        : entry.cmd === 'audit'
          ? formatAuditDetail(entry.args)
          : formatGenericDetail(entry.args)
    : '';
  if (entry.msg && detail) return `${detail} — ${entry.msg}`;
  if (entry.msg) return entry.msg;
  return detail;
}

/* ─── LogTable — clean, minimal table ─── */

function LogTable({ entries }: { entries: LogEntry[] }) {
  const t = useT();
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState<number>(10);

  useEffect(() => { setPage(0); }, [entries]);

  const totalPages = Math.max(1, Math.ceil(entries.length / pageSize));
  const start = page * pageSize;
  const visible = entries.slice(start, start + pageSize);

  return (
    <Card>
      <div className="overflow-x-auto">
        <table className="w-full text-left text-sm">
          <thead>
            <tr className="border-b-2 border-dashed border-muted-dark text-pencil-light">
              <th className="pb-3 pr-4 font-medium">{t('log.table.time')}</th>
              <th className="pb-3 pr-4 font-medium">{t('log.table.command')}</th>
              <th className="pb-3 pr-4 font-medium">{t('log.table.details')}</th>
              <th className="pb-3 pr-4 font-medium">{t('log.table.status')}</th>
              <th className="pb-3 font-medium text-right">{t('log.table.duration')}</th>
            </tr>
          </thead>
          <tbody>
            {visible.map((entry, i) => (
              <tr
                key={`${entry.ts}-${entry.cmd}-${start + i}`}
                className="border-b border-dashed border-muted hover:bg-paper-warm/60 transition-colors"
              >
                <td className="py-3 pr-4 text-pencil-light whitespace-nowrap font-mono text-xs">
                  {formatTimestamp(entry.ts)}
                </td>
                <td className="py-3 pr-4 font-medium text-pencil uppercase whitespace-nowrap">
                  {entry.cmd}
                </td>
                <td className="py-3 pr-4 text-pencil-light max-w-md break-words">
                  {formatDetail(entry) || <span className="text-muted-dark">—</span>}
                </td>
                <td className="py-3 pr-4 whitespace-nowrap">
                  <span className="inline-flex items-center gap-1.5">
                    <span
                      className="w-2 h-2 rounded-full shrink-0"
                      style={{ backgroundColor: statusColor(entry.status) }}
                    />
                    <span className="text-pencil-light text-xs">{entry.status}</span>
                  </span>
                </td>
                <td className="py-3 text-pencil-light text-right whitespace-nowrap font-mono text-xs">
                  {formatDuration(entry.ms)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {entries.length > PAGE_SIZES[0] && (
        <Pagination
          page={page}
          totalPages={totalPages}
          onPageChange={setPage}
          rangeText={`${start + 1}–${Math.min(start + pageSize, entries.length)} of ${entries.length}`}
          pageSize={{
            value: pageSize,
            options: PAGE_SIZES,
            onChange: (s) => { setPageSize(s); setPage(0); },
          }}
        />
      )}
    </Card>
  );
}

/* ─── SummaryLine — compact stats in one line ─── */

function SummaryLine({ stats, filtered }: { stats?: LogStatsResponse; filtered: boolean }) {
  const t = useT();
  if (!stats || stats.total === 0) return null;

  const rate = Math.round(stats.success_rate * 100);
  const rateColor = rate >= 90 ? 'var(--color-success)' : rate >= 70 ? 'var(--color-warning)' : 'var(--color-danger)';

  return (
    <p className="text-sm text-pencil-light">
      <span className="font-medium" style={{ color: rateColor }}>{t('log.summary.success', { rate })}</span>
      {' · '}
      {t('log.summary.entries', { total: stats.total })}
      {stats.last_operation && (
        <>
          {' · '}{t('log.summary.lastCommand')}{' '}
          <span className="font-medium text-pencil">{stats.last_operation.cmd}</span>
          {' '}
          {timeAgo(stats.last_operation.ts)}
        </>
      )}
      {filtered && ' · ' + t('log.summary.filtered')}
    </p>
  );
}

/* ─── LogPage ─── */

export default function LogPage() {
  const t = useT();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [tab, setTab] = useState<LogTab>('all');
  const [clearing, setClearing] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [cmdFilter, setCmdFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [timeRange, setTimeRange] = useState<TimeRange>('');

  const filters = useMemo(() => {
    const f: Record<string, string> = {};
    if (cmdFilter) f.cmd = cmdFilter;
    if (statusFilter) f.status = statusFilter;
    const since = timeRangeToSince(timeRange);
    if (since) f.since = since;
    return Object.keys(f).length > 0 ? f : undefined;
  }, [cmdFilter, statusFilter, timeRange]);

  const opsQuery = useQuery({
    queryKey: queryKeys.log('ops', 100, filters),
    queryFn: () => api.listLog('ops', 100, filters),
    enabled: tab === 'all' || tab === 'ops',
    staleTime: staleTimes.log,
  });

  const auditQuery = useQuery({
    queryKey: queryKeys.log('audit', 100, filters),
    queryFn: () => api.listLog('audit', 100, filters),
    enabled: tab === 'all' || tab === 'audit',
    staleTime: staleTimes.log,
  });

  const opsStatsQuery = useQuery({
    queryKey: queryKeys.logStats('ops', filters),
    queryFn: () => api.getLogStats('ops', filters),
    enabled: tab === 'all' || tab === 'ops',
    staleTime: staleTimes.log,
  });

  const auditStatsQuery = useQuery({
    queryKey: queryKeys.logStats('audit', filters),
    queryFn: () => api.getLogStats('audit', filters),
    enabled: tab === 'all' || tab === 'audit',
    staleTime: staleTimes.log,
  });

  const mergedStats = useMemo((): LogStatsResponse | undefined => {
    if (tab === 'ops') return opsStatsQuery.data;
    if (tab === 'audit') return auditStatsQuery.data;
    const ops = opsStatsQuery.data;
    const audit = auditStatsQuery.data;
    if (!ops && !audit) return undefined;
    const byCommand: Record<string, { total: number; ok: number; error: number; partial: number; blocked: number }> = {};
    for (const src of [ops, audit]) {
      if (!src) continue;
      for (const [cmd, cs] of Object.entries(src.by_command)) {
        const existing = byCommand[cmd] ?? { total: 0, ok: 0, error: 0, partial: 0, blocked: 0 };
        existing.total += cs.total;
        existing.ok += cs.ok;
        existing.error += cs.error;
        existing.partial += cs.partial;
        existing.blocked += cs.blocked;
        byCommand[cmd] = existing;
      }
    }
    const total = (ops?.total ?? 0) + (audit?.total ?? 0);
    const okTotal = Object.values(byCommand).reduce((sum, cs) => sum + cs.ok, 0);
    let lastOp = ops?.last_operation;
    if (audit?.last_operation) {
      if (!lastOp || new Date(audit.last_operation.ts).getTime() > new Date(lastOp.ts).getTime()) {
        lastOp = audit.last_operation;
      }
    }
    return { total, success_rate: total > 0 ? okTotal / total : 0, by_command: byCommand, last_operation: lastOp };
  }, [tab, opsStatsQuery.data, auditStatsQuery.data]);

  const opsEntries = opsQuery.data?.entries ?? [];
  const auditEntries = auditQuery.data?.entries ?? [];

  /* Merged timeline for "all" tab — single chronological view */
  const displayEntries = useMemo(() => {
    if (tab === 'ops') return opsEntries;
    if (tab === 'audit') return auditEntries;
    const merged = [...opsEntries, ...auditEntries];
    merged.sort((a, b) => new Date(b.ts).getTime() - new Date(a.ts).getTime());
    return merged;
  }, [tab, opsEntries, auditEntries]);

  const loading = (tab === 'all' || tab === 'ops') && opsQuery.isPending
    || (tab === 'all' || tab === 'audit') && auditQuery.isPending;

  const hasEntries = displayEntries.length > 0;

  const knownCommands = useMemo(() => {
    const cmds = new Set<string>();
    if (opsQuery.data?.commands) for (const cmd of opsQuery.data.commands) cmds.add(cmd);
    if (auditQuery.data?.commands) for (const cmd of auditQuery.data.commands) cmds.add(cmd);
    return Array.from(cmds).sort();
  }, [opsQuery.data?.commands, auditQuery.data?.commands]);

  const handleRefresh = () => {
    queryClient.invalidateQueries({ queryKey: ['log'] });
    queryClient.invalidateQueries({ queryKey: ['log-stats'] });
  };

  const handleClear = async () => {
    setClearing(true);
    try {
      if (tab === 'all') {
        await Promise.all([api.clearLog('ops'), api.clearLog('audit')]);
      } else {
        await api.clearLog(tab);
      }
      queryClient.invalidateQueries({ queryKey: ['log'] });
      queryClient.invalidateQueries({ queryKey: ['log-stats'] });
      toast(t('log.toast.cleared'), 'success');
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      setClearing(false);
      setConfirmOpen(false);
    }
  };

  if (loading && !hasEntries) return <PageSkeleton />;

  const hasFilter = !!filters;

  return (
    <div className="space-y-5 animate-fade-in">
      <PageHeader
        icon={<ScrollText size={24} strokeWidth={2.5} />}
        title={t('log.title')}
        subtitle={t('log.subtitle')}
        actions={
          <>
            <Button onClick={handleRefresh} variant="secondary" size="sm" disabled={loading}>
              {loading ? <Spinner size="sm" /> : <RefreshCw size={16} />}
              {t('log.refresh')}
            </Button>
            {hasEntries && (
              <Button onClick={() => setConfirmOpen(true)} variant="danger" size="sm" disabled={clearing}>
                <Trash2 size={16} />
                {t('log.clear')}
              </Button>
            )}
          </>
        }
      />

      {/* Toolbar: Tabs + Filters in one row */}
      <div data-tour="log-filters" className="flex flex-wrap items-end gap-3">
        <SegmentedControl
          value={tab}
          onChange={setTab}
          options={[
            { value: 'all', label: t('log.tab.all') },
            { value: 'ops', label: t('log.tab.ops') },
            { value: 'audit', label: t('log.tab.audit') },
          ]}
        />
        <div className="w-36">
          <Select
            label={t('log.filter.command')}
            value={cmdFilter}
            onChange={setCmdFilter}
            size="sm"
            options={[
              { value: '', label: t('log.filter.all') },
              ...knownCommands.map((cmd) => ({ value: cmd, label: cmd })),
            ]}
          />
        </div>
        <div className="w-28">
          <Select
            label={t('log.filter.status')}
            value={statusFilter}
            onChange={setStatusFilter}
            size="sm"
            options={STATUS_OPTIONS.map((s) => ({ value: s, label: s || t('log.filter.all') }))}
          />
        </div>
        <div>
          <span className="block text-sm text-pencil-light mb-1">{t('log.filter.time')}</span>
          <SegmentedControl
            value={timeRange}
            onChange={setTimeRange}
            options={TIME_RANGES.map((tr) => ({ value: tr.value, label: tr.value === '' ? t('log.timeRange.all') : tr.label }))}
          />
        </div>
      </div>

      {/* Compact summary */}
      <SummaryLine stats={mergedStats} filtered={hasFilter} />

      {/* Single unified log table */}
      {!hasEntries ? (
        <EmptyState
          icon={ScrollText}
          title={hasFilter ? t('log.empty.filterTitle') : t('log.empty.noEntriesTitle')}
          description={hasFilter
            ? t('log.empty.filterDescription')
            : t('log.empty.noEntriesDescription')}
        />
      ) : (
        <LogTable entries={displayEntries} />
      )}

      <ConfirmDialog
        open={confirmOpen}
        onConfirm={handleClear}
        onCancel={() => setConfirmOpen(false)}
        title={t('log.clearConfirm.title')}
        message={t('log.clearConfirm.message', {
          logType: tab === 'all' ? t('log.clearConfirm.all') : tab === 'audit' ? t('log.clearConfirm.audit') : t('log.clearConfirm.ops'),
        })}
        confirmText={t('log.clearConfirm.confirmText')}
        variant="danger"
        loading={clearing}
      />
    </div>
  );
}
