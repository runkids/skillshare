import { useState } from 'react';
import { Search, Star, Download, Globe } from 'lucide-react';
import Card from '../components/Card';
import Badge from '../components/Badge';
import HandButton from '../components/HandButton';
import { HandInput } from '../components/HandInput';
import SkillPickerModal from '../components/SkillPickerModal';
import EmptyState from '../components/EmptyState';
import { useToast } from '../components/Toast';
import { api, type SearchResult, type DiscoveredSkill } from '../api/client';

export default function SearchPage() {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[] | null>(null);
  const [searching, setSearching] = useState(false);
  const { toast } = useToast();

  // Install state
  const [installing, setInstalling] = useState<string | null>(null);

  // Discovery flow state
  const [discoveredSkills, setDiscoveredSkills] = useState<DiscoveredSkill[]>([]);
  const [showPicker, setShowPicker] = useState(false);
  const [pendingSource, setPendingSource] = useState('');
  const [batchInstalling, setBatchInstalling] = useState(false);

  const handleSearch = async (searchQuery?: string) => {
    const q = searchQuery ?? query;
    setSearching(true);
    try {
      const res = await api.search(q);
      setResults(res.results);
      if (res.results.length === 0) {
        toast(q ? 'No results found. Try different search terms.' : 'No popular skills found.', 'info');
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
        // Multiple skills — open picker for selection
        setDiscoveredSkills(disc.skills);
        setPendingSource(source);
        setShowPicker(true);
      } else if (disc.skills.length === 1) {
        // Single discovered skill — install via batch (uses InstallFromDiscovery)
        const res = await api.installBatch({ source, skills: disc.skills });
        toast(res.summary, 'success');
        for (const item of res.results) {
          if (item.error) {
            toast(`${item.name}: ${item.error}`, 'error');
          }
          if (item.warnings?.length) {
            item.warnings.forEach((w) => toast(`${item.name}: ${w}`, 'warning'));
          }
        }
      } else {
        // No skills discovered (discovery failed or non-git) — direct install
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
      toast(res.summary, 'success');
      for (const item of res.results) {
        if (item.error) {
          toast(`${item.name}: ${item.error}`, 'error');
        }
        if (item.warnings?.length) {
          item.warnings.forEach((w) => toast(`${item.name}: ${w}`, 'warning'));
        }
      }
      setShowPicker(false);
    } catch (e: unknown) {
      toast((e as Error).message, 'error');
    } finally {
      setBatchInstalling(false);
    }
  };

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
          Discover and install skills from GitHub
        </p>
      </div>

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
              placeholder="Search GitHub for skills..."
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
        <p className="text-sm text-muted-dark mt-3 flex items-center gap-1">
          <Globe size={12} strokeWidth={2} />
          Requires GITHUB_TOKEN environment variable for GitHub API access.
        </p>
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
                    <span className="flex items-center gap-1 text-sm text-warning">
                      <Star size={14} strokeWidth={2.5} fill="currentColor" />
                      {r.stars}
                    </span>
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
          description="Try different search terms or check your GITHUB_TOKEN."
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
            Type a query above to find skills on GitHub
          </p>
          <HandButton
            onClick={() => handleSearch('')}
            variant="secondary"
            size="sm"
          >
            <Star size={14} strokeWidth={2.5} />
            Browse Popular Skills
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
