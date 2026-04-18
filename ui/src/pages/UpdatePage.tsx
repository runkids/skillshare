import { useState, useEffect, useRef, useCallback, useMemo, useDeferredValue, forwardRef } from 'react';
import { useT } from '../i18n';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  ArrowUpCircle, RefreshCw, Search, Check, Zap, Trash2,
  Circle, CheckCircle, XCircle, MinusCircle, ShieldAlert, Loader2,
  LayoutGrid, Users, Globe, Puzzle, Bot,
} from 'lucide-react';
import { Virtuoso } from 'react-virtuoso';
import Card from '../components/Card';
import Button from '../components/Button';
import SplitButton from '../components/SplitButton';
import PageHeader from '../components/PageHeader';
import EmptyState from '../components/EmptyState';
import Badge from '../components/Badge';
import SegmentedControl from '../components/SegmentedControl';
import StreamProgressBar from '../components/StreamProgressBar';
import KindBadge from '../components/KindBadge';
import SourceBadge from '../components/SourceBadge';
import { PageSkeleton } from '../components/Skeleton';
import { Input } from '../components/Input';
import { Checkbox } from '../components/Checkbox';
import { useToast } from '../components/Toast';
import { api } from '../api/client';
import type { CheckResult } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { clearAuditCache } from '../lib/auditCache';
import { globToRegex } from '../lib/glob';
import { radius } from '../design';

/* ── Types ──────────────────────────────────────────── */

type UpdatePhase = 'selecting' | 'updating' | 'done';

type CheckStatus = 'unchecked' | 'checking' | 'behind' | 'up-to-date' | 'update-available' | 'error';

interface CheckItemStatus {
  status: CheckStatus;
  message?: string;
  behind?: number;
}

type TypeFilter = 'all' | 'tracked' | 'github';

type ResourceTab = 'skills' | 'agents';

interface UpdatableItem {
  name: string;
  flatName: string;
  kind: 'skill' | 'agent';
  isInRepo: boolean;
  source?: string;
  type?: string;
  relPath: string;
  installedAt?: string;
}

interface ItemUpdateStatus {
  name: string;
  kind?: 'skill' | 'agent';
  isRepo: boolean;
  status: 'pending' | 'in-progress' | 'success' | 'error' | 'blocked' | 'skipped';
  message?: string;
  auditRiskLabel?: string;
}

/* ── Component ──────────────────────────────────────── */

export default function UpdatePage() {
  const t = useT();
  const queryClient = useQueryClient();
  const { toast } = useToast();

  // Phase state machine
  const [phase, setPhase] = useState<UpdatePhase>('selecting');

  // Data: skills list
  const { data: skillsData, isPending } = useQuery({
    queryKey: queryKeys.skills.all,
    queryFn: () => api.listSkills(),
    staleTime: staleTimes.skills,
  });
  const allSkills = skillsData?.resources ?? [];

  // Tab state (skills vs agents)
  const [activeTab, setActiveTab] = useState<ResourceTab>('skills');

  // All updatable items (both skills and agents): tracked repos + GitHub-installed
  const allUpdatableItems: UpdatableItem[] = useMemo(
    () =>
      allSkills
        .filter((s) => s.isInRepo || s.source)
        .map((s) => ({
          name: s.name,
          flatName: s.flatName,
          kind: s.kind,
          isInRepo: s.isInRepo,
          source: s.source,
          type: s.type,
          relPath: s.relPath,
          installedAt: s.installedAt,
        })),
    [allSkills],
  );

  // Tab counts
  const skillTabCount = useMemo(
    () => allUpdatableItems.filter((i) => i.kind !== 'agent').length,
    [allUpdatableItems],
  );
  const agentTabCount = useMemo(
    () => allUpdatableItems.filter((i) => i.kind === 'agent').length,
    [allUpdatableItems],
  );

  // Tab-scoped items
  const updatableItems: UpdatableItem[] = useMemo(
    () =>
      activeTab === 'agents'
        ? allUpdatableItems.filter((i) => i.kind === 'agent')
        : allUpdatableItems.filter((i) => i.kind !== 'agent'),
    [allUpdatableItems, activeTab],
  );

  // Check state
  const [checkStatuses, setCheckStatuses] = useState<Map<string, CheckItemStatus>>(new Map());
  const [checking, setChecking] = useState(false);
  const [checkMode, setCheckMode] = useState<'all' | 'selected' | null>(null);
  const esRef = useRef<EventSource | null>(null);
  const startTimeRef = useRef<number>(0);

  // Selection state
  const [selected, setSelected] = useState<Set<string>>(new Set());

  // Filter state
  const [search, setSearch] = useState('');
  const [typeFilter, setTypeFilter] = useState<TypeFilter>('all');
  const deferredSearch = useDeferredValue(search);

  // Updating state
  const [itemStatuses, setItemStatuses] = useState<ItemUpdateStatus[]>([]);
  const itemRefs = useRef<Record<string, HTMLDivElement | null>>({});

  // Clean up EventSource on unmount
  useEffect(() => {
    return () => { esRef.current?.close(); };
  }, []);

  // Auto-scroll to the item currently being updated
  useEffect(() => {
    if (phase !== 'updating') return;
    const inProgressItem = itemStatuses.find((s) => s.status === 'in-progress');
    if (inProgressItem) {
      const el = itemRefs.current[inProgressItem.name];
      if (el) {
        el.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
    }
  }, [itemStatuses, phase]);

  /* ── Check logic ─────────────────────────────────── */

  const applyCheckResult = useCallback((result: CheckResult, filterNames?: Set<string>) => {
    setCheckStatuses((prev) => {
      const next = new Map(prev);

      // Tracked repos: propagate the repo's status to every item belonging to it.
      // The backend returns one entry per repo directory (e.g. `_awesome-claude-agents`)
      // but the UI uses per-item keys (`api-architect`, `backend-developer`, ...).
      for (const repo of result.tracked_repos) {
        const repoStatus: CheckItemStatus = {
          status: repo.status === 'behind' ? 'behind' : 'up-to-date',
          message: repo.message,
          behind: repo.behind,
        };
        for (const item of allUpdatableItems) {
          if (!item.isInRepo) continue;
          if (item.relPath.split('/')[0] !== repo.name) continue;
          if (filterNames && !filterNames.has(item.name)) continue;
          next.set(item.name, repoStatus);
        }
      }

      // Individual non-repo skills (GitHub-installed)
      for (const skill of result.skills) {
        const item = allUpdatableItems.find(
          (i) => !i.isInRepo && (i.name === skill.name || i.flatName === skill.name),
        );
        if (!item) continue;
        if (filterNames && !filterNames.has(item.name)) continue;
        next.set(item.name, {
          status: skill.status === 'update_available' ? 'update-available' : 'up-to-date',
        });
      }

      return next;
    });
  }, [allUpdatableItems]);

  const runCheck = useCallback((filterNames?: Set<string>) => {
    esRef.current?.close();
    setChecking(true);
    setCheckMode(filterNames ? 'selected' : 'all');
    startTimeRef.current = Date.now();

    // Mark items as checking. "Check All" marks every item (across both tabs)
    // because a full scan updates results for all of them.
    setCheckStatuses((prev) => {
      const next = new Map(prev);
      if (filterNames) {
        for (const name of filterNames) next.set(name, { status: 'checking' });
      } else {
        for (const item of allUpdatableItems) next.set(item.name, { status: 'checking' });
      }
      return next;
    });

    esRef.current = api.checkStream(
      () => {},
      () => {},
      () => {},
      (result) => {
        applyCheckResult(result, filterNames);
        setChecking(false);
        setCheckMode(null);
      },
      (err) => {
        toast(err.message, 'error');
        // Mark items as error
        setCheckStatuses((prev) => {
          const next = new Map(prev);
          if (filterNames) {
            for (const name of filterNames) next.set(name, { status: 'error' });
          } else {
            for (const item of allUpdatableItems) next.set(item.name, { status: 'error' });
          }
          return next;
        });
        setChecking(false);
        setCheckMode(null);
      },
    );
  }, [allUpdatableItems, applyCheckResult, toast]);

  const handleCheckAll = useCallback(() => runCheck(), [runCheck]);
  const handleCheckSelected = useCallback(() => {
    if (selected.size === 0) return;
    runCheck(new Set(selected));
  }, [runCheck, selected]);

  /* ── Tab switching ───────────────────────────────── */

  const changeTab = useCallback((tab: ResourceTab) => {
    setActiveTab(tab);
    setSelected(new Set());
    setSearch('');
    setTypeFilter('all');
  }, []);

  /* ── Filtering ───────────────────────────────────── */

  const filterCounts = useMemo(() => {
    const counts: Record<TypeFilter, number> = { all: updatableItems.length, tracked: 0, github: 0 };
    for (const item of updatableItems) {
      if (item.isInRepo) counts.tracked++;
      if ((item.type === 'github' || item.type === 'github-subdir') && !item.isInRepo) counts.github++;
    }
    return counts;
  }, [updatableItems]);

  const filtered = useMemo(() => {
    let list = updatableItems;
    if (deferredSearch.trim()) {
      const re = globToRegex(deferredSearch.trim());
      list = list.filter((s) => re.test(s.name) || re.test(s.relPath));
    }
    if (typeFilter === 'tracked') {
      list = list.filter((s) => s.isInRepo);
    } else if (typeFilter === 'github') {
      list = list.filter((s) => (s.type === 'github' || s.type === 'github-subdir') && !s.isInRepo);
    }
    // Sort by group (top-level directory of relPath) then by name
    return [...list].sort((a, b) => {
      const groupA = a.relPath.split('/')[0];
      const groupB = b.relPath.split('/')[0];
      if (groupA !== groupB) return groupA.localeCompare(groupB);
      return a.name.localeCompare(b.name);
    });
  }, [updatableItems, deferredSearch, typeFilter]);

  /* ── Selection ───────────────────────────────────── */

  const toggleSelect = useCallback((name: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      next.has(name) ? next.delete(name) : next.add(name);
      return next;
    });
  }, []);

  const allInViewSelected = filtered.length > 0 && filtered.every((s) => selected.has(s.name));

  const selectAll = useCallback(() => {
    setSelected((prev) => {
      const next = new Set(prev);
      filtered.forEach((s) => next.add(s.name));
      return next;
    });
  }, [filtered]);

  const deselectAll = useCallback(() => {
    setSelected((prev) => {
      const next = new Set(prev);
      filtered.forEach((s) => next.delete(s.name));
      return next;
    });
  }, [filtered]);

  /* ── Update logic ────────────────────────────────── */

  const invalidateSkillData = useCallback(() => {
    clearAuditCache(queryClient);
    queryClient.invalidateQueries({ queryKey: queryKeys.overview });
    queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
  }, [queryClient]);

  const handleUpdate = useCallback((opts?: { force?: boolean }) => {
    if (selected.size === 0) return;

    // Repo deduplication: multiple skills in same repo -> one update
    const seenRepos = new Set<string>();
    const names: string[] = [];
    const items: ItemUpdateStatus[] = [];

    for (const name of selected) {
      const item = updatableItems.find((i) => i.name === name);
      if (!item) continue;

      if (item.isInRepo) {
        const repoDir = item.relPath.split('/')[0];
        if (seenRepos.has(repoDir)) continue;
        seenRepos.add(repoDir);
        names.push(repoDir);
        items.push({ name: repoDir, kind: item.kind, isRepo: true, status: 'pending' });
      } else {
        names.push(item.flatName);
        items.push({ name: item.flatName, kind: item.kind, isRepo: false, status: 'pending' });
      }
    }

    setItemStatuses(items);
    startTimeRef.current = Date.now();
    setPhase('updating');

    let resultIndex = 0;

    esRef.current = api.updateAllStream(
      () => {
        setItemStatuses((prev) =>
          prev.map((s, idx) => (idx === 0 ? { ...s, status: 'in-progress' } : s)),
        );
      },
      (item) => {
        const i = resultIndex;
        resultIndex++;
        setItemStatuses((prev) =>
          prev.map((s, idx) => {
            if (idx === i) {
              return {
                ...s,
                kind: item.kind,
                status: actionToStatus(item.action),
                message: item.message,
                auditRiskLabel: item.auditRiskLabel,
              };
            }
            if (idx === i + 1) {
              return { ...s, status: 'in-progress' };
            }
            return s;
          }),
        );
      },
      () => {
        setPhase('done');
        invalidateSkillData();
      },
      (err) => {
        toast(err.message, 'error');
        setPhase('done');
      },
      { names, force: opts?.force ?? false },
    );
  }, [selected, updatableItems, invalidateSkillData, toast]);

  const patchItem = useCallback(
    (name: string, patch: Partial<ItemUpdateStatus>) =>
      setItemStatuses((prev) => prev.map((s) => (s.name === name ? { ...s, ...patch } : s))),
    [],
  );

  const handleRetryForce = useCallback(
    (name: string) => {
      patchItem(name, { status: 'in-progress', message: undefined });
      esRef.current = api.updateAllStream(
        () => {},
        (item) => {
          patchItem(name, {
            status: actionToStatus(item.action),
            message: item.message,
            auditRiskLabel: item.auditRiskLabel,
          });
        },
        () => invalidateSkillData(),
        (err) => patchItem(name, { status: 'error', message: err.message }),
        { names: [name], force: true },
      );
    },
    [patchItem, invalidateSkillData],
  );

  const handlePurge = useCallback(
    async (name: string) => {
      patchItem(name, { status: 'in-progress', message: t('update.updating.purging') });
      try {
        await api.batchUninstall({ names: [name], force: true });
        patchItem(name, { status: 'skipped', message: t('update.updating.purged') });
        invalidateSkillData();
      } catch (err) {
        patchItem(name, { status: 'error', message: (err as Error).message });
      }
    },
    [patchItem, invalidateSkillData],
  );

  const handleBackToList = useCallback(() => {
    setPhase('selecting');
    setItemStatuses([]);
    setSelected(new Set());
    setCheckStatuses(new Map());
    itemRefs.current = {};
  }, []);

  /* ── Derived counts ──────────────────────────────── */

  const { successCount, skippedCount, blockedCount, errorCount, completedCount } = useMemo(() => {
    let success = 0, skipped = 0, blocked = 0, error = 0;
    for (const s of itemStatuses) {
      switch (s.status) {
        case 'success': success++; break;
        case 'skipped': skipped++; break;
        case 'blocked': blocked++; break;
        case 'error': error++; break;
      }
    }
    return {
      successCount: success,
      skippedCount: skipped,
      blockedCount: blocked,
      errorCount: error,
      completedCount: success + skipped + blocked + error,
    };
  }, [itemStatuses]);

  /* ── Render ──────────────────────────────────────── */

  if (isPending) return <PageSkeleton />;

  // ── Selecting phase ──────────────────────────────

  if (phase === 'selecting') {
    const typeFilterOptions = [
      {
        value: 'all' as TypeFilter,
        label: <span className="inline-flex items-center gap-1.5"><LayoutGrid size={14} strokeWidth={2.5} />{t('update.filter.all')}</span>,
        count: filterCounts.all,
      },
      {
        value: 'tracked' as TypeFilter,
        label: <span className="inline-flex items-center gap-1.5"><Users size={14} strokeWidth={2.5} />{t('update.filter.tracked')}</span>,
        count: filterCounts.tracked,
      },
      {
        value: 'github' as TypeFilter,
        label: <span className="inline-flex items-center gap-1.5"><Globe size={14} strokeWidth={2.5} />{t('update.filter.github')}</span>,
        count: filterCounts.github,
      },
    ];

    return (
      <div className="space-y-3 animate-fade-in">
        <PageHeader
          icon={<ArrowUpCircle size={24} strokeWidth={2.5} />}
          title={t('update.header.title')}
          subtitle={t('update.header.subtitle')}
          actions={
            <>
              <Button
                variant="ghost"
                size="sm"
                onClick={handleCheckAll}
                loading={checking && checkMode === 'all'}
                disabled={checking || updatableItems.length === 0}
              >
                <RefreshCw size={16} />
                {t('update.header.checkAll')}
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={handleCheckSelected}
                loading={checking && checkMode === 'selected'}
                disabled={checking || selected.size === 0}
              >
                <RefreshCw size={16} />
                {t('update.header.checkSelected')}
              </Button>
              <SplitButton
                variant="primary"
                size="sm"
                onClick={() => handleUpdate()}
                disabled={selected.size === 0}
                dropdownAlign="right"
                items={[
                  {
                    label: t('update.header.forceUpdate'),
                    icon: <Zap size={14} strokeWidth={2.5} />,
                    onClick: () => handleUpdate({ force: true }),
                    confirm: true,
                  },
                ]}
              >
                <ArrowUpCircle size={16} />
                {t('update.header.updateSelected', { count: selected.size })}
              </SplitButton>
            </>
          }
        />

        {allUpdatableItems.length === 0 ? (
          <EmptyState
            icon={Check}
            title={t('update.empty.title')}
            description={t('update.empty.description')}
          />
        ) : (
          <>
            {/* Resource type tabs (Skills / Agents) */}
            <nav
              className="ss-resource-tabs flex items-center gap-6 border-b-2 border-muted -mx-4 px-4 md:-mx-8 md:px-8"
              role="tablist"
            >
              {([
                { key: 'skills' as ResourceTab, icon: <Puzzle size={16} strokeWidth={2.5} />, label: t('resources.tab.skills'), count: skillTabCount },
                { key: 'agents' as ResourceTab, icon: <Bot size={16} strokeWidth={2.5} />, label: t('resources.tab.agents'), count: agentTabCount },
              ]).map((tab) => (
                <button
                  key={tab.key}
                  type="button"
                  role="tab"
                  aria-selected={activeTab === tab.key}
                  onClick={() => changeTab(tab.key)}
                  className={`
                    ss-resource-tab
                    inline-flex items-center gap-1.5 px-1 pb-2.5 text-sm font-semibold cursor-pointer
                    transition-all duration-150 border-b-[3px] -mb-[2px]
                    ${activeTab === tab.key
                      ? 'border-pencil text-pencil'
                      : 'border-transparent text-pencil-light hover:text-pencil hover:border-muted-dark'
                    }
                  `}
                >
                  {tab.icon}
                  {tab.label}
                  <span className={`
                    text-[11px] font-medium px-1.5 py-0.5 rounded-[var(--radius-sm)]
                    ${activeTab === tab.key ? 'bg-pencil/10 text-pencil' : 'bg-muted text-pencil-light'}
                  `}>
                    {tab.count}
                  </span>
                </button>
              ))}
            </nav>

            {/* Sticky toolbar */}
            <div className="sticky top-0 z-20 bg-paper -mx-4 px-4 md:-mx-8 md:px-8 py-2 mb-1 space-y-2">
              {/* Search */}
              <div className="relative">
                <Search
                  size={18}
                  strokeWidth={2.5}
                  className="absolute left-4 top-1/2 -translate-y-1/2 text-muted-dark pointer-events-none"
                />
                <Input
                  placeholder={t('update.list.searchPlaceholder', { kind: activeTab === 'agents' ? t('resources.tab.agents').toLowerCase() : t('resources.tab.skills').toLowerCase() })}
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  className="!pl-11"
                />
              </div>

              {/* Filters + selection actions */}
              <div className="flex flex-wrap items-center gap-3">
                <SegmentedControl
                  value={typeFilter}
                  onChange={setTypeFilter}
                  size="sm"
                  options={typeFilterOptions}
                />
                <div className="flex items-center gap-2 ml-auto">
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={allInViewSelected ? deselectAll : selectAll}
                    disabled={filtered.length === 0}
                  >
                    {allInViewSelected ? t('update.list.deselectAll') : t('update.list.selectAll')}
                  </Button>
                  {selected.size > 0 && (
                    <Button variant="ghost" size="sm" onClick={() => setSelected(new Set())}>
                      {t('update.list.clear')}
                    </Button>
                  )}
                </div>
              </div>
            </div>

            {/* Virtuoso list */}
            {filtered.length === 0 ? (
              <div className="py-12 text-center">
                <p className="text-pencil-light text-sm">
                  {updatableItems.length === 0
                    ? t('update.list.noUpdatable', { kind: activeTab === 'agents' ? t('resources.tab.agents').toLowerCase() : t('resources.tab.skills').toLowerCase() })
                    : t('update.list.noMatch', { kind: activeTab === 'agents' ? t('resources.tab.agents').toLowerCase() : t('resources.tab.skills').toLowerCase() })}
                </p>
              </div>
            ) : (
              <div
                className="border border-muted bg-surface overflow-hidden"
                style={{ borderRadius: radius.md }}
              >
                <Virtuoso
                  useWindowScroll
                  totalCount={filtered.length}
                  overscan={200}
                  itemContent={(index) => {
                    const item = filtered[index];
                    const isSelected = selected.has(item.name);
                    const checkStatus = checkStatuses.get(item.name) ?? { status: 'unchecked' as CheckStatus };
                    return (
                      <button
                        type="button"
                        className={`
                          w-full text-left px-4 py-2.5 flex items-center gap-3
                          transition-colors duration-100 cursor-pointer
                          ${index > 0 ? 'border-t border-muted/40' : ''}
                          ${isSelected ? 'bg-blue/5' : 'hover:bg-muted/15'}
                        `}
                        onClick={() => toggleSelect(item.name)}
                      >
                        <Checkbox
                          label=""
                          checked={isSelected}
                          onChange={() => toggleSelect(item.name)}
                          size="sm"
                        />
                        <div className="flex-1 min-w-0">
                          <span className="font-mono text-sm text-pencil truncate flex items-center gap-1.5">
                            <KindBadge kind={item.kind} />
                            {item.name}
                          </span>
                          {item.source && (
                            <span className="text-xs text-pencil-light truncate block">
                              {[item.source, item.installedAt && formatRelativeTime(item.installedAt)]
                                .filter(Boolean)
                                .join(' · ')}
                            </span>
                          )}
                        </div>
                        <div className="flex items-center gap-1.5 shrink-0">
                          <SourceBadge type={item.type} isInRepo={item.isInRepo} />
                          <CheckStatusBadge status={checkStatus} />
                        </div>
                      </button>
                    );
                  }}
                />
              </div>
            )}
          </>
        )}
      </div>
    );
  }

  // ── Updating phase ───────────────────────────────

  if (phase === 'updating') {
    return (
      <div className="space-y-4 animate-fade-in">
        <PageHeader
          icon={<ArrowUpCircle size={24} strokeWidth={2.5} />}
          title={t('update.header.title')}
          subtitle={t('update.updating.subtitle')}
        />

        {/* Sticky progress bar */}
        <div className="sticky top-0 z-20 bg-paper -mx-4 px-4 md:-mx-8 md:px-8 pt-2 pb-3">
          <StreamProgressBar
            count={completedCount}
            total={itemStatuses.length}
            startTime={startTimeRef.current}
            icon={ArrowUpCircle}
            iconClassName=""
            labelDiscovering={t('update.progress.labelDiscovering')}
            labelRunning={t('update.progress.labelRunning')}
            units={t('update.progress.units')}
          />
        </div>

        {/* Per-item status cards */}
        <div className="space-y-2">
          {itemStatuses.map((item, i) => (
            <ItemStatusCard
              key={item.name}
              item={item}
              index={i}
              ref={(el) => { itemRefs.current[item.name] = el; }}
            />
          ))}
        </div>
      </div>
    );
  }

  // ── Done phase ───────────────────────────────────

  return (
    <div className="space-y-4 animate-fade-in">
      <PageHeader
        icon={<ArrowUpCircle size={24} strokeWidth={2.5} />}
        title={t('update.header.title')}
        subtitle={t('update.done.subtitle')}
        actions={
          <Button variant="ghost" size="sm" onClick={handleBackToList}>
            {t('update.done.backToList')}
          </Button>
        }
      />

      {/* Summary card */}
      <Card tilt>
        <div className="flex items-center justify-between flex-wrap gap-3">
          <div className="flex items-center gap-2 flex-wrap">
            {successCount > 0 && <Badge variant="success">{t('update.summary.updated', { count: successCount })}</Badge>}
            {skippedCount > 0 && <Badge>{t('update.summary.skipped', { count: skippedCount })}</Badge>}
            {blockedCount > 0 && <Badge variant="warning">{t('update.summary.blocked', { count: blockedCount })}</Badge>}
            {errorCount > 0 && <Badge variant="danger">{t('update.summary.failed', { count: errorCount })}</Badge>}
          </div>
        </div>
      </Card>

      {/* Per-item result cards with action buttons */}
      <div className="space-y-2">
        {itemStatuses.map((item, i) => (
          <ItemStatusCard
            key={item.name}
            item={item}
            index={i}
            showActions
            onRetryForce={handleRetryForce}
            onPurge={handlePurge}
          />
        ))}
      </div>
    </div>
  );
}

/* ── Shared item status card ─────────────────────────── */

interface ItemStatusCardProps {
  item: ItemUpdateStatus;
  index: number;
  showActions?: boolean;
  onRetryForce?: (name: string) => void;
  onPurge?: (name: string) => void;
}

const ItemStatusCard = forwardRef<HTMLDivElement, ItemStatusCardProps>(
  ({ item, index, showActions, onRetryForce, onPurge }, ref) => {
  const t = useT();
  return (
    <div
      ref={ref}
      className={`flex items-center gap-3 px-3 py-2 border transition-colors animate-fade-in ${
        item.status === 'error'
          ? 'border-danger/40 bg-danger-light/50'
          : item.status === 'blocked'
          ? 'border-warning/40 bg-warning-light/50'
          : 'border-muted hover:bg-muted/30'
      }`}
      style={{
        borderRadius: radius.sm,
        animationDelay: `${index * 50}ms`,
        animationFillMode: 'backwards',
      }}
    >
      <StatusIcon status={item.status} />
      <div className="flex-1 min-w-0">
        <span className="text-pencil font-medium flex items-center gap-1.5">
          {item.kind && <KindBadge kind={item.kind} />}
          {item.name}
        </span>
        {item.message && (
          <span
            className={`text-sm block ${
              item.status === 'error'
                ? 'text-danger font-medium whitespace-pre-wrap mt-1'
                : item.status === 'blocked'
                ? 'text-warning whitespace-pre-wrap mt-1'
                : 'text-pencil-light truncate'
            }`}
          >
            {item.message}
          </span>
        )}
      </div>
      <div className="flex items-center gap-2 shrink-0">
        {item.auditRiskLabel && item.auditRiskLabel !== 'clean' && (
          <Badge variant={item.auditRiskLabel === 'critical' || item.auditRiskLabel === 'high' ? 'danger' : 'warning'}>
            <ShieldAlert size={12} className="mr-1" />
            {item.auditRiskLabel}
          </Badge>
        )}
        {showActions && item.status === 'error' && (
          isStaleError(item.message) ? (
            <Button variant="danger" size="sm" onClick={() => onPurge?.(item.name)}>
              <Trash2 size={14} />
              {t('update.updating.purge')}
            </Button>
          ) : (
            <Button variant="danger" size="sm" onClick={() => onRetryForce?.(item.name)}>
              <RefreshCw size={14} />
              {t('update.updating.forceRetry')}
            </Button>
          )
        )}
        {showActions && item.status === 'blocked' && (
          <Button variant="warning" size="sm" onClick={() => onRetryForce?.(item.name)}>
            <RefreshCw size={14} />
            {t('update.updating.forceRetry')}
          </Button>
        )}
        <StatusBadge status={item.status} />
      </div>
    </div>
  );
  }
);
ItemStatusCard.displayName = 'ItemStatusCard';

/* ── Helper functions ──────────────────────────────── */

function formatRelativeTime(dateStr: string): string {
  const now = Date.now();
  const then = new Date(dateStr).getTime();
  if (isNaN(then)) return dateStr;
  const diff = Math.floor((now - then) / 1000);
  if (diff < 60) return 'just now';
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  if (diff < 2592000) return `${Math.floor(diff / 86400)}d ago`;
  if (diff < 31536000) return `${Math.floor(diff / 2592000)}mo ago`;
  return `${Math.floor(diff / 31536000)}y ago`;
}

function isStaleError(message?: string): boolean {
  if (!message) return false;
  return message.includes('does not exist in repository') || message.includes('not found in repository');
}

function actionToStatus(action: string): ItemUpdateStatus['status'] {
  switch (action) {
    case 'updated': return 'success';
    case 'error': return 'error';
    case 'blocked': return 'blocked';
    case 'skipped':
    case 'up-to-date': return 'skipped';
    default: return 'success';
  }
}

function StatusIcon({ status }: { status: ItemUpdateStatus['status'] }) {
  switch (status) {
    case 'pending':
      return <Circle size={16} className="text-muted-dark shrink-0" />;
    case 'in-progress':
      return <Loader2 size={16} className="text-blue animate-spin shrink-0" />;
    case 'success':
      return <CheckCircle size={16} className="text-success shrink-0" />;
    case 'error':
      return <XCircle size={16} className="text-danger shrink-0" />;
    case 'blocked':
      return <ShieldAlert size={16} className="text-warning shrink-0" />;
    case 'skipped':
      return <MinusCircle size={16} className="text-muted-dark shrink-0" />;
  }
}

function StatusBadge({ status }: { status: ItemUpdateStatus['status'] }) {
  const t = useT();
  switch (status) {
    case 'pending':
      return <Badge>{t('update.status.pending')}</Badge>;
    case 'in-progress':
      return <Badge variant="info">{t('update.status.updating')}</Badge>;
    case 'success':
      return <Badge variant="success">{t('update.status.updated')}</Badge>;
    case 'error':
      return <Badge variant="danger">{t('update.status.failed')}</Badge>;
    case 'blocked':
      return <Badge variant="warning">{t('update.status.blocked')}</Badge>;
    case 'skipped':
      return <Badge>{t('update.status.skipped')}</Badge>;
  }
}

function CheckStatusBadge({ status }: { status: CheckItemStatus }) {
  const t = useT();
  switch (status.status) {
    case 'unchecked':
      return <Badge size="sm">{t('update.check.unchecked')}</Badge>;
    case 'checking':
      return <Badge variant="info" size="sm"><Loader2 size={10} className="animate-spin mr-1" />{t('update.check.checking')}</Badge>;
    case 'behind':
      return <Badge variant="warning" size="sm">{status.behind ? t('update.check.behind', { count: status.behind }) : t('update.check.behindFallback')}</Badge>;
    case 'up-to-date':
      return <Badge variant="success" size="sm">{t('update.check.upToDate')}</Badge>;
    case 'update-available':
      return <Badge variant="warning" size="sm">{t('update.check.updateAvailable')}</Badge>;
    case 'error':
      return <Badge variant="danger" size="sm">{t('update.check.error')}</Badge>;
  }
}
