import { useMemo, useState, type ReactNode } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, CircleCheck, CircleX, Folder, GitBranch, Link2, Puzzle, RefreshCw, TriangleAlert, X } from 'lucide-react';
import { api, ApiError } from '../../api/client';
import type { MoveItemResult, Skill } from '../../api/client';
import { clearAuditCache } from '../../lib/auditCache';
import { baseName, existingFolders, joinFolder, linkPrefix, planFolder } from '../../lib/moveFolders';
import { queryKeys } from '../../lib/queryKeys';
import { invalidate } from '../../lib/queryEvents';
import { countLabel } from '../../lib/resourceGrouping';
import { plural, useT } from '../../i18n';
import Button from '../Button';
import DialogShell from '../DialogShell';
import FolderPicker from '../FolderPicker';
import SyncPreviewModal from '../SyncPreviewModal';
import { useNewFolderStep } from '../useNewFolderStep';

/** Refusal codes the dashboard has text for; anything else gets the generic line. */
const KNOWN_CODES = new Set([
  'skill_not_found', 'ambiguous_name', 'dest_exists', 'inside_tracked_repo', 'dest_inside_tracked_repo', 'linked_folder',
  'dest_is_skill', 'invalid_dest', 'dest_inside_source_folder', 'duplicate_dest', 'overlapping_sources', 'same_folder', 'name_collision',
]);
const known = (code?: string) => (code && KNOWN_CODES.has(code) ? code : undefined);
const codeOf = (err: unknown) => known(err instanceof ApiError ? err.code : undefined);

/** The API takes '.' for the source root. */
const wire = (dest: string) => (dest === '' ? '.' : dest);

/** An item that stays where it is is not a move. */
const unchanged = (r: MoveItemResult) => r.error_code === 'same_folder' || (r.success && r.from !== undefined && r.from === r.to);

function Header({ title, sub, onClose, onBack, closeDisabled }: { title: string; sub: ReactNode; onClose: () => void; onBack?: () => void; closeDisabled?: boolean }) {
  const t = useT();
  return (
    <>
      {onBack && <button type="button" className="ss-ib" aria-label={t('folderPicker.backAria')} onClick={onBack}><ArrowLeft size={18} /></button>}
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <h2 className="ss-h2">{title}</h2>
        <p className="text-[13px] text-ink-2">{sub}</p>
      </div>
      <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose} disabled={closeDisabled}><X size={18} /></button>
    </>
  );
}

/** The outcome of a move: one row per item, and what to do next. */
function MoveResults({ results, skills, destFor, running, onRetry, onSync, onClose, reason }: {
  results: MoveItemResult[];
  skills: Skill[];
  /** Where an item lands when the server did not say. */
  destFor: (name: string, from?: string) => string | undefined;
  running: boolean;
  onRetry: (names: string[]) => void;
  onSync: () => void;
  onClose: () => void;
  reason: (code?: string) => string;
}) {
  const t = useT();
  const byName = new Map(skills.map((s) => [s.flatName, s]));
  const done = results.filter((r) => r.success && !unchanged(r));
  const failed = results.filter((r) => !r.success && r.error_code !== 'same_folder');
  const collided = failed.filter((r) => r.error_code === 'name_collision');
  const title = failed.length === 0
    ? t(plural('move.result.allMoved', done.length), { count: done.length })
    : t('move.result.partial', { moved: done.length, failed: failed.length });
  return (
    <DialogShell open onClose={onClose} maxWidth="lg" padding="none" ariaLabel={title} preventClose={running}>
      <div className="dh">
        <Header title={title} sub={t('move.result.sub')} onClose={onClose} closeDisabled={running} />
      </div>
      <div className="db">
        <div className="ss-list !shadow-none max-h-72 overflow-y-auto">
          {results.map((r) => {
            const from = r.from ?? byName.get(r.name)?.relPath;
            return (
              <div key={r.name} className="ss-r !block !py-2.5">
                <div className="flex items-center gap-2.5">
                  {r.success ? <CircleCheck size={16} className="shrink-0 text-ok" /> : <CircleX size={16} className="shrink-0 text-bad" />}
                  <span className="nm m shrink-0">{byName.get(r.name)?.name ?? baseName(r.name)}</span>
                  {r.success && from && r.to && <span className="min-w-0 flex-1 truncate text-right font-mono text-xs text-ink-3">{from} → {r.to}</span>}
                </div>
                {!r.success && (
                  <p className="mt-1 pl-[26px] text-[13px] text-bad">
                    {reason(r.error_code)}
                    {r.error_code === 'dest_exists' && <> <code className="font-mono">{r.to ?? destFor(r.name, from)}</code></>}
                  </p>
                )}
              </div>
            );
          })}
        </div>
        {done.length > 0 && (
          <div className="ss-note warn">
            <RefreshCw size={16} />
            <div className="flex-1">{t('move.result.syncNote')}</div>
          </div>
        )}
      </div>
      <div className="df">
        {collided.length > 0 && (
          <Button variant="ghost" loading={running} onClick={() => onRetry(collided.map((r) => r.name))}>
            {t('move.retryForce')}
          </Button>
        )}
        <span className="flex-1" />
        <Button variant="secondary" onClick={onClose}>{t('move.result.later')}</Button>
        {done.length > 0 && (
          <Button variant="primary" onClick={onSync}>
            <RefreshCw size={15} />
            {t('syncPreview.syncNowButton')}
          </Button>
        )}
      </div>
    </DialogShell>
  );
}

interface Props {
  /** The skills to move; for a folder, leave it out and give `folder`. */
  skills?: Skill[];
  /** A folder (relPath) to move whole, with everything under it. */
  folder?: string;
  /** Every skill, to list folders and to see what is inside `folder`. */
  all: Skill[];
  /** Selected items left out because they cannot move (tracked repos, followed links). */
  skipped?: number;
  /** After a move that moved something, with the results. The dialog stays open on its result view. */
  onMoved?: (results: MoveItemResult[]) => void;
  onClose: () => void;
}

export function MoveDialog({ skills: given = [], folder, all, skipped = 0, onMoved, onClose }: Props) {
  const t = useT();
  const queryClient = useQueryClient();
  const [dest, setDest] = useState<string | null>(null);
  const [naming, setNaming] = useState(false);
  const [running, setRunning] = useState(false);
  const [results, setResults] = useState<MoveItemResult[] | null>(null);
  const [syncing, setSyncing] = useState(false);
  // What was asked for stays what is shown: a detail page changes its skill's name once the move is done.
  const [skills] = useState(given);

  const skillsOnly = useMemo(() => all.filter((s) => s.kind === 'skill'), [all]);
  const folders = useMemo(() => existingFolders(skillsOnly), [skillsOnly]);
  const plan = useMemo(() => (folder ? planFolder(all, folder) : null), [all, folder]);
  const names = folder ? [folder] : skills.map((s) => s.flatName);
  const disabledPaths = folder ? [folder] : undefined;
  const moving = plan ? plan.count : skills.length;
  const base = folder ? baseName(folder) : '';
  const [firstSkill] = skills;

  const newFolder = useNewFolderStep({
    kind: 'skill',
    folders,
    rootCount: skillsOnly.length,
    skill: firstSkill?.name ?? null,
    disabledPaths,
    onUse: (path) => {
      setDest(path);
      setNaming(false);
    },
    onBack: () => setNaming(false),
  });

  // What a move to `dest` would do: new paths, link names, refusals, filter warnings. Changes nothing, so
  // it is not worth repeating on focus, and it stops once the real move has run.
  const preview = useQuery({
    queryKey: queryKeys.movePreview(names, dest ?? ''),
    queryFn: () => api.moveResources({ names, dest: wire(dest ?? ''), dryRun: true }),
    enabled: dest !== null && !results,
    gcTime: 0,
    staleTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
  const planned = useMemo(() => new Map((preview.data?.results ?? []).map((r) => [r.name, r])), [preview.data]);

  const reason = (code?: string) => t(known(code) ? `move.error.${code}` : 'move.error.unknown');
  const requestError = preview.error ? reason(codeOf(preview.error)) : null;

  const run = async (targets: string[], force: boolean) => {
    if (dest === null) return;
    setRunning(true);
    try {
      const res = await api.moveResources({ names: targets, dest: wire(dest), force: force || undefined });
      clearAuditCache(queryClient);
      void invalidate(queryClient, 'skillsMoved');
      // A retry replaces only the rows it retried.
      setResults((prev) => (prev ? prev.map((r) => res.results.find((n) => n.name === r.name) ?? r) : res.results));
      if (res.results.some((r) => r.success && !unchanged(r))) onMoved?.(res.results);
    } catch (err) {
      // A failed retry marks only what it retried; what already moved stays moved.
      const code = codeOf(err);
      setResults((prev) => (prev
        ? prev.map((r) => (targets.includes(r.name) ? { ...r, success: false, error_code: code } : r))
        : names.map((name) => ({ name, success: false, error_code: code }))));
    } finally {
      setRunning(false);
    }
  };

  // Replaces the results dialog rather than stacking a second one on top; it slides in like a step.
  if (syncing) return <SyncPreviewModal open onClose={onClose} kind="skill" className="animate-step-fwd" />;

  if (results) {
    return (
      <MoveResults
        results={results}
        skills={skills}
        destFor={(name, from) => (dest === null ? undefined : joinFolder(dest, base || baseName(from ?? name)))}
        running={running}
        onRetry={(retry) => run(retry, true)}
        onSync={() => setSyncing(true)}
        onClose={onClose}
        reason={reason}
      />
    );
  }

  const failures = (preview.data?.results ?? []).filter((r) => !r.success && r.error_code !== 'same_folder');
  // Items the preview would move, and those that only collide in a target: the result view can retry them.
  const previewed = preview.data?.results ?? [];
  const wouldMove = dest === null ? moving : previewed.filter((r) => (r.success && !unchanged(r)) || r.error_code === 'name_collision').length;
  // A folder moves whole or not at all. The preview decides once there is one; before a destination is chosen
  // only what the list shows can. A collision is the one refusal the result view can retry.
  const folderBlocked = !!plan && (dest === null ? plan.blocked.length > 0 : failures.some((r) => r.error_code !== 'name_collision'));
  const canRun = dest !== null && !preview.isFetching && !requestError && !folderBlocked && wouldMove > 0;
  const targets = folder ? names : names.filter((n) => {
    const p = planned.get(n);
    return !p || p.success || p.error_code === 'name_collision';
  });

  const motion = newFolder.motion;
  const step = naming ? 'name' : 'form';

  const title = folder ? t('move.folderTitle', { name: folder }) : t('move.title');
  const newBase = dest === null ? null : joinFolder(dest, base);
  const firstFlat = folder ? undefined : [...planned.values()].find((r) => r.success && r.flatName)?.flatName;
  const caption = dest === null ? undefined
    : folder ? t('move.captionFolder', { prefix: linkPrefix(newBase!) })
      : t(plural('move.caption', moving), { name: firstFlat ?? `${linkPrefix(dest)}${firstSkill ? baseName(firstSkill.relPath) : ''}`, count: moving });

  const skillRow = (key: string, name: string, detail: string, tone = 'text-ink-3') => (
    <div key={key} className="ss-r !min-h-[42px]">
      <span className="ss-cat sm skill"><Puzzle size={14} /></span>
      <span className="nm m shrink-0">{name}</span>
      <span className={`min-w-0 flex-1 truncate text-right font-mono text-xs ${tone}`}>{detail}</span>
    </div>
  );

  return (
    <DialogShell open onClose={naming ? newFolder.back : onClose} maxWidth="lg" padding="none" ariaLabel={naming ? newFolder.title : title} preventClose={running}>
      <div key={`dh-${step}`} className={`dh ${motion.className}`} onAnimationEnd={motion.onAnimationEnd}>
        {naming
          ? <Header title={newFolder.title} sub={newFolder.sub} onClose={onClose} onBack={newFolder.back} />
          : <Header title={title} sub={folder ? t('move.folderSub') : t(plural('move.sub', moving), { count: moving })} onClose={onClose} closeDisabled={running} />}
      </div>
      <div key={`db-${step}`} className={`db ${motion.className}`} onAnimationEnd={motion.onAnimationEnd}>
        {naming ? newFolder.body : (
          <>
            <div className="ss-list !shadow-none max-h-64 overflow-y-auto">
              {plan ? (
                <>
                  <div className="ss-r !min-h-9 text-xs">
                    <span className="flex-1 font-semibold text-ink-3">
                      {t('move.folderSummary', {
                        skills: countLabel(t, 'skill', plan.count),
                        folders: t(plural('move.subfolders', plan.subfolders.length), { count: plan.subfolders.length }),
                      })}
                    </span>
                    {plan.blocked.length > 0 && (
                      <span className="inline-flex items-center gap-1.5 font-semibold text-bad">
                        <span className="size-1.5 rounded-full bg-bad" aria-hidden="true" />
                        {t(plural('move.blockedCount', plan.blocked.length), { count: plan.blocked.length })}
                      </span>
                    )}
                  </div>
                  {plan.direct.map((s) => skillRow(s.flatName, s.name, newBase === null ? s.relPath : joinFolder(newBase, baseName(s.relPath))))}
                  {plan.subfolders.map((f) => (
                    <div key={f.name} className="ss-r !min-h-[42px]">
                      <Folder size={15} className="shrink-0 text-ink-2" />
                      <span className="nm m shrink-0">{f.name}<span className="font-normal text-ink-3"> / {countLabel(t, 'skill', f.count)}</span></span>
                      <span className="min-w-0 flex-1 truncate text-right font-mono text-xs text-ink-3">{joinFolder(newBase ?? folder!, f.name)}</span>
                    </div>
                  ))}
                  {plan.blocked.map((b) => (
                    <div key={b.path} className="ss-r !min-h-[42px] bg-bad-bg">
                      {b.why === 'link' ? <Link2 size={15} className="shrink-0 text-bad" /> : <GitBranch size={15} className="shrink-0 text-bad" />}
                      <span className="nm m shrink-0">{b.path}</span>
                      <span className="min-w-0 flex-1 truncate text-right text-xs text-bad">{t(b.why === 'link' ? 'move.blockedLinked' : 'move.blockedTracked')}</span>
                    </div>
                  ))}
                </>
              ) : (
                skills.map((s) => {
                  const item = planned.get(s.flatName);
                  const bad = !!item && !item.success && item.error_code !== 'same_folder';
                  return skillRow(s.flatName, s.name, bad ? reason(item.error_code) : s.relPath, bad ? 'text-bad' : 'text-ink-3');
                })
              )}
            </div>

            <FolderPicker
              label={t('move.destination')}
              kind="skill"
              value={dest}
              placeholder={t('move.destinationPlaceholder')}
              onChange={setDest}
              folders={folders}
              rootCount={skillsOnly.length}
              disabledPaths={disabledPaths}
              onNewFolder={(parent) => { newFolder.start(parent); setNaming(true); }}
              caption={folder ? <>{t('move.notListed', { name: folder })}{caption && <><br />{caption}</>}</> : caption}
            />

            {plan && plan.blocked.length > 0 && folderBlocked && (
              <div className="ss-note warn">
                <TriangleAlert size={16} />
                <div className="flex-1">{t('move.folderFix', { names: plan.blocked.map((b) => b.path).join(', '), count: plan.count })}</div>
              </div>
            )}
            {plan && plan.blocked.length === 0 && failures.length > 0 && (
              <div className="ss-note warn">
                <TriangleAlert size={16} />
                <div className="flex-1">{reason(failures[0].error_code)}</div>
              </div>
            )}
            {requestError && (
              <div className="ss-note warn">
                <TriangleAlert size={16} />
                <div className="flex-1">{requestError}</div>
              </div>
            )}
            {(preview.data?.warnings.length ?? 0) > 0 && (
              <div className="ss-note warn">
                <TriangleAlert size={16} />
                <div className="flex-1">{t('move.filterWarning', { count: preview.data!.warnings.length })}</div>
              </div>
            )}
            {skipped > 0 && <p className="text-[13px] text-ink-2">{t('move.skipped', { count: skipped })}</p>}
          </>
        )}
      </div>
      <div key={`df-${step}`} className={`df ${motion.className}`} onAnimationEnd={motion.onAnimationEnd}>
        {naming ? newFolder.foot : (
          <>
            <Button variant="ghost" onClick={onClose} disabled={running}>{t('common.cancel')}</Button>
            <Button variant="primary" loading={running} disabled={!canRun} onClick={() => run(targets, false)}>
              {folder ? t('move.submitFolder') : t(plural('move.submit', wouldMove), { count: wouldMove })}
            </Button>
          </>
        )}
      </div>
    </DialogShell>
  );
}
