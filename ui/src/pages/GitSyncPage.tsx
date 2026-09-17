import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertCircle, AlertTriangle, ArrowDownToLine, ArrowUpFromLine, CircleCheck, CloudUpload, ExternalLink, FolderGit2, GitBranch, GitCommitHorizontal, Info, RefreshCw, X } from 'lucide-react';
import { api, ApiError, type GitStatus, type PullResponse } from '../api/client';
import Button from '../components/Button';
import ConfirmDialog from '../components/ConfirmDialog';
import DialogShell from '../components/DialogShell';
import EmptyState from '../components/EmptyState';
import { Input } from '../components/Input';
import PageHeader from '../components/PageHeader';
import { Select } from '../components/Select';
import { PageSkeleton } from '../components/Skeleton';
import { useToast } from '../components/Toast';
import { parseStatusLine } from '../components/git/gitView';
import { useAppContext } from '../context/AppContext';
import { useT } from '../i18n';
import { parseRemoteURL } from '../lib/parseRemoteURL';
import { queryKeys, staleTimes } from '../lib/queryKeys';

const SCOPES = ['skills', 'agents', 'extras', 'root'];
const TONE = { New: 'ok', Changed: 'warn', Renamed: 'warn', Deleted: 'bad' } as const;
const PULLED_SHOWN = 5;
type Setup = { kind: 'init' | 'scope' | 'remote'; scope: string };

export default function GitSyncPage() {
  const t = useT();
  const { isProjectMode } = useAppContext();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const { data: status, isPending, error } = useQuery({ queryKey: queryKeys.gitStatus, queryFn: () => api.gitStatus(), staleTime: staleTimes.gitStatus, enabled: !isProjectMode });
  const branches = useQuery({ queryKey: queryKeys.gitBranches, queryFn: () => api.gitBranches(), staleTime: staleTimes.gitStatus, enabled: !isProjectMode && !!status?.isRepo });

  const [message, setMessage] = useState('');
  const [dryRun, setDryRun] = useState(false);
  const [busy, setBusy] = useState<'commit' | 'push' | 'upload' | 'pull' | 'branch' | 'fetch' | 'nested' | 'scope' | null>(null);
  const [runError, setRunError] = useState('');
  // A first pull whose history cannot merge; the error note then offers a force pull.
  const [mergeFailed, setMergeFailed] = useState(false);
  const [confirmForce, setConfirmForce] = useState(false);
  const [note, setNote] = useState('');
  const [pulled, setPulled] = useState<PullResponse | null>(null);
  const [setup, setSetup] = useState<Setup | null>(null);

  const refresh = () => {
    for (const queryKey of [queryKeys.gitStatus, queryKeys.gitBranches, queryKeys.skills.all, queryKeys.overview, queryKeys.config, queryKeys.targets.all, queryKeys.diff()]) {
      void queryClient.invalidateQueries({ queryKey });
    }
  };
  const run = async (kind: NonNullable<typeof busy>, work: () => Promise<void>) => {
    setBusy(kind);
    setRunError('');
    setMergeFailed(false);
    setNote('');
    try {
      await work();
    } catch (err) {
      setRunError((err as Error).message);
      setMergeFailed(err instanceof ApiError && err.code === 'merge_failed');
    } finally {
      setBusy(null);
      refresh();
    }
  };

  const commit = (push: boolean) => run(push ? 'push' : 'commit', async () => {
    const res = await (push ? api.push : api.gitCommit)({ message: message.trim() || undefined, dryRun });
    // Dry runs and a clean tree come back as plain server messages.
    if (dryRun || res.message.startsWith('nothing')) return setNote(res.message);
    setMessage('');
    toast(t(push ? 'gitSync.toast.pushed' : 'gitSync.toast.committed'), 'success');
  });
  // A clean tree with commits the remote lacks: push them as they are.
  const upload = (count: number) => run('upload', async () => {
    await api.push({});
    toast(t(count === 1 ? 'gitSync.toast.uploaded.one' : 'gitSync.toast.uploaded.other', { count }), 'success');
  });
  const pull = (force = false) => run('pull', async () => {
    setPulled(null);
    const res = await api.pull({ force });
    setPulled(res);
    if (res.upToDate) toast(t('gitSync.pull.alreadyUpToDate'), 'info');
  });
  const checkout = (branch: string) => run('branch', async () => {
    const res = await api.gitCheckout(branch);
    toast(t('gitSync.toast.switchedTo', { branch: res.branch }), 'success');
  });
  const fetchBranches = () => run('fetch', async () => {
    queryClient.setQueryData(queryKeys.gitBranches, await api.gitBranches({ fetch: true }));
    toast(t('gitSync.toast.branchListRefreshed'), 'info');
  });
  const absorb = (dirs: string[]) => run('nested', async () => {
    const res = await api.gitAbsorbNested(dirs);
    toast(t('gitSync.nested.disabled', { dirs: res.disabled.join(', ') }), 'success');
  });
  const setRoot = (scope: string, remoteURL?: string) => run('scope', async () => {
    const res = await api.gitSetRoot(scope, remoteURL);
    toast(t('gitSync.scope.switched', { scope: res.scope }), 'success');
  });

  const header = (actions?: React.ReactNode) => <PageHeader title={t('gitSync.title')} subtitle={t('gitSync.subtitle')} actions={actions} />;

  if (isProjectMode) {
    return (
      <div className="animate-fade-in">
        {header()}
        <EmptyState icon={GitBranch} title={t('gitSync.projectMode.title')} description={t('gitSync.projectMode.description')} />
      </div>
    );
  }
  if (isPending) return <div className="animate-fade-in">{header()}<PageSkeleton /></div>;
  if (error || !status) return <div className="animate-fade-in">{header()}<div className="ss-note bad"><AlertCircle size={16} /><span className="flex-1">{error?.message}</span></div></div>;
  if (!status.gitInstalled) {
    return (
      <div className="animate-fade-in">
        {header()}
        <EmptyState icon={AlertTriangle} title={t('gitSync.gitNotInstalled.title')} description={t('gitSync.gitNotInstalled.hint')} />
      </div>
    );
  }

  const scope = status.scope || 'skills';
  const files = (status.files ?? []).map(parseStatusLine);
  const nested = status.nestedRepos ?? [];
  const remote = parseRemoteURL(status.remoteURL);
  const platform = remote && remote.platform !== 'other' ? t(`gitSync.platformLabel.${remote.platform}`) : null;
  const branchNames = branches.data ? [...branches.data.local, ...branches.data.remote] : [status.branch];
  const writing = busy !== null;

  return (
    <div className="animate-fade-in">
      {header(status.isRepo && (
        <span data-tour="git-actions" className="flex items-center gap-2.5">
          {status.hasRemote && !status.isDirty && status.ahead > 0 && (
            <Button variant="secondary" onClick={() => upload(status.ahead)} loading={busy === 'upload'} disabled={writing || nested.length > 0}>
              {busy !== 'upload' && <ArrowUpFromLine size={16} />}
              {t(status.ahead === 1 ? 'gitSync.actions.pushCommits.one' : 'gitSync.actions.pushCommits.other', { count: status.ahead })}
            </Button>
          )}
          <Button variant="secondary" onClick={() => pull()} loading={busy === 'pull'} disabled={writing || !status.hasRemote || status.isDirty} title={!status.hasRemote ? t('gitSync.noRemoteHint') : undefined}>
            {busy !== 'pull' && <ArrowDownToLine size={16} />}
            {t('gitSync.actions.pull')}
          </Button>
        </span>
      ))}

      <div className="mb-6 flex flex-col gap-4 empty:hidden">
        {status.scopeMismatch && status.mismatchScope && (
          <div className="ss-note bad !items-center">
            <AlertTriangle size={16} />
            <span className="flex-1"><b>{t('gitSync.mismatch.title')}.</b> {t('gitSync.mismatch.description', { scope, repoScope: status.mismatchScope })}</span>
            <Button variant="secondary" size="sm" onClick={() => setRoot(status.mismatchScope!)} loading={busy === 'scope'} disabled={writing}>{t('gitSync.mismatch.switch', { scope: status.mismatchScope })}</Button>
          </div>
        )}
        {nested.length > 0 && (
          <div className="ss-note warn !items-center">
            <FolderGit2 size={16} />
            <span className="flex-1"><b>{t('gitSync.nested.title')}.</b> {t('gitSync.nested.hint', { dirs: nested.join(', ') })}</span>
            <Button variant="secondary" size="sm" onClick={() => absorb(nested)} loading={busy === 'nested'} disabled={writing}>{t('gitSync.nested.disableButton')}</Button>
          </div>
        )}
        {status.configTracked && <div className="ss-note inf"><Info size={16} /><span className="flex-1">{t('gitSync.nested.configTracked')}</span></div>}
        {status.isRepo && status.isDirty && status.hasRemote && <div className="ss-note warn"><AlertCircle size={16} /><span className="flex-1">{t('gitSync.pull.blocked')}</span></div>}
        {runError && (
          <div className="ss-note bad">
            <AlertCircle size={16} />
            <span className="flex-1 whitespace-pre-wrap break-words">{runError}</span>
            {mergeFailed && <Button variant="secondary" size="sm" onClick={() => setConfirmForce(true)} disabled={writing}>{t('gitSync.pull.force.button')}</Button>}
            <button type="button" className="ss-ib !h-6 !w-6" aria-label={t('common.close')} onClick={() => setRunError('')}><X size={14} /></button>
          </div>
        )}
        {pulled && !pulled.upToDate && (
          <div className="ss-note inf">
            <CircleCheck size={16} />
            <div className="flex min-w-0 flex-1 flex-col gap-1.5">
              <span>
                {pulled.commits.length > 0 ? t(pulled.commits.length === 1 ? 'gitSync.pull.pulled.one' : 'gitSync.pull.pulled.other', { count: pulled.commits.length }) : t('gitSync.pull.pulledNone')}{' '}
                {t('gitSync.pull.synced')}
              </span>
              {pulled.commits.slice(0, PULLED_SHOWN).map((c) => (
                <span key={c.hash} className="truncate text-[13px]"><span className="font-mono text-[12.5px] text-ink-3">{c.hash}</span> {c.message}</span>
              ))}
              {pulled.commits.length > PULLED_SHOWN && <span className="text-[13px] text-ink-3">{t('gitSync.pull.more', { count: pulled.commits.length - PULLED_SHOWN })}</span>}
            </div>
          </div>
        )}
        {pulled?.warnings?.map((w) => <div key={w} className="ss-note warn"><AlertTriangle size={16} /><span className="flex-1 break-words">{w}</span></div>)}
      </div>

      {!status.isRepo ? (
        !status.scopeMismatch && (
          <EmptyState
            icon={GitBranch}
            title={t('gitSync.setup.title')}
            description={t('gitSync.setup.description')}
            action={<Button variant="primary" onClick={() => setSetup({ kind: 'init', scope })}>{t('gitSync.setup.button')}</Button>}
          />
        )
      ) : (
        <div className="grid grid-cols-[minmax(0,1fr)_340px] items-start gap-10">
          <div className="flex min-w-0 flex-col gap-4">
            <div className="ss-sec !mb-0">
              <h2 className="ss-h2">{t('gitSync.changes.title')}</h2>
              <span className="ss-cnt">{files.length}</span>
            </div>
            <div className="ss-list">
              {files.length === 0 ? (
                <div className="ss-r !min-h-11 text-[13px] text-ink-2"><CircleCheck size={16} className="text-ok" />{t('gitSync.changes.none')}</div>
              ) : (
                <>
                  <div className="ss-lh">
                    <span className="flex-1">{t('gitSync.changes.file')}</span>
                    <span>{t('gitSync.changes.change')}</span>
                  </div>
                  <div className="max-h-[320px] overflow-y-auto">
                    {files.map((f) => (
                      <div key={f.path} className="ss-r !min-h-10">
                        <span className="min-w-0 flex-1 truncate font-mono text-[13px]" title={f.path}>{f.path}</span>
                        <span className={`ss-st ${TONE[f.change]}`}>{f.change}</span>
                      </div>
                    ))}
                  </div>
                </>
              )}
            </div>
            <Input label={t('gitSync.commit.message')} placeholder="Update skills" value={message} onChange={(e) => setMessage(e.target.value)} disabled={writing} />
            <div className="flex items-center gap-2.5">
              <button type="button" role="switch" aria-checked={dryRun} aria-labelledby="git-dry-run" className={`ss-sw ${dryRun ? 'on' : ''}`} onClick={() => setDryRun(!dryRun)}>
                <i />
              </button>
              <span id="git-dry-run" className="text-[13px] font-semibold">{t('gitSync.dryRun')}</span>
              <span className="min-w-0 flex-1 truncate text-[13px] text-ink-3">{t('gitSync.dryRunHint')}</span>
              <Button variant="secondary" onClick={() => commit(false)} loading={busy === 'commit'} disabled={writing || !status.isDirty || nested.length > 0}>
                {busy !== 'commit' && <GitCommitHorizontal size={16} />}
                {t('gitSync.actions.commit')}
              </Button>
              <Button variant="primary" onClick={() => commit(true)} loading={busy === 'push'} disabled={writing || !status.isDirty || !status.hasRemote || nested.length > 0} title={!status.hasRemote ? t('gitSync.noRemoteHint') : undefined}>
                {busy !== 'push' && <CloudUpload size={16} />}
                {t('gitSync.actions.commitPush')}
              </Button>
            </div>
            {note && <div className="ss-note inf"><Info size={16} /><span className="flex-1">{note}</span></div>}
          </div>

          <aside className="flex flex-col gap-4">
            <div className="ss-box flex flex-col gap-4">
              <div className="flex items-center justify-between gap-3">
                <h3 className="text-[15px] font-semibold">{t('gitSync.repo.title')}</h3>
                {remote?.webURL && platform && (
                  <a href={remote.webURL} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 text-[13px] font-medium">
                    {platform}<ExternalLink size={13} />
                  </a>
                )}
              </div>
              <dl className="ss-kv !grid-cols-[84px_minmax(0,1fr)]">
                <dt>{t('gitSync.repo.remote')}</dt>
                <dd className="min-w-0">
                  {status.remoteURL ? (
                    <span className="block truncate font-mono text-[12.5px]" title={status.remoteURL}>{platform ? remote!.ownerRepo : status.remoteURL}</span>
                  ) : (
                    <span className="flex items-center gap-2">
                      <span className="ss-st warn">{t('gitSync.repo.noRemote')}</span>
                      <button type="button" className="text-[13px] font-semibold" onClick={() => setSetup({ kind: 'remote', scope })}>{t('gitSync.repo.addRemote')}</button>
                    </span>
                  )}
                </dd>
                <dt>{t('gitSync.repo.status')}</dt>
                <dd>
                  {status.isDirty
                    ? <span className="ss-st warn">{t(files.length === 1 ? 'gitSync.repo.dirty.one' : 'gitSync.repo.dirty.other', { count: files.length })}</span>
                    : status.hasRemote && status.ahead > 0
                      ? <span className="ss-st warn">{t(status.ahead === 1 ? 'gitSync.repo.ahead.one' : 'gitSync.repo.ahead.other', { count: status.ahead })}</span>
                      : <span className="ss-st ok">{t('gitSync.repo.clean')}</span>}
                </dd>
                {status.headHash && (
                  <>
                    <dt>{t('gitSync.repo.lastCommit')}</dt>
                    <dd className="min-w-0"><span className="block truncate" title={status.headMessage}><span className="font-mono text-[12.5px] text-ink-3">{status.headHash}</span> {status.headMessage}</span></dd>
                  </>
                )}
              </dl>
              <div className="ss-fld">
                <label>{t('gitSync.branch.label')}</label>
                <div className="flex items-center gap-2">
                  <Select
                    className="min-w-0 flex-1"
                    value={status.branch}
                    onChange={(b) => b !== status.branch && checkout(b)}
                    options={branchNames.map((b) => ({ value: b, label: b, description: branches.data?.remote.includes(b) ? t('gitSync.branch.remoteOnly') : undefined }))}
                    disabled={writing || status.isDirty}
                  />
                  {status.hasRemote && (
                    <button type="button" className="ss-ib" aria-label={t('gitSync.branch.fetchRemote')} title={t('gitSync.branch.fetchRemote')} onClick={fetchBranches} disabled={writing}>
                      <RefreshCw size={15} className={busy === 'fetch' ? 'animate-spin' : ''} />
                    </button>
                  )}
                </div>
                {status.isDirty && <span className="hp">{t('gitSync.branch.dirty')}</span>}
              </div>
              <div className="ss-fld">
                <label>{t('gitSync.scope.label')}</label>
                <Select value={scope} onChange={(s) => s !== scope && setSetup({ kind: 'scope', scope: s })} options={SCOPES.map((s) => ({ value: s, label: s }))} disabled={writing} />
                <span className="hp">{t('gitSync.scope.hint')}</span>
              </div>
            </div>
            {status.hasRemote && <div className="ss-note inf"><Info size={16} /><span className="flex-1">{t('gitSync.pull.hint')}</span></div>}
          </aside>
        </div>
      )}

      <ConfirmDialog
        open={confirmForce}
        variant="danger"
        title={t('gitSync.pull.force.title')}
        message={t('gitSync.pull.force.message', { scope })}
        confirmText={t('gitSync.pull.force.confirm')}
        onCancel={() => setConfirmForce(false)}
        onConfirm={() => { setConfirmForce(false); void pull(true); }}
      />

      {setup && (
        <SetupDialog
          setup={setup}
          status={status}
          onClose={() => setSetup(null)}
          onConfirm={(remoteURL) => { setSetup(null); void setRoot(setup.scope, remoteURL); }}
        />
      )}
    </div>
  );
}

/** Starts a repository, points the source at another scope, or adds a remote. All three go through POST /api/git/root. */
function SetupDialog({ setup, status, onClose, onConfirm }: { setup: Setup; status: GitStatus; onClose: () => void; onConfirm: (remoteURL?: string) => void }) {
  const t = useT();
  const [url, setUrl] = useState('');
  const title = t({ init: 'gitSync.scope.initTitle', scope: 'gitSync.scope.confirmTitle', remote: 'gitSync.repo.addRemote' }[setup.kind]);
  const required = setup.kind === 'remote';
  return (
    <DialogShell open onClose={onClose} padding="none" ariaLabel={title} className="!max-w-[500px]">
      <div className="dh">
        <h2 className="ss-h2">{title}</h2>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose}><X size={16} /></button>
      </div>
      <form className="contents" onSubmit={(e) => { e.preventDefault(); if (!required || url.trim()) onConfirm(url.trim() || undefined); }}>
        <div className="db text-[13.5px] leading-relaxed">
          {setup.kind !== 'remote' && <p>{t(setup.kind === 'init' ? 'gitSync.scope.initMessage' : 'gitSync.scope.confirmMessage', { scope: setup.scope })}</p>}
          <div className="ss-fld">
            <Input label={t(required ? 'gitSync.remote.label' : 'gitSync.scope.remoteLabel')} value={url} onChange={(e) => setUrl(e.target.value)} placeholder="git@github.com:you/skills.git" autoFocus />
            <span className="hp">{t(required ? 'gitSync.remote.hint' : status.hasRemote ? 'gitSync.scope.remoteHint' : 'gitSync.remote.laterHint')}</span>
          </div>
        </div>
        <div className="df">
          <Button type="button" variant="ghost" onClick={onClose}>{t('common.cancel')}</Button>
          <Button type="submit" variant="primary" disabled={required && !url.trim()}>{t(setup.kind === 'init' ? 'gitSync.setup.button' : setup.kind === 'remote' ? 'common.save' : 'common.confirm')}</Button>
        </div>
      </form>
    </DialogShell>
  );
}
