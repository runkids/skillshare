import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { TriangleAlert, X } from 'lucide-react';
import { api } from '../../api/client';
import type { SharedInstructionsTarget } from '../../api/client';
import AgentTile from '../AgentTile';
import Button from '../Button';
import CodeView from '../CodeView';
import DialogShell from '../DialogShell';
import Spinner from '../Spinner';
import { useT } from '../../i18n';
import { queryKeys } from '../../lib/queryKeys';
import { shortenHome } from '../../lib/paths';
import {
  blockMode,
  blockOwner,
  instructionsErrorMessage,
  managedBlocks,
  usesOf,
} from './instructionsView';
import { BoxHeader, InstructionsPreview } from './ViewTabs';

const range = (start: number, end: number) =>
  Array.from({ length: end - start + 1 }, (_, i) => start + i);

/**
 * A read-only look at a target's instruction file, opened from its row on the
 * AGENTS.md tab: what the tool reads right now, with the shared parts in place.
 * The source view tints each managed block, a hand-edited one in the warning
 * tone, and the footer offers Collect and Reapply for it. Editing stays on the
 * target page, which the footer links to.
 */
export default function TargetFileDialog({
  target,
  name,
  onResolve,
  onClose,
}: {
  target: SharedInstructionsTarget;
  /** The shared file whose tab is open; its block is the one described and resolved here. */
  name: string;
  onResolve: (action: 'collect' | 'reapply') => void;
  onClose: () => void;
}) {
  const t = useT();
  const [view, setView] = useState<'preview' | 'source'>('preview');
  const { data, error, isPending } = useQuery({
    queryKey: queryKeys.instructions.target(target.name),
    queryFn: () => api.getTargetInstructions(target.name),
  });
  const content = data?.content ?? '';
  const names = usesOf(target);
  const mine = target.assigned.find((a) => a.name === name);
  const isBlock = Boolean(mine && blockMode(mine.mode));
  // Blocks of a shared file this target reports as modified are tinted as such; the rest as managed.
  const modified = new Set(
    target.assigned.filter((a) => a.status === 'modified').map((a) => a.name),
  );
  const blocks = managedBlocks(content).map((b) => ({ ...b, owner: blockOwner(b, names) }));
  const lines = (edited: boolean) =>
    blocks
      .filter((b) => (b.owner !== null && modified.has(b.owner)) === edited)
      .flatMap((b) => range(b.start, b.end));

  return (
    <DialogShell
      open
      onClose={onClose}
      padding="none"
      ariaLabel={t('instructions.peek.label', { target: target.name })}
      className="!max-w-[860px]"
    >
      <div className="dh">
        <div className="flex min-w-0 items-center gap-3">
          <AgentTile target={target.name} />
          <div className="flex min-w-0 flex-col gap-0.5">
            <h2 className="ss-h2 font-mono">{target.name}</h2>
            <p className="truncate font-mono text-[12.5px] text-ink-2" title={target.path}>
              {shortenHome(target.path)}
            </p>
          </div>
        </div>
        <button type="button" className="ss-ib" aria-label={t('common.close')} onClick={onClose}>
          <X size={16} />
        </button>
      </div>
      <div className="db">
        {error ? (
          <div className="ss-note bad">
            <span className="flex-1">{instructionsErrorMessage(error, t)}</span>
          </div>
        ) : (
          <div className="ss-code flex max-h-[60vh] min-h-[200px] flex-col !overflow-hidden !p-0 !whitespace-normal">
            <BoxHeader
              content={isPending ? undefined : content}
              view={view}
              onChange={setView}
              views={[
                { value: 'preview', label: t('instructions.target.view.preview') },
                { value: 'source', label: t('instructions.target.view.source') },
              ]}
            />
            {isPending ? (
              <div className="flex flex-1 items-center justify-center">
                <Spinner />
              </div>
            ) : view === 'source' ? (
              <CodeView
                content={content}
                lang="markdown"
                tints={lines(false)}
                marks={lines(true)}
                className="min-h-0 flex-1 overflow-auto !rounded-none !border-0"
              />
            ) : (
              <InstructionsPreview content={content} names={names} />
            )}
          </div>
        )}
        {isBlock && mine?.status === 'modified' ? (
          <div className="ss-note warn !items-center">
            <TriangleAlert size={16} className="!mt-0" />
            <span className="flex-1">{t('instructions.row.modifiedBlock', { name })}</span>
            <Button variant="secondary" size="sm" onClick={() => onResolve('collect')}>
              {t('instructions.resolve.collect.itemBlock', { name })}
            </Button>
            <Button variant="secondary" size="sm" onClick={() => onResolve('reapply')}>
              {t('instructions.resolve.reapply.itemBlock', { name })}
            </Button>
          </div>
        ) : (
          isBlock && (
            <p className="flex items-center gap-2 text-[12.5px] text-ink-2">
              <span
                className="inline-block h-3 w-3 shrink-0 rounded-[3px] bg-link-bg"
                aria-hidden
              />
              {t('instructions.peek.block', { name })}
            </p>
          )
        )}
      </div>
      <div className="df">
        <span className="flex-1" />
        <Button variant="ghost" onClick={onClose}>
          {t('common.close')}
        </Button>
        <Link
          to={`/targets/${encodeURIComponent(target.name)}?tab=instructions`}
          className="ss-btn"
        >
          {t('instructions.peek.open')}
        </Link>
      </div>
    </DialogShell>
  );
}
