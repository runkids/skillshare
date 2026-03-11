import { useState, useEffect, useMemo } from 'react';
import { ScrollText, Trash2, RefreshCw, ChevronLeft, ChevronRight, Activity, Clock, Terminal, ShieldCheck } from 'lucide-react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import type { LogEntry, LogStatsResponse } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import Card from '../components/Card';
import HandButton from '../components/HandButton';
import Badge from '../components/Badge';
import ConfirmDialog from '../components/ConfirmDialog';
import EmptyState from '../components/EmptyState';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { HandSelect } from '../components/HandInput';
import { radius, shadows } from '../design';

/* ──────────────────────────────────────────────────────────────────────
 * Constants & helpers
 * ────────────────────────────────────────────────────────────────────── */

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

function statusBadge(status: string) {
  switch (status) {
    case 'ok':
      return <Badge variant="success">ok</Badge>;
    case 'error':
      return <Badge variant="danger">error</Badge>;
    case 'partial':
      return <Badge variant="warning">partial</Badge>;
    case 'blocked':
      return <Badge variant="danger">blocked</Badge>;
    default:
      return <Badge>{status}</Badge>;
  }
}

function formatTimestamp(ts: string): string {
  try {
    const d = new Date(ts);
    return d.toLocaleString(undefined, {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  } catch {
    return ts;
  }
}

function formatDuration(ms?: number): string {
  if (!ms || ms <= 0) return '';
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
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
  if (Array.isArray(v)) {
    return v.map((it) => String(it).trim()).filter(Boolean);
  }
  const s = asString(v);
  return s ? [s] : [];
}

function summarizeNames(names: string[], limit = 3): string {
  if (names.length <= limit) return names.join(', ');
  const shown = names.slice(0, limit).join(', ');
  return `${shown} (+${names.length - limit})`;
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

type AuditSkillLists = {
  failed: string[];
  warning: string[];
  low: string[];
  info: string[];
};

function getAuditSkillLists(entry: LogEntry): AuditSkillLists {
  if (entry.cmd !== 'audit' || !entry.args) {
    return { failed: [], warning: [], low: [], info: [] };
  }
  return {
    failed: asStringArray(entry.args.failed_skills),
    warning: asStringArray(entry.args.warning_skills),
    low: asStringArray(entry.args.low_skills),
    info: asStringArray(entry.args.info_skills),
  };
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
  if (entry.msg && detail) return `${detail} (${entry.msg})`;
  if (entry.msg) return entry.msg;
  return detail;
}

function renderDetail(entry: LogEntry) {
  const summary = formatDetail(entry);
  const lists = getAuditSkillLists(entry);

  if (lists.failed.length === 0 && lists.warning.length === 0 && lists.low.length === 0 && lists.info.length === 0) {
    return summary;
  }

  return (
    <div className="space-y-1">
      <div>{summary}</div>
      {lists.failed.length > 0 && (
        <div className="text-xs text-danger">
          failed skills: {summarizeNames(lists.failed, 6)}
        </div>
      )}
      {lists.warning.length > 0 && (
        <div className="text-xs text-warning">
          warning skills: {summarizeNames(lists.warning, 6)}
        </div>
      )}
      {lists.low.length > 0 && (
        <div className="text-xs text-blue">
          low skills: {summarizeNames(lists.low, 6)}
        </div>
      )}
      {lists.info.length > 0 && (
        <div className="text-xs text-pencil-light">
          info skills: {summarizeNames(lists.info, 6)}
        </div>
      )}
    </div>
  );
}

/* ──────────────────────────────────────────────────────────────────────
 * PillButton — reusable tab/filter button (dark-mode aware)
 * ────────────────────────────────────────────────────────────────────── */

function PillButton({
  active,
  onClick,
  children,
  className = '',
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <button
      onClick={onClick}
      className={`
        px-3 py-1.5 text-sm border-2 transition-all duration-150 cursor-pointer
        ${active
          ? 'bg-pencil text-paper border-pencil font-medium'
          : 'bg-transparent text-pencil border-muted-dark hover:border-pencil'
        }
        ${className}
      `}
      style={{
        borderRadius: radius.sm,
        boxShadow: active ? shadows.sm : 'none',
      }}
    >
      {children}
    </button>
  );
}

/* ──────────────────────────────────────────────────────────────────────
 * LogStatsBar — summary stat cards
 * ────────────────────────────────────────────────────────────────────── */

function LogStatsBar({ stats }: { stats: LogStatsResponse }) {
  const commands = Object.entries(stats.by_command).sort((a, b) => b[1].total - a[1].total);
  const rate = stats.total > 0 ? Math.round(stats.success_rate * 100) : 0;

  return (
    <div
      className="grid gap-3"
      style={{ gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))' }}
    >
      {/* Success rate */}
      <div
        className="flex items-center gap-3 p-3 border-2 border-l-[4px] transition-all"
        style={{
          borderRadius: radius.md,
          borderColor: 'var(--color-muted-dark)',
          borderLeftColor: rate >= 90 ? 'var(--color-success)' : rate >= 70 ? 'var(--color-warning)' : 'var(--color-danger)',
          boxShadow: shadows.sm,
        }}
      >
        <div
          className="w-9 h-9 flex items-center justify-center border-2 shrink-0"
          style={{
            borderRadius: radius.sm,
            borderColor: rate >= 90 ? 'var(--color-success)' : rate >= 70 ? 'var(--color-warning)' : 'var(--color-danger)',
            color: rate >= 90 ? 'var(--color-success)' : rate >= 70 ? 'var(--color-warning)' : 'var(--color-danger)',
          }}
        >
          <Activity size={18} strokeWidth={2.5} />
        </div>
        <div>
          <p className="text-xl font-bold leading-tight" style={{
            color: rate >= 90 ? 'var(--color-success)' : rate >= 70 ? 'var(--color-warning)' : 'var(--color-danger)',
          }}>
            {rate}%
          </p>
          <p className="text-xs text-pencil-light leading-tight">
            {stats.total} total
          </p>
        </div>
      </div>

      {/* Top commands */}
      {commands.slice(0, 4).map(([cmd, cs]) => {
        const hasErrors = cs.error > 0 || cs.blocked > 0;
        return (
          <div
            key={cmd}
            className="flex items-center gap-3 p-3 border-2 border-l-[4px] transition-all"
            style={{
              borderRadius: radius.md,
              borderColor: 'var(--color-muted-dark)',
              borderLeftColor: hasErrors ? 'var(--color-warning)' : 'var(--color-pencil-light)',
              boxShadow: shadows.sm,
            }}
          >
            <div
              className="w-9 h-9 flex items-center justify-center border-2 shrink-0"
              style={{
                borderRadius: radius.sm,
                borderColor: 'var(--color-pencil-light)',
                color: 'var(--color-pencil-light)',
              }}
            >
              {cmd === 'audit' ? <ShieldCheck size={18} strokeWidth={2.5} /> : <Terminal size={18} strokeWidth={2.5} />}
            </div>
            <div>
              <p className="text-lg font-bold text-pencil leading-tight uppercase">
                {cmd}
              </p>
              <div className="flex items-center gap-1.5 text-xs">
                <span className="text-pencil-light">{cs.total}</span>
                {cs.ok > 0 && <span className="text-success">{cs.ok} ok</span>}
                {cs.error > 0 && <span className="text-danger">{cs.error} err</span>}
                {cs.partial > 0 && <span className="text-warning">{cs.partial} partial</span>}
              </div>
            </div>
          </div>
        );
      })}

      {/* Last operation */}
      {stats.last_operation && (
        <div
          className="flex items-center gap-3 p-3 border-2 border-l-[4px] transition-all"
          style={{
            borderRadius: radius.md,
            borderColor: 'var(--color-muted-dark)',
            borderLeftColor: statusColor(stats.last_operation.status),
            boxShadow: shadows.sm,
          }}
        >
          <div
            className="w-9 h-9 flex items-center justify-center border-2 shrink-0"
            style={{
              borderRadius: radius.sm,
              borderColor: 'var(--color-pencil-light)',
              color: 'var(--color-pencil-light)',
            }}
          >
            <Clock size={18} strokeWidth={2.5} />
          </div>
          <div>
            <p className="text-sm font-bold text-pencil uppercase leading-tight">
              {stats.last_operation.cmd}
            </p>
            <div className="text-xs mt-0.5">
              {statusBadge(stats.last_operation.status)}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

/* ──────────────────────────────────────────────────────────────────────
 * LogTable — paginated table with status-colored left stripes
 * ────────────────────────────────────────────────────────────────────── */

const PAGE_SIZES = [10, 25, 50] as const;

function LogTable({ entries }: { entries: LogEntry[] }) {
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState<number>(10);

  useEffect(() => { setPage(0); }, [entries]);

  const totalPages = Math.max(1, Math.ceil(entries.length / pageSize));
  const start = page * pageSize;
  const visible = entries.slice(start, start + pageSize);

  return (
    <Card>
      <div className="overflow-x-auto">
        <table className="w-full text-left">
          <thead>
            <tr className="border-b-2 border-dashed border-muted-dark">
              <th className="pb-3 pr-4 text-pencil-light text-sm font-medium w-0" />
              <th className="pb-3 pr-4 text-pencil-light text-sm font-medium">Time</th>
              <th className="pb-3 pr-4 text-pencil-light text-sm font-medium">Command</th>
              <th className="pb-3 pr-4 text-pencil-light text-sm font-medium">Details</th>
              <th className="pb-3 pr-4 text-pencil-light text-sm font-medium">Status</th>
              <th className="pb-3 text-pencil-light text-sm font-medium text-right">Duration</th>
            </tr>
          </thead>
          <tbody>
            {visible.map((entry, i) => (
              <tr
                key={`${entry.ts}-${entry.cmd}-${start + i}`}
                className="border-b border-dashed border-muted hover:bg-paper-warm/60 transition-colors"
              >
                {/* Status stripe cell */}
                <td className="py-3 pr-0 w-1">
                  <div
                    className="w-1 h-6 rounded-full"
                    style={{ backgroundColor: statusColor(entry.status) }}
                    title={entry.status}
                  />
                </td>
                <td className="py-3 pr-4 text-pencil-light text-sm whitespace-nowrap">
                  {formatTimestamp(entry.ts)}
                </td>
                <td className="py-3 pr-4 font-medium text-pencil uppercase text-sm">
                  {entry.cmd}
                </td>
                <td className="py-3 pr-4 text-pencil-light text-sm max-w-2xl break-words">
                  {renderDetail(entry)}
                </td>
                <td className="py-3 pr-4">
                  {statusBadge(entry.status)}
                </td>
                <td className="py-3 text-pencil-light text-sm text-right whitespace-nowrap">
                  {formatDuration(entry.ms)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {entries.length > PAGE_SIZES[0] && (
        <div
          className="flex items-center justify-between pt-4 mt-4 border-t-2 border-dashed border-muted"
        >
          <div className="flex items-center gap-2 text-sm text-pencil-light">
            <span>Show</span>
            {PAGE_SIZES.map((size) => (
              <PillButton
                key={size}
                active={pageSize === size}
                onClick={() => { setPageSize(size); setPage(0); }}
              >
                {size}
              </PillButton>
            ))}
            <span className="ml-1">
              {start + 1}–{Math.min(start + pageSize, entries.length)} of {entries.length}
            </span>
          </div>

          <div className="flex items-center gap-1">
            <button
              onClick={() => setPage((p) => Math.max(0, p - 1))}
              disabled={page === 0}
              className={`p-1.5 border-2 transition-all duration-150 cursor-pointer ${
                page === 0
                  ? 'border-transparent text-muted-dark cursor-not-allowed'
                  : 'border-transparent text-pencil hover:bg-paper-warm hover:border-muted-dark'
              }`}
              style={{ borderRadius: radius.sm }}
            >
              <ChevronLeft size={20} />
            </button>
            <span className="text-sm text-pencil px-2">
              {page + 1} / {totalPages}
            </span>
            <button
              onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
              disabled={page >= totalPages - 1}
              className={`p-1.5 border-2 transition-all duration-150 cursor-pointer ${
                page >= totalPages - 1
                  ? 'border-transparent text-muted-dark cursor-not-allowed'
                  : 'border-transparent text-pencil hover:bg-paper-warm hover:border-muted-dark'
              }`}
              style={{ borderRadius: radius.sm }}
            >
              <ChevronRight size={20} />
            </button>
          </div>
        </div>
      )}
    </Card>
  );
}

/* ──────────────────────────────────────────────────────────────────────
 * Section — titled group of log entries
 * ────────────────────────────────────────────────────────────────────── */

function Section({ title, entries, emptyLabel, filtered }: { title: string; entries: LogEntry[]; emptyLabel: string; filtered?: boolean }) {
  return (
    <div className="space-y-3">
      <h3 className="text-xl font-bold text-pencil">
        {title}
      </h3>
      {entries.length === 0 ? (
        <EmptyState
          icon={ScrollText}
          title={filtered ? 'No matching entries' : 'No entries yet'}
          description={filtered
            ? `No ${emptyLabel} log entries match the current filters.`
            : `No ${emptyLabel} log entries recorded.`}
        />
      ) : (
        <LogTable entries={entries} />
      )}
    </div>
  );
}

/* ──────────────────────────────────────────────────────────────────────
 * LogPage — main page
 * ────────────────────────────────────────────────────────────────────── */

export default function LogPage() {
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [tab, setTab] = useState<LogTab>('all');
  const [clearing, setClearing] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);

  // Filter state
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
    return {
      total,
      success_rate: total > 0 ? okTotal / total : 0,
      by_command: byCommand,
      last_operation: lastOp,
    };
  }, [tab, opsStatsQuery.data, auditStatsQuery.data]);

  const opsEntries = opsQuery.data?.entries ?? [];
  const opsTotal = opsQuery.data?.total ?? 0;
  const opsTotalAll = opsQuery.data?.totalAll ?? 0;
  const auditEntries = auditQuery.data?.entries ?? [];
  const auditTotal = auditQuery.data?.total ?? 0;
  const auditTotalAll = auditQuery.data?.totalAll ?? 0;

  const loading = (tab === 'all' || tab === 'ops') && opsQuery.isPending
    || (tab === 'all' || tab === 'audit') && auditQuery.isPending;

  const hasEntries = tab === 'all'
    ? opsEntries.length > 0 || auditEntries.length > 0
    : tab === 'ops'
      ? opsEntries.length > 0
      : auditEntries.length > 0;

  const knownCommands = useMemo(() => {
    const cmds = new Set<string>();
    if (opsQuery.data?.commands) {
      for (const cmd of opsQuery.data.commands) cmds.add(cmd);
    }
    if (auditQuery.data?.commands) {
      for (const cmd of auditQuery.data.commands) cmds.add(cmd);
    }
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
      toast('Log cleared', 'success');
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      setClearing(false);
      setConfirmOpen(false);
    }
  };

  if (loading && !hasEntries) return <PageSkeleton />;

  const hasFilter = !!filters;

  const totalLabel = (() => {
    if (tab === 'all') {
      if (hasFilter) {
        return `${opsTotal} of ${opsTotalAll} ops / ${auditTotal} of ${auditTotalAll} audit`;
      }
      return `${opsTotal} ops / ${auditTotal} audit`;
    }
    const secTotal = tab === 'ops' ? opsTotal : auditTotal;
    const secTotalAll = tab === 'ops' ? opsTotalAll : auditTotalAll;
    if (hasFilter) {
      return `${secTotal} of ${secTotalAll} entries`;
    }
    return `${secTotal} entries`;
  })();

  return (
    <div className="space-y-6 animate-fade-in">
      {/* ─── Header ─── */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h2
            className="text-3xl font-bold text-pencil flex items-center gap-2"
          >
            <ScrollText size={28} strokeWidth={2.5} />
            Operations & Audit Log
          </h2>
          <p className="text-pencil-light mt-1">
            Persistent record of CLI and UI operations, including audit findings by skill
          </p>
        </div>
        <div className="flex items-center gap-2">
          <HandButton onClick={handleRefresh} variant="secondary" size="sm" disabled={loading}>
            <RefreshCw size={16} className={loading ? 'animate-spin' : ''} />
            Refresh
          </HandButton>
          {hasEntries && (
            <HandButton onClick={() => setConfirmOpen(true)} variant="danger" size="sm" disabled={clearing}>
              <Trash2 size={16} />
              Clear
            </HandButton>
          )}
        </div>
      </div>

      {/* ─── Tabs ─── */}
      <div className="flex flex-wrap items-center gap-2">
        {(['all', 'ops', 'audit'] as const).map((t) => (
          <PillButton key={t} active={tab === t} onClick={() => setTab(t)}>
            {t === 'all' ? 'All' : t === 'ops' ? 'Operations' : 'Audit'}
          </PillButton>
        ))}
        <span className="text-sm text-pencil-light ml-2">
          {totalLabel}
        </span>
      </div>

      {/* ─── Filters ─── */}
      <div className="flex flex-wrap items-end gap-3">
        <div className="w-36">
          <HandSelect
            label="Command"
            value={cmdFilter}
            onChange={setCmdFilter}
            options={[
              { value: '', label: 'All' },
              ...knownCommands.map((cmd) => ({ value: cmd, label: cmd })),
            ]}
          />
        </div>
        <div className="w-32">
          <HandSelect
            label="Status"
            value={statusFilter}
            onChange={setStatusFilter}
            options={STATUS_OPTIONS.map((s) => ({ value: s, label: s || 'All' }))}
          />
        </div>
        <div>
          <span
            className="block text-sm text-pencil-light mb-1"
          >
            Time
          </span>
          <div className="flex gap-1">
            {TIME_RANGES.map((tr) => (
              <PillButton
                key={tr.value}
                active={timeRange === tr.value}
                onClick={() => setTimeRange(tr.value)}
              >
                {tr.label}
              </PillButton>
            ))}
          </div>
        </div>
      </div>

      {/* ─── Stats ─── */}
      {mergedStats && mergedStats.total > 0 && (
        <LogStatsBar stats={mergedStats} />
      )}

      {/* ─── Log entries ─── */}
      {tab === 'all' ? (
        <div className="space-y-6">
          <Section title="Operations" entries={opsEntries} emptyLabel="operation" filtered={hasFilter} />
          <Section title="Audit" entries={auditEntries} emptyLabel="audit" filtered={hasFilter} />
        </div>
      ) : tab === 'ops' ? (
        <Section title="Operations" entries={opsEntries} emptyLabel="operation" filtered={hasFilter} />
      ) : (
        <Section title="Audit" entries={auditEntries} emptyLabel="audit" filtered={hasFilter} />
      )}

      <ConfirmDialog
        open={confirmOpen}
        onConfirm={handleClear}
        onCancel={() => setConfirmOpen(false)}
        title="Clear Log"
        message={`Clear the ${tab === 'all' ? 'operations and audit logs' : tab === 'audit' ? 'audit log' : 'operations log'}? This cannot be undone.`}
        confirmText="Clear"
        variant="danger"
        loading={clearing}
      />
    </div>
  );
}
