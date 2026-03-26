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
} from 'lucide-react';
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
import { radius, palette } from '../design';

/* ──────────────────────────────────────────────────────────────────────
 * Helpers
 * ────────────────────────────────────────────────────────────────────── */

function formatTokens(n: number): string {
  return `~${n.toLocaleString()}`;
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
  const [data, setData] = useState<AnalyzeResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedGroupIdx, setSelectedGroupIdx] = useState(0);
  const [lintFilter, setLintFilter] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await api.analyze();
      setData(res);
      setSelectedGroupIdx(0);
    } catch (err: any) {
      setError(err.message ?? 'Failed to load analysis');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const groups = useMemo(() => (data ? buildGroups(data.targets) : []), [data]);

  const activeGroup = groups[selectedGroupIdx] ?? null;
  const activeTarget = activeGroup?.target ?? null;

  // Subtitle
  const subtitle = useMemo(() => {
    if (!activeGroup) return undefined;
    if (groups.length === 1) {
      return `Targets: ${activeGroup.names.join(', ')}`;
    }
    return `Targets: ${activeGroup.names.join(', ')}`;
  }, [activeGroup, groups]);

  // Loading
  if (loading) {
    return (
      <div className="space-y-6">
        <PageHeader
          icon={<BarChart3 size={24} strokeWidth={2.5} />}
          title="Analyze"
          subtitle="Loading skill analysis..."
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
          title="Analyze"
        />
        <Card>
          <div className="flex flex-col items-center gap-3 py-8">
            <p className="text-danger">{error}</p>
            <Button variant="secondary" size="sm" onClick={fetchData}>
              <RefreshCw size={14} strokeWidth={2.5} />
              Retry
            </Button>
          </div>
        </Card>
      </div>
    );
  }

  // Empty
  if (!data || data.targets.length === 0) {
    return (
      <div className="space-y-6">
        <PageHeader
          icon={<BarChart3 size={24} strokeWidth={2.5} />}
          title="Analyze"
          actions={
            <Button variant="secondary" size="sm" onClick={fetchData}>
              <RefreshCw size={14} strokeWidth={2.5} />
              Refresh
            </Button>
          }
        />
        <EmptyState
          icon={BarChart3}
          title="No targets configured"
          description="Configure targets and sync skills to see analysis data"
        />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <PageHeader
        icon={<BarChart3 size={24} strokeWidth={2.5} />}
        title="Analyze"
        subtitle={subtitle}
        actions={
          <Button variant="secondary" size="sm" onClick={fetchData}>
            <RefreshCw size={14} strokeWidth={2.5} />
            Refresh
          </Button>
        }
      />

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
  const totalIssues = useMemo(
    () => target.skills.reduce((sum, s) => sum + (s.lint_issues?.length ?? 0), 0),
    [target.skills],
  );

  const cards = [
    {
      label: 'Skills',
      value: target.skill_count,
      icon: <Package size={18} strokeWidth={2.5} />,
      color: 'text-success',
      accent: palette.success,
    },
    {
      label: 'Always-loaded',
      value: formatTokens(target.always_loaded.estimated_tokens),
      unit: 'tokens',
      sub: `${target.always_loaded.chars.toLocaleString()} chars`,
      icon: <Zap size={18} strokeWidth={2.5} />,
      color: 'text-info',
      accent: palette.info,
    },
    {
      label: 'On-demand max',
      value: formatTokens(target.on_demand_max.estimated_tokens),
      unit: 'tokens',
      sub: `${target.on_demand_max.chars.toLocaleString()} chars`,
      icon: <ToggleRight size={18} strokeWidth={2.5} />,
      color: 'text-pencil',
      accent: 'var(--color-pencil-light)',
    },
    {
      label: 'Quality issues',
      value: totalIssues,
      icon: totalIssues > 0
        ? <AlertTriangle size={18} strokeWidth={2.5} />
        : <CheckCircle size={18} strokeWidth={2.5} />,
      color: totalIssues > 0 ? 'text-warning' : 'text-success',
      accent: totalIssues > 0 ? palette.warning : palette.success,
    },
  ];

  return (
    <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
      {cards.map((c) => (
        <div
          key={c.label}
          className="p-4 border-2 border-dashed"
          style={{
            borderColor: c.accent,
            borderRadius: radius.md,
            boxShadow: 'var(--shadow-sm)',
            backgroundColor: `color-mix(in srgb, ${c.accent} 6%, var(--color-surface))`,
          }}
        >
          <div className="flex items-center gap-2 mb-2">
            <span className={c.color}>{c.icon}</span>
            <span className="text-xs text-pencil-light uppercase tracking-wide font-medium">{c.label}</span>
          </div>
          <p className={`text-2xl font-bold ${c.color}`}>
            {c.value}
            {c.unit && <span className="text-xs font-medium text-pencil-light ml-1">{c.unit}</span>}
          </p>
          {c.sub && <p className="text-xs text-pencil-light mt-0.5">{c.sub}</p>}
        </div>
      ))}
    </div>
  );
}

/* ──────────────────────────────────────────────────────────────────────
 * Top-10 Heaviest Skills Chart
 * ────────────────────────────────────────────────────────────────────── */

function TopHeaviestChart({ skills }: { skills: AnalyzeSkill[] }) {
  const { sorted, maxTokens } = useMemo(() => {
    const top = [...skills]
      .sort((a, b) => b.description_tokens - a.description_tokens)
      .slice(0, 10);
    const max = top[0]?.description_tokens ?? 1;
    return { sorted: top, maxTokens: max };
  }, [skills]);

  /** Rank-based transparent blue: #1 = 45% opacity, #10 = 25% opacity */
  function barStyle(rank: number, total: number): { backgroundColor: string } {
    const opacity = 0.45 - (rank / Math.max(total - 1, 1)) * 0.2; // 0.45 → 0.25
    return { backgroundColor: `rgba(45, 93, 161, ${opacity.toFixed(2)})` };
  }

  return (
    <Card>
      <h3 className="text-base font-bold text-pencil mb-4 flex items-center gap-2">
        <BarChart3 size={16} strokeWidth={2.5} className="text-info" />
        Top-10 Heaviest Skills
        <span className="text-xs font-normal text-pencil-light">(by description tokens)</span>
      </h3>
      {sorted.length === 0 ? (
        <p className="text-pencil-light text-sm py-4">No skills to display</p>
      ) : (
        <div className="space-y-1.5">
          {sorted.map((skill, idx) => {
            const pct = maxTokens > 0 ? (skill.description_tokens / maxTokens) * 100 : 0;
            const tooltipText = `Description: ${formatTokens(skill.description_tokens)} tokens\nBody: ${formatTokens(skill.body_tokens)} tokens\nTotal: ${formatTokens(skill.description_tokens + skill.body_tokens)} tokens`;
            return (
              <Tooltip key={skill.name} content={tooltipText}>
                <div className="flex items-center gap-2 group py-0.5">
                  <span className="text-xs text-pencil-light font-mono shrink-0 w-5 text-right">
                    {idx + 1}
                  </span>
                  <span
                    className="text-sm text-pencil truncate shrink-0"
                    style={{ width: '150px' }}
                  >
                    {skill.name}
                  </span>
                  <div className="flex-1 h-5 bg-muted/30 overflow-hidden" style={{ borderRadius: radius.sm }}>
                    <div
                      className="h-full transition-all duration-500"
                      style={{
                        width: `${Math.max(pct, 3)}%`,
                        borderRadius: radius.sm,
                        ...barStyle(idx, sorted.length),
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
        Based on description token count across {skills.length} skills
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
        <AlertTriangle size={16} strokeWidth={2.5} className="text-warning" />
        Quality Issues
      </h3>
      {grouped.length === 0 ? (
        <div className="flex flex-col items-center py-6 text-center">
          <CheckCircle size={28} strokeWidth={2} className="text-success mb-2" />
          <p className="text-sm text-success font-medium">All skills pass quality checks</p>
        </div>
      ) : (
        <>
          <p className="text-xs text-pencil-light mb-3">
            {totalIssues} issue{totalIssues !== 1 ? 's' : ''} in {grouped.length} categor{grouped.length !== 1 ? 'ies' : 'y'}
          </p>
          <div className="space-y-2">
            {grouped.map((g) => {
              const pct = (g.count / maxCount) * 100;
              const barColor = g.severity === 'error' ? palette.danger : palette.warning;
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
                        backgroundColor: `color-mix(in srgb, ${barColor} 50%, transparent)`,
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
      result = result.filter((s) => s.name.toLowerCase().includes(q));
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
  const [prevFiltered, setPrevFiltered] = useState(filtered);
  if (filtered !== prevFiltered) { setPrevFiltered(filtered); setPage(0); }

  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const start = page * pageSize;
  const visible = filtered.slice(start, start + pageSize);

  return (
    <Card>
      {/* Toolbar */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center gap-3 mb-4">
        <h3 className="text-base font-bold text-pencil flex items-center gap-2">
          <FileText size={16} strokeWidth={2.5} className="text-info" />
          All Skills
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
                {lintFilter}
                <X size={12} strokeWidth={2.5} className="ml-1" />
              </Badge>
            </span>
          )}
          <div className="w-40">
            <Input
              placeholder="Search..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="!py-1.5 !px-3 !text-sm !border"
            />
          </div>
        </div>
      </div>

      {/* Table */}
      <div className="overflow-auto max-h-[calc(100vh-320px)]">
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
                ['name', 'Name'],
                ['desc', 'Desc Tokens'],
                ['body', 'Body Tokens'],
                ['total', 'Total'],
                ['issues', 'Issues'],
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
                <td colSpan={5} className="py-8 text-center text-pencil-light">
                  {search || lintFilter ? 'No skills match the current filters' : 'No skills found'}
                </td>
              </tr>
            ) : (
              visible.map((skill) => {
                const issueCount = skill.lint_issues?.length ?? 0;
                const total = skill.description_tokens + skill.body_tokens;
                return (
                  <tr
                    key={skill.name}
                    className="border-b border-dashed border-muted cursor-pointer hover:bg-paper-warm/60 transition-colors"
                    onClick={() => setSelectedSkill(skill)}
                  >
                    <td className="py-3 pr-4 font-medium text-pencil truncate">
                      <div className="flex items-center gap-2">
                        <span className="truncate">{skill.name}</span>
                        {skill.is_tracked && <Badge variant="info" size="sm">tracked</Badge>}
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
  if (!skill) return null;

  const total = skill.description_tokens + skill.body_tokens;
  const totalChars = skill.description_chars + skill.body_chars;

  return (
    <DialogShell open={!!skill} onClose={onClose} maxWidth="2xl">
      <Card>
        {/* Header */}
        <div className="flex items-start justify-between mb-4">
          <div>
            <h3 className="text-lg font-bold text-pencil flex items-center gap-2">
              {skill.name}
              {skill.is_tracked && <Badge variant="info" size="sm">tracked</Badge>}
            </h3>
            <p className="text-xs text-pencil-light font-mono mt-1">{skill.path}</p>
          </div>
          <button
            onClick={onClose}
            className="p-1 text-pencil-light hover:text-pencil transition-colors cursor-pointer"
          >
            <X size={18} strokeWidth={2.5} />
          </button>
        </div>

        {/* Token breakdown */}
        <div className="grid grid-cols-3 gap-3 mb-4">
          {[
            { label: 'Description', tokens: skill.description_tokens, chars: skill.description_chars, color: palette.info },
            { label: 'Body', tokens: skill.body_tokens, chars: skill.body_chars, color: palette.warning },
            { label: 'Total', tokens: total, chars: totalChars, color: palette.success },
          ].map((item) => (
            <div
              key={item.label}
              className="p-2.5 border-2 border-dashed"
              style={{
                borderRadius: radius.sm,
                borderColor: `color-mix(in srgb, ${item.color} 30%, transparent)`,
                backgroundColor: `color-mix(in srgb, ${item.color} 4%, transparent)`,
              }}
            >
              <p className="text-xs text-pencil-light uppercase tracking-wide">{item.label}</p>
              <p className="text-base font-bold text-pencil">{formatTokens(item.tokens)}</p>
              <p className="text-xs text-pencil-light">{item.chars.toLocaleString()} chars</p>
            </div>
          ))}
        </div>

        {/* Details */}
        {skill.targets && skill.targets.length > 0 && (
          <p className="text-sm text-pencil-light mb-4">
            <span className="text-pencil-light/60">Restricted to:</span>{' '}
            {skill.targets.join(', ')}
          </p>
        )}

        {/* Quality issues */}
        {skill.lint_issues && skill.lint_issues.length > 0 && (
          <div className="mb-4">
            <p className="text-xs text-pencil-light uppercase tracking-wide mb-2">Quality Issues</p>
            <div className="space-y-1.5">
              {skill.lint_issues.map((issue, idx) => (
                <LintIssueRow key={`${issue.rule}-${idx}`} issue={issue} onRuleClick={onLintFilter} />
              ))}
            </div>
          </div>
        )}

        {/* Description preview */}
        {skill.description && (
          <div>
            <p className="text-xs text-pencil-light uppercase tracking-wide mb-1">Description Preview</p>
            <p className="text-sm text-pencil-light">{skill.description}</p>
          </div>
        )}
      </Card>
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
