import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Bot, CircleCheck, CircleX, GitBranch, Puzzle, RefreshCw, Trash2, TriangleAlert } from 'lucide-react';
import { api } from '../../api/client';
import type { BatchUninstallItemResult, Skill } from '../../api/client';
import { clearAuditCache } from '../../lib/auditCache';
import { formatTrackedRepoName } from '../../lib/resourceNames';
import { countLabel, descendantCount, outermost, parentPath, repoOf, sourceLinkOf } from '../../lib/resourceGrouping';
import { useT } from '../../i18n';
import Button from '../Button';
import { Checkbox } from '../Checkbox';
import DialogShell from '../DialogShell';
import SyncPreviewModal from '../SyncPreviewModal';
import { useToast } from '../Toast';
import { invalidate } from '../../lib/queryEvents';

export function UninstallDialog({ kind, selection, all, onClose }: {
  kind: Skill['kind'];
  selection: Skill[];
  all: Skill[];
  onClose: (removed: boolean) => void;
}) {
  const t = useT();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const [force, setForce] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [running, setRunning] = useState(false);
  const [results, setResults] = useState<BatchUninstallItemResult[] | null>(null);

  // Followed folders and agents are removed one skill at a time.
  const repos = new Map<string, number>();
  const singles: Skill[] = [];
  for (const s of selection) {
    const repo = kind === 'skill' && !sourceLinkOf(s) ? repoOf(s) : undefined;
    if (repo) repos.set(repo, all.filter((x) => repoOf(x) === repo).length);
    else singles.push(s);
  }
  // A skill's folder holds its nested skills, so they go to the trash with it. Callers may pass agents in `all` too.
  const skills = kind === 'skill' ? all.filter((x) => x.kind === 'skill') : [];
  // A linked folder's root is only ever unlinked, never trashed, so its nested skills are removed one by one.
  const holds = (s: Skill) => sourceLinkOf(s)?.name !== s.relPath;
  const tops = kind === 'skill' ? outermost(singles, singles.filter(holds)) : singles;
  const inside = new Map(tops.map((s) => [s.flatName, holds(s) ? descendantCount(s, skills) : 0]));
  const nestedCount = [...inside.values()].reduce((a, b) => a + b, 0);
  const names = [...repos.keys(), ...tops.map((s) => s.flatName)];
  const removedCount = tops.length + nestedCount + [...repos.values()].reduce((a, b) => a + b, 0);

  const run = async (targets: string[], withForce: boolean) => {
    setRunning(true);
    try {
      const res = await api.batchUninstall({ names: targets, kind, force: withForce });
      clearAuditCache(queryClient);
      void invalidate(queryClient, 'skillsUninstalled');
      if (res.summary.failed === 0) {
        toast(t('batchUninstall.toast.success', { count: removedCount }), 'success');
        onClose(true);
        return;
      }
      setResults(res.results);
    } catch (err) {
      toast(t('batchUninstall.toast.uninstallFailed', { error: err instanceof Error ? err.message : String(err) }), 'error');
    } finally {
      setRunning(false);
    }
  };

  // Replaces the results dialog rather than stacking a second one on top.
  if (syncing) return <SyncPreviewModal open onClose={() => onClose(true)} kind={kind} />;

  if (results) {
    const failed = results.filter((r) => !r.success);
    return (
      <DialogShell open onClose={() => onClose(true)} maxWidth="lg" padding="none" ariaLabel={t('batchUninstall.results.partialResult')} preventClose={running}>
        <div className="dh">
          <div className="flex flex-col gap-1">
            <h2 className="ss-h2">{t('batchUninstall.results.partialResult')}</h2>
            <p className="text-[13px] text-ink-2">{t('resources.uninstall.resultSummary', { removed: results.length - failed.length, failed: failed.length })}</p>
          </div>
        </div>
        <div className="db">
          <div className="ss-list !shadow-none">
            {results.map((r) => (
              <div key={r.name} className="ss-r !min-h-11">
                {r.success ? <CircleCheck size={16} className="shrink-0 text-ok" /> : <CircleX size={16} className="shrink-0 text-bad" />}
                <span className="nm m flex-1 truncate">{formatTrackedRepoName(r.name)}</span>
                <span className={`text-[13px] ${r.success ? 'text-ink-2' : 'text-bad'}`}>{r.success ? t('resources.uninstall.movedToTrash') : r.error}</span>
              </div>
            ))}
          </div>
          <div className="ss-note warn">
            <RefreshCw size={16} />
            <div className="flex-1">{t('resources.uninstall.syncReminder')}</div>
          </div>
        </div>
        <div className="df">
          {!force && failed.length > 0 && (
            <Button variant="ghost" loading={running} onClick={() => { setForce(true); run(failed.map((r) => r.name), true); }}>
              {t('resources.uninstall.retryForce')}
            </Button>
          )}
          <span className="flex-1" />
          <Button variant="secondary" onClick={() => onClose(true)}>{t('batchUninstall.results.continueButton')}</Button>
          <Button variant="primary" onClick={() => setSyncing(true)}>
            <RefreshCw size={15} />
            {t('syncPreview.syncNowButton')}
          </Button>
        </div>
      </DialogShell>
    );
  }

  const title = t('resources.uninstall.title', { what: countLabel(t, kind, removedCount) });
  return (
    <DialogShell open onClose={() => onClose(false)} maxWidth="lg" padding="none" ariaLabel={title} preventClose={running}>
      <div className="dh">
        <h2 className="ss-h2">{title}</h2>
      </div>
      <div className="db">
        <div className="ss-list !shadow-none max-h-64 overflow-y-auto">
          {[...repos].map(([repo, n]) => (
            <div key={repo} className="ss-r !min-h-[42px]">
              <GitBranch size={15} className="shrink-0 text-ink-2" />
              <span className="nm m flex-1 truncate">{formatTrackedRepoName(repo)}</span>
              <span className="text-xs text-ink-3">{t('resources.uninstall.wholeRepo', { what: countLabel(t, 'skill', n) })}</span>
            </div>
          ))}
          {tops.map((s) => (
            <div key={s.flatName} className="ss-r !min-h-[42px]">
              <span className={`ss-cat sm ${kind}`}>{kind === 'agent' ? <Bot size={14} /> : <Puzzle size={14} />}</span>
              <span className="nm m flex-1 truncate">{s.name}</span>
              {inside.get(s.flatName)! > 0 && <span className="shrink-0 text-xs text-ink-3">{t('resources.uninstall.inside', { what: countLabel(t, kind, inside.get(s.flatName)!) })}</span>}
              <span className="font-mono text-xs text-ink-3 truncate">{parentPath(s)}</span>
            </div>
          ))}
        </div>
        {repos.size > 0 && (
          <div className="ss-note warn">
            <TriangleAlert size={16} />
            <div className="flex-1">{t('resources.uninstall.repoNote')}</div>
          </div>
        )}
        {nestedCount > 0 && (
          <div className="ss-note warn">
            <TriangleAlert size={16} />
            <div className="flex-1">{t('resources.uninstall.nestedNote', { what: countLabel(t, kind, nestedCount) })}</div>
          </div>
        )}
        {repos.size > 0 && (
          <Checkbox size="sm" label={t('batchUninstall.confirm.forceLabel')} checked={force} onChange={setForce} />
        )}
        {tops.some((s) => sourceLinkOf(s)) && <p className="text-[13px] text-ink-2">{t('resources.uninstall.linkedNote')}</p>}
        <p className="text-[13px] text-ink-2">{t('resources.uninstall.trashNote')}</p>
      </div>
      <div className="df">
        <Button variant="ghost" onClick={() => onClose(false)} disabled={running}>{t('common.cancel')}</Button>
        <Button variant="secondary" loading={running} onClick={() => run(names, force)}>
          <Trash2 size={15} />
          {t('resources.contextMenu.uninstall')}
        </Button>
      </div>
    </DialogShell>
  );
}
