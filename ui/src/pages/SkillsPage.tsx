import { useState, useMemo, useCallback, useEffect, forwardRef, memo, type ReactElement } from 'react';
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
  Plus,
  ChevronRight,
  ChevronDown,
  ChevronsUpDown,
  ChevronsDownUp,
} from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { VirtuosoGrid, Virtuoso } from 'react-virtuoso';
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
import { parseRemoteURL } from '../lib/parseRemoteURL';

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

/** Extract owner/repo from a git remote URL, fallback to raw string. */
function shortSource(source: string): string {
  return parseRemoteURL(source)?.ownerRepo ?? source;
}

/* -- Folder tree types & helpers -------------------- */

interface FolderNode {
  name: string;
  path: string;
  children: Map<string, FolderNode>;
  skills: Skill[];
  skillCount: number;
}

interface TreeNode {
  type: 'folder' | 'skill';
  name: string;
  path: string;
  depth: number;
  skill?: Skill;
  childCount: number;
  isRoot?: boolean;
}

/** Build a nested folder tree from skills' relPath values. Computes skillCount bottom-up. */
function buildTree(skills: Skill[]): FolderNode {
  const root: FolderNode = { name: '', path: '', children: new Map(), skills: [], skillCount: 0 };
  for (const skill of skills) {
    const rp = skill.relPath ?? '';
    const lastSlash = rp.lastIndexOf('/');
    if (lastSlash <= 0) {
      root.skills.push(skill);
      continue;
    }
    const dirPath = rp.substring(0, lastSlash);
    const segments = dirPath.split('/');
    let node = root;
    let currentPath = '';
    for (const seg of segments) {
      currentPath = currentPath ? `${currentPath}/${seg}` : seg;
      if (!node.children.has(seg)) {
        node.children.set(seg, { name: seg, path: currentPath, children: new Map(), skills: [], skillCount: 0 });
      }
      node = node.children.get(seg)!;
    }
    node.skills.push(skill);
  }
  // Compute skillCount bottom-up (O(n) total)
  function computeCounts(node: FolderNode): number {
    let count = node.skills.length;
    for (const child of node.children.values()) count += computeCounts(child);
    node.skillCount = count;
    return count;
  }
  computeCounts(root);
  return root;
}

/** Flatten the tree into a list of TreeNode for virtualized rendering. */
function flattenTree(
  root: FolderNode,
  collapsed: ReadonlySet<string>,
  isSearching: boolean,
): TreeNode[] {
  const result: TreeNode[] = [];

  function walkFolder(node: FolderNode, depth: number) {
    // Sort child folders alphabetically
    const sortedChildren = [...node.children.entries()].sort((a, b) => a[0].localeCompare(b[0]));
    for (const [, child] of sortedChildren) {
      const cc = child.skillCount;
      if (cc === 0) continue; // skip empty folders (filtered out)
      result.push({
        type: 'folder',
        name: child.name,
        path: child.path,
        depth,
        childCount: cc,
      });
      const isCollapsed = !isSearching && collapsed.has(child.path);
      if (!isCollapsed) {
        walkFolder(child, depth + 1);
      }
    }
    // Skills directly in this folder
    for (const skill of node.skills) {
      result.push({
        type: 'skill',
        name: skill.name,
        path: skill.relPath,
        depth,
        skill,
        childCount: 0,
      });
    }
  }

  // Walk top-level children
  const sortedChildren = [...root.children.entries()].sort((a, b) => a[0].localeCompare(b[0]));
  for (const [, child] of sortedChildren) {
    const cc = child.skillCount;
    if (cc === 0) continue;
    result.push({
      type: 'folder',
      name: child.name,
      path: child.path,
      depth: 0,
      childCount: cc,
    });
    const isCollapsed = !isSearching && collapsed.has(child.path);
    if (!isCollapsed) {
      walkFolder(child, 1);
    }
  }

  // Root-level skills last, under a virtual "(root)" folder
  if (root.skills.length > 0) {
    result.push({
      type: 'folder',
      name: '(root)',
      path: '',
      depth: 0,
      childCount: root.skills.length,
      isRoot: true,
    });
    const rootCollapsed = !isSearching && collapsed.has('');
    if (!rootCollapsed) {
      for (const skill of root.skills) {
        result.push({
          type: 'skill',
          name: skill.name,
          path: skill.relPath,
          depth: 1,
          skill,
          childCount: 0,
        });
      }
    }
  }

  return result;
}

/** Collect all folder paths from a tree (for Expand/Collapse All). */
function collectAllFolderPaths(root: FolderNode): string[] {
  const paths: string[] = [];
  function walk(node: FolderNode) {
    for (const child of node.children.values()) {
      paths.push(child.path);
      walk(child);
    }
  }
  walk(root);
  if (root.skills.length > 0) paths.push(''); // root virtual folder
  return paths;
}

const COLLAPSED_STORAGE_KEY = 'skillshare:folder-collapsed';

function loadCollapsed(): Set<string> {
  try {
    const raw = localStorage.getItem(COLLAPSED_STORAGE_KEY);
    if (raw) return new Set(JSON.parse(raw));
  } catch { /* ignore corrupt data */ }
  return new Set();
}

function saveCollapsed(collapsed: Set<string>) {
  localStorage.setItem(COLLAPSED_STORAGE_KEY, JSON.stringify([...collapsed]));
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
    <Link to={`/skills/${encodeURIComponent(skill.flatName)}`} className={`w-full h-full${skill.disabled ? ' opacity-50' : ''}`}>
      <div
        className="ss-card ss-skill-card relative p-5 pb-4 bg-surface cursor-pointer border border-muted shadow-sm rounded-[var(--radius-md)] transition-all duration-150 hover:shadow-hover hover:border-muted-dark h-full flex flex-col"
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
            {skill.disabled && <Badge variant="danger">disabled</Badge>}
            {skill.isInRepo && <Badge variant="default">tracked</Badge>}
            {!skill.isInRepo && getTypeLabel(skill.type) && <Badge variant="info">{getTypeLabel(skill.type)}</Badge>}
            {skill.branch && (
              <Badge variant="default">
                <GitBranch size={10} strokeWidth={2.5} className="inline -mt-px mr-0.5" />
                {skill.branch}
              </Badge>
            )}
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
    if (!node) return;
    const ro = new ResizeObserver(() => setToolbarH(node.offsetHeight));
    ro.observe(node);
    return () => ro.disconnect();
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
        className="mb-1!"
        actions={
          <Link to="/skills/new">
            <Button variant="primary" size="sm">
              <Plus size={16} strokeWidth={2.5} />
              New Skill
            </Button>
          </Link>
        }
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

      {/* Result count — hidden in folder view (merged into folder toolbar) */}
      {(filterType !== 'all' || search) && viewType !== 'grouped' && (
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
          <FolderTreeView
            skills={filtered}
            totalCount={skills.length}
            isSearching={!!search || filterType !== 'all'}
            stickyTop={toolbarH}
            onClearFilters={(filterType !== 'all' || search) ? () => { setFilterType('all'); setSearch(''); } : undefined}
          />
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

/* -- Folder tree view (virtualized flat list) -------- */

const INDENT_PX = 24;


function FolderTreeView({ skills, totalCount, isSearching, stickyTop = 0, onClearFilters }: {
  skills: Skill[];
  totalCount: number;
  isSearching: boolean;
  stickyTop?: number;
  onClearFilters?: () => void;
}) {
  const [collapsed, setCollapsed] = useState<Set<string>>(loadCollapsed);
  const [stickyFolder, setStickyFolder] = useState<{ node: TreeNode; index: number } | null>(null);

  const tree = useMemo(() => buildTree(skills), [skills]);

  const rows = useMemo(
    () => flattenTree(tree, collapsed, isSearching),
    [tree, collapsed, isSearching],
  );

  const folderCount = useMemo(() => {
    let count = 0;
    for (const r of rows) if (r.type === 'folder') count++;
    return count;
  }, [rows]);

  // Track scroll to find which folder should be sticky.
  // Uses DOM positions to find the row index at the toolbar edge,
  // then walks backwards in the rows DATA array (not DOM) to find
  // the nearest folder — works even if Virtuoso unmounted that folder row.
  useEffect(() => {
    let ticking = false;
    const onScroll = () => {
      if (ticking) return;
      ticking = true;
      requestAnimationFrame(() => {
        ticking = false;
        const allEls = document.querySelectorAll<HTMLElement>('[data-tree-idx]');
        if (allEls.length === 0) { setStickyFolder(null); return; }

        // Find the index of the first row at or below the toolbar bottom
        let edgeIdx = -1;
        for (const el of allEls) {
          if (el.getBoundingClientRect().top >= stickyTop) {
            edgeIdx = parseInt(el.dataset.treeIdx!, 10);
            break;
          }
        }
        // All rendered rows are above toolbar — use the last one's index + 1
        if (edgeIdx < 0) {
          const lastEl = allEls[allEls.length - 1];
          edgeIdx = parseInt(lastEl.dataset.treeIdx!, 10) + 1;
        }
        if (edgeIdx <= 0) { setStickyFolder(null); return; }

        // Walk backwards in rows DATA to find nearest folder above the edge
        for (let i = edgeIdx - 1; i >= 0; i--) {
          if (rows[i]?.type === 'folder') {
            setStickyFolder({ node: rows[i], index: i });
            return;
          }
        }
        setStickyFolder(null);
      });
    };
    window.addEventListener('scroll', onScroll, { passive: true });
    onScroll();
    return () => window.removeEventListener('scroll', onScroll);
  }, [rows, stickyTop]);

  const toggleFolder = useCallback((path: string) => {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      saveCollapsed(next);
      return next;
    });
  }, []);

  const expandAll = useCallback(() => {
    setCollapsed(new Set());
    saveCollapsed(new Set());
  }, []);

  const collapseAll = useCallback(() => {
    const all = new Set(collectAllFolderPaths(tree));
    setCollapsed(all);
    saveCollapsed(all);
  }, [tree]);

  const renderItem = useCallback((index: number): ReactElement => {
    const node = rows[index];
    const indentGuides = node.depth > 0 ? (
      Array.from({ length: node.depth }, (_, i) => (
        <span
          key={i}
          className="absolute top-0 bottom-0 border-l border-muted/40"
          style={{ left: i * INDENT_PX + 14 }}
        />
      ))
    ) : null;

    if (node.type === 'folder') {
      const isFolderCollapsed = !isSearching && collapsed.has(node.path);
      return (
        <div
          data-tree-idx={index}
          className={`relative flex items-center gap-1.5 py-1.5 px-1 cursor-pointer select-none hover:bg-muted/50 transition-colors${node.isRoot ? ' border-t border-muted/60 mt-2 pt-3' : ''}`}
          style={{ paddingLeft: node.depth * INDENT_PX + 4 }}
          onClick={() => toggleFolder(node.path)}
          role="treeitem"
          aria-expanded={!isFolderCollapsed}
        >
          {indentGuides}
          {isFolderCollapsed
            ? <ChevronRight size={14} strokeWidth={2.5} className="text-pencil-light shrink-0" />
            : <ChevronDown size={14} strokeWidth={2.5} className="text-pencil-light shrink-0" />
          }
          {isFolderCollapsed
            ? <Folder size={16} strokeWidth={2.5} className="text-pencil shrink-0" />
            : <FolderOpen size={16} strokeWidth={2.5} className="text-pencil shrink-0" />
          }
          <span className={`font-bold text-pencil shrink-0${node.isRoot ? ' text-pencil-light font-semibold' : ''}`}>
            {node.name}
          </span>
          <span
            className="text-[11px] text-pencil-light px-1.5 py-0 bg-muted shrink-0 ml-1.5"
            style={{ borderRadius: radius.sm }}
          >
            {node.childCount}
          </span>
        </div>
      );
    }

    const skill = node.skill!;
    const tooltipContent = (
      <div>
        <div>{skill.relPath}</div>
        {(skill.source || skill.installedAt) && (
          <>
            <hr className="border-paper/30 my-1" />
            {skill.source && <div>Source: {shortSource(skill.source)}</div>}
            {skill.installedAt && <div>Installed: {new Date(skill.installedAt).toLocaleDateString()}</div>}
          </>
        )}
      </div>
    );

    return (
      <div data-tree-idx={index}>
        <Tooltip content={tooltipContent} followCursor delay={1500}>
          <Link
            to={`/skills/${encodeURIComponent(skill.flatName)}`}
            className={`relative flex items-center gap-1.5 py-1 px-1 hover:bg-muted/50 transition-colors no-underline${skill.disabled ? ' opacity-40' : ''}`}
            style={{ paddingLeft: node.depth * INDENT_PX + 4 }}
          >
            {indentGuides}
            <span style={{ width: 14 }} className="shrink-0" />
            <Puzzle size={14} strokeWidth={2} className="text-pencil-light/60 shrink-0" />
            <span className="text-sm text-pencil truncate">{skill.name}</span>
            <span className="ml-auto shrink-0 flex items-center gap-1">
              {skill.disabled && <Badge variant="danger">disabled</Badge>}
              {skill.isInRepo
                ? <Badge variant="default">tracked</Badge>
                : getTypeLabel(skill.type)
                  ? <Badge variant="info">{getTypeLabel(skill.type)}</Badge>
                  : <Badge variant="default">local</Badge>
              }
              {skill.branch && (
                <Badge variant="default">
                  <GitBranch size={10} strokeWidth={2.5} className="inline -mt-px mr-0.5" />
                  {skill.branch}
                </Badge>
              )}
            </span>
          </Link>
        </Tooltip>
      </div>
    );
  }, [rows, collapsed, isSearching, toggleFolder]);

  return (
    <div>
      {/* Toolbar: stats + Expand/Collapse All */}
      <div className="flex items-center gap-2 mb-3 flex-wrap">
        <span className="text-sm text-pencil-light">
          {isSearching ? (
            <>
              Showing {skills.length} of {totalCount} skills
              {onClearFilters && (
                <>
                  {' '}&middot;{' '}
                  <Button variant="link" onClick={onClearFilters}>Clear filters</Button>
                </>
              )}
            </>
          ) : (
            <>{skills.length} skill{skills.length !== 1 ? 's' : ''} in {folderCount} folder{folderCount !== 1 ? 's' : ''}</>
          )}
        </span>
        {folderCount > 1 && (
          <span className="ml-auto flex items-center gap-2">
            <Button variant="ghost" size="sm" onClick={expandAll}>
              <ChevronsUpDown size={14} strokeWidth={2.5} /> Expand All
            </Button>
            <Button variant="ghost" size="sm" onClick={collapseAll}>
              <ChevronsDownUp size={14} strokeWidth={2.5} /> Collapse All
            </Button>
          </span>
        )}
      </div>

      {/* Sticky folder header — appears when parent folder scrolls out of view */}
      {stickyFolder && (
        <div className="sticky z-10 bg-paper -mx-4 px-4 md:-mx-8 md:px-8 border-b border-dashed border-muted" style={{ top: stickyTop }}>
          <div
            className="flex items-center gap-1.5 py-1.5 px-1 cursor-pointer select-none"
            style={{ paddingLeft: 4 }}
            onClick={() => {
              const allEls = document.querySelectorAll<HTMLElement>('[data-tree-idx]');
              if (allEls.length < 2) return;
              const firstEl = allEls[0];
              const lastEl = allEls[allEls.length - 1];
              const firstIdx = parseInt(firstEl.dataset.treeIdx!, 10);
              const lastIdx = parseInt(lastEl.dataset.treeIdx!, 10);
              const avgH = (lastEl.getBoundingClientRect().top - firstEl.getBoundingClientRect().top) / (lastIdx - firstIdx);
              // Estimated viewport position of the folder - desired position (toolbar bottom)
              const offset = firstEl.getBoundingClientRect().top + (stickyFolder.index - firstIdx) * avgH - stickyTop;
              window.scrollBy({ top: offset, behavior: 'smooth' });
            }}
          >
            <FolderOpen size={16} strokeWidth={2.5} className="text-pencil-light shrink-0" />
            <span className={`font-semibold text-sm${stickyFolder.node.isRoot ? ' text-pencil-light' : ' text-pencil'}`}>
              {stickyFolder.node.path || '(root)'}
            </span>
            <span
              className="text-xs text-pencil-light px-1.5 py-0 bg-muted shrink-0 ml-1"
              style={{ borderRadius: radius.sm }}
            >
              {stickyFolder.node.childCount}
            </span>
          </div>
        </div>
      )}

      {/* Virtualized tree */}
      <Virtuoso
        useWindowScroll
        totalCount={rows.length}
        overscan={600}
        itemContent={renderItem}
      />
    </div>
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
                <td className="py-3 pr-4 max-w-[300px]">
                  <Tooltip content={skill.name} block delay={1500}>
                    <Link
                      to={`/skills/${encodeURIComponent(skill.flatName)}`}
                      className="font-medium text-pencil hover:underline block truncate"
                    >
                      {skill.name}
                    </Link>
                  </Tooltip>
                </td>
                {/* Path */}
                <td className="py-3 pr-4 font-mono text-sm text-pencil-light max-w-[200px]">
                  <Tooltip content={skill.relPath} block delay={1500}><span className="block truncate">{skill.relPath}</span></Tooltip>
                </td>
                {/* Type badge */}
                <td className="py-3 pr-4">
                  <div className="flex items-center gap-1.5">
                    {skill.disabled && <Badge variant="danger">disabled</Badge>}
                    {skill.isInRepo ? (
                      <Badge variant="default">tracked</Badge>
                    ) : getTypeLabel(skill.type) ? (
                      <Badge variant="info">{getTypeLabel(skill.type)}</Badge>
                    ) : (
                      <Badge variant="default">local</Badge>
                    )}
                    {skill.branch && (
                      <Badge variant="default">
                        <GitBranch size={10} strokeWidth={2.5} className="inline -mt-px mr-0.5" />
                        {skill.branch}
                      </Badge>
                    )}
                  </div>
                </td>
                {/* Source */}
                <td className="py-3 text-sm text-pencil-light max-w-[280px]">
                  <Tooltip content={skill.source ?? '—'} block delay={1500}><span className="block truncate">{skill.source ? shortSource(skill.source) : '—'}</span></Tooltip>
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
