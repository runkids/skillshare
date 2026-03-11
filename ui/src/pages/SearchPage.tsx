import { useState, useCallback, useEffect, useRef, useMemo } from 'react';
import { Search, Star, Download, Globe, Database, Settings } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
import Card from '../components/Card';
import PageHeader from '../components/PageHeader';
import Badge from '../components/Badge';
import Button from '../components/Button';
import { Input, Select } from '../components/Input';
import SkillPickerModal from '../components/SkillPickerModal';
import HubManagerModal, { type SavedHub } from '../components/HubManagerModal';
import EmptyState from '../components/EmptyState';
import { useToast } from '../components/Toast';
import { api, type SearchResult, type DiscoveredSkill } from '../api/client';
import { queryKeys } from '../lib/queryKeys';

type SearchMode = 'github' | 'hub';

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

  // Install state
  const [installing, setInstalling] = useState<string | null>(null);

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

  // Reset visible count when results or filter change
  useEffect(() => {
    setVisibleCount(PAGE_SIZE);
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
      toast('Add a hub source first', 'error');
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
        toast(q ? 'No results found.' : 'No skills found.', 'info');
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
          for (const item of res.results) {
            if (item.error) {
              if (item.error.includes('security audit failed')) {
                hasAuditBlock = true;
                toast(`${item.name}: blocked by security audit`, 'error');
              } else {
                toast(`${item.name}: ${item.error}`, 'error');
              }
            }
            if (item.warnings?.length) {
              item.warnings.forEach((w) => toast(`${item.name}: ${w}`, 'warning'));
            }
          }
          toast(res.summary, hasAuditBlock ? 'warning' : 'success');
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
        for (const item of res.results) {
          if (item.error) {
            if (item.error.includes('security audit failed')) {
              hasAuditBlock = true;
              toast(`${item.name}: blocked by security audit`, 'error');
            } else {
              toast(`${item.name}: ${item.error}`, 'error');
            }
          }
          if (item.warnings?.length) {
            item.warnings.forEach((w) => toast(`${item.name}: ${w}`, 'warning'));
          }
        }
        toast(res.summary, hasAuditBlock ? 'warning' : 'success');
        queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
        queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      } else {
        const res = await api.install({ source });
        toast(
          `Installed: ${res.skillName ?? res.repoName} (${res.action})`,
          'success',
        );
        if (res.warnings?.length > 0) {
          res.warnings.forEach((w) => toast(w, 'warning'));
        }
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
      for (const item of res.results) {
        if (item.error) {
          if (item.error.includes('security audit failed')) {
            hasAuditBlock = true;
            toast(`${item.name}: blocked by security audit — use Force to override`, 'error');
          } else {
            toast(`${item.name}: ${item.error}`, 'error');
          }
        }
        if (item.warnings?.length) {
          item.warnings.forEach((w) => toast(`${item.name}: ${w}`, 'warning'));
        }
      }
      toast(res.summary, hasAuditBlock ? 'warning' : 'success');
      setShowPicker(false);
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
    <div className="animate-fade-in">
      <PageHeader icon={<Search size={24} strokeWidth={2.5} />} title="Search Skills" subtitle="Discover and install skills" />

      {/* Mode tabs */}
      <div className="flex gap-2 mb-4">
        <Button
          onClick={() => switchMode('github')}
          variant={mode === 'github' ? 'primary' : 'secondary'}
          size="sm"
        >
          <Globe size={14} strokeWidth={2.5} />
          GitHub
        </Button>
        <Button
          onClick={() => switchMode('hub')}
          variant={mode === 'hub' ? 'primary' : 'secondary'}
          size="sm"
        >
          <Database size={14} strokeWidth={2.5} />
          Hub
        </Button>
      </div>

      {/* Hub selector (only in hub mode) */}
      {mode === 'hub' && hubLoaded && (
        <Card className="mb-4 !overflow-visible">
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
                title="Manage hubs"
              >
                <Settings size={14} strokeWidth={2.5} />
                Manage
              </Button>
            </div>
          ) : (
            <div className="text-center py-3">
              <p className="text-base text-muted-dark mb-3">
                No hubs configured. Add one to get started.
              </p>
              <Button
                onClick={() => setShowHubManager(true)}
                variant="secondary"
                size="sm"
              >
                <Settings size={14} strokeWidth={2.5} />
                Manage Hubs
              </Button>
            </div>
          )}
          <p className="text-sm text-muted-dark mt-3 flex items-center gap-1.5">
            <Globe size={12} strokeWidth={2} />
            Found an awesome skill? Submit a PR to
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
      <Card className="mb-6">
        <div className="flex gap-3">
          <div className="relative flex-1">
            <Search
              size={18}
              strokeWidth={2.5}
              className="absolute left-4 top-1/2 -translate-y-1/2 text-muted-dark pointer-events-none"
            />
            <Input
              type="text"
              placeholder={mode === 'github' ? 'Search GitHub for skills...' : 'Search hub...'}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleSearch(query)}
              className="!pl-11"
            />
          </div>
          <Button
            onClick={() => handleSearch(query)}
            disabled={searching}
            variant="primary"
            size="md"
          >
            <Search size={16} strokeWidth={2.5} />
            {searching ? 'Searching...' : 'Search'}
          </Button>
        </div>
        {mode === 'github' && (
          <p className="text-sm text-muted-dark mt-3 flex items-center gap-1">
            <Globe size={12} strokeWidth={2} />
            Requires GITHUB_TOKEN environment variable for GitHub API access.
          </p>
        )}
      </Card>

      {/* Results */}
      {results && results.length > 0 && (
        <div className="space-y-4">
          <div className="flex items-center gap-3 flex-wrap">
            <p className="text-base text-pencil-light whitespace-nowrap">
              {filteredResults.length} result{filteredResults.length !== 1 ? 's' : ''} found
            </p>
            <div className="relative flex-1 min-w-[200px]">
              <Search
                size={14}
                strokeWidth={2.5}
                className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-dark pointer-events-none"
              />
              <Input
                type="text"
                placeholder="Filter by name, description, or tag..."
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
                className="!pl-8 !py-1.5 !text-sm"
              />
            </div>
          </div>
          {visible.map((r) => (
            <Card
              key={r.source}
              className="odd:rotate-[-0.15deg] even:rotate-[0.15deg]"
            >
              <div className="flex items-start justify-between gap-4">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1 flex-wrap">
                    <span
                      className="font-bold text-pencil"
                    >
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
                    <p className="text-base text-pencil-light mb-1.5">{r.description}</p>
                  )}
                  {r.tags && r.tags.length > 0 && (
                    <div className="flex flex-wrap gap-1.5 mb-1.5">
                      {r.tags.map((tag) => (
                        <Badge key={tag} variant="accent">#{tag}</Badge>
                      ))}
                    </div>
                  )}
                  <p
                    className="font-mono text-sm text-muted-dark truncate"
                  >
                    {r.source}
                  </p>
                </div>
                <Button
                  onClick={() => handleInstall(r.source, r.skill)}
                  disabled={installing === r.source}
                  variant="secondary"
                  size="sm"
                  className="shrink-0"
                >
                  <Download size={14} strokeWidth={2.5} />
                  {installing === r.source ? 'Installing...' : 'Install'}
                </Button>
              </div>
            </Card>
          ))}
          {hasMore && <div ref={sentinelRef} className="h-4" />}
        </div>
      )}

      {results && results.length === 0 && (
        <EmptyState
          icon={Search}
          title="No results found"
          description={
            mode === 'github'
              ? 'Try different search terms or check your GITHUB_TOKEN.'
              : 'Try different search terms or check your hub source.'
          }
        />
      )}

      {/* Initial state before any search */}
      {!results && !searching && (
        <EmptyState
          icon={Search}
          title="Start searching"
          description={
            mode === 'github'
              ? 'Type a query above to find skills on GitHub'
              : 'Type a query above, or search with empty query to browse all'
          }
          action={
            <Button
              onClick={() => handleSearch('')}
              variant="secondary"
              size="sm"
            >
              <Star size={14} strokeWidth={2.5} />
              {mode === 'github' ? 'Browse Popular Skills' : 'Browse All Skills'}
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
