import { useState } from 'react';
import { Link } from 'react-router-dom';
import {
  ShieldCheck,
  ShieldAlert,
  ShieldX,
  AlertTriangle,
  Info,
  FileText,
  FileEdit,
} from 'lucide-react';
import { api } from '../api/client';
import type { AuditAllResponse, AuditResult, AuditFinding } from '../api/client';
import Card from '../components/Card';
import HandButton from '../components/HandButton';
import Badge from '../components/Badge';
import EmptyState from '../components/EmptyState';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { wobbly, shadows } from '../design';

export default function AuditPage() {
  const { toast } = useToast();
  const [data, setData] = useState<AuditAllResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const runAudit = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await api.auditAll();
      setData(res);
      const { summary } = res;
      if (summary.failed > 0) {
        toast(`Audit complete: ${summary.failed} skill(s) blocked at ${summary.threshold}+`, 'warning');
      } else if (summary.warning > 0) {
        toast(`Audit complete: ${summary.warning} skill(s) with warnings`, 'warning');
      } else if (summary.low > 0 || summary.info > 0) {
        toast(`Audit complete: ${summary.low + summary.info} informational findings`, 'warning');
      } else {
        toast('Audit complete: all skills passed', 'success');
      }
    } catch (e: any) {
      setError(e.message);
      toast(e.message, 'error');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h2
            className="text-3xl font-bold text-pencil flex items-center gap-2"
            style={{ fontFamily: 'var(--font-heading)' }}
          >
            <ShieldCheck size={28} strokeWidth={2.5} />
            Security Audit
          </h2>
          <p
            className="text-pencil-light mt-1"
            style={{ fontFamily: 'var(--font-hand)' }}
          >
            Scan installed skills for malicious patterns and security threats
          </p>
        </div>
        <div className="flex items-center gap-3">
          <Link to="/audit/rules">
            <HandButton variant="secondary" size="lg">
              <FileEdit size={18} strokeWidth={2.5} />
              Custom Rules
            </HandButton>
          </Link>
          <HandButton
            variant="primary"
            size="lg"
            onClick={runAudit}
            disabled={loading}
          >
            <ShieldCheck size={18} strokeWidth={2.5} />
            {loading ? 'Scanning...' : 'Run Audit'}
          </HandButton>
        </div>
      </div>

      {/* Loading */}
      {loading && <PageSkeleton />}

      {/* Error */}
      {error && (
        <Card>
          <p className="text-danger">{error}</p>
        </Card>
      )}

      {/* Results */}
      {data && !loading && (
        <>
          {/* Stats Grid */}
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
            <StatCard label="Total" value={data.summary.total} icon={FileText} color="pencil" />
            <StatCard label="Passed" value={data.summary.passed} icon={ShieldCheck} color="success" />
            <StatCard label="Warnings" value={data.summary.warning} icon={AlertTriangle} color="warning" />
            <StatCard label="Blocked" value={data.summary.failed} icon={ShieldX} color="danger" />
            <StatCard label="Low" value={data.summary.low} icon={Info} color="blue" />
            <StatCard label="Info" value={data.summary.info} icon={Info} color="pencil-light" />
          </div>

          <Card variant="outlined">
            <div className="flex flex-wrap items-center gap-3 text-sm">
              <Badge variant={data.summary.failed > 0 ? 'danger' : data.summary.warning > 0 ? 'warning' : 'success'}>
                threshold: {data.summary.threshold}
              </Badge>
              <Badge variant={riskBadgeVariant(data.summary.riskLabel)}>
                risk: {data.summary.riskLabel.toUpperCase()} ({data.summary.riskScore}/100)
              </Badge>
              {(data.summary.scanErrors ?? 0) > 0 && <span className="text-danger">scan errors: {data.summary.scanErrors}</span>}
            </div>
          </Card>

          {/* Findings list */}
          {data.summary.failed === 0 && data.summary.warning === 0 && data.summary.low === 0 && data.summary.info === 0 ? (
            <EmptyState
              icon={ShieldCheck}
              title="All skills passed security audit"
              description="No malicious patterns or security threats detected"
            />
          ) : (
            <div className="space-y-4">
              {data.results
                .filter((r) => r.findings.length > 0)
                .sort((a, b) => {
                  const bySeverity = severityRank(a) - severityRank(b);
                  if (bySeverity !== 0) return bySeverity;
                  return b.riskScore - a.riskScore;
                })
                .map((result, i) => (
                  <SkillAuditCard key={result.skillName} result={result} index={i} />
                ))}
            </div>
          )}

          {/* Passed skills summary */}
          {data.summary.passed > 0 && (data.summary.failed > 0 || data.summary.warning > 0 || data.summary.low > 0 || data.summary.info > 0) && (
            <Card variant="outlined">
              <div className="flex items-center gap-2 text-success">
                <ShieldCheck size={18} strokeWidth={2.5} />
                <span
                  className="font-medium"
                  style={{ fontFamily: 'var(--font-hand)' }}
                >
                  {data.summary.passed} skill{data.summary.passed !== 1 ? 's' : ''} passed with no issues
                </span>
              </div>
            </Card>
          )}
        </>
      )}

      {/* Initial state - no scan run yet */}
      {!data && !loading && !error && (
        <EmptyState
          icon={ShieldCheck}
          title="No audit results yet"
          description="Click 'Run Audit' to scan your installed skills for security threats"
          action={
            <HandButton variant="primary" onClick={runAudit}>
              <ShieldCheck size={16} strokeWidth={2.5} /> Run Audit
            </HandButton>
          }
        />
      )}
    </div>
  );
}

function StatCard({
  label,
  value,
  icon: Icon,
  color,
}: {
  label: string;
  value: number;
  icon: typeof ShieldCheck;
  color: string;
}) {
  return (
    <Card>
      <div className="flex items-center gap-3">
        <div
          className={`w-10 h-10 flex items-center justify-center border-2 border-pencil bg-white text-${color}`}
          style={{ borderRadius: wobbly.sm, boxShadow: shadows.sm }}
        >
          <Icon size={20} strokeWidth={2.5} />
        </div>
        <div>
          <p
            className="text-2xl font-bold text-pencil"
            style={{ fontFamily: 'var(--font-heading)' }}
          >
            {value}
          </p>
          <p
            className="text-sm text-pencil-light"
            style={{ fontFamily: 'var(--font-hand)' }}
          >
            {label}
          </p>
        </div>
      </div>
    </Card>
  );
}

function SkillAuditCard({ result, index }: { result: AuditResult; index: number }) {
  const maxSeverity = getMaxSeverity(result.findings);
  const Icon = result.isBlocked ? ShieldAlert : Info;

  return (
    <Card
      decoration={index === 0 ? 'tape' : 'none'}
      style={{
        transform: `rotate(${index % 2 === 0 ? '-0.15' : '0.15'}deg)`,
      }}
    >
      <div className="space-y-3">
        {/* Skill name + badge */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Icon size={18} strokeWidth={2.5} className={severityTextColor(maxSeverity)} />
            <span
              className="font-medium text-pencil"
              style={{ fontFamily: 'var(--font-hand)' }}
            >
              {result.skillName}
            </span>
          </div>
          <div className="flex items-center gap-2">
            <Badge variant={severityBadgeVariant(maxSeverity)}>
              {result.findings.length} issue{result.findings.length !== 1 ? 's' : ''}
            </Badge>
            <Badge variant={riskBadgeVariant(result.riskLabel)}>
              {result.riskLabel.toUpperCase()} {result.riskScore}/100
            </Badge>
          </div>
        </div>

        {/* Findings */}
        <div className="space-y-2 border-t border-dashed border-pencil-light/40 pt-3">
          {result.findings.map((f, i) => (
            <FindingRow key={`${f.file}-${f.line}-${i}`} finding={f} />
          ))}
        </div>
      </div>
    </Card>
  );
}

function FindingRow({ finding }: { finding: AuditFinding }) {
  return (
    <div className="flex flex-col gap-1 text-sm">
      <div className="flex items-center gap-2 flex-wrap">
        <Badge variant={severityBadgeVariant(finding.severity)}>
          {finding.severity}
        </Badge>
        <span className="text-pencil">{finding.message}</span>
        <span className="text-pencil-light">
          {finding.file}:{finding.line}
        </span>
      </div>
      {finding.snippet && (
        <code className="text-xs text-pencil-light bg-paper-warm px-2 py-1 border border-dashed border-pencil-light/30 block overflow-x-auto">
          &quot;{finding.snippet}&quot;
        </code>
      )}
    </div>
  );
}

// Helpers

function severityBadgeVariant(sev: string): 'danger' | 'warning' | 'info' {
  switch (sev) {
    case 'CRITICAL': return 'danger';
    case 'HIGH': return 'warning';
    default: return 'info';
  }
}

function riskBadgeVariant(risk: string): 'danger' | 'warning' | 'info' | 'success' {
  switch (risk) {
    case 'critical':
      return 'danger';
    case 'high':
      return 'warning';
    case 'medium':
      return 'info';
    case 'low':
      return 'success';
    default:
      return 'success';
  }
}

function severityTextColor(sev: string): string {
  switch (sev) {
    case 'CRITICAL': return 'text-danger';
    case 'HIGH': return 'text-warning';
    case 'MEDIUM': return 'text-blue';
    case 'LOW': return 'text-blue';
    case 'INFO': return 'text-pencil-light';
    default: return 'text-blue';
  }
}

function getMaxSeverity(findings: AuditFinding[]): string {
  if (findings.some((f) => f.severity === 'CRITICAL')) return 'CRITICAL';
  if (findings.some((f) => f.severity === 'HIGH')) return 'HIGH';
  if (findings.some((f) => f.severity === 'MEDIUM')) return 'MEDIUM';
  if (findings.some((f) => f.severity === 'LOW')) return 'LOW';
  if (findings.some((f) => f.severity === 'INFO')) return 'INFO';
  return 'CLEAN';
}

function severityRank(result: AuditResult): number {
  const max = getMaxSeverity(result.findings);
  switch (max) {
    case 'CRITICAL': return 0;
    case 'HIGH': return 1;
    case 'MEDIUM': return 2;
    case 'LOW': return 3;
    case 'INFO': return 4;
    default: return 5;
  }
}
