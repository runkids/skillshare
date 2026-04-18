import { useState, useCallback, useEffect, useRef, useMemo } from 'react';
import { Search, Star, Download, Globe, Database, Settings, LayoutGrid, List } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
import { useT } from '../i18n';
import Card from '../components/Card';
import PageHeader from '../components/PageHeader';
import Badge from '../components/Badge';
import Button from '../components/Button';
import SegmentedControl from '../components/SegmentedControl';
import { Input, Select } from '../components/Input';
import SkillPickerModal from '../components/SkillPickerModal';
import HubManagerModal, { type SavedHub } from '../components/HubManagerModal';
import Pagination from '../components/Pagination';
import Tooltip from '../components/Tooltip';
import EmptyState from '../components/EmptyState';
import { useToast } from '../components/Toast';
import { api, type SearchResult, type DiscoveredSkill } from '../api/client';
import { clearAuditCache } from '../lib/auditCache';
import { queryKeys } from '../lib/queryKeys';
import { formatSkillDisplayName } from '../lib/resourceNames';

type SearchMode = 'github' | 'hub';
type SearchViewType = 'card' | 'table';

const TABLE_PAGE_SIZES = [10, 25, 50] as const;

const COMMUNITY_HUB: SavedHub = {
  label: 'Skillshare Hub',
  url: 'https://raw.githubusercontent.com/runkids/skillshare-hub/main/skillshare-hub.json',
  builtIn: true,
};

function mergeHubs(userHubs: SavedHub[]): SavedHub[] {
  return [COMMUNITY_HUB, ...userHubs.filter((h) => normalizeURL(h.url) !== normalizeURL(COMMUNITY_HUB.url))];
}

function normalizeURL(url: string): string {
  return url.trim().replace(/\/+$/, '');
}

export default function SearchPage() {
  const t = useT();
  const [mode, setMode] = useState<SearchMode>('github');
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [filter, setFilter] = useState('');
  const { toast } = useToast();
  const queryClient = useQueryClient();

  // Hub state
  const [selectedHub, setSelectedHub] = useState(COMMUNITY_HUB.url);
  const [savedHubs, setSavedHubs] = useState<SavedHub[]>([COMMUNITY_HUB]);
  const [showHubManager, setShowHubManager] = useState(false);
  const [hubLoaded, setHubLoaded] = useState(false);

  // View type (card vs table) — persisted in localStorage
  const [viewType, setViewType] = useState<SearchViewType>(() => {
    const saved = localStorage.getItem('skillshare:search-view');
    return saved === 'table' ? 'table' : 'card';
  });
  const changeViewType = useCallback((v: string) => {
    const vt = v as SearchViewType;
    setViewType(vt);
    localStorage.setItem('skillshare:search-view', vt);
  }, []);

  // Install state
  const [installing, setInstalling] = useState<string | null>(null);

  // Table pagination
  const [tablePage, setTablePage] = useState(0);
  const [tablePageSize, setTablePageSize] = useState<number>(10);

  // Incremental rendering
  const PAGE_SIZE = 20;
  const [visibleCount, setVisibleCount] = useState(PAGE_SIZE);
  const sentinelRef = useRef<HTMLDivElement>(null);

  const filteredResults = useMemo(() => {
    if (!results || results.length === 0) return [];
    if (!filter) return results;
    const f = filter.toLowerCase();
    return results.filter((r) =>
      r.name.toLowerCase().includes(f) ||
      r.description.toLowerCase().includes(f) ||
      (r.tags ?? []).some((t) => t.toLowerCase().includes(f)),
    );
  }, [results, filter]);

  const visible = filteredResults.slice(0, visibleCount);
  const hasMore = visible.length < filteredResults.length;

  // Discovery flow state
  const [discoveredSkills, setDiscoveredSkills] = useState<DiscoveredSkill[]>([]);
  const [showPicker, setShowPicker] = useState(false);
  const [pendingSource, setPendingSource] = useState('');
  const [batchInstalling, setBatchInstalling] = useState(false);

  // Fetch hub config from server on mount
  useEffect(() => {
    fetchHubConfig();
  }, []);

  // Reset visible count / table page when results or filter change
  useEffect(() => {
    setVisibleCount(PAGE_SIZE);
    setTablePage(0);
  }, [filteredResults]);

  // IntersectionObserver: load more when sentinel scrolls into view
  useEffect(() => {
    const el = sentinelRef.current;
    if (!el) return;
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          setVisibleCount((prev) => prev + PAGE_SIZE);
        }
      },
      { rootMargin: '200px' },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [hasMore]);

  const fetchHubConfig = async () => {
    try {
      const res = await api.getHubConfig();
      const merged = mergeHubs(
        res.hubs.map((h) => ({ label: h.label, url: h.url, builtIn: h.builtIn })),
      );
      setSavedHubs(merged);

      // Resolve default hub to a URL for the select
      if (res.default) {
        const match = merged.find(
          (h) => h.label.toLowerCase() === res.default.toLowerCase(),
        );
        if (match) {
          setSelectedHub(match.url);
        }
      }
    } catch {
      // Graceful fallback: use community hub only
    } finally {
      setHubLoaded(true);
    }
  };

  const switchMode = useCallback((newMode: SearchMode) => {
    setMode(newMode);
    setResults(null);
  }, []);

  const handleSearch = async (searchQuery?: string) => {
    const q = searchQuery ?? query;
    if (mode === 'hub' && !selectedHub) {
      toast(t('search.hub.addHubFirst'), 'error');
      return;
    }
    setSearching(true);
    setFilter('');
    try {
      let res: { results: SearchResult[] };
      if (mode === 'hub') {
        res = await api.searchHub(q, selectedHub);
      } else {
        res = await api.search(q);
      }
      setResults(res.results);
      if (res.results.length === 0) {
        toast(q ? t('search.results.noResultsFound') : t('search.results.noSkillsFound'), 'info');
      }
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setSearching(false);
    }
  };

  const handleInstall = async (source: string, skill?: string) => {
    setInstalling(source);
    try {
      const disc = await api.discover(source);
      // If hub entry specifies a skill, pre-filter to that skill
      if (skill && disc.skills.length > 1) {
        const matched = disc.skills.filter((s) => s.name === skill);
        if (matched.length > 0) {
          const res = await api.installBatch({ source, skills: matched });
          let hasAuditBlock = false;
          const batchErrors: string[] = [];
          for (const item of res.results) {
            if (item.error) {
              if (item.error.includes('security audit failed')) {
                hasAuditBlock = true;
                batchErrors.push(`${formatSkillDisplayName(item.name)}: blocked by security audit`);
              } else {
                batchErrors.push(`${formatSkillDisplayName(item.name)}: ${item.error}`);
              }
            }
            if (item.warnings?.length) {
              item.warnings.forEach((w) => toast(`${formatSkillDisplayName(item.name)}: ${w}`, 'warning'));
            }
          }
          if (batchErrors.length > 0) toast(t('common.nFailed', { count: batchErrors.length, details: batchErrors.join('; ') }), 'error');
          toast(res.summary, hasAuditBlock ? 'warning' : 'success');
          clearAuditCache(queryClient);
          queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
          queryClient.invalidateQueries({ queryKey: queryKeys.overview });
          return;
        }
        // skill not found in repo — fall through to picker
      }
      if (disc.skills.length > 1) {
        setDiscoveredSkills(disc.skills);
        setPendingSource(source);
        setShowPicker(true);
      } else if (disc.skills.length === 1) {
        const res = await api.installBatch({ source, skills: disc.skills });
        let hasAuditBlock = false;
        const batchErrors: string[] = [];
        for (const item of res.results) {
          if (item.error) {
            if (item.error.includes('security audit failed')) {
              hasAuditBlock = true;
              batchErrors.push(`${formatSkillDisplayName(item.name)}: blocked by security audit`);
            } else {
              batchErrors.push(`${formatSkillDisplayName(item.name)}: ${item.error}`);
            }
          }
          if (item.warnings?.length) {
            item.warnings.forEach((w) => toast(`${formatSkillDisplayName(item.name)}: ${w}`, 'warning'));
          }
        }
        if (batchErrors.length > 0) toast(t('common.nFailed', { count: batchErrors.length, details: batchErrors.join('; ') }), 'error');
        toast(res.summary, hasAuditBlock ? 'warning' : 'success');
        clearAuditCache(queryClient);
        queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
        queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      } else {
        const res = await api.install({ source });
        toast(
          t('search.toast.installed', { name: res.skillName ?? res.repoName ?? '', action: res.action }),
          'success',
        );
        if (res.warnings?.length > 0) {
          res.warnings.forEach((w) => toast(w, 'warning'));
        }
        clearAuditCache(queryClient);
        queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
        queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      }
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setInstalling(null);
    }
  };

  const handleBatchInstall = async (selected: DiscoveredSkill[]) => {
    setBatchInstalling(true);
    try {
      const res = await api.installBatch({
        source: pendingSource,
        skills: selected,
      });
      let hasAuditBlock = false;
      const batchErrors: string[] = [];
      for (const item of res.results) {
        if (item.error) {
          if (item.error.includes('security audit failed')) {
            hasAuditBlock = true;
            batchErrors.push(`${formatSkillDisplayName(item.name)}: blocked by security audit — use Force to override`);
          } else {
            batchErrors.push(`${formatSkillDisplayName(item.name)}: ${item.error}`);
          }
        }
        if (item.warnings?.length) {
          item.warnings.forEach((w) => toast(`${formatSkillDisplayName(item.name)}: ${w}`, 'warning'));
        }
      }
      if (batchErrors.length > 0) toast(t('common.nFailed', { count: batchErrors.length, details: batchErrors.join('; ') }), 'error');
      toast(res.summary, hasAuditBlock ? 'warning' : 'success');
      setShowPicker(false);
      clearAuditCache(queryClient);
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setBatchInstalling(false);
    }
  };

  const handleHubsSave = async (updated: SavedHub[]) => {
    const userOnly = updated.filter((h) => !h.builtIn);
    try {
      // Find the label that matches the currently selected hub URL
      const merged = mergeHubs(userOnly);
      const currentMatch = merged.find((h) => normalizeURL(h.url) === normalizeURL(selectedHub));
      const defaultLabel = currentMatch && !currentMatch.builtIn ? currentMatch.label : '';

      await api.putHubConfig({
        hubs: userOnly.map((h) => ({ label: h.label, url: h.url })),
        default: defaultLabel,
      });
      setSavedHubs(merged);

      // If selected hub was removed, fall back to community hub
      if (!merged.some((h) => normalizeURL(h.url) === normalizeURL(selectedHub))) {
        setSelectedHub(COMMUNITY_HUB.url);
      }
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    }
  };

  const handleSelectHub = async (url: string) => {
    setSelectedHub(url);
    setResults(null);

    // Persist selected hub as default on server
    const match = savedHubs.find((h) => normalizeURL(h.url) === normalizeURL(url));
    if (match && !match.builtIn) {
      try {
        const userOnly = savedHubs.filter((h) => !h.builtIn);
        await api.putHubConfig({
          hubs: userOnly.map((h) => ({ label: h.label, url: h.url })),
          default: match.label,
        });
      } catch {
        // Non-critical — selection still works locally
      }
    } else {
      // Selected community hub — clear default
      try {
        const userOnly = savedHubs.filter((h) => !h.builtIn);
        await api.putHubConfig({
          hubs: userOnly.map((h) => ({ label: h.label, url: h.url })),
          default: '',
        });
      } catch {
        // Non-critical
      }
    }
  };

  const handleHubManagerClose = () => {
    setShowHubManager(false);
    // Re-fetch to ensure we're in sync with server
    fetchHubConfig();
  };

  return (
    <div className="space-y-3 animate-fade-in">
      <PageHeader icon={<Search size={24} strokeWidth={2.5} />} title={t('search.title')} subtitle={t('search.subtitle')} />

      {/* Mode tabs + search */}
      <div className="flex flex-wrap items-end gap-3">
        <SegmentedControl
          value={mode}
          onChange={(v) => switchMode(v as SearchMode)}
          options={[
            { value: 'github', label: <span className="inline-flex items-center gap-1.5"><Globe size={14} strokeWidth={2.5} />GitHub</span> },
            { value: 'hub', label: <span className="inline-flex items-center gap-1.5"><Database size={14} strokeWidth={2.5} />Hub</span> },
          ]}
          connected
        />
      </div>

      {/* Hub selector (only in hub mode) */}
      {mode === 'hub' && hubLoaded && (
        <Card overflow>
          {savedHubs.length > 0 ? (
            <div className="flex items-center gap-2">
              <Select
                value={selectedHub}
                onChange={handleSelectHub}
                options={savedHubs.map((h) => ({ value: h.url, label: h.label }))}
                className="flex-1"
              />
              <Button
                onClick={() => setShowHubManager(true)}
                variant="ghost"
                size="sm"
                title={t('search.hub.manageButton')}
              >
                <Settings size={14} strokeWidth={2.5} />
                {t('search.hub.manageButton')}
              </Button>
            </div>
          ) : (
            <div className="text-center py-3">
              <p className="text-base text-muted-dark mb-3">
                {t('search.hub.noHubsConfigured')}
              </p>
              <Button
                onClick={() => setShowHubManager(true)}
                variant="secondary"
                size="sm"
              >
                <Settings size={14} strokeWidth={2.5} />
                {t('search.hub.manageHubsButton')}
              </Button>
            </div>
          )}
          <p className="text-sm text-muted-dark mt-3 flex items-center gap-1.5">
            <Globe size={12} strokeWidth={2} />
            {t('search.hub.submitPrPrompt')}
            {' '}
            <a
              href="https://github.com/runkids/skillshare-hub"
              target="_blank"
              rel="noopener noreferrer"
              className="text-blue hover:underline"
            >
              skillshare-hub
            </a>
            .
          </p>
        </Card>
      )}

      {/* Search box */}
      <div data-tour="search-input">
      <Card>
        <div className="flex gap-3">
          <div className="relative flex-1">
            <Search
              size={18}
              strokeWidth={2.5}
              className="absolute left-4 top-1/2 -translate-y-1/2 text-muted-dark pointer-events-none"
            />
            <Input
              type="text"
              placeholder={mode === 'github' ? t('search.input.placeholderGithub') : t('search.input.placeholderHub')}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleSearch(query)}
              className="!pl-11"
            />
          </div>
          <Button
            onClick={() => handleSearch(query)}
            variant="primary"
            size="md"
            loading={searching}
          >
            {!searching && <Search size={16} strokeWidth={2.5} />}
            {t('search.button.search')}
          </Button>
        </div>
        {mode === 'github' && (
          <p className="text-sm text-muted-dark mt-3 flex items-center gap-1">
            <Globe size={12} strokeWidth={2} />
            {t('search.githubTokenHint')}
          </p>
        )}
      </Card>
      </div>

      {/* Sticky toolbar: summary + filter + view toggle */}
      {results && results.length > 0 && (
        <div className="space-y-2">
          <div className="sticky top-0 z-20 bg-paper -mx-4 px-4 md:-mx-8 md:px-8 py-2">
            <div className="flex items-center gap-3 flex-wrap">
              <p className="text-sm text-pencil-light whitespace-nowrap">
                {filteredResults.length !== 1
                  ? t('search.results.countPlural', { count: filteredResults.length })
                  : t('search.results.count', { count: filteredResults.length })}
              </p>
              <div className="relative flex-1 min-w-[200px]">
                <Search
                  size={14}
                  strokeWidth={2.5}
                  className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-dark pointer-events-none"
                />
                <Input
                  type="text"
                  placeholder={t('search.input.filterPlaceholder')}
                  value={filter}
                  onChange={(e) => setFilter(e.target.value)}
                  className="!pl-8 !py-1.5 !text-sm"
                />
              </div>
              <SegmentedControl
                value={viewType}
                onChange={changeViewType}
                options={[
                  { value: 'card', label: <LayoutGrid size={16} strokeWidth={2.5} /> },
                  { value: 'table', label: <List size={16} strokeWidth={2.5} /> },
                ]}
                size="sm"
                connected
              />
            </div>
          </div>

          {/* Results: card view or table view */}
          {viewType === 'card' ? (
            <div className="space-y-3">
              {visible.map((r) => (
                <Card key={r.source} tilt>
                  <div className="flex items-start justify-between gap-4">
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 mb-1 flex-wrap">
                        <span className="font-bold text-pencil text-lg">
                          {r.name}
                        </span>
                        {r.stars > 0 && (
                          <span className="flex items-center gap-1 text-sm text-warning">
                            <Star size={14} strokeWidth={2.5} fill="currentColor" />
                            {r.stars}
                          </span>
                        )}
                        {r.owner && <Badge>{r.owner}</Badge>}
                      </div>
                      {r.description && (
                        <p className="text-pencil-light mb-2">{r.description}</p>
                      )}
                      {r.tags && r.tags.length > 0 && (
                        <div className="flex flex-wrap gap-1.5 mb-2">
                          {r.tags.map((tag) => (
                            <Badge key={tag} variant="info" size="sm">#{tag}</Badge>
                          ))}
                        </div>
                      )}
                      <p className="font-mono text-xs text-muted-dark truncate">
                        {r.source}
                      </p>
                    </div>
                    <Button
                      onClick={() => handleInstall(r.source, r.skill)}
                      variant="secondary"
                      size="sm"
                      loading={installing === r.source}
                      className="shrink-0"
                    >
                      {installing !== r.source && <Download size={14} strokeWidth={2.5} />}
                      {t('search.table.installButton')}
                    </Button>
                  </div>
                </Card>
              ))}
              {hasMore && <div ref={sentinelRef} className="h-4" />}
            </div>
          ) : (
            <SearchResultsTable
              results={filteredResults}
              page={tablePage}
              pageSize={tablePageSize}
              onPageChange={setTablePage}
              onPageSizeChange={(s) => { setTablePageSize(s); setTablePage(0); }}
              installing={installing}
              onInstall={handleInstall}
            />
          )}
        </div>
      )}

      {results && results.length === 0 && (
        <EmptyState
          icon={Search}
          title={t('search.empty.noResults.title')}
          description={
            mode === 'github'
              ? t('search.empty.noResults.description.github')
              : t('search.empty.noResults.description.hub')
          }
        />
      )}

      {/* Initial state before any search */}
      {!results && !searching && (
        <EmptyState
          icon={Search}
          title={t('search.empty.start.title')}
          description={
            mode === 'github'
              ? t('search.empty.start.description.github')
              : t('search.empty.start.description.hub')
          }
          action={
            <Button
              onClick={() => handleSearch('')}
              variant="secondary"
              size="sm"
            >
              <Star size={14} strokeWidth={2.5} />
              {mode === 'github' ? t('search.empty.start.browsePopularSkills') : t('search.empty.start.browseAllSkills')}
            </Button>
          }
        />
      )}

      {/* Hub Manager Modal */}
      <HubManagerModal
        open={showHubManager}
        hubs={savedHubs}
        onSave={handleHubsSave}
        onClose={handleHubManagerClose}
      />

      {/* Skill Picker Modal for multi-skill repos */}
      <SkillPickerModal
        open={showPicker}
        source={pendingSource}
        skills={discoveredSkills}
        onInstall={handleBatchInstall}
        onCancel={() => setShowPicker(false)}
        installing={batchInstalling}
      />
    </div>
  );
}

/* -- Table view with pagination ------------------- */

function SearchResultsTable({
  results,
  page,
  pageSize,
  onPageChange,
  onPageSizeChange,
  installing,
  onInstall,
}: {
  results: SearchResult[];
  page: number;
  pageSize: number;
  onPageChange: (p: number) => void;
  onPageSizeChange: (s: number) => void;
  installing: string | null;
  onInstall: (source: string, skill?: string) => void;
}) {
  const t = useT();
  const totalPages = Math.max(1, Math.ceil(results.length / pageSize));
  const start = page * pageSize;
  const visible = results.slice(start, start + pageSize);

  return (
    <Card>
      <div className="overflow-auto max-h-[calc(100vh-320px)]">
        <table className="w-full text-left">
          <thead className="sticky top-0 z-10 bg-surface">
            <tr className="border-b-2 border-dashed border-muted-dark">
              <th className="pb-3 pr-4 text-pencil-light text-sm font-medium">{t('search.table.columnName')}</th>
              <th className="pb-3 pr-4 text-pencil-light text-sm font-medium hidden md:table-cell">{t('search.table.columnDescription')}</th>
              <th className="pb-3 pr-4 text-pencil-light text-sm font-medium">{t('search.table.columnOwner')}</th>
              <th className="pb-3 pr-4 text-pencil-light text-sm font-medium">{t('search.table.columnStars')}</th>
              <th className="pb-3 pr-4 text-pencil-light text-sm font-medium hidden lg:table-cell">{t('search.table.columnTags')}</th>
              <th className="pb-3 text-pencil-light text-sm font-medium w-[100px]" />
            </tr>
          </thead>
          <tbody>
            {visible.map((r) => (
              <tr
                key={r.source}
                className="border-b border-dashed border-muted hover:bg-paper-warm/60 transition-colors"
              >
                {/* Name + source */}
                <td className="py-3 pr-4">
                  <span className="font-medium text-pencil">{r.name}</span>
                  <p className="font-mono text-xs text-muted-dark truncate max-w-[200px]">
                    {r.source}
                  </p>
                </td>
                {/* Description */}
                <td className="py-3 pr-4 text-sm text-pencil-light max-w-[300px] hidden md:table-cell">
                  <Tooltip content={r.description || '—'}>
                    <span className="block truncate">{r.description || '—'}</span>
                  </Tooltip>
                </td>
                {/* Owner */}
                <td className="py-3 pr-4">
                  {r.owner ? <Badge>{r.owner}</Badge> : <span className="text-muted-dark">—</span>}
                </td>
                {/* Stars */}
                <td className="py-3 pr-4">
                  {r.stars > 0 ? (
                    <span className="flex items-center gap-1 text-sm text-warning">
                      <Star size={14} strokeWidth={2.5} fill="currentColor" />
                      {r.stars}
                    </span>
                  ) : (
                    <span className="text-muted-dark">—</span>
                  )}
                </td>
                {/* Tags */}
                <td className="py-3 pr-4 hidden lg:table-cell">
                  <div className="flex flex-wrap gap-1 max-w-[200px]">
                    {(r.tags ?? []).slice(0, 3).map((tag) => (
                      <Badge key={tag} variant="info" size="sm">#{tag}</Badge>
                    ))}
                    {(r.tags ?? []).length > 3 && (
                      <Badge variant="default" size="sm">+{r.tags!.length - 3}</Badge>
                    )}
                  </div>
                </td>
                {/* Install */}
                <td className="py-3">
                  <Button
                    onClick={() => onInstall(r.source, r.skill)}
                    variant="secondary"
                    size="sm"
                    loading={installing === r.source}
                  >
                    {installing !== r.source && <Download size={14} strokeWidth={2.5} />}
                    {t('search.table.installButton')}
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {results.length > TABLE_PAGE_SIZES[0] && (
        <Pagination
          page={page}
          totalPages={totalPages}
          onPageChange={onPageChange}
          rangeText={`${start + 1}–${Math.min(start + pageSize, results.length)} of ${results.length}`}
          pageSize={{
            value: pageSize,
            options: TABLE_PAGE_SIZES,
            onChange: onPageSizeChange,
          }}
        />
      )}
    </Card>
  );
}
