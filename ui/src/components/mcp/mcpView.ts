import type { MCPPlan, MCPServer } from '../../api/mcp';

export type MCPChange = MCPPlan['changes'][number];

export interface MatrixRow {
  name: string;
  /** Undefined when the entry was removed from the source but is still in Agent files. */
  server?: MCPServer;
  cells: Record<string, MCPChange>;
}

export const statusVariant: Record<string, 'default' | 'success' | 'warning' | 'danger' | 'info'> = {
  add: 'info', update: 'info', restore: 'info', unchanged: 'success', conflict: 'warning', remove: 'danger',
};

/** One row per source server, plus rows for entries the plan will remove from Agents. */
export function buildMatrix(servers: Record<string, MCPServer>, plan: MCPPlan | null): MatrixRow[] {
  const rows = new Map<string, MatrixRow>(Object.entries(servers).map(([name, server]) => [name, { name, server, cells: {} }]));
  for (const change of plan?.changes ?? []) {
    const row = rows.get(change.name) ?? { name: change.name, cells: {} };
    row.cells[change.target] = change;
    rows.set(change.name, row);
  }
  return [...rows.values()];
}

export function countActions(changes: MCPChange[]): Record<string, number> {
  const counts: Record<string, number> = {};
  for (const change of changes) counts[change.action] = (counts[change.action] ?? 0) + 1;
  return counts;
}

export function groupByFile(changes: MCPChange[]) {
  const files = new Map<string, { target: string; path: string; changes: MCPChange[] }>();
  for (const change of changes) {
    const file = files.get(change.path) ?? { target: change.target, path: change.path, changes: [] };
    file.changes.push(change);
    files.set(change.path, file);
  }
  return [...files.values()];
}

// ponytail: keyed on the backend Change.Message text; add a reason code to mcp.Change if these start drifting.
const conflictKeys: Record<string, string> = {
  'managed by another Skillshare config': 'mcp.conflictOtherConfig',
  'Agent configuration changed; import it or explicitly replace this entry': 'mcp.conflictChanged',
  'existing entry is not managed; import it to explicitly adopt it': 'mcp.conflictUnmanaged',
  'entry changed after the backup; restore would overwrite newer changes': 'mcp.conflictAfterBackup',
};

export const describeMessage = (t: (key: string) => string, message = '') =>
  conflictKeys[message] ? t(conflictKeys[message]) : message;

/** Conflicts the user can settle by importing the Agent entry or replacing it with the source. */
export const isResolvable = (change: MCPChange) =>
  change.action === 'conflict' && ['mcp.conflictChanged', 'mcp.conflictUnmanaged'].includes(conflictKeys[change.message ?? '']);

/** Display names for the MCP clients, as in their own docs. */
export const targetLabel = (target: string) =>
  ({ pi: 'Pi', claude: 'Claude', codex: 'Codex', cursor: 'Cursor', vscode: 'VS Code', opencode: 'OpenCode', grok: 'Grok', antigravity: 'Antigravity', amp: 'Amp', 'claude-desktop': 'Claude Desktop', cline: 'Cline (VS Code)', copilot: 'Copilot CLI', factory: 'Factory', gemini: 'Gemini CLI', goose: 'Goose', junie: 'Junie', kiro: 'Kiro', lmstudio: 'LM Studio', warp: 'Warp', windsurf: 'Windsurf' })[target] ?? target;

/** Splits a command line into words, honouring single and double quotes. */
// ponytail: no backslash escapes; a word holding both quote kinds needs the YAML config.
export function splitCommand(line: string): string[] {
  const words: string[] = [];
  let word = '';
  let quote = '';
  let started = false;
  for (const ch of line) {
    if (quote) {
      if (ch === quote) quote = '';
      else word += ch;
    } else if (ch === '"' || ch === "'") {
      quote = ch;
      started = true;
    } else if (/\s/.test(ch)) {
      if (started) words.push(word);
      word = '';
      started = false;
    } else {
      word += ch;
      started = true;
    }
  }
  if (started) words.push(word);
  return words;
}

export const joinCommand = (words: string[]) =>
  words.map(w => (w === '' || /[\s"']/.test(w) ? (w.includes("'") ? `"${w}"` : `'${w}'`) : w)).join(' ');

export const describeEndpoint = (server: MCPServer) => server.url ?? joinCommand([server.command ?? '', ...(server.args ?? [])]);

/** Backup IDs start with the Unix time in nanoseconds. */
export const backupTime = (id: string) => new Date(Number(id.split('-')[0]) / 1e6);
