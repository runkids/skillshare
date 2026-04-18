import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  BarChart3,
  RefreshCw,
  AlertTriangle,
  CheckCircle,
  XCircle,
  Package,
  Zap,
  ToggleRight,
  X,
  FileText,
  HelpCircle,
  ExternalLink,
  Filter,
  SearchX,
} from 'lucide-react';
import { Link } from 'react-router-dom';
import { api } from '../api/client';
import type { AnalyzeResponse, AnalyzeTarget, AnalyzeSkill, AnalyzeLintIssue } from '../api/client';
import Card from '../components/Card';
import Button from '../components/Button';
import PageHeader from '../components/PageHeader';
import Badge from '../components/Badge';
import EmptyState from '../components/EmptyState';
import { PageSkeleton } from '../components/Skeleton';
import SegmentedControl from '../components/SegmentedControl';
import { Input } from '../components/Input';
import Tooltip from '../components/Tooltip';
import Pagination from '../components/Pagination';
import DialogShell from '../components/DialogShell';
import { radius } from '../design';
import { useT } from '../i18n';

/* ──────────────────────────────────────────────────────────────────────
 * Helpers
 * ────────────────────────────────────────────────────────────────────── */

function formatTokens(n: number): string {
  return `~${n.toLocaleString()}`;
}

/** Small colored icon block (matches Audit page pattern) */
function IconBlock({ children, className }: { children: React.ReactNode; className: string }) {
  return (
    <span
      className={`inline-flex items-center justify-center w-7 h-7 border-2 shrink-0 ${className}`}
      style={{ borderRadius: radius.sm }}
    >
      {children}
    </span>
  );
}

/** Convert kebab-case rule name to readable label */
function readableRule(rule: string): string {
  return rule.replace(/-/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
}

type SortKey = 'name' | 'desc' | 'body' | 'total' | 'issues';
type SortDir = 'asc' | 'desc';

/** Merge targets with identical fingerprints into groups */
interface TargetGroup {
  names: string[];
  target: AnalyzeTarget;
}

function buildGroups(targets: AnalyzeTarget[]): TargetGroup[] {
  const map = new Map<string, TargetGroup>();
  for (const t of targets) {
    const key = `${t.skill_count}|${t.always_loaded.chars}|${t.on_demand_max.chars}`;
    const existing = map.get(key);
    if (existing) {
      existing.names.push(t.name);
    } else {
      map.set(key, { names: [t.name], target: t });
    }
  }
  return Array.from(map.values());
}

function lintIssueIcon(severity: string) {
  if (severity === 'error') return <XCircle size={14} strokeWidth={2.5} className="text-danger shrink-0" />;
  return <AlertTriangle size={14} strokeWidth={2.5} className="text-warning shrink-0" />;
}

/* ──────────────────────────────────────────────────────────────────────
 * Main Page
 * ────────────────────────────────────────────────────────────────────── */

export default function AnalyzePage() {
  const t = useT();
  const [data, setData] = useState<AnalyzeResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedGroupIdx, setSelectedGroupIdx] = useState(0);
  const [lintFilter, setLintFilter] = useState<string | null>(null);
  const [showHelp, setShowHelp] = useState(false);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await api.analyze();
      setData(res);
      setSelectedGroupIdx(0);
    } catch (err: any) {
      setError(err.message ?? t('analyze.error.failedToLoad'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const groups = useMemo(() => (data ? buildGroups(data.targets ?? []) : []), [data]);

  const activeGroup = groups[selectedGroupIdx] ?? null;
  const activeTarget = activeGroup?.target ?? null;

  // Subtitle
  const subtitle = activeGroup ? `Targets: ${activeGroup.names.join(', ')}` : undefined;

  // Loading
  if (loading) {
    return (
      <div className="space-y-6">
        <PageHeader
          icon={<BarChart3 size={24} strokeWidth={2.5} />}
          title={t('analyze.title')}
          subtitle={t('analyze.loading')}
        />
        <PageSkeleton />
      </div>
    );
  }

  // Error
  if (error) {
    return (
      <div className="space-y-6">
        <PageHeader
          icon={<BarChart3 size={24} strokeWidth={2.5} />}
          title={t('analyze.title')}
        />
        <Card>
          <div className="flex flex-col items-center gap-3 py-8">
            <p className="text-danger">{error}</p>
            <Button variant="secondary" size="sm" onClick={fetchData}>
              <RefreshCw size={14} strokeWidth={2.5} />
              {t('analyze.retry')}
            </Button>
          </div>
        </Card>
      </div>
    );
  }

  // Empty
  if (!data || !data.targets?.length) {
    return (
      <div className="space-y-6">
        <PageHeader
          icon={<BarChart3 size={24} strokeWidth={2.5} />}
          title={t('analyze.title')}
          actions={
            <Button variant="secondary" size="sm" onClick={fetchData}>
              <RefreshCw size={14} strokeWidth={2.5} />
              {t('common.reloadPage')}
            </Button>
          }
        />
        <EmptyState
          icon={BarChart3}
          title={t('analyze.empty.title')}
          description={t('analyze.empty.description')}
        />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <PageHeader
        icon={<BarChart3 size={24} strokeWidth={2.5} />}
        title={t('analyze.title')}
        subtitle={subtitle}
        actions={
          <>
            <button
              onClick={() => setShowHelp(true)}
              className="inline-flex items-center justify-center w-8 h-8 text-pencil-light hover:text-pencil hover:bg-muted/50 transition-colors cursor-pointer rounded-[var(--radius-full)]"
            >
              <HelpCircle size={18} strokeWidth={2} />
            </button>
            <Button variant="secondary" size="sm" onClick={fetchData}>
              <RefreshCw size={14} strokeWidth={2.5} />
              {t('backup.actions.refresh')}
            </Button>
          </>
        }
      />

      {/* Help dialog */}
      <DialogShell open={showHelp} onClose={() => setShowHelp(false)} maxWidth="md">
        <div className="flex items-center justify-between mb-5">
          <div className="flex items-center gap-2">
            <HelpCircle size={20} strokeWidth={2.5} className="text-pencil" />
            <h2 className="text-lg font-bold text-pencil">{t('analyze.dialog.aboutTitle')}</h2>
          </div>
          <button onClick={() => setShowHelp(false)} className="w-8 h-8 flex items-center justify-center text-pencil-light hover:text-pencil transition-colors cursor-pointer" aria-label={t('common.close')}>
            <X size={18} strokeWidth={2.5} />
          </button>
        </div>
        <div className="space-y-3 text-sm text-pencil-light">
          <p>{t('analyze.dialog.intro')}</p>
          <ul className="space-y-2 list-disc pl-4">
            <li><span className="text-pencil font-medium">{t('analyze.dialog.alwaysLoaded')}</span> — {t('analyze.dialog.alwaysLoadedDesc')}</li>
            <li><span className="text-pencil font-medium">{t('analyze.dialog.onDemandMax')}</span> — {t('analyze.dialog.onDemandMaxDesc')}</li>
            <li><span className="text-pencil font-medium">{t('analyze.dialog.top10Chart')}</span> — {t('analyze.dialog.top10ChartDesc')}</li>
            <li><span className="text-pencil font-medium">{t('analyze.dialog.qualityIssues')}</span> — {t('analyze.dialog.qualityIssuesDesc')}</li>
          </ul>
          <p>{t('analyze.dialog.skillDetail')}</p>
        </div>
        <p className="mt-4 pt-3 border-t border-dashed border-pencil-light/30 text-xs text-pencil-light">
          {t('analyze.dialog.tokenDisclaimer')}
        </p>
      </DialogShell>

      {/* Target group switching */}
      {groups.length > 1 && (
        <SegmentedControl
          value={String(selectedGroupIdx)}
          onChange={(v) => setSelectedGroupIdx(Number(v))}
          options={groups.map((g, i) => ({
            value: String(i),
            label: g.names.join(', '),
            count: g.target.skill_count,
          }))}
        />
      )}

      {activeTarget && (
        <>
          {/* Summary cards */}
          <SummaryCards target={activeTarget} />

          {/* Chart + Lint summary */}
          <TopHeaviestChart skills={activeTarget.skills} />
          <LintSummary skills={activeTarget.skills} onRuleClick={setLintFilter} />

          {/* Full skill table */}
          <SkillTable skills={activeTarget.skills} lintFilter={lintFilter} onLintFilterChange={setLintFilter} />
        </>
      )}
    </div>
  );
}

/* ──────────────────────────────────────────────────────────────────────
 * Summary Cards (4)
 * ────────────────────────────────────────────────────────────────────── */

function SummaryCards({ target }: { target: AnalyzeTarget }) {
  const t = useT();
  const totalIssues = useMemo(
    () => target.skills.reduce((sum, s) => sum + (s.lint_issues?.length ?? 0), 0),
    [target.skills],
  );

  const cards = [
    {
      label: t('analyze.summary.skills'),
      value: target.skill_count,
      icon: <Package size={18} strokeWidth={2.5} />,
      color: 'text-success',
      iconClass: 'bg-success-light text-success border-success',
    },
    {
      label: t('analyze.summary.alwaysLoaded'),
      value: formatTokens(target.always_loaded.estimated_tokens),
      unit: t('analyze.summary.tokens'),
      sub: `${target.always_loaded.chars.toLocaleString()} chars`,
      icon: <Zap size={18} strokeWidth={2.5} />,
      color: 'text-info',
      iconClass: 'bg-info-light text-info border-info',
    },
    {
      label: t('analyze.summary.onDemandMax'),
      value: formatTokens(target.on_demand_max.estimated_tokens),
      unit: t('analyze.summary.tokens'),
      sub: `${target.on_demand_max.chars.toLocaleString()} chars`,
      icon: <ToggleRight size={18} strokeWidth={2.5} />,
      color: 'text-pencil',
      iconClass: 'bg-paper text-muted-dark border-muted-dark',
    },
    {
      label: t('analyze.summary.qualityIssues'),
      value: totalIssues,
      icon: totalIssues > 0
        ? <AlertTriangle size={18} strokeWidth={2.5} />
        : <CheckCircle size={18} strokeWidth={2.5} />,
      color: totalIssues > 0 ? 'text-warning' : 'text-success',
      iconClass: totalIssues > 0 ? 'bg-warning-light text-warning border-warning' : 'bg-success-light text-success border-success',
    },
  ];

  return (
    <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
      {cards.map((c) => (
        <Card key={c.label} padding="none">
          <div className="flex items-center gap-3 px-3.5 py-2.5 h-full">
            <div
              className={`w-10 h-10 flex items-center justify-center border-2 shrink-0 ${c.iconClass}`}
              style={{ borderRadius: radius.sm }}
            >
              {c.icon}
            </div>
            <div className="min-w-0">
              <p className="text-xs text-pencil-light uppercase tracking-wide font-medium">{c.label}</p>
              <p className={`text-lg font-bold ${c.color}`}>
                {c.value}
                {c.unit && <span className="text-xs font-medium text-pencil-light ml-1">{c.unit}</span>}
              </p>
              {c.sub && <p className="text-xs text-pencil-light">{c.sub}</p>}
            </div>
          </div>
        </Card>
      ))}
    </div>
  );
}

/* ──────────────────────────────────────────────────────────────────────
 * Top-10 Heaviest Skills Chart
 * ────────────────────────────────────────────────────────────────────── */

function TopHeaviestChart({ skills }: { skills: AnalyzeSkill[] }) {
  const t = useT();
  const { sorted, maxTokens } = useMemo(() => {
    const top = [...skills]
      .sort((a, b) => b.description_tokens - a.description_tokens)
      .slice(0, 10);
    const max = top[0]?.description_tokens ?? 1;
    return { sorted: top, maxTokens: max };
  }, [skills]);


  return (
    <Card>
      <h3 className="text-base font-bold text-pencil mb-4 flex items-center gap-2">
        <IconBlock className="bg-info-light text-info border-info"><BarChart3 size={14} strokeWidth={2.5} /></IconBlock>
        {t('analyze.chart.title')}
        <span className="text-xs font-normal text-pencil-light">{t('analyze.chart.titleSub')}</span>
      </h3>
      {sorted.length === 0 ? (
        <p className="text-pencil-light text-sm py-4">{t('analyze.chart.noSkills')}</p>
      ) : (
        <div className="space-y-1.5">
          {sorted.map((skill, idx) => {
            const pct = maxTokens > 0 ? (skill.description_tokens / maxTokens) * 100 : 0;
            const tooltipContent = (
              <div>
                <div>{skill.path}</div>
                <hr className="border-paper/30 my-1" />
                <div>Description: {formatTokens(skill.description_tokens)} tokens</div>
                <div>Body: {formatTokens(skill.body_tokens)} tokens</div>
                <div>Total: {formatTokens(skill.description_tokens + skill.body_tokens)} tokens</div>
              </div>
            );
            return (
              <Tooltip key={skill.name} content={tooltipContent} followCursor>
                <div className="flex items-center gap-2 group py-0.5 px-1 -mx-1 rounded-[var(--radius-sm)] hover:bg-info/8 transition-colors cursor-default">
                  <span className="text-xs text-pencil-light font-mono shrink-0 w-5 text-right">
                    {idx + 1}
                  </span>
                  <span
                    className="text-sm text-pencil truncate shrink-0"
                    style={{ width: '220px' }}
                  >
                    {skill.path}
                  </span>
                  <div className="flex-1 h-5 bg-muted/30 overflow-hidden" style={{ borderRadius: radius.sm }}>
                    <div
                      className="h-full transition-all duration-500"
                      style={{
                        width: `${Math.max(pct, 3)}%`,
                        borderRadius: radius.sm,
                        backgroundColor: 'var(--color-info)',
                        opacity: 0.35,
                      }}
                    />
                  </div>
                  <span className="text-xs text-pencil-light font-mono shrink-0 w-12 text-right">
                    {formatTokens(skill.description_tokens)}
                  </span>
                </div>
              </Tooltip>
            );
          })}
        </div>
      )}
      {/* Footer info */}
      <p className="text-xs text-pencil-light mt-4 pt-3 border-t border-dashed border-pencil-light/20">
        {t('analyze.chart.basedOn', { count: skills.length })}
      </p>
    </Card>
  );
}

/* ──────────────────────────────────────────────────────────────────────
 * Lint Issues Summary
 * ────────────────────────────────────────────────────────────────────── */

interface LintSummaryProps {
  skills: AnalyzeSkill[];
  onRuleClick?: (rule: string) => void;
}

function LintSummary({ skills, onRuleClick }: LintSummaryProps) {
  const t = useT();
  const { grouped, maxCount, totalIssues } = useMemo(() => {
    const map = new Map<string, { rule: string; severity: 'error' | 'warning'; count: number }>();
    for (const skill of skills) {
      for (const issue of skill.lint_issues ?? []) {
        const existing = map.get(issue.rule);
        if (existing) {
          existing.count++;
        } else {
          map.set(issue.rule, { rule: issue.rule, severity: issue.severity, count: 1 });
        }
      }
    }
    const items = Array.from(map.values()).sort((a, b) => b.count - a.count);
    const max = items[0]?.count ?? 1;
    const total = items.reduce((sum, g) => sum + g.count, 0);
    return { grouped: items, maxCount: max, totalIssues: total };
  }, [skills]);

  return (
    <Card>
      <h3 className="text-base font-bold text-pencil mb-1 flex items-center gap-2">
        <IconBlock className="bg-warning-light text-warning border-warning"><AlertTriangle size={14} strokeWidth={2.5} /></IconBlock>
        {t('analyze.lint.title')}
      </h3>
      {grouped.length === 0 ? (
        <div className="flex flex-col items-center py-6 text-center">
          <CheckCircle size={28} strokeWidth={2} className="text-success mb-2" />
          <p className="text-sm text-success font-medium">{t('analyze.lint.allPassed')}</p>
        </div>
      ) : (
        <>
          <p className="text-xs text-pencil-light mb-3">
            {t('analyze.lint.issueCount', {
              count: totalIssues,
              s: totalIssues !== 1 ? 's' : '',
              cats: grouped.length,
              ies: grouped.length !== 1 ? 'ies' : 'y',
            })}
          </p>
          <div className="space-y-2">
            {grouped.map((g) => {
              const pct = (g.count / maxCount) * 100;
              return (
                <button
                  key={g.rule}
                  onClick={() => onRuleClick?.(g.rule)}
                  className="w-full text-left cursor-pointer group"
                >
                  <div className="flex items-center gap-2 mb-0.5">
                    {lintIssueIcon(g.severity)}
                    <span className="text-sm text-pencil flex-1 truncate group-hover:text-info transition-colors">{readableRule(g.rule)}</span>
                    <Badge variant={g.severity === 'error' ? 'danger' : 'warning'} size="sm">
                      {g.count}
                    </Badge>
                  </div>
                  <div className="h-1.5 bg-muted/30 overflow-hidden ml-5" style={{ borderRadius: radius.full }}>
                    <div
                      className="h-full transition-all duration-300"
                      style={{
                        width: `${Math.max(pct, 4)}%`,
                        backgroundColor: g.severity === 'error' ? 'var(--color-danger-light)' : 'var(--color-warning-light)',
                        borderRadius: radius.full,
                      }}
                    />
                  </div>
                </button>
              );
            })}
          </div>
        </>
      )}
    </Card>
  );
}

/* ──────────────────────────────────────────────────────────────────────
 * Filtered Summary Bar
 * ────────────────────────────────────────────────────────────────────── */

function FilteredSummaryBar({ filtered, total }: { filtered: AnalyzeSkill[]; total: number }) {
  const t = useT();
  const summary = useMemo(() => {
    let descTokens = 0;
    let bodyTokens = 0;
    for (const s of filtered) {
      descTokens += s.description_tokens;
      bodyTokens += s.body_tokens;
    }
    return { descTokens, bodyTokens, totalTokens: descTokens + bodyTokens };
  }, [filtered]);

  const isActive = filtered.length !== total;

  const stats = [
    { label: t('analyze.filter.alwaysLabel'), value: formatTokens(summary.descTokens), icon: <Zap size={14} strokeWidth={2.5} />, colorClass: 'text-info', bgClass: 'bg-info-light border-info' },
    { label: t('analyze.filter.onDemandLabel'), value: formatTokens(summary.bodyTokens), icon: <ToggleRight size={14} strokeWidth={2.5} />, colorClass: 'text-pencil', bgClass: 'bg-paper border-muted-dark' },
    { label: t('analyze.filter.totalLabel'), value: formatTokens(summary.totalTokens), icon: <Package size={14} strokeWidth={2.5} />, colorClass: 'text-success', bgClass: 'bg-success-light border-success' },
  ];

  return (
    <div
      className="flex flex-wrap items-center gap-3 overflow-hidden transition-all duration-200 ease-out"
      style={{ maxHeight: isActive ? '48px' : '0px', opacity: isActive ? 1 : 0, marginBottom: isActive ? '16px' : '0px' }}
    >
      <div className="flex items-center gap-1.5 mr-1">
        <Filter size={16} className="text-pencil shrink-0" />
        <span className="text-sm font-medium text-pencil">
          {filtered.length}<span className="text-pencil-light font-normal">/{total}</span>
        </span>
      </div>
      {stats.map((s) => (
        <div
          key={s.label}
          className={`inline-flex items-center gap-1.5 px-2.5 py-1.5 border-2 ${s.bgClass} text-xs`}
          style={{ borderRadius: radius.sm }}
        >
          <span className={s.colorClass}>{s.icon}</span>
          <span className="text-pencil-light">{s.label}</span>
          <span className={`font-mono font-bold ${s.colorClass}`}>{s.value}</span>
        </div>
      ))}
    </div>
  );
}

/* ──────────────────────────────────────────────────────────────────────
 * Full Skill Table
 * ────────────────────────────────────────────────────────────────────── */

function SkillTable({
  skills,
  lintFilter,
  onLintFilterChange,
}: {
  skills: AnalyzeSkill[];
  lintFilter: string | null;
  onLintFilterChange: (rule: string | null) => void;
}) {
  const t = useT();
  const [sortKey, setSortKey] = useState<SortKey>('desc');
  const [sortDir, setSortDir] = useState<SortDir>('desc');
  const [search, setSearch] = useState('');
  const [selectedSkill, setSelectedSkill] = useState<AnalyzeSkill | null>(null);

  const toggleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortKey(key);
      setSortDir(key === 'name' ? 'asc' : 'desc');
    }
  };

  const filtered = useMemo(() => {
    let result = [...skills];

    // Search filter
    if (search) {
      const q = search.toLowerCase();
      result = result.filter((s) => {
        const searchField = s.path || s.name;
        return searchField.toLowerCase().includes(q);
      });
    }

    // Lint rule filter
    if (lintFilter) {
      result = result.filter((s) =>
        s.lint_issues?.some((i) => i.rule === lintFilter),
      );
    }

    // Sort
    result.sort((a, b) => {
      let cmp = 0;
      switch (sortKey) {
        case 'name':
          cmp = a.name.localeCompare(b.name);
          break;
        case 'desc':
          cmp = a.description_tokens - b.description_tokens;
          break;
        case 'body':
          cmp = a.body_tokens - b.body_tokens;
          break;
        case 'total':
          cmp = (a.description_tokens + a.body_tokens) - (b.description_tokens + b.body_tokens);
          break;
        case 'issues':
          cmp = (a.lint_issues?.length ?? 0) - (b.lint_issues?.length ?? 0);
          break;
      }
      return sortDir === 'asc' ? cmp : -cmp;
    });

    return result;
  }, [skills, search, lintFilter, sortKey, sortDir]);

  const sortIndicator = (key: SortKey) => {
    if (sortKey !== key) return <span className="text-muted-dark/40 ml-1 text-[10px]">{'\u25B2\u25BC'}</span>;
    return <span className="ml-1 text-info text-[10px]">{sortDir === 'asc' ? '\u25B2' : '\u25BC'}</span>;
  };

  const PAGE_SIZES = [10, 25, 50] as const;
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState<number>(25);

  // Reset to first page when filters change
  useEffect(() => { setPage(0); }, [search, lintFilter, sortKey, sortDir]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const start = page * pageSize;
  const visible = filtered.slice(start, start + pageSize);

  return (
    <Card>
      {/* Toolbar */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center gap-3 mb-4">
        <h3 className="text-base font-bold text-pencil flex items-center gap-2">
          <IconBlock className="bg-success-light text-success border-success"><FileText size={14} strokeWidth={2.5} /></IconBlock>
          {t('analyze.table.allSkills')}
          <span className="text-xs font-normal text-pencil-light">({filtered.length})</span>
        </h3>
        <div className="flex-1" />
        <div className="flex items-center gap-2 flex-wrap">
          {lintFilter && (
            <span
              className="inline-flex items-center gap-1 cursor-pointer"
              onClick={() => onLintFilterChange(null)}
            >
              <Badge variant="warning" size="md">
                {readableRule(lintFilter)}
                <X size={12} strokeWidth={2.5} className="ml-1" />
              </Badge>
            </span>
          )}
          <div className="w-40">
            <Input
              placeholder={t('analyze.table.searchPlaceholder')}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="!py-1.5 !px-3 !text-sm"
            />
          </div>
        </div>
      </div>

      {/* Filtered summary */}
      <FilteredSummaryBar filtered={filtered} total={skills.length} />

      {/* Table */}
      <div className="overflow-auto min-h-[420px] max-h-[calc(100vh-320px)]">
        <table className="w-full text-left table-fixed">
          <colgroup>
            <col />
            <col className="w-[130px]" />
            <col className="w-[130px]" />
            <col className="w-[110px]" />
            <col className="w-[90px]" />
          </colgroup>
          <thead className="sticky top-0 z-10 bg-surface">
            <tr className="border-b-2 border-dashed border-muted-dark">
              {([
                ['name', t('analyze.table.name')],
                ['desc', t('analyze.table.descTokens')],
                ['body', t('analyze.table.bodyTokens')],
                ['total', t('analyze.table.total')],
                ['issues', t('analyze.table.issues')],
              ] as [SortKey, string][]).map(([key, label]) => (
                <th
                  key={key}
                  className="pb-3 pr-4 text-pencil-light text-sm font-medium cursor-pointer select-none hover:text-pencil transition-colors"
                  onClick={() => toggleSort(key)}
                >
                  {label}
                  {sortIndicator(key)}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {visible.length === 0 ? (
              <tr>
                <td colSpan={5} className="py-6">
                  <EmptyState
                    icon={search || lintFilter ? SearchX : FileText}
                    title={search || lintFilter ? t('analyze.table.noMatch') : t('analyze.table.noSkills')}
                    description={search || lintFilter ? t('analyze.table.noMatchDesc') : undefined}
                  />
                </td>
              </tr>
            ) : (
              visible.map((skill) => {
                const issueCount = skill.lint_issues?.length ?? 0;
                const total = skill.description_tokens + skill.body_tokens;
                return (
                  <tr
                    key={skill.name}
                    className="border-b border-dashed border-muted cursor-pointer hover:bg-paper-warm/60 transition-colors group animate-in fade-in duration-150"
                    onClick={() => setSelectedSkill(skill)}
                  >
                    <td className="py-3 pr-4">
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-pencil truncate hover:underline">{skill.path}</span>
                        {skill.is_tracked && <Badge variant="info" size="sm">Tracked</Badge>}
                      </div>
                    </td>
                    <td className="py-3 pr-4 text-pencil-light font-mono text-xs">{formatTokens(skill.description_tokens)}</td>
                    <td className="py-3 pr-4 text-pencil-light font-mono text-xs">{formatTokens(skill.body_tokens)}</td>
                    <td className="py-3 pr-4 text-pencil font-mono text-xs font-medium">{formatTokens(total)}</td>
                    <td className="py-3">
                      {issueCount > 0 ? (
                        <Badge variant="warning" size="sm">{issueCount}</Badge>
                      ) : (
                        <Badge variant="success" size="sm">OK</Badge>
                      )}
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {filtered.length > PAGE_SIZES[0] && (
        <Pagination
          page={page}
          totalPages={totalPages}
          onPageChange={(p) => setPage(p)}
          rangeText={`${start + 1}–${Math.min(start + pageSize, filtered.length)} of ${filtered.length}`}
          pageSize={{
            value: pageSize,
            options: PAGE_SIZES,
            onChange: (s) => { setPageSize(s); setPage(0); },
          }}
        />
      )}

      {/* Skill detail dialog */}
      <SkillDetailDialog
        skill={selectedSkill}
        onClose={() => setSelectedSkill(null)}
        onLintFilter={(rule) => { onLintFilterChange(rule); setSelectedSkill(null); }}
      />
    </Card>
  );
}

/* ──────────────────────────────────────────────────────────────────────
 * Skill Detail Dialog
 * ────────────────────────────────────────────────────────────────────── */

function SkillDetailDialog({
  skill,
  onClose,
  onLintFilter,
}: {
  skill: AnalyzeSkill | null;
  onClose: () => void;
  onLintFilter: (rule: string) => void;
}) {
  const t = useT();
  if (!skill) return null;

  const total = skill.description_tokens + skill.body_tokens;
  const totalChars = skill.description_chars + skill.body_chars;

  return (
    <DialogShell open={!!skill} onClose={onClose} maxWidth="2xl" className="max-h-[80vh] overflow-y-auto">
      {/* Header */}
      <div className="flex items-center justify-between mb-5">
        <div className="flex items-center gap-2 min-w-0">
          <FileText size={20} strokeWidth={2.5} className="text-pencil shrink-0" />
          <h2 className="text-lg font-bold text-pencil truncate">{skill.path}</h2>
          {skill.is_tracked && <Badge variant="info" size="sm">Tracked</Badge>}
        </div>
        <button onClick={onClose} className="w-8 h-8 flex items-center justify-center text-pencil-light hover:text-pencil transition-colors cursor-pointer shrink-0" aria-label={t('common.close')}>
          <X size={18} strokeWidth={2.5} />
        </button>
      </div>

      {/* Token breakdown */}
      <div className="grid grid-cols-3 gap-3 mb-4">
        {[
          { label: t('analyze.breakdown.description'), tokens: skill.description_tokens, chars: skill.description_chars, bg: 'var(--color-info-light)' },
          { label: t('analyze.breakdown.body'), tokens: skill.body_tokens, chars: skill.body_chars, bg: 'var(--color-warning-light)' },
          { label: t('analyze.breakdown.total'), tokens: total, chars: totalChars, bg: 'var(--color-success-light)' },
        ].map((item) => (
          <div key={item.label} className="p-2.5" style={{ borderRadius: radius.sm, backgroundColor: item.bg }}>
            <p className="text-xs text-pencil-light uppercase tracking-wide">{item.label}</p>
            <p className="text-base font-bold text-pencil">{formatTokens(item.tokens)} <span className="text-xs font-medium text-pencil-light">{t('analyze.summary.tokens')}</span></p>
            <p className="text-xs text-pencil-light">{item.chars.toLocaleString()} chars</p>
          </div>
        ))}
      </div>

      {/* Details */}
      {skill.targets && skill.targets.length > 0 && (
        <p className="text-sm text-pencil-light mb-4">
          <span className="text-pencil-light/60">{t('analyze.dialog.restrictedTo')}</span>{' '}
          {skill.targets.join(', ')}
        </p>
      )}

      {/* Quality issues */}
      {skill.lint_issues && skill.lint_issues.length > 0 && (
        <div className="mb-4">
          <p className="text-xs text-pencil-light uppercase tracking-wide mb-2">{t('analyze.lint.title')}</p>
          <div className="space-y-1.5">
            {skill.lint_issues.map((issue, idx) => (
              <LintIssueRow key={`${issue.rule}-${idx}`} issue={issue} onRuleClick={onLintFilter} />
            ))}
          </div>
        </div>
      )}

      {/* Description preview */}
      {skill.description && (
        <>
          <p className="mt-4 pt-3 border-t border-dashed border-pencil-light/30 text-xs text-pencil-light uppercase tracking-wide mb-1">{t('analyze.dialog.descriptionPreview')}</p>
          <p className="text-sm text-pencil-light">{skill.description}</p>
        </>
      )}

      {/* Link to skill detail page */}
      <div className="mt-4 pt-3 border-t border-dashed border-pencil-light/30">
        <Link
          to={`/resources/${encodeURIComponent(skill.name)}`}
          className="inline-flex items-center gap-1.5 text-sm font-medium text-info hover:underline"
        >
          {t('analyze.dialog.viewSkillDetail')}
          <ExternalLink size={14} strokeWidth={2.5} />
        </Link>
      </div>
    </DialogShell>
  );
}

/* ──────────────────────────────────────────────────────────────────────
 * Lint Issue Row
 * ────────────────────────────────────────────────────────────────────── */

function LintIssueRow({ issue, onRuleClick }: { issue: AnalyzeLintIssue; onRuleClick: (rule: string) => void }) {
  return (
    <div className="flex items-start gap-2 text-sm">
      {lintIssueIcon(issue.severity)}
      <span className="text-pencil flex-1">{issue.message}</span>
      <span
        className="cursor-pointer hover:underline"
        onClick={(e) => {
          e.stopPropagation();
          onRuleClick(issue.rule);
        }}
      >
        <Badge variant={issue.severity === 'error' ? 'danger' : 'warning'} size="sm">
          {readableRule(issue.rule)}
        </Badge>
      </span>
    </div>
  );
}
