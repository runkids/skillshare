import { useState, useMemo } from 'react';
import {
  Stethoscope,
  RefreshCw,
  CheckCircle2,
  AlertTriangle,
  XCircle,
  ChevronDown,
  ChevronRight,
  ArrowUpCircle,
  PartyPopper,
} from 'lucide-react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import type { DoctorCheck, DoctorResponse } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import Card from '../components/Card';
import Button from '../components/Button';
import Badge from '../components/Badge';
import PageHeader from '../components/PageHeader';
import { PageSkeleton } from '../components/Skeleton';
import { palette } from '../design';

type StatusFilter = 'all' | 'error' | 'warning' | 'pass';

function statusIcon(status: DoctorCheck['status'], size = 16) {
  switch (status) {
    case 'pass':
      return <CheckCircle2 size={size} strokeWidth={2.5} style={{ color: palette.success }} />;
    case 'warning':
      return <AlertTriangle size={size} strokeWidth={2.5} style={{ color: palette.warning }} />;
    case 'error':
      return <XCircle size={size} strokeWidth={2.5} style={{ color: palette.danger }} />;
  }
}

function statusBadgeVariant(status: DoctorCheck['status']): 'success' | 'warning' | 'danger' {
  switch (status) {
    case 'pass': return 'success';
    case 'warning': return 'warning';
    case 'error': return 'danger';
  }
}

function CheckRow({ check }: { check: DoctorCheck }) {
  const [expanded, setExpanded] = useState(false);
  const hasDetails = check.details && check.details.length > 0;

  return (
    <div className="border-b border-muted last:border-b-0">
      <button
        onClick={() => hasDetails && setExpanded((v) => !v)}
        className={`w-full flex items-center gap-3 px-4 py-3 text-left transition-colors ${hasDetails ? 'cursor-pointer hover:bg-muted/20' : 'cursor-default'}`}
      >
        {statusIcon(check.status)}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="font-medium text-pencil text-sm">{check.name}</span>
            <Badge variant={statusBadgeVariant(check.status)} size="sm">
              {check.status}
            </Badge>
          </div>
          <p className="text-pencil-light text-sm mt-0.5 truncate">{check.message}</p>
        </div>
        {hasDetails && (
          <span className="text-pencil-light shrink-0">
            {expanded
              ? <ChevronDown size={16} strokeWidth={2.5} />
              : <ChevronRight size={16} strokeWidth={2.5} />}
          </span>
        )}
      </button>
      {expanded && hasDetails && (
        <div className="px-4 pb-3 pl-11">
          <ul className="space-y-1">
            {check.details!.map((detail, i) => (
              <li key={i} className="text-sm text-pencil-light flex items-start gap-2">
                <span className="text-muted-dark mt-0.5 shrink-0">&bull;</span>
                <span>{detail}</span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

export default function DoctorPage() {
  const queryClient = useQueryClient();
  const { data, isPending, error, isFetching, refetch } = useQuery({
    queryKey: queryKeys.doctor,
    queryFn: () => api.doctor(),
    staleTime: staleTimes.doctor,
  });
  const [filter, setFilter] = useState<StatusFilter>('all');

  const filteredChecks = useMemo(() => {
    if (!data) return [];
    if (filter === 'all') return data.checks;
    return data.checks.filter((c) => c.status === filter);
  }, [data, filter]);

  const allPassed = data && data.summary.errors === 0 && data.summary.warnings === 0;

  if (isPending) return <PageSkeleton />;

  if (error) {
    return (
      <div className="space-y-6">
        <PageHeader
          title="Health Check"
          icon={<Stethoscope size={28} strokeWidth={2.5} />}
        />
        <Card>
          <div className="text-danger text-sm">
            Failed to load health check: {error instanceof Error ? error.message : 'Unknown error'}
          </div>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Health Check"
        icon={<Stethoscope size={28} strokeWidth={2.5} />}
        subtitle="Diagnose your skillshare setup"
        actions={
          <Button
            variant="secondary"
            size="sm"
            onClick={() => refetch()}
            loading={isFetching}
          >
            <RefreshCw size={14} strokeWidth={2.5} />
            Re-check
          </Button>
        }
      />

      {/* Summary cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card>
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-full flex items-center justify-center" style={{ backgroundColor: `${palette.success}18` }}>
              <CheckCircle2 size={20} strokeWidth={2.5} style={{ color: palette.success }} />
            </div>
            <div>
              <div className="text-2xl font-bold text-pencil">{data!.summary.pass}</div>
              <div className="text-sm text-pencil-light">Passed</div>
            </div>
          </div>
        </Card>
        <Card>
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-full flex items-center justify-center" style={{ backgroundColor: `${palette.warning}18` }}>
              <AlertTriangle size={20} strokeWidth={2.5} style={{ color: palette.warning }} />
            </div>
            <div>
              <div className="text-2xl font-bold text-pencil">{data!.summary.warnings}</div>
              <div className="text-sm text-pencil-light">Warnings</div>
            </div>
          </div>
        </Card>
        <Card>
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-full flex items-center justify-center" style={{ backgroundColor: `${palette.danger}18` }}>
              <XCircle size={20} strokeWidth={2.5} style={{ color: palette.danger }} />
            </div>
            <div>
              <div className="text-2xl font-bold text-pencil">{data!.summary.errors}</div>
              <div className="text-sm text-pencil-light">Errors</div>
            </div>
          </div>
        </Card>
      </div>

      {/* All passed banner */}
      {allPassed && (
        <Card className="!bg-success-light border-success/30">
          <div className="flex items-center gap-3">
            <PartyPopper size={22} strokeWidth={2.5} style={{ color: palette.success }} />
            <div>
              <div className="font-semibold text-pencil">All checks passed!</div>
              <div className="text-sm text-pencil-light">
                Your skillshare setup is healthy. All {data!.summary.total} checks passed.
              </div>
            </div>
          </div>
        </Card>
      )}

      {/* Filter toggles */}
      <div className="flex items-center gap-2 flex-wrap">
        {(['all', 'error', 'warning', 'pass'] as StatusFilter[]).map((f) => {
          const isActive = filter === f;
          const count = f === 'all'
            ? data!.summary.total
            : f === 'error'
              ? data!.summary.errors
              : f === 'warning'
                ? data!.summary.warnings
                : data!.summary.pass;
          return (
            <button
              key={f}
              onClick={() => setFilter(f)}
              className={`px-3 py-1.5 text-sm font-medium rounded-[var(--radius-btn)] border-2 transition-all cursor-pointer ${
                isActive
                  ? 'bg-pencil text-paper border-pencil'
                  : 'bg-transparent text-pencil-light border-muted hover:border-muted-dark'
              }`}
            >
              {f === 'all' ? 'All' : f.charAt(0).toUpperCase() + f.slice(1)}
              <span className="ml-1.5 opacity-70">{count}</span>
            </button>
          );
        })}
      </div>

      {/* Checks list */}
      <Card padding="none">
        {filteredChecks.length === 0 ? (
          <div className="py-8 text-center text-pencil-light text-sm">
            No checks match the selected filter.
          </div>
        ) : (
          filteredChecks.map((check, i) => (
            <CheckRow key={`${check.name}-${i}`} check={check} />
          ))
        )}
      </Card>

      {/* Version info */}
      {data!.version && (
        <Card>
          <div className="flex items-center justify-between">
            <div>
              <div className="text-sm font-medium text-pencil">Version</div>
              <div className="text-sm text-pencil-light mt-0.5">
                Current: <span className="font-mono">{data!.version.current}</span>
                {data!.version.latest && (
                  <> &middot; Latest: <span className="font-mono">{data!.version.latest}</span></>
                )}
              </div>
            </div>
            {data!.version.update_available && (
              <Badge variant="info" size="md" dot>
                <ArrowUpCircle size={12} strokeWidth={2.5} />
                Update available
              </Badge>
            )}
          </div>
        </Card>
      )}
    </div>
  );
}
