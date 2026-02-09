import { useState, useEffect, useCallback } from 'react';
import { ScrollText, Trash2, RefreshCw } from 'lucide-react';
import { api } from '../api/client';
import type { LogEntry } from '../api/client';
import Card from '../components/Card';
import HandButton from '../components/HandButton';
import Badge from '../components/Badge';
import ConfirmDialog from '../components/ConfirmDialog';
import EmptyState from '../components/EmptyState';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { wobbly, shadows } from '../design';

type LogTab = 'all' | 'ops' | 'audit';

type LogSection = {
  entries: LogEntry[];
  total: number;
};

type AuditSkillLists = {
  failed: string[];
  warning: string[];
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
  if (critical > 0 || high > 0 || medium > 0) {
    parts.push(`sev(c/h/m)=${critical}/${high}/${medium}`);
  }

  const scanErrors = asInt(args.scan_errors);
  if (scanErrors != null && scanErrors > 0) parts.push(`scan-errors=${scanErrors}`);

  return parts.join(', ');
}

function getAuditSkillLists(entry: LogEntry): AuditSkillLists {
  if (entry.cmd !== 'audit' || !entry.args) {
    return { failed: [], warning: [] };
  }
  return {
    failed: asStringArray(entry.args.failed_skills),
    warning: asStringArray(entry.args.warning_skills),
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

  if (lists.failed.length === 0 && lists.warning.length === 0) {
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
    </div>
  );
}

function LogTable({ entries }: { entries: LogEntry[] }) {
  return (
    <Card>
      <div className="overflow-x-auto">
        <table className="w-full text-left" style={{ fontFamily: 'var(--font-hand)' }}>
          <thead>
            <tr className="border-b-2 border-dashed border-muted-dark">
              <th className="pb-3 pr-4 text-pencil-light text-sm font-medium">Time</th>
              <th className="pb-3 pr-4 text-pencil-light text-sm font-medium">Command</th>
              <th className="pb-3 pr-4 text-pencil-light text-sm font-medium">Details</th>
              <th className="pb-3 pr-4 text-pencil-light text-sm font-medium">Status</th>
              <th className="pb-3 text-pencil-light text-sm font-medium text-right">Duration</th>
            </tr>
          </thead>
          <tbody>
            {entries.map((entry, i) => (
              <tr
                key={`${entry.ts}-${entry.cmd}-${i}`}
                className="border-b border-dashed border-muted hover:bg-white/60 transition-colors"
              >
                <td className="py-2.5 pr-4 text-pencil-light text-sm whitespace-nowrap">
                  {formatTimestamp(entry.ts)}
                </td>
                <td className="py-2.5 pr-4 font-medium text-pencil uppercase text-sm">
                  {entry.cmd}
                </td>
                <td className="py-2.5 pr-4 text-pencil-light text-sm max-w-2xl break-words">
                  {renderDetail(entry)}
                </td>
                <td className="py-2.5 pr-4">
                  {statusBadge(entry.status)}
                </td>
                <td className="py-2.5 text-pencil-light text-sm text-right whitespace-nowrap">
                  {formatDuration(entry.ms)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  );
}

function Section({ title, entries, emptyLabel }: { title: string; entries: LogEntry[]; emptyLabel: string }) {
  return (
    <div className="space-y-3">
      <h3 className="text-xl font-bold text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
        {title}
      </h3>
      {entries.length === 0 ? (
        <EmptyState
          icon={ScrollText}
          title="No entries yet"
          description={`No ${emptyLabel} log entries recorded.`}
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
  const [ops, setOps] = useState<LogSection>({ entries: [], total: 0 });
  const [audit, setAudit] = useState<LogSection>({ entries: [], total: 0 });
  const [loading, setLoading] = useState(true);
  const [clearing, setClearing] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);

  const fetchLog = useCallback(async () => {
    setLoading(true);
    try {
      if (tab === 'all') {
        const [opsRes, auditRes] = await Promise.all([
          api.listLog('ops', 100),
          api.listLog('audit', 100),
        ]);
        setOps({ entries: opsRes.entries, total: opsRes.total });
        setAudit({ entries: auditRes.entries, total: auditRes.total });
      } else if (tab === 'ops') {
        const res = await api.listLog('ops', 100);
        setOps({ entries: res.entries, total: res.total });
      } else {
        const res = await api.listLog('audit', 100);
        setAudit({ entries: res.entries, total: res.total });
      }
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      setLoading(false);
    }
  }, [tab, toast]);

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

  const totalLabel = tab === 'all'
    ? `${ops.total} ops / ${audit.total} audit`
    : `${tab === 'ops' ? ops.total : audit.total} total entries`;

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

      {tab === 'all' ? (
        <div className="space-y-6">
          <Section title="Operations" entries={ops.entries} emptyLabel="operation" />
          <Section title="Audit" entries={audit.entries} emptyLabel="audit" />
        </div>
      ) : tab === 'ops' ? (
        <Section title="Operations" entries={ops.entries} emptyLabel="operation" />
      ) : (
        <Section title="Audit" entries={audit.entries} emptyLabel="audit" />
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
