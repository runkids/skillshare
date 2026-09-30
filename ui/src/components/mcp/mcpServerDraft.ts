import { mcpOffTargets, type MCPServer, type MCPValue } from '../../api/mcp';
import type { useT } from '../../i18n';
import { joinCommand, parsePiOptions, splitCommand } from './mcpView';

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
    command: server?.command ? joinCommand([server.command, ...(server.args ?? [])]) : '',
    env: envRows(server?.env), headers: envRows(server?.headers), url: server?.url ?? '', tokenEnv: server?.bearerToken?.fromEnv ?? '',
    targets: server?.targets ?? (off ? defaultTargets.filter((x) => offTargetSet.has(x)) : defaultTargets),
  };
}

/** Why Pi's other settings cannot be saved, or '' when they can. */
export const piOptionsError = (options: ReturnType<typeof parsePiOptions>, t: ReturnType<typeof useT>) =>
  options.invalid ? t('mcp.piOptionsInvalid') : options.taken ? t('mcp.piOptionsTaken', { field: options.taken }) : options.bad ? t('mcp.piOptionsBad', { field: options.bad }) : '';

export function validateServerDraft(draft: ServerDraft, off: boolean, editing: boolean, existingNames: string[], t: ReturnType<typeof useT>) {
  const { name, targets, command, http, url, piOptions } = draft;
  const trimmed = name.trim();
  const taken = !editing && existingNames.includes(trimmed);
  const nameError = trimmed && (!NAME.test(trimmed) || (targets.includes('pi') && trimmed.includes('.'))) ? t(targets.includes('pi') ? 'mcp.piNameHint' : 'mcp.nameHint') : taken ? t('mcp.nameTaken') : '';
  const words = splitCommand(command);
  // Pi settings stay in the source while Pi is unticked, so ticking it again brings them back (#289).
  const options = off ? {} : parsePiOptions(piOptions);
  const optionsError = piOptionsError(options, t);
  const canSave = Boolean(trimmed) && !nameError && (targets.length > 0 || !off) && (off || (http ? url.trim() !== '' : words.length > 0)) && !optionsError;
  const title = t(off ? (editing ? 'mcp.editOff' : 'mcp.addOff') : (editing ? 'mcp.editServer' : 'mcp.addServer'));
  const complete = Boolean(trimmed) && !nameError && targets.length > 0 && (off || (http ? url.trim() !== '' : words.length > 0));
  return { trimmed, nameError, words, options, optionsError, canSave, title, complete };
}

export type ServerValidation = ReturnType<typeof validateServerDraft>;
