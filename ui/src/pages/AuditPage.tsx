import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useT } from '../i18n';
import { Link } from 'react-router-dom';
import { Virtuoso } from 'react-virtuoso';
import {
  ShieldCheck,
  ShieldAlert,
  AlertTriangle,
  Info,
  FileEdit,
  Ban,
  CircleCheck,
  Gauge,
  Eye,
  Puzzle,
  Bot,
} from 'lucide-react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import type { AuditAllResponse, AuditResult, AuditFinding } from '../api/client';
import Card from '../components/Card';
import Button from '../components/Button';
import PageHeader from '../components/PageHeader';
import Badge from '../components/Badge';
import { Select } from '../components/Input';
import EmptyState from '../components/EmptyState';
import { useToast } from '../components/Toast';
import StreamProgressBar from '../components/StreamProgressBar';
import { radius, palette } from '../design';
import { severityBadgeVariant } from '../lib/severity';
import { BlockStamp, RiskMeter, riskColor, riskBgColor } from '../components/audit';
import ScrollToTop from '../components/ScrollToTop';
import KindBadge from '../components/KindBadge';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { getCachedAuditResult } from '../lib/auditCache';

type SeverityFilter = 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' | 'INFO';
type AuditKind = 'skills' | 'agents';

const CROSS_SKILL_NAME = '_cross-skill';

const severityFilterOptions: { value: SeverityFilter; label: string }[] = [
  { value: 'INFO', label: 'All (INFO+)' },
  { value: 'LOW', label: 'LOW+' },
  { value: 'MEDIUM', label: 'MEDIUM+' },
  { value: 'HIGH', label: 'HIGH+' },
  { value: 'CRITICAL', label: 'CRITICAL only' },
];

export default function AuditPage() {
  const t = useT();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const overviewQuery = useQuery({
    queryKey: queryKeys.overview,
    queryFn: () => api.getOverview(),
    staleTime: staleTimes.overview,
  });
  const [activeKind, setActiveKind] = useState<AuditKind>('skills');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [minSeverity, setMinSeverity] = useState<SeverityFilter>('MEDIUM');
  const [progress, setProgress] = useState<{ scanned: number; total: number } | null>(null);
  const esRef = useRef<EventSource | null>(null);
  const startTimeRef = useRef<number>(0);
  // Bump to trigger re-render after writing to query cache
  const [, setCacheTick] = useState(0);

  const installedCounts = {
    skills: overviewQuery.data?.skillCount,
    agents: overviewQuery.data?.agentCount,
  };
  const installedCount = installedCounts[activeKind];
  const dataCache = {
    skills: getCachedAuditResult(queryClient, 'skills', installedCounts.skills),
    agents: getCachedAuditResult(queryClient, 'agents', installedCounts.agents),
  };
  const data = dataCache[activeKind];

  // Clean up EventSource on unmount
  useEffect(() => {
    return () => {
      esRef.current?.close();
    };
  }, []);

  // Exclude synthetic _cross-skill result from real scan results.
  // Cross-skill analysis is a derived insight, not an actual scanned resource.
  const realResults = useMemo(() => {
    if (!data) return [];
    return data.results.filter((r) => r.skillName !== CROSS_SKILL_NAME);
  }, [data]);

  const totalFindings = useMemo(() => {
    return realResults.reduce((sum, result) => sum + result.findings.length, 0);
  }, [realResults]);

  const filteredResults = useMemo(() => {
    return realResults
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
  }, [realResults, minSeverity]);

  const visibleFindings = useMemo(
    () => filteredResults.reduce((sum, result) => sum + result.findings.length, 0),
    [filteredResults],
  );

  const showAuditToast = useCallback((res: AuditAllResponse) => {
    const { summary } = res;
    const noun = activeKind === 'agents' ? 'agent(s)' : 'skill(s)';
    if (summary.failed > 0) {
      toast(t('audit.toast.blocked', { count: summary.failed, noun, threshold: summary.threshold }), 'warning');
    } else if (summary.warning > 0) {
      toast(t('audit.toast.warnings', { count: summary.warning, noun }), 'warning');
    } else if (summary.low > 0 || summary.info > 0) {
      toast(t('audit.toast.informational', { count: summary.low + summary.info }), 'warning');
    } else {
      toast(t('audit.toast.allPassed', { activeKind }), 'success');
    }
  }, [toast, activeKind, t]);

  const runAudit = () => {
    if (installedCount === 0) {
      toast(t('audit.toast.noInstalled', { activeKind }), 'info');
      return;
    }

    setLoading(true);
    setError(null);
    setProgress(null);
    startTimeRef.current = Date.now();

    esRef.current = api.auditAllStream(
      (total) => setProgress({ scanned: 0, total }),
      (scanned) => setProgress((p) => p ? { ...p, scanned } : null),
      (res) => {
        queryClient.setQueryData(queryKeys.audit.all(activeKind), res);
        setCacheTick((n) => n + 1);
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
      activeKind,
    );
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div data-tour="audit-summary">
      <PageHeader
        icon={<ShieldCheck size={24} strokeWidth={2.5} />}
        title={t('audit.header.title')}
        subtitle={t('audit.header.subtitle')}
        actions={
          <>
            <Link to="/audit/rules">
              <Button variant="secondary" size="sm">
                <FileEdit size={16} strokeWidth={2.5} />
                {t('audit.header.customRules')}
              </Button>
            </Link>
            <Button
              variant="primary"
              size="sm"
              onClick={runAudit}
              disabled={loading || overviewQuery.isPending || installedCount === 0}
            >
              <ShieldCheck size={16} strokeWidth={2.5} />
              {loading ? t('audit.header.scanning') : t('audit.header.runAudit')}
            </Button>
          </>
        }
      />
      </div>

      {/* Kind tabs */}
      <nav className="ss-resource-tabs flex items-center gap-6 border-b-2 border-muted -mx-4 px-4 md:-mx-8 md:px-8" role="tablist">
        {([
          { key: 'skills' as AuditKind, icon: <Puzzle size={16} strokeWidth={2.5} />, label: t('resources.tab.skills'), count: installedCounts.skills },
          { key: 'agents' as AuditKind, icon: <Bot size={16} strokeWidth={2.5} />, label: t('resources.tab.agents'), count: installedCounts.agents },
        ]).map((tab) => (
          <button
            key={tab.key}
            role="tab"
            aria-selected={activeKind === tab.key}
            onClick={() => { if (!loading) setActiveKind(tab.key); }}
            disabled={loading}
            className={`
              ss-resource-tab
              inline-flex items-center gap-1.5 px-1 pb-2.5 text-sm font-semibold cursor-pointer
              transition-all duration-150 border-b-[3px] -mb-[2px]
              disabled:opacity-50
              ${activeKind === tab.key
                ? 'border-pencil text-pencil'
                : 'border-transparent text-pencil-light hover:text-pencil hover:border-muted-dark'
              }
            `}
          >
            {tab.icon}
            {tab.label}
            {tab.count != null && (
              <span className={`
                text-[11px] font-medium px-1.5 py-0.5 rounded-[var(--radius-sm)]
                ${activeKind === tab.key ? 'bg-pencil/10 text-pencil' : 'bg-muted text-pencil-light'}
              `}>
                {tab.count}
              </span>
            )}
          </button>
        ))}
      </nav>

      {/* Loading / Progress */}
      {loading && (
        <StreamProgressBar
          count={progress?.scanned ?? 0}
          total={progress?.total ?? 0}
          startTime={startTimeRef.current}
          icon={ShieldCheck}
          iconClassName="animate-pulse"
          labelDiscovering={t('audit.scanning', { kind: activeKind })}
          labelRunning={t('audit.scanning', { kind: activeKind })}
          units={activeKind}
        />
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
          {/* Inline summary */}
          <AuditSummaryLine summary={data.summary} />

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
              title={t('audit.empty.allPassed.title', { activeKind })}
              description={t('audit.empty.allPassed.description')}
            />
          ) : filteredResults.length === 0 ? (
            <EmptyState
              icon={Info}
              title={t('audit.empty.noFilterMatch.title')}
              description={t('audit.empty.noFilterMatch.description', { minSeverity })}
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
                  {t('audit.passed.withNoIssues', { count: data.summary.passed, noun: `${activeKind === 'agents' ? 'agent' : 'skill'}${data.summary.passed !== 1 ? 's' : ''}` })}
                </span>
              </div>
            </Card>
          )}
        </>
      )}

      {/* Initial state - no scan run yet */}
      {!data && !loading && !error && installedCount === 0 && (
        <EmptyState
          icon={ShieldCheck}
          title={t('audit.empty.noInstalled.title', { activeKind })}
          description={t('audit.empty.noInstalled.description', { noun: activeKind === 'agents' ? 'an agent' : 'a skill' })}
        />
      )}

      {!data && !loading && !error && installedCount !== 0 && (
        <EmptyState
          icon={ShieldCheck}
          title={t('audit.empty.noResults.title', { activeKind })}
          description={t('audit.empty.noResults.description', { activeKind })}
          action={
            <Button variant="primary" onClick={runAudit}>
              <ShieldCheck size={16} strokeWidth={2.5} /> {t('audit.empty.noResults.action')}
            </Button>
          }
        />
      )}

      <ScrollToTop />
    </div>
  );
}

/* ──────────────────────────────────────────────────────────────────────
 * AuditSummaryLine — compact inline summary
 * ────────────────────────────────────────────────────────────────────── */

function AuditSummaryLine({ summary }: { summary: AuditAllResponse['summary'] }) {
  const t = useT();
  return (
    <p className="text-sm text-pencil-light">
      <span className="font-medium text-pencil">{summary.total}</span> {t('audit.summary.scanned')}
      {' · '}<span className="font-medium text-success">{summary.passed}</span> {t('audit.summary.passed')}
      {summary.failed > 0 && (
        <>{' · '}<span className="font-medium text-danger">{summary.failed}</span> {t('audit.summary.blocked')}</>
      )}
    </p>
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
  const t = useT();
  const overallStatus = failed > 0 ? 'blocked' : warning > 0 ? 'warning' : 'clean';

  return (
    <Card variant="outlined" overflow className="z-30">
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
                {t('audit.triage.blockThreshold')}
              </p>
              <p
                className={`text-base font-bold ${overallStatus === 'blocked' ? 'text-danger' : overallStatus === 'warning' ? 'text-warning' : 'text-success'}`}
              >
                {threshold}
                {overallStatus === 'blocked' && ` ${t('audit.triage.blockedCount', { count: failed })}`}
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
                {t('audit.triage.aggregateRisk')}
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
            className="flex items-center gap-3 p-3 border-2 border-dashed border-pencil-light/30"
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
                {t('audit.triage.visibleFindings')}
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
            <Select
              label={t('audit.triage.minSeverity')}
              value={minSeverity}
              onChange={(value) => onSeverityChange(value)}
              size="sm"
              options={severityFilterOptions}
            />
          </div>
          <div className="flex items-center gap-4 flex-wrap py-1.5">
            {scanErrors > 0 && (
              <span className="text-danger text-sm flex items-center gap-1">
                <AlertTriangle size={14} strokeWidth={2.5} />
                {scanErrors !== 1 ? t('audit.triage.scanErrors', { count: scanErrors }) : t('audit.triage.scanError', { count: scanErrors })}
              </span>
            )}
            <p className="text-xs text-pencil-light">
              {t('audit.triage.helpText')}
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
  const t = useT();
  const maxSeverity = getMaxSeverity(result.findings);
  const iconColor = riskColor(result.riskLabel);
  const iconBg = riskBgColor(result.riskLabel);

  return (
    <Card className="!overflow-clip">
      {/* ── Sticky header: skill name (left) + block/risk indicators (right) ── */}
      <div
        className="sticky top-0 z-20 -mx-4 -mt-4 px-4 py-3 bg-surface border-b border-dashed border-pencil-light/30"
      >
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          {/* Left: skill icon + name + issue count */}
          <div className="flex items-center gap-2.5 min-w-0">
            <div
              className="w-8 h-8 flex items-center justify-center border-2 shrink-0"
              style={{ borderRadius: radius.sm, borderColor: iconColor, backgroundColor: iconBg, color: iconColor }}
            >
              {result.isBlocked ? (
                <ShieldAlert size={16} strokeWidth={2.5} />
              ) : (
                <ShieldCheck size={16} strokeWidth={2.5} />
              )}
            </div>
            {result.kind && <KindBadge kind={result.kind} />}
            <span
              className="font-bold text-pencil text-lg truncate"
            >
              {result.skillName}
            </span>
            <Badge variant={severityBadgeVariant(maxSeverity)}>
              {result.findings.length !== 1 ? t('audit.finding.issues', { count: result.findings.length }) : t('audit.finding.issue', { count: result.findings.length })}
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
      </div>

      {/* ── Findings list ── */}
      <div className="space-y-2 pt-3">
        {result.findings.map((f, i) => (
          <FindingRow key={`${f.file}-${f.line}-${i}`} finding={f} />
        ))}
      </div>
    </Card>
  );
}

/* BlockStamp and RiskMeter imported from ../components/audit */

/* ──────────────────────────────────────────────────────────────────────
 * FindingRow — severity dot + badge for visual hierarchy
 * ────────────────────────────────────────────────────────────────────── */

function FindingRow({ finding }: { finding: AuditFinding }) {
  const dotColor = severityStripeColor(finding.severity);

  return (
    <div
      className="flex flex-col gap-1.5 text-sm pl-3 py-2 transition-colors duration-100 hover:bg-paper-warm/60 rounded-md"
    >
      <div className="flex items-start gap-2 flex-wrap">
        <span
          className="w-2 h-2 rounded-full shrink-0 mt-1.5"
          style={{ backgroundColor: dotColor }}
        />
        <Badge variant={severityBadgeVariant(finding.severity)}>
          {finding.severity}
        </Badge>
        <span className="text-pencil flex-1">{finding.message}</span>
      </div>
      <span
        className="font-mono text-xs text-pencil-light"
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
            className="font-mono text-xs text-pencil-light block px-3 py-2 border-2 border-dashed border-muted overflow-x-auto bg-paper-warm"
            style={{
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
