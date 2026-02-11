import { useState, useMemo } from 'react';
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
} from 'lucide-react';
import Badge from '../components/Badge';
import { HandInput, HandSelect } from '../components/HandInput';
import { PageSkeleton } from '../components/Skeleton';
import EmptyState from '../components/EmptyState';
import Card from '../components/Card';
import { api, type Skill } from '../api/client';
import { useApi } from '../hooks/useApi';
import { wobbly, shadows } from '../design';

/* ── Color palette ──────────────────────────────── */

const postitColors = [
  { bg: '#fff9c4', border: '#e6d95a' }, // classic yellow
  { bg: '#bbdefb', border: '#64b5f6' }, // sky blue
  { bg: '#f8bbd0', border: '#f06292' }, // soft pink
  { bg: '#ffe0b2', border: '#ffb74d' }, // peach
  { bg: '#d1c4e9', border: '#9575cd' }, // lavender
  { bg: '#c8e6c9', border: '#81c784' }, // mint green
];

const trackedColors = [
  { bg: '#e8f5e9', border: '#2e7d32' }, // deep green 1
  { bg: '#c8e6c9', border: '#388e3c' }, // deep green 2
  { bg: '#dcedc8', border: '#558b2f' }, // olive green
];

function getPostitColor(skill: Skill, index: number) {
  if (skill.isInRepo) {
    return trackedColors[index % trackedColors.length];
  }
  return postitColors[index % postitColors.length];
}

function getRotation(index: number): string {
  const rotations = ['-1.2deg', '0.8deg', '-0.5deg', '1.5deg', '-0.3deg', '1deg'];
  return rotations[index % rotations.length];
}

/* ── Filter & Sort types ────────────────────────── */

type FilterType = 'all' | 'tracked' | 'github' | 'local';
type SortType = 'name-asc' | 'name-desc' | 'newest' | 'oldest';

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

/* ── Filter chip component ──────────────────────── */

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
            ? 'bg-pencil text-white border-pencil'
            : 'bg-white text-pencil-light border-muted hover:border-pencil hover:text-pencil'
        }
      `}
      style={{
        borderRadius: wobbly.full,
        fontFamily: 'var(--font-hand)',
        boxShadow: active ? '2px 2px 0px 0px #2d2d2d' : 'none',
      }}
    >
      {icon}
      <span>{label}</span>
      <span
        className={`
          text-xs px-1.5 py-0.5 rounded-full min-w-[20px] text-center
          ${active ? 'bg-white/20 text-white' : 'bg-muted text-pencil-light'}
        `}
      >
        {count}
      </span>
    </button>
  );
}

/* ── Main page ──────────────────────────────────── */

export default function SkillsPage() {
  const { data, loading, error } = useApi(() => api.listSkills());
  const [search, setSearch] = useState('');
  const [filterType, setFilterType] = useState<FilterType>('all');
  const [sortType, setSortType] = useState<SortType>('name-asc');

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

  // Apply text filter → type filter → sort
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

  if (loading) return <PageSkeleton />;
  if (error) {
    return (
      <Card variant="accent" className="text-center py-8">
        <p className="text-danger text-lg" style={{ fontFamily: 'var(--font-heading)' }}>
          Failed to load skills
        </p>
        <p className="text-pencil-light text-base mt-1">{error}</p>
      </Card>
    );
  }

  return (
    <div className="animate-sketch-in">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2
            className="text-3xl md:text-4xl font-bold text-pencil mb-1"
            style={{ fontFamily: 'var(--font-heading)' }}
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
          <HandInput
            type="text"
            placeholder="Filter skills..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="!pl-11"
          />
        </div>
        <div className="flex items-center gap-2 sm:w-52">
          <ArrowUpDown size={16} strokeWidth={2.5} className="text-pencil-light shrink-0" />
          <HandSelect
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
        <p className="text-pencil-light text-sm mb-4" style={{ fontFamily: 'var(--font-hand)' }}>
          Showing {filtered.length} of {skills.length} skills
          {filterType !== 'all' && (
            <>
              {' '}
              &middot;{' '}
              <button
                className="underline text-blue cursor-pointer hover:text-pencil transition-colors"
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

      {/* Skills grid */}
      {filtered.length > 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-5">
          {filtered.map((skill, i) => (
            <SkillPostit key={skill.flatName} skill={skill} index={i} />
          ))}
        </div>
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

/* ── Tracked (organization) card ────────────────── */

function TrackedPostit({ skill, index }: { skill: Skill; index: number }) {
  const color = getPostitColor(skill, index);
  const rotation = getRotation(index);

  // Extract repo name from relPath (e.g., "_awesome-skillshare-skills/frontend-dugong" → "awesome-skillshare-skills")
  const repoName = skill.relPath.startsWith('_')
    ? skill.relPath.split('/')[0].slice(1).replace(/__/g, '/')
    : undefined;

  return (
    <Link to={`/skills/${encodeURIComponent(skill.flatName)}`}>
      <div
        className="relative border-2 cursor-pointer transition-all duration-150 hover:scale-[1.03] hover:z-10 overflow-hidden"
        style={{
          backgroundColor: color.bg,
          borderColor: color.border,
          borderRadius: wobbly.md,
          boxShadow: `4px 4px 0px 0px ${color.border}`,
          transform: `rotate(${rotation})`,
          fontFamily: 'var(--font-hand)',
        }}
        onMouseEnter={(e) => {
          e.currentTarget.style.boxShadow = `8px 8px 0px 0px ${color.border}`;
          e.currentTarget.style.transform = `rotate(0deg) scale(1.03)`;
        }}
        onMouseLeave={(e) => {
          e.currentTarget.style.boxShadow = `4px 4px 0px 0px ${color.border}`;
          e.currentTarget.style.transform = `rotate(${rotation})`;
        }}
      >
        {/* Organization banner */}
        <div
          className="flex items-center gap-1.5 px-4 py-1.5 border-b-2"
          style={{
            backgroundColor: color.border,
            borderColor: color.border,
          }}
        >
          <Users size={13} strokeWidth={2.5} className="text-white" />
          <span
            className="text-xs text-white font-bold tracking-wide uppercase truncate"
            style={{ fontFamily: 'var(--font-heading)' }}
          >
            {repoName ?? 'Organization'}
          </span>
        </div>

        {/* Card body */}
        <div className="p-5 pb-4">
          {/* Skill name */}
          <div className="flex items-center gap-2 mb-2">
            <div className="shrink-0">
              <GitBranch size={18} strokeWidth={2.5} className="text-pencil" />
            </div>
            <h3
              className="font-bold text-pencil text-lg truncate leading-tight"
              style={{ fontFamily: 'var(--font-heading)' }}
            >
              {skill.name}
            </h3>
          </div>

          {/* Path */}
          <p
            className="text-sm text-pencil-light truncate mb-2"
            style={{ fontFamily: "'Courier New', monospace" }}
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
              <Badge variant="success">tracked</Badge>
            </div>
          </div>
        </div>
      </div>
    </Link>
  );
}

/* ── Regular skill card ─────────────────────────── */

function RegularPostit({ skill, index }: { skill: Skill; index: number }) {
  const color = getPostitColor(skill, index);
  const rotation = getRotation(index);

  return (
    <Link to={`/skills/${encodeURIComponent(skill.flatName)}`}>
      <div
        className="relative p-5 pb-4 border-2 cursor-pointer transition-all duration-150 hover:scale-[1.03] hover:z-10"
        style={{
          backgroundColor: color.bg,
          borderColor: color.border,
          borderRadius: wobbly.md,
          boxShadow: shadows.md,
          transform: `rotate(${rotation})`,
          fontFamily: 'var(--font-hand)',
        }}
        onMouseEnter={(e) => {
          e.currentTarget.style.boxShadow = shadows.lg;
          e.currentTarget.style.transform = `rotate(0deg) scale(1.03)`;
        }}
        onMouseLeave={(e) => {
          e.currentTarget.style.boxShadow = shadows.md;
          e.currentTarget.style.transform = `rotate(${rotation})`;
        }}
      >
        {/* Tape decoration at top */}
        <div
          className="absolute -top-2.5 left-1/2 -translate-x-1/2 w-14 h-5 opacity-50 z-10"
          style={{
            background: 'linear-gradient(135deg, rgba(255,255,255,0.7), rgba(200,200,180,0.4))',
            borderRadius: '2px',
            transform: `rotate(${index % 2 === 0 ? '-3deg' : '2deg'})`,
            border: '1px solid rgba(180,170,150,0.3)',
          }}
        />

        {/* Skill name */}
        <div className="flex items-center gap-2 mb-2">
          <div className="shrink-0">
            <Folder size={18} strokeWidth={2.5} className="text-pencil" />
          </div>
          <h3
            className="font-bold text-pencil text-lg truncate leading-tight"
            style={{ fontFamily: 'var(--font-heading)' }}
          >
            {skill.name}
          </h3>
        </div>

        {/* Path */}
        <p
          className="text-sm text-pencil-light truncate mb-2"
          style={{ fontFamily: "'Courier New', monospace" }}
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
            {getTypeLabel(skill.type) && <Badge variant="info">{getTypeLabel(skill.type)}</Badge>}
          </div>
        </div>
      </div>
    </Link>
  );
}

/* ── Card dispatcher ────────────────────────────── */

function SkillPostit({ skill, index }: { skill: Skill; index: number }) {
  if (skill.isInRepo) {
    return <TrackedPostit skill={skill} index={index} />;
  }
  return <RegularPostit skill={skill} index={index} />;
}
