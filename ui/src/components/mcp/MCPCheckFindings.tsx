import { CircleX, TriangleAlert } from 'lucide-react';
import type { MCPCheckFinding } from '../../api/mcpCheck';
import AgentIcon from '../AgentIcon';
import { useT } from '../../i18n';
import { describeMessage, parseToolNotice, toolGapKey } from './mcpView';

const notSynced = 'not synced yet; run skillshare sync mcp';

/** The finding in plain words. Checks without a sentence of their own keep the server's message. */
function useFindingText() {
  const t = useT();
  return (f: MCPCheckFinding) => {
    if (f.subject && (f.check === 'env' || f.check === 'command' || f.check === 'dns')) return t(`mcp.check.finding.${f.check}`, { subject: f.subject });
    if (f.check === 'sync' && f.message === notSynced) return t('mcp.check.finding.notSynced');
    const tools = f.check === 'tools' ? parseToolNotice(f.message) : undefined;
    if (tools) return t('mcp.tools.notAppliedHere', { parts: tools.gaps.map((gap) => t(toolGapKey(gap))).join(t('mcp.tools.partSeparator')) });
    return describeMessage(t, f.message);
  };
}

/** Beside the server name: its error count, or its warning count when it has no error. */
export function MCPCheckTag({ findings }: { findings: MCPCheckFinding[] }) {
  const t = useT();
  const errors = findings.filter((f) => f.level === 'error').length;
  if (errors > 0) return <span className="ss-tag bad">{t('mcp.check.tag.error', { count: errors })}</span>;
  return <span className="ss-tag warn">{t('mcp.check.tag.warning', { count: findings.length })}</span>;
}

/** One line per finding under the server's row, with the Agent it is about. */
export default function MCPCheckFindings({ findings }: { findings: MCPCheckFinding[] }) {
  const text = useFindingText();
  return (
    <div className="ss-r fold !min-h-0 flex-col !items-stretch !gap-1 !py-2.5 !pl-[54px]">
      {findings.map((f, i) => (
        <div key={i} className="flex min-h-[26px] items-center gap-[9px]">
          {f.level === 'error' ? <CircleX size={15} className="shrink-0 text-bad" /> : <TriangleAlert size={15} className="shrink-0 text-warn" />}
          {f.target && <span className="ss-at !h-[22px] !w-[22px] !rounded-md"><AgentIcon target={f.target} size={13} /></span>}
          <span className="min-w-0">{text(f)}</span>
        </div>
      ))}
    </div>
  );
}
