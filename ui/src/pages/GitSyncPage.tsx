import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useT } from '../i18n';
import ConfirmDialog from '../components/ConfirmDialog';
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
  Loader2,
  RefreshCw,
} from 'lucide-react';
import { useQuery, useQueryClient, useMutation } from '@tanstack/react-query';
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
import { Select } from '../components/Select';
import type { SelectOption } from '../components/Select';
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

function platformLabel(platform: Platform, t: (key: string) => string): string | null {
  switch (platform) {
    case 'github': return t('gitSync.platformLabel.github');
    case 'gitlab': return t('gitSync.platformLabel.gitlab');
    case 'bitbucket': return t('gitSync.platformLabel.bitbucket');
    default: return null;
  }
}

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

export default function GitSyncPage() {
  const t = useT();
  const { isProjectMode } = useAppContext();
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const { data: status, isPending, error } = useQuery({
    queryKey: queryKeys.gitStatus,
    queryFn: () => api.gitStatus(),
    staleTime: staleTimes.gitStatus,
  });


  const { data: branches } = useQuery({
    queryKey: queryKeys.gitBranches,
    queryFn: () => api.gitBranches(),
    staleTime: staleTimes.gitStatus,
    enabled: !isProjectMode && !!status?.isRepo,
  });

  const fetchBranchesMutation = useMutation({
    mutationFn: () => api.gitBranches({ fetch: true }),
    onSuccess: (data) => {
      queryClient.setQueryData(queryKeys.gitBranches, data);
      toast(t('gitSync.toast.branchListRefreshed'), 'info');
    },
    onError: (err: unknown) => {
      toast(errorMessage(err), 'error');
    },
  });

  const checkoutMutation = useMutation({
    mutationFn: (branch: string) => api.gitCheckout(branch),
    onSuccess: (res) => {
      toast(t('gitSync.toast.switchedTo', { branch: res.branch }), 'success');
      queryClient.invalidateQueries({ queryKey: queryKeys.gitStatus });
      queryClient.invalidateQueries({ queryKey: queryKeys.gitBranches });
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
    },
    onError: (err: unknown) => {
      toast(errorMessage(err), 'error');
    },
  });

  const [pendingScope, setPendingScope] = useState<string | null>(null);
  const [pendingRemote, setPendingRemote] = useState('');

  const setRootMutation = useMutation({
    mutationFn: ({ scope, remoteURL }: { scope: string; remoteURL?: string }) =>
      api.gitSetRoot(scope, remoteURL),
    onSuccess: (res) => {
      toast(t('gitSync.scope.switched', { scope: res.scope }), 'success');
      queryClient.invalidateQueries({ queryKey: queryKeys.gitStatus });
      queryClient.invalidateQueries({ queryKey: queryKeys.gitBranches });
      queryClient.invalidateQueries({ queryKey: queryKeys.config });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
    },
    onError: (err: unknown) => {
      toast(errorMessage(err), 'error');
    },
  });

  const [commitMsg, setCommitMsg] = useState('');
  const [pushDryRun, setPushDryRun] = useState(false);
  const [pullDryRun, setPullDryRun] = useState(false);
  const [committing, setCommitting] = useState(false);
  const [pushing, setPushing] = useState(false);
  const [pulling, setPulling] = useState(false);
  const [filesExpanded, setFilesExpanded] = useState(false);
  const [commitResult, setCommitResult] = useState<string | null>(null);
  const [pushResult, setPushResult] = useState<string | null>(null);
  const [pullResult, setPullResult] = useState<PullResponse | null>(null);

  const repoDisabled = !status?.isRepo;
  const remoteDisabled = !status?.hasRemote;

  // Build branch options for Select
  const branchOptions: SelectOption[] = [];
  if (branches) {
    for (const b of branches.local) {
      branchOptions.push({ value: b, label: b });
    }
    for (const b of branches.remote) {
      branchOptions.push({ value: b, label: `${b} (remote)`, description: 'Remote-only branch' });
    }
  }

  const handleCommit = async () => {
    setCommitting(true);
    setCommitResult(null);
    setPushResult(null);
    try {
      const res = await api.gitCommit({ message: commitMsg || undefined, dryRun: pushDryRun });
      setCommitResult(res.message);
      if (pushDryRun) {
        toast(res.message ?? '', 'info');
      } else {
        toast(res.message, 'success');
        setCommitMsg('');
      }
      queryClient.invalidateQueries({ queryKey: queryKeys.gitStatus });
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
    } catch (e: unknown) {
      toast(errorMessage(e), 'error');
    } finally {
      setCommitting(false);
    }
  };

  const handlePush = async () => {
    setPushing(true);
    setCommitResult(null);
    setPushResult(null);
    try {
      const res = await api.push({ message: commitMsg || undefined, dryRun: pushDryRun });
      setPushResult(res.message);
      if (pushDryRun) {
        toast(res.message ?? '', 'info');
      } else {
        toast(res.message, 'success');
        setCommitMsg('');
      }
      queryClient.invalidateQueries({ queryKey: queryKeys.gitStatus });
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
    } catch (e: unknown) {
      toast(errorMessage(e), 'error');
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
        toast(res.message || t('gitSync.pull.dryRunComplete'), 'info');
      } else if (res.upToDate) {
        toast(t('gitSync.toast.alreadyUpToDate'), 'info');
      } else {
        const n = res.commits?.length ?? 0;
        toast(t('gitSync.pull.pulled', { count: n }), 'success');
      }
      queryClient.invalidateQueries({ queryKey: queryKeys.gitStatus });
      queryClient.invalidateQueries({ queryKey: queryKeys.skills.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.overview });
    } catch (e: unknown) {
      toast(errorMessage(e), 'error');
    } finally {
      setPulling(false);
    }
  };

  if (isProjectMode) {
    return (
      <div className="space-y-5 animate-fade-in">
        <Card className="text-center py-12">
          <GitBranch size={40} strokeWidth={2} className="text-pencil-light mx-auto mb-4" />
          <h2
            className="text-2xl font-bold text-pencil mb-2"
          >
            {t('gitSync.projectMode.title')}
          </h2>
          <p className="text-pencil-light mb-4">
            {t('gitSync.projectMode.description')}
          </p>
          <Link
            to="/"
            className="text-blue hover:underline"
          >
            {t('common.back')}
          </Link>
        </Card>
      </div>
    );
  }

  if (isPending) {
    return (
      <div className="space-y-5 animate-fade-in">
        <PageHeader
          icon={<GitBranch size={24} strokeWidth={2.5} />}
          title={t('gitSync.title')}
          subtitle={t('gitSync.subtitle')}
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
          title={t('gitSync.title')}
          subtitle={t('gitSync.subtitle')}
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
        title={t('gitSync.title')}
        subtitle={t('gitSync.subtitle')}
      />

      {/* Repository Info Card — z-10 so branch dropdown renders above cards below */}
      <Card overflow className="relative z-10" padding="none">
        {!status?.isRepo ? (
          status && !status.gitInstalled ? (
            <div className="flex items-start gap-2 text-pencil p-4">
              <AlertTriangle size={18} strokeWidth={2.5} className="text-danger shrink-0 mt-0.5" />
              <div className="space-y-1 text-sm">
                <span className="font-bold">{t('gitSync.gitNotInstalled.title')}</span>
                <p className="text-pencil-light">{t('gitSync.gitNotInstalled.hint')}</p>
              </div>
            </div>
          ) : status?.scopeMismatch ? (
            <div className="flex items-start gap-2 text-pencil p-4">
              <AlertTriangle size={18} strokeWidth={2.5} className="text-warning shrink-0 mt-0.5" />
              <div className="space-y-1 text-sm">
                <div className="flex items-center gap-2">
                  <span className="font-bold">{t('gitSync.mismatch.title')}</span>
                  <Badge variant="warning">{status.mismatchScope}</Badge>
                </div>
                <p className="text-pencil-light">
                  {t('gitSync.mismatch.description', {
                    scope: status.scope || 'skills',
                    repoScope: status.mismatchScope ?? '',
                  })}
                </p>
                <p className="text-pencil-light font-mono text-xs truncate max-w-[480px]">
                  {status.mismatchDir}
                </p>
                <p className="text-pencil-light">{t('gitSync.mismatch.hint')}</p>
              </div>
            </div>
          ) : (
            <div className="p-4 space-y-3">
              <div className="flex items-center gap-2 text-pencil">
                <AlertTriangle size={18} strokeWidth={2.5} className="text-danger" />
                <span>{t('gitSync.notARepo')}</span>
                <Badge variant="danger">{t('gitSync.repo.notARepoLabel')}</Badge>
              </div>
              <Button
                variant="primary"
                size="sm"
                onClick={() => {
                  setPendingRemote('');
                  setPendingScope(status?.scope || 'skills');
                }}
              >
                {t('gitSync.scope.initButton')}
              </Button>
            </div>
          )
        ) : (() => {
          const parsed = parseRemoteURL(status.remoteURL);
          const linkLabel = parsed ? platformLabel(parsed.platform, t) : null;
          return (
            <>
              {/* ── Header: repo identity ── */}
              <div className="px-4 pt-4 pb-3 space-y-1.5">
                {status.hasRemote && status.remoteURL ? (
                  parsed ? (
                    <div className="flex items-center gap-2 flex-wrap">
                      {platformIcon(parsed.platform)}
                      <span className="font-bold text-pencil text-base">{parsed.ownerRepo}</span>
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
                      {status.isDirty ? (
                        <Badge variant="warning">{status.files?.length ?? 0} dirty</Badge>
                      ) : (
                        <Badge variant="success">clean</Badge>
                      )}
                    </div>
                  ) : (
                    <div className="flex items-center gap-2">
                      <GitBranch size={16} strokeWidth={2.5} />
                      <span className="font-bold text-pencil">{status.remoteURL}</span>
                    </div>
                  )
                ) : (
                  <div className="flex items-center gap-2">
                    <GitBranch size={16} strokeWidth={2.5} />
                    <span className="font-bold text-pencil">{t('gitSync.repo.localRepository')}</span>
                    <Badge variant="danger">{t('gitSync.repo.noRemote')}</Badge>
                  </div>
                )}

                {/* Raw URL — compact inline with copy */}
                {status.hasRemote && status.remoteURL && (
                  <div className="flex items-center gap-1 text-xs text-pencil-light">
                    <span className="font-mono truncate max-w-[400px]">{status.remoteURL}</span>
                    <CopyButton value={status.remoteURL} title="Copy remote URL" />
                  </div>
                )}
              </div>

              {/* ── Scope row ── */}
              <div className="px-4 py-2 border-t border-dashed border-pencil-light/20 flex items-center gap-3 text-sm">
                <span className="text-pencil-light">{t('gitSync.scope.label')}</span>
                <Select
                  value={status.scope || 'skills'}
                  onChange={(val) => {
                    if (val !== status.scope) {
                      setPendingRemote('');
                      setPendingScope(val);
                    }
                  }}
                  options={[
                    { value: 'skills', label: 'skills' },
                    { value: 'agents', label: 'agents' },
                    { value: 'extras', label: 'extras' },
                    { value: 'root', label: 'root' },
                  ]}
                  size="sm"
                  disabled={setRootMutation.isPending}
                  className="min-w-[120px]"
                />
              </div>

              {/* ── Status bar: branch / HEAD / tracking ── */}
              <div className="px-4 py-2.5 border-t border-dashed border-pencil-light/20 bg-muted/30 flex items-center gap-x-5 gap-y-2 flex-wrap text-sm">
                {/* Branch */}
                <div className="flex items-center gap-2">
                  <GitBranch size={14} strokeWidth={2.5} className="text-pencil-light" />
                  {branchOptions.length > 1 ? (
                    <>
                      <Select
                        value={status.branch || ''}
                        onChange={(val) => {
                          if (val !== status.branch) {
                            checkoutMutation.mutate(val);
                          }
                        }}
                        options={branchOptions}
                        size="sm"
                        disabled={!!branches?.isDirty || checkoutMutation.isPending}
                        className="min-w-[140px]"
                      />
                      <button
                        type="button"
                        title={t('gitSync.branch.fetchRemote')}
                        disabled={fetchBranchesMutation.isPending}
                        onClick={() => fetchBranchesMutation.mutate()}
                        className="p-1 rounded text-pencil-light hover:text-pencil hover:bg-muted/60 transition-colors disabled:opacity-50 cursor-pointer"
                      >
                        <RefreshCw size={14} strokeWidth={2.5} className={fetchBranchesMutation.isPending ? 'animate-spin' : ''} />
                      </button>
                      {checkoutMutation.isPending && (
                        <Loader2 size={14} className="animate-spin text-pencil-light" />
                      )}
                    </>
                  ) : (
                    <>
                      <strong>{status.branch || 'unknown'}</strong>
                      {status.hasRemote && (
                        <button
                          type="button"
                          title={t('gitSync.branch.fetchRemote')}
                          disabled={fetchBranchesMutation.isPending}
                          onClick={() => fetchBranchesMutation.mutate()}
                          className="p-1 rounded text-pencil-light hover:text-pencil hover:bg-muted/60 transition-colors disabled:opacity-50 cursor-pointer"
                        >
                          <RefreshCw size={14} strokeWidth={2.5} className={fetchBranchesMutation.isPending ? 'animate-spin' : ''} />
                        </button>
                      )}
                    </>
                  )}
                  {status.trackingBranch && (
                    <span className="text-pencil-light">→ {status.trackingBranch}</span>
                  )}
                </div>

                {/* Separator */}
                {status.headHash && <span className="hidden sm:inline text-pencil-light/30">|</span>}

                {/* HEAD */}
                {status.headHash && (
                  <div className="flex items-center gap-1.5">
                    <GitCommit size={14} strokeWidth={2.5} className="text-pencil-light" />
                    <code className="font-mono text-info">{status.headHash}</code>
                    {status.headMessage && (
                      <span className="text-pencil-light truncate max-w-[260px]" title={status.headMessage}>
                        {status.headMessage.length > 50
                          ? status.headMessage.slice(0, 50) + '…'
                          : status.headMessage}
                      </span>
                    )}
                  </div>
                )}
              </div>
            </>
          );
        })()}
      </Card>

      <ConfirmDialog
        open={pendingScope !== null}
        onCancel={() => {
          setPendingScope(null);
          setPendingRemote('');
        }}
        onConfirm={() => {
          if (pendingScope) {
            setRootMutation.mutate({ scope: pendingScope, remoteURL: pendingRemote.trim() || undefined });
          }
          setPendingScope(null);
          setPendingRemote('');
        }}
        title={t(status?.isRepo ? 'gitSync.scope.confirmTitle' : 'gitSync.scope.initTitle')}
        message={
          <div className="space-y-3">
            <p>
              {t(status?.isRepo ? 'gitSync.scope.confirmMessage' : 'gitSync.scope.initMessage', {
                scope: pendingScope ?? '',
              })}
            </p>
            <Input
              label={t('gitSync.scope.remoteLabel')}
              value={pendingRemote}
              onChange={(e) => setPendingRemote(e.target.value)}
              placeholder="git@github.com:user/skills.git"
              disabled={setRootMutation.isPending}
            />
            <p className="text-xs text-muted-dark">{t('gitSync.scope.remoteHint')}</p>
          </div>
        }
        loading={setRootMutation.isPending}
        variant="default"
      />

      {/* Push / Pull Actions */}
      <Card className={repoDisabled ? 'opacity-50 pointer-events-none' : ''} padding="none">
        <div data-tour="git-actions" className="grid grid-cols-1 md:grid-cols-2">
          {/* Push Section */}
          <div className="p-4 flex flex-col">
            <div className="space-y-4 flex-1">
              <h3 className="text-xl font-bold text-pencil flex items-center gap-2">
                <ArrowUpCircle size={20} strokeWidth={2.5} />
                {t('gitSync.push.title')}
              </h3>

              <Input
                label={t('gitSync.commit.message')}
                placeholder={t('gitSync.commit.placeholder')}
                value={commitMsg}
                onChange={(e) => setCommitMsg(e.target.value)}
              />

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
                    {t('gitSync.files.changedFiles', { count: status.files.length })}
                  </button>
                  {filesExpanded && (
                    <div className="mt-2 space-y-1 p-2 border border-dashed border-pencil-light/30 rounded">
                      {status.files.map((f, i) => (
                        <div key={i} className="flex items-center gap-2 text-sm">
                          {fileStatusBadge(f)}
                          <span className="font-mono truncate">{fileName(f)}</span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}

              {status && !status.isDirty && !commitResult && !pushResult && (
                <p className="text-sm text-pencil-light">
                  {t('gitSync.noUncommitted')}
                </p>
              )}

              {(commitResult || pushResult) && (
                <p className="text-sm flex items-center gap-1 text-success">
                  <CheckCircle size={14} strokeWidth={2.5} />
                  {commitResult || pushResult}
                </p>
              )}
            </div>

            <div className="space-y-3 mt-4 border-t border-dashed border-pencil-light/20 pt-3">
              <div className="flex items-center justify-between gap-4">
                <Checkbox label={t('gitSync.dryRun')} checked={pushDryRun} onChange={setPushDryRun} />
                <div className="flex flex-wrap justify-end gap-2">
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={handleCommit}
                    loading={committing}
                    disabled={(!status?.isDirty && !pushDryRun) || pushing}
                  >
                    {!committing && <GitCommit size={16} strokeWidth={2.5} />}
                    {committing ? t('gitSync.actions.committing') : t('gitSync.actions.commit')}
                  </Button>
                  <Button
                    variant="primary"
                    size="sm"
                    onClick={handlePush}
                    loading={pushing}
                    disabled={remoteDisabled || (!status?.isDirty && !pushDryRun) || committing}
                  >
                    {!pushing && <ArrowUpCircle size={16} strokeWidth={2.5} />}
                    {pushing ? t('gitSync.actions.pushing') : t('gitSync.actions.push')}
                  </Button>
                </div>
              </div>
            </div>
          </div>

          {/* Divider */}
          <div className="border-t md:border-t-0 md:border-l border-dashed border-pencil-light/20 p-4 flex flex-col">
            {/* Pull Section */}
            <div className="space-y-4 flex-1">
              <h3 className="text-xl font-bold text-pencil flex items-center gap-2">
                <ArrowDownCircle size={20} strokeWidth={2.5} />
                {t('gitSync.pull.title')}
              </h3>

              {status?.isDirty ? (
                <p className="text-sm text-warning flex items-center gap-1">
                  <AlertTriangle size={14} strokeWidth={2.5} />
                  {t('gitSync.pull.commitOrStash')}
                </p>
              ) : (
                <p className="text-sm text-pencil-light">
                  {t('gitSync.pull.fetchLatest')}
                </p>
              )}

              {/* Pull Results — in content area, above the action footer */}
              {pullResult && !pullResult.dryRun && !pullResult.upToDate && (
                <div className="space-y-2 border-t border-dashed border-pencil-light/30 pt-3">
                  {pullResult.commits?.length > 0 && (
                    <div className="space-y-1">
                      {pullResult.commits.map((c, i) => (
                        <div key={i} className="flex items-center gap-2 text-sm">
                          <GitCommit size={14} strokeWidth={2.5} className="text-info" />
                          <code className="font-mono text-info">{c.hash}</code>
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
                    <p className="text-sm text-pencil-light flex items-center gap-1">
                      <CheckCircle size={14} strokeWidth={2.5} className="text-success" />
                      {t('gitSync.pull.autoSynced', { count: pullResult.syncResults.length })}
                    </p>
                  )}
                </div>
              )}

              {pullResult && pullResult.upToDate && (
                <p className="text-sm text-pencil-light flex items-center gap-1">
                  <CheckCircle size={14} strokeWidth={2.5} className="text-success" />
                  {t('gitSync.pull.alreadyUpToDate')}
                </p>
              )}
            </div>

            <div className="space-y-3 mt-4 border-t border-dashed border-pencil-light/20 pt-3">
              <div className="flex items-center justify-between gap-4">
                <Checkbox label={t('gitSync.dryRun')} checked={pullDryRun} onChange={setPullDryRun} />
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={handlePull}
                  loading={pulling}
                  disabled={remoteDisabled || (!!status?.isDirty && !pullDryRun)}
                >
                  {!pulling && <ArrowDownCircle size={16} strokeWidth={2.5} />}
                  {pulling ? t('gitSync.actions.pulling') : t('gitSync.actions.pull')}
                </Button>
              </div>
            </div>
          </div>
        </div>
      </Card>
    </div>
  );
}
