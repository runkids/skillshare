import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { RefreshCw, Check, ArrowUpCircle, Loader2 } from 'lucide-react';
import Card from '../components/Card';
import HandButton from '../components/HandButton';
import { HandCheckbox } from '../components/HandInput';
import Badge from '../components/Badge';
import EmptyState from '../components/EmptyState';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { api } from '../api/client';
import { wobbly } from '../design';

export default function UpdatePage() {
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const [selectedRepos, setSelectedRepos] = useState<Set<string>>(new Set());
  const [selectedSkills, setSelectedSkills] = useState<Set<string>>(new Set());
  const [updating, setUpdating] = useState(false);

  const checkQuery = useQuery({
    queryKey: queryKeys.check,
    queryFn: () => api.check(),
    staleTime: staleTimes.check,
  });

  const { data, isPending, error } = checkQuery;

  if (isPending) return <PageSkeleton />;
  if (error) return <Card variant="accent"><p className="text-danger p-2" style={{ fontFamily: 'var(--font-hand)' }}>{(error as Error).message}</p></Card>;
  if (!data) return null;

  const updatableRepos = data.tracked_repos.filter((r) => r.status === 'behind');
  const updatableSkills = data.skills.filter((s) => s.status === 'update_available');
  const hasUpdates = updatableRepos.length > 0 || updatableSkills.length > 0;

  const toggleRepo = (name: string) => {
    setSelectedRepos((prev) => {
      const next = new Set(prev);
      next.has(name) ? next.delete(name) : next.add(name);
      return next;
    });
  };

  const toggleSkill = (name: string) => {
    setSelectedSkills((prev) => {
      const next = new Set(prev);
      next.has(name) ? next.delete(name) : next.add(name);
      return next;
    });
  };

  const selectAllRepos = () => {
    setSelectedRepos((prev) =>
      prev.size === updatableRepos.length ? new Set() : new Set(updatableRepos.map((r) => r.name))
    );
  };

  const selectAllSkills = () => {
    setSelectedSkills((prev) =>
      prev.size === updatableSkills.length ? new Set() : new Set(updatableSkills.map((s) => s.name))
    );
  };

  const totalSelected = selectedRepos.size + selectedSkills.size;

  const handleUpdate = async () => {
    if (totalSelected === 0) return;
    setUpdating(true);
    try {
      const names = [...selectedRepos, ...selectedSkills];
      for (const name of names) {
        await api.update({ name });
      }
      toast(`Updated ${names.length} item(s)`, 'success');
      setSelectedRepos(new Set());
      setSelectedSkills(new Set());
      queryClient.invalidateQueries({ queryKey: queryKeys.check });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
    } catch (e) {
      toast((e as Error).message, 'error');
    } finally {
      setUpdating(false);
    }
  };

  const upToDateRepos = data.tracked_repos.filter((r) => r.status === 'up_to_date').length;
  const upToDateSkills = data.skills.filter((s) => s.status === 'up_to_date').length;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div>
          <h1 className="text-2xl font-bold text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
            Updates
          </h1>
          <p className="text-pencil-light text-sm mt-1" style={{ fontFamily: 'var(--font-hand)' }}>
            Check and apply updates for tracked repositories and installed skills.
          </p>
        </div>
        <div className="flex gap-2">
          <HandButton
            variant="ghost"
            size="sm"
            onClick={() => queryClient.invalidateQueries({ queryKey: queryKeys.check })}
            disabled={checkQuery.isFetching}
          >
            <RefreshCw size={16} className={checkQuery.isFetching ? 'animate-spin' : ''} />
            Check Now
          </HandButton>
          {totalSelected > 0 && (
            <HandButton onClick={handleUpdate} disabled={updating}>
              {updating ? <Loader2 size={16} className="animate-spin" /> : <ArrowUpCircle size={16} />}
              Update Selected ({totalSelected})
            </HandButton>
          )}
        </div>
      </div>

      {!hasUpdates ? (
        <EmptyState
          icon={Check}
          title="Everything is up to date"
          description="All tracked repositories and skills are at their latest versions."
        />
      ) : (
        <>
          {updatableRepos.length > 0 && (
            <Card>
              <div className="flex items-center justify-between mb-3">
                <h2 className="text-lg font-bold text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
                  Tracked Repositories ({updatableRepos.length})
                </h2>
                <HandButton variant="ghost" size="sm" onClick={selectAllRepos}>
                  {selectedRepos.size === updatableRepos.length ? 'Deselect All' : 'Select All'}
                </HandButton>
              </div>
              <div className="space-y-2">
                {updatableRepos.map((repo) => (
                  <div
                    key={repo.name}
                    className="flex items-center gap-3 px-3 py-2 border border-pencil/10"
                    style={{ borderRadius: wobbly.sm }}
                  >
                    <HandCheckbox label="" checked={selectedRepos.has(repo.name)} onChange={() => toggleRepo(repo.name)} />
                    <span className="text-pencil font-medium flex-1" style={{ fontFamily: 'var(--font-hand)' }}>
                      {repo.name}
                    </span>
                    <Badge variant="warning">{repo.behind} commit(s) behind</Badge>
                  </div>
                ))}
              </div>
            </Card>
          )}

          {updatableSkills.length > 0 && (
            <Card>
              <div className="flex items-center justify-between mb-3">
                <h2 className="text-lg font-bold text-pencil" style={{ fontFamily: 'var(--font-heading)' }}>
                  Skills with Updates ({updatableSkills.length})
                </h2>
                <HandButton variant="ghost" size="sm" onClick={selectAllSkills}>
                  {selectedSkills.size === updatableSkills.length ? 'Deselect All' : 'Select All'}
                </HandButton>
              </div>
              <div className="space-y-2">
                {updatableSkills.map((skill) => (
                  <div
                    key={skill.name}
                    className="flex items-center gap-3 px-3 py-2 border border-pencil/10"
                    style={{ borderRadius: wobbly.sm }}
                  >
                    <HandCheckbox label="" checked={selectedSkills.has(skill.name)} onChange={() => toggleSkill(skill.name)} />
                    <div className="flex-1 min-w-0">
                      <span className="text-pencil font-medium block" style={{ fontFamily: 'var(--font-hand)' }}>
                        {skill.name}
                      </span>
                      {skill.source && (
                        <span className="text-pencil-light text-sm truncate block">{skill.source}</span>
                      )}
                    </div>
                    <Badge variant="info">Update available</Badge>
                  </div>
                ))}
              </div>
            </Card>
          )}
        </>
      )}

      {(upToDateRepos + upToDateSkills > 0) && (
        <Card variant="outlined">
          <p className="text-pencil-light text-sm" style={{ fontFamily: 'var(--font-hand)' }}>
            {upToDateRepos} repo(s) and {upToDateSkills} skill(s) already up to date.
          </p>
        </Card>
      )}
    </div>
  );
}
