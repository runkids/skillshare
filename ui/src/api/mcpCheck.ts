import { apiFetch } from './client';

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

export const mcpCheckApi = {
  /** Read-only. Findings that are errors still resolve; only a check that cannot run rejects. */
  run: () => apiFetch<MCPCheckReport>('/mcp/check'),
  /** Starts the saved server (every scope's of that name) and asks for its tools. */
  live: (name: string) => apiFetch<MCPCheckReport>(`/mcp/check?live=1&name=${encodeURIComponent(name)}`),
};
