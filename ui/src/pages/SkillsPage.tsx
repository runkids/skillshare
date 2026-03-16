import { useState, useMemo, useCallback, forwardRef, memo } from 'react';
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
} from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { VirtuosoGrid, GroupedVirtuoso } from 'react-virtuoso';
import type { GridComponents } from 'react-virtuoso';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import Badge from '../components/Badge';
import { Input, Select } from '../components/Input';
import { PageSkeleton } from '../components/Skeleton';
import EmptyState from '../components/EmptyState';
import Card from '../components/Card';
import Button from '../components/Button';
import PageHeader from '../components/PageHeader';
import SegmentedControl from '../components/SegmentedControl';
import Pagination from '../components/Pagination';
import { api } from '../api/client';
import type { Skill } from '../api/client';
import { radius } from '../design';
import ScrollToTop from '../components/ScrollToTop';
import Tooltip from '../components/Tooltip';

/* -- Sticky-note pastel palette (8 colors) --------- */

const SKILL_PASTELS = [
  '#fff9c4', '#dceefb', '#fce4ec', '#e0f2e1',
  '#f3e5f5', '#fff3e0', '#e0f7fa', '#fbe9e7',
];
const SKILL_PASTELS_DARK = [
  'rgba(255,249,196,0.08)', 'rgba(220,238,251,0.08)',
  'rgba(252,228,236,0.08)', 'rgba(224,242,225,0.08)',
  'rgba(243,229,245,0.08)', 'rgba(255,243,224,0.08)',
  'rgba(224,247,250,0.08)', 'rgba(251,233,231,0.08)',
];

/** Deterministic hash → palette index. Same string always maps to same color. */
function hashToIndex(s: string, len: number): number {
  let h = 0;
  for (let i = 0; i < s.length; i++) h = ((h << 5) - h + s.charCodeAt(i)) | 0;
  return ((h % len) + len) % len;
}

/** Extract owner/repo from a GitHub URL, e.g. "https://github.com/foo/bar" → "foo/bar" */
function shortSource(source: string): string {
  const m = source.match(/github\.com\/([^/]+\/[^/]+)/);
  return m ? m[1] : source;
}

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
}) {
  // Extract repo name from relPath (e.g., "_awesome-skillshare-skills/frontend-dugong" -> "awesome-skillshare-skills")
  const repoName = skill.isInRepo && skill.relPath.startsWith('_')
    ? skill.relPath.split('/')[0].slice(1).replace(/__/g, '/')
    : undefined;

  // Color key: tracked skills from the same repo share a color
  const colorKey = repoName ?? skill.name;
  const colorIdx = hashToIndex(colorKey, SKILL_PASTELS.length);

  return (
    <Link to={`/skills/${encodeURIComponent(skill.flatName)}`} className="w-full">
      <div
        className="ss-card ss-skill-card relative p-5 pb-4 bg-surface cursor-pointer border border-muted shadow-sm rounded-[var(--radius-md)] transition-all duration-150 hover:shadow-hover hover:border-muted-dark"
        style={{
          '--skill-pastel': SKILL_PASTELS[colorIdx],
          '--skill-pastel-dark': SKILL_PASTELS_DARK[colorIdx],
        } as React.CSSProperties}
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
            <span className="text-sm text-pencil-light truncate flex-1">{shortSource(skill.source)}</span>
          ) : (
            <span />
          )}
          <div className="flex items-center gap-1.5 shrink-0">
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

  const [toolbarH, setToolbarH] = useState(0);
  const toolbarRef = useCallback((node: HTMLDivElement | null) => {
    if (node) {
      const ro = new ResizeObserver(() => setToolbarH(node.offsetHeight));
      ro.observe(node);
      return () => ro.disconnect();
    }
  }, []);
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
    <div data-tour="skills-view" className="animate-fade-in">
      {/* Header */}
      <PageHeader
        icon={<Puzzle size={24} strokeWidth={2.5} />}
        title="Skills"
        subtitle={`${skills.length} skill${skills.length !== 1 ? 's' : ''} installed`}
      />

      {/* Sticky toolbar */}
      <div ref={toolbarRef} className="sticky top-0 z-20 bg-paper -mx-4 px-4 md:-mx-8 md:px-8 pt-2 pb-4">
        {/* Search + Sort row */}
        <div className="flex flex-col sm:flex-row gap-3 mb-2">
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
              size="sm"
              options={[
                { value: 'name-asc', label: 'Name A → Z' },
                { value: 'name-desc', label: 'Name Z → A' },
                { value: 'newest', label: 'Newest first' },
                { value: 'oldest', label: 'Oldest first' },
              ]}
            />
          </div>
          {/* View toggle */}
          <SegmentedControl
            value={viewType}
            onChange={changeViewType}
            options={[
              { value: 'grid', label: <LayoutGrid size={16} strokeWidth={2.5} /> },
              { value: 'grouped', label: <FolderOpen size={16} strokeWidth={2.5} /> },
              { value: 'table', label: <List size={16} strokeWidth={2.5} /> },
            ]}
            size="md"
            connected
          />
        </div>

        {/* Filter tabs */}
        <SegmentedControl
          value={filterType}
          onChange={setFilterType}
          options={filterOptions.map((opt) => ({
            value: opt.key,
            label: <span className="inline-flex items-center gap-1.5">{opt.icon}{opt.label}</span>,
            count: filterCounts[opt.key],
          }))}
        />
      </div>

      {/* Result count — outside sticky toolbar for natural spacing */}
      {(filterType !== 'all' || search) && (
        <p className="text-pencil-light text-sm mb-3">
          Showing {filtered.length} of {skills.length} skills
          {filterType !== 'all' && (
            <>
              {' '}
              &middot;{' '}
              <Button
                variant="link"
                onClick={() => {
                  setFilterType('all');
                  setSearch('');
                }}
              >
                Clear filters
              </Button>
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
              <SkillPostit skill={filtered[index]} />
            )}
          />
        ) : viewType === 'grouped' ? (
          <GroupedView dirs={grouped.dirs} groups={grouped.groups} stickyOffset={toolbarH} />
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

      <ScrollToTop />
    </div>
  );
}

/* -- Grouped view (GroupedVirtuoso for sticky headers + virtualization) -- */

function GroupedView({ dirs, groups, stickyOffset = 0 }: { dirs: string[]; groups: Map<string, Skill[]>; stickyOffset?: number }) {
  const showHeaders = dirs.length > 1 || (dirs.length === 1 && dirs[0] !== '');

  // Chunk each group's skills into rows of 3 for GroupedVirtuoso
  const { groupCounts, rows, dirCounts } = useMemo(() => {
    const counts: number[] = [];
    const allRows: Skill[][] = [];
    const dc: number[] = [];
    for (const dir of dirs) {
      const skills = groups.get(dir) ?? [];
      dc.push(skills.length);
      let rowCount = 0;
      for (let i = 0; i < skills.length; i += 3) {
        allRows.push(skills.slice(i, i + 3));
        rowCount++;
      }
      counts.push(rowCount || 1); // at least 1 row so group header shows
      if (skills.length === 0) allRows.push([]); // empty placeholder row
    }
    return { groupCounts: counts, rows: allRows, dirCounts: dc };
  }, [dirs, groups]);

  return (
    <GroupedVirtuoso
      useWindowScroll
      groupCounts={groupCounts}
      overscan={200}
      components={{
        TopItemList: ({ style, ...props }) => (
          <div {...props} style={{ ...style, top: stickyOffset, zIndex: 10 }} />
        ),
      }}
      groupContent={(index) => {
        if (!showHeaders) return <div />;
        const dir = dirs[index];
        return (
          <div
            className="bg-paper -mx-4 px-4 md:-mx-8 md:px-8 flex items-center gap-2 py-2"
            style={{ marginTop: index === 0 ? 0 : '1.5rem' }}
          >
            <Folder size={18} strokeWidth={2.5} className="text-pencil-light" />
            <h3 className="text-lg font-bold text-pencil">
              {dir || '(root)'}
            </h3>
            <span
              className="text-sm text-pencil-light px-2 py-0.5 bg-muted"
              style={{ borderRadius: radius.sm }}
            >
              {dirCounts[index]}
            </span>
          </div>
        );
      }}
      itemContent={(index) => {
        const row = rows[index];
        if (!row || row.length === 0) return <div />;
        return (
          <div className="flex flex-wrap gap-5 mb-5">
            {row.map((skill) => (
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

function SkillsTable({ skills }: { skills: Skill[] }) {
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState<number>(10);
  const [prevSkills, setPrevSkills] = useState(skills);
  if (skills !== prevSkills) {
    setPrevSkills(skills);
    setPage(0);
  }

  const totalPages = Math.max(1, Math.ceil(skills.length / pageSize));
  const start = page * pageSize;
  const visible = skills.slice(start, start + pageSize);

  return (
    <Card>
      <div className="overflow-auto max-h-[calc(100vh-320px)]">
        <table className="w-full text-left">
          <thead className="sticky top-0 z-10 bg-surface">
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
                  <Tooltip content={skill.relPath}><span className="block truncate">{skill.relPath}</span></Tooltip>
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
                  <Tooltip content={skill.source ?? '—'}><span className="block truncate">{skill.source ? shortSource(skill.source) : '—'}</span></Tooltip>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {skills.length > TABLE_PAGE_SIZES[0] && (
        <Pagination
          page={page}
          totalPages={totalPages}
          onPageChange={(p) => setPage(p)}
          rangeText={`${start + 1}–${Math.min(start + pageSize, skills.length)} of ${skills.length}`}
          pageSize={{
            value: pageSize,
            options: TABLE_PAGE_SIZES,
            onChange: (s) => { setPageSize(s); setPage(0); },
          }}
        />
      )}
    </Card>
  );
}
