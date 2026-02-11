import { useState, useEffect, useCallback, useMemo } from 'react';
import { ScrollText, Trash2, RefreshCw, ChevronLeft, ChevronRight } from 'lucide-react';
import { api } from '../api/client';
import type { LogEntry } from '../api/client';
import Card from '../components/Card';
import HandButton from '../components/HandButton';
import Badge from '../components/Badge';
import ConfirmDialog from '../components/ConfirmDialog';
import EmptyState from '../components/EmptyState';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { HandSelect } from '../components/HandInput';
import { wobbly, shadows } from '../design';

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

type LogSection = {
  entries: LogEntry[];
  total: number;
  totalAll: number;
};

type AuditSkillLists = {
  failed: string[];
  warning: string[];
  low: string[];
  info: string[];
};

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
        <div className="text-xs text-yellow-700">
          warning skills: {summarizeNames(lists.warning, 6)}
        </div>
      )}
      {lists.low.length > 0 && (
        <div className="text-xs text-blue-700">
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

const PAGE_SIZES = [10, 25, 50] as const;

function LogTable({ entries }: { entries: LogEntry[] }) {
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState<number>(25);

  // Reset to first page when entries change (e.g. filter applied)
  useEffect(() => { setPage(0); }, [entries]);

  const totalPages = Math.max(1, Math.ceil(entries.length / pageSize));
  const start = page * pageSize;
  const visible = entries.slice(start, start + pageSize);

  return (
    <Card>
      <div className="overflow-x-auto">
        <table className="w-full text-left" style={{ fontFamily: 'var(--font-hand)' }}>
          <thead>
            <tr className="border-b-2 border-dashed border-muted-dark">
              <th className="pb-3 pr-4 text-pencil-light text-base font-medium">Time</th>
              <th className="pb-3 pr-4 text-pencil-light text-base font-medium">Command</th>
              <th className="pb-3 pr-4 text-pencil-light text-base font-medium">Details</th>
              <th className="pb-3 pr-4 text-pencil-light text-base font-medium">Status</th>
              <th className="pb-3 text-pencil-light text-base font-medium text-right">Duration</th>
            </tr>
          </thead>
          <tbody>
            {visible.map((entry, i) => (
              <tr
                key={`${entry.ts}-${entry.cmd}-${start + i}`}
                className="border-b border-dashed border-muted hover:bg-white/60 transition-colors"
              >
                <td className="py-3 pr-4 text-pencil-light text-base whitespace-nowrap">
                  {formatTimestamp(entry.ts)}
                </td>
                <td className="py-3 pr-4 font-medium text-pencil uppercase text-base">
                  {entry.cmd}
                </td>
                <td className="py-3 pr-4 text-pencil-light text-base max-w-2xl break-words">
                  {renderDetail(entry)}
                </td>
                <td className="py-3 pr-4">
                  {statusBadge(entry.status)}
                </td>
                <td className="py-3 text-pencil-light text-base text-right whitespace-nowrap">
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
          style={{ fontFamily: 'var(--font-hand)' }}
        >
          <div className="flex items-center gap-2 text-base text-pencil-light">
            <span>Show</span>
            {PAGE_SIZES.map((size) => (
              <button
                key={size}
                onClick={() => { setPageSize(size); setPage(0); }}
                className={`px-2.5 py-1 text-base border-2 transition-all duration-100 ${
                  pageSize === size
                    ? 'bg-white border-pencil text-pencil font-medium'
                    : 'bg-transparent border-transparent text-pencil-light hover:text-pencil hover:bg-white/60'
                }`}
                style={{
                  borderRadius: wobbly.sm,
                  boxShadow: pageSize === size ? shadows.sm : 'none',
                }}
              >
                {size}
              </button>
            ))}
            <span className="ml-1">
              {start + 1}–{Math.min(start + pageSize, entries.length)} of {entries.length}
            </span>
          </div>

          <div className="flex items-center gap-1">
            <button
              onClick={() => setPage((p) => Math.max(0, p - 1))}
              disabled={page === 0}
              className={`p-1.5 border-2 transition-all duration-100 ${
                page === 0
                  ? 'border-transparent text-muted-dark cursor-not-allowed'
                  : 'border-transparent text-pencil-light hover:text-pencil hover:bg-white/60 hover:border-pencil'
              }`}
              style={{ borderRadius: wobbly.sm }}
            >
              <ChevronLeft size={20} />
            </button>
            <span className="text-base text-pencil px-2">
              {page + 1} / {totalPages}
            </span>
            <button
              onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
              disabled={page >= totalPages - 1}
              className={`p-1.5 border-2 transition-all duration-100 ${
                page >= totalPages - 1
                  ? 'border-transparent text-muted-dark cursor-not-allowed'
                  : 'border-transparent text-pencil-light hover:text-pencil hover:bg-white/60 hover:border-pencil'
              }`}
              style={{ borderRadius: wobbly.sm }}
            >
              <ChevronRight size={20} />
            </button>
          </div>
        </div>
      )}
    </Card>
  );
}

function Section({ title, entries, emptyLabel, filtered }: { title: string; entries: LogEntry[]; emptyLabel: string; filtered?: boolean }) {
  return (
    <div className="space-y-3">
      <h3 className="text-xl font-bold text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
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

export default function LogPage() {
  const { toast } = useToast();
  const [tab, setTab] = useState<LogTab>('all');
  const [ops, setOps] = useState<LogSection>({ entries: [], total: 0, totalAll: 0 });
  const [audit, setAudit] = useState<LogSection>({ entries: [], total: 0, totalAll: 0 });
  const [loading, setLoading] = useState(true);
  const [clearing, setClearing] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);

  // Filter state
  const [cmdFilter, setCmdFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [timeRange, setTimeRange] = useState<TimeRange>('');

  const filters = useMemo(() => {
    const f: { cmd?: string; status?: string; since?: string } = {};
    if (cmdFilter) f.cmd = cmdFilter;
    if (statusFilter) f.status = statusFilter;
    const since = timeRangeToSince(timeRange);
    if (since) f.since = since;
    return Object.keys(f).length > 0 ? f : undefined;
  }, [cmdFilter, statusFilter, timeRange]);

  // Distinct commands from all (unfiltered) log entries, returned by the API
  const [knownCommands, setKnownCommands] = useState<string[]>([]);

  const fetchLog = useCallback(async () => {
    setLoading(true);
    try {
      if (tab === 'all') {
        const [opsRes, auditRes] = await Promise.all([
          api.listLog('ops', 100, filters),
          api.listLog('audit', 100, filters),
        ]);
        setOps({ entries: opsRes.entries, total: opsRes.total, totalAll: opsRes.totalAll });
        setAudit({ entries: auditRes.entries, total: auditRes.total, totalAll: auditRes.totalAll });
        // Merge distinct commands from both logs (always unfiltered from backend)
        const cmds = new Set([...opsRes.commands, ...auditRes.commands]);
        setKnownCommands(Array.from(cmds).sort());
      } else if (tab === 'ops') {
        const res = await api.listLog('ops', 100, filters);
        setOps({ entries: res.entries, total: res.total, totalAll: res.totalAll });
        setKnownCommands(res.commands);
      } else {
        const res = await api.listLog('audit', 100, filters);
        setAudit({ entries: res.entries, total: res.total, totalAll: res.totalAll });
        setKnownCommands(res.commands);
      }
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      setLoading(false);
    }
  }, [tab, filters, toast]);

  useEffect(() => {
    fetchLog();
  }, [fetchLog]);

  const hasEntries = tab === 'all'
    ? ops.entries.length > 0 || audit.entries.length > 0
    : tab === 'ops'
      ? ops.entries.length > 0
      : audit.entries.length > 0;

  const handleClear = async () => {
    setClearing(true);
    try {
      if (tab === 'all') {
        await Promise.all([api.clearLog('ops'), api.clearLog('audit')]);
      } else {
        await api.clearLog(tab);
      }
      await fetchLog();
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
        return `${ops.total} of ${ops.totalAll} ops / ${audit.total} of ${audit.totalAll} audit`;
      }
      return `${ops.total} ops / ${audit.total} audit`;
    }
    const sec = tab === 'ops' ? ops : audit;
    if (hasFilter) {
      return `${sec.total} of ${sec.totalAll} entries`;
    }
    return `${sec.total} entries`;
  })();

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h2
            className="text-3xl font-bold text-pencil flex items-center gap-2"
            style={{ fontFamily: 'var(--font-heading)' }}
          >
            <ScrollText size={28} strokeWidth={2.5} />
            Operations & Audit Log
          </h2>
          <p className="text-pencil-light mt-1" style={{ fontFamily: 'var(--font-hand)' }}>
            Persistent record of CLI and UI operations, including audit findings by skill
          </p>
        </div>
        <div className="flex items-center gap-2">
          <HandButton onClick={fetchLog} variant="secondary" size="sm" disabled={loading}>
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

      <div className="flex flex-wrap gap-2">
        {(['all', 'ops', 'audit'] as const).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-4 py-2 text-base border-2 transition-all duration-100 ${
              tab === t
                ? 'bg-white border-pencil text-pencil font-medium'
                : 'bg-transparent border-transparent text-pencil-light hover:text-pencil hover:bg-white/60'
            }`}
            style={{
              borderRadius: wobbly.sm,
              boxShadow: tab === t ? shadows.sm : 'none',
              fontFamily: 'var(--font-hand)',
            }}
          >
            {t === 'all' ? 'All' : t === 'ops' ? 'Operations' : 'Audit'}
          </button>
        ))}
        <span className="self-center text-sm text-pencil-light ml-2" style={{ fontFamily: 'var(--font-hand)' }}>
          {totalLabel}
        </span>
      </div>

      {/* Filters */}
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
            className="block text-base text-pencil-light mb-1"
            style={{ fontFamily: 'var(--font-hand)' }}
          >
            Time
          </span>
          <div className="flex gap-1">
            {TIME_RANGES.map((tr) => (
              <button
                key={tr.value}
                onClick={() => setTimeRange(tr.value)}
                className={`px-3 py-2 text-sm border-2 transition-all duration-100 ${
                  timeRange === tr.value
                    ? 'bg-white border-pencil text-pencil font-medium'
                    : 'bg-transparent border-transparent text-pencil-light hover:text-pencil hover:bg-white/60'
                }`}
                style={{
                  borderRadius: wobbly.sm,
                  boxShadow: timeRange === tr.value ? shadows.sm : 'none',
                  fontFamily: 'var(--font-hand)',
                }}
              >
                {tr.label}
              </button>
            ))}
          </div>
        </div>
      </div>

      {tab === 'all' ? (
        <div className="space-y-6">
          <Section title="Operations" entries={ops.entries} emptyLabel="operation" filtered={hasFilter} />
          <Section title="Audit" entries={audit.entries} emptyLabel="audit" filtered={hasFilter} />
        </div>
      ) : tab === 'ops' ? (
        <Section title="Operations" entries={ops.entries} emptyLabel="operation" filtered={hasFilter} />
      ) : (
        <Section title="Audit" entries={audit.entries} emptyLabel="audit" filtered={hasFilter} />
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
