import { mcpOffTargets, type MCPServer, type MCPToolPolicy, type MCPValue } from '../../api/mcp';
import type { useT } from '../../i18n';
import { denyRemovesAll, hasToolPolicy, joinCommand, parsePiOptions, splitCommand, targetLabel } from './mcpView';

// Mirrors mcp.serverName.
const NAME = /^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$/;
const offTargetSet = new Set(mcpOffTargets);

export interface EnvRow { id: string; key: string; fromEnv: boolean; value: string }

const envRows = (env: Record<string, MCPValue> = {}): EnvRow[] =>
  Object.entries(env).map(([key, v]) => ({ id: crypto.randomUUID(), key, fromEnv: typeof v !== 'string', value: typeof v === 'string' ? v : v.fromEnv }));

export interface ServerDraft {
  name: string;
  http: boolean;
  piOptions: string;
  /** Other Agents' native fields as typed JSON, by Agent. */
  agentOptions: Record<string, string>;
  /** Kept as saved so an edit never drops it; empty parts are left out when saving. */
  tools: MCPToolPolicy;
  command: string;
  env: EnvRow[];
  headers: EnvRow[];
  url: string;
  tokenEnv: string;
  targets: string[];
}

export type DraftPatch = (patch: Partial<ServerDraft>) => void;

export function initialServerDraft(server: MCPServer | undefined, name: string, defaultTargets: string[], off: boolean): ServerDraft {
  return {
    name, http: Boolean(server?.url),
    piOptions: server?.piOptions ? JSON.stringify(server.piOptions, null, 2) : '',
    agentOptions: Object.fromEntries(Object.entries(server?.options ?? {}).map(([agent, fields]) => [agent, JSON.stringify(fields, null, 2)])),
    tools: server?.tools ?? {},
    command: server?.command ? joinCommand([server.command, ...(server.args ?? [])]) : '',
    env: envRows(server?.env), headers: envRows(server?.headers), url: server?.url ?? '', tokenEnv: server?.bearerToken?.fromEnv ?? '',
    targets: server?.targets ?? (off ? defaultTargets.filter((x) => offTargetSet.has(x)) : defaultTargets),
  };
}

/** How a new server is entered: fields, a pasted configuration, or Skillshare itself (`skillshare mcp serve`). */
export type AddMode = 'form' | 'paste' | 'serve';

/** The command that serves skills over MCP: every enabled skill, or those of one skills target. */
export const serveSkillsCommand = (target: string, project: boolean) =>
  joinCommand(['skillshare', 'mcp', 'serve', ...(target ? ['--target', target] : []), ...(project ? ['-p'] : [])]);

/** The skills target a `skillshare mcp serve` command serves ('' for every skill), or undefined for any other command. */
export function serveSkillsTarget(words: string[]): string | undefined {
  if (words[0] !== 'skillshare' || words[1] !== 'mcp' || words[2] !== 'serve') return undefined;
  let target = '';
  for (let i = 3; i < words.length; i++) {
    if (words[i] === '-p' || words[i] === '--project') continue;
    if (words[i] === '--target' && i + 1 < words.length) { target = words[++i]; continue; }
    return undefined;
  }
  return target;
}

/** Whether a source server runs `skillshare mcp serve`, so the dialog edits it in the Skillshare tab. */
export const isServeCommand = (server?: MCPServer) =>
  Boolean(server?.command && !server.url && serveSkillsTarget([server.command, ...(server.args ?? [])]) !== undefined);

/** Why Pi's other settings cannot be saved, or '' when they can. */
export const piOptionsError = (options: ReturnType<typeof parsePiOptions>, t: ReturnType<typeof useT>) =>
  options.invalid ? t('mcp.piOptionsInvalid')
    : options.taken ? t('mcp.piOptionsTaken', { field: options.taken })
    : options.adapterTools ? t('mcp.piOptionsAdapterTools', { field: options.adapterTools })
    : options.adapter ? t('mcp.piOptionsAdapter', { field: options.adapter })
    : options.overlap ? t('mcp.piOptionsToolsOverlap', { field: options.overlap })
    : options.bad ? t('mcp.piOptionsBad', { field: options.bad }) : '';

export function validateServerDraft(draft: ServerDraft, off: boolean, editing: boolean, existingNames: string[], t: ReturnType<typeof useT>) {
  const { name, targets, command, http, url, piOptions, tools } = draft;
  const trimmed = name.trim();
  const taken = !editing && existingNames.includes(trimmed);
  const nameError = trimmed && (!NAME.test(trimmed) || (targets.includes('pi') && trimmed.includes('.'))) ? t(targets.includes('pi') ? 'mcp.piNameHint' : 'mcp.nameHint') : taken ? t('mcp.nameTaken') : '';
  const words = splitCommand(command);
  // Pi settings stay in the source while Pi is unticked, so ticking it again brings them back (#289).
  const options = off ? {} : parsePiOptions(piOptions, hasToolPolicy(tools));
  const optionsError = piOptionsError(options, t);
  const toolsError = !off && denyRemovesAll(tools) ? t('mcp.tools.denyAll') : '';
  // Empty text drops an Agent; the server checks the fields themselves when saving.
  const agentOptions: Record<string, Record<string, unknown>> = {};
  let agentOptionsError = '';
  for (const [agent, text] of off ? [] : Object.entries(draft.agentOptions)) {
    if (!text.trim()) continue;
    let value: unknown;
    try { value = JSON.parse(text); } catch { value = undefined; }
    if (!value || typeof value !== 'object' || Array.isArray(value)) agentOptionsError ||= t('mcp.agentOptionsInvalid', { agent: targetLabel(agent) });
    else if (Object.keys(value).length > 0) agentOptions[agent] = value as Record<string, unknown>;
  }
  const canSave = Boolean(trimmed) && !nameError && (targets.length > 0 || !off) && (off || (http ? url.trim() !== '' : words.length > 0)) && !optionsError && !toolsError && !agentOptionsError;
  const title = t(off ? (editing ? 'mcp.editOff' : 'mcp.addOff') : (editing ? 'mcp.editServer' : 'mcp.addServer'));
  const complete = Boolean(trimmed) && !nameError && targets.length > 0 && (off || (http ? url.trim() !== '' : words.length > 0));
  return { trimmed, nameError, words, options, optionsError, toolsError, agentOptions, agentOptionsError, canSave, title, complete };
}

export type ServerValidation = ReturnType<typeof validateServerDraft>;
