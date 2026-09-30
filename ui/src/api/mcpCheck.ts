import { apiFetch } from './client';
import type { MCPMutation } from './mcp';

/** One result of `skillshare mcp check`. `subject` names the variable, command, or host; never a value. */
export interface MCPCheckFinding {
  level: 'error' | 'warning' | 'info';
  check: string;
  target: string;
  message: string;
  subject?: string;
}
/** `project` is the mcp.projects root a server belongs to; absent for a global server. */
/** `live`: what the server reported when probed, only with `?live=1`. */
export interface MCPCheckServer { name: string; project?: string; ok: boolean; findings: MCPCheckFinding[]; live?: { tools: number; toolNames?: string[] } }
export interface MCPCheckReport { servers: MCPCheckServer[]; summary: { errors: number; warnings: number } }
/** Why a probe failed, for the dashboard to phrase; `error` is the raw detail. */
export type MCPProbeErrorKind = 'connect' | 'timeout' | 'auth' | 'protocol' | 'command' | 'unknown';
export interface MCPProbe { live?: { tools: number; toolNames?: string[] }; errorKind?: MCPProbeErrorKind; error?: string }

export const mcpCheckApi = {
  /** Read-only. Findings that are errors still resolve; only a check that cannot run rejects. */
  run: () => apiFetch<MCPCheckReport>('/mcp/check'),
  /** Starts the server as the form describes it, saved or not, and asks for its tools. Saves nothing. */
  probe: (mutation: MCPMutation) => apiFetch<MCPProbe>('/mcp/probe', { method: 'POST', body: JSON.stringify({ mutation }) }),
};
