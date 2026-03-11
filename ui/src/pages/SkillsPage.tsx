import { useState, useMemo, forwardRef, memo, useEffect } from 'react';
import { createPortal } from 'react-dom';
import { Link } from 'react-router-dom';
import {
  Search,
  GitBranch,
  Folder,
  Puzzle,
  ArrowUpDown,
  Users,
  Globe,
  FolderOpen,
  LayoutGrid,
  List,
  Target,
  ChevronLeft,
  ChevronRight,
} from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { Virtuoso, VirtuosoGrid } from 'react-virtuoso';
import type { GridComponents } from 'react-virtuoso';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import Badge from '../components/Badge';
import { Input, Select } from '../components/Input';
import { PageSkeleton } from '../components/Skeleton';
import EmptyState from '../components/EmptyState';
import Card from '../components/Card';
import { api, type Skill } from '../api/client';
import { radius, shadows } from '../design';

/* -- Filter, Sort & View types -------------------- */

type FilterType = 'all' | 'tracked' | 'github' | 'local';
type SortType = 'name-asc' | 'name-desc' | 'newest' | 'oldest';
type ViewType = 'grid' | 'grouped' | 'table';

const filterOptions: { key: FilterType; label: string; icon: React.ReactNode }[] = [
  { key: 'all', label: 'All', icon: <LayoutGrid size={14} strokeWidth={2.5} /> },
  { key: 'tracked', label: 'Tracked', icon: <Users size={14} strokeWidth={2.5} /> },
  { key: 'github', label: 'GitHub', icon: <Globe size={14} strokeWidth={2.5} /> },
  { key: 'local', label: 'Local', icon: <FolderOpen size={14} strokeWidth={2.5} /> },
];

function matchFilter(skill: Skill, filterType: FilterType): boolean {
  switch (filterType) {
    case 'all':
      return true;
    case 'tracked':
      return skill.isInRepo;
    case 'github':
      return (skill.type === 'github' || skill.type === 'github-subdir') && !skill.isInRepo;
    case 'local':
      return !skill.type && !skill.isInRepo;
  }
}

function getTypeLabel(type?: string): string | undefined {
  if (!type) return undefined;
  if (type === 'github-subdir') return 'github';
  return type;
}

function sortSkills(skills: Skill[], sortType: SortType): Skill[] {
  const sorted = [...skills];
  switch (sortType) {
    case 'name-asc':
      return sorted.sort((a, b) => a.name.localeCompare(b.name));
    case 'name-desc':
      return sorted.sort((a, b) => b.name.localeCompare(a.name));
    case 'newest':
      return sorted.sort((a, b) => {
        if (!a.installedAt && !b.installedAt) return a.name.localeCompare(b.name);
        if (!a.installedAt) return 1;
        if (!b.installedAt) return -1;
        return new Date(b.installedAt).getTime() - new Date(a.installedAt).getTime();
      });
    case 'oldest':
      return sorted.sort((a, b) => {
        if (!a.installedAt && !b.installedAt) return a.name.localeCompare(b.name);
        if (!a.installedAt) return 1;
        if (!b.installedAt) return -1;
        return new Date(a.installedAt).getTime() - new Date(b.installedAt).getTime();
      });
  }
}

/* -- Filter chip component ------------------------ */

function FilterChip({
  label,
  icon,
  active,
  count,
  onClick,
}: {
  label: string;
  icon: React.ReactNode;
  active: boolean;
  count: number;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      className={`
        inline-flex items-center gap-1.5 px-3 py-1.5 border-2 text-sm
        transition-all duration-150 cursor-pointer select-none
        ${
          active
            ? 'bg-pencil text-paper border-pencil'
            : 'bg-surface text-pencil-light border-muted hover:border-pencil hover:text-pencil'
        }
      `}
      style={{
        borderRadius: radius.full,
        boxShadow: active ? shadows.hover : 'none',
      }}
    >
      {icon}
      <span>{label}</span>
      <span
        className={`
          text-xs px-1.5 py-0.5 rounded-full min-w-[20px] text-center
          ${active ? 'bg-paper/20 text-paper' : 'bg-muted text-pencil-light'}
        `}
      >
        {count}
      </span>
    </button>
  );
}

/* -- VirtuosoGrid components (OUTSIDE component function) -- */

const GridList = forwardRef<HTMLDivElement, React.ComponentPropsWithRef<'div'>>(
  ({ style, children, ...props }, ref) => (
    <div
      ref={ref}
      {...props}
      style={{ display: 'flex', flexWrap: 'wrap', gap: '1.25rem', ...style }}
    >
      {children}
    </div>
  ),
);
GridList.displayName = 'GridList';

const GridItem = ({ children, ...props }: React.ComponentPropsWithRef<'div'>) => (
  <div
    {...props}
    className="!w-full md:!w-[calc(50%-0.625rem)] xl:!w-[calc(33.333%-0.834rem)]"
    style={{ display: 'flex', flex: 'none', boxSizing: 'border-box' }}
  >
    {children}
  </div>
);

const GridPlaceholder = () => (
  <div
    className="!w-full md:!w-[calc(50%-0.625rem)] xl:!w-[calc(33.333%-0.834rem)]"
    style={{ display: 'flex', flex: 'none', boxSizing: 'border-box' }}
  >
    <div className="w-full h-32 bg-muted animate-pulse" style={{ borderRadius: radius.md }} />
  </div>
);

const gridComponents: GridComponents = {
  List: GridList as GridComponents['List'],
  Item: GridItem as GridComponents['Item'],
  ScrollSeekPlaceholder: GridPlaceholder as GridComponents['ScrollSeekPlaceholder'],
};

/* -- Skill card ----------------------------------- */

const SkillPostit = memo(function SkillPostit({
  skill,
}: {
  skill: Skill;
  index?: number;
}) {
  // Extract repo name from relPath (e.g., "_awesome-skillshare-skills/frontend-dugong" -> "awesome-skillshare-skills")
  const repoName = skill.isInRepo && skill.relPath.startsWith('_')
    ? skill.relPath.split('/')[0].slice(1).replace(/__/g, '/')
    : undefined;

  return (
    <Link to={`/skills/${encodeURIComponent(skill.flatName)}`} className="w-full">
      <div
        className="relative p-5 pb-4 border-2 border-muted bg-surface cursor-pointer transition-all duration-150 hover:border-pencil-light hover:shadow-md"
        style={{
          borderRadius: radius.md,
          boxShadow: shadows.sm,
          ...(skill.isInRepo ? { borderLeftWidth: '3px', borderLeftColor: 'var(--color-pencil-light)' } : {}),
        }}
      >
        {/* Skill name row */}
        <div className="flex items-center gap-2 mb-2">
          <div className="shrink-0">
            {skill.isInRepo
              ? <GitBranch size={18} strokeWidth={2.5} className="text-pencil-light" />
              : <Folder size={18} strokeWidth={2.5} className="text-pencil-light" />
            }
          </div>
          <h3 className="font-bold text-pencil text-lg truncate leading-tight">
            {skill.name}
          </h3>
        </div>

        {/* Org banner (tracked only) */}
        {skill.isInRepo && repoName && (
          <div className="flex items-center gap-1 mb-2">
            <Users size={12} strokeWidth={2.5} className="text-pencil-light shrink-0" />
            <span className="text-xs text-pencil-light truncate">{repoName}</span>
          </div>
        )}

        {/* Path */}
        <p
          className="font-mono text-sm text-pencil-light truncate mb-2"
        >
          {skill.relPath}
        </p>

        {/* Bottom row */}
        <div className="flex items-center justify-between gap-2 mt-auto">
          {skill.source ? (
            <span className="text-sm text-pencil-light truncate flex-1">{skill.source}</span>
          ) : (
            <span />
          )}
          <div className="flex items-center gap-1.5 shrink-0">
            {skill.targets && skill.targets.length > 0 && (
              <span
                className="inline-flex items-center gap-0.5"
                title={`Targets: ${skill.targets.join(', ')}`}
              >
                <Target size={13} strokeWidth={2.5} className="text-pencil-light" />
                <span className="text-xs text-pencil-light">{skill.targets.length}</span>
              </span>
            )}
            {skill.isInRepo && <Badge variant="default">tracked</Badge>}
            {!skill.isInRepo && getTypeLabel(skill.type) && <Badge variant="info">{getTypeLabel(skill.type)}</Badge>}
          </div>
        </div>
      </div>
    </Link>
  );
});

/* -- Main page ------------------------------------ */

export default function SkillsPage() {
  const { data, isPending, error } = useQuery({
    queryKey: queryKeys.skills.all,
    queryFn: () => api.listSkills(),
    staleTime: staleTimes.skills,
  });
  const [search, setSearch] = useState('');
  const [filterType, setFilterType] = useState<FilterType>('all');
  const [sortType, setSortType] = useState<SortType>('name-asc');
  const [viewType, setViewType] = useState<ViewType>(() => {
    const saved = localStorage.getItem('skillshare:skills-view');
    return (saved === 'grid' || saved === 'grouped' || saved === 'table') ? saved : 'grid';
  });

  const changeViewType = (v: ViewType) => {
    setViewType(v);
    localStorage.setItem('skillshare:skills-view', v);
  };

  const skills = data?.skills ?? [];

  // Compute counts for each filter type (before text search, so chips always show totals)
  const filterCounts = useMemo(() => {
    const counts: Record<FilterType, number> = {
      all: skills.length,
      tracked: 0,
      github: 0,
      local: 0,
    };
    for (const s of skills) {
      if (s.isInRepo) counts.tracked++;
      if ((s.type === 'github' || s.type === 'github-subdir') && !s.isInRepo) counts.github++;
      if (!s.type && !s.isInRepo) counts.local++;
    }
    return counts;
  }, [skills]);

  // Apply text filter -> type filter -> sort
  const filtered = useMemo(() => {
    const q = search.toLowerCase();
    const result = skills.filter(
      (s) =>
        (s.name.toLowerCase().includes(q) ||
          s.flatName.toLowerCase().includes(q) ||
          (s.source ?? '').toLowerCase().includes(q)) &&
        matchFilter(s, filterType),
    );
    return sortSkills(result, sortType);
  }, [skills, search, filterType, sortType]);

  // Group skills by parent directory for grouped view
  const grouped = useMemo(() => {
    const groups = new Map<string, Skill[]>();
    for (const skill of filtered) {
      const rp = skill.relPath ?? '';
      const lastSlash = rp.lastIndexOf('/');
      const dir = lastSlash > 0 ? rp.substring(0, lastSlash) : '';
      const existing = groups.get(dir) ?? [];
      existing.push(skill);
      groups.set(dir, existing);
    }
    // Sort directory keys: non-empty alphabetically first, then top-level ""
    const sortedDirs = [...groups.keys()].filter((k) => k !== '').sort();
    if (groups.has('')) sortedDirs.push('');
    return { dirs: sortedDirs, groups };
  }, [filtered]);

  if (isPending) return <PageSkeleton />;
  if (error) {
    return (
      <Card variant="accent" className="text-center py-8">
        <p className="text-danger text-lg">
          Failed to load skills
        </p>
        <p className="text-pencil-light text-base mt-1">{error.message}</p>
      </Card>
    );
  }

  return (
    <div className="animate-fade-in">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2
            className="text-3xl md:text-4xl font-bold text-pencil mb-1"
          >
            Skills
          </h2>
          <p className="text-pencil-light">
            {skills.length} skill{skills.length !== 1 ? 's' : ''} installed
          </p>
        </div>
      </div>

      {/* Search + Sort row */}
      <div className="flex flex-col sm:flex-row gap-3 mb-4">
        <div className="relative flex-1">
          <Search
            size={18}
            strokeWidth={2.5}
            className="absolute left-4 top-1/2 -translate-y-1/2 text-muted-dark pointer-events-none"
          />
          <Input
            type="text"
            placeholder="Filter skills..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="!pl-11"
          />
        </div>
        <div className="flex items-center gap-2 sm:w-52">
          <ArrowUpDown size={16} strokeWidth={2.5} className="text-pencil-light shrink-0" />
          <Select
            value={sortType}
            onChange={(v) => setSortType(v as SortType)}
            options={[
              { value: 'name-asc', label: 'Name A → Z' },
              { value: 'name-desc', label: 'Name Z → A' },
              { value: 'newest', label: 'Newest first' },
              { value: 'oldest', label: 'Oldest first' },
            ]}
          />
        </div>
        {/* View toggle */}
        <div className="flex items-center border-2 border-muted overflow-hidden" style={{ borderRadius: radius.sm }}>
          <button
            onClick={() => changeViewType('grid')}
            className={`px-4 py-2 transition-colors cursor-pointer ${viewType === 'grid' ? 'bg-pencil text-paper' : 'bg-surface text-pencil-light hover:text-pencil'}`}
            title="Grid view"
          >
            <LayoutGrid size={16} strokeWidth={2.5} />
          </button>
          <button
            onClick={() => changeViewType('grouped')}
            className={`px-4 py-2 transition-colors cursor-pointer ${viewType === 'grouped' ? 'bg-pencil text-paper' : 'bg-surface text-pencil-light hover:text-pencil'}`}
            title="Grouped view"
          >
            <FolderOpen size={16} strokeWidth={2.5} />
          </button>
          <button
            onClick={() => changeViewType('table')}
            className={`px-4 py-2 transition-colors cursor-pointer ${viewType === 'table' ? 'bg-pencil text-paper' : 'bg-surface text-pencil-light hover:text-pencil'}`}
            title="Table view"
          >
            <List size={16} strokeWidth={2.5} />
          </button>
        </div>
      </div>

      {/* Filter chips */}
      <div className="flex flex-wrap gap-2 mb-6">
        {filterOptions.map((opt) => (
          <FilterChip
            key={opt.key}
            label={opt.label}
            icon={opt.icon}
            active={filterType === opt.key}
            count={filterCounts[opt.key]}
            onClick={() => setFilterType(filterType === opt.key ? 'all' : opt.key)}
          />
        ))}
      </div>

      {/* Result count when filtered */}
      {(filterType !== 'all' || search) && (
        <p className="text-pencil-light text-sm mb-4">
          Showing {filtered.length} of {skills.length} skills
          {filterType !== 'all' && (
            <>
              {' '}
              &middot;{' '}
              <button
                className="link-subtle cursor-pointer"
                onClick={() => {
                  setFilterType('all');
                  setSearch('');
                }}
              >
                Clear filters
              </button>
            </>
          )}
        </p>
      )}

      {/* Skills grid / grouped / table view */}
      {filtered.length > 0 ? (
        viewType === 'grid' ? (
          <VirtuosoGrid
            useWindowScroll
            totalCount={filtered.length}
            overscan={200}
            components={gridComponents}
            scrollSeekConfiguration={{
              enter: (velocity) => Math.abs(velocity) > 800,
              exit: (velocity) => Math.abs(velocity) < 200,
            }}
            itemContent={(index) => (
              <SkillPostit skill={filtered[index]} index={index} />
            )}
          />
        ) : viewType === 'grouped' ? (
          <VirtualizedGroupedView dirs={grouped.dirs} groups={grouped.groups} />
        ) : (
          <SkillsTable skills={filtered} />
        )
      ) : (
        <EmptyState
          icon={Puzzle}
          title={search || filterType !== 'all' ? 'No matches' : 'No skills yet'}
          description={
            search || filterType !== 'all'
              ? 'Try a different search term or filter.'
              : 'Install skills from GitHub or add them to your source directory.'
          }
        />
      )}
    </div>
  );
}

/* -- Virtualized grouped view --------------------- */

type GroupRow =
  | { type: 'header'; dir: string; count: number; showHeader: boolean }
  | { type: 'cards'; skills: Skill[] };

function VirtualizedGroupedView({ dirs, groups }: { dirs: string[]; groups: Map<string, Skill[]> }) {
  const rows = useMemo(() => {
    const result: GroupRow[] = [];
    const showHeaders = dirs.length > 1 || (dirs.length === 1 && dirs[0] !== '');
    for (const dir of dirs) {
      const skills = groups.get(dir) ?? [];
      result.push({ type: 'header', dir, count: skills.length, showHeader: showHeaders });
      // Chunk skills into rows of 3
      for (let i = 0; i < skills.length; i += 3) {
        result.push({ type: 'cards', skills: skills.slice(i, i + 3) });
      }
    }
    return result;
  }, [dirs, groups]);

  return (
    <Virtuoso
      useWindowScroll
      totalCount={rows.length}
      overscan={200}
      itemContent={(index) => {
        const row = rows[index];
        if (row.type === 'header') {
          if (!row.showHeader) return <div />;
          return (
            <div className="flex items-center gap-2 mb-4 mt-8 first:mt-0" style={index === 0 ? { marginTop: 0 } : undefined}>
              <Folder size={18} strokeWidth={2.5} className="text-pencil-light" />
              <h3 className="text-lg font-bold text-pencil">
                {row.dir || '(root)'}
              </h3>
              <span
                className="text-sm text-pencil-light px-2 py-0.5 bg-muted"
                style={{ borderRadius: radius.sm }}
              >
                {row.count}
              </span>
            </div>
          );
        }
        return (
          <div className="flex flex-wrap gap-5 mb-5">
            {row.skills.map((skill) => (
              <div
                key={skill.flatName}
                className="!w-full md:!w-[calc(50%-0.625rem)] xl:!w-[calc(33.333%-0.834rem)]"
                style={{ display: 'flex', flex: 'none', boxSizing: 'border-box' }}
              >
                <SkillPostit skill={skill} />
              </div>
            ))}
          </div>
        );
      }}
    />
  );
}

/* -- Table view with pagination ------------------- */

const TABLE_PAGE_SIZES = [10, 25, 50] as const;

function TablePillButton({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      className={`
        px-3 py-1.5 text-sm border-2 transition-all duration-150 cursor-pointer
        ${active
          ? 'bg-pencil text-paper border-pencil font-medium'
          : 'bg-transparent text-pencil border-muted-dark hover:border-pencil'
        }
      `}
      style={{
        borderRadius: radius.sm,
        boxShadow: active ? shadows.sm : 'none',
      }}
    >
      {children}
    </button>
  );
}

function TruncateWithTip({ text }: { text: string }) {
  const [tip, setTip] = useState<{ x: number; y: number } | null>(null);

  return (
    <>
      <span
        className="block truncate"
        onMouseEnter={(e) => {
          const rect = e.currentTarget.getBoundingClientRect();
          setTip({ x: rect.left, y: rect.bottom + 4 });
        }}
        onMouseLeave={() => setTip(null)}
      >
        {text}
      </span>
      {tip && createPortal(
        <div
          className="fixed z-[9999] max-w-sm break-all whitespace-normal bg-pencil px-2.5 py-1.5 text-xs text-paper shadow-lg pointer-events-none"
          style={{ left: tip.x, top: tip.y, borderRadius: radius.sm }}
        >
          {text}
        </div>,
        document.body,
      )}
    </>
  );
}

function SkillsTable({ skills }: { skills: Skill[] }) {
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState<number>(10);

  useEffect(() => { setPage(0); }, [skills]);

  const totalPages = Math.max(1, Math.ceil(skills.length / pageSize));
  const start = page * pageSize;
  const visible = skills.slice(start, start + pageSize);

  return (
    <Card>
      <div className="overflow-x-auto">
        <table className="w-full text-left">
          <thead>
            <tr className="border-b-2 border-dashed border-muted-dark">
              <th className="pb-3 pr-4 text-pencil-light text-sm font-medium w-0" />
              <th className="pb-3 pr-4 text-pencil-light text-sm font-medium">Name</th>
              <th className="pb-3 pr-4 text-pencil-light text-sm font-medium">Path</th>
              <th className="pb-3 pr-4 text-pencil-light text-sm font-medium">Type</th>
              <th className="pb-3 text-pencil-light text-sm font-medium">Source</th>
            </tr>
          </thead>
          <tbody>
            {visible.map((skill) => (
              <tr
                key={skill.flatName}
                className="border-b border-dashed border-muted hover:bg-paper-warm/60 transition-colors"
              >
                {/* Status stripe */}
                <td className="py-3 pr-0 w-1">
                  <div
                    className="w-1 h-6 rounded-full"
                    style={{
                      backgroundColor: skill.isInRepo
                        ? 'var(--color-pencil-light)'
                        : 'var(--color-muted)',
                    }}
                    title={skill.isInRepo ? 'tracked' : 'local'}
                  />
                </td>
                {/* Name */}
                <td className="py-3 pr-4">
                  <Link
                    to={`/skills/${encodeURIComponent(skill.flatName)}`}
                    className="font-medium text-pencil hover:underline"
                  >
                    {skill.name}
                  </Link>
                </td>
                {/* Path */}
                <td className="py-3 pr-4 font-mono text-sm text-pencil-light max-w-[200px]">
                  <TruncateWithTip text={skill.relPath} />
                </td>
                {/* Type badge */}
                <td className="py-3 pr-4">
                  {skill.isInRepo ? (
                    <Badge variant="default">tracked</Badge>
                  ) : getTypeLabel(skill.type) ? (
                    <Badge variant="info">{getTypeLabel(skill.type)}</Badge>
                  ) : (
                    <Badge variant="default">local</Badge>
                  )}
                </td>
                {/* Source */}
                <td className="py-3 text-sm text-pencil-light max-w-[280px]">
                  <TruncateWithTip text={skill.source ?? '—'} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {skills.length > TABLE_PAGE_SIZES[0] && (
        <div className="flex items-center justify-between pt-4 mt-4 border-t-2 border-dashed border-muted">
          <div className="flex items-center gap-2 text-sm text-pencil-light">
            <span>Show</span>
            {TABLE_PAGE_SIZES.map((size) => (
              <TablePillButton
                key={size}
                active={pageSize === size}
                onClick={() => { setPageSize(size); setPage(0); }}
              >
                {size}
              </TablePillButton>
            ))}
            <span className="ml-1">
              {start + 1}–{Math.min(start + pageSize, skills.length)} of {skills.length}
            </span>
          </div>

          <div className="flex items-center gap-1">
            <button
              onClick={() => setPage((p) => Math.max(0, p - 1))}
              disabled={page === 0}
              className={`p-1.5 border-2 transition-all duration-150 cursor-pointer ${
                page === 0
                  ? 'border-transparent text-muted-dark cursor-not-allowed'
                  : 'border-transparent text-pencil hover:bg-paper-warm hover:border-muted-dark'
              }`}
              style={{ borderRadius: radius.sm }}
            >
              <ChevronLeft size={20} />
            </button>
            <span className="text-sm text-pencil px-2">
              {page + 1} / {totalPages}
            </span>
            <button
              onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
              disabled={page >= totalPages - 1}
              className={`p-1.5 border-2 transition-all duration-150 cursor-pointer ${
                page >= totalPages - 1
                  ? 'border-transparent text-muted-dark cursor-not-allowed'
                  : 'border-transparent text-pencil hover:bg-paper-warm hover:border-muted-dark'
              }`}
              style={{ borderRadius: radius.sm }}
            >
              <ChevronRight size={20} />
            </button>
          </div>
        </div>
      )}
    </Card>
  );
}
