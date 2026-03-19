import { useState, useEffect, useMemo, useCallback, useRef } from 'react';
import {
  ShieldCheck,
  Search,
  ChevronRight,
  List,
  FileEdit,
  RotateCcw,
  Eye,
  EyeOff,
  ChevronsDownUp,
  ChevronsUpDown,
} from 'lucide-react';
import { useQuery, useQueryClient, useMutation } from '@tanstack/react-query';
import Card from '../components/Card';
import Button from '../components/Button';
import PageHeader from '../components/PageHeader';
import Badge from '../components/Badge';
import ConfirmDialog from '../components/ConfirmDialog';
import { Input } from '../components/Input';
import SegmentedControl from '../components/SegmentedControl';
import EmptyState from '../components/EmptyState';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { api } from '../api/client';
import type { CompiledRule, PatternGroup } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { useAppContext } from '../context/AppContext';
import { radius, shadows } from '../design';
import { severityColor, severityBgColor, severityBadgeVariant } from '../lib/severity';
import AuditAssistantPanel from '../components/audit/AuditAssistantPanel';
import AuditRulesYaml from './AuditRulesYaml';

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

  // Selection + cross-view state
  const [selectedRuleId, setSelectedRuleId] = useState<string | null>(null);
  const [selectedPattern, setSelectedPattern] = useState<string | null>(null);
  const [pendingRegexTest, setPendingRegexTest] = useState<{ regex: string; exclude?: string } | null>(null);
  const [pendingJumpToId, setPendingJumpToId] = useState<string | null>(null);

  // Panel collapsed state (shared with AuditRulesYaml)
  const [panelCollapsed, setPanelCollapsed] = useState(() => {
    try { return localStorage.getItem('audit-panel-collapsed') === 'true'; }
    catch { return false; }
  });

  const togglePanel = useCallback(() => {
    setPanelCollapsed(prev => {
      const next = !prev;
      try { localStorage.setItem('audit-panel-collapsed', String(next)); }
      catch { /* ignore */ }
      return next;
    });
  }, []);

  // Cmd+B handler for panel toggle (both views)
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'b') {
        e.preventDefault();
        togglePanel();
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [togglePanel]);

  // Measure sticky toolbar height for nested sticky group headers
  const toolbarRef = useRef<HTMLDivElement>(null);
  const [toolbarHeight, setToolbarHeight] = useState(0);

  // Compiled rules query (structured view)
  const compiled = useQuery({
    queryKey: queryKeys.audit.compiled,
    queryFn: () => api.getCompiledRules(),
    staleTime: staleTimes.auditRules,
  });

  useEffect(() => {
    const el = toolbarRef.current;
    if (!el) return;
    const update = () => setToolbarHeight(el.offsetHeight);
    update();
    const ro = new ResizeObserver(update);
    ro.observe(el);
    return () => ro.disconnect();
  }, [viewMode, compiled.isPending]);

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
  // Single-pass tab counts + stats (replaces 9 separate .filter() calls)
  const { tabCounts, stats } = useMemo(() => {
    if (!compiled.data) {
      return {
        tabCounts: {} as Record<string, number>,
        stats: { total: 0, enabled: 0, disabled: 0, custom: 0, patterns: 0 },
      };
    }
    const rules = compiled.data.rules;
    const tc: Record<string, number> = { ALL: rules.length, CRITICAL: 0, HIGH: 0, MEDIUM: 0, LOW: 0, INFO: 0, DISABLED: 0 };
    let enabled = 0, disabled = 0, custom = 0;
    for (const r of rules) {
      if (!r.enabled) {
        disabled++;
        tc.DISABLED++;
      } else {
        enabled++;
        if (r.severity in tc) tc[r.severity]++;
      }
      if (r.source !== 'builtin') custom++;
    }
    return {
      tabCounts: tc,
      stats: { total: rules.length, enabled, disabled, custom, patterns: compiled.data.patterns.length },
    };
  }, [compiled.data]);

  // Derived: selected rule / pattern rules for assistant panel
  const selectedRule = useMemo(() => {
    if (!selectedRuleId || !compiled.data) return null;
    return compiled.data.rules.find(r => r.id === selectedRuleId) ?? null;
  }, [selectedRuleId, compiled.data]);

  const patternRules = useMemo(() => {
    if (!selectedPattern || !compiled.data) return [];
    return compiled.data.rules.filter(r => r.pattern === selectedPattern);
  }, [selectedPattern, compiled.data]);

  // Selection callbacks
  const handleRuleSelect = useCallback((ruleId: string) => {
    setSelectedRuleId(ruleId);
    setSelectedPattern(null);
  }, []);

  const handlePatternSelect = useCallback((pattern: string) => {
    setSelectedPattern(pattern);
    setSelectedRuleId(null);
  }, []);

  // Cross-view jump handlers
  const handleTestRegex = useCallback(() => {
    if (!selectedRule) return;
    setPendingRegexTest({ regex: selectedRule.regex, exclude: selectedRule.exclude });
    setViewMode('yaml');
  }, [selectedRule]);

  const handleEditInYaml = useCallback(() => {
    if (!selectedRule) return;
    setPendingJumpToId(selectedRule.id);
    setViewMode('yaml');
  }, [selectedRule]);

  const togglePatternExpanded = useCallback((pattern: string) => {
    setExpandedPatterns((prev) => {
      const next = new Set(prev);
      if (next.has(pattern)) next.delete(pattern);
      else next.add(pattern);
      return next;
    });
    handlePatternSelect(pattern);
  }, [handlePatternSelect]);

  const toggleRuleExpanded = useCallback((id: string) => {
    setExpandedRules((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
        handleRuleSelect(id);
      }
      return next;
    });
  }, [handleRuleSelect]);

  const expandAll = useCallback(() => {
    setExpandedPatterns(new Set(groupedRules.map(([p]) => p)));
  }, [groupedRules]);

  const collapseAll = useCallback(() => {
    setExpandedPatterns(new Set());
    setExpandedRules(new Set());
  }, []);

  const allExpanded = groupedRules.length > 0 && expandedPatterns.size === groupedRules.length;

  // Loading/error only for structured view (YAML view handles its own)
  if (viewMode === 'structured' && compiled.isPending) return <PageSkeleton />;
  if (viewMode === 'structured' && compiled.error) {
    return (
      <Card variant="accent" className="text-center py-8">
        <p className="text-danger text-lg">
          Failed to load audit rules
        </p>
        <p className="text-pencil-light text-sm mt-1">{compiled.error.message}</p>
      </Card>
    );
  }

  return (
    <div className="animate-fade-in">
      {/* ─── Header ─── */}
      <PageHeader
        icon={<ShieldCheck size={24} strokeWidth={2.5} />}
        title="Audit Rules"
        subtitle={
          isProjectMode
            ? 'Browse and manage project-level audit rules'
            : 'Browse and manage global audit rules'
        }
        className="mb-1!"
        backTo="/audit"
        actions={
          <>
            <Button
              onClick={() => setViewMode(viewMode === 'structured' ? 'yaml' : 'structured')}
              variant="secondary"
              size="sm"
            >
              {viewMode === 'structured' ? (
                <><FileEdit size={16} strokeWidth={2.5} /> Edit YAML</>
              ) : (
                <><List size={16} strokeWidth={2.5} /> Rule Browser</>
              )}
            </Button>
            {viewMode === 'structured' && stats.custom > 0 && (
              <Button
                onClick={() => setShowResetConfirm(true)}
                disabled={resetMutation.isPending}
                variant="danger"
                size="sm"
              >
                <RotateCcw size={16} strokeWidth={2.5} />
                {resetMutation.isPending ? 'Resetting...' : 'Reset All'}
              </Button>
            )}
          </>
        }
      />

      {/* ─── Structured View ─── */}
      {viewMode === 'structured' && compiled.data && (
        <div className="flex gap-4">
          <div className="flex-[3] min-w-0">
            {/* Sticky toolbar: summary + tabs + search */}
            <div ref={toolbarRef} className="sticky top-0 z-20 bg-paper pt-4 pb-4 -mx-1 px-1 space-y-3" style={{ boxShadow: '0 12px 0 0 var(--color-paper)' }}>
              {/* Inline summary */}
              <p className="text-sm text-pencil-light">
                <span className="font-medium text-pencil">{stats.total}</span> rules
                {' '}&middot;{' '}
                <span className="text-success">{stats.enabled} enabled</span>
                {stats.disabled > 0 && (
                  <>{' '}&middot;{' '}<span className="text-warning">{stats.disabled} disabled</span></>
                )}
                {stats.custom > 0 && (
                  <>{' '}&middot;{' '}<span className="text-blue">{stats.custom} custom</span></>
                )}
                {' '}&middot;{' '}
                {stats.patterns} patterns
              </p>

              {/* Severity tabs */}
              <SegmentedControl
                value={activeTab}
                onChange={setActiveTab}
                options={SEVERITY_TABS.map((tab) => ({
                  value: tab.value,
                  label: tab.label,
                  count: tabCounts[tab.value] ?? 0,
                }))}
                colorFn={(v) =>
                  v === 'ALL'
                    ? 'var(--color-pencil)'
                    : v === 'DISABLED'
                      ? 'var(--color-warning)'
                      : severityColor(v)
                }
              />

              {/* Search + expand/collapse */}
              <div className="flex items-center gap-3">
                <div className="relative flex-1">
                  <Search
                    size={16}
                    strokeWidth={2.5}
                    className="absolute left-3 top-1/2 -translate-y-1/2 text-pencil-light pointer-events-none"
                  />
                  <Input
                    type="text"
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    placeholder="Search by ID, message, regex, or pattern..."
                    className="!pl-9"
                  />
                </div>
                {groupedRules.length > 1 && (
                  <Button
                    onClick={allExpanded ? collapseAll : expandAll}
                    variant="secondary"
                    size="sm"
                  >
                    {allExpanded ? (
                      <><ChevronsDownUp size={16} strokeWidth={2.5} /> Collapse</>
                    ) : (
                      <><ChevronsUpDown size={16} strokeWidth={2.5} /> Expand</>
                    )}
                  </Button>
                )}
              </div>
            </div>

            {/* Pattern accordion list */}
            {groupedRules.length === 0 ? (
              <EmptyState
                icon={ShieldCheck}
                title="No rules match"
                description="Try adjusting your filter or search terms"
              />
            ) : (
              <div className="space-y-4 pt-3">
                {groupedRules.map(([pattern, rules]) => (
                  <PatternAccordion
                    key={pattern}
                    pattern={pattern}
                    rules={rules}
                    allPatterns={compiled.data!.patterns}
                    stickyTop={toolbarHeight}
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
          </div>

          <div className={`hidden lg:block transition-all duration-300 ease-in-out overflow-hidden ${
            panelCollapsed ? 'flex-[0] w-0 opacity-0 pointer-events-none' : 'flex-[2] opacity-100'
          }`}>
            <div className="sticky top-0" style={{ maxHeight: 'calc(100vh - 100px)' }}>
              <Card className="h-full !p-0 !overflow-visible min-w-[280px]">
                <AuditAssistantPanel
                  mode="structured"
                  selectedRule={selectedRule}
                  selectedPattern={selectedPattern}
                  patternRules={patternRules}
                  stats={stats}
                  compiledRules={compiled.data.rules}
                  onTestRegex={handleTestRegex}
                  onEditInYaml={handleEditInYaml}
                  onTogglePattern={handleTogglePattern}
                  isToggling={toggleMutation.isPending}
                  collapsed={panelCollapsed}
                  onToggleCollapse={togglePanel}
                  errors={[]}
                  changeCount={0}
                  fieldPath={null}
                  cursorLine={1}
                  source=""
                  diff={{ lines: [], changeCount: 0 }}
                  editorRef={{ current: null }}
                  onRevert={() => {}}
                />
              </Card>
            </div>
          </div>
        </div>
      )}

      {/* ─── YAML Editor View ─── */}
      {viewMode === 'yaml' && (
        <AuditRulesYaml
          panelCollapsed={panelCollapsed}
          onTogglePanel={togglePanel}
          isProjectMode={isProjectMode}
          pendingRegexTest={pendingRegexTest}
          onPendingRegexTestConsumed={() => setPendingRegexTest(null)}
          pendingJumpToId={pendingJumpToId}
          onPendingJumpToIdConsumed={() => setPendingJumpToId(null)}
        />
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
  stickyTop,
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
  stickyTop: number;
  isToggling: boolean;
}) {
  const group = allPatterns.find((p) => p.pattern === pattern);
  const maxSev = group?.maxSeverity ?? 'MEDIUM';
  const stripeColor = severityColor(maxSev);
  const enabledCount = rules.filter((r) => r.enabled).length;
  const disabledCount = rules.length - enabledCount;
  const allEnabled = disabledCount === 0;
  const enabledRatio = rules.length > 0 ? (enabledCount / rules.length) * 100 : 100;

  // Scroll accordion into view on expand/collapse transitions
  const accordionRef = useRef<HTMLDivElement>(null);
  const wasExpandedRef = useRef(isExpanded);

  useEffect(() => {
    if (!accordionRef.current) return;

    if (!wasExpandedRef.current && isExpanded) {
      // Expanding: scroll so the header + first items are visible
      // Use requestAnimationFrame to wait for DOM to update with expanded content
      requestAnimationFrame(() => {
        accordionRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' });
      });
    } else if (wasExpandedRef.current && !isExpanded) {
      // Collapsing: scroll back if accordion ended up off-screen
      const rect = accordionRef.current.getBoundingClientRect();
      if (rect.top < 0 || rect.top > window.innerHeight * 0.5) {
        accordionRef.current.scrollIntoView({ behavior: 'smooth', block: 'start' });
      }
    }
    wasExpandedRef.current = isExpanded;
  }, [isExpanded]);

  return (
    <div
      ref={accordionRef}
      className="border border-pencil-light/30 transition-all duration-150"
      style={{
        borderRadius: radius.md,
        scrollMarginTop: `${stickyTop + 28}px`,
        boxShadow: isExpanded ? shadows.sm : 'none',
        backgroundColor: isExpanded ? 'var(--color-paper-warm)' : 'transparent',
      }}
    >
      {/* Sticky group: header + controls bar stick together when expanded */}
      <div
        className={isExpanded ? 'sticky z-10' : ''}
        style={{
          borderRadius: isExpanded ? `${radius.md} ${radius.md} 0 0` : radius.md,
          ...(isExpanded ? {
            top: `${stickyTop + 12}px`,
            backgroundColor: 'var(--color-paper-warm)',
            boxShadow: [
              'inset 0 1px 0 0 color-mix(in srgb, var(--color-pencil-light) 30%, transparent)',
              'inset 1px 0 0 0 color-mix(in srgb, var(--color-pencil-light) 30%, transparent)',
              'inset -1px 0 0 0 color-mix(in srgb, var(--color-pencil-light) 30%, transparent)',
              '0 2px 8px rgba(0,0,0,0.12)',
            ].join(', '),
            borderBottom: '1px dashed color-mix(in srgb, var(--color-pencil-light) 30%, transparent)',
          } : {}),
        }}
      >
        <button
          onClick={onToggleExpand}
          className={`w-full flex items-center gap-3 px-4 py-3 text-left transition-colors cursor-pointer${!isExpanded ? ' hover:bg-paper-warm/40' : ''}`}
          style={{
            borderRadius: isExpanded ? `${radius.md} ${radius.md} 0 0` : radius.md,
          }}
        >
          <ChevronRight
            size={16}
            strokeWidth={2.5}
            className={`text-pencil-light shrink-0 transition-transform duration-200 ${isExpanded ? 'rotate-90' : ''}`}
          />
          <span
            className="w-2.5 h-2.5 rounded-full shrink-0"
            style={{ backgroundColor: stripeColor }}
          />
          <span className="font-bold text-pencil text-base flex-1">
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

          <span className="text-sm text-pencil-light shrink-0">
            {rules.length} rule{rules.length !== 1 ? 's' : ''}
          </span>
          {disabledCount > 0 && <Badge variant="warning">{disabledCount} off</Badge>}
          <Badge variant={severityBadgeVariant(maxSev)}>{maxSev}</Badge>
        </button>

        {/* Group controls bar — inside sticky group so it stays visible */}
        {isExpanded && (
          <div className="flex flex-wrap items-center gap-3 px-4 py-2.5 bg-paper-warm/30 border-t-2 border-dashed border-pencil-light/30">
            <span className="text-sm text-pencil-light">
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
                borderRadius: radius.sm,
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
        )}
      </div>

      {/* Rule rows — scroll below the sticky header+controls group */}
      {isExpanded && (
        <div className="divide-y divide-dashed divide-pencil-light/30">
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
      className={`transition-all duration-150 ${!rule.enabled ? 'opacity-50' : ''}`}
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
        <span
          className="w-2 h-2 rounded-full shrink-0"
          style={{ backgroundColor: stripeColor }}
        />
        <Badge variant={severityBadgeVariant(rule.severity)}>{rule.severity}</Badge>
        <div className="flex-1 min-w-0">
          <span className="text-sm text-pencil truncate block">
            {shortId}
          </span>
          <span className="text-xs text-pencil-light truncate block">
            {rule.message}
          </span>
        </div>
        {rule.source !== 'builtin' && (
          <span
            className="text-xs text-pencil-light px-1.5 py-0.5 border border-pencil-light/30 shrink-0"
            style={{ borderRadius: radius.sm }}
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
          className="mx-4 mb-3 ml-9 p-3 border-2 border-dashed border-pencil-light/30 space-y-2"
          style={{
            borderRadius: radius.sm,
            backgroundColor: severityBgColor(rule.severity),
          }}
        >
          <DetailRow label="Full ID" value={rule.id} mono />
          <div className="flex items-center gap-2 text-sm">
            <span className="text-pencil-light shrink-0 w-20">
              Severity
            </span>
            <SeverityPicker current={rule.severity} onSelect={onSetSeverity} disabled={isToggling} />
          </div>
          <DetailRow label="Message" value={rule.message} />
          {rule.regex && (
            <div className="flex items-start gap-2 text-sm">
              <span className="text-pencil-light shrink-0 w-20">
                Regex
              </span>
              <code
                className="font-mono text-xs text-pencil px-2 py-1 border border-pencil-light/30 bg-paper-warm break-all"
                style={{ borderRadius: radius.sm }}
              >
                {rule.regex}
              </code>
            </div>
          )}
          {rule.exclude && (
            <div className="flex items-start gap-2 text-sm">
              <span className="text-pencil-light shrink-0 w-20">
                Exclude
              </span>
              <code
                className="font-mono text-xs text-pencil px-2 py-1 border border-pencil-light/30 bg-paper-warm break-all"
                style={{ borderRadius: radius.sm }}
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
        borderRadius: radius.full,
        backgroundColor: enabled ? 'var(--color-success)' : 'var(--color-muted)',
        borderColor: enabled ? 'var(--color-success)' : 'var(--color-muted-dark)',
      }}
      title={enabled ? 'Disable rule' : 'Enable rule'}
    >
      <span
        className="absolute top-0.5 w-4 h-4 bg-white border border-pencil-light/30 transition-all duration-200"
        style={{
          borderRadius: radius.full,
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
      <span className="text-pencil-light shrink-0 w-20">
        {label}
      </span>
      <span
        className={`text-pencil break-all${mono ? ' font-mono' : ''}`}
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
        <span className="text-xs text-pencil-light mr-1">
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
              borderRadius: radius.sm,
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
