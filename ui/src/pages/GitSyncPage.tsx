import { useState } from 'react';
import { Link } from 'react-router-dom';
import {
  GitBranch,
  Globe,
  Folder,
  ArrowUpCircle,
  ArrowDownCircle,
  GitCommit,
  RefreshCw,
  AlertTriangle,
  CheckCircle,
  ChevronDown,
  ChevronRight,
} from 'lucide-react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import type { PullResponse } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { useAppContext } from '../context/AppContext';
import { shortenHome } from '../lib/paths';
import Card from '../components/Card';
import HandButton from '../components/HandButton';
import { HandInput, HandCheckbox } from '../components/HandInput';
import Badge from '../components/Badge';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';

function fileStatusBadge(line: string) {
  const code = line.trim().substring(0, 2).trim();
  if (code === 'M') return <Badge variant="warning">M</Badge>;
  if (code === 'A') return <Badge variant="success">A</Badge>;
  if (code === 'D') return <Badge variant="danger">D</Badge>;
  if (code === 'R') return <Badge variant="info">R</Badge>;
  if (code === '??') return <Badge variant="default">??</Badge>;
  return <Badge variant="default">{code}</Badge>;
}

function fileName(line: string): string {
  return line.trim().substring(2).trim();
}

export default function GitSyncPage() {
  const { isProjectMode } = useAppContext();
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const { data: status, isPending, error } = useQuery({
    queryKey: queryKeys.gitStatus,
    queryFn: () => api.gitStatus(),
    staleTime: staleTimes.gitStatus,
  });

  if (isProjectMode) {
    return (
      <div className="animate-sketch-in">
        <Card decoration="tape" className="text-center py-12">
          <GitBranch size={40} strokeWidth={2} className="text-pencil-light mx-auto mb-4" />
          <h2
            className="text-2xl font-bold text-pencil mb-2"
            style={{ fontFamily: 'var(--font-heading)' }}
          >
            Git Sync is not available in project mode
          </h2>
          <p className="text-pencil-light mb-4">
            Project skills are managed through your project's own version control.
          </p>
          <Link
            to="/"
            className="text-blue hover:underline"
            style={{ fontFamily: 'var(--font-hand)' }}
          >
            Back to Dashboard
          </Link>
        </Card>
      </div>
    );
  }

  const [commitMsg, setCommitMsg] = useState('');
  const [pushDryRun, setPushDryRun] = useState(false);
  const [pullDryRun, setPullDryRun] = useState(false);
  const [pushing, setPushing] = useState(false);
  const [pulling, setPulling] = useState(false);
  const [filesExpanded, setFilesExpanded] = useState(false);
  const [pushResult, setPushResult] = useState<string | null>(null);
  const [pullResult, setPullResult] = useState<PullResponse | null>(null);

  const disabled = !status?.isRepo || !status?.hasRemote;

  const handlePush = async () => {
    setPushing(true);
    setPushResult(null);
    try {
      const res = await api.push({ message: commitMsg || undefined, dryRun: pushDryRun });
      setPushResult(res.message);
      if (pushDryRun) {
        toast(res.message, 'info');
      } else {
        toast(res.message, 'success');
        setCommitMsg('');
      }
      queryClient.invalidateQueries({ queryKey: queryKeys.gitStatus });
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      setPushing(false);
    }
  };

  const handlePull = async () => {
    setPulling(true);
    setPullResult(null);
    try {
      const res = await api.pull({ dryRun: pullDryRun });
      setPullResult(res);
      if (pullDryRun) {
        toast(res.message || 'Dry run complete', 'info');
      } else if (res.upToDate) {
        toast('Already up to date', 'info');
      } else {
        const n = res.commits?.length ?? 0;
        toast(`Pulled ${n} commit(s) and synced`, 'success');
      }
      queryClient.invalidateQueries({ queryKey: queryKeys.gitStatus });
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
    } catch (e: any) {
      toast(e.message, 'error');
    } finally {
      setPulling(false);
    }
  };

  if (isPending) return <PageSkeleton />;

  if (error) {
    return (
      <Card>
        <p className="text-danger">{error.message}</p>
      </Card>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h2
          className="text-3xl font-bold text-pencil"
          style={{ fontFamily: 'var(--font-heading)' }}
        >
          Git Sync
        </h2>
        <p
          className="text-pencil-light mt-1"
          style={{ fontFamily: 'var(--font-hand)' }}
        >
          Push and pull your skills source directory via git
        </p>
      </div>

      {/* Repo Status Card */}
      <Card decoration="tape">
        <div className="space-y-3">
          {!status?.isRepo ? (
            <div className="flex items-center gap-2 text-pencil">
              <AlertTriangle size={18} strokeWidth={2.5} className="text-danger" />
              <span style={{ fontFamily: 'var(--font-hand)' }}>
                Source directory is not a git repository
              </span>
              <Badge variant="danger">not a repo</Badge>
            </div>
          ) : (
            <>
              <div className="flex items-center gap-3 flex-wrap">
                <div className="flex items-center gap-2">
                  <GitBranch size={16} strokeWidth={2.5} className="text-pencil-light" />
                  <span style={{ fontFamily: 'var(--font-hand)' }}>
                    Branch: <strong>{status.branch || 'unknown'}</strong>
                  </span>
                  {status.isDirty ? (
                    <Badge variant="warning">{status.files?.length ?? 0} dirty</Badge>
                  ) : (
                    <Badge variant="success">clean</Badge>
                  )}
                </div>
                <div className="flex items-center gap-2">
                  <Globe size={16} strokeWidth={2.5} className="text-pencil-light" />
                  <span style={{ fontFamily: 'var(--font-hand)' }}>Remote</span>
                  {status.hasRemote ? (
                    <Badge variant="success">connected</Badge>
                  ) : (
                    <Badge variant="danger">no remote</Badge>
                  )}
                </div>
              </div>
              <div className="flex items-center gap-2 text-sm text-pencil-light">
                <Folder size={14} strokeWidth={2.5} />
                <span
                  className="break-all"
                  style={{ fontFamily: "'Courier New', monospace" }}
                >
                  {shortenHome(status.sourceDir)}
                </span>
              </div>
            </>
          )}
        </div>
      </Card>

      {/* Push / Pull Grid */}
      <div
        className={`grid grid-cols-1 md:grid-cols-2 gap-6 ${disabled ? 'opacity-50 pointer-events-none' : ''}`}
      >
        {/* Push Section */}
        <Card variant="postit">
          <div className="space-y-4">
            <h3
              className="text-xl font-bold text-pencil flex items-center gap-2"
              style={{ fontFamily: 'var(--font-heading)' }}
            >
              <ArrowUpCircle size={20} strokeWidth={2.5} />
              Push Changes
            </h3>

            {/* Commit Message */}
            <HandInput
              label="Commit Message"
              placeholder="Describe your changes..."
              value={commitMsg}
              onChange={(e) => setCommitMsg(e.target.value)}
            />

            {/* Changed Files */}
            {status && status.files?.length > 0 && (
              <div>
                <button
                  className="flex items-center gap-1 text-sm text-pencil-light hover:text-pencil transition-colors"
                  onClick={() => setFilesExpanded(!filesExpanded)}
                  style={{ fontFamily: 'var(--font-hand)' }}
                >
                  {filesExpanded ? (
                    <ChevronDown size={14} strokeWidth={2.5} />
                  ) : (
                    <ChevronRight size={14} strokeWidth={2.5} />
                  )}
                  Changed Files ({status.files.length})
                </button>
                {filesExpanded && (
                  <div className="mt-2 space-y-1 p-2 border border-dashed border-pencil-light/40 rounded">
                    {status.files.map((f, i) => (
                      <div key={i} className="flex items-center gap-2 text-sm">
                        {fileStatusBadge(f)}
                        <span
                          className="truncate"
                          style={{ fontFamily: "'Courier New', monospace" }}
                        >
                          {fileName(f)}
                        </span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}

            {status && !status.isDirty && (
              <p
                className="text-sm text-pencil-light flex items-center gap-1"
                style={{ fontFamily: 'var(--font-hand)' }}
              >
                <CheckCircle size={14} strokeWidth={2.5} className="text-success" />
                Working tree clean
              </p>
            )}

            <div className="flex items-center justify-between gap-4 pt-2">
              <HandCheckbox
                label="Dry Run"
                checked={pushDryRun}
                onChange={setPushDryRun}
              />
              <HandButton
                variant="primary"
                size="lg"
                onClick={handlePush}
                disabled={pushing || (!status?.isDirty && !pushDryRun)}
                className="w-full md:w-auto"
              >
                {pushing ? (
                  <><RefreshCw size={18} strokeWidth={2.5} className="animate-spin" /> Pushing...</>
                ) : (
                  <><ArrowUpCircle size={18} strokeWidth={2.5} /> Push</>
                )}
              </HandButton>
            </div>

            {pushResult && (
              <p className="text-sm flex items-center gap-1 text-success">
                <CheckCircle size={14} strokeWidth={2.5} />
                {pushResult}
              </p>
            )}
          </div>
        </Card>

        {/* Pull Section */}
        <Card
          style={{ borderLeft: '3px solid var(--color-info, #2563eb)' }}
        >
          <div className="space-y-4">
            <h3
              className="text-xl font-bold text-pencil flex items-center gap-2"
              style={{ fontFamily: 'var(--font-heading)' }}
            >
              <ArrowDownCircle size={20} strokeWidth={2.5} />
              Pull Changes
            </h3>

            {status?.isDirty && (
              <p
                className="text-sm text-warning flex items-center gap-1"
                style={{ fontFamily: 'var(--font-hand)' }}
              >
                <AlertTriangle size={14} strokeWidth={2.5} />
                Commit or stash local changes before pulling
              </p>
            )}

            <div className="flex items-center justify-between gap-4 pt-2">
              <HandCheckbox
                label="Dry Run"
                checked={pullDryRun}
                onChange={setPullDryRun}
              />
              <HandButton
                variant="secondary"
                size="lg"
                onClick={handlePull}
                disabled={pulling || (!!status?.isDirty && !pullDryRun)}
                className="w-full md:w-auto"
              >
                {pulling ? (
                  <><RefreshCw size={18} strokeWidth={2.5} className="animate-spin" /> Pulling...</>
                ) : (
                  <><ArrowDownCircle size={18} strokeWidth={2.5} /> Pull</>
                )}
              </HandButton>
            </div>

            {/* Pull Results */}
            {pullResult && !pullResult.dryRun && !pullResult.upToDate && (
              <div className="space-y-2 border-t border-dashed border-pencil-light/40 pt-3">
                {pullResult.commits?.length > 0 && (
                  <div className="space-y-1">
                    {pullResult.commits.map((c, i) => (
                      <div key={i} className="flex items-center gap-2 text-sm">
                        <GitCommit size={14} strokeWidth={2.5} className="text-info" />
                        <code
                          className="text-info"
                          style={{ fontFamily: "'Courier New', monospace" }}
                        >
                          {c.hash}
                        </code>
                        <span className="truncate">{c.message}</span>
                      </div>
                    ))}
                  </div>
                )}
                {pullResult.stats && (
                  <p className="text-sm text-pencil-light">
                    <span className="text-success">+{pullResult.stats.insertions}</span>
                    {' '}
                    <span className="text-danger">-{pullResult.stats.deletions}</span>
                    {' across '}
                    {pullResult.stats.filesChanged} file(s)
                  </p>
                )}
                {pullResult.syncResults?.length > 0 && (
                  <p
                    className="text-sm text-pencil-light flex items-center gap-1"
                    style={{ fontFamily: 'var(--font-hand)' }}
                  >
                    <CheckCircle size={14} strokeWidth={2.5} className="text-success" />
                    Auto-synced to {pullResult.syncResults.length} target(s)
                  </p>
                )}
              </div>
            )}

            {pullResult && pullResult.upToDate && (
              <p className="text-sm text-pencil-light flex items-center gap-1">
                <CheckCircle size={14} strokeWidth={2.5} className="text-success" />
                Already up to date
              </p>
            )}
          </div>
        </Card>
      </div>
    </div>
  );
}
