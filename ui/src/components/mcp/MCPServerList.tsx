import { Fragment, useState } from 'react';
import { Ellipsis } from 'lucide-react';
import { useT } from '../../i18n';
import AgentIcon from '../AgentIcon';
import { mcpOffTargets } from '../../api/mcp';
import { describeEndpoint, targetLabel, writes, type MatrixRow } from './mcpView';
import { TargetPill, TargetToggles } from './TargetPicker';
import type { MCPCheckFinding } from '../../api/mcpCheck';
import MCPCheckFindings, { MCPCheckTag } from './MCPCheckFindings';

interface Props {
  rows: MatrixRow[];
  targets: string[];
  targetsOf: (name: string) => string[];
  onToggle: (name: string, target: string, on: boolean) => void;
  onMenu: (e: React.MouseEvent<HTMLButtonElement>, name: string) => void;
  /** Agents a switch-only entry can go to. mcp.projects cannot reach Claude's off list. */
  offTargets?: readonly string[];
  /** Holds the toggles while a save is on its way: each sends the revision it previewed. */
  disabled?: boolean;
  /** The last check's errors and warnings, by server name. */
  problems?: Record<string, MCPCheckFinding[]>;
}

const CHIPS = 6;

/**
 * One row per server. The agents it writes to are chips under its name, not columns:
 * the list grows downwards as more CLIs gain MCP support, so it never scrolls sideways.
 */
export default function MCPServerList({ rows, targets, targetsOf, onToggle, onMenu, offTargets = mcpOffTargets, disabled = false, problems = {} }: Props) {
  const t = useT();
  const [open, setOpen] = useState<string[]>([]);

  return (
    <div className="ss-list">
      {rows.map((row) => {
        // A switch-only entry works in a few Agents, so it offers and counts only those.
        const offered = row.server?.disabled ? targets.filter((x) => offTargets.includes(x)) : targets;
        const selected = row.server ? targetsOf(row.name).filter((x) => offered.includes(x)) : [];
        const expanded = open.includes(row.name);
        const http = Boolean(row.server?.url);
        const piOptions = Object.keys(row.server?.piOptions ?? {}).length > 0;
        const targetPill = <TargetPill selected={selected} text={`${selected.length}/${offered.length}`} expanded={expanded} label={t('mcp.chooseAgents', { name: row.name })} onClick={() => setOpen((prev) => (expanded ? prev.filter((x) => x !== row.name) : [...prev, row.name]))} />;
        return (
          <Fragment key={row.name}>
            <div className="ss-r !items-stretch !flex-col !gap-2 !py-3">
              <span className="flex min-w-0 items-center gap-2">
                <span title={row.name} className={`max-w-[45%] shrink-0 truncate font-mono font-semibold ${row.server ? '' : 'text-ink-3 line-through'}`}>{row.name}</span>
                {row.server && Object.values(row.cells).some(writes) && <span className="ss-tag warn">{t('plugins.pending')}</span>}
                {row.server && !row.server.disabled && row.server.targets?.length === 0 && <span className="ss-tag">{t('plugins.noAgentsYet')}</span>}
                {problems[row.name] && <MCPCheckTag findings={problems[row.name]} />}
                {!row.server && <span className="ss-st bad">{t('mcp.removedFromSource')}</span>}
                <span className="min-w-0 flex-1 truncate font-mono text-xs text-ink-3">{row.server && !row.server.disabled && describeEndpoint(row.server)}</span>
                {row.server && targetPill}
                {row.server && (
                  <button type="button" className="ss-ib -my-1.5 -mr-2" aria-label={t('mcp.moreActions', { name: row.name })} onClick={(e) => onMenu(e, row.name)}>
                    <Ellipsis size={16} />
                  </button>
                )}
              </span>
              {!row.server || row.server.disabled ? (
                <span className="flex min-w-0 items-center gap-2 text-xs text-ink-3">
                  <span className="truncate">{row.server ? t('mcp.offHere') : t('mcp.removedHint')}</span>
                </span>
              ) : selected.length > 0 && (
                <span className="flex min-w-0 flex-wrap items-center gap-1.5">
                  {selected.slice(0, CHIPS).map((target) => (
                    <span key={target} className="inline-flex h-6 min-w-0 max-w-full items-center gap-1.5 whitespace-nowrap rounded-full bg-sunken pl-1.5 pr-2.5 text-xs text-ink">
                      <AgentIcon target={target} size={14} />
                      {targetLabel(target)}
                      {/* Only whether other Pi settings exist, never their contents: piOptions may hold anything. */}
                      {target === 'pi' && <>
                        <span className="text-ink-2">· {String(row.server?.piOptions?.exposure ?? 'codemode')}</span>
                        {piOptions && <span className="text-ink-3">· {t('mcp.piOptions')}</span>}
                      </>}
                    </span>
                  ))}
                  {selected.length > CHIPS && <span className="text-xs text-ink-3">+{selected.length - CHIPS}</span>}
                </span>
              )}
            </div>
            {problems[row.name] && <MCPCheckFindings findings={problems[row.name]} />}
            {expanded && row.server && (
              <div className="ss-r fold !min-h-0 flex-wrap gap-x-6 gap-y-3.5 !py-3.5">
                <TargetToggles offered={offered} selected={selected} http={http} disabled={disabled} onToggle={(target, on) => onToggle(row.name, target, on)} />
              </div>
            )}
          </Fragment>
        );
      })}
    </div>
  );
}
