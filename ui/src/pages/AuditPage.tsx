import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { Virtuoso } from 'react-virtuoso';
import {
  ShieldCheck,
  ShieldAlert,
  ShieldX,
  AlertTriangle,
  Info,
  FileText,
  FileEdit,
  Ban,
  CircleCheck,
  Gauge,
  Eye,
} from 'lucide-react';
import { api } from '../api/client';
import type { AuditAllResponse, AuditResult, AuditFinding } from '../api/client';
import Card from '../components/Card';
import HandButton from '../components/HandButton';
import Badge from '../components/Badge';
import { HandSelect } from '../components/HandInput';
import EmptyState from '../components/EmptyState';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import StreamProgressBar from '../components/StreamProgressBar';
import { radius, shadows, palette } from '../design';
import { severityBadgeVariant } from '../lib/severity';
import { BlockStamp, RiskMeter, riskColor, riskBgColor } from '../components/audit';

type SeverityFilter = 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' | 'INFO';

const severityFilterOptions: { value: SeverityFilter; label: string }[] = [
  { value: 'INFO', label: 'All (INFO+)' },
  { value: 'LOW', label: 'LOW+' },
  { value: 'MEDIUM', label: 'MEDIUM+' },
  { value: 'HIGH', label: 'HIGH+' },
  { value: 'CRITICAL', label: 'CRITICAL only' },
];

export default function AuditPage() {
  const { toast } = useToast();
  const [data, setData] = useState<AuditAllResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [minSeverity, setMinSeverity] = useState<SeverityFilter>('MEDIUM');
  const [progress, setProgress] = useState<{ scanned: number; total: number } | null>(null);
  const esRef = useRef<EventSource | null>(null);
  const startTimeRef = useRef<number>(0);

  // Clean up EventSource on unmount
  useEffect(() => {
    return () => {
      esRef.current?.close();
    };
  }, []);

  const totalFindings = useMemo(() => {
    if (!data) return 0;
    return data.results.reduce((sum, result) => sum + result.findings.length, 0);
  }, [data]);

  const filteredResults = useMemo(() => {
    if (!data) return [];

    return data.results
      .map((result) => ({
        ...result,
        findings: result.findings.filter((finding) => isSeverityAtOrAbove(finding.severity, minSeverity)),
      }))
      .filter((result) => result.findings.length > 0)
      .sort((a, b) => {
        const bySeverity = severityRank(a) - severityRank(b);
        if (bySeverity !== 0) return bySeverity;
        return b.riskScore - a.riskScore;
      });
  }, [data, minSeverity]);

  const visibleFindings = useMemo(
    () => filteredResults.reduce((sum, result) => sum + result.findings.length, 0),
    [filteredResults],
  );

  const showAuditToast = useCallback((res: AuditAllResponse) => {
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
  }, [toast]);

  const runAudit = () => {
    setLoading(true);
    setError(null);
    setProgress(null);
    startTimeRef.current = Date.now();

    esRef.current = api.auditAllStream(
      (total) => setProgress({ scanned: 0, total }),
      (scanned) => setProgress((p) => p ? { ...p, scanned } : null),
      (res) => {
        setData(res);
        setLoading(false);
        setProgress(null);
        showAuditToast(res);
      },
      (err) => {
        setError(err.message);
        setLoading(false);
        setProgress(null);
        toast(err.message, 'error');
      },
    );
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div>
          <h2
            className="text-3xl font-bold text-pencil flex items-center gap-2"
          >
            <ShieldCheck size={28} strokeWidth={2.5} />
            Security Audit
          </h2>
          <p
            className="text-pencil-light mt-1"
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

      {/* Loading / Progress */}
      {loading && (
        progress ? (
          <StreamProgressBar
            count={progress.scanned}
            total={progress.total}
            startTime={startTimeRef.current}
            icon={ShieldCheck}
            iconClassName="animate-pulse-gentle"
            labelDiscovering="Scanning skills..."
            labelRunning="Scanning skills..."
            units="skills"
          />
        ) : (
          <PageSkeleton />
        )
      )}

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
            <StatCard label="Total" value={data.summary.total} icon={FileText} variant="neutral" />
            <StatCard label="Passed" value={data.summary.passed} icon={ShieldCheck} variant="success" />
            <StatCard label="Warnings" value={data.summary.warning} icon={AlertTriangle} variant="warning" />
            <StatCard label="Blocked" value={data.summary.failed} icon={ShieldX} variant="danger" />
            <StatCard label="Low" value={data.summary.low} icon={Info} variant="blue" />
            <StatCard label="Info" value={data.summary.info} icon={Info} variant="muted" />
          </div>

          {/* Triage Panel */}
          <TriagePanel
            threshold={data.summary.threshold}
            riskLabel={data.summary.riskLabel}
            riskScore={data.summary.riskScore}
            failed={data.summary.failed}
            warning={data.summary.warning}
            visibleFindings={visibleFindings}
            totalFindings={totalFindings}
            scanErrors={data.summary.scanErrors ?? 0}
            minSeverity={minSeverity}
            onSeverityChange={(v) => setMinSeverity(v as SeverityFilter)}
          />

          {/* Findings list */}
          {totalFindings === 0 ? (
            <EmptyState
              icon={ShieldCheck}
              title="All skills passed security audit"
              description="No malicious patterns or security threats detected"
            />
          ) : filteredResults.length === 0 ? (
            <EmptyState
              icon={Info}
              title="No findings match current filter"
              description={`Try lowering Min Severity below ${minSeverity}`}
            />
          ) : (
            <Virtuoso
              useWindowScroll
              totalCount={filteredResults.length}
              overscan={400}
              itemContent={(index) => (
                <div className="pb-5">
                  <SkillAuditCard key={filteredResults[index].skillName} result={filteredResults[index]} index={index} />
                </div>
              )}
            />
          )}

          {/* Passed skills summary */}
          {data.summary.passed > 0 && (data.summary.failed > 0 || data.summary.warning > 0 || data.summary.low > 0 || data.summary.info > 0) && (
            <Card variant="outlined">
              <div className="flex items-center gap-2 text-success">
                <ShieldCheck size={18} strokeWidth={2.5} />
                <span
                  className="font-medium"
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

/* ──────────────────────────────────────────────────────────────────────
 * StatCard — color-coded, bold, zero-muted
 * ────────────────────────────────────────────────────────────────────── */

type StatVariant = 'neutral' | 'success' | 'warning' | 'danger' | 'blue' | 'muted';

const statStyles: Record<StatVariant, { bg: string; border: string; iconBg: string; text: string; valueFaded: string }> = {
  neutral: {
    bg: 'bg-surface',
    border: 'border-pencil',
    iconBg: 'bg-paper-warm',
    text: 'text-pencil',
    valueFaded: 'text-pencil-light',
  },
  success: {
    bg: 'bg-success-light',
    border: 'border-success',
    iconBg: 'bg-surface',
    text: 'text-success',
    valueFaded: 'text-success/40',
  },
  warning: {
    bg: 'bg-warning-light',
    border: 'border-warning',
    iconBg: 'bg-surface',
    text: 'text-warning',
    valueFaded: 'text-warning/40',
  },
  danger: {
    bg: 'bg-danger-light',
    border: 'border-danger',
    iconBg: 'bg-surface',
    text: 'text-danger',
    valueFaded: 'text-danger/40',
  },
  blue: {
    bg: 'bg-info-light',
    border: 'border-blue',
    iconBg: 'bg-surface',
    text: 'text-blue',
    valueFaded: 'text-blue/40',
  },
  muted: {
    bg: 'bg-surface',
    border: 'border-pencil-light',
    iconBg: 'bg-paper-warm',
    text: 'text-pencil-light',
    valueFaded: 'text-pencil-light/40',
  },
};

function StatCard({
  label,
  value,
  icon: Icon,
  variant,
}: {
  label: string;
  value: number;
  icon: typeof ShieldCheck;
  variant: StatVariant;
}) {
  const isZero = value === 0;
  // When zero: muted styling; when non-zero and danger/warning: bold tinted background
  const activeVariant = isZero ? 'muted' : variant;
  const s = statStyles[activeVariant];

  return (
    <div
      className={`relative p-4 border-2 ${s.bg} ${s.border} transition-all duration-100 ${isZero ? 'opacity-60' : ''}`}
      style={{
        borderRadius: radius.md,
        boxShadow: isZero ? 'none' : shadows.sm,
      }}
    >
      <div className="flex items-center gap-3">
        <div
          className={`w-10 h-10 flex items-center justify-center border-2 ${s.border} ${s.iconBg} ${s.text}`}
          style={{ borderRadius: radius.sm, boxShadow: isZero ? 'none' : shadows.sm }}
        >
          <Icon size={20} strokeWidth={2.5} />
        </div>
        <div>
          <p
            className={`text-2xl font-bold ${isZero ? s.valueFaded : s.text}`}
          >
            {value}
          </p>
          <p
            className={`text-sm ${isZero ? 'text-pencil-light/50' : 'text-pencil-light'}`}
          >
            {label}
          </p>
        </div>
      </div>
      {/* Pulse dot for non-zero danger/warning */}
      {!isZero && (variant === 'danger' || variant === 'warning') && (
        <div className="absolute top-2 right-2">
          <span className={`block w-2.5 h-2.5 rounded-full ${variant === 'danger' ? 'bg-danger' : 'bg-warning'} animate-pulse-gentle`} />
        </div>
      )}
    </div>
  );
}

/* ──────────────────────────────────────────────────────────────────────
 * TriagePanel — structured report card for threshold/risk/filter
 * ────────────────────────────────────────────────────────────────────── */

function TriagePanel({
  threshold,
  riskLabel,
  riskScore,
  failed,
  warning,
  visibleFindings,
  totalFindings,
  scanErrors,
  minSeverity,
  onSeverityChange,
}: {
  threshold: string;
  riskLabel: string;
  riskScore: number;
  failed: number;
  warning: number;
  visibleFindings: number;
  totalFindings: number;
  scanErrors: number;
  minSeverity: SeverityFilter;
  onSeverityChange: (v: string) => void;
}) {
  const overallStatus = failed > 0 ? 'blocked' : warning > 0 ? 'warning' : 'clean';

  return (
    <Card variant="outlined" className="overflow-visible z-30">
      <div className="flex flex-col gap-4">
        {/* Top row: three indicator columns */}
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          {/* Block Threshold Indicator */}
          <div
            className="flex items-center gap-3 p-3 border-2 border-dashed"
            style={{
              borderRadius: radius.sm,
              borderColor: overallStatus === 'blocked' ? palette.danger : overallStatus === 'warning' ? palette.warning : palette.success,
              backgroundColor: overallStatus === 'blocked' ? 'rgba(192, 57, 43, 0.06)' : overallStatus === 'warning' ? 'rgba(212, 135, 14, 0.06)' : 'rgba(46, 139, 87, 0.06)',
            }}
          >
            <div
              className={`w-10 h-10 flex items-center justify-center border-2 shrink-0 ${
                overallStatus === 'blocked'
                  ? 'bg-danger text-white border-danger'
                  : overallStatus === 'warning'
                    ? 'bg-warning text-white border-warning'
                    : 'bg-success text-white border-success'
              }`}
              style={{ borderRadius: radius.sm }}
            >
              {overallStatus === 'blocked' ? (
                <Ban size={20} strokeWidth={3} />
              ) : overallStatus === 'warning' ? (
                <AlertTriangle size={20} strokeWidth={2.5} />
              ) : (
                <CircleCheck size={20} strokeWidth={2.5} />
              )}
            </div>
            <div className="min-w-0">
              <p className="text-xs text-pencil-light uppercase tracking-wide">
                Block Threshold
              </p>
              <p
                className={`text-base font-bold ${overallStatus === 'blocked' ? 'text-danger' : overallStatus === 'warning' ? 'text-warning' : 'text-success'}`}
              >
                {threshold}
                {overallStatus === 'blocked' && ` (${failed} blocked)`}
              </p>
            </div>
          </div>

          {/* Aggregate Risk Indicator */}
          <div
            className="flex items-center gap-3 p-3 border-2 border-dashed"
            style={{
              borderRadius: radius.sm,
              borderColor: riskColor(riskLabel),
              backgroundColor: riskBgColor(riskLabel),
            }}
          >
            <div
              className="w-10 h-10 flex items-center justify-center border-2 shrink-0 text-white"
              style={{
                borderRadius: radius.sm,
                backgroundColor: riskColor(riskLabel),
                borderColor: riskColor(riskLabel),
              }}
            >
              <Gauge size={20} strokeWidth={2.5} />
            </div>
            <div className="min-w-0">
              <p className="text-xs text-pencil-light uppercase tracking-wide">
                Aggregate Risk
              </p>
              <div className="flex items-center gap-2">
                <p
                  className="text-base font-bold"
                  style={{ color: riskColor(riskLabel) }}
                >
                  {riskLabel.toUpperCase()}
                </p>
                {/* Mini risk bar */}
                <div
                  className="flex-1 h-2 bg-muted/50 overflow-hidden max-w-20"
                  style={{ borderRadius: '999px' }}
                >
                  <div
                    className="h-full transition-all duration-300"
                    style={{
                      width: `${riskScore}%`,
                      backgroundColor: riskColor(riskLabel),
                      borderRadius: '999px',
                    }}
                  />
                </div>
                <span className="text-xs text-pencil-light font-mono">{riskScore}</span>
              </div>
            </div>
          </div>

          {/* Visibility + Filter */}
          <div
            className="flex items-center gap-3 p-3 border-2 border-dashed border-pencil-light/40"
            style={{
              borderRadius: radius.sm,
              backgroundColor: 'rgba(229, 224, 216, 0.15)',
            }}
          >
            <div
              className="w-10 h-10 flex items-center justify-center border-2 border-pencil-light bg-paper-warm text-pencil-light shrink-0"
              style={{ borderRadius: radius.sm }}
            >
              <Eye size={20} strokeWidth={2.5} />
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-xs text-pencil-light uppercase tracking-wide">
                Visible Findings
              </p>
              <p
                className="text-base font-bold text-pencil"
              >
                {visibleFindings}
                <span className="text-pencil-light font-normal text-sm"> / {totalFindings}</span>
              </p>
            </div>
          </div>
        </div>

        {/* Bottom row: filter + help */}
        <div className="flex flex-col sm:flex-row items-start sm:items-end gap-3 pt-2 border-t border-dashed border-pencil-light/30">
          <div className="w-full sm:w-56">
            <HandSelect
              label="Min Severity"
              value={minSeverity}
              onChange={(value) => onSeverityChange(value)}
              options={severityFilterOptions}
            />
          </div>
          <div className="flex items-center gap-4 flex-wrap">
            {scanErrors > 0 && (
              <span className="text-danger text-sm flex items-center gap-1">
                <AlertTriangle size={14} strokeWidth={2.5} />
                {scanErrors} scan error{scanErrors !== 1 ? 's' : ''}
              </span>
            )}
            <p className="text-xs text-pencil-light">
              Block = any finding at/above threshold. Aggregate = overall risk score for triage.
            </p>
          </div>
        </div>
      </div>
    </Card>
  );
}

/* ──────────────────────────────────────────────────────────────────────
 * SkillAuditCard — prominent block/risk header
 * ────────────────────────────────────────────────────────────────────── */

function SkillAuditCard({ result }: { result: AuditResult; index?: number }) {
  const maxSeverity = getMaxSeverity(result.findings);

  return (
    <Card>
      <div className="space-y-3">
        {/* ── Header: skill name (left) + block/risk indicators (right) ── */}
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          {/* Left: skill icon + name + issue count */}
          <div className="flex items-center gap-2.5 min-w-0">
            <div
              className={`w-8 h-8 flex items-center justify-center border-2 shrink-0 ${
                result.isBlocked
                  ? 'bg-danger-light border-danger text-danger'
                  : maxSeverity === 'HIGH' || maxSeverity === 'CRITICAL'
                    ? 'bg-warning-light border-warning text-warning'
                    : 'bg-info-light border-blue text-blue'
              }`}
              style={{ borderRadius: radius.sm }}
            >
              {result.isBlocked ? (
                <ShieldAlert size={16} strokeWidth={2.5} />
              ) : (
                <ShieldCheck size={16} strokeWidth={2.5} />
              )}
            </div>
            <span
              className="font-bold text-pencil text-lg truncate"
            >
              {result.skillName}
            </span>
            <Badge variant={severityBadgeVariant(maxSeverity)}>
              {result.findings.length} issue{result.findings.length !== 1 ? 's' : ''}
            </Badge>
          </div>

          {/* Right: block stamp + risk meter */}
          <div className="flex items-center gap-3 shrink-0">
            {/* Block status stamp */}
            <BlockStamp isBlocked={result.isBlocked} />
            {/* Risk indicator */}
            <RiskMeter riskLabel={result.riskLabel} riskScore={result.riskScore} />
          </div>
        </div>

        {/* ── Findings list ── */}
        <div className="space-y-2 pt-3 border-t border-dashed border-pencil-light/30">
          {result.findings.map((f, i) => (
            <FindingRow key={`${f.file}-${f.line}-${i}`} finding={f} />
          ))}
        </div>
      </div>
    </Card>
  );
}

/* BlockStamp and RiskMeter imported from ../components/audit */

/* ──────────────────────────────────────────────────────────────────────
 * FindingRow — severity color stripe + better visual hierarchy
 * ────────────────────────────────────────────────────────────────────── */

function FindingRow({ finding }: { finding: AuditFinding }) {
  const stripeColor = severityStripeColor(finding.severity);

  return (
    <div
      className="flex flex-col gap-1.5 text-sm border-l-[3px] pl-3 py-2 transition-colors duration-100 hover:bg-paper-warm/60"
      style={{
        borderLeftColor: stripeColor,
        borderRadius: '0 6px 6px 0',
      }}
    >
      <div className="flex items-start gap-2 flex-wrap">
        <Badge variant={severityBadgeVariant(finding.severity)}>
          {finding.severity}
        </Badge>
        <span className="text-pencil flex-1">{finding.message}</span>
      </div>
      <span
        className="text-xs text-pencil-light"
        style={{ fontFamily: "'Courier New', monospace" }}
      >
        {finding.file}:{finding.line}
      </span>
      {(finding.ruleId || finding.analyzer || finding.category) && (
        <span className="text-xs text-pencil-light/60">
          {[finding.ruleId, finding.analyzer, finding.category].filter(Boolean).join(' · ')}
        </span>
      )}
      {finding.snippet && (
        <div className="relative mt-1">
          <code
            className="text-xs text-pencil-light block px-3 py-2 border-2 border-dashed border-muted overflow-x-auto bg-paper-warm"
            style={{
              fontFamily: "'Courier New', monospace",
              borderRadius: radius.sm,
              boxShadow: 'var(--shadow-sm)',
            }}
          >
            &quot;{finding.snippet}&quot;
          </code>
        </div>
      )}
    </div>
  );
}


/* ──────────────────────────────────────────────────────────────────────
 * Helper functions
 * ────────────────────────────────────────────────────────────────────── */

/* riskColor and riskBgColor imported from ../components/audit */

function severityStripeColor(sev: string): string {
  switch (sev) {
    case 'CRITICAL': return palette.danger;
    case 'HIGH': return palette.warning;
    case 'MEDIUM': return palette.info;
    case 'LOW': return palette.info;
    case 'INFO': return '#e2dfd8';
    default: return '#e2dfd8';
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

function severityOrder(sev: string): number {
  switch (sev.toUpperCase()) {
    case 'CRITICAL': return 0;
    case 'HIGH': return 1;
    case 'MEDIUM': return 2;
    case 'LOW': return 3;
    case 'INFO': return 4;
    default: return 99;
  }
}

function isSeverityAtOrAbove(sev: string, min: SeverityFilter): boolean {
  return severityOrder(sev) <= severityOrder(min);
}
