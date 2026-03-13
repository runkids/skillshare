import { useState } from 'react';
import { Link } from 'react-router-dom';
import {
  GitBranch,
  ArrowUpCircle,
  ArrowDownCircle,
  GitCommit,
  AlertTriangle,
  CheckCircle,
  ChevronDown,
  ChevronRight,
  Github,
  Gitlab,
  ExternalLink,
} from 'lucide-react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import type { PullResponse } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { useAppContext } from '../context/AppContext';
import { parseRemoteURL } from '../lib/parseRemoteURL';
import type { Platform } from '../lib/parseRemoteURL';
import Card from '../components/Card';
import Button from '../components/Button';
import CopyButton from '../components/CopyButton';
import { Input, Checkbox } from '../components/Input';
import Badge from '../components/Badge';
import PageHeader from '../components/PageHeader';
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

function platformIcon(platform: Platform) {
  switch (platform) {
    case 'github':
      return <Github size={16} strokeWidth={2.5} />;
    case 'gitlab':
      return <Gitlab size={16} strokeWidth={2.5} />;
    default:
      return <GitBranch size={16} strokeWidth={2.5} />;
  }
}

function platformLabel(platform: Platform): string | null {
  switch (platform) {
    case 'github': return 'Open on GitHub';
    case 'gitlab': return 'Open on GitLab';
    case 'bitbucket': return 'Open on Bitbucket';
    default: return null;
  }
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
      <div className="space-y-5 animate-fade-in">
        <Card className="text-center py-12">
          <GitBranch size={40} strokeWidth={2} className="text-pencil-light mx-auto mb-4" />
          <h2
            className="text-2xl font-bold text-pencil mb-2"
          >
            Git Sync is not available in project mode
          </h2>
          <p className="text-pencil-light mb-4">
            Project skills are managed through your project's own version control.
          </p>
          <Link
            to="/"
            className="text-blue hover:underline"
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

  if (isPending) {
    return (
      <div className="space-y-5 animate-fade-in">
        <PageHeader
          icon={<GitBranch size={24} strokeWidth={2.5} />}
          title="Git Sync"
          subtitle="Push and pull your skills source directory via git"
        />
        <PageSkeleton />
      </div>
    );
  }

  if (error) {
    return (
      <div className="space-y-5 animate-fade-in">
        <PageHeader
          icon={<GitBranch size={24} strokeWidth={2.5} />}
          title="Git Sync"
          subtitle="Push and pull your skills source directory via git"
        />
        <Card variant="accent">
          <p className="text-danger">{error.message}</p>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-5 animate-fade-in">
      {/* Header */}
      <PageHeader
        icon={<GitBranch size={24} strokeWidth={2.5} />}
        title="Git Sync"
        subtitle="Push and pull your skills source directory via git"
      />

      {/* Repository Info Card */}
      <Card>
        <div className="space-y-3">
          {!status?.isRepo ? (
            <div className="flex items-center gap-2 text-pencil">
              <AlertTriangle size={18} strokeWidth={2.5} className="text-danger" />
              <span>Source directory is not a git repository</span>
              <Badge variant="danger">not a repo</Badge>
            </div>
          ) : (() => {
            const parsed = parseRemoteURL(status.remoteURL);
            const linkLabel = parsed ? platformLabel(parsed.platform) : null;
            return (
              <>
                {/* Remote URL section */}
                {status.hasRemote && status.remoteURL && (
                  <div className="space-y-1">
                    {parsed ? (
                      <div className="flex items-center gap-2 flex-wrap">
                        {platformIcon(parsed.platform)}
                        <span className="font-bold text-pencil">{parsed.ownerRepo}</span>
                        {parsed.webURL && linkLabel && (
                          <a
                            href={parsed.webURL}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="inline-flex items-center gap-1 text-sm text-blue hover:underline"
                          >
                            {linkLabel}
                            <ExternalLink size={12} strokeWidth={2.5} />
                          </a>
                        )}
                      </div>
                    ) : (
                      <div className="flex items-center gap-2">
                        <GitBranch size={16} strokeWidth={2.5} />
                        <span className="font-bold text-pencil">{status.remoteURL}</span>
                      </div>
                    )}
                    <div className="flex items-center gap-1 text-sm text-pencil-light">
                      <span className="font-mono break-all">{status.remoteURL}</span>
                      <CopyButton value={status.remoteURL} title="Copy remote URL" />
                    </div>
                  </div>
                )}

                {/* Branch / HEAD / Status */}
                <div className="flex items-center gap-x-6 gap-y-2 flex-wrap text-sm">
                  <div className="flex items-center gap-2">
                    <span className="text-pencil-light">Branch</span>
                    <strong>{status.branch || 'unknown'}</strong>
                    {status.trackingBranch && (
                      <span className="text-pencil-light">→ {status.trackingBranch}</span>
                    )}
                  </div>

                  {status.headHash && (
                    <div className="flex items-center gap-2">
                      <span className="text-pencil-light">HEAD</span>
                      <code className="font-mono text-info">{status.headHash}</code>
                      {status.headMessage && (
                        <span className="text-pencil-light truncate max-w-[300px]" title={status.headMessage}>
                          {status.headMessage.length > 60
                            ? status.headMessage.slice(0, 60) + '…'
                            : status.headMessage}
                        </span>
                      )}
                    </div>
                  )}
                </div>

                {/* Status badges */}
                <div className="flex items-center gap-4 flex-wrap">
                  <div className="flex items-center gap-2">
                    <span className="text-sm text-pencil-light">Status</span>
                    {status.isDirty ? (
                      <Badge variant="warning">{status.files?.length ?? 0} dirty</Badge>
                    ) : (
                      <Badge variant="success">clean</Badge>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="text-sm text-pencil-light">Remote</span>
                    {status.hasRemote ? (
                      <Badge variant="success">connected</Badge>
                    ) : (
                      <Badge variant="danger">no remote</Badge>
                    )}
                  </div>
                </div>
              </>
            );
          })()}
        </div>
      </Card>

      {/* Push / Pull Grid */}
      <div
        data-tour="git-actions"
        className={`grid grid-cols-1 md:grid-cols-2 gap-5 ${disabled ? 'opacity-50 pointer-events-none' : ''}`}
      >
        {/* Push Section */}
        <Card>
          <div className="space-y-4">
            <h3
              className="text-xl font-bold text-pencil flex items-center gap-2"
            >
              <ArrowUpCircle size={20} strokeWidth={2.5} />
              Push Changes
            </h3>

            {/* Commit Message */}
            <Input
              label="Commit Message"
              placeholder="Describe your changes..."
              value={commitMsg}
              onChange={(e) => setCommitMsg(e.target.value)}
            />

            {/* Changed Files */}
            {status && status.files?.length > 0 && (
              <div>
                <button
                  className="flex items-center gap-1 text-sm text-pencil-light hover:text-pencil transition-colors cursor-pointer"
                  onClick={() => setFilesExpanded(!filesExpanded)}
                >
                  {filesExpanded ? (
                    <ChevronDown size={14} strokeWidth={2.5} />
                  ) : (
                    <ChevronRight size={14} strokeWidth={2.5} />
                  )}
                  Changed Files ({status.files.length})
                </button>
                {filesExpanded && (
                  <div className="mt-2 space-y-1 p-2 border border-dashed border-pencil-light/30 rounded">
                    {status.files.map((f, i) => (
                      <div key={i} className="flex items-center gap-2 text-sm">
                        {fileStatusBadge(f)}
                        <span
                          className="font-mono truncate"
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
              >
                <CheckCircle size={14} strokeWidth={2.5} className="text-success" />
                Working tree clean
              </p>
            )}

            <div className="flex items-center justify-between gap-4 pt-2">
              <Checkbox
                label="Dry Run"
                checked={pushDryRun}
                onChange={setPushDryRun}
              />
              <Button
                variant="primary"
                size="sm"
                onClick={handlePush}
                loading={pushing}
                disabled={!status?.isDirty && !pushDryRun}
              >
                {!pushing && <ArrowUpCircle size={16} strokeWidth={2.5} />}
                {pushing ? 'Pushing...' : 'Push'}
              </Button>
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
        <Card>
          <div className="space-y-4">
            <h3
              className="text-xl font-bold text-pencil flex items-center gap-2"
            >
              <ArrowDownCircle size={20} strokeWidth={2.5} />
              Pull Changes
            </h3>

            {status?.isDirty && (
              <p
                className="text-sm text-warning flex items-center gap-1"
              >
                <AlertTriangle size={14} strokeWidth={2.5} />
                Commit or stash local changes before pulling
              </p>
            )}

            <div className="flex items-center justify-between gap-4 pt-2">
              <Checkbox
                label="Dry Run"
                checked={pullDryRun}
                onChange={setPullDryRun}
              />
              <Button
                variant="secondary"
                size="sm"
                onClick={handlePull}
                loading={pulling}
                disabled={!!status?.isDirty && !pullDryRun}
              >
                {!pulling && <ArrowDownCircle size={16} strokeWidth={2.5} />}
                {pulling ? 'Pulling...' : 'Pull'}
              </Button>
            </div>

            {/* Pull Results */}
            {pullResult && !pullResult.dryRun && !pullResult.upToDate && (
              <div className="space-y-2 border-t border-dashed border-pencil-light/30 pt-3">
                {pullResult.commits?.length > 0 && (
                  <div className="space-y-1">
                    {pullResult.commits.map((c, i) => (
                      <div key={i} className="flex items-center gap-2 text-sm">
                        <GitCommit size={14} strokeWidth={2.5} className="text-info" />
                        <code
                          className="font-mono text-info"
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
