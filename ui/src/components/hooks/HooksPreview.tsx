import { useState } from 'react';
import { Link } from 'react-router-dom';
import { AlertTriangle, TriangleAlert } from 'lucide-react';
import type { HookChange, HookFileDiff, HookPreview, HookUnmanaged } from '../../api/hooks';
import { useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import AgentIcon from '../AgentIcon';
import { Checkbox } from '../Checkbox';
import Tooltip from '../Tooltip';
import { actionLabel, blockedHint, groupByFile, hookLabel, hookMessage, needsTakeover } from './hooksView';
import { diffLines, eventLabels, foldDiff, lineEvent } from './hookCatalog';

const lineTone = { ' ': 'text-ink-3', '+': 'bg-diff-add-bg text-ink', '-': 'bg-diff-del-bg text-ink' };
const signTone = { ' ': '', '+': 'text-diff-add', '-': 'text-diff-del' };

/** A real line diff of one file; unchanged runs fold, long lines wrap, and a user's own hooks are marked as untouched on a line of their own. */
function FileDiff({ file, untouched, index, total }: { file: HookFileDiff; untouched: string[]; index: number; total: number }) {
  const t = useT();
  const lines = foldDiff(diffLines(file.before, file.after));
  const label = t('hooks.preview.diff', { path: file.path });
  return (
    <div role="region" className="max-h-[320px] overflow-y-auto border-t border-line py-1.5 font-mono text-[12px] leading-[1.7]" aria-label={total > 1 ? `${label} (${index + 1}/${total})` : label}>
      {lines.map((line, i) => {
        if ('skip' in line) return <div key={i} className="px-3.5 text-ink-3">{t('hooks.preview.folded', { count: String(line.skip) })}</div>;
        // An event line of the user's own shows as added only when it just gained or lost a comma; mark its new text.
        const event = line.op !== '-' ? lineEvent(line.text) : undefined;
        return (
          <div key={i} className={lineTone[line.op]}>
            <div className="flex gap-3 px-3.5">
              <span className={`w-3 shrink-0 select-none font-semibold ${signTone[line.op]}`} aria-hidden="true">{line.op === ' ' ? '' : line.op === '+' ? '+' : '−'}</span>
              <span className="min-w-0 flex-1 whitespace-pre-wrap [overflow-wrap:anywhere]">{line.text}</span>
            </div>
            {event && untouched.includes(event) && <div className="pl-[calc(0.875rem+1.5rem)] font-sans text-[11px] text-ink-3">{t('hooks.preview.untouched')}</div>}
          </div>
        );
      })}
    </div>
  );
}

/**
 * What a sync would write, one card per native file: the action, the events it changes and the file's
 * diff when the plan carries its contents. Nothing here has been written or executed. `onTakeover`
 * offers the take-over preview on conflicts `canTakeOver` accepts.
 */
export default function HooksPreview({ plan, unmanaged = [], canTakeOver, onTakeover }: { plan: HookPreview; unmanaged?: HookUnmanaged[]; canTakeOver?: (c: HookChange) => boolean; onTakeover?: (name: string) => void }) {
  const t = useT();
  const [hideSynced, setHideSynced] = useState(true);
  const unchanged = plan.changes.filter((c) => c.action === 'unchanged').length;
  const canHide = unchanged > 0 && unchanged < plan.changes.length;
  const files = groupByFile(plan.changes.filter((c) => !(canHide && hideSynced && c.action === 'unchanged')));
  // A plan with both global and project files says which is which; a single scope needs no label.
  const mixed = plan.changes.some((c) => c.root) && plan.changes.some((c) => !c.root);
  // Name the hook on a file card only when the plan covers more than one.
  const named = new Set(plan.changes.map((c) => c.name)).size > 1;
  // Adopting claims registrations already in the file, so its card has no diff and says so.
  const action = (c: HookChange) => (needsTakeover(c) ? t('hooks.status.unmanaged') : c.action === 'adopt' ? t('hooks.preview.adoptUnchanged') : actionLabel(t, c.action));
  return (
    <div className="flex flex-col gap-3">
      {plan.blocked && <div className="ss-note warn" role="alert"><AlertTriangle size={16} /><span className="flex-1">{blockedHint(t, plan)}</span></div>}
      {plan.warnings?.map((w) => <div key={w} className="flex items-start gap-2 text-[13px] text-ink-2"><TriangleAlert size={14} className="mt-0.5 shrink-0" /><span className="flex-1">{hookMessage(t, w)}</span></div>)}
      {plan.changes.length === 0 && <p className="text-[13px] text-ink-2">{t('hooks.preview.nothing')}</p>}
      {canHide && <Checkbox size="sm" label={t('mcp.hideSynced')} checked={hideSynced} onChange={setHideSynced} />}
      <div className="flex max-h-[55vh] flex-col gap-3 overflow-auto">
        {files.map((file) => {
          const single = file.changes.length === 1 ? file.changes[0] : undefined;
          const diffs = plan.files?.filter((f) => f.path === file.path) ?? [];
          const untouched = unmanaged.filter((u) => u.path === file.path).flatMap((u) => u.names);
          return (
            <section key={file.path} aria-label={file.path} className="shrink-0 overflow-hidden rounded-[12px] border border-line">
              <div className="flex min-h-11 items-center gap-2.5 bg-sunken px-3.5 py-2">
                <AgentIcon target={file.target} size={17} />
                <span className="shrink-0 font-semibold">{hookLabel(file.target)}</span>
                {single && named && <span className="min-w-0 truncate font-mono text-[13px]">{single.name}</span>}
                <span className="min-w-0 flex-1 truncate font-mono text-xs text-ink-3" title={file.path}>{shortenHome(file.path)}</span>
                {mixed && <span className="ss-tag shrink-0 !font-sans">{file.root ? t('hooks.preview.project', { name: file.root.split(/[\\/]/).pop() ?? file.root }) : t('hooks.preview.global')}</span>}
                {single && <span className="shrink-0 text-xs text-ink-2">{action(single)}</span>}
                {single && eventLabels(single).map((l) => <span key={l} className="shrink-0 font-mono text-xs text-ink-2">{l}</span>)}
              </div>
              {file.changes.filter((c) => !single || c.message || c.action === 'conflict').map((c) => (
                <div key={`${c.name}:${c.action}`} className="flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-line px-3.5 py-2">
                  {!single && <span className="min-w-[80px] shrink-0 text-xs text-ink-2">{action(c)}</span>}
                  {!single && <span className="min-w-0 truncate font-mono text-[13px]">{c.name}</span>}
                  {!single && eventLabels(c).map((l) => <span key={l} className="shrink-0 font-mono text-xs text-ink-2">{l}</span>)}
                  <span className="flex-1" />
                  {onTakeover && canTakeOver?.(c) && <button type="button" className="shrink-0 text-xs font-semibold text-ink-2 hover:text-ink" aria-label={`${t('hooks.takeoverMenu')} · ${c.name}`} onClick={() => onTakeover(c.name)}>{t('hooks.takeoverMenu')}</button>}
                  {c.action === 'conflict' && c.root && !canTakeOver?.(c) && <Link to={`/projects/${encodeURIComponent(c.root)}?tab=hooks`} className="shrink-0 text-xs font-semibold text-ink-2 hover:text-ink">{t('hooks.preview.openProject')}</Link>}
                  {c.message && <span className={c.action === 'conflict' ? 'w-full' : 'min-w-0 max-w-[55%]'}><Tooltip block content={hookMessage(t, c.message)}><span className={`block text-xs text-ink-2 ${c.action === 'conflict' ? 'w-full break-words leading-normal' : 'truncate'}`}>{hookMessage(t, c.message)}</span></Tooltip></span>}
                </div>
              ))}
              {diffs.map((diff, i) => <FileDiff key={i} file={diff} untouched={untouched} index={i} total={diffs.length} />)}
            </section>
          );
        })}
      </div>
    </div>
  );
}
