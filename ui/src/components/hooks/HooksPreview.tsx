import { useState } from 'react';
import { Link } from 'react-router-dom';
import { AlertTriangle } from 'lucide-react';
import type { HookChange, HookPlan } from '../../api/hooks';
import { useT } from '../../i18n';
import { shortenHome } from '../../lib/paths';
import AgentIcon from '../AgentIcon';
import { Checkbox } from '../Checkbox';
import Tooltip from '../Tooltip';
import { actionLabel, blockedHint, fileName, groupByFile, hookLabel, hookMessage, needsTakeover, statusTone } from './hooksView';

/** What a sync would write, one block per native file. Nothing here has been written or executed. `onTakeover` offers the take-over preview on conflicts `canTakeOver` accepts. */
export default function HooksPreview({ plan, canTakeOver, onTakeover }: { plan: HookPlan; canTakeOver?: (c: HookChange) => boolean; onTakeover?: (name: string) => void }) {
  const t = useT();
  const [hideSynced, setHideSynced] = useState(true);
  const unchanged = plan.changes.filter((c) => c.action === 'unchanged').length;
  const canHide = unchanged > 0 && unchanged < plan.changes.length;
  const files = groupByFile(plan.changes.filter((c) => !(canHide && hideSynced && c.action === 'unchanged')));
  return (
    <div className="flex flex-col gap-3">
      {plan.blocked && <div className="ss-note warn" role="alert"><AlertTriangle size={16} /><span className="flex-1">{blockedHint(t, plan)}</span></div>}
      {plan.changes.length === 0 && <p className="text-[13px] text-ink-2">{t('hooks.preview.nothing')}</p>}
      {canHide && <Checkbox size="sm" label={t('mcp.hideSynced')} checked={hideSynced} onChange={setHideSynced} />}
      <div className="flex max-h-[50vh] flex-col gap-3 overflow-auto">
        {files.map((file) => (
          <section key={file.path} aria-label={file.path} className="ss-list !shadow-none">
            <div className="ss-r fold !min-h-10">
              <span className="ss-at"><AgentIcon target={file.target} size={17} /></span>
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-semibold">{hookLabel(file.target)}</span>
                  <span className="ss-tag">{file.root ? t('hooks.preview.project', { name: fileName(file.root) }) : t('hooks.preview.global')}</span>
                </div>
                <span className="block truncate font-mono text-xs text-ink-3" title={file.path}>{shortenHome(file.path)}</span>
              </div>
            </div>
            {file.changes.map((c) => (
              <div key={`${c.name}:${c.action}`} className="ss-r !min-h-10 !flex-wrap !gap-y-1.5">
                <span className={`ss-st w-[110px] shrink-0 ${needsTakeover(c) ? 'warn' : statusTone[c.action] ?? ''}`}>{needsTakeover(c) ? t('hooks.status.unmanaged') : actionLabel(t, c.action)}</span>
                <span className="min-w-0 flex-1 truncate font-mono text-[13px]">{c.name}</span>
                {onTakeover && canTakeOver?.(c) && <button type="button" className="shrink-0 text-xs font-semibold text-ink-2 hover:text-ink" aria-label={`${t('hooks.takeoverMenu')} · ${c.name}`} onClick={() => onTakeover(c.name)}>{t('hooks.takeoverMenu')}</button>}
                {c.action === 'conflict' && c.root && !canTakeOver?.(c) && <Link to={`/projects/${encodeURIComponent(c.root)}?tab=hooks`} className="shrink-0 text-xs font-semibold text-ink-2 hover:text-ink">{t('hooks.preview.openProject')}</Link>}
                {c.message && <span className={c.action === 'conflict' ? 'w-full' : 'min-w-0 max-w-[55%]'}><Tooltip block content={hookMessage(t, c.message)}><span className={`block text-xs text-ink-2 ${c.action === 'conflict' ? 'w-full break-words leading-normal' : 'truncate'}`}>{hookMessage(t, c.message)}</span></Tooltip></span>}
              </div>
            ))}
          </section>
        ))}
      </div>
    </div>
  );
}
