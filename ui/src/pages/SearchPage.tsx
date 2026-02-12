import { useState, useCallback } from 'react';
import { Search, Star, Download, Globe, Database, Plus, Trash2, Server } from 'lucide-react';
import Card from '../components/Card';
import Badge from '../components/Badge';
import HandButton from '../components/HandButton';
import { HandInput } from '../components/HandInput';
import SkillPickerModal from '../components/SkillPickerModal';
import EmptyState from '../components/EmptyState';
import { useToast } from '../components/Toast';
import { api, type SearchResult, type DiscoveredSkill } from '../api/client';

type SearchMode = 'github' | 'index';

interface SavedIndex {
  label: string;
  url: string;
}

const BUILT_IN_INDEX: SavedIndex = {
  label: 'Current Skills',
  url: '/api/hub/index',
};

const LS_MODE = 'search:mode';
const LS_SELECTED = 'search:selectedIndex';
const LS_SAVED = 'search:savedIndexes';

function loadMode(): SearchMode {
  const v = localStorage.getItem(LS_MODE);
  return v === 'index' ? 'index' : 'github';
}

function loadSelectedIndex(): string {
  return localStorage.getItem(LS_SELECTED) ?? BUILT_IN_INDEX.url;
}

function loadSavedIndexes(): SavedIndex[] {
  try {
    const raw = localStorage.getItem(LS_SAVED);
    if (!raw) return [];
    return JSON.parse(raw) as SavedIndex[];
  } catch {
    return [];
  }
}

function normalizeURL(url: string): string {
  return url.trim().replace(/\/+$/, '');
}

export default function SearchPage() {
  const [mode, setMode] = useState<SearchMode>(loadMode);
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[] | null>(null);
  const [searching, setSearching] = useState(false);
  const { toast } = useToast();

  // Index state
  const [selectedIndex, setSelectedIndex] = useState(loadSelectedIndex);
  const [savedIndexes, setSavedIndexes] = useState<SavedIndex[]>(loadSavedIndexes);
  const [showAddIndex, setShowAddIndex] = useState(false);
  const [newLabel, setNewLabel] = useState('');
  const [newURL, setNewURL] = useState('');

  // Install state
  const [installing, setInstalling] = useState<string | null>(null);

  // Discovery flow state
  const [discoveredSkills, setDiscoveredSkills] = useState<DiscoveredSkill[]>([]);
  const [showPicker, setShowPicker] = useState(false);
  const [pendingSource, setPendingSource] = useState('');
  const [batchInstalling, setBatchInstalling] = useState(false);

  const allIndexes = [BUILT_IN_INDEX, ...savedIndexes];

  const switchMode = useCallback((newMode: SearchMode) => {
    setMode(newMode);
    setResults(null);
    localStorage.setItem(LS_MODE, newMode);
  }, []);

  const handleSearch = async (searchQuery?: string) => {
    const q = searchQuery ?? query;
    setSearching(true);
    try {
      let res: { results: SearchResult[] };
      if (mode === 'index') {
        res = await api.searchHub(q, selectedIndex);
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

  const handleInstall = async (source: string) => {
    setInstalling(source);
    try {
      const disc = await api.discover(source);
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
      } else {
        const res = await api.install({ source });
        toast(
          `Installed: ${res.skillName ?? res.repoName} (${res.action})`,
          'success',
        );
        if (res.warnings?.length > 0) {
          res.warnings.forEach((w) => toast(w, 'warning'));
        }
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
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setBatchInstalling(false);
    }
  };

  const handleAddIndex = () => {
    const url = normalizeURL(newURL);
    if (!url) {
      toast('URL is required', 'error');
      return;
    }
    const label = newLabel.trim() || url.split('/').pop() || 'Untitled';

    // Dedup check
    const allURLs = allIndexes.map((idx) => normalizeURL(idx.url));
    if (allURLs.includes(url)) {
      toast('This index URL already exists', 'error');
      return;
    }

    const updated = [...savedIndexes, { label, url }];
    setSavedIndexes(updated);
    localStorage.setItem(LS_SAVED, JSON.stringify(updated));
    setSelectedIndex(url);
    localStorage.setItem(LS_SELECTED, url);
    setNewLabel('');
    setNewURL('');
    setShowAddIndex(false);
    toast(`Added index: ${label}`, 'success');
  };

  const handleDeleteIndex = (url: string) => {
    const updated = savedIndexes.filter((idx) => normalizeURL(idx.url) !== normalizeURL(url));
    setSavedIndexes(updated);
    localStorage.setItem(LS_SAVED, JSON.stringify(updated));
    if (normalizeURL(selectedIndex) === normalizeURL(url)) {
      setSelectedIndex(BUILT_IN_INDEX.url);
      localStorage.setItem(LS_SELECTED, BUILT_IN_INDEX.url);
    }
    toast('Index removed', 'info');
  };

  const handleSelectIndex = (url: string) => {
    setSelectedIndex(url);
    localStorage.setItem(LS_SELECTED, url);
    setResults(null);
  };

  const currentIndexLabel = allIndexes.find((idx) => normalizeURL(idx.url) === normalizeURL(selectedIndex))?.label ?? selectedIndex;

  return (
    <div className="animate-sketch-in">
      {/* Header */}
      <div className="mb-6">
        <h2
          className="text-3xl md:text-4xl font-bold text-pencil mb-2"
          style={{ fontFamily: 'var(--font-heading)' }}
        >
          Search Skills
        </h2>
        <p className="text-pencil-light">
          Discover and install skills
        </p>
      </div>

      {/* Mode tabs */}
      <div className="flex gap-2 mb-4">
        <HandButton
          onClick={() => switchMode('github')}
          variant={mode === 'github' ? 'primary' : 'secondary'}
          size="sm"
        >
          <Globe size={14} strokeWidth={2.5} />
          GitHub
        </HandButton>
        <HandButton
          onClick={() => switchMode('index')}
          variant={mode === 'index' ? 'primary' : 'secondary'}
          size="sm"
        >
          <Database size={14} strokeWidth={2.5} />
          Private Index
        </HandButton>
      </div>

      {/* Index selector (only in index mode) */}
      {mode === 'index' && (
        <Card className="mb-4">
          <div className="flex items-center gap-2 mb-3">
            <Server size={16} strokeWidth={2.5} className="text-pencil-light" />
            <span className="font-medium text-pencil text-sm">Index Source</span>
          </div>
          <div className="flex flex-wrap gap-2 mb-3">
            {allIndexes.map((idx) => {
              const isSelected = normalizeURL(idx.url) === normalizeURL(selectedIndex);
              const isBuiltIn = idx.url === BUILT_IN_INDEX.url;
              return (
                <div key={idx.url} className="flex items-center gap-1">
                  <HandButton
                    onClick={() => handleSelectIndex(idx.url)}
                    variant={isSelected ? 'primary' : 'secondary'}
                    size="sm"
                  >
                    {idx.label}
                  </HandButton>
                  {!isBuiltIn && (
                    <button
                      onClick={() => handleDeleteIndex(idx.url)}
                      className="text-pencil-light hover:text-danger transition-colors p-1"
                      title="Remove index"
                    >
                      <Trash2 size={12} strokeWidth={2.5} />
                    </button>
                  )}
                </div>
              );
            })}
            <HandButton
              onClick={() => setShowAddIndex(!showAddIndex)}
              variant="secondary"
              size="sm"
            >
              <Plus size={14} strokeWidth={2.5} />
              Add Index
            </HandButton>
          </div>

          {/* Add index form */}
          {showAddIndex && (
            <div className="border-t border-dashed border-pencil-light pt-3 mt-1">
              <div className="flex gap-2 mb-2">
                <HandInput
                  type="text"
                  placeholder="Label (optional)"
                  value={newLabel}
                  onChange={(e) => setNewLabel(e.target.value)}
                  className="flex-1"
                />
                <HandInput
                  type="text"
                  placeholder="URL or file path"
                  value={newURL}
                  onChange={(e) => setNewURL(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && handleAddIndex()}
                  className="flex-[2]"
                />
                <HandButton onClick={handleAddIndex} variant="primary" size="sm">
                  Save
                </HandButton>
              </div>
              <p className="text-xs text-muted-dark">
                Enter an HTTP(S) URL or local file path to a skillshare-hub.json file.
              </p>
            </div>
          )}

          <p className="text-xs text-muted-dark mt-1">
            Searching: <span className="font-medium break-all">{currentIndexLabel}</span>
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
            <HandInput
              type="text"
              placeholder={mode === 'github' ? 'Search GitHub for skills...' : 'Search index...'}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleSearch(query)}
              className="!pl-11"
            />
          </div>
          <HandButton
            onClick={() => handleSearch(query)}
            disabled={searching}
            variant="primary"
            size="md"
          >
            <Search size={16} strokeWidth={2.5} />
            {searching ? 'Searching...' : 'Search'}
          </HandButton>
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
          <p className="text-base text-pencil-light">
            {results.length} result{results.length !== 1 ? 's' : ''} found
          </p>
          {results.map((r, i) => (
            <Card
              key={r.source}
              className={i % 2 === 0 ? 'rotate-[-0.15deg]' : 'rotate-[0.15deg]'}
            >
              <div className="flex items-start justify-between gap-4">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1 flex-wrap">
                    <span
                      className="font-bold text-pencil"
                      style={{ fontFamily: 'var(--font-heading)' }}
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
                  <p
                    className="text-sm text-muted-dark truncate"
                    style={{ fontFamily: "'Courier New', monospace" }}
                  >
                    {r.source}
                  </p>
                </div>
                <HandButton
                  onClick={() => handleInstall(r.source)}
                  disabled={installing === r.source}
                  variant="secondary"
                  size="sm"
                  className="shrink-0"
                >
                  <Download size={14} strokeWidth={2.5} />
                  {installing === r.source ? 'Installing...' : 'Install'}
                </HandButton>
              </div>
            </Card>
          ))}
        </div>
      )}

      {results && results.length === 0 && (
        <EmptyState
          icon={Search}
          title="No results found"
          description={
            mode === 'github'
              ? 'Try different search terms or check your GITHUB_TOKEN.'
              : 'Try different search terms or check your index.'
          }
        />
      )}

      {/* Initial state before any search */}
      {!results && !searching && (
        <div className="text-center py-12">
          <div
            className="inline-flex items-center justify-center w-20 h-20 bg-postit border-2 border-dashed border-pencil-light mb-4"
            style={{ borderRadius: '50%' }}
          >
            <Search size={32} strokeWidth={2} className="text-pencil-light" />
          </div>
          <p
            className="text-xl text-pencil mb-1"
            style={{ fontFamily: 'var(--font-heading)' }}
          >
            Start searching
          </p>
          <p className="text-base text-pencil-light mb-4">
            {mode === 'github'
              ? 'Type a query above to find skills on GitHub'
              : 'Type a query above, or search with empty query to browse all'}
          </p>
          <HandButton
            onClick={() => handleSearch('')}
            variant="secondary"
            size="sm"
          >
            <Star size={14} strokeWidth={2.5} />
            {mode === 'github' ? 'Browse Popular Skills' : 'Browse All Skills'}
          </HandButton>
        </div>
      )}

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
