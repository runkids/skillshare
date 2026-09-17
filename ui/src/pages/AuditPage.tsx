import { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertCircle, Bot, Puzzle, ShieldCheck } from 'lucide-react';
import { api, type AuditFinding, type AuditResult } from '../api/client';
import Button from '../components/Button';
import ConfirmDialog from '../components/ConfirmDialog';
import EmptyState from '../components/EmptyState';
import PageHeader from '../components/PageHeader';
import { Select } from '../components/Select';
import Spinner from '../components/Spinner';
import { useToast } from '../components/Toast';
import { formatRelativeTime, useI18n, useT } from '../i18n';
import { getCachedAuditResult } from '../lib/auditCache';
import { queryKeys, staleTimes } from '../lib/queryKeys';

type Kind = 'skills' | 'agents';
type Severity = AuditFinding['severity'];

const SEVERITIES: Severity[] = ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW', 'INFO'];
const SEV_CLASS: Record<Severity, string> = { CRITICAL: 'c', HIGH: 'h', MEDIUM: 'md', LOW: 'l', INFO: 'n' };
const rank = (s: Severity) => SEVERITIES.indexOf(s);
// Cross-skill analysis is a derived insight, not a scanned resource.
const CROSS_SKILL = '_cross-skill';
// LOW and INFO are the long tail; they stay behind a per-resource expander.
const MINOR = rank('MEDIUM');

export default function AuditPage() {
  const t = useT();
  const { locale } = useI18n();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const overview = useQuery({ queryKey: queryKeys.overview, queryFn: () => api.getOverview(), staleTime: staleTimes.overview });

  const [kind, setKind] = useState<Kind>('skills');
  const [only, setOnly] = useState<Severity | null>(null);
  const [open, setOpen] = useState<Set<string>>(new Set());
  const [scanning, setScanning] = useState(false);
  const [progress, setProgress] = useState<{ scanned: number; total: number } | null>(null);
  const [error, setError] = useState('');
  const [ignoring, setIgnoring] = useState<AuditFinding | null>(null);
  const stream = useRef<EventSource | null>(null);
  const [, redraw] = useState(0);

  useEffect(() => () => stream.current?.close(), []);

  const counts = { skills: overview.data?.skillCount, agents: overview.data?.agentCount };
  const installed = counts[kind];
  const data = getCachedAuditResult(queryClient, kind, installed);
  const scannedAt = queryClient.getQueryState(queryKeys.audit.all(kind))?.dataUpdatedAt;

  const scan = () => {
    setScanning(true);
    setError('');
    setProgress(null);
    stream.current = api.auditAllStream(
      (total) => setProgress({ scanned: 0, total }),
      (scanned) => setProgress((p) => (p ? { ...p, scanned } : null)),
      (res) => {
        queryClient.setQueryData(queryKeys.audit.all(kind), res);
        redraw((n) => n + 1);
        setScanning(false);
        setProgress(null);
        if (res.summary.failed > 0) toast(t(res.summary.failed === 1 ? 'audit.toast.blocked.one' : 'audit.toast.blocked.other', { count: res.summary.failed, threshold: res.summary.threshold }), 'warning');
        else if (res.summary.total > 0 && res.results.every((r) => r.findings.length === 0)) toast(t('audit.toast.clean'), 'success');
      },
      (err) => {
        setError(err.message);
        setScanning(false);
        setProgress(null);
      },
      kind,
    );
  };

  const setThreshold = async (value: string) => {
    try {
      await api.setAuditThreshold(value);
      toast(t('audit.threshold.saved', { threshold: value }), 'success');
      scan(); // what counts as blocked moves with the threshold, so the results need re-scoring
    } catch (err) {
      toast((err as Error).message, 'error');
    }
  };

  const ignoreRule = async (finding: AuditFinding) => {
    try {
      await api.toggleRule({ id: finding.ruleId, pattern: finding.pattern, enabled: false });
      toast(t('audit.ignore.done'), 'success');
      scan();
    } catch (err) {
      toast((err as Error).message, 'error');
    }
  };

  const results = (data?.results ?? [])
    .filter((r) => r.skillName !== CROSS_SKILL && r.findings.length > 0)
    .map((r) => ({ ...r, findings: [...r.findings].sort((a, b) => rank(a.severity) - rank(b.severity)) }))
    .filter((r) => !only || r.findings.some((f) => f.severity === only))
    .sort((a, b) => rank(a.findings[0].severity) - rank(b.findings[0].severity) || b.riskScore - a.riskScore);
  const findingCount = (data?.results ?? []).reduce((n, r) => n + (r.skillName === CROSS_SKILL ? 0 : r.findings.length), 0);

  const header = (
    <PageHeader
      className="!mb-0"
      title={t('audit.header.title')}
      subtitle={t('audit.header.subtitle')}
      actions={
        <>
          {data && scannedAt ? (
            <span className="text-[13px] text-ink-3">
              {t(kind === 'agents' ? 'audit.scanned.agents' : 'audit.scanned.skills', { count: data.summary.total, when: formatRelativeTime(scannedAt, locale) })}
            </span>
          ) : null}
          <Button variant={data ? 'secondary' : 'primary'} onClick={scan} loading={scanning} disabled={scanning || installed === 0}>
            {!scanning && <ShieldCheck size={15} />}
            {t(data ? 'audit.scanAgain' : 'audit.scan')}
          </Button>
        </>
      }
    />
  );

  return (
    <div className="ss-wrap animate-fade-in">
      {header}

      <nav className="ss-tabs" aria-label={t('audit.header.title')} data-tour="audit-summary">
        <Link to="/audit" className="on" aria-current="page">
          {t('audit.tab.findings')}
          {findingCount > 0 && <span className="ss-cnt">{findingCount}</span>}
        </Link>
        <Link to="/audit/rules">{t('audit.tab.rules')}</Link>
      </nav>

      <div className="flex flex-wrap items-center gap-3">
        <div className="ss-seg" role="tablist">
          {(['skills', 'agents'] as Kind[]).map((k) => (
            <button key={k} type="button" role="tab" aria-selected={kind === k} className={kind === k ? 'on' : ''} disabled={scanning} onClick={() => setKind(k)}>
              {k === 'skills' ? <Puzzle size={14} /> : <Bot size={14} />}
              {t(k === 'agents' ? 'resources.tab.agents' : 'resources.tab.skills')}
              {counts[k] != null && <span className="ss-cnt">{counts[k]}</span>}
            </button>
          ))}
        </div>
        {data && (
          <>
            <span className="flex-1" />
            <span className="text-[13px] text-ink-2">{t('audit.overallRisk')}</span>
            <span className="ss-prog w-[120px]"><i style={{ width: `${data.summary.riskScore}%` }} /></span>
            <span className="font-mono text-[13px] font-semibold">{data.summary.riskScore} / 100</span>
          </>
        )}
      </div>

      {scanning && (
        <div className="ss-note inf !items-center">
          <Spinner size="sm" />
          <span className="flex-1">{t(kind === 'agents' ? 'audit.scanning.agents' : 'audit.scanning.skills')}</span>
          {progress && progress.total > 0 && (
            <>
              <span className="ss-prog w-[160px]"><i style={{ width: `${(progress.scanned / progress.total) * 100}%` }} /></span>
              <span className="font-mono text-[13px]">{progress.scanned} / {progress.total}</span>
            </>
          )}
        </div>
      )}

      {error && <div className="ss-note bad"><AlertCircle size={16} /><span className="flex-1 break-words">{error}</span></div>}

      {data && (
        <>
          <div className="flex items-stretch gap-6">
            <div className="ss-counts flex-1 !grid-cols-5">
              {SEVERITIES.map((s) => {
                const n = data.summary[s.toLowerCase() as 'critical' | 'high' | 'medium' | 'low' | 'info'];
                return (
                  <button
                    key={s}
                    type="button"
                    className="!py-3.5 text-left"
                    aria-pressed={only === s}
                    disabled={n === 0}
                    title={t(only === s ? 'audit.filter.clear' : 'audit.filter.only', { severity: s })}
                    onClick={() => setOnly(only === s ? null : s)}
                  >
                    <span className="flex flex-col items-start gap-1.5">
                      <b className={n === 0 ? 'text-ink-3' : only === s ? 'text-accent' : ''}>{n}</b>
                      <span className={`ss-sev ${SEV_CLASS[s]}`}>{s}</span>
                    </span>
                  </button>
                );
              })}
            </div>
            <div className="ss-box flex w-[300px] flex-col justify-center gap-2 !py-3.5">
              <span className="text-[13px] text-ink-2">{t('audit.threshold.label')}</span>
              <Select
                value={data.summary.threshold}
                onChange={setThreshold}
                disabled={scanning}
                options={SEVERITIES.map((s) => ({ value: s, label: t(s === 'CRITICAL' ? 'audit.threshold.criticalOnly' : 'audit.threshold.andAbove', { severity: s }) }))}
              />
            </div>
          </div>

          {findingCount === 0 ? (
            <EmptyState icon={ShieldCheck} title={t(kind === 'agents' ? 'audit.empty.clean.agents' : 'audit.empty.clean.skills')} description={t('audit.empty.clean.hint', { threshold: data.summary.threshold })} />
          ) : (
            <div className="ss-list">
              {results.map((r) => (
                <ResultGroup
                  key={r.skillName}
                  result={r}
                  kind={kind}
                  only={only}
                  expanded={open.has(r.skillName)}
                  onToggle={() => setOpen((s) => { const next = new Set(s); if (!next.delete(r.skillName)) next.add(r.skillName); return next; })}
                  onIgnore={setIgnoring}
                />
              ))}
            </div>
          )}

          <p className="flex flex-wrap items-center gap-x-4 gap-y-1.5 text-[13px] text-ink-2">
            {data.summary.passed > 0 && <span className="ss-st ok">{t(kind === 'agents' ? 'audit.passed.agents' : 'audit.passed.skills', { count: data.summary.passed })}</span>}
            {(data.summary.scanErrors ?? 0) > 0 && (
              <span className="ss-st bad">{t((data.summary.scanErrors ?? 0) === 1 ? 'audit.scanErrors.one' : 'audit.scanErrors.other', { count: data.summary.scanErrors })}</span>
            )}
          </p>
        </>
      )}

      {!data && !scanning && !error && (
        installed === 0 ? (
          <EmptyState icon={ShieldCheck} title={t(kind === 'agents' ? 'audit.empty.none.agents' : 'audit.empty.none.skills')} description={t(kind === 'agents' ? 'audit.empty.none.hintAgents' : 'audit.empty.none.hintSkills')} />
        ) : (
          <EmptyState
            icon={ShieldCheck}
            title={t(kind === 'agents' ? 'audit.empty.notScanned.agents' : 'audit.empty.notScanned.skills')}
            description={t('audit.empty.notScanned.hint')}
            action={<Button variant="primary" onClick={scan}><ShieldCheck size={15} />{t('audit.scan')}</Button>}
          />
        )
      )}

      <ConfirmDialog
        open={ignoring !== null}
        variant="danger"
        title={t('audit.ignore.title')}
        message={t('audit.ignore.message', { rule: ignoring?.ruleId || ignoring?.pattern || '' })}
        confirmText={t('audit.ignore.confirm')}
        onCancel={() => setIgnoring(null)}
        onConfirm={() => { const f = ignoring; setIgnoring(null); if (f) void ignoreRule(f); }}
      />
    </div>
  );
}

/** One resource with its findings. LOW and INFO stay collapsed unless the reader asks for them. */
function ResultGroup({ result, kind, only, expanded, onToggle, onIgnore }: {
  result: AuditResult;
  kind: Kind;
  only: Severity | null;
  expanded: boolean;
  onToggle: () => void;
  onIgnore: (f: AuditFinding) => void;
}) {
  const t = useT();
  const shown = only ? result.findings.filter((f) => f.severity === only) : result.findings.filter((f) => expanded || rank(f.severity) <= MINOR);
  const hidden = only ? 0 : result.findings.length - result.findings.filter((f) => rank(f.severity) <= MINOR).length;

  return (
    <>
      <div className="ss-gh !min-h-[46px]">
        <span className={`ss-cat sm ${kind === 'agents' ? 'agent' : 'skill'}`}>{kind === 'agents' ? <Bot size={14} /> : <Puzzle size={14} />}</span>
        <Link to={`/${kind === 'agents' ? 'agents' : 'skills'}/${encodeURIComponent(result.skillName)}`} className="truncate font-mono font-semibold">{result.skillName}</Link>
        {result.skillName.startsWith('_') && <span className="ss-tag">tracked</span>}
        <span className="flex-1" />
        <span className="font-mono text-[12px] text-ink-3">{t('audit.risk', { score: result.riskScore })}</span>
        {result.isBlocked && <span className="ss-st bad">{t('audit.blocked')}</span>}
      </div>
      {shown.map((f, i) => (
        <div key={`${f.file}:${f.line}:${i}`} className="ss-r !items-start !py-3.5">
          <span className="w-[74px] shrink-0 pt-px"><span className={`ss-sev ${SEV_CLASS[f.severity]}`}>{f.severity}</span></span>
          <div className="flex min-w-0 flex-1 flex-col gap-2">
            <span className="text-[13.5px] font-semibold">{f.message}</span>
            {f.snippet && (
              <div className="ss-code max-w-full overflow-x-auto !px-3.5 !py-2.5">
                <span className="ln">{f.line}</span>{f.snippet}
              </div>
            )}
            <span className="truncate font-mono text-[12px] text-ink-3" title={f.file}>
              {f.file}:{f.line}{(f.ruleId || f.pattern) && ` · ${t('audit.rule', { rule: f.ruleId || f.pattern })}`}
            </span>
          </div>
          {(f.ruleId || f.pattern) && (
            <Button variant="ghost" size="sm" onClick={() => onIgnore(f)}>{t('audit.ignore.button')}</Button>
          )}
        </div>
      ))}
      {hidden > 0 && (
        <button type="button" className="ss-r w-full text-left text-[13px] text-ink-2" aria-expanded={expanded} onClick={onToggle}>
          <span className="ml-[74px]">{t(expanded ? 'audit.minor.hide' : 'audit.minor.show', { count: hidden })}</span>
        </button>
      )}
    </>
  );
}
