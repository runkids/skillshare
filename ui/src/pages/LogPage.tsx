import { useState, useEffect, useCallback } from 'react';
import { ScrollText, Trash2, RefreshCw } from 'lucide-react';
import { api } from '../api/client';
import type { LogEntry } from '../api/client';
import Card from '../components/Card';
import HandButton from '../components/HandButton';
import Badge from '../components/Badge';
import EmptyState from '../components/EmptyState';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { wobbly, shadows } from '../design';

type LogTab = 'ops' | 'audit';

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

function formatDetail(entry: LogEntry): string {
  if (entry.msg) return entry.msg;
  if (!entry.args) return '';
  const parts: string[] = [];
  if (entry.args.source) parts.push(String(entry.args.source));
  if (entry.args.name) parts.push(String(entry.args.name));
  if (entry.args.target) parts.push(String(entry.args.target));
  if (entry.args.message) parts.push(String(entry.args.message));
  if (entry.args.summary) parts.push(String(entry.args.summary));
  return parts.join(' ');
}

export default function LogPage() {
  const { toast } = useToast();
  const [tab, setTab] = useState<LogTab>('ops');
  const [entries, setEntries] = useState<LogEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [clearing, setClearing] = useState(false);

  const fetchLog = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.listLog(tab, 100);
      setEntries(res.entries);
      setTotal(res.total);
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      setLoading(false);
    }
  }, [tab, toast]);

  useEffect(() => {
    fetchLog();
  }, [fetchLog]);

  const handleClear = async () => {
    if (!confirm(`Clear the ${tab === 'audit' ? 'audit' : 'operations'} log?`)) return;
    setClearing(true);
    try {
      await api.clearLog(tab);
      setEntries([]);
      setTotal(0);
      toast('Log cleared', 'success');
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      setClearing(false);
    }
  };

  if (loading && entries.length === 0) return <PageSkeleton />;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h2
            className="text-3xl font-bold text-pencil flex items-center gap-2"
            style={{ fontFamily: 'var(--font-heading)' }}
          >
            <ScrollText size={28} strokeWidth={2.5} />
            Operation Log
          </h2>
          <p
            className="text-pencil-light mt-1"
            style={{ fontFamily: 'var(--font-hand)' }}
          >
            Persistent record of CLI and UI operations
          </p>
        </div>
        <div className="flex items-center gap-2">
          <HandButton onClick={fetchLog} variant="secondary" size="sm" disabled={loading}>
            <RefreshCw size={16} className={loading ? 'animate-spin' : ''} />
            Refresh
          </HandButton>
          {entries.length > 0 && (
            <HandButton onClick={handleClear} variant="danger" size="sm" disabled={clearing}>
              <Trash2 size={16} />
              Clear
            </HandButton>
          )}
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-2">
        {(['ops', 'audit'] as const).map((t) => (
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
            {t === 'ops' ? 'Operations' : 'Audit'}
          </button>
        ))}
        {total > 0 && (
          <span
            className="self-center text-sm text-pencil-light ml-2"
            style={{ fontFamily: 'var(--font-hand)' }}
          >
            {total} total entries
          </span>
        )}
      </div>

      {/* Log table */}
      {entries.length === 0 ? (
        <EmptyState
          icon={ScrollText}
          title="No entries yet"
          description={`No ${tab === 'audit' ? 'audit' : 'operation'} log entries recorded. Run some commands to start logging.`}
        />
      ) : (
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
                    key={i}
                    className="border-b border-dashed border-muted hover:bg-white/60 transition-colors"
                  >
                    <td className="py-2.5 pr-4 text-pencil-light text-sm whitespace-nowrap">
                      {formatTimestamp(entry.ts)}
                    </td>
                    <td className="py-2.5 pr-4 font-medium text-pencil uppercase text-sm">
                      {entry.cmd}
                    </td>
                    <td className="py-2.5 pr-4 text-pencil-light text-sm max-w-xs truncate">
                      {formatDetail(entry)}
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
      )}
    </div>
  );
}
