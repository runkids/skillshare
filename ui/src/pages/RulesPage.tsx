import { useMemo, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
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
import Card from '../components/Card';
import Badge from '../components/Badge';
import EmptyState from '../components/EmptyState';
import FilterChip from '../components/FilterChip';
import HandButton from '../components/HandButton';
import { HandInput, HandSelect } from '../components/HandInput';
import ManagedModeTabs, { type ManagedRulesMode } from '../components/ManagedModeTabs';
import { PageSkeleton } from '../components/Skeleton';
import SelectionToggle from '../components/SelectionToggle';
import { useToast } from '../components/Toast';
import { api, type ManagedRule, type RuleItem } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';

type RuleSort = 'name-asc' | 'name-desc' | 'tool-asc' | 'path-asc';
type RuleFilter = 'all' | 'project' | 'user' | 'collectible' | `tool:${string}`;

type FilterOption = {
  key: RuleFilter;
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

function getRuleRef(rule: RuleItem): string {
  return rule.id ?? rule.path;
}

function sortManagedRules(rules: ManagedRule[], sort: RuleSort): ManagedRule[] {
  const sorted = [...rules];
  switch (sort) {
    case 'name-asc':
      return sorted.sort((left, right) => compareText(left.name, right.name));
    case 'name-desc':
      return sorted.sort((left, right) => compareText(right.name, left.name));
    case 'tool-asc':
      return sorted.sort((left, right) => compareText(left.tool, right.tool) || compareText(left.name, right.name));
    case 'path-asc':
      return sorted.sort((left, right) => compareText(left.relativePath, right.relativePath));
  }
}

function sortDiscoveredRules(rules: RuleItem[], sort: RuleSort): RuleItem[] {
  const sorted = [...rules];
  switch (sort) {
    case 'name-asc':
      return sorted.sort((left, right) => compareText(left.name, right.name));
    case 'name-desc':
      return sorted.sort((left, right) => compareText(right.name, left.name));
    case 'tool-asc':
      return sorted.sort((left, right) => compareText(left.sourceTool, right.sourceTool) || compareText(left.name, right.name));
    case 'path-asc':
      return sorted.sort((left, right) => compareText(left.path, right.path));
  }
}

function managedMatchesSearch(rule: ManagedRule, search: string): boolean {
  if (!search) return true;
  const haystack = [rule.name, rule.tool, rule.relativePath, rule.content].join('\n').toLowerCase();
  return haystack.includes(search);
}

function discoveredMatchesSearch(rule: RuleItem, search: string): boolean {
  if (!search) return true;
  const haystack = [
    rule.name,
    rule.sourceTool,
    rule.scope,
    rule.path,
    rule.content,
    rule.collectReason ?? '',
  ].join('\n').toLowerCase();
  return haystack.includes(search);
}

function managedMatchesFilter(rule: ManagedRule, filter: RuleFilter): boolean {
  if (filter === 'all') return true;
  if (filter.startsWith('tool:')) return rule.tool === filter.slice(5);
  return true;
}

function discoveredMatchesFilter(rule: RuleItem, filter: RuleFilter): boolean {
  switch (filter) {
    case 'all':
      return true;
    case 'collectible':
      return Boolean(rule.collectible && rule.id);
    case 'project':
      return rule.scope === 'project';
    case 'user':
      return rule.scope === 'user';
    default:
      return filter.startsWith('tool:') ? rule.sourceTool === filter.slice(5) : true;
  }
}

function buildManagedFilterOptions(rules: ManagedRule[]): FilterOption[] {
  const byTool = new Map<string, number>();
  for (const rule of rules) {
    byTool.set(rule.tool, (byTool.get(rule.tool) ?? 0) + 1);
  }

  return [
    { key: 'all', label: 'All', count: rules.length, icon: <Boxes size={14} strokeWidth={2.5} /> },
    ...Array.from(byTool.entries())
      .sort(([left], [right]) => compareText(left, right))
      .map(([tool, count]) => ({
        key: `tool:${tool}` as RuleFilter,
        label: tool,
        count,
        icon: <Boxes size={14} strokeWidth={2.5} />,
      })),
  ];
}

function buildDiscoveredFilterOptions(rules: RuleItem[]): FilterOption[] {
  const byTool = new Map<string, number>();
  for (const rule of rules) {
    byTool.set(rule.sourceTool, (byTool.get(rule.sourceTool) ?? 0) + 1);
  }

  return [
    { key: 'all', label: 'All', count: rules.length, icon: <Boxes size={14} strokeWidth={2.5} /> },
    {
      key: 'collectible',
      label: 'Collectible',
      count: rules.filter((rule) => Boolean(rule.collectible && rule.id)).length,
      icon: <Sparkles size={14} strokeWidth={2.5} />,
    },
    {
      key: 'project',
      label: 'Project',
      count: rules.filter((rule) => rule.scope === 'project').length,
      icon: <FolderOpen size={14} strokeWidth={2.5} />,
    },
    {
      key: 'user',
      label: 'User',
      count: rules.filter((rule) => rule.scope === 'user').length,
      icon: <User size={14} strokeWidth={2.5} />,
    },
    ...Array.from(byTool.entries())
      .sort(([left], [right]) => compareText(left, right))
      .map(([tool, count]) => ({
        key: `tool:${tool}` as RuleFilter,
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
  { value: 'path-asc', label: 'Path A -> Z' },
];

export default function RulesPage() {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const { toast } = useToast();
  const [searchParams, setSearchParams] = useSearchParams();
  const parsedMode = searchParams.get('mode');
  const mode = (isManagedMode(parsedMode) ? parsedMode : 'managed') as ManagedRulesMode;
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [collectStatus, setCollectStatus] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [sort, setSort] = useState<RuleSort>('name-asc');
  const [filter, setFilter] = useState<RuleFilter>('all');

  const managedQuery = useQuery({
    queryKey: queryKeys.rules.managed,
    queryFn: () => api.managedRules.list(),
    staleTime: staleTimes.rules,
  });

  const discoveredQuery = useQuery({
    queryKey: queryKeys.rules.discovered,
    queryFn: () => api.listRules(),
    staleTime: staleTimes.rules,
  });

  const collectMutation = useMutation({
    mutationFn: (ids: string[]) =>
      api.managedRules.collect({
        ids,
        strategy: 'overwrite',
      }),
    onSuccess: (result) => {
      const created = result.created.length;
      const overwritten = result.overwritten.length;
      const skipped = result.skipped.length;
      setSelectedIds([]);
      setCollectStatus(`Collected ${created} created, ${overwritten} overwritten, ${skipped} skipped.`);
      toast('Selected rules collected.', 'success');
      queryClient.invalidateQueries({ queryKey: queryKeys.rules.managed, exact: true });
      queryClient.invalidateQueries({ queryKey: queryKeys.rules.discovered });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
    },
    onError: (error: Error) => {
      toast(error.message, 'error');
    },
  });

  const managedRules = useMemo(() => managedQuery.data?.rules ?? [], [managedQuery.data?.rules]);
  const discoveredRules = useMemo(() => discoveredQuery.data?.rules ?? [], [discoveredQuery.data?.rules]);
  const warnings = useMemo(() => discoveredQuery.data?.warnings ?? [], [discoveredQuery.data?.warnings]);
  const normalizedSearch = search.trim().toLowerCase();

  const managedFilterOptions = useMemo(() => buildManagedFilterOptions(managedRules), [managedRules]);
  const discoveredFilterOptions = useMemo(() => buildDiscoveredFilterOptions(discoveredRules), [discoveredRules]);
  const filterOptions = mode === 'managed' ? managedFilterOptions : discoveredFilterOptions;
  const activeFilter = filterOptions.some((option) => option.key === filter) ? filter : 'all';

  const filteredManagedRules = useMemo(
    () =>
      sortManagedRules(
        managedRules.filter((rule) => managedMatchesFilter(rule, activeFilter) && managedMatchesSearch(rule, normalizedSearch)),
        sort,
      ),
    [managedRules, activeFilter, normalizedSearch, sort],
  );

  const filteredDiscoveredRules = useMemo(
    () =>
      sortDiscoveredRules(
        discoveredRules.filter(
          (rule) => discoveredMatchesFilter(rule, activeFilter) && discoveredMatchesSearch(rule, normalizedSearch),
        ),
        sort,
      ),
    [discoveredRules, activeFilter, normalizedSearch, sort],
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

  const toggleSelected = (rule: RuleItem) => {
    const id = rule.id;
    if (!id) return;
    setSelectedIds((current) =>
      current.includes(id)
        ? current.filter((currentId) => currentId !== id)
        : [...current, id],
    );
  };

  const collectAndEdit = async (rule: RuleItem) => {
    if (!rule.id) return;
    const result = await collectMutation.mutateAsync([rule.id]);
    const managedId = result.created[0] ?? result.overwritten[0];
    if (managedId) {
      navigate(`/rules/manage/${managedId}`);
    }
  };

  if ((mode === 'managed' && managedQuery.isPending && !managedQuery.data) || (mode === 'discovered' && discoveredQuery.isPending && !discoveredQuery.data)) {
    return <PageSkeleton />;
  }

  if ((mode === 'managed' && managedQuery.error && !managedQuery.data) || (mode === 'discovered' && discoveredQuery.error && !discoveredQuery.data)) {
    const error = mode === 'managed' ? managedQuery.error : discoveredQuery.error;
    return (
      <Card variant="accent" className="py-8 text-center">
        <p className="text-lg text-danger" style={{ fontFamily: 'var(--font-heading)' }}>
          Failed to load rules
        </p>
        <p className="mt-1 text-sm text-pencil-light">{error?.message}</p>
      </Card>
    );
  }

  const activeRules = mode === 'managed' ? filteredManagedRules : filteredDiscoveredRules;
  const totalRules = mode === 'managed' ? managedRules.length : discoveredRules.length;
  const hasActiveFilters = normalizedSearch.length > 0 || activeFilter !== 'all';
  const modeLabel = mode === 'managed'
    ? `Rules · ${managedRules.length} item${managedRules.length !== 1 ? 's' : ''}`
    : `Discovered rules · ${discoveredRules.length} item${discoveredRules.length !== 1 ? 's' : ''}`;

  return (
    <div className="animate-sketch-in space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h2 className="mb-1 text-3xl font-bold text-pencil md:text-4xl" style={{ fontFamily: 'var(--font-heading)' }}>
            Rules
          </h2>
          <p className="text-pencil-light">{modeLabel}</p>
        </div>

        {mode === 'managed' && (
          <HandButton variant="secondary" size="sm" onClick={() => navigate('/rules/new')}>
            <Plus size={16} strokeWidth={2.5} />
            New Rule
          </HandButton>
        )}
      </div>

      <ManagedModeTabs
        mode={mode}
        onChange={openMode}
        managedCount={managedRules.length}
        discoveredCount={discoveredRules.length}
        managedLabel="Rules"
      />

      <div className="space-y-4">
        <div className="flex flex-col gap-4 lg:flex-row">
          <div className="relative flex-1">
            <Search size={18} strokeWidth={2.5} className="pointer-events-none absolute top-1/2 left-4 -translate-y-1/2 text-muted-dark" />
            <HandInput
              aria-label="Search rules"
              placeholder="Filter rules..."
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              className="pl-11"
            />
          </div>
          <div className="w-full lg:w-60">
            <HandSelect
              value={sort}
              onChange={(value) => setSort(value as RuleSort)}
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
            Showing {activeRules.length} of {totalRules}
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

      {activeRules.length === 0 ? (
        <Card>
          <EmptyState
            icon={ScrollText}
            title={
              totalRules === 0
                ? mode === 'managed'
                  ? 'No rules found'
                  : 'No discovered rules found'
                : 'No rules match these filters'
            }
            description={
              totalRules === 0
                ? mode === 'managed'
                  ? 'Create a rule to start publishing compiled files.'
                  : 'No rule files were discovered in the current user or project scope.'
                : 'Try a different search term or filter.'
            }
          />
        </Card>
      ) : mode === 'managed' ? (
        <div className="grid gap-4">
          {filteredManagedRules.map((rule) => (
            <Card key={rule.id} hover>
              <div className="flex items-start justify-between gap-4">
                <div className="min-w-0 space-y-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <h3 className="text-xl text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
                      {rule.name}
                    </h3>
                    <Badge variant="accent">{rule.tool}</Badge>
                  </div>
                  <p className="break-all text-sm text-pencil-light" style={{ fontFamily: "'Courier New', monospace" }}>
                    {rule.relativePath}
                  </p>
                  <p className="line-clamp-3 whitespace-pre-wrap text-sm text-pencil-light">
                    {rule.content}
                  </p>
                </div>

                <HandButton
                  variant="secondary"
                  size="sm"
                  className="shrink-0"
                  onClick={() => navigate(`/rules/manage/${rule.id}`)}
                >
                  <Eye size={16} strokeWidth={2.5} />
                  Edit Rule
                </HandButton>
              </div>
            </Card>
          ))}
        </div>
      ) : (
        <div className="grid gap-4">
          {filteredDiscoveredRules.map((rule) => {
            const checked = Boolean(rule.id && selectedIds.includes(rule.id));
            return (
              <Card key={getRuleRef(rule)}>
                <div className="flex items-start justify-between gap-4">
                  <div className="min-w-0 space-y-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <h3 className="text-xl text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
                        {rule.name}
                      </h3>
                      <Badge variant={rule.scope === 'project' ? 'info' : 'default'}>{rule.scope}</Badge>
                      <Badge variant="accent">{rule.sourceTool}</Badge>
                      {rule.collectible && <Badge variant="success">Collectible</Badge>}
                    </div>
                    <p className="break-all text-sm text-pencil-light" style={{ fontFamily: "'Courier New', monospace" }}>
                      {rule.path}
                    </p>
                    {rule.collectReason && <p className="text-sm text-pencil-light">{rule.collectReason}</p>}
                    <p className="line-clamp-3 whitespace-pre-wrap text-sm text-pencil-light">
                      {rule.content}
                    </p>
                  </div>

                  <div className="flex shrink-0 flex-col items-end gap-2">
                    {rule.collectible && rule.id && (
                      <SelectionToggle
                        label={`Collect ${rule.name}`}
                        checked={checked}
                        onChange={() => toggleSelected(rule)}
                      />
                    )}
                    <HandButton
                      variant="secondary"
                      size="sm"
                      onClick={() =>
                        navigate(`/rules/discovered/${encodeURIComponent(getRuleRef(rule))}`, {
                          state: { rule },
                        })
                      }
                      aria-label={`View rule ${rule.name}`}
                      className="shrink-0"
                    >
                      <Eye size={16} strokeWidth={2.5} />
                      View Rule
                    </HandButton>
                    {rule.collectible && rule.id && (
                      <HandButton
                        variant="ghost"
                        size="sm"
                        disabled={collectMutation.isPending}
                        onClick={() => collectAndEdit(rule)}
                      >
                        {collectMutation.isPending ? 'Collecting...' : 'Collect & Edit'}
                      </HandButton>
                    )}
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
