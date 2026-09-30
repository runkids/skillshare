import { Info } from 'lucide-react';
import { useT } from '../../i18n';
import { parseToolNotice, targetLabel, toolGapKey } from './mcpView';

// The server words these for the CLI. Each is matched by how it starts. The servers it names are
// in parentheses: the first pair, or one pair per setting. A project server's root is nested in its own.
const noticeKeys: [prefix: string, key: string, names?: 'first' | 'all'][] = [
  ['piExtension is ignored', 'mcp.notice.piMode', 'first'],
  ['piOptionsPrune is ignored', 'mcp.notice.piPrune', 'first'],
  ["Pi's built-in MCP does not read these pi-mcp-adapter piOptions", 'mcp.notice.adapterOptions', 'all'],
  ['Pi cannot turn off a global server per project', 'mcp.notice.piSwitch', 'first'],
  ["pi-mcp-adapter's directTools, includeTools and excludeTools are converted", 'mcp.notice.adapterToolsConverted'],
  ['these pi-mcp-adapter tool settings are ignored', 'mcp.notice.adapterToolsDropped'],
];

/** What each top-level pair of parentheses holds. */
const groupsIn = (text: string) => {
  const names: string[] = [];
  let depth = 0;
  let start = 0;
  for (let i = 0; i < text.length; i++) {
    if (text[i] === '(' && depth++ === 0) start = i + 1;
    else if (text[i] === ')' && depth > 0 && --depth === 0) names.push(text.slice(start, i));
  }
  return names;
};

const describeNotice = (t: (key: string, params?: Record<string, string>) => string, notice: string) => {
  const match = noticeKeys.find(([prefix]) => notice.startsWith(prefix));
  if (!match) return t('mcp.notice.other');
  const [, key, names] = match;
  const groups = groupsIn(notice);
  // One server can appear under several settings; it is named once.
  const listed = [...new Set((names === 'first' ? groups.slice(0, 1) : groups).flatMap((g) => g.split(', ')))];
  return t(key, names ? { names: listed.join(', ') } : undefined);
};

/** One line per server whose tool policy some Agents do not apply, naming each Agent and the parts it leaves out. */
const describeToolNotices = (t: (key: string, params?: Record<string, string>) => string, notices: string[]) => {
  const byServer = new Map<string, string[]>();
  for (const notice of notices) {
    const parsed = parseToolNotice(notice);
    if (!parsed) continue;
    const agent = t('mcp.tools.notAppliedFor', { agent: targetLabel(parsed.target), parts: parsed.gaps.map((gap) => t(toolGapKey(gap))).join(t('mcp.tools.partSeparator')) });
    for (const name of parsed.names) byServer.set(name, [...(byServer.get(name) ?? []), agent]);
  }
  return [...byServer].map(([name, agents]) => t('mcp.notice.toolPolicy', { name, agents: agents.join(t('mcp.tools.agentSeparator')) }));
};

/** Settings from before 0.23.0 the config still has, and tool policies some Agents do not apply. Neither is an error, so this only explains. */
export default function MCPNotices({ notices = [] }: { notices?: string[] }) {
  const t = useT();
  if (notices.length === 0) return null;
  const tools = notices.filter((n) => parseToolNotice(n));
  const other = notices.filter((n) => !parseToolNotice(n));
  return (
    <div className="ss-note">
      <Info size={16} className="mt-0.5 shrink-0 text-ink-2" />
      <div className="flex flex-1 flex-col gap-1.5">
        {[...new Set([...other.map((n) => describeNotice(t, n)), ...describeToolNotices(t, tools)])].map((text) => <span key={text}>{text}</span>)}
      </div>
    </div>
  );
}
