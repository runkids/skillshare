import { useState, useEffect, useMemo, useCallback } from 'react';
import { Link } from 'react-router-dom';
import {
  Save,
  FileCode,
  ShieldCheck,
  ArrowLeft,
  FilePlus,
  Search,
  ChevronRight,
  List,
  FileEdit,
  RotateCcw,
  Settings2,
  Eye,
  EyeOff,
  ChevronsDownUp,
  ChevronsUpDown,
} from 'lucide-react';
import CodeMirror from '@uiw/react-codemirror';
import { yaml } from '@codemirror/lang-yaml';
import { EditorView } from '@codemirror/view';
import { useQuery, useQueryClient, useMutation } from '@tanstack/react-query';
import Card from '../components/Card';
import HandButton from '../components/HandButton';
import Badge from '../components/Badge';
import ConfirmDialog from '../components/ConfirmDialog';
import { HandInput } from '../components/HandInput';
import EmptyState from '../components/EmptyState';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { api } from '../api/client';
import type { CompiledRule, PatternGroup } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { useAppContext } from '../context/AppContext';
import { handTheme } from '../lib/codemirror-theme';
import { wobbly, shadows } from '../design';

/* ──────────────────────────────────────────────────────────────────────
 * Constants & Types
 * ────────────────────────────────────────────────────────────────────── */

type SeverityTab = 'ALL' | 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' | 'INFO' | 'DISABLED';

const SEVERITY_TABS: { value: SeverityTab; label: string }[] = [
  { value: 'ALL', label: 'All' },
  { value: 'CRITICAL', label: 'Critical' },
  { value: 'HIGH', label: 'High' },
  { value: 'MEDIUM', label: 'Medium' },
  { value: 'LOW', label: 'Low' },
  { value: 'INFO', label: 'Info' },
  { value: 'DISABLED', label: 'Disabled' },
];

type ViewMode = 'structured' | 'yaml';

/* ──────────────────────────────────────────────────────────────────────
 * Color helpers
 * ────────────────────────────────────────────────────────────────────── */

function severityColor(sev: string): string {
  switch (sev) {
    case 'CRITICAL': return 'var(--color-danger)';
    case 'HIGH': return 'var(--color-warning)';
    case 'MEDIUM': return 'var(--color-blue)';
    case 'LOW': return 'var(--color-success)';
    case 'INFO': return 'var(--color-pencil-light)';
    default: return 'var(--color-pencil-light)';
  }
}

function severityBgColor(sev: string): string {
  switch (sev) {
    case 'CRITICAL': return 'var(--color-danger-light)';
    case 'HIGH': return 'var(--color-warning-light)';
    case 'MEDIUM': return 'var(--color-info-light)';
    case 'LOW': return 'var(--color-success-light)';
    case 'INFO': return 'transparent';
    default: return 'transparent';
  }
}

function severityBadgeVariant(sev: string): 'danger' | 'warning' | 'info' | 'default' {
  switch (sev) {
    case 'CRITICAL': return 'danger';
    case 'HIGH': return 'warning';
    case 'MEDIUM': return 'info';
    case 'LOW': return 'info';
    case 'INFO': return 'default';
    default: return 'default';
  }
}

/* ──────────────────────────────────────────────────────────────────────
 * Main Page
 * ────────────────────────────────────────────────────────────────────── */

export default function AuditRulesPage() {
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const { isProjectMode } = useAppContext();
  const [viewMode, setViewMode] = useState<ViewMode>('structured');
  const [activeTab, setActiveTab] = useState<SeverityTab>('ALL');
  const [search, setSearch] = useState('');
  const [expandedPatterns, setExpandedPatterns] = useState<Set<string>>(new Set());
  const [expandedRules, setExpandedRules] = useState<Set<string>>(new Set());
  const [showResetConfirm, setShowResetConfirm] = useState(false);

  // Compiled rules query (structured view)
  const compiled = useQuery({
    queryKey: queryKeys.audit.compiled,
    queryFn: () => api.getCompiledRules(),
    staleTime: staleTimes.auditRules,
  });

  // Raw YAML query (yaml editor view)
  const rawQuery = useQuery({
    queryKey: queryKeys.audit.rules,
    queryFn: () => api.getAuditRules(),
    staleTime: staleTimes.auditRules,
    enabled: viewMode === 'yaml',
  });

  // YAML editor state
  const [raw, setRaw] = useState('');
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [creating, setCreating] = useState(false);

  const extensions = useMemo(() => [yaml(), EditorView.lineWrapping, ...handTheme], []);

  useEffect(() => {
    if (rawQuery.data?.raw) {
      setRaw(rawQuery.data.raw);
      setDirty(false);
    }
  }, [rawQuery.data]);

  // Toggle mutation
  const toggleMutation = useMutation({
    mutationFn: (req: { id?: string; pattern?: string; enabled: boolean; severity?: string }) =>
      api.toggleRule(req),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.audit.compiled });
      queryClient.invalidateQueries({ queryKey: queryKeys.audit.rules });
    },
    onError: (e: Error) => {
      toast(e.message, 'error');
    },
  });

  // Reset mutation
  const resetMutation = useMutation({
    mutationFn: () => api.resetRules(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.audit.compiled });
      queryClient.invalidateQueries({ queryKey: queryKeys.audit.rules });
      toast('All custom rules reset to defaults.', 'success');
    },
    onError: (e: Error) => {
      toast(e.message, 'error');
    },
  });

  const handleToggleRule = useCallback(
    (id: string, enabled: boolean) => { toggleMutation.mutate({ id, enabled }); },
    [toggleMutation],
  );

  const handleTogglePattern = useCallback(
    (pattern: string, enabled: boolean) => { toggleMutation.mutate({ pattern, enabled }); },
    [toggleMutation],
  );

  const handleSetSeverity = useCallback(
    (id: string, severity: string) => { toggleMutation.mutate({ id, enabled: true, severity }); },
    [toggleMutation],
  );

  const handleSetPatternSeverity = useCallback(
    (pattern: string, severity: string) => { toggleMutation.mutate({ pattern, enabled: true, severity }); },
    [toggleMutation],
  );

  // Filter rules
  const filteredRules = useMemo(() => {
    if (!compiled.data) return [];
    let rules = compiled.data.rules;
    if (activeTab === 'DISABLED') {
      rules = rules.filter((r) => !r.enabled);
    } else if (activeTab !== 'ALL') {
      rules = rules.filter((r) => r.severity === activeTab);
    }
    if (search.trim()) {
      const q = search.toLowerCase();
      rules = rules.filter(
        (r) =>
          r.id.toLowerCase().includes(q) ||
          r.message.toLowerCase().includes(q) ||
          r.regex.toLowerCase().includes(q) ||
          r.pattern.toLowerCase().includes(q),
      );
    }
    return rules;
  }, [compiled.data, activeTab, search]);

  // Group filtered rules by pattern
  const groupedRules = useMemo(() => {
    const groups = new Map<string, CompiledRule[]>();
    for (const rule of filteredRules) {
      const list = groups.get(rule.pattern) ?? [];
      list.push(rule);
      groups.set(rule.pattern, list);
    }
    return [...groups.entries()].sort(([a], [b]) => a.localeCompare(b));
  }, [filteredRules]);

  // Tab counts
  const tabCounts = useMemo(() => {
    if (!compiled.data) return {};
    const rules = compiled.data.rules;
    return {
      ALL: rules.length,
      CRITICAL: rules.filter((r) => r.severity === 'CRITICAL').length,
      HIGH: rules.filter((r) => r.severity === 'HIGH').length,
      MEDIUM: rules.filter((r) => r.severity === 'MEDIUM').length,
      LOW: rules.filter((r) => r.severity === 'LOW').length,
      INFO: rules.filter((r) => r.severity === 'INFO').length,
      DISABLED: rules.filter((r) => !r.enabled).length,
    } as Record<string, number>;
  }, [compiled.data]);

  // Summary stats
  const stats = useMemo(() => {
    if (!compiled.data) return { total: 0, enabled: 0, disabled: 0, custom: 0, patterns: 0 };
    const rules = compiled.data.rules;
    return {
      total: rules.length,
      enabled: rules.filter((r) => r.enabled).length,
      disabled: rules.filter((r) => !r.enabled).length,
      custom: rules.filter((r) => r.source !== 'builtin').length,
      patterns: compiled.data.patterns.length,
    };
  }, [compiled.data]);

  const togglePatternExpanded = useCallback((pattern: string) => {
    setExpandedPatterns((prev) => {
      const next = new Set(prev);
      if (next.has(pattern)) next.delete(pattern);
      else next.add(pattern);
      return next;
    });
  }, []);

  const toggleRuleExpanded = useCallback((id: string) => {
    setExpandedRules((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const expandAll = useCallback(() => {
    setExpandedPatterns(new Set(groupedRules.map(([p]) => p)));
  }, [groupedRules]);

  const collapseAll = useCallback(() => {
    setExpandedPatterns(new Set());
    setExpandedRules(new Set());
  }, []);

  const allExpanded = groupedRules.length > 0 && expandedPatterns.size === groupedRules.length;

  // YAML editor handlers
  const handleChange = (value: string) => {
    setRaw(value);
    setDirty(value !== (rawQuery.data?.raw ?? ''));
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      await api.putAuditRules(raw);
      toast('Audit rules saved successfully.', 'success');
      setDirty(false);
      queryClient.invalidateQueries({ queryKey: queryKeys.audit.rules });
      queryClient.invalidateQueries({ queryKey: queryKeys.audit.compiled });
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  };

  const handleCreate = async () => {
    setCreating(true);
    try {
      await api.initAuditRules();
      toast('Audit rules file created.', 'success');
      queryClient.invalidateQueries({ queryKey: queryKeys.audit.rules });
      queryClient.invalidateQueries({ queryKey: queryKeys.audit.compiled });
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setCreating(false);
    }
  };

  const isPending = viewMode === 'structured' ? compiled.isPending : rawQuery.isPending;
  const error = viewMode === 'structured' ? compiled.error : rawQuery.error;

  if (isPending) return <PageSkeleton />;
  if (error) {
    return (
      <Card variant="accent" className="text-center py-8">
        <p className="text-danger text-lg" style={{ fontFamily: 'var(--font-heading)' }}>
          Failed to load audit rules
        </p>
        <p className="text-pencil-light text-sm mt-1">{error.message}</p>
      </Card>
    );
  }

  return (
    <div className="animate-sketch-in">
      {/* ─── Header ─── */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 mb-6">
        <div>
          <div className="flex items-center gap-2 mb-1">
            <Link
              to="/audit"
              className="text-pencil-light hover:text-pencil transition-colors"
              aria-label="Back to Security Audit"
            >
              <ArrowLeft size={20} strokeWidth={2.5} />
            </Link>
            <h2
              className="text-3xl font-bold text-pencil flex items-center gap-2"
              style={{ fontFamily: 'var(--font-heading)' }}
            >
              <ShieldCheck size={28} strokeWidth={2.5} />
              Audit Rules
            </h2>
          </div>
          <p className="text-pencil-light" style={{ fontFamily: 'var(--font-hand)' }}>
            {isProjectMode
              ? 'Browse and manage project-level audit rules'
              : 'Browse and manage global audit rules'}
          </p>
        </div>
        <div className="flex items-center gap-3">
          <HandButton
            onClick={() => setViewMode(viewMode === 'structured' ? 'yaml' : 'structured')}
            variant="secondary"
            size="md"
          >
            {viewMode === 'structured' ? (
              <><FileEdit size={16} strokeWidth={2.5} /> Edit YAML</>
            ) : (
              <><List size={16} strokeWidth={2.5} /> Rule Browser</>
            )}
          </HandButton>
          {viewMode === 'structured' && stats.custom > 0 && (
            <HandButton
              onClick={() => setShowResetConfirm(true)}
              disabled={resetMutation.isPending}
              variant="danger"
              size="md"
            >
              <RotateCcw size={16} strokeWidth={2.5} />
              {resetMutation.isPending ? 'Resetting...' : 'Reset All'}
            </HandButton>
          )}
          {viewMode === 'yaml' && rawQuery.data?.exists && (
            <>
              {dirty && (
                <span
                  className="text-sm text-warning px-2 py-1 bg-warning-light border border-warning"
                  style={{ borderRadius: wobbly.sm, fontFamily: 'var(--font-hand)' }}
                >
                  unsaved changes
                </span>
              )}
              <HandButton onClick={handleSave} disabled={saving || !dirty} variant="primary" size="md">
                <Save size={16} strokeWidth={2.5} />
                {saving ? 'Saving...' : 'Save'}
              </HandButton>
            </>
          )}
        </div>
      </div>

      {/* ─── Structured View ─── */}
      {viewMode === 'structured' && compiled.data && (
        <>
          {/* Summary stat cards */}
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-5">
            <MiniStat
              label="Total Rules"
              value={stats.total}
              icon={ShieldCheck}
              color="var(--color-pencil)"
              bg="transparent"
            />
            <MiniStat
              label="Enabled"
              value={stats.enabled}
              icon={Eye}
              color="var(--color-success)"
              bg="var(--color-success-light)"
            />
            <MiniStat
              label="Disabled"
              value={stats.disabled}
              icon={EyeOff}
              color={stats.disabled > 0 ? 'var(--color-warning)' : 'var(--color-pencil-light)'}
              bg={stats.disabled > 0 ? 'var(--color-warning-light)' : 'transparent'}
            />
            <MiniStat
              label="Custom"
              value={stats.custom}
              icon={Settings2}
              color={stats.custom > 0 ? 'var(--color-blue)' : 'var(--color-pencil-light)'}
              bg={stats.custom > 0 ? 'var(--color-info-light)' : 'transparent'}
            />
          </div>

          {/* Severity tabs */}
          <div className="flex flex-wrap gap-2 mb-4">
            {SEVERITY_TABS.map((tab) => {
              const count = tabCounts[tab.value] ?? 0;
              const isActive = activeTab === tab.value;
              const tabColor = tab.value === 'ALL'
                ? 'var(--color-pencil)'
                : tab.value === 'DISABLED'
                  ? 'var(--color-warning)'
                  : severityColor(tab.value);
              return (
                <button
                  key={tab.value}
                  onClick={() => setActiveTab(tab.value)}
                  className={`
                    px-3 py-1.5 border-2 text-sm font-medium transition-all duration-150 cursor-pointer
                    ${isActive
                      ? ''
                      : 'bg-transparent text-pencil border-muted-dark hover:border-pencil'
                    }
                  `}
                  style={{
                    borderRadius: wobbly.sm,
                    fontFamily: 'var(--font-hand)',
                    ...(isActive
                      ? { backgroundColor: tabColor, borderColor: tabColor, color: 'var(--color-paper)', boxShadow: shadows.sm }
                      : {}),
                  }}
                >
                  {tab.label}
                  {count > 0 && (
                    <span className={`ml-1.5 ${isActive ? 'opacity-80' : 'opacity-50'}`}>
                      {count}
                    </span>
                  )}
                </button>
              );
            })}
          </div>

          {/* Toolbar: search + expand/collapse */}
          <div className="flex items-center gap-3 mb-5">
            <div className="relative flex-1">
              <Search
                size={16}
                strokeWidth={2.5}
                className="absolute left-3 top-1/2 -translate-y-1/2 text-pencil-light pointer-events-none"
              />
              <HandInput
                type="text"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Search by ID, message, regex, or pattern..."
                className="!pl-9"
              />
            </div>
            {groupedRules.length > 1 && (
              <button
                onClick={allExpanded ? collapseAll : expandAll}
                className="shrink-0 flex items-center gap-1.5 px-3 py-2 text-sm text-pencil-light hover:text-pencil border-2 border-pencil-light/30 hover:border-pencil transition-all duration-150 cursor-pointer"
                style={{ borderRadius: wobbly.sm, fontFamily: 'var(--font-hand)' }}
                title={allExpanded ? 'Collapse all groups' : 'Expand all groups'}
              >
                {allExpanded ? (
                  <><ChevronsDownUp size={16} strokeWidth={2.5} /> Collapse</>
                ) : (
                  <><ChevronsUpDown size={16} strokeWidth={2.5} /> Expand</>
                )}
              </button>
            )}
          </div>

          {/* Pattern accordion list */}
          {groupedRules.length === 0 ? (
            <EmptyState
              icon={ShieldCheck}
              title="No rules match"
              description="Try adjusting your filter or search terms"
            />
          ) : (
            <div className="space-y-3">
              {groupedRules.map(([pattern, rules]) => (
                <PatternAccordion
                  key={pattern}
                  pattern={pattern}
                  rules={rules}
                  allPatterns={compiled.data!.patterns}
                  isExpanded={expandedPatterns.has(pattern)}
                  expandedRules={expandedRules}
                  onToggleExpand={() => togglePatternExpanded(pattern)}
                  onToggleRuleExpand={toggleRuleExpanded}
                  onToggleRule={handleToggleRule}
                  onTogglePattern={handleTogglePattern}
                  onSetSeverity={handleSetSeverity}
                  onSetPatternSeverity={handleSetPatternSeverity}
                  isToggling={toggleMutation.isPending}
                />
              ))}
            </div>
          )}
        </>
      )}

      {/* ─── YAML Editor View ─── */}
      {viewMode === 'yaml' && (
        <>
          {rawQuery.data && !rawQuery.data.exists && (
            <EmptyState
              icon={FilePlus}
              title="No custom rules file"
              description={`Create ${isProjectMode ? 'a project-level' : 'a global'} audit-rules.yaml to add or override security rules`}
              action={
                <HandButton variant="primary" onClick={handleCreate} disabled={creating}>
                  <FilePlus size={16} strokeWidth={2.5} />
                  {creating ? 'Creating...' : 'Create Rules File'}
                </HandButton>
              }
            />
          )}

          {rawQuery.data?.exists && (
            <Card decoration="tape">
              <div className="flex items-center gap-2 mb-3">
                <FileCode size={16} strokeWidth={2.5} className="text-blue" />
                <span className="text-base text-pencil-light" style={{ fontFamily: 'var(--font-hand)' }}>
                  {rawQuery.data.path}
                </span>
              </div>
              <div className="min-w-0 -mx-4 -mb-4">
                <CodeMirror
                  value={raw}
                  onChange={handleChange}
                  extensions={extensions}
                  theme="none"
                  height="500px"
                  basicSetup={{
                    lineNumbers: true,
                    foldGutter: true,
                    highlightActiveLine: true,
                    highlightSelectionMatches: true,
                    bracketMatching: true,
                    indentOnInput: true,
                    autocompletion: false,
                  }}
                />
              </div>
            </Card>
          )}
        </>
      )}

      {/* Reset confirmation dialog */}
      <ConfirmDialog
        open={showResetConfirm}
        onConfirm={() => {
          setShowResetConfirm(false);
          resetMutation.mutate();
        }}
        onCancel={() => setShowResetConfirm(false)}
        title="Reset All Custom Rules"
        message="This will delete your audit-rules.yaml and restore all rules to their built-in defaults. This action cannot be undone."
        confirmText="Reset All"
        cancelText="Cancel"
        variant="danger"
        loading={resetMutation.isPending}
      />
    </div>
  );
}

/* ──────────────────────────────────────────────────────────────────────
 * MiniStat — compact stat card for summary row
 * ────────────────────────────────────────────────────────────────────── */

function MiniStat({
  label,
  value,
  icon: Icon,
  color,
  bg,
}: {
  label: string;
  value: number;
  icon: typeof ShieldCheck;
  color: string;
  bg: string;
}) {
  const isZero = value === 0;
  return (
    <div
      className={`flex items-center gap-3 p-3 border-2 border-pencil-light/30 transition-all duration-150 ${isZero ? 'opacity-50' : ''}`}
      style={{
        borderRadius: wobbly.md,
        backgroundColor: bg,
        boxShadow: isZero ? 'none' : shadows.sm,
      }}
    >
      <div
        className="w-9 h-9 flex items-center justify-center border-2 shrink-0"
        style={{
          borderRadius: wobbly.sm,
          borderColor: color,
          color,
          backgroundColor: isZero ? 'transparent' : `${color}10`,
        }}
      >
        <Icon size={18} strokeWidth={2.5} />
      </div>
      <div className="min-w-0">
        <p
          className="text-xl font-bold leading-tight"
          style={{ fontFamily: 'var(--font-heading)', color }}
        >
          {value}
        </p>
        <p className="text-xs text-pencil-light leading-tight truncate" style={{ fontFamily: 'var(--font-hand)' }}>
          {label}
        </p>
      </div>
    </div>
  );
}

/* ──────────────────────────────────────────────────────────────────────
 * PatternAccordion — collapsible group with severity stripe
 * ────────────────────────────────────────────────────────────────────── */

function PatternAccordion({
  pattern,
  rules,
  allPatterns,
  isExpanded,
  expandedRules,
  onToggleExpand,
  onToggleRuleExpand,
  onToggleRule,
  onTogglePattern,
  onSetSeverity,
  onSetPatternSeverity,
  isToggling,
}: {
  pattern: string;
  rules: CompiledRule[];
  allPatterns: PatternGroup[];
  isExpanded: boolean;
  expandedRules: Set<string>;
  onToggleExpand: () => void;
  onToggleRuleExpand: (id: string) => void;
  onToggleRule: (id: string, enabled: boolean) => void;
  onTogglePattern: (pattern: string, enabled: boolean) => void;
  onSetSeverity: (id: string, severity: string) => void;
  onSetPatternSeverity: (pattern: string, severity: string) => void;
  isToggling: boolean;
}) {
  const group = allPatterns.find((p) => p.pattern === pattern);
  const maxSev = group?.maxSeverity ?? 'MEDIUM';
  const stripeColor = severityColor(maxSev);
  const enabledCount = rules.filter((r) => r.enabled).length;
  const disabledCount = rules.length - enabledCount;
  const allEnabled = disabledCount === 0;
  const enabledRatio = rules.length > 0 ? (enabledCount / rules.length) * 100 : 100;

  return (
    <div
      className="border-2 border-pencil-light/30 border-l-[4px] transition-all duration-150"
      style={{
        borderRadius: wobbly.md,
        borderLeftColor: stripeColor,
        boxShadow: isExpanded ? shadows.sm : 'none',
        backgroundColor: isExpanded ? 'var(--color-paper-warm)' : 'transparent',
      }}
    >
      {/* Header */}
      <button
        onClick={onToggleExpand}
        className="w-full flex items-center gap-3 px-4 py-3 text-left hover:bg-paper-warm/40 transition-colors cursor-pointer"
        style={{ borderRadius: `0 ${wobbly.md.split(' ')[1]} ${wobbly.md.split(' ')[2]} 0` }}
      >
        <ChevronRight
          size={16}
          strokeWidth={2.5}
          className={`text-pencil-light shrink-0 transition-transform duration-200 ${isExpanded ? 'rotate-90' : ''}`}
        />
        <span className="font-bold text-pencil text-base flex-1" style={{ fontFamily: 'var(--font-heading)' }}>
          {pattern}
        </span>

        {/* Mini enabled/disabled bar */}
        <div className="hidden sm:flex items-center gap-2">
          <div
            className="w-16 h-1.5 bg-muted/50 overflow-hidden"
            style={{ borderRadius: '999px' }}
            title={`${enabledCount} enabled / ${disabledCount} disabled`}
          >
            <div
              className="h-full transition-all duration-300"
              style={{
                width: `${enabledRatio}%`,
                backgroundColor: enabledRatio === 100 ? 'var(--color-success)' : 'var(--color-warning)',
                borderRadius: '999px',
              }}
            />
          </div>
        </div>

        <span className="text-sm text-pencil-light shrink-0" style={{ fontFamily: 'var(--font-hand)' }}>
          {rules.length} rule{rules.length !== 1 ? 's' : ''}
        </span>
        {disabledCount > 0 && <Badge variant="warning">{disabledCount} off</Badge>}
        <Badge variant={severityBadgeVariant(maxSev)}>{maxSev}</Badge>
      </button>

      {/* Expanded content */}
      {isExpanded && (
        <div className="border-t-2 border-dashed border-pencil-light/20">
          {/* Group controls bar */}
          <div className="flex flex-wrap items-center gap-3 px-4 py-2.5 bg-paper-warm/30">
            <span className="text-sm text-pencil-light" style={{ fontFamily: 'var(--font-hand)' }}>
              {enabledCount}/{rules.length} enabled
            </span>
            <div className="flex-1" />
            <SeverityPicker
              current={maxSev}
              onSelect={(sev) => onSetPatternSeverity(pattern, sev)}
              disabled={isToggling}
              label="Group severity"
            />
            <button
              onClick={(e) => {
                e.stopPropagation();
                onTogglePattern(pattern, !allEnabled);
              }}
              disabled={isToggling}
              className="flex items-center gap-1.5 px-3 py-1.5 text-sm border-2 transition-all duration-150 hover:bg-surface disabled:opacity-50 cursor-pointer"
              style={{
                borderRadius: wobbly.sm,
                fontFamily: 'var(--font-hand)',
                borderColor: allEnabled ? 'var(--color-warning)' : 'var(--color-success)',
                color: allEnabled ? 'var(--color-warning)' : 'var(--color-success)',
              }}
            >
              {allEnabled ? (
                <><EyeOff size={14} strokeWidth={2.5} /> Disable All</>
              ) : (
                <><Eye size={14} strokeWidth={2.5} /> Enable All</>
              )}
            </button>
          </div>

          {/* Rule rows */}
          <div className="divide-y divide-dashed divide-pencil-light/15">
            {rules.map((rule) => (
              <RuleRow
                key={rule.id}
                rule={rule}
                pattern={pattern}
                isExpanded={expandedRules.has(rule.id)}
                onToggleExpand={() => onToggleRuleExpand(rule.id)}
                onToggle={(enabled) => onToggleRule(rule.id, enabled)}
                onSetSeverity={(sev) => onSetSeverity(rule.id, sev)}
                isToggling={isToggling}
              />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

/* ──────────────────────────────────────────────────────────────────────
 * RuleRow — single rule with severity stripe, toggle switch, detail
 * ────────────────────────────────────────────────────────────────────── */

function RuleRow({
  rule,
  pattern,
  isExpanded,
  onToggleExpand,
  onToggle,
  onSetSeverity,
  isToggling,
}: {
  rule: CompiledRule;
  pattern: string;
  isExpanded: boolean;
  onToggleExpand: () => void;
  onToggle: (enabled: boolean) => void;
  onSetSeverity: (severity: string) => void;
  isToggling: boolean;
}) {
  const shortId = rule.id.startsWith(pattern + '-')
    ? rule.id.slice(pattern.length + 1)
    : rule.id;
  const stripeColor = severityColor(rule.severity);

  return (
    <div
      className={`border-l-[3px] transition-all duration-150 ${!rule.enabled ? 'opacity-50' : ''}`}
      style={{ borderLeftColor: stripeColor }}
    >
      {/* Main row */}
      <div
        className="flex items-center gap-3 px-4 py-2.5 hover:bg-paper-warm/30 transition-colors cursor-pointer"
        onClick={onToggleExpand}
      >
        <ChevronRight
          size={14}
          strokeWidth={2.5}
          className={`text-pencil-light/50 shrink-0 transition-transform duration-150 ${isExpanded ? 'rotate-90' : ''}`}
        />
        <Badge variant={severityBadgeVariant(rule.severity)}>{rule.severity}</Badge>
        <div className="flex-1 min-w-0">
          <span className="text-sm text-pencil truncate block" style={{ fontFamily: 'var(--font-hand)' }}>
            {shortId}
          </span>
          <span className="text-xs text-pencil-light truncate block" style={{ fontFamily: 'var(--font-hand)' }}>
            {rule.message}
          </span>
        </div>
        {rule.source !== 'builtin' && (
          <span
            className="text-xs text-pencil-light px-1.5 py-0.5 border border-pencil-light/30 shrink-0"
            style={{ borderRadius: wobbly.sm, fontFamily: 'var(--font-hand)' }}
          >
            {rule.source}
          </span>
        )}
        {/* Toggle switch with track */}
        <ToggleSwitch
          enabled={rule.enabled}
          onToggle={() => onToggle(!rule.enabled)}
          disabled={isToggling}
        />
      </div>

      {/* Expanded detail */}
      {isExpanded && (
        <div
          className="mx-4 mb-3 ml-9 p-3 border-2 border-dashed border-pencil-light/20 space-y-2"
          style={{
            borderRadius: wobbly.sm,
            backgroundColor: severityBgColor(rule.severity),
          }}
        >
          <DetailRow label="Full ID" value={rule.id} mono />
          <div className="flex items-center gap-2 text-sm">
            <span className="text-pencil-light shrink-0 w-20" style={{ fontFamily: 'var(--font-hand)' }}>
              Severity
            </span>
            <SeverityPicker current={rule.severity} onSelect={onSetSeverity} disabled={isToggling} />
          </div>
          <DetailRow label="Message" value={rule.message} />
          {rule.regex && (
            <div className="flex items-start gap-2 text-sm">
              <span className="text-pencil-light shrink-0 w-20" style={{ fontFamily: 'var(--font-hand)' }}>
                Regex
              </span>
              <code
                className="text-xs text-pencil px-2 py-1 border border-pencil-light/20 bg-paper-warm break-all"
                style={{ borderRadius: wobbly.sm, fontFamily: "'Courier New', monospace" }}
              >
                {rule.regex}
              </code>
            </div>
          )}
          {rule.exclude && (
            <div className="flex items-start gap-2 text-sm">
              <span className="text-pencil-light shrink-0 w-20" style={{ fontFamily: 'var(--font-hand)' }}>
                Exclude
              </span>
              <code
                className="text-xs text-pencil px-2 py-1 border border-pencil-light/20 bg-paper-warm break-all"
                style={{ borderRadius: wobbly.sm, fontFamily: "'Courier New', monospace" }}
              >
                {rule.exclude}
              </code>
            </div>
          )}
          <DetailRow label="Source" value={rule.source} />
        </div>
      )}
    </div>
  );
}

/* ──────────────────────────────────────────────────────────────────────
 * ToggleSwitch — proper toggle with track background
 * ────────────────────────────────────────────────────────────────────── */

function ToggleSwitch({
  enabled,
  onToggle,
  disabled,
}: {
  enabled: boolean;
  onToggle: () => void;
  disabled: boolean;
}) {
  return (
    <button
      role="switch"
      aria-checked={enabled}
      onClick={(e) => {
        e.stopPropagation();
        onToggle();
      }}
      disabled={disabled}
      className={`
        relative shrink-0 w-10 h-6 border-2 transition-all duration-200 cursor-pointer
        disabled:opacity-50 disabled:cursor-not-allowed
      `}
      style={{
        borderRadius: wobbly.full,
        backgroundColor: enabled ? 'var(--color-success)' : 'var(--color-muted)',
        borderColor: enabled ? 'var(--color-success)' : 'var(--color-muted-dark)',
      }}
      title={enabled ? 'Disable rule' : 'Enable rule'}
    >
      <span
        className="absolute top-0.5 w-4 h-4 bg-white border border-pencil-light/30 transition-all duration-200"
        style={{
          borderRadius: wobbly.full,
          left: enabled ? '18px' : '2px',
          boxShadow: '1px 1px 0 rgba(0,0,0,0.1)',
        }}
      />
    </button>
  );
}

/* ──────────────────────────────────────────────────────────────────────
 * DetailRow — key-value display in rule detail panel
 * ────────────────────────────────────────────────────────────────────── */

function DetailRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-start gap-2 text-sm">
      <span className="text-pencil-light shrink-0 w-20" style={{ fontFamily: 'var(--font-hand)' }}>
        {label}
      </span>
      <span
        className="text-pencil break-all"
        style={{ fontFamily: mono ? "'Courier New', monospace" : 'var(--font-hand)' }}
      >
        {value}
      </span>
    </div>
  );
}

/* ──────────────────────────────────────────────────────────────────────
 * SeverityPicker — inline severity selector with proper touch targets
 * ────────────────────────────────────────────────────────────────────── */

const SEVERITY_LEVELS = ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW', 'INFO'] as const;

const SEV_SHORT: Record<string, string> = {
  CRITICAL: 'CRIT',
  HIGH: 'HIGH',
  MEDIUM: 'MED',
  LOW: 'LOW',
  INFO: 'INFO',
};

function SeverityPicker({
  current,
  onSelect,
  disabled,
  label,
}: {
  current: string;
  onSelect: (severity: string) => void;
  disabled: boolean;
  label?: string;
}) {
  return (
    <div className="flex items-center gap-1" role="radiogroup" aria-label={label ?? 'Set severity'}>
      {label && (
        <span className="text-xs text-pencil-light mr-1" style={{ fontFamily: 'var(--font-hand)' }}>
          {label}
        </span>
      )}
      {SEVERITY_LEVELS.map((sev) => {
        const isActive = current === sev;
        const color = severityColor(sev);
        return (
          <button
            key={sev}
            role="radio"
            aria-checked={isActive}
            onClick={(e) => {
              e.stopPropagation();
              if (!isActive) onSelect(sev);
            }}
            disabled={disabled || isActive}
            className={`
              min-w-[40px] px-2 py-1 text-xs border-2 transition-all duration-150 cursor-pointer
              ${isActive
                ? 'font-bold'
                : 'hover:opacity-100'
              }
              disabled:cursor-default
            `}
            style={{
              borderRadius: wobbly.sm,
              fontFamily: 'var(--font-hand)',
              color: isActive ? 'var(--color-paper)' : color,
              borderColor: isActive ? color : `color-mix(in srgb, ${color} 35%, transparent)`,
              backgroundColor: isActive ? color : 'transparent',
              boxShadow: isActive ? '1px 1px 0 rgba(0,0,0,0.15)' : 'none',
              opacity: isActive ? 1 : 0.75,
            }}
            title={`Set severity to ${sev}`}
          >
            {SEV_SHORT[sev]}
          </button>
        );
      })}
    </div>
  );
}
