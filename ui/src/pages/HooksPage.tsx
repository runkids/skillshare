import { useMemo, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  AlertTriangle,
  ArrowUpDown,
  Boxes,
  Eye,
  FolderOpen,
  Plus,
  ScrollText,
  Search,
  Sparkles,
  User,
} from 'lucide-react';
import Badge from '../components/Badge';
import Card from '../components/Card';
import EmptyState from '../components/EmptyState';
import FilterChip from '../components/FilterChip';
import HandButton from '../components/HandButton';
import { HandInput, HandSelect } from '../components/HandInput';
import ManagedModeTabs, { type ManagedRulesMode } from '../components/ManagedModeTabs';
import { PageSkeleton } from '../components/Skeleton';
import SelectionToggle from '../components/SelectionToggle';
import { useToast } from '../components/Toast';
import { api, type ManagedHook } from '../api/client';
import {
  formatHookDiscoveryGroupTitle,
  getHookActionPayload,
  groupDiscoveredHooks,
  type HookDiscoveryGroup,
} from '../lib/hookDiscovery';
import { queryKeys, staleTimes } from '../lib/queryKeys';

type HookSort = 'name-asc' | 'name-desc' | 'tool-asc' | 'event-asc';
type HookFilter = 'all' | 'project' | 'user' | 'collectible' | `tool:${string}`;

type FilterOption = {
  key: HookFilter;
  label: string;
  count: number;
  icon: React.ReactNode;
};

function isManagedMode(mode: string | null): mode is ManagedRulesMode {
  return mode === 'managed' || mode === 'discovered';
}

function compareText(left: string, right: string): number {
  return left.localeCompare(right, undefined, { sensitivity: 'base' });
}

function sortManagedHooks(hooks: ManagedHook[], sort: HookSort): ManagedHook[] {
  const sorted = [...hooks];
  switch (sort) {
    case 'name-asc':
      return sorted.sort((left, right) => compareText(left.id, right.id));
    case 'name-desc':
      return sorted.sort((left, right) => compareText(right.id, left.id));
    case 'tool-asc':
      return sorted.sort((left, right) => compareText(left.tool, right.tool) || compareText(left.id, right.id));
    case 'event-asc':
      return sorted.sort((left, right) => compareText(left.event, right.event) || compareText(left.id, right.id));
  }
}

function sortDiscoveredGroups(groups: HookDiscoveryGroup[], sort: HookSort): HookDiscoveryGroup[] {
  const sorted = [...groups];
  switch (sort) {
    case 'name-asc':
      return sorted.sort((left, right) => compareText(formatHookDiscoveryGroupTitle(left), formatHookDiscoveryGroupTitle(right)));
    case 'name-desc':
      return sorted.sort((left, right) => compareText(formatHookDiscoveryGroupTitle(right), formatHookDiscoveryGroupTitle(left)));
    case 'tool-asc':
      return sorted.sort((left, right) => compareText(left.sourceTool, right.sourceTool) || compareText(formatHookDiscoveryGroupTitle(left), formatHookDiscoveryGroupTitle(right)));
    case 'event-asc':
      return sorted.sort((left, right) => compareText(left.event, right.event) || compareText(formatHookDiscoveryGroupTitle(left), formatHookDiscoveryGroupTitle(right)));
  }
}

function managedMatchesSearch(hook: ManagedHook, search: string): boolean {
  if (!search) return true;
  const haystack = [
    hook.id,
    hook.tool,
    hook.event,
    hook.matcher ?? '',
    ...hook.handlers.flatMap((handler) => [
      handler.type,
      handler.command ?? '',
      handler.url ?? '',
      handler.prompt ?? '',
      handler.timeout ?? '',
      handler.statusMessage ?? '',
    ]),
  ].join('\n').toLowerCase();
  return haystack.includes(search);
}

function discoveredMatchesSearch(group: HookDiscoveryGroup, search: string): boolean {
  if (!search) return true;
  const haystack = [
    group.id,
    formatHookDiscoveryGroupTitle(group),
    group.sourceTool,
    group.scope,
    group.event,
    group.matcher,
    group.collectReason ?? '',
    ...group.hooks.flatMap((hook) => [
      hook.actionType,
      getHookActionPayload(hook),
      hook.path,
      hook.statusMessage ?? '',
    ]),
  ].join('\n').toLowerCase();
  return haystack.includes(search);
}

function managedMatchesFilter(hook: ManagedHook, filter: HookFilter): boolean {
  if (filter === 'all') return true;
  if (filter.startsWith('tool:')) return hook.tool === filter.slice(5);
  return true;
}

function discoveredMatchesFilter(group: HookDiscoveryGroup, filter: HookFilter): boolean {
  switch (filter) {
    case 'all':
      return true;
    case 'collectible':
      return group.collectible;
    case 'project':
      return group.scope === 'project';
    case 'user':
      return group.scope === 'user';
    default:
      return filter.startsWith('tool:') ? group.sourceTool === filter.slice(5) : true;
  }
}

function buildManagedFilterOptions(hooks: ManagedHook[]): FilterOption[] {
  const byTool = new Map<string, number>();
  for (const hook of hooks) {
    byTool.set(hook.tool, (byTool.get(hook.tool) ?? 0) + 1);
  }

  return [
    { key: 'all', label: 'All', count: hooks.length, icon: <Boxes size={14} strokeWidth={2.5} /> },
    ...Array.from(byTool.entries())
      .sort(([left], [right]) => compareText(left, right))
      .map(([tool, count]) => ({
        key: `tool:${tool}` as HookFilter,
        label: tool,
        count,
        icon: <Boxes size={14} strokeWidth={2.5} />,
      })),
  ];
}

function buildDiscoveredFilterOptions(groups: HookDiscoveryGroup[]): FilterOption[] {
  const byTool = new Map<string, number>();
  for (const group of groups) {
    byTool.set(group.sourceTool, (byTool.get(group.sourceTool) ?? 0) + 1);
  }

  return [
    { key: 'all', label: 'All', count: groups.length, icon: <Boxes size={14} strokeWidth={2.5} /> },
    {
      key: 'collectible',
      label: 'Collectible',
      count: groups.filter((group) => group.collectible).length,
      icon: <Sparkles size={14} strokeWidth={2.5} />,
    },
    {
      key: 'project',
      label: 'Project',
      count: groups.filter((group) => group.scope === 'project').length,
      icon: <FolderOpen size={14} strokeWidth={2.5} />,
    },
    {
      key: 'user',
      label: 'User',
      count: groups.filter((group) => group.scope === 'user').length,
      icon: <User size={14} strokeWidth={2.5} />,
    },
    ...Array.from(byTool.entries())
      .sort(([left], [right]) => compareText(left, right))
      .map(([tool, count]) => ({
        key: `tool:${tool}` as HookFilter,
        label: tool,
        count,
        icon: <Boxes size={14} strokeWidth={2.5} />,
      })),
  ];
}

const sortOptions = [
  { value: 'name-asc', label: 'Name A -> Z' },
  { value: 'name-desc', label: 'Name Z -> A' },
  { value: 'tool-asc', label: 'Tool A -> Z' },
  { value: 'event-asc', label: 'Event A -> Z' },
];

export default function HooksPage() {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const { toast } = useToast();
  const [searchParams, setSearchParams] = useSearchParams();
  const parsedMode = searchParams.get('mode');
  const mode = (isManagedMode(parsedMode) ? parsedMode : 'managed') as ManagedRulesMode;
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [collectStatus, setCollectStatus] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [sort, setSort] = useState<HookSort>('name-asc');
  const [filter, setFilter] = useState<HookFilter>('all');

  const managedQuery = useQuery({
    queryKey: queryKeys.hooks.managed,
    queryFn: () => api.managedHooks.list(),
    staleTime: staleTimes.hooks,
  });

  const discoveredQuery = useQuery({
    queryKey: queryKeys.hooks.discovered,
    queryFn: () => api.listHooks(),
    staleTime: staleTimes.hooks,
  });

  const collectMutation = useMutation({
    mutationFn: (groupIds: string[]) =>
      api.managedHooks.collect({
        groupIds,
        strategy: 'overwrite',
      }),
    onSuccess: (result) => {
      const created = result.created.length;
      const overwritten = result.overwritten.length;
      const skipped = result.skipped.length;
      setSelectedIds([]);
      setCollectStatus(`Collected ${created} created, ${overwritten} overwritten, ${skipped} skipped.`);
      toast('Selected hooks collected.', 'success');
      queryClient.invalidateQueries({ queryKey: queryKeys.hooks.managed, exact: true });
      queryClient.invalidateQueries({ queryKey: queryKeys.hooks.discovered });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
    },
    onError: (error: Error) => {
      toast(error.message, 'error');
    },
  });

  const managedHooks = useMemo(() => managedQuery.data?.hooks ?? [], [managedQuery.data?.hooks]);
  const discoveredHooks = useMemo(() => discoveredQuery.data?.hooks ?? [], [discoveredQuery.data?.hooks]);
  const warnings = useMemo(() => discoveredQuery.data?.warnings ?? [], [discoveredQuery.data?.warnings]);
  const groups = useMemo(() => groupDiscoveredHooks(discoveredHooks), [discoveredHooks]);
  const normalizedSearch = search.trim().toLowerCase();

  const managedFilterOptions = useMemo(() => buildManagedFilterOptions(managedHooks), [managedHooks]);
  const discoveredFilterOptions = useMemo(() => buildDiscoveredFilterOptions(groups), [groups]);
  const filterOptions = mode === 'managed' ? managedFilterOptions : discoveredFilterOptions;
  const activeFilter = filterOptions.some((option) => option.key === filter) ? filter : 'all';

  const filteredManagedHooks = useMemo(
    () =>
      sortManagedHooks(
        managedHooks.filter((hook) => managedMatchesFilter(hook, activeFilter) && managedMatchesSearch(hook, normalizedSearch)),
        sort,
      ),
    [managedHooks, activeFilter, normalizedSearch, sort],
  );

  const filteredGroups = useMemo(
    () =>
      sortDiscoveredGroups(
        groups.filter((group) => discoveredMatchesFilter(group, activeFilter) && discoveredMatchesSearch(group, normalizedSearch)),
        sort,
      ),
    [groups, activeFilter, normalizedSearch, sort],
  );

  const openMode = (next: ManagedRulesMode) => {
    const params = new URLSearchParams(searchParams);
    if (next === 'managed') params.delete('mode');
    else params.set('mode', next);
    setSearchParams(params);
    setSelectedIds([]);
    setCollectStatus(null);
    setFilter('all');
    setSearch('');
  };

  const toggleSelected = (group: HookDiscoveryGroup) => {
    if (!group.collectible) return;
    setSelectedIds((current) =>
      current.includes(group.id)
        ? current.filter((currentId) => currentId !== group.id)
        : [...current, group.id],
    );
  };

  const collectAndEdit = async (group: HookDiscoveryGroup) => {
    const result = await collectMutation.mutateAsync([group.id]);
    const managedId = result.created[0] ?? result.overwritten[0];
    if (managedId) {
      navigate(`/hooks/manage/${managedId}`);
    }
  };

  const viewDiscoveredGroup = (group: HookDiscoveryGroup) => {
    navigate(`/hooks/discovered/${encodeURIComponent(group.id)}`, {
      state: { group },
    });
  };

  if ((mode === 'managed' && managedQuery.isPending && !managedQuery.data) || (mode === 'discovered' && discoveredQuery.isPending && !discoveredQuery.data)) {
    return <PageSkeleton />;
  }

  if ((mode === 'managed' && managedQuery.error && !managedQuery.data) || (mode === 'discovered' && discoveredQuery.error && !discoveredQuery.data)) {
    const error = mode === 'managed' ? managedQuery.error : discoveredQuery.error;
    return (
      <Card variant="accent" className="py-8 text-center">
        <p className="text-lg text-danger" style={{ fontFamily: 'var(--font-heading)' }}>
          Failed to load hooks
        </p>
        <p className="mt-1 text-sm text-pencil-light">{error?.message}</p>
      </Card>
    );
  }

  const activeItems = mode === 'managed' ? filteredManagedHooks : filteredGroups;
  const totalItems = mode === 'managed' ? managedHooks.length : groups.length;
  const hasActiveFilters = normalizedSearch.length > 0 || activeFilter !== 'all';
  const modeLabel = mode === 'managed'
    ? `Hooks · ${managedHooks.length} item${managedHooks.length !== 1 ? 's' : ''}`
    : `Discovered hooks · ${groups.length} item${groups.length !== 1 ? 's' : ''}`;

  return (
    <div className="animate-sketch-in space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h2 className="mb-1 text-3xl font-bold text-pencil md:text-4xl" style={{ fontFamily: 'var(--font-heading)' }}>
            Hooks
          </h2>
          <p className="text-pencil-light">{modeLabel}</p>
        </div>

        {mode === 'managed' && (
          <HandButton variant="secondary" size="sm" onClick={() => navigate('/hooks/new')}>
            <Plus size={16} strokeWidth={2.5} />
            New Hook
          </HandButton>
        )}
      </div>

      <ManagedModeTabs
        mode={mode}
        onChange={openMode}
        managedCount={managedHooks.length}
        discoveredCount={groups.length}
        managedLabel="Hooks"
        label="Hooks mode"
      />

      <div className="space-y-4">
        <div className="flex flex-col gap-4 lg:flex-row">
          <div className="relative flex-1">
            <Search size={18} strokeWidth={2.5} className="pointer-events-none absolute top-1/2 left-4 -translate-y-1/2 text-muted-dark" />
            <HandInput
              aria-label="Search hooks"
              placeholder="Filter hooks..."
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              className="pl-11"
            />
          </div>
          <div className="w-full lg:w-60">
            <HandSelect
              value={sort}
              onChange={(value) => setSort(value as HookSort)}
              options={sortOptions}
            />
          </div>
        </div>

        <div className="flex flex-wrap gap-2">
          {filterOptions.map((option) => (
            <FilterChip
              key={option.key}
              label={option.label}
              icon={option.icon}
              active={activeFilter === option.key}
              count={option.count}
              onClick={() => setFilter(option.key)}
            />
          ))}
        </div>

        {hasActiveFilters && (
          <p className="inline-flex items-center gap-2 text-sm text-pencil-light">
            <ArrowUpDown size={14} strokeWidth={2.5} />
            Showing {activeItems.length} of {totalItems}
          </p>
        )}
      </div>

      {mode === 'discovered' && warnings.length > 0 && (
        <Card variant="accent">
          <div className="flex items-start gap-3">
            <AlertTriangle size={18} strokeWidth={2.5} className="mt-0.5 shrink-0 text-warning" />
            <div>
              <h3 className="mb-2 text-lg text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
                Scan warnings
              </h3>
              <ul className="space-y-1 text-sm text-pencil-light">
                {warnings.map((warning) => (
                  <li key={warning}>{warning}</li>
                ))}
              </ul>
            </div>
          </div>
        </Card>
      )}

      {mode === 'discovered' && selectedIds.length > 0 && (
        <Card variant="postit" className="flex flex-wrap items-center justify-between gap-3">
          <p className="text-pencil" style={{ fontFamily: 'var(--font-hand)' }}>
            {selectedIds.length} selected
          </p>
          <HandButton
            type="button"
            size="sm"
            variant="secondary"
            onClick={() => collectMutation.mutate(selectedIds)}
            disabled={collectMutation.isPending}
          >
            Collect Selected
          </HandButton>
        </Card>
      )}

      {mode === 'discovered' && collectStatus && (
        <Card variant="accent">
          <p className="text-sm text-pencil-light">{collectStatus}</p>
        </Card>
      )}

      {activeItems.length === 0 ? (
        <Card>
          <EmptyState
            icon={ScrollText}
            title={
              totalItems === 0
                ? mode === 'managed'
                  ? 'No hooks found'
                  : 'No discovered hooks found'
                : 'No hooks match these filters'
            }
            description={
              totalItems === 0
                ? mode === 'managed'
                  ? 'Create a hook to start publishing compiled files.'
                  : 'No hook diagnostics were discovered in the current user or project scope.'
                : 'Try a different search term or filter.'
            }
          />
        </Card>
      ) : mode === 'managed' ? (
        <div className="grid gap-4">
          {filteredManagedHooks.map((hook) => (
            <Card key={hook.id} hover>
              <div className="flex items-start justify-between gap-4">
                <div className="min-w-0 space-y-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <h3 className="text-xl text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
                      {hook.id}
                    </h3>
                    <Badge variant="accent">{hook.tool}</Badge>
                    <Badge variant="warning">{hook.event}</Badge>
                  </div>
                  <p className="break-all text-sm text-pencil-light" style={{ fontFamily: "'Courier New', monospace" }}>
                    {hook.matcher || 'All'}
                  </p>
                  <p className="text-sm text-pencil-light">
                    {hook.handlers.length} handler{hook.handlers.length !== 1 ? 's' : ''}
                  </p>
                </div>

                <HandButton
                  variant="secondary"
                  size="sm"
                  className="shrink-0"
                  onClick={() => navigate(`/hooks/manage/${hook.id}`)}
                >
                  <Eye size={16} strokeWidth={2.5} />
                  Edit Hook
                </HandButton>
              </div>
            </Card>
          ))}
        </div>
      ) : (
        <div className="grid gap-4">
          {filteredGroups.map((group) => {
            const checked = selectedIds.includes(group.id);
            return (
              <Card key={group.id}>
                <div className="space-y-4">
                  <div className="flex items-start justify-between gap-4">
                    <div className="min-w-0 space-y-2">
                      <div className="flex flex-wrap items-center gap-2">
                        <h3 className="text-xl text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
                          <Link
                            to={`/hooks/discovered/${encodeURIComponent(group.id)}`}
                            state={{ group }}
                            className="hover:text-blue"
                          >
                            {formatHookDiscoveryGroupTitle(group)}
                          </Link>
                        </h3>
                        <Badge variant={group.scope === 'project' ? 'info' : 'default'}>{group.scope}</Badge>
                        <Badge variant="accent">{group.matcher}</Badge>
                        {group.collectible && <Badge variant="success">Collectible</Badge>}
                      </div>
                      <p className="break-all text-sm text-pencil-light" style={{ fontFamily: "'Courier New', monospace" }}>
                        {group.id}
                      </p>
                      {group.collectReason && <p className="text-sm text-pencil-light">{group.collectReason}</p>}
                    </div>

                    <div className="flex shrink-0 flex-col items-end gap-2">
                      <HandButton
                        variant="secondary"
                        size="sm"
                        onClick={() => viewDiscoveredGroup(group)}
                      >
                        <Eye size={16} strokeWidth={2.5} />
                        View Hook
                      </HandButton>
                      {group.collectible && (
                        <SelectionToggle
                          label={`Collect ${group.matcher}`}
                          checked={checked}
                          onChange={() => toggleSelected(group)}
                        />
                      )}
                      {group.collectible && (
                        <HandButton
                          variant="ghost"
                          size="sm"
                          disabled={collectMutation.isPending}
                          onClick={() => collectAndEdit(group)}
                        >
                          {collectMutation.isPending ? 'Collecting...' : 'Collect & Edit'}
                        </HandButton>
                      )}
                    </div>
                  </div>

                  <div className="space-y-3">
                    {group.hooks.map((hook, index) => (
                      <div
                        key={`${group.id}:${index}`}
                        className="space-y-2 border-t border-dashed border-muted-dark pt-3 first:border-t-0 first:pt-0"
                      >
                        <div className="flex flex-wrap items-center gap-2">
                          <Badge variant="default">{hook.actionType}</Badge>
                          {hook.timeout && <Badge variant="warning">{hook.timeout}</Badge>}
                          {hook.timeoutSec !== undefined && <Badge variant="info">{hook.timeoutSec}s</Badge>}
                        </div>
                        <p className="text-xs uppercase tracking-[0.2em] text-pencil-light">Action payload</p>
                        <pre className="overflow-x-auto whitespace-pre-wrap break-words rounded-md bg-surface px-3 py-2 text-sm text-pencil">
                          {getHookActionPayload(hook)}
                        </pre>
                        {hook.statusMessage && <p className="text-sm text-pencil-light">{hook.statusMessage}</p>}
                        <p className="break-all text-sm text-pencil-light" style={{ fontFamily: "'Courier New', monospace" }}>
                          {hook.path}
                        </p>
                      </div>
                    ))}
                  </div>
                </div>
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}
