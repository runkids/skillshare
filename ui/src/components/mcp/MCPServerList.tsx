import { Fragment, useState } from 'react';
import { Check, ChevronDown, Ellipsis, Plug } from 'lucide-react';
import AgentIcon from '../AgentIcon';
import { useT } from '../../i18n';
import { describeEndpoint, targetLabel, type MatrixRow } from './mcpView';

const STACK = 6;

interface Props {
  rows: MatrixRow[];
  targets: string[];
  targetsOf: (name: string) => string[];
  onToggle: (name: string, target: string, on: boolean) => void;
  onMenu: (e: React.MouseEvent<HTMLButtonElement>, name: string) => void;
}

/**
 * One row per server. The agents it writes to are chips inside the row, not columns:
 * the list grows downwards as more CLIs gain MCP support, so it never scrolls sideways.
 */
export default function MCPServerList({ rows, targets, targetsOf, onToggle, onMenu }: Props) {
  const t = useT();
  const [open, setOpen] = useState<string[]>([]);

  return (
    <div className="ss-list">
      {rows.map((row) => {
        const selected = row.server ? targetsOf(row.name).filter((x) => targets.includes(x)) : [];
        const expanded = open.includes(row.name);
        const http = Boolean(row.server?.url);
        return (
          <Fragment key={row.name}>
            <div className="ss-r">
              <span className="ss-cat sm mcp"><Plug size={14} /></span>
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                <span className="flex items-center gap-2">
                  <span title={row.name} className={`truncate font-mono font-semibold ${row.server ? '' : 'text-ink-3 line-through'}`}>{row.name}</span>
                  {row.server ? <span className="ss-tag">{http ? 'http' : 'stdio'}</span> : <span className="ss-st bad">{t('mcp.removedFromSource')}</span>}
                </span>
                <span className="truncate font-mono text-xs text-ink-3">{row.server ? describeEndpoint(row.server) : t('mcp.removedHint')}</span>
              </span>
              {row.server && (
                <>
                  {!expanded && selected.length > 0 && (
                    <span className="ss-stack" aria-hidden="true">
                      {selected.slice(0, STACK).map((target) => (
                        <span key={target} className="ss-at"><AgentIcon target={target} size={13} /></span>
                      ))}
                      {selected.length > STACK && <span className="ss-at text-[10px] font-semibold">+{selected.length - STACK}</span>}
                    </span>
                  )}
                  <button
                    type="button"
                    className="ss-btn sm"
                    aria-expanded={expanded}
                    aria-label={t('mcp.chooseAgents', { name: row.name })}
                    onClick={() => setOpen((prev) => (expanded ? prev.filter((x) => x !== row.name) : [...prev, row.name]))}
                  >
                    {selected.length}/{targets.length}
                    <ChevronDown size={14} className={expanded ? 'rotate-180' : ''} />
                  </button>
                  <button type="button" className="ss-ib" aria-label={t('mcp.moreActions', { name: row.name })} onClick={(e) => onMenu(e, row.name)}>
                    <Ellipsis size={16} />
                  </button>
                </>
              )}
            </div>
            {expanded && row.server && (
              <div className="ss-r fold !min-h-0 flex-wrap gap-x-6 gap-y-3.5 !py-3.5">
                {targets.map((target) => {
                  const on = selected.includes(target);
                  return (
                    <button
                      key={target}
                      type="button"
                      role="checkbox"
                      aria-checked={on}
                      className={`ss-tgl ${on ? 'on' : ''}`}
                      onClick={() => onToggle(row.name, target, !on)}
                      disabled={target === 'claude-desktop' && http && !on}
                    >
                      <span className="ic"><AgentIcon target={target} size={20} /><i><Check size={9} strokeWidth={3.5} /></i></span>
                      {targetLabel(target)}{target === 'claude-desktop' && <span className="ss-tag">stdio</span>}
                    </button>
                  );
                })}
              </div>
            )}
          </Fragment>
        );
      })}
    </div>
  );
}
